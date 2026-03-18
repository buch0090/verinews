package analyzer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/verinews/verinews/internal/claims"
	"github.com/verinews/verinews/internal/llm"
)

type SocraticAnalyzer struct {
	llm *llm.Client
}

func (a *SocraticAnalyzer) Mode() string { return "socratic" }

type SocraticOutput struct {
	MainClaim        string   `json:"main_claim"`
	Assumptions      []string `json:"assumptions"`
	MissingEvidence  []string `json:"missing_evidence"`
	WhatWouldDisprove string  `json:"what_would_disprove"`
	WhoBenefits      string   `json:"who_benefits"`
	QuestionsRaised  []string `json:"questions_raised"`
}

const socraticSystem = `You are applying Socratic method to a news article.

Your goal is to expose assumptions, gaps, and unexamined premises — not to judge truth, but to surface what is unquestioned.

Return a JSON object with:
- "main_claim": the central assertion the article is making (string)
- "assumptions": beliefs the article takes for granted without evidence (array of strings)
- "missing_evidence": what would strengthen or weaken the claims but is absent (array of strings)
- "what_would_disprove": what evidence or events would prove this article wrong (string)
- "who_benefits": who stands to gain if the reader believes this narrative (string)
- "questions_raised": the most important unanswered questions a critical reader should ask (array of strings)

Be specific. Do not summarise the article — interrogate it.
Return only valid JSON, no commentary.`

func (a *SocraticAnalyzer) Analyze(ctx context.Context, content string, cl []claims.Claim) (json.RawMessage, error) {
	userPrompt := fmt.Sprintf(
		"Article content:\n%s\n\nExtracted claims:\n%s",
		llm.Trim(content),
		formatClaims(cl),
	)

	var output SocraticOutput
	if err := a.llm.JSON(ctx, socraticSystem, userPrompt, &output); err != nil {
		return nil, err
	}
	return json.Marshal(output)
}
