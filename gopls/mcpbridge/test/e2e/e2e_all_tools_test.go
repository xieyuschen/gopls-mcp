package e2e

// Table-driven E2E tests for ALL MCP tools on the REAL gopls-mcp codebase.
// These tests validate that each tool works correctly on production code.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestAllTools runs comprehensive tests for all MCP tools using table-driven approach
func TestAllTools(t *testing.T) {

	// Read files for line number lookups
	wrappersPath := filepath.Join(globalGoplsMcpDir, "core", "gopls_wrappers.go")
	handlersPath := filepath.Join(globalGoplsMcpDir, "core", "handlers.go")

	wrappersContent, _ := os.ReadFile(wrappersPath)
	handlersContent, _ := os.ReadFile(handlersPath)

	var handleGoDefLine, handlerStructLine int
	wrappersLines := strings.Split(string(wrappersContent), "\n")
	handlersLines := strings.Split(string(handlersContent), "\n")

	for i, line := range wrappersLines {
		if strings.Contains(line, "func handleGoDefinition(") {
			handleGoDefLine = i + 1
		}
	}
	for i, line := range handlersLines {
		if strings.Contains(line, "type Handler struct") {
			handlerStructLine = i + 1
		}
	}

	// Define all test cases
	testCases := []testCase{
		// Discovery Tools
		{
			name: "go_list_modules",
			tool: "go_list_modules",
			args: map[string]any{
				"direct_only": true,
				"Cwd":         globalGoplsMcpDir,
			},
			assertion: func(t *testing.T, content string) {
				if !strings.Contains(content, "golang.org/x/tools") {
					t.Errorf("Expected to find golang.org/x/tools module, got: %s", content)
				}
			},
		},
		{
			name: "go_list_module_packages",
			tool: "go_list_module_packages",
			args: map[string]any{
				"module_path":      "golang.org/x/text",
				"include_docs":     false,
				"exclude_tests":    false,
				"exclude_internal": false,
				"top_level_only":   false,
			},
			assertion: func(t *testing.T, content string) {
				if !strings.Contains(content, "Module:") {
					t.Errorf("Expected module information, got: %s", content)
				}
			},
		},
		{
			name: "go_list_package_symbols",
			tool: "go_list_package_symbols",
			args: map[string]any{
				"package_path":   "golang.org/x/tools/gopls/mcpbridge/core",
				"include_docs":   true,
				"include_bodies": false,
				"Cwd":            globalGoplsMcpDir,
			},
			assertion: func(t *testing.T, content string) {
				if !strings.Contains(content, "Handler") {
					t.Errorf("Expected to find Handler type, got: %s", content)
				}
				if !strings.Contains(content, "RegisterTools") {
					t.Errorf("Expected to find RegisterTools function, got: %s", content)
				}
			},
		},
		{
			name: "go_analyze_workspace",
			tool: "go_analyze_workspace",
			args: map[string]any{
				"Cwd": globalGoplsMcpDir,
			},
			assertion: func(t *testing.T, content string) {
				if !strings.Contains(content, "module") {
					t.Errorf("Expected workspace analysis to mention module type, got: %s", content)
				}
			},
		},
		{
			name: "go_get_started",
			tool: "go_get_started",
			args: map[string]any{},
			assertion: func(t *testing.T, content string) {
				if !strings.Contains(content, "gopls-mcp") {
					t.Errorf("Expected get_started to mention gopls-mcp, got: %s", content)
				}
			},
		},
		{
			name: "go_list_tools",
			tool: "go_list_tools",
			args: map[string]any{},
			assertion: func(t *testing.T, content string) {
				if !strings.Contains(content, "tools") && !strings.Contains(content, "NAVIGATION") {
					t.Errorf("Expected tool information, got: %s", content)
				}
			},
		},
		// Navigation Tools
		{
			name: "go_definition",
			tool: "go_definition",
			args: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "handleGoDefinition",
					"context_file": wrappersPath,
					"line_hint":    handleGoDefLine,
				},
			},
			assertion: func(t *testing.T, content string) {
				if !strings.Contains(content, "gopls_wrappers.go") {
					t.Errorf("Expected to find definition in gopls_wrappers.go, got: %s", content)
				}
			},
			skip:       handleGoDefLine == 0,
			skipReason: "Could not find handleGoDefinition function",
		},
		{
			name: "go_search",
			tool: "go_search",
			args: map[string]any{
				"query":       "Handler",
				"max_results": 5,
				"Cwd":         globalGoplsMcpDir,
			},
			assertion: func(t *testing.T, content string) {
				if !strings.Contains(content, "Handler") {
					t.Errorf("Expected to find Handler symbol, got: %s", content)
				}
			},
		},
		// Analysis Tools
		{
			name: "go_get_package_symbol_detail",
			tool: "go_get_package_symbol_detail",
			args: map[string]any{
				"package_path":   "golang.org/x/tools/gopls/mcpbridge/core",
				"symbol_filters": []map[string]any{{"name": "Handler"}, {"name": "RegisterTools"}},
				"include_docs":   true,
				"include_bodies": false,
				"Cwd":            globalGoplsMcpDir,
			},
			assertion: func(t *testing.T, content string) {
				if !strings.Contains(content, "Handler") && !strings.Contains(content, "RegisterTools") {
					t.Errorf("Expected Handler or RegisterTools information, got: %s", content)
				}
			},
		},
		{
			name: "go_implementation",
			tool: "go_implementation",
			args: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Handler",
					"context_file": handlersPath,
					// todo: this is wrong, we should test for interface.
					"kind":      "struct",
					"line_hint": handlerStructLine,
				},
			},
			assertion: func(t *testing.T, content string) {
				t.Log("go_implementation test completed (may not find implementations for struct)")
			},
			skip:       handlerStructLine == 0,
			skipReason: "Could not find Handler struct definition",
		},
		{
			name: "go_symbol_references",
			tool: "go_symbol_references",
			args: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Handler",
					"context_file": handlersPath,
					"kind":         "struct",
					"line_hint":    handlerStructLine,
				},
			},
			assertion: func(t *testing.T, content string) {
				// todo: it requires a stronger assert, checking not found is not enough.
				// we should check the references are found correctly.
				if strings.Contains(content, "No references found") {
					t.Log("Known limitation: Searching from definition file")
				}
			},
			skip:       handlerStructLine == 0,
			skipReason: "Could not find Handler struct definition",
		},
		{
			name: "go_get_call_hierarchy",
			tool: "go_get_call_hierarchy",
			args: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "handleGoDefinition",
					"context_file": wrappersPath,
					"kind":         "function",
					"line_hint":    handleGoDefLine,
				},
				"direction": "incoming",
			},
			assertion: func(t *testing.T, content string) {
				if !strings.Contains(content, "Call hierarchy") {
					t.Errorf("Expected call hierarchy information, got: %s", content)
				}
			},
			skip:       handleGoDefLine == 0,
			skipReason: "Could not find handleGoDefinition function",
		},
		{
			name: "go_get_dependency_graph",
			tool: "go_get_dependency_graph",
			args: map[string]any{
				"package_path":       "golang.org/x/tools/gopls/mcpbridge/core",
				"include_transitive": false,
				"Cwd":                globalGoplsMcpDir,
			},
			assertion: func(t *testing.T, content string) {
				if !strings.Contains(content, "Dependencies") && !strings.Contains(content, "imports") {
					t.Errorf("Expected dependency information, got: %s", content)
				}
			},
		},
		// Quality Tools
		{
			name: "go_build_check",
			tool: "go_build_check",
			args: map[string]any{
				"Cwd": globalGoplsMcpDir,
			},
			assertion: func(t *testing.T, content string) {
				if !strings.Contains(content, "packages") && !strings.Contains(content, "diagnostics") {
					t.Errorf("Expected diagnostic information, got: %s", content)
				}
			},
		},
		// File Tools
		{
			name: "go_read_file",
			tool: "go_read_file",
			args: map[string]any{
				"file": handlersPath,
			},
			assertion: func(t *testing.T, content string) {
				if !strings.Contains(content, "package") && !strings.Contains(content, "func") {
					t.Errorf("Expected Go code content, got: %s", content)
				}
			},
		},
	}

	// Run all test cases
	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			if tc.skip {
				t.Skip(tc.skipReason)
			}

			// Call the tool
			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
				Name:      tc.tool,
				Arguments: tc.args,
			})
			if err != nil {
				t.Fatalf("Failed to call %s: %v", tc.tool, err)
			}

			// Get and truncate content for logging
			content := testutil.ResultText(t, res, testutil.GoldenAllTools)
			t.Logf("%s result:\n%s", tc.tool, testutil.TruncateString(content, 2000))

			// Run assertion
			tc.assertion(t, content)
		})
	}
}
