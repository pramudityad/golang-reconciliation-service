package main

import (
	"fmt"
	"os"

	"golang-reconciliation-service/cmd/reconciler/cmd"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Set version information
	cmd.SetVersionInfo(version, commit, date)
	
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}