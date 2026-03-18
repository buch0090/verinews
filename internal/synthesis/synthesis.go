package synthesis

import (
	"context"
	"encoding/json"

	"github.com/verinews/verinews/internal/analyzer"
	"github.com/verinews/verinews/internal/claims"
	"github.com/verinews/verinews/internal/store"
)

// Input holds all pipeline outputs needed to compute a truth score.
type Input struct {
	ArticleURL string
	Claims     []claims.Claim
	Analyses   []analyzer.Result
	Related    []store.RelatedArticle
}

// Label returns the human-readable tier for a truth score.
func Label(score float64) string {
	switch {
	case score >= 81:
		return "Well-Supported"
	case score >= 61:
		return "Mostly Supported"
	case score >= 31:
		return "Disputed"
	default:
		return "Highly Contested"
	}
}

// Score computes a 0–100 truth confidence score from the pipeline outputs.
//
// Weights:
//
//	30% evidence depth  — claims backed by cited evidence, weighted by confidence
//	30% logic integrity — Aristotelian logic score
//	20% outrage inverted — low outrage = more trustworthy
//	15% source credibility — domain trust score (DB lookup, default 0.5)
//	 5% opposition coverage — more related articles found = more confidence
func Score(ctx context.Context, st *store.Store, in Input) float64 {
	evidenceDepth := computeEvidenceDepth(in.Claims)
	logicScore := extractLogicScore(in.Analyses)
	outrageInverted := 1.0 - extractOutrageScore(in.Analyses)

	sourceScore := 0.5
	if st != nil && in.ArticleURL != "" {
		sourceScore = st.SourceTrustScore(ctx, in.ArticleURL)
	}

	// 0 related = 0.0, 3+ = 1.0
	coverage := float64(len(in.Related)) / 3.0
	if coverage > 1.0 {
		coverage = 1.0
	}

	raw := 0.30*evidenceDepth + 0.30*logicScore + 0.20*outrageInverted + 0.15*sourceScore + 0.05*coverage
	return raw * 100
}

func computeEvidenceDepth(cl []claims.Claim) float64 {
	if len(cl) == 0 {
		return 0.5
	}
	var sum float64
	for _, c := range cl {
		if c.EvidencePresent {
			sum += c.ConfidenceScore
		}
	}
	// Denominator is total claims — evidence-free claims drag the score down.
	return sum / float64(len(cl))
}

func extractLogicScore(analyses []analyzer.Result) float64 {
	for _, a := range analyses {
		if a.Mode == "aristotelian" {
			var out struct {
				LogicScore float64 `json:"logic_score"`
			}
			if err := json.Unmarshal(a.Output, &out); err == nil {
				return out.LogicScore
			}
		}
	}
	return 0.5
}

func extractOutrageScore(analyses []analyzer.Result) float64 {
	for _, a := range analyses {
		if a.Mode == "stoic" {
			var out struct {
				OutrageScore float64 `json:"outrage_score"`
			}
			if err := json.Unmarshal(a.Output, &out); err == nil {
				return out.OutrageScore
			}
		}
	}
	return 0.5
}
