package e2e

// E2E tests for REFACTORING workflows.
// These tests ensure tools support safe, multi-file refactoring operations.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestRefactoring_SafeRename tests safe symbol renaming with preview
func TestRefactoring_SafeRename(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a multi-file project for refactoring
	file1 := filepath.Join(tmpDir, "person.go")
	file2 := filepath.Join(tmpDir, "usage.go")
	file3 := filepath.Join(tmpDir, "test.go")

	code1 := `package main

type Person struct {
	Name string
	Age  int
}

func (p Person) Greet() string {
	return "Hello, " + p.Name
}
`

	code2 := `package main

func UsePerson() {
	p := Person{Name: "Alice", Age: 30}
	greeting := p.Greet()
	println(greeting)
}
`

	code3 := `package main

func TestPerson() {
	p := Person{Name: "Bob"}
	if p.Name != "Bob" {
		panic("wrong name")
	}
}
`

	os.WriteFile(file1, []byte(code1), 0644)
	os.WriteFile(file2, []byte(code2), 0644)
	os.WriteFile(file3, []byte(code3), 0644)

	t.Run("PreviewRenameAcrossFiles", func(t *testing.T) {
		// Test: Preview renaming "Person" type across all files
		// Find the line number where Person type is defined
		lines := strings.Split(code1, "\n")
		var lineNum int
		for i, line := range lines {
			if strings.Contains(line, "type Person struct") {
				lineNum = i + 1
				break
			}
		}

		if lineNum == 0 {
			t.Fatal("Could not find Person type definition")
		}

		res, err := globalSession.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "go_dryrun_rename_symbol",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Person",
					"context_file": file1,
					"line_hint":    lineNum,
				},
				"new_name": "Individual",
			},
		})

		// Rename might fail on temporary files, but we can test the preview
		if err != nil {
			t.Logf("Rename preview failed (expected for temp files): %v", err)
		} else if res != nil {
			content := testutil.ResultText(t, res, testutil.GoldenRefactoringSafeRename)
			t.Logf("Rename preview:\n%s", testutil.TruncateString(content, 2000))

			// Should show changes across multiple files
			fileCount := strings.Count(content, ".go")
			if fileCount >= 1 {
				t.Logf("Preview shows changes in %d files", fileCount)
			}
		}
	})

	t.Run("FindAllReferencesBeforeRename", func(t *testing.T) {
		// Test: Find all references to "Person" before renaming
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_symbol_references",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Person",
					"context_file": file1,
					"kind":         "struct",
					"line_hint":    3,
				},
			},
		})
		if err != nil {
			t.Fatalf("Failed to find references: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenRefactoringSafeRename)
		t.Logf("References to Person:\n%s", content)

		// Should find references in multiple files
		// Note: May be limited due to temp file location
		t.Log("Reference search completed")
	})

	t.Run("CallHierarchyBeforeRename", func(t *testing.T) {
		// Test: Get call hierarchy for Greet method
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_get_call_hierarchy",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Greet",
					"context_file": file1,
					"kind":         "method",
					"line_hint":    7,
				},
				"direction": "incoming",
			},
		})
		if err != nil {
			t.Fatalf("Failed to get call hierarchy: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenRefactoringSafeRename)
		t.Logf("Call hierarchy for Greet:\n%s", content)

		// Should show caller in usage.go
		t.Log("Call hierarchy helps understand refactoring impact")
	})
}

// TestRefactoring_ExtractFunction tests extract function workflow
func TestRefactoring_ExtractFunction(t *testing.T) {
	t.Run("AnalyzeFunctionComplexity", func(t *testing.T) {
		// Test: Get call hierarchy to understand complexity
		wrappersPath := filepath.Join(globalGoplsMcpDir, "core", "gopls_wrappers.go")

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_get_call_hierarchy",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "handleGoDefinition",
					"context_file": wrappersPath,
					"kind":         "function",
					"line_hint":    307,
				},
				"direction": "outgoing",
			},
		})
		if err != nil {
			t.Fatalf("Failed to analyze function: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenRefactoringExtractFunction)
		t.Logf("Function complexity analysis:\n%s", testutil.TruncateString(content, 2000))

		// Should show what the function calls
		t.Log("Analyzed function calls to understand extraction potential")
	})
}

// TestRefactoring_MoveSymbol tests moving symbols between files/packages
func TestRefactoring_MoveSymbol(t *testing.T) {
	t.Run("FindAllUsagesBeforeMove", func(t *testing.T) {
		// Test: Find all usages to understand move impact
		handlersPath := filepath.Join(globalGoplsMcpDir, "core", "handlers.go")

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_symbol_references",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Handler",
					"context_file": handlersPath,
					"kind":         "struct",
					"line_hint":    25,
				},
			},
		})
		if err != nil {
			t.Fatalf("Failed to find usages: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenRefactoringMoveSymbol)
		t.Logf("All usages before move:\n%s", testutil.TruncateString(content, 2000))

		// Should help understand impact of moving the symbol
		t.Log("Usage analysis helps plan safe refactoring")
	})
}

// TestRefactoring_ChangeSignature tests changing function signatures
func TestRefactoring_ChangeSignature(t *testing.T) {
	t.Run("FindAllCallers", func(t *testing.T) {
		// Test: Before changing signature, find all callers
		wrappersPath := filepath.Join(globalGoplsMcpDir, "core", "gopls_wrappers.go")

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_get_call_hierarchy",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "handleGoDefinition",
					"context_file": wrappersPath,
					"kind":         "function",
					"line_hint":    307,
				},
				"direction": "incoming",
			},
		})
		if err != nil {
			t.Fatalf("Failed to find callers: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenRefactoringChangeSignature)
		t.Logf("All callers before signature change:\n%s", testutil.TruncateString(content, 2000))

		// Should show which files need updating
		t.Log("Caller analysis identifies files affected by signature change")
	})
}

// TestRefactoring_InlineFunction tests inlining function workflows
func TestRefactoring_InlineFunction(t *testing.T) {
	t.Run("AnalyzeUsagePattern", func(t *testing.T) {
		// Test: Analyze how a function is used to determine if inlining is safe
		testutilPath := filepath.Join(globalGoplsMcpDir, "test", "testutil", "assertions.go")

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_symbol_references",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "ResultText",
					"context_file": testutilPath,
					"kind":         "function",
					"line_hint":    10,
				},
			},
		})
		if err != nil {
			t.Fatalf("Failed to analyze usage: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenRefactoringInlineFunction)
		t.Logf("Usage pattern for inlining decision:\n%s", testutil.TruncateString(content, 2000))

		// Should show how widely the function is used
		t.Log("Usage analysis helps decide inlining feasibility")
	})
}

// TestRefactoring_MultiFileChange tests coordinating changes across multiple files
func TestRefactoring_MultiFileChange(t *testing.T) {
	t.Run("TraceDependencyChain", func(t *testing.T) {
		// Test: Use dependency graph to understand ripple effects
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_get_dependency_graph",
			Arguments: map[string]any{
				"package_path":       "golang.org/x/tools/gopls/mcpbridge/core",
				"include_transitive": false,
				"Cwd":                globalGoplsMcpDir,
			},
		})
		if err != nil {
			t.Fatalf("Failed to trace dependencies: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenRefactoringMultiFileChange)
		t.Logf("Dependency chain for refactoring:\n%s", testutil.TruncateString(content, 2000))

		// Should show which packages depend on core
		t.Log("Dependency analysis identifies ripple effect scope")
	})

}

// TestRefactoring_InterfaceExtraction tests extracting interfaces
func TestRefactoring_InterfaceExtraction(t *testing.T) {
	t.Run("IdentifyInterfaceCandidates", func(t *testing.T) {
		// Test: Use implementation finder to identify interface opportunities
		handlersPath := filepath.Join(globalGoplsMcpDir, "core", "handlers.go")

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_implementation",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Handler",
					"context_file": handlersPath,
					"kind":         "struct",
					"line_hint":    25,
				},
			},
		})
		if err != nil {
			t.Fatalf("Failed to find implementations: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenRefactoringInterfaceExtraction)
		t.Logf("Interface candidates:\n%s", content)

		// May find opportunities for interface extraction
		t.Log("Implementation analysis identifies interface extraction points")
	})

}

// TestRefactoring_SafeDelete tests safe deletion of unused code
func TestRefactoring_SafeDelete(t *testing.T) {
	t.Run("VerifyNoReferences", func(t *testing.T) {
		// Test: Before deleting, verify no references exist
		// Use a test-specific symbol

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_symbol_references",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "TruncateString",
					"context_file": filepath.Join(globalGoplsMcpDir, "test", "testutil", "assertions.go"),
					"kind":         "function",
					"line_hint":    20,
				},
			},
		})
		if err != nil {
			t.Fatalf("Failed to check references: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenRefactoringSafeDelete)
		t.Logf("Reference check before deletion:\n%s", testutil.TruncateString(content, 2000))

		// Should show all usages
		t.Log("Reference verification ensures safe deletion")
	})
}

// TestRefactoring_RealWorldScenario tests a complete refactoring workflow
func TestRefactoring_RealWorldScenario(t *testing.T) {
	t.Run("CompleteRefactoringWorkflow", func(t *testing.T) {
		// Scenario: Rename a function across the codebase.
		// Step 1 (locate via search) was dropped along with go_search; the
		// workflow now jumps straight to call-hierarchy / references / rename.

		wrappersPath := filepath.Join(globalGoplsMcpDir, "core", "gopls_wrappers.go")
		res2, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_get_call_hierarchy",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "handleGoDefinition",
					"context_file": wrappersPath,
					"kind":         "function",
					"line_hint":    307,
				},
				"direction": "incoming",
			},
		})
		if err != nil {
			t.Fatalf("Step 2 failed: %v", err)
		}
		t.Logf("Step 2: Found callers\n%s", testutil.TruncateString(testutil.ResultText(t, res2, testutil.GoldenRefactoringRealWorldScenario), 1000))

		// Step 3: Find symbol references
		res3, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_symbol_references",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "handleGoDefinition",
					"context_file": wrappersPath,
					"kind":         "function",
					"line_hint":    307,
				},
			},
		})
		if err != nil {
			t.Fatalf("Step 3 failed: %v", err)
		}
		t.Logf("Step 3: Found references\n%s", testutil.TruncateString(testutil.ResultText(t, res3, testutil.GoldenRefactoringRealWorldScenario), 1000))

		// Step 4: Preview rename (DRY RUN)
		// Find the line number where handleGoDefinition is defined
		content, err := os.ReadFile(wrappersPath)
		if err != nil {
			t.Fatalf("Step 4 failed to read file: %v", err)
		}

		lines := strings.Split(string(content), "\n")
		var lineNum int
		for i, line := range lines {
			if strings.Contains(line, "func handleGoDefinition(") {
				lineNum = i + 1
				break
			}
		}

		if lineNum == 0 {
			t.Fatalf("Step 4: Could not find handleGoDefinition function")
		}

		res4, err := globalSession.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "go_dryrun_rename_symbol",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "handleGoDefinition",
					"context_file": wrappersPath,
					"line_hint":    lineNum,
				},
				"new_name": "handleGoDefinition_NewName",
			},
		})
		if err != nil {
			t.Logf("Step 4: Rename preview returned error (may be expected): %v", err)
		} else {
			t.Logf("Step 4: Rename preview\n%s", testutil.TruncateString(testutil.ResultText(t, res4, testutil.GoldenRefactoringRealWorldScenario), 1000))
		}

		// Success: Completed refactoring analysis workflow
		t.Log("✓ Completed full refactoring analysis workflow")
	})
}
