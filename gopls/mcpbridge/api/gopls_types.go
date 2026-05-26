// Package api provides input/output types for all gopls MCP tools.
package api

// ISymbolReferencesParams is the input for go_symbol_references tool.
type ISymbolReferencesParams struct {
	// Locator specifies the symbol to find references for.
	// This uses semantic information (symbol name, context file, package, scope)
	// instead of error-prone line/column numbers.
	Locator SymbolLocator `json:"locator" jsonschema:"semantic symbol locator (symbol_name, context_file, package_name, parent_scope, kind, line_hint)"`
}

// OSymbolReferencesResult is the output for go_symbol_references tool.
type OSymbolReferencesResult struct {
	Summary string `json:"summary" jsonschema:"symbol references summary"`
	// Symbols is the list of rich symbol information for each unique referenced symbol.
	// This provides signature, documentation, and snippet for each symbol.
	Symbols []*Symbol `json:"symbols,omitempty" jsonschema:"rich symbol information for each referenced symbol"`

	// TotalCount is the total number of references found.
	TotalCount int `json:"total_count,omitempty" jsonschema:"total number of references found"`
	// Returned is the number of references returned in this response.
	Returned int `json:"returned,omitempty" jsonschema:"number of references returned"`
	// Truncated indicates whether not all references were returned.
	Truncated bool `json:"truncated,omitempty" jsonschema:"whether the result was truncated due to size limits"`
	// Hint provides guidance when results are truncated.
	Hint string `json:"hint,omitempty" jsonschema:"suggestion for getting more details"`
}

// IRenameSymbolParams is the input for go_dryrun_rename_symbol tool.
type IRenameSymbolParams struct {
	// Locator specifies the symbol to rename.
	// This uses semantic information (symbol name, context file, package, scope, kind)
	// for precise symbol identification and disambiguation.
	Locator SymbolLocator `json:"locator" jsonschema:"semantic symbol locator (symbol_name, context_file, package_name, parent_scope, kind, line_hint)"`
	// NewName is the new name for the symbol.
	NewName string `json:"new_name" jsonschema:"the new name for the symbol"`
}

// ORenameSymbolResult is the output for go_dryrun_rename_symbol tool.
type ORenameSymbolResult struct {
	Summary string `json:"summary" jsonschema:"rename changes summary"`
	// Changes is a line-by-line diff format that's LLM-friendly.
	// Each change shows the complete old/new line content for easy verification and rewriting.
	Changes []RenameChange `json:"changes,omitempty" jsonschema:"line-by-line changes with full line content"`
}

// RenameChange represents a single line change in a rename operation.
// This is LLM-friendly: the complete line content allows LLMs to verify context
// and rewrite lines without calculating byte offsets.
type RenameChange struct {
	File string `json:"file" jsonschema:"file path"`
	Line int    `json:"line" jsonschema:"line number (1-indexed)"`
	// OldLine is the complete original line content.
	OldLine string `json:"old_line" jsonschema:"the complete content of the original line"`
	// NewLine is the complete new line content.
	NewLine string `json:"new_line" jsonschema:"the complete content of the new line"`
}

// IImplementationParams is the input for go_implementation tool.
type IImplementationParams struct {
	// Locator specifies the symbol to find implementations for.
	Locator SymbolLocator `json:"locator" jsonschema:"semantic symbol locator (symbol_name, context_file, package_name, parent_scope, kind, line_hint)"`
	// IncludeBody indicates whether to include the function body in the returned Symbols.
	IncludeBody bool `json:"include_body,omitempty" jsonschema:"whether to include function bodies in the returned Symbols (default: false)"`
}

// OImplementationResult is the output for go_implementation tool.
type OImplementationResult struct {
	// Symbols is the list of rich symbol information (name, kind, signature, docs, body, file_path, line).
	Symbols []*Symbol `json:"symbols,omitempty" jsonschema:"rich symbol information for each implementation"`
	// Summary is a human-readable summary of the results.
	Summary string `json:"summary" jsonschema:"implementation results summary"`
}

// IListToolsParams is the input for list_tools tool.
type IListToolsParams struct {
	// IncludeInputSchema indicates whether to include the JSON schema for input parameters.
	IncludeInputSchema bool `json:"includeInputSchema,omitempty" jsonschema:"whether to include input parameter schemas (default: false)"`
	// IncludeOutputSchema indicates whether to include the JSON schema for output parameters.
	IncludeOutputSchema bool `json:"includeOutputSchema,omitempty" jsonschema:"whether to include output parameter schemas (default: false)"`
	// CategoryFilter allows filtering tools by category.
	CategoryFilter string `json:"category_filter,omitempty" jsonschema:"filter tools by category (default: empty = all categories)"`
}

// ToolDocumentation represents documentation for a single MCP tool.
type ToolDocumentation struct {
	Name         string         `json:"name" jsonschema:"the tool name"`
	Description  string         `json:"description" jsonschema:"tool description"`
	InputSchema  map[string]any `json:"inputSchema,omitempty" jsonschema:"input parameter JSON schema"`
	OutputSchema map[string]any `json:"outputSchema,omitempty" jsonschema:"output parameter JSON schema"`
	Category     string         `json:"category,omitempty" jsonschema:"tool category for grouping"`
}

// OListToolsResult is the output for list_tools tool.
type OListToolsResult struct {
	Tools   []ToolDocumentation `json:"tools" jsonschema:"list of all available tools"`
	Count   int                 `json:"count" jsonschema:"total number of tools"`
	Summary string              `json:"summary" jsonschema:"tools summary"`
}

// IDefinitionParams is the input for go_definition tool.
type IDefinitionParams struct {
	// Locator is the semantic location of the symbol to find the definition for.
	Locator SymbolLocator `json:"locator" jsonschema:"the semantic location of the symbol (uses name, scope, kind instead of line/column)"`
	// IncludeBody indicates whether to include the function body in the returned Symbol.
	IncludeBody bool `json:"include_body,omitempty" jsonschema:"whether to include function body in the returned Symbol (default: false)"`
}

// ODefinitionResult is the output for go_definition tool.
type ODefinitionResult struct {
	// Symbol is the symbol at the definition location.
	Symbol *Symbol `json:"symbol,omitempty" jsonschema:"the symbol at the definition location"`
	// Summary is a human-readable summary of the result.
	Summary string `json:"summary" jsonschema:"definition result summary"`
}

// IDependencyGraphParams is the input for get_dependency_graph tool.
type IDependencyGraphParams struct {
	// PackagePath is the package import path (e.g., "net/http").
	// If empty, analyzes the main module's root package.
	PackagePath string `json:"package_path,omitempty" jsonschema:"the package import path (default: main module root)"`
	// Cwd is the current working directory (used to locate go.mod and project context).
	Cwd string `json:"Cwd,omitempty" jsonschema:"the current working directory to find the go.mod file (default: session view)"`
	// IncludeTransitive indicates whether to include transitive dependencies.
	IncludeTransitive bool `json:"include_transitive,omitempty" jsonschema:"whether to include transitive dependencies (default: false)"`
	// MaxDepth limits the depth of transitive dependency traversal.
	MaxDepth int `json:"max_depth,omitempty" jsonschema:"maximum depth for transitive dependencies (default: 0 = unlimited)"`
}

// ODependencyGraphResult is the output for get_dependency_graph tool.
type ODependencyGraphResult struct {
	PackagePath       string              `json:"package_path" jsonschema:"the analyzed package path"`
	PackageName       string              `json:"package_name" jsonschema:"the package name"`
	Dependencies      []PackageDependency `json:"dependencies,omitempty" jsonschema:"packages imported by this package"`
	Dependents        []PackageDependent  `json:"dependents,omitempty" jsonschema:"packages that import this package"`
	Summary           string              `json:"summary" jsonschema:"dependency graph summary"`
	TotalDependencies int                 `json:"total_dependencies,omitempty" jsonschema:"total number of dependencies"`
	TotalDependents   int                 `json:"total_dependents,omitempty" jsonschema:"total number of dependents"`
	Truncated         bool                `json:"truncated,omitempty" jsonschema:"whether results were truncated"`
}

// PackageDependency represents a package that is imported by the analyzed package.
type PackageDependency struct {
	Path       string `json:"path" jsonschema:"the package import path"`
	Name       string `json:"name,omitempty" jsonschema:"the package name"`
	ModulePath string `json:"module_path,omitempty" jsonschema:"the module path"`
	IsStdlib   bool   `json:"is_stdlib,omitempty" jsonschema:"is this a standard library package"`
	IsExternal bool   `json:"is_external,omitempty" jsonschema:"is this an external dependency"`
	Depth      int    `json:"depth,omitempty" jsonschema:"the dependency depth"`
}

// PackageDependent represents a package that imports the analyzed package.
type PackageDependent struct {
	Path       string `json:"path" jsonschema:"the package import path"`
	Name       string `json:"name,omitempty" jsonschema:"the package name"`
	ModulePath string `json:"module_path,omitempty" jsonschema:"the module path"`
	IsTest     bool   `json:"is_test,omitempty" jsonschema:"is this a test package"`
}

// ICallHierarchyParams is the input for get_call_hierarchy tool.
type ICallHierarchyParams struct {
	// Locator specifies the function to get call hierarchy for.
	Locator SymbolLocator `json:"locator" jsonschema:"semantic symbol locator (symbol_name, context_file, package_name, parent_scope, kind, line_hint)"`
	// Direction determines which direction to traverse: "incoming", "outgoing", or "both".
	Direction string `json:"direction,omitempty" jsonschema:"call hierarchy direction (incoming/outgoing/both, default: both)"`
	// Cwd optionally specifies the working directory for call hierarchy analysis.
	Cwd string `json:"Cwd,omitempty" jsonschema:"the working directory for call hierarchy analysis (default: use default view)"`
}

// OCallHierarchyResult is the output for get_call_hierarchy tool.
type OCallHierarchyResult struct {
	// Symbol is the symbol at the given position.
	Symbol Symbol `json:"symbol" jsonschema:"the symbol at the given position"`
	// IncomingCalls are the functions that call this function.
	IncomingCalls []CallHierarchyCall `json:"incoming_calls,omitempty" jsonschema:"functions that call this function"`
	// OutgoingCalls are the functions that this function calls.
	OutgoingCalls []CallHierarchyCall `json:"outgoing_calls,omitempty" jsonschema:"functions that this function calls"`
	// TotalIncoming is the total number of incoming calls.
	TotalIncoming int `json:"total_incoming,omitempty" jsonschema:"total number of incoming calls"`
	// TotalOutgoing is the total number of outgoing calls.
	TotalOutgoing int `json:"total_outgoing,omitempty" jsonschema:"total number of outgoing calls"`
	// Summary is a human-readable summary.
	Summary string `json:"summary" jsonschema:"call hierarchy summary"`
}

// CallHierarchyCall represents a call in the hierarchy.
type CallHierarchyCall struct {
	// From is the calling function (for incoming) or called function (for outgoing).
	From Symbol `json:"from" jsonschema:"the function in the call relationship"`
	// CallRanges are the locations where this call occurs (multiple if called multiple times).
	CallRanges []CallRange `json:"call_ranges,omitempty" jsonschema:"locations where this call occurs"`
}

// CallRange represents a location where a call occurs.
type CallRange struct {
	// File is the file path.
	File string `json:"file" jsonschema:"the file path"`
	// StartLine is the start line number (1-indexed).
	StartLine int `json:"start_line" jsonschema:"the start line number (1-indexed)"`
	// EndLine is the end line number (1-indexed).
	EndLine int `json:"end_line" jsonschema:"the end line number (1-indexed)"`
}
