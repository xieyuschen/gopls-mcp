package core

import (
	"testing"
)

func TestValidateSearchQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		// Valid identifiers
		{"empty string", "", true},
		{"simple identifier", "Run", false},
		{"camelCase identifier", "parseJSON", false},
		{"identifier with digits", "http2Server", false},
		{"underscore identifier", "my_func", false},
		{"single letter", "x", false},
		{"underscore only", "_", false},

		// Invalid: spaces
		{"natural language", "find handlers", true},
		{"leading space", " Run", true},

		// Invalid: dots
		{"qualified name", "server.Run", true},
		{"package qualified", "pkg.Func", true},

		// Invalid: slashes
		{"path separator", "api/Handler", true},

		// Invalid: parentheses
		{"function call", "Run()", true},
		{"method call", "obj.Method()", true},
		{"open paren only", "Run(", true},
		{"close paren only", "Run)", true},

		// Invalid: special characters
		{"hyphen", "my-func", true},
		{"at sign", "func@v2", true},
		{"hash", "Main#1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateSearchQuery(tt.query)
			if (got != "") != tt.wantErr {
				t.Errorf("validateSearchQuery(%q) = %q, wantErr %v", tt.query, got, tt.wantErr)
			}
		})
	}
}

func TestValidateSearchQuery_ErrorHints(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantHint  string // substring expected in the error message
	}{
		{"empty query", "", "non-empty symbol name"},
		{"spaces hint", "find handlers", "spaces"},
		{"dots hint", "server.Run", "dots"},
		{"slash hint", "api/Handler", "path separators"},
		{"parens hint", "Run()", "parentheses"},
		{"general invalid", "$invalid", "valid Go identifier"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateSearchQuery(tt.query)
			if got == "" {
				t.Fatalf("validateSearchQuery(%q) returned empty, expected error", tt.query)
			}
			// Check that the error contains the expected hint substring
			// We use the constant values directly for verification
			if tt.wantHint == "non-empty symbol name" && got != errEmptyQuery {
				t.Errorf("validateSearchQuery(%q) = %q, want %q", tt.query, got, errEmptyQuery)
			}
			if tt.wantHint == "spaces" && got != hintSpaces {
				t.Errorf("validateSearchQuery(%q) = %q, want %q", tt.query, got, hintSpaces)
			}
			if tt.wantHint == "dots" && got != hintDots {
				t.Errorf("validateSearchQuery(%q) = %q, want %q", tt.query, got, hintDots)
			}
			if tt.wantHint == "path separators" && got != hintSlash {
				t.Errorf("validateSearchQuery(%q) = %q, want %q", tt.query, got, hintSlash)
			}
			if tt.wantHint == "parentheses" && got != hintParens {
				t.Errorf("validateSearchQuery(%q) = %q, want %q", tt.query, got, hintParens)
			}
			if tt.wantHint == "valid Go identifier" && got != errNotIdentifier {
				t.Errorf("validateSearchQuery(%q) = %q, want %q", tt.query, got, errNotIdentifier)
			}
		})
	}
}

func TestEmptyResultsError(t *testing.T) {
	tests := []struct {
		name string
		cwd  string
		want string
	}{
		{"no cwd", "", errNoSymbolsFoundTryCwd},
		{"with cwd", "/some/path", errNoSymbolsFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := emptyResultsError(tt.cwd)
			if got != tt.want {
				t.Errorf("emptyResultsError(%q) = %q, want %q", tt.cwd, got, tt.want)
			}
		})
	}
}
