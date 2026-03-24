package core

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestExtractPath(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLen   int
		wantPath  []string
		wantErr   bool
	}{
		{"simple identifier", "Foo", 1, []string{"Foo"}, false},
		{"qualified two-part", "pkg.Func", 2, []string{"pkg", "Func"}, false},
		{"qualified three-part", "pkg.Type.Method", 3, []string{"pkg", "Type", "Method"}, false},
		{"invalid expression", "", 0, nil, true},
		{"call expression", "Foo()", 0, nil, true},
		{"select too deep", "a.b.c.d", 0, nil, true},
		{"literal", "42", 0, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := parser.ParseExpr(tt.input)
			if err != nil && !tt.wantErr {
				t.Fatalf("parser.ParseExpr(%q) unexpected error: %v", tt.input, err)
			}
			if err != nil {
				return // expected parse error, input is not a valid expression
			}

			got, err := extractPath(expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("extractPath() len = %d, want %d (got %v)", len(got), tt.wantLen, got)
				return
			}
			if tt.wantPath != nil {
				for i, w := range tt.wantPath {
					if got[i] != w {
						t.Errorf("extractPath()[%d] = %q, want %q", i, got[i], w)
					}
				}
			}
		})
	}
}

func TestTokenIsIdentifier(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"Foo", true},
		{"foo_bar", true},
		{"x1", true},
		{"_", true},
		{"main", true},
		{"", false},
		{"123", false},
		{"foo-bar", false},
		{"foo.bar", false},
		{"foo bar", false},
		{"pkg.Func", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := token.IsIdentifier(tt.input)
			if got != tt.want {
				t.Errorf("token.IsIdentifier(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
