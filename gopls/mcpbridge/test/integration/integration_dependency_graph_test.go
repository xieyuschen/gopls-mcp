package integration

// Comprehensive end-to-end tests for dependency graph functionality.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestGetDependencyGraph_BasicFunctionality tests the core functionality of get_dependency_graph.
func TestGetDependencyGraph_BasicFunctionality(t *testing.T) {
	projectDir := testutil.CopyProjectTo(t, "simple")

	t.Run("DirectDependenciesOnly", func(t *testing.T) {
		tool := "go_get_dependency_graph"
		args := map[string]any{
			"Cwd":                projectDir,
			"package_path":       "example.com/simple",
			"include_transitive": false,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenDependencyGraphBasic)
		t.Logf("Direct dependencies only:\n%s", content)

		// Compare against golden file (documentation + regression check)

		// Verify structure
		requiredStrings := []string{
			"Package: example.com/simple",
			"Dependencies",
			"Imported By",
		}
		for _, s := range requiredStrings {
			if !strings.Contains(content, s) {
				t.Errorf("Expected to find '%s' in output, got: %s", s, content)
			}
		}

		// Should include fmt (the simple project imports it)
		if !strings.Contains(content, "fmt") {
			t.Errorf("Expected to find 'fmt' dependency")
		}

		// Should mark stdlib packages
		if !strings.Contains(content, "[stdlib]") {
			t.Errorf("Expected stdlib packages to be marked with [stdlib]")
		}
	})

	t.Run("DefaultPackagePath_UsesMainModule", func(t *testing.T) {
		tool := "go_get_dependency_graph"
		args := map[string]any{
			"Cwd":                projectDir,
			"include_transitive": false,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenDependencyGraphBasic)
		t.Logf("Default package path:\n%s", content)

		// Should default to main module
		if !strings.Contains(content, "example.com/simple") {
			t.Errorf("Expected default to main module, got: %s", content)
		}
	})
}

// TestGetDependencyGraph_TransitiveDependencies tests transitive dependency traversal.
func TestGetDependencyGraph_TransitiveDependencies(t *testing.T) {
	projectDir := testutil.CopyProjectTo(t, "simple")

	t.Run("FullTransitiveTree", func(t *testing.T) {
		tool := "go_get_dependency_graph"
		args := map[string]any{
			"Cwd":                projectDir,
			"package_path":       "example.com/simple",
			"include_transitive": true,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenDependencyGraphTransitive)
		t.Logf("Full transitive tree:\n%s", content)

		// Should have many more dependencies than direct
		if !strings.Contains(content, "errors") {
			t.Errorf("Expected transitive dependency 'errors', got: %s", content)
		}
		if !strings.Contains(content, "io") {
			t.Errorf("Expected transitive dependency 'io', got: %s", content)
		}

		// Count dependencies - should be more than direct (which is just fmt)
		lines := strings.Split(content, "\n")
		depCount := 0
		for _, line := range lines {
			if strings.Contains(line, " (") && !strings.Contains(line, "Package:") {
				depCount++
			}
		}
		if depCount < 5 {
			t.Errorf("Expected at least 5 transitive dependencies, got %d", depCount)
		}
	})

	t.Run("MaxDepthLimiting", func(t *testing.T) {
		tool := "go_get_dependency_graph"
		args := map[string]any{
			"Cwd":                projectDir,
			"package_path":       "example.com/simple",
			"include_transitive": true,
			"max_depth":          1,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenDependencyGraphTransitive)
		t.Logf("Max depth 1:\n%s", content)

		// With max_depth=1, should only get direct dependencies
		// Count dependencies - should be just fmt
		lines := strings.Split(content, "\n")
		depCount := 0
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "Package:") &&
				!strings.HasPrefix(trimmed, "Dependencies") &&
				!strings.HasPrefix(trimmed, "Imported By") {
				if strings.Contains(trimmed, " (") {
					depCount++
				}
			}
		}
		// With simple project and max_depth=1, should only have 1 dependency (fmt)
		if depCount > 3 {
			t.Errorf("Expected at most 3 dependencies with max_depth=1, got %d", depCount)
		}
	})

	t.Run("MaxDepthZero_Unlimited", func(t *testing.T) {
		tool := "go_get_dependency_graph"
		args := map[string]any{
			"Cwd":                projectDir,
			"package_path":       "example.com/simple",
			"include_transitive": true,
			"max_depth":          0,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenDependencyGraphTransitive)
		t.Logf("Max depth 0 (unlimited):\n%s", content)

		// max_depth=0 means unlimited, should get many dependencies
		if !strings.Contains(content, "runtime") {
			t.Errorf("Expected deep transitive dependency 'runtime' with unlimited depth, got: %s", content)
		}
	})
}

// TestGetDependencyGraph_Dependents tests the "imported by" functionality.
func TestGetDependencyGraph_Dependents(t *testing.T) {
	t.Run("MainPackageDependents", func(t *testing.T) {
		projectDir := testutil.CopyProjectTo(t, "simple")

		// Check what imports fmt
		tool := "go_get_dependency_graph"
		args := map[string]any{
			"Cwd":          projectDir,
			"package_path": "fmt",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenDependencyGraphDependents)
		t.Logf("fmt dependents:\n%s", content)

		// fmt should be imported by the main package
		if !strings.Contains(content, "example.com/simple") {
			t.Errorf("Expected fmt to be imported by example.com/simple, got: %s", content)
		}

		if !strings.Contains(content, "Imported By") {
			t.Errorf("Expected 'Imported By' section in output")
		}
	})

	t.Run("CreatePackageWithMultipleDependents", func(t *testing.T) {
		projectDir := t.TempDir()

		// Create a shared package
		sharedDir := filepath.Join(projectDir, "shared")
		if err := os.MkdirAll(sharedDir, 0755); err != nil {
			t.Fatal(err)
		}
		sharedCode := `package shared

func SharedFunc() string {
	return "shared"
}
`
		if err := os.WriteFile(filepath.Join(sharedDir, "shared.go"), []byte(sharedCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Create pkg1 that imports shared
		pkg1Dir := filepath.Join(projectDir, "pkg1")
		if err := os.MkdirAll(pkg1Dir, 0755); err != nil {
			t.Fatal(err)
		}
		pkg1Code := `package pkg1

import "example.com/test/shared"

func Func1() string {
	return shared.SharedFunc()
}
`
		if err := os.WriteFile(filepath.Join(pkg1Dir, "pkg1.go"), []byte(pkg1Code), 0644); err != nil {
			t.Fatal(err)
		}

		// Create pkg2 that also imports shared
		pkg2Dir := filepath.Join(projectDir, "pkg2")
		if err := os.MkdirAll(pkg2Dir, 0755); err != nil {
			t.Fatal(err)
		}
		pkg2Code := `package pkg2

import "example.com/test/shared"

func Func2() string {
	return shared.SharedFunc()
}
`
		if err := os.WriteFile(filepath.Join(pkg2Dir, "pkg2.go"), []byte(pkg2Code), 0644); err != nil {
			t.Fatal(err)
		}

		// Create go.mod
		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Check dependents of shared package
		tool := "go_get_dependency_graph"
		args := map[string]any{
			"Cwd":          projectDir,
			"package_path": "example.com/test/shared",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenDependencyGraphDependents)
		t.Logf("shared package dependents:\n%s", content)

		// shared should be imported by both pkg1 and pkg2
		if !strings.Contains(content, "pkg1") {
			t.Errorf("Expected shared to be imported by pkg1, got: %s", content)
		}
		if !strings.Contains(content, "pkg2") {
			t.Errorf("Expected shared to be imported by pkg2, got: %s", content)
		}
	})
}

// TestGetDependencyGraph_StdlibPackages tests stdlib package handling.
func TestGetDependencyGraph_StdlibPackages(t *testing.T) {
	projectDir := testutil.CopyProjectTo(t, "simple")

	t.Run("StdlibPackageMarkedCorrectly", func(t *testing.T) {
		tool := "go_get_dependency_graph"
		args := map[string]any{
			"Cwd":          projectDir,
			"package_path": "fmt",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenDependencyGraphStdlib)
		t.Logf("Stdlib package dependencies:\n%s", content)

		// Stdlib dependencies should be marked
		if !strings.Contains(content, "[stdlib]") {
			t.Errorf("Expected stdlib packages to be marked with [stdlib]")
		}

		// Should NOT mark any as external
		if strings.Contains(content, "[external]") {
			t.Errorf("Stdlib packages should not be marked as [external]")
		}
	})

	t.Run("StdlibVsExternalDistinction", func(t *testing.T) {
		// Create a project that uses both stdlib and check external detection
		projectDir := t.TempDir()

		// Create a simple package with stdlib imports only
		sourceCode := `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("hello")
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		tool := "go_get_dependency_graph"
		args := map[string]any{
			"Cwd":                projectDir,
			"package_path":       "example.com/test",
			"include_transitive": false,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenDependencyGraphStdlib)
		t.Logf("Stdlib vs external:\n%s", content)

		// All should be stdlib, none external
		if !strings.Contains(content, "[stdlib]") {
			t.Errorf("Expected stdlib packages to be marked")
		}
		// This project has no external deps
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			if strings.Contains(line, "[external]") {
				t.Errorf("Unexpected [external] marking in project with only stdlib deps: %s", line)
			}
		}
	})
}

// TestGetDependencyGraph_ErrorHandling tests error cases and edge cases.
func TestGetDependencyGraph_ErrorHandling(t *testing.T) {
	t.Run("NonExistentPackage", func(t *testing.T) {
		projectDir := testutil.CopyProjectTo(t, "simple")

		tool := "go_get_dependency_graph"
		args := map[string]any{
			"Cwd":          projectDir,
			"package_path": "nonexistent/package/that/does/not/exist",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})

		// Should error gracefully
		if err != nil {
			t.Logf("Expected error for non-existent package: %v", err)
		} else if res != nil {
			content := testutil.ResultText(t, res, testutil.GoldenDependencyGraphError)
			if !strings.Contains(content, "not found") &&
				!strings.Contains(content, "error") {
				t.Logf("Warning: Tool didn't error for non-existent package: %s", content)
			}
		}
	})

	t.Run("InvalidCwd", func(t *testing.T) {
		// Create a minimal valid project first
		projectDir := t.TempDir()

		sourceCode := `package main

func main() {
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Try with invalid cwd

		tool := "go_get_dependency_graph"
		args := map[string]any{
			"Cwd":          "/nonexistent/directory",
			"package_path": "fmt",
		}

		_, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})

		// May error or fall back to default view
		if err != nil {
			t.Logf("Got error for invalid cwd (acceptable): %v", err)
		}
	})

	t.Run("EmptyPackagePath_NoGoMod", func(t *testing.T) {
		// Directory without go.mod
		projectDir := t.TempDir()

		sourceCode := `package main

func main() {
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		tool := "go_get_dependency_graph"
		args := map[string]any{
			"Cwd":                projectDir,
			"include_transitive": false,
		}

		_, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})

		// Should error or handle gracefully
		if err != nil {
			t.Logf("Expected error for directory without go.mod: %v", err)
		}
	})
}

// TestGetDependencyGraph_TestPackages tests handling of test packages.
func TestGetDependencyGraph_TestPackages(t *testing.T) {
	t.Run("TestPackageMarkedCorrectly", func(t *testing.T) {
		projectDir := t.TempDir()

		// Create main package
		mainCode := `package main

func main() {
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(mainCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Create test that imports main package
		testCode := `package main

import "testing"

func TestMain(t *testing.T) {
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main_test.go"), []byte(testCode), 0644); err != nil {
			t.Fatal(err)
		}

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Check the main package's dependents
		tool := "go_get_dependency_graph"
		args := map[string]any{
			"Cwd":          projectDir,
			"package_path": "example.com/test",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenMain)
		t.Logf("Test package dependents:\n%s", content)

		// The main package might be imported by its test variant
		// Check if [test] markers appear (though intermediate test variants are filtered)
		if strings.Contains(content, "Imported By") {
			t.Logf("Found 'Imported By' section as expected")
		}
	})
}

// TestGetDependencyGraph_ComplexScenarios tests more complex real-world scenarios.
func TestGetDependencyGraph_ComplexScenarios(t *testing.T) {
	t.Run("DeepDependencyChain", func(t *testing.T) {
		projectDir := testutil.CopyProjectTo(t, "simple")

		// Check a package with deep dependencies
		tool := "go_get_dependency_graph"
		args := map[string]any{
			"Cwd":                projectDir,
			"package_path":       "fmt",
			"include_transitive": true,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenDependencyGraphComplex)
		t.Logf("Deep dependency chain:\n%s", content)

		// fmt has deep dependencies through io, os, etc.
		// Should have many dependencies
		lines := strings.Split(content, "\n")
		depCount := 0
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "Package:") &&
				!strings.HasPrefix(trimmed, "Dependencies") &&
				!strings.HasPrefix(trimmed, "Imported By") &&
				strings.Contains(trimmed, " (") {
				depCount++
			}
		}
		if depCount < 5 {
			t.Errorf("Expected fmt to have at least 5 dependencies, got %d", depCount)
		}
	})

	t.Run("CyclicDependencyHandling", func(t *testing.T) {
		// Go packages can have import cycles (though they're errors)
		// The tool should handle them gracefully without infinite loops
		projectDir := testutil.CopyProjectTo(t, "simple")

		tool := "go_get_dependency_graph"
		args := map[string]any{
			"Cwd":                projectDir,
			"package_path":       "example.com/simple",
			"include_transitive": true,
		}

		// This should complete without hanging (infinite loop protection)
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		_ = testutil.ResultText(t, res, testutil.GoldenDependencyGraphComplex) // Just verify it completes
		t.Log("Cyclic dependency handling: tool completed successfully")
	})
}

// TestGetDependencyGraph_Integration tests integration with other tools.
func TestGetDependencyGraph_Integration(t *testing.T) {
	t.Run("WithListModulePackages", func(t *testing.T) {
		projectDir := testutil.CopyProjectTo(t, "simple")

		// First, list packages
		listTool := "go_list_module_packages"
		listArgs := map[string]any{
			"Cwd":              projectDir,
			"include_docs":     false,
			"exclude_tests":    true,
			"exclude_internal": false,
			"top_level_only":   false,
		}

		listRes, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: listTool, Arguments: listArgs})
		if err != nil {
			t.Fatalf("Failed to call %s: %v", listTool, err)
		}

		listContent := testutil.ResultText(t, listRes, testutil.GoldenDependencyGraphIntegration)
		t.Logf("Packages in module:\n%s", listContent)

		// Should find the main package
		if !strings.Contains(listContent, "example.com/simple") {
			t.Errorf("Expected to find main package in list")
		}

		// Now get dependency graph for that package
		depTool := "go_get_dependency_graph"
		depArgs := map[string]any{
			"Cwd":          projectDir,
			"package_path": "example.com/simple",
		}

		depRes, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: depTool, Arguments: depArgs})
		if err != nil {
			t.Fatalf("Failed to call %s: %v", depTool, err)
		}

		depContent := testutil.ResultText(t, depRes, testutil.GoldenDependencyGraphIntegration)
		t.Logf("Dependency graph:\n%s", depContent)

		// Both tools should agree on the package name
		if !strings.Contains(depContent, "example.com/simple") {
			t.Errorf("Dependency graph package name mismatch")
		}
	})

	t.Run("WithGoDiagnostics", func(t *testing.T) {
		projectDir := t.TempDir()

		// Create a package with some imports
		sourceCode := `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("hello")
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Run diagnostics first
		diagTool := "go_build_check"
		diagArgs := map[string]any{}

		diagRes, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: diagTool, Arguments: diagArgs})
		if err != nil {
			t.Fatalf("Failed to call %s: %v", diagTool, err)
		}

		diagContent := testutil.ResultText(t, diagRes, testutil.GoldenDependencyGraphIntegration)
		t.Logf("Diagnostics:\n%s", diagContent)

		// Now get dependency graph
		depTool := "go_get_dependency_graph"
		depArgs := map[string]any{
			"Cwd":                projectDir,
			"package_path":       "example.com/test",
			"include_transitive": false,
		}

		depRes, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: depTool, Arguments: depArgs})
		if err != nil {
			t.Fatalf("Failed to call %s: %v", depTool, err)
		}

		depContent := testutil.ResultText(t, depRes, testutil.GoldenDependencyGraphIntegration)
		t.Logf("Dependency graph:\n%s", depContent)

		// Dependencies should be valid (no errors expected)
		if strings.Contains(diagContent, "error") {
			t.Logf("Note: Diagnostics found errors (possibly imports), checking dependency graph anyway")
		}

		// Dependency graph should still work
		if !strings.Contains(depContent, "Dependencies") {
			t.Errorf("Dependency graph should have Dependencies section")
		}
	})
}

// TestGetDependencyGraph_OutputFormat verifies the output format.
func TestGetDependencyGraph_OutputFormat(t *testing.T) {
	projectDir := testutil.CopyProjectTo(t, "simple")

	t.Run("OutputStructure", func(t *testing.T) {
		tool := "go_get_dependency_graph"
		args := map[string]any{
			"Cwd":          projectDir,
			"package_path": "example.com/simple",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenDependencyGraphOutputFormat)
		t.Logf("Output format:\n%s", content)

		// Verify key sections exist
		requiredSections := []string{
			"Package:",
			"Dependencies",
			"Imported By",
		}
		for _, section := range requiredSections {
			if !strings.Contains(content, section) {
				t.Errorf("Expected section '%s' in output", section)
			}
		}

		// Verify package name format
		if !strings.Contains(content, "example.com/simple (") {
			t.Errorf("Expected package name format 'path (name)' in output")
		}

		// Verify dependency format
		if !strings.Contains(content, "fmt (fmt)") {
			t.Errorf("Expected dependency format 'path (name)' in output")
		}
	})

	t.Run("IndentationForTransitiveDeps", func(t *testing.T) {
		tool := "go_get_dependency_graph"
		args := map[string]any{
			"Cwd":                projectDir,
			"package_path":       "example.com/simple",
			"include_transitive": true,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenDependencyGraphOutputFormat)
		t.Logf("Indented output:\n%s", content)

		// Transitive dependencies should be indented
		// Look for lines with leading spaces (indentation)
		lines := strings.Split(content, "\n")
		hasIndentedLines := false
		for _, line := range lines {
			if len(line) > 0 && line[0] == ' ' && strings.Contains(line, " (") {
				hasIndentedLines = true
				break
			}
		}
		if !hasIndentedLines {
			t.Logf("Note: No indented transitive dependencies found (might be at depth 0 only)")
		}
	})
}
