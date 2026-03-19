package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/verinews/verinews/internal/analyzer"
	"github.com/verinews/verinews/internal/api"
	"github.com/verinews/verinews/internal/claims"
	"github.com/verinews/verinews/internal/config"
	"github.com/verinews/verinews/internal/db"
	"github.com/verinews/verinews/internal/llm"
	"github.com/verinews/verinews/internal/opposition"
	"github.com/verinews/verinews/internal/pipeline"
	"github.com/verinews/verinews/internal/scraper"
	"github.com/verinews/verinews/internal/store"
)

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Start the HTTP server",
	Run:   runWeb,
}

func runWeb(cmd *cobra.Command, args []string) {
	if config.C.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required for web mode")
	}

	conn, err := db.Connect(config.C.DatabaseURL)
	if err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}

	st := store.New(conn)
	llmClient := llm.New(config.C.OpenAIKey)

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

	srv, err := api.NewServer(p, st)
	if err != nil {
		log.Fatalf("building server: %v", err)
	}

	addr := fmt.Sprintf(":%s", config.C.AppPort)
	log.Printf("verinews web server listening on %s (env: %s)", addr, config.C.AppEnv)

	if err := http.ListenAndServe(addr, srv.Router()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
