# vfs -- Agent Instructions

> **Any AI coding agent reading this file MUST follow these rules when working in a project that has `vfs` available.**

## What is vfs?

vfs (Virtual Function Signatures) parses source files via AST and tree-sitter grammars, returning only exported function, class, and type signatures with bodies stripped. Instead of grepping or reading entire files -- which floods the context window with irrelevant implementation details -- vfs gives you a compact "table of contents" of any codebase. This reduces token consumption by **60-70%** compared to grep-based code search, leading to faster and more focused responses.

## Security & Privacy

vfs is a local-only, offline tool. It does not collect, transmit, or store any data outside the user's filesystem. It does not access API keys, secrets, or environment variables. It makes no network requests. Your source code never leaves the machine.

## Rule: Always Use vfs for Code Discovery

Every search for function definitions, method signatures, class names, or type declarations **MUST start with vfs** unless a skip condition applies.

### When to Use vfs

- "Where is function X defined?"
- "What functions relate to auth?"
- "Show me the API surface of this package"
- "How does module Y work?" (start by listing its exports)

### When to Skip vfs

1. **You already know the exact file and line** -- just read it.
2. **Searching inside function bodies** -- string literals, error messages, log strings, config keys -- vfs strips bodies, so use grep.
3. **Non-code files** -- JSON, CSS, Markdown, `.env`, config files -- use grep.
4. **The user gave you a file path** -- e.g. "look at line 50 of server.go" -- just read it.

### Decision Flow

```
User asks about code
  │
  ├─ Known file/line?          → Read directly
  ├─ Searching string literals? → Grep directly
  ├─ Non-code file?            → Grep directly
  │
  └─ Otherwise:
      1. MCP available? → search(paths, pattern)     ← preferred, works in sandbox
         CLI available? → vfs <path> -f <name>       ← fallback, requires shell access
      2. Found? → Read exact file + line range
      3. Nothing? → Fall back to Grep
```

## Sandboxed Environments (Cursor, Claude Code, etc.)

Many AI coding agents run inside a **sandbox** that restricts access to the host filesystem and installed binaries. In these environments, the `vfs` CLI binary -- even if installed on the host machine -- is **not accessible** from inside the sandbox. Running `command -v vfs` or `vfs . -f pattern` will fail with "command not found" or a permission error.

**The MCP server solves this.** MCP tools run outside the sandbox on the host machine, so the agent can call `search`, `extract`, etc. through the MCP protocol without needing direct access to the `vfs` binary.

### Why MCP is the preferred approach

| | CLI (`vfs . -f pattern`) | MCP (`search(paths, pattern)`) |
|---|---|---|
| Works in sandbox? | No -- binary not accessible | **Yes** -- MCP runs on host |
| Requires PATH setup? | Yes | No |
| Works across all editors? | Only if shell is available | Yes (Cursor, Claude Desktop, etc.) |

### Setup for sandboxed agents

1. **Configure the MCP server** in your editor (see [Cursor Configuration](#cursor-configuration) or [Claude Desktop Configuration](#claude-desktop-configuration) below).
2. The agent will detect the MCP `search` tool during pre-flight and use it automatically.
3. No CLI installation is needed inside the sandbox -- the MCP server handles everything.

> **If you are an AI agent reading this**: check for MCP tools **first**. If the `search` tool is available via MCP, use it -- do not attempt to shell out to `vfs`. The CLI fallback is only for environments where MCP is not configured and the shell has direct access to the binary.

## How to Use

### Option 1: MCP (preferred -- works everywhere including sandboxes)

If the MCP server is configured, call these tools directly. This is the **recommended approach** because it works regardless of whether the agent runs in a sandbox:

| Tool | Purpose | Parameters |
|------|---------|------------|
| `search` | Find signatures matching a pattern | `paths: string[]`, `pattern: string` |
| `extract` | Return all signatures from paths | `paths: string[]` |
| `stats` | Lifetime usage statistics | none |
| `list_languages` | Supported languages and extensions | none |

Examples:

```
search(paths: ["."], pattern: "HandleLogin")
extract(paths: ["./internal/handlers"])
```

MCP config (stdio, for Cursor/Claude Desktop -- works on macOS, Linux, and Windows):

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
- **Cursor**: `.cursor/mcp.json` (project) or `~/.cursor/mcp.json` (global). On Windows: `%USERPROFILE%\.cursor\mcp.json`.
- **Claude Desktop**: `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows).

> On Windows, `"command": "vfs"` resolves to `vfs.exe` on PATH automatically.

MCP config (HTTP, for Docker or remote):

```json
{
  "mcpServers": {
    "vfs": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

### Option 2: CLI (only when MCP is not available AND shell has access)

If MCP is not configured and the agent has direct shell access to the binary (i.e. **not** in a sandbox), run the CLI directly:

```bash
vfs <path> -f <pattern>     # filter signatures (case-insensitive)
vfs .                        # all exported sigs in current project
vfs ./internal ./pkg         # scan specific directories
vfs handler.go               # single file
```

> **Windows note**: The binary is `vfs.exe` but can be invoked as `vfs` from any terminal. Both forward slashes (`./src`) and backslashes (`.\src`) work as path separators.

### Pre-flight Check

On the **first code search** in a session, verify vfs is available. **Always check MCP first** -- it works in sandboxed environments where the CLI does not.

```
1. Check MCP: is the `search` tool available from the vfs MCP server?
   ├─ YES → use MCP for all vfs operations (preferred path)
   └─ NO  →
2. Check CLI: run `command -v vfs`
   ├─ YES → use CLI
   └─ NO  → vfs is not available (see below)
```

If **neither** MCP nor CLI is available:
1. Tell the user: *"This project recommends vfs for efficient code search, but it's not available. If you're in a sandboxed environment (Cursor, Claude Code), configure the vfs MCP server for best results. Otherwise, install the CLI."*
2. Offer options:
   - **MCP setup** (recommended for sandboxed agents): add vfs to `.cursor/mcp.json` or `claude_desktop_config.json` (see config examples above).
   - **CLI install** (for non-sandboxed environments): `make install` from the vfs repo (preferred -- runs pre-flight checks for Go, CGO, and C compiler), or `go install github.com/TrNgTien/vfs/cmd/vfs@latest`.
3. If CLI install fails:
   - **macOS**: missing C compiler or Xcode license -- tell the user to run `xcode-select --install` and/or `sudo xcodebuild -license accept`, then retry.
   - **Windows**: missing C compiler -- tell the user to install [MSYS2](https://www.msys2.org/) + MinGW-w64 (`pacman -S mingw-w64-x86_64-gcc`) and add `C:\msys64\mingw64\bin` to PATH. Alternatively, use [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) or Docker.
   - **Linux**: `apt install build-essential` (Debian/Ubuntu) or `yum groupinstall "Development Tools"` (RHEL/Fedora).
4. If the user declines, fall back to grep/read for the rest of the session.

> **Important**: In a sandbox, do not attempt `go install` or `make install` -- these will fail due to restricted permissions. Recommend MCP setup instead.

## Strict Rules

1. **NEVER start with grep/rg** for finding function definitions, method signatures, class names, or type declarations.
2. **NEVER read an entire file** to hunt for a function. Use vfs to locate it, then read only the specific lines.
3. **After vfs locates a signature**, read with the exact file and line range -- not the whole file.
4. **`-f` is case-insensitive** -- no need to search both `fare` and `Fare`.
5. **Do not pass `--no-record`** unless explicitly testing. Stats recording is on by default -- leave it.

## Examples

### Discovery: "what functions relate to auth?"

```
vfs . -f auth
  → src/handlers/auth.go:23: func HandleLogin(w http.ResponseWriter, r *http.Request)
  → src/services/auth.go:10: func ValidateToken(token string) (*Claims, error)
  → src/middleware/auth.go:5: func RequireAuth(next http.Handler) http.Handler

Read: src/handlers/auth.go L23-45
Read: src/services/auth.go L10-38
```

4 tool calls, ~80 lines ingested, zero noise.

### Pinpointing a single definition

WRONG:
```
grep "func.*CreateUser" ./src/     ← grep used first = violation
read user_service.go L1-200        ← scanning whole file = violation
```

RIGHT:
```
vfs ./src -f CreateUser
  → src/services/user.go:42: func CreateUser(name string, email string) (*User, error)

Read: src/services/user.go L42-78
```

### When grep IS correct

```
grep "INVALID_API_KEY" ./internal/   ← string literal inside function body
grep "database_url" ./*.yaml         ← non-code file
read handlers/upload.go L42-60       ← user gave exact file + line
```

## Supported Languages

| Language | Extensions |
|----------|-----------|
| Go | `.go` |
| JavaScript | `.js`, `.mjs`, `.cjs`, `.jsx` |
| TypeScript | `.ts`, `.mts`, `.cts`, `.tsx` |
| Python | `.py` |
| Rust | `.rs` |
| Java | `.java` |
| HCL/Terraform | `.tf`, `.hcl` |
| Dockerfile | `Dockerfile`, `Dockerfile.*`, `*.dockerfile` |
| Protobuf | `.proto` |
| SQL | `.sql` |
| YAML | `.yml`, `.yaml` |

For anything not in this table, use grep directly.

## Stats & Dashboard

Every vfs invocation records to `~/.vfs/history.jsonl` (on Windows: `%USERPROFILE%\.vfs\history.jsonl`). To view:

- **Terminal**: `vfs stats`
- **Dashboard**: `vfs dashboard` (opens http://localhost:3000)
- **Reset**: `vfs stats --reset`

## Running the Server

```bash
vfs serve                    # MCP server (:8080) + dashboard (:3000) foreground
vfs mcp                      # MCP server only (stdio, for editor integration)
vfs mcp --http :8080         # MCP server only (HTTP)
vfs dashboard                # dashboard only (:3000)
```

Docker:

```bash
docker run --rm -v $(pwd):/workspace -p 8080:8080 -p 3000:3000 vfs-mcp
```

Paths passed to MCP tools inside Docker must be relative to `/workspace`.
