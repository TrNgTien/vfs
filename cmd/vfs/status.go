package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check if the server is running",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	st, err := readState()
	if err != nil {
		fmt.Println("PID:         not running (no state file)")
		fmt.Println("MCP server:  not responding")
		fmt.Println("Dashboard:   not responding")
		return nil
	}

	pidRunning := isRunning(st.PID)
	mcpOK := probeHTTP("http://localhost" + st.MCPAddr + "/mcp")
	dashOK := probeHTTP("http://localhost:" + st.DashboardPort + "/")

	if pidRunning {
		fmt.Printf("PID:         %d (running)\n", st.PID)
	} else {
		fmt.Println("PID:         not running")
	}

	if mcpOK {
		fmt.Printf("MCP server:  running  (http://localhost%s/mcp)\n", st.MCPAddr)
	} else {
		fmt.Println("MCP server:  not responding")
	}

	if dashOK {
		fmt.Printf("Dashboard:   running  (http://localhost:%s/)\n", st.DashboardPort)
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
