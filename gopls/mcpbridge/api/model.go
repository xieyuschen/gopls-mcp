// Package api provides input/output types for the gopls-mcp MCP server.
package api

// SymbolKind represents the semantic category of a symbol.
// We stick to LSP-compatible kinds but simplify for LLM comprehension.
type SymbolKind string

const (
	SymbolKindField     SymbolKind = "field"     // Struct fields
	SymbolKindMethod    SymbolKind = "method"    // Methods attached to a receiver
	SymbolKindFunction  SymbolKind = "function"  // Standalone functions
	SymbolKindStruct    SymbolKind = "struct"    // Struct definitions
	SymbolKindInterface SymbolKind = "interface" // Interface definitions
	SymbolKindVariable  SymbolKind = "var"       // Variables
	SymbolKindConstant  SymbolKind = "const"     // Constants
	SymbolKindType      SymbolKind = "type"      // Other types (alias, basic types)
)

// Symbol represents a semantic unit of code.
// Designed to provide "Entropy-Free" context to the LLM in a flat list.
type Symbol struct {
	// Identity -----------------------------------------------------------

	// Name is the short identifier of the symbol.
	// Example: "Start", "Timeout", "User"
	Name string `json:"name" jsonschema:"the short name of the symbol"`

	// Kind defines the semantic category.
	Kind SymbolKind `json:"kind" jsonschema:"the semantic type (function, method, struct, etc.)"`

	// Context (The Anti-Entropy Fields) ----------------------------------

	// Receiver is CRITICAL for methods. It specifies the type that owns this method.
	// Format: "*Server", "User"
	// Example: If Name is "Start" and Receiver is "*Server", LLM knows it's Server.Start().
	Receiver string `json:"receiver,omitempty" jsonschema:"the receiver type for methods (e.g., *Server)"`

	// Parent specifies the enclosing structure for fields or nested types.
	// Example: If Name is "Timeout" and Parent is "Config", LLM knows it's Config.Timeout.
	Parent string `json:"parent,omitempty" jsonschema:"the parent struct/interface for fields"`

	// Contract (The Reasoning Fields) ------------------------------------

	// Signature is the exact code contract.
	// For functions: "func(ctx context.Context) error"
	// For variables: "int"
	// For structs: "struct { ... }" (usually truncated)
	Signature string `json:"signature" jsonschema:"the type signature or function definition"`

	// Implementation (The Physical Fields) -------------------------------

	// PackagePath is the import path of the package containing this symbol.
	// Example: "net/http", "github.com/user/project/pkg"
	// This is crucial for distinguishing symbols with the same name from different packages.
	PackagePath string `json:"package_path,omitempty" jsonschema:"import path of the package containing the symbol"`

	// FilePath is the relative path to the source file.
	FilePath string `json:"file_path" jsonschema:"source file path"`

	// Line is the starting line number.
	// We use int instead of full Pos struct to save tokens while keeping navigability.
	Line int `json:"line" jsonschema:"line number where symbol is defined"`

	// Content (Optional / Heavy) -----------------------------------------

	// Doc is the comment block associated with the symbol.
	Doc string `json:"doc,omitempty" jsonschema:"documentation comments"`

	// Body is the full implementation code.
	Body string `json:"body,omitempty" jsonschema:"full implementation code"`
}
