package swiftparser

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tree_sitter_swift "github.com/qiyue01/tree-sitter-swift/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func dumpAST(node *tree_sitter.Node, src []byte, indent int) {
	prefix := strings.Repeat("  ", indent)
	text := node.Utf8Text(src)
	if len(text) > 80 {
		text = text[:80] + "..."
	}
	text = strings.ReplaceAll(text, "\n", "\\n")
	fmt.Printf("%s%s [%d:%d] %q\n", prefix, node.Kind(), node.StartPosition().Row, node.StartPosition().Column, text)
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			dumpAST(child, src, indent+1)
		}
	}
}

func TestDumpAST(t *testing.T) {
	if os.Getenv("DUMP_AST") == "" {
		t.Skip("set DUMP_AST=1 to run")
	}

	src, err := os.ReadFile("testdata/sample.swift")
	if err != nil {
		t.Fatalf("reading test fixture: %v", err)
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()
	lang := tree_sitter.NewLanguage(tree_sitter_swift.Language())
	_ = parser.SetLanguage(lang)
	tree := parser.Parse(src, nil)
	defer tree.Close()

	root := tree.RootNode()
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child != nil {
			dumpAST(child, src, 0)
			fmt.Println("---")
		}
	}
}

func TestExtractExportedFuncs(t *testing.T) {
	src, err := os.ReadFile("testdata/sample.swift")
	if err != nil {
		t.Fatalf("reading test fixture: %v", err)
	}

	sigs, err := ExtractExportedFuncs("testdata/sample.swift", src)
	if err != nil {
		t.Fatalf("ExtractExportedFuncs: %v", err)
	}

	var lines []string
	for _, s := range sigs {
		lines = append(lines, s.Text)
	}
	joined := strings.Join(lines, "\n")

	t.Logf("Extracted signatures:\n%s", joined)

	mustContain := []string{
		// Top-level functions
		"func topLevelFunc(x: Int) -> String",
		"func fetchData(from url: URL)",

		// Top-level properties
		"var globalVar: Int",
		"let globalLet: String",

		// Type aliases
		"typealias Completion = (Result<Data, Error>) -> Void",
		"typealias StringDict = [String: Any]",

		// Class
		"class UserService: BaseService { ... }",
		"UserService.var name: String",
		"UserService.let id: Int",
		"UserService.func getUser(id: Int) -> User",
		"UserService.static func create() -> UserService",
		"UserService.init(name: String)",

		// Struct
		"struct User: Codable, Hashable { ... }",
		"User.let name: String",
		"User.func displayName() -> String",

		// Protocol
		"protocol DataSource { ... }",
		"DataSource.associatedtype Item",
		"DataSource.func fetchAll() -> [Item]",
		"DataSource.var count: Int",

		// Protocol with inheritance
		"protocol Configurable: AnyObject { ... }",

		// Enum
		"enum APIError: Error { ... }",

		// Extension
		"extension String { ... }",
		"String.func trimmed() -> String",
		"String.var isBlank: Bool",

		// Actor
		"actor DataStore { ... }",
		"DataStore.var items: [String]",
		"DataStore.func add(item: String)",

		// Generic class
		"class Repository<T: Codable> { ... }",
		"Repository.func findById(id: String) -> T?",

		// Nested types
		"class Outer { ... }",
		"Outer.struct Inner { ... }",
		"Outer.Inner.let value: Int",
		"Outer.enum Direction { ... }",

		// Open class
		"open class BaseComponent { ... }",
		"BaseComponent.open func render() -> String",
	}

	for _, want := range mustContain {
		if !strings.Contains(joined, want) {
			t.Errorf("missing expected signature: %q", want)
		}
	}

	mustNotContain := []string{
		"privateHelper",
		"InternalOnly",
		"_secret",
		"internalMethod",
		"import ",
	}

	for _, bad := range mustNotContain {
		if strings.Contains(joined, bad) {
			t.Errorf("should not contain %q, but found it in output", bad)
		}
	}
}

func TestExtractEmptyFile(t *testing.T) {
	sigs, err := ExtractExportedFuncs("empty.swift", []byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sigs) != 0 {
		t.Errorf("expected 0 sigs for empty file, got %d", len(sigs))
	}
}

func TestExtractPrivateOnly(t *testing.T) {
	src := []byte(`
private func helper() {}
fileprivate class Secret {}
private var hidden: Int = 0
`)
	sigs, err := ExtractExportedFuncs("private.swift", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sigs) != 0 {
		t.Errorf("expected 0 sigs for private-only file, got %d", len(sigs))
	}
}
