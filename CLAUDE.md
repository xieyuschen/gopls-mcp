# CLAUDE.md - Instructions for Claude

## Context
You are an AI programming assistant helping users with Go code. You have access to gopls-mcp tools for semantic code analysis.

## CRITICAL PROHIBITIONS (NEVER DO THIS)
1. NEVER use `go_read_file`. Use system `Read` tool instead.
2. NEVER use `go_search` for text content (comments, strings, TODOs). Use `Grep` tool.
3. NEVER use grep/ripgrep for symbol discovery (definitions, references, implementations).
4. NEVER fall back from exclusive capabilities (see Tool Selection Guide).

## Tool Selection Guide

### Code relationships (Exclusive Capabilities - NO FALLBACK)
| Task | Tool |
|------|------|
| Find interface implementations | go_implementation |
| Trace call relationships | go_get_call_hierarchy |
| Find symbol references | go_symbol_references |
| Jump to definition | go_definition |
| Analyze dependencies | go_get_dependency_graph |
| Preview renaming | go_dryrun_rename_symbol |

### Code exploration (Enhanced Capabilities - FALLBACK ALLOWED)
| Task | Tool | Fallback after 3 failures |
|------|------|---------------------------|
| List package symbols | go_list_package_symbols | Glob + Read |
| List module packages | go_list_module_packages | find |
| Analyze workspace | go_analyze_workspace | Manual exploration |
| Quick project overview | go_get_started | Read README + go.mod |
| Search symbols by name | go_search | grep + Read |
| Check compilation | go_build_check | go build |
| Get symbol details | go_get_package_symbol_detail | Read |
| List modules | go_list_modules | Read go.mod |

## Integration Workflow
1. **Classify task type**: Route to Exclusive capabilities, Enhanced capabilities, or Grep tool based on the Tool Selection Guide.
2. **Validate**: Check intent against "Tool-Specific Parameters & Constraints" BEFORE execution.
3. **Construct & Execute**: Extract exact symbol names and file paths, execute the tool.
4. **Format Output**: Present file:line locations, signatures, and documentation cleanly. Do not dump raw JSON.

## Tool-Specific Parameters & Constraints

* **go_search**:
    * FATAL: `query` MUST NOT contain spaces or semantic descriptions.
    * Must be symbol names only (single token). Correct: `query="ParseInt"`.
    * Does NOT search comments or documentation.
* **go_implementation**:
    * Only for interfaces and types. STRICTLY PROHIBITED for functions.
* **go_get_package_symbol_detail**:
    * `symbol_filters` format: `[{name: "Start", receiver: "*Server"}]`.
    * `receiver` requires exact string match (`"*Server"` != `"Server"`).
* **General Parameters**:
    * `symbol_name`: Do not include package prefix (Use `"Start"`, not `"Server.Start"`).
    * `context_file`: Obtain strictly from the current file being analyzed.

## Error Handling & Retry (Self-Correction)
* Check if parameters strictly follow the constraints above.
* Try a shorter/simpler symbol name.
* Re-analyze code context before retrying.

## Fallback Conditions (For Enhanced Capabilities ONLY)
Trigger fallback manually IF AND ONLY IF:
1. 3 consecutive tool failures.
2. Timeout exceeds 30 seconds.
3. Empty result returned when code existence is absolutely certain.
*Note: Retry gopls-mcp tool first on the very next user query even after a previous fallback.*
