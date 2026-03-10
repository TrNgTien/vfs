package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var (
	serveMCPAddr       string
	serveMCPPort       string
	serveDashboardPort string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run MCP server + dashboard (foreground)",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().StringVar(&serveMCPAddr, "mcp", "", "MCP HTTP listen address (e.g. :8080)")
	serveCmd.Flags().StringVar(&serveMCPPort, "port", "", "MCP HTTP port (shorthand for --mcp :PORT)")
	serveCmd.Flags().StringVar(&serveDashboardPort, "dashboard-port", "3000", "dashboard listen port")
}

func resolveMCPAddr(addrFlag, portFlag string) string {
	if addrFlag != "" {
		return addrFlag
	}
	if portFlag != "" {
		return ":" + portFlag
	}
	return ":8080"
}

func runServe(cmd *cobra.Command, args []string) error {
	mcpAddr := resolveMCPAddr(serveMCPAddr, serveMCPPort)

	srv := newMCPServer()
	handler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server { return srv }, nil)

	dashMux := newDashboardMux()

	errCh := make(chan error, 2)

	go func() {
		fmt.Fprintf(os.Stderr, "MCP server: http://localhost%s/mcp\n", mcpAddr)
		mux := http.NewServeMux()
		mux.Handle("/mcp", handler)
		mux.Handle("/mcp/", handler)
		errCh <- http.ListenAndServe(mcpAddr, mux)
	}()

	go func() {
		addr := ":" + serveDashboardPort
		fmt.Fprintf(os.Stderr, "Dashboard:  http://localhost%s\n", addr)
		errCh <- http.ListenAndServe(addr, dashMux)
	}()

	return <-errCh
}
