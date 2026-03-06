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
	pid, err := readPID()
	if err != nil {
		fmt.Println("vfs is not running")
		return nil
	}

	if !isRunning(pid) {
		os.Remove(pidFile())
		fmt.Println("vfs is not running")
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("finding process %d: %w", pid, err)
	}

	if err := terminateProcess(proc); err != nil {
		return fmt.Errorf("stopping vfs (pid %d): %w", pid, err)
	}

	os.Remove(pidFile())
	fmt.Printf("vfs stopped (pid %d)\n", pid)
	return nil
}
