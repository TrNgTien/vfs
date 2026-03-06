package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
)

var (
	upMCPAddr       string
	upDashboardPort string
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start MCP server + dashboard (detached)",
	RunE:  runUp,
}

func init() {
	upCmd.Flags().StringVar(&upMCPAddr, "mcp", ":8080", "MCP HTTP listen address")
	upCmd.Flags().StringVar(&upDashboardPort, "dashboard-port", "3000", "dashboard listen port")
}

func vfsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	return filepath.Join(home, ".vfs")
}

func pidFile() string { return filepath.Join(vfsDir(), "vfs.pid") }
func logFile() string { return filepath.Join(vfsDir(), "vfs.log") }

func readPID() (int, error) {
	data, err := os.ReadFile(pidFile())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

func runUp(cmd *cobra.Command, args []string) error {
	if pid, err := readPID(); err == nil && isRunning(pid) {
		fmt.Printf("vfs v%s is already running (pid %d)\n", version, pid)
		fmt.Printf("  MCP:       http://localhost%s/mcp\n", upMCPAddr)
		fmt.Printf("  dashboard: http://localhost:%s\n", upDashboardPort)
		return nil
	}

	if err := os.MkdirAll(vfsDir(), 0o755); err != nil {
		return fmt.Errorf("creating vfs dir: %w", err)
	}

	logF, err := os.OpenFile(logFile(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}

	child := exec.Command(exe, "serve", "--mcp", upMCPAddr, "--dashboard-port", upDashboardPort)
	child.Stdout = logF
	child.Stderr = logF
	setSysProcAttr(child)

	if err := child.Start(); err != nil {
		logF.Close()
		return fmt.Errorf("starting server: %w", err)
	}
	logF.Close()

	if err := os.WriteFile(pidFile(), []byte(strconv.Itoa(child.Process.Pid)), 0o644); err != nil {
		return fmt.Errorf("writing pid file: %w", err)
	}

	fmt.Printf("vfs v%s started (pid %d)\n", version, child.Process.Pid)
	fmt.Printf("  MCP:       http://localhost%s/mcp\n", upMCPAddr)
	fmt.Printf("  dashboard: http://localhost:%s\n", upDashboardPort)
	fmt.Printf("  log:       %s\n", logFile())
	fmt.Printf("  stop:      vfs down\n")

	return nil
}
