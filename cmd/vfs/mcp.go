package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/TrNgTien/vfs/internal/parser"
	"github.com/TrNgTien/vfs/internal/stats"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var mcpHTTPAddr string

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run MCP server (stdio or HTTP)",
	Long:  "Start the MCP server for AI assistant integration. Default is stdio transport; use --http for HTTP.",
	RunE:  runMCP,
}

func init() {
	mcpCmd.Flags().StringVar(&mcpHTTPAddr, "http", "", "HTTP listen address (e.g. :8080); omit for stdio")
}

func newMCPServer() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "vfs", Version: version}, nil)

	srv.AddTool(
		&mcp.Tool{
			Name:        "extract",
			Description: "Scan paths and return all exported signatures",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"paths":{"type":"array","items":{"type":"string"},"description":"File or directory paths to scan"}},"required":["paths"]}`),
		},
		handleExtract,
	)

	srv.AddTool(
		&mcp.Tool{
			Name:        "search",
			Description: "Extract signatures filtered by name pattern",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"paths":{"type":"array","items":{"type":"string"},"description":"File or directory paths to scan"},"pattern":{"type":"string","description":"Case-insensitive substring filter"}},"required":["paths","pattern"]}`),
		},
		handleSearch,
	)

	srv.AddTool(
		&mcp.Tool{
			Name:        "stats",
			Description: "Return lifetime usage statistics",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		handleMCPStats,
	)

	srv.AddTool(
		&mcp.Tool{
			Name:        "list_languages",
			Description: "List supported languages and extensions",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		handleListLanguages,
	)

	return srv
}

func runMCP(cmd *cobra.Command, args []string) error {
	srv := newMCPServer()

	if mcpHTTPAddr != "" {
		handler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server { return srv }, nil)
		fmt.Fprintf(os.Stderr, "MCP server listening on %s\n", mcpHTTPAddr)
		return http.ListenAndServe(mcpHTTPAddr, handler)
	}

	return srv.Run(context.Background(), &mcp.StdioTransport{})
}

type extractArgs struct {
	Paths []string `json:"paths"`
}

type searchArgs struct {
	Paths   []string `json:"paths"`
	Pattern string   `json:"pattern"`
}

func scanPaths(paths []string) ([]parser.FileResult, error) {
	var allResults []parser.FileResult
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("cannot access %s: %w", p, err)
		}
		if info.IsDir() {
			results, err := parser.ExtractFromDirDetailed(p)
			if err != nil {
				return nil, err
			}
			allResults = append(allResults, results...)
		} else {
			sigs, err := parser.ExtractFromFile(p)
			if err != nil {
				return nil, err
			}
			if len(sigs) > 0 {
				raw, _ := os.ReadFile(p)
				allResults = append(allResults, parser.FileResult{
					RelPath:  p,
					Sigs:     sigs,
					RawBytes: int64(len(raw)),
					RawLines: strings.Count(string(raw), "\n"),
				})
			}
		}
	}
	return allResults, nil
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}
}

func handleExtract(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args extractArgs
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return errorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	results, err := scanPaths(args.Paths)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	lines := formatSigLines(results)
	return textResult(strings.Join(lines, "\n")), nil
}

func handleSearch(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args searchArgs
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return errorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	results, err := scanPaths(args.Paths)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	lines := formatSigLines(results)
	filtered := filterSigLines(lines, args.Pattern)
	return textResult(strings.Join(filtered, "\n")), nil
}

func handleMCPStats(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	entries, err := stats.Load()
	if err != nil {
		return errorResult(fmt.Sprintf("loading stats: %v", err)), nil
	}
	if len(entries) == 0 {
		return textResult("no invocations recorded yet"), nil
	}

	s := stats.Summarize(entries)
	text := fmt.Sprintf(
		"Invocations: %d\nTotal tokens saved: ~%d\nAvg reduction: %.1f%%\nFirst: %s\nLast: %s",
		s.Invocations, s.TotalSaved, s.AvgReduction,
		s.FirstRecorded.Format("2006-01-02 15:04"),
		s.LastRecorded.Format("2006-01-02 15:04"),
	)
	return textResult(text), nil
}

func handleListLanguages(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	langs := supportedLanguages()
	var sb strings.Builder
	for _, l := range langs {
		sb.WriteString(fmt.Sprintf("%s: %s\n", l.Language, strings.Join(l.Extensions, ", ")))
	}
	return textResult(sb.String()), nil
}
