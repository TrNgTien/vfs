package parser

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/TrNgTien/vfs/internal/parser/goparser"
	"github.com/TrNgTien/vfs/internal/parser/tsparser"
)

var skipDirs = map[string]bool{
	"vendor":       true,
	"node_modules": true,
	".git":         true,
	"testdata":     true,
	"dist":         true,
	"build":        true,
	".next":        true,
}

func isGoFile(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func isJSTSFile(name string) bool {
	ext := filepath.Ext(name)
	_, ok := tsparser.LangForExt(ext)
	if !ok {
		return false
	}
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".d.ts") ||
		strings.Contains(lower, ".test.") ||
		strings.Contains(lower, ".spec.") ||
		strings.Contains(lower, ".min.") {
		return false
	}
	return true
}

// ExtractFromFile parses a single file (Go or JS/TS) and returns its exported signatures.
func ExtractFromFile(filePath string) ([]string, error) {
	name := filepath.Base(filePath)

	if isGoFile(name) {
		return goparser.ExtractExportedFuncs(filePath)
	}

	ext := filepath.Ext(name)
	if lang, ok := tsparser.LangForExt(ext); ok {
		src, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
		return tsparser.ExtractExportedFuncs(filePath, src, lang)
	}

	return nil, nil
}

// ExtractFromDir recursively walks root and returns signatures prefixed with relative paths.
func ExtractFromDir(root string) ([]string, error) {
	results, err := ExtractFromDirDetailed(root)
	if err != nil {
		return nil, err
	}
	var all []string
	for _, r := range results {
		for _, sig := range r.Sigs {
			all = append(all, r.RelPath+": "+sig)
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

		name := d.Name()
		isGo := isGoFile(name)
		isTS := isJSTSFile(name)
		if !isGo && !isTS {
			return nil
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var sigs []string
		if isGo {
			sigs, err = goparser.ExtractExportedFuncs(path)
		} else {
			ext := filepath.Ext(name)
			lang, _ := tsparser.LangForExt(ext)
			sigs, err = tsparser.ExtractExportedFuncs(path, raw, lang)
		}
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

// CountSourceFiles counts all parseable source files (Go + JS/TS) under root.
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
		if isGoFile(e.Name()) || isJSTSFile(e.Name()) {
			*count++
		}
	}
}
