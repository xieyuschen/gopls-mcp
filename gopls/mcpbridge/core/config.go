package core

import (
	"encoding/json"
	"fmt"

	"golang.org/x/tools/gopls/internal/settings"
)

// MCPConfig holds the configuration for the gopls-mcp MCP server.
// It can be unmarshaled from JSON provided during MCP initialization.
type MCPConfig struct {
	// Gopls contains native gopls configuration options.
	// These are passed directly to gopls's internal settings system.
	//
	// Example:
	// {
	//   "gopls": {
	//     "analyses": {"unusedparams": true},
	//     "buildFlags": ["-tags=integration"],
	//     "staticcheck": true
	//   }
	// }
	Gopls map[string]any `json:"gopls"`

	// Workdir is the working directory for the Go project
	// (can also be set via command-line flag)
	Workdir string `json:"workdir,omitempty"`

	// MaxResponseBytes is the global maximum response size in bytes.
	// ALL tools will respect this limit automatically to prevent oversized responses.
	// When a response exceeds this limit, it will be truncated and include
	// metadata indicating truncation.
	// Default: 32000 (32KB)
	// JSON field name: max_response_bytes
	MaxResponseBytes int `json:"max_response_bytes,omitempty"`

	// IdleTimeout is the duration of inactivity before gopls session resources
	// are released. Accepts Go duration strings: "5m", "30s", "500ms".
	// On next tool call the session is re-initialized automatically.
	// Default: "5m".
	IdleTimeout string `json:"idle_timeout,omitempty"`
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *MCPConfig {
	return &MCPConfig{
		Gopls:            make(map[string]any),
		MaxResponseBytes: 32000, // 32KB
		IdleTimeout:      "5m",
	}
}

// LoadConfig loads configuration from a JSON byte slice.
// If the data is empty or nil, returns a default configuration.
func LoadConfig(data []byte) (*MCPConfig, error) {
	if len(data) == 0 {
		return DefaultConfig(), nil
	}

	var config MCPConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Initialize maps if nil
	if config.Gopls == nil {
		config.Gopls = make(map[string]any)
	}
	if config.MaxResponseBytes == 0 {
		config.MaxResponseBytes = 32000 // 32KB
	}
	if config.IdleTimeout == "" {
		config.IdleTimeout = "5m"
	}

	return &config, nil
}

// LoadConfigFromMap loads configuration from a map[string]any.
// This is useful when the config comes from an untyped JSON source.
func LoadConfigFromMap(m map[string]any) (*MCPConfig, error) {
	if m == nil || len(m) == 0 {
		return DefaultConfig(), nil
	}

	// Convert to JSON and use LoadConfig for consistency
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	return LoadConfig(data)
}

// ApplyGoplsOptions applies the gopls configuration to a settings.Options struct.
// This uses gopls's native option parsing logic, so all standard gopls options
// are supported without any hardcoding.
func (c *MCPConfig) ApplyGoplsOptions(opts *settings.Options) error {
	if c == nil || len(c.Gopls) == 0 {
		return nil
	}

	// Use gopls's built-in Set method to apply options
	_, errs := opts.Set(c.Gopls)
	if len(errs) > 0 {
		// Return the first error, but log all of them
		return fmt.Errorf("failed to apply gopls options: %w (and %d more)", errs[0], len(errs)-1)
	}

	return nil
}

// GoplsOptions creates a new settings.Options struct with the gopls
// configuration from this MCPConfig already applied.
func (c *MCPConfig) GoplsOptions() (*settings.Options, error) {
	opts := &settings.Options{}
	if err := c.ApplyGoplsOptions(opts); err != nil {
		return nil, err
	}
	return opts, nil
}
