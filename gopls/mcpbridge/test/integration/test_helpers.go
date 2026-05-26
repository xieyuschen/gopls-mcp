package integration

// Helper utilities for table-driven integration tests.
// Reduces boilerplate and makes tests more maintainable.

import (
	"fmt"
	"strings"
	"testing"
)

// testCase defines a single tool test case
type testCase struct {
	// Name of the test case (will be used as subtest name)
	name string

	// Tool name to call
	tool string

	// Arguments to pass to the tool. Mutually exclusive with setup.
	args map[string]any

	// Optional: setup creates temp files and returns args. Takes priority over args.
	setup func(t *testing.T) map[string]any

	// Optional: project to copy from testdata (e.g., "simple", "generics")
	// If empty, uses the default shared workdir
	project string

	// Assertions to run on the tool output
	assertions []assertion

	// skip marks this test case for skipping
	skip       bool
	skipReason string
}

// assertion defines a check to run on tool output
type assertion struct {
	// Description of what's being checked
	description string

	// Check function returns true if the check passes
	check func(content string) bool

	// Error message to print if check fails
	errorMsg string
}

// Common assertion builders for convenience

// assertContains checks that content contains a substring
func assertContains(substring string) assertion {
	return assertion{
		description: fmt.Sprintf("contains %q", substring),
		check:       func(content string) bool { return strings.Contains(content, substring) },
		errorMsg:    fmt.Sprintf("expected content to contain %q", substring),
	}
}

// assertContainsAny checks that content contains at least one of the substrings
func assertContainsAny(substrings ...string) assertion {
	return assertion{
		description: fmt.Sprintf("contains any of %v", substrings),
		check: func(content string) bool {
			for _, s := range substrings {
				if strings.Contains(content, s) {
					return true
				}
			}
			return false
		},
		errorMsg: fmt.Sprintf("expected content to contain at least one of %v", substrings),
	}
}

// assertContainsAll checks that content contains all substrings
func assertContainsAll(substrings ...string) assertion {
	return assertion{
		description: fmt.Sprintf("contains all of %v", substrings),
		check: func(content string) bool {
			for _, s := range substrings {
				if !strings.Contains(content, s) {
					return false
				}
			}
			return true
		},
		errorMsg: fmt.Sprintf("expected content to contain all of %v", substrings),
	}
}

// assertCustom allows custom assertion logic
func assertCustom(desc string, check func(content string) bool, errorMsg string) assertion {
	return assertion{
		description: desc,
		check:       check,
		errorMsg:    errorMsg,
	}
}

// truncateString limits string length for logging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}
