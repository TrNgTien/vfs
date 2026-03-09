package hclparser

import (
	"fmt"
	"strings"
	"sync"

	"github.com/TrNgTien/vfs/internal/parser/sig"
	tree_sitter_hcl "github.com/tree-sitter-grammars/tree-sitter-hcl/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

var (
	hclLangOnce sync.Once
	hclLang     *tree_sitter.Language
)

func language() *tree_sitter.Language {
	hclLangOnce.Do(func() {
		hclLang = tree_sitter.NewLanguage(tree_sitter_hcl.Language())
	})
	return hclLang
}

// ExtractExportedFuncs parses an HCL/Terraform file and returns one-line
// signatures for each top-level block and attribute. Block bodies are
// replaced with "{ ... }" so the output acts as a table of contents.
//
// Example output:
//
//	resource "aws_instance" "web" { ... }
//	variable "region" { ... }
//	output "vpc_id" { ... }
//	locals { ... }
func ExtractExportedFuncs(filePath string, src []byte) ([]sig.Sig, error) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(language()); err != nil {
		return nil, fmt.Errorf("setting HCL language for %s: %w", filePath, err)
	}

	tree := parser.Parse(src, nil)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse %s", filePath)
	}
	defer tree.Close()

	root := tree.RootNode()
	var sigs []sig.Sig

	// config_file -> body -> (block | attribute)*
	body := findChild(root, "body")
	if body == nil {
		return nil, nil
	}

	for i := uint(0); i < body.ChildCount(); i++ {
		child := body.Child(i)
		if child == nil {
			continue
		}
		line := int(child.StartPosition().Row) + 1
		switch child.Kind() {
		case "block":
			if text := formatBlock(child, src); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: text})
			}
		case "attribute":
			if text := formatAttribute(child, src); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: text})
			}
		}
	}

	return sigs, nil
}

// formatBlock produces e.g. `resource "aws_instance" "web" { ... }`
func formatBlock(node *tree_sitter.Node, src []byte) string {
	var parts []string

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "identifier":
			parts = append(parts, child.Utf8Text(src))
		case "string_lit":
			parts = append(parts, child.Utf8Text(src))
		case "body", "block_start", "block_end":
			// skip body content
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " ") + " { ... }"
}

// formatAttribute produces e.g. `terraform_version = "~> 1.5"`
func formatAttribute(node *tree_sitter.Node, src []byte) string {
	text := strings.TrimSpace(node.Utf8Text(src))
	// Keep only the first line for multi-line expressions
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		text = strings.TrimSpace(text[:idx])
	}
	return text
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
