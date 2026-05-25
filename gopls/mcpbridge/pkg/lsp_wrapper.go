package pkg

import (
	"context"
	"fmt"

	"golang.org/x/tools/gopls/internal/cache"
	"golang.org/x/tools/gopls/internal/file"
	"golang.org/x/tools/gopls/internal/golang"
	"golang.org/x/tools/gopls/internal/protocol"
	"golang.org/x/tools/gopls/internal/settings"
)

// minimalServer is a wrapper around cache.Session that implements for DidChangeWatchedFiles.
type minimalServer struct {
	session *cache.Session
}

// Symbol implements workspace symbol search using gopls's internal golang package.
// This is the only method from protocol.Server that gopls-mcp handlers currently use.
func (s *minimalServer) Symbol(ctx context.Context, params *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	// Collect ALL snapshots from ALL views
	// This is critical for multi-module workspaces where each module has its own view
	views := s.session.Views()
	if len(views) == 0 {
		return nil, fmt.Errorf("no active views")
	}

	snapshots := make([]*cache.Snapshot, 0, len(views))
	releases := make([]func(), 0, len(views))

	for _, view := range views {
		snapshot, release, err := view.Snapshot()
		if err != nil {
			// Log but continue - skip views that can't produce snapshots
			continue
		}
		snapshots = append(snapshots, snapshot)
		releases = append(releases, release)
	}

	// Ensure all snapshots are released
	defer func() {
		for _, release := range releases {
			release()
		}
	}()

	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no valid snapshots available")
	}

	// Use gopls's internal symbol search
	// Based on: gopls/internal/golang/workspace_symbol.go WorkspaceSymbols()
	symbols, err := golang.WorkspaceSymbols(
		ctx,
		snapshots,
		params.Query,
		golang.WorkspaceSymbolsOptions{
			Matcher: settings.SymbolFuzzy,
			Style:   settings.DynamicSymbols,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search symbols: %w", err)
	}

	return symbols, nil
}

// DidChangeWatchedFiles notifies gopls that files have changed on disk.
// This is called by the file watcher when it detects filesystem changes.
// It implements the LSP protocol to ensure proper cache invalidation.
func (s *minimalServer) DidChangeWatchedFiles(ctx context.Context, params *protocol.DidChangeWatchedFilesParams) error {
	if len(params.Changes) == 0 {
		return nil
	}

	// Convert FileEvents to file.Modifications
	// This is the core internal API that gopls uses to process file changes
	modifications := s.fileEventsToModifications(params.Changes)

	// Call gopls's internal API directly
	// This handles all the cache invalidation, view updates, etc.
	_, err := s.session.DidModifyFiles(ctx, modifications)
	if err != nil {
		return fmt.Errorf("failed to process file changes: %w", err)
	}

	return nil
}

// fileEventsToModifications converts LSP FileEvents to file.Modifications
// This is the conversion that gopls uses internally when processing DidChangeWatchedFiles
func (s *minimalServer) fileEventsToModifications(events []protocol.FileEvent) []file.Modification {
	modifications := make([]file.Modification, 0, len(events))
	for _, event := range events {
		modifications = append(modifications, file.Modification{
			URI:    event.URI,
			Action: changeTypeToFileAction(event.Type),
			OnDisk: true, // Important: marks this as an on-disk change (not editor change)
		})
	}
	return modifications
}

// changeTypeToFileAction converts LSP FileChangeType to file.Action
// Based on gopls/internal/server/text_synchronization.go
func changeTypeToFileAction(ct protocol.FileChangeType) file.Action {
	switch ct {
	case protocol.Created:
		return file.Create
	case protocol.Changed:
		return file.Change
	case protocol.Deleted:
		return file.Delete
	default:
		return file.Change
	}
}
