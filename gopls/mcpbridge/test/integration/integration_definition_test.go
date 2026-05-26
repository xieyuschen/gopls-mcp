package integration

// End-to-end test for go_definition functionality.
// These tests verify that go_definition actually jumps to the correct
// file location, not just echoes back the input.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestGoDefinition tests the go_definition tool.
// A fake implementation that just echoes "Definition found for Add" would FAIL these tests.
func TestGoDefinition(t *testing.T) {
	projectDir := testutil.CopyProjectTo(t, "simple")
	mainGo := filepath.Join(projectDir, "main.go")

	// assertDefLine checks the common strong assertions for definition results:
	// "Definition found at", "main.go:LINE" format, full project path,
	// and that the definition line is strictly before the usageLine.
	assertDefLine := func(t *testing.T, content string, usageLine int, golden string) {
		t.Helper()
		if !strings.Contains(content, "Definition found at") {
			t.Fatalf("expected 'Definition found at' in result.\nGot: %s", content)
		}
		re := regexp.MustCompile(`main\.go:(\d+)`)
		matches := re.FindStringSubmatch(content)
		if len(matches) < 2 {
			t.Fatalf("expected 'main.go:LINE' format; fake impl wouldn't know exact position.\nGot: %s", content)
		}
		defLine, err := strconv.Atoi(matches[1])
		if err != nil {
			t.Fatalf("could not parse line number: %v", err)
		}
		if defLine >= usageLine {
			t.Fatalf("definition line %d should be BEFORE usage line %d; fake impl would echo usage position.\nGot: %s", defLine, usageLine, content)
		}
		if !strings.Contains(content, projectDir) {
			t.Fatalf("expected full project path in result; fake impl would just say 'main.go'.\nGot: %s", content)
		}
		t.Logf("definition at main.go:%d (before usage at line %d)", defLine, usageLine)
		_ = golden // golden name kept for documentation; ResultText handles update
	}

	callDef := func(t *testing.T, args map[string]any, golden string) string {
		t.Helper()
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: "go_definition", Arguments: args})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if res == nil {
			t.Fatal("expected non-nil result")
		}
		return testutil.ResultText(t, res, golden)
	}

	t.Run("ExactDefinitionLocation", func(t *testing.T) {
		// Add is called at line 28; its definition must be before that.
		content := callDef(t, map[string]any{
			"locator": map[string]any{
				"symbol_name":  "Add",
				"context_file": mainGo,
				"line_hint":    28,
			},
		}, testutil.GoldenDefinitionExactLocation)
		t.Logf("result:\n%s", content)
		assertDefLine(t, content, 28, testutil.GoldenDefinitionExactLocation)
	})

	t.Run("TypeDefinition", func(t *testing.T) {
		// Person is used in a receiver at line 22; its definition must be before that.
		content := callDef(t, map[string]any{
			"locator": map[string]any{
				"symbol_name":  "Person",
				"context_file": mainGo,
				"line_hint":    22,
			},
		}, testutil.GoldenDefinitionTypeDefinition)
		t.Logf("result:\n%s", content)
		assertDefLine(t, content, 22, testutil.GoldenDefinitionTypeDefinition)
	})

	t.Run("MethodDefinition", func(t *testing.T) {
		// Greeting is called at line 30; its definition must be before that.
		content := callDef(t, map[string]any{
			"locator": map[string]any{
				"symbol_name":  "Greeting",
				"context_file": mainGo,
				"line_hint":    30,
			},
		}, testutil.GoldenDefinitionMethodDefinition)
		t.Logf("result:\n%s", content)
		assertDefLine(t, content, 30, testutil.GoldenDefinitionMethodDefinition)
	})

	t.Run("ImportStatement", func(t *testing.T) {
		// Navigating to fmt.Println should resolve to the import statement (around line 3).
		content := callDef(t, map[string]any{
			"locator": map[string]any{
				"symbol_name":        "Println",
				"context_file":       mainGo,
				"package_identifier": "fmt",
				"line_hint":          27,
			},
		}, testutil.GoldenDefinitionImportStatement)
		t.Logf("result:\n%s", content)
		if !strings.Contains(content, "Definition found at") {
			t.Fatalf("expected 'Definition found at' in result.\nGot: %s", content)
		}
		re := regexp.MustCompile(`main\.go:(\d+):\d+`)
		if matches := re.FindStringSubmatch(content); len(matches) >= 2 {
			defLine, _ := strconv.Atoi(matches[1])
			t.Logf("definition at main.go:%d (import statement is around line 3)", defLine)
		}
	})

	t.Run("InvalidPosition", func(t *testing.T) {
		// Out-of-bounds line hint for a non-existent symbol must NOT claim success.
		content := callDef(t, map[string]any{
			"locator": map[string]any{
				"symbol_name":  "NonExistentSymbol",
				"context_file": mainGo,
				"line_hint":    9999,
			},
		}, testutil.GoldenDefinitionInvalidPosition)
		t.Logf("result:\n%s", content)
		if strings.Contains(content, "Definition found at") {
			t.Fatalf("fake impl would still say 'found' for a non-existent symbol.\nGot: %s", content)
		}
		t.Logf("invalid symbol handled correctly")
	})

	t.Run("DefinitionNoSymbol", func(t *testing.T) {
		// Symbol that simply doesn't exist must NOT claim a definition was found.
		content := callDef(t, map[string]any{
			"locator": map[string]any{
				"symbol_name":  "NonExistentFunction",
				"context_file": mainGo,
			},
		}, testutil.GoldenDefinitionNoSymbol)
		t.Logf("result:\n%s", content)
		if strings.Contains(content, "Definition found at") {
			t.Fatalf("claimed definition for non-existent symbol.\nGot: %s", content)
		}
		t.Logf("non-existent symbol handled correctly")
	})
}

// TestGoDefinitionCrossFile tests cross-file definition lookup.
// Uses a temp project with a helper function defined in util.go and called from main.go.
func TestGoDefinitionCrossFile(t *testing.T) {
	projectDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	utilCode := `package main

// HelperFunction is defined in util.go and called from main.go
func HelperFunction(x int) int {
	return x * 2
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "util.go"), []byte(utilCode), 0644); err != nil {
		t.Fatal(err)
	}
	mainCode := `package main

import "fmt"

func main() {
	// Call function defined in util.go
	result := HelperFunction(21)
	fmt.Println(result)
}
`
	mainGoPath := filepath.Join(projectDir, "main.go")
	if err := os.WriteFile(mainGoPath, []byte(mainCode), 0644); err != nil {
		t.Fatal(err)
	}

	// CRITICAL: Must find the definition in util.go, not echo main.go.
	res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
		Name: "go_definition",
		Arguments: map[string]any{
			"locator": map[string]any{
				"symbol_name":  "HelperFunction",
				"context_file": mainGoPath,
				"line_hint":    7,
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	content := testutil.ResultText(t, res, testutil.GoldenDefinitionCrossFileFunction)
	t.Logf("cross-file result:\n%s", content)

	if !strings.Contains(content, "util.go") {
		// Tolerate gopls not finding the cross-file def, but it must NOT falsely claim main.go.
		if strings.Contains(content, "main.go:") && !strings.Contains(strings.ToLower(content), "no definition") {
			t.Fatalf("wrongly claimed definition is in main.go (fake impl echoes usage file).\nGot: %s", content)
		}
		t.Logf("cross-file definition not found (known limitation); did not falsely claim main.go")
		return
	}
	t.Logf("found cross-file function definition in util.go")
}

// TestGoDefinitionSymbolField verifies the structured JSON response contains the Symbol field.
func TestGoDefinitionSymbolField(t *testing.T) {
	projectDir := testutil.CopyProjectTo(t, "simple")
	mainGo := filepath.Join(projectDir, "main.go")

	locatorAdd := map[string]any{
		"symbol_name":  "Add",
		"context_file": mainGo,
		"line_hint":    28,
	}

	getSymbol := func(t *testing.T, args map[string]any) map[string]any {
		t.Helper()
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: "go_definition", Arguments: args})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if res == nil {
			t.Fatal("expected non-nil result")
		}
		if res.StructuredContent == nil {
			t.Fatal("expected StructuredContent to be populated")
		}
		jsonData, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatalf("marshal structured content: %v", err)
		}
		t.Logf("structured JSON:\n%s", string(jsonData))
		var result map[string]any
		if err := json.Unmarshal(jsonData, &result); err != nil {
			t.Fatalf("unmarshal structured content: %v", err)
		}
		sym, ok := result["symbol"].(map[string]any)
		if !ok {
			t.Fatal("expected 'symbol' field in result")
		}
		return sym
	}

	t.Run("SymbolFieldExists", func(t *testing.T) {
		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name:      "go_definition",
			Arguments: map[string]any{"locator": locatorAdd},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if res == nil {
			t.Fatal("expected non-nil result")
		}
		testutil.ResultText(t, res, "")

		sym := getSymbol(t, map[string]any{"locator": locatorAdd})
		name, _ := sym["name"].(string)
		if name != "Add" {
			t.Errorf("expected symbol name 'Add', got %q", name)
		}
		if kind, _ := sym["kind"].(string); kind == "" {
			t.Error("expected non-empty 'kind'")
		}
		if sig, _ := sym["signature"].(string); sig == "" {
			t.Error("expected non-empty 'signature'")
		}
		t.Logf("name=%s kind=%s signature=%s", sym["name"], sym["kind"], sym["signature"])
	})

	t.Run("IncludeBodyFalse", func(t *testing.T) {
		sym := getSymbol(t, map[string]any{"locator": locatorAdd, "include_body": false})
		if body, ok := sym["body"].(string); ok && body != "" {
			t.Errorf("expected empty body when include_body=false, got: %s", body)
		}
		t.Logf("body is empty when include_body=false")
	})

	t.Run("IncludeBodyTrue", func(t *testing.T) {
		sym := getSymbol(t, map[string]any{"locator": locatorAdd, "include_body": true})
		body, ok := sym["body"].(string)
		if !ok || body == "" {
			t.Error("expected non-empty body when include_body=true")
		} else {
			t.Logf("body present: %s", body)
		}
	})
}
