# vfs

**Virtual Function Signatures** -- extract exported function, class, interface, and type signatures from source code with bodies stripped. Designed to reduce token consumption when AI coding agents explore codebases.

## Supported Languages

| Language   | Extensions                    | Parser        |
|------------|-------------------------------|---------------|
| Go         | `.go`                         | `go/ast`      |
| JavaScript | `.js`, `.mjs`, `.cjs`, `.jsx` | tree-sitter   |
| TypeScript | `.ts`, `.mts`, `.cts`, `.tsx` | tree-sitter   |
| Python     | `.py`                         | tree-sitter   |

## Install

```bash
go install github.com/TrNgTien/vfs/cmd/vfs@latest
```

Or build from source:

```bash
git clone https://github.com/TrNgTien/vfs.git
cd vfs
make install   # installs to $GOPATH/bin
make build     # or just build to ./bin/vfs
```

> Don't have Go installed? See [Docker](#docker) for a container-based alternative.

## Usage

```bash
# Scan entire project
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

# Run MCP server + dashboard together
vfs serve

# Run MCP server only (stdio, for Cursor/Claude Desktop)
vfs mcp

# Run dashboard only
vfs dashboard
```

### Output Format

One signature per line, prefixed with the relative file path:

```
internal/handlers/auth.go: func HandleLogin(c *gin.Context)
internal/handlers/auth.go: func HandleLogout(c *gin.Context)
internal/services/user.go: func NewUserService(repo UserRepo) *UserService
src/components/App.tsx: export function App(props: AppProps)
src/hooks/useAuth.ts: export const useAuth = () => { ... }
app/services/auth.py: class AuthService(BaseService)
app/services/auth.py: def authenticate(self, username: str, password: str) -> bool
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

#### `vfs serve`

Run the MCP server and dashboard together in a single process:

```bash
vfs serve                                    # MCP on :8080, dashboard on :3000
vfs serve --mcp :9090 --dashboard-port 4000  # custom ports
make serve                                   # build + serve
```

### Flags

| Flag           | Description                                          |
|----------------|------------------------------------------------------|
| `-f <pattern>` | Case-insensitive substring filter on output lines    |
| `--stats`      | Show token efficiency stats after output             |
| `--no-record`  | Skip logging this invocation to history              |

## MCP Server

vfs can run as a [Model Context Protocol](https://modelcontextprotocol.io/) server, exposing its signature-extraction capabilities as tools that AI assistants can call directly.

### Quick Start

```bash
# Stdio transport (default) -- for Cursor, Claude Desktop, etc.
vfs mcp

# HTTP transport -- for Docker, remote access, or custom clients
vfs mcp --http :8080
```

### Exposed Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `extract` | Scan paths and return all exported signatures | `paths` (string[], required) |
| `search` | Extract signatures filtered by name pattern | `paths` (string[], required), `pattern` (string, required) |
| `stats` | Return lifetime usage statistics | none |
| `list_languages` | List supported languages and extensions | none |

### Cursor Configuration

Add to `.cursor/mcp.json` in your project or `~/.cursor/mcp.json` globally:

```json
{
  "mcpServers": {
    "vfs": {
      "command": "vfs",
      "args": ["mcp"]
    }
  }
}
```

### Claude Desktop Configuration

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "vfs": {
      "command": "vfs",
      "args": ["mcp"]
    }
  }
}
```

### Docker (HTTP mode)

```json
{
  "mcpServers": {
    "vfs": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

## Docker

Run vfs in a container -- no local Go toolchain required. The image supports two modes:

| Mode | What happens | When to use |
|------|-------------|-------------|
| **Server** (default) | Starts MCP server on `:8080` + dashboard on `:3000` | AI assistant integration, always-on service |
| **CLI** | Runs `vfs` with the arguments you pass | One-off scans, CI pipelines, scripting |

### Build

```bash
docker build -t vfs-mcp .
# or:
make docker-build
```

### Server Mode (default)

```bash
docker run --rm -v $(pwd):/workspace -p 8080:8080 -p 3000:3000 vfs-mcp
```

- MCP endpoint: `http://localhost:8080/mcp`
- Dashboard: `http://localhost:3000`

### CLI Mode

Pass any `vfs` arguments after the image name:

```bash
# Scan a mounted project
docker run --rm -v $(pwd):/workspace vfs-mcp /workspace -f HandleLogin

# Show stats
docker run --rm vfs-mcp stats

# Run with --stats flag
docker run --rm -v $(pwd):/workspace vfs-mcp /workspace --stats
```

### Make Shortcuts

```bash
make docker-run                                    # server mode (MCP + dashboard)
make docker-cli ARGS='/workspace -f HandleLogin'   # CLI mode
```

## Dashboard

A built-in web dashboard for visualizing token savings, reduction trends, and agent activity over time. All data comes from `~/.vfs/history.jsonl` -- the same file that `vfs stats` reads.

### Quick Start

```bash
vfs dashboard                # open on http://localhost:3000
vfs dashboard --port 4000    # custom port
make dashboard               # build + open on :3000
```

### Panels

- **Summary cards**: Total invocations, lifetime tokens saved, average reduction %, number of projects
- **Cumulative Tokens Saved**: Time-series line chart showing total tokens saved growing over time
- **Reduction % Per Invocation**: Scatter chart showing how efficient each vfs call was
- **Agent Activity Heatmap**: Grid showing invocations by hour-of-day and day-of-week -- see when your agent is most active
- **Tokens Saved by Project**: Horizontal bar chart breaking down savings per project

The dashboard auto-refreshes every 30 seconds, so you can keep it open while working and watch the data update live.

### Docker

The Docker image runs the dashboard alongside the MCP server by default (see [Docker](#docker) above). Open `http://localhost:3000` to view it.

## How It Works

- **Go**: Parses with `go/ast`, walks `FuncDecl` nodes, nils out `Body`, prints with `go/printer`.
- **JS/TS**: Parses with [tree-sitter](https://github.com/tree-sitter/go-tree-sitter) + language grammars, walks `export_statement` nodes, extracts signatures with bodies stripped.
- **Python**: Parses with tree-sitter + `tree-sitter-python`, walks top-level `function_definition`, `class_definition`, `decorated_definition`, and UPPER_CASE constant assignments.

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

**Python**: Top-level public symbols (no leading `_`):
- `def foo(a, b) -> int`
- `async def fetch(url: str)`
- `class Foo(Base)`
- `@decorator def bar()`
- `FOO = 42` (module-level UPPER_CASE constants)

### Skipped Files/Directories

- `vendor/`, `node_modules/`, `.git/`, `testdata/`, `dist/`, `build/`, `.next/`
- `__pycache__/`, `.venv/`, `venv/`, `.tox/`
- `*_test.go`, `*.test.*`, `*.spec.*`, `*.d.ts`, `*.min.*`
- `test_*.py`, `*_test.py`, `conftest.py`

## Project Layout

```
cmd/vfs/
  main.go           CLI entry point
  root.go           Root command (scan paths, filter, record stats)
  mcp.go            MCP server (tool handlers, stdio/HTTP transport)
  serve.go          Combined MCP server + dashboard in one process
  dashboard.go      Dashboard HTTP server + API
  dashboard.html    Embedded SPA (dark theme, uPlot charts)
internal/
  parser/
    registry.go     Parser registration and extension matching
    types.go        Shared types (Stats, FileResult, ComputeStats)
    walker.go       Language-agnostic directory walker
    goparser/       Go parser (go/ast)
    tsparser/       JS/TS parser (tree-sitter)
    pyparser/       Python parser (tree-sitter)
    hclparser/      HCL/Terraform parser (tree-sitter)
    dockerparser/   Dockerfile parser (line-based)
    protoparser/    Protocol Buffers parser (line-based)
    sqlparser/      SQL DDL parser (line-based)
  stats/            Performance history tracking (~/.vfs/history.jsonl)
pkg/
  bench/            Side-by-side benchmark (grep/rg vs vfs)
Dockerfile          Multi-stage build (binary + CLI + MCP server)
entrypoint.sh       Docker entrypoint (server mode or CLI passthrough)
```

## Cursor Integration

A Cursor rule at `.cursor/rules/vfs-go-search.mdc` instructs the AI agent to use `vfs` instead of `grep`/`rg` when searching for function signatures. Copy this rule to other projects or add `vfs` instructions to your workspace `CLAUDE.md`.

## License

MIT
