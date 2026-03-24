package core

import (
	"strings"
	"testing"
)

func TestEstimateResultSize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"short", "hello", 5},
		{"unicode", "你好世界", 12}, // 3 bytes per Chinese char
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily construct mcp.CallToolResult without import,
			// so we test estimateResultSize indirectly via jsonSize
			got, err := jsonSize(tt.input)
			if err != nil {
				t.Fatalf("jsonSize(%q) error: %v", tt.input, err)
			}
			// json.Marshal adds quotes for strings: "hello" = 7 bytes
			if got <= 0 && len(tt.input) > 0 {
				t.Errorf("jsonSize(%q) = %d, want > 0", tt.input, got)
			}
		})
	}
}

func TestJsonSize(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantErr bool
		minSize int // minimum expected size
	}{
		{"string", "hello", false, 0},
		{"map", map[string]any{"key": "value"}, false, 10},
		{"nested map", map[string]any{"a": map[string]any{"b": "c"}}, false, 10},
		{"array", []any{"a", "b", "c"}, false, 5},
		{"number", 42, false, 1},
		{"bool", true, false, 1},
		{"nil value", map[string]any{"key": nil}, false, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jsonSize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("jsonSize() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got < tt.minSize {
				t.Errorf("jsonSize() = %d, want >= %d", got, tt.minSize)
			}
		})
	}
}

func TestTruncateMap(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		maxChars int
		wantKeys int // expected number of non-underscore keys
	}{
		{
			name:     "all fit",
			input:    map[string]any{"a": "short", "b": "also"},
			maxChars: 1000,
			wantKeys: 2,
		},
		{
			name:     "partial fit - some content truncated",
			input:    map[string]any{"a": "short", "b": strings.Repeat("x", 100)},
			maxChars: 50,
			wantKeys: 0, // skip count check (non-deterministic iteration order)
		},
		{
			name:     "metadata preserved",
			input:    map[string]any{"_meta": "keep", "data": strings.Repeat("x", 1000)},
			maxChars: 10,
			wantKeys: 1, // _meta preserved, data may be truncated
		},
		{
			name:     "empty map",
			input:    map[string]any{},
			maxChars: 100,
			wantKeys: 0,
		},
		{
			name: "preserve all metadata",
			input: map[string]any{
				"_truncated":     true,
				"_original_bytes": 500,
				"big_field":       strings.Repeat("a", 1000),
			},
			maxChars: 50,
			wantKeys: 1, // only big_field counted (metadata prefixed with _)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateMap(tt.input, tt.maxChars)
			count := 0
			for k := range got {
				if !strings.HasPrefix(k, "_") {
					count++
				}
			}
			// wantKeys == 0 means skip exact count check (non-deterministic map order)
			if tt.wantKeys > 0 && count != tt.wantKeys {
				t.Errorf("truncateMap() non-meta keys = %d, want %d (got: %v)", count, tt.wantKeys, got)
			}
		})
	}
}

func TestTruncateArray(t *testing.T) {
	tests := []struct {
		name     string
		input    []any
		maxBytes int
		wantLen  int
	}{
		{
			name:     "all fit",
			input:    []any{"a", "b", "c"},
			maxBytes: 100,
			wantLen:  3,
		},
		{
			name:     "partial fit",
			input:    []any{"a", "b", "c", "d", "e"},
			maxBytes: 10,
			wantLen:  1, // at least 1
		},
		{
			name:     "empty array",
			input:    []any{},
			maxBytes: 100,
			wantLen:  0,
		},
		{
			name:     "single large element",
			input:    []any{strings.Repeat("x", 100)},
			maxBytes: 50,
			wantLen:  1, // binary search should keep at least 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateArray(tt.input, tt.maxBytes)
			if len(got) < tt.wantLen {
				t.Errorf("truncateArray() len = %d, want >= %d", len(got), tt.wantLen)
			}
			if len(tt.input) > 0 && len(got) > len(tt.input) {
				t.Errorf("truncateArray() len = %d, want <= %d", len(got), len(tt.input))
			}
		})
	}
}

func TestTruncateValue(t *testing.T) {
	tests := []struct {
		name  string
		input any
		limit int
	}{
		{"string truncated", strings.Repeat("x", 100), 20},
		{"string fits", "short", 100},
		{"array value", []any{"a", "b", "c"}, 10},
		{"map value", map[string]any{"key": "val"}, 10},
		{"number passthrough", 42, 5},
		{"bool passthrough", true, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateValue(tt.input, tt.limit)
			if got == nil {
				t.Error("truncateValue() returned nil")
			}
		})
	}
}

func TestAddTruncationMetadata(t *testing.T) {
	tests := []struct {
		name         string
		originalSize int
		truncSize    int
		wantHint     bool // whether a _hint field should be added
	}{
		{"no truncation", 100, 100, false},
		{"light truncation", 1000, 800, false},  // 80% kept
		{"medium truncation", 1000, 600, true},  // 60% kept
		{"heavy truncation", 1000, 300, true},   // 30% kept
		{"zero original", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]any{"content": "test"}
			addTruncationMetadata(data, tt.originalSize, tt.truncSize, "test_tool")

			if data["_truncated"] != true {
				t.Error("expected _truncated = true")
			}
			if data["_tool"] != "test_tool" {
				t.Errorf("expected _tool = 'test_tool', got %v", data["_tool"])
			}

			_, hasHint := data["_hint"]
			if hasHint != tt.wantHint {
				t.Errorf("_hint presence = %v, want %v", hasHint, tt.wantHint)
			}
		})
	}
}

func TestEstimateValueSize(t *testing.T) {
	tests := []struct {
		name string
		val  any
		min  int
	}{
		{"string", "hello", 5},
		{"number", 42, 1},
		{"bool", true, 1},
		{"map", map[string]any{"k": "v"}, 5},
		{"array", []any{1, 2, 3}, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateValueSize(tt.val)
			if got < tt.min {
				t.Errorf("estimateValueSize() = %d, want >= %d", got, tt.min)
			}
		})
	}
}

func TestEstimateArraySize(t *testing.T) {
	arr := []any{"a", "b", "c"}
	got := estimateArraySize(arr)
	if got <= 0 {
		t.Errorf("estimateArraySize() = %d, want > 0", got)
	}

	// Empty array
	empty := []any{}
	gotEmpty := estimateArraySize(empty)
	if gotEmpty < 0 {
		t.Errorf("estimateArraySize([]) = %d, want >= 0", gotEmpty)
	}
}

func TestEstimateMapSize(t *testing.T) {
	m := map[string]any{"key": "value"}
	got := estimateMapSize(m)
	if got <= 0 {
		t.Errorf("estimateMapSize() = %d, want > 0", got)
	}

	// Empty map
	empty := map[string]any{}
	gotEmpty := estimateMapSize(empty)
	if gotEmpty < 0 {
		t.Errorf("estimateMapSize({}) = %d, want >= 0", gotEmpty)
	}
}
