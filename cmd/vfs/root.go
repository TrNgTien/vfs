package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TrNgTien/vfs/internal/parser"
	"github.com/TrNgTien/vfs/internal/stats"
	"github.com/spf13/cobra"
)

var (
	filter    string
	showStats bool
	noRecord  bool
	version   = "dev"
)

var rootCmd = &cobra.Command{
	Use:     "vfs [paths...]",
	Short:   "Extract exported function signatures from source code",
	Long:    "vfs parses source files via AST and tree-sitter, returning exported function/class/type signatures with bodies stripped.",
	Version: version,
	Args:    cobra.MinimumNArgs(1),
	RunE:    runRoot,
}

func init() {
	rootCmd.Flags().StringVarP(&filter, "filter", "f", "", "case-insensitive substring filter on output lines")
	rootCmd.Flags().BoolVar(&showStats, "stats", false, "show token efficiency stats after output")
	rootCmd.Flags().BoolVar(&noRecord, "no-record", false, "skip logging this invocation to history")
}

func runRoot(cmd *cobra.Command, args []string) error {
	start := time.Now()
	filterLower := strings.ToLower(filter)

	var allResults []parser.FileResult
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return fmt.Errorf("cannot access %s: %w", arg, err)
		}

		if info.IsDir() {
			results, err := parser.ExtractFromDirDetailed(arg)
			if err != nil {
				return fmt.Errorf("scanning %s: %w", arg, err)
			}
			allResults = append(allResults, results...)
		} else {
			sigs, err := parser.ExtractFromFile(arg)
			if err != nil {
				return fmt.Errorf("parsing %s: %w", arg, err)
			}
			if len(sigs) > 0 {
				rel := filepath.Base(arg)
				raw, _ := os.ReadFile(arg)
				allResults = append(allResults, parser.FileResult{
					RelPath:  rel,
					Sigs:     sigs,
					RawBytes: int64(len(raw)),
					RawLines: strings.Count(string(raw), "\n"),
				})
			}
		}
	}

	var lines []string
	for _, r := range allResults {
		for _, s := range r.Sigs {
			line := s.FormatLine(r.RelPath)
			if filterLower != "" && !strings.Contains(strings.ToLower(line), filterLower) {
				continue
			}
			lines = append(lines, line)
		}
	}

	for _, l := range lines {
		fmt.Println(l)
	}

	elapsed := time.Since(start)

	if showStats {
		st := parser.ComputeStats(allResults)
		printStats(st, len(lines), elapsed)
	}

	if !noRecord {
		recordInvocation(args, allResults, lines, elapsed)
	}

	return nil
}

func printStats(st parser.Stats, matchedLines int, elapsed time.Duration) {
	rawTokens := st.RawBytes / 4
	vfsTokens := int64(st.VFSBytes) / 4
	saved := rawTokens - vfsTokens
	var pct float64
	if rawTokens > 0 {
		pct = float64(saved) / float64(rawTokens) * 100
	}

	fmt.Fprintf(os.Stderr, "\n--- stats ---\n")
	fmt.Fprintf(os.Stderr, "Files scanned:    %d\n", st.FilesMatched)
	fmt.Fprintf(os.Stderr, "Signatures found: %d\n", st.ExportedFuncs)
	fmt.Fprintf(os.Stderr, "Lines output:     %d\n", matchedLines)
	fmt.Fprintf(os.Stderr, "Raw source:       %.1f KB (%d lines)\n", float64(st.RawBytes)/1024, st.RawLines)
	fmt.Fprintf(os.Stderr, "VFS output:       %.1f KB (%d lines)\n", float64(st.VFSBytes)/1024, st.VFSLines)
	fmt.Fprintf(os.Stderr, "Est. tokens:      %d raw → %d vfs (saved ~%d, %.1f%%)\n", rawTokens, vfsTokens, saved, pct)
	fmt.Fprintf(os.Stderr, "Duration:         %s\n", elapsed.Round(time.Millisecond))
}

func recordInvocationWithFilter(args []string, results []parser.FileResult, lines []string, elapsed time.Duration, filterPattern string) {
	st := parser.ComputeStats(results)
	rawTokens := st.RawBytes / 4
	vfsTokens := int64(st.VFSBytes) / 4
	saved := rawTokens - vfsTokens
	var pct float64
	if rawTokens > 0 {
		pct = float64(saved) / float64(rawTokens) * 100
	}

	project := "."
	if len(args) > 0 {
		if abs, err := filepath.Abs(args[0]); err == nil {
			project = abs
		}
	}

	_ = stats.Record(stats.Entry{
		Timestamp:     time.Now(),
		Project:       project,
		Filter:        filterPattern,
		FilesScanned:  st.FilesMatched,
		FilesMatched:  st.FilesMatched,
		RawBytes:      st.RawBytes,
		RawLines:      st.RawLines,
		VFSBytes:      st.VFSBytes,
		VFSLines:      len(lines),
		ExportedFuncs: st.ExportedFuncs,
		TokensSaved:   saved,
		ReductionPct:  pct,
		DurationMs:    elapsed.Milliseconds(),
	})
}

func recordInvocation(args []string, results []parser.FileResult, lines []string, elapsed time.Duration) {
	recordInvocationWithFilter(args, results, lines, elapsed, filter)
}

// formatSigLines formats FileResults into signature lines, used by MCP tools.
func formatSigLines(results []parser.FileResult) []string {
	var lines []string
	for _, r := range results {
		for _, s := range r.Sigs {
			lines = append(lines, s.FormatLine(r.RelPath))
		}
	}
	return lines
}

// filterSigLines applies a case-insensitive substring filter to lines.
func filterSigLines(lines []string, pattern string) []string {
	if pattern == "" {
		return lines
	}
	lower := strings.ToLower(pattern)
	var out []string
	for _, l := range lines {
		if strings.Contains(strings.ToLower(l), lower) {
			out = append(out, l)
		}
	}
	return out
}

// supportedLanguages returns a human-readable list of supported languages.
func supportedLanguages() []struct {
	Language   string
	Extensions []string
} {
	return []struct {
		Language   string
		Extensions []string
	}{
		{"Go", []string{".go"}},
		{"JavaScript", []string{".js", ".mjs", ".cjs", ".jsx"}},
		{"TypeScript", []string{".ts", ".mts", ".cts", ".tsx"}},
		{"Python", []string{".py"}},
		{"Rust", []string{".rs"}},
		{"Java", []string{".java"}},
		{"HCL/Terraform", []string{".tf", ".hcl"}},
		{"Dockerfile", []string{"Dockerfile", "Dockerfile.*", "*.dockerfile"}},
		{"Protobuf", []string{".proto"}},
		{"SQL", []string{".sql"}},
		{"YAML", []string{".yml", ".yaml"}},
	}
}
