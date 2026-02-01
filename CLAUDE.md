# CLAUDE.md - Instructions for Claude

## What is gopls-mcp?

[gopls-mcp](https://github.com/xieyuschen/gopls-mcp) is an MCP (Model Context Protocol) server that provides semantic code intelligence for Go projects. It uses gopls (the Go language server) to give Claude IDE-like understanding of your codebase, [gopls-mcp documentation site](https://gopls-mcp.org).

## ⚡ Critical Protocol: Semantic vs. Text Search

**Core Philosophy: gopls-mcp is a Symbol Indexer, NOT a Semantic Search Engine.**
You are interacting with a strict code analysis tool, not a natural language search engine.

**Strict Rule for `go_search`:**
The `query` argument MUST be a specific symbol name (e.g., "LoopVar", "PosBase").
*   ❌ **Forbidden:** `go_search("function that handles loop variables")` (Do NOT use sentences or spaces)
*   ❌ **Forbidden:** `go_search("syntax.File.Pos PosBase")` (Do NOT combine multiple concepts)
*   ✅ **Correct:** `go_search("LoopVar")` -> then analyze results.

## 🔍 `go_search`: What It Searches

**⚠️ CRITICAL: What `go_search` CANNOT Search**

Before using `go_search`, understand what it CANNOT do:
- ❌ **Cannot search string literals**: `"Hello World"`, `"error:"` in comments
- ❌ **Cannot search comments**: `// TODO: fix this`, `// This is a bug`
- ❌ **Cannot search documentation content**: Only searches symbol names
- ❌ **Cannot search code patterns**: "for loops", "if statements", "struct definitions"
- ❌ **Cannot search by semantic meaning**: "functions that handle HTTP", "error handling code"
- ❌ **Cannot search local variables**: `func f() { x := 1 }` - won't find "x"

`go_search` ONLY searches **top-level symbol identifiers** (function names, type names, variable names, struct fields, function parameters).

If you need to search for **text content**, use:
- `go_read_file` + manual text search to read files and search for strings
- `go_build_check` to find compilation errors (which show problematic code)
- Your IDE's search functionality (Ctrl+F) for text search

`go_search` searches symbol names with fuzzy matching. **Struct fields and function parameters are included by default.**

### What `go_search` Covers

**Top-Level Declarations (included):**
- ✅ Top-level functions: `func MyFunction()`
- ✅ Top-level types: `type MyStruct`, `type MyInterface`
- ✅ Top-level variables: `var MyVar`, `const MyConst`
- ✅ Methods: `func (s *Server) Start()`
- ✅ Packages: `package http`
- ✅ **Struct fields**: `type Server { Name string }` - finds "Name"
- ✅ **Function parameters**: `func Handle(ctx context.Context)` - finds "ctx"
- ✅ Interface methods
- ✅ Named return values: `func f() (err error)` - finds "err"

**NOT Included (local scope only):**
- ❌ Local variables in function bodies: `func f() { x := 1 }`
- ❌ Range variables: `for i, v := range`

**Performance:** Fast (~50-150ms)

**Example:**
```json
{"query": "Name"}  // Finds struct fields, parameters, top-level symbols named "Name"
```

| User Intent | ✅ Tool to Use | ❌ Forbidden Command |
|-------------|----------------|----------------------|
| **Find a symbol by name** | `go_search("Name")` (Strictly single token!) | `go_search("find function X")`, `grep` |
| **Find usages/references** | `go_symbol_references` | `grep -r "Name" .` |
| **Find definition/signature** | `go_definition` | `grep`, `cat file.go` (just to find loc) |
| **List package symbols** | `go_list_package_symbols` | `ls`, `find`, reading full files |
| **Explore modules** | `go_list_modules`, `go_list_module_packages` | `find . -name "*.go"` |
| **Check compilation** | `go_build_check` | `go build` |
| **Understand dependencies** | `go_get_dependency_graph` | `grep "import"` |
| **Trace call stack** | `go_get_call_hierarchy` | `grep` for callers |
| **Preview rename** | `go_dryrun_rename_symbol` | Manual search & replace |

**Rules:**
1. **Never use grep/ripgrep/find for symbol discovery.**
2. **Never read full files just to find signatures or definitions.**
3. **Never use `go build` for checking errors (use `go_build_check`).**
4. **Ambiguity Handling:** If the user asks for a definition (e.g., "How does X work?") but you do not know which file X is in, you MUST use `go_search` first to locate it, and THEN use `go_definition` or `go_read_file` on the specific result. Do not guess file paths.

## 🧭 Recommended Investigation Workflow

1.  **Global Search (If location unknown):** Use `go_search("KnownSymbolName")` to find the file.
    *   *Tip:* If `go_search("LongSpecificName")` fails, try `go_search("SpecificName")`. Never add spaces.
2.  **Package Exploration (If context needed):** Once a file is found, use `go_list_package_symbols` on its package to see available tools/structs.
3.  **Deep Dive:** Use `go_read_file` or `go_definition` on specific targets found in step 2.
4.  **Trace:** Use `go_symbol_references` to see how it's used.

**Note:** If `go_search` returns nothing, do NOT fall back to `grep`. Try a shorter, simpler symbol name (e.g., instead of `DebugFlags.LoopVar`, try `LoopVar`).

## 🛠 Tool Reference

<!-- AUTO-GENERATED: DO NOT EDIT -->
<!-- Generated by: go generate ./gopls/mcpbridge/core -->
<!-- Source: gopls/mcpbridge/core/doc.go -->
<!-- Marker: AUTO-GEN-START -->
### Discovery & Navigation

- **go_get_started**: Get a beginner-friendly guide to start exploring the Go project.

- **go_analyze_workspace**: Analyze the entire workspace to discover packages, entry points, and dependencies.

- **go_list_modules**: List current module and direct dependencies only.

- **go_list_module_packages**: List all packages in a given module.

- **go_list_package_symbols**: List all exported symbols (types, functions, constants, variables) in a package.

- **go_search**: Search symbols by name with fuzzy matching.

### Reading & Understanding

- **go_definition**: Jump to the definition of a symbol using semantic location (symbol name, package, scope).

- **go_symbol_references**: Find all usages of a symbol across the codebase using semantic location (symbol name, package, scope).

- **go_implementation**: Find all implementations of an interface or all interfaces implemented by a type using semantic location (symbol name, package, scope).

- **go_read_file**: Read file content through gopls.

- **go_get_package_symbol_detail**: Get detailed symbol information from a package.

- **go_get_call_hierarchy**: Get the call hierarchy for a function using semantic location (symbol name, package, scope).

### Analysis & Refactoring

- **go_get_dependency_graph**: Get the dependency graph for a package.

- **go_dryrun_rename_symbol**: Preview a symbol rename operation across all files (DRY RUN - no changes are applied).

### Verification

- **go_build_check**: Check for compilation and type errors.

### Meta


<!-- Marker: AUTO-GEN-END -->

<!-- To regenerate: cd gopls/mcpbridge/core && go generate -->

## 📚 Example Conversations (Few-Shot Learning)

### Example 1: Finding Where a Function is Defined
```
❌ BAD INTERACTION:
User: "Where is processRequest defined?"
Assistant: [Uses Grep] → grep -rn "func processRequest" .

✅ GOOD INTERACTION:
User: "Where is processRequest defined?"
Assistant: [Uses go_definition]
go_definition(
    symbol_name="processRequest",
    context_file="internal/handlers/request.go"
)
→ Returns: Definition at request.go:42 with signature and docs
```

### Example 2: Finding All Usages
```
❌ BAD INTERACTION:
User: "Show me all places that call Start()"
Assistant: [Uses Grep] → grep -rn "Start(" .

✅ GOOD INTERACTION:
User: "Show me all places that call Start()"
Assistant: [Uses go_symbol_references]
go_symbol_references(
    symbol_name="Start",
    parent_scope="Server",
    kind="method"
)
→ Returns: 3 usages with file:line:column and code context
```

### Example 3: Understanding a Package
```
❌ BAD INTERACTION:
User: "What's in the handlers package?"
Assistant: [Reads all files] → Read handlers.go, Read middleware.go, Read ...

✅ GOOD INTERACTION:
User: "What's in the handlers package?"
Assistant: [Uses go_list_package_symbols]
go_list_package_symbols(
    package_path="github.com/yourproject/handlers",
    include_docs=true
)
→ Returns: All exported symbols with signatures and docs
```

### Example 4: Searching for Text Content (WRONG TOOL)
```
❌ BAD INTERACTION:
User: "Find all occurrences of 'TODO:' in comments"
Assistant: [Attempts to use go_search]
go_search(query="TODO:")
→ Returns: "No symbols found" (Because go_search doesn't search comments!)

❌ ANOTHER BAD INTERACTION:
User: "Find the string 'error:' in log messages"
Assistant: [Attempts to use go_search]
go_search(query="error:")
→ Returns: "No symbols found" (Because go_search doesn't search string literals!)

✅ CORRECT APPROACH:
User: "Find all 'TODO:' comments"
Assistant: [Uses grep or IDE search - go_search is NOT for text]
// Use: grep -r "TODO:" ./... or IDE's Ctrl+F search functionality
```

### Example 5: Finding Related Functions (CORRECT USAGE)
```
❌ BAD INTERACTION (Natural Language Search):
User: "Find the function that handles parsing of integer values"
Assistant: [Uses go_search incorrectly]
go_search(query="function that handles parsing of integer values")
→ Returns: "No symbols found" (Because no symbol is named that sentence)

✅ GOOD INTERACTION (Symbol Search):
User: "Find the function that handles parsing of integer values"
Assistant: [Uses go_search with a likely keyword]
go_search(query="ParseInt")
→ Returns: ParseInt, ParseInt64, TryParseInt
```

### Example 6: Finding Struct Fields (Default Search)
```
User: "Find the 'port' field in Server struct"
Assistant: [Uses go_search with default mode]
go_search(query="port")
→ Returns:
  - field port (in Server) at server.go:15
  - param port (in function dial) at net.go:123

Note: Struct fields and parameters are included by default.
```

### Example 7: Searching for Common Symbol Names

When searching for common names that might appear in many places, `go_search` returns all matches:

```
User: "Find where symbol 'count' is used"
Assistant: [Uses go_search with default mode]
go_search(query="count")
→ Returns:
  - field count (in Counter) at counter.go:10
  - param count (in function process) at handler.go:42
  - (Note: Local variables are NOT included - use go_read_file to inspect function bodies)
```

**Important:** `go_search` does NOT include local variables in function bodies. To find local variables, read the specific file with `go_read_file` and search for the variable name manually.