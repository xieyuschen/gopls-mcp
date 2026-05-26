package e2e

// E2E tests for ERROR SCENARIOS and edge cases.
// These tests ensure tools handle broken code gracefully and provide helpful error messages.
// Refactored to use table-driven approach for better maintainability.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestErrorScenarios runs comprehensive tests for error scenarios using a
// table-driven approach. Subtests that targeted go_build_check, go_search,
// and go_list_package_symbols were removed when those tools were retired.
func TestErrorScenarios(t *testing.T) {
	testCases := []setupTestCase{
		// Definition lookup against a syntactically broken file must not crash.
		{
			name: "SyntaxErrors_DefinitionInBrokenFile",
			tool: "go_definition",
			setup: func(t *testing.T) map[string]any {
				tmpDir := t.TempDir()
				brokenFile := filepath.Join(tmpDir, "broken_syntax.go")
				brokenCode := testutil.ReadTestData("error/broken_syntax.go")
				if err := os.WriteFile(brokenFile, []byte(brokenCode), 0644); err != nil {
					t.Fatalf("Failed to write broken file: %v", err)
				}
				return map[string]any{
					"locator": map[string]any{
						"symbol_name":  "Println",
						"context_file": brokenFile,
						"kind":         "function",
						"line_hint":    3,
					},
				}
			},
			assertion: func(t *testing.T, content string) {
				t.Log("Definition completed without crashing on broken file")
			},
		},
		// Symbol references must survive missing imports.
		{
			name: "MissingImports_SymbolReferences",
			tool: "go_symbol_references",
			setup: func(t *testing.T) map[string]any {
				tmpDir := t.TempDir()
				missingImportFile := filepath.Join(tmpDir, "missing_import.go")
				code := testutil.ReadTestData("error/missing_import_ref.go")
				if err := os.WriteFile(missingImportFile, []byte(code), 0644); err != nil {
					t.Fatalf("Failed to write file: %v", err)
				}
				return map[string]any{
					"locator": map[string]any{
						"symbol_name":  "make",
						"context_file": missingImportFile,
						"kind":         "function",
						"line_hint":    4,
					},
				}
			},
			assertion: func(t *testing.T, content string) {
				t.Log("Symbol references completed despite import issues")
			},
		},
		// Looking up a non-existent symbol via go_definition must degrade gracefully.
		{
			name: "NonExistentSymbols_DefinitionToNonExistent",
			tool: "go_definition",
			args: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "DummySymbol",
					"context_file": filepath.Join(globalGoplsMcpDir, "test", "testdata", "dummy.go"),
					"line_hint":    1,
				},
			},
			assertion: func(t *testing.T, content string) {
				t.Log("Definition handled non-existent symbol gracefully")
			},
		},
		// Same expectation for go_symbol_references against a non-existent struct.
		{
			name: "NonExistentSymbols_ReferencesToNonExistent",
			tool: "go_symbol_references",
			args: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "FakeHandlerSymbolThatDoesNotExist",
					"context_file": filepath.Join(globalGoplsMcpDir, "core", "handlers.go"),
					"kind":         "struct",
					"line_hint":    25,
				},
			},
			assertion: func(t *testing.T, content string) {
				t.Log("Did not crash on non-existent symbol reference")
			},
		},
		// Dependency graph must finish even on a package with rich cross-module deps.
		{
			name: "CircularDependency_DependencyGraph",
			tool: "go_get_dependency_graph",
			args: map[string]any{
				"package_path":       "golang.org/x/tools/gopls/mcpbridge/core",
				"include_transitive": false,
				"Cwd":                globalGoplsMcpDir,
			},
			assertion: func(t *testing.T, content string) {
				t.Log("Dependency graph completed without hanging on potential cycles")
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			args := tc.args
			if tc.setup != nil {
				args = tc.setup(t)
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
				Name:      tc.tool,
				Arguments: args,
			})
			if err != nil {
				t.Logf("Expected error for %s: %v", tc.name, err)
				return
			}

			content := testutil.ResultText(t, res, testutil.GoldenErrorScenarios)
			t.Logf("%s result:\n%s", tc.tool, testutil.TruncateString(content, 2000))

			tc.assertion(t, content)
		})
	}
}
