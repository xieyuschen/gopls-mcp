package core

import (
	"strings"
	"testing"
)

func TestTruncateFileContent(t *testing.T) {
	content := strings.Repeat("line of content here\n", 100) // 100 lines

	tests := []struct {
		name         string
		maxBytes     int
		maxLines     int
		startLine    int
		wantEmpty    bool
		wantTrunc    bool
		wantErrHint  string // substring in error message
		wantLineHint string // substring in content for line truncation
	}{
		{
			name:      "no limits",
			maxBytes:  0,
			maxLines:  0,
			startLine: 1,
		},
		{
			name:         "line limit only",
			maxBytes:     0,
			maxLines:     10,
			startLine:    1,
			wantTrunc:    true,
			wantLineHint: "showing lines",
		},
		{
			name:        "byte limit only",
			maxBytes:    200,
			maxLines:    0,
			startLine:   1,
			wantTrunc:   true,
		},
		{
			name:         "both limits",
			maxBytes:     500,
			maxLines:     5,
			startLine:    1,
			wantTrunc:    true,
			wantLineHint: "showing lines",
		},
		{
			name:      "start line offset",
			maxBytes:  0,
			maxLines:  0,
			startLine: 50,
		},
		{
			name:        "start line too large",
			maxBytes:    0,
			maxLines:    0,
			startLine:   200,
			wantEmpty:   true,
			wantErrHint: "exceeds file length",
		},
		{
			name:         "start line + line limit",
			maxBytes:     0,
			maxLines:     5,
			startLine:    10,
			wantTrunc:    true,
			wantLineHint: "showing lines 10-14",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, linesRead, errMsg := TruncateFileContent(content, tt.maxBytes, tt.maxLines, tt.startLine)

			if tt.wantEmpty {
				if got != "" {
					t.Errorf("expected empty content, got %d bytes", len(got))
				}
				if errMsg == "" {
					t.Error("expected error message, got empty")
				}
				if tt.wantErrHint != "" && !strings.Contains(errMsg, tt.wantErrHint) {
					t.Errorf("error message %q should contain %q", errMsg, tt.wantErrHint)
				}
				return
			}

			if tt.wantErrHint != "" {
				t.Errorf("unexpected error: %s", errMsg)
			}

			if got == "" {
				t.Fatal("unexpected empty content")
			}

			// Check line count
			if tt.maxLines > 0 {
				if linesRead > tt.maxLines {
					t.Errorf("expected at most %d lines read, got %d", tt.maxLines, linesRead)
				}
			}

			// Check truncation indicator
			if tt.wantTrunc && !strings.Contains(got, "[TRUNCATED") {
				t.Errorf("expected truncation indicator in content, got:\n%s", got)
			}
			if !tt.wantTrunc && strings.Contains(got, "[TRUNCATED") {
				t.Errorf("unexpected truncation indicator in content")
			}

			// Check line hint
			if tt.wantLineHint != "" && !strings.Contains(got, tt.wantLineHint) {
				t.Errorf("expected line hint %q in content, got:\n%s", tt.wantLineHint, got)
			}
		})
	}
}

func TestTruncateFileContent_EmptyInput(t *testing.T) {
	got, linesRead, errMsg := TruncateFileContent("", 100, 10, 1)
	if got != "" {
		t.Errorf("expected empty content for empty input, got %q", got)
	}
	// Empty string split by "\n" yields [""] — 1 element, so linesRead = 1
	if linesRead != 1 {
		t.Errorf("expected 1 line for empty input (split produces ['']), got %d", linesRead)
	}
	if errMsg != "" {
		t.Errorf("expected no error for empty input, got %q", errMsg)
	}
}

func TestTruncateByBytes(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		maxBytes     int
		wantTrunc    bool
		wantLenLess  int // expected len < this value
	}{
		{
			name:      "content within limit",
			content:   "short content",
			maxBytes:  1000,
			wantTrunc: false,
		},
		{
			name:        "content exceeds limit",
			content:     strings.Repeat("abcdefghij", 100), // 1000 bytes
			maxBytes:    200,
			wantTrunc:   true,
			wantLenLess: 300, // content + indicator
		},
		{
			name:      "exact limit",
			content:   "exactly 20 byt", // 16 bytes
			maxBytes:  16,
			wantTrunc: false,
		},
		{
			name:      "zero max bytes",
			content:   "some content",
			maxBytes:  0,
			wantTrunc: false,
		},
		{
			name:      "negative max bytes",
			content:   "some content",
			maxBytes:  -1,
			wantTrunc: false,
		},
		{
			name:        "multi-byte utf8 content",
			content:     strings.Repeat("你好世界", 100), // 1200 bytes (3 bytes per char)
			maxBytes:    100,
			wantTrunc:   true,
			wantLenLess: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, wasTruncated := truncateByBytes(tt.content, tt.maxBytes)
			if wasTruncated != tt.wantTrunc {
				t.Errorf("truncateByBytes() truncated = %v, want %v", wasTruncated, tt.wantTrunc)
			}
			if !tt.wantTrunc && got != tt.content {
				t.Errorf("truncateByBytes() got = %q, want %q", got, tt.content)
			}
			if tt.wantTrunc && !strings.Contains(got, TruncationIndicator) {
				t.Errorf("truncated content missing TruncationIndicator, got:\n%s", got)
			}
			if tt.wantLenLess > 0 && len(got) >= tt.wantLenLess {
				t.Errorf("truncated content len = %d, want < %d", len(got), tt.wantLenLess)
			}
		})
	}
}
