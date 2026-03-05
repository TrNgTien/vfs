package main

import (
	"os"
)

func main() {
	rootCmd.AddCommand(benchCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(dashboardCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
