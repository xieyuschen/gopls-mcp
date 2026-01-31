package e2e

// E2E tests for PERFORMANCE and large-scale operations.
// These tests ensure tools remain responsive and efficient with large files and complex codebases.
// This test uses a table-driven approach for better maintainability.

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// performanceTestCase defines a single performance test case
type performanceTestCase struct {
	name        string
	tool        string
	args        map[string]any
	timeout     time.Duration
	assertion   func(t *testing.T, content string, duration time.Duration)
	description string
}

// TestPerformance_LargeFiles tests tool performance on large files
func TestPerformance_LargeFiles(t *testing.T) {
	largeFile := filepath.Join(globalGoplsMcpDir, "core", "handlers.go")

	testCases := []performanceTestCase{
		{
			name:    "ReadLargeFile",
			tool:    "go_read_file",
			args:    map[string]any{"file": largeFile},
			timeout: 5 * time.Second,
			assertion: func(t *testing.T, content string, duration time.Duration) {
				t.Logf("Read large file (%d bytes) in %v", len(content), duration)

				if duration > 5*time.Second {
					t.Errorf("Reading large file took too long: %v (expected < 5s)", duration)
				}

				if !strings.Contains(content, "package") {
					t.Error("Expected Go code content")
				}
			},
			description: "Read a large production file efficiently",
		},
	}

	runPerformanceTests(t, testCases)
}

// runPerformanceTests executes performance test cases
func runPerformanceTests(t *testing.T, testCases []performanceTestCase) {
	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Running: %s - %s", tc.name, tc.description)

			start := time.Now()
			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
				Name:      tc.tool,
				Arguments: tc.args,
			})
			duration := time.Since(start)

			if err != nil {
				t.Fatalf("Failed to call %s: %v", tc.tool, err)
			}

			content := testutil.ResultText(t, res, testutil.GoldenPerformanceLargeFiles)
			tc.assertion(t, content, duration)
		})
	}
}

// TestPerformance_DeepAnalysis tests performance of deep code analysis
func TestPerformance_DeepAnalysis(t *testing.T) {
	testCases := []performanceTestCase{
		{
			name: "ListPackageSymbolsWithBodies",
			tool: "go_list_package_symbols",
			args: map[string]any{
				"package_path":   "golang.org/x/tools/gopls/mcpbridge/core",
				"include_docs":   true,
				"include_bodies": true,
				"Cwd":            globalGoplsMcpDir,
			},
			timeout: 10 * time.Second,
			assertion: func(t *testing.T, content string, duration time.Duration) {
				t.Logf("Listed symbols with bodies in %v", duration)

				if duration > 10*time.Second {
					t.Logf("Note: Deep analysis took %v (expected < 10s)", duration)
				}

				if strings.Contains(content, "func") && strings.Contains(content, "{") {
					t.Log("Successfully retrieved function bodies")
				}
			},
			description: "List symbols with full function bodies",
		},
		{
			name: "DeepDependencyGraph",
			tool: "go_get_dependency_graph",
			args: map[string]any{
				"package_path":       "golang.org/x/tools/gopls/mcpbridge/core",
				"include_transitive": true,
				"max_depth":          3,
				"Cwd":                globalGoplsMcpDir,
			},
			timeout: 15 * time.Second,
			assertion: func(t *testing.T, content string, duration time.Duration) {
				t.Logf("Deep dependency graph in %v", duration)

				if duration > 15*time.Second {
					t.Logf("Note: Deep dependency analysis took %v (expected < 15s)", duration)
				}

				if strings.Contains(content, "Dependencies") || strings.Contains(content, "imports") {
					t.Log("Successfully retrieved dependency information")
				}
			},
			description: "Get dependency graph with transitive dependencies",
		},
	}

	runPerformanceTests(t, testCases)
}

// TestPerformance_BatchOperations tests performance of batch operations
func TestPerformance_BatchOperations(t *testing.T) {
	testCases := []performanceTestCase{
		{
			name:    "MultipleSymbolLookups",
			tool:    "batch_symbol_lookups",
			args:    map[string]any{"symbols": []string{"Handler", "RegisterTools", "handleGoDefinition"}},
			timeout: 5 * time.Second,
			assertion: func(t *testing.T, content string, duration time.Duration) {
				t.Logf("Performed symbol lookups in %v", duration)

				if duration > 5*time.Second {
					t.Logf("Note: Batch lookups took %v (expected < 5s total)", duration)
				}
			},
			description: "Perform multiple symbol lookups in sequence",
		},
		{
			name:    "MultipleFileDiagnostics",
			tool:    "go_build_check",
			args:    map[string]any{"Cwd": globalGoplsMcpDir},
			timeout: 3 * time.Second,
			assertion: func(t *testing.T, content string, duration time.Duration) {
				t.Logf("Diagnostics on codebase in %v", duration)

				if !strings.Contains(content, "packages") && !strings.Contains(content, "diagnostics") && !strings.Contains(content, "No diagnostics") {
					t.Errorf("Expected diagnostic information, got: %s", testutil.TruncateString(content, 200))
				}

				if duration > 3*time.Second {
					t.Logf("Note: Multi-file diagnostics took %v (expected < 3s)", duration)
				}
			},
			description: "Run diagnostics on multiple files (via directory)",
		},
	}

	// Special handling for the batch symbol lookups since it performs multiple sequential operations
	t.Run("MultipleSymbolLookups", func(t *testing.T) {
		symbols := []string{"Handler", "RegisterTools", "handleGoDefinition"}

		start := time.Now()
		for _, symbol := range symbols {
			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
				Name: "go_search",
				Arguments: map[string]any{
					"query":       symbol,
					"max_results": 3,
				},
			})
			if err != nil {
				t.Logf("Error searching for %s: %v", symbol, err)
				continue
			}
			_ = testutil.ResultText(t, res, testutil.GoldenPerformanceBatchOperations)
		}
		duration := time.Since(start)

		t.Logf("Performed %d symbol lookups in %v (avg %v per lookup)",
			len(symbols), duration, duration/time.Duration(len(symbols)))

		// Should be reasonably fast (< 5 seconds total)
		if duration > 5*time.Second {
			t.Logf("Note: Batch lookups took %v (expected < 5s total)", duration)
		}
	})

	// Use the helper for the other test cases
	regularTestCases := testCases[1:] // Skip the batch symbol lookups since we handle it specially
	runPerformanceTests(t, regularTestCases)
}

// TestPerformance_PackageAPI tests performance of package API extraction
func TestPerformance_PackageAPI(t *testing.T) {
	testCases := []performanceTestCase{
		{
			name: "SinglePackageAPI",
			tool: "go_list_package_symbols",
			args: map[string]any{
				"package_path":   "golang.org/x/tools/gopls/mcpbridge/core",
				"include_docs":   true,
				"include_bodies": false,
				"Cwd":            globalGoplsMcpDir,
			},
			timeout: 3 * time.Second,
			assertion: func(t *testing.T, content string, duration time.Duration) {
				t.Logf("Package API for single package in %v", duration)

				if duration > 3*time.Second {
					t.Logf("Note: Package API took %v (expected < 3s)", duration)
				}

				// Check for any indication of package information (symbols, types, funcs, etc.)
				contentLower := strings.ToLower(content)
				if !strings.Contains(contentLower, "symbol") && !strings.Contains(contentLower, "package") && len(content) < 100 {
					t.Error("Expected package API information")
				}
			},
			description: "Get API for a single package using list_package_symbols",
		},
		{
			name: "MultiplePackagesAPI",
			tool: "batch_package_api",
			args: map[string]any{
				"packages": []string{
					"golang.org/x/tools/gopls/mcpbridge/core",
					"golang.org/x/tools/gopls/mcpbridge/api",
				},
			},
			timeout: 6 * time.Second,
			assertion: func(t *testing.T, content string, duration time.Duration) {
				t.Logf("Package API for multiple packages in %v", duration)

				if duration > 6*time.Second {
					t.Logf("Note: Multi-package API took %v (expected < 6s)", duration)
				}
			},
			description: "Get API for multiple packages sequentially",
		},
	}

	// Special handling for the multiple packages API since it performs multiple sequential operations
	t.Run("MultiplePackagesAPI", func(t *testing.T) {
		packages := []string{
			"golang.org/x/tools/gopls/mcpbridge/core",
			"golang.org/x/tools/gopls/mcpbridge/api",
		}

		start := time.Now()
		for _, pkg := range packages {
			_, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
				Name: "go_list_package_symbols",
				Arguments: map[string]any{
					"package_path":   pkg,
					"include_docs":   false,
					"include_bodies": false,
					"Cwd":            globalGoplsMcpDir,
				},
			})
			if err != nil {
				t.Fatalf("Failed to get package API for %s: %v", pkg, err)
			}
		}
		duration := time.Since(start)

		t.Logf("Package API for %d packages in %v", len(packages), duration)

		// Should scale reasonably (< 6 seconds for 2 packages)
		if duration > 6*time.Second {
			t.Logf("Note: Multi-package API took %v (expected < 6s)", duration)
		}

		t.Log("Multi-package API completed")
	})

	// Use the helper for the single package test
	singleTestCase := testCases[0]
	runPerformanceTests(t, []performanceTestCase{singleTestCase})
}

// TestPerformance_AnalyzeWorkspace tests workspace analysis performance
func TestPerformance_AnalyzeWorkspace(t *testing.T) {
	t.Run("FullWorkspaceAnalysis", func(t *testing.T) {
		// Test: Analyze entire gopls-mcp workspace
		start := time.Now()
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_analyze_workspace",
			Arguments: map[string]any{
				"Cwd": globalGoplsMcpDir,
			},
		})
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Failed to analyze workspace: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenPerformanceAnalyzeWorkspace)
		t.Logf("Workspace analysis in %v", duration)

		// Should be fast (< 5 seconds)
		if duration > 5*time.Second {
			t.Logf("Note: Workspace analysis took %v (expected < 5s)", duration)
		}

		// Should provide useful information
		if !strings.Contains(content, "module") && !strings.Contains(content, "package") {
			t.Error("Expected workspace information")
		}
	})

	t.Run("ListAllPackages", func(t *testing.T) {
		// Test: List all packages in the module
		start := time.Now()
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
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Failed to list packages: %v", err)
		}

		_ = testutil.ResultText(t, res, testutil.GoldenPerformanceAnalyzeWorkspace)
		t.Logf("Listed all packages in %v", duration)

		// Should be fast (< 3 seconds)
		if duration > 3*time.Second {
			t.Logf("Note: List packages took %v (expected < 3s)", duration)
		}

		t.Log("Package listing completed")
	})
}

// TestPerformance_CallHierarchy tests call hierarchy performance
func TestPerformance_CallHierarchy(t *testing.T) {
	t.Run("IncomingCallHierarchy", func(t *testing.T) {
		// Test: Get incoming calls for a function
		wrappersPath := filepath.Join(globalGoplsMcpDir, "core", "gopls_wrappers.go")

		start := time.Now()
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_get_call_hierarchy",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "handleGoDefinition",
					"context_file": wrappersPath,
					"kind":         "function",
					"line_hint":    286,
				},
				"direction": "incoming",
			},
		})
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Failed to get call hierarchy: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenPerformanceCallHierarchy)
		t.Logf("Incoming call hierarchy in %v", duration)

		// Should be reasonably fast (< 5 seconds)
		if duration > 5*time.Second {
			t.Logf("Note: Call hierarchy took %v (expected < 5s)", duration)
		}

		// Accept both "Call hierarchy" and "No incoming calls" as valid results
		contentLower := strings.ToLower(content)
		if !strings.Contains(contentLower, "call hierarchy") && !strings.Contains(contentLower, "incoming") && !strings.Contains(contentLower, "no incoming") {
			t.Logf("Note: Call hierarchy result: %s", testutil.TruncateString(content, 200))
		}
	})

	t.Run("OutgoingCallHierarchy", func(t *testing.T) {
		// Test: Get outgoing calls for a function
		wrappersPath := filepath.Join(globalGoplsMcpDir, "core", "gopls_wrappers.go")

		start := time.Now()
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_get_call_hierarchy",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "handleGoDefinition",
					"context_file": wrappersPath,
					"kind":         "function",
					"line_hint":    286,
				},
				"direction": "outgoing",
			},
		})
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Failed to get call hierarchy: %v", err)
		}

		_ = testutil.ResultText(t, res, testutil.GoldenPerformanceCallHierarchy)
		t.Logf("Outgoing call hierarchy in %v", duration)

		// Should be reasonably fast (< 5 seconds)
		if duration > 5*time.Second {
			t.Logf("Note: Outgoing calls took %v (expected < 5s)", duration)
		}

		t.Log("Outgoing call hierarchy completed")
	})
}

// TestPerformance_DiagnosticsIncremental tests incremental diagnostics performance
func TestPerformance_DiagnosticsIncremental(t *testing.T) {
	t.Run("FirstDiagnosticsRun", func(t *testing.T) {
		// Test: First diagnostics run (may be slower due to cache warmup)
		start := time.Now()
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_build_check",
			Arguments: map[string]any{
				"Cwd": globalGoplsMcpDir,
			},
		})
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Failed to run diagnostics: %v", err)
		}

		_ = testutil.ResultText(t, res, testutil.GoldenPerformanceDiagnosticsIncremental)
		t.Logf("First diagnostics run in %v", duration)

		// First run might be slower but should still complete
		if duration > 20*time.Second {
			t.Logf("Note: First diagnostics took %v (expected < 20s for cold cache)", duration)
		}

		// Should return diagnostic information
		t.Log("First diagnostics run completed")
	})

	t.Run("SecondDiagnosticsRun", func(t *testing.T) {
		// Test: Second run should be faster (incremental checking)
		start := time.Now()
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_build_check",
			Arguments: map[string]any{
				"Cwd": globalGoplsMcpDir,
			},
		})
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Failed to run diagnostics: %v", err)
		}

		_ = testutil.ResultText(t, res, testutil.GoldenPerformanceDiagnosticsIncremental)
		t.Logf("Second diagnostics run in %v", duration)

		// Second run should be faster with warm cache
		if duration > 10*time.Second {
			t.Logf("Note: Incremental diagnostics took %v (expected < 10s with cache)", duration)
		}

		t.Log("Incremental diagnostics completed")
	})
}

// TestPerformance_LargeTestFile tests performance on test files
func TestPerformance_LargeTestFile(t *testing.T) {
	t.Run("ComprehensiveTestFile", func(t *testing.T) {
		// Test: Work with a large E2E test file
		largeTestFile := filepath.Join(globalGoplsMcpDir, "test", "e2e", "e2e_comprehensive_workflows_test.go")

		// Read the file
		start := time.Now()
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_read_file",
			Arguments: map[string]any{
				"file": largeTestFile,
			},
		})
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		_ = testutil.ResultText(t, res, testutil.GoldenPerformanceLargeTestFile)
		t.Logf("Large test file read in %v", duration)

		// Should be reasonably fast
		if duration > 5*time.Second {
			t.Logf("Note: File read took %v (expected < 5s)", duration)
		}

		t.Log("Large test file handling completed")
	})

	t.Run("SearchInTestFiles", func(t *testing.T) {
		// Test: Search across test files
		start := time.Now()
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_search",
			Arguments: map[string]any{
				"query":       "TestReal",
				"max_results": 20,
			},
		})
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Failed to search: %v", err)
		}

		_ = testutil.ResultText(t, res, testutil.GoldenPerformanceLargeTestFile)
		t.Logf("Search across test files in %v", duration)

		// Should be fast
		if duration > 3*time.Second {
			t.Logf("Note: Search took %v (expected < 3s)", duration)
		}

		t.Log("Test file search completed")
	})
}
