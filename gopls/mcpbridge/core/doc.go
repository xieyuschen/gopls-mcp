package core

// docMap maps tool names to their documentation.
// This centralizes all tool documentation in one place.
//
// Documentation style guide:
// - Focus on WHEN to use (vs alternatives)
// - Keep examples minimal (LLM has full JSON schema from MCP)
// - Cross-reference related tools
// - Only document non-obvious behavior/gotchas
var docMap = map[string]string{
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

**See also**: go_symbol_references for finding usages of the same symbol.
`,

	ToolGetCallHierarchy: `Get the call hierarchy for a function.

**When to use**: Understanding code flow, debugging call chains, tracing execution paths.

**Use this instead of**: Grep + manual file reading for call graph analysis.

**Direction**: "incoming" (what calls this), "outgoing" (what this calls), or "both".

**See also**: go_symbol_references for finding usages.
`,

	ToolGetDependencyGraph: `Get the dependency graph for a package.

**When to use**: Understanding architectural relationships, analyzing coupling, visualizing the package's place in the codebase.

**Output**: Both dependencies (what it imports) and dependents (what imports it).
`,

	ToolListTools: `List all available semantic analysis tools with documentation.

**When to use**: Discovering available capabilities.

**Output**: Complete tool catalog with names, descriptions, and usage examples.
`,
}
