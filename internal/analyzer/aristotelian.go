package analyzer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/verinews/verinews/internal/claims"
	"github.com/verinews/verinews/internal/llm"
)

type AristotelianAnalyzer struct {
	llm *llm.Client
}

func (a *AristotelianAnalyzer) Mode() string { return "aristotelian" }

type Fallacy struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AristotelianOutput struct {
	Premises       []string  `json:"premises"`
	Conclusion     string    `json:"conclusion"`
	ReasoningType  string    `json:"reasoning_type"` // inductive | deductive | mixed
	PremisesAreFactual bool  `json:"premises_are_factual"`
	Fallacies      []Fallacy `json:"fallacies"`
	LogicScore     float64   `json:"logic_score"` // 0.0 (broken) – 1.0 (sound)
}

const aristotelianSystem = `You are applying Aristotelian logic analysis to a news article.

Your goal is to evaluate the argument structure: what is being claimed, what it rests on, and whether the reasoning holds.

Return a JSON object with:
- "premises": the supporting statements the article relies on to reach its conclusion (array of strings)
- "conclusion": the main position the article argues toward (string)
- "reasoning_type": one of "inductive", "deductive", or "mixed"
- "premises_are_factual": true if the premises are presented as verifiable facts, false if interpretive (bool)
- "fallacies": logical fallacies detected, each with "name" and "description" (array of objects, empty if none)
- "logic_score": 0.0 to 1.0 rating of overall argument soundness — 1.0 means premises are solid and reasoning is valid, 0.0 means the argument collapses under scrutiny (float)

Common fallacies to watch for: ad hominem, straw man, false dichotomy, appeal to authority, post hoc, slippery slope, hasty generalisation, appeal to emotion.
Return only valid JSON, no commentary.`

func (a *AristotelianAnalyzer) Analyze(ctx context.Context, content string, cl []claims.Claim) (json.RawMessage, error) {
	userPrompt := fmt.Sprintf(
		"Article content:\n%s\n\nExtracted claims:\n%s",
		llm.Trim(content),
		formatClaims(cl),
	)

	var output AristotelianOutput
	if err := a.llm.JSON(ctx, aristotelianSystem, userPrompt, &output); err != nil {
		return nil, err
	}
	return json.Marshal(output)
}
