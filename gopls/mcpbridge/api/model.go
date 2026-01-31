// Package api provides input/output types for the gopls-mcp MCP server.
package api

// ===== Input types for MCP tools =====

// IListModules provides parameters for listing modules.
type IListModules struct {
	// DirectOnly indicates whether to show only direct dependencies.
	// When true (default), only the main module and direct dependencies are shown (indirect dependencies are excluded).
	// When false, all modules are shown (main + direct + indirect).
	DirectOnly *bool `json:"direct_only" jsonschema:"whether to show only direct dependencies (default: true)"`

	// Cwd is the current working directory (used to locate go.mod and project context).
	Cwd string `json:"Cwd,omitempty" jsonschema:"the current working directory to find the go.mod file (default: session view)"`
}

// IListModulePackages provides parameters for listing packages in a module.
type IListModulePackages struct {
	// ModulePath is the module path to list packages for (e.g., "github.com/user/project").
	// If empty, lists packages for the main module.
	ModulePath string `json:"module_path,omitempty" jsonschema:"the module path to list packages for (default: main module)"`
	// IncludeDocs indicates whether to include package documentation.
	// When false (default), only package names and paths are returned (faster).
	// When true, package documentation is included.
	IncludeDocs *bool `json:"include_docs" jsonschema:"whether to include package documentation (default: false)"`
	// ExcludeTests excludes test packages (packages ending with _test) when true.
	// When false (default), all packages are included.
	ExcludeTests *bool `json:"exclude_tests" jsonschema:"whether to exclude test packages (default: false)"`
	// ExcludeInternal excludes internal packages (packages containing /internal/) when true.
	// When false (default), all packages are included.
	ExcludeInternal *bool `json:"exclude_internal" jsonschema:"whether to exclude internal packages (default: false)"`
	// TopLevelOnly excludes nested packages (packages with more than 2 path segments beyond module path) when true.
	// For example, "github.com/user/mypkg" is included but "github.com/user/mypkg/subpkg" is excluded.
	// When false (default), all packages are included.
	TopLevelOnly *bool `json:"top_level_only" jsonschema:"whether to include only top-level packages (default: false)"`
	// Cwd is the current working directory (used to locate go.mod and project context).
	Cwd string `json:"Cwd,omitempty" jsonschema:"the current working directory to find the go.mod file (default: session view)"`
}

// IListPackageSymbols provides parameters for listing symbols in a package.
type IListPackageSymbols struct {
	// PackagePath is the package import path (e.g., "github.com/user/project/mypkg").
	PackagePath string `json:"package_path" jsonschema:"the package import path to list symbols for"`
	// IncludeDocs indicates whether to include symbol documentation.
	// When false (default), only symbol names and signatures are returned.
	// When true, full symbol documentation is included.
	IncludeDocs *bool `json:"include_docs" jsonschema:"whether to include symbol documentation (default: false)"`
	// IncludeBodies indicates whether to include function bodies.
	// When false (default), only signatures are returned.
	// When true, full function implementations are included.
	IncludeBodies *bool `json:"include_bodies" jsonschema:"whether to include function implementations (default: false)"`
	// Cwd is the current working directory (used to locate go.mod and project context).
	Cwd string `json:"Cwd,omitempty" jsonschema:"the current working directory to find the go.mod file (default: session view)"`
}

// IGetStarted provides parameters for getting started with a Go project.
type IGetStarted struct {
	// Cwd is the current working directory (used to locate go.mod and project context).
	Cwd string `json:"Cwd,omitempty" jsonschema:"the current working directory (default: session view)"`
}

// ===== Output types for MCP tools =====

// OListModules contains module information split into internal and external modules.
// This reduces noise for LLM consumption by separating workspace modules from external dependencies.
type OListModules struct {
	// Summary provides a quick overview of the module structure.
	Summary ModuleSummary `json:"summary" jsonschema:"summary of module structure"`
	// InternalModules are modules maintained by the current project (main module + submodules).
	// These are identified by prefix matching with the root module path.
	InternalModules []ModuleInfo `json:"internal_modules" jsonschema:"modules maintained by the current project"`
	// ExternalModules are third-party dependencies not maintained by the current project.
	// This list is limited to prevent overwhelming the LLM with transitive dependencies.
	ExternalModules []ModuleInfo `json:"external_modules" jsonschema:"third-party dependencies"`
}

// ModuleSummary provides a quick overview of the module structure.
type ModuleSummary struct {
	// RootModule is the path of the main module.
	RootModule string `json:"root_module" jsonschema:"the main module path"`
	// TotalModules is the total number of modules (internal + external).
	TotalModules int `json:"total_modules" jsonschema:"total number of modules"`
	// InternalCount is the number of internal modules (including main module).
	InternalCount int `json:"internal_count" jsonschema:"number of internal modules"`
	// ExternalCount is the number of external dependencies.
	ExternalCount int `json:"external_count" jsonschema:"number of external dependencies"`
}

// OListModulePackages contains packages in a module.
type OListModulePackages struct {
	// ModulePath is the module path.
	ModulePath string `json:"module_path" jsonschema:"the module path"`
	// Packages is a list of packages in the module.
	Packages []PackageInfo `json:"packages" jsonschema:"the packages in the module"`
}

// OListPackageSymbols contains symbols in a package.
type OListPackageSymbols struct {
	// PackagePath is the package path.
	PackagePath string `json:"package_path" jsonschema:"the package path"`
	// Symbols is a list of symbols in the package.
	Symbols []Symbol `json:"symbols" jsonschema:"the symbols in the package"`
	// TotalCount is the total number of symbols available.
	TotalCount int `json:"total_count" jsonschema:"total number of symbols in the package"`
	// Returned is the number of symbols returned in this response.
	Returned int `json:"returned" jsonschema:"number of symbols returned"`
	// Truncated indicates whether not all symbols were returned.
	Truncated bool `json:"truncated" jsonschema:"whether the result was truncated due to size limits"`
	// Hint provides guidance when results are truncated.
	Hint string `json:"hint,omitempty" jsonschema:"suggestion for getting more details"`
}

// OGetStarted contains a getting started guide for a Go project.
type OGetStarted struct {
	// Identity contains basic project information.
	Identity ProjectIdentity `json:"identity" jsonschema:"project identity information"`
	// Stats contains quick statistics about the project.
	Stats ProjectStats `json:"stats" jsonschema:"project statistics"`
	// EntryPoints contains suggested starting points for exploring the codebase.
	EntryPoints []GuideEntryPoint `json:"entry_points" jsonschema:"suggested entry points for exploration"`
	// Categories groups packages by their purpose.
	Categories map[string][]string `json:"categories" jsonschema:"packages grouped by category"`
	// NextSteps contains recommended actions for further exploration.
	NextSteps []string `json:"next_steps" jsonschema:"recommended next steps"`
}

// ProjectIdentity contains basic project information.
type ProjectIdentity struct {
	// Name is the module path.
	// todo: is the jsonschema correct here, can it has space??
	Name string `json:"name" jsonschema:"module path"`
	// Type is the project type (module, workspace, gopath, adhoc).
	Type string `json:"type" jsonschema:"project type"`
	// Root is the root directory of the project.
	Root string `json:"root" jsonschema:"project root directory"`
	// GoVersion is the Go version from go.mod (minimum required version).
	GoVersion string `json:"go_version,omitempty" jsonschema:"Go version from go.mod"`
	// GoRuntimeVersion is the actual Go runtime version being used by gopls.
	GoRuntimeVersion string `json:"go_runtime_version,omitempty" jsonschema:"Actual Go runtime version"`
	// Description is a brief description of the project.
	Description string `json:"description,omitempty" jsonschema:"project description"`
}

// ProjectStats contains quick statistics about a project.
type ProjectStats struct {
	// TotalPackages is the total number of non-test packages.
	TotalPackages int `json:"total_packages" jsonschema:"total non-test packages"`
	// MainPackages is the number of main packages.
	MainPackages int `json:"main_packages" jsonschema:"number of main packages"`
	// TestPackages is the number of test packages.
	TestPackages int `json:"test_packages" jsonschema:"number of test packages"`
	// Dependencies is the number of external dependencies.
	Dependencies int `json:"dependencies" jsonschema:"number of external dependencies"`
}

// GuideEntryPoint represents a suggested entry point for the get_started guide.
// Note: This is different from the EntryPoint type used by analyze_workspace,
// as it includes package paths and categories rather than just files and types.
type GuideEntryPoint struct {
	// todo: the category looks like come from current project, may need update.
	// Category is the type of entry point (main, core, api, test, etc.).
	Category string `json:"category" jsonschema:"entry point category"`
	// Path is the package import path.
	Path string `json:"path,omitempty" jsonschema:"package path"`
	// File is the file path (if applicable).
	File string `json:"file,omitempty" jsonschema:"file path"`
	// Description explains why this is an entry point.
	Description string `json:"description" jsonschema:"description of this entry point"`
}

// ===== Core data structures =====

// ModuleInfo represents simplified module information.
type ModuleInfo struct {
	// Path is the module import path (e.g., "example.com/module").
	Path string `json:"path" jsonschema:"the module path"`
	// Version is the module version (e.g., "v1.2.3").
	Version string `json:"version,omitempty" jsonschema:"the module version"`
	// Main indicates whether this is the main module.
	Main bool `json:"main" jsonschema:"is this the main module?"`
	// Indirect indicates whether this is an indirect dependency.
	Indirect bool `json:"indirect,omitempty" jsonschema:"is this module only an indirect dependency of main module?"`
	// FilePath is the absolute path to the module directory on disk.
	// Only populated when the module has a local replace directive in go.mod.
	// This is important because the Version field may not match the actual code when using local replaces.
	FilePath string `json:"file_path,omitempty" jsonschema:"absolute path to module directory (only for locally replaced modules)"`
	// Replaces indicates the original module path that this module replaces.
	// For example, if "github.com/new/api" replaces "github.com/old/api",
	// this field will be "github.com/old/api".
	Replaces string `json:"replaces,omitempty" jsonschema:"original module path replaced by this module"`
}

// PackageInfo represents simplified package information.
type PackageInfo struct {
	// Name is the package name (e.g., "http").
	Name string `json:"name" jsonschema:"the name of the package"`
	// Path is the import path (e.g., "net/http").
	Path string `json:"path" jsonschema:"the import path of a package"`
	// Docs is the package documentation (optional).
	Docs string `json:"docs,omitempty" jsonschema:"the documentation of a package"`
}

// Module represents a Go module.
type Module struct {
	// Path is the module import path (e.g., "example.com/module").
	Path string `json:"path" jsonschema:"the module path"`
	// Version is the module version (e.g., "v1.2.3").
	Version string `json:"version" jsonschema:"the module version"`
	// Main indicates whether this is the main module.
	Main bool `json:"main" jsonschema:"is this the main module?"`
	// Indirect indicates whether this is an indirect dependency.
	Indirect bool `json:"indirect" jsonschema:"is this module only an indirect dependency of main module?"`
	// Dir is the directory holding files for this module.
	Dir string `json:"dir" jsonschema:"directory holding files for this module, if any"`
	// GoMod is the path to the go.mod file.
	GoMod string `json:"go_mod" jsonschema:"path to go.mod file used when loading this module, if any"`
	// GoVersion is the Go version specified in the module.
	GoVersion string `json:"go_version" jsonschema:"go version used in module"`
	// TODO: Consider whether and how to support Replace field.
	// Replace *Module `json:"replace" jsonschema:"replaced by this module"`
}

// Pos represents a position in source code.
type Pos struct {
	// Filename is the file path.
	Filename string `json:"filename" jsonschema:"the filename of current position"`
	// Offset is the byte offset, starting at 0.
	Offset int `json:"offset" jsonschema:"the byte offset in the file, starting at 0"`
	// Line is the line number, starting at 1.
	Line int `json:"line" jsonschema:"line number, starting at 1"`
	// Column is the column number (byte count), starting at 1.
	Column int `json:"column" jsonschema:"column number, starting at 1 (byte count)"`
}

// SymbolKind represents the semantic category of a symbol.
// We stick to LSP-compatible kinds but simplify for LLM comprehension.
type SymbolKind string

const (
	SymbolKindField     SymbolKind = "field"     // Struct fields
	SymbolKindMethod    SymbolKind = "method"    // Methods attached to a receiver
	SymbolKindFunction  SymbolKind = "function"  // Standalone functions
	SymbolKindStruct    SymbolKind = "struct"    // Struct definitions
	SymbolKindInterface SymbolKind = "interface" // Interface definitions
	SymbolKindVariable  SymbolKind = "var"       // Variables
	SymbolKindConstant  SymbolKind = "const"     // Constants
	SymbolKindType      SymbolKind = "type"      // Other types (alias, basic types)
)

// Symbol represents a semantic unit of code.
// Designed to provide "Entropy-Free" context to the LLM in a flat list.
type Symbol struct {
	// Identity -----------------------------------------------------------

	// Name is the short identifier of the symbol.
	// Example: "Start", "Timeout", "User"
	Name string `json:"name" jsonschema:"the short name of the symbol"`

	// Kind defines the semantic category.
	Kind SymbolKind `json:"kind" jsonschema:"the semantic type (function, method, struct, etc.)"`

	// Context (The Anti-Entropy Fields) ----------------------------------

	// Receiver is CRITICAL for methods. It specifies the type that owns this method.
	// Format: "*Server", "User"
	// Example: If Name is "Start" and Receiver is "*Server", LLM knows it's Server.Start().
	Receiver string `json:"receiver,omitempty" jsonschema:"the receiver type for methods (e.g., *Server)"`

	// Parent specifies the enclosing structure for fields or nested types.
	// Example: If Name is "Timeout" and Parent is "Config", LLM knows it's Config.Timeout.
	Parent string `json:"parent,omitempty" jsonschema:"the parent struct/interface for fields"`

	// Contract (The Reasoning Fields) ------------------------------------

	// Signature is the exact code contract.
	// For functions: "func(ctx context.Context) error"
	// For variables: "int"
	// For structs: "struct { ... }" (usually truncated)
	// TODO: add method example.
	Signature string `json:"signature" jsonschema:"the type signature or function definition"`

	// Implementation (The Physical Fields) -------------------------------

	// PackagePath is the import path of the package containing this symbol.
	// Example: "net/http", "github.com/user/project/pkg"
	// This is crucial for distinguishing symbols with the same name from different packages.
	PackagePath string `json:"package_path,omitempty" jsonschema:"import path of the package containing the symbol"`

	// FilePath is the relative path to the source file.
	FilePath string `json:"file_path" jsonschema:"source file path"`

	// Line is the starting line number.
	// We use int instead of full Pos struct to save tokens while keeping navigability.
	Line int `json:"line" jsonschema:"line number where symbol is defined"`

	// Content (Optional / Heavy) -----------------------------------------

	// Doc is the comment block associated with the symbol.
	// Only included if include_docs=true.
	Doc string `json:"doc,omitempty" jsonschema:"documentation comments"`

	// Body is the full implementation code.
	// Only included if include_bodies=true.
	Body string `json:"body,omitempty" jsonschema:"full implementation code"`
}

// Package contains information about a Go package.
type Package struct {
	// Name is the package name (e.g., "http").
	Name string `json:"name" jsonschema:"the name of the package"`
	// Path is the import path (e.g., "net/http").
	Path string `json:"path" jsonschema:"the import path of a package"`
	// ModuleName is the module name (optional).
	ModuleName string `json:"module,omitempty" jsonschema:"the associated module of a package"`
	// ModuleVersion is the module version (optional).
	ModuleVersion string `json:"module_version,omitempty" jsonschema:"the associated module version of a package"`
	// Docs is the package documentation.
	Docs string `json:"docs,omitempty" jsonschema:"the documentation of a package"`
	// Symbols is the list of exported symbols in the package.
	Symbols []Symbol `json:"symbols,omitempty" jsonschema:"the symbols in a package"`
}
