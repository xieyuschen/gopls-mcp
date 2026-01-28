package core

//go:generate go run generate.go

import (
	"fmt"
	"io"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/api"
)

// Tool name constants
const (
	// Module and package discovery tools
	ToolListModules        = "go_list_modules"
	ToolListModulePackages = "go_list_module_packages"
	ToolListPackageSymbols = "go_list_package_symbols"

	// Integrated gopls MCP tools
	ToolGetPackageSymbolDetail = "go_get_package_symbol_detail"
	ToolGoBuildCheck           = "go_build_check"
	ToolGoSearch               = "go_search"
	ToolGoSymbolReferences     = "go_symbol_references"
	ToolGoDryrunRenameSymbol   = "go_dryrun_rename_symbol"
	ToolGoImplementation       = "go_implementation"
	ToolGoReadFile             = "go_read_file"
	ToolGoDefinition           = "go_definition"

	// Call hierarchy tools
	ToolGetCallHierarchy = "go_get_call_hierarchy"

	// Discovery tools
	ToolAnalyzeWorkspace   = "go_analyze_workspace"
	ToolGetStarted         = "go_get_started"
	ToolGetDependencyGraph = "go_get_dependency_graph"

	// Meta-tool
	ToolListTools = "go_list_tools"
)

// tools is the registry of all MCP tools provided by goplsmcp.
// This includes both the original gopls-mcp tools and the integrated gopls MCP tools.
// Note: list_tools is handled separately to avoid initialization cycles.
var tools = []Tool{
	// ===== Original gopls-mcp Tools =====
	// Module and package discovery tools
	GenericTool[api.IListModules, *api.OListModules]{
		Name:        ToolListModules,
		Description: "List current module and direct dependencies only. Returns module paths only (no packages). By default, transitive dependencies are excluded. Set direct_only=false to show all dependencies including transitive ones. Use this to understand the module structure before exploring packages.",
		Handler:     handleListModules, // implemented in handlers.go
	},
	GenericTool[api.IListModulePackages, *api.OListModulePackages]{
		Name:        ToolListModulePackages,
		Description: "List all packages in a given module. Returns package names and optionally documentation. Use this to discover packages within a module before exploring symbols.",
		Handler:     handleListModulePackages, // implemented in handlers.go
	},
	GenericTool[api.IListPackageSymbols, *api.OListPackageSymbols]{
		Name:        ToolListPackageSymbols,
		Description: "List all exported symbols (types, functions, constants, variables) in a package. Returns Symbol objects with name, kind, signature, receiver, documentation, and optional bodies. Use include_docs=true for documentation and include_bodies=true for function implementations. Use this to explore a package's API surface before diving into specific symbols with get_package_symbol_detail.",
		Handler:     handleListPackageSymbols, // implemented in handlers.go
	},

	// ===== Integrated gopls MCP Tools =====
	// These wrap the existing implementations from gopls/internal/mcp
	// Origin: gopls/internal/mcp/*.go

	GenericTool[api.IGetPackageSymbolDetailParams, *api.OGetPackageSymbolDetailResult]{
		Name:        ToolGetPackageSymbolDetail,
		Description: "Get detailed symbol information from a package. Returns Symbol objects with name, kind, signature, receiver (for methods), parent (for fields), documentation, and optional bodies. Symbol filters are REQUIRED - provide symbol_filters to retrieve specific symbols by name and receiver (e.g., [{name: \"Start\", receiver: \"*Server\"}]). For methods, receiver matching uses exact string match (e.g., \"*Server\" != \"Server\"). Use include_docs=true for documentation and include_bodies=true for function implementations. Use list_package_symbols to get all symbols in a package.",
		Handler:     handleGetPackageSymbolDetail, // wrapper for outlineHandler()
	},

	GenericTool[api.IDiagnosticsParams, *api.ODiagnosticsResult]{
		Name:        ToolGoBuildCheck,
		Description: "Check for compilation and type errors. FAST: uses incremental type checking (faster than 'go build'). Use this to verify code correctness and populate the workspace cache for other tools. Returns detailed error information with file/line/column.",
		Handler:     handleGoDiagnostics, // wrapper for workspaceDiagnosticsHandler()
	},

	GenericTool[api.ISearchParams, *api.OSearchResult]{
		Name:        ToolGoSearch,
		Description: "Find symbols (functions, types, constants) by name with fuzzy matching. Use this when user knows part of a symbol name but not the full name or location. Returns rich symbol information (name, kind, file, line) for fast exploration.",
		Handler:     handleGoSearch, // wrapper for searchHandler()
	},

	GenericTool[api.ISymbolReferencesParams, *api.OSymbolReferencesResult]{
		Name:        ToolGoSymbolReferences,
		Description: "Find all usages of a symbol across the codebase using semantic location (symbol name, package, scope). Use this before refactoring to assess impact or to understand how a symbol is used. REPLACES: grep + manual file reading for finding references.",
		Handler:     handleGoSymbolReferences, // uses semantic bridge (resolveSymbolLocatorToPosition)
	},

	GenericTool[api.IRenameSymbolParams, *api.ORenameSymbolResult]{
		Name:        ToolGoDryrunRenameSymbol,
		Description: "Preview a symbol rename operation across all files (DRY RUN - no changes are applied). Use go_symbol_references first to assess impact, then use this to preview the exact changes that would be made. Returns a unified diff showing all proposed modifications.",
		Handler:     handleGoRenameSymbol, // wrapper for renameSymbolHandler()
	},

	// todo: let's rethink about the location, can LLM give us a correct location?
	// or we can think about what's better.
	// todo: what happened if the location symbol is not an interface?
	GenericTool[api.IImplementationParams, *api.OImplementationResult]{
		Name:        ToolGoImplementation,
		Description: "Find all implementations of an interface or all interfaces implemented by a type using semantic location (symbol name, package, scope). Use this to understand type hierarchies, find all implementations of an interface, or discover design patterns in the codebase. REPLACES: grep + manual file reading for interface implementations.",
		Handler:     handleGoImplementation, // uses semantic bridge (golang.LLMImplementation)
	},

	// todo: is it necessary to support partially read?
	// e.g., read first N lines, or read from line X to line Y,
	// it will greatly reduce token cost and attention dilution.
	GenericTool[api.IReadFileParams, *api.OReadFileResult]{
		Name:        ToolGoReadFile,
		Description: "Read file content through gopls. SLOWER: reads full file from disk. Use this when you need to see actual code or implementation details. Note: unsaved editor changes not included.",
		Handler:     handleGoReadFile, // wrapper for snapshot.ReadFile()
	},

	// Navigation tools
	GenericTool[api.IDefinitionParams, *api.ODefinitionResult]{
		Name: ToolGoDefinition,
		// TODO: description is too long, don't use one line.
		Description: "Jump to the definition of a symbol using semantic location (symbol name, package, scope). REPLACES: grep + manual file reading. Use this when you see a function call or type reference and need to find where it's defined. Faster and more accurate than text search - uses type information from gopls.",
		Handler:     handleGoDefinition, // wrapper for golang.Definition()
	},

	// ===== Call Hierarchy Tools =====

	GenericTool[api.ICallHierarchyParams, *api.OCallHierarchyResult]{
		Name:        ToolGetCallHierarchy,
		Description: "Get the call hierarchy for a function using semantic location (symbol name, package, scope). Returns both incoming calls (what functions call this one) and outgoing calls (what functions this one calls). Use this to understand code flow, debug call chains, and trace execution paths through the codebase. REPLACES: grep + manual file reading for call graph analysis.",
		Handler:     handleGoCallHierarchy, // uses semantic bridge (golang.ResolveNode)
	},

	// ===== New Discovery Tools =====

	GenericTool[api.IAnalyzeWorkspaceParams, *api.OAnalyzeWorkspaceResult]{
		Name:        ToolAnalyzeWorkspace,
		Description: "Analyze the entire workspace to discover packages, entry points, and dependencies. Use this when exploring a new codebase to understand the project structure, find main packages, API endpoints, and get a comprehensive overview of the codebase.",
		Handler:     handleAnalyzeWorkspace, // new tool for workspace discovery
	},

	GenericTool[api.IGetStarted, *api.OGetStarted]{
		Name:        ToolGetStarted,
		Description: "Get a beginner-friendly guide to start exploring the Go project. Returns project identity, quick stats, entry points, package categories, and recommended next steps. Use this when you're new to a codebase and want to understand where to start.",
		Handler:     handleGetStarted, // new tool for getting started
	},

	// ===== Dependency Analysis Tools =====

	GenericTool[api.IDependencyGraphParams, *api.ODependencyGraphResult]{
		Name:        ToolGetDependencyGraph,
		Description: "Get the dependency graph for a package. Returns both dependencies (packages it imports) and dependents (packages that import it). Use this to understand architectural relationships, analyze coupling, and visualize the package's place in the codebase.",
		Handler:     handleGetDependencyGraph, // new tool for dependency graph analysis
	},
}

// RegisterTools registers all tools with the MCP server.
// The handler provides access to gopls's session and snapshot for all tool implementations.
// Integration point: called from gopls/internal/cmd/mcp.go or gopls/internal/mcp/mcp.go
func RegisterTools(server *mcp.Server, handler *Handler) {
	// Register the list_tools meta-tool first (special case to avoid init cycle)
	GenericTool[api.IListToolsParams, *api.OListToolsResult]{
		Name:        ToolListTools,
		Description: "List all available gopls-mcp tools with documentation and parameter schemas. These semantic analysis tools are more accurate (type-aware vs text matching), faster after warm-up (17x-587x per operation, breaks even after 3 queries), and save tokens compared to text-based search by leveraging gopls's cached type information.",
		Handler:     handleListTools,
	}.Register(server, handler)

	// Register all other tools
	for _, tool := range getTools() {
		tool.Register(server, handler)
	}
}

// GenerateReference writes the complete tool reference documentation to the provided writer.
// This is called by generate.go via go:generate and by tests for validation.
// It always writes the full documentation from scratch.
func GenerateReference(w io.Writer) error {
	// Write header and section

	fmt.Fprintf(w, `---
title: Reference
sidebar:
  order: 1
---
`)

	// Write each tool as a ### title with description
	for _, tool := range tools {
		name, des := tool.Details()
		fmt.Fprintf(w, "### `%s`\n\n", name)
		fmt.Fprintf(w, "> %s\n\n", des)
		fmt.Fprintf(w, "%s\n\n", tool.Docs())
	}

	return nil
}

// UpdateReference updates the reference.md with the current tool list.
// This is called by generate.go via go:generate.
// Deprecated: Use GenerateReference with io.Writer directly.
func UpdateReference(content string) string {
	var builder strings.Builder
	GenerateReference(&builder)
	return builder.String()
}

// Note: Tool handler implementations are in handlers.go
// to keep this file focused on registration and metadata.
