package analyzer

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/verinews/verinews/internal/claims"
	"github.com/verinews/verinews/internal/llm"
)

type Result struct {
	Mode   string          `json:"mode"`
	Output json.RawMessage `json:"output"`
}

type Analyzer interface {
	Mode() string
	Analyze(ctx context.Context, content string, claims []claims.Claim) (json.RawMessage, error)
}

type Registry struct {
	analyzers []Analyzer
}

func NewRegistry(llm *llm.Client) *Registry {
	return &Registry{
		analyzers: []Analyzer{
			&SocraticAnalyzer{llm: llm},
			&AristotelianAnalyzer{llm: llm},
			&PlatonicAnalyzer{llm: llm},
			&StoicAnalyzer{llm: llm},
		},
	}
}

// RunAll executes all analyzers concurrently and returns results in
// registration order. If any analyzer fails the whole set fails.
func (r *Registry) RunAll(ctx context.Context, content string, claims []claims.Claim) ([]Result, error) {
	results := make([]Result, len(r.analyzers))

	g, ctx := errgroup.WithContext(ctx)
	for i, a := range r.analyzers {
		i, a := i, a
		g.Go(func() error {
			output, err := a.Analyze(ctx, content, claims)
			if err != nil {
				return fmt.Errorf("%s analyzer: %w", a.Mode(), err)
			}
			results[i] = Result{Mode: a.Mode(), Output: output}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}
