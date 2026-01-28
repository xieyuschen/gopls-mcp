# gopls-mcp Benchmark Methodology

This document describes the technical methodology used by the benchmark suite to evaluate `gopls-mcp` performance.

## 1. Measurement Protocol

### 1.1 Timing Methodology

The benchmark suite uses Go's monotonic clock for precise duration measurements.

```go
// Each benchmark runs N iterations
stats := RunIterations(N, func() time.Duration {
    start := time.Now()  // Monotonic clock, nanosecond resolution
    res, err := session.CallTool(ctx, &mcp.CallToolParams{...})
    return time.Since(start)  // Wall-clock time measurement
})
```

**Measured Quantity:** Wall-clock time (includes I/O, parsing, serialization).
**Clock Source:** `time.Now()` (monotonic).
**Precision:** Nanosecond resolution.

### 1.2 Adaptive Sampling

To ensure statistical significance and handle variance (e.g., file system caching, CPU scheduling), the suite uses **Adaptive Sampling**:

1.  **Warmup Phase**: Runs 3 iterations (discarded) to populate the file system cache and stabilize the JVM/Go runtime.
2.  **Sampling Phase**: Runs iterations until the **Coefficient of Variation (CV)** drops below a target threshold (default 10-20%) or a maximum iteration count is reached (default 40).
3.  **Reporting**: Calculates Mean, Min, Max, and Standard Deviation.

### 1.3 What Is Measured

*   **Warm Cache Performance**: Most benchmarks measure steady-state performance.
*   **Cold Start**: A specific "Cold Start" benchmark measures the time from process spawn to the first successful query.

---

## 2. Benchmark Categories

The suite covers the following operational categories:

*   **Package Discovery**: Listing packages and retrieving API metadata.
*   **Symbol Search**: Fuzzy searching for symbols (vs `grep`).
*   **Error Detection**: Incremental diagnostic checks (vs `go build`).
*   **Documentation**: Retrieving symbol documentation (vs `go doc`).
*   **Code Navigation**: Jumping to definitions and finding references.
*   **Module Discovery**: Listing module dependencies.

---

## 3. Comparison Methodology

Where applicable, `gopls-mcp` tools are compared against traditional CLI equivalents to calculate a **Speedup Factor**.

### 3.1 Speedup Calculation

```
Speedup = Mean(Traditional Tool) / Mean(MCP Tool)
```

**Note:** This calculation assumes a "warm" state. For interactive workflows (like an IDE or AI agent), the persistent server model of `gopls-mcp` amortizes startup costs, whereas CLI tools pay the startup cost on every invocation.

### 3.2 Equivalent Operations

Direct comparisons are made where user intent is identical, even if the underlying operation differs slightly:

*   **`go_build_check` vs `go build`**:
    *   *User Intent*: "Check my code for errors."
    *   *Difference*: `go build` compiles the binary; `go_build_check` performs incremental type-checking.
    *   *Validity*: For the user's goal of validation, the comparison is valid.

*   **`go_search` vs `grep`**:
    *   *User Intent*: "Find this symbol."
    *   *Difference*: `grep` searches text; `go_search` queries the semantic index.

---

## 4. Test Environment

Specific hardware and software details (CPU, RAM, Go version) are **automatically captured** during the benchmark run and included in the generated report (`RESULTS.md`).