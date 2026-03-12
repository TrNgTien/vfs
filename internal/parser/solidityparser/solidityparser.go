package solidityparser

import (
	"fmt"
	"strings"
	"sync"

	"github.com/TrNgTien/vfs/internal/parser/sig"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

var (
	solLangOnce sync.Once
	solLang     *tree_sitter.Language
)

func language() *tree_sitter.Language {
	solLangOnce.Do(func() {
		solLang = tree_sitter.NewLanguage(Language())
	})
	return solLang
}

// ExtractExportedFuncs parses a Solidity source file and returns signatures
// of non-private declarations: contracts, interfaces, libraries, functions,
// events, modifiers, structs, enums, state variables, errors, constructors,
// receive/fallback functions, user-defined types, and constants.
func ExtractExportedFuncs(filePath string, src []byte) ([]sig.Sig, error) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(language()); err != nil {
		return nil, fmt.Errorf("setting Solidity language for %s: %w", filePath, err)
	}

	tree := parser.Parse(src, nil)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse %s", filePath)
	}
	defer tree.Close()

	root := tree.RootNode()
	var sigs []sig.Sig

	walkSourceFile(root, src, &sigs)

	return sigs, nil
}

func walkSourceFile(node *tree_sitter.Node, src []byte, sigs *[]sig.Sig) {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "contract_declaration":
			extractContract(child, src, "", "contract", sigs)
		case "interface_declaration":
			extractContract(child, src, "", "interface", sigs)
		case "library_declaration":
			extractContract(child, src, "", "library", sigs)
		case "function_definition":
			extractFunction(child, src, "", sigs)
		case "struct_declaration":
			extractStruct(child, src, "", sigs)
		case "enum_declaration":
			extractEnum(child, src, "", sigs)
		case "event_definition":
			extractEvent(child, src, "", sigs)
		case "error_declaration":
			extractError(child, src, "", sigs)
		case "constant_variable_declaration":
			extractConstant(child, src, "", sigs)
		case "user_defined_type_definition":
			extractUserDefinedType(child, src, "", sigs)
		}
	}
}

func extractContract(node *tree_sitter.Node, src []byte, outerName, keyword string, sigs *[]sig.Sig) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nameNode.Utf8Text(src)
	qualName := qualifiedName(outerName, name)

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	s.WriteString(keyword)
	s.WriteByte(' ')
	s.WriteString(name)

	if inh := collectInheritance(node, src); inh != "" {
		s.WriteString(" is ")
		s.WriteString(inh)
	}
	s.WriteString(" { ... }")

	*sigs = append(*sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})

	body := node.ChildByFieldName("body")
	if body == nil {
		return
	}
	walkContractBody(body, src, qualName, sigs)
}

func collectInheritance(node *tree_sitter.Node, src []byte) string {
	var parents []string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "inheritance_specifier" {
			ancestor := child.ChildByFieldName("ancestor")
			if ancestor != nil {
				parents = append(parents, ancestor.Utf8Text(src))
			}
		}
	}
	return strings.Join(parents, ", ")
}

func walkContractBody(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "function_definition":
			extractFunction(child, src, outerName, sigs)
		case "constructor_definition":
			extractConstructor(child, src, outerName, sigs)
		case "modifier_definition":
			extractModifier(child, src, outerName, sigs)
		case "event_definition":
			extractEvent(child, src, outerName, sigs)
		case "struct_declaration":
			extractStruct(child, src, outerName, sigs)
		case "enum_declaration":
			extractEnum(child, src, outerName, sigs)
		case "state_variable_declaration":
			extractStateVariable(child, src, outerName, sigs)
		case "error_declaration":
			extractError(child, src, outerName, sigs)
		case "fallback_receive_definition":
			extractFallbackReceive(child, src, outerName, sigs)
		case "user_defined_type_definition":
			extractUserDefinedType(child, src, outerName, sigs)
		}
	}
}

func extractFunction(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}

	vis := getVisibility(node, src)
	if vis == "private" {
		return
	}

	name := nameNode.Utf8Text(src)

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	s.WriteString("function ")
	s.WriteString(name)
	s.WriteString(collectParams(node, src))

	if vis != "" {
		s.WriteByte(' ')
		s.WriteString(vis)
	}
	if mut := getStateMutability(node, src); mut != "" {
		s.WriteByte(' ')
		s.WriteString(mut)
	}
	if isVirtual(node) {
		s.WriteString(" virtual")
	}
	if ov := getOverride(node, src); ov != "" {
		s.WriteByte(' ')
		s.WriteString(ov)
	}
	for _, mod := range getModifierInvocations(node, src) {
		s.WriteByte(' ')
		s.WriteString(mod)
	}
	if ret := getReturnType(node, src); ret != "" {
		s.WriteByte(' ')
		s.WriteString(ret)
	}

	*sigs = append(*sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})
}

func extractConstructor(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	s.WriteString("constructor")
	s.WriteString(collectParams(node, src))

	if vis := getVisibility(node, src); vis != "" {
		s.WriteByte(' ')
		s.WriteString(vis)
	}
	if mut := getStateMutability(node, src); mut != "" {
		s.WriteByte(' ')
		s.WriteString(mut)
	}
	for _, mod := range getModifierInvocations(node, src) {
		s.WriteByte(' ')
		s.WriteString(mod)
	}

	*sigs = append(*sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})
}

func extractModifier(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nameNode.Utf8Text(src)

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	s.WriteString("modifier ")
	s.WriteString(name)
	s.WriteString(collectParams(node, src))

	*sigs = append(*sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})
}

func extractEvent(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nameNode.Utf8Text(src)

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	s.WriteString("event ")
	s.WriteString(name)
	s.WriteString(collectEventParams(node, src))

	*sigs = append(*sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})
}

func extractStruct(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nameNode.Utf8Text(src)

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	s.WriteString("struct ")
	s.WriteString(name)
	s.WriteString(" { ... }")

	*sigs = append(*sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})
}

func extractEnum(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nameNode.Utf8Text(src)

	values := collectEnumValues(node, src)

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	s.WriteString("enum ")
	s.WriteString(name)
	if values != "" {
		s.WriteString(" { ")
		s.WriteString(values)
		s.WriteString(" }")
	} else {
		s.WriteString(" { ... }")
	}

	*sigs = append(*sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})
}

func extractStateVariable(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	vis := getVisibility(node, src)
	if vis == "private" {
		return
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nameNode.Utf8Text(src)

	typeName := getTypeName(node, src)

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	if typeName != "" {
		s.WriteString(typeName)
		s.WriteByte(' ')
	}
	if vis != "" {
		s.WriteString(vis)
		s.WriteByte(' ')
	}
	s.WriteString(name)

	*sigs = append(*sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})
}

func extractError(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nameNode.Utf8Text(src)

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	s.WriteString("error ")
	s.WriteString(name)
	s.WriteString(collectErrorParams(node, src))

	*sigs = append(*sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})
}

func extractFallbackReceive(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	kind := ""
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		text := child.Utf8Text(src)
		if text == "receive" || text == "fallback" {
			kind = text
			break
		}
	}
	if kind == "" {
		return
	}

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	s.WriteString(kind)
	s.WriteString("()")

	if vis := getVisibility(node, src); vis != "" {
		s.WriteByte(' ')
		s.WriteString(vis)
	}
	if mut := getStateMutability(node, src); mut != "" {
		s.WriteByte(' ')
		s.WriteString(mut)
	}

	*sigs = append(*sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})
}

func extractConstant(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nameNode.Utf8Text(src)
	typeName := getTypeName(node, src)

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	if typeName != "" {
		s.WriteString(typeName)
		s.WriteString(" constant ")
	} else {
		s.WriteString("constant ")
	}
	s.WriteString(name)

	*sigs = append(*sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})
}

func extractUserDefinedType(node *tree_sitter.Node, src []byte, outerName string, sigs *[]sig.Sig) {
	text := strings.TrimSpace(node.Utf8Text(src))
	text = strings.TrimSuffix(text, ";")
	text = strings.TrimSpace(text)

	var s strings.Builder
	if outerName != "" {
		s.WriteString(outerName)
		s.WriteByte('.')
	}
	s.WriteString(text)

	*sigs = append(*sigs, sig.Sig{
		Line: int(node.StartPosition().Row) + 1,
		Text: s.String(),
	})
}

// --- helpers ---

func getVisibility(node *tree_sitter.Node, src []byte) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "visibility" {
			return child.Utf8Text(src)
		}
	}
	return ""
}

func getStateMutability(node *tree_sitter.Node, src []byte) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "state_mutability" {
			return child.Utf8Text(src)
		}
	}
	return ""
}

func isVirtual(node *tree_sitter.Node) bool {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "virtual" {
			return true
		}
	}
	return false
}

func getOverride(node *tree_sitter.Node, src []byte) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "override_specifier" {
			return child.Utf8Text(src)
		}
	}
	return ""
}

func getModifierInvocations(node *tree_sitter.Node, src []byte) []string {
	var mods []string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "modifier_invocation" {
			mods = append(mods, child.Utf8Text(src))
		}
	}
	return mods
}

func getReturnType(node *tree_sitter.Node, src []byte) string {
	ret := node.ChildByFieldName("return_type")
	if ret == nil {
		return ""
	}
	return ret.Utf8Text(src)
}

func getTypeName(node *tree_sitter.Node, src []byte) string {
	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		return typeNode.Utf8Text(src)
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		k := child.Kind()
		if k == "type_name" || k == "primitive_type" || k == "user_defined_type" ||
			k == "mapping" || k == "array_type" {
			return child.Utf8Text(src)
		}
	}
	return ""
}

func collectParams(node *tree_sitter.Node, src []byte) string {
	var params []string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "parameter" {
			params = append(params, child.Utf8Text(src))
		}
	}
	return "(" + strings.Join(params, ", ") + ")"
}

func collectEventParams(node *tree_sitter.Node, src []byte) string {
	var params []string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "event_parameter" {
			params = append(params, child.Utf8Text(src))
		}
	}
	return "(" + strings.Join(params, ", ") + ")"
}

func collectErrorParams(node *tree_sitter.Node, src []byte) string {
	var params []string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "error_parameter" {
			params = append(params, child.Utf8Text(src))
		}
	}
	return "(" + strings.Join(params, ", ") + ")"
}

func collectEnumValues(node *tree_sitter.Node, src []byte) string {
	body := node.ChildByFieldName("body")
	if body == nil {
		return ""
	}
	var values []string
	for i := uint(0); i < body.ChildCount(); i++ {
		child := body.Child(i)
		if child != nil && child.Kind() == "enum_value" {
			values = append(values, child.Utf8Text(src))
		}
	}
	return strings.Join(values, ", ")
}

func qualifiedName(outer, name string) string {
	if outer == "" {
		return name
	}
	return outer + "." + name
}
