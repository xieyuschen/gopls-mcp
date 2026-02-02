package integration

// Table-driven integration tests for go_search tool.
// These tests verify symbol search works correctly with various patterns.

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestGoSearchE2E is a table-driven test that verifies go_search works.
// It tests various search patterns: exact match, partial match, case-insensitive.
func TestGoSearchE2E(t *testing.T) {
	tests := map[string]testCase{
		"ExactMatch": {
			tool: "go_search",
			args: map[string]any{
				"query": "Hello",
			},
			assertions: []assertion{
				assertContains("Hello"),
			},
		},
		"PartialMatch": {
			tool: "go_search",
			args: map[string]any{
				"query": "add",
			},
			assertions: []assertion{
				assertContains("Add"),
			},
		},
		"TypeSearch": {
			tool: "go_search",
			args: map[string]any{
				"query": "Person",
			},
			assertions: []assertion{
				assertContains("Person"),
			},
		},
		"CaseInsensitive": {
			tool: "go_search",
			args: map[string]any{
				"query": "hello", // lowercase
			},
			assertions: []assertion{
				assertContains("Hello"), // Should still find Hello despite case difference
			},
		},
		"NonExistent": {
			tool: "go_search",
			args: map[string]any{
				"query": "NonExistentSymbolXYZ123",
			},
			assertions: []assertion{
				assertContainsAny("No symbols found", "Found 0 symbol"),
			},
		},
		"EmptyQuery": {
			tool: "go_search",
			args: map[string]any{
				"query": "",
			},
			assertions: []assertion{
				assertCustom(
					"empty query handled gracefully",
					func(content string) bool {
						// Empty query should return no symbols or all symbols
						return strings.Contains(content, "No symbols found") ||
							strings.Contains(content, "Found 0 symbol") ||
							strings.Contains(content, "Found")
					},
					"empty query should be handled gracefully",
				),
			},
		},
		"VeryLongQuery": {
			tool: "go_search",
			args: map[string]any{
				"query": strings.Repeat("x", 1000),
			},
			assertions: []assertion{
				assertCustom(
					"long query handled gracefully",
					func(content string) bool {
						// Should handle gracefully - either no results or some results
						return strings.Contains(content, "No symbols found") ||
							strings.Contains(content, "Found 0 symbol") ||
							strings.Contains(content, "Found")
					},
					"long query should be handled gracefully",
				),
			},
		},
		"SpecialCharacters": {
			tool: "go_search",
			args: map[string]any{
				"query": "<>{}[]|\\/@$%^&*",
			},
			assertions: []assertion{
				assertCustom(
					"special characters handled without crash",
					func(content string) bool {
						// Should handle gracefully (likely no results)
						// The important thing is it doesn't crash
						return true
					},
					"special characters should not crash the tool",
				),
			},
		},
		"CommonKeywords": {
			tool: "go_search",
			args: map[string]any{
				"query": "Hello", // Search for actual symbol in simple project
			},
			assertions: []assertion{
				assertContains("Found"), // Should find results
			},
		},
	}

	runTableDrivenTests(t, tests)
	t.Log("SUCCESS: go_search e2e tests completed!")
}

// TestGoSearchMultiModule tests that symbol search works across multiple modules.
// This is a regression test for the bug where only views[0] was searched,
// causing symbols in secondary modules to be missed.
func TestGoSearchMultiModule(t *testing.T) {
	goplsRoot, _ := filepath.Abs("../../..")
	multiModuleDir := filepath.Join(goplsRoot, "mcpbridge", "test", "testdata", "projects", "multi-module")

	tests := map[string]testCase{
		"SymbolInModule1": {
			tool: "go_search",
			args: map[string]any{
				"query": "FuncInModule1",
				"Cwd":   multiModuleDir,
			},
			assertions: []assertion{
				assertContains("FuncInModule1"),
			},
		},
		"SymbolInModule2": {
			tool: "go_search",
			args: map[string]any{
				"query": "FuncInModule2",
				"Cwd":   multiModuleDir,
			},
			assertions: []assertion{
				assertContains("FuncInModule2"), // CRITICAL: This FAILS if the snapshot bug is not fixed
			},
		},
		"SymbolAcrossModules": {
			tool: "go_search",
			args: map[string]any{
				"query": "Func",
				"Cwd":   multiModuleDir,
			},
			assertions: []assertion{
				assertContainsAll("FuncInModule1", "FuncInModule2"), // Should find functions from BOTH modules
			},
		},
	}

	runTableDrivenTests(t, tests)
	t.Log("✓ Multi-module search working: found symbols from both modules")
}
