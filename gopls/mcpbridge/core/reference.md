---
title: Reference
sidebar:
  order: 1
---

## About These Tools

The gopls-mcp server provides **semantic analysis tools** powered by gopls (the Go language server).
These tools are type-aware, fast, and token-efficient compared to text-based alternatives like grep.

### Why Use Semantic Tools?

| Approach | Problem | Solution |
|----------|---------|----------|
| grep/ripgrep | Text matching, no semantic understanding | Semantic tools understand types, scopes, and interfaces |
| Manual file reading | High token cost, attention dilution | Targeted queries return only what you need |
| go build | Slow code generation | Incremental type checking (~500x faster) |

### Key Concepts

- **Symbol Locator**: Most tools use symbol_name + context_file to identify symbols semantically
- **JSON Schema**: Each tool's input schema is available via the MCP protocol (not duplicated here)
- **Tool Relationships**: Tools cross-reference each other - see "See also" sections

---

### `go_list_modules`

> List current module and direct dependencies only. Returns module paths only (no packages). By default, transitive dependencies are excluded. Set direct_only=false to show all dependencies including transitive ones. Use this to understand the module structure before exploring packages.

List current module and direct dependencies only.

**When to use**: Understanding module structure before exploring packages.

**Use this instead of**: Reading go.mod manually.

**Note**: By default excludes transitive dependencies. Set direct_only=false to include all.

**See also**: go_list_module_packages for discovering packages within a module.


### `go_list_module_packages`

> List all packages in a given module. Returns package names and optionally documentation. Use this to discover packages within a module before exploring symbols.

List all packages in a given module.

**When to use**: Discovering packages within a module before exploring symbols.

**Use this instead of**: Running find/ls and reading files manually.

**See also**: go_list_package_symbols for exploring symbols within a package.


### `go_list_package_symbols`

> List all exported symbols (types, functions, constants, variables) in a package. Returns Symbol objects with name, kind, signature, receiver, documentation, and optional bodies. Use include_docs=true for documentation and include_bodies=true for function implementations. Use this to explore a package's API surface before diving into specific symbols with get_package_symbol_detail.

List all exported symbols (types, functions, constants, variables) in a package.

**When to use**: Exploring a package's API surface before diving into specific symbols.

**Use this instead of**: Reading entire package files to find what's exported.

**Output**: Returns Symbol objects with name, kind, signature, receiver, documentation. Use include_bodies=true for implementations.

**See also**: go_get_package_symbol_detail for specific symbols, go_search for finding symbols by name.


### `go_get_package_symbol_detail`

> Get detailed symbol information from a package. Returns Symbol objects with name, kind, signature, receiver (for methods), parent (for fields), documentation, and optional bodies. Symbol filters are REQUIRED - provide symbol_filters to retrieve specific symbols by name and receiver (e.g., [{name: "Start", receiver: "*Server"}]). For methods, receiver matching uses exact string match (e.g., "*Server" != "Server"). Use include_docs=true for documentation and include_bodies=true for function implementations. Use list_package_symbols to get all symbols in a package.

Get detailed symbol information from a package.

**When to use**: Retrieving specific symbol details (signatures, docs, bodies) after discovering them with go_list_package_symbols.

**Required**: symbol_filters parameter to specify which symbols to retrieve (e.g., [{name: "Start", receiver: "*Server"}]).

**Note**: For methods, receiver matching is exact (e.g., "*Server" != "Server").

**See also**: go_definition for jumping to definitions while reading code.


### `go_build_check`

> Check for compilation and type errors. FAST: uses incremental type checking (faster than 'go build'). Use this to verify code correctness and populate the workspace cache for other tools. Returns detailed error information with file/line/column.

Check for compilation and type errors using incremental type checking.

**When to use**: Verifying code correctness after making changes.

**Use this instead of**: Running "go build" (this is ~500x faster by skipping code generation).

**Output**: Detailed error information with file/line/column.

**Note**: This also populates the workspace cache for faster subsequent tool calls.


### `go_search`

> Find symbols (functions, types, constants) by name with fuzzy matching. Use this when user knows part of a symbol name but not the full name or location. Returns rich symbol information (name, kind, file, line) for fast exploration.

Find symbols (functions, types, constants) by name with fuzzy matching.

**When to use**: You know part of a symbol's NAME (identifier) but not the full name or location.

**Critical - This ONLY searches symbol names**:
- ✅ Searches for identifier names: "formatSymbol", "Diag", "Server"
- ❌ NOT for code patterns, phrases, or concepts
- ❌ NOT for signatures like "func PackageDiagnostics"
- ❌ NOT for descriptions like "diagnostic deduplicate logic"

**Use this instead of**: Grep/ripgrep when searching for symbol identifiers by name.

**Output**: Symbol name, kind, file, line. Does NOT include signature/docs/body (use go_definition for those).

**Example**: Searching "formatSymbol" matches formatPackageSymbols, formatPackageSymbolDetail, FormatSymbolSummary.

**See also**: go_definition for full details, go_list_package_symbols for exploring all symbols in a package.


### `go_symbol_references`

> Find all usages of a symbol across the codebase using semantic location (symbol name, package, scope). Use this before refactoring to assess impact or to understand how a symbol is used. REPLACES: grep + manual file reading for finding references.

Find all usages of a symbol across the codebase.

**When to use**: Before refactoring to assess impact, or understanding how a symbol is used.

**Use this instead of**: Grep + manual file reading for finding references.

**Input**: Use semantic locator (symbol_name + context_file). The context_file is where you see the symbol used.

**Output**: Reference locations (file, line, column) plus rich symbol information.

**See also**: go_dryrun_rename_symbol to preview rename operations.


### `go_dryrun_rename_symbol`

> Preview a symbol rename operation across all files (DRY RUN - no changes are applied). Use go_symbol_references first to assess impact, then use this to preview the exact changes that would be made. Returns a unified diff showing all proposed modifications.

Preview a symbol rename operation across all files (DRY RUN).

**When to use**: Before renaming to see exact changes that will be made.

**Output**: Unified diff showing all proposed modifications.

**Workflow**: Use go_symbol_references first to assess impact, then this to preview changes.

**Note**: This is a dry run - no changes are applied.


### `go_implementation`

> Find all implementations of an interface or all interfaces implemented by a type using semantic location (symbol name, package, scope). Use this to understand type hierarchies, find all implementations of an interface, or discover design patterns in the codebase. REPLACES: grep + manual file reading for interface implementations.

Find all implementations of an interface or all interfaces implemented by a type.

**When to use**: Understanding type hierarchies, finding all implementations of an interface, discovering design patterns.

**Use this instead of**: Grep + manual file reading for interface implementations.

**Common pitfalls**:
- Standard library interfaces (io.Reader, error, fmt.Stringer) are not supported
- context_file should point to the definition, not usage
- For methods, set parent_scope to the interface name
- Empty result may mean no implementations exist (not an error)

**See also**: go_symbol_references for finding usages.


### `go_definition`

> Jump to the definition of a symbol using semantic location (symbol name, package, scope). REPLACES: grep + manual file reading. Use this when you see a function call or type reference and need to find where it's defined. Faster and more accurate than text search - uses type information from gopls.

Jump to the definition of a symbol.

**When to use**: You see a function call or type reference and need to find where it's defined.

**Use this instead of**: Grep + manual file reading.

**Input**: Use semantic locator (symbol_name + context_file where you see the symbol).

**See also**: go_get_package_symbol_detail for exploring package APIs.


### `go_get_call_hierarchy`

> Get the call hierarchy for a function using semantic location (symbol name, package, scope). Returns both incoming calls (what functions call this one) and outgoing calls (what functions this one calls). Use this to understand code flow, debug call chains, and trace execution paths through the codebase. REPLACES: grep + manual file reading for call graph analysis.

Get the call hierarchy for a function.

**When to use**: Understanding code flow, debugging call chains, tracing execution paths.

**Use this instead of**: Grep + manual file reading for call graph analysis.

**Direction**: "incoming" (what calls this), "outgoing" (what this calls), or "both".

**See also**: go_symbol_references for finding usages.


### `go_analyze_workspace`

> Analyze the entire workspace to discover packages, entry points, and dependencies. Use this when exploring a new codebase to understand the project structure, find main packages, API endpoints, and get a comprehensive overview of the codebase.

Analyze the entire workspace to discover packages, entry points, and dependencies.

**When to use**: First encountering a new codebase and want a comprehensive overview.

**Output**: High-level map of packages, entry points, and dependencies.

**See also**: go_get_started for a beginner-friendly guide.


### `go_get_started`

> Get a beginner-friendly guide to start exploring the Go project. Returns project identity, quick stats, entry points, package categories, and recommended next steps. Use this when you're new to a codebase and want to understand where to start.

Get a beginner-friendly guide to start exploring the Go project.

**When to use**: New to a codebase and want to understand where to start.

**Output**: Project identity, quick stats, entry points, package categories, recommended next steps.


### `go_get_dependency_graph`

> Get the dependency graph for a package. Returns both dependencies (packages it imports) and dependents (packages that import it). Use this to understand architectural relationships, analyze coupling, and visualize the package's place in the codebase.

Get the dependency graph for a package.

**When to use**: Understanding architectural relationships, analyzing coupling, visualizing the package's place in the codebase.

**Output**: Both dependencies (what it imports) and dependents (what imports it).

**See also**: go_list_modules for understanding module structure.


