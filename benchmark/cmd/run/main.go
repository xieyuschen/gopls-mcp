// Command run executes the gopls-mcp benchmark suite.
//
// Usage:
//
//	go run ./benchmark/cmd/run \
//	  --target /path/to/go/project \
//	  --gopls-mcp-bin $(which gopls-mcp) \
//	  --task all \
//	  --out benchmark/results/$(date +%Y%m%d-%H%M%S)
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/xieyuschen/gopls-mcp/benchmark/internal/report"
	"github.com/xieyuschen/gopls-mcp/benchmark/internal/runner"
	"github.com/xieyuschen/gopls-mcp/benchmark/internal/task"
)

func main() {
	target := flag.String("target", "", "Path to the target Go project (default: repo root)")
	goplsBin := flag.String("gopls-mcp-bin", "", "Path to gopls-mcp binary (required for gopls run)")
	taskFlag := flag.String("task", "all", "Task name to run, or 'all'")
	outDir := flag.String("out", "", "Output directory for results (default: benchmark/results/<timestamp>)")
	model := flag.String("model", "", "Claude model override (default: claude default)")
	claudeBin := flag.String("claude-bin", "claude", "Path to the claude CLI binary")
	promptFile := flag.String("prompt-file", "", "Path to gopls-mcp system prompt file (default: auto-detect gopls-mcp.prompt)")
	flag.Parse()

	// Resolve target repo: default to the directory two levels up from this binary's
	// module (benchmark/ → repo root).
	repoRoot := resolveRepoRoot()
	if *target == "" {
		*target = repoRoot
	}

	// Resolve gopls-mcp binary.
	if *goplsBin == "" {
		log.Fatal("--gopls-mcp-bin is required; build with: go build -o /tmp/gopls-mcp . (from repo root)")
	}

	// Resolve gopls-mcp skill/prompt content to inject via --append-system-prompt.
	// Canonical source is plugin/skills/gopls-mcp/SKILL.md (added in plugin PR).
	// Fall back to the old standalone prompt file if present.
	goplsPrompt := ""
	if *promptFile != "" {
		data, err := os.ReadFile(*promptFile)
		if err != nil {
			log.Fatalf("read prompt file: %v", err)
		}
		goplsPrompt = stripFrontmatter(string(data))
	} else {
		candidates := []string{
			filepath.Join(repoRoot, "plugin", "skills", "gopls-mcp", "SKILL.md"),
			filepath.Join(repoRoot, "gopls", "gopls-mcp.prompt"),
			filepath.Join(repoRoot, "gopls", "CLAUDE.md"),
		}
		for _, c := range candidates {
			data, err := os.ReadFile(c)
			if err == nil {
				goplsPrompt = stripFrontmatter(string(data))
				log.Printf("using gopls-mcp skill from %s", c)
				break
			}
		}
	}

	// Resolve output directory.
	if *outDir == "" {
		*outDir = filepath.Join(repoRoot, "benchmark", "results",
			time.Now().Format("20060102-150405"))
	}

	// Select tasks.
	var tasks []task.Task
	if *taskFlag == "all" {
		tasks = task.All()
	} else {
		for _, name := range strings.Split(*taskFlag, ",") {
			t, ok := task.ByName(strings.TrimSpace(name))
			if !ok {
				log.Fatalf("unknown task: %q; available: %s", name, listTaskNames())
			}
			tasks = append(tasks, t)
		}
	}

	log.Printf("benchmark: %d task(s), target=%s, out=%s", len(tasks), *target, *outDir)

	ctx := context.Background()
	var pairs []report.Pair

	for _, t := range tasks {
		log.Printf("running task %q …", t.Name)

		baseCfg := runner.Config{
			TaskName:    t.Name,
			Prompt:      t.Prompt,
			WorkDir:     *target,
			GoplsMCPBin: *goplsBin,
			GoplsPrompt: goplsPrompt,
			Model:       *model,
			ClaudeBin:   *claudeBin,
		}

		plainCfg := baseCfg
		plainCfg.Mode = runner.ModePlain

		goplsCfg := baseCfg
		goplsCfg.Mode = runner.ModeGopls

		// Run plain and gopls-mcp concurrently — they are fully independent.
		var plainRes, goplsRes runner.RunResult
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); plainRes = runner.Run(ctx, plainCfg) }()
		go func() { defer wg.Done(); goplsRes = runner.Run(ctx, goplsCfg) }()
		wg.Wait()

		// Score answers against ground truth.
		if t.Checker.Total() > 0 {
			if plainRes.Parsed != nil {
				plainRes.CheckResult = t.Checker.Check(plainRes.Parsed.FinalMessage)
			}
			if goplsRes.Parsed != nil {
				goplsRes.CheckResult = t.Checker.Check(goplsRes.Parsed.FinalMessage)
			}
		}

		logResult("plain", plainRes)
		logResult("gopls-mcp", goplsRes)

		pairs = append(pairs, report.Pair{Plain: plainRes, Gopls: goplsRes})
	}

	if err := report.Write(*outDir, pairs); err != nil {
		log.Fatalf("write report: %v", err)
	}
	log.Printf("results written to %s", *outDir)
	fmt.Printf("\nReport: %s/benchmark-report.md\n", *outDir)
	fmt.Printf("Raw:    %s/benchmark-results.jsonl\n", *outDir)
}

func logResult(mode string, r runner.RunResult) {
	if r.Err != nil {
		log.Printf("  [%s] ERROR: %v", mode, r.Err)
		return
	}
	m := r.Metrics
	cr := r.CheckResult
	scoreStr := "n/a"
	if cr.Total > 0 {
		scoreStr = fmt.Sprintf("%d/%d (%.0f%%)", cr.Found, cr.Total, cr.Score*100)
		if len(cr.Missing) > 0 {
			scoreStr += " missing=" + strings.Join(cr.Missing, ",")
		}
	}
	log.Printf("  [%s] %s  calls=%d (gopls=%d grep=%d read=%d bash=%d init=%d)  tokens=%d+%d  cost=$%.4f  score=%s",
		mode, r.Duration.Round(time.Millisecond),
		m.TotalCalls, m.GoplsCalls, m.GrepCalls, m.ReadCalls, m.BashCalls, m.InitCalls,
		m.InputTokens, m.OutputTokens, m.TotalCostUSD, scoreStr)
}

func resolveRepoRoot() string {
	// Walk up from the current working directory until we find a go.mod
	// whose module name does NOT match our own benchmark module.
	dir, _ := os.Getwd()
	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && strings.Contains(string(data), "xieyuschen/gopls-mcp") &&
			!strings.Contains(string(data), "xieyuschen/gopls-mcp/benchmark") {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// fallback: two directories up from this file's expected location
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "..", "..")
}

// stripFrontmatter removes YAML frontmatter (--- ... ---) from skill files.
// Returns the content after the closing --- delimiter, or the full string if
// no frontmatter is found.
func stripFrontmatter(s string) string {
	if !strings.HasPrefix(s, "---") {
		return s
	}
	// Find the closing delimiter starting after the opening ---.
	_, after, found := strings.Cut(s[3:], "\n---")
	if !found {
		return s
	}
	return strings.TrimLeft(after, "\n")
}

func listTaskNames() string {
	names := make([]string, 0, len(task.Suite))
	for _, t := range task.Suite {
		names = append(names, t.Name)
	}
	return strings.Join(names, ", ")
}
