package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	upMCPAddr       string
	upMCPPort       string
	upDashboardPort string
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start MCP server + dashboard (detached)",
	RunE:  runUp,
}

func init() {
	upCmd.Flags().StringVar(&upMCPAddr, "mcp", "", "MCP HTTP listen address (e.g. :8080)")
	upCmd.Flags().StringVar(&upMCPPort, "port", "", "MCP HTTP port (shorthand for --mcp :PORT)")
	upCmd.Flags().StringVar(&upDashboardPort, "dashboard-port", "3000", "dashboard listen port")
}

type serverState struct {
	PID           int    `json:"pid"`
	MCPAddr       string `json:"mcp_addr"`
	DashboardPort string `json:"dashboard_port"`
}

func vfsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	return filepath.Join(home, ".vfs")
}

func stateFile() string { return filepath.Join(vfsDir(), "vfs.state") }
func logFile() string   { return filepath.Join(vfsDir(), "vfs.log") }

func readState() (*serverState, error) {
	data, err := os.ReadFile(stateFile())
	if err != nil {
		return nil, err
	}
	var s serverState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func writeState(s *serverState) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(stateFile(), data, 0o644)
}

func removeState() { os.Remove(stateFile()) }

func runUp(cmd *cobra.Command, args []string) error {
	mcpAddr := resolveMCPAddr(upMCPAddr, upMCPPort)

	if st, err := readState(); err == nil && isRunning(st.PID) {
		portChanged := st.MCPAddr != mcpAddr
		dashChanged := st.DashboardPort != upDashboardPort

		fmt.Printf("vfs v%s is already running (pid %d)\n", version, st.PID)
		fmt.Printf("  MCP:       http://localhost%s/mcp\n", st.MCPAddr)
		fmt.Printf("  dashboard: http://localhost:%s\n", st.DashboardPort)

		if portChanged || dashChanged {
			fmt.Println()
			fmt.Println("Port change requested but server is already running.")
			fmt.Println("Run 'vfs down' first, then 'vfs up' with the new port.")
		}
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

	child := exec.Command(exe, "serve", "--mcp", mcpAddr, "--dashboard-port", upDashboardPort)
	child.Stdout = logF
	child.Stderr = logF
	setSysProcAttr(child)

	if err := child.Start(); err != nil {
		logF.Close()
		return fmt.Errorf("starting server: %w", err)
	}
	logF.Close()

	st := &serverState{
		PID:           child.Process.Pid,
		MCPAddr:       mcpAddr,
		DashboardPort: upDashboardPort,
	}
	if err := writeState(st); err != nil {
		return fmt.Errorf("writing state file: %w", err)
	}

	fmt.Printf("vfs v%s started (pid %d)\n", version, child.Process.Pid)
	fmt.Printf("  MCP:       http://localhost%s/mcp\n", mcpAddr)
	fmt.Printf("  dashboard: http://localhost:%s\n", upDashboardPort)
	fmt.Printf("  log:       %s\n", logFile())
	fmt.Printf("  stop:      vfs down\n")

	return nil
}
