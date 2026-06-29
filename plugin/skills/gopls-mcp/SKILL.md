---
name: gopls-mcp
description: >
  Semantic Go code analysis using gopls — type-aware definitions, references, implementations,
  call hierarchy, dependency graphs, and rename previews. Use these tools for semantic Go tasks;
  never fall back to grep — gopls sees types, scopes, and interfaces that text search misses.
metadata:
  category: go-code-analysis
  requires:
    mcp_servers: ["gopls-mcp"]
---

# Project Instructions for AI Code Assistant with gopls-mcp

## What gopls-mcp does (and what it doesn't)

gopls-mcp is **strictly a semantic Go layer** built on top of gopls's type
checker. It exposes six tools — that's the whole surface area:

| Task | Tool |
|------|------|
| Jump to definition | `go_definition` |
| Find interface implementations | `go_implementation` |
| Find symbol references | `go_symbol_references` |
| Trace call relationships | `go_get_call_hierarchy` |
| Analyze package dependencies | `go_get_dependency_graph` |
| Preview a symbol rename | `go_dryrun_rename_symbol` |

These cannot be replaced by `Grep` + `Read`, because Go's type system makes
interfaces, scopes, and identity invisible to text search.

For *anything else* — listing packages, reading `go.mod`, searching symbol
names, running `go build`, scanning files — use your native tools (`Glob`,
`Grep`, `Read`, `Bash`). They are already optimal for those jobs, so we
deliberately don't ship duplicates.

## Routing rules

1. **Semantic relationship?** Use the table above. Never fall back to grep —
   you'd miss interface satisfaction, shadowing, cross-package identity.
2. **Text search / file listing / build / mod?** Use `Grep` / `Glob` /
   `Read` / `Bash`. There is no gopls-mcp tool for these.
3. **Output**: present `file:line` locations, signatures, and docs cleanly.
   Never dump raw JSON.

## Tool-specific constraints

* **`go_implementation`**: Interfaces and types only. **Not** for functions.
* **General locator parameters**:
  * `symbol_name`: bare identifier, no package prefix
    (`"Start"`, not `"Server.Start"`).
  * `context_file`: an absolute path to the file you are currently reading
    or where the symbol appears. The resolver uses this for scope and
    import disambiguation.

## Error handling

If a semantic tool returns nothing, do **not** silently fall back to grep:

1. Verify the symbol name is spelled exactly as in source (case-sensitive).
2. Confirm `context_file` is correct and inside a Go module.
3. For `go_implementation`, check that `parent_scope` points at the
   interface, not the concrete type.

Only after these checks fail should you fall back to manual `Read` of the
relevant file — and only as diagnosis, not as a substitute capability.
