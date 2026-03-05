package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TrNgTien/vfs/internal/parser"
	"github.com/TrNgTien/vfs/internal/stats"
	"github.com/TrNgTien/vfs/pkg/bench"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "bench":
			runBench(os.Args[2:])
			return
		case "stats":
			runStats(os.Args[2:])
			return
		}
	}

	var filter string
	var showStats bool
	var noRecord bool
	var paths []string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-f" && i+1 < len(args):
			filter = args[i+1]
			i++
		case strings.HasPrefix(args[i], "-f="):
			filter = strings.TrimPrefix(args[i], "-f=")
		case args[i] == "--stats":
			showStats = true
		case args[i] == "--no-record":
			noRecord = true
		case args[i] == "-h" || args[i] == "--help":
			printHelp()
			os.Exit(0)
		default:
			paths = append(paths, args[i])
		}
	}

	if len(paths) == 0 {
		printHelp()
		os.Exit(1)
	}

	startTime := time.Now()

	var allResults []parser.FileResult
	var totalFilesScanned int

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		if info.IsDir() {
			results, err := parser.ExtractFromDirDetailed(p)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			totalFilesScanned += parser.CountSourceFiles(p)
			allResults = append(allResults, results...)
		} else {
			sigs, err := parser.ExtractFromFile(p)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			totalFilesScanned++
			if len(sigs) > 0 {
				allResults = append(allResults, parser.FileResult{
					RelPath: p,
					Sigs:    sigs,
				})
			}
		}
	}

	lowerFilter := strings.ToLower(filter)
	var matchedResults []parser.FileResult

	for _, r := range allResults {
		var matchedSigs []string
		for _, sig := range r.Sigs {
			line := r.RelPath + ": " + sig
			if lowerFilter != "" && !strings.Contains(strings.ToLower(line), lowerFilter) {
				continue
			}
			fmt.Println(line)
			matchedSigs = append(matchedSigs, sig)
		}
		if len(matchedSigs) > 0 {
			matchedResults = append(matchedResults, parser.FileResult{
				RelPath:  r.RelPath,
				Sigs:     matchedSigs,
				RawBytes: r.RawBytes,
				RawLines: r.RawLines,
			})
		}
	}

	st := parser.ComputeStats(matchedResults)
	st.FilesScanned = totalFilesScanned

	reduction := float64(0)
	if st.RawBytes > 0 {
		reduction = (1 - float64(st.VFSBytes)/float64(st.RawBytes)) * 100
	}
	tokensSaved := estimateTokensSaved(st.RawBytes, int64(st.VFSBytes))

	if showStats {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "--- vfs stats ---")
		fmt.Fprintf(os.Stderr, "Files scanned:       %d\n", st.FilesScanned)
		fmt.Fprintf(os.Stderr, "Files with results:  %d\n", st.FilesMatched)
		fmt.Fprintf(os.Stderr, "Exported symbols:    %d\n", st.ExportedFuncs)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "Without vfs (read matched files):  %s  (%d lines)\n", humanBytes(st.RawBytes), st.RawLines)
		fmt.Fprintf(os.Stderr, "With vfs (signatures only):        %s  (%d lines)\n", humanBytes(int64(st.VFSBytes)), st.VFSLines)
		fmt.Fprintf(os.Stderr, "Reduction:                         %.1f%%\n", reduction)
		fmt.Fprintf(os.Stderr, "Tokens saved (est):                ~%d\n", tokensSaved)
	}

	if !noRecord {
		project, _ := filepath.Abs(paths[0])
		_ = stats.Record(stats.Entry{
			Timestamp:     startTime,
			Project:       project,
			Filter:        filter,
			FilesScanned:  st.FilesScanned,
			FilesMatched:  st.FilesMatched,
			RawBytes:      st.RawBytes,
			RawLines:      st.RawLines,
			VFSBytes:      st.VFSBytes,
			VFSLines:      st.VFSLines,
			ExportedFuncs: st.ExportedFuncs,
			TokensSaved:   tokensSaved,
			ReductionPct:  reduction,
			DurationMs:    time.Since(startTime).Milliseconds(),
		})
	}
}

func printHelp() {
	fmt.Fprintln(os.Stderr, "vfs - extract exported function/type signatures from source code")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Supported languages: Go, JavaScript, TypeScript, JSX, TSX")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  vfs <path> [path...]              scan files/directories")
	fmt.Fprintln(os.Stderr, "  vfs <path> -f <pattern>           filter signatures by pattern")
	fmt.Fprintln(os.Stderr, "  vfs <path> --stats                show token efficiency stats")
	fmt.Fprintln(os.Stderr, "  vfs stats [--reset]               show/reset lifetime stats")
	fmt.Fprintln(os.Stderr, "  vfs bench --self                  quick self-test benchmark")
	fmt.Fprintln(os.Stderr, "  vfs bench -f <pattern> <path>     benchmark on any project")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Flags:")
	fmt.Fprintln(os.Stderr, "  -f <pattern>   case-insensitive substring filter on output lines")
	fmt.Fprintln(os.Stderr, "  --stats        show token efficiency stats (raw source vs vfs output)")
	fmt.Fprintln(os.Stderr, "  --no-record    skip logging this invocation to ~/.vfs/history.jsonl")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Examples:")
	fmt.Fprintln(os.Stderr, "  vfs .                             all exported sigs in current project")
	fmt.Fprintln(os.Stderr, "  vfs ./internal ./pkg              scan specific directories")
	fmt.Fprintln(os.Stderr, "  vfs ./src -f handleLogin          filter to matching signatures")
	fmt.Fprintln(os.Stderr, "  vfs handler.go                    single Go file")
	fmt.Fprintln(os.Stderr, "  vfs ./src/components              scan React components")
}

func humanBytes(b int64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func estimateTokensSaved(rawBytes, vfsBytes int64) int64 {
	return (rawBytes - vfsBytes) / 4
}

func runBench(args []string) {
	var filter string
	var dir string
	var showOutput bool
	var selfTest bool

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-f" && i+1 < len(args):
			filter = args[i+1]
			i++
		case strings.HasPrefix(args[i], "-f="):
			filter = strings.TrimPrefix(args[i], "-f=")
		case args[i] == "--show-output":
			showOutput = true
		case args[i] == "--self":
			selfTest = true
		case args[i] == "-h" || args[i] == "--help":
			fmt.Fprintln(os.Stderr, "usage: vfs bench -f <pattern> <path>")
			fmt.Fprintln(os.Stderr, "       vfs bench --self")
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "  Runs a 3-way comparison: read-all-files vs grep/rg vs vfs.")
			fmt.Fprintln(os.Stderr, "  Shows exactly how many tokens each approach sends to an LLM.")
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "  --self          run benchmark on the vfs project itself (zero-config)")
			fmt.Fprintln(os.Stderr, "  --show-output   print the actual output from grep and vfs")
			os.Exit(0)
		default:
			dir = args[i]
		}
	}

	if selfTest {
		dir = "."
		filter = "Extract"
	}

	if dir == "" || filter == "" {
		fmt.Fprintln(os.Stderr, "usage: vfs bench -f <pattern> <path>")
		fmt.Fprintln(os.Stderr, "       vfs bench --self")
		os.Exit(1)
	}

	absDir, _ := filepath.Abs(dir)
	fmt.Fprintf(os.Stderr, "Benchmark: pattern=%q  path=%s\n\n", filter, absDir)

	readResult, err := bench.RunReadFile(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read-files error: %v\n", err)
		os.Exit(1)
	}

	grepResult, err := bench.RunGrep(filter, dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grep/rg error: %v\n", err)
		os.Exit(1)
	}

	vfsResult, err := bench.RunVFS(filter, dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vfs error: %v\n", err)
		os.Exit(1)
	}

	if showOutput {
		fmt.Printf("========== %s output ==========\n", grepResult.Tool)
		fmt.Print(grepResult.Output)
		fmt.Println()
		fmt.Println("========== vfs output ==========")
		fmt.Print(vfsResult.Output)
		fmt.Println()
	}

	col1 := "Read all files"
	col2 := grepResult.Tool
	col3 := "vfs"

	fmt.Println("What an LLM agent receives for the same query, 3 approaches:")
	fmt.Println()
	fmt.Println("┌─────────────────┬──────────────────┬──────────────────┬──────────────────┐")
	fmt.Printf("│                 │ %-16s │ %-16s │ %-16s │\n", col1, col2, col3)
	fmt.Println("├─────────────────┼──────────────────┼──────────────────┼──────────────────┤")
	fmt.Printf("│ Output size     │ %-16s │ %-16s │ %-16s │\n",
		humanBytes(int64(readResult.Bytes)), humanBytes(int64(grepResult.Bytes)), humanBytes(int64(vfsResult.Bytes)))
	fmt.Printf("│ Lines           │ %-16d │ %-16d │ %-16d │\n",
		readResult.Lines, grepResult.Lines, vfsResult.Lines)
	fmt.Printf("│ Est. tokens     │ %-16d │ %-16d │ %-16d │\n",
		readResult.Tokens, grepResult.Tokens, vfsResult.Tokens)
	fmt.Printf("│ Time            │ %-16s │ %-16s │ %-16s │\n",
		readResult.Duration.Round(time.Millisecond), grepResult.Duration.Round(time.Millisecond), vfsResult.Duration.Round(time.Millisecond))
	fmt.Println("└─────────────────┴──────────────────┴──────────────────┴──────────────────┘")

	fmt.Println()
	fmt.Println("Token savings vs reading all files:")
	if readResult.Tokens > 0 && vfsResult.Tokens > 0 {
		vsRead := (1 - float64(vfsResult.Tokens)/float64(readResult.Tokens)) * 100
		fmt.Printf("  vfs saves %.1f%% tokens vs reading all files (%d -> %d tokens)\n",
			vsRead, readResult.Tokens, vfsResult.Tokens)
	}
	if grepResult.Tokens > 0 && vfsResult.Tokens > 0 {
		vsGrep := (1 - float64(vfsResult.Tokens)/float64(grepResult.Tokens)) * 100
		fmt.Printf("  vfs saves %.1f%% tokens vs %s             (%d -> %d tokens)\n",
			vsGrep, grepResult.Tool, grepResult.Tokens, vfsResult.Tokens)
	}

	fmt.Println()
	fmt.Println("Reproduce & verify these numbers yourself:")
	fmt.Println()
	fmt.Printf("  # 1. Read all files (what an LLM sees with cat/Read):\n")
	fmt.Printf("  %s\n", readResult.Command)
	fmt.Println()
	fmt.Printf("  # 2. grep/rg search (what an LLM sees with Grep tool):\n")
	fmt.Printf("  %s | wc -c    # bytes\n", grepResult.Command)
	fmt.Printf("  %s | wc -l    # lines\n", grepResult.Command)
	fmt.Println()
	fmt.Printf("  # 3. vfs search (structured signatures only):\n")
	fmt.Printf("  %s | wc -c    # bytes\n", vfsResult.Command)
	fmt.Printf("  %s | wc -l    # lines\n", vfsResult.Command)
}

func runStats(args []string) {
	for _, a := range args {
		switch a {
		case "--reset":
			if err := stats.Reset(); err != nil {
				fmt.Fprintf(os.Stderr, "reset failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Fprintln(os.Stderr, "History cleared.")
			return
		case "-h", "--help":
			fmt.Fprintln(os.Stderr, "usage: vfs stats [--reset]")
			fmt.Fprintln(os.Stderr, "  Shows cumulative token savings across all recorded invocations.")
			fmt.Fprintln(os.Stderr, "  --reset   clear the history file")
			os.Exit(0)
		}
	}

	entries, err := stats.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "loading history: %v\n", err)
		os.Exit(1)
	}

	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "No history recorded yet. Run vfs on a project to start tracking.")
		return
	}

	s := stats.Summarize(entries)

	fmt.Fprintln(os.Stderr, "--- vfs lifetime stats ---")
	fmt.Fprintf(os.Stderr, "Invocations:         %d\n", s.Invocations)
	fmt.Fprintf(os.Stderr, "Total tokens saved:  ~%s\n", formatNumber(s.TotalSaved))
	fmt.Fprintf(os.Stderr, "Total raw scanned:   %s  (%s lines)\n", humanBytes(s.TotalRawBytes), formatNumber(int64(s.TotalRawLines)))
	fmt.Fprintf(os.Stderr, "Total vfs output:    %s  (%s lines)\n", humanBytes(s.TotalVFSBytes), formatNumber(int64(s.TotalVFSLines)))
	fmt.Fprintf(os.Stderr, "Avg reduction:       %.1f%%\n", s.AvgReduction)
	fmt.Fprintf(os.Stderr, "First recorded:      %s\n", s.FirstRecorded.Local().Format("2006-01-02 15:04"))
	fmt.Fprintf(os.Stderr, "Last recorded:       %s\n", s.LastRecorded.Local().Format("2006-01-02 15:04"))
}

func formatNumber(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1_000_000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d,%03d,%03d", n/1_000_000, (n%1_000_000)/1000, n%1000)
}
