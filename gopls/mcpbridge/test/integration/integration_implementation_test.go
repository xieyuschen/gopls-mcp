package integration

// End-to-end test for go_implementation functionality.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestGoImplementationE2E is an end-to-end test that verifies go_implementation works.
func TestGoImplementationE2E(t *testing.T) {
	t.Run("FindInterfaceImplementations", func(t *testing.T) {
		// Create a test project with an interface and multiple implementations
		projectDir := t.TempDir()

		// Initialize go.mod
		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a file with an interface and implementations
		sourceCode := `package main

import "fmt"

// Shape is an interface for geometric shapes
type Shape interface {
	Area() float64
	Perimeter() float64
}

// Rectangle is a concrete implementation of Shape
type Rectangle struct {
	Width  float64
	Height float64
}

func (r Rectangle) Area() float64 {
	return r.Width * r.Height
}

func (r Rectangle) Perimeter() float64 {
	return 2 * (r.Width + r.Height)
}

// Circle is another concrete implementation of Shape
type Circle struct {
	Radius float64
}

func (c Circle) Area() float64 {
	return 3.14159 * c.Radius * c.Radius
}

func (c Circle) Perimeter() float64 {
	return 2 * 3.14159 * c.Radius
}

// Triangle is a third implementation of Shape
type Triangle struct {
	Base   float64
	Height float64
	SideA  float64
	SideB  float64
}

func (t Triangle) Area() float64 {
	return 0.5 * t.Base * t.Height
}

func (t Triangle) Perimeter() float64 {
	return t.Base + t.SideA + t.SideB
}

func main() {
	shapes := []Shape{
		Rectangle{Width: 10, Height: 5},
		Circle{Radius: 7},
		Triangle{Base: 3, Height: 4, SideA: 5, SideB: 5},
	}

	for _, shape := range shapes {
		fmt.Printf("Area: %.2f, Perimeter: %.2f\n", shape.Area(), shape.Perimeter())
	}
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Start gopls-mcp

		tool := "go_implementation"

		// Find all implementations of Shape interface using semantic locator
		// Line 36: type Shape interface {
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":  "Shape",
				"context_file": mainGoPath,
				"kind":         "interface",
				"line_hint":    36,
			},
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenImplementationInterface)
		t.Logf("Interface implementations:\n%s", content)

		// Compare against golden file (documentation + regression check)

		// Should find implementations
		if !strings.Contains(content, "implementation") && !strings.Contains(content, "Rectangle") && !strings.Contains(content, "Circle") {
			t.Errorf("Expected to find implementations of Shape interface, got: %s", content)
		}

		// Verify the summary mentions implementations found
		if strings.Contains(content, "Found 0 implementation") {
			t.Errorf("Expected to find implementations, but got 0: %s", content)
		}
	})

	t.Run("FindInterfacesImplementedByType", func(t *testing.T) {
		// Create a test project with a concrete type implementing interfaces
		projectDir := t.TempDir()

		// Initialize go.mod
		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a file with a type implementing multiple interfaces
		sourceCode := `package main

import "io"

// MyReader implements io.Reader, io.Writer, and io.Closer
type MyReader struct {
	data []byte
	pos  int
}

func (m *MyReader) Read(p []byte) (n int, err error) {
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}
	n = copy(p, m.data[m.pos:])
	m.pos += n
	return n, nil
}

func (m *MyReader) Write(p []byte) (n int, err error) {
	m.data = append(m.data, p...)
	return len(p), nil
}

func (m *MyReader) Close() error {
	m.data = nil
	m.pos = 0
	return nil
}

func main() {
	r := &MyReader{data: []byte("hello")}
	_ = r
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Start gopls-mcp

		tool := "go_implementation"

		// Find what interfaces MyReader implements
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":        "MyReader",
				"context_file":       mainGoPath,
				"package_identifier": "main",
				"line_hint":          8,
			},
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenImplementationInterfaceByType)
		t.Logf("Interfaces implemented by type:\n%s", content)

		// Compare against golden file (documentation + regression check)

		// Should find interfaces
		if !strings.Contains(content, "interface") && !strings.Contains(content, "implementation") {
			t.Errorf("Expected to find interfaces implemented by MyReader, got: %s", content)
		}
	})

	t.Run("FindMethodImplementations", func(t *testing.T) {
		// Create a test project with interface methods
		projectDir := t.TempDir()

		// Initialize go.mod
		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a file with interface and method implementations
		sourceCode := `package main

import "fmt"

// Writer interface defines a write method
type Writer interface {
	Write(data string) error
}

// FileWriter implements Writer
type FileWriter struct {
	path string
}

func (f FileWriter) Write(data string) error {
	// Simulated file write
	fmt.Printf("Writing to %s: %s\n", f.path, data)
	return nil
}

// ConsoleWriter implements Writer
type ConsoleWriter struct{}

func (c ConsoleWriter) Write(data string) error {
	fmt.Println(data)
	return nil
}

func main() {
	writers := []Writer{
		FileWriter{path: "/tmp/test.txt"},
		ConsoleWriter{},
	}

	for _, writer := range writers {
		writer.Write("Hello, World!")
	}
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Start gopls-mcp

		tool := "go_implementation"

		// Find implementations of Write method
		args := map[string]any{
			"locator": map[string]any{
				"symbol_name":        "Write",
				"context_file":       mainGoPath,
				"package_identifier": "main",
				"line_hint":          7,
			},
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenImplementationMethod)
		t.Logf("Method implementations:\n%s", content)

		// Should find method implementations
		if !strings.Contains(content, "implementation") {
			t.Errorf("Expected to find implementations of Write method, got: %s", content)
		}
	})
}
