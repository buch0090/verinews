package main

import (
	"io/fs"
	"log"

	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"
	"github.com/verinews/verinews/internal/config"
	"github.com/verinews/verinews/internal/db"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Database migration commands",
}

var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Run all pending migrations",
	Run:   runMigrateUp,
}

var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Roll back the last migration",
	Run:   runMigrateDown,
}

var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print migration status",
	Run:   runMigrateStatus,
}

func init() {
	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateDownCmd)
	migrateCmd.AddCommand(migrateStatusCmd)
}

func migrateConn() *goose.Provider {
	conn, err := db.Connect(config.C.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	migrationsFS, err := fs.Sub(db.MigrationsFS, "migrations")
	if err != nil {
		log.Fatalf("migrations fs: %v", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, conn, migrationsFS)
	if err != nil {
		log.Fatalf("goose provider: %v", err)
	}
	return provider
}

func runMigrateUp(cmd *cobra.Command, args []string) {
	results, err := migrateConn().Up(cmd.Context())
	if err != nil {
		log.Fatalf("migrate up: %v", err)
	}
	for _, r := range results {
		log.Printf("OK   %s", r.Source.Path)
	}
}

func runMigrateDown(cmd *cobra.Command, args []string) {
	result, err := migrateConn().Down(cmd.Context())
	if err != nil {
		log.Fatalf("migrate down: %v", err)
	}
	log.Printf("rolled back %s", result.Source.Path)
}

func runMigrateStatus(cmd *cobra.Command, args []string) {
	sources, err := migrateConn().Status(cmd.Context())
	if err != nil {
		log.Fatalf("migrate status: %v", err)
	}
	for _, s := range sources {
		log.Printf("%-5s %s", s.State, s.Source.Path)
	}
}
