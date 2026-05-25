// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package golang

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/tools/gopls/internal/cache"
	"golang.org/x/tools/gopls/internal/protocol"
	"golang.org/x/tools/gopls/internal/settings"
	"golang.org/x/tools/gopls/mcpbridge/api"
)

// SearchWorkspaceSymbolsForLLM performs workspace-wide symbol search for LLM usage.
//
// WHY THIS EXISTS:
//
//	gopls's existing WorkspaceSymbols() is designed for editors, where:
//	  - User opens files incrementally
//	  - Files are parsed on-demand as they're opened
//	  - Metadata is loaded lazily
//
//	LLMs have a different usage pattern:
//	  - First call is often go_search with NO files open
//	  - No metadata loaded, no files parsed, nothing in cache
//	  - Need full workspace scan immediately
//
//	This function bridges that gap by:
//	  1. Eagerly loading metadata for workspace packages
//	  2. Reusing WorkspaceSymbols() for matching (parallel, fuzzy, symbolization)
//	  3. Converting results to MCP API format
//
// The key difference: This forces workspace loading, whereas original gopls
// assumes workspace is already being used interactively.
func SearchWorkspaceSymbolsForLLM(ctx context.Context, snapshot *cache.Snapshot, query string, maxResults int) ([]*api.Symbol, error) {
	if maxResults <= 0 {
		maxResults = 10
	}

	// Handle empty query
	if query == "" {
		return []*api.Symbol{}, nil
	}

	// 1. Force metadata loading (this is the key difference from editor workflow!)
	// This ensures workspace packages are discovered and indexed
	_, err := snapshot.LoadMetadataGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load metadata: %v", err)
	}

	// 2. REUSE gopls's WorkspaceSymbols
	// This gives us:
	//   - Parallel matching using all CPUs
	//   - Sophisticated fuzzy matching
	//   - Proper symbolization (package-qualified, fully-qualified, etc.)
	//   - WorkspacePackages filtering
	symbolInfos, err := WorkspaceSymbols(
		ctx,
		[]*cache.Snapshot{snapshot},
		query,
		WorkspaceSymbolsOptions{
			Matcher: settings.SymbolFuzzy,
			Style:   settings.DynamicSymbols,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("workspace symbols failed: %v", err)
	}

	// 3. Convert protocol.SymbolInformation to api.Symbol
	symbols := make([]*api.Symbol, 0, len(symbolInfos))
	for _, symInfo := range symbolInfos {
		if symInfo.Name == "" {
			continue
		}

		// Extract receiver info for methods
		receiver := ""
		if symInfo.Kind == protocol.Method {
			receiver = symInfo.ContainerName
		}

		apiSym := &api.Symbol{
			Name:        symInfo.Name,
			Kind:        convertSymbolKind(symInfo.Kind),
			Receiver:    receiver,
			PackagePath: extractPackagePath(symInfo),
			FilePath:    symInfo.Location.URI.Path(),
			Line:        int(symInfo.Location.Range.Start.Line + 1), // Convert to 1-indexed
		}
		symbols = append(symbols, apiSym)

		// Apply maxResults limit
		if len(symbols) >= maxResults {
			break
		}
	}

	return symbols, nil
}

// extractPackagePath attempts to extract the package path from a SymbolInformation.
// This is best-effort since SymbolInformation doesn't always contain package info.
func extractPackagePath(sym protocol.SymbolInformation) string {
	// Try to get package path from container name if it looks like a package path
	if sym.ContainerName != "" && strings.Contains(sym.ContainerName, ".") {
		// Check if it looks like a package path (contains dots)
		return sym.ContainerName
	}
	return ""
}

// convertSymbolKind converts protocol.SymbolKind to api.SymbolKind string.
func convertSymbolKind(kind protocol.SymbolKind) api.SymbolKind {
	switch kind {
	case protocol.Field:
		return api.SymbolKindField
	case protocol.Method:
		return api.SymbolKindMethod
	case protocol.Function:
		return api.SymbolKindFunction
	case protocol.Struct:
		return api.SymbolKindStruct
	case protocol.Interface:
		return api.SymbolKindInterface
	case protocol.Variable:
		return api.SymbolKindVariable
	case protocol.Constant:
		return api.SymbolKindConstant
	case protocol.Class:
		// Type declarations
		return api.SymbolKindType
	default:
		return api.SymbolKindType
	}
}
