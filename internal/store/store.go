package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/verinews/verinews/internal/analyzer"
	"github.com/verinews/verinews/internal/claims"
	"github.com/verinews/verinews/internal/scraper"
)

type RelatedArticle struct {
	URL                     string
	Title                   string
	Publisher               string
	Snippet                 string
	RelationType            string
	NarrativeDistanceScore  float64
	ClaimContradictionScore float64
}

type ArticleRow struct {
	ID         int64
	URL        string
	Title      string
	Publisher  string
	Author     string
	Status     string
	TruthScore float64
	CreatedAt  time.Time
}

type ClaimRow struct {
	Text            string
	Type            string
	EvidencePresent bool
	ConfidenceScore float64
}

type AnalysisRow struct {
	Mode   string
	Output json.RawMessage
}

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func URLHash(rawURL string) string {
	h := sha256.Sum256([]byte(rawURL))
	return fmt.Sprintf("%x", h)
}

// CreatePending inserts an article with status='queued' and returns its ID.
// If the article already exists, returns the existing ID.
func (s *Store) CreatePending(ctx context.Context, rawURL string) (int64, error) {
	hash := URLHash(rawURL)

	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM articles WHERE url_hash = $1`, hash).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("checking article: %w", err)
	}

	err = s.db.QueryRowContext(ctx,
		`INSERT INTO articles (url, url_hash, status) VALUES ($1, $2, 'queued') RETURNING id`,
		rawURL, hash,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("inserting pending article: %w", err)
	}
	return id, nil
}

// UpsertArticle creates or updates the article with scraped content. Returns (id, isNew, error).
func (s *Store) UpsertArticle(ctx context.Context, a *scraper.Result) (int64, bool, error) {
	hash := URLHash(a.URL)

	var id int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM articles WHERE url_hash = $1`, hash,
	).Scan(&id)
	if err == nil {
		// Exists — update with scraped content (e.g. created by CreatePending first)
		_, err = s.db.ExecContext(ctx,
			`UPDATE articles SET title=$1, publisher=$2, author=$3, published_at=$4,
			 raw_content=$5, cleaned_content=$6, status='processing' WHERE id=$7`,
			a.Title, a.Publisher, a.Author, a.PublishedAt, a.RawHTML, a.Content, id,
		)
		return id, false, err
	}
	if err != sql.ErrNoRows {
		return 0, false, fmt.Errorf("checking article: %w", err)
	}

	err = s.db.QueryRowContext(ctx,
		`INSERT INTO articles (url, url_hash, title, publisher, author, published_at, raw_content, cleaned_content, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'processing')
		 RETURNING id`,
		a.URL, hash, a.Title, a.Publisher, a.Author, a.PublishedAt,
		a.RawHTML, a.Content,
	).Scan(&id)
	if err != nil {
		return 0, false, fmt.Errorf("inserting article: %w", err)
	}
	return id, true, nil
}

func (s *Store) UpdateStatus(ctx context.Context, id int64, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE articles SET status = $1 WHERE id = $2`, status, id,
	)
	return err
}

func (s *Store) UpdateTruthScore(ctx context.Context, id int64, score float64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE articles SET truth_score = $1, status = 'complete' WHERE id = $2`, score, id,
	)
	return err
}

func (s *Store) SaveClaims(ctx context.Context, articleID int64, cl []claims.Claim) error {
	for _, c := range cl {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO claims (article_id, claim_text, claim_type, evidence_present, confidence_score)
			 VALUES ($1, $2, $3, $4, $5)`,
			articleID, c.Text, c.Type, c.EvidencePresent, c.ConfidenceScore,
		)
		if err != nil {
			return fmt.Errorf("saving claim: %w", err)
		}
	}
	return nil
}

func (s *Store) SaveAnalysis(ctx context.Context, articleID int64, result analyzer.Result) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO analyses (article_id, philosopher_mode, json_output)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (article_id, philosopher_mode)
		 DO UPDATE SET json_output = EXCLUDED.json_output, completed_at = NOW()`,
		articleID, result.Mode, []byte(result.Output),
	)
	return err
}

func (s *Store) SaveRelatedArticles(ctx context.Context, articleID int64, related []RelatedArticle) error {
	for _, r := range related {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO related_articles (article_id, url, title, publisher, snippet, relation_type, narrative_distance_score, claim_contradiction_score)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			articleID, r.URL, r.Title, r.Publisher, r.Snippet, r.RelationType,
			r.NarrativeDistanceScore, r.ClaimContradictionScore,
		)
		if err != nil {
			return fmt.Errorf("saving related article: %w", err)
		}
	}
	return nil
}

// SourceTrustScore looks up the domain trust score from source_credibility.
// Returns 0.5 (neutral) if the domain is not found.
func (s *Store) SourceTrustScore(ctx context.Context, rawURL string) float64 {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0.5
	}
	domain := u.Hostname()

	var score float64
	err = s.db.QueryRowContext(ctx,
		`SELECT trust_score FROM source_credibility WHERE domain = $1`, domain,
	).Scan(&score)
	if err != nil {
		return 0.5
	}
	return score
}

func (s *Store) ListArticles(ctx context.Context, limit int) ([]ArticleRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, url, title, publisher, author, status, truth_score, created_at
		 FROM articles WHERE status = 'complete'
		 ORDER BY created_at DESC LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ArticleRow
	for rows.Next() {
		var row ArticleRow
		var title, publisher, author sql.NullString
		var truthScore sql.NullFloat64
		if err := rows.Scan(&row.ID, &row.URL, &title, &publisher, &author, &row.Status, &truthScore, &row.CreatedAt); err != nil {
			return nil, err
		}
		row.Title = title.String
		row.Publisher = publisher.String
		row.Author = author.String
		row.TruthScore = truthScore.Float64
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *Store) GetArticle(ctx context.Context, id int64) (*ArticleRow, error) {
	var row ArticleRow
	var title, publisher, author sql.NullString
	var truthScore sql.NullFloat64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, url, title, publisher, author, status, truth_score, created_at
		 FROM articles WHERE id = $1`, id,
	).Scan(&row.ID, &row.URL, &title, &publisher, &author, &row.Status, &truthScore, &row.CreatedAt)
	if err != nil {
		return nil, err
	}
	row.Title = title.String
	row.Publisher = publisher.String
	row.Author = author.String
	row.TruthScore = truthScore.Float64
	return &row, nil
}

func (s *Store) GetClaims(ctx context.Context, articleID int64) ([]ClaimRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT claim_text, claim_type, evidence_present, confidence_score
		 FROM claims WHERE article_id = $1 ORDER BY id`, articleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ClaimRow
	for rows.Next() {
		var r ClaimRow
		var claimType sql.NullString
		if err := rows.Scan(&r.Text, &claimType, &r.EvidencePresent, &r.ConfidenceScore); err != nil {
			return nil, err
		}
		r.Type = claimType.String
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) GetAnalyses(ctx context.Context, articleID int64) ([]AnalysisRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT philosopher_mode, json_output
		 FROM analyses WHERE article_id = $1 ORDER BY philosopher_mode`, articleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []AnalysisRow
	for rows.Next() {
		var r AnalysisRow
		if err := rows.Scan(&r.Mode, &r.Output); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) GetRelatedArticlesByID(ctx context.Context, articleID int64) ([]RelatedArticle, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT url, title, publisher, snippet, relation_type, narrative_distance_score, claim_contradiction_score
		 FROM related_articles WHERE article_id = $1 ORDER BY id`, articleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []RelatedArticle
	for rows.Next() {
		var r RelatedArticle
		var title, publisher, snippet, relationType sql.NullString
		if err := rows.Scan(&r.URL, &title, &publisher, &snippet, &relationType,
			&r.NarrativeDistanceScore, &r.ClaimContradictionScore); err != nil {
			return nil, err
		}
		r.Title = title.String
		r.Publisher = publisher.String
		r.Snippet = snippet.String
		r.RelationType = relationType.String
		result = append(result, r)
	}
	return result, rows.Err()
}
