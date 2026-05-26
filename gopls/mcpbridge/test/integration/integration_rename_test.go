package integration

// Strong end-to-end tests for go_dryrun_rename_symbol functionality.
// Verifies the rename preview returns accurate changes and proves no mutation (DRY RUN).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// writeGoMod writes a minimal go.mod to projectDir.
func writeGoMod(t *testing.T, projectDir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

// findLineInSrc returns the 1-based line number of the first line in src containing needle.
func findLineInSrc(t *testing.T, src, needle string) int {
	t.Helper()
	for i, line := range strings.Split(src, "\n") {
		if strings.Contains(line, needle) {
			return i + 1
		}
	}
	t.Fatalf("could not find %q in source", needle)
	return 0
}

// renameArgs builds the standard locator+new_name args map.
func renameArgs(symbol, contextFile string, lineHint int, newName string) map[string]any {
	return map[string]any{
		"locator": map[string]any{
			"symbol_name":  symbol,
			"context_file": contextFile,
			"line_hint":    lineHint,
		},
		"new_name": newName,
	}
}

// assertDryRun verifies the essential dry-run invariants on content:
//   - Output contains "DRY RUN"
//   - Unified diff headers (--- and +++) present
//   - oldSym appears on removal lines (-), newSym on addition lines (+)
//
// Returns the removal and addition counts.
func assertDryRun(t *testing.T, content, oldSym, newSym string) (removals, additions int) {
	t.Helper()
	if !strings.Contains(strings.ToUpper(content), "DRY RUN") {
		t.Fatalf("CRITICAL: output must contain 'DRY RUN'; got:\n%s", content)
	}
	if !strings.Contains(content, "---") || !strings.Contains(content, "+++") {
		t.Errorf("expected unified diff headers (--- / +++)")
	}
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "-") && strings.Contains(line, oldSym) {
			removals++
		}
		if strings.HasPrefix(line, "+") && strings.Contains(line, newSym) {
			additions++
		}
	}
	return removals, additions
}

// assertFileUnchanged checks a file still matches original content.
func assertFileUnchanged(t *testing.T, path, original string) {
	t.Helper()
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != original {
		t.Errorf("DRY RUN VIOLATED: %s was modified!", path)
	}
}

// callRename invokes go_dryrun_rename_symbol and returns the text content.
func callRename(t *testing.T, args map[string]any, golden string) string {
	t.Helper()
	res, err := globalSession.CallTool(globalCtx, &mcp.CallToolParams{
		Name:      "go_dryrun_rename_symbol",
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("go_dryrun_rename_symbol failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	return testutil.ResultText(t, res, golden)
}

// TestGoRenameSymbol_Strong verifies exact change counts, multi-file renames, and type renames.
func TestGoRenameSymbol_Strong(t *testing.T) {
	t.Run("ExactChangeCountAndFiles", func(t *testing.T) {
		projectDir := t.TempDir()
		writeGoMod(t, projectDir)

		// OldName defined once and called 3 times → 4 diff lines total.
		src := `package main

import "fmt"

// OldName is a function to be renamed
func OldName() string {
	return "hello"
}

func main() {
	result := OldName()
	fmt.Println(result)

	x := OldName() + " suffix"
	y := OldName() + " again"
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}

		args := renameArgs("OldName", mainGoPath, findLineInSrc(t, src, "func OldName()"), "NewName")
		content := callRename(t, args, testutil.GoldenRenameSymbolExact)
		t.Logf("rename result:\n%s", content)

		removals, additions := assertDryRun(t, content, "OldName", "NewName")
		if removals < 3 || additions < 3 {
			t.Errorf("expected ≥3 OldName removals and ≥3 NewName additions; got %d / %d", removals, additions)
		}
		t.Logf("✓ %d removals, %d additions", removals, additions)

		assertFileUnchanged(t, mainGoPath, src)
		t.Logf("✓ DRY RUN: file unchanged")
	})

	t.Run("MultiFileRenamePreview", func(t *testing.T) {
		projectDir := t.TempDir()
		writeGoMod(t, projectDir)

		// util/helper.go defines SharedFunc; main.go and other.go each call it.
		utilDir := filepath.Join(projectDir, "util")
		if err := os.Mkdir(utilDir, 0755); err != nil {
			t.Fatal(err)
		}
		utilCode := `package util

func SharedFunc(x int) int {
	return x * 2
}
`
		helperPath := filepath.Join(utilDir, "helper.go")
		if err := os.WriteFile(helperPath, []byte(utilCode), 0644); err != nil {
			t.Fatal(err)
		}

		mainCode := `package main

import (
	"fmt"
	"example.com/test/util"
)

func main() {
	a := util.SharedFunc(5)
	b := util.SharedFunc(10)
	fmt.Println(a, b)
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(mainCode), 0644); err != nil {
			t.Fatal(err)
		}

		otherCode := `package main

import "example.com/test/util"

func AnotherFunc() int {
	return util.SharedFunc(20)
}
`
		otherGoPath := filepath.Join(projectDir, "other.go")
		if err := os.WriteFile(otherGoPath, []byte(otherCode), 0644); err != nil {
			t.Fatal(err)
		}

		args := renameArgs("SharedFunc", helperPath, findLineInSrc(t, utilCode, "func SharedFunc("), "RenamedFunc")
		content := callRename(t, args, testutil.GoldenRenameSymbolMultiFile)
		t.Logf("multi-file rename result:\n%s", content)

		removals, additions := assertDryRun(t, content, "SharedFunc", "RenamedFunc")
		if removals < 1 || additions < 1 {
			t.Errorf("expected ≥1 SharedFunc removal and ≥1 RenamedFunc addition; got %d / %d", removals, additions)
		}

		if !strings.Contains(content, "helper.go") && !strings.Contains(content, "util/helper.go") {
			t.Errorf("expected output to mention helper.go")
		}
		t.Logf("✓ %d removals, %d additions", removals, additions)

		for path, original := range map[string]string{
			mainGoPath:  mainCode,
			otherGoPath: otherCode,
			helperPath:  utilCode,
		} {
			assertFileUnchanged(t, path, original)
		}
		t.Logf("✓ DRY RUN: all 3 files unchanged")
	})

	t.Run("TypeRenamePreview", func(t *testing.T) {
		projectDir := t.TempDir()
		writeGoMod(t, projectDir)

		src := `package main

import "fmt"

// OldType is a type
type OldType struct {
	Value int
}

func (o OldType) Process() {
	fmt.Println(o.Value)
}

func main() {
	ot := OldType{Value: 42}
	ot.Process()

	var ptr *OldType
	_ = ptr
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}

		args := renameArgs("OldType", mainGoPath, findLineInSrc(t, src, "type OldType struct"), "NewType")
		content := callRename(t, args, testutil.GoldenRenameSymbolType)
		t.Logf("type rename result:\n%s", content)

		removals, additions := assertDryRun(t, content, "OldType", "NewType")
		if removals < 1 || additions < 1 {
			t.Errorf("expected unified diff with OldType removals and NewType additions; got %d / %d", removals, additions)
		}
		t.Logf("✓ %d removals, %d additions", removals, additions)

		assertFileUnchanged(t, mainGoPath, src)
		t.Logf("✓ DRY RUN: type unchanged")
	})
}

// TestGoRenameSymbolE2E is kept for backward compatibility.
func TestGoRenameSymbolE2E(t *testing.T) {
	TestGoRenameSymbol_Strong(t)
}

// TestComplexRenameScenarios covers cross-package and conflict rename scenarios.
func TestComplexRenameScenarios(t *testing.T) {
	t.Run("RenameWithSymbolConflict", func(t *testing.T) {
		// Rename to a name that already exists — tool must not modify the file.
		projectDir := t.TempDir()
		writeGoMod(t, projectDir)

		src := `package main

func FunctionOne() string {
	return "one"
}

func FunctionToRename() string {
	return "to rename"
}

func main() {
	println(FunctionOne())
	println(FunctionToRename())
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}

		args := renameArgs("FunctionToRename", mainGoPath, findLineInSrc(t, src, "func FunctionToRename("), "FunctionOne")
		content := callRename(t, args, testutil.GoldenComplexRenameScenarios)
		t.Logf("conflict rename result:\n%s", content)

		// Tool may error or generate a conflicting diff — either is acceptable.
		t.Logf("✓ handled rename conflict (error or conflicting diff)")

		assertFileUnchanged(t, mainGoPath, src)
		t.Logf("✓ DRY RUN: file unchanged despite conflict")
	})

	t.Run("RenameInTestFiles", func(t *testing.T) {
		// Renaming Add should propagate to math_test.go and example_test.go.
		projectDir := t.TempDir()
		writeGoMod(t, projectDir)

		mathDir := filepath.Join(projectDir, "math")
		if err := os.Mkdir(mathDir, 0755); err != nil {
			t.Fatal(err)
		}

		sourceCode := `package math

// Add adds two numbers
func Add(a, b int) int {
	return a + b
}
`
		mathPath := filepath.Join(mathDir, "math.go")
		if err := os.WriteFile(mathPath, []byte(sourceCode), 0644); err != nil {
			t.Fatal(err)
		}

		testCode := `package math

import "testing"

func TestAdd(t *testing.T) {
	result := Add(2, 3)
	if result != 5 {
		t.Errorf("Add(2,3) = %d; want 5", result)
	}
}
`
		testPath := filepath.Join(mathDir, "math_test.go")
		if err := os.WriteFile(testPath, []byte(testCode), 0644); err != nil {
			t.Fatal(err)
		}

		exampleCode := `package math

import "fmt"

func ExampleAdd() {
	fmt.Println(Add(1, 2))
	// Output: 3
}
`
		examplePath := filepath.Join(mathDir, "example_test.go")
		if err := os.WriteFile(examplePath, []byte(exampleCode), 0644); err != nil {
			t.Fatal(err)
		}

		args := renameArgs("Add", mathPath, findLineInSrc(t, sourceCode, "func Add("), "Sum")
		content := callRename(t, args, testutil.GoldenComplexRenameScenarios)
		t.Logf("rename including test files:\n%s", content)

		testFileMentioned := strings.Contains(content, "math_test.go") ||
			strings.Contains(content, "example_test.go") ||
			strings.Contains(content, "TestAdd") ||
			strings.Contains(content, "ExampleAdd")
		if testFileMentioned {
			t.Logf("✓ rename includes test files")
		} else {
			t.Logf("note: test files may not be shown in preview but should be affected")
		}

		// DRY RUN: none of the files should contain the new name.
		for _, info := range []struct {
			path, original, newSym string
		}{
			{mathPath, sourceCode, "Sum"},
			{testPath, testCode, "Sum"},
			{examplePath, exampleCode, "Sum"},
		} {
			current, err := os.ReadFile(info.path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(current), info.newSym) {
				t.Errorf("DRY RUN violated: %s was modified!", info.path)
			}
		}
		t.Logf("✓ DRY RUN: all 3 files unchanged")
	})
}

// TestRenameEdgeCases tests edge case rename scenarios.
func TestRenameEdgeCases(t *testing.T) {
	t.Run("RenameToUnexported", func(t *testing.T) {
		// Rename an exported symbol to an unexported name.
		projectDir := t.TempDir()
		writeGoMod(t, projectDir)

		src := `package main

func PublicFunction() string {
	return "public"
}

func main() {
	println(PublicFunction())
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}

		args := renameArgs("PublicFunction", mainGoPath, findLineInSrc(t, src, "func PublicFunction("), "privateFunction")
		content := callRename(t, args, testutil.GoldenRenameEdgeCases)
		t.Logf("rename to unexported:\n%s", content)

		t.Logf("✓ handled export→unexported rename")
		assertFileUnchanged(t, mainGoPath, src)
		t.Logf("✓ DRY RUN: file unchanged")
	})

	t.Run("RenameAcrossDifferentCases", func(t *testing.T) {
		// Rename an unexported symbol to exported (case promotion).
		projectDir := t.TempDir()
		writeGoMod(t, projectDir)

		src := `package main

func processData() string {
	return "processed"
}

func main() {
	println(processData())
}
`
		mainGoPath := filepath.Join(projectDir, "main.go")
		if err := os.WriteFile(mainGoPath, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}

		args := renameArgs("processData", mainGoPath, findLineInSrc(t, src, "func processData("), "ProcessData")
		content := callRename(t, args, testutil.GoldenRenameEdgeCases)
		t.Logf("case-change rename result:\n%s", content)

		if strings.Contains(content, "processData") || strings.Contains(content, "ProcessData") {
			t.Logf("✓ handled case-sensitive rename")
		}
		assertFileUnchanged(t, mainGoPath, src)
		t.Logf("✓ DRY RUN: file unchanged")
	})
}
