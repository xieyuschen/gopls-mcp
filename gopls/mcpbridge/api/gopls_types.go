// Package api provides input/output types for all gopls MCP tools.
// This includes both the original gopls-mcp tools and the integrated gopls MCP tools.
package api

// ===== Existing gopls-mcp Tool Types =====

// (Already defined in model.go - keeping this comment for reference)

// ===== Integrated gopls MCP Tool Types =====

// SymbolFilter specifies which symbols to retrieve from a package.
// This is a precision filter for the get_package_symbol_detail tool.
// Matching logic:
// - For methods: both Receiver and Name must match (exact string match)
// - For non-methods: only Name must match (Receiver is ignored)
//
// Receiver matching uses EXACT string match:
// - "*Server" matches only methods with receiver "*Server"
// - "Server" matches only methods with receiver "Server" (not "*Server")
// - It is the LLM's responsibility to provide the correct receiver string
type SymbolFilter struct {
	// Receiver is the type receiver for methods (optional).
	// Example: "*Server", "Client", "User"
	// Uses EXACT string match - "*" prefix matters!
	// If specified, only methods with this exact receiver are matched.
	Receiver string `json:"receiver,omitempty" jsonschema:"the receiver type for methods (e.g., *Server, exact string match)"`
	// Name is the symbol name to match (required).
	// Example: "Start", "HandleRequest", "Config"
	Name string `json:"name" jsonschema:"the symbol name to match"`
}

// IGetPackageSymbolDetailParams is the input for get_package_symbol_detail tool.
// This is a precision tool for retrieving specific symbol details.
// Use list_package_symbols for getting all symbols in a package.
type IGetPackageSymbolDetailParams struct {
	// PackagePath is the Go package import path (e.g., "net/http", "github.com/user/project").
	PackagePath string `json:"package_path" jsonschema:"the Go package import path"`
	// SymbolFilters filters which symbols to return (REQUIRED).
	// If empty or nil, an error is returned (filters are required for this precision tool).
	// If non-empty, only symbols matching the filters are returned.
	// Each filter specifies an optional receiver and required name.
	// Example: [{name: "Start", receiver: "*Server"}] to get Server.Start method only.
	SymbolFilters []SymbolFilter `json:"symbol_filters" jsonschema:"filters for specific symbols (REQUIRED - empty filters will return error)"`
	// IncludeDocs indicates whether to include symbol documentation.
	// When false (default), only symbol names and signatures are returned.
	// When true, full symbol documentation is included.
	IncludeDocs bool `json:"include_docs" jsonschema:"whether to include symbol documentation (default: false)"`
	// IncludeBodies indicates whether to include function bodies.
	// When false (default), only signatures are returned.
	// When true, full function implementations are included.
	IncludeBodies bool `json:"include_bodies" jsonschema:"whether to include function implementations (default: false)"`
	// Cwd optionally specifies the working directory for package resolution.
	// When set, creates/uses a view for that directory (useful for testing with temp directories).
	// When empty, searches all available views (normal usage).
	Cwd string `json:"Cwd,omitempty" jsonschema:"the working directory for package resolution (default: search all views)"`
}

// OGetPackageSymbolDetailResult is the output for get_package_symbol_detail tool.
type OGetPackageSymbolDetailResult struct {
	Symbols []Symbol `json:"symbols" jsonschema:"the symbols matching the provided filters"`
}

// IDiagnosticsParams is the input for go_build_check tool.
type IDiagnosticsParams struct {
	// todo: the input 'files' are weird, let's say LLM tries to verify the whole change,
	// how could it know which files are active files? this needs improvement.
	//
	// Files are absolute paths to active files for deeper diagnostics
	Files []string `json:"files,omitempty" jsonschema:"absolute paths to active files, if any"`
	// Cwd optionally specifies the working directory for diagnostics.
	// When set, creates/uses a view for that directory (useful for testing with temp directories).
	// When empty, uses the default view (normal usage).
	Cwd string `json:"Cwd,omitempty" jsonschema:"the working directory for diagnostics (default: use default view)"`
}

// ODiagnosticsResult is the output for go_build_check tool.
type ODiagnosticsResult struct {
	// TODO: is the Summary helpful and necessary?
	Summary     string       `json:"summary" jsonschema:"diagnostics summary"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty" jsonschema:"list of diagnostics found"`
}

// Diagnostic represents a single diagnostic message.
type Diagnostic struct {
	File     string `json:"file" jsonschema:"file path"`
	Severity string `json:"severity" jsonschema:"severity of the diagnostic"`
	Message  string `json:"message" jsonschema:"diagnostic message"`
	// Line and Column are retained for reference but unreliable for LLM navigation.
	// The CodeSnippet field below provides the actual source code context.
	Line   int `json:"line,omitempty" jsonschema:"line number (1-indexed)"`
	Column int `json:"column,omitempty" jsonschema:"column number (1-indexed)"`
	// CodeSnippet is the actual source line containing the error.
	// This is more reliable than line/column numbers for LLM understanding.
	CodeSnippet string `json:"code_snippet" jsonschema:"the source code line containing the diagnostic"`
}

// ISearchParams is the input for go_search tool.
type ISearchParams struct {
	Query string `json:"query" jsonschema:"the fuzzy search query to use for matching symbols"`
	// MaxResults limits the number of search results returned. If 0 or not set, defaults to 10.
	MaxResults int `json:"max_results,omitempty" jsonschema:"maximum number of results (default: 10, 0 = unlimited)"`
	// Cwd optionally specifies the working directory for symbol search.
	// When set, creates/uses a view for that directory (useful for testing with temp directories).
	// When empty, searches all available views (normal usage).
	Cwd string `json:"Cwd,omitempty" jsonschema:"the working directory for symbol search (default: search all views)"`
}

// OSearchResult is the output for go_search tool.
type OSearchResult struct {
	Summary string `json:"summary" jsonschema:"search results summary"`
	// Symbols is the list of matching symbols with rich information.
	Symbols []*Symbol `json:"symbols,omitempty" jsonschema:"matching symbols with signatures and documentation"`
}

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
	// This allows the LLM to verify it's modifying the correct location.
	OldLine string `json:"old_line" jsonschema:"the complete content of the original line"`
	// NewLine is the complete new line content.
	// LLMs excel at rewriting entire lines rather than calculating character offsets.
	NewLine string `json:"new_line" jsonschema:"the complete content of the new line"`
}

// IImplementationParams is the input for go_implementation tool.
type IImplementationParams struct {
	// Locator specifies the symbol to find implementations for.
	// This uses semantic information (symbol name, context file, package, scope)
	// instead of error-prone line/column numbers.
	Locator SymbolLocator `json:"locator" jsonschema:"semantic symbol locator (symbol_name, context_file, package_name, parent_scope, kind, line_hint)"`
	// IncludeBody indicates whether to include the function body in the returned Symbols.
	// When false (default), only signature and documentation are returned.
	// When true, the function body implementation is also included.
	IncludeBody bool `json:"include_body,omitempty" jsonschema:"whether to include function bodies in the returned Symbols (default: false)"`
}

// OImplementationResult is the output for go_implementation tool.
type OImplementationResult struct {
	// Symbols is the list of rich symbol information (name, kind, signature, docs, body, file_path, line).
	// Each Symbol already contains location information (FilePath, Line) plus semantic details.
	Symbols []*Symbol `json:"symbols,omitempty" jsonschema:"rich symbol information for each implementation"`
	// Summary is a human-readable summary of the results.
	Summary string `json:"summary" jsonschema:"implementation results summary"`
}

// IReadFileParams is the input for go_read_file tool.
type IReadFileParams struct {
	// File is the absolute path to the file to read.
	File string `json:"file" jsonschema:"the absolute path to the file"`
	// MaxBytes limits the number of bytes returned. If 0 or not set, returns all content.
	MaxBytes int `json:"max_bytes,omitempty" jsonschema:"maximum bytes to return (default: 0 = unlimited)"`
	// MaxLines limits the number of lines returned. If 0 or not set, returns all content.
	MaxLines int `json:"max_lines,omitempty" jsonschema:"maximum lines to return (default: 0 = unlimited)"`
	// Offset specifies the starting line number (1-indexed). If 0 or not set, starts from line 1.
	Offset int `json:"offset,omitempty" jsonschema:"starting line number (1-indexed, default: 1)"`
}

// OReadFileResult is the output for go_read_file tool.
type OReadFileResult struct {
	// Content is the file content (potentially truncated).
	// Note: This includes any unsaved changes (overlays) from the editor.
	Content string `json:"content" jsonschema:"the file content including unsaved changes (truncated if limits were specified)"`
	// TotalLines is the total number of lines in the file (for reference).
	TotalLines int `json:"total_lines,omitempty" jsonschema:"total number of lines in the file"`
	// TotalBytes is the total number of bytes in the file (for reference).
	TotalBytes int `json:"total_bytes,omitempty" jsonschema:"total number of bytes in the file"`
}

// IListToolsParams is the input for list_tools tool.
type IListToolsParams struct {
	// IncludeInputSchema indicates whether to include the JSON schema for input parameters.
	IncludeInputSchema bool `json:"includeInputSchema,omitempty" jsonschema:"whether to include input parameter schemas (default: false)"`
	// IncludeOutputSchema indicates whether to include the JSON schema for output parameters.
	IncludeOutputSchema bool `json:"includeOutputSchema,omitempty" jsonschema:"whether to include output parameter schemas (default: false)"`
	// CategoryFilter allows filtering tools by category (e.g., "analysis", "navigation", "information").
	// If empty, all categories are returned.
	CategoryFilter string `json:"category_filter,omitempty" jsonschema:"filter tools by category (default: empty = all categories)"`
}

// ToolDocumentation represents documentation for a single MCP tool.
type ToolDocumentation struct {
	// Name is the tool name.
	Name string `json:"name" jsonschema:"the tool name"`
	// Description is a human-readable description of what the tool does.
	Description string `json:"description" jsonschema:"tool description"`
	// InputSchema is the JSON schema for the input parameters (if requested).
	InputSchema map[string]any `json:"inputSchema,omitempty" jsonschema:"input parameter JSON schema"`
	// OutputSchema is the JSON schema for the output parameters (if requested).
	OutputSchema map[string]any `json:"outputSchema,omitempty" jsonschema:"output parameter JSON schema"`
	// Category is the tool category (e.g., "environment", "analysis", "navigation").
	Category string `json:"category,omitempty" jsonschema:"tool category for grouping"`
}

// OListToolsResult is the output for list_tools tool.
type OListToolsResult struct {
	// Tools is the list of all available MCP tools with their documentation.
	Tools []ToolDocumentation `json:"tools" jsonschema:"list of all available tools"`
	// Count is the total number of tools available.
	Count int `json:"count" jsonschema:"total number of tools"`
	// Summary is a human-readable summary.
	Summary string `json:"summary" jsonschema:"tools summary"`
}

// IDefinitionParams is the input for go_definition tool.
type IDefinitionParams struct {
	// Locator is the semantic location of the symbol to find the definition for.
	// This uses stable semantic anchors (Names, Scopes, Kinds) instead of error-prone line/column numbers.
	Locator SymbolLocator `json:"locator" jsonschema:"the semantic location of the symbol (uses name, scope, kind instead of line/column)"`
	// IncludeBody indicates whether to include the function body in the returned Symbol.
	// When false (default), only signature and documentation are returned.
	// When true, the function body implementation is also included.
	IncludeBody bool `json:"include_body,omitempty" jsonschema:"whether to include function body in the returned Symbol (default: false)"`
}

// ODefinitionResult is the output for go_definition tool.
type ODefinitionResult struct {
	// Symbol is the symbol at the definition location.
	// Includes name, kind, signature, file, line, documentation, and optionally the body.
	// Note: Line number is provided (column omitted as code snippet is included).
	Symbol *Symbol `json:"symbol,omitempty" jsonschema:"the symbol at the definition location"`
	// Summary is a human-readable summary of the result.
	Summary string `json:"summary" jsonschema:"definition result summary"`
}

// IAnalyzeWorkspaceParams is the input for analyze_workspace tool.
type IAnalyzeWorkspaceParams struct {
	// Cwd is the directory to analyze (uses session view if empty).
	Cwd string `json:"Cwd,omitempty" jsonschema:"directory to analyze (default: session view)"`
}

// EntryPoint represents a code entry point discovered during workspace analysis.
type EntryPoint struct {
	// Type is the type of entry point (e.g., "main", "test", "api").
	Type string `json:"type" jsonschema:"the type of entry point"`
	// Name is the name of the entry point.
	Name string `json:"name" jsonschema:"the name of the entry point"`
	// File is the file path.
	File string `json:"file" jsonschema:"the file path"`
	// Description provides additional context.
	Description string `json:"description,omitempty" jsonschema:"description of the entry point"`
}

// OAnalyzeWorkspaceResult is the output for analyze_workspace tool.
type OAnalyzeWorkspaceResult struct {
	// ModulePath is the module path.
	ModulePath string `json:"module_path" jsonschema:"the module path"`
	// Packages is the list of packages discovered.
	Packages []WorkspacePackage `json:"packages" jsonschema:"packages discovered"`
	// EntryPoints are the discovered entry points.
	EntryPoints []EntryPoint `json:"entry_points" jsonschema:"discovered entry points"`
	// Dependencies are the external dependencies.
	Dependencies []Module `json:"dependencies,omitempty" jsonschema:"external dependencies"`
	// ProjectType is the type of project (module, workspace, gopath, adhoc).
	ProjectType string `json:"project_type,omitempty" jsonschema:"the project type"`
	// Diagnostics contains analysis diagnostics.
	Diagnostics AnalysisDiagnostics `json:"diagnostics,omitempty" jsonschema:"analysis diagnostics"`
	// Summary is a human-readable summary.
	Summary string `json:"summary" jsonschema:"workspace analysis summary"`
	// TotalPackages is the total number of packages discovered (may be > returned).
	TotalPackages int `json:"total_packages,omitempty" jsonschema:"total packages discovered"`
	// TotalEntryPoints is the total number of entry points found (may be > returned).
	TotalEntryPoints int `json:"total_entry_points,omitempty" jsonschema:"total entry points found"`
	// TotalDependencies is the total number of dependencies (may be > returned in summary).
	TotalDependencies int `json:"total_dependencies,omitempty" jsonschema:"total dependencies"`
	// Truncated indicates whether the results were truncated due to size limits.
	Truncated bool `json:"truncated,omitempty" jsonschema:"whether results were truncated"`
	// Hint provides guidance when results are truncated.
	Hint string `json:"hint,omitempty" jsonschema:"hint for further exploration"`
}

// WorkspacePackage represents a package in the workspace.
type WorkspacePackage struct {
	// Path is the package import path.
	Path string `json:"path" jsonschema:"the package import path"`
	// Name is the package name.
	Name string `json:"name" jsonschema:"the package name"`
	// Dir is the package directory.
	Dir string `json:"dir,omitempty" jsonschema:"the package directory"`
	// IsMain indicates whether this is a main package.
	IsMain bool `json:"is_main" jsonschema:"is this a main package?"`
	// ModulePath is the module path (if part of a module).
	ModulePath string `json:"module_path,omitempty" jsonschema:"the module path"`
	// HasTests indicates whether the package has test files.
	HasTests bool `json:"has_tests,omitempty" jsonschema:"does the package have tests?"`
}

// DiagnosticReport represents a single diagnostic report from gopls.
type DiagnosticReport struct {
	// File is the file path where the diagnostic was reported.
	File string `json:"file" jsonschema:"the file path"`
	// Line and Column are retained for reference but unreliable for LLM navigation.
	// The CodeSnippet field below provides the actual source code context.
	Line   int `json:"line,omitempty" jsonschema:"the line number (1-indexed)"`
	Column int `json:"column,omitempty" jsonschema:"the column number (1-indexed)"`
	// Severity is the severity level (error, warning, info, hint).
	Severity string `json:"severity" jsonschema:"the severity level"`
	// Message is the diagnostic message.
	Message string `json:"message" jsonschema:"the diagnostic message"`
	// Source is the source of the diagnostic (e.g., "go", "compiler").
	Source string `json:"source" jsonschema:"the diagnostic source"`
	// DiagnosticCode is the diagnostic code (if available), e.g., "deprecated", "unused".
	DiagnosticCode string `json:"diagnostic_code,omitempty" jsonschema:"the diagnostic code identifier"`
	// CodeSnippet is the actual source line containing the error.
	// This is more reliable than line/column numbers for LLM understanding.
	CodeSnippet string `json:"code_snippet" jsonschema:"the source code line containing the diagnostic"`
}

// IDependencyGraphParams is the input for get_dependency_graph tool.
type IDependencyGraphParams struct {
	// todo: this may not be useful, LLM may not know the package path to analyze.
	// PackagePath is the package import path (e.g., "net/http").
	// If empty, analyzes the main module's root package.
	PackagePath string `json:"package_path,omitempty" jsonschema:"the package import path (default: main module root)"`
	// Cwd is the current working directory (used to locate go.mod and project context).
	Cwd string `json:"Cwd,omitempty" jsonschema:"the current working directory to find the go.mod file (default: session view)"`
	// IncludeTransitive indicates whether to include transitive dependencies.
	// When false (default), only direct dependencies are returned.
	// When true, the full dependency tree (all levels) is included.
	IncludeTransitive bool `json:"include_transitive,omitempty" jsonschema:"whether to include transitive dependencies (default: false)"`
	// MaxDepth limits the depth of transitive dependency traversal.
	// If 0 or not set, and include_transitive is true, all levels are included.
	// If set, traversal stops at this depth (1 = direct dependencies only).
	MaxDepth int `json:"max_depth,omitempty" jsonschema:"maximum depth for transitive dependencies (default: 0 = unlimited)"`
}

// ODependencyGraphResult is the output for get_dependency_graph tool.
type ODependencyGraphResult struct {
	// PackagePath is the analyzed package path.
	PackagePath string `json:"package_path" jsonschema:"the analyzed package path"`
	// PackageName is the package name.
	PackageName string `json:"package_name" jsonschema:"the package name"`
	// Dependencies is the list of packages this package imports.
	Dependencies []PackageDependency `json:"dependencies,omitempty" jsonschema:"packages imported by this package"`
	// Dependents is the list of packages that import this package.
	Dependents []PackageDependent `json:"dependents,omitempty" jsonschema:"packages that import this package"`
	// Summary is a human-readable summary.
	Summary string `json:"summary" jsonschema:"dependency graph summary"`
	// TotalDependencies is the total number of dependencies found.
	TotalDependencies int `json:"total_dependencies,omitempty" jsonschema:"total number of dependencies"`
	// TotalDependents is the total number of dependents found.
	TotalDependents int `json:"total_dependents,omitempty" jsonschema:"total number of dependents"`
	// Truncated indicates whether results were truncated due to size limits.
	Truncated bool `json:"truncated,omitempty" jsonschema:"whether results were truncated"`
}

// PackageDependency represents a package that is imported by the analyzed package.
type PackageDependency struct {
	// Path is the package import path.
	Path string `json:"path" jsonschema:"the package import path"`
	// Name is the package name.
	Name string `json:"name,omitempty" jsonschema:"the package name"`
	// ModulePath is the module path (if part of a module).
	ModulePath string `json:"module_path,omitempty" jsonschema:"the module path"`
	// IsStdlib indicates whether this is a standard library package.
	IsStdlib bool `json:"is_stdlib,omitempty" jsonschema:"is this a standard library package"`
	// IsExternal indicates whether this is an external dependency.
	IsExternal bool `json:"is_external,omitempty" jsonschema:"is this an external dependency"`
	// Depth is the dependency depth (0 = direct, 1+ = transitive).
	Depth int `json:"depth,omitempty" jsonschema:"the dependency depth"`
}

// PackageDependent represents a package that imports the analyzed package.
type PackageDependent struct {
	// Path is the package import path.
	Path string `json:"path" jsonschema:"the package import path"`
	// Name is the package name.
	Name string `json:"name,omitempty" jsonschema:"the package name"`
	// ModulePath is the module path (if part of a module).
	ModulePath string `json:"module_path,omitempty" jsonschema:"the module path"`
	// IsTest indicates whether this is a test package.
	IsTest bool `json:"is_test,omitempty" jsonschema:"is this a test package"`
}

// ICallHierarchyParams is the input for get_call_hierarchy tool.
type ICallHierarchyParams struct {
	// Locator specifies the function to get call hierarchy for.
	// This uses semantic information (symbol name, context file, package, scope)
	// instead of error-prone line/column numbers.
	Locator SymbolLocator `json:"locator" jsonschema:"semantic symbol locator (symbol_name, context_file, package_name, parent_scope, kind, line_hint)"`
	// Direction determines which direction to traverse: "incoming", "outgoing", or "both".
	// Default is "both".
	Direction string `json:"direction,omitempty" jsonschema:"call hierarchy direction (incoming/outgoing/both, default: both)"`
	// Cwd optionally specifies the working directory for call hierarchy analysis.
	// This is useful when analyzing files in temporary directories or specific workspaces.
	// If not set, the default view is used.
	Cwd string `json:"Cwd,omitempty" jsonschema:"the working directory for call hierarchy analysis (default: use default view)"`
}

// OCallHierarchyResult is the output for get_call_hierarchy tool.
type OCallHierarchyResult struct {
	// Symbol is the symbol at the given position.
	// Contains name, kind, signature, file, line, documentation, and optionally body.
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
// Note: Line numbers only (no columns) for cleaner LLM consumption.
type CallRange struct {
	// File is the file path.
	File string `json:"file" jsonschema:"the file path"`
	// StartLine is the start line number (1-indexed).
	StartLine int `json:"start_line" jsonschema:"the start line number (1-indexed)"`
	// EndLine is the end line number (1-indexed).
	EndLine int `json:"end_line" jsonschema:"the end line number (1-indexed)"`
}
