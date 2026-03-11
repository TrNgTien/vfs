package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/TrNgTien/vfs/internal/parser"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

//go:embed mcp_instructions.md
var mcpInstructions string

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
	srv := mcp.NewServer(&mcp.Implementation{Name: "vfs", Version: version}, &mcp.ServerOptions{
		Instructions: mcpInstructions,
	})

	srv.AddTool(
		&mcp.Tool{
			Name:        "search",
			Description: "PREFERRED over Grep/Read for code discovery. Find function definitions, method signatures, class names, and type declarations by pattern. Parses source via AST and returns compact signatures with file paths and line numbers -- use this BEFORE Grep or Read when looking for where code is defined. Saves 60-70% tokens vs grep. Supports Go, JS, TS, Python, Rust, Java, C#, Dart, HCL, Dockerfile, Protobuf, SQL, YAML.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"paths":{"type":"array","items":{"type":"string"},"description":"File or directory paths to scan (e.g. [\".\"] for whole project, [\"./src\"] for a subdirectory)"},"pattern":{"type":"string","description":"Case-insensitive substring filter for function/class/type names (e.g. \"HandleLogin\", \"auth\", \"User\")"}},"required":["paths","pattern"]}`),
		},
		handleSearch,
	)

	srv.AddTool(
		&mcp.Tool{
			Name:        "extract",
			Description: "List all exported function/class/type signatures from source files. Returns a compact table of contents of any codebase with bodies stripped. Use when you need to understand the full API surface of a package or directory, not just search for a specific name.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"paths":{"type":"array","items":{"type":"string"},"description":"File or directory paths to scan (e.g. [\"./internal/handlers\"] or [\"server.go\"])"}},"required":["paths"]}`),
		},
		handleExtract,
	)

	srv.AddTool(
		&mcp.Tool{
			Name:        "list_languages",
			Description: "List programming languages and file extensions supported by vfs.",
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

	start := time.Now()
	results, err := scanPaths(args.Paths)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	lines := formatSigLines(results)
	recordInvocation(args.Paths, results, lines, time.Since(start))
	return textResult(strings.Join(lines, "\n")), nil
}

func handleSearch(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args searchArgs
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return errorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	start := time.Now()
	results, err := scanPaths(args.Paths)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	lines := formatSigLines(results)
	filtered := filterSigLines(lines, args.Pattern)
	recordInvocationWithFilter(args.Paths, results, filtered, time.Since(start), args.Pattern)
	return textResult(strings.Join(filtered, "\n")), nil
}

func handleListLanguages(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	langs := supportedLanguages()
	var sb strings.Builder
	for _, l := range langs {
		sb.WriteString(fmt.Sprintf("%s: %s\n", l.Language, strings.Join(l.Extensions, ", ")))
	}
	return textResult(sb.String()), nil
}
