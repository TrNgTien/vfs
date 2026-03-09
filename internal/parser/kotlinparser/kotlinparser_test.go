package kotlinparser

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tree_sitter_kotlin "github.com/tree-sitter-grammars/tree-sitter-kotlin/bindings/go"
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

	src, err := os.ReadFile("testdata/sample.kt")
	if err != nil {
		t.Fatalf("reading test fixture: %v", err)
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()
	lang := tree_sitter.NewLanguage(tree_sitter_kotlin.Language())
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
	src, err := os.ReadFile("testdata/sample.kt")
	if err != nil {
		t.Fatalf("reading test fixture: %v", err)
	}

	sigs, err := ExtractExportedFuncs("testdata/sample.kt", src)
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
		"fun topLevelFun(x: Int): String",
		"suspend fun fetchData(url: String, timeout: Int = 30): String",

		// Top-level properties
		"val appName: String",
		"var retryCount: Int",
		"const val MAX_RETRIES: Int",

		// Type aliases
		"typealias StringMap = Map<String, String>",
		"typealias Callback<T> = (T) -> Unit",

		// Data class
		"data class User(val name: String, val email: String) { ... }",

		// Sealed class
		"sealed class Result<T> { ... }",
		"Result.data class Success<T>(val data: T) : Result<T>() { ... }",
		"Result.data class Error(val message: String) : Result<Nothing>() { ... }",

		// Abstract class
		"abstract class BaseService { ... }",
		"BaseService.abstract fun initialize(): Unit",
		"BaseService.open fun configure()",

		// Regular class with inheritance
		"class UserService : BaseService() { ... }",
		"UserService.fun getUser(id: Int): User",
		"UserService.override fun initialize()",

		// Companion object
		"UserService.Companion { ... }",
		"UserService.Companion.fun create(): UserService",
		"UserService.Companion.const val TAG: String",

		// Interface
		"interface Repository<T> { ... }",
		"Repository.fun findById(id: String): T?",
		"Repository.fun save(entity: T): Unit",
		"Repository.val count: Int",

		// Object declaration
		"object NetworkModule { ... }",
		"NetworkModule.fun provideClient(): Any",
		"NetworkModule.val baseUrl: String",

		// Enum class
		"enum class Status { ... }",
		"Status.fun isActive(): Boolean",

		// Annotation class
		"annotation class Inject { ... }",

		// Named companion
		"Config.Factory { ... }",

		// Inline function
		"inline fun <reified T> fromJson(json: String): T",
	}

	for _, want := range mustContain {
		if !strings.Contains(joined, want) {
			t.Errorf("missing expected signature: %q", want)
		}
	}

	mustNotContain := []string{
		"privateHelper",
		"_InternalClass",
		"_secret",
		"internalHelper",
		"InternalOnlyClass",
		"internalSetup",
		"import ",
		"package ",
	}

	for _, bad := range mustNotContain {
		if strings.Contains(joined, bad) {
			t.Errorf("should not contain %q, but found it in output", bad)
		}
	}
}

func TestExtractEmptyFile(t *testing.T) {
	sigs, err := ExtractExportedFuncs("empty.kt", []byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sigs) != 0 {
		t.Errorf("expected 0 sigs for empty file, got %d", len(sigs))
	}
}

func TestExtractPrivateOnly(t *testing.T) {
	src := []byte(`
private fun helper() {}
private class Secret {}
internal fun internalOnly() {}
private val _hidden: String = "secret"
`)
	sigs, err := ExtractExportedFuncs("private.kt", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sigs) != 0 {
		t.Errorf("expected 0 sigs for private-only file, got %d", len(sigs))
	}
}
