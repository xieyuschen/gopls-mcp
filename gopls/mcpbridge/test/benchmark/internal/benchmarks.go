package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// extractTextContent extracts text from an MCP tool result.
func extractTextContent(res *mcp.CallToolResult) string {
	var text strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	return text.String()
}

// validateContent checks if content is non-empty and not an error message.
// Returns (isValid, isEmpty, isError).
func validateContent(content string) (isValid bool, isEmpty bool, isError bool) {
	if content == "" {
		return false, true, false
	}

	trimmed := strings.TrimSpace(content)

	// Check for error indicators
	errorIndicators := []string{
		"error:", "Error:", "ERROR:",
		"failed:", "Failed:", "FAILED:",
		"unable to", "Unable to", "UNABLE TO",
		"not found", "Not found", "NOT FOUND",
	}

	lowerContent := strings.ToLower(trimmed)
	for _, indicator := range errorIndicators {
		if strings.Contains(lowerContent, strings.ToLower(indicator)) {
			return false, false, true
		}
	}

	// Check if content is suspiciously short (< 100 bytes for package API)
	if len(trimmed) < 100 {
		return false, false, false
	}

	return true, false, false
}

// CountItemsFromJSON attempts to parse JSON output and count actual items.
// If parsing fails, falls back to simple line counting.
func CountItemsFromJSON(content string) int {
	// Try to parse as JSON array
	var jsonArray []interface{}
	if err := json.Unmarshal([]byte(content), &jsonArray); err == nil {
		return len(jsonArray)
	}

	// Try to parse as JSON object with "items" or "results" field
	var jsonObject map[string]interface{}
	if err := json.Unmarshal([]byte(content), &jsonObject); err == nil {
		// Look for common item fields
		for _, key := range []string{"items", "results", "symbols", "packages", "functions"} {
			if items, ok := jsonObject[key].([]interface{}); ok {
				return len(items)
			}
		}
	}

	// Fallback: count lines (better than substring counting)
	if content != "" {
		return len(strings.Split(strings.TrimSpace(content), "\n"))
	}

	return 0
}

// FindSymbolLocation searches for a symbol in the codebase and returns its location.
// This avoids hardcoded line numbers that break when files change.
func FindSymbolLocation(projectDir, symbolName, symbolType string) (filePath string, line int, err error) {
	// Use grep to find the symbol definition
	var grepPattern string
	switch symbolType {
	case "type":
		grepPattern = fmt.Sprintf("^type %s ", symbolName)
	case "func":
		grepPattern = fmt.Sprintf("^func .+ %s\\(", symbolName)
	default:
		grepPattern = symbolName
	}

	cmd := exec.Command("grep", "-r", "-n", grepPattern, projectDir)
	output, err := cmd.Output()
	if err != nil {
		return "", 0, fmt.Errorf("grep failed: %w", err)
	}

	// Parse grep output: "path/to/file.go:123:    type SymbolName struct"
	// OR: "path/to/file.go:type SymbolName struct" (if on first line)
	lines := strings.Split(string(output), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", 0, fmt.Errorf("symbol %s not found", symbolName)
	}

	// Use first match
	firstMatch := lines[0]

	// Try parsing with line number first: "file:line:content"
	parts := strings.SplitN(firstMatch, ":", 3)
	if len(parts) >= 3 {
		filePath = parts[0]
		lineStr := parts[1]
		// Check if lineStr is a number
		if lineNum, err := strconv.Atoi(strings.TrimSpace(lineStr)); err == nil {
			line = lineNum
			return filePath, line, nil
		}
	}

	// Fallback: format is "file:content" (no line number)
	// Try extracting line number from the grep -n output separately
	cmd = exec.Command("grep", "-r", "-n", grepPattern, projectDir)
	output, err = cmd.Output()
	if err != nil {
		return "", 0, fmt.Errorf("grep -n failed: %w", err)
	}

	// Parse with different strategy: look for first line that has ":\d+:" pattern
	lines = strings.Split(string(output), "\n")
	for _, lineOutput := range lines {
		if lineOutput == "" {
			continue
		}
		// Match pattern: file.go:line:content
		re := regexp.MustCompile(`^([^:]+):(\d+):`)
		matches := re.FindStringSubmatch(lineOutput)
		if len(matches) >= 3 {
			filePath = matches[1]
			lineNum, err := strconv.Atoi(matches[2])
			if err == nil {
				return filePath, lineNum, nil
			}
		}
	}

	return "", 0, fmt.Errorf("could not parse grep output for symbol %s, output: %s", symbolName, firstMatch)
}

// ProjectInfo contains statistics about the project being benchmarked.
type ProjectInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Packages int    `json:"packages"`
	Files    int    `json:"files"`
	LOC      int    `json:"lines_of_code"`
}

// BenchmarkResult represents a single benchmark result with statistics.
type BenchmarkResult struct {
	Name            string
	Category        string
	Duration        time.Duration // Mean duration
	MinDuration     time.Duration // Minimum duration across iterations
	MaxDuration     time.Duration // Maximum duration across iterations
	StdDev          time.Duration // Standard deviation
	Iterations      int           // Number of iterations run
	Success         bool
	Error           string
	ItemsFound      int
	BytesProcessed  int
	ComparisonNote  string
	SpeedupFactor   float64
	TraditionalMean time.Duration  // Mean traditional tool duration (for comparisons)
	TraditionalMin  time.Duration  // Min traditional tool duration
	TraditionalMax  time.Duration  // Max traditional tool duration
	Memory          *MemoryMetrics // Memory usage after benchmark (optional)
}

// ColdStartMetrics holds cold start analysis data.
type ColdStartMetrics struct {
	ServerStartupTime    time.Duration // Time from process start to first successful query
	FirstQueryTime       time.Duration // Time for the first query after server is ready
	AverageWarmQueryTime time.Duration // Average query time after warmup
	BreakEvenOperations  int           // Number of operations after which MCP is faster
	BreakEvenReason      string        // Explanation of the break-even calculation
}

// MemoryMetrics holds memory usage statistics.
type MemoryMetrics struct {
	HeapAlloc    uint64 // Current heap allocation in bytes
	HeapSys      uint64 // Heap memory obtained from system in bytes
	HeapInUse    uint64 // Heap in-use by application
	StackInUse   uint64 // Stack memory in use
	MSpanInUse   uint64 // MSpan structures in use
	MCacheInUse  uint64 // MCache structures in use
	Goroutines   int    // Number of goroutines
	NextGC       uint64 // Next GC target heap size
	LastGCTime   string // Time of last garbage collection
	NumGC        uint32 // Number of garbage collections
	PauseTotalNs uint64 // Total GC pause time in nanoseconds
	ProcessRSS   uint64 // Process resident set size (via ps) in bytes
	ProcessVSz   uint64 // Process virtual memory size (via ps) in bytes
}

// GetMemoryMetrics collects memory usage statistics from the current Go runtime.
func GetMemoryMetrics() *MemoryMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics := &MemoryMetrics{
		HeapAlloc:    m.HeapAlloc,
		HeapSys:      m.HeapSys,
		HeapInUse:    m.HeapInuse,
		StackInUse:   m.StackInuse,
		MSpanInUse:   m.MSpanInuse,
		MCacheInUse:  m.MCacheInuse,
		Goroutines:   runtime.NumGoroutine(),
		NextGC:       m.NextGC,
		LastGCTime:   time.Unix(0, int64(m.LastGC)).Format(time.RFC3339),
		NumGC:        m.NumGC,
		PauseTotalNs: m.PauseTotalNs,
	}

	return metrics
}

// GetProcessMemoryStats retrieves memory stats for a process ID using ps.
// Returns RSS (resident set size) and VSz (virtual size) in bytes.
//
// IMPORTANT: We only care about RSS (actual physical memory in use).
// VSz is included for completeness but not displayed, as it includes
// shared libraries and memory mappings which can be misleading.
//
// Platform support:
// - macOS/BSD: ps -o rss,vsz -p <PID> (in KB)
// - Linux: ps -o rss,vsz -p <PID> (in KB)
// - WSL: Same as Linux (uses Linux kernel)
func GetProcessMemoryStats(pid int) (rss, vsize uint64, err error) {
	// This command format works on macOS, Linux, and WSL
	// rss = resident set size (physical memory in use) ← **WE CARE ABOUT THIS**
	// vsz = virtual memory size (total virtual memory) ← misleading, not displayed
	cmd := exec.Command("ps", "-o", "rss,vsz", "-p", strconv.Itoa(pid))
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to run ps command: %w", err)
	}

	// Parse ps output:
	//   RSS      VSz  (header)
	// 63760  402024  (data, in KB)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return 0, 0, fmt.Errorf("unexpected ps output format: expected at least 2 lines, got %d", len(lines))
	}

	// Parse the second line (first line is header)
	// Use strings.Fields to handle variable whitespace between columns
	fields := strings.Fields(lines[1])
	if len(fields) < 2 {
		return 0, 0, fmt.Errorf("unexpected ps output format: expected at least 2 fields, got %d", len(fields))
	}

	// ps reports in KB on all platforms
	rssKB, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse RSS value '%s': %w", fields[0], err)
	}

	vsizeKB, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse VSz value '%s': %w", fields[1], err)
	}

	// Convert KB to bytes (VSz stored in JSON for completeness, but not displayed)
	return rssKB * 1024, vsizeKB * 1024, nil
}

// FormatBytes formats a byte count as a human-readable string.
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ===================================================================
// Table-Driven Benchmark Infrastructure
// ===================================================================

// simpleBenchmarkCase defines a simple benchmark that calls a single MCP tool.
// For complex benchmarks with comparisons or custom logic, use standalone functions.
type simpleBenchmarkCase struct {
	name       string
	category   string
	tool       string
	args       func(projectDir string) map[string]any // Dynamic args based on projectDir
	validate   func(content string) (items int, bytes int, err error)
	skip       bool
	skipReason string
}

// runSimpleBenchmark executes a single simple benchmark case.
func runSimpleBenchmark(ctx context.Context, session *mcp.ClientSession, projectDir string, tc simpleBenchmarkCase) BenchmarkResult {
	if tc.skip {
		return BenchmarkResult{
			Name:     tc.name,
			Category: tc.category,
			Success:  false,
			Error:    fmt.Sprintf("Skipped: %s", tc.skipReason),
		}
	}

	start := time.Now()
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      tc.tool,
		Arguments: tc.args(projectDir),
	})
	duration := time.Since(start)

	if err != nil {
		return BenchmarkResult{
			Name:     tc.name,
			Category: tc.category,
			Success:  false,
			Error:    err.Error(),
		}
	}

	content := extractTextContent(res)
	items, bytes, err := tc.validate(content)
	if err != nil {
		return BenchmarkResult{
			Name:     tc.name,
			Category: tc.category,
			Success:  false,
			Error:    err.Error(),
		}
	}

	return BenchmarkResult{
		Name:           tc.name,
		Category:       tc.category,
		Duration:       duration,
		Success:        true,
		ItemsFound:     items,
		BytesProcessed: bytes,
		ComparisonNote: fmt.Sprintf("MCP: %v", duration),
		SpeedupFactor:  0,
	}
}

// ===================================================================
// Category 1: Package Discovery Speed
// ===================================================================

func BenchmarkPackageDiscovery(ctx context.Context, session *mcp.ClientSession, projectDir string) []BenchmarkResult {
	cases := []simpleBenchmarkCase{
		{
			name:     "List Project Packages",
			category: "Package Discovery",
			tool:     "go_list_module_packages",
			args: func(projectDir string) map[string]any {
				return map[string]any{
					"Cwd":              projectDir,
					"include_docs":     false, // Faster - we don't need docs for counting packages
					"exclude_tests":    false, // Include all packages for fair comparison with go list
					"exclude_internal": false,
					"top_level_only":   false,
				}
			},
			validate: func(content string) (int, int, error) {
				packages := strings.Count(content, "\n  ") + strings.Count(content, "\n ")
				return packages, len(content), nil
			},
		},
		{
			name:     "Get Package API with Bodies",
			category: "Package Discovery",
			tool:     "go_list_package_symbols",
			args: func(projectDir string) map[string]any {
				return map[string]any{
					"package_path":   "golang.org/x/tools/gopls/internal/golang",
					"include_bodies": true,
					"include_docs":   false,
				}
			},
			validate: func(content string) (int, int, error) {
				isValid, isEmpty, isError := validateContent(content)
				if !isValid {
					return 0, 0, fmt.Errorf("Invalid content: isEmpty=%v, isError=%v", isEmpty, isError)
				}
				if len(content) < 1024 {
					return 0, 0, fmt.Errorf("Suspiciously small response: %d bytes (expected >1KB)", len(content))
				}
				return 0, len(content), nil
			},
		},
		{
			name:     "Get Package API Signatures Only",
			category: "Package Discovery",
			tool:     "go_list_package_symbols",
			args: func(projectDir string) map[string]any {
				return map[string]any{
					"package_path":   "golang.org/x/tools/gopls/internal/golang",
					"include_bodies": false,
					"include_docs":   false,
				}
			},
			validate: func(content string) (int, int, error) {
				isValid, isEmpty, isError := validateContent(content)
				if !isValid {
					return 0, 0, fmt.Errorf("Invalid content: isEmpty=%v, isError=%v", isEmpty, isError)
				}
				if len(content) < 100 {
					return 0, 0, fmt.Errorf("Suspiciously small response: %d bytes (expected >100 bytes)", len(content))
				}
				return 0, len(content), nil
			},
		},
	}

	var results []BenchmarkResult
	for _, tc := range cases {
		result := runSimpleBenchmark(ctx, session, projectDir, tc)
		// Add more detailed comparison notes
		if result.Success {
			if tc.name == "List Project Packages" {
				result.ComparisonNote = fmt.Sprintf("MCP: %v | Use Comparison benchmark for measured speedup", result.Duration)
			} else if strings.Contains(tc.name, "Bodies") {
				result.ComparisonNote = fmt.Sprintf("MCP: %v | With bodies - for learning patterns (%d bytes)", result.Duration, result.BytesProcessed)
			} else {
				result.ComparisonNote = fmt.Sprintf("MCP: %v | Signatures only - fast API exploration (%d bytes)", result.Duration, result.BytesProcessed)
			}
		}
		results = append(results, result)
	}

	return results
}

// ===================================================================
// Category 2: Symbol Search Accuracy
// ===================================================================

func BenchmarkSymbolSearch(ctx context.Context, session *mcp.ClientSession, projectDir string) []BenchmarkResult {
	cases := []simpleBenchmarkCase{
		{
			name:     "Symbol Search (Fuzzy)",
			category: "Symbol Search",
			tool:     "go_search",
			args: func(projectDir string) map[string]any {
				return map[string]any{
					"query": "Implementation",
				}
			},
			validate: func(content string) (int, int, error) {
				symbolsFound := CountItemsFromJSON(content)
				return symbolsFound, len(content), nil
			},
		},
	}

	var results []BenchmarkResult
	for _, tc := range cases {
		result := runSimpleBenchmark(ctx, session, projectDir, tc)
		if result.Success {
			result.ComparisonNote = fmt.Sprintf("MCP: %v | See Comparison: grep -r for measured speedup (%d symbols)", result.Duration, result.ItemsFound)
		}
		results = append(results, result)
	}

	// Add complex comparison benchmark (keep as-is)
	results = append(results, benchmarkStdlibSearch(ctx, session))

	return results
}

func benchmarkStdlibSearch(ctx context.Context, session *mcp.ClientSession) BenchmarkResult {
	name := "Stdlib Package API"
	category := "Symbol Search"

	// Compare: go doc fmt (traditional) vs list_package_symbols fmt (MCP)
	// This measures getting structured information about a stdlib package
	config := DefaultBenchmarkConfig()
	config.MinIterations = 15
	config.MaxIterations = 40
	config.TargetCV = 15.0

	// Traditional method: go doc fmt
	traditionalStats := RunBenchmarkWithConfig(func() time.Duration {
		start := time.Now()
		cmd := exec.Command("go", "doc", "fmt")
		_, _ = cmd.CombinedOutput()
		return time.Since(start)
	}, config)

	// MCP method: list_package_symbols for stdlib package
	mcpStats := RunBenchmarkWithConfig(func() time.Duration {
		start := time.Now()
		_, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "go_list_package_symbols",
			Arguments: map[string]any{
				"package_path":   "fmt",
				"include_bodies": false,
				"include_docs":   false,
			},
		})
		if err != nil {
			return 0
		}
		return time.Since(start)
	}, config)

	// Check if all iterations succeeded
	if mcpStats.Mean == 0 {
		return BenchmarkResult{
			Name:     name,
			Category: category,
			Success:  false,
			Error:    "MCP tool failed",
		}
	}

	// Calculate speedup
	speedupFactor := float64(traditionalStats.Mean) / float64(mcpStats.Mean)
	actualIterations := len(traditionalStats.Values)

	return BenchmarkResult{
		Name:        name,
		Category:    category,
		Duration:    mcpStats.Mean,
		MinDuration: mcpStats.Min,
		MaxDuration: mcpStats.Max,
		StdDev:      mcpStats.StdDev,
		Iterations:  actualIterations,
		Success:     true,
		ItemsFound:  1, // One package
		ComparisonNote: fmt.Sprintf("Traditional (go doc fmt): %v (±%v), MCP: %v (±%v, structured)",
			traditionalStats.Mean,
			traditionalStats.StdDev,
			mcpStats.Mean,
			mcpStats.StdDev),
		SpeedupFactor:   speedupFactor,
		TraditionalMean: traditionalStats.Mean,
		TraditionalMin:  traditionalStats.Min,
		TraditionalMax:  traditionalStats.Max,
	}
}

// ===================================================================
// Category 3: Function Bodies Retrieval
// ===================================================================

func BenchmarkFunctionBodies(ctx context.Context, session *mcp.ClientSession, projectDir string) []BenchmarkResult {
	cases := []simpleBenchmarkCase{
		{
			name:     "Function Bodies with Bodies (Learning Mode)",
			category: "Function Bodies",
			tool:     "go_list_package_symbols",
			args: func(projectDir string) map[string]any {
				return map[string]any{
					"package_path":   "golang.org/x/tools/gopls/internal/golang",
					"include_bodies": true,
					"include_docs":   false,
				}
			},
			validate: func(content string) (int, int, error) {
				isValid, isEmpty, isError := validateContent(content)
				if !isValid {
					return 0, 0, fmt.Errorf("Invalid content: isEmpty=%v, isError=%v", isEmpty, isError)
				}
				if len(content) < 500 {
					return 0, 0, fmt.Errorf("Suspiciously small response: %d bytes (expected >500 bytes)", len(content))
				}
				functionCount := strings.Count(content, "func ")
				return functionCount, len(content), nil
			},
		},
		{
			name:     "Function Signatures Only (Fast)",
			category: "Function Bodies",
			tool:     "go_list_package_symbols",
			args: func(projectDir string) map[string]any {
				return map[string]any{
					"package_path":   "golang.org/x/tools/gopls/internal/golang",
					"include_bodies": false,
					"include_docs":   false,
				}
			},
			validate: func(content string) (int, int, error) {
				functions := strings.Count(content, "func ")
				return functions, len(content), nil
			},
		},
	}

	var results []BenchmarkResult
	for _, tc := range cases {
		result := runSimpleBenchmark(ctx, session, projectDir, tc)
		if result.Success {
			if strings.Contains(tc.name, "Bodies") {
				result.ComparisonNote = fmt.Sprintf("MCP: %v | Critical for AI to learn patterns (%d bytes, %d functions)", result.Duration, result.BytesProcessed, result.ItemsFound)
			} else {
				result.ComparisonNote = fmt.Sprintf("MCP signatures: %v | Fast API exploration", result.Duration)
			}
		}
		results = append(results, result)
	}

	return results
}

// ===================================================================
// Category 4: Type Hierarchy Navigation
// ===================================================================

func BenchmarkTypeHierarchy(ctx context.Context, session *mcp.ClientSession, projectDir string) []BenchmarkResult {
	implFile := filepath.Join(projectDir, "internal", "golang", "implementation.go")

	cases := []simpleBenchmarkCase{
		{
			name:     "Find Interface Implementations",
			category: "Type Hierarchy",
			tool:     "go_implementation",
			args: func(projectDir string) map[string]any {
				return map[string]any{
					"locator": map[string]any{
						"symbol_name":  "implementationsMsets",
						"context_file": implFile,
						"kind":         "function",
						"line_hint":    170,
					},
				}
			},
			validate: func(content string) (int, int, error) {
				implementations := strings.Count(content, "implementation(s)")
				return implementations, len(content), nil
			},
			skip: func() bool {
				_, err := os.Stat(implFile)
				return os.IsNotExist(err)
			}(),
			skipReason: "Test file not found",
		},
	}

	var results []BenchmarkResult
	for _, tc := range cases {
		result := runSimpleBenchmark(ctx, session, projectDir, tc)
		if result.Success {
			result.ComparisonNote = fmt.Sprintf("MCP: %v | No traditional comparison baseline", result.Duration)
		}
		results = append(results, result)
	}

	return results
}

// ===================================================================
// Category 5: Error Detection Speed
// ===================================================================

func BenchmarkErrorDetection(ctx context.Context, session *mcp.ClientSession, projectDir string) BenchmarkResult {
	cases := []simpleBenchmarkCase{
		{
			name:     "Workspace Diagnostics",
			category: "Error Detection",
			tool:     "go_build_check",
			args: func(projectDir string) map[string]any {
				return map[string]any{
					"files": []string{},
				}
			},
			validate: func(content string) (int, int, error) {
				// No validation needed, just return content length
				return 0, len(content), nil
			},
		},
	}

	for _, tc := range cases {
		result := runSimpleBenchmark(ctx, session, projectDir, tc)
		if result.Success {
			result.ComparisonNote = fmt.Sprintf("MCP: %v | See Comparison: go build for measured speedup", result.Duration)
		}
		return result // Return first result
	}

	return BenchmarkResult{
		Name:     "Workspace Diagnostics",
		Category: "Error Detection",
		Success:  false,
		Error:    "No valid benchmark cases",
	}
}

// ===================================================================
// Category 6: File Reading Consistency
// ===================================================================

func BenchmarkFileReading(ctx context.Context, session *mcp.ClientSession, projectDir string) []BenchmarkResult {
	testFile := filepath.Join(projectDir, "internal", "golang", "implementation.go")

	cases := []simpleBenchmarkCase{
		{
			name:     "Read File via gopls",
			category: "File Reading",
			tool:     "go_read_file",
			args: func(projectDir string) map[string]any {
				return map[string]any{
					"file": testFile,
				}
			},
			validate: func(content string) (int, int, error) {
				return 0, len(content), nil
			},
		},
	}

	var results []BenchmarkResult
	for _, tc := range cases {
		result := runSimpleBenchmark(ctx, session, projectDir, tc)
		if result.Success {
			result.ComparisonNote = fmt.Sprintf("gopls snapshot.ReadFile: %v | Note: Uses gopls file reading API", result.Duration)
		}
		results = append(results, result)
	}

	return results
}

// ===================================================================
// Category 7: Comparison with Traditional Tools
// ===================================================================

func BenchmarkComparisonWithTraditionalTools(ctx context.Context, session *mcp.ClientSession, projectDir string) []BenchmarkResult {
	var results []BenchmarkResult

	// Compare: go list ./... vs list_project_defined_packages
	results = append(results, compareGoList(ctx, session, projectDir))

	// Compare: grep -r vs go_search
	results = append(results, compareGrep(ctx, session, projectDir))

	// Compare: go build vs go_build_check
	results = append(results, compareGoBuild(ctx, session, projectDir))

	return results
}

func compareGoList(ctx context.Context, session *mcp.ClientSession, projectDir string) BenchmarkResult {
	name := "Comparison: go list ./..."
	category := "Comparison"

	// Use adaptive sampling configuration
	config := DefaultBenchmarkConfig()
	config.MinIterations = 15
	config.MaxIterations = 40
	config.TargetCV = 15.0 // 15% target for go list (moderate variance acceptable)

	// Run traditional method with adaptive sampling
	traditionalStats := RunBenchmarkWithConfig(func() time.Duration {
		start := time.Now()
		cmd := exec.Command("go", "list", "./...")
		cmd.Dir = projectDir
		_, _ = cmd.CombinedOutput()
		return time.Since(start)
	}, config)

	// Run MCP method with adaptive sampling
	mcpStats := RunBenchmarkWithConfig(func() time.Duration {
		start := time.Now()
		_, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "go_list_module_packages",
			Arguments: map[string]any{
				"Cwd":              projectDir,
				"include_docs":     false, // Faster - we don't need docs for counting packages
				"exclude_tests":    false, // Include all packages for fair comparison
				"exclude_internal": false,
				"top_level_only":   false,
			},
		})
		if err != nil {
			return 0
		}
		return time.Since(start)
	}, config)

	// Check if all iterations succeeded
	if mcpStats.Mean == 0 {
		return BenchmarkResult{
			Name:     name,
			Category: category,
			Success:  false,
			Error:    "MCP tool failed",
		}
	}

	// Get package count from one run
	cmd := exec.Command("go", "list", "./...")
	cmd.Dir = projectDir
	output, _ := cmd.CombinedOutput()
	traditionalLines := len(strings.Split(string(output), "\n"))

	// Calculate speedup using means
	speedupFactor := float64(traditionalStats.Mean) / float64(mcpStats.Mean)

	// Record actual iterations performed
	actualIterations := len(traditionalStats.Values)

	return BenchmarkResult{
		Name:            name,
		Category:        category,
		Duration:        mcpStats.Mean,
		MinDuration:     mcpStats.Min,
		MaxDuration:     mcpStats.Max,
		StdDev:          mcpStats.StdDev,
		Iterations:      actualIterations,
		Success:         true,
		ItemsFound:      traditionalLines,
		ComparisonNote:  fmt.Sprintf("Traditional: %v (±%v), MCP: %v (±%v)", traditionalStats.Mean, traditionalStats.StdDev, mcpStats.Mean, mcpStats.StdDev),
		SpeedupFactor:   speedupFactor,
		TraditionalMean: traditionalStats.Mean,
		TraditionalMin:  traditionalStats.Min,
		TraditionalMax:  traditionalStats.Max,
	}
}

func compareGrep(ctx context.Context, session *mcp.ClientSession, projectDir string) BenchmarkResult {
	name := "Comparison: grep -r"
	category := "Comparison"

	// Use adaptive sampling configuration
	config := DefaultBenchmarkConfig()
	config.MinIterations = 15
	config.MaxIterations = 40
	config.TargetCV = 15.0

	// Run traditional method with adaptive sampling
	traditionalStats := RunBenchmarkWithConfig(func() time.Duration {
		start := time.Now()
		cmd := exec.Command("grep", "-r", "Implementation", projectDir)
		output, err := cmd.CombinedOutput()

		// FIX: Check for errors
		if err != nil {
			// Grep failed - return 0 to indicate failure
			fmt.Printf("Warning: grep failed: %v\n", err)
			return 0
		}

		// Validate output
		if len(output) == 0 {
			return 0 // No results means grep failed
		}

		return time.Since(start)
	}, config)

	// FIX: Check if traditional benchmark failed
	if traditionalStats.Mean == 0 {
		return BenchmarkResult{
			Name:     name,
			Category: category,
			Success:  false,
			Error:    "Traditional grep tool failed - cannot compare",
		}
	}

	// Run MCP method with adaptive sampling
	mcpStats := RunBenchmarkWithConfig(func() time.Duration {
		start := time.Now()
		_, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "go_search",
			Arguments: map[string]any{
				"query": "Implementation",
			},
		})
		if err != nil {
			return 0
		}
		return time.Since(start)
	}, config)

	// Check if all iterations succeeded
	if mcpStats.Mean == 0 {
		return BenchmarkResult{
			Name:     name,
			Category: category,
			Success:  false,
			Error:    "MCP tool failed",
		}
	}

	// Get grep output line count from one run
	cmd := exec.Command("grep", "-r", "Implementation", projectDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return BenchmarkResult{
			Name:     name,
			Category: category,
			Success:  false,
			Error:    fmt.Sprintf("Failed to get grep output count: %v", err),
		}
	}
	grepLines := len(strings.Split(string(output), "\n"))

	// Calculate speedup using means
	speedupFactor := float64(traditionalStats.Mean) / float64(mcpStats.Mean)
	actualIterations := len(traditionalStats.Values)

	return BenchmarkResult{
		Name:        name,
		Category:    category,
		Duration:    mcpStats.Mean,
		MinDuration: mcpStats.Min,
		MaxDuration: mcpStats.Max,
		StdDev:      mcpStats.StdDev,
		Iterations:  actualIterations,
		Success:     true,
		ItemsFound:  grepLines,
		ComparisonNote: fmt.Sprintf("grep: %v (±%v, %d lines), MCP: %v (±%v, semantic)",
			traditionalStats.Mean, traditionalStats.StdDev, grepLines, mcpStats.Mean, mcpStats.StdDev),
		SpeedupFactor:   speedupFactor,
		TraditionalMean: traditionalStats.Mean,
		TraditionalMin:  traditionalStats.Min,
		TraditionalMax:  traditionalStats.Max,
	}
}

func compareGoBuild(ctx context.Context, session *mcp.ClientSession, projectDir string) BenchmarkResult {
	name := "Comparison: go build"
	category := "Comparison"

	// Use adaptive sampling configuration
	config := DefaultBenchmarkConfig()
	config.MinIterations = 15
	config.MaxIterations = 40
	config.TargetCV = 20.0 // 20% target for go build (higher tolerance for compilation)

	// Run traditional method with adaptive sampling
	traditionalStats := RunBenchmarkWithConfig(func() time.Duration {
		start := time.Now()
		cmd := exec.Command("go", "build", "./...")
		cmd.Dir = projectDir
		_ = cmd.Run()
		return time.Since(start)
	}, config)

	// Run MCP method with adaptive sampling
	mcpStats := RunBenchmarkWithConfig(func() time.Duration {
		start := time.Now()
		_, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "go_build_check",
			Arguments: map[string]any{
				"files": []string{},
			},
		})
		if err != nil {
			return 0
		}
		return time.Since(start)
	}, config)

	// Check if all iterations succeeded
	if mcpStats.Mean == 0 {
		return BenchmarkResult{
			Name:     name,
			Category: category,
			Success:  false,
			Error:    "MCP tool failed",
		}
	}

	// Calculate speedup using means
	speedupFactor := float64(traditionalStats.Mean) / float64(mcpStats.Mean)
	actualIterations := len(traditionalStats.Values)

	return BenchmarkResult{
		Name:        name,
		Category:    category,
		Duration:    mcpStats.Mean,
		MinDuration: mcpStats.Min,
		MaxDuration: mcpStats.Max,
		StdDev:      mcpStats.StdDev,
		Iterations:  actualIterations,
		Success:     true,
		ComparisonNote: fmt.Sprintf("go build: %v (±%v), MCP diagnostics: %v (±%v)",
			traditionalStats.Mean, traditionalStats.StdDev, mcpStats.Mean, mcpStats.StdDev),
		SpeedupFactor:   speedupFactor,
		TraditionalMean: traditionalStats.Mean,
		TraditionalMin:  traditionalStats.Min,
		TraditionalMax:  traditionalStats.Max,
	}
}

// ===================================================================
// Category 8: Cold Start & Break-Even Analysis
// ===================================================================

// BenchmarkColdStart performs a cold start analysis of the MCP server.
// This measures the time from process start to first successful query,
// then calculates the break-even point where MCP becomes faster than CLI.
func BenchmarkColdStart(projectDir string, binaryPath string) BenchmarkResult {
	name := "Cold Start: Server to First Query"
	category := "Cold Start"

	var goplsMcpPath string
	var cleanupNeeded bool

	// Use pre-built binary if provided, otherwise build a temp one
	if binaryPath != "" {
		// Verify the pre-built binary exists
		if info, err := os.Stat(binaryPath); err != nil {
			return BenchmarkResult{
				Name:     name,
				Category: category,
				Success:  false,
				Error:    fmt.Sprintf("pre-built binary not found: %v", err),
			}
		} else if info.Size() == 0 {
			return BenchmarkResult{
				Name:     name,
				Category: category,
				Success:  false,
				Error:    fmt.Sprintf("pre-built binary is empty (0 bytes)"),
			}
		}
		goplsMcpPath = binaryPath
		cleanupNeeded = false
	} else {
		// Get gopls directory ( Navigate from gopls/mcpbridge/test/benchmark to gopls)
		goplsDir, err := filepath.Abs("../../..")
		if err != nil {
			return BenchmarkResult{
				Name:     name,
				Category: category,
				Success:  false,
				Error:    fmt.Sprintf("failed to get gopls directory: %v", err),
			}
		}

		// Build gopls-mcp to temp location
		goplsMcpPath = filepath.Join(goplsDir, "goplsmcp-coldstart.tmp")
		buildCmd := exec.Command("go", "build", "-o", goplsMcpPath, "./mcpbridge")
		buildCmd.Dir = goplsDir

		output, err := buildCmd.CombinedOutput()
		if err != nil {
			return BenchmarkResult{
				Name:     name,
				Category: category,
				Success:  false,
				Error:    fmt.Sprintf("failed to build gopls-mcp: %v\n%s", err, output),
			}
		}

		// Verify the binary was created and is valid
		info, statErr := os.Stat(goplsMcpPath)
		if statErr != nil {
			return BenchmarkResult{
				Name:     name,
				Category: category,
				Success:  false,
				Error:    fmt.Sprintf("binary not found after build: %v", statErr),
			}
		}

		if info.Size() == 0 {
			return BenchmarkResult{
				Name:     name,
				Category: category,
				Success:  false,
				Error:    "binary is empty (0 bytes) - build may have failed silently",
			}
		}

		// Ensure the binary has execute permissions
		if err := os.Chmod(goplsMcpPath, 0755); err != nil {
			return BenchmarkResult{
				Name:     name,
				Category: category,
				Success:  false,
				Error:    fmt.Sprintf("failed to set executable permissions: %v", err),
			}
		}
		cleanupNeeded = true
	}

	// Setup cleanup for temp binary
	if cleanupNeeded {
		defer os.Remove(goplsMcpPath)
	}

	// Measure time from process start to first successful query
	serverStartTime := time.Now()

	// Start gopls-mcp
	goplsMcpCmd := exec.Command(goplsMcpPath, "-workdir", projectDir)
	client := mcp.NewClient(&mcp.Implementation{Name: "coldstart-benchmark", Version: "v0.0.1"}, nil)
	ctx := context.Background()
	mcpSession, err := client.Connect(ctx, &mcp.CommandTransport{Command: goplsMcpCmd}, nil)
	if err != nil {
		return BenchmarkResult{
			Name:     name,
			Category: category,
			Success:  false,
			Error:    fmt.Sprintf("failed to connect to gopls-mcp: %v", err),
		}
	}
	defer mcpSession.Close()

	// Get the MCP server process PID
	var serverPID int
	if goplsMcpCmd.Process != nil {
		serverPID = goplsMcpCmd.Process.Pid
	} else {
		return BenchmarkResult{
			Name:     name,
			Category: category,
			Success:  false,
			Error:    "failed to get MCP server process PID",
		}
	}

	// Collect initial process memory stats (for comparison)
	initialRSS, initialVSz, err := GetProcessMemoryStats(serverPID)
	if err != nil {
		// Log but don't fail - ps might not be available on all systems
		fmt.Printf("Warning: Failed to get initial memory stats: %v\n", err)
	}

	// Immediately run first query - no warmup
	firstQueryStart := time.Now()
	_, err = mcpSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "go_list_module_packages",
		Arguments: map[string]any{
			"Cwd":              projectDir,
			"include_docs":     false,
			"exclude_tests":    false,
			"exclude_internal": false,
			"top_level_only":   false,
		},
	})
	if err != nil {
		return BenchmarkResult{
			Name:     name,
			Category: category,
			Success:  false,
			Error:    fmt.Sprintf("first query failed: %v", err),
		}
	}
	firstQueryTime := time.Since(firstQueryStart)

	// Total time from server start to first successful result
	serverStartupTime := time.Since(serverStartTime)

	// Now measure warm query times (after cache is built)
	config := DefaultBenchmarkConfig()
	config.MinIterations = 15
	config.MaxIterations = 30
	config.TargetCV = 15.0
	config.EnableWarmup = false // Already warm after first query

	warmStats := RunBenchmarkWithConfig(func() time.Duration {
		start := time.Now()
		_, err := mcpSession.CallTool(ctx, &mcp.CallToolParams{
			Name: "go_list_module_packages",
			Arguments: map[string]any{
				"Cwd":              projectDir,
				"include_docs":     false,
				"exclude_tests":    false,
				"exclude_internal": false,
				"top_level_only":   false,
			},
		})
		if err != nil {
			return 0
		}
		return time.Since(start)
	}, config)

	if warmStats.Mean == 0 {
		return BenchmarkResult{
			Name:     name,
			Category: category,
			Success:  false,
			Error:    "warm queries failed",
		}
	}

	// Get traditional tool time for comparison
	traditionalConfig := DefaultBenchmarkConfig()
	traditionalConfig.MinIterations = 10
	traditionalConfig.MaxIterations = 20
	traditionalConfig.TargetCV = 15.0

	traditionalStats := RunBenchmarkWithConfig(func() time.Duration {
		start := time.Now()
		cmd := exec.Command("go", "list", "./...")
		cmd.Dir = projectDir
		_, _ = cmd.CombinedOutput()
		return time.Since(start)
	}, traditionalConfig)

	// Calculate break-even point
	// Formula: After N operations, total time with MCP = total time with CLI
	// N * warm_mcp_time + startup_time = N * cli_time
	// N * (cli_time - warm_mcp_time) = startup_time
	// N = startup_time / (cli_time - warm_mcp_time)

	breakEvenOps := 0
	breakEvenReason := ""

	if traditionalStats.Mean > warmStats.Mean {
		timeSavedPerOp := traditionalStats.Mean - warmStats.Mean
		breakEvenOps = int(math.Round(float64(serverStartupTime) / float64(timeSavedPerOp)))
		breakEvenReason = fmt.Sprintf("Startup overhead recouped after %d operations. Each subsequent operation saves ~%v per query.",
			breakEvenOps, timeSavedPerOp)
	} else {
		breakEvenOps = -1 // Never breaks even
		breakEvenReason = "MCP query time is not faster than CLI, so no break-even point exists."
	}

	// Create detailed comparison note
	note := fmt.Sprintf("Cold Start: %v (total) | First Query: %v | Warm Query Avg: %v (±%v) | CLI Avg: %v (±%v) | %s",
		serverStartupTime,
		firstQueryTime,
		warmStats.Mean, warmStats.StdDev,
		traditionalStats.Mean, traditionalStats.StdDev,
		breakEvenReason)

	// FIX: Collect Go runtime memory metrics AFTER queries
	// Trigger GC to get accurate metrics
	runtime.GC()
	finalMemory := GetMemoryMetrics() // FIX: Actually call the function!

	// Collect final process memory metrics
	finalRSS, finalVSz, err := GetProcessMemoryStats(serverPID)
	if err != nil {
		fmt.Printf("Warning: Failed to get final memory stats: %v\n", err)
		// Use initial values as fallback
		finalRSS = initialRSS
		finalVSz = initialVSz
	}

	// FIX: Update memory metrics with Go runtime data
	finalMemory.ProcessRSS = finalRSS
	finalMemory.ProcessVSz = finalVSz

	return BenchmarkResult{
		Name:            name,
		Category:        category,
		Duration:        serverStartupTime,
		Success:         true,
		Iterations:      len(warmStats.Values) + 1,
		ComparisonNote:  note,
		SpeedupFactor:   float64(breakEvenOps),
		TraditionalMean: traditionalStats.Mean,
		TraditionalMin:  traditionalStats.Min,
		TraditionalMax:  traditionalStats.Max,
		Memory:          finalMemory, // FIX: Now populated with Go runtime stats
	}
}

// ===================================================================
// Helper Functions
// ===================================================================

// calculateCV computes the coefficient of variation as a percentage.
func calculateCV(stdDev, mean time.Duration) float64 {
	if mean == 0 {
		return 0
	}
	return (float64(stdDev) / float64(mean)) * 100
}

// IterationStats holds statistics from multiple benchmark runs.
type IterationStats struct {
	Mean   time.Duration
	Min    time.Duration
	Max    time.Duration
	StdDev time.Duration
	Values []time.Duration
}

// CalculateStats computes mean, min, max, and standard deviation.
func CalculateStats(durations []time.Duration) IterationStats {
	if len(durations) == 0 {
		return IterationStats{}
	}

	// Calculate sum, min, max
	var sum time.Duration
	minD := durations[0]
	maxD := durations[0]
	for _, d := range durations {
		sum += d
		if d < minD {
			minD = d
		}
		if d > maxD {
			maxD = d
		}
	}
	mean := sum / time.Duration(len(durations))

	// Calculate standard deviation
	var varianceSum float64
	meanFloat := float64(mean)
	for _, d := range durations {
		diff := float64(d) - meanFloat
		varianceSum += diff * diff
	}
	variance := varianceSum / float64(len(durations))
	stdDev := time.Duration(int64(math.Sqrt(variance)))

	return IterationStats{
		Mean:   mean,
		Min:    minD,
		Max:    maxD,
		StdDev: stdDev,
		Values: durations,
	}
}

// BenchmarkConfig holds configuration for benchmark execution.
type BenchmarkConfig struct {
	MinIterations    int     // Minimum iterations (default: 10)
	MaxIterations    int     // Maximum iterations (default: 50)
	TargetCV         float64 // Target coefficient of variation (default: 0.10 for 10%)
	WarmupIterations int     // Number of warmup runs before measurement (default: 3)
	EnableWarmup     bool    // Whether to perform warmup
	EnableAdaptive   bool    // Whether to use adaptive sampling
}

// DefaultBenchmarkConfig returns sensible defaults.
func DefaultBenchmarkConfig() BenchmarkConfig {
	return BenchmarkConfig{
		MinIterations:    10,
		MaxIterations:    50,
		TargetCV:         10.0, // 10%
		WarmupIterations: 3,
		EnableWarmup:     true,
		EnableAdaptive:   true,
	}
}

// RunIterations runs a function N times and returns statistics.
// Deprecated: Use RunBenchmarkWithConfig for better accuracy.
func RunIterations(iterations int, fn func() time.Duration) IterationStats {
	config := DefaultBenchmarkConfig()
	config.MinIterations = iterations
	config.MaxIterations = iterations
	config.EnableAdaptive = false
	return RunBenchmarkWithConfig(fn, config)
}

// RunBenchmarkWithConfig runs a benchmark with warmup and adaptive sampling.
// Performs warmup iterations (not counted), then runs adaptive sampling until:
// 1. Min iterations reached AND CV <= TargetCV, OR
// 2. Max iterations reached
func RunBenchmarkWithConfig(fn func() time.Duration, config BenchmarkConfig) IterationStats {
	// Step 1: Warmup phase (not counted in statistics)
	if config.EnableWarmup && config.WarmupIterations > 0 {
		for i := 0; i < config.WarmupIterations; i++ {
			_ = fn()
		}
		// Small pause to let caches stabilize
		time.Sleep(10 * time.Millisecond)
	}

	// Step 2: Adaptive sampling phase
	durations := make([]time.Duration, 0, config.MaxIterations)

	for iteration := 0; iteration < config.MaxIterations; iteration++ {
		// Run one iteration
		durations = append(durations, fn())

		// Calculate current statistics
		stats := CalculateStats(durations)
		cv := calculateCV(stats.StdDev, stats.Mean)

		// Check if we've reached minimum iterations and target CV
		if len(durations) >= config.MinIterations {
			if !config.EnableAdaptive {
				// Fixed number of iterations
				break
			}
			if cv <= config.TargetCV {
				// Target precision achieved
				break
			}
			// Continue sampling to reduce variance
		}
	}

	return CalculateStats(durations)
}

// GatherProjectInfo collects statistics about the project.
func GatherProjectInfo(projectDir string) (*ProjectInfo, error) {
	info := &ProjectInfo{
		Name: filepath.Base(projectDir),
		Path: projectDir,
	}

	// Count Go files
	err := filepath.Walk(projectDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip vendor, testdata, .git
		if strings.Contains(path, "vendor") || strings.Contains(path, ".git") || strings.Contains(path, "testdata") {
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !fi.IsDir() && strings.HasSuffix(path, ".go") {
			info.Files++
			// Count lines
			content, err := os.ReadFile(path)
			if err == nil {
				info.LOC += len(strings.Split(string(content), "\n"))
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Count packages using go list
	cmd := exec.Command("go", "list", "./...")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	if err == nil {
		info.Packages = len(strings.Split(strings.TrimSpace(string(output)), "\n"))
	}

	return info, nil
}

func calculateSpeedup(mcpDuration, traditionalDuration time.Duration) float64 {
	if traditionalDuration == 0 {
		return 1.0
	}
	return float64(traditionalDuration) / float64(mcpDuration)
}

// ===================================================================
// Category 9: Go Definition (Code Navigation)
// ===================================================================

// BenchmarkGoDefinition benchmarks jump-to-definition performance.
// Compares MCP's semantic navigation vs traditional grep+file-inspection approach.
func BenchmarkGoDefinition(ctx context.Context, session *mcp.ClientSession, projectDir string) []BenchmarkResult {
	var results []BenchmarkResult
	results = append(results, compareGoDefinition(ctx, session, projectDir))
	return results
}

func compareGoDefinition(ctx context.Context, session *mcp.ClientSession, projectDir string) BenchmarkResult {
	name := "Comparison: go_definition (via grep)"
	category := "Code Navigation"

	// DYNAMIC SYMBOL DISCOVERY: Find PrepareItem location instead of hardcoding
	symbolFile, symbolLine, err := FindSymbolLocation(projectDir, "PrepareItem", "type")
	if err != nil {
		return BenchmarkResult{
			Name:     name,
			Category: category,
			Success:  false,
			Error:    fmt.Sprintf("failed to find test symbol: %v", err),
		}
	}

	config := DefaultBenchmarkConfig()
	config.MinIterations = 15
	config.MaxIterations = 40
	config.TargetCV = 20.0

	// Traditional method: grep to find file, then read specific lines
	traditionalStats := RunBenchmarkWithConfig(func() time.Duration {
		start := time.Now()
		grepCmd := exec.Command("grep", "-r", "^type PrepareItem struct", projectDir)
		grepOutput, err := grepCmd.CombinedOutput()
		if err != nil {
			return 0
		}
		lines := strings.Split(string(grepOutput), "\n")
		if len(lines) == 0 {
			return 0
		}
		parts := strings.Split(lines[0], ":")
		if len(parts) < 2 {
			return 0
		}
		testFile := strings.TrimSpace(parts[0])
		if _, err := os.ReadFile(testFile); err != nil {
			return 0
		}
		return time.Since(start)
	}, config)

	// MCP method: go_definition to jump directly
	mcpStats := RunBenchmarkWithConfig(func() time.Duration {
		// Verify the file still exists
		if _, err := os.Stat(symbolFile); os.IsNotExist(err) {
			return 0
		}

		defStart := time.Now()
		_, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "go_definition",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "PrepareItem",
					"context_file": symbolFile,
					"kind":         "type",
					"line_hint":    symbolLine,
				},
			},
		})
		if err != nil {
			return 0
		}
		return time.Since(defStart)
	}, config)

	if mcpStats.Mean == 0 {
		return BenchmarkResult{
			Name:     name,
			Category: category,
			Success:  false,
			Error:    "MCP tool failed",
		}
	}

	speedupFactor := float64(traditionalStats.Mean) / float64(mcpStats.Mean)
	actualIterations := len(traditionalStats.Values)

	return BenchmarkResult{
		Name:            name,
		Category:        category,
		Duration:        mcpStats.Mean,
		MinDuration:     mcpStats.Min,
		MaxDuration:     mcpStats.Max,
		StdDev:          mcpStats.StdDev,
		Iterations:      actualIterations,
		Success:         true,
		ComparisonNote:  fmt.Sprintf("grep+read: %v (±%v), MCP semantic: %v (±%v)", traditionalStats.Mean, traditionalStats.StdDev, mcpStats.Mean, mcpStats.StdDev),
		SpeedupFactor:   speedupFactor,
		TraditionalMean: traditionalStats.Mean,
		TraditionalMin:  traditionalStats.Min,
		TraditionalMax:  traditionalStats.Max,
	}
}

// ===================================================================
// Category 10: Go Hover (Quick Documentation)
// ===================================================================

func compareSymbolReferences(ctx context.Context, session *mcp.ClientSession, projectDir string) BenchmarkResult {
	name := "Comparison: go_symbol_references (via grep)"
	category := "Symbol Analysis"

	// DYNAMIC SYMBOL DISCOVERY
	symbolFile, _, err := FindSymbolLocation(projectDir, "PrepareItem", "type")
	if err != nil {
		return BenchmarkResult{
			Name:     name,
			Category: category,
			Success:  false,
			Error:    fmt.Sprintf("failed to find test symbol: %v", err),
		}
	}

	config := DefaultBenchmarkConfig()
	config.MinIterations = 15
	config.MaxIterations = 40
	config.TargetCV = 20.0

	// Traditional method: grep for symbol name and count occurrences
	traditionalStats := RunBenchmarkWithConfig(func() time.Duration {
		start := time.Now()
		cmd := exec.Command("grep", "-r", "PrepareItem", projectDir)
		output, err := cmd.CombinedOutput()

		// CHECK ERROR RETURN (fix from Issue #5)
		if len(output) == 0 || err != nil {
			return 0 // Grep failed
		}

		// Count lines to get reference count
		_ = len(strings.Split(string(output), "\n"))
		return time.Since(start)
	}, config)

	// MCP method: go_symbol_references for semantic usage finding
	mcpStats := RunBenchmarkWithConfig(func() time.Duration {
		start := time.Now()
		_, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "go_symbol_references",
			Arguments: map[string]any{
				"locator": map[string]any{
					"symbol_name":  "PrepareItem",
					"context_file": symbolFile,
					"kind":         "type",
					"line_hint":    0,
				},
			},
		})
		if err != nil {
			return 0
		}
		return time.Since(start)
	}, config)

	if mcpStats.Mean == 0 {
		return BenchmarkResult{
			Name:     name,
			Category: category,
			Success:  false,
			Error:    "MCP tool failed",
		}
	}

	speedupFactor := float64(traditionalStats.Mean) / float64(mcpStats.Mean)
	actualIterations := len(traditionalStats.Values)

	return BenchmarkResult{
		Name:            name,
		Category:        category,
		Duration:        mcpStats.Mean,
		MinDuration:     mcpStats.Min,
		MaxDuration:     mcpStats.Max,
		StdDev:          mcpStats.StdDev,
		Iterations:      actualIterations,
		Success:         true,
		ComparisonNote:  fmt.Sprintf("grep count: %v (±%v), MCP semantic: %v (±%v)", traditionalStats.Mean, traditionalStats.StdDev, mcpStats.Mean, mcpStats.StdDev),
		SpeedupFactor:   speedupFactor,
		TraditionalMean: traditionalStats.Mean,
		TraditionalMin:  traditionalStats.Min,
		TraditionalMax:  traditionalStats.Max,
	}
}

// ===================================================================
// Category 12: List Modules
// ===================================================================

// BenchmarkListModules benchmarks listing all modules in a project.
// Compares MCP vs go list -m.
func BenchmarkListModules(ctx context.Context, session *mcp.ClientSession, projectDir string) []BenchmarkResult {
	var results []BenchmarkResult
	results = append(results, compareListModules(ctx, session, projectDir))
	return results
}

func compareListModules(ctx context.Context, session *mcp.ClientSession, projectDir string) BenchmarkResult {
	name := "Comparison: list_modules (via go list -m)"
	category := "Module Discovery"

	config := DefaultBenchmarkConfig()
	config.MinIterations = 15
	config.MaxIterations = 40
	config.TargetCV = 15.0

	// Traditional method: go list -m
	traditionalStats := RunBenchmarkWithConfig(func() time.Duration {
		start := time.Now()
		cmd := exec.Command("go", "list", "-m")
		cmd.Dir = projectDir
		_, _ = cmd.CombinedOutput()
		return time.Since(start)
	}, config)

	// MCP method: list_modules
	mcpStats := RunBenchmarkWithConfig(func() time.Duration {
		start := time.Now()
		_, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "go_list_modules",
			Arguments: map[string]any{
				"Cwd":         projectDir,
				"direct_only": true,
			},
		})
		if err != nil {
			return 0
		}
		return time.Since(start)
	}, config)

	if mcpStats.Mean == 0 {
		return BenchmarkResult{
			Name:     name,
			Category: category,
			Success:  false,
			Error:    "MCP tool failed",
		}
	}

	speedupFactor := float64(traditionalStats.Mean) / float64(mcpStats.Mean)
	actualIterations := len(traditionalStats.Values)

	// Get module count from go list -m
	cmd := exec.Command("go", "list", "-m")
	cmd.Dir = projectDir
	output, _ := cmd.CombinedOutput()
	moduleCount := len(strings.Split(strings.TrimSpace(string(output)), "\n"))

	return BenchmarkResult{
		Name:            name,
		Category:        category,
		Duration:        mcpStats.Mean,
		MinDuration:     mcpStats.Min,
		MaxDuration:     mcpStats.Max,
		StdDev:          mcpStats.StdDev,
		Iterations:      actualIterations,
		Success:         true,
		ItemsFound:      moduleCount,
		ComparisonNote:  fmt.Sprintf("go list -m: %v (±%v), MCP: %v (±%v)", traditionalStats.Mean, traditionalStats.StdDev, mcpStats.Mean, mcpStats.StdDev),
		SpeedupFactor:   speedupFactor,
		TraditionalMean: traditionalStats.Mean,
		TraditionalMin:  traditionalStats.Min,
		TraditionalMax:  traditionalStats.Max,
	}
}

// ===================================================================
// Category 13: Dependency Graph (No Comparison)
// ===================================================================

// BenchmarkDependencyGraph benchmarks dependency graph generation.
// No direct comparison, but measures performance characteristics.
func BenchmarkDependencyGraph(ctx context.Context, session *mcp.ClientSession, projectDir string) []BenchmarkResult {
	cases := []simpleBenchmarkCase{
		{
			name:     "Dependency Graph Generation",
			category: "Dependency Analysis",
			tool:     "go_get_dependency_graph",
			args: func(projectDir string) map[string]any {
				return map[string]any{
					"package_path":       projectDir, // Analyze main package
					"include_transitive": true,       // Include all dependencies
					"Cwd":                projectDir,
				}
			},
			validate: func(content string) (int, int, error) {
				// Count dependencies by counting lines
				deps := strings.Count(content, "→") + strings.Count(content, "└─")
				return deps, len(content), nil
			},
		},
	}

	var results []BenchmarkResult
	for _, tc := range cases {
		result := runSimpleBenchmark(ctx, session, projectDir, tc)
		if result.Success {
			result.ComparisonNote = fmt.Sprintf("MCP: %v | No comparison baseline - unique feature", result.Duration)
		}
		results = append(results, result)
	}

	return results
}
