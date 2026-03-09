package swiftparser

import (
	"fmt"
	"strings"

	"github.com/TrNgTien/vfs/internal/parser/sig"
	tree_sitter_swift "github.com/qiyue01/tree-sitter-swift/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ExtractExportedFuncs parses a Swift source file and returns signatures of
// non-private declarations: classes, structs, enums, protocols, extensions,
// actors, functions, properties, type aliases, and initializers.
// Swift defaults to internal access; we exclude private and fileprivate.
func ExtractExportedFuncs(filePath string, src []byte) ([]sig.Sig, error) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	lang := tree_sitter.NewLanguage(tree_sitter_swift.Language())
	if err := parser.SetLanguage(lang); err != nil {
		return nil, fmt.Errorf("setting Swift language for %s: %w", filePath, err)
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
	case "class_declaration":
		return extractClassLike(node, src, outerName)
	case "protocol_declaration":
		return extractProtocol(node, src, outerName)
	case "function_declaration":
		return extractFunction(node, src, outerName)
	case "property_declaration":
		return extractProperty(node, src, outerName)
	case "typealias_declaration":
		return extractTypealias(node, src, outerName)
	case "init_declaration":
		return extractInit(node, src, outerName)
	case "deinit_declaration":
		return extractDeinit(node, src, outerName)
	case "subscript_declaration":
		return extractSubscript(node, src, outerName)
	}
	return nil
}

// --- class / struct / enum / extension / actor ---

func extractClassLike(node *tree_sitter.Node, src []byte, outerName string) []sig.Sig {
	modifiers := collectModifiers(node)
	if isPrivate(modifiers) {
		return nil
	}

	var keyword string
	var baseName string
	var typeParams string
	var inheritance []string
	var bodyNode *tree_sitter.Node

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "class", "struct", "enum", "extension", "actor":
			keyword = child.Kind()
		case "type_identifier":
			if baseName == "" {
				baseName = child.Utf8Text(src)
			}
		case "user_type":
			if keyword == "extension" && baseName == "" {
				baseName = child.Utf8Text(src)
			}
		case "inheritance_specifier":
			inheritance = append(inheritance, child.Utf8Text(src))
		case "class_body", "enum_class_body":
			bodyNode = child
		case "type_parameters":
			typeParams = child.Utf8Text(src)
		}
	}

	if keyword == "" {
		return nil
	}
	if keyword != "extension" && baseName == "" {
		return nil
	}

	displayName := baseName + typeParams

	var s strings.Builder
	writeModifiers(modifiers, &s)
	s.WriteString(keyword)
	s.WriteByte(' ')
	s.WriteString(displayName)
	if len(inheritance) > 0 {
		s.WriteString(": ")
		s.WriteString(strings.Join(inheritance, ", "))
	}
	s.WriteString(" { ... }")

	qualifiedName := baseName
	if outerName != "" {
		qualifiedName = outerName + "." + baseName
	}

	text := s.String()
	if outerName != "" {
		text = outerName + "." + text
	}

	var sigs []sig.Sig
	sigs = append(sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: text,
	})

	if bodyNode != nil {
		sigs = append(sigs, extractMembers(bodyNode, src, qualifiedName)...)
	}

	return sigs
}

// --- protocol ---

func extractProtocol(node *tree_sitter.Node, src []byte, outerName string) []sig.Sig {
	modifiers := collectModifiers(node)
	if isPrivate(modifiers) {
		return nil
	}

	var name string
	var inheritance []string
	var bodyNode *tree_sitter.Node

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "type_identifier":
			if name == "" {
				name = child.Utf8Text(src)
			}
		case "inheritance_specifier":
			inheritance = append(inheritance, child.Utf8Text(src))
		case "protocol_body":
			bodyNode = child
		case "type_parameters":
			name += child.Utf8Text(src)
		}
	}

	if name == "" {
		return nil
	}

	var s strings.Builder
	writeModifiers(modifiers, &s)
	s.WriteString("protocol ")
	s.WriteString(name)
	if len(inheritance) > 0 {
		s.WriteString(": ")
		s.WriteString(strings.Join(inheritance, ", "))
	}
	s.WriteString(" { ... }")

	qualifiedName := name
	if outerName != "" {
		qualifiedName = outerName + "." + name
	}

	text := s.String()
	if outerName != "" {
		text = outerName + "." + text
	}

	var sigs []sig.Sig
	sigs = append(sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: text,
	})

	if bodyNode != nil {
		sigs = append(sigs, extractProtocolMembers(bodyNode, src, qualifiedName)...)
	}

	return sigs
}

// --- function ---

func extractFunction(node *tree_sitter.Node, src []byte, outerName string) []sig.Sig {
	modifiers := collectModifiers(node)
	if isPrivate(modifiers) {
		return nil
	}

	sigText := formatFuncSignature(node, src, modifiers)
	if sigText == "" {
		return nil
	}

	if outerName != "" {
		sigText = outerName + "." + sigText
	}

	return []sig.Sig{{
		Line: int(node.StartPosition().Row) + 1,
		Text: sigText,
	}}
}

func formatFuncSignature(node *tree_sitter.Node, src []byte, modifiers []string) string {
	var name string
	var params []string
	var returnType string
	var typeParams string

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "simple_identifier":
			if name == "" {
				name = child.Utf8Text(src)
			}
		case "(":
			// start of params
		case ")":
			// end of params
		case "parameter":
			params = append(params, child.Utf8Text(src))
		case "->":
			// return arrow
		case "user_type", "array_type", "dictionary_type", "tuple_type",
			"optional_type", "function_type", "opaque_type", "existential_type":
			returnType = child.Utf8Text(src)
		case "type_parameters":
			typeParams = child.Utf8Text(src)
		}
	}

	if name == "" {
		return ""
	}

	var s strings.Builder
	writeModifiers(modifiers, &s)
	s.WriteString("func ")
	s.WriteString(name)
	s.WriteString(typeParams)
	s.WriteByte('(')
	s.WriteString(strings.Join(params, ", "))
	s.WriteByte(')')
	if returnType != "" {
		s.WriteString(" -> ")
		s.WriteString(returnType)
	}

	return s.String()
}

// --- property ---

func extractProperty(node *tree_sitter.Node, src []byte, outerName string) []sig.Sig {
	modifiers := collectModifiers(node)
	if isPrivate(modifiers) {
		return nil
	}

	var keyword string
	var name string
	var typeStr string

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "value_binding_pattern":
			keyword = extractBindingKeyword(child)
		case "pattern":
			name = findSimpleIdentifier(child, src)
		case "type_annotation":
			typeStr = extractTypeAnnotation(child, src)
		}
	}

	if name == "" {
		return nil
	}

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	writeModifiers(modifiers, &s)
	s.WriteString(keyword)
	s.WriteByte(' ')
	s.WriteString(name)
	if typeStr != "" {
		s.WriteString(": ")
		s.WriteString(typeStr)
	}

	return []sig.Sig{{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	}}
}

// --- typealias ---

func extractTypealias(node *tree_sitter.Node, src []byte, outerName string) []sig.Sig {
	modifiers := collectModifiers(node)
	if isPrivate(modifiers) {
		return nil
	}

	text := strings.TrimSpace(node.Utf8Text(src))
	if outerName != "" {
		text = outerName + "." + text
	}

	return []sig.Sig{{
		Line: int(node.StartPosition().Row) + 1,
		Text: text,
	}}
}

// --- init ---

func extractInit(node *tree_sitter.Node, src []byte, outerName string) []sig.Sig {
	modifiers := collectModifiers(node)
	if isPrivate(modifiers) {
		return nil
	}

	var params []string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "parameter" {
			params = append(params, child.Utf8Text(src))
		}
	}

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	s.WriteString("init(")
	s.WriteString(strings.Join(params, ", "))
	s.WriteByte(')')

	return []sig.Sig{{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	}}
}

// --- deinit ---

func extractDeinit(node *tree_sitter.Node, src []byte, outerName string) []sig.Sig {
	modifiers := collectModifiers(node)
	if isPrivate(modifiers) {
		return nil
	}

	_ = src
	text := "deinit"
	if outerName != "" {
		text = outerName + "." + text
	}

	return []sig.Sig{{
		Line: int(node.StartPosition().Row) + 1,
		Text: text,
	}}
}

// --- subscript ---

func extractSubscript(node *tree_sitter.Node, src []byte, outerName string) []sig.Sig {
	modifiers := collectModifiers(node)
	if isPrivate(modifiers) {
		return nil
	}

	text := strings.TrimSpace(node.Utf8Text(src))
	if idx := strings.IndexByte(text, '{'); idx >= 0 {
		text = strings.TrimSpace(text[:idx])
	}
	if outerName != "" {
		text = outerName + "." + text
	}

	return []sig.Sig{{
		Line: int(node.StartPosition().Row) + 1,
		Text: text,
	}}
}

// --- class/enum/actor members ---

func extractMembers(body *tree_sitter.Node, src []byte, className string) []sig.Sig {
	var sigs []sig.Sig

	for i := uint(0); i < body.ChildCount(); i++ {
		child := body.Child(i)
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "function_declaration":
			sigs = append(sigs, extractFunction(child, src, className)...)
		case "property_declaration":
			sigs = append(sigs, extractProperty(child, src, className)...)
		case "init_declaration":
			sigs = append(sigs, extractInit(child, src, className)...)
		case "deinit_declaration":
			sigs = append(sigs, extractDeinit(child, src, className)...)
		case "subscript_declaration":
			sigs = append(sigs, extractSubscript(child, src, className)...)
		case "class_declaration":
			sigs = append(sigs, extractClassLike(child, src, className)...)
		case "typealias_declaration":
			sigs = append(sigs, extractTypealias(child, src, className)...)
		}
	}

	return sigs
}

// --- protocol members ---

func extractProtocolMembers(body *tree_sitter.Node, src []byte, protoName string) []sig.Sig {
	var sigs []sig.Sig

	for i := uint(0); i < body.ChildCount(); i++ {
		child := body.Child(i)
		if child == nil {
			continue
		}

		line := int(child.StartPosition().Row) + 1

		switch child.Kind() {
		case "protocol_function_declaration":
			if text := formatProtocolFunc(child, src); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: protoName + "." + text})
			}
		case "protocol_property_declaration":
			if text := formatProtocolProp(child, src); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: protoName + "." + text})
			}
		case "associatedtype_declaration":
			if text := formatAssociatedType(child, src); text != "" {
				sigs = append(sigs, sig.Sig{Line: line, Text: protoName + "." + text})
			}
		}
	}

	return sigs
}

func formatProtocolFunc(node *tree_sitter.Node, src []byte) string {
	var name string
	var params []string
	var returnType string

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "simple_identifier":
			if name == "" {
				name = child.Utf8Text(src)
			}
		case "parameter":
			params = append(params, child.Utf8Text(src))
		case "user_type", "array_type", "dictionary_type", "tuple_type",
			"optional_type", "function_type", "opaque_type", "existential_type":
			returnType = child.Utf8Text(src)
		}
	}

	if name == "" {
		return ""
	}

	var s strings.Builder
	s.WriteString("func ")
	s.WriteString(name)
	s.WriteByte('(')
	s.WriteString(strings.Join(params, ", "))
	s.WriteByte(')')
	if returnType != "" {
		s.WriteString(" -> ")
		s.WriteString(returnType)
	}

	return s.String()
}

func formatProtocolProp(node *tree_sitter.Node, src []byte) string {
	var name string
	var typeStr string
	var keyword string

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "pattern":
			keyword = extractBindingKeyword(child)
			name = findSimpleIdentifier(child, src)
		case "type_annotation":
			typeStr = extractTypeAnnotation(child, src)
		}
	}

	if name == "" {
		return ""
	}

	var s strings.Builder
	s.WriteString(keyword)
	s.WriteByte(' ')
	s.WriteString(name)
	if typeStr != "" {
		s.WriteString(": ")
		s.WriteString(typeStr)
	}

	return s.String()
}

func formatAssociatedType(node *tree_sitter.Node, src []byte) string {
	name := ""
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "type_identifier" {
			name = child.Utf8Text(src)
			break
		}
	}
	if name == "" {
		return ""
	}
	return "associatedtype " + name
}

// --- modifier helpers ---

func collectModifiers(node *tree_sitter.Node) []string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "modifiers" {
			var mods []string
			for j := uint(0); j < child.ChildCount(); j++ {
				mod := child.Child(j)
				if mod == nil {
					continue
				}
				switch mod.Kind() {
				case "visibility_modifier":
					if inner := mod.Child(0); inner != nil {
						mods = append(mods, inner.Kind())
					}
				case "property_modifier":
					if inner := mod.Child(0); inner != nil {
						mods = append(mods, inner.Kind())
					}
				case "member_modifier":
					if inner := mod.Child(0); inner != nil {
						mods = append(mods, inner.Kind())
					}
				case "function_modifier":
					if inner := mod.Child(0); inner != nil {
						mods = append(mods, inner.Kind())
					}
				case "mutation_modifier":
					if inner := mod.Child(0); inner != nil {
						mods = append(mods, inner.Kind())
					}
				case "inheritance_modifier":
					if inner := mod.Child(0); inner != nil {
						mods = append(mods, inner.Kind())
					}
				default:
					if inner := mod.Child(0); inner != nil {
						mods = append(mods, inner.Kind())
					}
				}
			}
			return mods
		}
	}
	return nil
}

func isPrivate(modifiers []string) bool {
	for _, m := range modifiers {
		if m == "private" || m == "fileprivate" {
			return true
		}
	}
	return false
}

func writeModifiers(modifiers []string, b *strings.Builder) {
	for _, m := range modifiers {
		switch m {
		case "static", "class", "override", "open", "final", "mutating",
			"nonmutating", "nonisolated", "async":
			b.WriteString(m)
			b.WriteByte(' ')
		}
	}
}

// --- tree helpers ---

func findSimpleIdentifier(node *tree_sitter.Node, src []byte) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "simple_identifier" {
			return child.Utf8Text(src)
		}
	}
	return ""
}

func extractBindingKeyword(node *tree_sitter.Node) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "var", "let":
			return child.Kind()
		case "value_binding_pattern":
			return extractBindingKeyword(child)
		}
	}
	return "var"
}

func extractTypeAnnotation(node *tree_sitter.Node, src []byte) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil || child.Kind() == ":" {
			continue
		}
		if child.Kind() != ":" {
			text := child.Utf8Text(src)
			if text != ":" {
				return text
			}
		}
	}
	return ""
}
