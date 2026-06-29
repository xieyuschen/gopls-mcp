// Package task defines the benchmark task model and built-in suite.
package task

// Tag categorizes a task by the primary semantic capability it exercises.
type Tag string

const (
	TagDefinition     Tag = "definition"
	TagReferences     Tag = "references"
	TagImplementation Tag = "implementation"
	TagCallHierarchy  Tag = "call-hierarchy"
	TagDependency     Tag = "dependency"
	TagSamePackage    Tag = "same-package"
	TagCrossPackage   Tag = "cross-package"
	TagCrossProject   Tag = "cross-project"
)

// Task is a single benchmark scenario: the same natural-language Prompt is
// sent to both the plain (no-MCP) and gopls-mcp runs. The runner measures
// which tools each agent reaches for and how many tokens it spends.
type Task struct {
	Name        string
	Description string
	// Prompt is the natural-language question sent to Claude verbatim.
	// It should not mention specific tool names so both runs get a fair shot.
	Prompt  string
	Tags    []Tag
	Checker Checker // ground-truth facts for automated answer scoring
}
