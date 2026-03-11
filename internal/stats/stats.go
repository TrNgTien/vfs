package stats

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Entry struct {
	Timestamp     time.Time `json:"ts"`
	Project       string    `json:"project"`
	Filter        string    `json:"filter,omitempty"`
	FilesScanned  int       `json:"files_scanned"`
	FilesMatched  int       `json:"files_matched"`
	RawBytes      int64     `json:"raw_bytes"`
	RawLines      int       `json:"raw_lines"`
	VFSBytes      int       `json:"vfs_bytes"`
	VFSLines      int       `json:"vfs_lines"`
	ExportedFuncs int       `json:"exported_funcs"`
	TokensSaved   int64     `json:"tokens_saved"`
	ReductionPct  float64   `json:"reduction_pct"`
	DurationMs    int64     `json:"duration_ms"`
}

type Summary struct {
	Invocations   int
	TotalRawBytes int64
	TotalRawLines int
	TotalVFSBytes int64
	TotalVFSLines int
	TotalSaved    int64
	AvgReduction  float64
	FirstRecorded time.Time
	LastRecorded  time.Time

	Searches      int
	Extracts      int
	AvgDurationMs float64
	SearchHitRate float64
	EmptySearches int
}

func historyPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	return filepath.Join(home, ".vfs", "history.jsonl")
}

func Record(entry Entry) error {
	p := historyPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("creating stats dir: %w", err)
	}

	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening history file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling entry: %w", err)
	}
	data = append(data, '\n')

	_, err = f.Write(data)
	return err
}

func Load() ([]Entry, error) {
	f, err := os.Open(historyPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}

func Summarize(entries []Entry) Summary {
	if len(entries) == 0 {
		return Summary{}
	}

	s := Summary{
		Invocations:   len(entries),
		FirstRecorded: entries[0].Timestamp,
		LastRecorded:  entries[0].Timestamp,
	}

	var totalReduction float64
	var totalDuration int64
	var searchHits int
	for _, e := range entries {
		s.TotalRawBytes += e.RawBytes
		s.TotalRawLines += e.RawLines
		s.TotalVFSBytes += int64(e.VFSBytes)
		s.TotalVFSLines += e.VFSLines
		s.TotalSaved += e.TokensSaved
		totalReduction += e.ReductionPct
		totalDuration += e.DurationMs

		if e.Filter != "" {
			s.Searches++
			if e.VFSLines > 0 {
				searchHits++
			} else {
				s.EmptySearches++
			}
		} else {
			s.Extracts++
		}

		if e.Timestamp.Before(s.FirstRecorded) {
			s.FirstRecorded = e.Timestamp
		}
		if e.Timestamp.After(s.LastRecorded) {
			s.LastRecorded = e.Timestamp
		}
	}

	n := float64(len(entries))
	s.AvgReduction = totalReduction / n
	s.AvgDurationMs = float64(totalDuration) / n
	if s.Searches > 0 {
		s.SearchHitRate = float64(searchHits) / float64(s.Searches) * 100
	}
	return s
}

func Reset() error {
	p := historyPath()
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return nil
	}
	return os.Truncate(p, 0)
}
