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
	serveDashboardPort string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run MCP server + dashboard (foreground)",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().StringVar(&serveMCPAddr, "mcp", ":8080", "MCP HTTP listen address")
	serveCmd.Flags().StringVar(&serveDashboardPort, "dashboard-port", "3000", "dashboard listen port")
}

func runServe(cmd *cobra.Command, args []string) error {
	srv := newMCPServer()
	handler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server { return srv }, nil)

	dashMux := newDashboardMux()

	errCh := make(chan error, 2)

	go func() {
		fmt.Fprintf(os.Stderr, "MCP server: http://localhost%s/mcp\n", serveMCPAddr)
		mux := http.NewServeMux()
		mux.Handle("/mcp", handler)
		mux.Handle("/mcp/", handler)
		errCh <- http.ListenAndServe(serveMCPAddr, mux)
	}()

	go func() {
		addr := ":" + serveDashboardPort
		fmt.Fprintf(os.Stderr, "Dashboard:  http://localhost%s\n", addr)
		errCh <- http.ListenAndServe(addr, dashMux)
	}()

	return <-errCh
}
