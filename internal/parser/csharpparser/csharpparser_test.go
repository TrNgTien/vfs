package csharpparser

import (
	"os"
	"strings"
	"testing"
)

func TestExtractExportedFuncs(t *testing.T) {
	src, err := os.ReadFile("testdata/sample.cs")
	if err != nil {
		t.Fatalf("reading test fixture: %v", err)
	}

	sigs, err := ExtractExportedFuncs("testdata/sample.cs", src)
	if err != nil {
		t.Fatalf("ExtractExportedFuncs: %v", err)
	}

	var lines []string
	for _, s := range sigs {
		lines = append(lines, s.Text)
	}
	joined := strings.Join(lines, "\n")

	mustContain := []string{
		// Class with base list
		"public class UserService : IUserService, IDisposable { ... }",
		// Const field (namespace-qualified)
		"public const string MyApp.Services.UserService.DefaultRole",
		// Static readonly field
		"public static readonly int MyApp.Services.UserService.MaxRetries",
		// Constructor
		"public MyApp.Services.UserService.UserService(string connectionString)",
		// Async method
		"public async Task<User> MyApp.Services.UserService.GetByIdAsync(int id)",
		// Simple method
		"public void MyApp.Services.UserService.Dispose()",
		// Property with accessors
		"public string MyApp.Services.UserService.Name { get; set; }",
		"public int MyApp.Services.UserService.Count { get; private set; }",
		// Interface
		"public interface IUserService { ... }",
		// Interface method
		"public Task<User> MyApp.Services.IUserService.GetByIdAsync(int id)",
		// Struct
		"public struct Point { ... }",
		// Struct properties
		"public double MyApp.Services.Point.X { get; set; }",
		// Struct constructor
		"public MyApp.Services.Point.Point(double x, double y)",
		// Struct method
		"public double MyApp.Services.Point.DistanceTo(Point other)",
		// Enum
		"public enum Status { ... }",
		// Record
		"public record UserRecord(string Name, int Age)",
		// Record class with base list
		"public record class DetailedRecord(string Id) : UserRecord",
		// Delegate
		"public delegate void EventCallback(object sender, EventArgs args)",
		// Generic delegate
		"public delegate Task<TResult> AsyncFunc<TInput, TResult>(TInput input)",
		// Generic class with constraint
		"public abstract class Repository<TEntity> where TEntity : class, new() { ... }",
		// Abstract methods
		"public abstract Task<TEntity> MyApp.Services.Repository.FindAsync(int id)",
		"public abstract Task<IEnumerable<TEntity>> MyApp.Services.Repository.GetAllAsync()",
		// Nested class
		"public class NestedConfig { ... }",
	}

	for _, want := range mustContain {
		if !strings.Contains(joined, want) {
			t.Errorf("missing expected signature: %q\nGot:\n%s", want, joined)
		}
	}

	mustNotContain := []string{
		"InternalOnly",
		"InternalMethod",
		"_secret",
		"_connectionString",
		"ShouldNotAppear",
	}

	for _, bad := range mustNotContain {
		if strings.Contains(joined, bad) {
			t.Errorf("should not contain %q, but found it in:\n%s", bad, joined)
		}
	}
}

func TestExtractFileScopedNamespace(t *testing.T) {
	src := []byte(`namespace MyApp;

public class AppConfig
{
    public string Name { get; set; }
}

public interface IService
{
    public void Execute();
}
`)
	sigs, err := ExtractExportedFuncs("test.cs", src)
	if err != nil {
		t.Fatalf("ExtractExportedFuncs: %v", err)
	}

	var lines []string
	for _, s := range sigs {
		lines = append(lines, s.Text)
	}
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "public class AppConfig") {
		t.Errorf("missing AppConfig class, got:\n%s", joined)
	}
	if !strings.Contains(joined, "public interface IService") {
		t.Errorf("missing IService interface, got:\n%s", joined)
	}
}

func TestExtractEmptyFile(t *testing.T) {
	sigs, err := ExtractExportedFuncs("empty.cs", []byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sigs) != 0 {
		t.Errorf("expected 0 sigs for empty file, got %d", len(sigs))
	}
}

func TestExtractNoPublic(t *testing.T) {
	src := []byte(`namespace Internal
{
    internal class Secret
    {
        internal void DoStuff() { }
    }
    class DefaultAccess
    {
        void Hidden() { }
    }
}
`)
	sigs, err := ExtractExportedFuncs("internal.cs", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sigs) != 0 {
		t.Errorf("expected 0 sigs for internal-only file, got %d", len(sigs))
	}
}
