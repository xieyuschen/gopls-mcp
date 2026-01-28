package integration

// End-to-end tests for empty Cwd parameter handling.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestEmptyCwdE2E tests that tools work correctly when Cwd parameter is empty or not provided.
// This is CRITICAL - we added Cwd field to IAnalyzeWorkspaceParams and other tools,
// and need to verify the fallback to default view works correctly.
func TestEmptyCwdE2E(t *testing.T) {
	t.Run("ListModules_NoCwd", func(t *testing.T) {
		// Use the simple test project

		tool := "go_list_modules"
		args := map[string]any{
			"direct_only": true,
			// Don't provide "cwd" at all - should use default view
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenEmptyCWD)
		t.Logf("List modules (no Cwd):\n%s", content)

		// Should find modules from the project directory
		if !strings.Contains(content, "example.com/simple") {
			t.Errorf("Expected to find 'example.com/simple' module when Cwd not provided, got: %s", content)
		}
	})

	t.Run("ListModules_EmptyStringCwd", func(t *testing.T) {

		tool := "go_list_modules"
		args := map[string]any{
			"direct_only": true,
			"Cwd":         "", // Explicitly pass empty string
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenEmptyCWD)
		t.Logf("List modules (empty Cwd string):\n%s", content)

		// Should find modules from the project directory
		if !strings.Contains(content, "example.com/simple") {
			t.Errorf("Expected to find 'example.com/simple' module when Cwd is empty string, got: %s", content)
		}
	})

	t.Run("ListModulePackages_NoCwd", func(t *testing.T) {

		tool := "go_list_module_packages"
		args := map[string]any{
			// Don't provide "cwd"
			"include_docs":     false,
			"exclude_tests":    false,
			"exclude_internal": false,
			"top_level_only":   false,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenEmptyCWD)
		t.Logf("List module packages (no Cwd):\n%s", content)

		// Should find packages from the project directory
		if !strings.Contains(content, "simple") {
			t.Errorf("Expected to find 'simple' package when Cwd not provided, got: %s", content)
		}
	})

	t.Run("AnalyzeWorkspace_NoCwd", func(t *testing.T) {

		tool := "go_analyze_workspace"
		args := map[string]any{
			// Don't provide "cwd"
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenEmptyCWD)
		t.Logf("Analyze workspace (no Cwd):\n%s", content)

		// Should analyze the workspace
		if !strings.Contains(content, "example.com/simple") {
			t.Errorf("Expected workspace analysis to find module when Cwd not provided, got: %s", content)
		}
	})

	t.Run("GetStarted_NoCwd", func(t *testing.T) {

		tool := "go_get_started"
		args := map[string]any{
			// Don't provide "cwd"
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenEmptyCWD)
		t.Logf("Get started (no Cwd):\n%s", content)

		// Should provide getting started guide
		if !strings.Contains(content, "example.com/simple") {
			t.Errorf("Expected get_started to find module when Cwd not provided, got: %s", content)
		}
	})

	t.Run("GoDiagnostics_NoCwd", func(t *testing.T) {
		// Create a test project with some code
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
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		tool := "go_build_check"
		args := map[string]any{
			// Don't provide "cwd" or "files"
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenEmptyCWD)
		t.Logf("Diagnostics (no Cwd):\n%s", content)

		// Should return some diagnostic result (even if no errors)
		if content == "" {
			t.Error("Expected diagnostic output even when Cwd not provided")
		}
	})

	t.Run("InvalidCwd_DoesNotCrash", func(t *testing.T) {

		tool := "go_list_modules"
		args := map[string]any{
			"direct_only": true,
			"Cwd":         "/nonexistent/directory/that/does/not/exist",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})

		// Invalid Cwd should error gracefully, not crash
		if err != nil {
			t.Logf("Expected error for invalid Cwd: %v", err)
		} else if res != nil {
			content := testutil.ResultText(t, res, testutil.GoldenEmptyCWD)
			t.Logf("Result for invalid Cwd: %s", content)

			// If no error, should mention the issue
			if !strings.Contains(content, "not found") &&
				!strings.Contains(content, "no such") &&
				!strings.Contains(content, "error") &&
				!strings.Contains(content, "failed") {
				t.Logf("Warning: Tool didn't error for invalid Cwd, but that might be OK if it falls back to default view")
			}
		}
	})

	t.Run("ConsistentAcrossTools_NoCwd", func(t *testing.T) {
		// Verify that all tools use the same default view when Cwd is not provided

		tools := []string{
			"go_list_modules",
			"go_list_module_packages",
			"go_analyze_workspace",
			"go_get_started",
		}

		results := make(map[string]string)

		for _, toolName := range tools {
			args := map[string]any{} // No Cwd

			// Add required parameters for specific tools
			if toolName == "go_list_modules" {
				args["direct_only"] = true
			}
			if toolName == "go_list_module_packages" {
				args["include_docs"] = false
				args["exclude_tests"] = false
				args["exclude_internal"] = false
				args["top_level_only"] = false
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: toolName, Arguments: args})
			if err != nil {
				t.Fatalf("Tool %s failed with no Cwd: %v", toolName, err)
			}

			if res == nil {
				t.Fatalf("Tool %s returned nil result with no Cwd", toolName)
			}

			content := testutil.ResultText(t, res, testutil.GoldenEmptyCWD)
			results[toolName] = content

			// All tools should find the same module
			if !strings.Contains(content, "example.com/simple") {
				t.Errorf("Tool %s did not find 'example.com/simple' module with no Cwd", toolName)
			}
		}

		t.Logf("All tools consistently found module with no Cwd provided")
		for toolName, content := range results {
			t.Logf("%s: %d characters", toolName, len(content))
		}
	})
}
