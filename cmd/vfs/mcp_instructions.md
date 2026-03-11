# Code Search Strategy: Navigate with vfs, Understand with Grep/Read

> vfs is a **navigation tool** (find where things are), not an **understanding tool** (know how things work).
> Signatures without bodies create false confidence. Always read implementation before claiming to understand behavior.

## How vfs Works

vfs parses source files via AST and returns **exported signatures with bodies stripped**. It supports Go, JS, TS, Python, Rust, Java, C#, Dart, Kotlin, Swift, Ruby, HCL, Dockerfile, Protobuf, SQL, and YAML.

**What vfs gives you:** `internal/services/fare.go:42: func CalculateFare(req *FareRequest) (*FareResponse, error)`
**What vfs hides:** The 50 lines of implementation inside that function.

This makes vfs excellent for **locating** definitions, but dangerous for **understanding** behavior.

## Step 1: Classify Your Intent

Before searching, determine what you need:

| Intent | Description | Primary tool | Depth required |
|--------|-------------|-------------|----------------|
| **Locate** | "Which file defines X?" | vfs | Signature only — no Read needed |
| **Understand** | "How does X work?" | vfs → Read body + context | Full implementation + dependencies |
| **Modify** | "Change how X behaves" | vfs → Read body + callers | Full implementation + Grep for usages |
| **Debug** | "Why does X fail?" | Grep + Read | Bodies, callers, error paths — vfs alone is useless here |

## Step 2: Search (vfs for navigation)

Use vfs as the **first step** to locate definitions — not as the final answer.

### CRITICAL: Use Absolute Paths

MCP runs on the host outside any sandbox. **Always use absolute paths** in `paths` arguments. Relative paths resolve against the MCP server's working directory, not the project being edited.

### MCP Tools

| Tool | Purpose | Parameters | Example |
|------|---------|------------|---------|
| `search` | Find definitions by name | `paths: string[]`, `pattern: string` | `search(paths: ["/abs/path"], pattern: "auth")` |
| `extract` | List all exported signatures | `paths: string[]` | `extract(paths: ["/abs/path/internal/handlers"])` |
| `list_languages` | Supported languages and extensions | none | `list_languages()` |

### CLI Fallback

If MCP is unavailable, use the CLI:

```
vfs <path> -f <pattern>           # filter signatures (case-insensitive)
vfs .                             # all exported sigs in current project
vfs ./internal ./pkg              # scan specific directories
```

## Step 3: Read with Sufficient Depth (Anti-Hallucination)

> **NEVER assume you understand a function's behavior from its signature alone.**
> A signature is an address, not a description. You must read the body before making claims.

After vfs locates a signature, determine how much to read based on your intent:

### Locate intent — signature is enough
```
vfs search → found fare.go:42: func CalculateFare(...)
Answer: "CalculateFare is defined in internal/services/fare.go at line 42."
Done. No Read needed.
```

### Understand intent — read the full function + surrounding context
```
vfs search → found fare.go:42: func CalculateFare(...)

Read: fare.go L1-20    (imports + package-level vars — reveals dependencies)
Read: fare.go L42-90   (the full function body — reveals actual behavior)
```

**Why read imports/package-level context:** A function that imports `"encoding/csv"` behaves very differently from one that imports `"net/http"`. The signature won't tell you this.

**Minimum read range:** For any function body, read at least:
- The complete function (not just the first 10 lines — logic often lives at the end)
- Package-level variables and init() if they exist (first 20-30 lines of file)
- Types referenced in the signature if they're in the same package

### Modify intent — read body + find all callers
```
vfs search → found fare.go:42: func CalculateFare(...)

Read: fare.go L1-20    (imports)
Read: fare.go L42-90   (full body)
Grep: "CalculateFare" across the codebase  (find all callers before changing)
```

### Debug intent — skip vfs, start with Grep/Read
When debugging, you need to follow execution flow through bodies. vfs strips the information you need most. Start with Grep for error messages, log strings, or the failing function name, then Read the relevant bodies.

## When to Skip vfs Entirely

Use Grep/Read directly when:

1. **You already know the exact file and line** — just Read it.
2. **Searching inside function bodies** — string literals, config keys, error messages, log strings.
3. **Non-code files** — JSON, CSS, Markdown, `.env`.
4. **The user gave you a file path** — e.g. "look at line 50 of client.go".
5. **Debugging** — you need execution flow through bodies, not signatures.
6. **Finding callers/usages** — vfs finds definitions, not call sites. Use Grep for "who calls X?"

## Generated Files

vfs automatically skips protobuf-generated files (`*.pb.go`, `*_pb2.py`, `*_pb.js`, `*_pb.ts`, `*.pb.dart`, `*_pb.rb`, etc.) and other common generated patterns (`.g.dart`, `.generated.cs`, `.freezed.dart`). They will not appear in search results.

If you encounter a generated file through Grep or Read, **do not modify it** — find and edit the source (`.proto`, codegen config) instead.

## When to Use vfs + Grep Together

The strongest search pattern combines both:

```
# 1. vfs: locate the definition
vfs search("CalculateFare")
  → internal/services/fare.go:42

# 2. Read: understand the implementation
Read fare.go L1-20 (imports), L42-90 (body)

# 3. Grep: find all callers and usages
Grep "CalculateFare" across internal/
  → internal/services/booking.go:88: resp, err := s.CalculateFare(req)
  → internal/services/fare_test.go:12: ...

# Now you have: where it's defined + how it works + who uses it
```

## Hallucination Guardrails

These are hard rules to prevent false confidence:

1. **Never describe what a function "does" based only on its name/signature.** You must Read the body first. A function named `GetAddress` might create addresses, call external APIs, or have side effects the name doesn't reveal.
2. **Never assume a function follows a pattern** just because other functions in the same file do. Always verify by reading the specific body.
3. **If you're about to say "this function probably..."** — STOP. That word "probably" means you haven't read the implementation. Read it first.
4. **After vfs, ask yourself: "Do I need to understand the behavior, or just the location?"** If behavior, you MUST Read the body before proceeding.
5. **When making code changes**, always Read the full function body AND Grep for callers. Never modify a function based on signature alone.

## Examples

### Locate — "where is the fare logic?"
```
vfs search("fare") → internal/services/fare.go:42: func CalculateFare(...)
Answer: "CalculateFare is defined in internal/services/fare.go at line 42."
```

### Understand — "how does fare calculation work?"
```
vfs search("CalculateFare") → internal/services/fare.go:42
Read: fare.go L1-20     ← imports reveal dependencies
Read: fare.go L42-90    ← full body reveals actual logic
Now describe the behavior based on what you READ, not what you assumed.
```

### Modify — "change fare calculation to add surcharge"
```
vfs search("CalculateFare") → internal/services/fare.go:42
Read: fare.go L1-20     ← imports
Read: fare.go L42-90    ← full body — understand current logic
Grep: "CalculateFare"   ← find all callers to assess impact
Now make the change with full context.
```

### Debug — skip vfs, go straight to bodies
```
Grep: "fare calculation failed" in internal/   ← find the error message
Read: fare.go L55-80                           ← read the error path
Grep: "CalculateFare" in internal/             ← trace the call chain
```

### When Grep IS the right first tool (skip vfs)
```
Grep: "INVALID_API_KEY" in ./internal/     ← string literal inside function body
Grep: "database_url" in ./*.env            ← non-code file
Read: handlers/upload.go L42-60            ← user gave exact file path
```

## Self-Check Before Answering

After searching and reading, ask yourself these questions before responding:

1. **Did I read the function body?** If I'm about to describe behavior and I only have a signature — STOP and Read the body first.
2. **Did I read the imports?** Dependencies reveal the real mechanism (embedded CSV vs HTTP API vs database).
3. **Am I saying "probably" or "likely"?** Those words mean I'm guessing. Read more code until I can state facts.
4. **For modifications: did I find all callers?** Grep for the function name across the codebase before suggesting changes.
5. **Is my answer based on what I READ or what I ASSUMED from the name?** If assumed, go back and read.

## Supported File Types

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
| HCL / Terraform | `.tf`, `.hcl` |
| Dockerfile | `Dockerfile`, `Dockerfile.*`, `*.dockerfile` |
| Protobuf | `.proto` |
| SQL | `.sql` |
| YAML | `.yml`, `.yaml` |

For anything not in this table, use Grep directly.
