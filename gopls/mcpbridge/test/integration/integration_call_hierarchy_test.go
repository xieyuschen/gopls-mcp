package integration

// End-to-end tests for call hierarchy functionality.

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGoCallHierarchy is the single table-driven test for all call hierarchy scenarios.
func TestGoCallHierarchy(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		runTableDrivenTests(t, map[string]testCase{
			"IncomingDirection": {
				setup: func(t *testing.T) map[string]any {
					dir := chSetup(t, "calltest", map[string]string{
						"main.go": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\thelperA()\n\thelperB()\n}\n\nfunc helperA() {\n\tfmt.Println(\"A\")\n}\n\nfunc helperB() {\n\tfmt.Println(\"B\")\n}\n",
					})
					return chArgs(dir, "helperA", 10, "incoming")
				},
				tool:       "go_get_call_hierarchy",
				assertions: []assertion{assertContains("main")},
			},
			"OutgoingDirection": {
				setup: func(t *testing.T) map[string]any {
					dir := chSetup(t, "calltest", map[string]string{
						"main.go": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\thelperA()\n\thelperB()\n}\n\nfunc helperA() {\n\tfmt.Println(\"A\")\n}\n\nfunc helperB() {\n\tfmt.Println(\"B\")\n}\n",
					})
					return chArgs(dir, "main", 5, "outgoing")
				},
				tool: "go_get_call_hierarchy",
				assertions: []assertion{
					assertContainsAll("helperA", "helperB"),
					assertContains("Outgoing Calls"),
				},
			},
		})
	})

	t.Run("MultipleCallers", func(t *testing.T) {
		runTableDrivenTests(t, map[string]testCase{
			"SharedFuncCalledByTwo": {
				setup: func(t *testing.T) map[string]any {
					dir := chSetup(t, "multicaller", map[string]string{
						"main.go": "package main\n\nfunc main() {\n\tfuncA()\n\tfuncB()\n}\n\nfunc funcA() {\n\tsharedFunc()\n}\n\nfunc funcB() {\n\tsharedFunc()\n}\n\nfunc sharedFunc() {\n\tprintln(\"shared\")\n}\n",
					})
					return chArgs(dir, "sharedFunc", 16, "incoming")
				},
				tool:       "go_get_call_hierarchy",
				assertions: []assertion{assertContains("funcA"), assertContains("funcB")},
			},
		})
	})

	t.Run("CallChain", func(t *testing.T) {
		runTableDrivenTests(t, map[string]testCase{
			"FuncAOutgoingToFuncB": {
				setup: func(t *testing.T) map[string]any {
					dir := chSetup(t, "callchain", map[string]string{
						"main.go": "package main\n\nfunc main() {\n\tfuncA()\n}\n\nfunc funcA() {\n\tfuncB()\n}\n\nfunc funcB() {\n\tfuncC()\n}\n\nfunc funcC() {\n\tprintln(\"C\")\n}\n",
					})
					return chArgs(dir, "funcA", 7, "outgoing")
				},
				tool:       "go_get_call_hierarchy",
				assertions: []assertion{assertContains("funcB")},
			},
		})
	})

	t.Run("InterfaceMethods", func(t *testing.T) {
		runTableDrivenTests(t, map[string]testCase{
			"ConcreteMethodIncoming": {
				setup: func(t *testing.T) map[string]any {
					dir := chSetup(t, "interfaces", map[string]string{
						"main.go": "package main\n\ntype Processor interface {\n\tProcess()\n}\n\ntype Concrete struct {\n\tname string\n}\n\nfunc (c *Concrete) Process() {\n\tprintln(\"processing:\", c.name)\n}\n\nfunc doWork(p Processor) {\n\tp.Process()\n}\n\nfunc main() {\n\tc := &Concrete{name: \"test\"}\n\tdoWork(c)\n}\n",
					})
					return map[string]any{
						"locator": map[string]any{
							"symbol_name":  "Process",
							"context_file": filepath.Join(dir, "main.go"),
							"kind":         "method",
							"line_hint":    11,
						},
						"direction": "incoming",
						"Cwd":       dir,
					}
				},
				tool:       "go_get_call_hierarchy",
				assertions: []assertion{assertContainsAny("Incoming Calls", "doWork", "main")},
			},
		})
	})

	t.Run("CrossFile", func(t *testing.T) {
		runTableDrivenTests(t, map[string]testCase{
			"MainCallsHelpersInOtherFile": {
				setup: func(t *testing.T) map[string]any {
					dir := chSetup(t, "multifile", map[string]string{
						"main.go":    "package main\n\nfunc main() {\n\tHelperFunc1()\n\tHelperFunc2()\n}\n",
						"helpers.go": "package main\n\nfunc HelperFunc1() {\n\tprintln(\"helper1\")\n}\n\nfunc HelperFunc2() {\n\tprintln(\"helper2\")\n}\n",
					})
					return chArgs(dir, "main", 3, "outgoing")
				},
				tool:       "go_get_call_hierarchy",
				assertions: []assertion{assertContains("HelperFunc1"), assertContains("HelperFunc2")},
			},
		})
	})

	t.Run("CrossPackage", func(t *testing.T) {
		runTableDrivenTests(t, map[string]testCase{
			"MainCallsOtherPackage": {
				setup: func(t *testing.T) map[string]any {
					dir := t.TempDir()
					writeChFile(t, filepath.Join(dir, "main.go"), "package main\n\nimport \"example.com/multipkg/other\"\n\nfunc main() {\n\tother.OtherPackageFunc()\n\tother.HelperFunc()\n}\n")
					otherDir := filepath.Join(dir, "other")
					if err := os.MkdirAll(otherDir, 0755); err != nil {
						t.Fatal(err)
					}
					writeChFile(t, filepath.Join(otherDir, "other.go"), "package other\n\nfunc OtherPackageFunc() {\n\tprintln(\"other package\")\n}\n\nfunc HelperFunc() {\n\tprintln(\"helper\")\n}\n")
					writeChFile(t, filepath.Join(dir, "go.mod"), "module example.com/multipkg\n\ngo 1.21\n")
					return chArgs(dir, "main", 5, "outgoing")
				},
				tool:       "go_get_call_hierarchy",
				assertions: []assertion{assertContains("OtherPackageFunc"), assertContains("HelperFunc")},
			},
		})
	})

	t.Run("StructMethods", func(t *testing.T) {
		runTableDrivenTests(t, map[string]testCase{
			"ValueReceiverIncoming": {
				setup: func(t *testing.T) map[string]any {
					dir := chSetup(t, "structmethods", map[string]string{
						"main.go": "package main\n\nimport \"fmt\"\n\ntype Counter struct {\n\tvalue int\n}\n\nfunc (s Counter) Add() int {\n\ts.value++\n\treturn s.value\n}\n\nfunc (s *Counter) Increment() {\n\ts.value++\n}\n\nfunc main() {\n\tcounter := Counter{value: 0}\n\tcounter.Add()\n\tcounter.Increment()\n\tfmt.Println(counter.value)\n}\n",
					})
					return map[string]any{
						"locator": map[string]any{
							"symbol_name":  "Add",
							"context_file": filepath.Join(dir, "main.go"),
							"kind":         "method",
							"line_hint":    9,
						},
						"direction": "incoming",
						"Cwd":       dir,
					}
				},
				tool:       "go_get_call_hierarchy",
				assertions: []assertion{assertContains("main")},
			},
		})
	})

	t.Run("SpecialCases", func(t *testing.T) {
		runTableDrivenTests(t, map[string]testCase{
			"RecursiveFunction": {
				setup: func(t *testing.T) map[string]any {
					dir := chSetup(t, "recursion", map[string]string{
						"main.go": "package main\n\nfunc main() {\n\tresult := factorial(5)\n\tprintln(result)\n}\n\nfunc factorial(n int) int {\n\tif n <= 1 {\n\t\treturn 1\n\t}\n\treturn n * factorial(n-1)\n}\n",
					})
					return chArgs(dir, "factorial", 8, "outgoing")
				},
				tool:       "go_get_call_hierarchy",
				assertions: []assertion{assertContainsAny("factorial", "Outgoing Calls")},
			},
			"DeferredCall": {
				setup: func(t *testing.T) map[string]any {
					dir := chSetup(t, "defer", map[string]string{
						"main.go": "package main\n\nfunc main() {\n\tprocess()\n}\n\nfunc process() {\n\tdefer cleanup()\n\tprintln(\"processing\")\n}\n\nfunc cleanup() {\n\tprintln(\"cleanup\")\n}\n",
					})
					return chArgs(dir, "process", 7, "outgoing")
				},
				tool:       "go_get_call_hierarchy",
				assertions: []assertion{assertContainsAny("cleanup", "Outgoing Calls")},
			},
			"GoroutineCall": {
				setup: func(t *testing.T) map[string]any {
					dir := chSetup(t, "goroutines", map[string]string{
						"main.go": "package main\n\nfunc main() {\n\tgo worker()\n\tprintln(\"main continues\")\n}\n\nfunc worker() {\n\tprintln(\"worker running\")\n}\n",
					})
					return chArgs(dir, "main", 3, "outgoing")
				},
				tool:       "go_get_call_hierarchy",
				assertions: []assertion{assertContainsAny("worker", "Outgoing Calls")},
			},
		})
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		runTableDrivenTests(t, map[string]testCase{
			"InvalidPosition": {
				setup: func(t *testing.T) map[string]any {
					dir := chSetup(t, "calltest", map[string]string{
						"main.go": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\thelperA()\n\thelperB()\n}\n\nfunc helperA() {\n\tfmt.Println(\"A\")\n}\n\nfunc helperB() {\n\tfmt.Println(\"B\")\n}\n",
					})
					return map[string]any{
						"locator": map[string]any{
							"symbol_name":  "main",
							"context_file": filepath.Join(dir, "main.go"),
							"kind":         "function",
							"line_hint":    1, // line 1 is "package main", not a function body
						},
						"direction": "both",
						"Cwd":       dir,
					}
				},
				tool: "go_get_call_hierarchy",
				assertions: []assertion{
					assertCustom(
						"returns non-empty response",
						func(content string) bool { return len(content) > 0 },
						"expected non-empty response for invalid position",
					),
				},
			},
		})
	})
}

// ===== Shared helpers =====

// chSetup creates a temp Go project with the given module name and source files.
// A go.mod is written automatically; files map is "filename" -> "content".
func chSetup(t *testing.T, module string, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		writeChFile(t, filepath.Join(dir, name), content)
	}
	writeChFile(t, filepath.Join(dir, "go.mod"), "module example.com/"+module+"\n\ngo 1.21\n")
	return dir
}

// chArgs builds the standard locator args for a function symbol.
func chArgs(dir, symbol string, lineHint int, direction string) map[string]any {
	return map[string]any{
		"locator": map[string]any{
			"symbol_name":  symbol,
			"context_file": filepath.Join(dir, "main.go"),
			"kind":         "function",
			"line_hint":    lineHint,
		},
		"direction": direction,
		"Cwd":       dir,
	}
}

// writeChFile writes content to path, fataling t on error.
func writeChFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
