package integration

// End-to-end test for file watching functionality.
// Verifies that when files are modified externally, gopls-mcp's file watcher
// detects the changes and invalidates the gopls cache, so subsequent tool
// calls see the updated content.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/mcpbridge/test/testutil"
)

// TestFileWatchingE2E simulates the real-world Claude Code workflow:
//  1. User has a Go project with gopls-mcp running.
//  2. Claude writes a new function to disk.
//  3. The watcher should detect the change and invalidate the cache so
//     subsequent tool calls see the new symbol.
func TestFileWatchingE2E(t *testing.T) {
	projectDir := testutil.CopyProjectTo(t, "simple")
	logFile := filepath.Join(projectDir, "gopls-mcp.log")

	mcpSession, ctx, _ := testutil.StartMCPServerWithLogfile(t, projectDir, logFile)

	// Trigger gopls session and watcher initialization with an initial tool call.
	// (Resources are lazy-initialized on first call, so the watcher only starts now.)
	mainGoPath := filepath.Join(projectDir, "main.go")
	if _, err := mcpSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "go_definition",
		Arguments: map[string]any{
			"locator": map[string]any{
				"symbol_name":  "Hello",
				"context_file": mainGoPath,
			},
		},
	}); err != nil {
		t.Fatalf("Failed to warm up gopls-mcp: %v", err)
	}

	// Watcher is initialized now; give fsnotify a moment to settle.
	time.Sleep(500 * time.Millisecond)

	originalContent, err := os.ReadFile(mainGoPath)
	if err != nil {
		t.Fatal(err)
	}

	// Append a new function to main.go (simulating an editor writing to disk).
	newContent := string(originalContent)
	lastBraceIndex := strings.LastIndex(newContent, "}")
	if lastBraceIndex == -1 {
		t.Fatal("Could not find closing brace in main.go")
	}
	newContent = newContent[:lastBraceIndex+1] +
		"\n\n// Goodbye returns a farewell message\nfunc Goodbye() string {\n\treturn \"goodbye world\"\n}"

	if err := os.WriteFile(mainGoPath, []byte(newContent), 0644); err != nil {
		t.Fatal(err)
	}

	// The watcher batches events with 500ms delay; metadata reload takes more.
	time.Sleep(2 * time.Second)

	logContent, _ := os.ReadFile(logFile)
	logText := string(logContent)

	if !strings.Contains(logText, "[gopls-mcp/watcher] Detected") {
		t.Error("watcher did not detect file change")
	}
	if !strings.Contains(logText, "[gopls-mcp/watcher] Cache invalidated") {
		t.Error("cache was not invalidated")
	}

	// Ask gopls-mcp to find the definition of the newly-added function.
	// If the watcher correctly invalidated the cache, gopls now knows about Goodbye.
	tool := "go_definition"
	args := map[string]any{
		"locator": map[string]any{
			"symbol_name":  "Goodbye",
			"context_file": mainGoPath,
		},
	}

	res, err := mcpSession.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("Failed to call %s: %v", tool, err)
	}

	content := testutil.ResultText(t, res, testutil.GoldenFileWatching)
	if !strings.Contains(content, "main.go") || !strings.Contains(content, "Definition found") {
		t.Errorf("gopls-mcp did not pick up the new function. Output:\n%s\nLogs:\n%s",
			content, logText)
	}
}
