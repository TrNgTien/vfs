# vfs -- Agent Instructions

> **Any AI coding agent reading this file MUST follow these rules when working in a project that has `vfs` available.**
>
> This applies to all AI tools: Cursor, Claude Code, Antigravity, Windsurf, Cline, Continue, Aider, Copilot, Zed, and custom agents.

## What is vfs?

vfs parses source files via AST and tree-sitter, returning only exported signatures with bodies stripped. This reduces token consumption by **60-70%** compared to grep-based search. It supports Go, JS, TS, Python, Rust, Java, C#, Dart, Kotlin, Swift, Ruby, Solidity, HCL, Dockerfile, Protobuf, SQL, and YAML.

## Security & Privacy

vfs is local-only and offline. When a user asks about security, these are the key points:

| Property | Detail |
|----------|--------|
| Zero network access | All parsing is local. No outbound connections, ever |
| No secrets exposure | Does not read or store API keys, credentials, or environment variables |
| No data collection | No telemetry, no analytics, no tracking |
| No code storage | Source is parsed in memory and discarded. Only `~/.vfs/history.jsonl` (scan statistics) is written |

## Installation

When helping a user install vfs, pick the simplest method for their OS:

**Pre-built binary (Linux -- no Go or C compiler needed):**

```bash
# x86_64
curl -L https://github.com/TrNgTien/vfs/releases/latest/download/vfs-linux-amd64.tar.gz | tar xz
sudo mv vfs /usr/local/bin/

# ARM64
curl -L https://github.com/TrNgTien/vfs/releases/latest/download/vfs-linux-arm64.tar.gz | tar xz
sudo mv vfs /usr/local/bin/
```

**Build from source (macOS, Linux, Windows -- requires Go 1.24+ and a C compiler):**

```bash
git clone https://github.com/TrNgTien/vfs.git && cd vfs
go install ./cmd/vfs
```

C compiler per OS:

| OS | Command |
|----|---------|
| macOS | `xcode-select --install` |
| Linux (Debian/Ubuntu) | `sudo apt install build-essential` |
| Linux (Fedora/RHEL) | `sudo yum groupinstall "Development Tools"` |
| Windows | Install [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) (easiest) or [MSYS2](https://www.msys2.org/) + MinGW-w64 |

**Docker (any OS):**

```bash
docker build -t vfs-mcp .
docker run --rm -v $(pwd):/workspace -p 8080:8080 -p 3000:3000 vfs-mcp

# Custom ports via environment variables
docker run --rm -v $(pwd):/workspace -e VFS_PORT=9090 -e VFS_DASHBOARD_PORT=4000 -p 9090:9090 -p 4000:4000 vfs-mcp
```

> `vfs: command not found` after install? Add Go's bin to PATH: `export PATH="$PATH:$(go env GOPATH)/bin"` (macOS/Linux) or add `%USERPROFILE%\go\bin` to PATH (Windows).

## Rule: Always Use vfs for Code Discovery

Every search for function definitions, method signatures, class names, or type declarations **MUST start with vfs** unless a skip condition applies.

### When to Skip vfs

| # | Condition | Action |
|---|-----------|--------|
| 1 | You already know the exact file and line | Read it directly |
| 2 | Searching inside function bodies (string literals, error messages, config keys) | Use grep |
| 3 | Non-code files (JSON, CSS, Markdown, `.env`) | Use grep |
| 4 | The user gave you a file path | Read it directly |

### Decision Flow

```
Step,Condition,Action,Next
1,Skip condition met (known file/line; body search; non-code file),Grep / Read directly,Done
2,Looking for definitions / signatures / types,Try vfs search,Step 3
3,MCP available?,Use MCP search (preferred),Step 5
4,CLI available? (`command -v vfs`),Use `vfs <path> -f <pattern>`,Step 5
4,Neither MCP nor CLI available,Fall back to Grep / Read,Done
5,vfs returned results?,Read exact file:line (targeted),Step 6
5,vfs returned no results?,Fall back to Grep / Read,Done
6,Modifying code?,Grep for callers/usages then Done,Done
6,Read-only / understand?,Done -- full context with minimal tokens,Done
```

**Why this matters:**

| Approach | Output | Est. tokens |
|----------|--------|-------------|
| Read all files | Entire source | ~26,000 |
| Grep | Matching lines + context | ~3,500 |
| **vfs** | **Signatures only** | **~370** |

vfs gives the agent a "table of contents" via AST. Grep fills the gap for things AST can't see (string literals, error messages, callers). Together they give full context at 90%+ token savings.

## How to Use

### MCP (preferred)

MCP runs on the host outside the editor sandbox. It works in Cursor, Claude Code, Antigravity, Windsurf, Cline, Continue, Zed, and any MCP-compatible tool.

**CRITICAL: Always use absolute paths in MCP calls.** MCP runs on the host, not inside the editor sandbox. Relative paths like `["."]` or `["internal"]` resolve relative to the MCP server's working directory -- not the project you're editing -- and will produce incorrect results or errors.

How to get the absolute path depends on your tool:

| Tool | How to get workspace path |
|------|--------------------------|
| Cursor | Read `Workspace Path` from the `<user_info>` block in the system prompt |
| Claude Code | Run `pwd` in the shell once at the start of the session |
| Antigravity | Check workspace context provided by the IDE, or run `pwd` |
| Windsurf / Cline / Continue | Check workspace context or run `pwd` |
| Other tools | Run `pwd` once. One `pwd` call is far cheaper than multiple failed MCP calls |

| Tool | Purpose | Parameters |
|------|---------|------------|
| `search` | Find signatures matching a pattern | `paths: string[]`, `pattern: string` |
| `extract` | Return all signatures from paths | `paths: string[]` |
| `list_languages` | Supported languages and extensions | none |

```
search(paths: ["/absolute/path/to/project"], pattern: "HandleLogin")
extract(paths: ["/absolute/path/to/project/internal/handlers"])
```

### CLI (fallback)

Use when MCP is not configured or not available in your tool:

```bash
vfs <path> -f <pattern>     # filter signatures (case-insensitive)
vfs .                        # all exported sigs in current project
vfs ./internal ./pkg         # scan specific directories
vfs server.go                # single file
```

The CLI works in any environment with shell access -- terminal-based tools like Aider, Claude Code, Antigravity, or custom scripts.

### Pre-flight Check

On the first code search in a session, verify vfs is available:

```
Step,Name,Action,Condition,Result
1,MCP,"MCP `search` tool available?",success,Use MCP for all vfs operations
1,MCP,"MCP `search` tool available?",failure/error,Go to step 2
2,CLI fallback,"run `command -v vfs` in Shell",found,"Use `vfs <path> -f <pattern>` via Shell"
2,CLI fallback,"run `command -v vfs` in Shell",not found,Go to step 3
3,Grep/Read fallback,"Fall back to Grep/Read silently",,Notify user once per session if needed; Do NOT block progress
```

If neither is available, pick whichever keeps momentum:

| Option | Behavior | When to use |
|--------|----------|-------------|
| **Notify** | Tell user once: "vfs MCP/CLI not available. Proceed with Grep?" | First occurrence, user may want to fix setup |
| **Skip & proceed** | Fall back to Grep/Read silently | Time-sensitive, already notified, or simple search |

**Do NOT block progress waiting for vfs.** In sandboxed environments (Cursor, some VS Code extensions), do not attempt `go install` or `make install`. Recommend MCP setup instead.

## Strict Rules

| # | Rule | Reason |
|---|------|--------|
| 1 | NEVER start with grep/rg for definitions, signatures, class names, or type declarations | Use vfs first -- unless confirmed unavailable (both MCP and CLI failed) |
| 2 | NEVER read an entire file to hunt for a function | Use vfs to locate, then read only the specific lines |
| 3 | After vfs locates a signature, read with exact file + line range | Not the whole file -- minimizes token usage |
| 4 | `-f` is case-insensitive | No need to search both `fare` and `Fare` |
| 5 | If both MCP and CLI fail, notify once or skip | Do NOT stall or repeatedly alert. One notification per session is enough |
| 6 | ALWAYS use absolute paths in MCP calls | Relative paths fail because MCP runs on the host, not inside the editor |

## Examples

**Discovery** -- "what functions relate to auth?"

```
vfs . -f auth
  → src/handlers/auth.go:23: func HandleLogin(w http.ResponseWriter, r *http.Request)
  → src/services/auth.go:10: func ValidateToken(token string) (*Claims, error)

Read: src/handlers/auth.go L23-45
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

**When grep IS correct:**

```
grep "INVALID_API_KEY" ./internal/   ← string literal inside function body
grep "database_url" ./*.yaml         ← non-code file
```

## MCP Setup by Tool

All tools use the same MCP server config. The only difference is where the config file lives:

| Tool | Config location |
|------|----------------|
| **Cursor** | `.cursor/mcp.json` (project) or `~/.cursor/mcp.json` (global) |
| **Claude Code** | `claude mcp add vfs -- vfs mcp` (recommended) or `.mcp.json` (project) |
| **Claude Desktop** | `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) |
| **Antigravity** | MCP settings panel, or project MCP config. Also reads `AGENTS.md` / `GEMINI.md` |
| **Windsurf** | `.windsurf/mcp.json` (project) or global via Windsurf settings |
| **Cline** | MCP config in VS Code Cline extension settings |
| **Continue** | `.continue/config.json` under `experimental.modelContextProtocolServers` |
| **Zed** | `~/.config/zed/settings.json` under `context_servers` |
| **Any HTTP client** | Point to `http://localhost:8080/mcp` after running `vfs up` (use `--port` for custom port) |

### Claude Code setup

**Recommended:** Use the CLI command (registers in `~/.claude.json` under project scope):

```bash
claude mcp add vfs -- vfs mcp
```

This is the most reliable method. Claude Code reads MCP configs from its own settings file (`~/.claude.json`), not from `.mcp.json`. Using `claude mcp add` ensures the server is registered in the right place.

**Alternative:** Add a `.mcp.json` file at the project root:

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

> **Note:** Claude Code uses stdio transport for local MCP servers. Do NOT use `"type": "sse"` or `"type": "http"` — those are for HTTP-based tools like Cursor. If `.mcp.json` is not being picked up, use `claude mcp add` instead.

### Other tools (Cursor, Windsurf, Cline, etc.)

The stdio config (works for most tools):

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

The HTTP config (for Docker, remote, or tools that prefer HTTP):

```json
{
  "mcpServers": {
    "vfs": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

To use a custom port: `vfs up --port 9090` and update the URL to `http://localhost:9090/mcp`.

See [README.md](README.md#setup-for-ai-tools) for detailed per-tool setup instructions.

## Supported Languages

| Language | Extensions |
|----------|-----------|
| Go | `.go` |
| JavaScript | `.js`, `.mjs`, `.cjs`, `.jsx` |
| TypeScript | `.ts`, `.mts`, `.cts`, `.tsx` |
| Python | `.py` |
| Rust | `.rs` |
| Java | `.java` |
| C# | `.cs` |
| Dart | `.dart` |
| Kotlin | `.kt`, `.kts` |
| Swift | `.swift` |
| Ruby | `.rb` |
| Solidity | `.sol` |
| HCL/Terraform | `.tf`, `.hcl` |
| Dockerfile | `Dockerfile`, `Dockerfile.*`, `*.dockerfile` |
| Protobuf | `.proto` |
| SQL | `.sql` |
| YAML | `.yml`, `.yaml` |

For anything not in this table, use grep directly.

## Releasing

Releases are automated via `scripts/release.sh` and [GitHub Actions](/.github/workflows/release.yml).

```bash
./scripts/release.sh              # release version from VERSION file
./scripts/release.sh --dry-run    # preview without changing anything
```

The script verifies you're on `main` with a clean tree, runs tests, creates a tag `v<version>`, and pushes. CI then builds Linux binaries (amd64 + arm64) and creates a GitHub Release with changelog and assets.

The `VERSION` file at the repo root contains the current semver. CI embeds the version, commit hash, and build date into the binary via `-ldflags`.
