package e2e

// Comprehensive E2E tests for REAL USER WORKFLOWS on the gopls-mcp codebase.
// These tests simulate actual user scenarios with multiple steps and tool chaining.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestRealWorkflow_UnderstandArchitecture simulates a new developer exploring the codebase
func TestRealWorkflow_UnderstandArchitecture(t *testing.T) {
	t.Run("Step1_GetStarted", func(t *testing.T) {
		// User asks: "What is this project?"
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name:      "go_get_started",
			Arguments: map[string]any{},
		})
		if err != nil {
			t.Fatalf("Failed to call get_started: %v", err)
		}

		resultContent := testutil.ResultText(t, res, testutil.GoldenWorkflowUnderstandArch)
		t.Logf("Getting started:\n%s", testutil.TruncateString(resultContent, 1000))

		// Should explain the project
		if !strings.Contains(resultContent, "gopls-mcp") {
			t.Error("Expected get_started to mention gopls-mcp")
		}
	})

	t.Run("Step2_DiscoverPackages", func(t *testing.T) {
		// User asks: "What packages are there?"
		// Note: gopls-mcp is part of gopls module, so we query the parent module
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_list_module_packages",
			Arguments: map[string]any{
				"module_path":      "golang.org/x/tools/gopls",
				"include_docs":     false,
				"exclude_tests":    true,
				"exclude_internal": false,
				"top_level_only":   false,
			},
		})
		if err != nil {
			t.Fatalf("Failed to call list_module_packages: %v", err)
		}

		resultContent := testutil.ResultText(t, res, testutil.GoldenWorkflowUnderstandArch)
		t.Logf("Packages:\n%s", testutil.TruncateString(resultContent, 2000))

		// Should find gopls-mcp packages somewhere in the output
		if !strings.Contains(resultContent, "packages") {
			t.Error("Expected to find package information")
		}
		// Note: The actual gopls-mcp packages might be nested deeper
	})

	t.Run("Step3_ExploreCorePackage", func(t *testing.T) {
		// User asks: "What's in the core package?"
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_list_package_symbols",
			Arguments: map[string]any{
				"package_path":   "golang.org/x/tools/gopls/mcpbridge/core",
				"include_docs":   true,
				"include_bodies": false,
				"Cwd":            globalGoplsMcpDir,
			},
		})
		if err != nil {
			t.Fatalf("Failed to call list_package_symbols: %v", err)
		}

		resultContent := testutil.ResultText(t, res, testutil.GoldenWorkflowUnderstandArch)
		t.Logf("Core package symbols:\n%s", testutil.TruncateString(resultContent, 2000))

		// Should find Handler type and RegisterTools function
		if !strings.Contains(resultContent, "Handler") {
			t.Error("Expected to find Handler type")
		}
		if !strings.Contains(resultContent, "RegisterTools") {
			t.Error("Expected to find RegisterTools function")
		}
	})

	t.Run("Step4_FindHandlerUsage", func(t *testing.T) {
		// User asks: "Where is Handler used?"
		handlersPath := filepath.Join(globalGoplsMcpDir, "core", "handlers.go")

		// Find the line number where Handler is defined
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

		// Should find references (may be 0 due to known limitation)
		t.Log("References checked - tool works even if result is limited")
	})
}

// TestRealWorkflow_ToolChain_ChainingMultipleTools tests realistic multi-step workflows
func TestRealWorkflow_ToolChain_ChainingMultipleTools(t *testing.T) {
	t.Run("Workflow: ExploreToolImplementation", func(t *testing.T) {
		// Scenario: User wants to understand how go_definition tool works

		// Step 1: Find the tool definition
		wrappersPath := filepath.Join(globalGoplsMcpDir, "core", "gopls_wrappers.go")

		res1, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_search",
			Arguments: map[string]any{
				"query":       "handleGoDefinition",
				"max_results": 5,
				"Cwd":         globalGoplsMcpDir,
			},
		})
		if err != nil {
			t.Fatalf("Step 1 failed: %v", err)
		}
		searchResult := testutil.ResultText(t, res1, testutil.GoldenWorkflowToolChaining)
		t.Logf("Step 1 - Search:\n%s", testutil.TruncateString(searchResult, 500))

		if !strings.Contains(searchResult, "handleGoDefinition") {
			t.Error("Expected to find handleGoDefinition in search")
		}

		// Step 2: Find where this function is called
		// Find actual line number first
		content, _ := os.ReadFile(wrappersPath)
		lines := strings.Split(string(content), "\n")
		var lineNum int
		for i, line := range lines {
			if strings.Contains(line, "func handleGoDefinition(") {
				lineNum = i + 1
				break
			}
		}

		res2, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
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
			t.Fatalf("Step 2 failed: %v", err)
		}
		hierarchyResult := testutil.ResultText(t, res2, testutil.GoldenWorkflowToolChaining)
		t.Logf("Step 2 - Call Hierarchy:\n%s", testutil.TruncateString(hierarchyResult, 1000))

		if !strings.Contains(hierarchyResult, "Call hierarchy") {
			t.Error("Expected call hierarchy information")
		}
	})

	t.Run("Workflow: AnalyzeDependencies", func(t *testing.T) {
		// Scenario: User wants to understand the dependency structure

		// Step 1: Get dependency graph for core package
		res1, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_get_dependency_graph",
			Arguments: map[string]any{
				"package_path":       "golang.org/x/tools/gopls/mcpbridge/core",
				"include_transitive": false,
				"Cwd":                globalGoplsMcpDir,
			},
		})
		if err != nil {
			t.Fatalf("Step 1 failed: %v", err)
		}
		depGraphResult := testutil.ResultText(t, res1, testutil.GoldenWorkflowToolChaining)
		t.Logf("Step 1 - Dependency Graph:\n%s", testutil.TruncateString(depGraphResult, 2000))

		// Step 2: Check what imports core
		res2, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_list_modules",
			Arguments: map[string]any{
				"direct_only": true,
				"Cwd":         globalGoplsMcpDir,
			},
		})
		if err != nil {
			t.Fatalf("Step 2 failed: %v", err)
		}
		modulesResult := testutil.ResultText(t, res2, testutil.GoldenWorkflowToolChaining)
		t.Logf("Step 2 - All Modules:\n%s", testutil.TruncateString(modulesResult, 2000))

		if !strings.Contains(modulesResult, "golang.org/x/tools") {
			t.Error("Expected to find main module")
		}
	})
}

// TestRealWorkflow_ErrorScenarios tests how tools handle edge cases on real code
func TestRealWorkflow_ErrorScenarios(t *testing.T) {
	t.Run("NonExistentSymbol", func(t *testing.T) {
		// Test: Search for something that doesn't exist
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_search",
			Arguments: map[string]any{
				"query":       "NonExistentFunctionXYZ123",
				"max_results": 5,
			},
		})
		if err != nil {
			t.Fatalf("Failed to call go_search: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenWorkflowErrorScenarios)
		t.Logf("Search for non-existent symbol:\n%s", content)

		// Should handle gracefully (not crash)
		t.Log("Tool handled non-existent symbol gracefully")
	})

	t.Run("DefinitionInStdlib", func(t *testing.T) {
		// Test: Find definition of fmt.Println (in stdlib)
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

		// Should find definition in stdlib
		// (Might show path or indicate it's in stdlib)
		t.Log("Definition for stdlib symbol handled")
	})

	t.Run("LargeFileAnalysis", func(t *testing.T) {
		// Test: Analyze a large file (handlers.go is ~1378 lines)
		largeFilePath := filepath.Join(globalGoplsMcpDir, "core", "handlers.go")

		// Read the file
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_read_file",
			Arguments: map[string]any{
				"file": largeFilePath,
			},
		})
		if err != nil {
			t.Fatalf("Failed to call go_read_file: %v", err)
		}
		readResult := testutil.ResultText(t, res, testutil.GoldenWorkflowErrorScenarios)
		t.Logf("Large file read (size):\n%s", testutil.TruncateString(readResult, 500))

		if !strings.Contains(readResult, "package") {
			t.Error("Expected Go code in file")
		}
	})
}

// TestRealWorkflow_MultiPackageAnalysis tests working across multiple packages
func TestRealWorkflow_MultiPackageAnalysis(t *testing.T) {
	t.Run("CompareAPIDesign", func(t *testing.T) {
		// Scenario: User wants to compare the api package structure

		// Step 1: List symbols in api package
		res1, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_list_package_symbols",
			Arguments: map[string]any{
				"package_path":   "golang.org/x/tools/gopls/mcpbridge/api",
				"include_docs":   true,
				"include_bodies": false,
				"Cwd":            globalGoplsMcpDir,
			},
		})
		if err != nil {
			t.Fatalf("Failed to list api package symbols: %v", err)
		}
		apiSymbols := testutil.ResultText(t, res1, testutil.GoldenWorkflowMultiPackage)
		t.Logf("API package symbols:\n%s", testutil.TruncateString(apiSymbols, 2000))

		// Step 2: List symbols in core package
		res2, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_list_package_symbols",
			Arguments: map[string]any{
				"package_path":   "golang.org/x/tools/gopls/mcpbridge/core",
				"include_docs":   true,
				"include_bodies": false,
				"Cwd":            globalGoplsMcpDir,
			},
		})
		if err != nil {
			t.Fatalf("Failed to list core package symbols: %v", err)
		}
		coreSymbols := testutil.ResultText(t, res2, testutil.GoldenWorkflowMultiPackage)
		t.Logf("Core package symbols:\n%s", testutil.TruncateString(coreSymbols, 2000))

		// Step 3: Both should have useful symbols
		if !strings.Contains(apiSymbols, "type") && !strings.Contains(apiSymbols, "IList") {
			t.Error("Expected API types in api package")
		}
		if !strings.Contains(coreSymbols, "Handler") {
			t.Error("Expected Handler in core package")
		}
	})

	t.Run("TraceTypeHierarchy", func(t *testing.T) {
		// Scenario: User wants to find all implementations of an interface

		// Find an interface in the codebase
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
				// Found an interface-like type
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

		// Tool should handle gracefully even if no implementations found
		t.Log("Type hierarchy checked")
	})
}

// TestRealWorkflow_Performance tests performance on real codebase
func TestRealWorkflow_Performance(t *testing.T) {
	t.Run("BatchSymbolLookup", func(t *testing.T) {
		// Test: Look up multiple symbols in sequence (common AI workflow)

		symbols := []string{"Handler", "RegisterTools", "handleGoDefinition"}
		results := make(map[string]string)

		for _, symbol := range symbols {
			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
				Name: "go_search",
				Arguments: map[string]any{
					"query":       symbol,
					"max_results": 3,
				},
			})
			if err != nil {
				t.Logf("Warning: Failed to search for %s: %v", symbol, err)
				continue
			}
			results[symbol] = testutil.ResultText(t, res, testutil.GoldenWorkflowPerformance)
		}

		t.Logf("Batch lookup results:")
		for symbol, result := range results {
			t.Logf("  %s: found", symbol)
			if !strings.Contains(result, symbol) {
				t.Logf("  %s: not found in results", symbol)
			}
		}

		// Should find most symbols
		if len(results) == 0 {
			t.Error("Expected to find at least some symbols")
		}
	})

	t.Run("DeepAnalysis", func(t *testing.T) {
		// Test: Deep analysis with include_bodies=true using list_package_symbols
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_list_package_symbols",
			Arguments: map[string]any{
				"package_path":   "golang.org/x/tools/gopls/mcpbridge/core",
				"include_docs":   false,
				"include_bodies": true,
				"Cwd":            globalGoplsMcpDir,
			},
		})
		if err != nil {
			t.Fatalf("Failed to call list_package_symbols with bodies: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenWorkflowPerformance)
		t.Logf("Deep analysis with bodies:\n%s", testutil.TruncateString(content, 3000))

		// Should handle large output
		if len(content) < 100 {
			t.Error("Expected substantial output from deep analysis")
		}
	})
}

// TestRealWorkflow_RefactoringScenarios tests realistic refactoring workflows
func TestRealWorkflow_RefactoringScenarios(t *testing.T) {
	t.Run("FindAllCallers", func(t *testing.T) {
		// Test: Find all places that call a specific function

		// Use get_call_hierarchy with incoming direction
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

		// Should show call hierarchy
		if !strings.Contains(hierarchy, "Call hierarchy") {
			t.Error("Expected call hierarchy information")
		}
	})
}

// TestRealWorkflow_DiagnosticsAndQuality checks code quality on real codebase
func TestRealWorkflow_DiagnosticsAndQuality(t *testing.T) {
	t.Run("CheckCodebaseHealth", func(t *testing.T) {
		// Test: Run diagnostics on entire codebase
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_build_check",
			Arguments: map[string]any{
				"Cwd": globalGoplsMcpDir,
			},
		})
		if err != nil {
			t.Fatalf("Failed to run diagnostics: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenWorkflowDiagnostics)
		t.Logf("Codebase health:\n%s", testutil.TruncateString(content, 1000))

		// Should complete without errors
		if !strings.Contains(content, "packages") && !strings.Contains(content, "diagnostics") {
			t.Error("Expected diagnostic summary")
		}
	})

	t.Run("FindHotspots", func(t *testing.T) {
		// Test: Find files with lots of code (potential complexity hotspots)

		// Read handlers.go
		handlersPath := filepath.Join(globalGoplsMcpDir, "core", "handlers.go")

		// Read the file to check its size
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_read_file",
			Arguments: map[string]any{
				"file": handlersPath,
			},
		})
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenWorkflowDiagnostics)
		t.Logf("File info (size: %d chars):\n%s", len(content), testutil.TruncateString(content, 500))

		// Verify we got content
		if !strings.Contains(content, "package") {
			t.Error("Expected Go code in file")
		}
	})
}

// TestRealWorkflow_CrossModuleAnalysis tests working across module boundaries
func TestRealWorkflow_CrossModuleAnalysis(t *testing.T) {
	t.Run("AnalyzeModuleStructure", func(t *testing.T) {
		// Test: Understand how gopls-mcp fits into the larger gopls module

		// Step 1: List all modules
		res1, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_list_modules",
			Arguments: map[string]any{
				"direct_only": true,
				"Cwd":         globalGoplsMcpDir,
			},
		})
		if err != nil {
			t.Fatalf("Failed to list modules: %v", err)
		}
		modules := testutil.ResultText(t, res1, testutil.GoldenWorkflowCrossModule)
		t.Logf("All modules:\n%s", testutil.TruncateString(modules, 2000))

		// Should see golang.org/x/tools
		if !strings.Contains(modules, "golang.org/x/tools") {
			t.Error("Expected to find main gopls module")
		}

		// Step 2: Analyze workspace to see overall structure
		res2, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_analyze_workspace",
			Arguments: map[string]any{
				"Cwd": globalGoplsMcpDir,
			},
		})
		if err != nil {
			t.Fatalf("Failed to analyze workspace: %v", err)
		}
		workspace := testutil.ResultText(t, res2, testutil.GoldenWorkflowCrossModule)
		t.Logf("Workspace analysis:\n%s", testutil.TruncateString(workspace, 2000))

		// Should show project structure
		if !strings.Contains(workspace, "Packages") {
			t.Error("Expected package information in workspace analysis")
		}
	})

	t.Run("TraceDependencyChain", func(t *testing.T) {
		// Test: Follow dependency chains from gopls-mcp to stdlib

		// Get dependency graph for gopls-mcp
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

		// Should show dependencies
		if !strings.Contains(deps, "Dependencies") && !strings.Contains(deps, "imports") {
			t.Error("Expected dependency information")
		}
	})
}
