package core

import (
	"context"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/internal/cache"
	"golang.org/x/tools/gopls/internal/cache/metadata"
	goplsmcp "golang.org/x/tools/gopls/internal/mcp"
	"golang.org/x/tools/gopls/internal/protocol"
	"golang.org/x/tools/gopls/internal/settings"
	"golang.org/x/tools/gopls/mcpbridge/api"
)

// InitFunc creates the gopls session, symbler, and file watcher on first use.
// The returned io.Closer is the file watcher; may be nil if watching failed.
type InitFunc func(ctx context.Context) (*cache.Session, Symbler, io.Closer, error)

// Handler implements gopls-mcp MCP tools using gopls's existing APIs.
// Resources are initialized lazily on first tool call and released after
// idleTimeout of inactivity.
type Handler struct {
	// initFn creates gopls session + watcher on first tool call.
	initFn InitFunc
	// config holds the gopls-mcp configuration (response limits, etc.)
	config      *MCPConfig
	idleTimeout time.Duration
	// options holds gopls configuration for dynamic view creation (test mode).
	options *settings.Options

	// lazy-initialized resources, protected by initMu.
	initMu  sync.Mutex
	session *cache.Session
	symbler Symbler
	watcher io.Closer
	timer   *time.Timer

	// allowDynamicViews enables creating new gopls views on-demand for e2e testing.
	allowDynamicViews bool
	// dynamicViews tracks dynamically created views for cleanup in test mode.
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

// WithOptions sets gopls options used for dynamic view creation (test mode).
func WithOptions(opts *settings.Options) HandlerOption {
	return func(h *Handler) {
		h.options = opts
	}
}

// WithDynamicViews enables dynamic view creation for e2e testing.
// TEST-ONLY: This allows the handler to create new gopls views on-demand when
// a Cwd parameter doesn't match any existing view.
func WithDynamicViews(allow bool) HandlerOption {
	return func(h *Handler) {
		h.allowDynamicViews = allow
	}
}

type Symbler interface {
	Symbol(ctx context.Context, params *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error)
}

// NewHandler creates a new Handler that initializes gopls lazily on first tool call.
func NewHandler(initFn InitFunc, opts ...HandlerOption) *Handler {
	h := &Handler{
		initFn:       initFn,
		options:      settings.DefaultOptions(),
		config:       DefaultConfig(),
		dynamicViews: make(map[string]func()),
	}
	for _, opt := range opts {
		opt(h)
	}
	if d, err := time.ParseDuration(h.config.IdleTimeout); err == nil && d > 0 {
		h.idleTimeout = d
	} else {
		h.idleTimeout = 5 * time.Minute
	}
	return h
}

// ensureSession initializes the gopls session if not already done.
func (h *Handler) ensureSession(ctx context.Context) error {
	h.initMu.Lock()
	defer h.initMu.Unlock()
	if h.session != nil {
		return nil
	}
	session, symbler, watcher, err := h.initFn(context.Background())
	if err != nil {
		return fmt.Errorf("gopls initialization failed: %w", err)
	}
	h.session = session
	h.symbler = symbler
	h.watcher = watcher
	h.timer = time.AfterFunc(h.idleTimeout, h.shutdownResources)
	log.Printf("[gopls-mcp] Session initialized (idle timeout: %v)", h.idleTimeout)
	return nil
}

// resetIdleTimer postpones the idle shutdown by another idleTimeout duration.
func (h *Handler) resetIdleTimer() {
	h.initMu.Lock()
	defer h.initMu.Unlock()
	if h.timer != nil {
		h.timer.Reset(h.idleTimeout)
	}
}

// shutdownResources releases all gopls resources.
func (h *Handler) shutdownResources() {
	h.initMu.Lock()
	defer h.initMu.Unlock()
	if h.watcher != nil {
		h.watcher.Close()
		h.watcher = nil
	}
	if h.session != nil {
		h.session.Shutdown(context.Background())
		h.session = nil
		h.symbler = nil
	}
	h.timer = nil
	log.Printf("[gopls-mcp] All resources released (idle timeout reached)")
}

// snapshot returns the best default snapshot for workspace queries.
func (h *Handler) snapshot() (*cache.Snapshot, func(), error) {
	if h.session == nil {
		return nil, nil, fmt.Errorf("no active session")
	}
	views := h.session.Views()
	if len(views) == 0 {
		return nil, nil, fmt.Errorf("no active views")
	}
	return views[0].Snapshot()
}

// snapshotForDir returns a snapshot for the given directory, or the default snapshot if dir is empty.
func (h *Handler) snapshotForDir(dir string) (*cache.Snapshot, func(), error) {
	view, err := h.getView(dir)
	if err != nil {
		return nil, nil, err
	}
	return view.Snapshot()
}

// viewForDir finds the view that contains the given directory.
// If allowDynamicViews is enabled (TEST MODE), creates a new view if no existing view contains it.
func (h *Handler) viewForDir(dir string) (*cache.View, error) {
	dir = filepath.Clean(dir)

	views := h.session.Views()
	for _, v := range views {
		root := v.Root().Path()
		if strings.HasPrefix(dir, root) || dir == root {
			return v, nil
		}
	}

	if !h.allowDynamicViews {
		return nil, fmt.Errorf("no view found for directory %s", dir)
	}

	ctx := context.Background()
	dirURI := protocol.URIFromPath(dir)

	goEnv, err := cache.FetchGoEnv(ctx, dirURI, h.options)
	if err != nil {
		return nil, fmt.Errorf("failed to load Go env for %s: %w", dir, err)
	}

	folder := &cache.Folder{
		Dir:     dirURI,
		Options: h.options,
		Env:     *goEnv,
	}

	view, _, releaseView, err := h.session.NewView(ctx, folder)
	if err != nil {
		return nil, fmt.Errorf("failed to create view for %s: %w", dir, err)
	}

	h.dynamicViewsMu.Lock()
	h.dynamicViews[dir] = releaseView
	h.dynamicViewsMu.Unlock()

	return view, nil
}

// getView returns a view for the given directory, or the first available view if dir is empty.
func (h *Handler) getView(dir string) (*cache.View, error) {
	if dir != "" {
		return h.viewForDir(dir)
	}
	if h.session == nil {
		return nil, fmt.Errorf("no active session")
	}
	views := h.session.Views()
	if len(views) == 0 {
		return nil, fmt.Errorf("no views available")
	}
	return views[0], nil
}

// ===== go_get_dependency_graph =====

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
		return nil, nil, fmt.Errorf("failed to load metadata graph: %w", err)
	}

	// Determine target package path
	targetPkgPath := input.PackagePath
	if targetPkgPath == "" {
		modFiles := view.ModFiles()
		if len(modFiles) == 0 {
			return nil, nil, fmt.Errorf("no go.mod files found in view")
		}
		for _, modURI := range modFiles {
			modPath, err := goplsmcp.ModulePath(ctx, snapshot, modURI)
			if err == nil {
				targetPkgPath = modPath
				break
			}
		}
		if targetPkgPath == "" {
			return nil, nil, fmt.Errorf("failed to determine main module path")
		}
	}

	pkgPath := metadata.PackagePath(targetPkgPath)
	mps := md.ForPackagePath[pkgPath]
	if len(mps) == 0 {
		return nil, nil, fmt.Errorf("package not found: %s", targetPkgPath)
	}

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

	dependencies := []api.PackageDependency{}
	seen := make(map[string]bool)
	collectDependencies(mp, md, mainModulePath, &dependencies, seen, input.IncludeTransitive, input.MaxDepth, 0)

	dependents := []api.PackageDependent{}
	pkgID := mp.ID
	for _, dependent := range md.ImportedBy[pkgID] {
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
	if maxDepth > 0 && currentDepth >= maxDepth {
		return
	}

	for depPath, depID := range mp.DepsByPkgPath {
		depPathStr := string(depPath)

		if seen[depPathStr] {
			continue
		}
		seen[depPathStr] = true

		depPkg := md.Packages[depID]
		if depPkg == nil {
			continue
		}

		if depPkg.IsIntermediateTestVariant() {
			continue
		}

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
	if !strings.Contains(pkgPath, ".") {
		return true
	}
	stdlibPrefixes := []string{"golang.org/x/"}
	for _, prefix := range stdlibPrefixes {
		if strings.HasPrefix(pkgPath, prefix) {
			return true
		}
	}
	return false
}

func formatDependencyGraph(result *api.ODependencyGraphResult) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Package: %s (%s)\n\n", result.PackagePath, result.PackageName)

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

	if result.Summary != "" {
		fmt.Fprintf(&b, "Summary: %s\n", result.Summary)
	}

	return b.String()
}
