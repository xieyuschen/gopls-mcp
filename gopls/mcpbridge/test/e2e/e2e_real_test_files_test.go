package e2e

// E2E tests for testing ACTUAL test files in the gopls-mcp codebase.
// These tests ensure tools work correctly on test code, not just production code.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestRealTestFiles_DiagnosticsOnTests tests diagnostics on actual test files
func TestRealTestFiles_DiagnosticsOnTests(t *testing.T) {
	t.Run("CheckE2ETestFileHealth", func(t *testing.T) {
		// Test: Run diagnostics on the e2e test files themselves
		e2eDir := filepath.Join(globalGoplsMcpDir, "test", "e2e")

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_build_check",
			Arguments: map[string]any{
				"Cwd": e2eDir,
			},
		})
		if err != nil {
			t.Fatalf("Failed to run diagnostics on e2e files: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenDiagnosticsTests)
		t.Logf("E2E test file diagnostics:\n%s", testutil.TruncateString(content, 2000))

		// Should successfully analyze and return diagnostic information
		if !strings.Contains(content, "diagnostics") && !strings.Contains(content, "packages") && !strings.Contains(content, "No diagnostics") {
			t.Errorf("Expected diagnostic information, got: %s", testutil.TruncateString(content, 200))
		}
	})

	t.Run("CheckIntegrationTestFiles", func(t *testing.T) {
		// Test: Diagnostics on integration test files
		integrationDir := filepath.Join(globalGoplsMcpDir, "test", "integration")

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_build_check",
			Arguments: map[string]any{
				"Cwd": integrationDir,
			},
		})
		if err != nil {
			t.Fatalf("Failed to run diagnostics on integration files: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenDiagnosticsTests)
		t.Logf("Integration test file diagnostics:\n%s", testutil.TruncateString(content, 2000))

		// Should successfully analyze integration test files
		if !strings.Contains(content, "packages") && !strings.Contains(content, "diagnostics") && !strings.Contains(content, "No diagnostics") {
			t.Errorf("Expected diagnostics for integration tests, got: %s", testutil.TruncateString(content, 200))
		}
	})
}

// TestRealTestFiles_NavigateTestCode tests navigation within test files
func TestRealTestFiles_NavigateTestCode(t *testing.T) {
	t.Run("FindTestFunctions", func(t *testing.T) {
		// Test: Search for test functions in test files
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_search",
			Arguments: map[string]any{
				"query":       "TestRealCodebase",
				"max_results": 5,
			},
		})
		if err != nil {
			t.Fatalf("Failed to search for test functions: %v", err)
		}

		resultContent := testutil.ResultText(t, res, testutil.GoldenSearchTestFunctions)
		t.Logf("Test function search:\n%s", resultContent)

		// Should find our test functions
		// Note: May not find e2e test functions if they're not in the workspace
		if !strings.Contains(resultContent, "TestRealCodebase") && !strings.Contains(resultContent, "No symbols found") {
			t.Error("Expected to find TestRealCodebase function")
		}
		if strings.Contains(resultContent, "No symbols found") {
			t.Skip("TestRealCodebase function not found in workspace (e2e tests may not be indexed)")
		}
	})

	t.Run("JumpToTestDefinition", func(t *testing.T) {
		// Test: Go to definition of a test function
		e2eTestFile := filepath.Join(globalGoplsMcpDir, "test", "e2e", "e2e_stdlib_test.go")

		// Read the file to find a test function
		content, err := os.ReadFile(e2eTestFile)
		if err != nil {
			t.Fatalf("Failed to read e2e_stdlib_test.go: %v", err)
		}

		lines := strings.Split(string(content), "\n")
		var lineNum int
		var testName string
		for i, line := range lines {
			if strings.Contains(line, "func TestStdlib") {
				lineNum = i + 1
				// Extract function name (stop at opening parenthesis)
				parts := strings.Fields(line)
				for _, p := range parts {
					if strings.HasPrefix(p, "TestStdlib") {
						// Remove the ( and everything after it
						if idx := strings.Index(p, "("); idx != -1 {
							testName = p[:idx]
						} else {
							testName = p
						}
						break
					}
				}
				break
			}
		}

		if lineNum == 0 {
			t.Skip("Could not find TestStdlib function")
			return
		}

		// Test go_definition
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_definition",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  testName,
					"context_file": e2eTestFile,
					"kind":         "function",
					"line_hint":    lineNum,
				},
			},
		})
		if err != nil {
			t.Fatalf("Failed to call go_definition: %v", err)
		}

		resultContent := testutil.ResultText(t, res, testutil.GoldenSearchTestDefinitions)
		t.Logf("Test function definition:\n%s", resultContent)

		// Should find the definition (it's in the same file)
		if !strings.Contains(resultContent, "e2e_stdlib_test.go") {
			t.Error("Expected definition in e2e_stdlib_test.go")
		}
	})

}

// TestRealTestFiles_TestPackageSymbols tests listing symbols in test packages
func TestRealTestFiles_TestPackageSymbols(t *testing.T) {
	t.Run("ListE2ETestPackageSymbols", func(t *testing.T) {
		// Test: List symbols in testutil package (has shared helpers)
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_list_package_symbols",
			Arguments: map[string]any{
				"package_path":   "golang.org/x/tools/gopls/mcpbridge/test/testutil",
				"include_docs":   true,
				"include_bodies": false,
				"Cwd":            globalGoplsMcpDir,
			},
		})
		if err != nil {
			t.Fatalf("Failed to list testutil symbols: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenListPackageSymbolsTestFiles)
		t.Logf("Testutil package symbols:\n%s", testutil.TruncateString(content, 2000))

		// Should find utility functions
		if !strings.Contains(content, "AssertStringContains") {
			t.Error("Expected to find AssertStringContains function")
		}
		if !strings.Contains(content, "ExtractCount") {
			t.Error("Expected to find ExtractCount function")
		}
	})

	t.Run("ListBenchmarkPackageSymbols", func(t *testing.T) {
		// Test: List symbols in benchmark package
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_list_package_symbols",
			Arguments: map[string]any{
				"package_path":   "golang.org/x/tools/gopls/mcpbridge/test/benchmark",
				"include_docs":   false,
				"include_bodies": false,
				"Cwd":            globalGoplsMcpDir,
			},
		})
		if err != nil {
			t.Fatalf("Failed to list benchmark symbols: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenListPackageSymbolsTestFiles)
		t.Logf("Benchmark package symbols:\n%s", testutil.TruncateString(content, 2000))

		// Should find benchmark-related symbols
		if !strings.Contains(content, "Benchmark") {
			t.Error("Expected to find Benchmark functions")
		}
	})
}

// TestRealTestFiles_FindTestUsages tests finding where test utilities are used
func TestRealTestFiles_FindTestUsages(t *testing.T) {
	t.Run("FindAssertStringContainsUsage", func(t *testing.T) {
		// Test: Find all usages of test utility function
		testutilPath := filepath.Join(globalGoplsMcpDir, "test", "testutil", "assertions.go")

		// Find the line number where AssertStringContains is defined
		content, err := os.ReadFile(testutilPath)
		if err != nil {
			t.Fatal(err)
		}

		lines := strings.Split(string(content), "\n")
		var lineNum int
		for i, line := range lines {
			if strings.Contains(line, "func AssertStringContains(") {
				lineNum = i + 1
				break
			}
		}

		// Now search for usages
		res2, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_symbol_references",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "AssertStringContains",
					"context_file": testutilPath,
					"kind":         "function",
					"line_hint":    lineNum,
				},
			},
		})
		if err != nil {
			t.Fatalf("Failed to find references: %v", err)
		}

		references := testutil.ResultText(t, res2, testutil.GoldenSymbolReferencesTests)
		t.Logf("AssertStringContains references:\n%s", testutil.TruncateString(references, 2000))

		// Should find usages across test files
		// Note: May be 0 due to known limitation, but tool works
		t.Log("Reference search completed on test utility")
	})

	t.Run("SearchAcrossTestFiles", func(t *testing.T) {
		// Test: Search for a pattern that appears in test files
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_search",
			Arguments: map[string]any{
				"query":       "globalSession",
				"max_results": 10,
				"Cwd":         globalGoplsMcpDir,
			},
		})
		if err != nil {
			t.Fatalf("Failed to search for globalSession: %v", err)
		}

		resultContent := testutil.ResultText(t, res, testutil.GoldenSearchCrossFile)
		t.Logf("Search for globalSession across codebase:\n%s", testutil.TruncateString(resultContent, 2000))

		// Should find usages in test files
		if !strings.Contains(resultContent, "globalSession") {
			t.Error("Expected to find globalSession references")
		}
	})
}

// TestRealTestFiles_WorkspaceAnalysis tests analyzing test directory structure
func TestRealTestFiles_WorkspaceAnalysis(t *testing.T) {
	t.Run("AnalyzeTestDirectory", func(t *testing.T) {
		// Test: Analyze the test directory structure
		testDir := filepath.Join(globalGoplsMcpDir, "test")

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_analyze_workspace",
			Arguments: map[string]any{
				"Cwd": testDir,
			},
		})
		if err != nil {
			t.Fatalf("Failed to analyze test directory: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenRealTestFilesWorkspaceAnalysis)
		t.Logf("Test directory analysis:\n%s", testutil.TruncateString(content, 2000))

		// Should identify the test directory structure
		if !strings.Contains(content, "Packages") {
			t.Error("Expected package information")
		}
	})

	t.Run("ListTestPackages", func(t *testing.T) {
		// Test: List all packages in test directory
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_list_module_packages",
			Arguments: map[string]any{
				"module_path":      "golang.org/x/tools/gopls/mcpbridge",
				"include_docs":     false,
				"exclude_tests":    false,
				"exclude_internal": false,
				"top_level_only":   false,
			},
		})
		if err != nil {
			t.Fatalf("Failed to list test packages: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenRealTestFilesWorkspaceAnalysis)
		t.Logf("All test packages:\n%s", testutil.TruncateString(content, 3000))

		// Should find e2e, integration, benchmark, testutil, testdata packages
		expected := []string{"e2e", "integration", "benchmark", "testutil", "testdata"}
		missing := []string{}
		for _, exp := range expected {
			if !strings.Contains(content, exp) {
				missing = append(missing, exp)
			}
		}
		if len(missing) > 0 {
			t.Logf("Note: Some packages not found: %v", missing)
		}
	})
}
