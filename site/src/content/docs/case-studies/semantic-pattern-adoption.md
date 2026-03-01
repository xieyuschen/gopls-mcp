---
title: Semantic Pattern Adoption
sidebar:
  order: 2
---

This case study shows how gopls-mcp helps code AI agent to understand existing project patterns,
and adopt the same semantic logic.

### Background

In gopls-mcp implementation, we follow the same idea of `go_diagnostics` to implement `go_build_check` mcp tool.
We can ask claude to build check the project to ensure everything works.

Given a project where `main_test.go` exists (causing duplicate analysis) and `main.go` has an unused variable:

```go title="main.go" collapse={8-13}
package main

func main() {
	x := 42 // unused
}

// project structures:
// tree .
// .
// ├── CLAUDE.md
// ├── go.mod
// ├── main.go
// └── main_test.go
```


[gopls-mcp@b6de828f1](https://github.com/xieyuschen/gopls-mcp/commit/b6de828f1d44a19c4fdfd46271579ddd672fdda3) reports duplicated diagnostics for project above.

```ansi collapse={15-21}
[1;32m❯ check whether current project could be built? [0m

● I'll check if the current project builds successfully using the 
  semantic build check tool.
  ⎿ {
      [1;33m"diagnostics": [
        {
          "code_snippet": "x := 42 // unused",
          "column": 2,
          "file": "/home/xieyuschen/codespace/go-playground/main.go",
          "line": 4,
          "message": "declared and not used: x",
          "severity": "Error"
        },
        {
          "code_snippet": "x := 42 // unused",
          "column": 2,
          "file": "/home/xieyuschen/codespace/go-playground/main.go",
          "line": 4,
          "message": "declared and not used: x",
          "severity": "Error"
        }
      ], [0m
    ...
    }
```

This indicates that we missed some logic handled inside gopls.

### How gopls-mcp helps to solve this issue?

Enter claude, ensures `gopls-mcp` is available as claude mcp setting is different per project.

```ansi
[1;32m❯ You should use precise semantic analysis gopls-mcp server to understand
  current go project, see @CLAUDE.md for more info. 
  When i use gopls-mcp mcp tool go_build_check defined in 
  @gopls/mcpbridge/core/server.go, I got the dulicapted diagnostics.

  Native gopls diagnostics should handle this well, tell me how gopls does 
  it and use the same way to fix it. [0m
```

By using gopls-mcp tools, AI agent can focus on reasoning further code changes instead of excessively searching symbols and usages from huge amount of code lines.

We can highlight some notable changes for AI agent when it integrates with gopls-mcp.

#### Understand Implementation
The AI agent starts by searching for `handleGoDiagnostics` to understand how diagnostics are currently processed.

```ansi collapse={6-9}
● I'll help you fix the duplicate diagnostics issue in gopls-mcp. Let me first 
  search for the handleGoDiagnostics function to understand the current implementation.

[1;32m● gopls-mcp - go_search (MCP)[0m[2;32m(Cwd: "/home/xieyuschen/codespace/gopls-mcp-case-st[0m
[2;32m                             udy-mimic/gopls", query: "handleGoDiagnostics")[0m
[2;32m  ⎿ {                                                                      [0m
[2;32m      "summary": "Found 2 symbol(s):\n  - handleGoDiagnostics (Function in[0m
[2;32m    /home/xieyuschen/codespace/gopls-mcp-case-study-mimic/gopls/mcpbridge/c[0m
[2;32m    … +24 lines (ctrl+o to expand)[0m

● [33mRead(mcpbridge/core/gopls_wrappers.go)[0m
  ⎿  Read 1550 lines
```

#### Trace Native Behavior
To adapt gopls's behavior, the agent investigates `PackageDiagnostics` to see how the native implementation returns diagnostics.

```ansi collapse={6-10,16-20}
● Now let me look at how gopls handles diagnostics deduplication. I'll search for 
  the PackageDiagnostics function to understand how it returns diagnostics.

[1;32m● gopls-mcp - go_search (MCP)[0m[2;32m(Cwd: "/home/xieyuschen/codespace/gopls-mcp-case-st[0m
[2;32m                             udy-mimic/gopls", query: "PackageDiagnostics")[0m
[2;32m  ⎿ {                                                                      [0m
[2;32m      "summary": "Found 8 symbol(s):\n  - Snapshot.PackageDiagnostics (Meth[0m
[2;32m    od in /home/xieyuschen/codespace/gopls-mcp-case-study-mimic/gopls/inter[0m
[2;32m    … +92 lines (ctrl+o to expand)[0m

[1;32m● gopls-mcp - go_definition (MCP)[0m[2;32m(include_body: true, locator: {"symbol_name":"P[0m
[2;32m                                 ackageDiagnostics","context_file":"/home/xieyus[0m
[2;32m                                 chen/codespace/gopls-mcp-case-study-mimic/gopls[0m
[2;32m                                 /internal/cache/snapshot.go","kind":"method","l[0m
[2;32m                                 ine_hint":534,"parent_scope":"Snapshot"})[0m
[2;32m  ⎿ {                                                                      [0m
[2;32m      "summary": "Definition found at /home/xieyuschen/codespace/gopls-mcp-[0m
[2;32m    case-study-mimic/gopls/internal/cache/snapshot.go:534\n\n**Name**: `Pac[0m
[2;32m    … +58 lines (ctrl+o to expand)[0m
```

#### Analyze Diagnostics Flow
The agent continues by tracing `workspaceDiagnosticsHandler` and `summarizeDiagnostics` to see if deduplication happens at the workspace level.

```ansi collapse={6-10,19-23}
● Now let me search for how gopls's native workspace diagnostics handler 
  deduplicates diagnostics:

[1;32m● gopls-mcp - go_search (MCP)[0m[2;32m(Cwd: "/home/xieyuschen/codespace/gopls-mcp-case-st[0m
[2;32m                             udy-mimic/gopls", query:[0m
[2;32m                             "workspaceDiagnosticsHandler")[0m
[2;32m  ⎿ {                                                                      [0m
[2;32m      "summary": "Found 1 symbol(s):\n  - handler.workspaceDiagnosticsHandl[0m
[2;32m    er (Method in /home/xieyuschen/codespace/gopls-mcp-case-study-mimic/gop[0m
[2;32m    … +13 lines (ctrl+o to expand)[0m

● [33mRead(internal/mcp/workspace_diagnostics.go)[0m
  ⎿  Read 93 lines

● Let me check the summarizeDiagnostics function to see if there's deduplication 
  happening there:

[1;32m● gopls-mcp - go_search (MCP)[0m[2;32m(Cwd: "/home/xieyuschen/codespace/gopls-mcp-case-st[0m
[2;32m                             udy-mimic/gopls", query: "summarizeDiagnostics")[0m
[2;32m  ⎿ {                                                                      [0m
[2;32m      "summary": "Found 1 symbol(s):\n  - summarizeDiagnostics (Function in[0m
[2;32m     /home/xieyuschen/codespace/gopls-mcp-case-study-mimic/gopls/internal/m[0m
[2;32m    … +13 lines (ctrl+o to expand)[0m

● [33mRead(internal/mcp/file_diagnostics.go)[0m
  ⎿  Read 204 lines
```

#### Discover Deduplication Logic
The agent correctly identifies `CombineDiagnostics` as the place where gopls handles deduplication.

```ansi collapse={5-9,16-20}
● Let me look at the CombineDiagnostics function which might handle deduplication:

[1;32m● gopls-mcp - go_search (MCP)[0m[2;32m(Cwd: "/home/xieyuschen/codespace/gopls-mcp-case-st[0m
[2;32m                             udy-mimic/gopls", query: "CombineDiagnostics")[0m
[2;32m  ⎿ {                                                                      [0m
[2;32m      "summary": "Found 1 symbol(s):\n  - CombineDiagnostics (Function in /[0m
[2;32m    home/xieyuschen/codespace/gopls-mcp-case-study-mimic/gopls/internal/gol[0m
[2;32m    … +13 lines (ctrl+o to expand)[0m

[1;32m● gopls-mcp - go_definition (MCP)[0m[2;32m(include_body: true, locator:[0m
[2;32m                                 {"context_file":"/home/xieyuschen/codespace/gop[0m
[2;32m                                 ls-mcp-case-study-mimic/gopls/internal/golang/d[0m
[2;32m                                 iagnostics.go","kind":"function","line_hint":83[0m
[2;32m                                 ,"symbol_name":"CombineDiagnostics"})[0m
[2;32m  ⎿ {                                                                      [0m
[2;32m      "summary": "Definition found at /home/xieyuschen/codespace/gopls-mcp-[0m
[2;32m    case-study-mimic/gopls/internal/golang/diagnostics.go:83\n\n**Name**: `[0m
[2;32m    … +82 lines (ctrl+o to expand)[0m
```

#### Solution Pattern
The agent then explains the root cause and the solution derived from the codebase analysis:

```ansi
[1;32m❯ how does gopls solve this issue? [0m
  ⎿  ⧉ Selected 1 lines from mcpbridge/core/server.go in Visual Studio Code

● Based on my investigation, here\s how gopls solves the duplicate diagnostics issue:

  The Root Cause

  When a file belongs to multiple packages (e.g., a package with test files), 
  PackageDiagnostics returns the same diagnostic multiple times - once for each 
  package that includes the file.

  gopls\s Solution: CombineDiagnostics

  In gopls/internal/golang/diagnostics.go:83, gopls uses a map-based 
  deduplication approach:

  func CombineDiagnostics(tdiags []*cache.Diagnostic, adiags []*cache.Diagnostic) 
  []*cache.Diagnostic {
      // Build index using a composite key
      type key struct {
          Range   protocol.Range
          message string
      }
      ...
  }
```

#### Implement Fix
Finally, the agent applies the findings by attempting to update the code to use a map-based deduplication strategy, mirroring the `CombineDiagnostics` implementation found in `gopls/internal/golang/diagnostics.go:83`.

```ansi
● Now I understand the issue and the fix! Let me create a todo list for this fix:

● Now I\ll fix the handleGoDiagnostics function to deduplicate diagnostics using 
  the same approach as gopls:

● [33mUpdate(mcpbridge/core/gopls_wrappers.go)[0m
  ⎿  User rejected update to mcpbridge/core/gopls_wrappers.go
      389      return nil, nil, fmt.Errorf("diagnostics failed: %v",     
           err)                                                          
      390    }                                                           
      391                                                                
[31m      392 -  var diagnostics []api.Diagnostic                            [0m
[31m      393 -  var summary strings.Builder                                 [0m
[31m      394 -  total := 0                                                  [0m
[32m      392 +  // Deduplicate diagnostics using the same approach as gopls [0m
[32m      393 +  // Use a map with key based on range and message to         [0m
[32m          +identify duplicates                                           [0m
[32m      394 +  type key struct {                                           [0m
[32m      395 +    Range   protocol.Range                                    [0m
[32m      396 +    message string                                            [0m
[32m      397 +  }                                                           [0m
[32m      398 +  uniqueDiags := make(map[key]*cache.Diagnostic)              [0m
[32m      399 +                                                              [0m
      400    for _, diags := range reports {                             
      401      for _, diag := range diags {                              
[31m      402 -      total++                                                 [0m
[32m      402 +      k := key{diag.Range, diag.Message}                      [0m
[32m      403 +      if _, exists := uniqueDiags[k]; !exists {               [0m
[32m      404 +        uniqueDiags[k] = diag                                 [0m
[32m      405 +      }                                                       [0m
[32m      406 +    }                                                         [0m
[32m      407 +  }                                                           [0m
[32m      408 +                                                              [0m
[32m      409 +  var diagnostics []api.Diagnostic                            [0m
[32m      410 +  var summary strings.Builder                                 [0m
[32m      411 +  total := len(uniqueDiags)                                   [0m
[32m      412 +  for _, diag := range uniqueDiags {                          [0m
      413        // Convert DiagnosticSeverity to string                 
      414        severityStr := "Unknown"                                
      415        switch diag.Severity {
```

### Resources

[Raw Claude Thread](/semantic-pattern-adoption-raw-thread)
