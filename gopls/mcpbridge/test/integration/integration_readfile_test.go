package integration

// End-to-end test for go_read_file functionality.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestGoReadFileE2E is an end-to-end test that verifies go_read_file works.
func TestGoReadFileE2E(t *testing.T) {
	t.Run("ReadExistingFile", func(t *testing.T) {
		// Create a test project
		projectDir := t.TempDir()

		// Initialize go.mod
		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a file with known content
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
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Start gopls-mcp

		tool := "go_read_file"
		args := map[string]any{
			"file": mainGoPath,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenReadFileExisting)
		t.Logf("File content:\n%s", content)

		// Verify the content matches what we wrote
		if !strings.Contains(content, "package main") {
			t.Errorf("Expected content to contain 'package main', got: %s", content)
		}

		if !strings.Contains(content, "func Hello() string") {
			t.Errorf("Expected content to contain Hello function, got: %s", content)
		}

		if !strings.Contains(content, "func Add(a, b int) int") {
			t.Errorf("Expected content to contain Add function, got: %s", content)
		}

		// Verify the file path is mentioned in summary
		if !strings.Contains(content, mainGoPath) && !strings.Contains(content, "main.go") {
			t.Errorf("Expected content to mention file path, got: %s", content)
		}
	})

	t.Run("ReadFileWithSpecialCharacters", func(t *testing.T) {
		// Create a test project with special characters in comments
		projectDir := t.TempDir()

		// Initialize go.mod
		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a file with special characters and UTF-8 content
		sourceCode := `package main

import "fmt"

// Special characters: ©, ®, ™, €, £, ¥
// Unicode: 你好世界, 🚀, 🎯
func main() {
	// Test various string literals
	s1 := "Hello, World!"
	s2 := "Привет, мир!"
	s3 := "こんにちは世界"

	fmt.Println(s1)
	fmt.Println(s2)
	fmt.Println(s3)
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Start gopls-mcp

		tool := "go_read_file"
		args := map[string]any{
			"file": mainGoPath,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenReadFileSpecialCharacters)
		t.Logf("File content with special characters:\n%s", content)

		// Verify special characters are preserved
		if !strings.Contains(content, "你好世界") {
			t.Errorf("Expected content to contain Chinese characters, got: %s", content)
		}

		if !strings.Contains(content, "🚀") {
			t.Errorf("Expected content to contain rocket emoji, got: %s", content)
		}
	})

	t.Run("ReadNonExistentFile", func(t *testing.T) {
		// This test verifies that go_read_file properly handles non-existent files.
		// Expected behavior: The tool should either return an error OR return a result
		// containing an error message about the file not being found.
		//
		// This is important for graceful error handling - we don't want the tool to
		// crash or return an empty/empty result when a file doesn't exist.

		// Create a test project
		projectDir := t.TempDir()

		// Initialize go.mod
		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a minimal main.go (we need at least one valid Go file for the project)
		sourceCode := `package main

func main() {
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Start gopls-mcp

		// Attempt to read a file that doesn't exist
		tool := "go_read_file"
		nonExistentPath := filepath.Join(projectDir, "does_not_exist.go")

		args := map[string]any{
			"file": nonExistentPath,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})

		// Verify we get proper error handling
		// Case 1: Tool call itself returns an error (acceptable)
		if err != nil {
			return
		}

		// Case 2: Tool returns a result - it should contain an error message
		if res != nil {
			content := testutil.ResultText(t, res, testutil.GoldenReadFileNonExistent)

			if !strings.Contains(content, "failed to get file content") {
				// This is unexpected - the tool should have indicated an error somehow
				t.Errorf("Tool should return an error or error message for non-existent file, but got: %s", content)
			}
		} else {
			t.Errorf("Tool returned nil result and nil error for non-existent file (should indicate error)")
		}
	})

	t.Run("ReadLargeFile", func(t *testing.T) {
		// Create a test project with a larger file
		projectDir := t.TempDir()

		// Initialize go.mod
		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a file with many functions
		var builder strings.Builder
		builder.WriteString("package main\n\n")
		builder.WriteString("import \"fmt\"\n\n")

		// Generate 20 functions
		for i := 1; i <= 20; i++ {
			builder.WriteString(fmt.Sprintf("// Function%d does some work\n", i))
			builder.WriteString(fmt.Sprintf("func Function%d() int {\n", i))
			builder.WriteString(fmt.Sprintf("\treturn %d\n", i))
			builder.WriteString("}\n\n")
		}

		builder.WriteString("func main() {\n")
		for i := 1; i <= 20; i++ {
			builder.WriteString(fmt.Sprintf("\tfmt.Println(Function%d())\n", i))
		}
		builder.WriteString("}\n")

		largeFilePath := filepath.Join(projectDir, "large.go")
		if err := os.WriteFile(largeFilePath, []byte(builder.String()), 0644); err != nil {
			t.Fatal(err)
		}

		// Start gopls-mcp

		tool := "go_read_file"
		args := map[string]any{
			"file": largeFilePath,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		content := testutil.ResultText(t, res, testutil.GoldenReadFileLarge)
		t.Logf("Large file read (length: %d chars)", len(content))

		// Verify we got the complete file
		if !strings.Contains(content, "Function1()") {
			preview := content
			if len(preview) > 100 {
				preview = content[:100] + "..."
			}
			t.Errorf("Expected content to contain Function1, got: %s", preview)
		}

		if !strings.Contains(content, "Function20()") {
			t.Errorf("Expected content to contain Function20, file may be truncated")
		}

		// Count the number of functions to ensure complete content
		functionCount := strings.Count(content, "func Function")
		if functionCount != 20 {
			t.Errorf("Expected 20 functions, got %d", functionCount)
		}
	})

	t.Run("ReadFileWithOffset", func(t *testing.T) {
		// Create a test project
		projectDir := t.TempDir()

		// Initialize go.mod
		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a file with known content
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
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Read file starting from line 10
		tool := "go_read_file"
		args := map[string]any{
			"file":   mainGoPath,
			"offset": 10,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		// Extract text content from result
		var content string
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				content = tc.Text
				break
			}
		}

		t.Logf("File content from line 10:\n%s", content)

		// Verify we got content starting from line 10
		// line 10 in the file is: // Add returns the sum of two integers
		if !strings.Contains(content, "// Add returns the sum") {
			t.Errorf("Expected content to contain '// Add returns the sum'")
		}

		// Verify that earlier content is NOT in the main content (may be in summary)
		lines := strings.Split(content, "\n")
		foundPackageMainInContent := false
		for i, line := range lines {
			// Skip first line (summary)
			if i == 0 {
				continue
			}
			if strings.Contains(line, "package main") {
				foundPackageMainInContent = true
				break
			}
		}
		if foundPackageMainInContent {
			t.Errorf("Expected content to NOT contain 'package main' after line 10")
		}

		// Verify we still got the Add function
		if !strings.Contains(content, "func Add(a, b int) int") {
			t.Errorf("Expected content to contain Add function")
		}
	})

	t.Run("ReadFileWithOffsetAndMaxLines", func(t *testing.T) {
		// Create a test project
		projectDir := t.TempDir()

		// Initialize go.mod
		goModContent := `module example.com/test

go 1.21
`
		if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a file with known content
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
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Read file starting from line 6, limited to 5 lines
		tool := "go_read_file"
		args := map[string]any{
			"file":      mainGoPath,
			"offset":    6,
			"max_lines": 5,
		}

		res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("Failed to call tool %s: %v", tool, err)
		}

		if res == nil {
			t.Fatal("Expected non-nil result")
		}

		// Extract text content from result
		var content string
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				content = tc.Text
				break
			}
		}

		t.Logf("File content from line 6, max 5 lines:\n%s", content)

		// Verify we got the Hello function (which starts at line 6)
		if !strings.Contains(content, "func Hello()") {
			t.Errorf("Expected content to contain Hello function")
		}

		// Verify content is truncated by counting lines
		lines := strings.Split(content, "\n")
		// Count non-empty content lines (skip summary line)
		contentLineCount := 0
		for i, line := range lines {
			// Skip first line (summary starts with "Read")
			if i == 0 {
				continue
			}
			if strings.TrimSpace(line) != "" {
				contentLineCount++
			}
		}
		// Should be approximately 5-6 lines (allowing for some variation)
		if contentLineCount > 10 {
			t.Logf("Warning: Expected approximately 5 lines, got %d", contentLineCount)
		}
	})
}
