<p align="center">
  <img src="static/logo.jpg" alt="vfs logo" width="200" style="border-radius: 40px;" />
</p>

# vfs

**Virtual Function Signatures** -- extract exported function, class, interface, and type signatures from source code with bodies stripped.

## Why vfs?

AI coding agents (Cursor, Claude Code, Copilot, etc.) waste tokens by grepping or reading entire files. vfs parses source via AST and tree-sitter, returning only exported signatures -- a compact "table of contents" of any codebase.

**60-70% fewer tokens per search.**

1. A [cursor rule](.cursor/rules/vfs-agent-search.mdc) or [AGENTS.md](AGENTS.md) instructs the agent to call `vfs` instead of grep/cat.
2. Agent calls vfs via **MCP** (preferred -- works in sandboxed editors) or CLI.
3. vfs returns signatures like `func HandleLogin(c *gin.Context)` with file:line.
4. Agent reads only the lines it needs.

## Benchmark

Self-benchmark on this repository (pattern `"Extract"`, 4,178 lines of source):

|                 | Read all files | grep       | vfs        |
|-----------------|----------------|------------|------------|
| Output size     | 101.9 KB       | 13.8 KB    | 1.5 KB     |
| Lines           | 4,178          | 148        | 15         |
| Est. tokens     | 26,079         | 3,537      | 373        |

- **vfs saves 98.6% tokens** vs reading all files (26,079 -> 373)
- **vfs saves 89.5% tokens** vs grep (3,537 -> 373)

Run it yourself:

```bash
vfs bench --self                                   # self-test on vfs source
vfs bench -f HandleLogin /path/to/go-project       # benchmark on any project
vfs bench -f Login /path/to/project --show-output  # show actual output
```

## Security & Privacy

> **Local-first by design.** Your source code never leaves your machine.

- **Zero network access** -- all parsing is local via AST and tree-sitter. No outbound connections, ever.
- **No secrets exposure** -- does not read, access, or store API keys, credentials, or environment variables.
- **No data collection** -- no telemetry, no analytics, no tracking.
- **No code storage** -- source is parsed in memory and discarded. Only `~/.vfs/history.jsonl` (scan statistics) is written.
- **Fully offline** -- install once, use forever.

## Supported Languages

| Language        | Extensions                              | Parser      |
|-----------------|-----------------------------------------|-------------|
| Go              | `.go`                                   | `go/ast`    |
| JavaScript      | `.js`, `.mjs`, `.cjs`, `.jsx`           | tree-sitter |
| TypeScript      | `.ts`, `.mts`, `.cts`, `.tsx`           | tree-sitter |
| Python          | `.py`                                   | tree-sitter |
| Rust            | `.rs`                                   | tree-sitter |
| Java            | `.java`                                 | tree-sitter |
| HCL / Terraform | `.tf`, `.hcl`                           | tree-sitter |
| Dockerfile      | `Dockerfile`, `Dockerfile.*`            | line-based  |
| Protobuf        | `.proto`                                | line-based  |
| SQL             | `.sql`                                  | line-based  |
| YAML            | `.yml`, `.yaml`                         | line-based  |

## Install

| Your situation | Method | What you need |
|---|---|---|
| **Linux** | [Pre-built binary](#pre-built-binary) | Nothing |
| **macOS / Linux / Windows** | [Build from source](#build-from-source) | Go 1.24+, C compiler |
| **Any OS** | [Docker](#docker) | Docker |

### Pre-built binary

Download from [GitHub Releases](https://github.com/TrNgTien/vfs/releases). No Go, no C compiler needed. Each release includes SHA-256 checksums.

```bash
# Linux x86_64
curl -L https://github.com/TrNgTien/vfs/releases/latest/download/vfs-linux-amd64.tar.gz | tar xz
sudo mv vfs /usr/local/bin/

# Linux ARM64
curl -L https://github.com/TrNgTien/vfs/releases/latest/download/vfs-linux-arm64.tar.gz | tar xz
sudo mv vfs /usr/local/bin/
```

### Build from source

Requires **Go 1.24+** and a **C compiler**:

- **macOS**: `xcode-select --install`
- **Linux**: `sudo apt install build-essential` (Debian/Ubuntu) or `sudo yum groupinstall "Development Tools"` (Fedora/RHEL)
- **Windows**: install [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) (easiest) or [MSYS2](https://www.msys2.org/) + MinGW-w64

```bash
git clone https://github.com/TrNgTien/vfs.git && cd vfs
go install ./cmd/vfs
```

> **`vfs: command not found`?** Add Go's bin to your PATH: `export PATH="$PATH:$(go env GOPATH)/bin"` (macOS/Linux) or add `%USERPROFILE%\go\bin` to PATH (Windows).

### Docker

```bash
docker build -t vfs-mcp .
docker run --rm -v $(pwd):/workspace -p 8080:8080 -p 3000:3000 vfs-mcp
```

## Quick Start

```bash
vfs . -f HandleLogin          # find a function
vfs ./internal ./pkg          # scan directories
vfs . --stats                 # show token savings
vfs up                        # start MCP server + dashboard (background)
vfs status                    # check if running
vfs down                      # stop
```

Open the dashboard at http://localhost:3000.

Run `vfs --help` for all commands and flags.

## MCP Server (for AI agents)

AI coding agents in Cursor, Claude Code, etc. run inside a **sandbox** that blocks access to host binaries. The `vfs` CLI won't work from inside the agent's shell. MCP tools run **outside** the sandbox on the host, so the agent can call `search`, `extract`, etc. directly.

**If you use an AI coding agent, configuring the MCP server is the recommended setup.**

Add to `.cursor/mcp.json` (Cursor) or `claude_desktop_config.json` (Claude Desktop):

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

Config file locations:
- **Cursor**: `.cursor/mcp.json` (project) or `~/.cursor/mcp.json` (global)
- **Claude Desktop**: `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows)

For Docker, use the HTTP endpoint instead:

```json
{
  "mcpServers": {
    "vfs": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

| MCP Tool | Description | Parameters |
|------|-------------|------------|
| `search` | Find signatures matching a pattern | `paths` (string[]), `pattern` (string) |
| `extract` | Return all exported signatures | `paths` (string[]) |
| `stats` | Lifetime usage statistics | none |
| `list_languages` | Supported languages and extensions | none |

For agent-specific integration rules, see [AGENTS.md](AGENTS.md).

## Releasing

Releases are automated via `scripts/release.sh` and [GitHub Actions](.github/workflows/release.yml). The script runs pre-flight checks (clean tree, tests pass, tag available), creates an annotated tag, and pushes. CI then builds Linux binaries (amd64 + arm64) and creates a GitHub Release with changelog and downloadable assets.

```bash
./scripts/release.sh              # release version from VERSION file
./scripts/release.sh --dry-run    # preview without changing anything
make release                      # same thing via make
```

The `VERSION` file at the repo root contains the current semver. See [AGENTS.md](AGENTS.md#releasing) for full details.

## License

MIT
