package main

import (
	"log"

	"github.com/spf13/cobra"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Start the River queue worker",
	Run:   runWorker,
}

func runWorker(cmd *cobra.Command, args []string) {
	log.Println("starting verinews worker...")

	// TODO: connect to DB, register River workers, start consuming
}
