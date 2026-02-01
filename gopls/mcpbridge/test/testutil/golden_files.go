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

	// Discovery Tools (go_list_*)
	GoldenListModules                     = "go_list_modules_e2e.golden"
	GoldenListModulePackages              = "go_list_module_packages_e2e.golden"
	GoldenListPackageSymbols              = "go_list_package_symbols_e2e.golden"
	GoldenListPackageSymbolsComprehensive = "go_list_package_symbols_e2e_comprehensive.golden"
	GoldenListPackageSymbolsTestFiles     = "go_list_package_symbols_test_files_e2e.golden"
	GoldenListTools                       = "go_list_tools_e2e.golden"
	GoldenAnalyzeWorkspace                = "go_analyze_workspace_e2e.golden"
	GoldenGetPackageSymbolDetail          = "go_get_package_symbol_detail_e2e.golden"
	GoldenGetStarted                      = "go_get_started_e2e.golden"

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
	GoldenDefinitionCrossFileType     = "go_definition_cross_file_type.golden"
	GoldenDefinitionCrossFileMethod   = "go_definition_cross_file_method.golden"

	// go_implementation
	GoldenImplementationInterface       = "go_implementation_interface.golden"
	GoldenImplementationInterfaceByType = "go_implementation_interface_by_type.golden"
	GoldenImplementationMethod          = "go_implementation_method.golden"

	// go_symbol_references
	GoldenSymbolReferencesExactCount   = "go_symbol_references_exact_count.golden"
	GoldenSymbolReferencesCrossFile    = "go_symbol_references_cross_file.golden"
	GoldenSymbolReferencesNoReferences = "go_symbol_references_no_references.golden"
	GoldenSymbolReferencesType         = "go_symbol_references_type.golden"
	GoldenSymbolReferencesTests        = "go_symbol_references_test_files_e2e.golden"

	// Call Hierarchy Tool (go_get_call_hierarchy)
	// Note: Each subtest has its own golden file to avoid overwrites
	GoldenCallHierarchyBasicBothDirections       = "go_get_call_hierarchy_basic_both_directions.golden"
	GoldenCallHierarchyBasicIncomingOnly         = "go_get_call_hierarchy_basic_incoming_only.golden"
	GoldenCallHierarchyBasicOutgoingOnly         = "go_get_call_hierarchy_basic_outgoing_only.golden"
	GoldenCallHierarchyBasicDefaultDirection     = "go_get_call_hierarchy_basic_default_direction.golden"
	GoldenCallHierarchyComplexMultipleCallers    = "go_get_call_hierarchy_complex_multiple_callers.golden"
	GoldenCallHierarchyComplexCallChain          = "go_get_call_hierarchy_complex_call_chain.golden"
	GoldenCallHierarchyError                     = "go_get_call_hierarchy_error_handling.golden"
	GoldenCallHierarchyInterface                 = "go_get_call_hierarchy_interface_methods.golden"
	GoldenCallHierarchyMultipleCallsSameCaller   = "go_get_call_hierarchy_multiple_calls_same_caller.golden"
	GoldenCallHierarchyMultipleCallsDifferent    = "go_get_call_hierarchy_multiple_calls_different.golden"
	GoldenCallHierarchyMultipleFilesCrossFile    = "go_get_call_hierarchy_multiple_files_cross_file.golden"
	GoldenCallHierarchyMultipleFilesCrossPackage = "go_get_call_hierarchy_multiple_files_cross_package.golden"
	GoldenCallHierarchyOutputFormat              = "go_get_call_hierarchy_output_format.golden"
	GoldenCallHierarchySpecialCasesRecursive     = "go_get_call_hierarchy_special_cases_recursive.golden"
	GoldenCallHierarchySpecialCasesDefer         = "go_get_call_hierarchy_special_cases_defer.golden"
	GoldenCallHierarchySpecialCasesGoroutine     = "go_get_call_hierarchy_special_cases_goroutine.golden"
	GoldenCallHierarchyStdlibCalls               = "go_get_call_hierarchy_stdlib_calls.golden"
	GoldenCallHierarchyStructMethodsValue        = "go_get_call_hierarchy_struct_methods_value.golden"
	GoldenCallHierarchyStructMethodsPointer      = "go_get_call_hierarchy_struct_methods_pointer.golden"
	GoldenCallHierarchyStructMethodsMethodCalls  = "go_get_call_hierarchy_struct_methods_method_calls.golden"

	// Dependency Graph Tool (go_get_dependency_graph)
	GoldenDependencyGraphBasic        = "go_get_dependency_graph_basic_functionality.golden"
	GoldenDependencyGraphComplex      = "go_get_dependency_graph_complex_scenarios.golden"
	GoldenDependencyGraphDependents   = "go_get_dependency_graph_dependents.golden"
	GoldenDependencyGraphError        = "go_get_dependency_graph_error_handling.golden"
	GoldenDependencyGraphIntegration  = "go_get_dependency_graph_integration.golden"
	GoldenDependencyGraphOutputFormat = "go_get_dependency_graph_output_format.golden"
	GoldenDependencyGraphStdlib       = "go_get_dependency_graph_stdlib_packages.golden"
	GoldenDependencyGraphTransitive   = "go_get_dependency_graph_transitive_dependencies.golden"

	// Diagnostics Tool (go_build_check)
	GoldenDiagnostics               = "go_build_check_e2e.golden"
	GoldenDiagnosticsCleanProject   = "go_build_check_clean_project.golden"
	GoldenDiagnosticsSyntaxError    = "go_build_check_syntax_error.golden"
	GoldenDiagnosticsTypeError      = "go_build_check_type_error.golden"
	GoldenDiagnosticsImportError    = "go_build_check_import_error.golden"
	GoldenDiagnosticsUnusedVariable = "go_build_check_unused_variable.golden"
	GoldenDiagnosticsDeduplication  = "go_build_check_deduplication.golden"
	GoldenDiagnosticsTests          = "go_build_check_test_files_e2e.golden"

	// Read File Tool (go_read_file)
	GoldenReadFile                  = "go_read_file_e2e.golden"
	GoldenReadFileExisting          = "go_read_file_existing.golden"
	GoldenReadFileSpecialCharacters = "go_read_file_special_characters.golden"
	GoldenReadFileNonExistent       = "go_read_file_non_existent.golden"
	GoldenReadFileLarge             = "go_read_file_large.golden"
	GoldenReadFileOffset            = "go_read_file_offset.golden"
	GoldenReadFileOffsetMaxLines    = "go_read_file_offset_maxlines.golden"

	// Rename Symbol Tool (go_dryrun_rename_symbol)
	GoldenRenameSymbolStrong    = "go_dryrun_rename_symbol_strong.golden"
	GoldenRenameSymbolExact     = "go_dryrun_rename_symbol_exact_count.golden"
	GoldenRenameSymbolMultiFile = "go_dryrun_rename_symbol_multi_file.golden"
	GoldenRenameSymbolType      = "go_dryrun_rename_symbol_type.golden"

	// Search Tool (go_search)
	GoldenSearch                = "go_search_e2e.golden"
	GoldenSearchTests           = "go_search_test_files_e2e.golden"
	GoldenSearchTestFunctions   = "go_search_test_functions.golden"
	GoldenSearchTestDefinitions = "go_search_test_definitions.golden"
	GoldenSearchCrossFile       = "go_search_cross_file.golden"

	// ===== Corner Cases & Special Scenarios =====
	// These tests don't directly correspond to a single MCP tool
	// They test specific language features, workflows, or edge cases

	// Generics Support (cross-cutting tests for all tools with generics)
	GoldenGenericsSupport           = "etc_generics_support.golden"
	GoldenGenericsBasicFunctions    = "etc_generics_basic_functions.golden"
	GoldenGenericsConstraints       = "etc_generics_constraints.golden"
	GoldenGenericsGenericInterfaces = "etc_generics_generic_interfaces.golden"
	GoldenGenericsGenericTypes      = "etc_generics_generic_types.golden"
	GoldenGenericsNestedGenerics    = "etc_generics_nested_generics.golden"
	GoldenGenericsRealWorldUsage    = "etc_generics_real_world_usage.golden"
	GoldenGenericsTypeInference     = "etc_generics_type_inference.golden"
	GoldenInterfaceSatisfaction     = "etc_interface_satisfaction.golden"

	// Refactoring Scenarios (multi-tool refactoring workflows)
	GoldenRefactoringSafeRename          = "etc_refactoring_safe_rename.golden"
	GoldenRefactoringSafeDelete          = "etc_refactoring_safe_delete.golden"
	GoldenRefactoringExtractFunction     = "etc_refactoring_extract_function.golden"
	GoldenRefactoringInlineFunction      = "etc_refactoring_inline_function.golden"
	GoldenRefactoringMoveSymbol          = "etc_refactoring_move_symbol.golden"
	GoldenRefactoringChangeSignature     = "etc_refactoring_change_signature.golden"
	GoldenRefactoringInterfaceExtraction = "etc_refactoring_interface_extraction.golden"
	GoldenRefactoringMultiFileChange     = "etc_refactoring_multi_file_change.golden"
	GoldenRefactoringRealWorldScenario   = "etc_refactoring_real_world_scenario.golden"
	GoldenRenameEdgeCases                = "etc_rename_edge_cases.golden"
	GoldenComplexRenameScenarios         = "etc_complex_rename_scenarios.golden"

	// Performance Tests
	GoldenPerformanceAnalyzeWorkspace       = "etc_performance_analyze_workspace.golden"
	GoldenPerformanceBatchOperations        = "etc_performance_batch_operations.golden"
	GoldenPerformanceCallHierarchy          = "etc_performance_call_hierarchy.golden"
	GoldenPerformanceDiagnosticsIncremental = "etc_performance_diagnostics_incremental.golden"
	GoldenPerformanceLargeFiles             = "etc_performance_large_files.golden"
	GoldenPerformanceLargeTestFile          = "etc_performance_large_test_file.golden"

	// Comprehensive Workflows (All Tools)
	GoldenAllTools           = "etc_e2e_all_tools.golden"
	GoldenRealCodebase       = "etc_e2e_real_codebase.golden"
	GoldenRealCodebaseRename = "etc_e2e_refactoring.golden"

	// Real-World Workflow Tests
	GoldenWorkflowCrossModule    = "etc_real_cross_module_analysis.golden"
	GoldenWorkflowDiagnostics    = "etc_real_diagnostics_and_quality.golden"
	GoldenWorkflowErrorScenarios = "etc_real_error_scenarios.golden"
	GoldenWorkflowMultiPackage   = "etc_real_multi_package_analysis.golden"
	GoldenWorkflowPerformance    = "etc_real_performance.golden"
	GoldenWorkflowRefactoring    = "etc_real_refactoring_scenarios.golden"
	GoldenWorkflowToolChaining   = "etc_real_tool_chain_chaining_multiple_tools.golden"
	GoldenWorkflowUnderstandArch = "etc_real_understand_architecture.golden"

	// Real Test Files
	GoldenRealTestFilesWorkspaceAnalysis = "etc_real_test_files_workspace_analysis.golden"

	// Error Scenarios
	GoldenErrorScenarios = "etc_e2e_error_scenarios.golden"
	GoldenErrorHandling  = "etc_error_handling_e2e.golden"
	GoldenAddNegative    = "etc_add_negative.golden"
	GoldenSomething      = "etc_something.golden"
	GoldenMain           = "etc_main.golden"

	// File Watching
	GoldenFileWatching = "etc_file_watching_e2e.golden"

	// Empty CWD
	GoldenEmptyCWD = "etc_empty_cwd_e2e.golden"

	// Cache Behavior
	GoldenCacheWarmedOnStartup = "etc_cache_is_warmed_on_startup.golden"
	GoldenCacheWarmupRace      = "etc_cache_warmup_race_condition.golden"

	// Stdlib Deep Dive Tests
	GoldenStdlibComplexTypes = "etc_stdlib_complex_types.golden"
	GoldenStdlibNavigation   = "etc_stdlib_navigation.golden"
	GoldenStdlibReferences   = "etc_stdlib_references.golden"
	GoldenStdlibContext      = "etc_stdlib_context_deep_dive.golden"
	GoldenStdlibDatabaseSQL  = "etc_stdlib_database_sql_deep_dive.golden"
	GoldenStdlibEncodingJSON = "etc_stdlib_encoding_json_deep_dive.golden"
	GoldenStdlibInterfaces   = "etc_stdlib_interfaces.golden"
	GoldenStdlibIO           = "etc_stdlib_io_deep_dive.golden"
	GoldenStdlibNetHTTP      = "etc_stdlib_net_http_deep_dive.golden"
	GoldenStdlibSync         = "etc_stdlib_sync_deep_dive.golden"
	GoldenStdlibTime         = "etc_stdlib_time_deep_dive.golden"
)
