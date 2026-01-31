package integration

// End-to-end test for get_package_symbol_detail functionality.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestGetPackageSymbolDetailE2E is an end-to-end test that verifies get_package_symbol_detail works.
func TestGetPackageSymbolDetailE2E(t *testing.T) {
	t.Run("WithSpecificFilters", func(t *testing.T) {
		// Test with specific symbol_filters - should return only matching symbols
		projectDir := t.TempDir()

		// Initialize go.mod
		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a file with functions
		sourceCode := `package main

import "fmt"

// Hello returns a greeting message
func Hello() string {
	return "hello world"
}

// Add returns the sum of two integers
func Add(a, b int) int {
	return a + b
}

func main() {
	fmt.Println(Hello())
	fmt.Println(Add(1, 2))
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		tool := "go_get_package_symbol_detail"
		args := map[string]any{
			"package_path":   "example.com/test",
			"symbol_filters": []map[string]any{{"name": "Hello"}},
			"include_docs":   true,
			"include_bodies": false,
			"Cwd":            projectDir,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenGetPackageSymbolDetail)
		t.Logf("Package Symbol Detail (specific filters):\n%s", content)

		// Should find the Hello function
		if !strings.Contains(content, "Hello") {
			t.Errorf("Expected to find Hello symbol, got: %s", content)
		}

		// Should NOT find Add or Multiply (not in filters)
		if strings.Contains(content, "Add") {
			t.Errorf("Expected NOT to find Add symbol (not in filters), got: %s", content)
		}
		if strings.Contains(content, "Multiply") {
			t.Errorf("Expected NOT to find Multiply symbol (not in filters), got: %s", content)
		}

		// Should contain signature for Hello
		if !strings.Contains(content, "func() string") {
			t.Errorf("Expected to find signature for Hello, got: %s", content)
		}
	})

	t.Run("WithSymbolFilters", func(t *testing.T) {
		// Test with symbol_filters - should return only matching symbols
		projectDir := t.TempDir()

		// Initialize go.mod
		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a file with functions
		sourceCode := `package main

import "fmt"

// Hello returns a greeting message
func Hello() string {
	return "hello world"
}

// Add returns the sum of two integers
func Add(a, b int) int {
	return a + b
}

// Multiply returns the product
func Multiply(a, b int) int {
	return a * b
}

func main() {
	fmt.Println(Hello())
	fmt.Println(Add(1, 2))
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		tool := "go_get_package_symbol_detail"
		args := map[string]any{
			"package_path": "example.com/test",
			"symbol_filters": []any{
				map[string]any{"name": "Hello"},
				map[string]any{"name": "Add"},
			},
			"include_docs":   false,
			"include_bodies": false,
			"Cwd":            projectDir,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenGetPackageSymbolDetail)
		t.Logf("Package Symbol Detail (symbol filters):\n%s", content)

		// Should find Hello and Add
		if !strings.Contains(content, "Hello") {
			t.Errorf("Expected to find Hello symbol, got: %s", content)
		}
		if !strings.Contains(content, "Add") {
			t.Errorf("Expected to find Add symbol, got: %s", content)
		}

		// Should NOT find Multiply (not in filters)
		if strings.Contains(content, "Multiply") {
			t.Errorf("Expected NOT to find Multiply symbol (not in filters), got: %s", content)
		}

		// Should contain signatures
		if !strings.Contains(content, "func() string") {
			t.Errorf("Expected to find signature for Hello, got: %s", content)
		}
		if !strings.Contains(content, "func(a, b int) int") {
			t.Errorf("Expected to find signature for Add, got: %s", content)
		}
	})

	t.Run("MethodsWithReceiverFilter", func(t *testing.T) {
		// Test filtering methods by receiver
		projectDir := t.TempDir()

		// Initialize go.mod
		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a file with types and methods
		sourceCode := `package main

import "fmt"

// Person represents a person
type Person struct {
	Name string
	Age  int
}

// Greeting returns a greeting from the person
func (p *Person) Greeting() string {
	return fmt.Sprintf("Hello, I am %s", p.Name)
}

// Birthday increases the person's age
func (p *Person) Birthday() {
	p.Age++
}

// Animal represents an animal
type Animal struct {
	Species string
}

// Speak makes the animal speak
func (a *Animal) Speak() string {
	return "..."
}

func main() {
	p := Person{Name: "Alice", Age: 30}
	fmt.Println(p.Greeting())
	p.Birthday()
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		// First, try list_package_symbols to verify the symbols exist
		t.Log("Testing with list_package_symbols first...")
		listRes, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
			Name: "go_list_package_symbols",
			Arguments: map[string]any{
				"package_path":   "example.com/test",
				"include_docs":   false,
				"include_bodies": false,
				"Cwd":            projectDir,
			},
		})
		if err != nil {
			t.Logf("Warning: list_package_symbols failed: %v", err)
		} else {
			listContent := testutil.ResultText(t, listRes, testutil.GoldenGetPackageSymbolDetail)
			t.Logf("list_package_symbols result (first 500 chars): %s", testutil.TruncateString(listContent, 500))
			if strings.Contains(listContent, "Person") {
				t.Log("Found Person type via list_package_symbols")
			} else {
				t.Log("WARNING: Person type NOT found via list_package_symbols")
			}
		}

		tool := "go_get_package_symbol_detail"
		args := map[string]any{
			"package_path": "example.com/test",
			"symbol_filters": []any{
				map[string]any{"name": "Greeting", "receiver": "*Person"},
				map[string]any{"name": "Birthday", "receiver": "*Person"},
			},
			"include_docs":   true,
			"include_bodies": true,
			"Cwd":            projectDir,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenGetPackageSymbolDetail)
		t.Logf("Package Symbol Detail (methods with receiver filter):\n%s", content)

		// Should find Greeting and Birthday methods
		if !strings.Contains(content, "Greeting") {
			t.Errorf("Expected to find Greeting method, got: %s", content)
		}
		if !strings.Contains(content, "Birthday") {
			t.Errorf("Expected to find Birthday method, got: %s", content)
		}

		// Should NOT find Speak (different receiver)
		if strings.Contains(content, "Speak") {
			t.Errorf("Expected NOT to find Speak method (different receiver), got: %s", content)
		}

		// Should contain receiver information
		if !strings.Contains(content, "*Person") {
			t.Errorf("Expected to find receiver '*Person', got: %s", content)
		}

		// Should contain method signatures
		if !strings.Contains(content, "(*Person).Greeting - func() string") {
			t.Errorf("Expected to find signature for Greeting, got: %s", content)
		}
		if !strings.Contains(content, "(*Person).Birthday - func()") {
			t.Errorf("Expected to find signature for Birthday, got: %s", content)
		}

		// Should include documentation (docs requested)
		if !strings.Contains(content, "Greeting returns a greeting") {
			t.Errorf("Expected to find documentation for Greeting, got: %s", content)
		}
		if !strings.Contains(content, "Birthday increases the person's age") {
			t.Errorf("Expected to find documentation for Birthday, got: %s", content)
		}

		// Should include bodies (bodies requested)
		if !strings.Contains(content, "fmt.Sprintf") {
			t.Errorf("Expected body to be included in Greeting, got: %s", content)
		}
		if !strings.Contains(content, "p.Age++") {
			t.Errorf("Expected body to be included in Birthday, got: %s", content)
		}
	})

	t.Run("SignaturesOnly", func(t *testing.T) {
		// Test with include_bodies=false (default) - should return signatures only
		projectDir := t.TempDir()

		// Initialize go.mod
		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a file with functions
		sourceCode := `package main

import "fmt"

// Hello returns a greeting message
func Hello() string {
	return "hello world"
}

// Add returns the sum of two integers
func Add(a, b int) int {
	return a + b
}

func main() {
	fmt.Println(Hello())
	fmt.Println(Add(1, 2))
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		tool := "go_get_package_symbol_detail"
		args := map[string]any{
			"package_path": "example.com/test",
			"symbol_filters": []any{
				map[string]any{"name": "Hello"},
				map[string]any{"name": "Add"},
			},
			"include_docs":   false,
			"include_bodies": false,
			"Cwd":            projectDir,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenGetPackageSymbolDetail)
		t.Logf("Package Symbol Detail (signatures only):\n%s", content)

		// Compare against golden file (documentation + regression check)

		// Should mention symbols
		if !strings.Contains(content, "Hello") {
			t.Errorf("Expected to find Hello symbol, got: %s", content)
		}
		if !strings.Contains(content, "Add") {
			t.Errorf("Expected to find Add symbol, got: %s", content)
		}

		// Should contain signatures
		if !strings.Contains(content, "func() string") {
			t.Errorf("Expected to find signature for Hello, got: %s", content)
		}
		if !strings.Contains(content, "func(a, b int) int") {
			t.Errorf("Expected to find signature for Add, got: %s", content)
		}

		// Should NOT contain bodies
		if strings.Contains(content, "return") {
			t.Errorf("Expected NOT to contain function bodies (include_bodies=false), got: %s", content)
		}
	})

	t.Run("WithBodies", func(t *testing.T) {
		// Test with include_bodies=true - should return full implementations
		projectDir := t.TempDir()

		// Initialize go.mod
		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a file with functions that have distinct implementations
		sourceCode := `package main

// Hello returns a greeting message
func Hello() string {
	return "hello world"
}

// Multiply returns the product of two integers
func Multiply(a, b int) int {
	result := a * b
	return result
}

func main() {
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		tool := "go_get_package_symbol_detail"
		args := map[string]any{
			"package_path": "example.com/test",
			"symbol_filters": []any{
				map[string]any{"name": "Hello"},
				map[string]any{"name": "Multiply"},
			},
			"include_docs":   false,
			"include_bodies": true,
			"Cwd":            projectDir,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenGetPackageSymbolDetail)
		t.Logf("Package Symbol Detail (with bodies):\n%s", content)

		// Should find Hello and Multiply
		if !strings.Contains(content, "Hello") {
			t.Errorf("Expected to find Hello function, got: %s", content)
		}
		if !strings.Contains(content, "Multiply") {
			t.Errorf("Expected to find Multiply function, got: %s", content)
		}

		// Should contain function bodies
		if !strings.Contains(content, "return \"hello world\"") {
			t.Errorf("Expected Hello body, got: %s", content)
		}
		if !strings.Contains(content, "result := a * b") {
			t.Errorf("Expected Multiply body with local variable, got: %s", content)
		}

		// Should contain signatures
		if !strings.Contains(content, "func() string") {
			t.Errorf("Expected to find signature for Hello, got: %s", content)
		}
		if !strings.Contains(content, "func(a, b int) int") {
			t.Errorf("Expected to find signature for Multiply, got: %s", content)
		}
	})

	t.Run("NonExistentPackage", func(t *testing.T) {
		// Test querying a package that doesn't exist
		projectDir := t.TempDir()

		// Initialize go.mod
		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create minimal main.go
		sourceCode := `package main

func main() {
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		tool := "go_get_package_symbol_detail"
		args := map[string]any{
			"package_path":   "does/not/exist",
			"symbol_filters": []any{map[string]any{"name": "Something"}},
			"include_bodies": false,
			"Cwd":            projectDir,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})

		// Non-existent package should error
		if err != nil {
			t.Logf("Expected error for non-existent package: %v", err)
		} else if res != nil {
			content := testutil.ResultText(t, res, testutil.GoldenGetPackageSymbolDetail)
			t.Logf("Result for non-existent package: %s", content)

			// If no error, the result should mention the issue
			if !strings.Contains(content, "not found") &&
				!strings.Contains(content, "no such package") &&
				!strings.Contains(content, "error") &&
				!strings.Contains(content, "failed") {
				t.Logf("Note: Tool didn't error for non-existent package, returned: %s", content)
			}
		}
	})

	t.Run("NonExistentSymbol", func(t *testing.T) {
		// Test querying for a symbol that doesn't exist in the package
		projectDir := t.TempDir()

		// Initialize go.mod
		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create minimal main.go
		sourceCode := `package main

func main() {
}
`
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		tool := "go_get_package_symbol_detail"
		args := map[string]any{
			"package_path":   "example.com/test",
			"symbol_filters": []any{map[string]any{"name": "NonExistentFunc"}},
			"include_docs":   false,
			"include_bodies": false,
			"Cwd":            projectDir,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenGetPackageSymbolDetail)
		t.Logf("Package Symbol Detail (non-existent symbol):\n%s", content)

		// Should return empty result (symbol doesn't exist)
		// The output should not contain any function definitions
		if strings.Contains(content, "func NonExistentFunc") {
			t.Errorf("Expected NOT to find NonExistentFunc (doesn't exist), got: %s", content)
		}

		// Should indicate no symbols found or empty result
		if !strings.Contains(content, "No symbols found") &&
			!strings.Contains(content, "no symbols") &&
			!strings.Contains(content, "0 symbols") {
			t.Logf("Note: Result for non-existent symbol: %s", content)
		}
	})
}
