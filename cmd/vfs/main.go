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
<<<<<<< HEAD
=======
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
>>>>>>> main

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
