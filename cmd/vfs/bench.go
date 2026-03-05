package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/TrNgTien/vfs/pkg/bench"
	"github.com/spf13/cobra"
)

var (
	benchFilter     string
	benchSelf       bool
	benchShowOutput bool
)

var benchCmd = &cobra.Command{
	Use:   "bench [path]",
	Short: "Run token savings benchmark (grep vs vfs vs read-all)",
	RunE:  runBench,
}

func init() {
	benchCmd.Flags().StringVarP(&benchFilter, "filter", "f", "", "pattern to search for")
	benchCmd.Flags().BoolVar(&benchSelf, "self", false, "benchmark on vfs's own source")
	benchCmd.Flags().BoolVar(&benchShowOutput, "show-output", false, "print actual tool output")
}

func runBench(cmd *cobra.Command, args []string) error {
	dir := "."
	pattern := benchFilter

	if benchSelf {
		_, thisFile, _, _ := runtime.Caller(0)
		dir = filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
		if pattern == "" {
			pattern = "Extract"
		}
	} else if len(args) > 0 {
		dir = args[len(args)-1]
	}

	if pattern == "" {
		return fmt.Errorf("--filter/-f is required (or use --self for a quick self-test)")
	}

	abs, _ := filepath.Abs(dir)
	fmt.Fprintf(os.Stderr, "Benchmarking on: %s (pattern: %q)\n\n", abs, pattern)

	readResult, err := bench.RunReadFile(dir)
	if err != nil {
		return fmt.Errorf("read-all failed: %w", err)
	}

	grepResult, err := bench.RunGrep(pattern, dir)
	if err != nil {
		return fmt.Errorf("grep failed: %w", err)
	}

	vfsResult, err := bench.RunVFS(pattern, dir)
	if err != nil {
		return fmt.Errorf("vfs failed: %w", err)
	}

	printBenchTable(readResult, grepResult, vfsResult)

	if benchShowOutput {
		fmt.Println("\n--- grep output ---")
		fmt.Print(grepResult.Output)
		fmt.Println("\n--- vfs output ---")
		fmt.Print(vfsResult.Output)
	}

	return nil
}

func printBenchTable(readAll, grep, vfs *bench.Result) {
	fmt.Printf("%-18s %-15s %-12s %-12s\n", "", "Read all files", "grep", "vfs")
	fmt.Printf("%-18s %-15s %-12s %-12s\n",
		strings.Repeat("-", 18),
		strings.Repeat("-", 15),
		strings.Repeat("-", 12),
		strings.Repeat("-", 12),
	)
	fmt.Printf("%-18s %-15s %-12s %-12s\n", "Output size",
		formatBytes(readAll.Bytes), formatBytes(grep.Bytes), formatBytes(vfs.Bytes))
	fmt.Printf("%-18s %-15d %-12d %-12d\n", "Lines",
		readAll.Lines, grep.Lines, vfs.Lines)
	fmt.Printf("%-18s %-15d %-12d %-12d\n", "Est. tokens",
		readAll.Tokens, grep.Tokens, vfs.Tokens)

	if readAll.Tokens > 0 {
		fmt.Printf("\nvfs saves %.1f%% tokens vs reading all files (%d → %d)\n",
			(1-float64(vfs.Tokens)/float64(readAll.Tokens))*100,
			readAll.Tokens, vfs.Tokens)
	}
	if grep.Tokens > 0 {
		fmt.Printf("vfs saves %.1f%% tokens vs grep (%d → %d)\n",
			(1-float64(vfs.Tokens)/float64(grep.Tokens))*100,
			grep.Tokens, vfs.Tokens)
	}
}

func formatBytes(b int) string {
	switch {
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d B", b)
	}
}
