package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/spf13/cobra"
	"github.com/verinews/verinews/internal/analyzer"
	"github.com/verinews/verinews/internal/claims"
	"github.com/verinews/verinews/internal/config"
	"github.com/verinews/verinews/internal/db"
	"github.com/verinews/verinews/internal/llm"
	"github.com/verinews/verinews/internal/opposition"
	"github.com/verinews/verinews/internal/pipeline"
	"github.com/verinews/verinews/internal/scraper"
	"github.com/verinews/verinews/internal/store"
	"github.com/verinews/verinews/internal/synthesis"
)

var analyzeFormat string

var analyzeCmd = &cobra.Command{
	Use:   "analyze <url>",
	Short: "Run the full pipeline on a URL and print results",
	Args:  cobra.ExactArgs(1),
	Run:   runAnalyze,
}

func init() {
	analyzeCmd.Flags().StringVarP(&analyzeFormat, "format", "f", "human", "Output format: human|json")
}

func runAnalyze(cmd *cobra.Command, args []string) {
	url := args[0]

	llmClient := llm.New(config.C.OpenAIKey)

	// Optional: DB persistence
	var st *store.Store
	if config.C.DatabaseURL != "" {
		conn, err := db.Connect(config.C.DatabaseURL)
		if err != nil {
			log.Printf("warning: DB connection failed (%v) — running without persistence", err)
		} else {
			st = store.New(conn)
		}
	}

	// Optional: NewsAPI opposition discovery
	var disc *opposition.Discoverer
	if config.C.NewsAPIKey != "" {
		disc = opposition.New(config.C.NewsAPIKey)
	}

	p := pipeline.New(
		scraper.New(),
		claims.NewExtractor(llmClient),
		analyzer.NewRegistry(llmClient),
		st,
		disc,
	)

	result, err := p.Run(context.Background(), url)
	if err != nil {
		log.Fatalf("pipeline error: %v", err)
	}

	switch analyzeFormat {
	case "json":
		printJSON(result)
	default:
		printHuman(result)
	}
}

func printJSON(result *pipeline.Result) {
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("marshaling result: %v", err)
	}
	fmt.Println(string(b))
}

func printHuman(result *pipeline.Result) {
	a := result.Article

	fmt.Println()
	fmt.Println(rule('═', 70))
	fmt.Printf("  %s\n", a.Title)
	fmt.Println(rule('═', 70))
	if a.Publisher != "" || a.Author != "" {
		fmt.Printf("  %s", a.Publisher)
		if a.Author != "" {
			fmt.Printf(" · %s", a.Author)
		}
		fmt.Println()
	}
	if a.PublishedAt != nil {
		fmt.Printf("  %s\n", a.PublishedAt.Format("2 Jan 2006"))
	}
	fmt.Printf("  %s\n", a.URL)
	if result.ArticleID != 0 {
		fmt.Printf("  DB id: %d\n", result.ArticleID)
	}

	fmt.Println()
	fmt.Println(rule('─', 70))
	fmt.Printf("  CLAIMS (%d)\n", len(result.Claims))
	fmt.Println(rule('─', 70))
	printClaims(result.Claims)

	for _, analysis := range result.Analyses {
		fmt.Println()
		fmt.Println(rule('─', 70))
		fmt.Printf("  %s\n", strings.ToUpper(analysis.Mode))
		fmt.Println(rule('─', 70))
		printAnalysis(analysis)
	}

	if len(result.RelatedArticles) > 0 {
		fmt.Println()
		fmt.Println(rule('─', 70))
		fmt.Printf("  RELATED COVERAGE (%d)\n", len(result.RelatedArticles))
		fmt.Println(rule('─', 70))
		for _, r := range result.RelatedArticles {
			fmt.Printf("  • %s\n", r.Title)
			fmt.Printf("    %s — %s\n", r.Publisher, r.URL)
		}
	}

	fmt.Println()
	fmt.Println(rule('═', 70))
	fmt.Printf("  TRUTH SCORE: %.1f / 100 — %s\n", result.TruthScore, synthesis.Label(result.TruthScore))
	fmt.Println(rule('═', 70))
	fmt.Println()
}

func printClaims(cl []claims.Claim) {
	for i, c := range cl {
		marker := "○"
		if c.EvidencePresent {
			marker = "●"
		}
		fmt.Printf("  %s %d. [%-11s] %s\n", marker, i+1, c.Type, c.Text)
	}
	fmt.Println()
	fmt.Println("  ● = evidence present  ○ = no evidence")
}

func printAnalysis(result analyzer.Result) {
	var m map[string]any
	if err := json.Unmarshal(result.Output, &m); err != nil {
		fmt.Printf("  (could not parse output: %v)\n", err)
		return
	}
	b, _ := json.MarshalIndent(m, "  ", "  ")
	fmt.Println(string(b))
}

func rule(ch rune, n int) string {
	return "  " + strings.Repeat(string(ch), n)
}
