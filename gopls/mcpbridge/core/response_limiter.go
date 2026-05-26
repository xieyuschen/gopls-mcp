package core

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// applyResponseLimits checks if a response exceeds max bytes and truncates if needed.
// This is a unified limiter that works for ALL tool responses.
// Uses the global maxBytes config only (no per-request override).
func applyResponseLimits(result *mcp.CallToolResult, maxBytes int, toolName string) *mcp.CallToolResult {
	if result == nil || len(result.Content) == 0 {
		return result
	}

	// Check current size
	currentSize := estimateResultSize(result)
	if currentSize <= maxBytes {
		return result // Within limit
	}

	// Need to truncate - find and truncate the text content
	for i, content := range result.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			result.Content[i] = truncateTextContent(textContent, maxBytes, currentSize, toolName)
			break
		}
	}

	return result
}

// estimateResultSize estimates the size of a result in bytes.
func estimateResultSize(result *mcp.CallToolResult) int {
	size := 0
	for _, content := range result.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			size += len(textContent.Text)
		}
	}
	return size
}

// truncateTextContent truncates text content to fit within maxBytes.
// It tries to parse as JSON and truncate arrays intelligently.
func truncateTextContent(textContent *mcp.TextContent, maxBytes, currentSize int, toolName string) *mcp.TextContent {
	text := textContent.Text
	targetSize := (maxBytes * 80) / 100 // Target 80% to leave room for metadata

	// Try to parse as JSON
	var data map[string]any
	if err := json.Unmarshal([]byte(text), &data); err == nil {
		// Successfully parsed - truncate intelligently
		truncated := truncateMap(data, targetSize)

		// Add truncation metadata if needed
		if !reflect.DeepEqual(truncated, data) {
			addTruncationMetadata(truncated, currentSize, estimateMapSize(truncated), toolName)
		}

		// Marshal back to JSON
		if jsonBytes, err := json.Marshal(truncated); err == nil {
			return &mcp.TextContent{Text: string(jsonBytes)}
		}
	}

	// Fallback: hard truncate
	if len(text) > maxBytes {
		truncated := text[:maxBytes]
		// Try to add truncation notice
		if strings.HasSuffix(truncated, "}") || strings.HasSuffix(truncated, "]") {
			truncated = truncated[:len(truncated)-1] + ", \"_truncated\": true}"
		}
		return &mcp.TextContent{Text: truncated}
	}

	return textContent
}

// truncateMap recursively truncates a map to fit within maxChars.
func truncateMap(data map[string]any, maxChars int) map[string]any {
	result := make(map[string]any)
	currentSize := 0

	for key, value := range data {
		// Skip metadata fields
		if strings.HasPrefix(key, "_") {
			result[key] = value
			continue
		}

		// Check if adding this field would exceed limit
		valueSize := estimateValueSize(value)
		if currentSize+valueSize > maxChars {
			// Try to truncate the value
			result[key] = truncateValue(value, maxChars-currentSize)
			break
		}

		result[key] = value
		currentSize += valueSize
	}

	return result
}

// truncateValue truncates a value to fit within remaining bytes.
func truncateValue(value any, remainingBytes int) any {
	switch v := value.(type) {
	case []any:
		return truncateArray(v, remainingBytes)
	case map[string]any:
		return truncateMap(v, remainingBytes)
	case string:
		if len(v) > remainingBytes {
			if len(v) > remainingBytes {
				return v[:remainingBytes] + "..."
			}
		}
		return v
	default:
		return v
	}
}

// truncateArray truncates an array to fit within maxBytes.
func truncateArray(arr []any, maxBytes int) []any {
	if len(arr) == 0 {
		return arr
	}

	// Binary search for the max number of elements
	low, high := 1, len(arr)
	bestLen := 1

	for low <= high {
		mid := (low + high) / 2
		candidate := arr[:mid]
		size := estimateArraySize(candidate)

		if size <= maxBytes {
			bestLen = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}

	if bestLen < len(arr) {
		return arr[:bestLen]
	}

	return arr
}

// estimateMapSize estimates the JSON size of a map.
func estimateMapSize(data map[string]any) int {
	size, _ := jsonSize(data)
	return size
}

// estimateValueSize estimates the JSON size of a value.
func estimateValueSize(value any) int {
	size, _ := jsonSize(value)
	return size
}

// estimateArraySize estimates the JSON size of an array.
func estimateArraySize(arr []any) int {
	size, _ := jsonSize(arr)
	return size
}

// jsonSize returns the byte count of JSON marshaled data.
func jsonSize(data any) (int, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return 0, err
	}
	return len(jsonBytes), nil
}

// addTruncationMetadata adds truncation metadata to a response.
func addTruncationMetadata(data map[string]any, originalSize, truncatedSize int, toolName string) {
	data["_truncated"] = true
	data["_original_bytes"] = originalSize
	data["_truncated_bytes"] = truncatedSize
	data["_tool"] = toolName

	if originalSize > 0 {
		percentKept := (truncatedSize * 100) / originalSize
		data["_percent_kept"] = percentKept

		if percentKept < 50 {
			data["_hint"] = "Response was heavily truncated. Use more specific queries to reduce results."
		} else if percentKept < 80 {
			data["_hint"] = fmt.Sprintf("Response partially truncated (%d%% of original).", percentKept)
		}
	}
}

// WrapWithResponseLimits is a decorator that wraps tool handlers with automatic response limiting.
// Usage:
//
//	handler := WrapWithResponseLimits(handleGetDependencyGraph, h.config.MaxResponseBytes, "go_get_dependency_graph")
func WrapWithResponseLimits[In, Out any](
	handler func(context.Context, *Handler, *mcp.CallToolRequest, In) (*mcp.CallToolResult, *Out, error),
	maxBytes int,
	toolName string,
) func(context.Context, *Handler, *mcp.CallToolRequest, In) (*mcp.CallToolResult, *Out, error) {
	return func(ctx context.Context, h *Handler, req *mcp.CallToolRequest, input In) (*mcp.CallToolResult, *Out, error) {
		// Call the original handler
		result, output, err := handler(ctx, h, req, input)
		if err != nil {
			return result, output, err
		}

		// Apply response limits
		if result != nil {
			result = applyResponseLimits(result, maxBytes, toolName)
		}

		return result, output, nil
	}
}
