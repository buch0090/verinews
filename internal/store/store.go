package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"net/url"

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

// UpsertArticle creates or returns the existing article. Returns (id, isNew, error).
func (s *Store) UpsertArticle(ctx context.Context, a *scraper.Result) (int64, bool, error) {
	hash := URLHash(a.URL)

	var id int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM articles WHERE url_hash = $1`, hash,
	).Scan(&id)
	if err == nil {
		return id, false, nil
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
