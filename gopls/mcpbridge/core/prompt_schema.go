package core

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// PromptSchemaStartMarker and PromptSchemaEndMarker delimit the generated
// static tool schema block embedded in gopls-mcp.prompt.
const (
	PromptSchemaStartMarker = "<!-- Marker: TOOL-SCHEMA-START -->"
	PromptSchemaEndMarker   = "<!-- Marker: TOOL-SCHEMA-END -->"
)

// GeneratePromptToolSchemas returns the exact tool descriptions and input
// schemas currently registered with the MCP SDK, formatted for embedding in
// gopls-mcp.prompt.
func GeneratePromptToolSchemas(ctx context.Context) (string, error) {
	server := mcp.NewServer(&mcp.Implementation{Name: "gopls-mcp-test"}, nil)
	RegisterTools(server, NewHandler(nil))

	client := mcp.NewClient(&mcp.Implementation{Name: "schema-reader"}, nil)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		return "", err
	}
	defer serverSession.Close()

	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		return "", err
	}
	defer clientSession.Close()

	res, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		return "", err
	}

	type promptTool struct {
		name     string
		required map[string]bool
		props    []schemaProp
	}
	promptTools := make([]promptTool, 0, len(res.Tools))
	var locatorProps []schemaProp
	var locatorRequired map[string]bool
	for _, tool := range res.Tools {
		if tool.Name == ToolListTools {
			continue
		}
		schema, err := schemaMap(tool.InputSchema)
		if err != nil {
			return "", fmt.Errorf("read schema for %s: %w", tool.Name, err)
		}
		props := topLevelProps(schema)
		if rawProps, ok := schema["properties"].(map[string]any); ok && locatorProps == nil {
			loc, ok := rawProps["locator"].(map[string]any)
			if !ok {
				continue
			}
			locProps := topLevelProps(loc)
			locatorProps = locProps
			locatorRequired = requiredSet(loc)
		}
		promptTools = append(promptTools, promptTool{
			name:     tool.Name,
			required: requiredSet(schema),
			props:    props,
		})
	}
	sort.Slice(promptTools, func(i, j int) bool {
		return promptTools[i].name < promptTools[j].name
	})

	var b strings.Builder
	b.WriteString("### Tool input schemas (compact)\n\n")
	b.WriteString("The following MCP tools are already available. Call them directly by name; do not call ToolSearch just to load their schemas.\n\n")
	if len(locatorProps) > 0 {
		b.WriteString("Common `locator` object fields:\n")
		for _, prop := range locatorProps {
			fmt.Fprintf(&b, "- `%s` %s", prop.name, prop.typ)
			if locatorRequired[prop.name] {
				b.WriteString(" required")
			}
			if prop.description != "" {
				fmt.Fprintf(&b, ": %s", prop.description)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("Tool inputs:\n")
	for _, tool := range promptTools {
		fmt.Fprintf(&b, "- `%s`:", tool.name)
		for i, prop := range tool.props {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, " `%s` %s", prop.name, prop.typ)
			if tool.required[prop.name] {
				b.WriteString(" required")
			}
			if prop.name != "locator" && prop.description != "" {
				fmt.Fprintf(&b, " (%s)", prop.description)
			}
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

type schemaProp struct {
	name        string
	typ         string
	description string
}

func schemaMap(schema any) (map[string]any, error) {
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func topLevelProps(schema map[string]any) []schemaProp {
	rawProps, _ := schema["properties"].(map[string]any)
	props := make([]schemaProp, 0, len(rawProps))
	for name, raw := range rawProps {
		propSchema, _ := raw.(map[string]any)
		typ, _ := propSchema["type"].(string)
		desc, _ := propSchema["description"].(string)
		props = append(props, schemaProp{name: name, typ: typ, description: desc})
	}
	sort.Slice(props, func(i, j int) bool {
		return props[i].name < props[j].name
	})
	return props
}

func requiredSet(schema map[string]any) map[string]bool {
	required := make(map[string]bool)
	rawRequired, _ := schema["required"].([]any)
	for _, raw := range rawRequired {
		if name, ok := raw.(string); ok {
			required[name] = true
		}
	}
	return required
}
