package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/TrNgTien/vfs/internal/stats"
	"github.com/spf13/cobra"
)

//go:embed dashboard.html
var dashboardHTML []byte

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
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(dashboardHTML)
	})

	return mux
}
