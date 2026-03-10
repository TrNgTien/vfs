package rubyparser

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_ruby "github.com/tree-sitter/tree-sitter-ruby/bindings/go"
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

	src, err := os.ReadFile("testdata/sample.rb")
	if err != nil {
		t.Fatalf("reading test fixture: %v", err)
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()
	lang := tree_sitter.NewLanguage(tree_sitter_ruby.Language())
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
	src, err := os.ReadFile("testdata/sample.rb")
	if err != nil {
		t.Fatalf("reading test fixture: %v", err)
	}

	sigs, err := ExtractExportedFuncs("testdata/sample.rb", src)
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
		"VERSION = ...",
		"MAX_RETRIES = ...",

		// Top-level method
		"def top_level_helper(x, y)",

		// Module
		"module Serializable { ... }",
		"Serializable.FORMATS = ...",
		"Serializable.def serialize",
		"Serializable.def self.included(base)",

		// Nested module
		"Serializable.module ClassMethods { ... }",
		"Serializable.ClassMethods.def from_json(data)",

		// Class (no superclass)
		"class Animal { ... }",
		"Animal.KINGDOM = ...",
		"Animal.def initialize(name, age)",
		"Animal.def speak",
		"Animal.def to_s",
		"Animal.def self.create(name, age)",

		// Singleton class methods (class << self)
		"Animal.def self.species_count",

		// Subclass with inheritance
		"class Dog < Animal { ... }",
		"Dog.DEFAULT_TRICKS = ...",
		"Dog.def speak",
		"Dog.def fetch(item)",
		"Dog.def self.good_boy?(dog)",

		// Nested services
		"module Services { ... }",
		"Services.module Authentication { ... }",
		"Services.Authentication.class TokenService { ... }",
		"Services.Authentication.TokenService.TOKEN_EXPIRY = ...",
		"Services.Authentication.TokenService.def generate(user_id)",
		"Services.Authentication.TokenService.def self.validate(token)",

		"Services.class BaseService { ... }",
		"Services.BaseService.def call",
		"Services.BaseService.def self.call(**args)",

		// Configuration
		"class Configuration { ... }",
		"Configuration.def self.load(path)",
		"Configuration.def self.default",
		"Configuration.def get(key)",
	}

	for _, want := range mustContain {
		if !strings.Contains(joined, want) {
			t.Errorf("missing expected signature: %q", want)
		}
	}

	mustNotContain := []string{
		"_private_helper",
		"internal_id",
		"secret_method",
		"encode",
		"pack_status",
		"some_var",
		"require",
	}

	for _, bad := range mustNotContain {
		if strings.Contains(joined, bad) {
			t.Errorf("should not contain %q, but found it in output", bad)
		}
	}
}

func TestExtractEmptyFile(t *testing.T) {
	sigs, err := ExtractExportedFuncs("empty.rb", []byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sigs) != 0 {
		t.Errorf("expected 0 sigs for empty file, got %d", len(sigs))
	}
}

func TestExtractPrivateOnly(t *testing.T) {
	src := []byte(`
def _hidden_method
  "secret"
end

class _InternalClass
  def _do_stuff
    nil
  end
end
`)
	sigs, err := ExtractExportedFuncs("private.rb", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sigs) != 0 {
		t.Errorf("expected 0 sigs for private-only file, got %d", len(sigs))
	}
}
