// Package runner executes headless Claude Code sessions and captures results.
package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/xieyuschen/gopls-mcp/benchmark/internal/metric"
	"github.com/xieyuschen/gopls-mcp/benchmark/internal/parser"
	"github.com/xieyuschen/gopls-mcp/benchmark/internal/task"
)

// plainPreamble is prepended to every plain-group task prompt. It prevents
// the agent from wasting time on gopls-mcp tools that may appear in ToolSearch
// results (from a globally installed plugin) but whose MCP server is not running.
const plainPreamble = `NOTE: You do NOT have access to any gopls-mcp semantic tools (go_definition,
go_symbol_references, go_implementation, etc.). Do NOT search for them, do NOT try
to invoke them, and do NOT spawn subagents or subprocesses to start gopls-mcp.
Use only Bash, Read, and Grep to answer the question.`

// goplsPreamble is prepended to every gopls-group task prompt. It tells the
// agent that semantic MCP tools are available, how to call them (deferred two-step
// protocol), and that their results are authoritative.
const goplsPreamble = `IMPORTANT: You have access to gopls-mcp semantic tools via MCP. These are DEFERRED tools.

HOW TO CALL THEM — follow this two-step protocol exactly:
1. Call ToolSearch with query "select:mcp__gopls-mcp__<tool_name>" to load its schema.
2. Then call the tool DIRECTLY as mcp__gopls-mcp__<tool_name> in the SAME conversation turn.

Available tools and when to use them:
  - mcp__gopls-mcp__go_definition        : jump to a symbol's definition
  - mcp__gopls-mcp__go_symbol_references : find every reference to a symbol
  - mcp__gopls-mcp__go_implementation    : find concrete types that implement an interface
  - mcp__gopls-mcp__go_get_call_hierarchy: trace callers and callees of a function
  - mcp__gopls-mcp__go_get_dependency_graph: map package imports
  - mcp__gopls-mcp__go_dryrun_rename_symbol: preview a rename

STRICT RULES — violations waste many minutes:
- Do NOT spawn sub-agents or Agent tool calls to use these tools — call them DIRECTLY yourself.
- Do NOT use Bash/Python subprocesses to invoke the gopls-mcp binary manually.
- Do NOT fall back to grep/bash/read for tasks these tools cover.
- When a gopls-mcp tool returns results, treat them as complete and correct — do NOT re-verify.`

// Mode controls which variant is run.
type Mode string

const (
	ModePlain Mode = "plain"     // no gopls-mcp MCP server
	ModeGopls Mode = "gopls-mcp" // gopls-mcp MCP server + prompt injected
)

// Config holds parameters for a single benchmark run.
type Config struct {
	Mode          Mode
	TaskName      string
	Prompt        string
	WorkDir       string // cwd passed to claude (the target Go project)
	GoplsMCPBin   string // path to gopls-mcp binary (required for ModeGopls)
	GoplsPrompt   string // content of gopls-mcp.prompt to inject (required for ModeGopls)
	Model         string // empty = claude default
	ClaudeBin     string // path to claude CLI; defaults to "claude" in PATH
}

// RunResult is the outcome of a single headless claude invocation.
type RunResult struct {
	Mode        Mode
	TaskName    string
	Duration    time.Duration
	Parsed      *parser.Result
	Metrics     metric.Metrics
	CheckResult task.CheckResult // automated answer quality score
	RawOutput   []byte           // full stdout from claude (stream-json JSONL)
	Err         error
}

// Run executes a single headless claude session and returns the result.
func Run(ctx context.Context, cfg Config) RunResult {
	start := time.Now()
	res := RunResult{Mode: cfg.Mode, TaskName: cfg.TaskName}

	claudeBin := cfg.ClaudeBin
	if claudeBin == "" {
		claudeBin = "claude"
	}

	prompt := cfg.Prompt
	if cfg.Mode == ModeGopls {
		// gopls-mcp tools are deferred: they start as "pending" in the init
		// event and are not in the initial tool list. Prepend an explicit
		// preamble so the agent knows to use the semantic tools rather than
		// falling back immediately to Bash/Read/Grep.
		prompt = goplsPreamble + "\n\n" + prompt
	} else {
		// Plain run: the global gopls-mcp plugin may still advertise tools via
		// ToolSearch, but the MCP server is not running (--strict-mcp-config
		// with empty config). Prepend a counter-prompt so the agent does not
		// waste time attempting to invoke unavailable semantic tools.
		prompt = plainPreamble + "\n\n" + prompt
	}

	args := []string{
		"-p", prompt,
		"--output-format", "stream-json",
		"--verbose",
	}
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}

	// For the gopls run: load MCP server + write a settings file that
	// pre-approves all gopls-mcp tool calls so they don't block on permission prompts.
	// --dangerously-skip-permissions prevents MCP subprocess spawning, so we use
	// --settings with an allow-list instead.
	var mcpConfigPath, settingsPath string
	if cfg.Mode == ModeGopls {
		if cfg.GoplsMCPBin == "" {
			res.Err = fmt.Errorf("GoplsMCPBin is required for ModeGopls")
			return res
		}
		var err error
		mcpConfigPath, err = writeMCPConfig(cfg.GoplsMCPBin)
		if err != nil {
			res.Err = fmt.Errorf("write mcp config: %w", err)
			return res
		}
		defer func() { _ = os.Remove(mcpConfigPath) }()

		settingsPath, err = writeGoplsSettings()
		if err != nil {
			res.Err = fmt.Errorf("write gopls settings: %w", err)
			return res
		}
		defer func() { _ = os.Remove(settingsPath) }()

		args = append(args,
			"--strict-mcp-config",
			"--mcp-config", mcpConfigPath,
			"--settings", settingsPath,
		)
		if cfg.GoplsPrompt != "" {
			args = append(args, "--append-system-prompt", cfg.GoplsPrompt)
		}
	} else {
		// Plain run: disable ALL MCP servers (including any globally configured
		// ones such as gopls-mcp) so the agent can only use built-in tools.
		// --strict-mcp-config with an empty config achieves this.
		emptyMCP, err := writeEmptyMCPConfig()
		if err != nil {
			res.Err = fmt.Errorf("write empty mcp config: %w", err)
			return res
		}
		defer func() { _ = os.Remove(emptyMCP) }()

		plainSettings, err := writePlainSettings()
		if err != nil {
			res.Err = fmt.Errorf("write plain settings: %w", err)
			return res
		}
		defer func() { _ = os.Remove(plainSettings) }()

		args = append(args,
			"--strict-mcp-config",
			"--mcp-config", emptyMCP,
			"--settings", plainSettings,
		)
	}

	cmd := exec.CommandContext(ctx, claudeBin, args...)
	cmd.Dir = cfg.WorkDir
	cmd.Stdin = strings.NewReader("\n") // provide minimal stdin so claude doesn't skip MCP init

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// claude exits non-zero on some errors; preserve output anyway.
		res.Err = fmt.Errorf("claude exited: %w (stderr: %s)", err, stderr.String())
	}
	_ = stderr // captured for debugging; log if needed

	res.Duration = time.Since(start)
	res.RawOutput = stdout.Bytes()

	parsed, parseErr := parser.ParseStream(bytes.NewReader(res.RawOutput))
	if parseErr != nil && res.Err == nil {
		res.Err = fmt.Errorf("parse stream: %w", parseErr)
	}
	if parsed != nil {
		res.Parsed = parsed
		res.Metrics = metric.Compute(parsed)
	}
	return res
}
