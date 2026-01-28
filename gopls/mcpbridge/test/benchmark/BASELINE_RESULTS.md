✅ Generated RESULTS.md from benchmark_results.json
-24 00:19:55

**Go Version:** go1.25.0

**Platform:** linux/amd64

## Table of Contents

- [Test Environment](#test-environment)
- [Summary](#summary)
- [Comparison Benchmarks](#comparison-benchmarks)
- [All Benchmarks](#all-benchmarks)
- [Performance by Category](#performance-by-category)
- [Variance Analysis](#variance-analysis)

---

## Test Environment

### Project Information

| Metric | Value |
|--------|-------|
| Project | gopls |
| Path | /home/xieyuschen/codespace/gopls-mcp/gopls |
| Packages | 118 |
| Files | 629 |
| Lines of Code | 177201 |

## Summary

### Overall Statistics

| Metric | Value |
|--------|-------|
| Total Benchmarks | 19 |
| Successful | 19 ✅ |
| Failed | 0 ❌ |
| Average Duration | 84.06ms |
| Total Duration | 48.52s |
| Total Items Found | 905 |
| Speedup Range | 1.9x - 587.3x |

## Comparison Benchmarks

These benchmarks measure **steady-state speedup** after warm cache is achieved.
**Note:** Cold start overhead (server startup) is measured separately. See README.md "Cold Start Analysis" section.

| Benchmark | Iterations | MCP Mean | Min | Max | StdDev | CV | Traditional | Steady-State Speedup |
|-----------|------------|----------|-----|-----|--------|-----|-------------|----------------------|
| **go list ./...** | 15 | 1.21ms | 1.14ms | 1.43ms | ±70.4µs | 5.8% | 316.50ms | **262.0x** |
| **grep -r** | 15 | 7.40ms | 6.11ms | 11.57ms | ±1.11ms | 14.9% | 126.32ms | **17.1x** |
| **go build** | 15 | 2.10ms | 1.89ms | 2.48ms | ±172.7µs | 8.2% | 1.23s | **587.3x** |

### Detailed Results

#### go list ./...

- **Iterations:** 15
- **MCP Mean:** 1.21ms (±70.4µs)
- **Range:** 1.14ms - 1.43ms
- **Traditional:** 316.50ms
- **Steady-State Speedup:** 262.0x faster
- **Note:** Speedup measured after warm cache. Cold start adds ~1.2s overhead (see README.md).
- **Variance (CV):** 5.8% - ✅ Very consistent
- **Items Found:** 119

#### grep -r

- **Iterations:** 15
- **MCP Mean:** 7.40ms (±1.11ms)
- **Range:** 6.11ms - 11.57ms
- **Traditional:** 126.32ms
- **Steady-State Speedup:** 17.1x faster
- **Note:** Speedup measured after warm cache. Cold start adds ~1.2s overhead (see README.md).
- **Variance (CV):** 14.9% - ⚠️ Moderate variance
- **Items Found:** 447

#### go build

- **Iterations:** 15
- **MCP Mean:** 2.10ms (±172.7µs)
- **Range:** 1.89ms - 2.48ms
- **Traditional:** 1.23s
- **Steady-State Speedup:** 587.3x faster
- **Note:** Speedup measured after warm cache. Cold start adds ~1.2s overhead (see README.md).
- **Why This Comparison:** While `go build` performs full compilation and
  `go_build_check` provides incremental type-checking, both serve the same
  user intent: **'check for errors after changes'**.
  go_build_check provides the instant feedback loop developers expect
  in their editor, making this the relevant comparison for interactive workflows.
- **Variance (CV):** 8.2% - ✅ Very consistent

## All Benchmarks

**Note:** Speedup factors shown are **steady-state** (warm cache). Cold start overhead is measured separately.

| Category | Benchmark | Duration | Iterations | Steady-State Speedup |
|----------|-----------|----------|------------|----------------------|
| Cold Start | Cold Start: Server to First Query | 750.62ms | 31 | 2.0x |
| Package Discovery | List Project Packages | 478.90ms | 1 | N/A |
| Package Discovery | Get Package API with Bodies | 389.5µs | 1 | N/A |
| Package Discovery | Get Package API Signatures Only | 326.4µs | 1 | N/A |
| Symbol Search | Symbol Search (Fuzzy) | 25.49ms | 1 | N/A |
| Symbol Search | Stdlib Package API | 1.89ms | 15 | 92.6x |
| Function Bodies | Function Bodies with Bodies (Learning Mode) | 413.7µs | 1 | N/A |
| Function Bodies | Function Signatures Only (Fast) | 269.9µs | 1 | N/A |
| Type Hierarchy | Find Interface Implementations | 76.98ms | 1 | N/A |
| Error Detection | Workspace Diagnostics | 45.04ms | 1 | N/A |
| File Reading | Read File via gopls | 2.81ms | 1 | N/A |
| Comparison | Comparison: go list ./... | 1.21ms | 15 | 262.0x |
| Comparison | Comparison: grep -r | 7.40ms | 15 | 17.1x |
| Comparison | Comparison: go build | 2.10ms | 15 | 587.3x |
| Code Navigation | Comparison: go_definition (via grep) | 40.73ms | 15 | 4.3x |
| Symbol Analysis | Comparison: go_symbol_references (via grep) | 82.42ms | 15 | 1.9x |
| Module Discovery | Comparison: list_modules (via go list -m) | 491.2µs | 15 | 66.2x |
| Dependency Analysis | Dependency Graph Generation | 493.7µs | 1 | N/A |

## Performance by Category

- **Cold Start:** 750.62ms average (1 tests)
- **Type Hierarchy:** 76.98ms average (1 tests)
- **File Reading:** 2.81ms average (1 tests)
- **Comparison:** 3.57ms average (3 tests)
- **Code Navigation:** 40.73ms average (1 tests)
- **Documentation:** 0ms average (0 tests - removed)
- **Package Discovery:** 159.87ms average (3 tests)
- **Symbol Search:** 13.69ms average (2 tests)
- **Function Bodies:** 341.8µs average (2 tests)
- **Error Detection:** 45.04ms average (1 tests)
- **Symbol Analysis:** 82.42ms average (1 tests)
- **Module Discovery:** 491.2µs average (1 tests)
- **Dependency Analysis:** 493.7µs average (1 tests)

## Variance Analysis

### Coefficient of Variation (CV)

CV = (StdDev / Mean) × 100%

| CV Range | Interpretation |
|-----------|----------------|
| < 10% | ✅ Very consistent, reliable |
| 10-25% | ⚠️ Moderate variance, acceptable |
| > 25% | ❌ High variance, results less reliable |

### Variance by Comparison Benchmark

| Benchmark | Iterations | StdDev | CV | Reliability |
|-----------|------------|--------|-----|-------------|
| Stdlib Package API | 15 | ±122.7µs | 6.5% | ✅ Very consistent |
| go list ./... | 15 | ±70.4µs | 5.8% | ✅ Very consistent |
| grep -r | 15 | ±1.11ms | 14.9% | ⚠️ Moderate variance |
| go build | 15 | ±172.7µs | 8.2% | ✅ Very consistent |
| go_definition (via grep) | 15 | ±4.25ms | 10.4% | ⚠️ Moderate variance |
| go_symbol_references (via grep) | 15 | ±4.93ms | 6.0% | ✅ Very consistent |
| list_modules (via go list -m) | 15 | ±38.0µs | 7.7% | ✅ Very consistent |

---

## How to Regenerate This Report

```bash
cd gopls/mcpbridge/test/benchmark
# Run benchmarks
go run benchmark_main.go -compare
# Generate report from JSON
go run reportgen/main.go benchmark_results.json
```

**Note:** This report was generated from `benchmark_results.json` on 2026-01-24 00:19:55
