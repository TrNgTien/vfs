package main

import (
	"os"
)

func main() {
	rootCmd.AddCommand(benchCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(mcpCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
