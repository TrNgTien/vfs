package csharpparser

import (
	"fmt"
	"strings"

	"github.com/TrNgTien/vfs/internal/parser/sig"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_csharp "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
)

// ExtractExportedFuncs parses a C# source file and returns signatures of
// public declarations: classes, structs, interfaces, enums, records, delegates,
// and their public members (methods, constructors, properties).
func ExtractExportedFuncs(filePath string, src []byte) ([]sig.Sig, error) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	lang := tree_sitter.NewLanguage(tree_sitter_csharp.Language())
	if err := parser.SetLanguage(lang); err != nil {
		return nil, fmt.Errorf("setting C# language for %s: %w", filePath, err)
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
		sigs = append(sigs, extractTopLevel(child, src, "")...)
	}

	return sigs, nil
}

func extractTopLevel(node *tree_sitter.Node, src []byte, outerName string) []sig.Sig {
	switch node.Kind() {
	case "namespace_declaration":
		return extractNamespace(node, src, outerName)
	case "file_scoped_namespace_declaration":
		return extractFileScopedNamespace(node, src, outerName)
	case "class_declaration":
		return extractTypeLike(node, src, "class", outerName)
	case "struct_declaration":
		return extractTypeLike(node, src, "struct", outerName)
	case "interface_declaration":
		return extractTypeLike(node, src, "interface", outerName)
	case "enum_declaration":
		return extractTypeLike(node, src, "enum", outerName)
	case "record_declaration":
		return extractRecord(node, src, outerName)
	case "delegate_declaration":
		return extractDelegate(node, src, outerName)
	}
	return nil
}

func extractNamespace(node *tree_sitter.Node, src []byte, outerName string) []sig.Sig {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	nsName := nameNode.Utf8Text(src)
	if outerName != "" {
		nsName = outerName + "." + nsName
	}

	bodyNode := node.ChildByFieldName("body")
	if bodyNode == nil {
		return nil
	}

	var sigs []sig.Sig
	for i := uint(0); i < bodyNode.ChildCount(); i++ {
		child := bodyNode.Child(i)
		if child == nil {
			continue
		}
		sigs = append(sigs, extractTopLevel(child, src, nsName)...)
	}
	return sigs
}

func extractFileScopedNamespace(node *tree_sitter.Node, src []byte, outerName string) []sig.Sig {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	nsName := nameNode.Utf8Text(src)
	if outerName != "" {
		nsName = outerName + "." + nsName
	}

	// File-scoped namespaces have no body node; type declarations are siblings
	var sigs []sig.Sig
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		sigs = append(sigs, extractTopLevel(child, src, nsName)...)
	}
	return sigs
}

func extractTypeLike(node *tree_sitter.Node, src []byte, keyword string, outerName string) []sig.Sig {
	mods := collectModifiers(node, src)
	if outerName == "" && !hasPublicModifier(mods) {
		return nil
	}
	if outerName != "" && !hasPublicModifier(mods) {
		return nil
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Utf8Text(src)

	var s strings.Builder
	writeAttributes(node, src, &s)
	writeModifiers(mods, &s)
	s.WriteString(keyword)
	s.WriteByte(' ')
	s.WriteString(name)
	writeTypeParams(node, src, &s)
	writeBaseList(node, src, &s)
	writeConstraints(node, src, &s)
	s.WriteString(" { ... }")

	qualifiedName := qualifyName(outerName, name)

	var sigs []sig.Sig
	sigs = append(sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		sigs = append(sigs, extractMembers(bodyNode, src, qualifiedName)...)
	}

	return sigs
}

func extractRecord(node *tree_sitter.Node, src []byte, outerName string) []sig.Sig {
	mods := collectModifiers(node, src)
	if !hasPublicModifier(mods) {
		return nil
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Utf8Text(src)

	// Detect "record struct" vs "record class" vs plain "record"
	recordKind := detectRecordKind(node, src)

	var s strings.Builder
	writeAttributes(node, src, &s)
	writeModifiers(mods, &s)
	s.WriteString(recordKind)
	s.WriteByte(' ')
	s.WriteString(name)
	writeTypeParams(node, src, &s)
	writeParamList(node, src, &s)
	writeBaseList(node, src, &s)
	writeConstraints(node, src, &s)

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		s.WriteString(" { ... }")
	}

	qualifiedName := qualifyName(outerName, name)

	var sigs []sig.Sig
	sigs = append(sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})

	if bodyNode != nil {
		sigs = append(sigs, extractMembers(bodyNode, src, qualifiedName)...)
	}

	return sigs
}

func extractDelegate(node *tree_sitter.Node, src []byte, outerName string) []sig.Sig {
	mods := collectModifiers(node, src)
	if !hasPublicModifier(mods) {
		return nil
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	var s strings.Builder
	writeAttributes(node, src, &s)
	writeModifiers(mods, &s)
	s.WriteString("delegate ")

	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		s.WriteString(typeNode.Utf8Text(src))
		s.WriteByte(' ')
	}

	s.WriteString(nameNode.Utf8Text(src))
	writeTypeParams(node, src, &s)

	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode != nil {
		s.WriteString(paramsNode.Utf8Text(src))
	}

	writeConstraints(node, src, &s)

	return []sig.Sig{{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	}}
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
		case "property_declaration":
			if text := formatProperty(child, src, className); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: text})
			}
		case "field_declaration":
			if text := formatField(child, src, className); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: text})
			}
		case "class_declaration":
			sigs = append(sigs, extractTypeLike(child, src, "class", className)...)
		case "struct_declaration":
			sigs = append(sigs, extractTypeLike(child, src, "struct", className)...)
		case "interface_declaration":
			sigs = append(sigs, extractTypeLike(child, src, "interface", className)...)
		case "enum_declaration":
			sigs = append(sigs, extractTypeLike(child, src, "enum", className)...)
		case "record_declaration":
			sigs = append(sigs, extractRecord(child, src, className)...)
		case "delegate_declaration":
			sigs = append(sigs, extractDelegate(child, src, className)...)
		}
	}

	return sigs
}

func formatMethod(node *tree_sitter.Node, src []byte, className string) string {
	mods := collectModifiers(node, src)
	if !hasPublicModifier(mods) {
		return ""
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return ""
	}

	var b strings.Builder
	writeAttributes(node, src, &b)
	writeModifiers(mods, &b)

	returnsNode := node.ChildByFieldName("returns")
	if returnsNode != nil {
		b.WriteString(returnsNode.Utf8Text(src))
		b.WriteByte(' ')
	}

	b.WriteString(className)
	b.WriteByte('.')
	b.WriteString(nameNode.Utf8Text(src))

	writeTypeParams(node, src, &b)

	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode != nil {
		b.WriteString(paramsNode.Utf8Text(src))
	}

	writeConstraints(node, src, &b)

	return b.String()
}

func formatConstructor(node *tree_sitter.Node, src []byte, className string) string {
	mods := collectModifiers(node, src)
	if !hasPublicModifier(mods) {
		return ""
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return ""
	}

	var b strings.Builder
	writeModifiers(mods, &b)
	b.WriteString(className)
	b.WriteByte('.')
	b.WriteString(nameNode.Utf8Text(src))

	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode != nil {
		b.WriteString(paramsNode.Utf8Text(src))
	}

	return b.String()
}

func formatProperty(node *tree_sitter.Node, src []byte, className string) string {
	mods := collectModifiers(node, src)
	if !hasPublicModifier(mods) {
		return ""
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return ""
	}

	var b strings.Builder
	writeAttributes(node, src, &b)
	writeModifiers(mods, &b)

	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		b.WriteString(typeNode.Utf8Text(src))
		b.WriteByte(' ')
	}

	b.WriteString(className)
	b.WriteByte('.')
	b.WriteString(nameNode.Utf8Text(src))

	accessorsNode := node.ChildByFieldName("accessors")
	if accessorsNode != nil {
		b.WriteString(" ")
		b.WriteString(formatAccessorList(accessorsNode, src))
	}

	return b.String()
}

func formatField(node *tree_sitter.Node, src []byte, className string) string {
	mods := collectModifiers(node, src)
	if !hasPublicModifier(mods) || !isConstOrStaticReadonly(mods) {
		return ""
	}

	var varDecl *tree_sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "variable_declaration" {
			varDecl = child
			break
		}
	}
	if varDecl == nil {
		return ""
	}

	var fieldType string
	var names []string
	for i := uint(0); i < varDecl.ChildCount(); i++ {
		child := varDecl.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "variable_declarator":
			idNode := child.ChildByFieldName("name")
			if idNode == nil {
				for j := uint(0); j < child.ChildCount(); j++ {
					if c := child.Child(j); c != nil && c.Kind() == "identifier" {
						idNode = c
						break
					}
				}
			}
			if idNode != nil {
				names = append(names, idNode.Utf8Text(src))
			}
		default:
			if fieldType == "" && isTypeNode(child.Kind()) {
				fieldType = child.Utf8Text(src)
			}
		}
	}

	if len(names) == 0 {
		return ""
	}

	var b strings.Builder
	writeModifiers(mods, &b)
	if fieldType != "" {
		b.WriteString(fieldType)
		b.WriteByte(' ')
	}
	b.WriteString(className)
	b.WriteByte('.')
	b.WriteString(strings.Join(names, ", "))

	return b.String()
}

func formatAccessorList(node *tree_sitter.Node, src []byte) string {
	var accessors []string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Kind() == "accessor_declaration" {
			text := child.Utf8Text(src)
			if idx := strings.IndexByte(text, '{'); idx >= 0 {
				text = strings.TrimSpace(text[:idx])
			}
			if idx := strings.Index(text, "=>"); idx >= 0 {
				text = strings.TrimSpace(text[:idx])
			}
			text = strings.TrimSpace(text)
			text = strings.TrimRight(text, ";")
			if text != "" {
				accessors = append(accessors, text+";")
			}
		}
	}
	if len(accessors) == 0 {
		return "{ ... }"
	}
	return "{ " + strings.Join(accessors, " ") + " }"
}

// --- helpers ---

func collectModifiers(node *tree_sitter.Node, src []byte) []string {
	var mods []string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "modifier" {
			mods = append(mods, child.Utf8Text(src))
		}
	}
	return mods
}

func hasPublicModifier(mods []string) bool {
	for _, m := range mods {
		if m == "public" {
			return true
		}
	}
	return false
}

func isConstOrStaticReadonly(mods []string) bool {
	for _, m := range mods {
		if m == "const" {
			return true
		}
	}
	hasStatic, hasReadonly := false, false
	for _, m := range mods {
		if m == "static" {
			hasStatic = true
		}
		if m == "readonly" {
			hasReadonly = true
		}
	}
	return hasStatic && hasReadonly
}

func writeModifiers(mods []string, b *strings.Builder) {
	if len(mods) == 0 {
		return
	}
	b.WriteString(strings.Join(mods, " "))
	b.WriteByte(' ')
}

func writeAttributes(node *tree_sitter.Node, src []byte, b *strings.Builder) {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "attribute_list" {
			b.WriteString(child.Utf8Text(src))
			b.WriteByte(' ')
		}
	}
}

func writeTypeParams(node *tree_sitter.Node, src []byte, b *strings.Builder) {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "type_parameter_list" {
			b.WriteString(child.Utf8Text(src))
			break
		}
	}
}

func writeParamList(node *tree_sitter.Node, src []byte, b *strings.Builder) {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "parameter_list" {
			b.WriteString(child.Utf8Text(src))
			break
		}
	}
}

func writeBaseList(node *tree_sitter.Node, src []byte, b *strings.Builder) {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "base_list" {
			b.WriteByte(' ')
			b.WriteString(child.Utf8Text(src))
			break
		}
	}
}

func writeConstraints(node *tree_sitter.Node, src []byte, b *strings.Builder) {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "type_parameter_constraints_clause" {
			b.WriteByte(' ')
			b.WriteString(child.Utf8Text(src))
		}
	}
}

func detectRecordKind(node *tree_sitter.Node, src []byte) string {
	text := node.Utf8Text(src)
	if strings.HasPrefix(text, "record struct") ||
		containsModifiedPrefix(node, src, "record struct") {
		return "record struct"
	}
	if strings.HasPrefix(text, "record class") ||
		containsModifiedPrefix(node, src, "record class") {
		return "record class"
	}
	return "record"
}

func containsModifiedPrefix(node *tree_sitter.Node, src []byte, prefix string) bool {
	start := node.StartByte()
	end := node.EndByte()
	if int(end) > len(src) {
		end = uint(len(src))
	}
	snippet := string(src[start:end])
	// Strip modifiers from the beginning to find the record keyword
	for _, kw := range []string{"public", "internal", "protected", "private", "sealed",
		"abstract", "partial", "static", "new", "unsafe"} {
		snippet = strings.TrimPrefix(snippet, kw+" ")
		snippet = strings.TrimPrefix(snippet, kw+"\n")
		snippet = strings.TrimPrefix(snippet, kw+"\t")
	}
	snippet = strings.TrimLeft(snippet, " \t\n\r")
	return strings.HasPrefix(snippet, prefix)
}

func qualifyName(outerName, name string) string {
	if outerName != "" {
		return outerName + "." + name
	}
	return name
}

func isTypeNode(kind string) bool {
	switch kind {
	case "predefined_type", "identifier", "generic_name", "qualified_name",
		"nullable_type", "array_type", "tuple_type", "pointer_type",
		"function_pointer_type", "ref_type":
		return true
	}
	return false
}
