package core

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/internal/cache"
	"golang.org/x/tools/gopls/internal/cache/metadata"
	"golang.org/x/tools/gopls/internal/cache/parsego"
	"golang.org/x/tools/gopls/internal/golang"
	"golang.org/x/tools/gopls/internal/protocol"
	"golang.org/x/tools/gopls/internal/settings"
	"golang.org/x/tools/gopls/mcpbridge/api"
)

// Handler implements gopls-mcp MCP tools using gopls's existing APIs.
// This bridges the MCP tool requests to gopls's session, snapshot, and analysis capabilities.
type Handler struct {
	session   *cache.Session
	lspServer protocol.Server
	// options holds the gopls configuration options used for creating views.
	options *settings.Options
	// config holds the gopls-mcp configuration (response limits, etc.)
	config *MCPConfig
	// allowDynamicViews enables creating new gopls views on-demand for e2e testing.
	// When false (default), viewForDir returns an error if no existing view matches.
	// When true (test mode), viewForDir creates a new view for the directory.
	// This is a TEST-ONLY feature to allow one gopls-mcp process to service multiple
	// test projects, sharing the gopls cache across all tests for better performance.
	allowDynamicViews bool
	// dynamicViews tracks dynamically created views for cleanup in test mode.
	// Maps directory path to release function.
	dynamicViews   map[string]func()
	dynamicViewsMu sync.Mutex
}

// HandlerOption configures the Handler behavior.
type HandlerOption func(*Handler)

// WithConfig sets the gopls-mcp configuration for the handler.
func WithConfig(config *MCPConfig) HandlerOption {
	return func(h *Handler) {
		h.config = config
	}
}

// WithDynamicViews enables dynamic view creation for e2e testing.
// TEST-ONLY: This allows the handler to create new gopls views on-demand when
// a Cwd parameter doesn't match any existing view.
//
// WARNING: This is intended for e2e testing only. Normal users should not use this,
// as it can lead to unexpected behavior and increased memory usage.
//
// Use case: E2E tests can start one gopls-mcp process and reuse it across multiple
// test projects, avoiding the overhead of starting a new process for each test.
//
// Example usage in tests:
//
//	testutil.StartMCPServer(t, workdir, testutil.WithDynamicViews())
func WithDynamicViews(allow bool) HandlerOption {
	return func(h *Handler) {
		h.allowDynamicViews = allow
	}
}

// NewHandler creates a new Handler backed by gopls's session and LSP server.
// Based on: gopls/internal/mcp/mcp.go handler struct (line 31-34)
//
// Note: This does NOT eagerly populate the cache. The gopls snapshot uses
// lazy loading, where packages are loaded on-demand when tool handlers
// call methods like WorkspaceMetadata(), LoadMetadataGraph(), etc.
// These methods internally call awaitLoaded() which triggers reloadWorkspace().
func NewHandler(session *cache.Session, lspServer protocol.Server, opts ...HandlerOption) *Handler {
	h := &Handler{
		session:           session,
		lspServer:         lspServer,
		options:           settings.DefaultOptions(), // Default gopls options
		config:            DefaultConfig(),           // Default gopls-mcp config
		allowDynamicViews: false,                     // Production mode: no dynamic views
		dynamicViews:      make(map[string]func()),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// snapshot returns the best default snapshot for workspace queries.
// Based on: gopls/internal/mcp/mcp.go snapshot() method (line 316-322)
func (h *Handler) snapshot() (*cache.Snapshot, func(), error) {
	views := h.session.Views()
	if len(views) == 0 {
		return nil, nil, fmt.Errorf("no active views")
	}
	return views[0].Snapshot()
}

// viewForDir finds the view that contains the given directory.
// This is needed for tools that take a Cwd parameter.
//
// If allowDynamicViews is enabled (TEST MODE), this function will create a new
// view for the directory if no existing view contains it. This allows e2e tests
// to reuse a single gopls-mcp process across multiple test projects.
//
// PRODUCTION USAGE: Normal users start one gopls-mcp per project and should not
// rely on dynamic view creation.
func (h *Handler) viewForDir(dir string) (*cache.View, error) {
	dir = filepath.Clean(dir)

	// First, check if an existing view contains this directory
	views := h.session.Views()
	for _, v := range views {
		root := v.Root().Path()
		if strings.HasPrefix(dir, root) || dir == root {
			return v, nil
		}
	}

	// TEST-ONLY: Dynamic view creation for e2e testing
	if !h.allowDynamicViews {
		// Production mode: return error, don't create new views
		return nil, fmt.Errorf("no view found for directory %s", dir)
	}

	// Test mode: create a new view for this directory
	// This allows one gopls-mcp process to service multiple test projects,
	// sharing the gopls cache (GOROOT, stdlib, etc.) across all tests.
	ctx := context.Background()
	dirURI := protocol.URIFromPath(dir)

	// Fetch Go environment for this directory
	goEnv, err := cache.FetchGoEnv(ctx, dirURI, h.options)
	if err != nil {
		return nil, fmt.Errorf("failed to load Go env for %s: %w", dir, err)
	}

	// Create a new view for this directory
	folder := &cache.Folder{
		Dir:     dirURI,
		Options: h.options,
		Env:     *goEnv,
	}

	view, _, releaseView, err := h.session.NewView(ctx, folder)
	if err != nil {
		return nil, fmt.Errorf("failed to create view for %s: %w", dir, err)
	}

	// Track the view for cleanup (though we never call these in test mode)
	h.dynamicViewsMu.Lock()
	h.dynamicViews[dir] = releaseView
	h.dynamicViewsMu.Unlock()

	return view, nil
}

// getView returns a view for the given directory, or the first available view if dir is empty.
// This is a convenience wrapper for handlers that accept an optional Cwd parameter.
func (h *Handler) getView(dir string) (*cache.View, error) {
	if dir != "" {
		return h.viewForDir(dir)
	}
	views := h.session.Views()
	if len(views) == 0 {
		return nil, fmt.Errorf("no views available")
	}
	return views[0], nil
}

// ===== MCP Tool Handler Implementations =====

// handleListModules returns current module and all required modules, split into internal and external.
// Uses: view.ModFiles() and snapshot.ParseMod() from gopls/internal/cache
func handleListModules(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.IListModules) (*mcp.CallToolResult, *api.OListModules, error) {
	view, err := h.getView(input.Cwd)
	if err != nil {
		return nil, nil, err
	}

	snapshot, release, err := view.Snapshot()
	if err != nil {
		return nil, nil, err
	}
	defer release()

	modFiles := view.ModFiles()
	if len(modFiles) == 0 {
		return nil, nil, fmt.Errorf("no go.mod files found in view")
	}

	allModules := []api.ModuleInfo{}
	seen := map[string]bool{}
	// DirectOnly defaults to true (show only direct dependencies by default)
	directOnly := input.DirectOnly == nil || *input.DirectOnly
	var mainModulePath string

	// Track two types of replaces:
	// 1. Local file path replaces (old module -> local path)
	// 2. Module replaces (old module -> new module with version)
	localReplaces := make(map[string]string)       // module path -> local file path
	moduleReplaces := make(map[string]replaceInfo) // old module -> new module info

	for _, modURI := range modFiles {
		fh, err := snapshot.ReadFile(ctx, modURI)
		if err != nil {
			continue
		}

		pmf, err := snapshot.ParseMod(ctx, fh)
		if err != nil {
			continue
		}

		if pmf.File == nil || pmf.File.Module == nil {
			continue
		}

		// Get the directory containing the go.mod file (for resolving relative paths)
		modDir := filepath.Dir(modURI.Path())

		// Process replace directives
		for _, repl := range pmf.File.Replace {
			if isLocalPath(repl.New.Path) {
				// Local file path replace (e.g., ../local-module)
				resolvedPath := repl.New.Path
				if strings.HasPrefix(repl.New.Path, ".") {
					resolvedPath = filepath.Join(modDir, repl.New.Path)
					resolvedPath = filepath.Clean(resolvedPath)
				}
				localReplaces[repl.Old.Path] = resolvedPath
			} else {
				// Module replace (e.g., github.com/old/module => github.com/new/module v1.0.0)
				moduleReplaces[repl.Old.Path] = replaceInfo{
					newPath:    repl.New.Path,
					newVersion: repl.New.Version,
				}
			}
		}

		// Main module
		modPath := pmf.File.Module.Mod.Path
		mainModulePath = modPath
		// Main module always has a file path (its own directory)
		localReplaces[modPath] = modDir
		allModules = append(allModules, api.ModuleInfo{
			Path: modPath,
			Main: true,
		})
		seen[modPath] = true

		// Requirements
		for _, req := range pmf.File.Require {
			if seen[req.Mod.Path] {
				continue
			}
			// Skip indirect dependencies if DirectOnly is true
			if directOnly && req.Indirect {
				continue
			}
			allModules = append(allModules, api.ModuleInfo{
				Path:     req.Mod.Path,
				Version:  req.Mod.Version,
				Main:     false,
				Indirect: req.Indirect,
			})
		}
	}

	// Add replacement modules and update original modules with replace info
	modulesMap := make(map[string]*api.ModuleInfo) // path -> pointer to module in allModules
	for i := range allModules {
		modulesMap[allModules[i].Path] = &allModules[i]
	}

	// Process all replacements
	for oldPath, repl := range moduleReplaces {
		// Add the new replacement module if not already present
		if _, exists := modulesMap[repl.newPath]; !exists {
			newMod := api.ModuleInfo{
				Path:     repl.newPath,
				Version:  repl.newVersion,
				Main:     false,
				Replaces: oldPath, // This module replaces the old one
			}
			allModules = append(allModules, newMod)
			modulesMap[repl.newPath] = &allModules[len(allModules)-1]
		}
		// Note: The old module remains in the list. The replacement module shows Replaces field.
	}

	// Split modules into internal and external, and populate FilePath/Replaces
	internal := []api.ModuleInfo{}
	external := []api.ModuleInfo{}
	for _, mod := range allModules {
		// Set FilePath for locally replaced modules
		if filePath, ok := localReplaces[mod.Path]; ok {
			mod.FilePath = filePath
		}
		if isInternal(mod, mainModulePath) {
			internal = append(internal, mod)
		} else {
			external = append(external, mod)
		}
	}

	// Sort internal by overlap, external alphabetically
	if len(internal) > 1 {
		sortModulesByOverlap(internal, mainModulePath)
	}
	sort.Slice(external, func(i, j int) bool {
		return external[i].Path < external[j].Path
	})

	// Build result
	result := &api.OListModules{
		Summary: api.ModuleSummary{
			RootModule:    mainModulePath,
			TotalModules:  len(allModules),
			InternalCount: len(internal),
			ExternalCount: len(external),
		},
		InternalModules: internal,
		ExternalModules: external,
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: formatModulesList(result)}}}, result, nil
}

// replaceInfo holds information about a module replacement.
type replaceInfo struct {
	newPath    string // The new module path
	newVersion string // The new module version
}

// handleListModulePackages returns all packages in a given module.
// Uses: snapshot.LoadMetadataGraph() from gopls/internal/cache
func handleListModulePackages(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.IListModulePackages) (*mcp.CallToolResult, *api.OListModulePackages, error) {
	view, err := h.getView(input.Cwd)
	if err != nil {
		return nil, nil, err
	}

	snapshot, release, err := view.Snapshot()
	if err != nil {
		return nil, nil, err
	}
	defer release()

	md, err := snapshot.LoadMetadataGraph(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load metadata graph: %v", err)
	}

	// Determine target module path
	targetModulePath := input.ModulePath
	if targetModulePath == "" {
		// Use main module path
		modFiles := view.ModFiles()
		if len(modFiles) == 0 {
			return nil, nil, fmt.Errorf("no go.mod files found in view")
		}
		for _, modURI := range modFiles {
			fh, err := snapshot.ReadFile(ctx, modURI)
			if err != nil {
				continue
			}
			pmf, err := snapshot.ParseMod(ctx, fh)
			if err != nil {
				continue
			}
			if pmf.File != nil && pmf.File.Module != nil {
				targetModulePath = pmf.File.Module.Mod.Path
				break
			}
		}
		if targetModulePath == "" {
			return nil, nil, fmt.Errorf("failed to determine main module path")
		}
	}

	// Collect packages for the target module
	packages := []api.PackageInfo{} // Initialize as empty slice to avoid null in JSON
	for _, mps := range md.ForPackagePath {
		if len(mps) == 0 {
			continue
		}
		mp := mps[0] // first is best

		// Check if package belongs to target module
		if mp.Module == nil || mp.Module.Path != targetModulePath {
			continue
		}

		pkgPath := string(mp.PkgPath)

		// Apply ExcludeTests filter
		if input.ExcludeTests != nil && *input.ExcludeTests {
			// Check if package name ends with _test
			if strings.HasSuffix(string(mp.Name), "_test") {
				continue
			}
		}

		// Apply ExcludeInternal filter
		if input.ExcludeInternal != nil && *input.ExcludeInternal {
			// Check if package path contains /internal/ (middle) or ends with /internal
			if strings.Contains(pkgPath, "/internal/") || strings.HasSuffix(pkgPath, "/internal") {
				continue
			}
		}

		// Apply TopLevelOnly filter
		if input.TopLevelOnly != nil && *input.TopLevelOnly {
			// Only include packages at the top level (no subdirectories beyond module path)
			// Remove module path prefix to get relative package path
			relPath := strings.TrimPrefix(pkgPath, targetModulePath+"/")
			// Top-level packages have 0 slashes (no nested directories)
			if strings.Contains(relPath, "/") {
				continue
			}
		}

		pkgInfo := api.PackageInfo{
			Name: string(mp.Name),
			Path: pkgPath,
		}

		// Add docs if requested
		if input.IncludeDocs != nil && *input.IncludeDocs {
			pkgInfo.Docs = extractPackageDocs(ctx, snapshot, mp)
		}

		packages = append(packages, pkgInfo)
	}

	result := &api.OListModulePackages{
		ModulePath: targetModulePath,
		Packages:   packages,
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: formatModulePackages(result)}}}, result, nil
}

// handleListPackageSymbols returns all symbols in a package.
// Uses: snapshot.LoadMetadataGraph() and golang.DocumentSymbols() from gopls/internal/cache
func handleListPackageSymbols(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.IListPackageSymbols) (*mcp.CallToolResult, *api.OListPackageSymbols, error) {
	view, err := h.getView(input.Cwd)
	if err != nil {
		return nil, nil, err
	}

	snapshot, release, err := view.Snapshot()
	if err != nil {
		return nil, nil, err
	}
	defer release()

	md, err := snapshot.LoadMetadataGraph(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load metadata graph: %v", err)
	}

	pkgPath := metadata.PackagePath(input.PackagePath)
	mps := md.ForPackagePath[pkgPath]
	if len(mps) == 0 {
		return nil, nil, fmt.Errorf("package not found: %s", input.PackagePath)
	}

	mp := mps[0]              // first is best
	symbols := []api.Symbol{} // Initialize as empty slice to avoid null in JSON

	// Determine options
	includeDocs := input.IncludeDocs != nil && *input.IncludeDocs
	includeBodies := input.IncludeBodies != nil && *input.IncludeBodies

	// Extract symbols from package files
	for _, uri := range mp.CompiledGoFiles {
		fh, err := snapshot.ReadFile(ctx, uri)
		if err != nil {
			continue
		}

		// Parse the file to get AST for docs and bodies
		pgf, err := snapshot.ParseGo(ctx, fh, parsego.Full)
		if err != nil {
			continue
		}

		// Get LSP symbols for structure
		syms, err := golang.DocumentSymbols(ctx, snapshot, fh)
		if err != nil {
			continue
		}

		// Build a map of symbol positions to docs/bodies from AST
		docMap := make(map[string]string)
		bodyMap := make(map[string]string)

		if includeDocs || includeBodies {
			for _, decl := range pgf.File.Decls {
				var name string
				var doc string
				var body string

				switch decl := decl.(type) {
				case *ast.FuncDecl:
					if decl.Name == nil {
						continue
					}
					name = decl.Name.Name
					// Build receiver prefix for methods
					if decl.Recv != nil && len(decl.Recv.List) > 0 {
						recvType := types.ExprString(decl.Recv.List[0].Type)
						name = fmt.Sprintf("(%s).%s", recvType, name)
					}
					// Extract documentation
					if decl.Doc != nil {
						doc = string(decl.Doc.Text())
					}
					// Extract body if requested
					if includeBodies && decl.Body != nil {
						body = golang.ExtractBodyText(pgf, decl.Body)
					}

				case *ast.GenDecl:
					for _, spec := range decl.Specs {
						switch spec := spec.(type) {
						case *ast.TypeSpec:
							if spec.Name == nil {
								continue
							}
							name = spec.Name.Name
							// Extract documentation
							if spec.Doc != nil {
								doc = string(spec.Doc.Text())
							} else if decl.Doc != nil {
								doc = string(decl.Doc.Text())
							}

						case *ast.ValueSpec:
							if decl.Tok == token.CONST {
								for _, n := range spec.Names {
									if n.Name == "_" {
										continue
									}
									name = n.Name
									// Extract documentation
									if spec.Doc != nil {
										doc = string(spec.Doc.Text())
									} else if decl.Doc != nil {
										doc = string(decl.Doc.Text())
									}
									docMap[name] = doc
								}
								continue
							}
						}
					}
				}

				if name != "" {
					if doc != "" {
						docMap[name] = doc
					}
					if body != "" {
						bodyMap[name] = body
					}
				}
			}
		}

		// Convert symbols, adding docs and bodies from the AST
		for _, sym := range syms {
			if !isExported(sym.Name) {
				continue
			}

			converted := convertDocumentSymbol(sym, uri.Path(), input.PackagePath)

			// Add documentation from AST
			if includeDocs {
				if doc, ok := docMap[sym.Name]; ok {
					converted.Doc = doc
				}
			}

			// Add body from AST
			if includeBodies {
				if body, ok := bodyMap[sym.Name]; ok {
					converted.Body = body
				}
			}

			symbols = append(symbols, converted)
		}
	}

	// Apply truncation for very large packages
	// Threshold: 200 symbols (reasonable balance between completeness and size)
	const symbolThreshold = 200
	totalSymbols := len(symbols)
	returnedSymbols := symbols
	truncated := false
	var hint string

	if totalSymbols > symbolThreshold {
		returnedSymbols = symbols[:symbolThreshold]
		truncated = true
		hint = fmt.Sprintf("Large package: %d symbols found (showing first %d). Use go_search to find specific symbols, then go_definition for details.",
			totalSymbols, symbolThreshold)
	}

	result := &api.OListPackageSymbols{
		PackagePath: input.PackagePath,
		Symbols:     returnedSymbols,
		TotalCount:  totalSymbols,
		Returned:    len(returnedSymbols),
		Truncated:   truncated,
		Hint:        hint,
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: formatPackageSymbols(result, includeDocs, includeBodies)}}}, result, nil
}

// handleGetStarted provides a getting started guide for the Go project.
// Uses: snapshot.LoadMetadataGraph(), view.ModFiles() from gopls/internal/cache
func handleGetStarted(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.IGetStarted) (*mcp.CallToolResult, *api.OGetStarted, error) {
	view, err := h.getView(input.Cwd)
	if err != nil {
		return nil, nil, err
	}

	snapshot, release, err := view.Snapshot()
	if err != nil {
		return nil, nil, err
	}
	defer release()

	md, err := snapshot.LoadMetadataGraph(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load metadata graph: %v", err)
	}

	// Get module path
	modulePath := ""
	goVersion := ""
	modFiles := view.ModFiles()
	if len(modFiles) > 0 {
		for _, modURI := range modFiles {
			fh, err := snapshot.ReadFile(ctx, modURI)
			if err != nil {
				continue
			}
			pmf, err := snapshot.ParseMod(ctx, fh)
			if err != nil {
				continue
			}
			if pmf.File != nil && pmf.File.Module != nil {
				modulePath = pmf.File.Module.Mod.Path
				goVersion = pmf.File.Go.Version
				break
			}
		}
	}

	// Build identity
	identity := api.ProjectIdentity{
		Name:             modulePath,
		Type:             determineProjectType(view),
		Root:             view.Root().Path(),
		GoVersion:        goVersion,
		GoRuntimeVersion: runtime.Version(),
	}

	// Add description based on project type
	switch identity.Type {
	case "module":
		identity.Description = "Go module project"
	case "workspace":
		identity.Description = "Go workspace (multi-module)"
	case "gopath":
		identity.Description = "GOPATH-based project"
	case "adhoc":
		identity.Description = "Ad-hoc Go package"
	}

	// Collect statistics and categorize packages
	stats := api.ProjectStats{}
	categories := make(map[string][]string)
	mainPackages := 0
	testPackages := 0

	for _, mps := range md.ForPackagePath {
		if len(mps) == 0 {
			continue
		}
		mp := mps[0]

		// Skip packages not in this module
		if mp.Module == nil || mp.Module.Path != modulePath {
			continue
		}

		pkgPath := string(mp.PkgPath)

		// Count test packages
		if strings.HasSuffix(string(mp.Name), "_test") {
			testPackages++
		} else if mp.Name == "main" {
			mainPackages++
		}

		// Categorize packages
		category := categorizePackage(pkgPath)
		categories[category] = append(categories[category], pkgPath)
	}

	stats.TotalPackages = len(md.ForPackagePath) - testPackages
	stats.MainPackages = mainPackages
	stats.TestPackages = testPackages

	// Count dependencies
	stats.Dependencies = 0
	if len(modFiles) > 0 {
		for _, modURI := range modFiles {
			fh, err := snapshot.ReadFile(ctx, modURI)
			if err != nil {
				continue
			}
			pmf, err := snapshot.ParseMod(ctx, fh)
			if err != nil {
				continue
			}
			if pmf.File != nil {
				stats.Dependencies = len(pmf.File.Require)
			}
		}
	}

	// Find entry points
	entryPoints := findEntryPointsForGetStarted(ctx, snapshot, md, modulePath, categories)

	// Build next steps
	nextSteps := buildNextSteps(identity, stats, categories)

	result := &api.OGetStarted{
		Identity:    identity,
		Stats:       stats,
		EntryPoints: entryPoints,
		Categories:  categories,
		NextSteps:   nextSteps,
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: formatGetStarted(result)}}}, result, nil
}

// categorizePackage categorizes a package by its purpose.
// TODO: is this necessary, i think give a category is out of current scope.
func categorizePackage(pkgPath string) string {
	switch {
	case strings.HasPrefix(pkgPath, "cmd/"):
		return "CLI commands"
	case strings.Contains(pkgPath, "/server") || strings.Contains(pkgPath, "/api") || strings.Contains(pkgPath, "/handler"):
		return "API & Server"
	case strings.Contains(pkgPath, "/internal/"):
		return "Internal packages"
	case strings.Contains(pkgPath, "/test/"):
		return "Testing"
	case strings.HasSuffix(pkgPath, "/mcp") || strings.Contains(pkgPath, "/mcp/"):
		return "MCP tools"
	case strings.Contains(pkgPath, "/cache") || strings.Contains(pkgPath, "/session"):
		return "Core infrastructure"
	case strings.Contains(pkgPath, "/analysis"):
		return "Analysis"
	case strings.Contains(pkgPath, "/protocol"):
		return "LSP protocol"
	case strings.Contains(pkgPath, "/util"):
		return "Utilities"
	default:
		return "Other"
	}
}

// findEntryPointsForGetStarted finds suggested entry points for exploration.
func findEntryPointsForGetStarted(ctx context.Context, snapshot *cache.Snapshot, md *metadata.Graph, modulePath string, categories map[string][]string) []api.GuideEntryPoint {
	entryPoints := []api.GuideEntryPoint{} // Initialize as empty slice to avoid null in JSON

	// Find main packages
	for _, mps := range md.ForPackagePath {
		if len(mps) == 0 {
			continue
		}
		mp := mps[0]

		// Skip packages not in this module
		if mp.Module == nil || mp.Module.Path != modulePath {
			continue
		}

		if mp.Name == "main" {
			pkgPath := string(mp.PkgPath)
			file := ""
			if len(mp.CompiledGoFiles) > 0 {
				file = mp.CompiledGoFiles[0].Path()
			}

			entryPoints = append(entryPoints, api.GuideEntryPoint{
				Category:    "main",
				Path:        pkgPath,
				File:        file,
				Description: fmt.Sprintf("Main package: %s", pkgPath),
			})
		}
	}

	// Add core infrastructure entry points
	if corePkgs, ok := categories["Core infrastructure"]; ok && len(corePkgs) > 0 {
		for _, pkg := range corePkgs[:min(2, len(corePkgs))] {
			entryPoints = append(entryPoints, api.GuideEntryPoint{
				Category:    "core",
				Path:        pkg,
				Description: fmt.Sprintf("Core: %s", pkg),
			})
		}
	}

	// Add API entry points
	if apiPkgs, ok := categories["API & Server"]; ok && len(apiPkgs) > 0 {
		for _, pkg := range apiPkgs[:min(2, len(apiPkgs))] {
			entryPoints = append(entryPoints, api.GuideEntryPoint{
				Category:    "api",
				Path:        pkg,
				Description: fmt.Sprintf("API: %s", pkg),
			})
		}
	}

	return entryPoints
}

// buildNextSteps builds recommended next steps for exploration.
func buildNextSteps(identity api.ProjectIdentity, stats api.ProjectStats, categories map[string][]string) []string {
	var steps []string

	steps = append(steps, "→ Use go_search to find specific symbols or types")
	steps = append(steps, "→ Use list_module_packages with exclude_tests=true for a clean package overview")

	if stats.MainPackages > 0 {
		steps = append(steps, fmt.Sprintf("→ Explore the %d main package(s) to understand entry points", stats.MainPackages))
	}

	if corePkgs, ok := categories["Core infrastructure"]; ok && len(corePkgs) > 0 {
		steps = append(steps, fmt.Sprintf("→ Check core packages like %s for key data structures", corePkgs[0]))
	}

	steps = append(steps, "→ Use go_build_check to check for compilation errors")
	steps = append(steps, "→ Use list_package_symbols to explore a package's API surface")

	return steps
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatDependencyGraph(result *api.ODependencyGraphResult) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Package: %s (%s)\n\n", result.PackagePath, result.PackageName)

	// Dependencies section
	if result.TotalDependencies > 0 {
		fmt.Fprintf(&b, "Dependencies (%d):\n", result.TotalDependencies)
		for _, dep := range result.Dependencies {
			indent := ""
			if dep.Depth > 0 {
				indent = strings.Repeat("  ", dep.Depth)
			}
			fmt.Fprintf(&b, "  %s%s (%s)", indent, dep.Path, dep.Name)
			if dep.IsStdlib {
				fmt.Fprintf(&b, " [stdlib]")
			}
			if dep.IsExternal {
				fmt.Fprintf(&b, " [external]")
			}
			if dep.ModulePath != "" {
				fmt.Fprintf(&b, " from %s", dep.ModulePath)
			}
			fmt.Fprintln(&b)
		}
		fmt.Fprintln(&b)
	} else {
		fmt.Fprintf(&b, "Dependencies: None\n\n")
	}

	// Dependents section
	if result.TotalDependents > 0 {
		fmt.Fprintf(&b, "Imported By (%d):\n", result.TotalDependents)
		for _, dep := range result.Dependents {
			fmt.Fprintf(&b, "  %s (%s)", dep.Path, dep.Name)
			if dep.IsTest {
				fmt.Fprintf(&b, " [test]")
			}
			if dep.ModulePath != "" {
				fmt.Fprintf(&b, " from %s", dep.ModulePath)
			}
			fmt.Fprintln(&b)
		}
		fmt.Fprintln(&b)
	} else {
		fmt.Fprintf(&b, "Imported By: None\n\n")
	}

	// Summary
	if result.Summary != "" {
		fmt.Fprintf(&b, "Summary: %s\n", result.Summary)
	}

	return b.String()
}

// handleGetDependencyGraph returns the dependency graph for a package.
// Uses: snapshot.LoadMetadataGraph() from gopls/internal/cache
func handleGetDependencyGraph(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.IDependencyGraphParams) (*mcp.CallToolResult, *api.ODependencyGraphResult, error) {
	view, err := h.getView(input.Cwd)
	if err != nil {
		return nil, nil, err
	}

	snapshot, release, err := view.Snapshot()
	if err != nil {
		return nil, nil, err
	}
	defer release()

	md, err := snapshot.LoadMetadataGraph(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load metadata graph: %v", err)
	}

	// Determine target package path
	targetPkgPath := input.PackagePath
	if targetPkgPath == "" {
		// Use main module's root package
		modFiles := view.ModFiles()
		if len(modFiles) == 0 {
			return nil, nil, fmt.Errorf("no go.mod files found in view")
		}
		for _, modURI := range modFiles {
			fh, err := snapshot.ReadFile(ctx, modURI)
			if err != nil {
				continue
			}
			pmf, err := snapshot.ParseMod(ctx, fh)
			if err != nil {
				continue
			}
			if pmf.File != nil && pmf.File.Module != nil {
				targetPkgPath = pmf.File.Module.Mod.Path
				break
			}
		}
		if targetPkgPath == "" {
			return nil, nil, fmt.Errorf("failed to determine main module path")
		}
	}

	// Find the target package
	pkgPath := metadata.PackagePath(targetPkgPath)
	mps := md.ForPackagePath[pkgPath]
	if len(mps) == 0 {
		return nil, nil, fmt.Errorf("package not found: %s", targetPkgPath)
	}

	// Use the first (best) package variant
	mp := mps[0]

	// Get main module path for identifying external dependencies
	mainModulePath := ""
	modFiles := view.ModFiles()
	if len(modFiles) > 0 {
		for _, modURI := range modFiles {
			fh, err := snapshot.ReadFile(ctx, modURI)
			if err != nil {
				continue
			}
			pmf, err := snapshot.ParseMod(ctx, fh)
			if err != nil {
				continue
			}
			if pmf.File != nil && pmf.File.Module != nil {
				mainModulePath = pmf.File.Module.Mod.Path
				break
			}
		}
	}

	// Collect dependencies
	dependencies := []api.PackageDependency{}
	seen := make(map[string]bool) // track visited packages for transitive traversal
	collectDependencies(mp, md, mainModulePath, &dependencies, seen, input.IncludeTransitive, input.MaxDepth, 0)

	// Collect dependents (packages that import this package)
	dependents := []api.PackageDependent{}
	pkgID := mp.ID
	for _, dependent := range md.ImportedBy[pkgID] {
		// Skip intermediate test variants
		if dependent.IsIntermediateTestVariant() {
			continue
		}
		depPath := string(dependent.PkgPath)
		dependents = append(dependents, api.PackageDependent{
			Path:       depPath,
			Name:       string(dependent.Name),
			ModulePath: getModulePath(dependent),
			IsTest:     strings.HasSuffix(string(dependent.Name), "_test"),
		})
	}

	result := &api.ODependencyGraphResult{
		PackagePath:       targetPkgPath,
		PackageName:       string(mp.Name),
		Dependencies:      dependencies,
		Dependents:        dependents,
		TotalDependencies: len(dependencies),
		TotalDependents:   len(dependents),
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: formatDependencyGraph(result)}}}, result, nil
}

// collectDependencies recursively collects dependencies for a package.
func collectDependencies(mp *metadata.Package, md *metadata.Graph, mainModulePath string, deps *[]api.PackageDependency, seen map[string]bool, includeTransitive bool, maxDepth, currentDepth int) {
	// Check if we should stop at max depth
	if maxDepth > 0 && currentDepth >= maxDepth {
		return
	}

	// Collect direct dependencies from DepsByPkgPath
	for depPath, depID := range mp.DepsByPkgPath {
		depPathStr := string(depPath)

		// Skip if already seen
		if seen[depPathStr] {
			continue
		}
		seen[depPathStr] = true

		// Get the dependency package
		depPkg := md.Packages[depID]
		if depPkg == nil {
			continue
		}

		// Skip intermediate test variants
		if depPkg.IsIntermediateTestVariant() {
			continue
		}

		// Determine if this is stdlib or external
		isStdlib := isStdlibPackage(depPathStr)
		isExternal := !isStdlib && mainModulePath != "" && !strings.HasPrefix(depPathStr, mainModulePath)

		*deps = append(*deps, api.PackageDependency{
			Path:       depPathStr,
			Name:       string(depPkg.Name),
			ModulePath: getModulePath(depPkg),
			IsStdlib:   isStdlib,
			IsExternal: isExternal,
			Depth:      currentDepth,
		})

		// Recursively collect transitive dependencies if requested
		if includeTransitive {
			collectDependencies(depPkg, md, mainModulePath, deps, seen, includeTransitive, maxDepth, currentDepth+1)
		}
	}
}

// getModulePath returns the module path for a package.
func getModulePath(mp *metadata.Package) string {
	if mp.Module != nil {
		return mp.Module.Path
	}
	return ""
}

// isStdlibPackage checks if a package is part of the Go standard library.
func isStdlibPackage(pkgPath string) bool {
	// Standard library packages have no dot in their path
	// or are in well-known stdlib prefixes
	if !strings.Contains(pkgPath, ".") {
		return true
	}
	// Some special cases
	stdlibPrefixes := []string{"golang.org/x/"}
	for _, prefix := range stdlibPrefixes {
		if strings.HasPrefix(pkgPath, prefix) {
			return true
		}
	}
	return false
}

// ===== Helper Functions =====

// sortModulesByOverlap sorts modules by their overlap with the main module.
// The main module stays first. Other modules are sorted by the length of their
// common prefix with the main module path (descending).
// This groups related modules together, e.g., "github.com/user/project-sub" near "github.com/user/project".
func sortModulesByOverlap(modules []api.ModuleInfo, mainModulePath string) {
	// Separate main module from dependencies
	mainIdx := -1
	deps := make([]api.ModuleInfo, 0, len(modules)-1)

	for i, mod := range modules {
		if mod.Main {
			mainIdx = i
		} else {
			deps = append(deps, mod)
		}
	}

	// Sort dependencies by overlap with main module path
	sort.Slice(deps, func(i, j int) bool {
		overlapI := commonPrefixLength(mainModulePath, deps[i].Path)
		overlapJ := commonPrefixLength(mainModulePath, deps[j].Path)

		// Primary sort: by overlap length (more overlap = higher priority)
		if overlapI != overlapJ {
			return overlapI > overlapJ
		}

		// Secondary sort: alphabetically by path for consistent ordering
		return deps[i].Path < deps[j].Path
	})

	// Rebuild modules list with main first, then sorted dependencies
	if mainIdx >= 0 {
		result := make([]api.ModuleInfo, 0, len(modules))
		result = append(result, modules[mainIdx])
		result = append(result, deps...)
		copy(modules, result)
	}
}

// commonPrefixLength returns the length of the common prefix between two strings.
func commonPrefixLength(a, b string) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return minLen
}

// isInternal determines if a module is part of the workspace (maintained by the current project).
// A module is considered internal if:
// 1. It's the main module itself
// 2. It's a submodule (path starts with root module path, e.g., github.com/foo/bar-api under github.com/foo/bar)
// 3. It's a local replace (monorepo pattern, path starts with . or / or is absolute Windows path)
func isInternal(mod api.ModuleInfo, rootModulePath string) bool {
	// Rule 1: Main module is always internal
	if mod.Main {
		return true
	}

	// Rule 2: Prefix matching for submodules
	// e.g., root: github.com/foo/bar, dep: github.com/foo/bar/baz -> internal
	if strings.HasPrefix(mod.Path, rootModulePath) {
		// Ensure we match at module boundary (either exact match or followed by /)
		if len(mod.Path) == len(rootModulePath) || mod.Path[len(rootModulePath)] == '/' {
			return true
		}
	}

	// Rule 3: Local replace (monorepo / k8s style)
	// Note: If a module has a FilePath, it means it was replaced to a local path
	if mod.FilePath != "" {
		return true
	}

	return false
}

// isLocalPath checks if a path is a local filesystem path (not a module path).
// Local paths start with:
// - . (relative path)
// - / (absolute Unix path)
// - Windows absolute paths (e.g., C:\)
func isLocalPath(path string) bool {
	if path == "" {
		return false
	}
	// Relative path
	if strings.HasPrefix(path, ".") {
		return true
	}
	// Absolute Unix path
	if strings.HasPrefix(path, "/") {
		return true
	}
	// Windows absolute path (e.g., C:\, D:\)
	if len(path) > 1 && path[1] == ':' {
		return true
	}
	return false
}

// extractPackageInfo extracts package information from gopls metadata.
// Uses: golang.DocumentSymbols() from gopls/internal/golang/symbols.go:23
func extractPackageInfo(ctx context.Context, snapshot *cache.Snapshot, mp *metadata.Package, includeSymbols bool) api.Package {
	pkg := api.Package{
		Name: string(mp.Name),    // Convert metadata.PackageName to string
		Path: string(mp.PkgPath), // Convert metadata.PackagePath to string
		Docs: extractPackageDocs(ctx, snapshot, mp),
	}

	// Module info
	if mp.Module != nil {
		pkg.ModuleName = mp.Module.Path
		pkg.ModuleVersion = mp.Module.Version
	}

	if !includeSymbols {
		return pkg
	}

	// Extract symbols from package files using golang.DocumentSymbols
	for _, uri := range mp.CompiledGoFiles {
		fh, err := snapshot.ReadFile(ctx, uri)
		if err != nil {
			continue
		}

		symbols, err := golang.DocumentSymbols(ctx, snapshot, fh)
		if err != nil {
			continue
		}

		for _, sym := range symbols {
			if isExported(sym.Name) {
				pkg.Symbols = append(pkg.Symbols, convertDocumentSymbol(sym, uri.Path(), string(mp.PkgPath)))
			}
		}
	}

	return pkg
}

// extractPackageDocs extracts package-level documentation from AST.
// Uses: snapshot.ParseGo() from gopls/internal/cache with parsego.Header mode
// Based on: gopls/internal/cache/parsego/parse.go (line 37-44)
func extractPackageDocs(ctx context.Context, snapshot *cache.Snapshot, mp *metadata.Package) string {
	if len(mp.CompiledGoFiles) == 0 {
		return ""
	}

	for _, uri := range mp.CompiledGoFiles {
		fh, err := snapshot.ReadFile(ctx, uri)
		if err != nil {
			continue
		}

		// Use parsego.Header mode to get package docs without full AST
		pgf, err := snapshot.ParseGo(ctx, fh, parsego.Header)
		if err != nil {
			continue
		}

		if pgf.File.Doc != nil {
			return string(pgf.File.Doc.Text())
		}
	}

	return ""
}

// convertDocumentSymbol converts LSP DocumentSymbol to our MCP Symbol type.
// Based on: gopls/internal/protocol/document_symbols.go
func convertDocumentSymbol(ds protocol.DocumentSymbol, filePath, packagePath string) api.Symbol {
	// Extract receiver and parent info from the symbol detail/name
	receiver, parent := "", ""
	detail := ds.Detail

	// For methods, try multiple approaches to extract receiver
	// Approach 1: Parse from detail string (format: "func (*Type)Method(params...) return")
	if ds.Kind == protocol.Method {
		// Try extracting from detail first
		if detail != "" {
			// Extract receiver from detail like "func (*Type)MethodName(...)"
			if idx := strings.Index(detail, ")"); idx != -1 {
				if startIdx := strings.Index(detail, "("); startIdx != -1 && startIdx < idx {
					receiver = strings.TrimSpace(detail[startIdx+1 : idx])
				}
			}
		}

		// Approach 2: If detail doesn't have receiver, try parsing from name (some LSP versions use "(Type).Method" format)
		cleanName := ds.Name
		if receiver == "" && strings.Contains(ds.Name, ")") {
			if idx := strings.Index(ds.Name, ")"); idx != -1 {
				if startIdx := strings.Index(ds.Name, "("); startIdx != -1 && startIdx < idx {
					receiver = strings.TrimSpace(ds.Name[startIdx+1 : idx])
					// Strip the receiver prefix from the name: "(*Person).Greeting" -> "Greeting"
					if idx+1 < len(ds.Name) && ds.Name[idx+1] == '.' {
						cleanName = ds.Name[idx+2:]
					}
				}
			}
		}

		// Update ds.Name to use the cleaned name without receiver prefix
		ds.Name = cleanName
	}

	// For fields, try to infer parent from the symbol hierarchy
	// This is a simplification - the parent would ideally come from DocumentSymbol.Parent
	if ds.Kind == protocol.Field {
		// Parent would be set from the enclosing struct
		// For now, leave empty as LSP DocumentSymbol doesn't always provide this directly
	}

	sym := api.Symbol{
		Name:        ds.Name,
		Kind:        convertSymbolKind(ds.Kind),
		PackagePath: packagePath,
		Signature:   detail,
		Receiver:    receiver,
		Parent:      parent,
		FilePath:    filePath,
		Line:        int(ds.Range.Start.Line),
		// Note: Doc and Body are set separately from AST in handleListPackageSymbols
	}

	return sym
}

// convertSymbolKind converts LSP SymbolKind to MCP SymbolKind.
// Based on: gopls/internal/protocol/protocol.go SymbolKind constants
func convertSymbolKind(kind protocol.SymbolKind) api.SymbolKind {
	switch kind {
	case protocol.Function:
		return api.SymbolKindFunction
	case protocol.Method:
		return api.SymbolKindMethod
	case protocol.Struct:
		return api.SymbolKindStruct
	case protocol.Interface:
		return api.SymbolKindInterface
	case protocol.Variable:
		return api.SymbolKindVariable
	case protocol.Constant:
		return api.SymbolKindConstant
	case protocol.Field:
		return api.SymbolKindField
	default:
		return api.SymbolKindType
	}
}

// isExported checks if a name is exported (starts with uppercase letter).
// Handles method names with receivers like "(Counter).Increment".
func isExported(name string) bool {
	if len(name) == 0 {
		return false
	}
	// Handle method names with receivers: "(Type).MethodName"
	if name[0] == '(' {
		// Find the closing parenthesis and get the method name after it
		for i := 0; i < len(name); i++ {
			if name[i] == ')' && i+2 < len(name) && name[i+1] == '.' {
				methodName := name[i+2:]
				return len(methodName) > 0 && methodName[0] >= 'A' && methodName[0] <= 'Z'
			}
		}
		return false
	}
	return name[0] >= 'A' && name[0] <= 'Z'
}

// ===== Formatting Functions =====

func formatModulesList(result *api.OListModules) string {
	var b strings.Builder

	// Summary section
	fmt.Fprintf(&b, "Module Summary\n")
	fmt.Fprintf(&b, "  Root: %s\n", result.Summary.RootModule)
	fmt.Fprintf(&b, "  Total: %d (Internal: %d, External: %d)\n\n",
		result.Summary.TotalModules, result.Summary.InternalCount, result.Summary.ExternalCount)

	// Internal modules section
	fmt.Fprintf(&b, "Internal Modules (%d):\n", len(result.InternalModules))
	for _, mod := range result.InternalModules {
		if mod.Main {
			fmt.Fprintf(&b, "  [MAIN] %s", mod.Path)
		} else {
			fmt.Fprintf(&b, "  %s", mod.Path)
		}
		if mod.Version != "" {
			fmt.Fprintf(&b, " @ %s", mod.Version)
		}
		if mod.FilePath != "" {
			fmt.Fprintf(&b, " -> %s", mod.FilePath)
		}
		if mod.Replaces != "" {
			fmt.Fprintf(&b, " (replaces %s)", mod.Replaces)
		}
		fmt.Fprintln(&b)
	}

	// External modules section
	fmt.Fprintf(&b, "\nExternal Modules (%d):\n", len(result.ExternalModules))
	for _, mod := range result.ExternalModules {
		fmt.Fprintf(&b, "  %s", mod.Path)
		if mod.Version != "" {
			fmt.Fprintf(&b, " @ %s", mod.Version)
		}
		if mod.Indirect {
			fmt.Fprintf(&b, " (indirect)")
		}
		if mod.Replaces != "" {
			fmt.Fprintf(&b, " (replaces %s)", mod.Replaces)
		}
		fmt.Fprintln(&b)
	}

	return b.String()
}

func formatModulePackages(result *api.OListModulePackages) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Module: %s (%d packages):\n", result.ModulePath, len(result.Packages))
	for _, pkg := range result.Packages {
		fmt.Fprintf(&b, "  %s (%s)", pkg.Path, pkg.Name)
		if pkg.Docs != "" {
			// Truncate docs if too long
			docs := pkg.Docs
			if len(docs) > 80 {
				docs = docs[:77] + "..."
			}
			fmt.Fprintf(&b, " - %s", docs)
		}
		fmt.Fprintln(&b)
	}
	return b.String()
}

func formatPackageSymbols(result *api.OListPackageSymbols, includeDocs, includeBodies bool) string {
	var b strings.Builder
	suffix := ""
	if includeDocs {
		suffix = " with docs"
	}
	if includeBodies {
		if suffix != "" {
			suffix += " and bodies"
		} else {
			suffix = " with bodies"
		}
	}
	fmt.Fprintf(&b, "Package: %s%s (%d symbols):\n", result.PackagePath, suffix, len(result.Symbols))
	for _, sym := range result.Symbols {
		fmt.Fprintf(&b, "  %s %s", sym.Kind, sym.Name)
		if sym.Signature != "" {
			fmt.Fprintf(&b, " - %s", sym.Signature)
		}
		if includeDocs && sym.Doc != "" {
			// Truncate docs if too long
			docs := sym.Doc
			if len(docs) > 80 {
				docs = docs[:77] + "..."
			}
			fmt.Fprintf(&b, " // %s", docs)
		}
		if includeBodies && sym.Body != "" {
			// Show body on the same line for compactness
			body := sym.Body
			if len(body) > 60 {
				body = body[:57] + "..."
			}
			fmt.Fprintf(&b, " = %s", body)
		}
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}

func formatGetStarted(result *api.OGetStarted) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Getting Started with %s\n\n", result.Identity.Name)
	fmt.Fprintf(&b, "## Project\n\n")
	fmt.Fprintf(&b, "  **Type:** %s\n", result.Identity.Type)
	if result.Identity.Description != "" {
		fmt.Fprintf(&b, "  **Description:** %s\n", result.Identity.Description)
	}
	if result.Identity.GoVersion != "" || result.Identity.GoRuntimeVersion != "" {
		if result.Identity.GoRuntimeVersion != "" {
			fmt.Fprintf(&b, "  **Go Version:** %s (runtime: %s)\n", result.Identity.GoVersion, result.Identity.GoRuntimeVersion)
		} else {
			fmt.Fprintf(&b, "  **Go Version:** %s\n", result.Identity.GoVersion)
		}
	}
	fmt.Fprintf(&b, "  **Root:** %s\n\n", result.Identity.Root)

	fmt.Fprintf(&b, "## Quick Stats\n\n")
	fmt.Fprintf(&b, "  **Total Packages:** %d\n", result.Stats.TotalPackages)
	fmt.Fprintf(&b, "  **Main Packages:** %d\n", result.Stats.MainPackages)
	fmt.Fprintf(&b, "  **Test Packages:** %d\n", result.Stats.TestPackages)
	fmt.Fprintf(&b, "  **Dependencies:** %d\n\n", result.Stats.Dependencies)

	if len(result.EntryPoints) > 0 {
		fmt.Fprintf(&b, "## Suggested Entry Points\n\n")
		for _, ep := range result.EntryPoints {
			fmt.Fprintf(&b, "  **[%s]** %s\n", strings.ToTitle(ep.Category), ep.Description)
			if ep.Path != "" {
				fmt.Fprintf(&b, "    Path: %s\n", ep.Path)
			}
			if ep.File != "" {
				fmt.Fprintf(&b, "    File: %s\n", ep.File)
			}
			fmt.Fprintln(&b)
		}
	}

	if len(result.Categories) > 0 {
		fmt.Fprintf(&b, "## Package Categories\n\n")
		// Sort categories by name for consistency
		cats := make([]string, 0, len(result.Categories))
		for name := range result.Categories {
			cats = append(cats, name)
		}
		sort.Strings(cats)

		for _, cat := range cats {
			pkgs := result.Categories[cat]
			fmt.Fprintf(&b, "  **%s** (%d)\n", cat, len(pkgs))
			// Show up to 3 packages per category
			for i, pkg := range pkgs {
				if i >= 3 {
					fmt.Fprintf(&b, "    ... and %d more\n", len(pkgs)-3)
					break
				}
				fmt.Fprintf(&b, "    - %s\n", pkg)
			}
			fmt.Fprintln(&b)
		}
	}

	if len(result.NextSteps) > 0 {
		fmt.Fprintf(&b, "## Next Steps\n\n")
		for _, step := range result.NextSteps {
			fmt.Fprintf(&b, "  %s\n", step)
		}
		fmt.Fprintln(&b)
	}

	return b.String()
}
