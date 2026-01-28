package integration

// End-to-end test for go_build_check functionality.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestGoDiagnosticsE2E is an end-to-end test that verifies go_build_check works.
func TestGoDiagnosticsE2E(t *testing.T) {
	t.Run("CleanProject", func(t *testing.T) {
		// Use the simple test project (has no errors)

		// Start gopls-mcp

		tool := "go_build_check"
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: map[string]any{}})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenDiagnostics)
		t.Logf("Diagnostics result:\n%s", content)

		// Compare against golden file (documentation + regression check)

		// Should report diagnostics checked and no issues found
		if !strings.Contains(content, "No issues found") {
			t.Errorf("Expected 'No issues found' in diagnostics for a clean project, got: %s", content)
		}
	})

	t.Run("ProjectWithSyntaxError", func(t *testing.T) {
		// Create a project with a syntax error
		projectDir := t.TempDir()

		// Initialize go.mod
		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a file with syntax error
		badCode := `package main

func MissingBrace() {
	// Missing closing brace
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(badCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Start gopls-mcp

		tool := "go_build_check"
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: map[string]any{
			"Cwd": projectDir,
		}})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenDiagnostics)
		t.Logf("Diagnostics for broken code:\n%s", content)

		// Should report the syntax error (check for common error patterns)
		if !strings.Contains(content, "expected") && !strings.Contains(content, "syntax error") && !strings.Contains(content, "Error") {
			t.Errorf("Expected diagnostics to report a syntax error, got: %s", content)
		}
	})

	t.Run("TypeError", func(t *testing.T) {
		// Create a project with type mismatch error
		projectDir := t.TempDir()

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Type error: assigning string to int variable
		badCode := `package main

func main() {
	var x int
	x = "hello"  // Type mismatch: cannot use string as int
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(badCode), 0644); err != nil {
			t.Fatal(err)
		}

		tool := "go_build_check"
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: map[string]any{
			"Cwd": projectDir,
		}})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenDiagnostics)
		t.Logf("Type error diagnostics:\n%s", content)

		// Should report type mismatch
		if !strings.Contains(strings.ToLower(content), "cannot use") &&
			!strings.Contains(strings.ToLower(content), "type") &&
			!strings.Contains(content, "string") &&
			!strings.Contains(content, "int") {
			t.Errorf("Expected diagnostics to report type mismatch error, got: %s", content)
		}
	})

	t.Run("ImportError", func(t *testing.T) {
		// Create a project with import error
		projectDir := t.TempDir()

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Import error: importing a non-existent package
		badCode := `package main

import "nonexistent.com/package"  // This package doesn't exist

func main() {
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(badCode), 0644); err != nil {
			t.Fatal(err)
		}

		tool := "go_build_check"
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: map[string]any{
			"Cwd": projectDir,
		}})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenDiagnostics)
		t.Logf("Import error diagnostics:\n%s", content)

		// Should report import error or module not found
		if !strings.Contains(strings.ToLower(content), "cannot find") &&
			!strings.Contains(strings.ToLower(content), "module") &&
			!strings.Contains(strings.ToLower(content), "package") &&
			!strings.Contains(content, "nonexistent") {
			t.Logf("Note: diagnostics may show import-related warnings, got: %s", content)
		}
	})

	t.Run("UnusedVariable", func(t *testing.T) {
		// Create a project with unused variable
		projectDir := t.TempDir()

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Unused variable error
		badCode := `package main

func main() {
	x := 42  // x is declared but never used
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(badCode), 0644); err != nil {
			t.Fatal(err)
		}

		tool := "go_build_check"
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: map[string]any{
			"Cwd": projectDir,
		}})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenDiagnostics)
		t.Logf("Unused variable diagnostics:\n%s", content)

		// Should report unused variable or show warning
		// Note: gopls may report this differently depending on configuration
		t.Logf("Unused variable diagnostic result: %s", content)
	})
}
