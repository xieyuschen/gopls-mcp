package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/internal/cache"
	"golang.org/x/tools/gopls/internal/protocol"
)

// formatReferenceItems is the shared implementation for formatting reference locations.
// Both formatReferences and formatReferencesWithCount delegate to this.
func formatReferenceItems(ctx context.Context, snapshot *cache.Snapshot, refs []protocol.Location, totalCount int, hint string) string {
	if len(refs) == 0 {
		return "No references found."
	}
	if totalCount == 0 {
		totalCount = len(refs)
	}

	var builder strings.Builder
	truncated := totalCount > len(refs)
	if truncated {
		fmt.Fprintf(&builder, "The object has %v reference(s) (showing first %d):\n", totalCount, len(refs))
	} else {
		fmt.Fprintf(&builder, "The object has %v reference(s):\n", totalCount)
	}

	for i, r := range refs {
		fmt.Fprintf(&builder, "\nReference %d\n", i+1)
		fmt.Fprintf(&builder, "Located in the file: %s\n", filepath.ToSlash(r.URI.Path()))
		refFh, err := snapshot.ReadFile(ctx, r.URI)
		if err != nil {
			continue
		}
		content, err := refFh.Content()
		if err != nil {
			continue
		}
		lines := strings.Split(string(content), "\n")
		var lineContent string
		if int(r.Range.Start.Line) < len(lines) {
			lineContent = strings.TrimLeftFunc(lines[r.Range.Start.Line], unicode.IsSpace)
		} else {
			continue
		}
		fmt.Fprintf(&builder, "Line %d: %s\n", r.Range.Start.Line+1, lineContent)
	}

	if truncated && hint != "" {
		builder.WriteString("\n")
		builder.WriteString(hint)
	}

	return builder.String()
}

// formatReferences formats symbol references into an MCP result.
func formatReferences(ctx context.Context, snapshot *cache.Snapshot, refs []protocol.Location) (*mcp.CallToolResult, error) {
	text := formatReferenceItems(ctx, snapshot, refs, 0, "")
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil
}

// formatReferencesWithCount formats symbol references with truncation metadata.
func formatReferencesWithCount(ctx context.Context, snapshot *cache.Snapshot, refs []protocol.Location, totalCount int, truncated bool, hint string) (string, error) {
	if !truncated {
		totalCount = len(refs) // Normalize: when not truncated, totalCount should match actual count
	}
	return formatReferenceItems(ctx, snapshot, refs, totalCount, hint), nil
}
