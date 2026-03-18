package pipeline

import (
	"context"
	"fmt"

	"github.com/verinews/verinews/internal/analyzer"
	"github.com/verinews/verinews/internal/claims"
	"github.com/verinews/verinews/internal/scraper"
)

type Result struct {
	Article  *scraper.Result
	Claims   []claims.Claim
	Analyses []analyzer.Result
}

type Pipeline struct {
	scraper   *scraper.Scraper
	extractor *claims.Extractor
	registry  *analyzer.Registry
}

func New(scraper *scraper.Scraper, extractor *claims.Extractor, registry *analyzer.Registry) *Pipeline {
	return &Pipeline{
		scraper:   scraper,
		extractor: extractor,
		registry:  registry,
	}
}

func (p *Pipeline) Run(ctx context.Context, url string) (*Result, error) {
	fmt.Println("→ scraping article...")
	article, err := p.scraper.Scrape(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("scrape: %w", err)
	}

	fmt.Println("→ extracting claims...")
	cl, err := p.extractor.Extract(ctx, article.Content)
	if err != nil {
		return nil, fmt.Errorf("extract claims: %w", err)
	}

	fmt.Printf("→ running philosophical analysis (%d claims found)...\n", len(cl))
	analyses, err := p.registry.RunAll(ctx, article.Content, cl)
	if err != nil {
		return nil, fmt.Errorf("analyze: %w", err)
	}

	return &Result{
		Article:  article,
		Claims:   cl,
		Analyses: analyses,
	}, nil
}
