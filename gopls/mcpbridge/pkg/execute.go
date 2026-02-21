package pkg

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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

	// Override workdir from config if set
	if config.Workdir != "" {
		projectDir = config.Workdir
		log.Printf("[gopls-mcp] Using workdir from config: %s", projectDir)
	}

	// Create gopls cache
	ctx := context.Background()
	goplsCache := cache.New(nil)
	session := cache.NewSession(ctx, goplsCache)

	// Initialize gopls options (REQUIRED for view creation)
	// Start with defaults and apply user configuration
	options := settings.DefaultOptions()

	// Apply gopls configuration from MCP config
	// This allows users to set native gopls options like analyses, buildFlags, etc.
	if err := config.ApplyGoplsOptions(options); err != nil {
		log.Printf("[gopls-mcp] Warning: Failed to apply some gopls options: %v", err)
		// Continue anyway - partial configuration is better than failure
	} else {
		log.Printf("[gopls-mcp] Gopls options applied successfully")
	}

	// Fetch Go environment for the working directory (REQUIRED for view creation)
	// This loads GOROOT, GOPATH, GOVERSION, and other critical environment info
	dirURI := protocol.URIFromPath(projectDir)
	goEnv, err := cache.FetchGoEnv(ctx, dirURI, options)
	if err != nil {
		log.Fatalf("[gopls-mcp] Failed to load Go env: %v", err)
	}

	// Create a view for the working directory with proper initialization
	// This enables gopls to analyze the Go project
	folder := &cache.Folder{
		Dir:     dirURI,
		Options: options,
		Env:     *goEnv,
	}
	view, _, releaseView, err := session.NewView(ctx, folder)
	if err != nil {
		log.Fatalf("[gopls-mcp] Failed to create view for %s: %v", projectDir, err)
	}
	defer releaseView()

	log.Printf("[gopls-mcp] Created view for %s (type: %v)", projectDir, view.Type())
	releaseView() // Release the initial snapshot since we won't use it

	// Create a minimal LSP server stub that implements the methods we need
	// The gopls-mcp handlers use the Symbol method for search
	// The watcher uses DidChangeWatchedFiles to notify gopls of file changes
	lspServer := &minimalServer{session: session}

	// Build directory skip function from directoryFilters so the file
	// watcher excludes the same directories that gopls analysis ignores
	// (e.g. node_modules). See https://github.com/xieyuschen/gopls-mcp/issues/10.
	var watcherOpts []filewatcher.Option
	if filters := options.DirectoryFilters; len(filters) > 0 {
		watcherOpts = append(watcherOpts, makeDirectoryFilterSkipFunc(filters, projectDir))
	}

	// Start file change watcher
	// This keeps the gopls cache up-to-date when files are edited
	var fileWatcher *watcher.Watcher
	fileWatcher, err = watcher.New(lspServer, projectDir, watcherOpts...)
	if err != nil {
		log.Printf("[gopls-mcp] Failed to start file watcher: %v", err)
		// Continue anyway - tools will work but file changes won't be detected
	} else {
		defer fileWatcher.Close()
		log.Printf("[gopls-mcp] File watcher started for %s", projectDir)
	}

	// Create gopls-mcp handler backed by gopls session
	// Pass the config to enable response limits
	var handlerOpts []core.HandlerOption
	handlerOpts = append(handlerOpts, core.WithConfig(config))
	// Check environment variable for dynamic view creation (test-only)
	if os.Getenv(allowDynamicViewsEnv) == "true" || os.Getenv(allowDynamicViewsEnv) == "1" {
		log.Printf("[gopls-mcp] Dynamic views enabled via %s (TEST-ONLY)", allowDynamicViewsEnv)
		handlerOpts = append(handlerOpts, core.WithDynamicViews(true))
	}
	coreHandler := core.NewHandler(session, lspServer, handlerOpts...)

	// Create MCP server and register all gopls-mcp tools
	server := mcp.NewServer(&mcp.Implementation{Name: mcpName, Version: version}, nil)
	core.RegisterTools(server, coreHandler)

	log.Printf("[gopls-mcp] Registered %d MCP tools for Go analysis", 18)
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
