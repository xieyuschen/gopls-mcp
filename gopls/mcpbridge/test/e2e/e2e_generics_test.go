package e2e

// E2E tests for GENERIC TYPES and functions.
// These tests ensure semantic tools correctly handle Go generics
// (type parameters, constraints, inference). Subtests that exercised
// go_list_package_symbols and go_build_check were dropped along with
// those tools; the remaining cases cover go_definition and go_implementation.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// setupTestFile creates a temporary directory with go.mod and source file
func setupTestFile(t *testing.T, moduleName, sourceCode string) (string, string) {
	tmpDir := t.TempDir()
	goModFile := filepath.Join(tmpDir, "go.mod")
	sourceFile := filepath.Join(tmpDir, moduleName+".go")

	goMod := `module ` + moduleName + `

go 1.21
`
	if err := os.WriteFile(goModFile, []byte(goMod), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	if err := os.WriteFile(sourceFile, []byte(sourceCode), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	return tmpDir, sourceFile
}

// TestGenerics_BasicFunctions verifies go_definition on generic functions.
func TestGenerics_BasicFunctions(t *testing.T) {
	code := testutil.ReadTestData("generics/basic_functions.go")
	tmpDir, sourceFile := setupTestFile(t, "generics", code)
	_ = tmpDir

	t.Run("definition_in_generic_function", func(t *testing.T) {
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_definition",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "First",
					"context_file": sourceFile,
					"kind":         "function",
					"line_hint":    4,
				},
			},
		})
		if err != nil {
			t.Fatalf("Failed to call go_definition: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenGenericsBasicFunctions)
		t.Logf("Definition in generic function:\n%s", content)

		if !strings.Contains(content, "First") && !strings.Contains(content, "generic.go") {
			t.Errorf("Expected to find generic function definition, got: %s", content)
		}
	})
}

// TestGenerics_GenericTypes verifies go_definition on methods of generic types.
func TestGenerics_GenericTypes(t *testing.T) {
	code := testutil.ReadTestData("generics/generic_types.go")
	tmpDir, sourceFile := setupTestFile(t, "generictypes", code)
	_ = tmpDir

	t.Run("definition_of_generic_method", func(t *testing.T) {
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_definition",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Get",
					"context_file": sourceFile,
					"kind":         "method",
					"line_hint":    21,
				},
			},
		})
		if err != nil {
			t.Fatalf("Failed to call go_definition: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenGenericsGenericTypes)
		t.Logf("Definition of generic method:\n%s", content)

		if !strings.Contains(content, "generictypes.go") && !strings.Contains(content, "Get") && !strings.Contains(content, "Container") {
			t.Errorf("Expected to find method definition on generic type, got: %s", content)
		}
	})
}

// TestGenerics_GenericInterfaces verifies go_implementation on generic interfaces.
func TestGenerics_GenericInterfaces(t *testing.T) {
	tmpDir := t.TempDir()
	interfaceFile := filepath.Join(tmpDir, "generic_iface.go")
	goModFile := filepath.Join(tmpDir, "go.mod")

	goMod := `module genericiface

go 1.21
`
	if err := os.WriteFile(goModFile, []byte(goMod), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	code := testutil.ReadTestData("generics/interfaces.go")
	if err := os.WriteFile(interfaceFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to write interface file: %v", err)
	}

	t.Run("ImplementationsOfGenericInterface", func(t *testing.T) {
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_implementation",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Processor",
					"context_file": interfaceFile,
					"kind":         "interface",
					"line_hint":    4,
				},
			},
		})
		if err != nil {
			t.Fatalf("Failed to find implementations: %v", err)
		}

		content := testutil.ResultText(t, res, testutil.GoldenGenericsGenericInterfaces)
		t.Logf("Implementations of generic interface:\n%s", content)

		// Generic interface implementations may not be fully supported; the
		// tool just needs to complete without crashing.
		t.Log("Implementation search completed for generic interface")
	})
}
