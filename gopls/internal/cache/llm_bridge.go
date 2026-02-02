package cache

import (
	"context"
	"go/parser"
	"go/token"

	"golang.org/x/tools/gopls/internal/cache/parsego"
	"golang.org/x/tools/gopls/internal/file"
)

// ParseGoImpl is an exported version of the internal parseGoImpl function.
//
// This function is provided for external tools (such as LLM/MCP bridges) that need
// to parse Go source files WITHOUT using the parseCache. This is useful for
// one-time operations like symbol search, where:
//   - Files are parsed once for immediate analysis
//   - Parse results are not cached
//   - Memory is reclaimed immediately after use
//
// The parseCache is optimized for repeated access to the same files during
// active editing. For batch operations on many files, bypassing the cache
// avoids unnecessary memory accumulation.
//
// Usage Example:
//
//	fset := token.NewFileSet()
//	pgf, err := cache.ParseGoImpl(ctx, fset, fh, parser.ParseComments, false)
//	if err != nil {
//	    return err
//	}
//	// Extract symbols from pgf...
//	// pgf is eligible for GC when it goes out of scope
//
// This wrapper exists to avoid modifying the internal parseGoImpl function.
// When cherry-picking changes from upstream gopls, this file should be
// reviewed but typically will not need changes.
func ParseGoImpl(ctx context.Context, fset *token.FileSet, fh file.Handle, mode parser.Mode, purgeFuncBodies bool) (*parsego.File, error) {
	return parseGoImpl(ctx, fset, fh, mode, purgeFuncBodies)
}
