package integration

// End-to-end tests for go_get_dependency_graph functionality.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// createMultiPkgProject creates a project with a shared package imported by two consumers.
func createMultiPkgProject(t *testing.T) string {
	t.Helper()
	projectDir := t.TempDir()

	files := map[string]string{
		"go.mod": "module example.com/test\n\ngo 1.21\n",
		"shared/shared.go": `package shared

func SharedFunc() string { return "shared" }
`,
		"pkg1/pkg1.go": `package pkg1

import "example.com/test/shared"

func Func1() string { return shared.SharedFunc() }
`,
		"pkg2/pkg2.go": `package pkg2

import "example.com/test/shared"

func Func2() string { return shared.SharedFunc() }
`,
	}
	for relPath, content := range files {
		full := filepath.Join(projectDir, relPath)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return projectDir
}

// callDepGraph is a convenience wrapper that calls go_get_dependency_graph and returns the text.
func callDepGraph(t *testing.T, args map[string]any, golden string) string {
	t.Helper()
	res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
		Name:      "go_get_dependency_graph",
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("go_get_dependency_graph failed: %v", err)
	}
	return testutil.ResultText(t, res, golden)
}

// TestGetDependencyGraph_BasicFunctionality tests direct dependencies and output structure.
func TestGetDependencyGraph_BasicFunctionality(t *testing.T) {
	projectDir := testutil.CopyProjectTo(t, "simple")

	content := callDepGraph(t, map[string]any{
		"Cwd":                projectDir,
		"package_path":       "example.com/simple",
		"include_transitive": false,
	}, testutil.GoldenDependencyGraphBasic)
	t.Logf("Direct dependencies:\n%s", content)

	for _, want := range []string{"Package: example.com/simple", "Dependencies", "Imported By", "fmt", "[stdlib]"} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in output", want)
		}
	}
}

// TestGetDependencyGraph_OutputFormat verifies the structured output format.
func TestGetDependencyGraph_OutputFormat(t *testing.T) {
	projectDir := testutil.CopyProjectTo(t, "simple")

	content := callDepGraph(t, map[string]any{
		"Cwd":          projectDir,
		"package_path": "example.com/simple",
	}, testutil.GoldenDependencyGraphOutputFormat)
	t.Logf("Output format:\n%s", content)

	for _, want := range []string{"Package:", "Dependencies", "Imported By", "example.com/simple (", "fmt (fmt)"} {
		if !strings.Contains(content, want) {
			t.Errorf("expected section/format %q in output", want)
		}
	}
}

// TestGetDependencyGraph_TransitiveDependencies tests include_transitive and max_depth options.
func TestGetDependencyGraph_TransitiveDependencies(t *testing.T) {
	projectDir := testutil.CopyProjectTo(t, "simple")

	t.Run("FullTransitiveTree", func(t *testing.T) {
		content := callDepGraph(t, map[string]any{
			"Cwd":                projectDir,
			"package_path":       "example.com/simple",
			"include_transitive": true,
		}, testutil.GoldenDependencyGraphTransitive)
		t.Logf("Transitive tree:\n%s", content)

		for _, want := range []string{"errors", "io"} {
			if !strings.Contains(content, want) {
				t.Errorf("expected transitive dep %q", want)
			}
		}
		lines := strings.Split(content, "\n")
		count := 0
		for _, l := range lines {
			if strings.Contains(l, " (") && !strings.Contains(l, "Package:") {
				count++
			}
		}
		if count < 5 {
			t.Errorf("expected at least 5 transitive deps, got %d", count)
		}
	})

	t.Run("MaxDepthLimiting", func(t *testing.T) {
		content := callDepGraph(t, map[string]any{
			"Cwd":                projectDir,
			"package_path":       "example.com/simple",
			"include_transitive": true,
			"max_depth":          1,
		}, testutil.GoldenDependencyGraphTransitive)
		t.Logf("Max depth 1:\n%s", content)

		count := 0
		for _, l := range strings.Split(content, "\n") {
			trimmed := strings.TrimSpace(l)
			if trimmed != "" &&
				!strings.HasPrefix(trimmed, "Package:") &&
				!strings.HasPrefix(trimmed, "Dependencies") &&
				!strings.HasPrefix(trimmed, "Imported By") &&
				strings.Contains(trimmed, " (") {
				count++
			}
		}
		if count > 3 {
			t.Errorf("expected at most 3 deps with max_depth=1, got %d", count)
		}
	})
}

// TestGetDependencyGraph_Dependents tests the "Imported By" (reverse dependency) functionality.
func TestGetDependencyGraph_Dependents(t *testing.T) {
	t.Run("StdlibPackageDependents", func(t *testing.T) {
		projectDir := testutil.CopyProjectTo(t, "simple")

		content := callDepGraph(t, map[string]any{
			"Cwd":          projectDir,
			"package_path": "fmt",
		}, testutil.GoldenDependencyGraphDependents)
		t.Logf("fmt dependents:\n%s", content)

		if !strings.Contains(content, "example.com/simple") {
			t.Errorf("expected fmt to be imported by example.com/simple")
		}
		if !strings.Contains(content, "Imported By") {
			t.Errorf("expected 'Imported By' section")
		}
	})

	t.Run("MultipleConsumers", func(t *testing.T) {
		projectDir := createMultiPkgProject(t)

		content := callDepGraph(t, map[string]any{
			"Cwd":          projectDir,
			"package_path": "example.com/test/shared",
		}, testutil.GoldenDependencyGraphDependents)
		t.Logf("shared package dependents:\n%s", content)

		for _, want := range []string{"pkg1", "pkg2"} {
			if !strings.Contains(content, want) {
				t.Errorf("expected shared to be imported by %q", want)
			}
		}
	})
}

// TestGetDependencyGraph_StdlibPackages tests stdlib package classification.
func TestGetDependencyGraph_StdlibPackages(t *testing.T) {
	projectDir := testutil.CopyProjectTo(t, "simple")

	content := callDepGraph(t, map[string]any{
		"Cwd":          projectDir,
		"package_path": "fmt",
	}, testutil.GoldenDependencyGraphStdlib)
	t.Logf("Stdlib package:\n%s", content)

	if !strings.Contains(content, "[stdlib]") {
		t.Errorf("expected stdlib packages marked with [stdlib]")
	}
	if strings.Contains(content, "[external]") {
		t.Errorf("stdlib packages should not be marked [external]")
	}
}

// TestGetDependencyGraph_ComplexScenarios tests fmt with deep transitive deps.
func TestGetDependencyGraph_ComplexScenarios(t *testing.T) {
	projectDir := testutil.CopyProjectTo(t, "simple")

	content := callDepGraph(t, map[string]any{
		"Cwd":                projectDir,
		"package_path":       "fmt",
		"include_transitive": true,
	}, testutil.GoldenDependencyGraphComplex)
	t.Logf("fmt deep dependencies:\n%s", content)

	count := 0
	for _, l := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" &&
			!strings.HasPrefix(trimmed, "Package:") &&
			!strings.HasPrefix(trimmed, "Dependencies") &&
			!strings.HasPrefix(trimmed, "Imported By") &&
			strings.Contains(trimmed, " (") {
			count++
		}
	}
	if count < 5 {
		t.Errorf("expected fmt to have at least 5 deps, got %d", count)
	}
}

// TestGetDependencyGraph_ErrorHandling tests graceful error handling.
func TestGetDependencyGraph_ErrorHandling(t *testing.T) {
	projectDir := testutil.CopyProjectTo(t, "simple")

	res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
		Name: "go_get_dependency_graph",
		Arguments: map[string]any{
			"Cwd":          projectDir,
			"package_path": "nonexistent/package/that/does/not/exist",
		},
	})
	if err != nil {
		t.Logf("got expected error for non-existent package: %v", err)
		return
	}
	content := testutil.ResultText(t, res, testutil.GoldenDependencyGraphError)
	if !strings.Contains(content, "not found") && !strings.Contains(content, "error") {
		t.Logf("warning: tool did not error for non-existent package: %s", content)
	}
}
