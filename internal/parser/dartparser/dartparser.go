package dartparser

import (
	"fmt"
	"strings"

	"github.com/TrNgTien/vfs/internal/parser/sig"
	tree_sitter_dart "github.com/UserNobody14/tree-sitter-dart/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ExtractExportedFuncs parses a Dart source file and returns signatures of
// public declarations: classes, mixins, enums, extensions, typedefs, and
// top-level functions/variables. In Dart, names starting with _ are private.
func ExtractExportedFuncs(filePath string, src []byte) ([]sig.Sig, error) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	lang := tree_sitter.NewLanguage(tree_sitter_dart.Language())
	if err := parser.SetLanguage(lang); err != nil {
		return nil, fmt.Errorf("setting Dart language for %s: %w", filePath, err)
	}

	tree := parser.Parse(src, nil)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse %s", filePath)
	}
	defer tree.Close()

	return extractFromRoot(tree.RootNode(), src), nil
}

// extractFromRoot iterates top-level children of the program node.
// Dart top-level const/final declarations are split across sibling nodes
// (e.g., const_builtin + type_identifier + static_final_declaration_list),
// so we need stateful iteration rather than per-child dispatch.
func extractFromRoot(root *tree_sitter.Node, src []byte) []sig.Sig {
	var sigs []sig.Sig
	count := root.ChildCount()

	for i := uint(0); i < count; i++ {
		child := root.Child(i)
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "class_definition":
			sigs = append(sigs, extractClass(child, src)...)
		case "enum_declaration":
			sigs = append(sigs, extractEnum(child, src)...)
		case "mixin_declaration":
			sigs = append(sigs, extractMixin(child, src)...)
		case "extension_declaration":
			sigs = append(sigs, extractExtension(child, src)...)
		case "extension_type_declaration":
			sigs = append(sigs, extractExtensionType(child, src)...)
		case "type_alias":
			sigs = append(sigs, extractTypeAlias(child, src)...)
		case "function_signature":
			sigs = append(sigs, extractTopLevelFunc(child, src)...)
		case "getter_signature":
			sigs = append(sigs, extractTopLevelGetter(child, src)...)
		case "setter_signature":
			sigs = append(sigs, extractTopLevelSetter(child, src)...)

		case "const_builtin", "final_builtin":
			// const/final TYPE NAME = VALUE; are split into sibling nodes.
			// Collect keyword, optional type, and declaration list.
			keyword := child.Utf8Text(src)
			line := int(child.StartPosition().Row) + 1
			typeName := ""
			if next := peekChild(root, i+1); next != nil && isTypeLikeNode(next.Kind()) {
				typeName = next.Utf8Text(src)
				i++
			}
			if next := peekChild(root, i+1); next != nil && next.Kind() == "static_final_declaration_list" {
				sigs = append(sigs, buildConstSig(next, src, keyword, typeName, "", line)...)
				i++
			}

		case "static_final_declaration_list":
			sigs = append(sigs, buildConstSig(child, src, "", "", "", int(child.StartPosition().Row)+1)...)
		}
	}

	return sigs
}

// --- class ---

func extractClass(node *tree_sitter.Node, src []byte) []sig.Sig {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Utf8Text(src)
	if isPrivate(name) {
		return nil
	}

	var s strings.Builder
	writeClassModifiers(node, &s)
	s.WriteString("class ")
	s.WriteString(name)
	writeChildByKind(node, src, "type_parameters", &s, "")
	writeSuperclass(node, src, &s)
	writeChildByKind(node, src, "interfaces", &s, " ")
	s.WriteString(" { ... }")

	var sigs []sig.Sig
	sigs = append(sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})

	if body := node.ChildByFieldName("body"); body != nil {
		sigs = append(sigs, extractClassMembers(body, src, name)...)
	}

	return sigs
}

// --- enum ---

func extractEnum(node *tree_sitter.Node, src []byte) []sig.Sig {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Utf8Text(src)
	if isPrivate(name) {
		return nil
	}

	var s strings.Builder
	s.WriteString("enum ")
	s.WriteString(name)
	writeChildByKind(node, src, "type_parameters", &s, "")
	writeChildByKind(node, src, "mixins", &s, " ")
	writeChildByKind(node, src, "interfaces", &s, " ")
	s.WriteString(" { ... }")

	return []sig.Sig{{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	}}
}

// --- mixin ---

func extractMixin(node *tree_sitter.Node, src []byte) []sig.Sig {
	name := findFirstChild(node, src, "identifier")
	if name == "" || isPrivate(name) {
		return nil
	}

	var s strings.Builder
	s.WriteString("mixin ")
	s.WriteString(name)
	writeChildByKind(node, src, "type_parameters", &s, "")

	if onType := findFirstChild(node, src, "type_identifier"); onType != "" {
		s.WriteString(" on ")
		s.WriteString(onType)
	}
	writeChildByKind(node, src, "interfaces", &s, " ")
	s.WriteString(" { ... }")

	var sigs []sig.Sig
	sigs = append(sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "class_body" {
			sigs = append(sigs, extractClassMembers(child, src, name)...)
		}
	}

	return sigs
}

// --- extension ---

func extractExtension(node *tree_sitter.Node, src []byte) []sig.Sig {
	name := findFirstChild(node, src, "identifier")
	if isPrivate(name) {
		return nil
	}

	onType := findFirstChild(node, src, "type_identifier")

	var s strings.Builder
	s.WriteString("extension ")
	if name != "" {
		s.WriteString(name)
		s.WriteByte(' ')
	}
	if onType != "" {
		s.WriteString("on ")
		s.WriteString(onType)
		s.WriteByte(' ')
	}
	s.WriteString("{ ... }")

	return []sig.Sig{{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	}}
}

// --- extension type ---

func extractExtensionType(node *tree_sitter.Node, src []byte) []sig.Sig {
	name := findFirstChild(node, src, "identifier")
	if name == "" || isPrivate(name) {
		return nil
	}

	text := strings.TrimSpace(node.Utf8Text(src))
	if idx := strings.IndexByte(text, '{'); idx >= 0 {
		text = strings.TrimSpace(text[:idx]) + " { ... }"
	}

	return []sig.Sig{{
		Line: int(node.StartPosition().Row) + 1,
		Text: text,
	}}
}

// --- type alias ---

func extractTypeAlias(node *tree_sitter.Node, src []byte) []sig.Sig {
	name := findFirstChild(node, src, "type_identifier")
	if name == "" {
		name = findFirstChild(node, src, "identifier")
	}
	if name == "" || isPrivate(name) {
		return nil
	}

	text := strings.TrimSpace(node.Utf8Text(src))
	text = strings.TrimSuffix(text, ";")
	return []sig.Sig{{
		Line: int(node.StartPosition().Row) + 1,
		Text: text,
	}}
}

// --- top-level function ---

func extractTopLevelFunc(node *tree_sitter.Node, src []byte) []sig.Sig {
	name := findFuncName(node, src)
	if name == "" || isPrivate(name) {
		return nil
	}
	return []sig.Sig{{
		Line: int(node.StartPosition().Row) + 1,
		Text: strings.TrimSpace(node.Utf8Text(src)),
	}}
}

// --- top-level getter ---

func extractTopLevelGetter(node *tree_sitter.Node, src []byte) []sig.Sig {
	name := findGetterName(node, src)
	if name == "" || isPrivate(name) {
		return nil
	}
	return []sig.Sig{{
		Line: int(node.StartPosition().Row) + 1,
		Text: strings.TrimSpace(node.Utf8Text(src)),
	}}
}

// --- top-level setter ---

func extractTopLevelSetter(node *tree_sitter.Node, src []byte) []sig.Sig {
	name := findSetterName(node, src)
	if name == "" || isPrivate(name) {
		return nil
	}
	return []sig.Sig{{
		Line: int(node.StartPosition().Row) + 1,
		Text: strings.TrimSpace(node.Utf8Text(src)),
	}}
}

// --- class members ---

func extractClassMembers(body *tree_sitter.Node, src []byte, className string) []sig.Sig {
	var sigs []sig.Sig

	for i := uint(0); i < body.ChildCount(); i++ {
		child := body.Child(i)
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "declaration":
			sigs = append(sigs, extractDeclaration(child, src, className)...)
		case "method_signature":
			sigs = append(sigs, extractMethodSignature(child, src, className)...)
		}
	}

	return sigs
}

func extractDeclaration(node *tree_sitter.Node, src []byte, className string) []sig.Sig {
	var sigs []sig.Sig

	// A declaration can contain constructor_signature, function_signature,
	// getter_signature, setter_signature, or field declarations
	// (final_builtin/const_builtin + type + initialized_identifier_list).
	keyword := ""
	typeName := ""

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		line := int(child.StartPosition().Row) + 1

		switch child.Kind() {
		case "constructor_signature":
			if text := formatConstructor(child, src, className); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: text})
			}
		case "function_signature":
			if text := formatMethod(child, src, className, false); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: text})
			}
		case "getter_signature":
			if text := formatGetter(child, src, className, false); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: text})
			}
		case "setter_signature":
			if text := formatSetter(child, src, className, false); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: text})
			}
		case "factory_constructor_signature", "redirecting_factory_constructor_signature":
			if text := formatFactory(child, src, className); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: text})
			}
		case "const_builtin", "final_builtin":
			keyword = child.Utf8Text(src)
		case "static":
			keyword = "static " + keyword
		case "type_identifier", "void_type", "inferred_type", "function_type":
			typeName = child.Utf8Text(src)
		case "initialized_identifier_list":
			if text := formatField(child, src, className, keyword, typeName); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: text})
			}
		case "static_final_declaration_list":
			sigs = append(sigs, buildConstSig(child, src, keyword, typeName, className, line)...)
		}
	}

	return sigs
}

func extractMethodSignature(node *tree_sitter.Node, src []byte, className string) []sig.Sig {
	var sigs []sig.Sig
	isStatic := false

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		line := int(child.StartPosition().Row) + 1

		switch child.Kind() {
		case "static":
			isStatic = true
		case "function_signature":
			if text := formatMethod(child, src, className, isStatic); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: text})
			}
		case "getter_signature":
			if text := formatGetter(child, src, className, isStatic); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: text})
			}
		case "setter_signature":
			if text := formatSetter(child, src, className, isStatic); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: text})
			}
		case "factory_constructor_signature", "redirecting_factory_constructor_signature":
			if text := formatFactory(child, src, className); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: text})
			}
		case "operator_signature":
			if text := formatOperator(child, src, className); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: text})
			}
		}
	}

	return sigs
}

// --- member formatters ---

func formatMethod(node *tree_sitter.Node, src []byte, className string, isStatic bool) string {
	name := findFuncName(node, src)
	if name == "" || isPrivate(name) {
		return ""
	}
	text := strings.TrimSpace(node.Utf8Text(src))
	if isStatic {
		text = "static " + text
	}
	return className + "." + text
}

func formatGetter(node *tree_sitter.Node, src []byte, className string, isStatic bool) string {
	name := findGetterName(node, src)
	if name == "" || isPrivate(name) {
		return ""
	}
	text := strings.TrimSpace(node.Utf8Text(src))
	if isStatic {
		text = "static " + text
	}
	return className + "." + text
}

func formatSetter(node *tree_sitter.Node, src []byte, className string, isStatic bool) string {
	name := findSetterName(node, src)
	if name == "" || isPrivate(name) {
		return ""
	}
	text := strings.TrimSpace(node.Utf8Text(src))
	if isStatic {
		text = "static " + text
	}
	return className + "." + text
}

func formatConstructor(node *tree_sitter.Node, src []byte, className string) string {
	text := strings.TrimSpace(node.Utf8Text(src))
	return className + "." + text
}

func formatFactory(node *tree_sitter.Node, src []byte, className string) string {
	text := strings.TrimSpace(node.Utf8Text(src))
	return className + "." + text
}

func formatOperator(node *tree_sitter.Node, src []byte, className string) string {
	text := strings.TrimSpace(node.Utf8Text(src))
	return className + "." + text
}

func formatField(node *tree_sitter.Node, src []byte, className, keyword, typeName string) string {
	names := collectDeclaredNames(node, src)
	if len(names) == 0 {
		return ""
	}
	for _, n := range names {
		if isPrivate(n) {
			return ""
		}
	}

	var b strings.Builder
	if keyword != "" {
		b.WriteString(keyword)
		b.WriteByte(' ')
	}
	if typeName != "" {
		b.WriteString(typeName)
		b.WriteByte(' ')
	}
	b.WriteString(className)
	b.WriteByte('.')
	b.WriteString(strings.Join(names, ", "))
	return b.String()
}

// --- const/final builder ---

func buildConstSig(node *tree_sitter.Node, src []byte, keyword, typeName, className string, line int) []sig.Sig {
	names := collectStaticFinalNames(node, src)
	if len(names) == 0 {
		return nil
	}
	for _, n := range names {
		if isPrivate(n) {
			return nil
		}
	}

	var b strings.Builder
	if keyword != "" {
		b.WriteString(keyword)
		b.WriteByte(' ')
	}
	if typeName != "" {
		b.WriteString(typeName)
		b.WriteByte(' ')
	}
	if className != "" {
		b.WriteString(className)
		b.WriteByte('.')
	}
	b.WriteString(strings.TrimSpace(node.Utf8Text(src)))

	return []sig.Sig{{Line: line, Text: b.String()}}
}

// --- helpers ---

func isPrivate(name string) bool {
	return strings.HasPrefix(name, "_")
}

func peekChild(node *tree_sitter.Node, idx uint) *tree_sitter.Node {
	if idx >= node.ChildCount() {
		return nil
	}
	return node.Child(idx)
}

func isTypeLikeNode(kind string) bool {
	switch kind {
	case "type_identifier", "void_type", "inferred_type", "function_type",
		"nullable_type", "record_type":
		return true
	}
	return false
}

func findFirstChild(node *tree_sitter.Node, src []byte, kind string) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == kind {
			return child.Utf8Text(src)
		}
	}
	return ""
}

func findFuncName(node *tree_sitter.Node, src []byte) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "identifier" {
			return child.Utf8Text(src)
		}
	}
	return ""
}

func findGetterName(node *tree_sitter.Node, src []byte) string {
	pastGet := false
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Kind() == "get" {
			pastGet = true
			continue
		}
		if pastGet && child.Kind() == "identifier" {
			return child.Utf8Text(src)
		}
	}
	return ""
}

func findSetterName(node *tree_sitter.Node, src []byte) string {
	pastSet := false
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Kind() == "set" {
			pastSet = true
			continue
		}
		if pastSet && child.Kind() == "identifier" {
			return child.Utf8Text(src)
		}
	}
	return ""
}

func collectDeclaredNames(node *tree_sitter.Node, src []byte) []string {
	var names []string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Kind() == "initialized_identifier" {
			for j := uint(0); j < child.ChildCount(); j++ {
				gc := child.Child(j)
				if gc != nil && gc.Kind() == "identifier" {
					names = append(names, gc.Utf8Text(src))
				}
			}
		}
	}
	return names
}

func collectStaticFinalNames(node *tree_sitter.Node, src []byte) []string {
	var names []string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Kind() == "static_final_declaration" {
			for j := uint(0); j < child.ChildCount(); j++ {
				gc := child.Child(j)
				if gc != nil && gc.Kind() == "identifier" {
					names = append(names, gc.Utf8Text(src))
					break
				}
			}
		}
	}
	return names
}

func writeClassModifiers(node *tree_sitter.Node, b *strings.Builder) {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "abstract", "sealed", "base", "final", "interface", "mixin":
			b.WriteString(child.Kind())
			b.WriteByte(' ')
		}
	}
}

func writeSuperclass(node *tree_sitter.Node, src []byte, b *strings.Builder) {
	sc := node.ChildByFieldName("superclass")
	if sc == nil {
		return
	}
	b.WriteByte(' ')
	b.WriteString(sc.Utf8Text(src))
}

func writeChildByKind(node *tree_sitter.Node, src []byte, kind string, b *strings.Builder, prefix string) {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == kind {
			b.WriteString(prefix)
			b.WriteString(child.Utf8Text(src))
			return
		}
	}
}
