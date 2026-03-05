# vfs -- Agent Instructions

> **Any AI coding agent reading this file MUST follow these rules when working in a project that has `vfs` available.**

vfs extracts exported function/class/type signatures from source code with bodies stripped. It reduces token consumption by **60-70%** compared to grep-based code search.

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
  ├─ Known file/line?        → Read directly
  ├─ Searching string literals? → Grep directly
  ├─ Non-code file?          → Grep directly
  │
  └─ Otherwise:
      1. vfs <path> -f <name>     (or MCP: search)
      2. Found? → Read exact file + line range
      3. Nothing? → Fall back to Grep
```

## How to Use

### Option 1: MCP (preferred if available)

If the MCP server is running, call these tools directly:

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

MCP config (stdio, for Cursor/Claude Desktop):

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

### Option 2: CLI

If MCP is not available, run the binary directly:

```bash
vfs <path> -f <pattern>     # filter signatures (case-insensitive)
vfs .                        # all exported sigs in current project
vfs ./internal ./pkg         # scan specific directories
vfs handler.go               # single file
```

### Pre-flight Check

On the **first code search** in a session, verify vfs is available:

- **MCP**: check if the `search` tool is listed
- **CLI**: run `command -v vfs`

If not available:
1. Tell the user: *"This project recommends vfs for efficient code search, but it's not installed."*
2. Offer to install: `go install github.com/TrNgTien/vfs/cmd/vfs@latest` or `make install` in the vfs repo.
3. If the user declines, fall back to grep/read for the rest of the session.

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

Every vfs invocation records to `~/.vfs/history.jsonl`. To view:

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
