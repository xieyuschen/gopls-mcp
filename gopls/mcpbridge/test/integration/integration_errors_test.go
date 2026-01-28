package integration

// End-to-end tests for error handling and edge cases.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestErrorHandlingE2E tests that tools handle errors gracefully and don't crash.
// This is CRITICAL for production stability.
func TestErrorHandlingE2E(t *testing.T) {
	t.Run("InvalidFilePath", func(t *testing.T) {
		projectDir := t.TempDir()

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		sourceCode := `package main

func main() {
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		t.Run("GoReadFile_NonExistent", func(t *testing.T) {
			tool := "go_read_file"
			args := map[string]any{
				"file": "/nonexistent/path/to/file.go",
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})

			// Should error gracefully, not crash
			if err != nil {
				t.Logf("Expected error for nonexistent file: %v", err)
			} else if res != nil {
				content := testutil.ResultText(t, res, testutil.GoldenErrorHandling)
				if !strings.Contains(content, "not found") &&
					!strings.Contains(content, "no such file") &&
					!strings.Contains(content, "error") {
					t.Logf("Warning: Tool didn't error for nonexistent file: %s", content)
				}
			}
		})

		t.Run("GoSymbolReferences_NonExistentFile", func(t *testing.T) {
			tool := "go_symbol_references"
			args := map[string]any{
				"locator": map[string]any{
					"symbol_name":  "SomeSymbol",
					"context_file": "/nonexistent/file.go",
					"kind":         "function",
					"line_hint":    1,
				},
			}

			_, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})

			if err != nil {
				t.Logf("Expected error for nonexistent file: %v", err)
			}
			// Result can be nil or have error message
		})
	})

	t.Run("OutOfRangePosition", func(t *testing.T) {
		projectDir := t.TempDir()

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		sourceCode := `package main

func main() {
	println("hello")
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		t.Run("GoImplementation_OutOfRange", func(t *testing.T) {
			tool := "go_implementation"
			args := map[string]any{
				"file":   mainGoPath,
				"line":   9999, // Way beyond file size
				"column": 1,
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})

			// Should handle gracefully
			if err != nil {
				t.Logf("Expected error for out of range position: %v", err)
			} else if res != nil {
				content := testutil.ResultText(t, res, testutil.GoldenErrorHandling)
				t.Logf("Result for out of range position: %s", content)
				// Should not crash - may return empty or error message
			}
		})

		t.Run("GoSymbolReferences_OutOfRange", func(t *testing.T) {
			tool := "go_symbol_references"
			args := map[string]any{
				"locator": map[string]any{
					"symbol_name":  "NonExistentSymbol",
					"context_file": mainGoPath,
					"kind":         "function",
					"line_hint":    1,
				},
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})

			// Should handle gracefully - symbol not found is OK
			if err != nil {
				t.Logf("Got error for non-existent symbol: %v", err)
			} else if res != nil {
				content := testutil.ResultText(t, res, testutil.GoldenErrorHandling)
				t.Logf("Result for non-existent symbol: %s", content)
			}
		})
	})

	t.Run("MalformedGoCode", func(t *testing.T) {
		projectDir := t.TempDir()

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a file with syntax errors
		malformedCode := `package main

func main( {
	// Missing closing parenthesis
	println("hello"
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(malformedCode), 0644); err != nil {
			t.Fatal(err)
		}

		t.Run("GoDiagnostics_MalformedCode", func(t *testing.T) {
			tool := "go_build_check"
			args := map[string]any{
				"files": []string{mainGoPath},
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
			if err != nil {
				t.Fatalf("Failed to call go_build_check on malformed code: %v", err)
			}

			if res == nil {
				t.Fatal("Expected non-nil result")
			}

			content := testutil.ResultText(t, res, testutil.GoldenErrorHandling)
			t.Logf("Diagnostics for malformed code:\n%s", content)

			// Should report syntax errors
			if !strings.Contains(content, "error") &&
				!strings.Contains(content, "expected") &&
				!strings.Contains(content, "syntax") {
				t.Logf("Warning: Expected diagnostic errors for malformed code")
			}
		})

		t.Run("GoPackageAPI_MalformedCode", func(t *testing.T) {
			tool := "go_get_package_symbol_detail"
			args := map[string]any{
				"packagePaths":   []string{"example.com/test"},
				"include_bodies": false,
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})

			// Should handle gracefully - may return partial results or error
			if err != nil {
				t.Logf("Expected error for malformed code: %v", err)
			} else if res != nil {
				content := testutil.ResultText(t, res, testutil.GoldenErrorHandling)
				t.Logf("Package API for malformed code:\n%s", content)
			}
		})
	})

	t.Run("EmptyAndNullParameters", func(t *testing.T) {
		projectDir := t.TempDir()

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		sourceCode := `package main

func main() {
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		t.Run("GoPackageAPI_EmptyPackageList", func(t *testing.T) {
			tool := "go_get_package_symbol_detail"
			args := map[string]any{
				"packagePaths":   []string{}, // Empty list
				"include_bodies": false,
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})

			// Empty package list should error or return empty result
			if err != nil {
				t.Logf("Empty package list caused error: %v", err)
			} else if res != nil {
				content := testutil.ResultText(t, res, testutil.GoldenErrorHandling)
				t.Logf("Result for empty package list: %s", content)
			}
		})

		t.Run("GoSearch_EmptyQuery", func(t *testing.T) {
			tool := "go_search"
			args := map[string]any{
				"query": "", // Empty search query
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})

			// Empty query should return empty results or error
			if err != nil {
				t.Logf("Empty query caused error: %v", err)
			} else if res != nil {
				content := testutil.ResultText(t, res, testutil.GoldenErrorHandling)
				t.Logf("Result for empty query: %s", content)
			}
		})
	})

	t.Run("NonExistentSymbol", func(t *testing.T) {
		projectDir := t.TempDir()

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		sourceCode := `package main

func main() {
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		t.Run("GoRenameSymbol_NonExistent", func(t *testing.T) {
			// Find any line number (doesn't matter since symbol doesn't exist)
			lineNum := 1 // Use line 1 as a hint

			tool := "go_dryrun_rename_symbol"
			args := map[string]any{
				"locator": map[string]any{
					"symbol_name":  "NonExistentFunction",
					"context_file": mainGoPath,
					"line_hint":    lineNum,
				},
				"new_name": "NewName",
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})

			// Should handle gracefully - may error or return empty diff
			if err != nil {
				t.Logf("Expected error for non-existent symbol: %v", err)
			} else if res != nil {
				content := testutil.ResultText(t, res, testutil.GoldenErrorHandling)
				t.Logf("Rename result for non-existent symbol: %s", content)
				// Should indicate no changes or error
			}
		})
	})

	t.Run("VeryLongInput", func(t *testing.T) {
		projectDir := t.TempDir()

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		sourceCode := `package main

func main() {
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		t.Run("GoSearch_VeryLongQuery", func(t *testing.T) {
			// Create a very long search query
			longQuery := strings.Repeat("a", 10000)

			tool := "go_search"
			args := map[string]any{
				"query": longQuery,
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})

			// Should handle gracefully - may return empty or error
			if err != nil {
				t.Logf("Very long query caused error (acceptable): %v", err)
			} else if res != nil {
				content := testutil.ResultText(t, res, testutil.GoldenErrorHandling)
				t.Logf("Result for very long query (length: %d): %s", len(content), content[:min(100, len(content))])
			}
		})
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
