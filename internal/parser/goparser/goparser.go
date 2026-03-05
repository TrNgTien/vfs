package goparser

import (
	"bytes"
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/printer"
	"go/token"
	"strings"
)

// ExtractExportedFuncs parses a Go source file and returns the signatures
// of all exported functions with bodies stripped.
func ExtractExportedFuncs(filePath string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filePath, err)
	}

	var sigs []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !ast.IsExported(fn.Name.Name) {
			continue
		}

		fn.Body = nil

		var buf bytes.Buffer
		if err := printer.Fprint(&buf, fset, fn); err != nil {
			return nil, fmt.Errorf("printing func %s: %w", fn.Name.Name, err)
		}
		sigs = append(sigs, strings.TrimSpace(buf.String()))
	}

	return sigs, nil
}
