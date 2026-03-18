package claims

import (
	"context"
	"fmt"

	"github.com/verinews/verinews/internal/llm"
)

type Claim struct {
	Text            string  `json:"text"`
	Type            string  `json:"type"`             // factual | opinion | predictive
	EvidencePresent bool    `json:"evidence_present"`
	ConfidenceScore float64 `json:"confidence_score"` // 0.0–1.0
}

type Extractor struct {
	llm *llm.Client
}

func NewExtractor(llm *llm.Client) *Extractor {
	return &Extractor{llm: llm}
}

const systemPrompt = `You are an analytical assistant that extracts claims from news articles.

Return a JSON object with a "claims" array. Each claim must have:
- "text": the specific claim being made (string)
- "type": one of "factual", "opinion", or "predictive"
- "evidence_present": true if the article provides supporting evidence or sources for this claim (bool)
- "confidence_score": how clearly this is stated as a claim, from 0.0 (implied) to 1.0 (explicit) (float)

Focus on substantive claims — skip filler, transitions, and obvious background. Aim for 5–15 claims per article.
Return only valid JSON, no commentary.`

type response struct {
	Claims []Claim `json:"claims"`
}

func (e *Extractor) Extract(ctx context.Context, articleContent string) ([]Claim, error) {
	userPrompt := fmt.Sprintf("Extract the claims from this news article:\n\n%s", llm.Trim(articleContent))

	var resp response
	if err := e.llm.JSON(ctx, systemPrompt, userPrompt, &resp); err != nil {
		return nil, fmt.Errorf("extracting claims: %w", err)
	}

	if err := validate(resp.Claims); err != nil {
		return nil, err
	}

	return resp.Claims, nil
}

func validate(claims []Claim) error {
	for i, c := range claims {
		if c.Text == "" {
			return fmt.Errorf("claim %d has empty text", i)
		}
		switch c.Type {
		case "factual", "opinion", "predictive":
		default:
			return fmt.Errorf("claim %d has invalid type %q", i, c.Type)
		}
		if c.ConfidenceScore < 0 || c.ConfidenceScore > 1 {
			return fmt.Errorf("claim %d confidence_score out of range: %f", i, c.ConfidenceScore)
		}
	}
	return nil
}
