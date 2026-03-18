package analyzer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/verinews/verinews/internal/claims"
	"github.com/verinews/verinews/internal/llm"
)

type StoicAnalyzer struct {
	llm *llm.Client
}

func (a *StoicAnalyzer) Mode() string { return "stoic" }

type StoicOutput struct {
	WithinControl      []string `json:"within_control"`
	OutsideControl     []string `json:"outside_control"`
	EmotionalTriggers  []string `json:"emotional_triggers"`
	OutrageEngineered  bool     `json:"outrage_engineered"`
	OutrageScore       float64  `json:"outrage_score"` // 0.0–1.0
	Signal             string   `json:"signal"`
	Noise              []string `json:"noise"`
}

const stoicSystem = `You are applying Stoic philosophy (Epictetus) to a news article.

The Stoics distinguished between what is within our control (our judgements and responses) and what is not (external events). Your task is to cut through emotional manipulation and identify what actually matters.

Return a JSON object with:
- "within_control": aspects of this situation that individuals or communities can actually act on (array of strings)
- "outside_control": events or forces in the story that are beyond individual influence (array of strings)
- "emotional_triggers": specific words, phrases, or framings designed to provoke fear, anger, or outrage (array of strings)
- "outrage_engineered": true if the article appears deliberately constructed to provoke emotional reaction rather than inform (bool)
- "outrage_score": 0.0 (purely informational) to 1.0 (maximally outrage-driven) (float)
- "signal": the genuinely important, actionable information in this article stripped of noise (string)
- "noise": emotional, sensational, or irrelevant elements that distract from the signal (array of strings)

Be precise about emotional_triggers — quote or closely paraphrase actual language from the article.
Return only valid JSON, no commentary.`

func (a *StoicAnalyzer) Analyze(ctx context.Context, content string, cl []claims.Claim) (json.RawMessage, error) {
	userPrompt := fmt.Sprintf(
		"Article content:\n%s\n\nExtracted claims:\n%s",
		llm.Trim(content),
		formatClaims(cl),
	)

	var output StoicOutput
	if err := a.llm.JSON(ctx, stoicSystem, userPrompt, &output); err != nil {
		return nil, err
	}
	return json.Marshal(output)
}
