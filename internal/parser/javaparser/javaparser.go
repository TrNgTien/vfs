package javaparser

import (
	"fmt"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

// ExtractExportedFuncs parses a Java source file and returns signatures of
// public top-level declarations: classes, interfaces, enums, records, and
// their public methods. Annotations are preserved as prefixes.
func ExtractExportedFuncs(filePath string, src []byte) ([]string, error) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	lang := tree_sitter.NewLanguage(tree_sitter_java.Language())
	if err := parser.SetLanguage(lang); err != nil {
		return nil, fmt.Errorf("setting Java language for %s: %w", filePath, err)
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
		extracted := extractTopLevel(child, src, "")
		sigs = append(sigs, extracted...)
	}

	return sigs, nil
}

func extractTopLevel(node *tree_sitter.Node, src []byte, outerName string) []string {
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

func extractClassLike(node *tree_sitter.Node, src []byte, keyword string, outerName string) []string {
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

	var sig strings.Builder
	for _, ann := range annotations {
		sig.WriteString(ann)
		sig.WriteByte(' ')
	}
	sig.WriteString(strings.Join(filterModifiers(modifiers), " "))
	if sig.Len() > 0 && !strings.HasSuffix(sig.String(), " ") {
		sig.WriteByte(' ')
	}
	sig.WriteString(keyword)
	sig.WriteByte(' ')
	sig.WriteString(name)
	sig.WriteString(typeParams)
	if recordParams != "" {
		sig.WriteString(recordParams)
	}
	if superclass != "" {
		sig.WriteByte(' ')
		sig.WriteString(superclass)
	}
	if interfaces != "" {
		sig.WriteByte(' ')
		sig.WriteString(interfaces)
	}
	sig.WriteString(" { ... }")

	var sigs []string
	sigs = append(sigs, sig.String())

	qualifiedName := name
	if outerName != "" {
		qualifiedName = outerName + "." + name
	}

	if bodyNode != nil {
		sigs = append(sigs, extractMembers(bodyNode, src, qualifiedName)...)
	}

	return sigs
}

func extractMembers(body *tree_sitter.Node, src []byte, className string) []string {
	var sigs []string

	for i := uint(0); i < body.ChildCount(); i++ {
		child := body.Child(i)
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "method_declaration":
			if sig := formatMethod(child, src, className); sig != "" {
				sigs = append(sigs, sig)
			}
		case "constructor_declaration":
			if sig := formatConstructor(child, src, className); sig != "" {
				sigs = append(sigs, sig)
			}
		case "field_declaration":
			if sig := formatField(child, src, className); sig != "" {
				sigs = append(sigs, sig)
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
