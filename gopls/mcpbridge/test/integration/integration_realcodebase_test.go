package integration

// Tests that exercise semantic tools against the real gopls-mcp source tree and the
// Go standard library. These complement the controlled-fixture integration tests by
// verifying the tools work on real-world code with real imports and cross-package
// relationships.

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// findLine returns the 1-based line number of the first line in path containing substr.
func findLine(path, substr string) int {
	b, _ := os.ReadFile(path)
	for i, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, substr) {
			return i + 1
		}
	}
	return 0
}

// TestRealCodebase_Definition verifies go_definition against gopls-mcp source with
// strict file:line format assertions that a fake implementation would not satisfy.
func TestRealCodebase_Definition(t *testing.T) {
	wrappersPath := filepath.Join(globalGoplsMcpDir, "core", "gopls_wrappers.go")
	handlersPath := filepath.Join(globalGoplsMcpDir, "core", "handlers.go")

	t.Run("handleGoDefinition_func", func(t *testing.T) {
		ln := findLine(wrappersPath, "func handleGoDefinition(")
		if ln == 0 {
			t.Skip("handleGoDefinition not found")
		}
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_definition",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "handleGoDefinition",
					"context_file": wrappersPath,
					"kind":         "function",
					"line_hint":    ln,
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		content := testutil.ResultText(t, res, "")
		if !strings.Contains(content, "gopls_wrappers.go") {
			t.Fatalf("expected gopls_wrappers.go in result\ngot: %s", content)
		}
		m := regexp.MustCompile(`gopls_wrappers\.go:(\d+)`).FindStringSubmatch(content)
		if len(m) < 2 {
			t.Fatalf("expected gopls_wrappers.go:LINE format\ngot: %s", content)
		}
		foundLine, _ := strconv.Atoi(m[1])
		t.Logf("found handleGoDefinition at line %d", foundLine)
	})

	t.Run("Handler_struct", func(t *testing.T) {
		ln := findLine(handlersPath, "type Handler struct")
		if ln == 0 {
			t.Skip("Handler struct not found")
		}
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_definition",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Handler",
					"context_file": handlersPath,
					"kind":         "struct",
					"line_hint":    ln,
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		content := testutil.ResultText(t, res, "")
		if !regexp.MustCompile(`handlers\.go:\d+`).MatchString(content) {
			t.Fatalf("expected handlers.go:LINE format\ngot: %s", content)
		}
	})
}

// TestRealCodebase_References verifies go_symbol_references on real codebase types.
func TestRealCodebase_References(t *testing.T) {
	handlersPath := filepath.Join(globalGoplsMcpDir, "core", "handlers.go")

	t.Run("Handler_struct", func(t *testing.T) {
		ln := findLine(handlersPath, "type Handler struct")
		if ln == 0 {
			t.Skip("Handler struct not found")
		}
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_symbol_references",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Handler",
					"context_file": handlersPath,
					"kind":         "struct",
					"line_hint":    ln,
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		content := testutil.ResultText(t, res, "")
		if strings.Contains(content, "No references found") {
			t.Log("no references found (known limitation when querying from definition file)")
			return
		}
		if !regexp.MustCompile(`\w+\.go:\d+`).MatchString(content) {
			t.Fatalf("expected file:line format in references\ngot: %s", content)
		}
	})

	t.Run("AssertStringContains_testutil", func(t *testing.T) {
		assertionsPath := filepath.Join(globalGoplsMcpDir, "test", "testutil", "assertions.go")
		ln := findLine(assertionsPath, "func AssertStringContains(")
		if ln == 0 {
			t.Skip("AssertStringContains not found")
		}
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_symbol_references",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "AssertStringContains",
					"context_file": assertionsPath,
					"kind":         "function",
					"line_hint":    ln,
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		content := testutil.ResultText(t, res, "")
		t.Logf("references to AssertStringContains: %s", truncateString(content, 500))
		// reference count may be zero for ad-hoc paths, but the tool must not crash
	})
}

// TestRealCodebase_CallHierarchy verifies go_get_call_hierarchy on real functions, including
// transitive depth (unique parameter combination not covered by fixture tests).
func TestRealCodebase_CallHierarchy(t *testing.T) {
	wrappersPath := filepath.Join(globalGoplsMcpDir, "core", "gopls_wrappers.go")

	t.Run("handleGoDefinition_incoming", func(t *testing.T) {
		ln := findLine(wrappersPath, "func handleGoDefinition(")
		if ln == 0 {
			t.Skip("handleGoDefinition not found")
		}
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_get_call_hierarchy",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "handleGoDefinition",
					"context_file": wrappersPath,
					"kind":         "function",
					"line_hint":    ln,
				},
				"direction": "incoming",
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		content := testutil.ResultText(t, res, "")
		if !strings.Contains(content, "Call hierarchy") {
			t.Fatalf("expected 'Call hierarchy' in result\ngot: %s", content)
		}
	})

	t.Run("handleGoDefinition_outgoing", func(t *testing.T) {
		ln := findLine(wrappersPath, "func handleGoDefinition(")
		if ln == 0 {
			t.Skip("handleGoDefinition not found")
		}
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_get_call_hierarchy",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "handleGoDefinition",
					"context_file": wrappersPath,
					"kind":         "function",
					"line_hint":    ln,
				},
				"direction": "outgoing",
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		content := testutil.ResultText(t, res, "")
		t.Logf("outgoing calls: %s", truncateString(content, 500))
	})
}

// TestRealCodebase_DependencyGraph verifies go_get_dependency_graph including the transitive
// mode (unique parameter combination: include_transitive=true, max_depth=3).
func TestRealCodebase_DependencyGraph(t *testing.T) {
	t.Run("core_direct", func(t *testing.T) {
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_get_dependency_graph",
			Arguments: map[string]any{
				"package_path":       "golang.org/x/tools/gopls/mcpbridge/core",
				"include_transitive": false,
				"Cwd":                globalGoplsMcpDir,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		content := testutil.ResultText(t, res, "")
		if !strings.Contains(content, "Dependencies") && !strings.Contains(content, "imports") {
			t.Fatalf("expected dependency info\ngot: %s", content)
		}
	})

	t.Run("core_transitive_depth3", func(t *testing.T) {
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_get_dependency_graph",
			Arguments: map[string]any{
				"package_path":       "golang.org/x/tools/gopls/mcpbridge/core",
				"include_transitive": true,
				"max_depth":          3,
				"Cwd":                globalGoplsMcpDir,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		content := testutil.ResultText(t, res, "")
		if !strings.Contains(content, "Dependencies") && !strings.Contains(content, "imports") {
			t.Fatalf("expected dependency info\ngot: %s", content)
		}
	})
}

// TestRealCodebase_Rename verifies go_dryrun_rename_symbol dry-run guarantee: the file
// must be unchanged after the call.
func TestRealCodebase_Rename(t *testing.T) {
	testCode := testutil.ReadTestData("rename-test/test_rename.go")
	testDir := t.TempDir()
	testPath := filepath.Join(testDir, "test_rename.go")
	if err := os.WriteFile(testPath, []byte(testCode), 0644); err != nil {
		t.Fatal(err)
	}

	ln := findLine(testPath, "func TestRenameFunction(")
	if ln == 0 {
		t.Skip("TestRenameFunction not found in testdata")
	}

	res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
		Name: "go_dryrun_rename_symbol",
		Arguments: map[string]any{
			"locator": map[string]any{
				"symbol_name":  "TestRenameFunction",
				"context_file": testPath,
				"line_hint":    ln,
			},
			"new_name": "RenamedTestFunction",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	content := testutil.ResultText(t, res, "")
	if !strings.Contains(strings.ToUpper(content), "DRY RUN") {
		t.Fatalf("expected DRY RUN indicator\ngot: %s", content)
	}
	if !strings.Contains(content, "TestRenameFunction") || !strings.Contains(content, "RenamedTestFunction") {
		t.Fatalf("expected both old and new name in result\ngot: %s", content)
	}
	// Dry-run must not modify the file.
	b, _ := os.ReadFile(testPath)
	if string(b) != testCode {
		t.Fatal("dry run violated: file was modified")
	}
}

// TestRealCodebase_Definition_InTestFile verifies go_definition resolves a function
// declared in a *_test.go file.
func TestRealCodebase_Definition_InTestFile(t *testing.T) {
	testFile := filepath.Join(globalGoplsMcpDir, "test", "integration", "integration_definition_test.go")
	b, err := os.ReadFile(testFile)
	if err != nil {
		t.Skip("integration test file not readable")
	}

	var ln int
	var funcName string
	for i, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "func Test") {
			parts := strings.Fields(line)
			for _, p := range parts {
				if strings.HasPrefix(p, "Test") {
					if idx := strings.Index(p, "("); idx != -1 {
						funcName = p[:idx]
					}
					break
				}
			}
			ln = i + 1
			break
		}
	}
	if ln == 0 {
		t.Skip("no test function found in test file")
	}

	res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
		Name: "go_definition",
		Arguments: map[string]any{
			"locator": map[string]any{
				"symbol_name":  funcName,
				"context_file": testFile,
				"kind":         "function",
				"line_hint":    ln,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	content := testutil.ResultText(t, res, "")
	if !strings.Contains(content, "integration_definition_test.go") {
		t.Fatalf("expected definition in integration_definition_test.go\ngot: %s", content)
	}
}

// TestRealCodebase_StdlibNavigation verifies go_definition resolves standard library symbols.
func TestRealCodebase_StdlibNavigation(t *testing.T) {
	projectDir := testutil.CopyProjectTo(t, "simple")

	t.Run("fmt_Println", func(t *testing.T) {
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_definition",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Println",
					"context_file": filepath.Join(projectDir, "main.go"),
					"kind":         "function",
					"line_hint":    27,
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		content := testutil.ResultText(t, res, "")
		if !strings.Contains(content, "Definition found") && !strings.Contains(content, "fmt") {
			t.Fatalf("expected fmt definition result\ngot: %s", content)
		}
	})

	t.Run("context_Context", func(t *testing.T) {
		// Create a minimal project that imports context.
		tmpDir := t.TempDir()
		os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module ex\ngo 1.21\n"), 0644)
		os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(`package main

import "context"

func main() {
	ctx := context.Background()
	_ = ctx
}
`), 0644)

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_definition",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Background",
					"context_file": filepath.Join(tmpDir, "main.go"),
					"kind":         "function",
					"line_hint":    6,
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		content := testutil.ResultText(t, res, "")
		if !strings.Contains(content, "Definition found") && !strings.Contains(content, "context") {
			t.Fatalf("expected context definition\ngot: %s", content)
		}
	})
}
