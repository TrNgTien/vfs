package bench

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Result struct {
	Tool     string
	Command  string
	Output   string
	Bytes    int
	Lines    int
	Duration time.Duration
	Tokens   int
}

func findGrep() (string, string) {
	if path, err := exec.LookPath("rg"); err == nil {
		return path, "rg"
	}
	if path, err := exec.LookPath("grep"); err == nil {
		return path, "grep"
	}
	return "", ""
}

func findVFS() string {
	if path, err := exec.LookPath("vfs"); err == nil {
		return path
	}
	candidates := []string{"./bin/vfs", "bin/vfs"}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}

// RunGrep executes rg or grep against all supported source files.
// This represents what an LLM agent would see without vfs.
func RunGrep(pattern, dir string) (*Result, error) {
	binPath, binName := findGrep()
	if binPath == "" {
		return nil, fmt.Errorf("neither rg nor grep found in PATH")
	}

	var args []string
	if binName == "rg" {
		args = []string{
			"-i",
			"-g", "*.go", "-g", "*.js", "-g", "*.ts", "-g", "*.tsx", "-g", "*.jsx", "-g", "*.py",
			"-g", "!*_test.go", "-g", "!*.test.*", "-g", "!*.spec.*", "-g", "!*.d.ts", "-g", "!*.min.*",
			"-g", "!test_*.py", "-g", "!*_test.py", "-g", "!conftest.py",
			pattern, dir,
		}
	} else {
		args = []string{
			"-r", "-i", "-n",
			"--include=*.go", "--include=*.js", "--include=*.ts", "--include=*.tsx", "--include=*.jsx",
			"--include=*.py",
			"--exclude=*_test.go", "--exclude=*.test.*", "--exclude=*.spec.*", "--exclude=*.d.ts",
			"--exclude=test_*.py", "--exclude=*_test.py", "--exclude=conftest.py",
			"--exclude-dir=vendor", "--exclude-dir=.git", "--exclude-dir=node_modules",
			"--exclude-dir=testdata", "--exclude-dir=dist", "--exclude-dir=build", "--exclude-dir=.next",
			"--exclude-dir=__pycache__", "--exclude-dir=.venv", "--exclude-dir=venv", "--exclude-dir=.tox",
			pattern, dir,
		}
	}

	cmd := exec.Command(binPath, args...)
	start := time.Now()
	out, err := cmd.Output()
	elapsed := time.Since(start)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return &Result{
				Tool:     binName,
				Command:  formatCmd(binName, args),
				Duration: elapsed,
			}, nil
		}
		return nil, fmt.Errorf("%s failed: %w", binName, err)
	}

	output := string(out)
	lines := strings.Count(output, "\n")
	return &Result{
		Tool:     binName,
		Command:  formatCmd(binName, args),
		Output:   output,
		Bytes:    len(out),
		Lines:    lines,
		Duration: elapsed,
		Tokens:   len(out) / 4,
	}, nil
}

// RunVFS executes the vfs binary for the given pattern and path.
func RunVFS(pattern, dir string) (*Result, error) {
	vfsPath := findVFS()
	if vfsPath == "" {
		return nil, fmt.Errorf("vfs not found -- run 'make build' or 'make install' first")
	}

	args := []string{dir, "-f", pattern, "--no-record"}
	cmd := exec.Command(vfsPath, args...)
	start := time.Now()
	out, err := cmd.Output()
	elapsed := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("vfs failed: %w", err)
	}

	output := string(out)
	lines := strings.Count(output, "\n")
	return &Result{
		Tool:     "vfs",
		Command:  formatCmd("vfs", args),
		Output:   output,
		Bytes:    len(out),
		Lines:    lines,
		Duration: elapsed,
		Tokens:   len(out) / 4,
	}, nil
}

// RunReadFile simulates what an LLM does when it reads all matching source files
// in full -- the worst-case baseline. It walks the directory, reads every supported
// source file, and sums up the total bytes/lines.
func RunReadFile(dir string) (*Result, error) {
	start := time.Now()
	var totalBytes int
	var totalLines int
	var fileCount int

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			switch name {
			case "vendor", "node_modules", ".git", "testdata", "dist", "build", ".next",
				"__pycache__", ".venv", "venv", ".tox":
				return filepath.SkipDir
			}
			return nil
		}

		name := info.Name()
		if !isSupportedFile(name) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		totalBytes += len(data)
		totalLines += strings.Count(string(data), "\n")
		fileCount++
		return nil
	})
	elapsed := time.Since(start)

	if err != nil {
		return nil, err
	}

	return &Result{
		Tool:     fmt.Sprintf("cat (%d files)", fileCount),
		Command:  fmt.Sprintf("find %s -name '*.go' -o -name '*.ts' ... | xargs cat", dir),
		Bytes:    totalBytes,
		Lines:    totalLines,
		Duration: elapsed,
		Tokens:   totalBytes / 4,
	}, nil
}

func isSupportedFile(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, "_test.go") ||
		strings.Contains(lower, ".test.") ||
		strings.Contains(lower, ".spec.") ||
		strings.HasSuffix(lower, ".d.ts") ||
		strings.Contains(lower, ".min.") {
		return false
	}
	if strings.HasSuffix(lower, ".py") {
		return !strings.HasPrefix(lower, "test_") &&
			!strings.HasSuffix(lower, "_test.py") &&
			lower != "conftest.py"
	}
	exts := []string{".go", ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts"}
	ext := filepath.Ext(name)
	for _, e := range exts {
		if ext == e {
			return true
		}
	}
	return false
}

func formatCmd(bin string, args []string) string {
	return bin + " " + strings.Join(args, " ")
}
