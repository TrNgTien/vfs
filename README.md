<p align="center">
  <img src="static/vfs.png" alt="vfs logo" width="200" />
</p>

# vfs

**Extract exported function signatures with bodies stripped -- cutting 60-70% of the tokens your AI agent reads on every code search.**

[![CI](https://github.com/TrNgTien/vfs/actions/workflows/ci.yml/badge.svg)](https://github.com/TrNgTien/vfs/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/TrNgTien/vfs)](https://github.com/TrNgTien/vfs/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[Install](#installation) • [Quick Start](#quick-start) • [Commands](#commands) • [Setup for AI Tools](#setup-for-ai-tools) • [Supported Languages](#supported-languages) • [Security & Privacy](#security--privacy)

---

vfs parses source files via AST and tree-sitter and returns only exported signatures -- a compact "table of contents" of any codebase. One command (or one MCP call) replaces grep-and-read-everything with exact `file:line` pointers. Single Go binary, zero network access, works with any AI tool.

## What VFS Does

| Instead of                                       | vfs returns                                                            |
| ------------------------------------------------ | ---------------------------------------------------------------------- |
| `grep -r "HandleLogin" .` → 345 lines of matches | 21 signature lines                                                     |
| Reading 7,979 lines to find one function         | `auth.go:23: func HandleLogin(w http.ResponseWriter, r *http.Request)` |
| ~48,757 tokens of context                        | ~570 tokens                                                            |

## How Savings Work

vfs measures **bytes of source that reach the model**, estimated as `bytes / 4`. Bodies are stripped at parse time, so a function that costs 50 lines of context costs one line. The savings are real but the absolute token numbers are approximate -- vfs ships no tokenizer.

On this repository (`vfs bench --self`, pattern `"Extract"`):

|             | Read all files | grep    | vfs    |
| ----------- | -------------- | ------- | ------ |
| Output size | 190.5 KB       | 47.5 KB | 2.2 KB |
| Lines       | 7,979          | 345     | 21     |
| Est. tokens | 48,757         | 12,154  | 570    |

- **98.8% fewer tokens** vs reading all files (48,757 → 570)
- **95.3% fewer tokens** vs grep (12,154 → 570)

> These are reductions in context tokens, not in your bill -- but for a tool your agent runs on every search, they add up fast.

## Installation

### Pre-built binary (Linux -- nothing else needed)

```bash
# x86_64
curl -L https://github.com/TrNgTien/vfs/releases/latest/download/vfs-linux-amd64.tar.gz | tar xz
sudo mv vfs /usr/local/bin/

# ARM64
curl -L https://github.com/TrNgTien/vfs/releases/latest/download/vfs-linux-arm64.tar.gz | tar xz
sudo mv vfs /usr/local/bin/
```

### Build from source (macOS / Linux / Windows)

Requires **Go 1.24+** and a **C compiler** (tree-sitter C bindings):

| OS                    | Command                                                                                                         |
| --------------------- | --------------------------------------------------------------------------------------------------------------- |
| macOS                 | `xcode-select --install`                                                                                        |
| Linux (Debian/Ubuntu) | `sudo apt install build-essential`                                                                              |
| Linux (Fedora/RHEL)   | `sudo yum groupinstall "Development Tools"`                                                                     |
| Windows               | Install [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) (easiest) or [MSYS2](https://www.msys2.org/) + MinGW-w64 |

```bash
git clone https://github.com/TrNgTien/vfs.git && cd vfs
go install ./cmd/vfs
```

> `vfs: command not found` after install? Add Go's bin to PATH: `export PATH="$PATH:$(go env GOPATH)/bin"` (macOS/Linux) or add `%USERPROFILE%\go\bin` to PATH (Windows).

### Docker (any OS)

```bash
docker build -t vfs-mcp .
docker run --rm -v $(pwd):/workspace -p 8080:8080 -p 3000:3000 vfs-mcp

# Custom ports via environment variables
docker run --rm -v $(pwd):/workspace -e VFS_PORT=9090 -e VFS_DASHBOARD_PORT=4000 -p 9090:9090 -p 4000:4000 vfs-mcp
```

### Verify Installation

```bash
vfs . -f HandleLogin   # Should show signatures, not an error
vfs bench --self       # Should show the savings table
```

## Quick Start

```bash
# Find a function by name (case-insensitive)
vfs . -f HandleLogin

# List all signatures in a directory or file
vfs ./internal ./pkg
vfs server.go

# Show token savings after output
vfs . -f auth --stats

# Start the MCP server + dashboard (detached)
vfs up
# Check / stop it
vfs status
vfs down
```

Then open the dashboard at `http://localhost:3000` for usage stats over time.

## How It Works

```
  Without vfs:                                    With vfs:

  Agent  --grep "HandleLogin"-->  shell          Agent  --vfs search-->  vfs (AST)
    ^                                |             ^                          |
    |        345 lines of matches    |             |   2 sigs, ~570 tokens     |
    +--------------------------------+             +--- file:line + signature -+
```

Four properties make the savings work:

1. **AST parsing** - understands structure, not just text. Finds `class User`, `type User struct`, `func (u *User) Login`, not just string matches.
2. **Bodies stripped** - you get the signature, not the 50 lines underneath it.
3. **Case-insensitive filter** - one search finds `fare`, `Fare`, and `FARE`.
4. **Token accounting** - every invocation is measured and reported (`--stats`, `vfs stats`, dashboard).

The companion rule in this repo ([`.cursor/rules/vfs-agent-search.mdc`](.cursor/rules/vfs-agent-search.mdc)) teaches agents when to use vfs (locate/understand/modify) vs grep (debug, string literals, callers) -- see [Setup for AI Tools](#setup-for-ai-tools).

## Commands

### Search

```bash
vfs .                        # All exported signatures (recursive)
vfs ./src ./lib              # Multiple directories
vfs handler.go               # Single file
vfs . -f auth                # Case-insensitive filter
vfs . -f auth --stats        # + token savings report
vfs . -f auth --no-record    # Skip history logging
```

### Benchmarks

```bash
vfs bench --self                            # Self-test on vfs source
vfs bench -f HandleLogin /path/to/project    # Benchmark any project
vfs bench -f Login /path --show-output       # Show actual grep/vfs output
```

### Statistics

```bash
vfs stats                   # Lifetime token savings
vfs stats --json            # JSON export for dashboards/CI
vfs stats --reset           # Clear all history
```

### MCP Server

```bash
vfs mcp                     # stdio transport (default, editor integration)
vfs mcp --http :8080        # HTTP transport (Docker / remote)
vfs serve                   # MCP + dashboard, foreground
vfs serve --port 9090 --dashboard-port 4000   # custom ports
vfs up                      # Same, detached (background)
vfs status                  # Is it running? show endpoints
vfs down                    # Stop the detached server
vfs dashboard               # Dashboard UI only
```

MCP tools exposed: `search`, `extract`, `list_languages`.

## Global Flags

| Flag             | Description                                            |
| ---------------- | ------------------------------------------------------ |
| `-f`, `--filter` | Case-insensitive substring filter on signature names   |
| `--stats`        | Print token savings (raw vs vfs) to stderr             |
| `--no-record`    | Skip logging this invocation to `~/.vfs/history.jsonl` |
| `--version`      | Show version, commit, and build date                   |

## Examples

**"Where is the login handler?"**

```
# Without vfs: grep + read
grep -r "HandleLogin" .            # 345 lines of matches
cat auth.go                        # then read the whole file

# With vfs
vfs . -f login
  → internal/handlers/auth.go:23:   func HandleLogin(w http.ResponseWriter, r *http.Request)
  → internal/services/auth.go:10:   func ValidateToken(token string) (*Claims, error)
  → internal/middleware/jwt.go:45:  func RequireLogin(next http.Handler) http.Handler
```

**Multi-language project:**

```
vfs . -f user
  → internal/services/user.go:42:     func CreateUser(name string, email string) (*User, error)
  → src/hooks/useUser.ts:8:           export function useUser(id: string): UserState
  → app/models/user.py:15:            class User(BaseModel)
  → src/api/UserService.java:22:      public class UserService
  → contracts/UserRegistry.sol:10:    contract UserRegistry is Ownable { ... }
```

**When grep IS the right tool** -- vfs finds definitions, not bodies, callers, or config keys:

```
grep "INVALID_API_KEY" ./internal/     # string literal inside a function body
grep "database_url" ./*.yaml           # non-code file
grep "CalculateFare" ./internal/       # who CALLS it (vfs finds the definition)
```

## Setup for AI Tools

Two steps -- both required:

1. **Connect vfs** -- make it reachable from the agent (MCP preferred, CLI fallback).
2. **Add an agent rule** -- tell the agent it *should* call vfs before grep. Without the rule, the agent ignores vfs even when installed.

### Step 1: Connect vfs

| Method                | How it works                                                 | Best for                                             |
| --------------------- | ------------------------------------------------------------ | ---------------------------------------------------- |
| **MCP (recommended)** | Agent calls `search` / `extract` / `list_languages` directly | Editors with MCP support (most modern AI tools)      |
| **CLI**               | Agent runs `vfs <path> -f <pattern>` via shell               | Terminal tools, scripts, sandboxed-free environments |

**Stdio config** (Cursor, Claude Code, Claude Desktop, Antigravity, Windsurf, Cline):

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

**Claude Code** (preferred): `claude mcp add vfs -- vfs mcp` -- registers in `~/.claude.json`.

**Continue**:

```json
{
  "experimental": {
    "modelContextProtocolServers": [
      { "transport": { "type": "stdio", "command": "vfs", "args": ["mcp"] } }
    ]
  }
}
```

**Zed**:

```json
{
  "context_servers": {
    "vfs": { "command": { "path": "vfs", "args": ["mcp"] } }
  }
}
```

**HTTP config** (Docker, remote, or tools that prefer HTTP):

```bash
vfs up        # MCP on :8080, dashboard on :3000
vfs up --port 9090
```

```json
{
  "mcpServers": { "vfs": { "url": "http://localhost:8080/mcp" } }
}
```

### Step 2: Add an agent rule (required)

Installing vfs is not enough -- agents don't know it exists. This repo ships a production-ready rule at [`.cursor/rules/vfs-agent-search.mdc`](.cursor/rules/vfs-agent-search.mdc), and a one-command installer to put it into your tool:

```bash
./scripts/install-rules.sh                   # auto-detect your AI tool
./scripts/install-rules.sh --agent cursor    # install for one tool
./scripts/install-rules.sh --agent all       # every tool
./scripts/install-rules.sh --project ../myapp --agent claude   # another project
./scripts/install-rules.sh --list            # supported tools + paths
```

Same via Make: `make rules` or `make rules RULES_ARGS="--agent windsurf --project ../myapp"`.

The installer is idempotent and safe: plain-markdown tools (Claude Code, Cline, Aider, Antigravity) get the rule **appended** under a `## vfs:` heading -- your existing config is never overwritten -- while frontmatter tools (Cursor, Windsurf, Continue, OpenCode) get a verbatim copy.

| Tool                 | Rule file                 | Notes                                                  |
| -------------------- | ------------------------- | ------------------------------------------------------ |
| **Cursor**           | `.cursor/rules/vfs.mdc`   | Copy of `.mdc` (frontmatter kept)                      |
| **Claude Code**      | `CLAUDE.md`               | Appended, frontmatter stripped                         |
| **Antigravity**      | `GEMINI.md`               | Appended, frontmatter stripped; also reads `AGENTS.md` |
| **Windsurf**         | `.windsurf/rules/vfs.md`  | Copy of `.mdc`                                         |
| **Cline / Roo Code** | `.clinerules`             | Appended, frontmatter stripped                         |
| **Continue**         | `.continue/rules/vfs.md`  | Copy of `.mdc`                                         |
| **Aider**            | `.aider.conventions.md`   | Appended, frontmatter stripped                         |
| **OpenCode**         | `.opencode/rules/vfs.mdc` | Copy of `.mdc`                                         |

Without the rule, this is what happens:

```
You: "Where is the login handler?"

❌ No rule:   Agent greps + reads → 345 lines → 12,000+ tokens
✅ With rule: Agent calls vfs → reads 23 lines → ~570 tokens
```

## Supported Languages

| Language        | Extensions                    | Parser      |
| --------------- | ----------------------------- | ----------- |
| Go              | `.go`                         | `go/ast`    |
| JavaScript      | `.js`, `.mjs`, `.cjs`, `.jsx` | tree-sitter |
| TypeScript      | `.ts`, `.mts`, `.cts`, `.tsx` | tree-sitter |
| Python          | `.py`                         | tree-sitter |
| Rust            | `.rs`                         | tree-sitter |
| Java            | `.java`                       | tree-sitter |
| C#              | `.cs`                         | tree-sitter |
| Dart            | `.dart`                       | tree-sitter |
| Kotlin          | `.kt`, `.kts`                 | tree-sitter |
| Swift           | `.swift`                      | tree-sitter |
| Ruby            | `.rb`                         | tree-sitter |
| Solidity        | `.sol`                        | tree-sitter |
| HCL / Terraform | `.tf`, `.hcl`                 | tree-sitter |
| Dockerfile      | `Dockerfile`, `Dockerfile.*`  | line-based  |
| Protobuf        | `.proto`                      | line-based  |
| SQL             | `.sql`                        | line-based  |
| YAML            | `.yml`, `.yaml`               | line-based  |

## Security & Privacy

vfs is local-only and offline.

| Property            | Detail                                                                                             |
| ------------------- | -------------------------------------------------------------------------------------------------- |
| Zero network access | All parsing is local. No outbound connections, ever                                                |
| No secrets exposure | Does not read or store API keys, credentials, or environment variables                             |
| No data collection  | No telemetry, no analytics, no tracking                                                            |
| No code storage     | Source is parsed in memory and discarded. Only `~/.vfs/history.jsonl` (scan statistics) is written |

## Configuration

There is no config file -- vfs is zero-config. The only state it keeps is scan history for stats and the dashboard:

| File                   | Purpose                                                       |
| ---------------------- | ------------------------------------------------------------- |
| `~/.vfs/history.jsonl` | One line per invocation (paths, filter, bytes, tokens saved)  |
| `~/.vfs/vfs.state`     | Running server PID + endpoints (used by `up`/`down`/`status`) |

Clear it anytime with `vfs stats --reset`.

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines. When you add a language, update the supported-languages tables in `AGENTS.md` and `README.md`.

Releases are automated via [`.github/workflows/release.yml`](.github/workflows/release.yml) -- see [`scripts/release.sh`](scripts/release.sh).

## Used By

> Using vfs in your project or company? [Submit a PR](#how-to-add-your-use-case) to be listed here.

|    Logo     | Name                     | Website     | Use Case                 | Blog     |
| :---------: | :----------------------- | :---------- | :----------------------- | :------- |
| *(img url)* | *(Company/Project Name)* | [Website]() | *(Use Case Description)* | [Read]() |

### How to Add Your Use Case

1. Fork this repo and create a branch: `usecase/<your-company-or-project>`
2. Add one row to the table above (logo, name, website, one-line use case, optional blog).
3. Open a pull request titled `usecase: add <Company/Project>`.

No proprietary details needed -- a high-level description is fine.

## License

MIT