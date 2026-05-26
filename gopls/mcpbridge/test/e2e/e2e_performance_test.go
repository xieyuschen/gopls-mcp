package e2e

// E2E tests for PERFORMANCE on the kept semantic tools. The subtests that
// timed go_build_check, go_search, go_list_package_symbols,
// go_list_module_packages and go_analyze_workspace were removed along with
// those tools.

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// performanceTestCase defines a single performance test case.
type performanceTestCase struct {
	name        string
	tool        string
	args        map[string]any
	timeout     time.Duration
	assertion   func(t *testing.T, content string, duration time.Duration)
	description string
}

// runPerformanceTests executes performance test cases.
func runPerformanceTests(t *testing.T, testCases []performanceTestCase) {
	for _, tc := range testCases {
		tc := tc
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

// TestPerformance_DeepAnalysis tests performance of deep code analysis with
// the dependency graph tool.
func TestPerformance_DeepAnalysis(t *testing.T) {
	testCases := []performanceTestCase{
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

// TestPerformance_CallHierarchy tests call hierarchy performance.
func TestPerformance_CallHierarchy(t *testing.T) {
	wrappersPath := filepath.Join(globalGoplsMcpDir, "core", "gopls_wrappers.go")

	t.Run("IncomingCallHierarchy", func(t *testing.T) {
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

		if duration > 5*time.Second {
			t.Logf("Note: Call hierarchy took %v (expected < 5s)", duration)
		}

		contentLower := strings.ToLower(content)
		if !strings.Contains(contentLower, "call hierarchy") && !strings.Contains(contentLower, "incoming") && !strings.Contains(contentLower, "no incoming") {
			t.Logf("Note: Call hierarchy result: %s", testutil.TruncateString(content, 200))
		}
	})

	t.Run("OutgoingCallHierarchy", func(t *testing.T) {
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

		if duration > 5*time.Second {
			t.Logf("Note: Outgoing calls took %v (expected < 5s)", duration)
		}

		t.Log("Outgoing call hierarchy completed")
	})
}
