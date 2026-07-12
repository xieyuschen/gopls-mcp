//go:build ignore

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"golang.org/x/tools/gopls/mcpbridge/core"
)

const (
	claudePath         = "../../CLAUDE.md"
	autoGenStartMarker = "<!-- Marker: AUTO-GEN-START -->"
	autoGenEndMarker   = "<!-- Marker: AUTO-GEN-END -->"
)

var promptSchemaPaths = []string{
	"../../gopls-mcp.prompt",
	"../../../plugin/skills/gopls-mcp/SKILL.md",
}

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

	// 3. Update prompt-embedded compact tool schemas.
	if err := updatePromptSchemas(context.Background(), promptSchemaPaths); err != nil {
		fmt.Fprintf(os.Stderr, "Error updating prompt schemas: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("prompt schemas updated successfully")
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
	updated, err := replaceBetweenMarkers(string(content), toolRef, autoGenStartMarker, autoGenEndMarker)
	if err != nil {
		return fmt.Errorf("replacing content between markers: %w", err)
	}

	// Write back
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		return fmt.Errorf("writing CLAUDE.md: %w", err)
	}

	return nil
}

// updatePromptSchemas updates every prompt surface that embeds the compact MCP
// tool schema block.
func updatePromptSchemas(ctx context.Context, paths []string) error {
	oldLogOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(oldLogOutput)

	toolSchemas, err := core.GeneratePromptToolSchemas(ctx)
	if err != nil {
		return fmt.Errorf("generating prompt tool schemas: %w", err)
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		updated, err := replaceBetweenMarkers(
			string(content),
			toolSchemas,
			core.PromptSchemaStartMarker,
			core.PromptSchemaEndMarker,
		)
		if err != nil {
			return fmt.Errorf("replacing schema block in %s: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

// replaceBetweenMarkers replaces content between marker pairs, preserving the
// markers themselves.
func replaceBetweenMarkers(content, newContent, startMarker, endMarker string) (string, error) {
	startIdx := strings.Index(content, startMarker)
	if startIdx == -1 {
		return "", fmt.Errorf("marker %q not found in file", startMarker)
	}

	endIdx := strings.Index(content, endMarker)
	if endIdx == -1 {
		return "", fmt.Errorf("marker %q not found in file", endMarker)
	}

	if startIdx >= endIdx {
		return "", fmt.Errorf("start marker appears after end marker")
	}

	// Include the markers themselves in the replacement
	var buf bytes.Buffer
	buf.WriteString(content[:startIdx])
	buf.WriteString(startMarker)
	buf.WriteString("\n")
	buf.WriteString(newContent)
	if !strings.HasSuffix(newContent, "\n") {
		buf.WriteString("\n")
	}
	buf.WriteString(endMarker)
	buf.WriteString(content[endIdx+len(endMarker):])

	return buf.String(), nil
}
