package testutil

// Golden file names used across tests.
// Grouped by functionality for better organization.
//
// Naming convention:
// - MCP tool tests: <mcp_tool>_<case>.golden (e.g., go_definition_e2e_strong.golden)
// - Corner cases (non-MCP-tool tests): workflow_<description>.golden
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
	GoldenListTools                       = "go_list_tools_e2e.golden"
	GoldenAnalyzeWorkspace                = "go_analyze_workspace_e2e.golden"
	GoldenGetPackageSymbolDetail          = "go_get_package_symbol_detail_e2e.golden"
	GoldenGetStarted                      = "go_get_started_e2e.golden"

	// Navigation Tools (go_definition, go_implementation, go_symbol_references)
	GoldenDefinition          = "go_definition_e2e_strong.golden"
	GoldenDefinitionCrossFile = "go_definition_cross_file_strong.golden"
	GoldenImplementation      = "go_implementation_e2e.golden"
	GoldenSymbolReferences    = "go_symbol_references_strong.golden"

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
	GoldenDiagnostics = "go_build_check_e2e.golden"

	// Read File Tool (go_read_file)
	GoldenReadFile = "go_read_file_e2e.golden"

	// Rename Symbol Tool (go_dryrun_rename_symbol)
	GoldenRenameSymbolStrong = "go_dryrun_rename_symbol_strong.golden"

	// ===== Corner Cases & Special Scenarios =====
	// These tests don't directly correspond to a single MCP tool
	// They test specific language features, workflows, or edge cases

	// Generics Support
	GoldenGenericsSupport           = "workflow_generics_support.golden"
	GoldenGenericsBasicFunctions    = "workflow_generics_basic_functions.golden"
	GoldenGenericsConstraints       = "workflow_generics_constraints.golden"
	GoldenGenericsGenericInterfaces = "workflow_generics_generic_interfaces.golden"
	GoldenGenericsGenericTypes      = "workflow_generics_generic_types.golden"
	GoldenGenericsNestedGenerics    = "workflow_generics_nested_generics.golden"
	GoldenGenericsRealWorldUsage    = "workflow_generics_real_world_usage.golden"
	GoldenGenericsTypeInference     = "workflow_generics_type_inference.golden"
	GoldenInterfaceSatisfaction     = "workflow_interface_satisfaction.golden"

	// Refactoring Scenarios
	GoldenRefactoringSafeRename          = "workflow_refactoring_safe_rename.golden"
	GoldenRefactoringSafeDelete          = "workflow_refactoring_safe_delete.golden"
	GoldenRefactoringExtractFunction     = "workflow_refactoring_extract_function.golden"
	GoldenRefactoringInlineFunction      = "workflow_refactoring_inline_function.golden"
	GoldenRefactoringMoveSymbol          = "workflow_refactoring_move_symbol.golden"
	GoldenRefactoringChangeSignature     = "workflow_refactoring_change_signature.golden"
	GoldenRefactoringInterfaceExtraction = "workflow_refactoring_interface_extraction.golden"
	GoldenRefactoringMultiFileChange     = "workflow_refactoring_multi_file_change.golden"
	GoldenRefactoringRealWorldScenario   = "workflow_refactoring_real_world_scenario.golden"
	GoldenRenameEdgeCases                = "workflow_rename_edge_cases.golden"
	GoldenComplexRenameScenarios         = "workflow_complex_rename_scenarios.golden"

	// Performance Tests
	GoldenPerformanceAnalyzeWorkspace       = "workflow_performance_analyze_workspace.golden"
	GoldenPerformanceBatchOperations        = "workflow_performance_batch_operations.golden"
	GoldenPerformanceCallHierarchy          = "workflow_performance_call_hierarchy.golden"
	GoldenPerformanceDiagnosticsIncremental = "workflow_performance_diagnostics_incremental.golden"
	GoldenPerformanceLargeFiles             = "workflow_performance_large_files.golden"
	GoldenPerformanceLargeTestFile          = "workflow_performance_large_test_file.golden"

	// Comprehensive Workflows (All Tools)
	GoldenAllTools           = "workflow_e2e_all_tools.golden"
	GoldenRealCodebase       = "workflow_e2e_real_codebase.golden"
	GoldenRealCodebaseRename = "workflow_e2e_refactoring.golden"

	// Real-World Workflow Tests
	GoldenWorkflowCrossModule    = "workflow_real_workflow_cross_module_analysis.golden"
	GoldenWorkflowDiagnostics    = "workflow_real_workflow_diagnostics_and_quality.golden"
	GoldenWorkflowErrorScenarios = "workflow_real_workflow_error_scenarios.golden"
	GoldenWorkflowMultiPackage   = "workflow_real_workflow_multi_package_analysis.golden"
	GoldenWorkflowPerformance    = "workflow_real_workflow_performance.golden"
	GoldenWorkflowRefactoring    = "workflow_real_workflow_refactoring_scenarios.golden"
	GoldenWorkflowToolChaining   = "workflow_real_workflow_tool_chain_chaining_multiple_tools.golden"
	GoldenWorkflowUnderstandArch = "workflow_real_workflow_understand_architecture.golden"

	// Real Test Files
	GoldenRealTestFilesDiagnostics       = "workflow_real_test_files_diagnostics_on_tests.golden"
	GoldenRealTestFilesFindTestUsages    = "workflow_real_test_files_find_test_usages.golden"
	GoldenRealTestFilesNavigate          = "workflow_real_test_files_navigate_test_code.golden"
	GoldenRealTestFilesPackageSymbols    = "workflow_real_test_files_test_package_symbols.golden"
	GoldenRealTestFilesWorkspaceAnalysis = "workflow_real_test_files_workspace_analysis.golden"

	// Error Scenarios
	GoldenErrorScenarios = "workflow_e2e_error_scenarios.golden"
	GoldenErrorHandling  = "workflow_error_handling_e2e.golden"
	GoldenAddNegative    = "workflow_add_negative.golden"
	GoldenSomething      = "workflow_something.golden"
	GoldenMain           = "workflow_main.golden"

	// File Watching
	GoldenFileWatching = "workflow_file_watching_e2e.golden"

	// Empty CWD
	GoldenEmptyCWD = "workflow_empty_cwd_e2e.golden"

	// Cache Behavior
	GoldenCacheWarmedOnStartup = "workflow_cache_is_warmed_on_startup.golden"
	GoldenCacheWarmupRace      = "workflow_cache_warmup_race_condition.golden"

	// Stdlib Deep Dive Tests
	GoldenStdlibComplexTypes = "workflow_stdlib_complex_types.golden"
	GoldenStdlibNavigation   = "workflow_stdlib_navigation.golden"
	GoldenStdlibReferences   = "workflow_stdlib_references.golden"
	GoldenStdlibContext      = "workflow_stdlib_context_deep_dive.golden"
	GoldenStdlibDatabaseSQL  = "workflow_stdlib_database_sql_deep_dive.golden"
	GoldenStdlibEncodingJSON = "workflow_stdlib_encoding_json_deep_dive.golden"
	GoldenStdlibInterfaces   = "workflow_stdlib_interfaces.golden"
	GoldenStdlibIO           = "workflow_stdlib_io_deep_dive.golden"
	GoldenStdlibNetHTTP      = "workflow_stdlib_net_http_deep_dive.golden"
	GoldenStdlibSync         = "workflow_stdlib_sync_deep_dive.golden"
	GoldenStdlibTime         = "workflow_stdlib_time_deep_dive.golden"
)
