// Package report generates benchmark output in JSONL and Markdown formats.
package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xieyuschen/gopls-mcp/benchmark/internal/metric"
	"github.com/xieyuschen/gopls-mcp/benchmark/internal/runner"
	"github.com/xieyuschen/gopls-mcp/benchmark/internal/task"
)

// TaskRecord is one pair of plain+gopls-mcp results for a single task, persisted as JSONL.
type TaskRecord struct {
	Timestamp string     `json:"timestamp"`
	Task      string     `json:"task"`
	Plain     runSummary `json:"plain"`
	GoplsMCP  runSummary `json:"gopls-mcp"`
}

type runSummary struct {
	DurationMS   int64          `json:"duration_ms"`
	Metrics      metric.Metrics `json:"metrics"`
	Score        float64        `json:"score"`        // fraction of ground-truth items found
	ScoreFound   int            `json:"score_found"`
	ScoreTotal   int            `json:"score_total"`
	Missing      []string       `json:"missing,omitempty"`
	FinalMessage string         `json:"final_message,omitempty"`
	IsError      bool           `json:"is_error"`
	Err          string         `json:"err,omitempty"`
}

func toSummary(r runner.RunResult) runSummary {
	s := runSummary{
		DurationMS: r.Duration.Milliseconds(),
		Metrics:    r.Metrics,
		Score:      r.CheckResult.Score,
		ScoreFound: r.CheckResult.Found,
		ScoreTotal: r.CheckResult.Total,
		Missing:    r.CheckResult.Missing,
	}
	if r.Parsed != nil {
		s.FinalMessage = r.Parsed.FinalMessage
		s.IsError = r.Parsed.IsError
	}
	if r.Err != nil {
		s.Err = r.Err.Error()
		s.IsError = true
	}
	return s
}

// Write saves the benchmark results to outDir:
//   - benchmark-results.jsonl  (one JSON line per task)
//   - benchmark-report.md      (human-readable comparison table)
//   - <task>-plain.jsonl       (raw claude events, plain run)
//   - <task>-gopls-mcp.jsonl   (raw claude events, gopls-mcp run)
func Write(outDir string, pairs []Pair) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	// Write raw events for each run.
	for _, p := range pairs {
		for _, r := range []runner.RunResult{p.Plain, p.Gopls} {
			fname := fmt.Sprintf("%s-%s.jsonl", r.TaskName, string(r.Mode))
			if err := os.WriteFile(filepath.Join(outDir, fname), r.RawOutput, 0o644); err != nil {
				return err
			}
		}
	}

	// Write JSONL summary.
	jsonlPath := filepath.Join(outDir, "benchmark-results.jsonl")
	f, err := os.Create(jsonlPath)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	for _, p := range pairs {
		rec := TaskRecord{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Task:      p.Plain.TaskName,
			Plain:     toSummary(p.Plain),
			GoplsMCP:  toSummary(p.Gopls),
		}
		if err := enc.Encode(rec); err != nil {
			_ = f.Close()
			return err
		}
	}
	if err := f.Close(); err != nil {
		return err
	}

	// Write Markdown report.
	md := buildMarkdown(pairs)
	return os.WriteFile(filepath.Join(outDir, "benchmark-report.md"), []byte(md), 0o644)
}

// Pair holds the plain and gopls-mcp results for one task.
type Pair struct {
	Plain runner.RunResult
	Gopls runner.RunResult
}

func buildMarkdown(pairs []Pair) string {
	var b strings.Builder

	b.WriteString("# gopls-mcp Benchmark Report\n\n")
	fmt.Fprintf(&b, "Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339))

	// Summary table.
	b.WriteString("## Summary\n\n")
	b.WriteString("| Task | Score plain | Score gopls-mcp | Plain calls | gopls-mcp calls | Time (plain) | Time (gopls-mcp) |\n")
	b.WriteString("|------|------------:|----------------:|------------:|----------------:|-------------:|-----------------:|\n")

	for _, p := range pairs {
		plain := p.Plain.Metrics
		gm := p.Gopls.Metrics
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %d | %s | %s |\n",
			p.Plain.TaskName,
			formatScore(p.Plain.CheckResult),
			formatScore(p.Gopls.CheckResult),
			plain.TotalCalls,
			gm.TotalCalls,
			formatDur(p.Plain.Duration),
			formatDur(p.Gopls.Duration),
		)
	}
	b.WriteString("\n")

	// Per-task detail.
	b.WriteString("## Per-Task Detail\n\n")
	for _, p := range pairs {
		writeTaskSection(&b, p)
	}

	return b.String()
}

func writeTaskSection(b *strings.Builder, p Pair) {
	fmt.Fprintf(b, "### %s\n\n", p.Plain.TaskName)

	b.WriteString("**Tool call breakdown**\n\n")
	b.WriteString("| Tool | plain | gopls-mcp |\n")
	b.WriteString("|------|------:|----------:|\n")

	// Merge all tool names from both runs.
	allTools := map[string]bool{}
	for t := range p.Plain.Metrics.ByTool {
		allTools[t] = true
	}
	for t := range p.Gopls.Metrics.ByTool {
		allTools[t] = true
	}
	names := make([]string, 0, len(allTools))
	for t := range allTools {
		names = append(names, t)
	}
	sort.Strings(names)
	for _, t := range names {
		fmt.Fprintf(b, "| `%s` | %d | %d |\n", t,
			p.Plain.Metrics.ByTool[t],
			p.Gopls.Metrics.ByTool[t])
	}

	b.WriteString("\n**Token usage**\n\n")
	b.WriteString("| Metric | plain | gopls-mcp |\n")
	b.WriteString("|--------|------:|----------:|\n")
	writeTokenRow(b, "input_tokens", p.Plain.Metrics.InputTokens, p.Gopls.Metrics.InputTokens)
	writeTokenRow(b, "output_tokens", p.Plain.Metrics.OutputTokens, p.Gopls.Metrics.OutputTokens)
	writeTokenRow(b, "cache_read", p.Plain.Metrics.CacheReadTokens, p.Gopls.Metrics.CacheReadTokens)
	fmt.Fprintf(b, "| total_cost_usd | $%.4f | $%.4f |\n",
		p.Plain.Metrics.TotalCostUSD, p.Gopls.Metrics.TotalCostUSD)

	fmt.Fprintf(b, "\n**Duration**\n\n- plain: %s\n- gopls-mcp: %s\n\n",
		formatDur(p.Plain.Duration), formatDur(p.Gopls.Duration))

	// Answer quality: show both final messages side-by-side for human review.
	b.WriteString("**Answer quality** *(human review required)*\n\n")
	b.WriteString("<details><summary>plain answer</summary>\n\n")
	if p.Plain.Err != nil {
		fmt.Fprintf(b, "> ERROR: %v\n\n", p.Plain.Err)
	} else {
		plainMsg := "(no final message)"
		if p.Plain.Parsed != nil && p.Plain.Parsed.FinalMessage != "" {
			plainMsg = p.Plain.Parsed.FinalMessage
		}
		fmt.Fprintf(b, "```\n%s\n```\n\n", plainMsg)
	}
	b.WriteString("</details>\n\n")

	b.WriteString("<details><summary>gopls-mcp answer</summary>\n\n")
	if p.Gopls.Err != nil {
		fmt.Fprintf(b, "> ERROR: %v\n\n", p.Gopls.Err)
	} else {
		goplsMsg := "(no final message)"
		if p.Gopls.Parsed != nil && p.Gopls.Parsed.FinalMessage != "" {
			goplsMsg = p.Gopls.Parsed.FinalMessage
		}
		fmt.Fprintf(b, "```\n%s\n```\n\n", goplsMsg)
	}
	b.WriteString("</details>\n\n")
}

func writeTokenRow(b *strings.Builder, name string, plain, goplsMCP int) {
	fmt.Fprintf(b, "| %s | %d | %d |\n", name, plain, goplsMCP)
}

func formatScore(cr task.CheckResult) string {
	if cr.Total == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%d/%d (%.0f%%)", cr.Found, cr.Total, cr.Score*100)
}

func formatDur(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
