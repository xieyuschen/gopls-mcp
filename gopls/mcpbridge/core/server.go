package core

//go:generate go run generate.go

import (
	"fmt"
	"io"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/api"
)

// Tool name constants.
// gopls-mcp focuses on semantic analysis that grep/Read cannot replicate.
// Tools that overlap with the assistant's native file-reading capabilities
// (search, build check, package listing, project overview) have been removed
// to keep the surface area sharp.
const (
	// Navigation tools (semantic, type-aware)
	ToolGoDefinition       = "go_definition"
	ToolGoImplementation   = "go_implementation"
	ToolGoSymbolReferences = "go_symbol_references"
	ToolGetCallHierarchy   = "go_get_call_hierarchy"

	// Refactoring tools
	ToolGoDryrunRenameSymbol = "go_dryrun_rename_symbol"

	// Dependency analysis
	ToolGetDependencyGraph = "go_get_dependency_graph"

	// Meta-tool
	ToolListTools = "go_list_tools"
)

// tools is the registry of all MCP tools provided by goplsmcp.
// Note: list_tools is handled separately to avoid initialization cycles.
var tools = []Tool{
	GenericTool[api.ISymbolReferencesParams, *api.OSymbolReferencesResult]{
		Name:        ToolGoSymbolReferences,
		Description: "Find all usages of a symbol across the codebase using semantic location (symbol name, package, scope). Use this before refactoring to assess impact or to understand how a symbol is used. REPLACES: grep + manual file reading for finding references.",
		Handler:     handleGoSymbolReferences,
	},

	GenericTool[api.IRenameSymbolParams, *api.ORenameSymbolResult]{
		Name:        ToolGoDryrunRenameSymbol,
		Description: "Preview a symbol rename operation across all files (DRY RUN - no changes are applied). Use go_symbol_references first to assess impact, then use this to preview the exact changes that would be made. Returns a unified diff showing all proposed modifications.",
		Handler:     handleGoRenameSymbol,
	},

	GenericTool[api.IImplementationParams, *api.OImplementationResult]{
		Name:        ToolGoImplementation,
		Description: "Find all implementations of an interface or all interfaces implemented by a type using semantic location (symbol name, package, scope). Use this to understand type hierarchies, find all implementations of an interface, or discover design patterns in the codebase. REPLACES: grep + manual file reading for interface implementations.",
		Handler:     handleGoImplementation,
	},

	GenericTool[api.IDefinitionParams, *api.ODefinitionResult]{
		Name:        ToolGoDefinition,
		Description: "Jump to the definition of a symbol using semantic location (symbol name, package, scope). REPLACES: grep + manual file reading. Use this when you see a function call or type reference and need to find where it's defined. Faster and more accurate than text search - uses type information from gopls.",
		Handler:     handleGoDefinition,
	},

	GenericTool[api.ICallHierarchyParams, *api.OCallHierarchyResult]{
		Name:        ToolGetCallHierarchy,
		Description: "Get the call hierarchy for a function using semantic location (symbol name, package, scope). Returns both incoming calls (what functions call this one) and outgoing calls (what functions this one calls). Use this to understand code flow, debug call chains, and trace execution paths through the codebase. REPLACES: grep + manual file reading for call graph analysis.",
		Handler:     handleGoCallHierarchy,
	},

	GenericTool[api.IDependencyGraphParams, *api.ODependencyGraphResult]{
		Name:        ToolGetDependencyGraph,
		Description: "Get the dependency graph for a package. Returns both dependencies (packages it imports) and dependents (packages that import it). Use this to understand architectural relationships, analyze coupling, and visualize the package's place in the codebase.",
		Handler:     handleGetDependencyGraph,
	},
}

// RegisterTools registers all tools with the MCP server.
// The handler provides access to gopls's session and snapshot for all tool implementations.
func RegisterTools(server *mcp.Server, handler *Handler) int {
	// Register the list_tools meta-tool first (special case to avoid init cycle)
	GenericTool[api.IListToolsParams, *api.OListToolsResult]{
		Name:        ToolListTools,
		Description: "List all available gopls-mcp tools with documentation and parameter schemas. These semantic analysis tools are type-aware (vs grep text matching) and operate on gopls's cached type information.",
		Handler:     handleListTools,
	}.Register(server, handler)

	tools := getTools()
	for _, tool := range tools {
		tool.Register(server, handler)
	}
	return 1 + len(tools) // 1 for list_tools meta-tool + registered tools
}

// GenerateReference writes the complete tool reference documentation to the provided writer.
// This is called by generate.go via go:generate and by tests for validation.
func GenerateReference(w io.Writer) error {
	fmt.Fprintf(w, `---
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
go.mod, running ` + "`go build`" + `, fuzzy symbol search — your assistant can already do those
with its native tools. What it *cannot* do is resolve Go's type system, and that's what's here.

### Key Concepts

- **Symbol Locator**: Most tools use symbol_name + context_file to identify symbols semantically
- **JSON Schema**: Each tool's input schema is available via the MCP protocol (not duplicated here)
- **Tool Relationships**: Tools cross-reference each other - see "See also" sections

---

`)

	for _, tool := range tools {
		name, des := tool.Details()
		docs, err := tool.Docs()
		if err != nil {
			return fmt.Errorf("failed to get docs for tool %s: %w", name, err)
		}
		fmt.Fprintf(w, "### `%s`\n\n", name)
		fmt.Fprintf(w, "> %s\n\n", des)
		fmt.Fprintf(w, "%s\n\n", docs)
	}

	return nil
}

// GenerateCLAUDEToolReference generates a concise tool reference for CLAUDE.md.
// All gopls-mcp tools are exclusive semantic capabilities — there is no fallback,
// because grep/Read cannot do what these tools do.
func GenerateCLAUDEToolReference() (string, error) {
	var buf strings.Builder

	buf.WriteString("### Semantic tools (Exclusive - no grep/Read fallback)\n")
	buf.WriteString("| Task | Tool |\n")
	buf.WriteString("|------|------|\n")

	entries := []struct {
		task string
		tool string
	}{
		{"Find interface implementations", "go_implementation"},
		{"Trace call relationships", "go_get_call_hierarchy"},
		{"Find symbol references", "go_symbol_references"},
		{"Jump to definition", "go_definition"},
		{"Analyze dependencies", "go_get_dependency_graph"},
		{"Preview renaming", "go_dryrun_rename_symbol"},
	}

	for _, entry := range entries {
		fmt.Fprintf(&buf, "| %s | %s |\n", entry.task, entry.tool)
	}

	return buf.String(), nil
}

// UpdateReference updates the reference.md with the current tool list.
// Deprecated: Use GenerateReference with io.Writer directly.
func UpdateReference(content string) string {
	var builder strings.Builder
	GenerateReference(&builder)
	return builder.String()
}

// Note: Tool handler implementations are in handlers.go and gopls_wrappers.go
// to keep this file focused on registration and metadata.
