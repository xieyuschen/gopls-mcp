// Package watcher provides file watching functionality for goplsmcp.
// It uses gopls's filewatcher package to detect file changes and notifies
// the gopls LSP server via DidChangeWatchedFiles notifications.
package watcher

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"golang.org/x/tools/gopls/internal/filewatcher"
	"golang.org/x/tools/gopls/internal/protocol"
)

// Watcher watches for file changes and notifies the gopls LSP server.
type Watcher struct {
	server   ChangeWatchedFiles // LSP server to notify of file changes
	fw       *filewatcher.Watcher
	dir      string
	stopCh   chan struct{}
	stopOnce sync.Once

	// Event processing
	eventQueue []protocol.FileEvent
	eventMu    sync.Mutex
	eventReady chan struct{}
}

type ChangeWatchedFiles interface {
	DidChangeWatchedFiles(context.Context, *protocol.DidChangeWatchedFilesParams) error
}

// New creates a new file watcher for the given directory.
// It watches for changes to Go files and notifies the gopls LSP server via DidChangeWatchedFiles.
//
// The watcher automatically:
// - Watches the directory recursively
// - Filters Go files (.go, .mod, .sum, .work, .s)
// - Skips irrelevant directories (.git, _*, testdata)
// - Batches events (500ms delay) to avoid excessive notifications
//
// The server parameter should implement protocol.ServerDidChangeWatchedFiles to handle
// file change notifications and invalidate the gopls cache appropriately.
func New(server ChangeWatchedFiles, dir string, opts ...filewatcher.Option) (*Watcher, error) {
	w := &Watcher{
		server:     server,
		dir:        dir,
		stopCh:     make(chan struct{}),
		eventReady: make(chan struct{}, 1),
	}

	// Create the file watcher
	var (
		queue    []protocol.FileEvent
		queueMu  sync.Mutex
		nonempty = make(chan struct{}, 1)
	)

	// Process file change events and notify the LSP server
	go func() {
		for {
			select {
			case <-nonempty:
				queueMu.Lock()
				events := queue
				queue = nil
				queueMu.Unlock()

				if len(events) > 0 {
					w.notifyServer(events)
				}
			case <-w.stopCh:
				return
			}
		}
	}()

	// Ensure goroutines get CPU time by yielding
	// This is important when running in stdio mode where the main goroutine is blocked
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runtime.Gosched()
			case <-w.stopCh:
				return
			}
		}
	}()

	errHandler := func(err error) {
		log.Printf("[gopls-mcp/watcher] Watch error: %v", err)
	}

	fw, err := filewatcher.New(500*time.Millisecond, nil, func(events []protocol.FileEvent) {
		if len(events) == 0 {
			return
		}

		log.Printf("[gopls-mcp/watcher] Detected %d file changes", len(events))
		for _, e := range events {
			log.Printf("[gopls-mcp/watcher]   - %s %s", e.Type, e.URI.Path())
		}

		// Queue the events for processing
		queueMu.Lock()
		queue = append(queue, events...)
		queueMu.Unlock()

		select {
		case nonempty <- struct{}{}:
		default:
		}
	}, errHandler, opts...)

	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	w.fw = fw

	// Start watching the directory recursively
	if err := fw.WatchDir(dir); err != nil {
		fw.Close()
		return nil, fmt.Errorf("failed to watch directory %s: %w", dir, err)
	}

	log.Printf("[gopls-mcp/watcher] Started watching %s", dir)
	return w, nil
}

// notifyServer sends file change notifications to the LSP server.
// This implements the file change notification flow that gopls expects.
func (w *Watcher) notifyServer(events []protocol.FileEvent) {
	log.Printf("[gopls-mcp/watcher] notifyServer called with %d events", len(events))
	ctx := context.Background()

	// Notify the LSP server about file changes
	// The server's DidChangeWatchedFiles method will handle cache invalidation
	log.Printf("[gopls-mcp/watcher] Calling DidChangeWatchedFiles...")
	err := w.server.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: events,
	})
	if err != nil {
		log.Printf("[gopls-mcp/watcher] Failed to notify server: %v", err)
		return
	}

	log.Printf("[gopls-mcp/watcher] Cache invalidated")
	log.Printf("[gopls-mcp/watcher] Successfully notified server of file changes")
}

// Close stops the file watcher and cleans up resources.
func (w *Watcher) Close() error {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	return w.fw.Close()
}

// Dir returns the directory being watched.
func (w *Watcher) Dir() string {
	return w.dir
}
