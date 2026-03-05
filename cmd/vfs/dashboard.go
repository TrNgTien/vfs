package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/TrNgTien/vfs/internal/stats"
	"github.com/spf13/cobra"
)

var dashboardPort string

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Run the dashboard web UI",
	RunE:  runDashboard,
}

func init() {
	dashboardCmd.Flags().StringVar(&dashboardPort, "port", "3000", "dashboard listen port")
}

func runDashboard(cmd *cobra.Command, args []string) error {
	addr := ":" + dashboardPort
	mux := newDashboardMux()
	fmt.Fprintf(os.Stderr, "Dashboard: http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func newDashboardMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/history", func(w http.ResponseWriter, r *http.Request) {
		entries, err := stats.Load()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(entries)
	})

	mux.HandleFunc("/api/summary", func(w http.ResponseWriter, r *http.Request) {
		entries, err := stats.Load()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s := stats.Summarize(entries)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(s)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		htmlPath := findDashboardHTML()
		if htmlPath == "" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, fallbackDashboardHTML)
			return
		}
		http.ServeFile(w, r, htmlPath)
	})

	return mux
}

func findDashboardHTML() string {
	candidates := []string{
		"cmd/vfs/dashboard.html",
		"dashboard.html",
	}

	exe, err := os.Executable()
	if err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "dashboard.html"))
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

const fallbackDashboardHTML = `<!DOCTYPE html>
<html><head><title>vfs dashboard</title></head>
<body style="font-family:system-ui;background:#1a1a2e;color:#e0e0e0;padding:2rem">
<h1>vfs dashboard</h1>
<p>dashboard.html not found. Place it next to the vfs binary or in cmd/vfs/.</p>
<p>API endpoints: <a href="/api/history">/api/history</a> | <a href="/api/summary">/api/summary</a></p>
</body></html>`
