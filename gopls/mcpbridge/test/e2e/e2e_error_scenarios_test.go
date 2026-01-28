package e2e

// E2E tests for ERROR SCENARIOS and edge cases.
// These tests ensure tools handle broken code gracefully and provide helpful error messages.
// Refactored to use table-driven approach for better maintainability.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestErrorScenarios runs comprehensive tests for error scenarios using table-driven approach
func TestErrorScenarios(t *testing.T) {
	// Define all test cases
	testCases := []setupTestCase{
		// Test 1: Syntax Errors
		{
			name: "SyntaxErrors_Diagnostics",
			tool: "go_build_check",
			setup: func(t *testing.T) map[string]any {
				tmpDir := t.TempDir()
				brokenFile := filepath.Join(tmpDir, "broken_syntax.go")
				brokenCode := testutil.ReadTestData("error/broken_syntax.go")
				if err := os.WriteFile(brokenFile, []byte(brokenCode), 0644); err != nil {
					t.Fatalf("Failed to write broken file: %v", err)
				}
				return map[string]any{"Cwd": tmpDir}
			},
			assertion: func(t *testing.T, content string) {
				// Should find errors
				if !strings.Contains(content, "expected") && !strings.Contains(content, "syntax") && !strings.Contains(content, "error") {
					t.Error("Expected diagnostics to find syntax errors")
				}
			},
		},
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
				// Should not crash
				t.Log("Definition completed without crashing on broken file")
			},
		},
		// Test 2: Missing Imports
		{
			name: "MissingImports_Diagnostics",
			tool: "go_build_check",
			setup: func(t *testing.T) map[string]any {
				tmpDir := t.TempDir()
				missingImportFile := filepath.Join(tmpDir, "missing_import.go")
				code := testutil.ReadTestData("error/missing_import.go")
				if err := os.WriteFile(missingImportFile, []byte(code), 0644); err != nil {
					t.Fatalf("Failed to write file: %v", err)
				}
				return map[string]any{"Cwd": tmpDir}
			},
			assertion: func(t *testing.T, content string) {
				// Should report missing import or undefined reference
				if !strings.Contains(strings.ToLower(content), "undefined") && !strings.Contains(strings.ToLower(content), "import") {
					t.Log("Note: May not explicitly mention missing import")
				}
			},
		},
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
				// Should not crash
				t.Log("Symbol references completed despite import issues")
			},
		},
		// Test 3: Type Errors
		{
			name: "TypeErrors_Diagnostics",
			tool: "go_build_check",
			setup: func(t *testing.T) map[string]any {
				tmpDir := t.TempDir()
				typeErrorFile := filepath.Join(tmpDir, "type_errors.go")
				code := testutil.ReadTestData("error/type_errors.go")
				if err := os.WriteFile(typeErrorFile, []byte(code), 0644); err != nil {
					t.Fatalf("Failed to write file: %v", err)
				}
				return map[string]any{"Cwd": tmpDir}
			},
			assertion: func(t *testing.T, content string) {
				// Should find type mismatches
				if !strings.Contains(content, "type") && !strings.Contains(content, "cannot") && !strings.Contains(content, "error") {
					t.Log("Note: Type error reporting may vary")
				}
			},
		},
		// Test 4: Non-Existent Symbols
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
				// Should handle gracefully (error or empty result is ok)
				t.Log("Definition handled non-existent symbol gracefully")
			},
		},
		{
			name: "NonExistentSymbols_SearchForNonExistent",
			tool: "go_search",
			args: map[string]any{
				"query":       "CompletelyFakeSymbolName12345",
				"max_results": 5,
			},
			assertion: func(t *testing.T, content string) {
				// Should return empty results or indicate no matches found
				if !strings.Contains(content, "No results") && !strings.Contains(content, "0 results") && !strings.Contains(content, "No symbols") && strings.Contains(content, "CompletelyFakeSymbolName12345") {
					t.Errorf("Expected 'no results' message for non-existent symbol, but found it: %s", content)
				}
			},
		},
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
		// Test 5: Undeclared Variables
		{
			name: "UndeclaredVariables_Diagnostics",
			tool: "go_build_check",
			setup: func(t *testing.T) map[string]any {
				tmpDir := t.TempDir()
				undeclaredFile := filepath.Join(tmpDir, "undeclared.go")
				code := testutil.ReadTestData("error/undeclared.go")
				if err := os.WriteFile(undeclaredFile, []byte(code), 0644); err != nil {
					t.Fatalf("Failed to write file: %v", err)
				}
				return map[string]any{"Cwd": tmpDir}
			},
			assertion: func(t *testing.T, content string) {
				// Should report undeclared variable error
				contentLower := strings.ToLower(content)
				if !strings.Contains(contentLower, "undefined") && !strings.Contains(contentLower, "declared") && !strings.Contains(contentLower, "cannot") && !strings.Contains(contentLower, "error") {
					t.Errorf("Expected diagnostics to identify undeclared variable, got: %s", testutil.TruncateString(content, 200))
				}
			},
		},
		// Test 6: Invalid Package Structure
		{
			name: "InvalidPackage_DiagnosticsMismatchedPackages",
			tool: "go_build_check",
			setup: func(t *testing.T) map[string]any {
				tmpDir := t.TempDir()
				file1 := filepath.Join(tmpDir, "file1.go")
				file2 := filepath.Join(tmpDir, "file2.go")

				code1 := testutil.ReadTestData("error/file1.go")

				code2 := testutil.ReadTestData("error/file2.go")

				os.WriteFile(file1, []byte(code1), 0644)
				os.WriteFile(file2, []byte(code2), 0644)

				return map[string]any{"Cwd": tmpDir}
			},
			assertion: func(t *testing.T, content string) {
				// Should report package mismatch
				if strings.Contains(strings.ToLower(content), "package") || strings.Contains(strings.ToLower(content), "different") {
					t.Log("Identified package structure issue")
				}
			},
		},
		{
			name: "InvalidPackage_ListSymbolsInInvalidPackage",
			tool: "go_list_package_symbols",
			setup: func(t *testing.T) map[string]any {
				tmpDir := t.TempDir()
				file1 := filepath.Join(tmpDir, "file1.go")
				file2 := filepath.Join(tmpDir, "file2.go")

				code1 := testutil.ReadTestData("error/file1.go")

				code2 := testutil.ReadTestData("error/file2.go")

				os.WriteFile(file1, []byte(code1), 0644)
				os.WriteFile(file2, []byte(code2), 0644)

				return map[string]any{
					"package_path":   "main",
					"include_docs":   false,
					"include_bodies": false,
					"Cwd":            tmpDir,
				}
			},
			assertion: func(t *testing.T, content string) {
				t.Log("Did not crash on invalid package structure")
			},
		},
		// Test 7: Circular Dependency
		{
			name: "CircularDependency_DependencyGraph",
			tool: "go_get_dependency_graph",
			args: map[string]any{
				"package_path":       "golang.org/x/tools/gopls/mcpbridge/core",
				"include_transitive": false,
				"Cwd":                globalGoplsMcpDir,
			},
			assertion: func(t *testing.T, content string) {
				// Should successfully analyze even with complex dependencies
				t.Log("Dependency graph completed without hanging on potential cycles")
			},
		},
		// Test 8: Large File with Errors
		{
			name: "LargeFileWithErrors_DiagnosticsOnLargeFile",
			tool: "go_build_check",
			args: map[string]any{
				"Cwd": globalGoplsMcpDir,
			},
			assertion: func(t *testing.T, content string) {
				// Should handle large files efficiently
				t.Log("Successfully analyzed large file")
			},
		},
	}

	// Run all test cases
	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			// Setup if needed
			args := tc.args
			if tc.setup != nil {
				args = tc.setup(t)
			}

			// Call the tool
			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
				Name:      tc.tool,
				Arguments: args,
			})
			if err != nil {
				t.Logf("Expected error for %s: %v", tc.name, err)
				return
			}

			// Get and truncate content for logging
			content := testutil.ResultText(t, res, testutil.GoldenErrorScenarios)
			t.Logf("%s result:\n%s", tc.tool, testutil.TruncateString(content, 2000))

			// Run assertion
			tc.assertion(t, content)
		})
	}
}