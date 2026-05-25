package golang

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/printer"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/gopls/internal/cache"
	"golang.org/x/tools/gopls/internal/cache/metadata"
	"golang.org/x/tools/gopls/internal/cache/methodsets"
	"golang.org/x/tools/gopls/internal/cache/parsego"
	"golang.org/x/tools/gopls/internal/file"
	"golang.org/x/tools/gopls/internal/protocol"
	"golang.org/x/tools/gopls/mcpbridge/api"
)

// This file provides a "Semantic Bridge" for LLMs to query gopls internal APIs
// without going through the LSP protocol layer (no protocol.Position needed).
//
// It directly resolves semantic intent (SymbolLocator) to gopls internal types
// (ast.Node, types.Object) and calls internal logic directly.

// ResolveNodeResult is the output of ResolveNode, containing rich information
// about the resolved symbol.
type ResolveNodeResult struct {
	// Node is the AST node representing the symbol (e.g., *ast.Ident, *ast.TypeSpec, *ast.FuncDecl)
	Node ast.Node
	// Pos is the token position of the symbol
	Pos token.Pos
	// Object is the types.Object representing the symbol (if type-checking succeeded)
	Object types.Object
	// EnclosingFunc is the name of the function containing this symbol
	EnclosingFunc string
	// IsDefinition indicates whether this node is a definition (not just a reference)
	IsDefinition bool
}

// SourceContext provides rich context about a symbol, suitable for LLM consumption.
type SourceContext struct {
	// File allows the LLM to know which file to modify later.
	File string `json:"file"`

	// Line numbers are sufficient for human reference or coarse-grained location.
	// Columns are removed because they are noisy and prone to tokenizer errors.
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`

	// Symbol name (e.g., "Start")
	Symbol string `json:"symbol"`

	// Kind helps the LLM distinguish between "struct", "interface", "method", "func".
	// This is cheaply available from types.Object.
	Kind string `json:"kind"`

	// Signature helps the LLM index results quickly (e.g., "func (s *Server) Start(ctx context.Context) error")
	Signature string `json:"signature"`

	// DocComment conveys intent (e.g., "Start initializes the server...")
	DocComment string `json:"doc_comment,omitempty"`

	// Snippet is the HERO field. Zero-RTT access to the code.
	Snippet string `json:"snippet"`
}

// ResolveNode resolves a SymbolLocator to the matching AST node and types.Object.
//
// It performs a stateful AST walk with proper parent scope tracking to match
// the semantic criteria in the SymbolLocator:
//   - Strict symbol name matching
//   - Optional package name filtering (for imports)
//   - Optional parent scope filtering (for receivers or local variables)
//   - Optional kind filtering (function, method, struct, etc.)
//   - Fuzzy line hint matching (for disambiguation)
//
// This function bypasses the LSP protocol layer entirely and works directly
// with gopls internal types.
func ResolveNode(ctx context.Context, snapshot *cache.Snapshot, fh file.Handle, locator api.SymbolLocator) (*ResolveNodeResult, error) {
	// Get the package and parse tree for the context file
	pkg, pgf, err := NarrowestPackageForFile(ctx, snapshot, fh.URI())
	if err != nil {
		return nil, fmt.Errorf("failed to get package for %s: %w", locator.ContextFile, err)
	}

	// Build a cursor for efficient AST traversal
	info := pkg.TypesInfo()

	// Find all matching symbols with scope tracking
	scopeStack := []scopeFrame{{node: pgf.File, enclosingFunc: ""}}

	var candidates []ResolveNodeResult
	var bestCandidate *ResolveNodeResult

	// Walk the AST to find matching symbols
	ast.Inspect(pgf.File, func(n ast.Node) bool {
		if n == nil {
			// Pop from scope stack when returning
			if len(scopeStack) > 1 {
				scopeStack = scopeStack[:len(scopeStack)-1]
			}
			return false
		}

		// Update scope stack for this node
		currentFrame := updateScopeStack(n, scopeStack)
		scopeStack = append(scopeStack, currentFrame)

		// Only process identifiers that match the symbol name
		ident, ok := n.(*ast.Ident)
		if !ok || ident.Name != locator.SymbolName {
			return true
		}

		// Get the type information for this identifier
		obj := info.Uses[ident]
		isDef := false
		if obj == nil {
			obj = info.Defs[ident]
			isDef = (obj != nil)
		}

		// Extract parent scope and kind information
		parentInfo := extractNodeParentAndKind(ident, obj, scopeStack)

		// Apply filters
		if !matchesLocatorFilters(locator, parentInfo.parent, parentInfo.kind).passed {
			return true
		}

		// Found a match!
		candidate := ResolveNodeResult{
			Node:          n,
			Pos:           ident.Pos(),
			Object:        obj,
			EnclosingFunc: scopeStack[len(scopeStack)-1].enclosingFunc,
			IsDefinition:  isDef,
		}

		candidates = append(candidates, candidate)
		bestCandidate = selectBestCandidate(bestCandidate, &candidate, pgf, locator, isDef)

		return true
	})

	if bestCandidate == nil {
		if len(candidates) == 0 {
			return nil, fmt.Errorf("symbol '%s' not found in file", locator.SymbolName)
		}
		bestCandidate = &candidates[0]
	}

	return bestCandidate, nil
}

// LLMImplementation finds all implementations of an interface method or type.
//
// It takes a SymbolLocator (semantic identifier) and returns SourceContext for each
// implementation, including the symbol's location, signature, documentation, and code snippet.
//
// Use cases:
//   - Find all concrete types implementing an interface
//   - Find all implementations of an interface method
//   - Discover type hierarchies in the codebase
//
// Limitations:
//   - Only finds implementations of interfaces defined in the codebase
//   - Does NOT work with standard library interfaces (io.Reader, error, fmt.Stringer, etc.)
//   - May not detect method promotion from embedded structs
//   - Requires the interface to be defined in the workspace (not vendored/external)
//
// Example - Find implementations of Writer interface:
//
//	locator := api.SymbolLocator{
//	    SymbolName:  "Write",
//	    ParentScope: "Writer",  // interface name
//	    Kind:        "method",
//	    ContextFile: "/path/to/interfaces.go",
//	}
//	impls, err := golang.LLMImplementation(ctx, snapshot, locator)
//	// Returns: File, FileWriter, ConsoleWriter, etc.
//
// Example - Find what interfaces a type implements:
//
//	locator := api.SymbolLocator{
//	    SymbolName:  "FileWriter",
//	    Kind:        "struct",
//	    ContextFile: "/path/to/file.go",
//	}
//	impls, err := golang.LLMImplementation(ctx, snapshot, locator)
//	// Returns: Writer, io.Closer, etc.
//
// This bypasses the LSP protocol layer and works directly with gopls internals,
// making it faster and more accurate than text-based search.
func LLMImplementation(ctx context.Context, snapshot *cache.Snapshot, locator api.SymbolLocator) ([]SourceContext, error) {
	// Use the unified ResolveSymbol function with default options (include docs and bodies)
	info, err := ResolveSymbol(ctx, snapshot, locator, ResolveOptions{
		FindImplementations: true,
		IncludeDocs:         true,
		IncludeBodies:       true,
	})
	if err != nil {
		return nil, err
	}

	return info.Implementations, nil
}

// ===== Helper Functions for ResolveNode =====
// These functions are extracted from ResolveNode for better testability.

// scopeFrame represents a frame in the scope stack during AST traversal.
type scopeFrame struct {
	node          ast.Node
	enclosingFunc string
}

// updateScopeStack updates the scope stack for the given node.
// It returns the updated frame that should be pushed onto the stack.
func updateScopeStack(n ast.Node, currentStack []scopeFrame) scopeFrame {
	currentFrame := scopeFrame{node: n, enclosingFunc: currentStack[len(currentStack)-1].enclosingFunc}

	// Update enclosing scope if this is a function declaration
	if fn, ok := n.(*ast.FuncDecl); ok {
		if fn.Recv == nil {
			currentFrame.enclosingFunc = fn.Name.Name
		} else {
			recvTypeName := getReceiverTypeName(fn.Recv)
			currentFrame.enclosingFunc = fmt.Sprintf("(%s).%s", recvTypeName, fn.Name.Name)
		}
	}

	// Update enclosing scope if this is a type declaration (struct/interface)
	// This allows us to correctly resolve fields within their parent type
	if spec, ok := n.(*ast.TypeSpec); ok {
		currentFrame.enclosingFunc = spec.Name.Name
	}

	return currentFrame
}

// nodeParentInfo contains the extracted parent scope and kind information for a node.
type nodeParentInfo struct {
	parent string
	kind   string
}

// extractNodeParentAndKind extracts the parent scope and kind for a given identifier.
// It uses both type information and AST structure to determine the parent.
func extractNodeParentAndKind(ident *ast.Ident, obj types.Object, currentStack []scopeFrame) nodeParentInfo {
	var nodeParent string
	var nodeKind string

	if obj != nil {
		// Get parent from object (for methods, fields, etc.)
		nodeParent = getParentScope(obj, currentStack[len(currentStack)-1].enclosingFunc)
		nodeKind = getKindString(obj)
	} else {
		// No type info available - use scope stack
		nodeParent = currentStack[len(currentStack)-1].enclosingFunc
	}

	// Check if this is a selector expression (e.g., fmt.Println)
	// We need to check the parent to see if this is part of a SelectorExpr
	if parent := currentStack[len(currentStack)-1].node; parent != nil {
		if sel, ok := parent.(*ast.SelectorExpr); ok && sel.Sel == ident {
			// This ident is the selector part of a SelectorExpr
			// Extract the base identifier, handling nested selectors
			nodeParent = extractSelector(sel.X)
		}
	}

	return nodeParentInfo{parent: nodeParent, kind: nodeKind}
}

// filterResult indicates whether a node passed all filters and why it didn't.
type filterResult struct {
	passed bool
	reason string
}

// matchesLocatorFilters checks if a node matches all the filters specified in the locator.
func matchesLocatorFilters(locator api.SymbolLocator, nodeParent, nodeKind string) filterResult {
	// Parent scope filter
	if locator.ParentScope != "" {
		normalizeParent := func(s string) string {
			return strings.TrimPrefix(s, "*")
		}
		if normalizeParent(nodeParent) != normalizeParent(locator.ParentScope) {
			return filterResult{false, "parent scope mismatch"}
		}
	}

	// Kind filter
	if locator.Kind != "" && nodeKind != "" {
		if !normalizeKindMatches(nodeKind, locator.Kind) {
			return filterResult{false, "kind mismatch"}
		}
	}

	return filterResult{passed: true}
}

// candidateScore represents the score of a candidate for selection.
type candidateScore struct {
	candidate    *ResolveNodeResult
	confidence   float64
	isDefinition bool
}

// scoreCandidate calculates a score for a candidate based on the locator's preferences.
func scoreCandidate(candidate ResolveNodeResult, pgf *parsego.File, locator api.SymbolLocator) candidateScore {
	score := candidateScore{
		candidate:    &candidate,
		isDefinition: candidate.IsDefinition,
	}

	if locator.LineHint > 0 {
		pos := pgf.Tok.Position(candidate.Pos)
		line := pos.Line
		distance := abs(line - int(locator.LineHint))
		score.confidence = 1.0 / float64(distance+1)
	} else {
		score.confidence = 0.5 // Default confidence when no line hint
	}

	return score
}

// selectBestCandidate selects the best candidate from the current best and a new candidate.
func selectBestCandidate(current, new *ResolveNodeResult, pgf *parsego.File, locator api.SymbolLocator, newIsDef bool) *ResolveNodeResult {
	if current == nil {
		return new
	}

	if locator.LineHint > 0 {
		// Use line hint to pick the best match
		newScore := scoreCandidate(*new, pgf, locator)
		currentScore := scoreCandidate(*current, pgf, locator)

		if newScore.confidence > currentScore.confidence {
			return new
		}
		return current
	}

	// No line hint, prefer definitions over references
	if newIsDef && !current.IsDefinition {
		return new
	}

	return current
}

// Helper functions

// extractSelector extracts the base identifier from a selector expression.
// For example:
//   - "fmt.Println" -> "fmt"
//   - "pkg.subpkg.Symbol" -> "pkg"
//   - "x.y.Symbol" -> "x"
//
// This handles nested selectors by recursively extracting the leftmost identifier.
func extractSelector(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.SelectorExpr:
		// Recursively extract from the left side of the selector
		return extractSelector(expr.X)
	case *ast.ParenExpr:
		// Handle parenthesized expressions: (*ptr).Method
		return extractSelector(expr.X)
	default:
		return ""
	}
}

// getReceiverTypeName extracts the receiver type name from a receiver field list
func getReceiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}

	expr := recv.List[0].Type
	switch t := expr.(type) {
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return "*" + ident.Name
		}
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr, *ast.IndexListExpr:
		// TODO: Support generic receivers (e.g., Map[K, V])
		// Currently returns empty string, which will cause parent scope filtering to fail
		// for methods on generic types. To fix this, we need to extract the base type name
		// from the index expression (e.g., "Map" from "Map[K, V]").
		return ""
	}
	return ""
}

// getParentScope extracts the parent scope name from a types.Object
func getParentScope(obj types.Object, enclosingFunc string) string {
	if obj == nil {
		return enclosingFunc
	}

	// Check if it's a method
	if fn, ok := obj.(*types.Func); ok {
		if sig, ok := fn.Type().(*types.Signature); ok {
			recv := sig.Recv()
			if recv != nil {
				// Method with receiver
				named := getNamedType(recv.Type())
				if named != nil {
					return "*" + named.Obj().Name()
				}
			}
		}
	}

	// Check if it's a field
	if v, ok := obj.(*types.Var); ok && v.IsField() {
		// The scopeStack now tracks TypeSpec nodes, so enclosingFunc
		// will contain the struct/interface name for fields.
		// Example: type Server struct { port int } -> enclosingFunc = "Server"
		return enclosingFunc
	}

	return enclosingFunc
}

// getNamedType extracts the *types.Named from a type
func getNamedType(t types.Type) *types.Named {
	switch t := t.(type) {
	case *types.Named:
		return t
	case *types.Pointer:
		return getNamedType(t.Elem())
	default:
		return nil
	}
}

// getKindString returns the kind string for a types.Object
func getKindString(obj types.Object) string {
	switch obj.(type) {
	case *types.Func:
		return "function"
	case *types.Var:
		if obj.(*types.Var).IsField() {
			return "field"
		}
		return "variable"
	case *types.Const:
		return "const"
	case *types.TypeName:
		return "type"
	default:
		return ""
	}
}

// normalizeKindMatches checks if two kind strings match after normalization
func normalizeKindMatches(a, b string) bool {
	return normalizeKind(a) == normalizeKind(b)
}

func normalizeKind(kind string) string {
	kind = strings.ToLower(kind)
	switch kind {
	case "function", "method":
		return "function"
	case "struct", "interface", "type":
		return "type"
	case "variable", "field":
		return kind
	case "const":
		return kind
	default:
		return kind
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// sourceContextFromLocation converts a protocol.Location to a minimal SourceContext.
// This is used as a fallback when we can't get full symbol information.
func sourceContextFromLocation(snapshot *cache.Snapshot, loc protocol.Location) SourceContext {
	return SourceContext{
		File:      loc.URI.Path(),
		StartLine: int(loc.Range.Start.Line + 1),
		EndLine:   int(loc.Range.End.Line + 1),
	}
}

// findNodeAtPos finds the declaration node at a given position in a file.
// It searches upward from the identifier to find the enclosing declaration
// (e.g., *ast.FuncDecl, *ast.TypeSpec, *ast.ValueSpec) that contains the doc comment.
func findNodeAtPos(pgf *parsego.File, line, col uint32) ast.Node {
	// Convert LSP position (0-indexed) to offset
	offset, err := pgf.Mapper.PositionOffset(protocol.Position{Line: line, Character: col})
	if err != nil {
		return nil
	}

	// Convert offset to token.Pos
	pos := pgf.Tok.Pos(offset)
	if !pos.IsValid() {
		return nil
	}

	// Use cursor to find the node at this position
	cur, ok := pgf.Cursor().FindByPos(pos, pos)
	if !ok {
		return nil
	}

	// Walk up the tree to find a declaration node with documentation
	for {
		node := cur.Node()
		if node == nil {
			break
		}

		// Check if this is a declaration node type
		switch node.(type) {
		case *ast.FuncDecl, *ast.GenDecl, *ast.TypeSpec, *ast.ValueSpec:
			return node
		}

		cur = cur.Parent()
	}

	return nil
}

// findIdentifierAtPos finds the identifier at a given position in a file
func findIdentifierAtPos(pgf *parsego.File, line, col uint32) *ast.Ident {
	// Convert LSP position (0-indexed) to offset
	offset, err := pgf.Mapper.PositionOffset(protocol.Position{Line: line, Character: col})
	if err != nil {
		return nil
	}

	// Convert offset to token.Pos
	pos := pgf.Tok.Pos(offset)
	if !pos.IsValid() {
		return nil
	}

	// Use cursor to find the node at this position
	// FindByPos requires both start and end positions
	cur, ok := pgf.Cursor().FindByPos(pos, pos)
	if !ok {
		return nil
	}

	// Check if the current node is an identifier
	if ident, ok := cur.Node().(*ast.Ident); ok {
		return ident
	}

	// Walk up to find the enclosing identifier
	for {
		node := cur.Node()
		if node == nil {
			break
		}

		if ident, ok := node.(*ast.Ident); ok {
			return ident
		}

		cur = cur.Parent()
	}
	return nil
}

// formatObjectString returns a string representation of a types.Object
func formatObjectString(obj types.Object) string {
	if obj == nil {
		return ""
	}

	switch obj := obj.(type) {
	case *types.Func:
		return obj.String()
	case *types.Var:
		return obj.String()
	case *types.Const:
		return obj.String()
	case *types.TypeName:
		return obj.String()
	default:
		return fmt.Sprintf("%v", obj)
	}
}

// extractDocComment extracts the documentation comment for an object from its AST node.
func extractDocComment(node ast.Node) string {
	if node == nil {
		return ""
	}

	// Check different node types for documentation
	switch n := node.(type) {
	case *ast.FuncDecl:
		if n.Doc != nil {
			return n.Doc.Text()
		}
	case *ast.GenDecl:
		if n.Doc != nil {
			return n.Doc.Text()
		}
	case *ast.TypeSpec:
		if n.Doc != nil {
			return n.Doc.Text()
		}
	}

	return ""
}

// buildSourceContext creates a rich SourceContext from types.Object and ast.Node.
// This is the core function that populates all fields in SourceContext.
//
// Parameters:
//   - fset: FileSet for position information
//   - obj: types.Object containing type information
//   - node: ast.Node containing syntax tree and documentation
func buildSourceContext(fset *token.FileSet, obj types.Object, node ast.Node) SourceContext {
	// 1. Position information
	pos := node.Pos()
	end := node.End()
	position := fset.Position(pos)
	endPos := fset.Position(end)

	// 2. Extract Kind
	kind := getKindFromObject(obj)

	// 3. Extract Snippet (HERO field)
	var snippet string
	var snippetBuf bytes.Buffer
	if err := printer.Fprint(&snippetBuf, fset, node); err != nil {
		// Fallback: try to format the node
		snippet = ""
	} else {
		// Try to format the snippet for better readability
		formatted, err := format.Source(snippetBuf.Bytes())
		if err != nil {
			// If formatting fails, use the raw output
			snippet = snippetBuf.String()
		} else {
			snippet = string(formatted)
		}
	}

	// 4. Extract DocComment from AST node
	docComment := extractDocComment(node)

	// 5. Build signature using gopls's type formatter
	signature := types.ObjectString(obj, nil) // nil = no qualifier needed
	// Clean up verbose package paths in signatures for LLM readability
	signature = cleanSignature(signature)

	return SourceContext{
		File:       position.Filename,
		StartLine:  position.Line,
		EndLine:    endPos.Line,
		Symbol:     obj.Name(),
		Kind:       kind,
		Signature:  signature,
		Snippet:    snippet,
		DocComment: docComment,
	}
}

// cleanSignature removes verbose package paths from type signatures.
// This makes signatures more readable and token-efficient for LLMs.
//
// Before: func (command-line-arguments/path/to/file/main.go.FileWriter).Write(data string) error
// After:  func (FileWriter).Write(data string) error
func cleanSignature(sig string) string {
	// Remove package paths from receiver types
	// Pattern: (package.path.to.file.Type) -> (Type)
	// This handles both regular and command-line-arguments package prefixes

	// First, handle receiver declarations like: func (pkg/path.Type)Method(...)
	// We want to keep: func (Type)Method(...)

	result := sig

	// Match receiver patterns: (full.package.path.TypeName) or (*full.package.path.TypeName)
	// and replace with just (TypeName) or (*TypeName)

	// This regex finds patterns like:
	// - (command-line-arguments/.../file.go.Type)
	// - (example.com/pkg.Type)
	// - (*example.com/pkg.Type)
	// And extracts just the type name without the package path

	// For now, use a simple heuristic: find the last ')' before the first '.' in the receiver
	// and remove everything from '(' up to and including the last '/'

	// Split by lines to handle multi-line signatures
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		// Look for receiver pattern at the start of the function declaration
		if strings.HasPrefix(line, "func (") {
			// Find the closing paren of the receiver
			receiverEnd := strings.Index(line[5:], ")")
			if receiverEnd == -1 {
				continue
			}
			receiverEnd += 5 // Adjust for "func ("

			// Extract the receiver declaration
			receiver := line[5:receiverEnd]

			// Clean up the receiver by removing package paths
			cleanReceiver := cleanReceiverType(receiver)

			// Reconstruct the line
			lines[i] = "func " + cleanReceiver + line[receiverEnd+1:]
		}
	}

	return strings.Join(lines, "\n")
}

// cleanReceiverType removes package paths from a receiver type declaration.
// Examples:
//   - "(command-line-arguments/path/file.go.FileWriter)" -> "(FileWriter)"
//   - "(*example.com/pkg.Type)" -> "(*Type)"
//   - "(Type)" -> "(Type)" (unchanged)
func cleanReceiverType(receiver string) string {
	// Check if there's a pointer
	isPointer := strings.HasPrefix(receiver, "(*")
	if isPointer {
		receiver = receiver[2:] // Remove "(*"
	} else if strings.HasPrefix(receiver, "(") {
		receiver = receiver[1:] // Remove "("
	}

	// Find the last occurrence of path separators
	// Common patterns: "pkg/path.Type" or "path/to/file.Type"
	lastSlash := strings.LastIndex(receiver, "/")
	lastDot := strings.LastIndex(receiver, ".")

	// The type name should be after the last path separator
	var typeName string
	if lastSlash != -1 && lastDot > lastSlash {
		typeName = receiver[lastDot+1:]
	} else if lastSlash != -1 {
		typeName = receiver[lastSlash+1:]
	} else {
		typeName = receiver
	}

	// Remove trailing paren if present
	typeName = strings.TrimSuffix(typeName, ")")

	// Reconstruct with pointer if needed
	if isPointer {
		return "(*" + typeName + ")"
	}
	return "(" + typeName + ")"
}

// getKindFromObject determines the kind of a types.Object.
func getKindFromObject(obj types.Object) string {
	if obj == nil {
		return "unknown"
	}

	switch obj.(type) {
	case *types.Func:
		// Distinguish between method and function
		if sig, ok := obj.Type().(*types.Signature); ok && sig.Recv() != nil {
			return "method"
		}
		return "function"
	case *types.TypeName:
		// Try to determine if it's a struct or interface from the AST
		// This is a best-effort approximation
		return "type"
	case *types.Var:
		if v, ok := obj.(*types.Var); ok && v.IsField() {
			return "field"
		}
		return "variable"
	case *types.Const:
		return "const"
	default:
		return "unknown"
	}
}

// ===== AST Utility Functions =====
// These functions provide AST-level operations for symbol extraction and analysis.

// FindSymbolPosition searches for a symbol by name in a parsed Go file.
// It returns the token.Pos of the symbol declaration.
//
// This is useful for locating symbols without full type information,
// or for quick lookups when you only need the position.
func FindSymbolPosition(pgf *parsego.File, symbolName string) (token.Pos, bool) {
	// Search for function declarations
	for _, decl := range pgf.File.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if decl.Name != nil && decl.Name.Name == symbolName {
				return decl.Name.Pos(), true
			}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					if spec.Name != nil && spec.Name.Name == symbolName {
						return spec.Name.Pos(), true
					}
				case *ast.ValueSpec:
					for _, n := range spec.Names {
						if n.Name == symbolName {
							return n.Pos(), true
						}
					}
				}
			}
		}
	}
	return token.NoPos, false
}

// ExtractBodyText extracts the source text of an AST block statement.
// It cleans up the text by removing extra whitespace and newlines.
//
// This is useful for extracting function bodies for display or analysis.
func ExtractBodyText(pgf *parsego.File, body *ast.BlockStmt) string {
	if body == nil {
		return ""
	}

	// Get positions as token.Pos
	startPos := body.Pos()
	endPos := body.End()

	// Convert to byte offsets, adjusting for file start
	fileStart := pgf.File.FileStart
	start := startPos - fileStart
	end := endPos - fileStart

	// Get the source content
	content := pgf.Src

	// Extract the body text
	if start >= 0 && int32(end) <= int32(len(content)) && start < end {
		bodyText := string(content[start:end])

		// Clean up: remove extra whitespace and newlines
		bodyText = strings.TrimSpace(bodyText)
		bodyText = strings.ReplaceAll(bodyText, "\n", " ")
		bodyText = strings.ReplaceAll(bodyText, "\t", " ")
		// Collapse multiple spaces
		for strings.Contains(bodyText, "  ") {
			bodyText = strings.ReplaceAll(bodyText, "  ", " ")
		}

		return bodyText
	}

	return ""
}

// ExtractBodyForSymbol extracts the function body text for a symbol by name.
// It searches for the function declaration in the given file and returns
// the cleaned body text.
//
// This is a convenience function that combines FindSymbolPosition and ExtractBodyText.
func ExtractBodyForSymbol(ctx context.Context, snapshot *cache.Snapshot, symbolName string, defFile string) string {
	// Read the file at the definition location
	uri := protocol.URIFromPath(defFile)
	defFh, err := snapshot.ReadFile(ctx, uri)
	if err != nil {
		return ""
	}

	pgf, err := snapshot.ParseGo(ctx, defFh, parsego.Full)
	if err != nil {
		return ""
	}

	// Find the function declaration by name
	for _, decl := range pgf.File.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Body != nil {
			if fd.Name != nil && fd.Name.Name == symbolName {
				return ExtractBodyText(pgf, fd.Body)
			}
		}
	}

	return ""
}

// ConvertLSPSymbolKind converts LSP SymbolKind to API SymbolKind.
// This provides a mapping between the LSP protocol's symbol kind enumeration
// and our API's SymbolKind type.
func ConvertLSPSymbolKind(kind protocol.SymbolKind) api.SymbolKind {
	switch kind {
	case protocol.Function:
		return api.SymbolKindFunction
	case protocol.Method:
		return api.SymbolKindMethod
	case protocol.Struct:
		return api.SymbolKindStruct
	case protocol.Interface:
		return api.SymbolKindInterface
	case protocol.Variable:
		return api.SymbolKindVariable
	case protocol.Constant:
		return api.SymbolKindConstant
	case protocol.Field:
		return api.SymbolKindField
	case protocol.Property:
		return api.SymbolKindField // Treat properties as fields
	case protocol.Class:
		return api.SymbolKindStruct // Treat classes as structs
	case protocol.Module:
		return api.SymbolKindType // Treat modules as types
	case protocol.Package:
		return api.SymbolKindType // Treat packages as types
	default:
		return api.SymbolKindType
	}
}

// ===== LLMRename - Semantic Bridge for Rename Operations =====

// LLMRename performs a dry-run rename operation and returns both unified diff and line changes.
//
// This is a high-level function that:
// 1. Uses ResolveNode to find the symbol position
// 2. Calls the internal Rename logic to compute changes
// 3. Returns both a unified diff format and LLM-friendly line changes
//
// This bypasses the LSP protocol layer and works directly with gopls internals.
func LLMRename(ctx context.Context, snapshot *cache.Snapshot, locator api.SymbolLocator, newName string) (string, []api.RenameChange, error) {
	// First, resolve the node to get the position
	fh, err := snapshot.ReadFile(ctx, protocol.URIFromPath(locator.ContextFile))
	if err != nil {
		return "", nil, fmt.Errorf("failed to read file: %w", err)
	}

	result, err := ResolveNode(ctx, snapshot, fh, locator)
	if err != nil {
		return "", nil, err
	}

	// Convert token.Pos to protocol.Position
	pkg, _, err := NarrowestPackageForFile(ctx, snapshot, fh.URI())
	if err != nil {
		return "", nil, fmt.Errorf("failed to get package: %w", err)
	}

	posn := pkg.FileSet().Position(result.Pos)
	if !posn.IsValid() {
		return "", nil, fmt.Errorf("invalid position for symbol '%s'", locator.SymbolName)
	}

	position := protocol.Position{
		Line:      uint32(posn.Line - 1),
		Character: uint32(posn.Column - 1),
	}

	// Call the internal Rename function
	changes, err := Rename(ctx, snapshot, fh, protocol.Range{Start: position, End: position}, newName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to compute rename: %w", err)
	}

	// Convert changes to unified diff format
	unifiedDiff, err := generateUnifiedDiff(ctx, snapshot, changes)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate unified diff: %w", err)
	}

	// Convert changes to LLM-friendly line-based format
	lineChanges, err := generateLineChanges(ctx, snapshot, changes)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate line changes: %w", err)
	}

	return unifiedDiff, lineChanges, nil
}

// ===== Symbol Resolution Infrastructure =====
// This section provides unified symbol resolution infrastructure.
// All symbol-based operations (definition, implementations, references, etc.)
// go through ResolveSymbol, making the logic easy to test and extend.

// SymbolInfo contains comprehensive information about a symbol lookup.
// Different operations populate different fields.
type SymbolInfo struct {
	// Locations contains all definition locations for the symbol.
	// For most symbols, this is a single location. For interface methods,
	// there may be multiple locations (all concrete implementations).
	Locations []protocol.Location `json:"locations"`

	// Implementations contains all implementations (if this is an interface)
	// or all interfaces (if this is a concrete type).
	// Populated when Options.FindImplementations = true.
	Implementations []SourceContext `json:"implementations,omitempty"`

	// Definition contains the full symbol information at its definition location.
	// Populated when Options.IncludeDefinition = true.
	Definition *SourceContext `json:"definition,omitempty"`

	// References contains all usage locations for this symbol.
	// Populated when Options.FindReferences = true.
	References []SourceContext `json:"references,omitempty"`
}

// ResolveOptions controls what information is fetched during symbol resolution.
// By default, only the minimal required data is fetched to avoid expensive operations.
type ResolveOptions struct {
	// FindDefinitions fetches all definition locations for the symbol.
	// Default: true (most operations need to know where the symbol is defined)
	FindDefinitions bool `json:"find_definitions"`

	// FindImplementations finds all implementations (for interfaces) or
	// all implemented interfaces (for concrete types).
	// This uses the internal methodsets package and can be expensive.
	// Default: false
	FindImplementations bool `json:"find_implementations"`

	// IncludeDefinition populates the Definition field with rich symbol
	// information (signature, documentation, snippet) from the primary definition.
	// Default: false
	IncludeDefinition bool `json:"include_definition"`

	// IncludeBodies includes function/method bodies in implementation results.
	// Only used when FindImplementations = true.
	// Default: false
	IncludeBodies bool `json:"include_bodies"`

	// IncludeDocs includes documentation comments in implementation results.
	// Only used when FindImplementations = true.
	// Default: false
	IncludeDocs bool `json:"include_docs"`
}

// ResolveSymbol is the unified entry point for all symbol-based operations.
//
// It resolves a SymbolLocator to concrete symbol information and optionally:
// - Finds all definition locations
// - Finds all implementations (for interfaces) or interfaces (for types)
// - Finds all references (future)
//
// This function centralizes all symbol resolution logic, making it easier to:
// - Test: One function to test with different options
// - Extend: Add new operations (references, callers, etc.) without duplicating resolution logic
// - Optimize: Cache results, batch operations, etc.
//
// Example - Find where a symbol is defined:
//
//	info, err := golang.ResolveSymbol(ctx, snapshot, locator, golang.ResolveOptions{
//	    FindDefinitions: true,
//	})
//	for _, loc := range info.Locations {
//	    fmt.Printf("Defined at %s:%d\n", loc.URI.Path(), loc.Range.Start.Line)
//	}
//
// Example - Find all implementations of an interface:
//
//	info, err := golang.ResolveSymbol(ctx, snapshot, locator, golang.ResolveOptions{
//	    FindImplementations: true,
//	    IncludeDocs: true,
//	})
//	for _, impl := range info.Implementations {
//	    fmt.Printf("- %s: %s\n", impl.Symbol, impl.Signature)
//	}
func ResolveSymbol(ctx context.Context, snapshot *cache.Snapshot, locator api.SymbolLocator, options ResolveOptions) (*SymbolInfo, error) {
	info := &SymbolInfo{}

	// Default: always find definitions (it's fast and most operations need it)
	if !options.FindDefinitions && !options.FindImplementations {
		return info, fmt.Errorf("at least one of FindDefinitions or FindImplementations must be true")
	}

	// Step 1: Resolve the locator to get the symbol position
	fh, err := snapshot.ReadFile(ctx, protocol.URIFromPath(locator.ContextFile))
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	result, err := ResolveNode(ctx, snapshot, fh, locator)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve symbol: %w", err)
	}

	if result.Object == nil {
		return nil, fmt.Errorf("symbol '%s' has no type information", locator.SymbolName)
	}

	// Step 2: Get package and position for gopls operations
	pkg, pgf, err := NarrowestPackageForFile(ctx, snapshot, fh.URI())
	if err != nil {
		return nil, fmt.Errorf("failed to get package: %w", err)
	}

	posn := pkg.FileSet().Position(result.Pos)
	if !posn.IsValid() {
		return nil, fmt.Errorf("invalid position for symbol '%s'", locator.SymbolName)
	}

	position := protocol.Position{
		Line:      uint32(posn.Line - 1),
		Character: uint32(posn.Column - 1),
	}

	// Step 3: Find definitions (if requested)
	if options.FindDefinitions {
		locations, err := Definition(ctx, snapshot, fh, protocol.Range{Start: position, End: position})
		if err != nil {
			return nil, fmt.Errorf("failed to find definitions: %w", err)
		}
		info.Locations = locations

		// Optionally include definition details
		if options.IncludeDefinition && len(locations) > 0 {
			// Get the primary definition location
			defLoc := locations[0]

			// Find the AST node at the definition location (contains doc comment)
			defNode := findNodeAtPos(pgf, defLoc.Range.Start.Line, defLoc.Range.Start.Character)

			// Build rich source context if we have both object and node
			if result.Object != nil && defNode != nil {
				srcCtx := buildSourceContext(pkg.FileSet(), result.Object, defNode)
				info.Definition = &srcCtx
			} else {
				// Fallback to minimal source context
				srcCtx := sourceContextFromLocation(snapshot, defLoc)
				if result.Object != nil {
					srcCtx.Signature = formatObjectString(result.Object)
					srcCtx.Kind = getKindFromObject(result.Object)
				}
				info.Definition = &srcCtx
			}
		}
	}

	// Step 4: Find implementations (if requested)
	if options.FindImplementations {
		// Build a cursor from the resolved position
		cur, ok := pgf.Cursor().FindByPos(result.Pos, result.Pos)
		if !ok {
			return info, fmt.Errorf("failed to create cursor for position")
		}

		// Use the internal implementations logic
		const relation = methodsets.TypeRelation(0) // infer direction

		err = implementationsMsets(ctx, snapshot, pkg, cur, relation, func(_ metadata.PackagePath, _ string, _ bool, loc protocol.Location) {
			// Get symbol information from the package containing the implementation
			implPkg, implPgf, err := NarrowestPackageForFile(ctx, snapshot, loc.URI)
			if err != nil {
				// Fallback to minimal source context
				info.Implementations = append(info.Implementations, sourceContextFromLocation(snapshot, loc))
				return
			}

			// Find the identifier at the implementation location
			implIdent := findIdentifierAtPos(implPgf, loc.Range.Start.Line, loc.Range.Start.Character)
			if implIdent == nil {
				info.Implementations = append(info.Implementations, sourceContextFromLocation(snapshot, loc))
				return
			}

			// Get the types.Object for this identifier
			implObj := implPkg.TypesInfo().Defs[implIdent]
			if implObj == nil {
				implObj = implPkg.TypesInfo().Uses[implIdent]
			}

			// Find the AST node for this implementation (contains doc comment)
			implNode := findNodeAtPos(implPgf, loc.Range.Start.Line, loc.Range.Start.Character)

			// Build rich source context if we have both object and node
			if implObj != nil && implNode != nil {
				srcCtx := buildSourceContext(pkg.FileSet(), implObj, implNode)
				info.Implementations = append(info.Implementations, srcCtx)
			} else {
				// Fallback to minimal source context
				srcCtx := sourceContextFromLocation(snapshot, loc)
				srcCtx.Symbol = implIdent.Name
				if implObj != nil {
					srcCtx.Signature = formatObjectString(implObj)
				}
				info.Implementations = append(info.Implementations, srcCtx)
			}
		})
		if err != nil {
			return info, fmt.Errorf("failed to find implementations: %w", err)
		}

		// Filter implementations if bodies/docs were not requested
		if !options.IncludeBodies || !options.IncludeDocs {
			filtered := make([]SourceContext, 0, len(info.Implementations))
			for _, impl := range info.Implementations {
				if !options.IncludeBodies {
					impl.Snippet = ""
				}
				if !options.IncludeDocs {
					impl.DocComment = ""
				}
				filtered = append(filtered, impl)
			}
			info.Implementations = filtered
		}
	}

	return info, nil
}

// GoDefinition finds the definition location(s) for a symbol identified by a SymbolLocator.
func generateUnifiedDiff(ctx context.Context, snapshot *cache.Snapshot, changes []protocol.DocumentChange) (string, error) {
	var diff strings.Builder

	// Group changes by file
	type fileChange struct {
		uri      protocol.DocumentURI
		edits    []protocol.TextEdit
		content  string
		original string
	}

	fileChanges := make(map[protocol.DocumentURI]*fileChange)

	for _, docChange := range changes {
		if docChange.TextDocumentEdit == nil {
			continue
		}

		edit := docChange.TextDocumentEdit
		uri := edit.TextDocument.URI

		// Read the original file content
		fh, err := snapshot.ReadFile(ctx, uri)
		if err != nil {
			continue // Skip files we can't read
		}

		contentBytes, err := fh.Content()
		if err != nil {
			continue
		}

		originalContent := string(contentBytes)

		// Get text edits
		textEdits := protocol.AsTextEdits(edit.Edits)

		if _, exists := fileChanges[uri]; !exists {
			fileChanges[uri] = &fileChange{
				uri:      uri,
				edits:    textEdits,
				original: originalContent,
			}
		} else {
			// Append edits if file already exists
			fileChanges[uri].edits = append(fileChanges[uri].edits, textEdits...)
		}
	}

	// Generate diff for each file
	for uri, fc := range fileChanges {
		// Sort edits by position (descending order to apply from end to start)
		sortEdits(fc.edits)

		// Apply edits to get modified content
		modified := applyEdits(fc.original, fc.edits)

		// Generate unified diff for this file
		fileDiff := unifiedDiffForFile(uri.Path(), fc.original, modified)
		diff.WriteString(fileDiff)
		diff.WriteString("\n")
	}

	return diff.String(), nil
}

// generateLineChanges converts DocumentChange results to LLM-friendly line-based format.
// This format provides complete line content for easy verification and rewriting by LLMs.
func generateLineChanges(ctx context.Context, snapshot *cache.Snapshot, changes []protocol.DocumentChange) ([]api.RenameChange, error) {
	var lineChanges []api.RenameChange

	for _, docChange := range changes {
		if docChange.TextDocumentEdit == nil {
			continue
		}

		edit := docChange.TextDocumentEdit
		uri := edit.TextDocument.URI
		filePath := uri.Path()

		// Read the original file content
		fh, err := snapshot.ReadFile(ctx, uri)
		if err != nil {
			continue // Skip files we can't read
		}

		contentBytes, err := fh.Content()
		if err != nil {
			continue
		}

		originalContent := string(contentBytes)
		lines := strings.Split(originalContent, "\n")

		// Get text edits
		textEdits := protocol.AsTextEdits(edit.Edits)

		// Convert each text edit to a line-based change
		for _, textEdit := range textEdits {
			startLine := int(textEdit.Range.Start.Line)
			endLine := int(textEdit.Range.End.Line)

			// Get the original line(s) affected
			if startLine >= len(lines) {
				continue
			}

			oldLine := ""
			if endLine < len(lines) {
				// Multi-line edit: collect all affected lines
				var oldLines []string
				for i := startLine; i <= endLine; i++ {
					oldLines = append(oldLines, lines[i])
				}
				oldLine = strings.Join(oldLines, "\n")
			} else if startLine < len(lines) {
				oldLine = lines[startLine]
			}

			// Apply the edit to get the new line content
			newLine := applyEditToLine(oldLine, textEdit)

			lineChanges = append(lineChanges, api.RenameChange{
				File:    filePath,
				Line:    startLine + 1, // 1-indexed for display
				OldLine: oldLine,
				NewLine: newLine,
			})
		}
	}

	return lineChanges, nil
}

// applyEditToLine applies a single text edit to a line (or multi-line) content.
func applyEditToLine(content string, edit protocol.TextEdit) string {
	startCol := int(edit.Range.Start.Character)
	endCol := int(edit.Range.End.Character)

	// Handle single-line edit
	if edit.Range.Start.Line == edit.Range.End.Line {
		if startCol <= len(content) && endCol <= len(content) {
			return content[:startCol] + edit.NewText + content[endCol:]
		}
	}

	// For multi-line edits or edge cases, just return the new text
	return edit.NewText
}

// sortEdits sorts text edits in descending order by position.
// This ensures that applying edits from end to start doesn't invalidate positions.
func sortEdits(edits []protocol.TextEdit) {
	// Sort by start position (descending), then by end position (descending)
	for i := 0; i < len(edits)-1; i++ {
		for j := i + 1; j < len(edits); j++ {
			iStart := edits[i].Range.Start
			jStart := edits[j].Range.Start

			// Compare positions
			if jStart.Line > iStart.Line ||
				(jStart.Line == iStart.Line && jStart.Character > iStart.Character) {
				edits[i], edits[j] = edits[j], edits[i]
			}
		}
	}
}

// applyEdits applies text edits to the original content.
func applyEdits(original string, edits []protocol.TextEdit) string {
	// Convert original content to lines for easier editing
	lines := strings.Split(original, "\n")

	// Apply edits from end to start (already sorted)
	for _, edit := range edits {
		startLine := int(edit.Range.Start.Line)
		startCol := int(edit.Range.Start.Character)
		endLine := int(edit.Range.End.Line)
		endCol := int(edit.Range.End.Character)

		if startLine >= len(lines) {
			continue
		}

		// Get the prefix and suffix for the edit range
		prefix := ""
		suffix := ""

		if startLine == endLine {
			// Single-line edit
			line := lines[startLine]
			if startCol <= len(line) && endCol <= len(line) {
				prefix = line[:startCol]
				suffix = line[endCol:]
				lines[startLine] = prefix + edit.NewText + suffix
			}
		} else {
			// Multi-line edit
			if startLine < len(lines) {
				prefix = lines[startLine][:min(startCol, len(lines[startLine]))]
			}
			if endLine < len(lines) {
				suffix = lines[endLine][min(endCol, len(lines[endLine])):]
			}

			// Replace the range with new text
			newLines := strings.Split(edit.NewText, "\n")
			result := make([]string, 0, len(lines)-(endLine-startLine)+len(newLines))
			result = append(result, lines[:startLine]...)
			result = append(result, newLines...)
			result = append(result, lines[endLine+1:]...)

			// Join first/last line with prefix/suffix
			if len(result) > startLine {
				result[startLine] = prefix + result[startLine]
			}
			lastIdx := startLine + len(newLines) - 1
			if lastIdx < len(result) && lastIdx >= 0 {
				result[lastIdx] = result[lastIdx] + suffix
			}

			lines = result
		}
	}

	return strings.Join(lines, "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// unifiedDiffForFile generates a unified diff for a single file.
func unifiedDiffForFile(filePath string, original, modified string) string {
	var diff strings.Builder

	// Split into lines
	origLines := strings.Split(original, "\n")
	modLines := strings.Split(modified, "\n")

	// Generate diff header
	diff.WriteString(fmt.Sprintf("--- a/%s\n", filePath))
	diff.WriteString(fmt.Sprintf("+++ b/%s\n", filePath))

	// Generate hunks using simple line-by-line comparison
	hunks := generateHunks(origLines, modLines)

	for _, hunk := range hunks {
		diff.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n",
			hunk.origStart, hunk.origCount,
			hunk.modStart, hunk.modCount))

		for _, line := range hunk.lines {
			diff.WriteString(line)
			diff.WriteString("\n")
		}
	}

	return diff.String()
}

// hunk represents a single hunk in a unified diff.
type hunk struct {
	origStart int // 1-indexed
	origCount int
	modStart  int // 1-indexed
	modCount  int
	lines     []string
}

// generateHunks generates hunks for the unified diff.
// This is a simplified implementation that finds contiguous changes.
func generateHunks(origLines, modLines []string) []hunk {
	var hunks []hunk

	i, j := 0, 0
	for i < len(origLines) || j < len(modLines) {
		// Find the next difference
		if i < len(origLines) && j < len(modLines) && origLines[i] == modLines[j] {
			i++
			j++
			continue
		}

		// Found a difference, collect the hunk
		// Context size (number of unchanged lines before/after)
		const contextSize = 3

		// Add context lines before the change
		contextStart := max(0, i-contextSize)
		if contextStart > 0 {
			// We have previous context, this is not the first hunk
		}

		var lines []string

		// Add context before
		for k := contextStart; k < i; k++ {
			lines = append(lines, " "+origLines[k])
		}

		// Collect changed lines
		origCount := i - contextStart
		modCount := j - contextStart

		// Find the end of the change
		origEnd := i
		modEnd := j

		// Count deletions
		for origEnd < len(origLines) && (modEnd >= len(modLines) || origLines[origEnd] != modLines[modEnd]) {
			lines = append(lines, "-"+origLines[origEnd])
			origEnd++
			origCount++
		}

		// Count additions
		for modEnd < len(modLines) && (origEnd >= len(origLines) || origLines[origEnd] != modLines[modEnd]) {
			lines = append(lines, "+"+modLines[modEnd])
			modEnd++
			modCount++
		}

		// Add context after
		contextEnd := min(len(origLines), origEnd+contextSize)
		for k := origEnd; k < contextEnd; k++ {
			lines = append(lines, " "+origLines[k])
			origCount++
			modCount++
		}

		hunks = append(hunks, hunk{
			origStart: contextStart + 1,
			origCount: origCount,
			modStart:  contextStart + 1,
			modCount:  modCount,
			lines:     lines,
		})

		i = origEnd
		j = modEnd
	}

	return hunks
}

// GoDefinition finds the definition location(s) for a symbol identified by a SymbolLocator.
//
// This is a convenience wrapper around ResolveSymbol that only returns definition locations.
// For more detailed information including documentation and snippets, use ResolveSymbol directly.
func GoDefinition(ctx context.Context, snapshot *cache.Snapshot, locator api.SymbolLocator) ([]protocol.Location, error) {
	// Use the unified ResolveSymbol function
	info, err := ResolveSymbol(ctx, snapshot, locator, ResolveOptions{
		FindDefinitions: true,
	})
	if err != nil {
		return nil, err
	}

	return info.Locations, nil
}

// ===== Symbol Information Extraction Utilities =====
// These functions extract rich symbol information from gopls internal structures.

// ExtractSymbolAtDefinition extracts symbol information (name, kind, signature, docs, body)
// at the given definition location using the internal hover() function.
//
// This is a core utility for rich symbol extraction that bridges gopls internal APIs
// with LLM-friendly output formats.
//
// By using the internal hover() function, we get clean documentation without
// pkg.go.dev markdown links, and properly formatted signatures from gopls.
func ExtractSymbolAtDefinition(ctx context.Context, snapshot *cache.Snapshot, loc protocol.Location, includeBody bool) *api.Symbol {
	// Read the file at the definition location
	fh, err := snapshot.ReadFile(ctx, loc.URI)
	if err != nil {
		return nil
	}

	// Use the start position from the location
	rng := protocol.Range{
		Start: loc.Range.Start,
		End:   loc.Range.Start,
	}

	// Call the internal hover() function to get hoverResult with clean documentation
	// This gives us access to the raw hoverResult before markdown conversion
	_, h, err := hover(ctx, snapshot, fh, rng)
	if err != nil || h == nil {
		// If hover fails, return basic symbol info from location
		return &api.Symbol{
			Name:     "<symbol>",
			FilePath: loc.URI.Path(),
			Line:     int(loc.Range.Start.Line + 1),
		}
	}

	// Extract clean documentation (no pkg.go.dev links!) and signature
	documentation := h.FullDocumentation
	signature := h.Signature

	// Extract name, receiver, and kind from SymbolName and signature
	// SymbolName format: "pkg.Name" for functions, "(pkg.Type).Method" for methods
	name := "<symbol>"
	kind := api.SymbolKindType
	receiver := ""

	if h.SymbolName != "" {
		// Parse SymbolName to extract name and receiver
		// Format: "pkg.FuncName" or "(pkg.Type).MethodName"
		if strings.HasPrefix(h.SymbolName, "(") {
			// Method: "(pkg.Type).MethodName"
			// Extract receiver and name
			parenEnd := strings.Index(h.SymbolName, ")")
			if parenEnd != -1 {
				// Receiver is between parentheses
				receiverWithPkg := h.SymbolName[1:parenEnd]
				// Remove package prefix from receiver
				lastDot := strings.LastIndex(receiverWithPkg, ".")
				if lastDot != -1 {
					receiver = receiverWithPkg[lastDot+1:]
				} else {
					receiver = receiverWithPkg
				}

				// Method name is after ")."
				dotAfterParen := strings.Index(h.SymbolName[parenEnd:], ".")
				if dotAfterParen != -1 {
					name = h.SymbolName[parenEnd+dotAfterParen+1:]
					kind = api.SymbolKindMethod
				}
			}
		} else {
			// Function or variable: "pkg.Name"
			// Extract just the name (after the last dot)
			lastDot := strings.LastIndex(h.SymbolName, ".")
			if lastDot != -1 {
				name = h.SymbolName[lastDot+1:]
			} else {
				name = h.SymbolName
			}
		}
	}

	// If we couldn't parse SymbolName, fall back to parsing the signature
	if name == "<symbol>" && signature != "" {
		name, receiver, kind = parseSignatureForName(signature)
	}

	// Extract package path from LinkPath
	packagePath := h.LinkPath
	// LinkPath may include module version, strip it if present
	// Format: "module@version/pkg/path" or just "pkg/path"
	if atIndex := strings.Index(packagePath, "@"); atIndex != -1 {
		slashAfterVersion := strings.Index(packagePath[atIndex:], "/")
		if slashAfterVersion != -1 {
			packagePath = packagePath[:atIndex] + packagePath[atIndex+slashAfterVersion:]
		}
	}

	sym := &api.Symbol{
		Name:        name,
		Kind:        kind,
		Signature:   signature,
		Receiver:    receiver,
		PackagePath: packagePath,
		FilePath:    loc.URI.Path(),
		Line:        int(loc.Range.Start.Line + 1),
		Doc:         documentation,
	}

	// Extract function body if requested
	if includeBody && kind == api.SymbolKindFunction {
		body := ExtractBodyForSymbol(ctx, snapshot, name, loc.URI.Path())
		sym.Body = body
	}

	return sym
}

// parseSignatureForName extracts name, receiver, and kind from a signature string.
// This is a fallback when SymbolName parsing fails.
func parseSignatureForName(signature string) (name, receiver string, kind api.SymbolKind) {
	// Try to extract name from signature
	// Common patterns: "func Name(...)", "func (recv) Name(...)", "type Name struct", "var Name ..."
	sigLines := strings.Split(signature, "\n")
	if len(sigLines) > 0 {
		firstLine := sigLines[0]
		parts := strings.Fields(firstLine)
		if len(parts) >= 2 {
			if parts[0] == "func" || parts[0] == "type" || parts[0] == "var" || parts[0] == "const" {
				rawName := parts[1]
				// Check if this is a method with receiver: "(Type)MethodName"
				if strings.HasPrefix(rawName, "(") {
					if idx := strings.Index(rawName, ")"); idx != -1 && idx+1 < len(rawName) {
						receiver = strings.TrimSpace(rawName[1:idx])
						name = rawName[idx+1:]
						// Remove parameter list from method names
						if idx := strings.Index(name, "("); idx != -1 {
							name = name[:idx]
						}
						kind = api.SymbolKindMethod
					}
				} else {
					name = rawName
					// Remove parameter list from function names
					if idx := strings.Index(name, "("); idx != -1 {
						name = name[:idx]
					}
				}
				// Set kind based on declaration
				switch parts[0] {
				case "func":
					kind = api.SymbolKindFunction
				case "type":
					kind = api.SymbolKindType
				case "var":
					kind = api.SymbolKindVariable
				case "const":
					kind = api.SymbolKindConstant
				}
			}
		}
	}
	return
}

// FormatSymbolSummary formats symbol information for display.
// This produces a markdown-formatted summary suitable for LLM consumption.
func FormatSymbolSummary(sym *api.Symbol) string {
	if sym == nil {
		return ""
	}

	var parts []string
	if sym.Name != "" {
		parts = append(parts, fmt.Sprintf("\n\n**Name**: `%s`", sym.Name))
	} else {
		// Ensure we always start with a blank line, even if Name is empty
		parts = append(parts, "\n")
	}
	if sym.Kind != "" {
		parts = append(parts, fmt.Sprintf("**Kind**: %s", sym.Kind))
	}
	if sym.Receiver != "" {
		parts = append(parts, fmt.Sprintf("**Receiver**: `%s`", sym.Receiver))
	}
	if sym.Parent != "" {
		parts = append(parts, fmt.Sprintf("**Parent**: `%s`", sym.Parent))
	}
	if sym.Signature != "" {
		parts = append(parts, fmt.Sprintf("\n**Signature**\n%s", sym.Signature))
	}
	if sym.Doc != "" {
		parts = append(parts, fmt.Sprintf("\n**Documentation**\n%s", sym.Doc))
	}
	if sym.Body != "" {
		parts = append(parts, fmt.Sprintf("\n**Body**\n%s", sym.Body))
	}

	return strings.Join(parts, "\n")
}
