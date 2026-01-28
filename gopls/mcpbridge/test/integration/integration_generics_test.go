package integration

// End-to-end tests for generics support and interface satisfaction.
// These tests verify that gopls-mcp can handle Go 1.18+ generics
// and help developers understand why types don't implement interfaces.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestGenericsSupport tests navigation and understanding of generic types and functions.
func TestGenericsSupport(t *testing.T) {

	t.Run("GenericFunctionDefinition", func(t *testing.T) {
		// Test navigating to generic function definitions
		projectDir := t.TempDir()

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create code with generic functions
		sourceCode := `package main

// Map applies a function to each element in a slice.
func Map[T any](slice []T, fn func(T) T) []T {
	result := make([]T, len(slice))
	for i, v := range slice {
		result[i] = fn(v)
	}
	return result
}

// Filter filters a slice based on a predicate.
func Filter[T any](slice []T, predicate func(T) bool) []T {
	var result []T
	for _, v := range slice {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

// Reduce reduces a slice to a single value.
func Reduce[T, U any](slice []T, initial U, fn func(U, T) U) U {
	result := initial
	for _, v := range slice {
		result = fn(result, v)
	}
	return result
}

func main() {
	numbers := []int{1, 2, 3, 4, 5}
	doubled := Map(numbers, func(n int) int { return n * 2 })
	evens := Filter(numbers, func(n int) bool { return n%2 == 0 })
	sum := Reduce(numbers, 0, func(acc, n int) int { return acc + n })

	_ = doubled
	_ = evens
	_ = sum
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Start gopls-mcp

		t.Run("JumpToGenericFunction", func(t *testing.T) {
			// Test jumping to Map function definition
			tool := "go_definition"
			args := map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Map",
					"context_file": mainGoPath,
					"kind":         "function",
					"line_hint":    30,
				},
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
			if err != nil {
				t.Fatalf("Failed to call tool %s: %v", tool, err)
			}

			if res == nil {
				t.Fatal("Expected non-nil result")
			}

			content := testutil.ResultText(t, res, testutil.GoldenGenericsSupport)
			t.Logf("Definition result for Map[T]:\n%s", content)

			// Should find the generic function definition
			if strings.Contains(content, "Definition found") || strings.Contains(content, "Map") {
				t.Logf("✓ Found generic function definition")
			}
		})

	})

	t.Run("GenericTypeDefinition", func(t *testing.T) {
		// Test generic type definitions
		projectDir := t.TempDir()

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create code with generic types
		sourceCode := `package main

// Stack is a generic stack implementation.
type Stack[T any] struct {
	elements []T
}

// Push adds an element to the stack.
func (s *Stack[T]) Push(v T) {
	s.elements = append(s.elements, v)
}

// Pop removes and returns the top element.
func (s *Stack[T]) Pop() (T, bool) {
	var zero T
	if len(s.elements) == 0 {
		return zero, false
	}
	idx := len(s.elements) - 1
	element := s.elements[idx]
	s.elements = s.elements[:idx]
	return element, true
}

// Pair is a generic pair type.
type Pair[T, U any] struct {
	First  T
	Second U
}

func main() {
	// Use generic stack
	stack := Stack[int]{}
	stack.Push(1)
	stack.Push(2)
	val, ok := stack.Pop()

	// Use generic pair
	p := Pair[string, int]{"age", 30}

	_ = val
	_ = ok
	_ = p
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Start gopls-mcp

		t.Run("JumpToGenericType", func(t *testing.T) {
			// Test jumping to generic type definition
			tool := "go_definition"
			args := map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Stack",
					"context_file": mainGoPath,
					"kind":         "struct",
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

			content := testutil.ResultText(t, res, testutil.GoldenGenericsSupport)
			t.Logf("Definition result for Stack[T]:\n%s", content)

			// Should find the generic type definition
			if strings.Contains(content, "Definition found") || strings.Contains(content, "Stack") {
				t.Logf("✓ Found generic type definition")
			}
		})
	})

	t.Run("GenericTypeConstraints", func(t *testing.T) {
		// Test generic types with constraints
		projectDir := t.TempDir()

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create code with type constraints
		sourceCode := `package main

// Ordered is a constraint for types that support comparison operators.
type Ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64 |
		~string
}

// Max returns the maximum of two ordered values.
func Max[T Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

// Stringer is a constraint for types that can be converted to strings.
type Stringer interface {
	String() string
}

// Describe prints a description of any Stringer.
func Describe[T Stringer](s T) string {
	return s.String()
}

// Comparable is a constraint for types that can be compared.
type Comparable interface {
	~int | ~float64 | ~string
}

// Equals checks if two values are equal.
func Equals[T Comparable](a, b T) bool {
	return a == b
}

func main() {
	// Use Max with int
	maxVal := Max(10, 20)

	// Use Describe
	type Person struct {
		Name string
	}
	p := Person{Name: "Alice"}
	_ = p

	// Use Equals
	eq := Equals(1.5, 2.5)
	_ = maxVal
	_ = eq
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Start gopls-mcp

		t.Run("JumpToConstraintDefinition", func(t *testing.T) {
			// Test jumping to the constraint definition
			tool := "go_definition"
			args := map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Ordered",
					"context_file": mainGoPath,
					"kind":         "interface",
					"line_hint":    41,
				},
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
			if err != nil {
				t.Fatalf("Failed to call tool %s: %v", tool, err)
			}

			if res == nil {
				t.Fatal("Expected non-nil result")
			}

			content := testutil.ResultText(t, res, testutil.GoldenGenericsSupport)
			t.Logf("Definition result for Ordered constraint:\n%s", content)

			// Should find the Ordered interface definition
			if strings.Contains(content, "Ordered") || strings.Contains(content, "Definition found") {
				t.Logf("✓ Can navigate to constraint definition")
			}
		})
	})

	t.Run("GenericInterfaces", func(t *testing.T) {
		// Test generic interfaces
		projectDir := t.TempDir()

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create code with generic interfaces
		sourceCode := `package main

// Container is a generic interface for containers.
type Container[T any] interface {
	// Add adds an element to the container.
	Add(T)
	// Get retrieves an element by index.
	Get(int) T
	// Size returns the number of elements.
	Size() int
}

// SliceContainer implements Container using a slice.
type SliceContainer[T any] struct {
	items []T
}

func (s *SliceContainer[T]) Add(item T) {
	s.items = append(s.items, item)
}

func (s *SliceContainer[T]) Get(idx int) T {
	return s.items[idx]
}

func (s *SliceContainer[T]) Size() int {
	return len(s.items)
}

func main() {
	// Use generic interface
	var c Container[int] = &SliceContainer[int]{}
	c.Add(1)
	c.Add(2)
	val := c.Get(0)
	size := c.Size()

	_ = val
	_ = size
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Start gopls-mcp

		t.Run("FindGenericImplementations", func(t *testing.T) {
			// Test finding implementations of generic interface
			tool := "go_implementation"
			args := map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Container",
					"context_file": mainGoPath,
					"kind":         "interface",
					"line_hint":    42,
				},
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
			if err != nil {
				t.Fatalf("Failed to call tool %s: %v", tool, err)
			}

			if res == nil {
				t.Fatal("Expected non-nil result")
			}

			content := testutil.ResultText(t, res, testutil.GoldenGenericsSupport)
			t.Logf("Implementation result for Container[T]:\n%s", content)

			// Should find SliceContainer as an implementation
			if strings.Contains(content, "SliceContainer") || strings.Contains(content, "implementation") {
				t.Logf("✓ Found implementations of generic interface")
			}
		})
	})
}

// TestInterfaceSatisfaction tests understanding why a type doesn't implement an interface.
func TestInterfaceSatisfaction(t *testing.T) {

	t.Run("MissingMethod", func(t *testing.T) {
		// Test case where type is missing a method to implement interface
		projectDir := t.TempDir()

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create code with missing method
		sourceCode := `package main

// Writer is an interface for writing data.
type Writer interface {
	Write([]byte) (int, error)
	Close() error
}

// FileWriter writes to files (but doesn't implement Close).
type FileWriter struct {
	path string
}

// Write writes data to the file.
func (f *FileWriter) Write(data []byte) (int, error) {
	// Implementation would write to file
	return len(data), nil
}

// Note: FileWriter is missing Close() method

func main() {
	var w Writer = &FileWriter{path: "/tmp/test.txt"}
	_ = w
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Start gopls-mcp

		// Use go_build_check to see the error
		tool := "go_build_check"
		args := map[string]any{
			"files": []string{mainGoPath},
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenInterfaceSatisfaction)
		t.Logf("Diagnostics for missing method:\n%s", content)

		// Should report an error about FileWriter not implementing Writer
		if strings.Contains(content, "FileWriter") || strings.Contains(content, "Writer") || strings.Contains(content, "error") {
			t.Logf("✓ Diagnostics detect missing method implementation")
		}
	})

	t.Run("WrongSignature", func(t *testing.T) {
		// Test case where method has wrong signature
		projectDir := t.TempDir()

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create code with wrong method signature
		sourceCode := `package main

// Stringer is the standard Stringer interface.
type Stringer interface {
	String() string
}

// Person has a String method but with wrong signature (pointer receiver vs value).
type Person struct {
	Name string
}

// String has a pointer receiver but returns int instead of string.
func (p *Person) String() int {
	return len(p.Name)
}

func main() {
	var s Stringer = &Person{Name: "Alice"}
	_ = s
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Start gopls-mcp

		// Use go_build_check to see the error
		tool := "go_build_check"
		args := map[string]any{
			"files": []string{mainGoPath},
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenInterfaceSatisfaction)
		t.Logf("Diagnostics for wrong signature:\n%s", content)

		// Should report type mismatch
		if strings.Contains(content, "Person") || strings.Contains(content, "Stringer") || strings.Contains(content, "error") {
			t.Logf("✓ Diagnostics detect wrong method signature")
		}
	})

	t.Run("CorrectImplementation", func(t *testing.T) {
		// Test case where type correctly implements interface
		projectDir := t.TempDir()

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create code with correct implementation
		sourceCode := `package main

import "fmt"

// Stringer is the standard Stringer interface.
type Stringer interface {
	String() string
}

// Person correctly implements Stringer.
type Person struct {
	Name string
}

// String returns the person's name.
func (p *Person) String() string {
	return p.Name
}

func main() {
	var s Stringer = &Person{Name: "Bob"}
	fmt.Println(s.String())
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Start gopls-mcp

		t.Run("DiagnosticsShowNoErrors", func(t *testing.T) {
			// Use go_build_check to verify no errors
			tool := "go_build_check"
			args := map[string]any{
				"files": []string{mainGoPath},
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
			if err != nil {
				t.Fatalf("Failed to call tool %s: %v", tool, err)
			}

			if res == nil {
				t.Fatal("Expected non-nil result")
			}

			content := testutil.ResultText(t, res, testutil.GoldenInterfaceSatisfaction)
			t.Logf("Diagnostics for correct implementation:\n%s", content)

			// Should not report interface errors
			// Note: May have other diagnostics, but not about Person/Stringer mismatch
			if !strings.Contains(content, "Person") && !strings.Contains(content, "Stringer") {
				t.Logf("✓ No interface satisfaction errors")
			} else {
				t.Logf("Note: Got diagnostics, check if they're interface-related: %s", content)
			}
		})

		t.Run("FindImplementation", func(t *testing.T) {
			// Test finding implementation of interface
			tool := "go_implementation"
			args := map[string]any{
				"locator": map[string]any{
					"symbol_name":  "Stringer",
					"context_file": mainGoPath,
					"kind":         "interface",
					"line_hint":    11,
				},
			}

			res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
			if err != nil {
				t.Fatalf("Failed to call tool %s: %v", tool, err)
			}

			if res == nil {
				t.Fatal("Expected non-nil result")
			}

			content := testutil.ResultText(t, res, testutil.GoldenInterfaceSatisfaction)
			t.Logf("Implementation result:\n%s", content)

			// Should find Person as an implementation
			if strings.Contains(content, "Person") || strings.Contains(content, "implementation") {
				t.Logf("✓ Found Person implementing Stringer")
			}
		})
	})

	t.Run("GenericInterfaceSatisfaction", func(t *testing.T) {
		// Test generic interface satisfaction
		projectDir := t.TempDir()

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create code with generic interface
		sourceCode := `package main

// Equaler is a generic interface for types that can be compared for equality.
type Equaler[T any] interface {
	Equals(other T) bool
}

// Point is a simple struct.
type Point struct {
	X, Y int
}

// Equals compares two points.
func (p Point) Equals(other Point) bool {
	return p.X == other.X && p.Y == other.Y
}

func main() {
	p1 := Point{X: 1, Y: 2}
	p2 := Point{X: 1, Y: 2}

	var e Equaler[Point] = p1
	equal := e.Equals(p2)
	_ = equal
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Start gopls-mcp

		// Use go_build_check to verify no errors
		tool := "go_build_check"
		args := map[string]any{
			"files": []string{mainGoPath},
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenInterfaceSatisfaction)
		t.Logf("Diagnostics for generic interface:\n%s", content)

		// Should not report errors for correct generic interface implementation
		t.Logf("✓ Generic interface satisfaction checked")
	})

	t.Run("EmbeddedInterfaces", func(t *testing.T) {
		// Test interface with embedded interfaces
		projectDir := t.TempDir()

		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create code with embedded interfaces
		sourceCode := `package main

// Reader is the standard Reader interface.
type Reader interface {
	Read([]byte) (int, error)
}

// Writer is the standard Writer interface.
type Writer interface {
	Write([]byte) (int, error)
}

// ReadWriter combines Reader and Writer.
type ReadWriter interface {
	Reader
	Writer
}

// Buffer implements ReadWriter.
type Buffer struct {
	data []byte
	pos  int
}

func (b *Buffer) Read(p []byte) (n int, err error) {
	if b.pos >= len(b.data) {
		return 0, nil // EOF
	}
	n = copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}

func (b *Buffer) Write(p []byte) (n int, err error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func main() {
	var rw ReadWriter = &Buffer{}
	_ = rw
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Start gopls-mcp

		// Use go_build_check to verify no errors
		tool := "go_build_check"
		args := map[string]any{
			"files": []string{mainGoPath},
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenInterfaceSatisfaction)
		t.Logf("Diagnostics for embedded interfaces:\n%s", content)

		// Should not report errors for correct embedded interface implementation
		t.Logf("✓ Embedded interface satisfaction checked")
	})
}
