// Package metric computes benchmark metrics from a parsed run result.
package metric

import (
	"strings"

	"github.com/xieyuschen/gopls-mcp/benchmark/internal/parser"
)

// GoplsTools is the set of tool names provided by gopls-mcp.
var GoplsTools = map[string]bool{
	"go_definition":           true,
	"go_implementation":       true,
	"go_symbol_references":    true,
	"go_get_call_hierarchy":   true,
	"go_get_dependency_graph": true,
	"go_dryrun_rename_symbol": true,
	"go_list_tools":           true,
	// also match mcp-prefixed variants like mcp__gopls-mcp__go_definition
}

// Metrics holds the computed comparison metrics for one run.
type Metrics struct {
	// Tool call counts by bucket.
	GoplsCalls  int `json:"gopls_calls"`
	GrepCalls   int `json:"grep_calls"`
	ReadCalls   int `json:"read_calls"`
	BashCalls   int `json:"bash_calls"`
	InitCalls   int `json:"init_calls"`  // ToolSearch — one-time schema loading, not work
	OtherCalls  int `json:"other_calls"`
	TotalCalls  int `json:"total_calls"`

	// Per-tool breakdown: tool name → call count.
	ByTool map[string]int `json:"by_tool"`

	// Token counts.
	InputTokens         int     `json:"input_tokens"`
	OutputTokens        int     `json:"output_tokens"`
	CacheReadTokens     int     `json:"cache_read_tokens"`
	CacheCreationTokens int     `json:"cache_creation_tokens"`
	TotalCostUSD        float64 `json:"total_cost_usd"`
}

// Compute derives Metrics from a parsed stream result.
func Compute(r *parser.Result) Metrics {
	m := Metrics{
		ByTool:              make(map[string]int),
		InputTokens:         r.Usage.InputTokens,
		OutputTokens:        r.Usage.OutputTokens,
		CacheReadTokens:     r.Usage.CacheReadTokens,
		CacheCreationTokens: r.Usage.CacheCreationTokens,
		TotalCostUSD:        r.Usage.TotalCostUSD,
	}

	for _, tc := range r.ToolCalls {
		bare := bareName(tc.Name)
		m.ByTool[bare]++
		m.TotalCalls++

		switch {
		case isGopls(tc.Name):
			m.GoplsCalls++
		case bare == "Grep" || bare == "grep":
			m.GrepCalls++
		case bare == "Read":
			m.ReadCalls++
		case bare == "Bash":
			m.BashCalls++
		case bare == "ToolSearch":
			m.InitCalls++ // schema-loading overhead, not semantic work
		default:
			m.OtherCalls++
		}
	}
	return m
}

// bareName strips MCP server prefixes (mcp__<server>__<tool>) returning just
// the tool name.
func bareName(name string) string {
	if idx := strings.LastIndex(name, "__"); idx >= 0 {
		return name[idx+2:]
	}
	return name
}

func isGopls(name string) bool {
	bare := bareName(name)
	if GoplsTools[bare] {
		return true
	}
	// match mcp__gopls-mcp__go_* variants before stripping
	return strings.Contains(name, "gopls") && strings.HasPrefix(bare, "go_")
}
