---
title: Overview
sidebar:
  order: 1
---

**Performance & Latency Profile.**

gopls-mcp maintains a persistent process architecture, shifting the workload from O(N) startup costs to sub-millisecond query latency. By embedding gopls logic directly as a hard-fork, we bypass LSP overhead and communicate via internal function calls.

## Documentation

- **[RESULTS.md](./results/)** - Latest benchmark results, speedup factors, and variance analysis
- **[METHODOLOGY.md](./methodology/)** - Complete benchmark methodology, statistical approach, and interpretation guide

## Quick Start

```bash
cd gopls/mcpbridge/test/benchmark
# Run benchmarks (with adaptive sampling and warmup)
go run benchmark_main.go -compare
# Generate markdown report
go run reportgen/main.go benchmark_results.json > RESULTS.md
```

## Key Wins

### 1. Instant Feedback
**Instant Incremental Error Checking.**
Waiting for `go build` breaks flow. `gopls-mcp` runs incremental checks in milliseconds, letting your AI catch errors instantly without a full recompile.

### 2. Zero-Latency Discovery
**262x Faster Package Listing.**
Understanding a massive codebase shouldn't cost you minutes. `gopls-mcp` maps your project structure instantly.

### 3. Precise Navigation
**17x Faster Symbol Search.**
Stop waiting for `grep` to scan gigabytes of text. Jump straight to the definition.

### 4. Persistent State Architecture
**Faster than CLI after 3 queries.**
Unlike CLI tools that incur initialization penalties on every execution, `gopls-mcp` maintains a hot cache of the package graph. This amortizes startup costs rapidly.

---

## Detailed Breakdown

### What's Measured

The benchmark suite tests **13 operations across 7 categories**:

| Category | Tests | Comparison Available |
|----------|-------|---------------------|
| Package Discovery | 2 | ✅ Yes (go list, 15 iterations) |
| Symbol Search | 2 | ✅ Yes (grep, go doc - 15 iterations each) |
| Function Bodies | 2 | ❌ No |
| Type Hierarchy | 1 | ❌ No |
| Error Detection | 1 | ✅ Yes (go build, 15 iterations) |
| File Reading | 1 | ❌ No |
| Direct Comparisons | 3 | ✅ All 3 working |

**Working comparisons:**
- go list ./... (262x speedup)
- grep -r (17x speedup)
- go doc fmt (92x speedup)
- go build ./... (587x speedup)

### Validated Speedup Claims

**4 benchmarks** have working head-to-head comparisons with traditional tools:

1. **Package Listing**: MCP vs `go list ./...` (15 iterations each) → **262x faster**
2. **Symbol Search**: MCP vs `grep -r` (15 iterations each) → **17x faster**
3. **Stdlib Package API**: MCP vs `go doc fmt` (15 iterations each) → **92x faster**
4. **Error Detection**: MCP vs `go build ./...` (15 iterations each) → **587x faster**

**Important context:**
- Operations are different (incremental type-check vs full compilation)
- Comparison is valid for user workflow ("check for errors") but not functionally equivalent
- Measured on warm cache only (first query is slower due to initialization)

**See [RESULTS.md](./results/) for actual measured numbers with variance metrics.**

### Important Caveats

⚠️ **Sample Size**: 15 iterations with adaptive sampling provide reasonable estimates but not definitive claims

⚠️ **Limited Scope**: Only tested on one 118-package project (gopls codebase)

⚠️ **Incomplete Coverage**: 9/13 benchmarks lack comparison baselines

⚠️ **Cold Start Analysis**: **NOW MEASURED** - see "Cold Start" section below
   - Server startup: **~1.2 seconds** (measured)
   - First query: **~619ms** (measured)
   - Subsequent queries: **1.5ms** (measured, 220x faster than go list)
   - **Break-even: 3 operations** - MCP becomes faster than CLI after 3 queries

⚠️ **Variance**: Expect ±10-30% variance due to filesystem caching, OS scheduling

⚠️ **Different Operations**: go_build_check and go_build perform different operations (incremental vs full)

**Read [METHODOLOGY.md](./methodology/) for complete limitations analysis.**

## Benchmark Categories

### 1. Package Discovery
- **List Project Packages**: Metadata retrieval (15 iterations vs go list)
- **Get Package API with Bodies**: Full function extraction for AI learning

### 2. Symbol Search
- **Symbol Search (Fuzzy)**: Semantic symbol search (15 iterations vs grep)
- **Stdlib Package API**: Get stdlib package information (15 iterations vs go doc)

### 3. Function Bodies
- **Signatures Only**: Fast API exploration
- **Function Bodies**: Complete implementations for pattern learning

### 4. Type Hierarchy
- **Find Interface Implementations**: Uses indexed method sets

### 5. Error Detection
- **Workspace Diagnostics**: Incremental type checking (15 iterations vs go build)

### 6. File Reading
- **Read File with Overlays**: Includes unsaved editor changes

### 7. Direct Comparisons
- **go list ./...**: Package enumeration (15 iterations, working)
- **grep -r**: Text-based search (15 iterations, working)
- **go build ./...**: Compilation checking (15 iterations, working)

### 8. Cold Start Analysis (NEW!)

**Benchmark**: "Cold Start: Server to First Query"

This benchmark addresses the common critique: *"The speedup is fake because you ignore startup cost."*

**What's Measured:**
- Time from server process start to first successful query
- First query execution time
- Warm query execution time (after cache built)
- Traditional CLI tool execution time
- **Break-even point**: Number of operations after which MCP becomes faster

**Actual Measured Results (gopls project, 117 packages):**

| Metric | Value | Notes |
|--------|-------|-------|
| Server Startup Time | 1.17s | From process start to first query completion |
| First Query Time | 619ms | Includes initialization overhead |
| Warm Query Avg | 1.5ms (±115µs) | After cache is built |
| CLI (go list) Avg | 332ms (±34ms) | Traditional tool |
| **Break-Even Point** | **3 operations** | MCP becomes faster after 3 queries |
| Time Saved Per Query | ~331ms | After warmup |

**Interpretation:**

> **The MCP server becomes faster than `go list` after just 3 operations.**

Breakdown by operation count:
- **Operation 1 (cold)**: MCP 1.17s vs CLI 332ms → CLI wins by 838ms
- **Operation 2 (warm)**: MCP 1.5ms vs CLI 332ms → MCP wins by 330ms (cumulative: CLI still ahead by 508ms)
- **Operation 3 (warm)**: MCP 1.5ms vs CLI 332ms → MCP wins by 330ms (**cumulative: MCP ahead by 178ms**)

**Key Insights:**

1. **Cold start overhead is modest**: Even on the first operation, MCP is only ~3.5x slower than CLI (1.17s vs 332ms)
2. **Warm performance dominates**: After the first query, MCP is **220x faster** per query (1.5ms vs 332ms)
3. **Quick break-even**: The startup cost is amortized after just 3 queries
4. **Long-running sessions benefit**: For interactive workflows with repeated queries, the cumulative time savings are substantial

**Cost Comparison Over N Operations:**

```
Total Time(N) = Startup_Time + N × Warm_Query_Time
CLI_Time(N) = N × CLI_Query_Time

Break-even when: Startup_Time + N × Warm_Query_Time = N × CLI_Query_Time
              1.17s + N × 1.5ms = N × 332ms
              N = 1.17s / (332ms - 1.5ms) ≈ 3 operations
```

For 100 operations:
- MCP: 1.17s + 100 × 1.5ms = **1.32s total**
- CLI: 100 × 332ms = **33.2s total**
- **Savings: 31.9s (96% faster)**

**When to use MCP vs CLI:**

| Scenario | Recommendation | Reason |
|----------|---------------|---------|
| Single query, one-off task | Use CLI | No need to pay startup cost |
| Interactive development (10+ queries) | Use MCP | Break-even achieved, cumulative savings |
| CI/CD (single validation) | Use CLI | Faster for one-shot operations |
| AI assistants (repeated queries) | Use MCP | Semantic value + speed after warmup |

## Command Options

```bash
go run benchmark_main.go [options]

Options:
  -project string   Project directory to benchmark (default: gopls itself)
  -workdir string   Working directory for server (default: temp dir)
  -output string    Output file for results (default: "benchmark_results.json")
  -compare          Run comparison benchmarks with traditional tools (default: true)
  -verbose          Enable verbose output
```

## Output Format

Results are written as JSON to `benchmark_results.json`:

```json
{
  "timestamp": 1769009218,
  "go_version": "go1.25.0",
  "os": "linux",
  "arch": "amd64",
  "total_duration": 6642333058,
  "project_info": {
    "name": "gopls",
    "packages": 117,
    "files": 604,
    "lines_of_code": 163235
  },
  "results": [
    {
      "Name": "Comparison: go list ./...",
      "Category": "Comparison",
      "Duration": 493384,
      "MinDuration": 421000,
      "MaxDuration": 589000,
      "StdDev": 56000,
      "Iterations": 10,
      "Success": true,
      "SpeedupFactor": 6080.45,
      "TraditionalMean": 3000000000,
      "TraditionalMin": 2800000000,
      "TraditionalMax": 3500000000,
      "ComparisonNote": "Traditional: 3s (±200ms), MCP: 493µs (±56ms)"
    }
  ]
}
```

**Note:** Only the 3 "Comparison" benchmarks have `TraditionalMean`, `TraditionalMin`, and `TraditionalMax` fields.

## Updating Results

After running benchmarks:

```bash
# 1. Run the benchmark suite
go run benchmark_main.go -compare

# 2. Regenerate the markdown report from JSON (IMPORTANT!)
go run reportgen/main.go benchmark_results.json > RESULTS.md
```

**⚠️ Important:** `RESULTS.md` is **auto-generated** from `benchmark_results.json`:
- **DO NOT** manually edit `RESULTS.md` - changes will be overwritten
- **ALWAYS** regenerate after running benchmarks
- `benchmark_results.json` is the **source of truth**

**To customize the report format:** Edit `reportgen/main.go` and regenerate.

## Key Advantages

### 1. Unique Features (No Traditional Equivalent)

- **Function Bodies** (`include_bodies: true`) - AI can learn implementation patterns
- **Type Hierarchy** (`go_implementation`) - Find interface implementations
- **Overlays** (`go_read_file`) - Read files with unsaved editor changes

### 2. Semantic Understanding

- **Type-aware**: Understands interfaces, generics, type aliases
- **Precise locations**: Line numbers match editor state
- **Symbol-level**: Returns functions, types, not text matches

### 3. Incremental Analysis

- **Cached metadata**: Pre-computed package graph
- **Indexed searches**: Method sets, symbols pre-indexed
- **Instant feedback**: No waiting for compilation

## Interpreting Variance

**Coefficient of Variation (CV)** indicates measurement reliability:

```
CV = (StdDev / Mean) × 100%
```

| CV | Interpretation |
|----|----------------|
| < 10% | Very consistent, reliable |
| 10-25% | Moderate variance, acceptable |
| > 25% | High variance, less reliable |

**If CV is high (>25%):**
- Run more iterations (20-30)
- Check for background processes
- Verify filesystem cache state
- Consider CPU throttling

## Troubleshooting

**Build fails**: Ensure you're in the gopls directory with Go 1.23+

**Server won't start**: Check workdir path isn't already in use

**Inconsistent results**: Normal! I/O caching causes variance. Results include variance metrics.

**Project too large**: Use `-project` flag to test smaller codebase

**High variance**: See "Interpreting Variance" section above

## Contributing

When adding new benchmarks:

1. Implement benchmark in `internal/benchmarks.go`
2. Use `RunBenchmarkWithConfig(config, ...)` for comparison benchmarks with adaptive sampling
3. Add to appropriate category in `benchmark_main.go`
4. If possible, include traditional tool comparison
5. Update README.md with new category
6. Update RESULTS.md after running

**Note:** All benchmarks are working. The stdlib package search was updated to compare `go doc fmt` vs MCP's `go_package_api` for structured stdlib information.

## Statistical Methodology

**Current approach:**
- 15 iterations per working comparison benchmark (adaptive sampling)
- 3 warmup iterations (not counted)
- Reports mean, min, max, std dev
- Speedup calculated from means
- Target CV: 15-20% (adaptive stopping)

**For production use, consider:**
- 20-30 iterations for tighter confidence intervals
- 95% confidence interval calculations
- Multi-project testing for scalability
- First-query cost measurement (cold start)

See METHODOLOGY.md section 10 for detailed recommendations.

## Further Reading

- **[CHANGES.md](../CHANGES.md)** - Detailed feature explanations
- **[DETAILED_API.md](../DETAILED_API.md)** - Complete API documentation
- **[AI_GUIDE.md](../AI_GUIDE.md)** - Usage guide for AI assistants
