# vfs -- Agent Integration Guide

vfs extracts exported function/class/type signatures from source code with bodies stripped. It reduces token consumption by 60-70% compared to grep-based code search.

## When to Use vfs

Use vfs as your **first step** for code discovery: finding function definitions, method signatures, class names, type declarations, or understanding a codebase's API surface.

Do NOT use vfs for:
- Searching inside function bodies (string literals, error messages, log strings)
- Non-code files (JSON, YAML, CSS, Markdown, `.env`, config)
- Files you already know the exact path and line for

## MCP Server

vfs exposes four tools via the Model Context Protocol. If the MCP server is available, prefer these over CLI invocations.

### Tools

| Tool | Purpose | Required Params |
|------|---------|-----------------|
| `extract` | Return all exported signatures from paths | `paths: string[]` |
| `search` | Return signatures matching a pattern | `paths: string[]`, `pattern: string` |
| `stats` | Lifetime usage statistics | none |
| `list_languages` | Supported languages and extensions | none |

### Connection

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

Docker (HTTP transport):

```json
{
  "mcpServers": {
    "vfs": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

### MCP Examples

Discover all signatures in a directory:

```
extract(paths: ["/workspace/internal"])
```

Find a specific function:

```
search(paths: ["/workspace"], pattern: "HandleLogin")
```

## CLI Usage

If MCP is not available, use the CLI directly.

```bash
vfs <path> -f <pattern>     # filter signatures (case-insensitive)
vfs .                        # all exported sigs in current project
vfs ./internal ./pkg         # scan specific directories
vfs handler.go               # single file
```

## Workflow

```
1. User asks about code (function, class, type, feature)
       |
2. Can I skip vfs? (known file, non-code, searching inside bodies)
       |
   YES --> use Read/Grep directly
   NO  --> call `search` (MCP) or `vfs <path> -f <name>` (CLI)
       |
3. Got signatures with file:line --> Read only those lines
   Got nothing                   --> fall back to Grep
```

## Supported Languages

| Language | Extensions |
|----------|-----------|
| Go | `.go` |
| JavaScript | `.js`, `.mjs`, `.cjs`, `.jsx` |
| TypeScript | `.ts`, `.mts`, `.cts`, `.tsx` |
| Python | `.py` |
| HCL/Terraform | `.tf`, `.hcl` |
| Dockerfile | `Dockerfile`, `Dockerfile.*`, `*.dockerfile` |
| Protobuf | `.proto` |
| SQL | `.sql` |
| YAML | `.yml`, `.yaml` |

## Docker

```bash
docker run --rm -v $(pwd):/workspace -p 8080:8080 -p 3000:3000 vfs-mcp
```

- Port 8080: MCP server (Streamable HTTP at `/mcp`)
- Port 3000: Stats dashboard (open in browser)

Paths passed to MCP tools must be relative to `/workspace` inside the container.
