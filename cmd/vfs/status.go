package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

var (
	statusMCPAddr       string
	statusMCPPort       string
	statusDashboardPort string
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check if the server is running",
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().StringVar(&statusMCPAddr, "mcp", "", "MCP HTTP address to probe (e.g. :8080)")
	statusCmd.Flags().StringVar(&statusMCPPort, "port", "", "MCP HTTP port to probe (shorthand for --mcp :PORT)")
	statusCmd.Flags().StringVar(&statusDashboardPort, "dashboard-port", "3000", "dashboard port to probe")
}

func runStatus(cmd *cobra.Command, args []string) error {
	mcpAddr := resolveMCPAddr(statusMCPAddr, statusMCPPort)

	pid, err := readPID()
	pidRunning := err == nil && isRunning(pid)

	mcpOK := probeHTTP("http://localhost" + mcpAddr + "/mcp")
	dashOK := probeHTTP("http://localhost:" + statusDashboardPort + "/")

	if pidRunning {
		fmt.Printf("PID:         %d (running)\n", pid)
	} else {
		fmt.Println("PID:         not running")
	}

	if mcpOK {
		fmt.Printf("MCP server:  running  (http://localhost%s/mcp)\n", mcpAddr)
	} else {
		fmt.Println("MCP server:  not responding")
	}

	if dashOK {
		fmt.Printf("Dashboard:   running  (http://localhost:%s/)\n", statusDashboardPort)
	} else {
		fmt.Println("Dashboard:   not responding")
	}

	return nil
}

func probeHTTP(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}
