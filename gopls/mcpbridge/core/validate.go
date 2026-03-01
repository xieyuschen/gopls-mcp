package core

import (
	"go/token"
	"strings"
)

// Validation error messages for go_search queries.
const (
	// Empty query
	errEmptyQuery = `go_search requires a non-empty symbol name. Correct: go_search query="Run"`

	// Not a valid identifier
	errNotIdentifier = `go_search requires a valid Go identifier (letters, digits, underscores only).
Use bare symbol name.
Correct: go_search query="Run" or go_search query="parseJSON".
Incorrect: go_search query="server.Run" or go_search query="find handlers".
Use Grep for text search, go_definition for specific implementations.`

	// Specific hints for common mistakes
	hintSpaces = `Query contains spaces. Use bare symbol name only. Correct: go_search query="Run"`
	hintDots   = `Query contains dots. Use bare symbol name, not qualified name. Correct: go_search query="Run" not go_search query="server.Run"`
	hintSlash  = `Query contains path separators. Use bare symbol name only. Correct: go_search query="Handler" not go_search query="api/Handler"`
	hintParens = `Query contains parentheses. Use function name only. Correct: go_search query="Run" not go_search query="Run()"`

	// No symbols found hints
	errNoSymbolsFound = `No symbols found.
Check spelling or use go_list_package_symbols to browse available symbols.
Use go_definition with context_file for specific implementations.`

	errNoSymbolsFoundTryCwd = `No symbols found.
Your workspace may have multiple modules. Try providing Cwd parameter.
Example: go_search query="Symbol" Cwd="/path/to/module".
Or use go_list_package_symbols to browse available symbols.`
)

// validateSearchQuery validates a search query and returns an error message if invalid.
// Returns empty string if the query is valid.
//
// go_search accepts valid Go identifiers only: letters, digits, underscores.
// Common mistakes: spaces (natural language), dots (qualified names), slashes (paths).
func validateSearchQuery(query string) string {
	if query == "" {
		return errEmptyQuery
	}

	// Check if it's a valid Go identifier using Go's standard library
	if token.IsIdentifier(query) {
		return "" // Valid
	}

	// Not a valid identifier - provide specific hint based on what we see
	if strings.Contains(query, " ") {
		return hintSpaces
	}
	if strings.Contains(query, ".") {
		return hintDots
	}
	if strings.Contains(query, "/") {
		return hintSlash
	}
	if strings.ContainsAny(query, "()") {
		return hintParens
	}

	// Fallback: general error for non-identifier characters
	return errNotIdentifier
}

// emptyResultsError returns an error message for empty search results.
// Provides different hints based on whether cwd was provided.
func emptyResultsError(cwd string) string {
	if cwd == "" {
		return errNoSymbolsFoundTryCwd
	}
	return errNoSymbolsFound
}
