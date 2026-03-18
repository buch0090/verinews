package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/verinews/verinews/internal/config"
)

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Start the HTTP server",
	Run:   runWeb,
}

func runWeb(cmd *cobra.Command, args []string) {
	addr := fmt.Sprintf(":%s", config.C.AppPort)
	log.Printf("starting verinews web server on %s (env: %s)", addr, config.C.AppEnv)

	// TODO: wire chi router
	if err := http.ListenAndServe(addr, http.NewServeMux()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
