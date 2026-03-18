package analyzer

import (
	"fmt"
	"strings"

	"github.com/verinews/verinews/internal/claims"
)

// formatClaims renders claims as a compact numbered list for inclusion in prompts.
func formatClaims(cl []claims.Claim) string {
	if len(cl) == 0 {
		return "(no claims extracted)"
	}
	var b strings.Builder
	for i, c := range cl {
		fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, c.Type, c.Text)
	}
	return b.String()
}
