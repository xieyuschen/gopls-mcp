package runner

import (
	"encoding/json"
	"os"
)

// goplsToolNames lists every tool exposed by gopls-mcp, using the
// mcp__<server-name>__<tool> naming convention that Claude Code uses.
var goplsToolNames = []string{
	"mcp__gopls-mcp__go_definition",
	"mcp__gopls-mcp__go_implementation",
	"mcp__gopls-mcp__go_symbol_references",
	"mcp__gopls-mcp__go_get_call_hierarchy",
	"mcp__gopls-mcp__go_get_dependency_graph",
	"mcp__gopls-mcp__go_dryrun_rename_symbol",
	"mcp__gopls-mcp__go_list_tools",
}

type mcpServerConfig struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type mcpConfig struct {
	McpServers map[string]mcpServerConfig `json:"mcpServers"`
}

// writeGoplsSettings writes a temporary Claude Code settings JSON file that
// pre-approves all gopls-mcp tool calls, avoiding interactive permission prompts
// while still allowing MCP subprocess spawning (unlike --dangerously-skip-permissions).
func writeGoplsSettings() (string, error) {
	type permissions struct {
		Allow []string `json:"allow"`
	}
	type settings struct {
		Permissions permissions `json:"permissions"`
	}
	// Also allow the standard read/navigation tools the agent will reach for.
	// ToolSearch is the first step of the deferred-tool protocol (loads schemas
	// for mcp__gopls-mcp__* tools) and must be pre-approved so it never prompts.
	allow := append(goplsToolNames,
		"Bash", "Read", "Grep", "Glob", "LS",
		"mcp__gopls-mcp__*",
		"ToolSearch",
	)
	s := settings{Permissions: permissions{Allow: allow}}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp("", "gopls-mcp-bench-settings-*.json")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), f.Close()
}

// writeEmptyMCPConfig writes an MCP config with no servers, used with
// --strict-mcp-config to disable all global MCP servers in the plain run.
func writeEmptyMCPConfig() (string, error) {
	f, err := os.CreateTemp("", "gopls-mcp-bench-empty-mcp-*.json")
	if err != nil {
		return "", err
	}
	if _, err := f.Write([]byte(`{"mcpServers":{}}`)); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), f.Close()
}

// writePlainSettings writes settings that pre-approve standard navigation tools
// for the plain (no-MCP) run so Read/Bash/Grep calls don't block.
func writePlainSettings() (string, error) {
	type permissions struct {
		Allow []string `json:"allow"`
	}
	type settings struct {
		Permissions permissions `json:"permissions"`
	}
	s := settings{Permissions: permissions{Allow: []string{
		"Bash", "Read", "Grep", "Glob", "LS",
	}}}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp("", "gopls-mcp-bench-plain-settings-*.json")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), f.Close()
}

// writeMCPConfig writes a temporary MCP config file pointing at goplsMCPBin
// and returns the file path. The caller is responsible for removing the file.
func writeMCPConfig(goplsMCPBin string) (string, error) {
	cfg := mcpConfig{
		McpServers: map[string]mcpServerConfig{
			"gopls-mcp": {
				Type:    "stdio",
				Command: goplsMCPBin,
				Args:    []string{},
			},
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp("", "gopls-mcp-bench-*.json")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), f.Close()
}
