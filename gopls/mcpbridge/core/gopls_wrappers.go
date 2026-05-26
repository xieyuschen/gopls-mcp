package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/internal/cache"
	"golang.org/x/tools/gopls/internal/golang"
	"golang.org/x/tools/gopls/internal/protocol"
	"golang.org/x/tools/gopls/internal/util/safetoken"
	"golang.org/x/tools/gopls/mcpbridge/api"
)

// This file contains wrapper handlers for the existing gopls MCP tools.
// These wrap the original implementations from gopls/internal/mcp to provide
// a unified interface through the gopls-mcp Handler pattern.

// ===== Common Helpers =====

// buildRichSymbol creates an api.Symbol from a CallHierarchyItem and enriches it
// with rich information (signature, doc, receiver, body, package path).
func buildRichSymbol(ctx context.Context, snapshot *cache.Snapshot, name string, kind protocol.SymbolKind, uri protocol.DocumentURI, rng protocol.Range, pkgPath string) api.Symbol {
	symbol := api.Symbol{
		Name:        name,
		Kind:        golang.ConvertLSPSymbolKind(kind),
		PackagePath: pkgPath,
		FilePath:    string(uri.Path()),
		Line:        int(rng.Start.Line + 1),
	}

	loc := protocol.Location{URI: uri, Range: rng}
	richSymbol := golang.ExtractSymbolAtDefinition(ctx, snapshot, loc, false)
	if richSymbol != nil && richSymbol.Name != "<symbol>" {
		symbol.Signature = richSymbol.Signature
		symbol.Doc = richSymbol.Doc
		symbol.Receiver = richSymbol.Receiver
		symbol.Body = richSymbol.Body
		if richSymbol.PackagePath != "" {
			symbol.PackagePath = richSymbol.PackagePath
		}
	}

	return symbol
}

// pkgPathForFile returns the package path for the given file URI, or empty string on error.
func pkgPathForFile(ctx context.Context, snapshot *cache.Snapshot, uri protocol.DocumentURI) string {
	if pkg, _, err := golang.NarrowestPackageForFile(ctx, snapshot, uri); err == nil && pkg != nil {
		return string(pkg.Metadata().PkgPath)
	}
	return ""
}

// buildCallRanges converts protocol ranges to api.CallRange slice.
func buildCallRanges(file string, ranges []protocol.Range) []api.CallRange {
	callRanges := make([]api.CallRange, 0, len(ranges))
	for _, rng := range ranges {
		callRanges = append(callRanges, api.CallRange{
			File:      file,
			StartLine: int(rng.Start.Line + 1),
			EndLine:   int(rng.End.Line + 1),
		})
	}
	return callRanges
}

// formatCallHierarchySection formats a list of call hierarchy entries into a summary string.
func formatCallHierarchySection(title string, calls []api.CallHierarchyCall) string {
	if len(calls) == 0 {
		return title + ": None\n"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s (%d):\n", title, len(calls)))
	for i, call := range calls {
		b.WriteString(fmt.Sprintf("  %d. %s at %s:%d\n", i+1, call.From.Name, call.From.FilePath, call.From.Line))

		if call.From.PackagePath != "" {
			b.WriteString(fmt.Sprintf("     package: %s\n", call.From.PackagePath))
		}

		if call.From.Signature != "" {
			for _, sigLine := range strings.Split(call.From.Signature, "\n") {
				if sigLine != "" {
					b.WriteString(fmt.Sprintf("     %s\n", sigLine))
				}
			}
		}

		if call.From.Doc != "" {
			for _, docLine := range strings.Split(call.From.Doc, "\n") {
				if trimmed := strings.TrimSpace(docLine); trimmed != "" {
					b.WriteString(fmt.Sprintf("     // %s\n", trimmed))
					break
				}
			}
		}

		if len(call.CallRanges) > 1 {
			b.WriteString(fmt.Sprintf("     (called %d times)\n", len(call.CallRanges)))
		}
	}
	return b.String()
}

// ===== go_definition =====
// Origin: gopls/internal/golang/definition.go Definition()

func handleGoDefinition(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.IDefinitionParams) (*mcp.CallToolResult, *api.ODefinitionResult, error) {
	dir := filepath.Dir(input.Locator.ContextFile)
	snapshot, release, err := h.snapshotForDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get snapshot for %s: %w", dir, err)
	}
	defer release()

	info, err := golang.ResolveSymbol(ctx, snapshot, input.Locator, golang.ResolveOptions{
		FindDefinitions:   true,
		IncludeDefinition: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve symbol '%s': %w", input.Locator.SymbolName, err)
	}

	if len(info.Locations) == 0 {
		summary := fmt.Sprintf("No definition found for symbol '%s' in %s", input.Locator.SymbolName, input.Locator.ContextFile)
		result := &api.ODefinitionResult{Summary: summary}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}}, result, nil
	}

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
		if input.IncludeBody {
			sym.Body = srcCtx.Snippet
		}
	}

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

	uri := protocol.URIFromPath(input.Locator.ContextFile)
	fh, err := snapshot.ReadFile(ctx, uri)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file %s: %w", input.Locator.ContextFile, err)
	}

	nodeResult, err := golang.ResolveNode(ctx, snapshot, fh, input.Locator)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve symbol '%s': %w", input.Locator.SymbolName, err)
	}

	pkg, _, err := golang.NarrowestPackageForFile(ctx, snapshot, uri)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get package: %w", err)
	}

	posn := safetoken.StartPosition(pkg.FileSet(), nodeResult.Pos)
	if !posn.IsValid() {
		return nil, nil, fmt.Errorf("invalid position for symbol '%s'", input.Locator.SymbolName)
	}

	position := protocol.Position{
		Line:      uint32(posn.Line - 1),
		Character: uint32(posn.Column - 1),
	}

	// includeDeclaration=false to exclude the definition itself
	locations, err := golang.References(ctx, snapshot, fh, protocol.Range{Start: position, End: position}, false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find references: %w", err)
	}

	var symbols []*api.Symbol
	if defLocs, err := golang.Definition(ctx, snapshot, fh, protocol.Range{Start: position, End: position}); err == nil && len(defLocs) > 0 {
		if sym := golang.ExtractSymbolAtDefinition(ctx, snapshot, defLocs[0], true); sym != nil {
			symbols = append(symbols, sym)
		}
	}

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

			fh, err := snapshot.ReadFile(ctx, loc.URI)
			if err == nil {
				content, err := fh.Content()
				if err == nil {
					lines := strings.Split(string(content), "\n")
					lineIdx := int(loc.Range.Start.Line)
					if lineIdx >= 0 && lineIdx < len(lines) {
						line := strings.TrimSpace(lines[lineIdx])
						if len(line) > 0 && len(line) < 100 {
							summary.WriteString(fmt.Sprintf("\n   %s", line))
						}
					}
				}
			}
			summary.WriteString("\n")
		}
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

func handleGoRenameSymbol(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.IRenameSymbolParams) (*mcp.CallToolResult, *api.ORenameSymbolResult, error) {
	snapshot, release, err := h.snapshot()
	if err != nil {
		return nil, nil, err
	}
	defer release()

	unifiedDiff, lineChanges, err := golang.LLMRename(ctx, snapshot, input.Locator, input.NewName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compute rename: %w", err)
	}

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

func handleGoImplementation(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.IImplementationParams) (*mcp.CallToolResult, *api.OImplementationResult, error) {
	snapshot, release, err := h.snapshot()
	if err != nil {
		return nil, nil, err
	}
	defer release()

	sourceContexts, err := golang.LLMImplementation(ctx, snapshot, input.Locator)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find implementations for '%s': %w", input.Locator.SymbolName, err)
	}

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
		if input.IncludeBody {
			sym.Body = srcCtx.Snippet
		}
		symbols = append(symbols, sym)
	}

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

// handleListTools returns documentation for all available MCP tools.
func handleListTools(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.IListToolsParams) (*mcp.CallToolResult, *api.OListToolsResult, error) {
	toolDocs := []api.ToolDocumentation{}

	for _, tool := range tools {
		name, description := tool.Details()

		doc := api.ToolDocumentation{
			Name:        name,
			Description: description,
		}

		category := categorizeTool(name)
		doc.Category = category

		if input.CategoryFilter != "" && category != input.CategoryFilter {
			continue
		}

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

	summaryHeader := fmt.Sprintf("gopls-mcp provides %d tools for Go project analysis\n\n", len(toolDocs))

	categories := make(map[string][]string)
	for _, doc := range toolDocs {
		categories[doc.Category] = append(categories[doc.Category], doc.Name)
	}

	var summary strings.Builder
	summary.WriteString(summaryHeader)

	categoryOrder := []string{"meta", "analysis", "navigation", "refactoring"}
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
	switch name {
	case "go_list_tools":
		return "meta"
	case "go_get_dependency_graph":
		return "analysis"
	case "go_symbol_references",
		"go_implementation",
		"go_definition",
		"go_get_call_hierarchy":
		return "navigation"
	case "go_dryrun_rename_symbol":
		return "refactoring"
	default:
		return "other"
	}
}

// getToolSchemas returns the input and output schemas for a tool.
func getToolSchemas(toolName string) map[string]map[string]any {
	schemas := map[string]map[string]any{}

	switch toolName {
	case "go_implementation":
		schemas["input"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"locator": map[string]any{
					"type":        "object",
					"description": "Semantic symbol locator",
				},
			},
		}
		schemas["output"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"symbols": map[string]any{
					"type":        "array",
					"description": "Implementation symbols",
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

// ===== go_get_call_hierarchy =====
// New tool for call hierarchy analysis

func handleGoCallHierarchy(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input api.ICallHierarchyParams) (*mcp.CallToolResult, *api.OCallHierarchyResult, error) {
	snapshot, release, err := h.snapshotForDir(input.Cwd)
	if err != nil {
		return nil, nil, err
	}
	defer release()

	uri := protocol.URIFromPath(input.Locator.ContextFile)
	fh, err := snapshot.ReadFile(ctx, uri)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	nodeResult, err := golang.ResolveNode(ctx, snapshot, fh, input.Locator)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve symbol '%s': %w", input.Locator.SymbolName, err)
	}

	pkg, _, err := golang.NarrowestPackageForFile(ctx, snapshot, uri)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get package: %w", err)
	}

	posn := safetoken.StartPosition(pkg.FileSet(), nodeResult.Pos)
	if !posn.IsValid() {
		return nil, nil, fmt.Errorf("invalid position for symbol '%s'", input.Locator.SymbolName)
	}

	position := protocol.Position{
		Line:      uint32(posn.Line - 1),
		Character: uint32(posn.Column - 1),
	}

	direction := input.Direction
	if direction == "" {
		direction = "both"
	}

	items, err := golang.PrepareCallHierarchy(ctx, snapshot, fh, protocol.Range{Start: position, End: position})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to prepare call hierarchy: %w", err)
	}

	if len(items) == 0 {
		summary := fmt.Sprintf("No function found for symbol '%s' in %s",
			input.Locator.SymbolName, input.Locator.ContextFile)
		result := &api.OCallHierarchyResult{Summary: summary}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}}, result, nil
	}

	item := items[0]

	pkgPath := ""
	if mps, err := snapshot.MetadataForFile(ctx, item.URI, false); err == nil && len(mps) > 0 {
		pkgPath = string(mps[0].PkgPath)
	}

	symbol := buildRichSymbol(ctx, snapshot, item.Name, item.Kind, item.URI, item.Range, pkgPath)

	result := &api.OCallHierarchyResult{
		Symbol: symbol,
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Call hierarchy for %s at %s:%d\n\n", symbol.Name, symbol.FilePath, symbol.Line))

	if direction == "incoming" || direction == "both" {
		incoming, err := golang.IncomingCalls(ctx, snapshot, fh, protocol.Range{Start: position, End: position})
		if err == nil && len(incoming) > 0 {
			incomingCalls := make([]api.CallHierarchyCall, 0, len(incoming))
			for _, call := range incoming {
				fromPkgPath := pkgPathForFile(ctx, snapshot, call.From.URI)
				from := buildRichSymbol(ctx, snapshot, call.From.Name, call.From.Kind, call.From.URI, call.From.Range, fromPkgPath)

				incomingCalls = append(incomingCalls, api.CallHierarchyCall{
					From:       from,
					CallRanges: buildCallRanges(call.From.URI.Path(), call.FromRanges),
				})
			}
			result.IncomingCalls = incomingCalls
			result.TotalIncoming = len(incomingCalls)
		}
	}

	if direction == "outgoing" || direction == "both" {
		outgoing, err := golang.OutgoingCalls(ctx, snapshot, fh, position)
		if err == nil && len(outgoing) > 0 {
			outgoingCalls := make([]api.CallHierarchyCall, 0, len(outgoing))
			for _, call := range outgoing {
				toPkgPath := pkgPathForFile(ctx, snapshot, call.To.URI)
				to := buildRichSymbol(ctx, snapshot, call.To.Name, call.To.Kind, call.To.URI, call.To.Range, toPkgPath)

				outgoingCalls = append(outgoingCalls, api.CallHierarchyCall{
					From:       to,
					CallRanges: buildCallRanges(symbol.FilePath, call.FromRanges),
				})
			}
			result.OutgoingCalls = outgoingCalls
			result.TotalOutgoing = len(outgoingCalls)
		}
	}

	summary.WriteString(formatCallHierarchySection("Incoming Calls", result.IncomingCalls))
	summary.WriteString("\n")
	summary.WriteString(formatCallHierarchySection("Outgoing Calls", result.OutgoingCalls))

	result.Summary = summary.String()

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary.String()}}}, result, nil
}
