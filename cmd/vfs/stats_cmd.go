package main

import (
	"fmt"

	"github.com/TrNgTien/vfs/internal/stats"
	"github.com/spf13/cobra"
)

var statsReset bool

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show lifetime token savings statistics",
	RunE:  runStats,
}

func init() {
	statsCmd.Flags().BoolVar(&statsReset, "reset", false, "clear all history")
}

func runStats(cmd *cobra.Command, args []string) error {
	if statsReset {
		if err := stats.Reset(); err != nil {
			return fmt.Errorf("resetting stats: %w", err)
		}
		fmt.Println("history cleared")
		return nil
	}

	entries, err := stats.Load()
	if err != nil {
		return fmt.Errorf("loading stats: %w", err)
	}
	if len(entries) == 0 {
		fmt.Println("no invocations recorded yet")
		return nil
	}

	s := stats.Summarize(entries)

	fmt.Println("--- vfs lifetime stats ---")
	fmt.Printf("Invocations:         %d\n", s.Invocations)
	fmt.Printf("Total tokens saved:  ~%s\n", formatTokenCount(s.TotalSaved))
	fmt.Printf("Total raw scanned:   %s  (%s lines)\n",
		formatByteCount(s.TotalRawBytes), formatCount(s.TotalRawLines))
	fmt.Printf("Total vfs output:    %s  (%s lines)\n",
		formatByteCount(s.TotalVFSBytes), formatCount(s.TotalVFSLines))
	fmt.Printf("Avg reduction:       %.1f%%\n", s.AvgReduction)
	fmt.Printf("First recorded:      %s\n", s.FirstRecorded.Format("2006-01-02 15:04"))
	fmt.Printf("Last recorded:       %s\n", s.LastRecorded.Format("2006-01-02 15:04"))

	return nil
}

func formatTokenCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%s", formatCount(int(n)))
	default:
		return fmt.Sprintf("%d", n)
	}
}

func formatByteCount(n int64) string {
	switch {
	case n >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	case n >= 1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func formatCount(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d", n)
}
