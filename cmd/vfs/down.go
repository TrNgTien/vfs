package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop the background server",
	RunE:  runDown,
}

func runDown(cmd *cobra.Command, args []string) error {
	st, err := readState()
	if err != nil {
		fmt.Println("vfs is not running")
		return nil
	}

	if !isRunning(st.PID) {
		removeState()
		fmt.Println("vfs is not running")
		return nil
	}

	proc, err := os.FindProcess(st.PID)
	if err != nil {
		return fmt.Errorf("finding process %d: %w", st.PID, err)
	}

	if err := terminateProcess(proc); err != nil {
		return fmt.Errorf("stopping vfs (pid %d): %w", st.PID, err)
	}

	removeState()
	fmt.Printf("vfs stopped (pid %d)\n", st.PID)
	return nil
}
