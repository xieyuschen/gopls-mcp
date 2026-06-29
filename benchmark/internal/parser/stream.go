// Package parser parses the claude --output-format stream-json JSONL event stream.
package parser

import (
	"bufio"
	"encoding/json"
	"io"
)

// ToolCall records one completed tool invocation extracted from the event stream.
type ToolCall struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input,omitempty"`
}

// TokenUsage holds the cumulative token counts from the result event.
type TokenUsage struct {
	InputTokens          int     `json:"input_tokens"`
	OutputTokens         int     `json:"output_tokens"`
	CacheReadTokens      int     `json:"cache_read_input_tokens"`
	CacheCreationTokens  int     `json:"cache_creation_input_tokens"`
	TotalCostUSD         float64 `json:"total_cost_usd"`
}

// Result is the parsed summary of a single claude headless run.
type Result struct {
	ToolCalls    []ToolCall `json:"tool_calls"`
	Usage        TokenUsage `json:"usage"`
	FinalMessage string     `json:"final_message"`
	IsError      bool       `json:"is_error"`
	// RawEvents is preserved for offline analysis.
	RawEvents []json.RawMessage `json:"-"`
}

// ParseStream reads a claude stream-json JSONL event stream from r and returns
// the extracted tool calls, token usage, and final assistant message.
func ParseStream(r io.Reader) (*Result, error) {
	res := &Result{}
	pending := map[string]ToolCall{} // tool_use id → pending call

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		res.RawEvents = append(res.RawEvents, json.RawMessage(append([]byte(nil), line...)))

		var ev map[string]json.RawMessage
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}

		var evType string
		if err := json.Unmarshal(ev["type"], &evType); err != nil {
			continue
		}

		switch evType {
		case "assistant":
			parseAssistantEvent(ev, pending, res)
		case "user":
			parseUserEvent(ev, pending, res)
		case "result":
			parseResultEvent(ev, res)
		}
	}
	return res, scanner.Err()
}

func parseAssistantEvent(ev map[string]json.RawMessage, pending map[string]ToolCall, res *Result) {
	var msg struct {
		Content []json.RawMessage `json:"content"`
		Usage   struct {
			InputTokens         int `json:"input_tokens"`
			OutputTokens        int `json:"output_tokens"`
			CacheReadTokens     int `json:"cache_read_input_tokens"`
			CacheCreationTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(ev["message"], &msg); err != nil {
		return
	}

	res.Usage.InputTokens += msg.Usage.InputTokens
	res.Usage.OutputTokens += msg.Usage.OutputTokens
	res.Usage.CacheReadTokens += msg.Usage.CacheReadTokens
	res.Usage.CacheCreationTokens += msg.Usage.CacheCreationTokens

	for _, rawBlock := range msg.Content {
		var block struct {
			Type  string         `json:"type"`
			ID    string         `json:"id"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
			Text  string         `json:"text"`
		}
		if err := json.Unmarshal(rawBlock, &block); err != nil {
			continue
		}
		switch block.Type {
		case "tool_use":
			pending[block.ID] = ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			}
		case "text":
			if block.Text != "" {
				res.FinalMessage = block.Text
			}
		}
	}
}

func parseUserEvent(ev map[string]json.RawMessage, pending map[string]ToolCall, res *Result) {
	var msg struct {
		Content []json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(ev["message"], &msg); err != nil {
		return
	}
	for _, rawBlock := range msg.Content {
		var block struct {
			Type      string `json:"type"`
			ToolUseID string `json:"tool_use_id"`
		}
		if err := json.Unmarshal(rawBlock, &block); err != nil {
			continue
		}
		if block.Type == "tool_result" {
			if call, ok := pending[block.ToolUseID]; ok {
				delete(pending, block.ToolUseID)
				res.ToolCalls = append(res.ToolCalls, call)
			}
		}
	}
}

func parseResultEvent(ev map[string]json.RawMessage, res *Result) {
	var result struct {
		IsError      bool    `json:"is_error"`
		TotalCostUSD float64 `json:"total_cost_usd"`
		Result       string  `json:"result"`
		Usage        struct {
			InputTokens         int `json:"input_tokens"`
			OutputTokens        int `json:"output_tokens"`
			CacheReadTokens     int `json:"cache_read_input_tokens"`
			CacheCreationTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(ev["result"], &result.Result); err == nil && result.Result != "" {
		res.FinalMessage = result.Result
	}
	// Some versions put cost/usage at top level of the result event.
	if raw, ok := ev["total_cost_usd"]; ok {
		_ = json.Unmarshal(raw, &res.Usage.TotalCostUSD)
	}
	if raw, ok := ev["is_error"]; ok {
		_ = json.Unmarshal(raw, &res.IsError)
	}
}
