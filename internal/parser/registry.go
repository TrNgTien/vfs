package parser

import (
	"path/filepath"
	"strings"

	"github.com/TrNgTien/vfs/internal/parser/dockerparser"
	"github.com/TrNgTien/vfs/internal/parser/goparser"
	"github.com/TrNgTien/vfs/internal/parser/hclparser"
	"github.com/TrNgTien/vfs/internal/parser/protoparser"
	"github.com/TrNgTien/vfs/internal/parser/pyparser"
	"github.com/TrNgTien/vfs/internal/parser/sqlparser"
	"github.com/TrNgTien/vfs/internal/parser/tsparser"
	"github.com/TrNgTien/vfs/internal/parser/yamlparser"
)

// ExtractFunc is the common signature for all language extractors.
// It receives the file path and raw source bytes, returning signature lines.
type ExtractFunc func(filePath string, src []byte) ([]string, error)

// Extractor binds a set of file extensions to an extraction function,
// with an optional predicate to skip certain file names.
type Extractor struct {
	Extensions []string
	Skip       func(name string) bool
	Extract    ExtractFunc
}

var registry []Extractor

func init() {
	registerGo()
	registerTSJS()
	registerPython()
	registerHCL()
	registerDockerfile()
	registerProto()
	registerSQL()
	registerYAML()
}

// Register adds an extractor to the global registry.
// Call from init() in each parser package or from this file.
func Register(e Extractor) {
	registry = append(registry, e)
}

// FindExtractor returns the first registered extractor whose extension
// matches fileName and whose Skip predicate (if any) does not reject it.
func FindExtractor(fileName string) *Extractor {
	ext := strings.ToLower(filepath.Ext(fileName))
	lower := strings.ToLower(fileName)

	for i := range registry {
		e := &registry[i]
		if !matchesExt(e.Extensions, ext, lower) {
			continue
		}
		if e.Skip != nil && e.Skip(lower) {
			return nil
		}
		return e
	}
	return nil
}

func matchesExt(extensions []string, ext, lowerName string) bool {
	for _, candidate := range extensions {
		switch {
		case strings.HasPrefix(candidate, "="):
			// Exact base-name match (e.g. "=dockerfile")
			if lowerName == candidate[1:] {
				return true
			}
		case strings.HasPrefix(candidate, "^"):
			// Prefix match on base name (e.g. "^dockerfile.")
			if strings.HasPrefix(lowerName, candidate[1:]) {
				return true
			}
		default:
			// Extension match (e.g. ".tf")
			if ext == candidate {
				return true
			}
		}
	}
	return false
}

// --- built-in registrations ---

func registerGo() {
	Register(Extractor{
		Extensions: []string{".go"},
		Skip: func(name string) bool {
			return strings.HasSuffix(name, "_test.go")
		},
		Extract: func(filePath string, _ []byte) ([]string, error) {
			return goparser.ExtractExportedFuncs(filePath)
		},
	})
}

func registerTSJS() {
	Register(Extractor{
		Extensions: []string{".js", ".mjs", ".cjs", ".ts", ".mts", ".cts", ".jsx", ".tsx"},
		Skip: func(name string) bool {
			return strings.HasSuffix(name, ".d.ts") ||
				strings.Contains(name, ".test.") ||
				strings.Contains(name, ".spec.") ||
				strings.Contains(name, ".min.")
		},
		Extract: func(filePath string, src []byte) ([]string, error) {
			ext := filepath.Ext(filePath)
			lang, ok := tsparser.LangForExt(ext)
			if !ok {
				return nil, nil
			}
			return tsparser.ExtractExportedFuncs(filePath, src, lang)
		},
	})
}

func registerPython() {
	Register(Extractor{
		Extensions: []string{".py"},
		Skip: func(name string) bool {
			base := filepath.Base(name)
			return strings.HasPrefix(base, "test_") ||
				strings.HasSuffix(base, "_test.py") ||
				base == "conftest.py" ||
				base == "setup.py"
		},
		Extract: func(filePath string, src []byte) ([]string, error) {
			return pyparser.ExtractExportedFuncs(filePath, src)
		},
	})
}

func registerHCL() {
	Register(Extractor{
		Extensions: []string{".tf", ".hcl"},
		Extract: func(filePath string, src []byte) ([]string, error) {
			return hclparser.ExtractExportedFuncs(filePath, src)
		},
	})
}

func registerDockerfile() {
	Register(Extractor{
		// "=dockerfile"       -> exact match for "Dockerfile"
		// "^dockerfile."      -> prefix match for "Dockerfile.dev", "Dockerfile.prod", etc.
		// ".dockerfile"       -> extension match for "app.dockerfile"
		Extensions: []string{"=dockerfile", "^dockerfile.", ".dockerfile"},
		Extract: func(filePath string, src []byte) ([]string, error) {
			return dockerparser.ExtractExportedFuncs(filePath, src)
		},
	})
}

func registerProto() {
	Register(Extractor{
		Extensions: []string{".proto"},
		Extract: func(filePath string, src []byte) ([]string, error) {
			return protoparser.ExtractExportedFuncs(filePath, src)
		},
	})
}

func registerSQL() {
	Register(Extractor{
		Extensions: []string{".sql"},
		Extract: func(filePath string, src []byte) ([]string, error) {
			return sqlparser.ExtractExportedFuncs(filePath, src)
		},
	})
}

func registerYAML() {
	Register(Extractor{
		Extensions: []string{".yml", ".yaml"},
		Skip: func(name string) bool {
			// Skip lockfiles and generated manifests
			return name == "pnpm-lock.yaml" ||
				name == "package-lock.yaml" ||
				strings.HasSuffix(name, ".lock.yml") ||
				strings.HasSuffix(name, ".lock.yaml")
		},
		Extract: func(filePath string, src []byte) ([]string, error) {
			return yamlparser.ExtractExportedFuncs(filePath, src)
		},
	})
}
