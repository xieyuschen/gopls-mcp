package core

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GenericTool is a type-safe wrapper for MCP tools that use our Handler pattern.
// This bridges the MCP SDK to gopls's session/snapshot APIs via the Handler type.
type GenericTool[In, Out any] struct {
	Name        string
	Description string
	// Handler takes a Handler with access to gopls session/snapshot
	// Note: Out is typically a pointer type like *api.OGoInfo, so we return Out not *Out
	Handler func(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input In) (*mcp.CallToolResult, Out, error)
}

// Tool interface defines the contract for MCP tools.
type Tool interface {
	Docs() (string, error)
	Register(server *mcp.Server, handler *Handler)
	Details() (string, string)
}

const defaultMaxResponseBytes = 32 * 1024 // 32KB

// Register registers the tool with the MCP server using a Handler.
// The Handler provides access to gopls's session and snapshot.
// Automatically applies response size limits from handler config.
func (t GenericTool[In, Out]) Register(server *mcp.Server, handler *Handler) {
	// Get max bytes from config
	maxBytes := handler.config.MaxResponseBytes
	// set max bytes limit to prevent response consumes too many user tokens,
	// as they are input tokens user need to pay for.
	if maxBytes == 0 {
		maxBytes = defaultMaxResponseBytes
	}

	// Create a wrapper function that applies response limits
	wrapped := func(ctx context.Context, req *mcp.CallToolRequest, input In) (*mcp.CallToolResult, Out, error) {
		result, output, err := t.Handler(ctx, handler, req, input)
		if err != nil {
			return result, output, err
		}

		// Apply response limits to ALL tools
		if result != nil {
			result = applyResponseLimits(result, maxBytes, t.Name)
		}

		return result, output, nil
	}
	mcp.AddTool(server, &mcp.Tool{Name: t.Name, Description: t.Description}, wrapped)

	log.Printf("[gopls-mcp] Registered tool %s: %s (max_bytes=%d)", t.Name, t.Description, maxBytes)
}

// Details returns the tool name and description.
func (t GenericTool[In, Out]) Details() (name, description string) {
	return t.Name, t.Description
}

func (t GenericTool[In, Out]) Docs() (string, error) {
	doc, ok := docMap[t.Name]
	if !ok {
		return "", fmt.Errorf("documentation not found for tool: %s", t.Name)
	}
	return doc, nil
}

// getTools returns the list of registered tools.
// This is exported to allow handlers to access the tools list without init cycles.
func getTools() []Tool {
	return tools
}
