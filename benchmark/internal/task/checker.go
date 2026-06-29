package task

import (
	"strings"
)

// Checker holds the ground-truth facts used to automatically score an answer.
// Each item in MustContain is worth one point; Score = found / total.
type Checker struct {
	// MustContain: every string here must appear (case-insensitive) in the
	// answer. Use distinctive identifiers (file names, symbols, line numbers)
	// that a correct, complete answer would naturally include.
	MustContain []string
}

// CheckResult is the outcome of evaluating one answer.
type CheckResult struct {
	Score   float64  // 0.0–1.0
	Found   int      // items matched
	Total   int      // total items
	Missing []string // items not found in answer
}

// Total returns the number of ground-truth items.
func (c Checker) Total() int { return len(c.MustContain) }

// Check evaluates answer against the ground truth.
func (c Checker) Check(answer string) CheckResult {
	if len(c.MustContain) == 0 {
		return CheckResult{Score: 1.0}
	}
	lower := strings.ToLower(answer)
	var missing []string
	for _, item := range c.MustContain {
		if !strings.Contains(lower, strings.ToLower(item)) {
			missing = append(missing, item)
		}
	}
	found := len(c.MustContain) - len(missing)
	return CheckResult{
		Score:   float64(found) / float64(len(c.MustContain)),
		Found:   found,
		Total:   len(c.MustContain),
		Missing: missing,
	}
}
