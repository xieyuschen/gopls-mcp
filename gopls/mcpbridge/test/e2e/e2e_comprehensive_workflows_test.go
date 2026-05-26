package e2e

// Comprehensive E2E tests for REAL USER WORKFLOWS on the gopls-mcp codebase.
// These tests simulate actual multi-step user scenarios.
//
// Sub-workflows that relied on go_get_started, go_list_modules,
// go_list_module_packages, go_list_package_symbols, go_search, and
// go_build_check were dropped along with those tools; the remaining
// scenarios cover go_definition, go_implementation, go_symbol_references,
// go_get_call_hierarchy and go_get_dependency_graph.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestRealWorkflow_UnderstandArchitecture simulates a developer exploring how
// the Handler type is used across the codebase.
func TestRealWorkflow_UnderstandArchitecture(t *testing.T) {
	t.Run("FindHandlerUsage", func(t *testing.T) {
		handlersPath := filepath.Join(globalGoplsMcpDir, "core", "handlers.go")

		content, err := os.ReadFile(handlersPath)
		if err != nil {
			t.Fatal(err)
		}

		lines := strings.Split(string(content), "\n")
		var lineNum int
		for i, line := range lines {
			if strings.Contains(line, "type Handler struct") {
				lineNum = i + 1
				break
			}
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_symbol_references",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Handler",
					"context_file": handlersPath,
					"kind":         "struct",
					"line_hint":    lineNum,
				},
			},
		})
		if err != nil {
			t.Fatalf("Failed to call go_symbol_references: %v", err)
		}

		resultContent := testutil.ResultText(t, res, testutil.GoldenWorkflowUnderstandArch)
		t.Logf("Handler references:\n%s", testutil.TruncateString(resultContent, 2000))
		t.Log("References checked - tool works even if result is limited")
	})
}

// TestRealWorkflow_ToolChain_ChainingMultipleTools tests realistic multi-step
// workflows that combine call-hierarchy and dependency-graph tools.
func TestRealWorkflow_ToolChain_ChainingMultipleTools(t *testing.T) {
	t.Run("ExploreToolImplementation", func(t *testing.T) {
		wrappersPath := filepath.Join(globalGoplsMcpDir, "core", "gopls_wrappers.go")

		content, _ := os.ReadFile(wrappersPath)
		lines := strings.Split(string(content), "\n")
		var lineNum int
		for i, line := range lines {
			if strings.Contains(line, "func handleGoDefinition(") {
				lineNum = i + 1
				break
			}
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_get_call_hierarchy",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "handleGoDefinition",
					"context_file": wrappersPath,
					"kind":         "function",
					"line_hint":    lineNum,
				},
				"direction": "incoming",
			},
		})
		if err != nil {
			t.Fatalf("Failed to call go_get_call_hierarchy: %v", err)
		}

		hierarchyResult := testutil.ResultText(t, res, testutil.GoldenWorkflowToolChaining)
		t.Logf("Call hierarchy:\n%s", testutil.TruncateString(hierarchyResult, 1000))

		if !strings.Contains(hierarchyResult, "Call hierarchy") {
			t.Error("Expected call hierarchy information")
		}
	})

	t.Run("AnalyzeDependencies", func(t *testing.T) {
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_get_dependency_graph",
			Arguments: map[string]any{
				"package_path":       "golang.org/x/tools/gopls/mcpbridge/core",
				"include_transitive": false,
				"Cwd":                globalGoplsMcpDir,
			},
		})
		if err != nil {
			t.Fatalf("Failed to call go_get_dependency_graph: %v", err)
		}

		depGraphResult := testutil.ResultText(t, res, testutil.GoldenWorkflowToolChaining)
		t.Logf("Dependency graph:\n%s", testutil.TruncateString(depGraphResult, 2000))

		if !strings.Contains(depGraphResult, "Dependencies") && !strings.Contains(depGraphResult, "imports") {
			t.Error("Expected dependency information")
		}
	})
}

// TestRealWorkflow_ErrorScenarios tests how tools handle edge cases on real code.
func TestRealWorkflow_ErrorScenarios(t *testing.T) {
	t.Run("DefinitionInStdlib", func(t *testing.T) {
		mainPath := filepath.Join(globalGoplsMcpDir, "test", "testdata", "projects", "simple", "main.go")

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_definition",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Println",
					"context_file": mainPath,
					"kind":         "function",
					"line_hint":    27,
				},
			},
		})
		if err != nil {
			t.Fatalf("Failed to call go_definition: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenWorkflowErrorScenarios)
		t.Logf("Definition of fmt.Println:\n%s", content)

		t.Log("Definition for stdlib symbol handled")
	})
}

// TestRealWorkflow_MultiPackageAnalysis tests working across multiple packages.
func TestRealWorkflow_MultiPackageAnalysis(t *testing.T) {
	t.Run("TraceTypeHierarchy", func(t *testing.T) {
		apiPath := filepath.Join(globalGoplsMcpDir, "api", "gopls_types.go")

		content, err := os.ReadFile(apiPath)
		if err != nil {
			t.Skipf("Could not read api file: %v", err)
			return
		}

		lines := strings.Split(string(content), "\n")
		var interfaceLine int
		for i, line := range lines {
			if strings.Contains(line, "type I") && strings.Contains(line, " struct {") {
				interfaceLine = i + 1
				break
			}
		}

		if interfaceLine == 0 {
			t.Skip("Could not find interface type to test")
			return
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_implementation",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "I",
					"context_file": apiPath,
					"kind":         "interface",
					"line_hint":    interfaceLine,
				},
			},
		})
		if err != nil {
			t.Fatalf("Failed to call go_implementation: %v", err)
		}

		result := testutil.ResultText(t, res, testutil.GoldenWorkflowMultiPackage)
		t.Logf("Type hierarchy:\n%s", result)
		t.Log("Type hierarchy checked")
	})
}

// TestRealWorkflow_RefactoringScenarios tests realistic refactoring workflows.
func TestRealWorkflow_RefactoringScenarios(t *testing.T) {
	t.Run("FindAllCallers", func(t *testing.T) {
		wrappersPath := filepath.Join(globalGoplsMcpDir, "core", "gopls_wrappers.go")

		content, _ := os.ReadFile(wrappersPath)
		lines := strings.Split(string(content), "\n")
		var lineNum int
		for i, line := range lines {
			if strings.Contains(line, "func handleGoDefinition(") {
				lineNum = i + 1
				break
			}
		}

		if lineNum == 0 {
			t.Skip("Could not find handleGoDefinition function")
			return
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_get_call_hierarchy",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "handleGoDefinition",
					"context_file": wrappersPath,
					"kind":         "function",
					"line_hint":    lineNum,
				},
				"direction": "incoming",
			},
		})
		if err != nil {
			t.Fatalf("Failed to get call hierarchy: %v", err)
		}

		hierarchy := testutil.ResultText(t, res, testutil.GoldenWorkflowRefactoring)
		t.Logf("All callers:\n%s", testutil.TruncateString(hierarchy, 2000))

		if !strings.Contains(hierarchy, "Call hierarchy") {
			t.Error("Expected call hierarchy information")
		}
	})
}

// TestRealWorkflow_CrossModuleAnalysis follows dependency chains.
func TestRealWorkflow_CrossModuleAnalysis(t *testing.T) {
	t.Run("TraceDependencyChain", func(t *testing.T) {
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_get_dependency_graph",
			Arguments: map[string]any{
				"package_path":       "golang.org/x/tools/gopls/mcpbridge/core",
				"include_transitive": false,
				"Cwd":                globalGoplsMcpDir,
			},
		})
		if err != nil {
			t.Fatalf("Failed to get dependency graph: %v", err)
		}

		deps := testutil.ResultText(t, res, testutil.GoldenWorkflowCrossModule)
		t.Logf("Dependency chain:\n%s", testutil.TruncateString(deps, 2000))

		if !strings.Contains(deps, "Dependencies") && !strings.Contains(deps, "imports") {
			t.Error("Expected dependency information")
		}
	})
}
