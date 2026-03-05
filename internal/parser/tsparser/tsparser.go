package tsparser

import (
	"fmt"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

type Lang int

const (
	LangJS Lang = iota
	LangTS
	LangTSX
)

func LangForExt(ext string) (Lang, bool) {
	switch ext {
	case ".js", ".mjs", ".cjs":
		return LangJS, true
	case ".ts", ".mts", ".cts":
		return LangTS, true
	case ".jsx", ".tsx":
		return LangTSX, true
	default:
		return 0, false
	}
}

func newLanguage(lang Lang) *tree_sitter.Language {
	switch lang {
	case LangTS:
		return tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript())
	case LangTSX:
		return tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTSX())
	default:
		return tree_sitter.NewLanguage(tree_sitter_javascript.Language())
	}
}

// ExtractExportedFuncs parses a JS/TS source file and returns signatures
// of exported functions, classes, interfaces, types, and const/let/var declarations.
func ExtractExportedFuncs(filePath string, src []byte, lang Lang) ([]string, error) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(newLanguage(lang)); err != nil {
		return nil, fmt.Errorf("setting language for %s: %w", filePath, err)
	}

	tree := parser.Parse(src, nil)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse %s", filePath)
	}
	defer tree.Close()

	root := tree.RootNode()
	var sigs []string

	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child == nil {
			continue
		}

		sig := extractTopLevel(child, src)
		if sig != "" {
			sigs = append(sigs, sig)
		}
	}

	return sigs, nil
}

func extractTopLevel(node *tree_sitter.Node, src []byte) string {
	kind := node.Kind()

	switch kind {
	case "export_statement":
		return extractExportStatement(node, src)
	default:
		return ""
	}
}

func extractExportStatement(node *tree_sitter.Node, src []byte) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "function_declaration", "generator_function_declaration":
			return formatFuncDecl(child, src, "export ")
		case "class_declaration":
			return formatClassDecl(child, src, "export ")
		case "interface_declaration":
			return formatInterfaceDecl(child, src, "export ")
		case "type_alias_declaration":
			return "export " + strings.TrimSpace(child.Utf8Text(src))
		case "lexical_declaration":
			return formatLexicalDecl(child, src, "export ")
		case "variable_declaration":
			return formatLexicalDecl(child, src, "export ")
		case "abstract_class_declaration":
			return formatClassDecl(child, src, "export abstract ")
		case "enum_declaration":
			return formatEnumDecl(child, src, "export ")
		}
	}

	// "export default ..." or "export { ... }"
	text := node.Utf8Text(src)
	if idx := strings.Index(text, "{"); idx != -1 {
		if end := strings.Index(text, "}"); end > idx {
			return strings.TrimSpace(text[:end+1])
		}
	}

	first := firstLine(text)
	return strings.TrimSpace(first)
}

func formatFuncDecl(node *tree_sitter.Node, src []byte, prefix string) string {
	var name, params, returnType string
	isAsync := false
	isGenerator := node.Kind() == "generator_function_declaration"

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "async":
			isAsync = true
		case "identifier":
			name = child.Utf8Text(src)
		case "formal_parameters":
			params = child.Utf8Text(src)
		case "type_annotation":
			returnType = child.Utf8Text(src)
		}
	}

	var b strings.Builder
	b.WriteString(prefix)
	if isAsync {
		b.WriteString("async ")
	}
	b.WriteString("function")
	if isGenerator {
		b.WriteByte('*')
	}
	if name != "" {
		b.WriteByte(' ')
		b.WriteString(name)
	}

	typeParams := findChild(node, "type_parameters")
	if typeParams != nil {
		b.WriteString(typeParams.Utf8Text(src))
	}

	b.WriteString(params)
	if returnType != "" {
		b.WriteString(returnType)
	}
	return b.String()
}

func formatClassDecl(node *tree_sitter.Node, src []byte, prefix string) string {
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString("class ")

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "type_identifier", "identifier":
			b.WriteString(child.Utf8Text(src))
		case "type_parameters":
			b.WriteString(child.Utf8Text(src))
		case "class_heritage":
			b.WriteByte(' ')
			b.WriteString(child.Utf8Text(src))
		case "class_body":
			b.WriteString(" { ... }")
			return b.String()
		}
	}
	return b.String()
}

func formatInterfaceDecl(node *tree_sitter.Node, src []byte, prefix string) string {
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString("interface ")

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "type_identifier", "identifier":
			b.WriteString(child.Utf8Text(src))
		case "type_parameters":
			b.WriteString(child.Utf8Text(src))
		case "extends_type_clause":
			b.WriteByte(' ')
			b.WriteString(child.Utf8Text(src))
		case "object_type", "interface_body":
			b.WriteString(" { ... }")
			return b.String()
		}
	}
	return b.String()
}

func formatEnumDecl(node *tree_sitter.Node, src []byte, prefix string) string {
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString("enum ")

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "identifier":
			b.WriteString(child.Utf8Text(src))
		case "enum_body":
			b.WriteString(" { ... }")
			return b.String()
		}
	}
	return b.String()
}

func formatLexicalDecl(node *tree_sitter.Node, src []byte, prefix string) string {
	text := node.Utf8Text(src)

	if isArrowOrFuncExpr(text) {
		return prefix + stripFuncBody(text)
	}

	first := firstLine(text)
	if strings.Contains(first, "{") || strings.Contains(first, "[") {
		return prefix + strings.TrimSpace(first) + " ..."
	}
	return prefix + strings.TrimSpace(first)
}

func isArrowOrFuncExpr(text string) bool {
	return strings.Contains(text, "=>") || strings.Contains(text, "function(") || strings.Contains(text, "function (")
}

func stripFuncBody(text string) string {
	depth := 0
	inStr := byte(0)
	for i := 0; i < len(text); i++ {
		c := text[i]
		if inStr != 0 {
			if c == inStr && (i == 0 || text[i-1] != '\\') {
				inStr = 0
			}
			continue
		}
		switch c {
		case '"', '\'', '`':
			inStr = c
		case '{':
			depth++
			if depth == 1 {
				prefix := strings.TrimSpace(text[:i])
				if strings.HasSuffix(prefix, "=>") {
					return prefix + " { ... }"
				}
				return prefix + " { ... }"
			}
		}
	}
	return firstLine(text)
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}

func findChild(node *tree_sitter.Node, kind string) *tree_sitter.Node {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == kind {
			return child
		}
	}
	return nil
}
