package dartparser

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tree_sitter_dart "github.com/UserNobody14/tree-sitter-dart/bindings/go"
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

	src := []byte(`
abstract class BaseService {
  Future<void> initialize();
  void dispose();
}

class UserService extends BaseService with LoggingMixin implements Disposable {
  final String baseUrl;
  UserService(this.baseUrl);
  UserService.fromConfig(Map<String, dynamic> config) : baseUrl = config['baseUrl'] as String;
  factory UserService.create() { return UserService('url'); }
  Future<User> getUser(int id) async { return User(); }
  static UserService get instance => _inst;
  String get apiKey => _key;
  set timeout(int value) => _timeout = value;
  void _privateMethod() {}
}

const String appName = 'MyApp';
final double pi = 3.14;
typedef JsonMap = Map<String, dynamic>;
void main() { }
Future<String> fetchData(String url) async { return ''; }
String get appVersion => '1.0';
mixin LoggingMixin on BaseService { void log(String msg) {} }
extension StringExt on String { String capitalize() { return ''; } }
enum Status { active, inactive }
sealed class AuthState {}
`)

	parser := tree_sitter.NewParser()
	defer parser.Close()
	lang := tree_sitter.NewLanguage(tree_sitter_dart.Language())
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
	src, err := os.ReadFile("testdata/sample.dart")
	if err != nil {
		t.Fatalf("reading test fixture: %v", err)
	}

	sigs, err := ExtractExportedFuncs("testdata/sample.dart", src)
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
		// Top-level constants
		"const String appName",
		"const int maxRetries",

		// Typedefs
		"typedef JsonMap = Map<String, dynamic>",
		"typedef Compare<T> = int Function(T a, T b)",

		// Top-level functions
		"void main()",
		"Future<String> fetchData(String url, {int timeout = 30})",

		// Top-level getter
		"String get appVersion",

		// Abstract class
		"abstract class BaseService { ... }",

		// Class with extends, with, implements
		"class UserService extends BaseService with LoggingMixin implements Disposable { ... }",

		// Constructor
		"UserService.UserService(this._apiKey, {required this.baseUrl})",

		// Named constructor
		"UserService.UserService.fromConfig(Map<String, dynamic> config)",

		// Factory constructor
		"UserService.factory UserService.create()",

		// Methods
		"UserService.Future<User> getUser(int id)",

		// Getter
		"UserService.String get apiKey",

		// Generic class
		"class Repository<T extends Model> { ... }",

		// Sealed class
		"sealed class AuthState { ... }",

		// Mixin with on clause
		"mixin LoggingMixin on BaseService { ... }",

		// Mixin without on clause
		"mixin Serializable { ... }",

		// Extension
		"extension StringExtension on String { ... }",

		// Enum
		"enum Status { ... }",
		"enum Color { ... }",

		// Flutter widget
		"class MyHomePage extends StatefulWidget { ... }",

		// Implements multiple interfaces
		"class ApiClient implements HttpClient, Configurable { ... }",
	}

	for _, want := range mustContain {
		if !strings.Contains(joined, want) {
			t.Errorf("missing expected signature: %q", want)
		}
	}

	mustNotContain := []string{
		"_privateMethod",
		"_MyHomePageState",
		"_counter",
		"_incrementCounter",
		"import ",
		"final String UserService._apiKey",
	}

	for _, bad := range mustNotContain {
		if strings.Contains(joined, bad) {
			t.Errorf("should not contain %q, but found it in output", bad)
		}
	}
}

func TestExtractEmptyFile(t *testing.T) {
	sigs, err := ExtractExportedFuncs("empty.dart", []byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sigs) != 0 {
		t.Errorf("expected 0 sigs for empty file, got %d", len(sigs))
	}
}

func TestExtractPrivateOnly(t *testing.T) {
	src := []byte(`
class _InternalWidget extends StatelessWidget {
  void _build() {}
}

void _helper() {}
const String _secret = 'hidden';
`)
	sigs, err := ExtractExportedFuncs("private.dart", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sigs) != 0 {
		t.Errorf("expected 0 sigs for private-only file, got %d", len(sigs))
	}
}
