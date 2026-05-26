---
title: Reference
sidebar:
  order: 1
---

## About These Tools

gopls-mcp provides **semantic analysis tools** powered by gopls (the Go language server).
These tools are type-aware: they understand interfaces, scopes, and type identity in ways
that grep and file reads cannot.

### Scope

gopls-mcp deliberately ships only what grep + Read cannot do well. Listing packages, reading
go.mod, running `go build`, fuzzy symbol search — your assistant can already do those
with its native tools. What it *cannot* do is resolve Go's type system, and that's what's here.

### Key Concepts

- **Symbol Locator**: Most tools use symbol_name + context_file to identify symbols semantically
- **JSON Schema**: Each tool's input schema is available via the MCP protocol (not duplicated here)
- **Tool Relationships**: Tools cross-reference each other - see "See also" sections

---

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

**See also**: go_symbol_references for finding usages of the same symbol.


### `go_get_call_hierarchy`

> Get the call hierarchy for a function using semantic location (symbol name, package, scope). Returns both incoming calls (what functions call this one) and outgoing calls (what functions this one calls). Use this to understand code flow, debug call chains, and trace execution paths through the codebase. REPLACES: grep + manual file reading for call graph analysis.

Get the call hierarchy for a function.

**When to use**: Understanding code flow, debugging call chains, tracing execution paths.

**Use this instead of**: Grep + manual file reading for call graph analysis.

**Direction**: "incoming" (what calls this), "outgoing" (what this calls), or "both".

**See also**: go_symbol_references for finding usages.


### `go_get_dependency_graph`

> Get the dependency graph for a package. Returns both dependencies (packages it imports) and dependents (packages that import it). Use this to understand architectural relationships, analyze coupling, and visualize the package's place in the codebase.

Get the dependency graph for a package.

**When to use**: Understanding architectural relationships, analyzing coupling, visualizing the package's place in the codebase.

**Output**: Both dependencies (what it imports) and dependents (what imports it).


