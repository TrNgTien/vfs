# vfs

**Virtual Function Signatures** -- extract exported function, class, interface, and type signatures from source code with bodies stripped. Designed to reduce token consumption when AI coding agents explore codebases.

## Supported Languages

| Language   | Extensions                    | Parser        |
|------------|-------------------------------|---------------|
| Go         | `.go`                         | `go/ast`      |
| JavaScript | `.js`, `.mjs`, `.cjs`, `.jsx` | tree-sitter   |
| TypeScript | `.ts`, `.mts`, `.cts`, `.tsx` | tree-sitter   |

## Install

```bash
go install github.com/TrNgTien/vfs/cmd/vfs@latest
```

Or build from source:

```bash
git clone https://github.com/TrNgTien/vfs.git
cd vfs
make install
```

## Usage

```bash
# Scan entire project (Go + JS/TS)
vfs .

# Scan specific directories
vfs ./internal ./pkg ./src

# Filter by pattern (case-insensitive)
vfs . -f HandleLogin

# Single file
vfs handler.go
vfs src/components/App.tsx

# Show token efficiency stats
vfs . --stats

# Show token stats with filter
vfs ./src -f useAuth --stats
```

### Output Format

One signature per line, prefixed with the relative file path:

```
internal/handlers/auth.go: func HandleLogin(c *gin.Context)
internal/handlers/auth.go: func HandleLogout(c *gin.Context)
internal/services/user.go: func NewUserService(repo UserRepo) *UserService
src/components/App.tsx: export function App(props: AppProps)
src/hooks/useAuth.ts: export const useAuth = () => { ... }
```

### Subcommands

#### `vfs stats`

View cumulative token savings across all recorded invocations:

```bash
vfs stats          # show lifetime stats
vfs stats --reset  # clear history
```

Output:

```
--- vfs lifetime stats ---
Invocations:         142
Total tokens saved:  ~384,200
Total raw scanned:   12.4 MB  (38,420 lines)
Total vfs output:    892.0 KB  (4,210 lines)
Avg reduction:       92.8%
First recorded:      2026-03-01 10:15
Last recorded:       2026-03-05 14:30
```

Every invocation is automatically logged to `~/.vfs/history.jsonl`. Use `--no-record` to skip.

#### `vfs bench`

Run a 3-way comparison showing how many tokens each approach sends to an LLM:

| Approach | What it does |
|----------|-------------|
| **Read all files** | `cat` every source file -- worst case baseline |
| **grep/rg** | Text search -- what an LLM agent does with Grep tool |
| **vfs** | Structured signatures only -- bodies stripped |

```bash
# Quick self-test (zero config, runs on vfs's own source):
make bench
# or:
vfs bench --self

# Benchmark on any project:
vfs bench -f HandleLogin /path/to/go-project
vfs bench -f useAuth /path/to/react-app
make bench-on DIR=~/projects/myapp PATTERN=Login

# Show actual output from both tools:
vfs bench -f Login /path/to/project --show-output
```

Example output:

```
Benchmark: pattern="Extract"  path=/Users/you/vfs

What an LLM agent receives for the same query, 3 approaches:

┌─────────────────┬──────────────────┬──────────────────┬──────────────────┐
│                 │ Read all files   │ rg               │ vfs              │
├─────────────────┼──────────────────┼──────────────────┼──────────────────┤
│ Output size     │ 17.0 KB          │ 1.8 KB           │ 512 B            │
│ Lines           │ 794              │ 22               │ 5                │
│ Est. tokens     │ 4352             │ 460              │ 128              │
│ Time            │ 1ms              │ 12ms             │ 45ms             │
└─────────────────┴──────────────────┴──────────────────┴──────────────────┘

Token savings vs reading all files:
  vfs saves 97.1% tokens vs reading all files (4352 -> 128 tokens)
  vfs saves 72.2% tokens vs rg             (460 -> 128 tokens)

Reproduce & verify these numbers yourself:

  # 1. Read all files:
  find . -name '*.go' -o -name '*.ts' ... | xargs cat

  # 2. grep/rg search:
  rg -i -g *.go ... Extract . | wc -c
  rg -i -g *.go ... Extract . | wc -l

  # 3. vfs search:
  vfs . -f Extract --no-record | wc -c
  vfs . -f Extract --no-record | wc -l
```

The benchmark prints exact commands you can copy-paste to independently verify every number.

### Flags

| Flag           | Description                                          |
|----------------|------------------------------------------------------|
| `-f <pattern>` | Case-insensitive substring filter on output lines    |
| `--stats`      | Show token efficiency stats after output             |
| `--no-record`  | Skip logging this invocation to history              |

## How It Works

- **Go**: Parses with `go/ast`, walks `FuncDecl` nodes, nils out `Body`, prints with `go/printer`.
- **JS/TS**: Parses with [tree-sitter](https://github.com/tree-sitter/go-tree-sitter) + language grammars, walks `export_statement` nodes, extracts signatures with bodies stripped.

### What Gets Extracted

**Go**: All exported functions and methods (capitalized names).

**JS/TS**: Exported declarations:
- `export function foo()`
- `export default function foo()`
- `export const foo = () => {}`
- `export class Foo`
- `export interface Foo`
- `export type Foo = ...`
- `export enum Foo`
- `export { foo, bar }`

### Skipped Files/Directories

- `vendor/`, `node_modules/`, `.git/`, `testdata/`, `dist/`, `build/`, `.next/`
- `*_test.go`, `*.test.*`, `*.spec.*`, `*.d.ts`, `*.min.*`

## Project Layout

```
cmd/vfs/            CLI entry point
internal/
  parser/
    types.go        Shared types (Stats, FileResult, ComputeStats)
    walker.go       Language-agnostic directory walker
    goparser/       Go parser (go/ast)
    tsparser/       JS/TS parser (tree-sitter)
  stats/            Performance history tracking (~/.vfs/history.jsonl)
pkg/
  bench/            Side-by-side benchmark (grep/rg vs vfs)
```

## Cursor Integration

A Cursor rule at `.cursor/rules/vfs-go-search.mdc` instructs the AI agent to use `vfs` instead of `grep`/`rg` when searching for function signatures. Copy this rule to other projects or add `vfs` instructions to your workspace `CLAUDE.md`.

## License

MIT
