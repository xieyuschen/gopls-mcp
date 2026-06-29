package task

// Suite is the default benchmark suite targeting the gopls-mcp codebase itself.
//
// Prompts are phrased as natural user questions without file-path hints or tool
// names, so each agent freely chooses its approach (grep/read vs. semantic tools).
//
// Tasks are grouped by capability (definition / references / implementation) and
// then by cross-boundary difficulty (same-package / cross-package / cross-project).
var Suite = []Task{

	// ── go_definition: same-package ───────────────────────────────────────────
	{
		Name:        "def-same-pkg",
		Description: "Find the full definition of the Handler struct (same package)",
		Prompt: `Look at the Go codebase in this directory.

Find the definition of the Handler struct that implements the gopls-mcp MCP tools.
List all its fields with their names and types.
Report the exact file path and line number where the struct is defined.`,
		Tags: []Tag{TagDefinition, TagSamePackage},
		// handlers.go:29; key fields: initFn, config, session, symbler, watcher
		Checker: Checker{MustContain: []string{
			"handlers.go", "29",
			"initFn", "session", "symbler", "watcher",
		}},
	},

	// ── go_definition: cross-package (same repo) ─────────────────────────────
	{
		Name:        "def-cross-pkg",
		Description: "Find MCPConfig definition from its use in execute.go (cross-package, same repo)",
		Prompt: `Look at the Go codebase in this directory.

Somewhere in the pkg sub-package there is a variable whose type is MCPConfig,
referenced via a package qualifier.

1. Find the line where MCPConfig is first used.
2. Find the full definition of MCPConfig: the exact file, line number, and all
   its fields with types. The definition is not in the same file as the usage.`,
		Tags: []Tag{TagDefinition, TagCrossPackage},
		// config.go:12; fields: Gopls, Workdir, MaxResponseBytes, IdleTimeout
		Checker: Checker{MustContain: []string{
			"config.go", "12",
			"Gopls", "Workdir", "MaxResponseBytes",
		}},
	},

	// ── go_definition: cross-project (stdlib) ────────────────────────────────
	{
		Name:        "def-cross-stdlib",
		Description: "Find io.Closer definition (stdlib) from its use as a field in handlers.go",
		Prompt: `Look at the Go codebase in this directory.

In the Handler struct there is a field whose type is io.Closer.

1. Find that field (name it and give its line number).
2. Find the full definition of io.Closer: which package declares it, what methods
   does it require, and what is the exact file path (inside the Go standard library
   installation on this machine) where it is defined?`,
		Tags: []Tag{TagDefinition, TagCrossProject},
		// watcher field at handlers.go:43; io.Closer has Close() error
		Checker: Checker{MustContain: []string{
			"watcher", "io.Closer", "Close() error",
		}},
	},

	// ── go_definition: cross-project (third-party) ───────────────────────────
	{
		Name:        "def-cross-third-party",
		Description: "Find mcp.CallToolRequest definition (third-party go-sdk) from its use in handlers",
		Prompt: `Look at the Go codebase in this directory.

Several handler functions in the core package accept a parameter of type
mcp.CallToolRequest (pointer).

1. Find one such handler function and note the line where CallToolRequest appears.
2. Find the full definition of CallToolRequest: which external module and package
   declares it, what are all its fields and their types, and where exactly is the
   source file on this machine?`,
		Tags: []Tag{TagDefinition, TagCrossProject},
		// defined in github.com/modelcontextprotocol/go-sdk/mcp
		Checker: Checker{MustContain: []string{
			"modelcontextprotocol", "CallToolRequest",
		}},
	},

	// ── go_symbol_references: same-package ───────────────────────────────────
	{
		Name:        "ref-same-pkg",
		Description: "Find all call sites of resolveSymbol (same package)",
		Prompt: `Look at the Go codebase in this directory.

Find every place in the codebase that calls the function resolveSymbol.
For each call site list: file path, line number, and the name of the enclosing function.`,
		Tags: []Tag{TagReferences, TagSamePackage},
		// symbol_resolution.go:36 inside symbolLocation
		Checker: Checker{MustContain: []string{
			"symbol_resolution.go", "36", "symbolLocation",
		}},
	},

	// ── go_symbol_references: cross-package (same repo) ──────────────────────
	{
		Name:        "ref-cross-pkg",
		Description: "Find all call sites of RegisterTools across packages",
		Prompt: `Look at the Go codebase in this directory.

Find every place in the codebase that calls the function RegisterTools.
For each call site list: file path, line number, and the name of the enclosing function.
Note: the function is defined in one package but may be called from a different package.`,
		Tags: []Tag{TagReferences, TagCrossPackage},
		// execute.go:249 inside Execute()
		Checker: Checker{MustContain: []string{
			"execute.go", "249", "Execute",
		}},
	},

	// ── go_symbol_references: cross-project (third-party) ────────────────────
	{
		Name:        "ref-cross-third-party",
		Description: "Find all call sites of mcp.AddTool (from third-party go-sdk) in this codebase",
		Prompt: `Look at the Go codebase in this directory.

The function AddTool comes from the external MCP SDK package
(github.com/modelcontextprotocol/go-sdk/mcp).

Find every place in this codebase that calls mcp.AddTool.
For each call site list: file path, line number, and the enclosing function name.`,
		Tags: []Tag{TagReferences, TagCrossProject},
		// tool.go:62 (Register method) + mcp.go:273,278,… (multiple sites)
		// Check for at least the mcpbridge call site and one mcp.go site.
		Checker: Checker{MustContain: []string{
			"tool.go", "62", "mcp.go",
		}},
	},

	// ── go_implementation: same-package ──────────────────────────────────────
	{
		Name:        "impl-same-pkg",
		Description: "Find all types implementing the Tool interface (same package)",
		Prompt: `Look at the Go codebase in this directory.

There is a Tool interface somewhere in this codebase.
Find every concrete type that implements this interface.
List each type with its file path and package.`,
		Tags: []Tag{TagImplementation, TagSamePackage},
		// GenericTool[In, Out] in tool.go is the sole implementation
		Checker: Checker{MustContain: []string{
			"GenericTool", "tool.go",
		}},
	},

	// ── go_implementation: cross-package (same repo) ─────────────────────────
	{
		Name:        "impl-cross-pkg",
		Description: "Find all types implementing the Symbler interface (defined in core, impl in pkg)",
		Prompt: `Look at the Go codebase in this directory.

There is an interface named Symbler defined in the core package.
Find every concrete type in this codebase that implements it.
List each type with its file path and package.
Note: the implementation may live in a different package from where the interface is defined.`,
		Tags: []Tag{TagImplementation, TagCrossPackage},
		// minimalServer in gopls/mcpbridge/pkg/lsp_wrapper.go
		Checker: Checker{MustContain: []string{
			"minimalServer", "lsp_wrapper.go",
		}},
	},

	// ── go_get_call_hierarchy ─────────────────────────────────────────────────
	{
		Name:        "call-hierarchy",
		Description: "Trace who calls formatReferences and what it calls",
		Prompt: `Look at the Go codebase in this directory.

For the function named formatReferences:
1. List every function that calls it (its callers), with file path and line number.
2. List every function that it calls (its callees), with file path and line number.`,
		Tags: []Tag{TagCallHierarchy},
		// references_formatter.go:63; callee is formatReferenceItems
		// Two formatReferences exist; at least the mcpbridge one must be found.
		Checker: Checker{MustContain: []string{
			"references_formatter.go", "formatReferenceItems",
		}},
	},

	// ── go_get_dependency_graph ───────────────────────────────────────────────
	{
		Name:        "dependency-graph",
		Description: "Map the dependency graph of the core package",
		Prompt: `Look at the Go codebase in this directory.

Find the package named "core" inside the mcpbridge directory.
Produce a dependency map of that package:
- Which packages does it import directly?
Group results by: stdlib / internal-to-this-module / external.`,
		Tags: []Tag{TagDependency},
		// must mention the key external dep and at least one internal path
		Checker: Checker{MustContain: []string{
			"modelcontextprotocol", "golang.org/x/tools",
		}},
	},
}

// All returns a copy of the full built-in suite.
func All() []Task { return append([]Task(nil), Suite...) }

// ByName looks up a task by name. Returns the task and true if found.
func ByName(name string) (Task, bool) {
	for _, t := range Suite {
		if t.Name == name {
			return t, true
		}
	}
	return Task{}, false
}
