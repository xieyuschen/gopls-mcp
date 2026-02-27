package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/internal/cache"
	goplsmcp "golang.org/x/tools/gopls/internal/mcp"
	"golang.org/x/tools/gopls/mcpbridge/api"
)

// ===== analyze_workspace Tool =====
// Provides comprehensive workspace discovery and analysis

const (
	// defaultMaxPackages is the default maximum number of packages to return from analyze_workspace.
	defaultMaxPackages = 50
	// defaultMaxEntryPoints is the default maximum number of entry points to return.
	defaultMaxEntryPoints = 20
	// defaultMaxSummaryDependencies is the default number of dependencies to show in summary.
	defaultMaxSummaryDependencies = 15
)

// todo: replace go doc is important, seems now i don't have a good guide.
func handleAnalyzeWorkspace(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.IAnalyzeWorkspaceParams) (*mcp.CallToolResult, *api.OAnalyzeWorkspaceResult, error) {
	startTime := time.Now()

	view, err := h.getView(input.Cwd)
	if err != nil {
		return nil, nil, err
	}

	snapshot, release, err := view.Snapshot()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get snapshot: %v", err)
	}
	defer release()

	// Determine project type
	projectType := determineProjectType(view)

	// Discover all packages
	allPackages, err := discoverPackages(ctx, snapshot, view)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to discover packages: %v", err)
	}

	// Find all entry points
	// todo: the entry point findings are wrong, quite raw.
	allEntryPoints := findEntryPoints(ctx, snapshot, allPackages)

	// Get module dependencies
	allDependencies, err := getModuleDependencies(ctx, snapshot, view)
	if err != nil {
		// Non-fatal error, continue without dependencies
		allDependencies = []api.Module{}
	}

	// Build result with all data first
	result := &api.OAnalyzeWorkspaceResult{
		Packages:          allPackages,
		EntryPoints:       allEntryPoints,
		Dependencies:      allDependencies,
		ProjectType:       projectType,
		TotalPackages:     len(allPackages),
		TotalEntryPoints:  len(allEntryPoints),
		TotalDependencies: len(allDependencies),
	}

	// Build diagnostics
	diagnostics := api.AnalysisDiagnostics{
		WorkspaceDiagnostics: api.WorkspaceDiagnostics{
			AnalysisDuration: time.Since(startTime),
		},
		ProjectType:      projectType,
		ModulesFound:     len(allDependencies),
		EntryPointsFound: len(allEntryPoints),
	}

	pkgs := snapshot.WorkspacePackages()
	indexedPackages := 0
	pkgs.All()(func(id cache.PackageID, path cache.PackagePath) bool {
		indexedPackages++
		return true
	})
	diagnostics.IndexedPackages = indexedPackages
	diagnostics.TotalPackages = indexedPackages

	if indexedPackages > 0 {
		diagnostics.CacheStatus = "full"
	} else {
		diagnostics.CacheStatus = "empty"
	}
	result.Diagnostics = diagnostics

	// TODO: the logic should be unified instead of here?
	// Estimate response size and truncate if needed
	maxBytes := h.config.MaxResponseBytes
	if maxBytes == 0 {
		maxBytes = defaultMaxResponseBytes // Default (32KB)
	}

	// Build summary first to see size
	summary := buildWorkspaceSummary(view, projectType, result.Packages, result.EntryPoints, result.Dependencies, defaultMaxSummaryDependencies)
	estimatedSize := len(summary)

	// Target size: 80% of max to leave room for metadata
	targetSize := (maxBytes * 80) / 100

	// Calculate how many packages/entry points we can return
	maxPackages := defaultMaxPackages
	maxEntryPoints := defaultMaxEntryPoints
	maxSummaryDeps := defaultMaxSummaryDependencies

	// Iteratively reduce limits until we fit within budget
	for estimatedSize > targetSize && (maxPackages > 10 || maxEntryPoints > 10) {
		if maxPackages > 10 {
			maxPackages = maxPackages * 3 / 4
		}
		if maxEntryPoints > 10 {
			maxEntryPoints = maxEntryPoints * 3 / 4
		}
		if maxSummaryDeps > 5 {
			maxSummaryDeps = maxSummaryDeps * 3 / 4
		}

		// Rebuild summary with new limits
		summary = buildWorkspaceSummary(view, projectType,
			allPackages[:min(len(allPackages), maxPackages)],
			allEntryPoints[:min(len(allEntryPoints), maxEntryPoints)],
			allDependencies, maxSummaryDeps)
		estimatedSize = utf8.RuneCountInString(summary)
	}

	// Apply final limits
	truncated := false
	var hints []string

	if len(allPackages) > maxPackages {
		result.Packages = allPackages[:maxPackages]
		truncated = true
		hints = append(hints, fmt.Sprintf("• Showing %d of %d packages (use list_module_packages for full list)", maxPackages, len(allPackages)))
	}

	if len(allEntryPoints) > maxEntryPoints {
		result.EntryPoints = allEntryPoints[:maxEntryPoints]
		truncated = true
		hints = append(hints, fmt.Sprintf("• Showing %d of %d entry points (use get_started for full exploration)", maxEntryPoints, len(allEntryPoints)))
	}

	// Rebuild summary with final limits
	summary = buildWorkspaceSummary(view, projectType, result.Packages, result.EntryPoints, result.Dependencies, maxSummaryDeps)

	if truncated {
		result.Truncated = true
		result.Hint = "Response truncated to fit within size limits:\n" + strings.Join(hints, "\n")
	}

	result.Summary = summary

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}}, result, nil
}

// determineProjectType determines the project type from the view.
func determineProjectType(view *cache.View) string {
	switch view.Type() {
	case cache.GoModView:
		return "module"
	case cache.GoWorkView:
		return "workspace"
	case cache.GOPATHView:
		return "gopath"
	case cache.AdHocView:
		return "adhoc"
	default:
		return "unknown"
	}
}

// discoverPackages discovers all packages in the workspace.
func discoverPackages(ctx context.Context, snapshot *cache.Snapshot, view *cache.View) ([]api.WorkspacePackage, error) {
	packages := []api.WorkspacePackage{} // Initialize as empty slice to avoid null in JSON

	md, err := snapshot.LoadMetadataGraph(ctx)
	if err != nil {
		return nil, err
	}

	rootPath := view.Root().Path()

	// Iterate through all packages
	for _, mps := range md.ForPackagePath {
		if len(mps) == 0 {
			continue
		}

		mp := mps[0] // first is best

		// Check if package is in workspace
		inWorkspace := false
		for _, uri := range mp.CompiledGoFiles {
			if strings.HasPrefix(uri.Path(), rootPath) {
				inWorkspace = true
				break
			}
		}

		if !inWorkspace {
			continue
		}

		// Extract directory from first compiled Go file
		pkgDir := ""
		if len(mp.CompiledGoFiles) > 0 {
			pkgDir = filepath.Dir(mp.CompiledGoFiles[0].Path())
		}

		pkg := api.WorkspacePackage{
			Path:   string(mp.PkgPath),
			Name:   string(mp.Name),
			Dir:    pkgDir,
			IsMain: mp.Name == "main",
		}

		// Add module info
		if mp.Module != nil {
			pkg.ModulePath = mp.Module.Path
		}

		// Check for test files
		for _, uri := range mp.CompiledGoFiles {
			if strings.HasSuffix(uri.Path(), "_test.go") {
				pkg.HasTests = true
				break
			}
		}

		packages = append(packages, pkg)
	}

	return packages, nil
}

// findEntryPoints identifies suggested entry points for exploring the codebase.
func findEntryPoints(_ context.Context, _ *cache.Snapshot, packages []api.WorkspacePackage) []api.EntryPoint {
	entryPoints := []api.EntryPoint{} // Initialize as empty slice to avoid null in JSON

	// Find main packages
	for _, pkg := range packages {
		if pkg.IsMain {
			// Look for main.go file
			mainFile := filepath.Join(pkg.Dir, "main.go")
			entryPoints = append(entryPoints, api.EntryPoint{
				File:        mainFile,
				Description: fmt.Sprintf("Main package entry point (%s)", pkg.Path),
				Type:        "main",
			})
		}

		// Find test files
		if pkg.HasTests {
			testFile := filepath.Join(pkg.Dir, pkg.Name+"_test.go")
			entryPoints = append(entryPoints, api.EntryPoint{
				File:        testFile,
				Description: fmt.Sprintf("Test file for %s package", pkg.Name),
				Type:        "test",
			})
		}
	}

	// Find API packages (common HTTP/API patterns)
	for _, pkg := range packages {
		if strings.Contains(pkg.Path, "/api/") ||
			strings.Contains(pkg.Path, "/http/") ||
			strings.Contains(pkg.Path, "/server/") ||
			strings.Contains(pkg.Path, "/handler/") ||
			strings.HasSuffix(pkg.Path, "/api") {
			// Find first Go file in the package
			for _, ext := range []string{".go"} {
				pattern := filepath.Join(pkg.Dir, "*"+ext)
				entryPoints = append(entryPoints, api.EntryPoint{
					File:        pattern,
					Description: fmt.Sprintf("API package: %s", pkg.Path),
					Type:        "api",
				})
				break
			}
		}
	}

	return entryPoints
}

// getModuleDependencies extracts module dependencies from go.mod.
func getModuleDependencies(ctx context.Context, snapshot *cache.Snapshot, view *cache.View) ([]api.Module, error) {
	modules := []api.Module{} // Initialize as empty slice to avoid null in JSON

	for _, modURI := range view.ModFiles() {
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

		// Main module - use gopls's ModulePath utility
		modPath, err := goplsmcp.ModulePath(ctx, snapshot, modURI)
		if err != nil {
			continue
		}

		modules = append(modules, api.Module{
			Path:  modPath,
			Main:  true,
			Dir:   filepath.Dir(modURI.Path()),
			GoMod: modURI.Path(),
		})

		// Requirements
		for _, req := range pmf.File.Require {
			modules = append(modules, api.Module{
				Path:     req.Mod.Path,
				Version:  req.Mod.Version,
				Main:     false,
				Indirect: req.Indirect,
			})
		}
	}

	return modules, nil
}

// buildWorkspaceSummary creates a human-readable workspace summary.
func buildWorkspaceSummary(view *cache.View, projectType string, packages []api.WorkspacePackage, entryPoints []api.EntryPoint, dependencies []api.Module, maxDeps int) string {
	var b strings.Builder

	rootPath := view.Root().Path()

	fmt.Fprintf(&b, "# Workspace Analysis\n\n")
	fmt.Fprintf(&b, "**Project Type:** %s\n", projectType)
	fmt.Fprintf(&b, "**Root:** %s\n\n", rootPath)

	// Package summary
	fmt.Fprintf(&b, "## Packages (%d total)\n\n", len(packages))

	// Group packages by module
	modules := make(map[string][]api.WorkspacePackage)
	for _, pkg := range packages {
		modPath := pkg.ModulePath
		if modPath == "" {
			modPath = "(unknown)"
		}
		modules[modPath] = append(modules[modPath], pkg)
	}

	for modPath, pkgs := range modules {
		fmt.Fprintf(&b, "### Module: %s (%d packages)\n", modPath, len(pkgs))
		for _, pkg := range pkgs {
			extra := ""
			if pkg.IsMain {
				extra = " [main]"
			}
			if pkg.HasTests {
				extra += " [tests]"
			}
			fmt.Fprintf(&b, "  - %s %s\n", pkg.Path, extra)
		}
		fmt.Fprintln(&b)
	}

	// Entry points
	if len(entryPoints) > 0 {
		fmt.Fprintf(&b, "## Suggested Entry Points (%d)\n\n", len(entryPoints))
		for i, ep := range entryPoints {
			fmt.Fprintf(&b, "%d. **%s**: %s\n", i+1, ep.Type, ep.Description)
			fmt.Fprintf(&b, "   File: %s\n\n", ep.File)
		}
	}

	// Dependencies
	if len(dependencies) > 0 {
		// Count main vs dependencies
		mainCount := 0
		depCount := 0
		for _, mod := range dependencies {
			if mod.Main {
				mainCount++
			} else {
				depCount++
			}
		}

		fmt.Fprintf(&b, "## Dependencies (%d modules", depCount)
		if depCount > maxDeps {
			fmt.Fprintf(&b, ", showing first %d", maxDeps)
		}
		fmt.Fprintf(&b, ")\n\n")

		if depCount > 0 {
			fmt.Fprintf(&b, "**Direct dependencies:**\n")
			shown := 0
			for _, mod := range dependencies {
				if !mod.Main {
					indirect := ""
					if mod.Indirect {
						indirect = " (indirect)"
					}
					fmt.Fprintf(&b, "  - %s@%s%s\n", mod.Path, mod.Version, indirect)
					shown++
					if shown >= maxDeps {
						break
					}
				}
			}
			if depCount > maxDeps {
				fmt.Fprintf(&b, "  ... and %d more (use list_modules to see all)\n", depCount-maxDeps)
			}
		}
		fmt.Fprintln(&b)
	}

	return b.String()
}
