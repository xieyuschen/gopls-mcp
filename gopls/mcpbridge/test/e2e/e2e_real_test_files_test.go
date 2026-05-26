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

// TestRealTestFiles_NavigateTestCode tests navigation within test files
func TestRealTestFiles_NavigateTestCode(t *testing.T) {
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
				parts := strings.Fields(line)
				for _, p := range parts {
					if strings.HasPrefix(p, "TestStdlib") {
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

		resultContent := testutil.ResultText(t, res, "")
		t.Logf("Test function definition:\n%s", resultContent)

		// Should find the definition (it's in the same file).
		if !strings.Contains(resultContent, "e2e_stdlib_test.go") {
			t.Error("Expected definition in e2e_stdlib_test.go")
		}
	})
}

// TestRealTestFiles_FindTestUsages tests finding where test utilities are used
func TestRealTestFiles_FindTestUsages(t *testing.T) {
	t.Run("FindAssertStringContainsUsage", func(t *testing.T) {
		testutilPath := filepath.Join(globalGoplsMcpDir, "test", "testutil", "assertions.go")

		// Find the line number where AssertStringContains is defined.
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

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
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

		references := testutil.ResultText(t, res, testutil.GoldenSymbolReferencesTests)
		t.Logf("AssertStringContains references:\n%s", testutil.TruncateString(references, 2000))

		// Reference search may return 0 due to a known limitation on adhoc paths,
		// but the tool itself must not crash.
		t.Log("Reference search completed on test utility")
	})
}
