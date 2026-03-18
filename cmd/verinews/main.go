package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/verinews/verinews/internal/config"
)

var rootCmd = &cobra.Command{
	Use:   "verinews",
	Short: "VeriNews — philosophical truth engine for news",
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(config.Init)
	rootCmd.AddCommand(webCmd)
	rootCmd.AddCommand(workerCmd)
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(migrateCmd)
}
