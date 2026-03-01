package core

// docMap maps tool names to their documentation.
// This centralizes all tool documentation in one place.
//
// Documentation style guide:
// - Focus on WHEN to use (vs alternatives)
// - Keep examples minimal (LLM has full JSON schema from MCP)
// - Remove boilerplate - philosophy is in the shared preamble
// - Cross-reference related tools
// - Only document non-obvious behavior/gotchas
var docMap = map[string]string{
	ToolListModules: `List current module and direct dependencies only.

**When to use**: Understanding module structure before exploring packages.

**Use this instead of**: Reading go.mod manually.

**Note**: By default excludes transitive dependencies. Set direct_only=false to include all.

**See also**: go_list_module_packages for discovering packages within a module.
`,

	ToolListModulePackages: `List all packages in a given module.

**When to use**: Discovering packages within a module before exploring symbols.

**Use this instead of**: Running find/ls and reading files manually.

**See also**: go_list_package_symbols for exploring symbols within a package.
`,

	ToolListPackageSymbols: `List all exported symbols (types, functions, constants, variables) in a package.

**When to use**: Exploring a package's API surface before diving into specific symbols.

**Use this instead of**: Reading entire package files to find what's exported.

**Output**: Returns Symbol objects with name, kind, signature, receiver, documentation. Use include_bodies=true for implementations.

**See also**: go_get_package_symbol_detail for specific symbols, go_search for finding symbols by name.
`,

	ToolGetPackageSymbolDetail: `Get detailed symbol information from a package.

**When to use**: Retrieving specific symbol details (signatures, docs, bodies) after discovering them with go_list_package_symbols.

**Required**: symbol_filters parameter to specify which symbols to retrieve (e.g., [{name: "Start", receiver: "*Server"}]).

**Note**: For methods, receiver matching is exact (e.g., "*Server" != "Server").

**See also**: go_definition for jumping to definitions while reading code.
`,

	ToolGoBuildCheck: `Check for compilation and type errors using incremental type checking.

**When to use**: Verifying code correctness after making changes.

**Use this instead of**: Running "go build" (this is ~500x faster by skipping code generation).

**Output**: Detailed error information with file/line/column.

**Note**: This also populates the workspace cache for faster subsequent tool calls.
`,

	ToolGoSearch: `Find symbols (functions, types, constants) by name with fuzzy matching.

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
`,

	ToolGoSymbolReferences: `Find all usages of a symbol across the codebase.

**When to use**: Before refactoring to assess impact, or understanding how a symbol is used.

**Use this instead of**: Grep + manual file reading for finding references.

**Input**: Use semantic locator (symbol_name + context_file). The context_file is where you see the symbol used.

**Output**: Reference locations (file, line, column) plus rich symbol information.

**See also**: go_dryrun_rename_symbol to preview rename operations.
`,

	ToolGoDryrunRenameSymbol: `Preview a symbol rename operation across all files (DRY RUN).

**When to use**: Before renaming to see exact changes that will be made.

**Output**: Unified diff showing all proposed modifications.

**Workflow**: Use go_symbol_references first to assess impact, then this to preview changes.

**Note**: This is a dry run - no changes are applied.
`,

	ToolGoImplementation: `Find all implementations of an interface or all interfaces implemented by a type.

**When to use**: Understanding type hierarchies, finding all implementations of an interface, discovering design patterns.

**Use this instead of**: Grep + manual file reading for interface implementations.

**Common pitfalls**:
- Standard library interfaces (io.Reader, error, fmt.Stringer) are not supported
- context_file should point to the definition, not usage
- For methods, set parent_scope to the interface name
- Empty result may mean no implementations exist (not an error)

**See also**: go_symbol_references for finding usages.
`,

	ToolGoDefinition: `Jump to the definition of a symbol.

**When to use**: You see a function call or type reference and need to find where it's defined.

**Use this instead of**: Grep + manual file reading.

**Input**: Use semantic locator (symbol_name + context_file where you see the symbol).

**See also**: go_get_package_symbol_detail for exploring package APIs.
`,

	ToolGetCallHierarchy: `Get the call hierarchy for a function.

**When to use**: Understanding code flow, debugging call chains, tracing execution paths.

**Use this instead of**: Grep + manual file reading for call graph analysis.

**Direction**: "incoming" (what calls this), "outgoing" (what this calls), or "both".

**See also**: go_symbol_references for finding usages.
`,

	ToolAnalyzeWorkspace: `Analyze the entire workspace to discover packages, entry points, and dependencies.

**When to use**: First encountering a new codebase and want a comprehensive overview.

**Output**: High-level map of packages, entry points, and dependencies.

**See also**: go_get_started for a beginner-friendly guide.
`,

	ToolGetStarted: `Get a beginner-friendly guide to start exploring the Go project.

**When to use**: New to a codebase and want to understand where to start.

**Output**: Project identity, quick stats, entry points, package categories, recommended next steps.
`,

	ToolGetDependencyGraph: `Get the dependency graph for a package.

**When to use**: Understanding architectural relationships, analyzing coupling, visualizing the package's place in the codebase.

**Output**: Both dependencies (what it imports) and dependents (what imports it).

**See also**: go_list_modules for understanding module structure.
`,

	ToolListTools: `List all available semantic analysis tools with documentation.

**When to use**: Discovering available capabilities before analyzing code.

**Output**: Complete tool catalog with names, descriptions, and usage examples.

**Note**: Use this to discover what's available rather than falling back to text-based search.
`,
}
