package rustparser

import (
	"fmt"
	"strings"
	"sync"

	"github.com/TrNgTien/vfs/internal/parser/sig"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
)

var (
	rsLangOnce sync.Once
	rsLang     *tree_sitter.Language
)

func language() *tree_sitter.Language {
	rsLangOnce.Do(func() {
		rsLang = tree_sitter.NewLanguage(tree_sitter_rust.Language())
	})
	return rsLang
}

// ExtractExportedFuncs parses a Rust source file and returns signatures of
// top-level public items: functions, structs, enums, traits, type aliases,
// constants, statics, impl blocks (with pub methods), and modules.
func ExtractExportedFuncs(filePath string, src []byte) ([]sig.Sig, error) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(language()); err != nil {
		return nil, fmt.Errorf("setting Rust language for %s: %w", filePath, err)
	}

	tree := parser.Parse(src, nil)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse %s", filePath)
	}
	defer tree.Close()

	root := tree.RootNode()
	var sigs []sig.Sig

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

func extractTopLevel(node *tree_sitter.Node, src []byte) []sig.Sig {
	kind := node.Kind()
	line := int(node.StartPosition().Row) + 1

	switch kind {
	case "function_item":
		if text := formatPubItem(node, src, formatFuncItem); text != "" {
			return []sig.Sig{{Line: line, Text: text}}
		}
	case "struct_item":
		if text := formatPubItem(node, src, formatStructItem); text != "" {
			return []sig.Sig{{Line: line, Text: text}}
		}
	case "enum_item":
		if text := formatPubItem(node, src, formatEnumItem); text != "" {
			return []sig.Sig{{Line: line, Text: text}}
		}
	case "trait_item":
		if text := formatPubItem(node, src, formatTraitItem); text != "" {
			return []sig.Sig{{Line: line, Text: text}}
		}
	case "type_item":
		if text := formatPubItem(node, src, formatTypeItem); text != "" {
			return []sig.Sig{{Line: line, Text: text}}
		}
	case "const_item":
		if text := formatPubItem(node, src, formatConstOrStatic); text != "" {
			return []sig.Sig{{Line: line, Text: text}}
		}
	case "static_item":
		if text := formatPubItem(node, src, formatConstOrStatic); text != "" {
			return []sig.Sig{{Line: line, Text: text}}
		}
	case "mod_item":
		if text := formatPubItem(node, src, formatModItem); text != "" {
			return []sig.Sig{{Line: line, Text: text}}
		}
	case "impl_item":
		return extractImplBlock(node, src)
	case "attribute_item":
		// #[cfg(...)] etc. -- skip standalone attributes
	}

	return nil
}

type formatter func(node *tree_sitter.Node, src []byte) string

// formatPubItem checks that the node has a visibility_modifier starting with "pub",
// then delegates to the given formatter.
func formatPubItem(node *tree_sitter.Node, src []byte, fn formatter) string {
	if !isPub(node, src) {
		return ""
	}
	return fn(node, src)
}

func isPub(node *tree_sitter.Node, src []byte) bool {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "visibility_modifier" {
			text := child.Utf8Text(src)
			return strings.HasPrefix(text, "pub")
		}
	}
	return false
}

func formatFuncItem(node *tree_sitter.Node, src []byte) string {
	var b strings.Builder
	pastParams := false
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "visibility_modifier":
			b.WriteString(child.Utf8Text(src))
			b.WriteByte(' ')
		case "function_modifiers":
			b.WriteString(child.Utf8Text(src))
			b.WriteByte(' ')
		case "fn":
			b.WriteString("fn ")
		case "identifier":
			if !pastParams {
				b.WriteString(child.Utf8Text(src))
			}
		case "type_parameters":
			b.WriteString(child.Utf8Text(src))
		case "parameters":
			b.WriteString(child.Utf8Text(src))
			pastParams = true
		case "->":
			b.WriteString(" -> ")
		case "primitive_type", "type_identifier", "scoped_type_identifier",
			"generic_type", "reference_type", "tuple_type", "array_type",
			"function_type", "pointer_type", "bounded_type", "empty_type",
			"abstract_type", "dynamic_type", "never_type", "macro_invocation":
			if pastParams {
				b.WriteString(child.Utf8Text(src))
			}
		case "where_clause":
			b.WriteByte(' ')
			b.WriteString(child.Utf8Text(src))
		case "block":
			return b.String()
		}
	}
	return b.String()
}

func formatStructItem(node *tree_sitter.Node, src []byte) string {
	var b strings.Builder
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "visibility_modifier":
			b.WriteString(child.Utf8Text(src))
			b.WriteByte(' ')
		case "struct":
			b.WriteString("struct ")
		case "type_identifier":
			b.WriteString(child.Utf8Text(src))
		case "type_parameters":
			b.WriteString(child.Utf8Text(src))
		case "where_clause":
			b.WriteByte(' ')
			b.WriteString(child.Utf8Text(src))
		case "field_declaration_list":
			b.WriteString(" { ... }")
			return b.String()
		case "ordered_field_declaration_list":
			b.WriteString(child.Utf8Text(src))
			return b.String()
		}
	}
	return b.String()
}

func formatEnumItem(node *tree_sitter.Node, src []byte) string {
	var b strings.Builder
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "visibility_modifier":
			b.WriteString(child.Utf8Text(src))
			b.WriteByte(' ')
		case "enum":
			b.WriteString("enum ")
		case "type_identifier":
			b.WriteString(child.Utf8Text(src))
		case "type_parameters":
			b.WriteString(child.Utf8Text(src))
		case "where_clause":
			b.WriteByte(' ')
			b.WriteString(child.Utf8Text(src))
		case "enum_variant_list":
			b.WriteString(" { ... }")
			return b.String()
		}
	}
	return b.String()
}

func formatTraitItem(node *tree_sitter.Node, src []byte) string {
	var b strings.Builder
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "visibility_modifier":
			b.WriteString(child.Utf8Text(src))
			b.WriteByte(' ')
		case "trait":
			b.WriteString("trait ")
		case "type_identifier":
			b.WriteString(child.Utf8Text(src))
		case "type_parameters":
			b.WriteString(child.Utf8Text(src))
		case "trait_bounds":
			b.WriteString(": ")
			b.WriteString(child.Utf8Text(src))
		case "where_clause":
			b.WriteByte(' ')
			b.WriteString(child.Utf8Text(src))
		case "declaration_list":
			b.WriteString(" { ... }")
			return b.String()
		}
	}
	return b.String()
}

func formatTypeItem(node *tree_sitter.Node, src []byte) string {
	text := strings.TrimSpace(node.Utf8Text(src))
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		text = strings.TrimSpace(text[:idx])
	}
	return text
}

func formatConstOrStatic(node *tree_sitter.Node, src []byte) string {
	text := strings.TrimSpace(node.Utf8Text(src))
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		text = strings.TrimSpace(text[:idx])
	}
	return text
}

func formatModItem(node *tree_sitter.Node, src []byte) string {
	var b strings.Builder
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "visibility_modifier":
			b.WriteString(child.Utf8Text(src))
			b.WriteByte(' ')
		case "mod":
			b.WriteString("mod ")
		case "identifier":
			b.WriteString(child.Utf8Text(src))
		case "declaration_list":
			b.WriteString(" { ... }")
			return b.String()
		}
	}
	return b.String()
}

// extractImplBlock extracts the impl header and any pub methods inside.
func extractImplBlock(node *tree_sitter.Node, src []byte) []sig.Sig {
	var header strings.Builder
	var sigs []sig.Sig

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "impl":
			header.WriteString("impl ")
		case "type_parameters":
			header.WriteString(child.Utf8Text(src))
		case "type_identifier", "scoped_type_identifier", "generic_type":
			header.WriteString(child.Utf8Text(src))
		case "for":
			header.WriteString(" for ")
		case "where_clause":
			header.WriteString(" ")
			header.WriteString(child.Utf8Text(src))
		case "declaration_list":
			prefix := header.String()
			sigs = append(sigs, extractImplMethods(child, src, prefix)...)
		}
	}

	return sigs
}

func extractImplMethods(declList *tree_sitter.Node, src []byte, implHeader string) []sig.Sig {
	var sigs []sig.Sig

	for i := uint(0); i < declList.ChildCount(); i++ {
		child := declList.Child(i)
		if child == nil {
			continue
		}
		if child.Kind() == "function_item" && isPub(child, src) {
			fnSig := formatFuncItem(child, src)
			if fnSig != "" {
				line := int(child.StartPosition().Row) + 1
				sigs = append(sigs, sig.Sig{
					Line: line,
					Text: implHeader + "::" + trimVisibility(fnSig),
				})
			}
		}
	}

	return sigs
}

func trimVisibility(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "pub(crate) ") {
		return s[len("pub(crate) "):]
	}
	if strings.HasPrefix(s, "pub(super) ") {
		return s[len("pub(super) "):]
	}
	if strings.HasPrefix(s, "pub ") {
		return s[len("pub "):]
	}
	return s
}
