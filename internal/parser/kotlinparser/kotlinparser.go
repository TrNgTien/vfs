package kotlinparser

import (
	"fmt"
	"strings"

	"github.com/TrNgTien/vfs/internal/parser/sig"
	tree_sitter_kotlin "github.com/tree-sitter-grammars/tree-sitter-kotlin/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ExtractExportedFuncs parses a Kotlin source file and returns signatures of
// public declarations: classes, interfaces, objects, enums, functions,
// properties, and type aliases. In Kotlin, top-level declarations without
// a visibility modifier are public by default.
func ExtractExportedFuncs(filePath string, src []byte) ([]sig.Sig, error) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	lang := tree_sitter.NewLanguage(tree_sitter_kotlin.Language())
	if err := parser.SetLanguage(lang); err != nil {
		return nil, fmt.Errorf("setting Kotlin language for %s: %w", filePath, err)
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
		return extractClass(node, src, outerName)
	case "object_declaration":
		return extractObject(node, src, outerName)
	case "function_declaration":
		return extractFunction(node, src, outerName)
	case "property_declaration":
		return extractProperty(node, src, outerName)
	case "type_alias":
		return extractTypeAlias(node, src)
	}
	return nil
}

// --- class / interface / enum ---

func extractClass(node *tree_sitter.Node, src []byte, outerName string) []sig.Sig {
	modifiers := collectModifiers(node, src)
	if isPrivateOrInternal(modifiers) {
		return nil
	}

	var keyword string
	var name string
	var typeParams string
	var delegation string
	var bodyNode *tree_sitter.Node
	var primaryCtor string

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "class", "interface":
			keyword = child.Kind()
		case "identifier":
			name = child.Utf8Text(src)
		case "type_parameters":
			typeParams = child.Utf8Text(src)
		case "delegation_specifiers":
			delegation = child.Utf8Text(src)
		case "class_body", "enum_class_body":
			bodyNode = child
		case "primary_constructor":
			primaryCtor = extractPrimaryConstructor(child, src)
		}
	}

	if name == "" {
		return nil
	}

	var s strings.Builder
	writeClassModifiers(modifiers, &s)
	if keyword == "interface" {
		s.WriteString("interface ")
	} else {
		s.WriteString("class ")
	}
	s.WriteString(name)
	s.WriteString(typeParams)
	if primaryCtor != "" {
		s.WriteString(primaryCtor)
	}
	if delegation != "" {
		s.WriteString(" : ")
		s.WriteString(delegation)
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
		sigs = append(sigs, extractMembers(bodyNode, src, qualifiedName)...)
	}

	return sigs
}

func extractPrimaryConstructor(node *tree_sitter.Node, src []byte) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "class_parameters" {
			return child.Utf8Text(src)
		}
	}
	return ""
}

// --- object ---

func extractObject(node *tree_sitter.Node, src []byte, outerName string) []sig.Sig {
	modifiers := collectModifiers(node, src)
	if isPrivateOrInternal(modifiers) {
		return nil
	}

	name := findChildText(node, src, "identifier")
	if name == "" {
		return nil
	}

	qualifiedName := name
	if outerName != "" {
		qualifiedName = outerName + "." + name
	}

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	s.WriteString("object ")
	s.WriteString(name)

	delegation := findChildText(node, src, "delegation_specifiers")
	if delegation != "" {
		s.WriteString(" : ")
		s.WriteString(delegation)
	}
	s.WriteString(" { ... }")

	var sigs []sig.Sig
	sigs = append(sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})

	if body := findChildByKind(node, "class_body"); body != nil {
		sigs = append(sigs, extractMembers(body, src, qualifiedName)...)
	}

	return sigs
}

// --- function ---

func extractFunction(node *tree_sitter.Node, src []byte, outerName string) []sig.Sig {
	modifiers := collectModifiers(node, src)
	if isPrivateOrInternal(modifiers) {
		return nil
	}

	name := findChildText(node, src, "identifier")
	if name == "" {
		return nil
	}

	sigText := formatFuncSignature(node, src, modifiers)
	if outerName != "" {
		sigText = outerName + "." + sigText
	}

	return []sig.Sig{{
		Line: int(node.StartPosition().Row) + 1,
		Text: sigText,
	}}
}

func formatFuncSignature(node *tree_sitter.Node, src []byte, modifiers []string) string {
	var s strings.Builder

	writeFuncModifiers(modifiers, &s)
	s.WriteString("fun ")

	// Track position: receiver type appears before the identifier,
	// return type appears after ":"
	seenIdentifier := false
	seenColon := false

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "type_parameters":
			s.WriteString(child.Utf8Text(src))
			if !seenIdentifier {
				s.WriteByte(' ')
			}
		case "user_type", "nullable_type", "function_type":
			if !seenIdentifier {
				// Receiver type (extension function)
				s.WriteString(child.Utf8Text(src))
			} else if seenColon {
				// Return type
				s.WriteString(child.Utf8Text(src))
			}
		case ".":
			if !seenIdentifier {
				s.WriteByte('.')
			}
		case "identifier":
			s.WriteString(child.Utf8Text(src))
			seenIdentifier = true
		case "function_value_parameters":
			s.WriteString(child.Utf8Text(src))
		case ":":
			if seenIdentifier {
				s.WriteString(": ")
				seenColon = true
			}
		case "modifiers", "fun", "function_body":
			// skip
		}
	}

	return s.String()
}

// --- property ---

func extractProperty(node *tree_sitter.Node, src []byte, outerName string) []sig.Sig {
	modifiers := collectModifiers(node, src)
	if isPrivateOrInternal(modifiers) {
		return nil
	}

	var keyword string
	var varDecl string
	var line int

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "val", "var":
			keyword = child.Kind()
			line = int(child.StartPosition().Row) + 1
		case "variable_declaration":
			varDecl = child.Utf8Text(src)
		}
	}

	if keyword == "" || varDecl == "" {
		return nil
	}

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	writePropModifiers(modifiers, &s)
	s.WriteString(keyword)
	s.WriteByte(' ')
	s.WriteString(varDecl)

	return []sig.Sig{{
		Line: line,
		Text: s.String(),
	}}
}

// --- type alias ---

func extractTypeAlias(node *tree_sitter.Node, src []byte) []sig.Sig {
	modifiers := collectModifiers(node, src)
	if isPrivateOrInternal(modifiers) {
		return nil
	}

	text := strings.TrimSpace(node.Utf8Text(src))
	return []sig.Sig{{
		Line: int(node.StartPosition().Row) + 1,
		Text: text,
	}}
}

// --- class members ---

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
		case "companion_object":
			sigs = append(sigs, extractCompanionObject(child, src, className)...)
		case "class_declaration":
			sigs = append(sigs, extractClass(child, src, className)...)
		case "object_declaration":
			sigs = append(sigs, extractObject(child, src, className)...)
		}
	}

	return sigs
}

func extractCompanionObject(node *tree_sitter.Node, src []byte, className string) []sig.Sig {
	companionName := className + ".Companion"

	name := findChildText(node, src, "identifier")
	if name != "" {
		companionName = className + "." + name
	}

	var sigs []sig.Sig
	sigs = append(sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: companionName + " { ... }",
	})

	if body := findChildByKind(node, "class_body"); body != nil {
		sigs = append(sigs, extractMembers(body, src, companionName)...)
	}

	return sigs
}

// --- modifier helpers ---

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
				// Unwrap modifier wrapper nodes to get the actual keyword
				if mod.ChildCount() > 0 {
					inner := mod.Child(0)
					if inner != nil {
						mods = append(mods, inner.Kind())
					}
				} else {
					mods = append(mods, mod.Utf8Text(src))
				}
			}
			return mods
		}
	}
	return nil
}

func isPrivateOrInternal(modifiers []string) bool {
	for _, m := range modifiers {
		if m == "private" || m == "internal" {
			return true
		}
	}
	return false
}

func writeClassModifiers(modifiers []string, b *strings.Builder) {
	for _, m := range modifiers {
		switch m {
		case "data", "sealed", "abstract", "enum", "annotation", "open", "inner", "value":
			b.WriteString(m)
			b.WriteByte(' ')
		}
	}
}

func writeFuncModifiers(modifiers []string, b *strings.Builder) {
	for _, m := range modifiers {
		switch m {
		case "suspend", "inline", "infix", "operator", "tailrec", "external", "override", "open", "abstract":
			b.WriteString(m)
			b.WriteByte(' ')
		}
	}
}

func writePropModifiers(modifiers []string, b *strings.Builder) {
	for _, m := range modifiers {
		switch m {
		case "const", "override", "open", "abstract", "lateinit":
			b.WriteString(m)
			b.WriteByte(' ')
		}
	}
}

// --- tree helpers ---

func findChildText(node *tree_sitter.Node, src []byte, kind string) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == kind {
			return child.Utf8Text(src)
		}
	}
	return ""
}

func findChildByKind(node *tree_sitter.Node, kind string) *tree_sitter.Node {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == kind {
			return child
		}
	}
	return nil
}
