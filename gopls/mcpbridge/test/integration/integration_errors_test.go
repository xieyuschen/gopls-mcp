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

		t.Run("GoDefinition_MalformedCode", func(t *testing.T) {
			// Even with syntax errors, looking up an undefined symbol must not crash.
			tool := "go_definition"
			args := map[string]any{
				"locator": map[string]any{
					"symbol_name":  "main",
					"context_file": mainGoPath,
				},
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
			if err != nil {
				t.Logf("Expected error against malformed code: %v", err)
			} else if res != nil {
				content := testutil.ResultText(t, res, testutil.GoldenErrorHandling)
				t.Logf("Definition for malformed code:\n%s", content)
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

		t.Run("GoDefinition_EmptySymbol", func(t *testing.T) {
			tool := "go_definition"
			args := map[string]any{
				"locator": map[string]any{
					"symbol_name":  "",
					"context_file": filepath.Join(projectDir, "main.go"),
				},
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
			if err != nil {
				t.Logf("Empty symbol_name caused error: %v", err)
			} else if res != nil {
				content := testutil.ResultText(t, res, testutil.GoldenErrorHandling)
				t.Logf("Result for empty symbol_name: %s", content)
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

		t.Run("GoDefinition_VeryLongSymbolName", func(t *testing.T) {
			longSymbol := strings.Repeat("a", 10000)
			tool := "go_definition"
			args := map[string]any{
				"locator": map[string]any{
					"symbol_name":  longSymbol,
					"context_file": filepath.Join(projectDir, "main.go"),
				},
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
			if err != nil {
				t.Logf("Very long symbol caused error (acceptable): %v", err)
			} else if res != nil {
				content := testutil.ResultText(t, res, testutil.GoldenErrorHandling)
				snippet := content
				if len(snippet) > 100 {
					snippet = snippet[:100]
				}
				t.Logf("Result for very long symbol (length: %d): %s", len(content), snippet)
			}
		})
	})
}
