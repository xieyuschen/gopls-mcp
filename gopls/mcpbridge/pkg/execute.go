package pkg

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/gopls/internal/cache"
	"golang.org/x/tools/gopls/internal/filewatcher"
	"golang.org/x/tools/gopls/internal/protocol"
	"golang.org/x/tools/gopls/internal/settings"
	"golang.org/x/tools/gopls/mcpbridge/core"
	"golang.org/x/tools/gopls/mcpbridge/watcher"
)

const (
	// mcpName is the name of the MCP server.
	mcpName = "gopls-mcp"
)

func init() {
	flag.Usage = func() {
		fmt.Fprint(os.Stderr,
			`gopls-mcp - semantic code analysis designed for code agent.

See docs: https://gopls-mcp.org

Options:
`)
		flag.PrintDefaults()
	}
}

var (
	// version is the version of the binary (set by main.go).
	version = "dev"
	// showVersion requests printing the version and exiting.
	showVersion = flag.Bool("version", false, "Print version information and exit")
	// showHelp requests printing help information and exiting.
	showHelp = flag.Bool("help", false, "Print help information and exit")
	// addr is the address to listen on (enables HTTP mode).
	addr = flag.String("addr", "", "Address to listen on (e.g., localhost:8080)")
	// verbose enables verbose logging.
	verbose = flag.Bool("verbose", false, "Enable verbose logging")
	// workdirFlag is the Go project directory to analyze (flag).
	workdirFlag = flag.String("workdir", "", "Path to the Go project directory (default is current directory)")
	// configFlag is the path to the MCP configuration file (optional).
	configFlag = flag.String("config", "", "Path to gopls-mcp configuration file (JSON format)")
	// logfile is the path to a log file for debugging (optional).
	// When set, logs are written to this file even in stdio mode.
	logfile = flag.String("logfile", "", "Path to log file for debugging (writes logs even in stdio mode)")
	// directoryFiltersFlag allows setting gopls directoryFilters via CLI flag.
	// Filters use the same syntax as gopls directoryFilters (e.g. "-**/node_modules,-vendor").
	directoryFiltersFlag = flag.String("directory-filters", "", "Comma-separated directory filters (e.g. \"-**/node_modules,-vendor\")")
	// idleTimeoutFlag sets the duration of inactivity before gopls resources are released.
	// Accepts Go duration strings: "5m", "30s", "500ms". Overrides the config file value.
	idleTimeoutFlag = flag.String("idle-timeout", "", "Inactivity duration before releasing gopls resources (e.g. \"5m\", \"30s\", default: 5m)")
)

const (
	// allowDynamicViewsEnv is the environment variable name for enabling dynamic view creation.
	// TEST-ONLY: This allows the handler to create new gopls views on-demand when a Cwd parameter
	// doesn't match any existing view.
	// WARNING: This is intended for e2e testing only. Normal users should not need this,
	// as they typically work with a single project.
	// Production usage: Run one gopls-mcp instance per project directory.
	allowDynamicViewsEnv = "GOPMCS_ALLOW_DYNAMIC_VIEWS"
)

func Execute() {
	helpAndUsage()

	// Pre-flight check: verify Go toolchain is available before configuring logging.
	// This must run BEFORE log.SetOutput(io.Discard) in stdio mode, because
	// log.Fatalf would write to void and the user would see no error at all.
	if err := checkGoEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "[gopls-mcp] %v\n", err)
		os.Exit(1)
	}

	// Configure logging based on transport mode
	// CRITICAL: In stdio mode, NEVER log to stdout/stderr as it corrupts MCP protocol
	if *addr != "" {
		// HTTP mode: logging to stdout is OK
		if *verbose {
			log.SetFlags(log.LstdFlags | log.Lshortfile)
			log.SetOutput(os.Stdout)
		}
	} else if *logfile != "" {
		// Stdio mode with logfile: write logs to file for debugging
		logFile, err := os.OpenFile(*logfile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Printf("[gopls-mcp] Failed to open log file %s: %v (logs discarded)", *logfile, err)
			log.SetOutput(io.Discard)
		} else {
			log.SetFlags(log.LstdFlags | log.Lshortfile)
			log.SetOutput(logFile)
			log.Printf("[gopls-mcp] Logging to file: %s", *logfile)
		}
	} else {
		// Stdio mode: ALWAYS discard logs, verbose flag ignored for safety
		log.SetOutput(io.Discard)
	}

	// Set the project directory to analyze
	projectDir := *workdirFlag
	if projectDir == "" {
		if dir, err := os.Getwd(); err == nil {
			projectDir = dir
		} else {
			log.Fatalf("[gopls-mcp] Failed to get working directory: %v", err)
		}
	}

	// Load configuration from file if provided
	var config *core.MCPConfig
	if *configFlag != "" {
		log.Printf("[gopls-mcp] Loading configuration from %s", *configFlag)
		data, err := os.ReadFile(*configFlag)
		if err != nil {
			log.Fatalf("[gopls-mcp] Failed to read config file: %v", err)
		}
		config, err = core.LoadConfig(data)
		if err != nil {
			log.Fatalf("[gopls-mcp] Failed to parse config: %v", err)
		}
		log.Printf("[gopls-mcp] Configuration loaded successfully")
	} else {
		// Use default configuration
		config = core.DefaultConfig()
	}

	// Merge gopls settings from .vscode/settings.json (lower priority than config file).
	// Only settings not already set by the config file are applied.
	if vsGopls := loadVSCodeGoplsSettings(projectDir); len(vsGopls) > 0 {
		log.Printf("[gopls-mcp] Merging gopls settings from .vscode/settings.json")
		if config.Gopls == nil {
			config.Gopls = make(map[string]any)
		}
		for k, v := range vsGopls {
			if _, alreadySet := config.Gopls[k]; !alreadySet {
				config.Gopls[k] = v
			}
		}
	}

	// Merge CLI directory filters into config (overrides config file value)
	if *directoryFiltersFlag != "" {
		parts := strings.Split(*directoryFiltersFlag, ",")
		// Convert to []any so opts.Set (which expects JSON-style types) accepts it.
		// Skip empty strings from extra commas to avoid validation failure in gopls
		// which would silently discard all filters.
		var filters []any
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				filters = append(filters, s)
			}
		}
		if config.Gopls == nil {
			config.Gopls = make(map[string]any)
		}
		config.Gopls["directoryFilters"] = filters
		log.Printf("[gopls-mcp] Directory filters from CLI: %v", filters)
	}

	// Merge CLI idle-timeout into config (overrides config file value)
	if *idleTimeoutFlag != "" {
		config.IdleTimeout = *idleTimeoutFlag
	}

	// Override workdir from config if set
	if config.Workdir != "" {
		projectDir = config.Workdir
		log.Printf("[gopls-mcp] Using workdir from config: %s", projectDir)
	}

	// Build gopls options from config (needed for watcher filters and view creation).
	options := settings.DefaultOptions()
	if err := config.ApplyGoplsOptions(options); err != nil {
		log.Printf("[gopls-mcp] Warning: Failed to apply some gopls options: %v", err)
	}

	// Build directory skip function from directoryFilters so the file
	// watcher excludes the same directories that gopls analysis ignores
	// (e.g. node_modules). See https://github.com/xieyuschen/gopls-mcp/issues/10.
	var watcherOpts []filewatcher.Option
	if filters := options.DirectoryFilters; len(filters) > 0 {
		watcherOpts = append(watcherOpts, makeDirectoryFilterSkipFunc(filters, projectDir))
	}

	// initFn creates the gopls session, file watcher, and LSP server on first
	// tool call (lazy init). Resources are released by shutdownResources after
	// the idle timeout and recreated here on the next call.
	initFn := func(ctx context.Context) (*cache.Session, core.Symbler, io.Closer, error) {
		goplsCache := cache.New(nil)
		session := cache.NewSession(ctx, goplsCache)

		dirURI := protocol.URIFromPath(projectDir)
		goEnv, err := cache.FetchGoEnv(ctx, dirURI, options)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to load Go env: %w", err)
		}

		folder := &cache.Folder{
			Dir:     dirURI,
			Options: options,
			Env:     *goEnv,
		}
		_, _, releaseView, err := session.NewView(ctx, folder)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to create view for %s: %w", projectDir, err)
		}
		releaseView()

		lspServer := &minimalServer{session: session}

		fw, err := watcher.New(lspServer, projectDir, watcherOpts...)
		if err != nil {
			log.Printf("[gopls-mcp] Failed to start file watcher: %v (file changes won't be detected)", err)
			return session, lspServer, nil, nil
		}
		log.Printf("[gopls-mcp] Initialized gopls for %s", projectDir)
		return session, lspServer, fw, nil
	}

	// Create gopls-mcp handler with lazy init and idle timeout.
	var handlerOpts []core.HandlerOption
	handlerOpts = append(handlerOpts, core.WithConfig(config))
	handlerOpts = append(handlerOpts, core.WithOptions(options))
	if os.Getenv(allowDynamicViewsEnv) == "true" || os.Getenv(allowDynamicViewsEnv) == "1" {
		log.Printf("[gopls-mcp] Dynamic views enabled via %s (TEST-ONLY)", allowDynamicViewsEnv)
		handlerOpts = append(handlerOpts, core.WithDynamicViews(true))
	}
	coreHandler := core.NewHandler(initFn, handlerOpts...)

	// Create MCP server and register all gopls-mcp tools
	server := mcp.NewServer(&mcp.Implementation{Name: mcpName, Version: version}, nil)
	log.Printf("[gopls-mcp] Registered %d MCP tools for Go analysis", core.RegisterTools(server, coreHandler))
	log.Printf("[gopls-mcp] Working directory: %s", projectDir)

	if *addr != "" {
		// HTTP mode
		handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
			return server
		}, &mcp.StreamableHTTPOptions{JSONResponse: true})

		http.Handle("/", handler)
		log.Printf("[gopls-mcp] Starting %s HTTP server at %s", mcpName, *addr)
		if err := http.ListenAndServe(*addr, nil); err != nil {
			log.Fatalf("[gopls-mcp] HTTP server failed: %v", err)
		}
		return
	}

	// Stdio mode (default)
	log.Printf("[gopls-mcp] Starting %s in stdio mode", mcpName)

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Run server in a goroutine so we can handle signals
	ctx := context.Background()
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- server.Run(ctx, &mcp.StdioTransport{})
	}()

	// Wait for either server error or signal
	select {
	case err := <-serverErrCh:
		// Server ended (likely connection closed)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[gopls-mcp] Server ended: %v\n", err)
		}
	case sig := <-sigCh:
		// Received signal, exit gracefully
		fmt.Fprintf(os.Stderr, "[gopls-mcp] Received signal: %v\n", sig)
	}
	// Always exit cleanly - stdio mode ends when client closes connection
}

func makeDirectoryFilterSkipFunc(filters []string, root string) filewatcher.Option {
	pathIncluded := cache.PathIncludeFunc(filters)
	cleanRoot := filepath.Clean(root)
	return filewatcher.WithSkipDir(func(dirPath string) bool {
		rel, err := filepath.Rel(cleanRoot, dirPath)
		if err != nil {
			return false
		}
		rel = filepath.ToSlash(rel)
		return !pathIncluded(rel)
	})
}

func helpAndUsage() {
	flag.Parse()

	// Handle help subcommand (e.g., "gopls-mcp help" or "gopls-mcp help <topic>")
	args := flag.Args()
	if len(args) > 0 && args[0] == "help" {
		flag.Usage()
		os.Exit(0)
	}

	// Handle --version flag
	if *showVersion {
		fmt.Printf("gopls-mcp version %s\n", version)
		os.Exit(0)
	}

	// Handle --help flag
	if *showHelp {
		flag.Usage()
		os.Exit(0)
	}
}

// loadVSCodeGoplsSettings reads the "gopls" section from .vscode/settings.json
// in the given directory, if it exists. Returns nil when the file is absent or
// contains no gopls settings.
func loadVSCodeGoplsSettings(dir string) map[string]any {
	settingsPath := filepath.Join(dir, ".vscode", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil
	}
	var all map[string]any
	if err := json.Unmarshal(data, &all); err != nil {
		log.Printf("[gopls-mcp] Warning: failed to parse %s: %v", settingsPath, err)
		return nil
	}
	gopls, ok := all["gopls"]
	if !ok {
		return nil
	}
	m, ok := gopls.(map[string]any)
	if !ok {
		return nil
	}
	return m
}

// checkGoEnv verifies that the Go toolchain is available and functional.
// It must be called before log output is redirected (e.g. to io.Discard in stdio mode),
// so that errors are always visible via stderr.
func checkGoEnv() error {
	// 1. Check that the "go" binary is reachable via PATH.
	goPath, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("go command not found in PATH: %v\n"+
			"Please install Go (https://go.dev/dl/) or ensure it is in your PATH.", err)
	}

	// 2. Verify that "go" can actually execute (e.g. not a broken symlink).
	cmd := exec.Command(goPath, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go version failed: %v\n"+
			"Output: %s\n"+
			"Please verify your Go installation at %s.", err, strings.TrimSpace(string(output)), goPath)
	}

	// 3. Check GOROOT — if go env GOROOT fails, the toolchain is misconfigured.
	cmd = exec.Command(goPath, "env", "GOROOT")
	output, err = cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) == "" {
		return fmt.Errorf("go env GOROOT failed: %v\n"+
			"Output: %s\n"+
			"Please check your Go installation (GOROOT=%s).", err, strings.TrimSpace(string(output)), os.Getenv("GOROOT"))
	}

	return nil
}
