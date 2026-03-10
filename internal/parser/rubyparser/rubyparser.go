package rubyparser

import (
	"fmt"
	"strings"
	"sync"
	"unicode"

	"github.com/TrNgTien/vfs/internal/parser/sig"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_ruby "github.com/tree-sitter/tree-sitter-ruby/bindings/go"
)

var (
	rubyLangOnce sync.Once
	rubyLang     *tree_sitter.Language
)

func language() *tree_sitter.Language {
	rubyLangOnce.Do(func() {
		rubyLang = tree_sitter.NewLanguage(tree_sitter_ruby.Language())
	})
	return rubyLang
}

// ExtractExportedFuncs parses a Ruby source file and returns signatures of
// public declarations: modules, classes, methods, singleton methods, and
// constant assignments. In Ruby, names starting with _ are conventionally
// private, and methods after a `private` call are private.
func ExtractExportedFuncs(filePath string, src []byte) ([]sig.Sig, error) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(language()); err != nil {
		return nil, fmt.Errorf("setting Ruby language for %s: %w", filePath, err)
	}

	tree := parser.Parse(src, nil)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse %s", filePath)
	}
	defer tree.Close()

	root := tree.RootNode()
	var sigs []sig.Sig

	extractBody(root, src, "", false, &sigs)

	return sigs, nil
}

// extractBody walks children of a body node (program or body_statement)
// and extracts public declarations. It tracks visibility modifiers
// (public/private/protected) to skip private methods. When inSingleton
// is true, regular methods are emitted as `def self.name` (class << self).
func extractBody(node *tree_sitter.Node, src []byte, outerName string, inSingleton bool, sigs *[]sig.Sig) {
	private := false

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "identifier":
			text := child.Utf8Text(src)
			switch text {
			case "private", "protected":
				private = true
			case "public":
				private = false
			}

		case "call":
			if isVisibilityCall(child, src) {
				vis := childText(child, src, "method")
				switch vis {
				case "private", "protected":
					private = true
				case "public":
					private = false
				}
			}

		case "class":
			extractClass(child, src, outerName, sigs)

		case "module":
			extractModule(child, src, outerName, sigs)

		case "method":
			if !private {
				if inSingleton {
					extractMethodAsSingleton(child, src, outerName, sigs)
				} else {
					extractMethod(child, src, outerName, sigs)
				}
			}

		case "singleton_method":
			if !private {
				extractSingletonMethod(child, src, outerName, sigs)
			}

		case "singleton_class":
			extractSingletonClass(child, src, outerName, sigs)

		case "assignment":
			if !private {
				extractConstant(child, src, outerName, sigs)
			}
		}
	}
}

func extractClass(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nameNode.Utf8Text(src)
	if isPrivateName(name) {
		return
	}

	var s strings.Builder
	qualName := qualifiedName(outerName, name)
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	s.WriteString("class ")
	s.WriteString(name)

	if sup := node.ChildByFieldName("superclass"); sup != nil {
		// superclass node contains: `<` token + the parent class name node
		for j := uint(0); j < sup.ChildCount(); j++ {
			ch := sup.Child(j)
			if ch != nil && ch.Kind() != "<" {
				s.WriteString(" < ")
				s.WriteString(ch.Utf8Text(src))
				break
			}
		}
	}
	s.WriteString(" { ... }")

	*sigs = append(*sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})

	if body := node.ChildByFieldName("body"); body != nil {
		extractBody(body, src, qualName, false, sigs)
	}
}

func extractModule(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nameNode.Utf8Text(src)
	if isPrivateName(name) {
		return
	}

	qualName := qualifiedName(outerName, name)

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	s.WriteString("module ")
	s.WriteString(name)
	s.WriteString(" { ... }")

	*sigs = append(*sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})

	if body := node.ChildByFieldName("body"); body != nil {
		extractBody(body, src, qualName, false, sigs)
	}
}

func extractMethod(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nameNode.Utf8Text(src)
	if isPrivateName(name) {
		return
	}

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	s.WriteString("def ")
	s.WriteString(name)

	if params := node.ChildByFieldName("parameters"); params != nil {
		s.WriteString(params.Utf8Text(src))
	}

	*sigs = append(*sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})
}

func extractSingletonMethod(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nameNode.Utf8Text(src)
	if isPrivateName(name) {
		return
	}

	objNode := node.ChildByFieldName("object")
	obj := "self"
	if objNode != nil {
		obj = objNode.Utf8Text(src)
	}

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	s.WriteString("def ")
	s.WriteString(obj)
	s.WriteByte('.')
	s.WriteString(name)

	if params := node.ChildByFieldName("parameters"); params != nil {
		s.WriteString(params.Utf8Text(src))
	}

	*sigs = append(*sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})
}

// extractMethodAsSingleton emits a method inside `class << self` as `def self.name`.
func extractMethodAsSingleton(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nameNode.Utf8Text(src)
	if isPrivateName(name) {
		return
	}

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	s.WriteString("def self.")
	s.WriteString(name)

	if params := node.ChildByFieldName("parameters"); params != nil {
		s.WriteString(params.Utf8Text(src))
	}

	*sigs = append(*sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})
}

func extractSingletonClass(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	if body := node.ChildByFieldName("body"); body != nil {
		extractBody(body, src, outerName, true, sigs)
	}
}

func extractConstant(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	if node.ChildCount() < 3 {
		return
	}

	lhs := node.Child(0)
	if lhs == nil {
		return
	}

	name := lhs.Utf8Text(src)
	if !isConstantName(name) {
		return
	}

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	s.WriteString(name)
	s.WriteString(" = ...")

	*sigs = append(*sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})
}

// isVisibilityCall checks if a call node is a bare `private`, `protected`,
// or `public` call with no arguments (visibility modifier toggle).
func isVisibilityCall(node *tree_sitter.Node, src []byte) bool {
	methodNode := findChildByKind(node, "identifier")
	if methodNode == nil {
		return false
	}
	name := methodNode.Utf8Text(src)
	if name != "private" && name != "protected" && name != "public" {
		return false
	}
	// Bare call (no arguments) toggles visibility for subsequent methods.
	// If there are argument_list children with content, it's a per-method call.
	args := node.ChildByFieldName("arguments")
	if args != nil && args.ChildCount() > 0 {
		return false
	}
	return true
}

func isPrivateName(name string) bool {
	return strings.HasPrefix(name, "_")
}

// isConstantName returns true if the name starts with an uppercase letter
// (Ruby constant convention).
func isConstantName(name string) bool {
	if name == "" {
		return false
	}
	return unicode.IsUpper(rune(name[0]))
}

func qualifiedName(outer, name string) string {
	if outer == "" {
		return name
	}
	return outer + "." + name
}

func childText(node *tree_sitter.Node, src []byte, fieldName string) string {
	child := node.ChildByFieldName(fieldName)
	if child == nil {
		return ""
	}
	return child.Utf8Text(src)
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
