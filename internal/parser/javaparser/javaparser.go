package javaparser

import (
	"fmt"
	"strings"
	"sync"

	"github.com/TrNgTien/vfs/internal/parser/sig"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

var (
	javaLangOnce sync.Once
	javaLang     *tree_sitter.Language
)

func language() *tree_sitter.Language {
	javaLangOnce.Do(func() {
		javaLang = tree_sitter.NewLanguage(tree_sitter_java.Language())
	})
	return javaLang
}

// ExtractExportedFuncs parses a Java source file and returns signatures of
// public top-level declarations: classes, interfaces, enums, records, and
// their public methods. Annotations are preserved as prefixes.
func ExtractExportedFuncs(filePath string, src []byte) ([]sig.Sig, error) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(language()); err != nil {
		return nil, fmt.Errorf("setting Java language for %s: %w", filePath, err)
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
		extracted := extractTopLevel(child, src, "")
		sigs = append(sigs, extracted...)
	}

	return sigs, nil
}

func extractTopLevel(node *tree_sitter.Node, src []byte, outerName string) []sig.Sig {
	kind := node.Kind()

	switch kind {
	case "class_declaration":
		return extractClassLike(node, src, "class", outerName)
	case "interface_declaration":
		return extractClassLike(node, src, "interface", outerName)
	case "enum_declaration":
		return extractClassLike(node, src, "enum", outerName)
	case "record_declaration":
		return extractClassLike(node, src, "record", outerName)
	case "annotation_type_declaration":
		return extractClassLike(node, src, "@interface", outerName)
	}

	return nil
}

func extractClassLike(node *tree_sitter.Node, src []byte, keyword string, outerName string) []sig.Sig {
	modifiers := collectModifiers(node, src)
	if outerName == "" && !hasPublicModifier(modifiers) {
		return nil
	}

	var name string
	var typeParams string
	var superclass string
	var interfaces string
	var annotations []string
	var bodyNode *tree_sitter.Node
	var recordParams string

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "modifiers":
			annotations = collectAnnotations(child, src)
		case "identifier":
			name = child.Utf8Text(src)
		case "type_parameters":
			typeParams = child.Utf8Text(src)
		case "superclass":
			superclass = child.Utf8Text(src)
		case "super_interfaces", "extends_interfaces":
			interfaces = child.Utf8Text(src)
		case "class_body", "interface_body", "enum_body":
			bodyNode = child
		case "formal_parameters":
			recordParams = child.Utf8Text(src)
		}
	}

	if name == "" {
		return nil
	}

	var s strings.Builder
	for _, ann := range annotations {
		s.WriteString(ann)
		s.WriteByte(' ')
	}
	s.WriteString(strings.Join(filterModifiers(modifiers), " "))
	if s.Len() > 0 && !strings.HasSuffix(s.String(), " ") {
		s.WriteByte(' ')
	}
	s.WriteString(keyword)
	s.WriteByte(' ')
	s.WriteString(name)
	s.WriteString(typeParams)
	if recordParams != "" {
		s.WriteString(recordParams)
	}
	if superclass != "" {
		s.WriteByte(' ')
		s.WriteString(superclass)
	}
	if interfaces != "" {
		s.WriteByte(' ')
		s.WriteString(interfaces)
	}
	s.WriteString(" { ... }")

	var sigs []sig.Sig
	sigs = append(sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})

	qualifiedName := name
	if outerName != "" {
		qualifiedName = outerName + "." + name
	}

	if bodyNode != nil {
		sigs = append(sigs, extractMembers(bodyNode, src, qualifiedName)...)
	}

	return sigs
}

func extractMembers(body *tree_sitter.Node, src []byte, className string) []sig.Sig {
	var sigs []sig.Sig

	for i := uint(0); i < body.ChildCount(); i++ {
		child := body.Child(i)
		if child == nil {
			continue
		}

		line := int(child.StartPosition().Row) + 1

		switch child.Kind() {
		case "method_declaration":
			if text := formatMethod(child, src, className); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: text})
			}
		case "constructor_declaration":
			if text := formatConstructor(child, src, className); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: text})
			}
		case "field_declaration":
			if text := formatField(child, src, className); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: text})
			}
		case "class_declaration":
			nested := extractClassLike(child, src, "class", className)
			sigs = append(sigs, nested...)
		case "interface_declaration":
			nested := extractClassLike(child, src, "interface", className)
			sigs = append(sigs, nested...)
		case "enum_declaration":
			nested := extractClassLike(child, src, "enum", className)
			sigs = append(sigs, nested...)
		case "record_declaration":
			nested := extractClassLike(child, src, "record", className)
			sigs = append(sigs, nested...)
		}
	}

	return sigs
}

func formatMethod(node *tree_sitter.Node, src []byte, className string) string {
	modifiers := collectModifiers(node, src)
	if !hasPublicModifier(modifiers) {
		return ""
	}

	var annotations []string
	var returnType, name, params, typeParams string

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "modifiers":
			annotations = collectAnnotations(child, src)
		case "type_identifier", "void_type", "integral_type", "floating_point_type",
			"boolean_type", "generic_type", "array_type", "scoped_type_identifier":
			returnType = child.Utf8Text(src)
		case "identifier":
			name = child.Utf8Text(src)
		case "formal_parameters":
			params = child.Utf8Text(src)
		case "type_parameters":
			typeParams = child.Utf8Text(src)
		}
	}

	if name == "" {
		return ""
	}

	var b strings.Builder
	for _, ann := range annotations {
		b.WriteString(ann)
		b.WriteByte(' ')
	}
	b.WriteString(strings.Join(filterModifiers(modifiers), " "))
	if b.Len() > 0 && !strings.HasSuffix(b.String(), " ") {
		b.WriteByte(' ')
	}
	if typeParams != "" {
		b.WriteString(typeParams)
		b.WriteByte(' ')
	}
	if returnType != "" {
		b.WriteString(returnType)
		b.WriteByte(' ')
	}
	b.WriteString(className)
	b.WriteByte('.')
	b.WriteString(name)
	b.WriteString(params)

	return b.String()
}

func formatConstructor(node *tree_sitter.Node, src []byte, className string) string {
	modifiers := collectModifiers(node, src)
	if !hasPublicModifier(modifiers) {
		return ""
	}

	var name, params string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "identifier":
			name = child.Utf8Text(src)
		case "formal_parameters":
			params = child.Utf8Text(src)
		}
	}

	if name == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString(strings.Join(filterModifiers(modifiers), " "))
	if b.Len() > 0 {
		b.WriteByte(' ')
	}
	b.WriteString(className)
	b.WriteByte('.')
	b.WriteString(name)
	b.WriteString(params)

	return b.String()
}

func formatField(node *tree_sitter.Node, src []byte, className string) string {
	modifiers := collectModifiers(node, src)
	if !hasPublicModifier(modifiers) || !isStaticFinal(modifiers) {
		return ""
	}

	var fieldType, name, value string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "modifiers":
			// handled above
		case "type_identifier", "integral_type", "floating_point_type",
			"boolean_type", "generic_type", "array_type", "scoped_type_identifier",
			"void_type":
			fieldType = child.Utf8Text(src)
		case "variable_declarator":
			declText := child.Utf8Text(src)
			name = declText
			if eqIdx := strings.IndexByte(declText, '='); eqIdx >= 0 {
				name = strings.TrimSpace(declText[:eqIdx])
				value = strings.TrimSpace(declText[eqIdx:])
			}
		}
	}

	if name == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString(strings.Join(filterModifiers(modifiers), " "))
	if b.Len() > 0 {
		b.WriteByte(' ')
	}
	if fieldType != "" {
		b.WriteString(fieldType)
		b.WriteByte(' ')
	}
	b.WriteString(className)
	b.WriteByte('.')
	b.WriteString(name)
	if value != "" {
		b.WriteByte(' ')
		b.WriteString(value)
	}

	return b.String()
}

func collectModifiers(node *tree_sitter.Node, src []byte) []string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "modifiers" {
			var mods []string
			for j := uint(0); j < child.ChildCount(); j++ {
				mod := child.Child(j)
				if mod == nil {
					continue
				}
				if mod.Kind() != "marker_annotation" && mod.Kind() != "annotation" {
					mods = append(mods, mod.Utf8Text(src))
				}
			}
			return mods
		}
	}
	return nil
}

func collectAnnotations(modNode *tree_sitter.Node, src []byte) []string {
	var annotations []string
	for i := uint(0); i < modNode.ChildCount(); i++ {
		child := modNode.Child(i)
		if child == nil {
			continue
		}
		if child.Kind() == "marker_annotation" || child.Kind() == "annotation" {
			annotations = append(annotations, child.Utf8Text(src))
		}
	}
	return annotations
}

func hasPublicModifier(modifiers []string) bool {
	for _, m := range modifiers {
		if m == "public" {
			return true
		}
	}
	return false
}

func isStaticFinal(modifiers []string) bool {
	hasStatic, hasFinal := false, false
	for _, m := range modifiers {
		if m == "static" {
			hasStatic = true
		}
		if m == "final" {
			hasFinal = true
		}
	}
	return hasStatic && hasFinal
}

// filterModifiers returns modifiers without access modifiers (public/private/protected),
// keeping only keywords like static, final, abstract, synchronized, etc.
func filterModifiers(modifiers []string) []string {
	var filtered []string
	for _, m := range modifiers {
		switch m {
		case "public", "private", "protected":
			filtered = append(filtered, m)
		default:
			filtered = append(filtered, m)
		}
	}
	return filtered
}
