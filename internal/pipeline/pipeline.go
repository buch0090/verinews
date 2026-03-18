package pipeline

import (
	"context"
	"fmt"

	"github.com/verinews/verinews/internal/analyzer"
	"github.com/verinews/verinews/internal/claims"
	"github.com/verinews/verinews/internal/opposition"
	"github.com/verinews/verinews/internal/scraper"
	"github.com/verinews/verinews/internal/store"
	"github.com/verinews/verinews/internal/synthesis"
)

type Result struct {
	ArticleID      int64
	Article        *scraper.Result
	Claims         []claims.Claim
	Analyses       []analyzer.Result
	RelatedArticles []store.RelatedArticle
	TruthScore     float64
}

type Pipeline struct {
	scraper    *scraper.Scraper
	extractor  *claims.Extractor
	registry   *analyzer.Registry
	store      *store.Store          // nil = no DB persistence
	discoverer *opposition.Discoverer // nil = skip opposition search
}

func New(
	scraper *scraper.Scraper,
	extractor *claims.Extractor,
	registry *analyzer.Registry,
	st *store.Store,
	disc *opposition.Discoverer,
) *Pipeline {
	return &Pipeline{
		scraper:    scraper,
		extractor:  extractor,
		registry:   registry,
		store:      st,
		discoverer: disc,
	}
}

func (p *Pipeline) Run(ctx context.Context, url string) (*Result, error) {
	// 1. Scrape
	fmt.Println("→ scraping article...")
	article, err := p.scraper.Scrape(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("scrape: %w", err)
	}

	// 2. Persist article (with dedup)
	var articleID int64
	if p.store != nil {
		id, isNew, err := p.store.UpsertArticle(ctx, article)
		if err != nil {
			return nil, fmt.Errorf("save article: %w", err)
		}
		articleID = id
		if !isNew {
			fmt.Printf("  (article already in DB, id=%d)\n", id)
		}
	}

	// 3. Extract claims
	fmt.Println("→ extracting claims...")
	cl, err := p.extractor.Extract(ctx, article.Content)
	if err != nil {
		return nil, fmt.Errorf("extract claims: %w", err)
	}
	if p.store != nil && articleID != 0 {
		if err := p.store.SaveClaims(ctx, articleID, cl); err != nil {
			fmt.Printf("  warning: saving claims: %v\n", err)
		}
	}

	// 4. Philosophical analysis
	fmt.Printf("→ running philosophical analysis (%d claims found)...\n", len(cl))
	analyses, err := p.registry.RunAll(ctx, article.Content, cl)
	if err != nil {
		return nil, fmt.Errorf("analyze: %w", err)
	}
	if p.store != nil && articleID != 0 {
		for _, a := range analyses {
			if err := p.store.SaveAnalysis(ctx, articleID, a); err != nil {
				fmt.Printf("  warning: saving analysis %s: %v\n", a.Mode, err)
			}
		}
	}

	// 5. Opposition discovery
	var related []store.RelatedArticle
	if p.discoverer != nil && article.Title != "" {
		fmt.Println("→ searching for related coverage...")
		related, err = p.discoverer.Discover(ctx, article.Title)
		if err != nil {
			fmt.Printf("  warning: opposition discovery: %v\n", err)
		} else {
			fmt.Printf("  found %d related articles\n", len(related))
			if p.store != nil && articleID != 0 {
				if err := p.store.SaveRelatedArticles(ctx, articleID, related); err != nil {
					fmt.Printf("  warning: saving related articles: %v\n", err)
				}
			}
		}
	}

	// 6. Truth synthesis
	fmt.Println("→ computing truth score...")
	truthScore := synthesis.Score(ctx, p.store, synthesis.Input{
		ArticleURL: article.URL,
		Claims:     cl,
		Analyses:   analyses,
		Related:    related,
	})

	if p.store != nil && articleID != 0 {
		if err := p.store.UpdateTruthScore(ctx, articleID, truthScore); err != nil {
			fmt.Printf("  warning: updating truth score: %v\n", err)
		}
	}

	return &Result{
		ArticleID:       articleID,
		Article:         article,
		Claims:          cl,
		Analyses:        analyses,
		RelatedArticles: related,
		TruthScore:      truthScore,
	}, nil
}
