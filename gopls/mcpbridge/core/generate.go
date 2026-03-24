//go:build ignore

package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"golang.org/x/tools/gopls/mcpbridge/core"
)

const (
	claudePath         = "../../CLAUDE.md"
	autoGenStartMarker = "<!-- Marker: AUTO-GEN-START -->"
	autoGenEndMarker   = "<!-- Marker: AUTO-GEN-END -->"
)

func main() {
	// 1. Generate reference.md
	referencePath := "reference.md"
	refFile, err := os.Create(referencePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating reference.md: %v\n", err)
		os.Exit(1)
	}
	defer refFile.Close()

	if err := core.GenerateReference(refFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating reference: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("reference.md updated successfully")

	// 2. Update CLAUDE.md with tool reference
	if err := updateCLAUDEMD(claudePath); err != nil {
		fmt.Fprintf(os.Stderr, "Error updating CLAUDE.md: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("CLAUDE.md updated successfully")
}

// updateCLAUDEMD updates the CLAUDE.md file by replacing content between the auto-gen markers
func updateCLAUDEMD(path string) error {
	// Read the current CLAUDE.md
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading CLAUDE.md: %w", err)
	}

	// Generate the new tool reference section
	toolRef, err := core.GenerateCLAUDEToolReference()
	if err != nil {
		return fmt.Errorf("generating tool reference: %w", err)
	}

	// Find and replace content between markers
	updated, err := replaceBetweenMarkers(string(content), toolRef)
	if err != nil {
		return fmt.Errorf("replacing content between markers: %w", err)
	}

	// Write back
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		return fmt.Errorf("writing CLAUDE.md: %w", err)
	}

	return nil
}

// replaceBetweenMarkers replaces content between AUTO-GEN-START and AUTO-GEN-END markers
func replaceBetweenMarkers(content, newContent string) (string, error) {
	startIdx := strings.Index(content, autoGenStartMarker)
	if startIdx == -1 {
		return "", fmt.Errorf("marker %q not found in file", autoGenStartMarker)
	}

	endIdx := strings.Index(content, autoGenEndMarker)
	if endIdx == -1 {
		return "", fmt.Errorf("marker %q not found in file", autoGenEndMarker)
	}

	if startIdx >= endIdx {
		return "", fmt.Errorf("start marker appears after end marker")
	}

	// Include the markers themselves in the replacement
	var buf bytes.Buffer
	buf.WriteString(content[:startIdx])
	buf.WriteString(autoGenStartMarker)
	buf.WriteString("\n")
	buf.WriteString(newContent)
	buf.WriteString("\n")
	buf.WriteString(autoGenEndMarker)
	buf.WriteString(content[endIdx+len(autoGenEndMarker):])

	return buf.String(), nil
}
