package core

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

const (
	autoGenStartMarker = "<!-- Marker: AUTO-GEN-START -->"
	autoGenEndMarker   = "<!-- Marker: AUTO-GEN-END -->"
)

// TestGenerate ensures that the generated reference.md is up to date.
// It generates the complete file content to a buffer and compares it with the actual file.
func TestGenerate(t *testing.T) {
	// Read the actual reference.md file
	referencePath := "reference.md"
	actualContent, err := os.ReadFile(referencePath)
	if err != nil {
		t.Fatalf("Failed to read reference.md: %v", err)
	}

	// Generate the complete file content to a buffer
	var buf bytes.Buffer
	if err := GenerateReference(&buf); err != nil {
		t.Fatalf("GenerateReference failed: %v", err)
	}

	generatedContent := buf.Bytes()

	// Compare generated content with actual file content
	if string(generatedContent) != string(actualContent) {
		t.Errorf("reference.md is out of sync with generated content.\n"+
			"Run 'go generate ./gopls/mcpbridge/core' to update.\n\n"+
			"Generated length: %d, Actual length: %d", len(generatedContent), len(actualContent))
	}

	t.Logf("reference.md is up to date with %d tools", len(tools))
}

// TestGenerateCLAUDEToolReference ensures that the CLAUDE.md tool reference generator works.
func TestGenerateCLAUDEToolReference(t *testing.T) {
	content, err := GenerateCLAUDEToolReference()
	if err != nil {
		t.Fatalf("GenerateCLAUDEToolReference failed: %v", err)
	}

	// Check that content is not empty
	if content == "" {
		t.Fatal("Generated content is empty")
	}

	// Check that it contains expected sections (table format)
	expectedSections := []string{
		"### Semantic tools (Exclusive - no grep/Read fallback)",
		"| Task | Tool |",
	}

	for _, section := range expectedSections {
		if !strings.Contains(content, section) {
			t.Errorf("Generated content missing section: %s", section)
		}
	}

	// Check that it references the surviving semantic tools
	knownTools := []string{
		"go_definition",
		"go_implementation",
		"go_symbol_references",
		"go_get_call_hierarchy",
		"go_get_dependency_graph",
		"go_dryrun_rename_symbol",
	}

	for _, tool := range knownTools {
		if !strings.Contains(content, tool) {
			t.Errorf("Generated content missing tool: %s", tool)
		}
	}

	t.Logf("CLAUDE.md tool reference generated successfully (%d bytes)", len(content))
}

// TestCLAUDEMDMarkers ensures that CLAUDE.md has the auto-generation markers.
func TestCLAUDEMDMarkers(t *testing.T) {
	claudePath := "../../CLAUDE.md"
	content, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("Failed to read CLAUDE.md: %v", err)
	}

	contentStr := string(content)

	// Check for start marker
	if !strings.Contains(contentStr, autoGenStartMarker) {
		t.Error("CLAUDE.md missing start marker: " + autoGenStartMarker)
	}

	// Check for end marker
	if !strings.Contains(contentStr, autoGenEndMarker) {
		t.Error("CLAUDE.md missing end marker: " + autoGenEndMarker)
	}

	// Check that start appears before end
	startIdx := strings.Index(contentStr, autoGenStartMarker)
	endIdx := strings.Index(contentStr, autoGenEndMarker)

	if startIdx == -1 || endIdx == -1 {
		return // Already reported above
	}

	if startIdx >= endIdx {
		t.Error("Start marker appears after end marker in CLAUDE.md")
	}

	t.Log("CLAUDE.md markers are present and in correct order")
}
