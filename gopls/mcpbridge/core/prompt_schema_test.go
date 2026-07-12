package core

import (
	"io"
	"log"
	"os"
	"strings"
	"testing"
)

func TestPromptToolSchemasMatchRegisteredSchemas(t *testing.T) {
	oldLogOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(oldLogOutput)

	want, err := GeneratePromptToolSchemas(t.Context())
	if err != nil {
		t.Fatalf("generate prompt tool schemas: %v", err)
	}

	for _, path := range []string{
		"../../gopls-mcp.prompt",
		"../../../plugin/skills/gopls-mcp/SKILL.md",
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		got, ok := sectionBetween(string(content), PromptSchemaStartMarker, PromptSchemaEndMarker)
		if !ok {
			t.Fatalf("%s missing schema markers %q and %q", path, PromptSchemaStartMarker, PromptSchemaEndMarker)
		}

		if got != want {
			t.Fatalf("%s tool schema block is out of sync with registered MCP schemas; update the block between %s and %s", path, PromptSchemaStartMarker, PromptSchemaEndMarker)
		}
	}
}

func sectionBetween(s, start, end string) (string, bool) {
	startIdx := strings.Index(s, start)
	if startIdx < 0 {
		return "", false
	}
	startIdx += len(start)
	if strings.HasPrefix(s[startIdx:], "\r\n") {
		startIdx += len("\r\n")
	} else if strings.HasPrefix(s[startIdx:], "\n") {
		startIdx += len("\n")
	}
	endIdx := strings.Index(s[startIdx:], end)
	if endIdx < 0 {
		return "", false
	}
	return s[startIdx : startIdx+endIdx], true
}
