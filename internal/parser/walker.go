package parser

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/TrNgTien/vfs/internal/parser/sig"
)

var skipDirs = map[string]bool{
	"vendor":       true,
	"node_modules": true,
	".git":         true,
	"testdata":     true,
	"dist":         true,
	"build":        true,
	".next":        true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	".tox":         true,
	".terraform":   true,
	"target":       true,
}

// ExtractFromFile parses a single file and returns its exported signatures.
// Parse errors are logged as warnings and the file is skipped (returns nil, nil).
func ExtractFromFile(filePath string) ([]sig.Sig, error) {
	name := filepath.Base(filePath)
	ext := FindExtractor(name)
	if ext == nil {
		return nil, nil
	}

	src, err := os.ReadFile(filePath)
	if err != nil {
		WarnFunc(filePath, err)
		return nil, nil
	}

	sigs, err := ext.Extract(filePath, src)
	if err != nil {
		WarnFunc(filePath, err)
		return nil, nil
	}
	return sigs, nil
}

// ExtractFromDir recursively walks root and returns formatted signature lines.
func ExtractFromDir(root string) ([]string, error) {
	results, err := ExtractFromDirDetailed(root)
	if err != nil {
		return nil, err
	}
	var all []string
	for _, r := range results {
		for _, s := range r.Sigs {
			all = append(all, s.FormatLine(r.RelPath))
		}
	}
	return all, nil
}

// WarnFunc is called when a file is skipped due to a read or parse error.
// Defaults to printing to stderr. Override in tests.
var WarnFunc = func(path string, err error) {
	fmt.Fprintf(os.Stderr, "vfs: warning: %s: %v (skipped)\n", path, err)
}

// ExtractFromDirDetailed returns per-file results with raw source sizes.
// Parse errors are logged as warnings; the offending file is skipped.
func ExtractFromDirDetailed(root string) ([]FileResult, error) {
	var results []FileResult

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			WarnFunc(path, err)
			return nil
		}

		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		ext := FindExtractor(d.Name())
		if ext == nil {
			return nil
		}

		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			WarnFunc(path, readErr)
			return nil
		}

		sigs, parseErr := ext.Extract(path, raw)
		if parseErr != nil {
			WarnFunc(path, parseErr)
			return nil
		}
		if len(sigs) == 0 {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		if rel == "" {
			rel = path
		}

		results = append(results, FileResult{
			RelPath:  rel,
			Sigs:     sigs,
			RawBytes: int64(len(raw)),
			RawLines: bytes.Count(raw, []byte{'\n'}),
		})
		return nil
	})

	return results, err
}

// CountSourceFiles counts all parseable source files under root.
func CountSourceFiles(root string) int {
	count := 0
	countWalk(root, &count)
	return count
}

func countWalk(dir string, count *int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			if skipDirs[e.Name()] {
				continue
			}
			countWalk(filepath.Join(dir, e.Name()), count)
			continue
		}
		if FindExtractor(e.Name()) != nil {
			*count++
		}
	}
}
