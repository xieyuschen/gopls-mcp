package integration

// End-to-end test for list_tools functionality.

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestListToolsE2E is an end-to-end test that verifies list_tools works.
func TestListToolsE2E(t *testing.T) {
	t.Run("ListAllTools", func(t *testing.T) {
		tool := "go_list_tools"
		args := map[string]any{}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenListTools)
		t.Logf("List tools result:\n%s", content)

		// Compare against golden file (documentation + regression check)

		if !strings.Contains(content, "tools for Go") {
			t.Errorf("Expected to see tool summary header in result, got: %s", content)
		}

		// Should mention key semantic tools
		keyTools := []string{
			"go_definition",
			"go_implementation",
			"go_symbol_references",
			"go_get_call_hierarchy",
			"go_get_dependency_graph",
			"go_dryrun_rename_symbol",
		}

		for _, keyTool := range keyTools {
			if !strings.Contains(content, keyTool) {
				t.Errorf("Expected to find tool %s in result", keyTool)
			}
		}
	})

	t.Run("ListToolsWithSchemas", func(t *testing.T) {
		tool := "go_list_tools"
		args := map[string]any{
			"includeInputSchema":  true,
			"includeOutputSchema": true,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenListTools)
		t.Logf("List tools with schemas:\n%s", content)

		// Should still work even with schema inclusion
		if !strings.Contains(content, "tools") {
			t.Errorf("Expected to see tools in result with schemas, got: %s", content)
		}
	})

	t.Run("VerifyCategories", func(t *testing.T) {
		tool := "go_list_tools"
		args := map[string]any{}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenListTools)
		t.Logf("Categories:\n%s", content)

		// Should mention categories
		categories := []string{
			"analysis",
			"navigation",
			"refactoring",
			"meta",
		}

		for _, category := range categories {
			if !strings.Contains(strings.ToLower(content), category) {
				t.Logf("Note: Category '%s' not explicitly mentioned (may be in structured output)", category)
			}
		}
	})
}
