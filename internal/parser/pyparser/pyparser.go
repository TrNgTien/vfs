package pyparser

import (
	"fmt"
	"strings"
	"unicode"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

// ExtractExportedFuncs parses a Python source file and returns signatures of
// top-level public functions, classes, and module-level UPPER_CASE constants.
// Private names (leading underscore) are skipped.
func ExtractExportedFuncs(filePath string, src []byte) ([]string, error) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	lang := tree_sitter.NewLanguage(tree_sitter_python.Language())
	if err := parser.SetLanguage(lang); err != nil {
		return nil, fmt.Errorf("setting Python language for %s: %w", filePath, err)
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
		extracted := extractTopLevel(child, src)
		sigs = append(sigs, extracted...)
	}

	return sigs, nil
}

func extractTopLevel(node *tree_sitter.Node, src []byte) []string {
	switch node.Kind() {
	case "function_definition":
		if sig := formatFuncDef(node, src, ""); sig != "" {
			return []string{sig}
		}
	case "class_definition":
		if sig := formatClassDef(node, src, ""); sig != "" {
			return []string{sig}
		}
	case "decorated_definition":
		return extractDecorated(node, src)
	case "expression_statement":
		if sig := extractConstAssignment(node, src); sig != "" {
			return []string{sig}
		}
	}
	return nil
}

func formatFuncDef(node *tree_sitter.Node, src []byte, prefix string) string {
	var name, params, returnType string

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "identifier":
			name = child.Utf8Text(src)
		case "parameters":
			params = child.Utf8Text(src)
		case "type":
			returnType = child.Utf8Text(src)
		}
	}

	if name == "" || strings.HasPrefix(name, "_") {
		return ""
	}

	// tree-sitter-python: "async def foo():" is parsed as a function_definition
	// whose source text starts with "async". Check the raw text prefix.
	fullText := node.Utf8Text(src)
	isAsync := strings.HasPrefix(strings.TrimSpace(fullText), "async ")

	var b strings.Builder
	b.WriteString(prefix)
	if isAsync {
		b.WriteString("async ")
	}
	b.WriteString("def ")
	b.WriteString(name)
	b.WriteString(params)
	if returnType != "" {
		b.WriteString(" -> ")
		b.WriteString(returnType)
	}
	return b.String()
}

func formatClassDef(node *tree_sitter.Node, src []byte, prefix string) string {
	var name string
	var argList string

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "identifier":
			name = child.Utf8Text(src)
		case "argument_list":
			argList = child.Utf8Text(src)
		}
	}

	if name == "" || strings.HasPrefix(name, "_") {
		return ""
	}

	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString("class ")
	b.WriteString(name)
	if argList != "" {
		b.WriteString(argList)
	}
	return b.String()
}

func extractDecorated(node *tree_sitter.Node, src []byte) []string {
	var decorators []string
	var innerSig string

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "decorator":
			decorators = append(decorators, strings.TrimSpace(child.Utf8Text(src)))
		case "function_definition":
			innerSig = formatFuncDef(child, src, "")
		case "class_definition":
			innerSig = formatClassDef(child, src, "")
		}
	}

	if innerSig == "" {
		return nil
	}

	prefix := strings.Join(decorators, " ") + " "
	return []string{prefix + innerSig}
}

func extractConstAssignment(node *tree_sitter.Node, src []byte) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Kind() != "assignment" {
			continue
		}

		lhs := child.ChildByFieldName("left")
		if lhs == nil {
			continue
		}
		name := lhs.Utf8Text(src)
		if !isUpperSnakeCase(name) || strings.HasPrefix(name, "_") {
			continue
		}

		return strings.TrimSpace(child.Utf8Text(src))
	}
	return ""
}

func isUpperSnakeCase(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsUpper(r) && r != '_' && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
