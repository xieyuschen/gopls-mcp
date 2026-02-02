package core

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/internal/cache"
	"golang.org/x/tools/gopls/internal/cache/metadata"
	"golang.org/x/tools/gopls/internal/cache/parsego"
	"golang.org/x/tools/gopls/internal/golang"
	"golang.org/x/tools/gopls/internal/protocol"
	"golang.org/x/tools/gopls/internal/util/safetoken"
	"golang.org/x/tools/gopls/mcpbridge/api"
)

// This file contains wrapper handlers for the existing gopls MCP tools.
// These wrap the original implementations from gopls/internal/mcp to provide
// a unified interface through the gopls-mcp Handler pattern.
//
// Origin: gopls/internal/mcp/*.go handlers are wrapped here

// ===== get_package_symbol_detail =====
// Origin: gopls/internal/mcp/outline.go outlineHandler()

func handleGetPackageSymbolDetail(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.IGetPackageSymbolDetailParams) (*mcp.CallToolResult, *api.OGetPackageSymbolDetailResult, error) {
	// Require filters - this is a precision tool, not a "list all" tool
	if len(input.SymbolFilters) == 0 {
		return nil, nil, fmt.Errorf("symbol_filters is required for get_package_symbol_detail (this is a precision tool). Use list_package_symbols to get all symbols in a package")
	}
	var snapshot *cache.Snapshot
	var release func()
	var err error

	// Use Cwd if provided, otherwise use default view
	if input.Cwd != "" {
		view, err := h.viewForDir(input.Cwd)
		if err != nil {
			return nil, nil, err
		}
		snapshot, release, err = view.Snapshot()
		if err != nil {
			return nil, nil, err
		}
		defer release()
	} else {
		snapshot, release, err = h.snapshot()
		if err != nil {
			return nil, nil, err
		}
		defer release()
	}

	md, err := snapshot.LoadMetadataGraph(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load metadata: %v", err)
	}

	// Get the single package
	pkgPath := metadata.PackagePath(input.PackagePath)
	mps := md.ForPackagePath[pkgPath]
	if len(mps) == 0 {
		return nil, nil, fmt.Errorf("package not found: %s", input.PackagePath)
	}
	mp := mps[0] // first is best

	// Extract symbols from the package
	symbols := []api.Symbol{}
	includeDocs := input.IncludeDocs
	includeBodies := input.IncludeBodies

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

	// Apply symbol filters (validated to be non-empty at function start)
	filteredSymbols := filterSymbols(symbols, input.SymbolFilters)

	result := &api.OGetPackageSymbolDetailResult{
		Symbols: filteredSymbols,
	}

	// Format for human-readable output
	formatted := formatPackageSymbolDetail(result, includeDocs, includeBodies)
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: formatted}}}, result, nil
}

// filterSymbols filters symbols based on the provided filters.
// For methods: both Receiver and Name must match (exact string match)
// For non-methods: only Name must match (Receiver is ignored)
//
// IMPORTANT: This function assumes filters is non-empty.
// The caller (handleGetPackageSymbolDetail) validates this before calling.
func filterSymbols(symbols []api.Symbol, filters []api.SymbolFilter) []api.Symbol {

	result := []api.Symbol{}
	for _, sym := range symbols {
		for _, filter := range filters {
			// Check if name matches
			if sym.Name != filter.Name {
				continue
			}

			// For methods, receiver must also match
			if sym.Kind == api.SymbolKindMethod {
				if filter.Receiver != "" && sym.Receiver == filter.Receiver {
					result = append(result, sym)
					break // matched, don't check other filters
				} else if filter.Receiver == "" {
					// If no receiver specified, match any method with this name
					result = append(result, sym)
					break
				}
			} else {
				// For non-methods, just match on name
				result = append(result, sym)
				break
			}
		}
	}
	return result
}

// formatPackageSymbolDetail formats symbols for human-readable output.
func formatPackageSymbolDetail(result *api.OGetPackageSymbolDetailResult, includeDocs, includeBodies bool) string {
	var b strings.Builder

	// Build header
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
	if suffix == "" {
		suffix = " (signatures only)"
	}

	b.WriteString(fmt.Sprintf("Symbols (%d)%s:\n", len(result.Symbols), suffix))

	// Format each symbol
	for _, sym := range result.Symbols {
		// Kind and name
		kind := strings.ToLower(string(sym.Kind))
		if sym.Receiver != "" && sym.Kind == api.SymbolKindMethod {
			b.WriteString(fmt.Sprintf("  %s (%s).%s - %s\n", kind, sym.Receiver, sym.Name, sym.Signature))
		} else {
			b.WriteString(fmt.Sprintf("  %s %s - %s\n", kind, sym.Name, sym.Signature))
		}

		// Documentation
		if includeDocs && sym.Doc != "" {
			docLines := strings.Split(sym.Doc, "\n")
			for _, line := range docLines {
				b.WriteString(fmt.Sprintf("    %s\n", line))
			}
		}

		// Body
		if includeBodies && sym.Body != "" {
			b.WriteString(fmt.Sprintf("   = { %s }\n", sym.Body))
		}

		// File location
		if sym.FilePath != "" {
			b.WriteString(fmt.Sprintf("    at %s:%d\n", sym.FilePath, sym.Line))
		}

		b.WriteString("\n")
	}

	return b.String()
}

// formatPackageSymbolsForAPI formats symbols for the package API tool output.
func formatPackageSymbolsForAPI(result *api.OListPackageSymbols, includeDocs, includeBodies bool) string {
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
		if sym.Receiver != "" {
			fmt.Fprintf(&b, " [%s]", sym.Receiver)
		}
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

// ===== go_build_check =====
// Origin: gopls/internal/mcp/workspace_diagnostics.go workspaceDiagnosticsHandler()

func handleGoDiagnostics(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.IDiagnosticsParams) (*mcp.CallToolResult, *api.ODiagnosticsResult, error) {
	var snapshot *cache.Snapshot
	var release func()
	var err error

	// Use Cwd if provided, otherwise use default view
	if input.Cwd != "" {
		view, err := h.viewForDir(input.Cwd)
		if err != nil {
			return nil, nil, err
		}
		snapshot, release, err = view.Snapshot()
		if err != nil {
			return nil, nil, err
		}
		defer release()
	} else {
		snapshot, release, err = h.snapshot()
		if err != nil {
			return nil, nil, err
		}
		defer release()
	}

	// Ensure metadata is loaded. This is critical for populating the workspace.
	if _, err := snapshot.LoadMetadataGraph(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to load metadata: %v", err)
	}

	// Get workspace package IDs
	pkgMap := snapshot.WorkspacePackages()
	var ids []cache.PackageID
	for id := range pkgMap.All() {
		ids = append(ids, id)
	}

	// Get diagnostics (returns map[URI][]diagnostics)
	reports, err := snapshot.PackageDiagnostics(ctx, ids...)
	if err != nil {
		return nil, nil, fmt.Errorf("diagnostics failed: %v", err)
	}

	// Deduplicate diagnostics using native gopls hash
	// This matches the exact deduplication behavior of native gopls
	seen := make(map[string]struct{})
	var diagnostics []api.Diagnostic
	var summary strings.Builder

	// Iterate by file URI (like native gopls)
	for _, diags := range reports {
		if len(diags) == 0 {
			continue
		}

		// Deduplicate and collect diagnostics for this file
		for _, diag := range diags {
			// Use native gopls hash for exact deduplication matching
			// Hash includes: Range, Severity, Source, Code, Message, Tags, Related, BundledFixes
			key := diag.Hash().String()
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}

			// Convert DiagnosticSeverity to string
			severityStr := "Unknown"
			switch diag.Severity {
			case 1:
				severityStr = "Error"
			case 2:
				severityStr = "Warning"
			case 3:
				severityStr = "Information"
			case 4:
				severityStr = "Hint"
			}

			// Extract code snippet at diagnostic location
			codeSnippet := ""
			if fh, err := snapshot.ReadFile(ctx, diag.URI); err == nil {
				if content, err := fh.Content(); err == nil && content != nil {
					lines := strings.Split(string(content), "\n")
					lineIdx := int(diag.Range.Start.Line)
					if lineIdx >= 0 && lineIdx < len(lines) {
						codeSnippet = strings.TrimSpace(lines[lineIdx])
					}
				}
			}

			diagnostics = append(diagnostics, api.Diagnostic{
				File:        diag.URI.Path(),
				Severity:    severityStr,
				Message:     diag.Message,
				Line:        int(diag.Range.Start.Line) + 1,
				Column:      int(diag.Range.Start.Character) + 1,
				CodeSnippet: codeSnippet,
			})
		}
	}

	// Format summary (per file, like native gopls)
	if len(diagnostics) == 0 {
		summary.WriteString(fmt.Sprintf("Workspace diagnostics checked for %d packages. No issues found.", len(ids)))
	} else {
		summary.WriteString(fmt.Sprintf("Found %d unique diagnostic(s):\n", len(diagnostics)))
		for _, diag := range diagnostics {
			locInfo := fmt.Sprintf("%s:%d:%d", diag.File, diag.Line, diag.Column)
			if diag.CodeSnippet != "" {
				summary.WriteString(fmt.Sprintf("- %s: %s\n  Code: %s\n  [%s]\n", locInfo, diag.Message, diag.CodeSnippet, diag.Severity))
			} else {
				summary.WriteString(fmt.Sprintf("- %s: %s (%s)\n", locInfo, diag.Message, diag.Severity))
			}
		}
	}

	result := &api.ODiagnosticsResult{
		Summary:     summary.String(),
		Diagnostics: diagnostics,
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary.String()}}}, result, nil
}

// ===== go_search =====
// Origin: gopls/internal/mcp/search.go searchHandler()

func handleGoSearch(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.ISearchParams) (*mcp.CallToolResult, *api.OSearchResult, error) {
	// Handle empty query gracefully
	if input.Query == "" {
		result := &api.OSearchResult{
			Summary: "No symbols found - empty query",
			Symbols: []*api.Symbol{},
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: result.Summary}}}, result, nil
	}

	// Validate query: go_search only accepts symbol names (no spaces or dots)
	// This prevents natural language queries which don't work with symbol search
	if strings.Contains(input.Query, " ") || strings.Contains(input.Query, ".") {
		return nil, nil, fmt.Errorf("invalid query: go_search accepts only a single symbol name (no spaces or dots). query=%q", input.Query)
	}

	var snapshot *cache.Snapshot
	var release func()
	var err error

	// Use Cwd if provided, otherwise use default view
	if input.Cwd != "" {
		view, err := h.viewForDir(input.Cwd)
		if err != nil {
			return nil, nil, err
		}
		snapshot, release, err = view.Snapshot()
		if err != nil {
			return nil, nil, err
		}
		defer release()
	} else {
		snapshot, release, err = h.snapshot()
		if err != nil {
			return nil, nil, err
		}
		defer release()
	}

	symbols, err := searchProjectFiles(ctx, snapshot, input.Query, input.MaxResults)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to search project files: %v", err)
	}

	// Build summary
	summary := buildSearchSummary(symbols, input.MaxResults)

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}}, &api.OSearchResult{
		Summary: summary,
		Symbols: symbols,
	}, nil
}

// searchProjectFiles searches for symbols in workspace files by parsing them directly.
// This bypasses the LSP server and parseCache for better memory efficiency.
//
// Strategy:
// 1. Get workspace packages from snapshot (metadata graph already filtered)
// 2. Collect Go files from workspace packages only (no dependencies)
// 3. Filter out ignored files (testdata, hidden files, etc.)
// 4. Parse each file directly using cache.ParseGoImpl (no caching)
// 5. Extract symbols and fuzzy match against query
// 6. Return top matches
func searchProjectFiles(ctx context.Context, snapshot *cache.Snapshot, query string, maxResults int) ([]*api.Symbol, error) {
	if maxResults <= 0 {
		maxResults = 10
	}

	root := snapshot.View().Root().Path()
	fileURIs := collectGoFilesFromFS(root)
	filteredURIs := filterIgnoredFiles(snapshot, fileURIs)
	symbols := extractSymbolsFromFiles(ctx, snapshot, filteredURIs)
	matched := fuzzyMatchSymbols(symbols, query)
	matched = sortAndLimitSymbols(matched, maxResults)

	return matched, nil
}

// collectGoFilesFromFS walks the filesystem to find all .go files in the workspace.
// This bypasses the gopls snapshot's lazy-loaded metadata graph.
// Note: Filtering is done later by filterIgnoredFiles() which uses snapshot.IgnoredFile().
func collectGoFilesFromFS(rootPath string) map[protocol.DocumentURI]bool {
	seenFiles := make(map[protocol.DocumentURI]bool)

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.Mode()&os.ModeSymlink != 0 {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Ext(path) == ".go" {
			uri := protocol.URIFromPath(path)
			seenFiles[uri] = true
		}

		return nil
	})

	if err != nil {
		log.Printf("warning: error walking directory tree: %v", err)
	}

	return seenFiles
}

// filterIgnoredFiles filters out files that should be ignored per Go conventions:
// - Files in testdata directories
// - Files starting with '.' or '_' in any path segment
// - Files filtered by user's directory filters
func filterIgnoredFiles(snapshot *cache.Snapshot, uris map[protocol.DocumentURI]bool) []protocol.DocumentURI {
	var filtered []protocol.DocumentURI

	for uri := range uris {
		if snapshot.IgnoredFile(uri) {
			continue
		}

		filtered = append(filtered, uri)
	}

	return filtered
}

// extractSymbolsFromFiles parses files and extracts symbols.
// Each file is parsed, symbols extracted, then AST is discarded (GC-friendly).
// Processes files concurrently using a worker pool for better performance.
func extractSymbolsFromFiles(ctx context.Context, snapshot *cache.Snapshot, uris []protocol.DocumentURI) []*api.Symbol {
	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8
	}

	type result struct {
		symbols []*api.Symbol
	}

	results := make(chan result, len(uris))
	urisChan := make(chan protocol.DocumentURI, len(uris))

	for _, uri := range uris {
		urisChan <- uri
	}
	close(urisChan)

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fset := token.NewFileSet()

			for uri := range urisChan {
				if ctx.Err() != nil {
					return
				}

				fh, err := snapshot.ReadFile(ctx, uri)
				if err != nil {
					continue
				}

				pgf, err := cache.ParseGoImpl(ctx, fset, fh, 0, false)
				if err != nil {
					continue
				}

				fileSymbols := extractTopLevelSymbols(pgf)
				results <- result{symbols: fileSymbols}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allSymbols []*api.Symbol
	for res := range results {
		allSymbols = append(allSymbols, res.symbols...)
	}

	return allSymbols
}

// extractTopLevelSymbols walks the AST and extracts top-level declarations.
// This includes functions, types, variables, and constants.
func extractTopLevelSymbols(pgf *parsego.File) []*api.Symbol {
	var symbols []*api.Symbol

	for _, decl := range pgf.File.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			// Extract function
			symbols = append(symbols, &api.Symbol{
				Name:     d.Name.Name,
				Kind:     api.SymbolKindFunction,
				FilePath: pgf.URI.Path(),
				Line:     int(pgf.Tok.Line(d.Pos())),
			})

		case *ast.GenDecl:
			// Extract types, variables, constants
			tok := d.Tok
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					// Type declaration - determine kind based on type
					kind := determineTypeKind(s)
					symbols = append(symbols, &api.Symbol{
						Name:     s.Name.Name,
						Kind:     kind,
						FilePath: pgf.URI.Path(),
						Line:     int(pgf.Tok.Line(s.Pos())),
					})

				case *ast.ValueSpec:
					// Variable or constant declaration
					kind := api.SymbolKindVariable
					if tok == token.CONST {
						kind = api.SymbolKindConstant
					}

					for _, name := range s.Names {
						symbols = append(symbols, &api.Symbol{
							Name:     name.Name,
							Kind:     kind,
							FilePath: pgf.URI.Path(),
							Line:     int(pgf.Tok.Line(name.Pos())),
						})
					}
				}
			}
		}
	}

	return symbols
}

// determineTypeKind determines the SymbolKind for a TypeSpec
func determineTypeKind(spec *ast.TypeSpec) api.SymbolKind {
	if spec.Type == nil {
		return api.SymbolKindType
	}

	switch spec.Type.(type) {
	case *ast.StructType:
		return api.SymbolKindStruct
	case *ast.InterfaceType:
		return api.SymbolKindInterface
	default:
		// For type aliases and other types
		return api.SymbolKindType
	}
}

// fuzzyMatchSymbols performs fuzzy matching on symbol names against the query.
// Uses a simple substring match for now (can be enhanced with better fuzzy matching).
func fuzzyMatchSymbols(symbols []*api.Symbol, query string) []*api.Symbol {
	if query == "" {
		return symbols
	}

	query = strings.ToLower(query)
	var matched []*api.Symbol

	for _, sym := range symbols {
		if strings.Contains(strings.ToLower(sym.Name), query) {
			matched = append(matched, sym)
		}
	}

	return matched
}

// sortAndLimitSymbols sorts symbols by relevance and limits to maxResults.
// For now, simple alphabetical sorting (can be enhanced with relevance scoring).
// Returns the sorted and limited slice.
func sortAndLimitSymbols(symbols []*api.Symbol, maxResults int) []*api.Symbol {
	sortSymbols(symbols)

	if len(symbols) > maxResults {
		// Clear references to symbols beyond maxResults for GC
		for i := maxResults; i < len(symbols); i++ {
			symbols[i] = nil
		}
		return symbols[:maxResults]
	}
	return symbols
}

func sortSymbols(symbols []*api.Symbol) {
	for i := 1; i < len(symbols); i++ {
		key := symbols[i]
		j := i - 1
		for j >= 0 && symbols[j].Name > key.Name {
			symbols[j+1] = symbols[j]
			j--
		}
		symbols[j+1] = key
	}
}

// buildSearchSummary builds a human-readable summary of search results.
func buildSearchSummary(symbols []*api.Symbol, maxResults int) string {
	if len(symbols) == 0 {
		return "No symbols found."
	}

	var summary string
	if len(symbols) > maxResults {
		summary = fmt.Sprintf("Found %d symbol(s) (showing first %d):\n", len(symbols), maxResults)
	} else {
		summary = fmt.Sprintf("Found %d symbol(s):\n", len(symbols))
	}

	for _, sym := range symbols {
		summary += fmt.Sprintf("  - %s (%s in %s:%d)\n", sym.Name, sym.Kind, sym.FilePath, sym.Line)
	}

	if len(symbols) > maxResults {
		summary += fmt.Sprintf("... and %d more (use max_results for more)\n", len(symbols)-maxResults)
	}

	return summary
}

// ===== go_definition =====
// Origin: gopls/internal/golang/definition.go Definition()

func handleGoDefinition(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.IDefinitionParams) (*mcp.CallToolResult, *api.ODefinitionResult, error) {
	// Get the view for the directory containing the context file
	// This is critical for cross-file definitions to work correctly
	dir := filepath.Dir(input.Locator.ContextFile)
	view, err := h.viewForDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get view for %s: %w", dir, err)
	}

	snapshot, release, err := view.Snapshot()
	if err != nil {
		return nil, nil, err
	}
	defer release()

	// Use the unified ResolveSymbol to get both locations and rich definition info
	info, err := golang.ResolveSymbol(ctx, snapshot, input.Locator, golang.ResolveOptions{
		FindDefinitions:   true,
		IncludeDefinition: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve symbol '%s': %v", input.Locator.SymbolName, err)
	}

	if len(info.Locations) == 0 {
		summary := fmt.Sprintf("No definition found for symbol '%s' in %s", input.Locator.SymbolName, input.Locator.ContextFile)
		result := &api.ODefinitionResult{Summary: summary}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}}, result, nil
	}

	// Convert SourceContext to api.Symbol
	var sym *api.Symbol
	if info.Definition != nil {
		srcCtx := info.Definition
		sym = &api.Symbol{
			Name:      srcCtx.Symbol,
			Kind:      api.SymbolKind(srcCtx.Kind),
			Signature: srcCtx.Signature,
			Doc:       srcCtx.DocComment,
			FilePath:  srcCtx.File,
			Line:      srcCtx.StartLine,
		}
		// Include body (snippet) if requested
		if input.IncludeBody {
			sym.Body = srcCtx.Snippet
		}
	}

	// Build summary
	loc := info.Locations[0]
	summary := fmt.Sprintf("Definition found at %s:%d", loc.URI.Path(), loc.Range.Start.Line+1)
	if len(info.Locations) > 1 {
		summary += fmt.Sprintf("\n(%d additional location(s) available)", len(info.Locations)-1)
	}
	if sym != nil {
		summary += golang.FormatSymbolSummary(sym)
	}

	result := &api.ODefinitionResult{
		Symbol:  sym,
		Summary: summary,
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}}, result, nil
}

// ===== go_symbol_references =====
// Origin: gopls/internal/mcp/symbol_references.go symbolReferencesHandler()

func handleGoSymbolReferences(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.ISymbolReferencesParams) (*mcp.CallToolResult, *api.OSymbolReferencesResult, error) {
	snapshot, release, err := h.snapshot()
	if err != nil {
		return nil, nil, err
	}
	defer release()

	// Read the context file
	uri := protocol.URIFromPath(input.Locator.ContextFile)
	fh, err := snapshot.ReadFile(ctx, uri)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file %s: %v", input.Locator.ContextFile, err)
	}

	// Resolve the symbol using the semantic bridge
	nodeResult, err := golang.ResolveNode(ctx, snapshot, fh, input.Locator)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve symbol '%s': %v", input.Locator.SymbolName, err)
	}

	// Get the package for the file to access the file set
	pkg, _, err := golang.NarrowestPackageForFile(ctx, snapshot, uri)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get package: %w", err)
	}

	// Convert token.Pos to protocol.Position
	posn := safetoken.StartPosition(pkg.FileSet(), nodeResult.Pos)
	if !posn.IsValid() {
		return nil, nil, fmt.Errorf("invalid position for symbol '%s'", input.Locator.SymbolName)
	}

	position := protocol.Position{
		Line:      uint32(posn.Line - 1),
		Character: uint32(posn.Column - 1),
	}

	// Call gopls's References function
	// includeDeclaration=false to exclude the definition itself
	locations, err := golang.References(ctx, snapshot, fh, position, false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find references: %v", err)
	}

	// Extract rich Symbol information for the referenced symbol
	// Use the original symbol's definition to get signature, docs, etc.
	var symbols []*api.Symbol
	if defLocs, err := golang.Definition(ctx, snapshot, fh, position); err == nil && len(defLocs) > 0 {
		// Extract symbol information at the definition location
		if sym := golang.ExtractSymbolAtDefinition(ctx, snapshot, defLocs[0], true); sym != nil {
			symbols = append(symbols, sym)
		}
	}

	// Build summary
	var summary strings.Builder
	if len(locations) == 0 {
		summary.WriteString(fmt.Sprintf("No references found for %q in %s",
			input.Locator.SymbolName, input.Locator.ContextFile))
	} else {
		summary.WriteString(fmt.Sprintf("Found %d reference(s) to %q:\n",
			len(locations), input.Locator.SymbolName))
		for i, loc := range locations {
			summary.WriteString(fmt.Sprintf("%d. %s:%d:%d",
				i+1, loc.URI.Path(), loc.Range.Start.Line+1, loc.Range.Start.Character+1))

			// Try to get context by reading the file at this location
			fh, err := snapshot.ReadFile(ctx, loc.URI)
			if err == nil {
				// Get the line of code containing the reference
				content, err := fh.Content()
				if err == nil {
					lines := strings.Split(string(content), "\n")
					lineIdx := int(loc.Range.Start.Line)
					if lineIdx >= 0 && lineIdx < len(lines) {
						line := strings.TrimSpace(lines[lineIdx])
						// Show the line of code for context
						if len(line) > 0 && len(line) < 100 {
							summary.WriteString(fmt.Sprintf("\n   %s", line))
						}
					}
				}
			}
			summary.WriteString("\n")
		}
		// Add referenced symbol details if available
		if len(symbols) > 0 {
			sym := symbols[0]
			if sym.Signature != "" {
				summary.WriteString(fmt.Sprintf("\nReferenced Symbol: %s\n", sym.Signature))
			}
			if sym.Doc != "" {
				summary.WriteString(fmt.Sprintf("Documentation: %s\n", sym.Doc))
			}
		}
	}

	result := &api.OSymbolReferencesResult{
		Summary:    summary.String(),
		Symbols:    symbols,
		TotalCount: len(locations),
		Returned:   len(locations),
		Truncated:  false,
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary.String()}}}, result, nil
}

// ===== go_dryrun_rename_symbol =====
// Origin: gopls/internal/mcp/rename_symbol.go renameSymbolHandler()
//
// Refactored to use SymbolLocator + semantic bridge (LLMRename)

func handleGoRenameSymbol(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.IRenameSymbolParams) (*mcp.CallToolResult, *api.ORenameSymbolResult, error) {
	snapshot, release, err := h.snapshot()
	if err != nil {
		return nil, nil, err
	}
	defer release()

	// Use the semantic bridge to generate both unified diff and line changes
	unifiedDiff, lineChanges, err := golang.LLMRename(ctx, snapshot, input.Locator, input.NewName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compute rename: %v", err)
	}

	// Build summary
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("DRY RUN: Preview rename %q to %q\n\n", input.Locator.SymbolName, input.NewName))
	summary.WriteString(unifiedDiff)

	result := &api.ORenameSymbolResult{
		Summary: summary.String(),
		Changes: lineChanges,
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary.String()}}}, result, nil
}

// ===== go_implementation =====
// Origin: gopls/internal/golang/implementation.go Implementation()
//
// Refactored to use SymbolLocator + semantic bridge (LLMImplementation)

func handleGoImplementation(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.IImplementationParams) (*mcp.CallToolResult, *api.OImplementationResult, error) {
	snapshot, release, err := h.snapshot()
	if err != nil {
		return nil, nil, err
	}
	defer release()

	// Use the semantic bridge to find implementations
	// LLMImplementation directly returns SourceContext with rich information
	sourceContexts, err := golang.LLMImplementation(ctx, snapshot, input.Locator)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find implementations for '%s': %v", input.Locator.SymbolName, err)
	}

	// Convert SourceContext to Symbol (rich information including location)
	symbols := make([]*api.Symbol, 0, len(sourceContexts))

	for _, srcCtx := range sourceContexts {
		sym := &api.Symbol{
			Name:      srcCtx.Symbol,
			Kind:      api.SymbolKind(srcCtx.Kind),
			Signature: srcCtx.Signature,
			FilePath:  srcCtx.File,
			Line:      srcCtx.StartLine,
			Doc:       srcCtx.DocComment,
		}
		// Include body (snippet) if requested
		if input.IncludeBody {
			sym.Body = srcCtx.Snippet
		}
		symbols = append(symbols, sym)
	}

	// Build summary with rich information from SourceContext
	var summary string
	if len(symbols) == 0 {
		summary = fmt.Sprintf("No implementations found for symbol '%s' in %s",
			input.Locator.SymbolName, input.Locator.ContextFile)
	} else {
		summary = fmt.Sprintf("Found %d implementation(s) for symbol '%s':\n",
			len(symbols), input.Locator.SymbolName)
		for i, sym := range symbols {
			summary += fmt.Sprintf("%d. %s at %s:%d",
				i+1, sym.Name, sym.FilePath, sym.Line)
			summary += golang.FormatSymbolSummary(sym)
			summary += "\n"
		}
	}

	result := &api.OImplementationResult{
		Symbols: symbols,
		Summary: summary,
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}}, result, nil
}

// ===== go_read_file =====
// Origin: NEW - reads file content using gopls snapshot
//
// CRITICAL: This uses snapshot.ReadFile() instead of os.ReadFile to ensure:
// 1. Content matches what gopls used for AST/type analysis
// 2. Line numbers match other tools (implementation, diagnostics, etc.)
// Note: Overlays (unsaved editor changes) are not currently supported by the MCP server

func handleGoReadFile(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.IReadFileParams) (*mcp.CallToolResult, *api.OReadFileResult, error) {
	uri := protocol.URIFromPath(input.File)
	snapshot, release, err := h.snapshot()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get snapshot: %v", err)
	}
	defer release()

	// Use snapshot.ReadFile to get file handle from gopls
	fh, err := snapshot.ReadFile(ctx, uri)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %v", err)
	}

	// Read content from disk
	contentBytes, err := fh.Content()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get file content: %v", err)
	}

	fullContent := string(contentBytes)
	totalLines := len(strings.Split(fullContent, "\n"))
	totalBytes := len(contentBytes)

	// Determine starting line (1-indexed, default to 1 if not specified or invalid)
	startLine := input.Offset
	if startLine < 1 {
		startLine = 1
	}

	// Apply truncation limits
	truncatedContent, _, truncationErr := TruncateFileContent(
		fullContent,
		input.MaxBytes,
		input.MaxLines,
		startLine,
	)

	// Build the result with truncated content
	result := &api.OReadFileResult{
		Content:    truncatedContent,
		TotalLines: totalLines,
		TotalBytes: totalBytes,
	}

	// Build summary message
	var summaryMsg string
	if truncationErr != "" {
		summaryMsg = fmt.Sprintf("Read %s: %s", input.File, truncationErr)
	} else {
		// Include offset information if starting from a line other than 1
		if startLine > 1 {
			summaryMsg = fmt.Sprintf("Read %s from line %d (%d bytes, %d total lines)",
				input.File, startLine, totalBytes, totalLines)
		} else {
			summaryMsg = fmt.Sprintf("Read %s (%d bytes, %d lines)",
				input.File, totalBytes, totalLines)
		}
	}

	// Build display content
	summary := fmt.Sprintf("%s\n%s", summaryMsg, truncatedContent)

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}}, result, nil
}

// handleListTools returns documentation for all available MCP tools.
// This allows AI agents to discover what tools are available and how to use them.
func handleListTools(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.IListToolsParams) (*mcp.CallToolResult, *api.OListToolsResult, error) {
	// Get all registered tools
	toolDocs := []api.ToolDocumentation{}

	for _, tool := range tools {
		// Extract tool details using reflection-like interface
		name, description := tool.Details()

		doc := api.ToolDocumentation{
			Name:        name,
			Description: description,
		}

		// Determine category based on tool name
		category := categorizeTool(name)
		doc.Category = category

		// Apply category filter if specified
		if input.CategoryFilter != "" && category != input.CategoryFilter {
			continue
		}

		// Add schemas if requested
		if input.IncludeInputSchema || input.IncludeOutputSchema {
			schemas := getToolSchemas(name)
			if input.IncludeInputSchema {
				doc.InputSchema = schemas["input"]
			}
			if input.IncludeOutputSchema {
				doc.OutputSchema = schemas["output"]
			}
		}

		toolDocs = append(toolDocs, doc)

	}

	result := &api.OListToolsResult{
		Tools: toolDocs,
		Count: len(toolDocs),
	}

	// Build summary with tool list
	summaryHeader := fmt.Sprintf("gopls-mcp provides %d tools for Go project analysis\n\n", len(toolDocs))

	// Group tools by category
	categories := make(map[string][]string)
	for _, doc := range toolDocs {
		categories[doc.Category] = append(categories[doc.Category], doc.Name)
	}

	// Build summary
	var summary strings.Builder
	summary.WriteString(summaryHeader)

	// List tools by category
	categoryOrder := []string{"meta", "environment", "analysis", "navigation", "refactoring", "information"}
	for _, cat := range categoryOrder {
		if tools, ok := categories[cat]; ok && len(tools) > 0 {
			summary.WriteString(fmt.Sprintf("%s:\n", strings.ToTitle(cat)))
			for _, toolName := range tools {
				summary.WriteString(fmt.Sprintf("  - %s\n", toolName))
			}
			summary.WriteString("\n")
		}
	}

	summary.WriteString("Use this tool with includeInputSchema=true and includeOutputSchema=true\n")
	summary.WriteString("to get detailed parameter schemas for each tool.")

	result.Summary = summary.String()

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary.String()}}}, result, nil
}

// categorizeTool returns a category for a tool based on its name.
func categorizeTool(name string) string {
	switch {
	// Meta tools
	case name == "go_list_tools":
		return "meta"

	// Environment
	case name == "get_go_env":
		return "environment"

	// Analysis
	case name == "go_build_check",
		name == "go_analyze_workspace",
		name == "go_get_dependency_graph":
		return "analysis"

	// Navigation
	case name == "go_search",
		name == "go_symbol_references",
		name == "go_implementation",
		name == "go_definition",
		name == "go_get_call_hierarchy":
		return "navigation"

	// Refactoring
	case name == "go_dryrun_rename_symbol":
		return "refactoring"

	// Information
	case strings.HasPrefix(name, "list_") || strings.HasPrefix(name, "fetch_"),
		strings.HasPrefix(name, "go_list_"),
		name == "go_get_package_symbol_detail",
		name == "go_read_file",
		name == "go_get_started":
		return "information"

	default:
		return "other"
	}
}

// getToolSchemas returns the input and output schemas for a tool.
// This is a simplified version that returns basic schema information.
func getToolSchemas(toolName string) map[string]map[string]any {
	schemas := map[string]map[string]any{}

	// Define schemas for each tool
	switch toolName {
	case "get_go_env":
		schemas["input"] = map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
		schemas["output"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"version": map[string]any{
					"type":        "string",
					"description": "Go version",
				},
				"goroot": map[string]any{
					"type":        "string",
					"description": "GOROOT path",
				},
				"gobin": map[string]any{
					"type":        "string",
					"description": "GOBIN path",
				},
			},
		}

	case "list_stdlib_packages":
		schemas["input"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"include_symbols": map[string]any{
					"type":        "boolean",
					"description": "Include exported symbols",
				},
			},
		}
		schemas["output"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"packages": map[string]any{
					"type":        "array",
					"description": "Standard library packages",
				},
			},
		}

	case "go_get_package_symbol_detail":
		schemas["input"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"package_path": map[string]any{
					"type":        "string",
					"description": "Go package import path",
				},
				"symbol_filters": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "object"},
					"description": "Filters for specific symbols (REQUIRED)",
				},
				"include_docs": map[string]any{
					"type":        "boolean",
					"description": "Include symbol documentation",
				},
				"include_bodies": map[string]any{
					"type":        "boolean",
					"description": "Include function implementations",
				},
				"Cwd": map[string]any{
					"type":        "string",
					"description": "Working directory for package resolution",
				},
			},
		}
		schemas["output"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"symbols": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "object"},
					"description": "Matching symbols with details",
				},
			},
		}

	case "go_search":
		schemas["input"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search query (fuzzy matching)",
				},
			},
		}
		schemas["output"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"results": map[string]any{
					"type":        "array",
					"description": "Matching symbols",
				},
			},
		}

	case "go_implementation":
		schemas["input"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file": map[string]any{
					"type":        "string",
					"description": "File path",
				},
				"line": map[string]any{
					"type":        "integer",
					"description": "Line number (1-indexed)",
				},
				"column": map[string]any{
					"type":        "integer",
					"description": "Column number (1-indexed, UTF-16)",
				},
			},
		}
		schemas["output"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"locations": map[string]any{
					"type":        "array",
					"description": "Implementation locations",
				},
			},
		}

	case "go_read_file":
		schemas["input"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file": map[string]any{
					"type":        "string",
					"description": "File path",
				},
				"max_bytes": map[string]any{
					"type":        "integer",
					"description": "Maximum bytes to return (0 = unlimited)",
				},
				"max_lines": map[string]any{
					"type":        "integer",
					"description": "Maximum lines to return (0 = unlimited)",
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "Starting line number (1-indexed, default: 1)",
				},
			},
		}
		schemas["output"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"content": map[string]any{
					"type":        "string",
					"description": "File content from disk",
				},
			},
		}

	case "go_list_tools":
		schemas["input"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"includeInputSchema": map[string]any{
					"type":        "boolean",
					"description": "Include input schemas",
				},
				"includeOutputSchema": map[string]any{
					"type":        "boolean",
					"description": "Include output schemas",
				},
			},
		}
		schemas["output"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tools": map[string]any{
					"type":        "array",
					"description": "Tool documentation",
				},
			},
		}

	default:
		// Return generic schemas for unknown tools
		schemas["input"] = map[string]any{
			"type":        "object",
			"description": "Tool-specific input parameters",
		}
		schemas["output"] = map[string]any{
			"type":        "object",
			"description": "Tool-specific output",
		}
	}

	return schemas
}

// ===== go_call_hierarchy =====
// New tool for call hierarchy analysis
//
// Refactored to use SymbolLocator + semantic bridge (ResolveNode)

func handleGoCallHierarchy(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.ICallHierarchyParams) (*mcp.CallToolResult, *api.OCallHierarchyResult, error) {
	var snapshot *cache.Snapshot
	var release func()
	var err error

	// Use Cwd if provided, otherwise use default view
	if input.Cwd != "" {
		view, err := h.viewForDir(input.Cwd)
		if err != nil {
			return nil, nil, err
		}
		snapshot, release, err = view.Snapshot()
		if err != nil {
			return nil, nil, err
		}
		defer release()
	} else {
		snapshot, release, err = h.snapshot()
		if err != nil {
			return nil, nil, err
		}
		defer release()
	}

	// Read the context file
	uri := protocol.URIFromPath(input.Locator.ContextFile)
	fh, err := snapshot.ReadFile(ctx, uri)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %v", err)
	}

	// Resolve the symbol using the semantic bridge
	nodeResult, err := golang.ResolveNode(ctx, snapshot, fh, input.Locator)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve symbol '%s': %v", input.Locator.SymbolName, err)
	}

	// Get the package for the file to access the file set
	pkg, _, err := golang.NarrowestPackageForFile(ctx, snapshot, uri)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get package: %w", err)
	}

	// Convert token.Pos to protocol.Position
	posn := safetoken.StartPosition(pkg.FileSet(), nodeResult.Pos)
	if !posn.IsValid() {
		return nil, nil, fmt.Errorf("invalid position for symbol '%s'", input.Locator.SymbolName)
	}

	position := protocol.Position{
		Line:      uint32(posn.Line - 1),
		Character: uint32(posn.Column - 1),
	}

	// Determine direction (default to "both")
	direction := input.Direction
	if direction == "" {
		direction = "both"
	}

	// Get the call hierarchy item for this position
	items, err := golang.PrepareCallHierarchy(ctx, snapshot, fh, position)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to prepare call hierarchy: %v", err)
	}

	if len(items) == 0 {
		summary := fmt.Sprintf("No function found for symbol '%s' in %s",
			input.Locator.SymbolName, input.Locator.ContextFile)
		result := &api.OCallHierarchyResult{Summary: summary}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}}, result, nil
	}

	item := items[0]

	// Extract package path for this symbol
	pkgPath := ""
	if mps, err := snapshot.MetadataForFile(ctx, item.URI, false); err == nil && len(mps) > 0 {
		pkgPath = string(mps[0].PkgPath)
	}

	// Extract rich symbol information using the existing helper
	// This gives us signature, documentation, receiver info, etc.
	loc := protocol.Location{
		URI:   item.URI,
		Range: item.Range,
	}
	richSymbol := golang.ExtractSymbolAtDefinition(ctx, snapshot, loc, false) // Don't include body by default for performance

	// Build the symbol - use rich info if available, otherwise fall back to basic info
	symbol := api.Symbol{
		Name:        item.Name,
		Kind:        golang.ConvertLSPSymbolKind(item.Kind),
		PackagePath: pkgPath,
		FilePath:    item.URI.Path(),
		Line:        int(item.Range.Start.Line + 1),
	}

	// Add rich details if extraction succeeded
	if richSymbol != nil && richSymbol.Name != "<symbol>" {
		symbol.Signature = richSymbol.Signature
		symbol.Doc = richSymbol.Doc
		symbol.Receiver = richSymbol.Receiver
		symbol.Body = richSymbol.Body
		// Use PackagePath from richSymbol if available (it may have extracted it from hover)
		if richSymbol.PackagePath != "" {
			symbol.PackagePath = richSymbol.PackagePath
		}
	}

	result := &api.OCallHierarchyResult{
		Symbol: symbol,
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Call hierarchy for %s at %s:%d\n\n", symbol.Name, symbol.FilePath, symbol.Line))

	// Get incoming calls (what calls this function)
	if direction == "incoming" || direction == "both" {
		incoming, err := golang.IncomingCalls(ctx, snapshot, fh, position)
		if err == nil && len(incoming) > 0 {
			incomingCalls := make([]api.CallHierarchyCall, 0, len(incoming))
			for _, call := range incoming {
				// Extract rich symbol information for the caller
				loc := protocol.Location{
					URI:   call.From.URI,
					Range: call.From.Range,
				}
				richSymbol := golang.ExtractSymbolAtDefinition(ctx, snapshot, loc, false) // Don't include body by default

				// Extract package path for this symbol
				pkgPath := ""
				if pkg, _, err := golang.NarrowestPackageForFile(ctx, snapshot, call.From.URI); err == nil && pkg != nil {
					pkgPath = string(pkg.Metadata().PkgPath)
				}

				// Build the symbol with rich info if available
				from := api.Symbol{
					Name:        call.From.Name,
					Kind:        golang.ConvertLSPSymbolKind(call.From.Kind),
					PackagePath: pkgPath,
					FilePath:    call.From.URI.Path(),
					Line:        int(call.From.Range.Start.Line + 1),
				}

				// Add rich details if extraction succeeded
				if richSymbol != nil && richSymbol.Name != "<symbol>" {
					from.Signature = richSymbol.Signature
					from.Doc = richSymbol.Doc
					from.Receiver = richSymbol.Receiver
					from.Body = richSymbol.Body
					// Use PackagePath from richSymbol if available
					if richSymbol.PackagePath != "" {
						from.PackagePath = richSymbol.PackagePath
					}
				}

				callRanges := make([]api.CallRange, 0, len(call.FromRanges))
				for _, rng := range call.FromRanges {
					callRanges = append(callRanges, api.CallRange{
						File:      call.From.URI.Path(),
						StartLine: int(rng.Start.Line + 1),
						EndLine:   int(rng.End.Line + 1),
					})
				}

				incomingCalls = append(incomingCalls, api.CallHierarchyCall{
					From:       from,
					CallRanges: callRanges,
				})
			}
			result.IncomingCalls = incomingCalls
			result.TotalIncoming = len(incomingCalls)
		}
	}

	// Get outgoing calls (what this function calls)
	if direction == "outgoing" || direction == "both" {
		outgoing, err := golang.OutgoingCalls(ctx, snapshot, fh, position)
		if err == nil && len(outgoing) > 0 {
			outgoingCalls := make([]api.CallHierarchyCall, 0, len(outgoing))
			for _, call := range outgoing {
				// Extract rich symbol information for the callee
				loc := protocol.Location{
					URI:   call.To.URI,
					Range: call.To.Range,
				}
				richSymbol := golang.ExtractSymbolAtDefinition(ctx, snapshot, loc, false) // Don't include body by default

				// Extract package path for this symbol
				pkgPath := ""
				if pkg, _, err := golang.NarrowestPackageForFile(ctx, snapshot, call.To.URI); err == nil && pkg != nil {
					pkgPath = string(pkg.Metadata().PkgPath)
				}

				// Build the symbol with rich info if available
				to := api.Symbol{
					Name:        call.To.Name,
					Kind:        golang.ConvertLSPSymbolKind(call.To.Kind),
					PackagePath: pkgPath,
					FilePath:    call.To.URI.Path(),
					Line:        int(call.To.Range.Start.Line + 1),
				}

				// Add rich details if extraction succeeded
				if richSymbol != nil && richSymbol.Name != "<symbol>" {
					to.Signature = richSymbol.Signature
					to.Doc = richSymbol.Doc
					to.Receiver = richSymbol.Receiver
					to.Body = richSymbol.Body
					// Use PackagePath from richSymbol if available
					if richSymbol.PackagePath != "" {
						to.PackagePath = richSymbol.PackagePath
					}
				}

				callRanges := make([]api.CallRange, 0, len(call.FromRanges))
				for _, rng := range call.FromRanges {
					callRanges = append(callRanges, api.CallRange{
						File:      symbol.FilePath, // FromRanges are in the current file
						StartLine: int(rng.Start.Line + 1),
						EndLine:   int(rng.End.Line + 1),
					})
				}

				outgoingCalls = append(outgoingCalls, api.CallHierarchyCall{
					From:       to,
					CallRanges: callRanges,
				})
			}
			result.OutgoingCalls = outgoingCalls
			result.TotalOutgoing = len(outgoingCalls)
		}
	}

	// Build formatted summary
	if result.TotalIncoming > 0 {
		summary.WriteString(fmt.Sprintf("Incoming Calls (%d):\n", result.TotalIncoming))
		for i, call := range result.IncomingCalls {
			// Format: "  1. functionName at file.go:123"
			summary.WriteString(fmt.Sprintf("  %d. %s at %s:%d\n", i+1, call.From.Name, call.From.FilePath, call.From.Line))

			// Add package path if available (clean format without pkg.go.dev links)
			if call.From.PackagePath != "" {
				summary.WriteString(fmt.Sprintf("     package: %s\n", call.From.PackagePath))
			}

			// Add signature if available
			if call.From.Signature != "" {
				// Indent the signature
				sigLines := strings.Split(call.From.Signature, "\n")
				for _, sigLine := range sigLines {
					if sigLine != "" {
						summary.WriteString(fmt.Sprintf("     %s\n", sigLine))
					}
				}
			}

			// Add documentation if available (first line only for brevity)
			if call.From.Doc != "" {
				docLines := strings.Split(call.From.Doc, "\n")
				// Find first non-empty line
				for _, docLine := range docLines {
					trimmed := strings.TrimSpace(docLine)
					if trimmed != "" {
						summary.WriteString(fmt.Sprintf("     // %s\n", trimmed))
						break
					}
				}
			}

			// Add call count if multiple call ranges
			if len(call.CallRanges) > 1 {
				summary.WriteString(fmt.Sprintf("     (called %d times)\n", len(call.CallRanges)))
			}
		}
		summary.WriteString("\n")
	} else {
		summary.WriteString("Incoming Calls: None\n\n")
	}

	if result.TotalOutgoing > 0 {
		summary.WriteString(fmt.Sprintf("Outgoing Calls (%d):\n", result.TotalOutgoing))
		for i, call := range result.OutgoingCalls {
			// Format: "  1. functionName at file.go:123"
			summary.WriteString(fmt.Sprintf("  %d. %s at %s:%d\n", i+1, call.From.Name, call.From.FilePath, call.From.Line))

			// Add package path if available (clean format without pkg.go.dev links)
			if call.From.PackagePath != "" {
				summary.WriteString(fmt.Sprintf("     package: %s\n", call.From.PackagePath))
			}

			// Add signature if available
			if call.From.Signature != "" {
				// Indent the signature
				sigLines := strings.Split(call.From.Signature, "\n")
				for _, sigLine := range sigLines {
					if sigLine != "" {
						summary.WriteString(fmt.Sprintf("     %s\n", sigLine))
					}
				}
			}

			// Add documentation if available (first line only for brevity)
			if call.From.Doc != "" {
				docLines := strings.Split(call.From.Doc, "\n")
				// Find first non-empty line
				for _, docLine := range docLines {
					trimmed := strings.TrimSpace(docLine)
					if trimmed != "" {
						summary.WriteString(fmt.Sprintf("     // %s\n", trimmed))
						break
					}
				}
			}

			// Add call count if multiple call ranges
			if len(call.CallRanges) > 1 {
				summary.WriteString(fmt.Sprintf("     (called %d times)\n", len(call.CallRanges)))
			}
		}
	} else {
		summary.WriteString("Outgoing Calls: None\n")
	}

	result.Summary = summary.String()

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary.String()}}}, result, nil
}
