# CLAUDE.md - AI Agent Guidelines

## 🧠 Core Philosophy
**You are an expert Go developer utilizing the `gopls-mcp` server.**
Your primary goal is to use semantic analysis tools instead of crude text search.
This ensures accuracy, saves tokens, and mimics how human experts use IDEs.

## 🚫 Restrictions
*   **DO NOT** use `grep`, `ripgrep`, or `find` to locate symbol definitions or references.
*   **DO NOT** use `go build` to check for compilation errors - use `go_build_check` instead (~500x faster).
*   **DO NOT** read full files solely to understand a single function's signature.
*   **DO NOT** make assumptions about package structure; use discovery tools.

## 🛠 Preferred Tool Workflows

### 1. Discovery & Navigation
| Goal | Tool | Why |
|---|---|---|
| **Start Here** | `get_started` | Overview of the project, entry points, and next steps. |
| **Map Project** | `analyze_workspace` | High-level map of packages and dependencies. |
| **Find Packages** | `list_module_packages` | Discover all packages in a module (useful when exploring new codebases). |
| **Find Symbol** | `go_search` | Fuzzy find functions/types by name (semantic). |
| **Explore Pkg** | `list_package_symbols` | List all symbols in a package (replaces `ls` + read). |

### 2. Reading & Understanding
| Goal | Tool | Why |
|---|---|---|
| **Go to Def** | `go_definition` | Jump directly to code definition (replaces `grep`). |
| **Find Usage** | `go_symbol_references` | Find callers/usages (semantic, not text matching). |
| **Implementations**| `go_implementation`| Find interface implementations (Crucial for Go). |
| **Check API** | `go_package_api` | See public interface of a package or dependency. |
| **Quick Docs** | `go_hover` | Read doc comments and signatures instantly. |
| **Hierarchy** | `get_call_hierarchy` | Trace incoming/outgoing calls to understand flow. |
| **Dependencies**| `get_dependency_graph`| Visualize package imports and dependents. |

### 3. Verification (CRITICAL)
| Goal | Tool | Why |
|---|---|---|
| **Check Errors/Build**| `go_build_check` | **REPLACES `go build`.** Run after edits to verify compilation. ~500x faster than `go build` via incremental type checking. Returns detailed errors with file/line/column. |
| **Dry Run Refactor** | `go_rename_symbol` | Preview refactors safely before applying changes. |

## 🏗️ Repository Context: gopls-mcp

This repository contains the `gopls-mcp` server source code, which wraps the official `gopls`.

*   **Project Root:** `gopls/mcpbridge` (Focus your work here)
*   **Build Command:** `go build .`
*   **Test Command:** `go test ./gopls/mcpbridge/...`

### 🧑‍💻 Contributing / Extending gopls-mcp
If you are adding a new tool to the server:
1.  **Implement Handler:** Add the logic in `gopls/mcpbridge/core/handlers.go` (or a new file in `core/`).
2.  **Register Tool:** Add the `GenericTool` definition to `tools` slice in `gopls/mcpbridge/core/server.go`.
3.  **Update Docs:** Run `go generate ./gopls/mcpbridge/core` to update the README automatically.

## 💡 Tips & Edge Cases

### File Reading (Use Sparingly)
Most of the time, you don't need to read full files. Use semantic tools instead:
- Use `go_file_context` for fast package metadata (no disk I/O)
- Use `go_read_file` only when you need to see actual implementation details
- Prefer `go_package_api` or `list_package_symbols` for understanding code structure

### Module Debugging
- Use `list_modules` to debug module dependency problems