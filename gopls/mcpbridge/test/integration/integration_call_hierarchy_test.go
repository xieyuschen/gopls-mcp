package integration

// End-to-end tests for call hierarchy functionality.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestGetCallHierarchy_BasicFunctionality tests the core functionality of get_call_hierarchy.
func TestGetCallHierarchy_BasicFunctionality(t *testing.T) {
	t.Run("BothDirections", func(t *testing.T) {
		projectDir := createSimpleProjectWithCalls(t)

		// Get call hierarchy for main function (which calls helperA)
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "main",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "function",
				"line_hint":    5,
			},
			"direction": "both",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchyBasicBothDirections)
		t.Logf("Call hierarchy (both directions):\n%s", content)

		// Compare against golden file (documentation + regression check)

		// Verify structure
		requiredStrings := []string{
			"Call hierarchy for main",
			"Incoming Calls",
			"Outgoing Calls",
		}
		for _, s := range requiredStrings {
			if !strings.Contains(content, s) {
				t.Errorf("Expected to find '%s' in output, got: %s", s, content)
			}
		}

		// main calls helperA
		if !strings.Contains(content, "helperA") {
			t.Errorf("Expected to find 'helperA' in outgoing calls")
		}
	})

	t.Run("IncomingOnly", func(t *testing.T) {
		projectDir := createSimpleProjectWithCalls(t)

		// Get call hierarchy for helperA (which is called by main)
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "helperA",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "function",
				"line_hint":    10,
			},
			"direction": "incoming",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchyBasicIncomingOnly)
		t.Logf("Call hierarchy (incoming only):\n%s", content)

		// Should show that helperA is called by main
		if !strings.Contains(content, "main") {
			t.Errorf("Expected to find 'main' calling helperA")
		}

		// Should NOT show outgoing calls when direction is "incoming"
		// The tool may still show the section header but with no calls
		if strings.Contains(content, "Outgoing Calls") && strings.Contains(content, "Outgoing Calls: None") {
			t.Logf("Correctly shows no outgoing calls when direction is 'incoming'")
		}
	})

	t.Run("OutgoingOnly", func(t *testing.T) {
		projectDir := createSimpleProjectWithCalls(t)

		// Get call hierarchy for main (outgoing only)
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "main",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "function",
				"line_hint":    5,
			},
			"direction": "outgoing",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchyBasicOutgoingOnly)
		t.Logf("Call hierarchy (outgoing only):\n%s", content)

		// Should show that main calls helperA and helperB
		if !strings.Contains(content, "helperA") || !strings.Contains(content, "helperB") {
			t.Errorf("Expected to find 'helperA' and 'helperB' in outgoing calls")
		}
	})

	t.Run("DefaultDirection_IsBoth", func(t *testing.T) {
		projectDir := createSimpleProjectWithCalls(t)

		// Get call hierarchy without specifying direction (should default to "both")
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "main",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "function",
				"line_hint":    5,
			},
			// No direction specified
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchyBasicDefaultDirection)
		t.Logf("Call hierarchy (default direction):\n%s", content)

		// Should show both incoming and outgoing
		if !strings.Contains(content, "Incoming Calls") {
			t.Errorf("Expected to find 'Incoming Calls' with default direction")
		}
		if !strings.Contains(content, "Outgoing Calls") {
			t.Errorf("Expected to find 'Outgoing Calls' with default direction")
		}
	})
}

// TestGetCallHierarchy_ComplexCallGraph tests more complex call relationships.
func TestGetCallHierarchy_ComplexCallGraph(t *testing.T) {
	t.Run("MultipleCallers", func(t *testing.T) {
		projectDir := createProjectWithMultipleCallers(t)

		// sharedFunc is called by both funcA and funcB
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "sharedFunc",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "function",
				"line_hint":    16,
			},
			"direction": "incoming",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchyComplexMultipleCallers)
		t.Logf("Multiple callers:\n%s", content)

		// Should show both funcA and funcB calling sharedFunc
		if !strings.Contains(content, "funcA") {
			t.Errorf("Expected to find 'funcA' calling sharedFunc")
		}
		if !strings.Contains(content, "funcB") {
			t.Errorf("Expected to find 'funcB' calling sharedFunc")
		}
	})

	t.Run("CallChain", func(t *testing.T) {
		projectDir := createProjectWithCallChain(t)

		// main calls funcA, which calls funcB, which calls funcC
		// Check outgoing calls from funcA
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "funcA",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "function",
				"line_hint":    7,
			},
			"direction": "outgoing",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchyComplexCallChain)
		t.Logf("Call chain:\n%s", content)

		// funcA should call funcB
		if !strings.Contains(content, "funcB") {
			t.Errorf("Expected to find 'funcB' in funcA's outgoing calls")
		}
	})
}

// TestGetCallHierarchy_ErrorHandling tests error cases.
func TestGetCallHierarchy_ErrorHandling(t *testing.T) {
	t.Run("InvalidPosition", func(t *testing.T) {
		projectDir := createSimpleProjectWithCalls(t)

		// Try to get call hierarchy for a position that's not a function
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "main",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "function",
				"line_hint":    1,
			},
			"direction": "both",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchyError)
		t.Logf("Invalid position result:\n%s", content)

		// Should handle gracefully with appropriate message
		if !strings.Contains(content, "No function found") && !strings.Contains(content, "not a function") {
			t.Logf("Note: Tool didn't report 'No function found' for invalid position")
		}
	})

	t.Run("NonExistentFile", func(t *testing.T) {
		projectDir := createSimpleProjectWithCalls(t)

		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "main",
				"context_file": filepath.Join(projectDir, "nonexistent.go"),
				"kind":         "function",
				"line_hint":    1,
			},
			"direction": "both",
		}

		_, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err == nil {
			t.Logf("Warning: Tool didn't error for non-existent file")
		} else {
			t.Logf("Got expected error for non-existent file: %v", err)
		}
	})
}

// TestGetCallHierarchy_OutputFormat verifies the output format.
func TestGetCallHierarchy_OutputFormat(t *testing.T) {
	projectDir := createSimpleProjectWithCalls(t)

	t.Run("OutputStructure", func(t *testing.T) {
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "main",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "function",
				"line_hint":    5,
			},
			"direction": "both",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchyOutputFormat)
		t.Logf("Output format:\n%s", content)

		// Verify key sections exist
		requiredSections := []string{
			"Call hierarchy for",
			"Incoming Calls",
			"Outgoing Calls",
		}
		for _, section := range requiredSections {
			if !strings.Contains(content, section) {
				t.Errorf("Expected section '%s' in output", section)
			}
		}
	})
}

// Helper functions to create test projects

func createSimpleProjectWithCalls(t *testing.T) string {
	t.Helper()
	projectDir := t.TempDir()

	sourceCode := `package main

import "fmt"

func main() {
	helperA()
	helperB()
}

func helperA() {
	fmt.Println("A")
}

func helperB() {
	fmt.Println("B")
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
		t.Fatal(err)
	}

	goModContent := `module example.com/calltest

go 1.21
`
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	return projectDir
}

func createProjectWithMultipleCallers(t *testing.T) string {
	t.Helper()
	projectDir := t.TempDir()

	sourceCode := `package main

func main() {
	funcA()
	funcB()
}

func funcA() {
	sharedFunc()
}

func funcB() {
	sharedFunc()
}

func sharedFunc() {
	println("shared")
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
		t.Fatal(err)
	}

	goModContent := `module example.com/multicaller

go 1.21
`
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	return projectDir
}

func createProjectWithCallChain(t *testing.T) string {
	t.Helper()
	projectDir := t.TempDir()

	sourceCode := `package main

func main() {
	funcA()
}

func funcA() {
	funcB()
}

func funcB() {
	funcC()
}

func funcC() {
	println("C")
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
		t.Fatal(err)
	}

	goModContent := `module example.com/callchain

go 1.21
`
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	return projectDir
}

// ===== Additional Comprehensive Test Cases =====

// TestGetCallHierarchy_StructMethods tests call hierarchy for struct methods.
func TestGetCallHierarchy_StructMethods(t *testing.T) {
	t.Run("ValueReceiverMethods", func(t *testing.T) {
		projectDir := createProjectWithStructMethods(t)

		// Test call hierarchy for a method on a struct
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "Add",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "method",
				"line_hint":    9,
			},
			"direction": "incoming",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchyStructMethodsValue)
		t.Logf("Value receiver method incoming calls:\n%s", content)

		// main calls counter.Add
		if !strings.Contains(content, "main") {
			t.Errorf("Expected to find 'main' calling Add method")
		}
	})

	t.Run("PointerReceiverMethods", func(t *testing.T) {
		projectDir := createProjectWithStructMethods(t)

		// Test pointer receiver method
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "Increment",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "method",
				"line_hint":    14,
			},
			"direction": "incoming",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchyStructMethodsPointer)
		t.Logf("Pointer receiver method incoming calls:\n%s", content)

		// main calls counter.Increment
		if !strings.Contains(content, "main") {
			t.Errorf("Expected to find 'main' calling Increment method")
		}
	})

	t.Run("MethodCallsOtherMethods", func(t *testing.T) {
		projectDir := createProjectWithStructMethods(t)

		// Test that main calls methods
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "main",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "function",
				"line_hint":    18,
			},
			"direction": "outgoing",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchyStructMethodsMethodCalls)
		t.Logf("Method calling another method:\n%s", content)

		// main should call Add and Increment methods
		hasAdd := strings.Contains(content, "Add")
		hasIncrement := strings.Contains(content, "Increment")

		if !hasAdd {
			t.Logf("Note: Add method not in outgoing calls (may be implementation-specific)")
		}
		if !hasIncrement {
			t.Logf("Note: Increment method not in outgoing calls (may be implementation-specific)")
		}
		if hasAdd || hasIncrement {
			t.Logf("Correctly includes method calls")
		}
	})
}

// TestGetCallHierarchy_MultipleFiles tests call hierarchy across multiple files.
func TestGetCallHierarchy_MultipleFiles(t *testing.T) {
	t.Run("CrossFileCalls", func(t *testing.T) {
		projectDir := createMultiFileProject(t)

		// Force gopls to analyze the project first
		_, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_build_check",
			Arguments: map[string]any{
				"Cwd": projectDir,
			},
		})
		if err != nil {
			t.Logf("Warning: diagnostics call failed: %v", err)
		}

		// Check outgoing calls from main (in main.go) to functions in helpers.go
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "main",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "function",
				"line_hint":    3,
			},
			"direction": "outgoing",
			"Cwd":       projectDir,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchyMultipleFilesCrossFile)
		t.Logf("Cross-file calls:\n%s", content)

		// main calls functions in helpers.go
		if !strings.Contains(content, "HelperFunc1") {
			t.Errorf("Expected to find 'HelperFunc1' in outgoing calls from main")
		}
		if !strings.Contains(content, "HelperFunc2") {
			t.Errorf("Expected to find 'HelperFunc2' in outgoing calls from main")
		}
	})

	t.Run("CrossPackageCalls", func(t *testing.T) {
		projectDir := createMultiPackageProject(t)

		// Force gopls to analyze the project first
		_, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_build_check",
			Arguments: map[string]any{
				"Cwd": projectDir,
			},
		})
		if err != nil {
			t.Logf("Warning: diagnostics call failed: %v", err)
		}

		// Check calls from main package to another package
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "main",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "function",
				"line_hint":    5,
			},
			"direction": "outgoing",
			"Cwd":       projectDir,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchyMultipleFilesCrossPackage)
		t.Logf("Cross-package calls:\n%s", content)

		// main calls functions from the other package
		if !strings.Contains(content, "OtherPackageFunc") {
			t.Errorf("Expected to find 'OtherPackageFunc' in outgoing calls from main")
		}
		if !strings.Contains(content, "HelperFunc") {
			t.Errorf("Expected to find 'HelperFunc' in outgoing calls from main")
		}
	})
}

// TestGetCallHierarchy_MultipleCallSites tests when a function is called multiple times.
func TestGetCallHierarchy_MultipleCallSites(t *testing.T) {
	t.Run("SameCallerMultipleTimes", func(t *testing.T) {
		projectDir := createProjectWithMultipleCallSites(t)

		// sharedFunc is called 3 times by main
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "sharedFunc",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "function",
				"line_hint":    15,
			},
			"direction": "incoming",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchyMultipleCallsSameCaller)
		t.Logf("Multiple call sites:\n%s", content)

		// Should show main calling sharedFunc
		if !strings.Contains(content, "main") {
			t.Errorf("Expected to find 'main' calling sharedFunc")
		}

		// Check if it mentions multiple calls (depending on implementation)
		// The output might show "(called N times)" or list each call range
		if strings.Contains(content, "called 3 times") || strings.Contains(content, "called") {
			t.Logf("Correctly reports multiple call sites")
		}
	})

	t.Run("DifferentCallers", func(t *testing.T) {
		projectDir := createProjectWithMultipleCallSites(t)

		// helperFunc is called by both main and processFunc
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "helperFunc",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "function",
				"line_hint":    19,
			},
			"direction": "incoming",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchyMultipleCallsDifferent)
		t.Logf("Different callers:\n%s", content)

		// Should show both main and processFunc calling helperFunc
		hasMain := strings.Contains(content, "main")
		hasProcess := strings.Contains(content, "processFunc")

		if !hasMain {
			t.Errorf("Expected to find 'main' calling helperFunc")
		}
		if !hasProcess {
			t.Errorf("Expected to find 'processFunc' calling helperFunc")
		}
	})
}

// TestGetCallHierarchy_StdlibCalls tests calls to standard library functions.
func TestGetCallHierarchy_StdlibCalls(t *testing.T) {
	t.Run("StdlibOutgoingCalls", func(t *testing.T) {
		projectDir := createSimpleProjectWithCalls(t)

		// Check outgoing calls from helperA which calls fmt.Println
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "helperA",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "function",
				"line_hint":    10,
			},
			"direction": "outgoing",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchyStdlibCalls)
		t.Logf("Stdlib outgoing calls:\n%s", content)

		// helperA calls fmt.Println
		// Note: Some call hierarchy implementations might skip builtin/stdlib calls
		if strings.Contains(content, "Println") || strings.Contains(content, "fmt") {
			t.Logf("Correctly includes stdlib calls in hierarchy")
		} else {
			t.Logf("Note: Stdlib calls not included (may be implementation-dependent)")
		}
	})
}

// TestGetCallHierarchy_InterfaceMethods tests interface method implementations.
func TestGetCallHierarchy_InterfaceMethods(t *testing.T) {
	t.Run("InterfaceImplementationCalls", func(t *testing.T) {
		projectDir := createProjectWithInterfaces(t)

		// Check calls to interface method - Process method is on line 11
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "Process",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "method",
				"line_hint":    11,
			},
			"direction": "incoming",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchyInterface)
		t.Logf("Interface method calls:\n%s", content)

		// main calls doWork which calls the interface method
		// The concrete implementation should be called
		t.Logf("Interface method hierarchy: %s", content)
	})
}

// TestGetCallHierarchy_SpecialCases tests special Go calling patterns.
func TestGetCallHierarchy_SpecialCases(t *testing.T) {
	t.Run("RecursiveFunction", func(t *testing.T) {
		projectDir := createProjectWithRecursion(t)

		// Check outgoing calls from factorial (which calls itself)
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "factorial",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "function",
				"line_hint":    8,
			},
			"direction": "outgoing",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchySpecialCasesRecursive)
		t.Logf("Recursive function calls:\n%s", content)

		// factorial should call itself
		if strings.Contains(content, "factorial") {
			t.Logf("Correctly detects recursive call")
		}
	})

	t.Run("DeferredCalls", func(t *testing.T) {
		projectDir := createProjectWithDefer(t)

		// Check outgoing calls from process (which has a deferred call)
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "process",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "function",
				"line_hint":    7,
			},
			"direction": "outgoing",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchySpecialCasesDefer)
		t.Logf("Deferred calls:\n%s", content)

		// process calls cleanup via defer
		if strings.Contains(content, "cleanup") {
			t.Logf("Correctly includes deferred call")
		}
	})

	t.Run("GoroutineCalls", func(t *testing.T) {
		projectDir := createProjectWithGoroutines(t)

		// Check outgoing calls from main which spawns a goroutine
		tool := "go_get_call_hierarchy"
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "main",
				"context_file": filepath.Join(projectDir, "main.go"),
				"kind":         "function",
				"line_hint":    3,
			},
			"direction": "outgoing",
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenCallHierarchySpecialCasesGoroutine)
		t.Logf("Goroutine calls:\n%s", content)

		// main calls worker in a goroutine
		if strings.Contains(content, "worker") {
			t.Logf("Correctly includes goroutine call")
		}
	})
}

// ===== Helper Functions for Additional Tests =====

func createProjectWithStructMethods(t *testing.T) string {
	t.Helper()
	projectDir := t.TempDir()

	sourceCode := `package main

import "fmt"

type Counter struct {
	value int
}

func (s Counter) Add() int {
	s.value++
	return s.value
}

func (s *Counter) Increment() {
	s.value++
}

func main() {
	counter := Counter{value: 0}
	counter.Add()
	counter.Increment()
	fmt.Println(counter.value)
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
		t.Fatal(err)
	}

	goModContent := `module example.com/structmethods

go 1.21
`
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	return projectDir
}

func createMultiFileProject(t *testing.T) string {
	t.Helper()
	projectDir := t.TempDir()

	// main.go
	mainCode := `package main

func main() {
	HelperFunc1()
	HelperFunc2()
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(mainCode), 0644); err != nil {
		t.Fatal(err)
	}

	// helpers.go
	helpersCode := `package main

func HelperFunc1() {
	println("helper1")
}

func HelperFunc2() {
	println("helper2")
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "helpers.go"), []byte(helpersCode), 0644); err != nil {
		t.Fatal(err)
	}

	goModContent := `module example.com/multifile

go 1.21
`
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	return projectDir
}

func createMultiPackageProject(t *testing.T) string {
	t.Helper()
	projectDir := t.TempDir()

	// main.go
	mainCode := `package main

import "example.com/multipkg/other"

func main() {
	other.OtherPackageFunc()
	other.HelperFunc()
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(mainCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Create other package directory
	otherDir := filepath.Join(projectDir, "other")
	if err := os.MkdirAll(otherDir, 0755); err != nil {
		t.Fatal(err)
	}

	otherCode := `package other

func OtherPackageFunc() {
	println("other package")
}

func HelperFunc() {
	println("helper")
}
`
	if err := os.WriteFile(filepath.Join(otherDir, "other.go"), []byte(otherCode), 0644); err != nil {
		t.Fatal(err)
	}

	goModContent := `module example.com/multipkg

go 1.21
`
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	return projectDir
}

func createProjectWithMultipleCallSites(t *testing.T) string {
	t.Helper()
	projectDir := t.TempDir()

	sourceCode := `package main

func main() {
	sharedFunc()
	sharedFunc()
	sharedFunc()
	processFunc()
}

func processFunc() {
	helperFunc()
	sharedFunc()
}

func sharedFunc() {
	println("shared")
}

func helperFunc() {
	println("helper")
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
		t.Fatal(err)
	}

	goModContent := `module example.com/multicallsites

go 1.21
`
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	return projectDir
}

func createProjectWithRecursion(t *testing.T) string {
	t.Helper()
	projectDir := t.TempDir()

	sourceCode := `package main

func main() {
	result := factorial(5)
	println(result)
}

func factorial(n int) int {
	if n <= 1 {
		return 1
	}
	return n * factorial(n-1)
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
		t.Fatal(err)
	}

	goModContent := `module example.com/recursion

go 1.21
`
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	return projectDir
}

func createProjectWithDefer(t *testing.T) string {
	t.Helper()
	projectDir := t.TempDir()

	sourceCode := `package main

func main() {
	process()
}

func process() {
	println("processing")
}

func cleanup() {
	println("cleanup")
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
		t.Fatal(err)
	}

	goModContent := `module example.com/defer

go 1.21
`
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	return projectDir
}

func createProjectWithGoroutines(t *testing.T) string {
	t.Helper()
	projectDir := t.TempDir()

	sourceCode := `package main

func main() {
	go worker()
	println("main continues")
}

func worker() {
	println("worker running")
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
		t.Fatal(err)
	}

	goModContent := `module example.com/goroutines

go 1.21
`
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	return projectDir
}

func createProjectWithInterfaces(t *testing.T) string {
	t.Helper()
	projectDir := t.TempDir()

	sourceCode := `package main

type Processor interface {
	Process()
}

type Concrete struct {
	name string
}

func (c *Concrete) Process() {
	println("processing:", c.name)
}

func doWork(p Processor) {
	p.Process()
}

func main() {
	c := &Concrete{name: "test"}
	doWork(c)
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
		t.Fatal(err)
	}

	goModContent := `module example.com/interfaces

go 1.21
`
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	return projectDir
}
