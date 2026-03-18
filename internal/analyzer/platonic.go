package analyzer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/verinews/verinews/internal/claims"
	"github.com/verinews/verinews/internal/llm"
)

type PlatonicAnalyzer struct {
	llm *llm.Client
}

func (a *PlatonicAnalyzer) Mode() string { return "platonic" }

type PlatonicOutput struct {
	SurfaceNarrative    string   `json:"surface_narrative"`
	ShadowOnWall        string   `json:"shadow_on_wall"`
	DeeperReality       string   `json:"deeper_reality"`
	SystemicIncentives  []string `json:"systemic_incentives"`
	FramingDevices      []string `json:"framing_devices"`
}

const platonicSystem = `You are applying Plato's Allegory of the Cave to a news article.

In the allegory, prisoners mistake shadows on a wall for reality. Your task is to identify what "shadows" this article presents — the surface-level narrative — and what deeper structural reality may be obscured.

Return a JSON object with:
- "surface_narrative": what the article presents as the story (string)
- "shadow_on_wall": the simplified or distorted projection of events being shown to the reader (string)
- "deeper_reality": the underlying structural, systemic, or political reality the surface narrative may be masking (string)
- "systemic_incentives": institutional, economic, or political forces that shape how this story is told and what is emphasised (array of strings)
- "framing_devices": specific language, selection of facts, or narrative choices that steer the reader's interpretation (array of strings)

Do not assume malicious intent — systemic forces often operate without conscious manipulation.
Return only valid JSON, no commentary.`

func (a *PlatonicAnalyzer) Analyze(ctx context.Context, content string, cl []claims.Claim) (json.RawMessage, error) {
	userPrompt := fmt.Sprintf(
		"Article content:\n%s\n\nExtracted claims:\n%s",
		llm.Trim(content),
		formatClaims(cl),
	)

	var output PlatonicOutput
	if err := a.llm.JSON(ctx, platonicSystem, userPrompt, &output); err != nil {
		return nil, err
	}
	return json.Marshal(output)
}
