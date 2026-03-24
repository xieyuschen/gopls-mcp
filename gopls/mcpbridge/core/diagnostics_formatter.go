package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/internal/cache"
	"golang.org/x/tools/gopls/internal/protocol"
	"golang.org/x/tools/gopls/mcpbridge/api"
)

// formatDiagnostics formats gopls diagnostics into a human-readable output.
// This processes the raw diagnostics from gopls and returns structured results.
func formatDiagnosticsFull(ctx context.Context, snapshot *cache.Snapshot, ids []cache.PackageID) (*mcp.CallToolResult, *[]api.DiagnosticReport, int, int, error) {
	allReports := []api.DiagnosticReport{} // Initialize as empty slice to avoid null in JSON
	totalErrors := 0
	totalWarnings := 0

	// Get diagnostics for all packages
	diagMap, err := snapshot.PackageDiagnostics(ctx, ids...)
	if err != nil {
		return nil, nil, 0, 0, fmt.Errorf("failed to get diagnostics: %w", err)
	}

	// Process diagnostics for each package
	for _, diags := range diagMap {
		for _, diag := range diags {
			// Count errors vs warnings
			severity := "unknown"
			switch diag.Severity {
			case protocol.SeverityError:
				severity = "error"
				totalErrors++
			case protocol.SeverityWarning:
				severity = "warning"
				totalWarnings++
			case protocol.SeverityInformation:
				severity = "info"
			case protocol.SeverityHint:
				severity = "hint"
			}

			// Get file path and line/column
			filePath := diag.URI.Path()
			line := 0
			column := 0
			if diag.Range.Start.Line > 0 {
				line = int(diag.Range.Start.Line) + 1 // Convert to 1-indexed
			}
			if diag.Range.Start.Character > 0 {
				column = int(diag.Range.Start.Character) + 1 // Convert to 1-indexed
			}

			// Extract diagnostic code if available
			diagnosticCode := ""
			if diag.Code != "" {
				diagnosticCode = string(diag.Code)
			}

			// Build source string (e.g., "go", "compiler")
			source := "gopls"
			if diag.Source != "" {
				source = string(diag.Source)
			}

			// Extract code snippet at diagnostic location
			codeSnippet := ""
			if fh, err := snapshot.ReadFile(ctx, diag.URI); err == nil {
				if content, err := fh.Content(); err == nil && content != nil {
					// Get the file content and extract the specific line
					lines := strings.Split(string(content), "\n")
					lineIdx := int(diag.Range.Start.Line)
					if lineIdx >= 0 && lineIdx < len(lines) {
						codeSnippet = strings.TrimSpace(lines[lineIdx])
					}
				}
			}

			report := api.DiagnosticReport{
				File:           filePath,
				Line:           line,
				Column:         column,
				Severity:       severity,
				Message:        diag.Message,
				Source:         source,
				DiagnosticCode: diagnosticCode,
				CodeSnippet:    codeSnippet,
			}

			allReports = append(allReports, report)
		}
	}

	// Build summary text
	var summary strings.Builder
	if totalErrors > 0 || totalWarnings > 0 {
		fmt.Fprintf(&summary, "Found %d error(s) and %d warning(s)\n\n", totalErrors, totalWarnings)

		// Group diagnostics by file
		byFile := make(map[string][]api.DiagnosticReport)
		for _, report := range allReports {
			byFile[report.File] = append(byFile[report.File], report)
		}

		// Show most problematic files first
		for file, reports := range byFile {
			fmt.Fprintf(&summary, "## %s\n", file)
			for _, r := range reports {
				loc := fmt.Sprintf("%d:%d", r.Line, r.Column)
				fmt.Fprintf(&summary, "  %s [%s]: %s\n", loc, r.Severity, r.Message)
				if r.CodeSnippet != "" {
					fmt.Fprintf(&summary, "    Code: %s\n", r.CodeSnippet)
				}
				if r.DiagnosticCode != "" {
					fmt.Fprintf(&summary, "    Diagnostic: %s\n", r.DiagnosticCode)
				}
			}
			fmt.Fprintln(&summary)
		}

		// Add suggestion if there are many diagnostics
		if len(allReports) > 50 {
			fmt.Fprintf(&summary, "\n... and %d more diagnostic(s)\n", len(allReports)-50)
		}
	} else {
		fmt.Fprintf(&summary, "No diagnostics found - workspace is clean!\n")
	}

	content := []mcp.Content{&mcp.TextContent{Text: summary.String()}}
	return &mcp.CallToolResult{Content: content}, &allReports, totalErrors, totalWarnings, nil
}
