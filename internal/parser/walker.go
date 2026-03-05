package parser

import (
	"bytes"
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
func ExtractFromFile(filePath string) ([]sig.Sig, error) {
	name := filepath.Base(filePath)
	ext := FindExtractor(name)
	if ext == nil {
		return nil, nil
	}

	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return ext.Extract(filePath, src)
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

// ExtractFromDirDetailed returns per-file results with raw source sizes.
func ExtractFromDirDetailed(root string) ([]FileResult, error) {
	var results []FileResult

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
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

		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		sigs, err := ext.Extract(path, raw)
		if err != nil {
			return err
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
