---
title: Reference
sidebar:
  order: 1
---
### `go_list_modules`

> List current module and direct dependencies only. Returns module paths only (no packages). By default, transitive dependencies are excluded. Set direct_only=false to show all dependencies including transitive ones. Use this to understand the module structure before exploring packages.

list_modules allows the LLM to get all module dependencies of the current project.

This MCP tool offers a comprehensive overview of existing go.mod usage,
it covers the direct and indirect dependencies and a semantic understanding of go.mod directives.
The tool offers a roadmap of current project by separating scopes to internal submodules and third-party modules.

LLM should use this MCP tool to retrieve a well-structured roadmap for dependencies instead of reading go.mod by itself.
LLM should be careful that the third-party modules might be huge depending on current modules.


### `go_list_module_packages`

> List all packages in a given module. Returns package names and optionally documentation. Use this to discover packages within a module before exploring symbols.

list_module_packages allows the LLM to get all packages of a given module.

This MCP tool offers a semantically accurate, fast, and token-saving way to discover packages.

Standard LLM workflows require multiple turns of 'ls' and file readings to finally understand the package structure.
The pain points of this procedure are:

(1) Attention Dilution: The output of raw file readings dilutes the LLM's attention—and attention is a scarce resource.
(2) Token Cost: It forces the model to consume tokens proportional to file length, just to find a package name.
(3) Low Precision: The understanding remains at a textual level, not a semantic-level accurate overview.

This tool offers a comprehensive and fast overview of module packages.
It effectively prevents the attention dilution and heavy token cost caused by reading files.
It performs best when you want to understand the module structure without looking into code details.

LLM should use this tool to get package information instead of reading project files manually.
It preserves the LLM's attention context, eliminates textual analysis hallucinations, and significantly saves tokens.


### `go_list_package_symbols`

> List all exported symbols (types, functions, constants, variables) in a package. Returns Symbol objects with name, kind, signature, receiver, documentation, and optional bodies. Use include_docs=true for documentation and include_bodies=true for function implementations. Use this to explore a package's API surface before diving into specific symbols with get_package_symbol_detail.

list_package_symbols allows the LLM to get all exported symbols of a given package.

This MCP tool offers a semantically accurate, fast, and token-saving way to discover package APIs.

Standard LLM workflows require multiple turns of file readings and textual search to understand the symbols defined in a package.
The pain points of this procedure are:

(1) Attention Dilution: The output of full file content dilutes the LLM's attention with implementation details.
(2) Token Cost: It forces the model to consume tokens for function bodies and comments, just to find function signatures.
(3) Low Precision: The understanding relies on textual matching, which often misses cross-file definitions or struct methods.

This tool offers a comprehensive and structural overview of package symbols.
It effectively prevents the attention dilution and heavy token cost caused by reading entire files.
It performs best when you want to understand the package interface or locate specific symbols without reading the implementation.

LLM should use this tool to get symbol information instead of reading package files manually.
It preserves the LLM's attention context, eliminates textual analysis hallucinations, and significantly saves tokens.


### `go_get_package_symbol_detail`

> Get detailed symbol information from a package. Returns Symbol objects with name, kind, signature, receiver (for methods), parent (for fields), documentation, and optional bodies. Symbol filters are REQUIRED - provide symbol_filters to retrieve specific symbols by name and receiver (e.g., [{name: "Start", receiver: "*Server"}]). For methods, receiver matching uses exact string match (e.g., "*Server" != "Server"). Use include_docs=true for documentation and include_bodies=true for function implementations. Use list_package_symbols to get all symbols in a package.

get_package_symbol_detail is a precision tool to get specific symbol details only.

This MCP tool offers lightweight, fast and fine-grained details for interested symbols.

Standard LLM workflows go through every file and do textual search to get selected symbol details,
the pain points are:

(1) Attention Dilution: it shares the same problem of list_package_symbols,
and after context grows, the LLM loses the attention of previous full list_package_symbols responses,
and requires another work for interested symbols, and this further dilutes the attention in a long thread.
The frequent symbol finding populates the context.
(2) Token Cost: It forces the model to consume tokens for function bodies and comments just to find several symbols only.
(3) Low Precision: The understanding relies on textual matching, which often misses cross-file definitions or struct methods.

LLM should use this tool to get symbol details of package instead of reading package files manually.


### `go_build_check`

> Check for compilation and type errors. FAST: uses incremental type checking (faster than 'go build'). Use this to verify code correctness and populate the workspace cache for other tools. Returns detailed error information with file/line/column.

go_build_check allows the LLM to check syntax and type errors in a lightweight and fast way.

Standard LLM workflows verify code change works by triggering 'go build' by shell command
to ensure syntax and type usages are correct.
However, 'go build' is too heavy in this case because it generates the real executable code we don't need.
This slows down the feedback to the LLM and user-facing latency is high.

This MCP tool offers a lightweight tool to satisfy the LLM verification requirement after code change,
by eliminating compiler backend code generation, it offers a faster tool to meet the requirement.

LLM should use this tool to verify code change instead of using 'go build' to make the latency smaller
to gain a fluent user experience.


### `go_search`

> Find symbols (functions, types, constants) by name with fuzzy matching. Use this when user knows part of a symbol name but not the full name or location. Returns rich symbol information (name, kind, file, line) for fast exploration.

go_search allows the LLM to fuzzy find symbols by name.

This MCP tool offers a flexible way to locate symbols when the exact name is unknown.

Standard LLM workflows might resort to 'grep' or repetitive file reading to find symbols.
The pain points of this procedure are:

(1) Attention Dilution: Grep output can be noisy and lack context.
(2) Token Cost: Reading many files to find a symbol is expensive.
(3) Low Precision: Textual search lacks semantic understanding.

This tool uses gopls's index for fast and relevant results.

OUTPUT:
  Returns rich Symbol information including:
  - name: Symbol name
  - kind: Symbol kind (function, method, struct, interface, etc.)
  - file_path: Source file path
  - line: Line number where symbol is defined
  - signature: NOT included (use go_definition for full details)
  - doc: NOT included (use go_definition for full details)
  - body: NOT included (use go_definition for full details)

NOTE: For full signature and documentation, use go_definition on the specific symbol.
This tool is optimized for fast exploration and discovery.

LLM should use this tool when exploring the codebase or looking for specific functionality.


### `go_symbol_references`

> Find all usages of a symbol across the codebase using semantic location (symbol name, package, scope). Use this before refactoring to assess impact or to understand how a symbol is used. REPLACES: grep + manual file reading for finding references.

go_symbol_references allows the LLM to get all usages of a symbol under current project.

Standard LLM workflows cannot handle this as it requires textual matches over almost every file under the current project.
It greatly dilutes LLM attention and costs a lot of tokens, and it's quite slow due to the nature of IO and matching.

This MCP tool offers fast, lightweight and accurate results to report all usages of a given symbol,
and it can fully prevent LLM mixed the similar symbols as the same one due to textual matching nature.

It makes the LLM get rid of tedious and heavy file reading and matching workflow,
and makes the LLM focus on the user assignments instead of getting lost in retrieving information.

LLM should use this tool to retrieve symbol usages to understand how to use a symbol,
evaluate the impacts of a symbol, and so on. LLM should never consider doing this stuff by itself.

INPUT FORMAT:
  Use semantic locator instead of error-prone file/line/column numbers:
  - symbol_name: The symbol name to find references for (e.g., "HandleRequest", "Start")
  - context_file: File where the symbol is used (absolute path)
  - package_name: Package import alias (e.g., "http", "fmt") - optional
  - parent_scope: Receiver type for methods (e.g., "*Server") - optional
  - kind: Symbol kind ("function", "method", "variable", "const") - optional
  - line_hint: Approximate line number for disambiguation - optional

EXAMPLE:
  Find all references to a function:
  {
    "locator": {
      "symbol_name": "HandleRequest",
      "context_file": "/path/to/handler.go",
      "kind": "function"
    }
  }

OUTPUT:
  Returns both:
  - references: List of reference locations (file, line, column)
  - symbols: Rich symbol information (signature, documentation, snippet) for the referenced symbol


### `go_dryrun_rename_symbol`

> Preview a symbol rename operation across all files (DRY RUN - no changes are applied). Use go_symbol_references first to assess impact, then use this to preview the exact changes that would be made. Returns a unified diff showing all proposed modifications.

go_dryrun_rename_symbol allows the LLM to get semantic-level suggestions for rename symbol action.

Standard LLM renaming workflows is an evolving process; it consistently finds symbols by textual matches,
renames them and runs 'go build' to ensure whether it succeeds. If not, it will check the failure and repeat until succeeds.
This causes unnecessary context growth and lacks the ability to give the user a correct and accurate overview of renaming impact.
Besides, it dilutes attention and costs tokens.

This MCP tool offers a fast, dry-run output for the LLM to understand all necessary changes for a renaming action.
It's fast and semantically correct.

LLM should use this tool to evaluate renaming impacts and based on the results to rename,
instead of trying to analyze and change the findings by itself.


### `go_implementation`

> Find all implementations of an interface or all interfaces implemented by a type using semantic location (symbol name, package, scope). Use this to understand type hierarchies, find all implementations of an interface, or discover design patterns in the codebase. REPLACES: grep + manual file reading for interface implementations.

go_implementation allows the LLM to retrieve implementation of an interface.

Standard LLM is blind when trying to find implementations of an interface due to Go interface nature.
It goes through files and textual matches structures. As interface implementation is type sensitive
and the implementation may be put in different package files, LLM needs a lot of attention and tokens
to handle it but output an incomplete or wrong result.

This MCP tool offers a fast, accurate and correct output to find implementations of an interface.
LLM MUST use this MCP tool rather than trying to find implementations by itself.

INPUT FORMAT:
  Use semantic locator instead of error-prone line/column numbers:
  - symbol_name: The interface or type name (e.g., "Handler", "Server")
  - context_file: File where the symbol is used (absolute path)
  - package_name: Package import alias (e.g., "http", "fmt") - optional
  - parent_scope: Receiver type for methods (e.g., "*Server") - optional
  - kind: Symbol kind ("interface", "type") - optional, helps disambiguate
  - line_hint: Approximate line number for disambiguation - optional

OUTPUT FORMAT:
  Returns a list of symbols with rich information:
  - name: The implementing type or method name (e.g., "FileWriter", "Write")
  - signature: Full function signature (e.g., "func (f *FileWriter) Write(data string) error")
  - file_path: Absolute path to the implementation file
  - line: Starting line number
  - kind: Symbol kind ("struct", "interface", "method", etc.)
  - doc: Documentation comment (if available)
  - body: Full code snippet of the implementation (HERO field)

COMMON PITFALLS:
  ✗ Standard library interfaces: io.Reader, error, fmt.Stringer don't work
    → Only interfaces defined in your codebase are supported
  ✗ Wrong context_file: Pointing to usage instead of definition
    → Point context_file to where the interface is defined
  ✗ Missing parent_scope for methods
    → For interface methods, set parent_scope to the interface name
  ✗ Empty result doesn't mean error
    → Check if the interface actually has implementations

EXAMPLES:
  1. Find implementations of an interface:
  {
    "locator": {
      "symbol_name": "Handler",
      "context_file": "/path/to/interfaces.go",
      "kind": "interface"
    }
  }
  → Returns: FileHandler, MemoryHandler, MockHandler, etc.

  2. Find implementations of an interface method:
  {
    "locator": {
      "symbol_name": "ServeHTTP",
      "parent_scope": "Handler",
      "kind": "method",
      "context_file": "/path/to/handler.go"
    }
  }
  → Returns: All ServeHTTP method implementations

  3. Find what interfaces a type implements:
  {
    "locator": {
      "symbol_name": "FileWriter",
      "context_file": "/path/to/file.go",
      "kind": "struct"
    }
  }
  → Returns: Writer, io.Closer, etc.


### `go_read_file`

> Read file content through gopls. SLOWER: reads full file from disk. Use this when you need to see actual code or implementation details. Note: unsaved editor changes not included.

go_read_file offers a lightweight file reading without involving disk IO.

As this MCP server natively loads files into memory and does further analysis, it offers a faster way to read file content.


### `go_definition`

> Jump to the definition of a symbol using semantic location (symbol name, package, scope). REPLACES: grep + manual file reading. Use this when you see a function call or type reference and need to find where it's defined. Faster and more accurate than text search - uses type information from gopls.

go_definition offers LSP feature to return function declaration details.
It's designed for LLM to use during reading code, unlike get_package_symbol_detail which is used for exploring.

Standard LLM workflow to read code requires to keep locating and loading files to understand function calls across a lot of packages.
This requires to frequently locate and load files and parse them via a textual matching way.

This MCP tool offers an approach of retrieving precise definitions when the LLM focuses on code reading in a file.
It doesn't require the LLM to quickly switch via multiple files to understand functions and symbols it needs to proceed.

LLM should use it to get more precise information during reading code, instead of reading files and parsing them by the LLM itself.

The tool uses SymbolLocator for LLM-friendly input:
- symbol_name: The exact name of the symbol (e.g., "HandleRequest", not "fmt.Println")
- context_file: The absolute path of the file where you see the symbol
- package_name: Optional import alias if the symbol is imported (e.g., "fmt", "json")
- parent_scope: Optional enclosing scope (receiver type for methods, function name for local variables)
- kind: Optional symbol kind filter ("function", "method", "struct", "interface", "variable", "const")
- line_hint: Optional approximate line number for disambiguation (treated as fuzzy search hint)

Example: To find definition of "Add" function call in main.go:
{
  "symbol_name": "Add",
  "context_file": "/path/to/main.go"
}


### `go_get_call_hierarchy`

> Get the call hierarchy for a function using semantic location (symbol name, package, scope). Returns both incoming calls (what functions call this one) and outgoing calls (what functions this one calls). Use this to understand code flow, debug call chains, and trace execution paths through the codebase. REPLACES: grep + manual file reading for call graph analysis.

get_call_hierarchy allows the LLM to explore the call graph of a function.

This MCP tool offers a powerful way to understand code flow and dependencies.

Standard LLM workflows struggle to trace call chains across multiple files.
The pain points of this procedure are:

(1) Attention Dilution: Manually tracking calls requires keeping many files in context.
(2) Token Cost: Reading all involved files consumes significant tokens.
(3) Complexity: It is hard to maintain a mental model of deep call stacks.

This tool provides a structured view of incoming and outgoing calls.

LLM should use this tool to debug, refactor, or understand complex logic.

INPUT FORMAT:
  Use semantic locator instead of error-prone line/column numbers:
  - symbol_name: The function name (e.g., "HandleRequest", "Main")
  - context_file: File where the function is called or defined (absolute path)
  - package_name: Package import alias (e.g., "http") - optional
  - parent_scope: Receiver type for methods (e.g., "*Server") - optional
  - kind: Symbol kind ("function", "method") - optional, helps disambiguate
  - line_hint: Approximate line number for disambiguation - optional

DIRECTION:
  - "incoming": Show what functions call this function
  - "outgoing": Show what this function calls
  - "both" (default): Show both directions

EXAMPLE:
  Get call hierarchy for a function:
  {
    "locator": {
      "symbol_name": "HandleRequest",
      "context_file": "/path/to/server.go",
      "parent_scope": "*Server",
      "kind": "method"
    },
    "direction": "both"
  }


### `go_analyze_workspace`

> Analyze the entire workspace to discover packages, entry points, and dependencies. Use this when exploring a new codebase to understand the project structure, find main packages, API endpoints, and get a comprehensive overview of the codebase.

analyze_workspace allows the LLM to get a comprehensive overview of the project.

This MCP tool offers a high-level map of packages, entry points, and dependencies.

Standard LLM workflows lack a good way to "see" the whole project at once.
The pain points of this procedure are:

(1) Exploration Overhead: It takes many steps to discover the project structure manually.
(2) Missing Context: It is easy to miss important parts of the codebase.

This tool provides a starting point for exploration.

LLM should use this tool when first encountering a new codebase.


### `go_get_started`

> Get a beginner-friendly guide to start exploring the Go project. Returns project identity, quick stats, entry points, package categories, and recommended next steps. Use this when you're new to a codebase and want to understand where to start.

get_started allows the LLM to get a beginner-friendly guide to the project.

This MCP tool offers curated information to help the LLM (and user) hit the ground running.

Standard LLM workflows require the user to explain the project or the LLM to guess where to start.
The pain points of this procedure are:

(1) Ramp-up Time: It takes time to figure out the "what" and "how" of a project.
(2) Lack of Direction: Without guidance, exploration can be aimless.

This tool provides a structured introduction.

LLM should use this tool to orient itself and the user.


### `go_get_dependency_graph`

> Get the dependency graph for a package. Returns both dependencies (packages it imports) and dependents (packages that import it). Use this to understand architectural relationships, analyze coupling, and visualize the package's place in the codebase.

get_dependency_graph allows the LLM to visualize package dependencies.

This MCP tool offers a clear view of how packages relate to each other.

Standard LLM workflows can infer dependencies from imports but lack a holistic view.
The pain points of this procedure are:

(1) Limited Scope: It is hard to see the "big picture" of architecture from individual files.
(2) Coupling Analysis: Detecting cycles or tight coupling is difficult manually.

This tool provides a graph structure of dependencies.

LLM should use this tool to understand architecture and impact of changes.


