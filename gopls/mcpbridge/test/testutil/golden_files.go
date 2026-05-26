package testutil

// Golden file names used across tests.
// Grouped by functionality for better organization.
//
// Naming convention:
// - MCP tool tests: <mcp_tool>_<case>.golden (e.g., go_definition_e2e_strong.golden)
// - Corner cases (non-MCP-tool tests): etc<description>.golden
//
// To add a new golden file:
// 1. Add it to the appropriate group below
// 2. Run tests to verify no duplicates (TestGoldenFileUnique)

const (
	// ===== MCP Tool Tests =====
	// All constants below follow the pattern: <mcp_tool>_<case>.golden
	// where <mcp_tool> is the actual MCP tool name from core/server.go

	// Meta tool
	GoldenListTools = "go_list_tools_e2e.golden"

	// Navigation Tools (go_definition, go_implementation, go_symbol_references)
	// go_definition - basic functionality
	GoldenDefinitionExactLocation    = "go_definition_exact_location.golden"
	GoldenDefinitionTypeDefinition   = "go_definition_type_definition.golden"
	GoldenDefinitionMethodDefinition = "go_definition_method_definition.golden"
	GoldenDefinitionImportStatement  = "go_definition_import_statement.golden"
	GoldenDefinitionInvalidPosition  = "go_definition_invalid_position.golden"
	GoldenDefinitionNoSymbol         = "go_definition_no_symbol.golden"
	// go_definition - cross-file
	GoldenDefinitionCrossFileFunction = "go_definition_cross_file_function.golden"

	// go_implementation
	GoldenImplementationInterface       = "go_implementation_interface.golden"
	GoldenImplementationInterfaceByType = "go_implementation_interface_by_type.golden"
	GoldenImplementationMethod          = "go_implementation_method.golden"

	// go_symbol_references
	GoldenSymbolReferencesExactCount   = "go_symbol_references_exact_count.golden"
	GoldenSymbolReferencesCrossFile    = "go_symbol_references_cross_file.golden"
	GoldenSymbolReferencesNoReferences = "go_symbol_references_no_references.golden"
	GoldenSymbolReferencesType         = "go_symbol_references_type.golden"

	// Call Hierarchy Tool (go_get_call_hierarchy) — no golden files (table-driven, uses "")

	// Dependency Graph Tool (go_get_dependency_graph)
	GoldenDependencyGraphBasic        = "go_get_dependency_graph_basic_functionality.golden"
	GoldenDependencyGraphComplex      = "go_get_dependency_graph_complex_scenarios.golden"
	GoldenDependencyGraphDependents   = "go_get_dependency_graph_dependents.golden"
	GoldenDependencyGraphError        = "go_get_dependency_graph_error_handling.golden"
	GoldenDependencyGraphOutputFormat = "go_get_dependency_graph_output_format.golden"
	GoldenDependencyGraphStdlib       = "go_get_dependency_graph_stdlib_packages.golden"
	GoldenDependencyGraphTransitive   = "go_get_dependency_graph_transitive_dependencies.golden"

	// Rename Symbol Tool (go_dryrun_rename_symbol)
	GoldenRenameSymbolExact     = "go_dryrun_rename_symbol_exact_count.golden"
	GoldenRenameSymbolMultiFile = "go_dryrun_rename_symbol_multi_file.golden"
	GoldenRenameSymbolType      = "go_dryrun_rename_symbol_type.golden"

	// ===== Corner Cases & Special Scenarios =====
	// These tests don't directly correspond to a single MCP tool
	// They test specific language features, workflows, or edge cases

	// Generics Support (cross-cutting tests for all tools with generics)
	GoldenGenericsSupport       = "etc_generics_support.golden"
	GoldenInterfaceSatisfaction = "etc_interface_satisfaction.golden"

	// Rename edge cases and complex scenarios
	GoldenRenameEdgeCases        = "etc_rename_edge_cases.golden"
	GoldenComplexRenameScenarios = "etc_complex_rename_scenarios.golden"

	// Error Scenarios
	GoldenErrorHandling = "etc_error_handling_e2e.golden"

	// File Watching
	GoldenFileWatching = "etc_file_watching_e2e.golden"
)
