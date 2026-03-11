package stats

import (
	"testing"
	"time"
)

func TestSummarize_OperationalFields(t *testing.T) {
	now := time.Now()
	entries := []Entry{
		{Timestamp: now.Add(-3 * time.Hour), Filter: "auth", VFSLines: 5, DurationMs: 100},
		{Timestamp: now.Add(-2 * time.Hour), Filter: "user", VFSLines: 0, DurationMs: 200},
		{Timestamp: now.Add(-1 * time.Hour), Filter: "", VFSLines: 42, DurationMs: 300},
		{Timestamp: now, Filter: "login", VFSLines: 3, DurationMs: 400},
	}

	s := Summarize(entries)

	if s.Invocations != 4 {
		t.Errorf("Invocations = %d, want 4", s.Invocations)
	}
	if s.Searches != 3 {
		t.Errorf("Searches = %d, want 3", s.Searches)
	}
	if s.Extracts != 1 {
		t.Errorf("Extracts = %d, want 1", s.Extracts)
	}
	if s.EmptySearches != 1 {
		t.Errorf("EmptySearches = %d, want 1", s.EmptySearches)
	}

	wantAvgMs := 250.0
	if s.AvgDurationMs != wantAvgMs {
		t.Errorf("AvgDurationMs = %f, want %f", s.AvgDurationMs, wantAvgMs)
	}

	// 2 hits out of 3 searches = 66.67%
	wantHitRate := float64(2) / float64(3) * 100
	if diff := s.SearchHitRate - wantHitRate; diff > 0.01 || diff < -0.01 {
		t.Errorf("SearchHitRate = %f, want %f", s.SearchHitRate, wantHitRate)
	}
}

func TestSummarize_Empty(t *testing.T) {
	s := Summarize(nil)
	if s.Invocations != 0 || s.Searches != 0 || s.Extracts != 0 {
		t.Errorf("expected zero summary for nil entries, got %+v", s)
	}
}

func TestSummarize_AllExtracts(t *testing.T) {
	entries := []Entry{
		{Timestamp: time.Now(), Filter: "", VFSLines: 10, DurationMs: 50},
		{Timestamp: time.Now(), Filter: "", VFSLines: 20, DurationMs: 150},
	}

	s := Summarize(entries)

	if s.Searches != 0 {
		t.Errorf("Searches = %d, want 0", s.Searches)
	}
	if s.Extracts != 2 {
		t.Errorf("Extracts = %d, want 2", s.Extracts)
	}
	if s.SearchHitRate != 0 {
		t.Errorf("SearchHitRate = %f, want 0 (no searches)", s.SearchHitRate)
	}
	if s.AvgDurationMs != 100 {
		t.Errorf("AvgDurationMs = %f, want 100", s.AvgDurationMs)
	}
}
