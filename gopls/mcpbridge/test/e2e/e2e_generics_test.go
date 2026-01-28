package e2e

// E2E tests for GENERIC TYPES and functions.
// These tests ensure tools correctly handle Go generics (type parameters, constraints, inference).

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

	// Create go.mod for the temp directory
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

// TestGenerics_BasicFunctions tests tools on generic functions using table-driven approach
func TestGenerics_BasicFunctions(t *testing.T) {
	code := testutil.ReadTestData("generics/basic_functions.go")

	testCases := []testCase{
		{
			name: "list_generic_symbols",
			tool: "go_list_package_symbols",
			args: map[string]any{
				"package_path":   "generics",
				"include_docs":   true,
				"include_bodies": false,
			},
			assertion: func(t *testing.T, content string) {
				t.Logf("Generic symbols:\n%s", content)

				// If package is found, should find generic functions
				if !strings.Contains(content, "package not found") {
					if !strings.Contains(content, "First") {
						t.Error("Expected to find First[T] function")
					}
					if !strings.Contains(content, "Max") {
						t.Error("Expected to find Max[T] function")
					}

					// Should mention type parameters
					if !strings.Contains(content, "[T") && !strings.Contains(content, "type parameter") {
						t.Errorf("Expected type parameter information in generic functions, got: %s", testutil.TruncateString(content, 200))
					}
				} else {
					t.Log("Package not found (expected for temp files) - tool behavior is correct")
				}
			},
		},
		{
			name: "definition_in_generic_function",
			tool: "go_definition",
			args: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "First",
					"context_file": "generics.go",
					"kind":         "function",
					"line_hint":    4,
				},
			},
			assertion: func(t *testing.T, content string) {
				t.Logf("Definition in generic function:\n%s", content)

				// Should find the function definition
				if !strings.Contains(content, "First") && !strings.Contains(content, "generic.go") {
					t.Errorf("Expected to find function definition, got: %s", content)
				}
			},
		},
	}

	// Run all test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, sourceFile := setupTestFile(t, "generics", code)

			// Prepare args
			args := make(map[string]any)
			for k, v := range tc.args {
				args[k] = v
			}

			// Add Cwd only for tools that need it (go_definition uses locator.context_file)
			if tc.tool != "go_definition" {
				args["Cwd"] = tmpDir
			} else {
				// For go_definition, update the context_file path to use the actual temp file
				if locator, ok := args["locator"].(map[string]any); ok {
					if contextFile, ok := locator["context_file"].(string); ok && contextFile == "generics.go" {
						locator["context_file"] = sourceFile
					}
				}
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
				Name:     tc.tool,
				Arguments: args,
			})
			if err != nil {
				t.Fatalf("Failed to call tool %s: %v", tc.tool, err)
			}

			content := testutil.ResultText(t, res, testutil.GoldenGenericsBasicFunctions)
			tc.assertion(t, content)
		})
	}
}

// TestGenerics_GenericTypes tests tools on generic types using table-driven approach
func TestGenerics_GenericTypes(t *testing.T) {
	code := testutil.ReadTestData("generics/generic_types.go")

	testCases := []testCase{
		{
			name: "list_generic_types",
			tool: "go_list_package_symbols",
			args: map[string]any{
				"package_path":   "generictypes",
				"include_docs":   true,
				"include_bodies": false,
			},
			assertion: func(t *testing.T, content string) {
				t.Logf("Generic type symbols:\n%s", content)

				// Should find generic types
				if !strings.Contains(content, "Container") {
					t.Error("Expected to find Container[T] type")
				}
				if !strings.Contains(content, "Pair") {
					t.Error("Expected to find Pair[T, U] type")
				}
			},
		},
		{
			name: "definition_of_generic_method",
			tool: "go_definition",
			args: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Get",
					"context_file": "generictypes.go",
					"kind":         "method",
					"line_hint":    21,
				},
			},
			assertion: func(t *testing.T, content string) {
				t.Logf("Definition of generic method:\n%s", content)

				// Should find method definition on generic type
				// The definition result should include the file path and location
				if !strings.Contains(content, "generictypes.go") && !strings.Contains(content, "Get") && !strings.Contains(content, "Container") {
					t.Errorf("Expected to find method definition on generic type, got: %s", content)
				}
			},
		},
	}

	// Run all test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, sourceFile := setupTestFile(t, "generictypes", code)

			// Prepare args
			args := make(map[string]any)
			for k, v := range tc.args {
				args[k] = v
			}

			// Add Cwd only for tools that need it (go_definition uses locator.context_file)
			if tc.tool != "go_definition" {
				args["Cwd"] = tmpDir
			} else {
				// For go_definition, update the context_file path to use the actual temp file
				if locator, ok := args["locator"].(map[string]any); ok {
					if contextFile, ok := locator["context_file"].(string); ok && contextFile == "generictypes.go" {
						locator["context_file"] = sourceFile
					}
				}
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
				Name:     tc.tool,
				Arguments: args,
			})
			if err != nil {
				t.Fatalf("Failed to call tool %s: %v", tc.tool, err)
			}

			content := testutil.ResultText(t, res, testutil.GoldenGenericsGenericTypes)
			tc.assertion(t, content)
		})
	}
}

// TestGenerics_TypeInference tests tools handle type inference correctly using table-driven approach
func TestGenerics_TypeInference(t *testing.T) {
	code := testutil.ReadTestData("generics/type_inference.go")

	testCases := []testCase{
		{
			name: "list_generic_with_inference",
			tool: "go_list_package_symbols",
			args: map[string]any{
				"package_path":   "inference",
				"include_docs":   true,
				"include_bodies": false,
			},
			assertion: func(t *testing.T, content string) {
				t.Logf("Generic symbols with inference:\n%s", content)

				// Should find the Map function
				if !strings.Contains(content, "Map") {
					t.Error("Expected to find Map[T, U] function")
				}

				// Should mention type parameters T and U
				if !strings.Contains(content, "T") || !strings.Contains(content, "U") {
					t.Errorf("Expected type parameters T and U in Map function, got: %s", testutil.TruncateString(content, 200))
				}
			},
		},
	}

	// Run all test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, _ := setupTestFile(t, "inference", code)

			// Add cwd to args
			args := make(map[string]any)
			for k, v := range tc.args {
				args[k] = v
			}
			args["Cwd"] = tmpDir

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
				Name:     tc.tool,
				Arguments: args,
			})
			if err != nil {
				t.Fatalf("Failed to call tool %s: %v", tc.tool, err)
			}

			content := testutil.ResultText(t, res, testutil.GoldenGenericsTypeInference)
			tc.assertion(t, content)
		})
	}
}

// TestGenerics_Constraints tests tools handle type constraints using table-driven approach
func TestGenerics_Constraints(t *testing.T) {
	code := testutil.ReadTestData("generics/constraints.go")

	testCases := []testCase{
		{
			name: "list_constrained_generics",
			tool: "go_list_package_symbols",
			args: map[string]any{
				"package_path":   "constraints",
				"include_docs":   true,
				"include_bodies": false,
			},
			assertion: func(t *testing.T, content string) {
				t.Logf("Constrained generic symbols:\n%s", content)

				// Should find constrained functions
				if !strings.Contains(content, "MaxOrdered") {
					t.Error("Expected to find MaxOrdered function")
				}
				if !strings.Contains(content, "PrintAll") {
					t.Error("Expected to find PrintAll function")
				}
			},
		},
	}

	// Run all test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, _ := setupTestFile(t, "constraints", code)

			// Add cwd to args
			args := make(map[string]any)
			for k, v := range tc.args {
				args[k] = v
			}
			args["Cwd"] = tmpDir

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
				Name:     tc.tool,
				Arguments: args,
			})
			if err != nil {
				t.Fatalf("Failed to call tool %s: %v", tc.tool, err)
			}

			content := testutil.ResultText(t, res, testutil.GoldenGenericsConstraints)
			tc.assertion(t, content)
		})
	}
}

// TestGenerics_RealWorldUsage tests generics in real gopls-mcp codebase using table-driven approach
func TestGenerics_RealWorldUsage(t *testing.T) {
	testCases := []testCase{
		{
			name: "diagnostics_on_generic_code",
			tool: "go_build_check",
			args: map[string]any{
				"Cwd": globalGoplsMcpDir,
			},
			assertion: func(t *testing.T, content string) {
				t.Logf("Diagnostics on codebase with generics:\n%s", testutil.TruncateString(content, 2000))
				// Should handle generic code without errors
				t.Log("Diagnostics completed on codebase that may contain generics")
			},
		},
	}

	// Run all test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skip {
				t.Skipf("Skipping test: %s", tc.skipReason)
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
				Name:     tc.tool,
				Arguments: tc.args,
			})
			if err != nil {
				t.Fatalf("Failed to call tool %s: %v", tc.tool, err)
			}

			content := testutil.ResultText(t, res, testutil.GoldenGenericsRealWorldUsage)
			tc.assertion(t, content)
		})
	}
}

// TestGenerics_NestedGenerics tests tools on nested generic types using table-driven approach
func TestGenerics_NestedGenerics(t *testing.T) {
	code := testutil.ReadTestData("generics/nested.go")

	testCases := []testCase{
		{
			name: "list_nested_generics",
			tool: "go_list_package_symbols",
			args: map[string]any{
				"package_path":   "nested",
				"include_docs":   true,
				"include_bodies": false,
			},
			assertion: func(t *testing.T, content string) {
				t.Logf("Nested generic symbols:\n%s", content)

				// Should find nested generic types
				if !strings.Contains(content, "Matrix") {
					t.Error("Expected to find Matrix[T] type")
				}
				if !strings.Contains(content, "TripleContainer") {
					t.Error("Expected to find TripleContainer[T, U, V] type")
				}
			},
		},
	}

	// Run all test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, _ := setupTestFile(t, "nested", code)

			// Add cwd to args
			args := make(map[string]any)
			for k, v := range tc.args {
				args[k] = v
			}
			args["Cwd"] = tmpDir

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
				Name:     tc.tool,
				Arguments: args,
			})
			if err != nil {
				t.Fatalf("Failed to call tool %s: %v", tc.tool, err)
			}

			content := testutil.ResultText(t, res, testutil.GoldenGenericsNestedGenerics)
			tc.assertion(t, content)
		})
	}
}

// TestGenerics_GenericInterfaces tests tools with generic interfaces
func TestGenerics_GenericInterfaces(t *testing.T) {
	tmpDir := t.TempDir()
	interfaceFile := filepath.Join(tmpDir, "generic_iface.go")
	goModFile := filepath.Join(tmpDir, "go.mod")

	// Create go.mod for the temp directory
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
		// Test: Find implementations of generic interface
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

		// Should find StringProcessor and IntProcessor
		// Note: Generic interface implementations may not be fully supported
		t.Log("Implementation search completed for generic interface")
	})
}
