package core

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/tools/gopls/internal/cache"
)

// mockCloser tracks Close() calls for testing.
type mockCloser struct {
	closed atomic.Bool
}

func (m *mockCloser) Close() error {
	m.closed.Store(true)
	return nil
}

// newTestHandler creates a Handler with a mock initFn that returns a real (empty)
// cache.Session and a mock file watcher. The idle timeout is set via idleTimeout
// directly so tests can use sub-second values.
func newTestHandler(t *testing.T, idleTimeout time.Duration) (*Handler, *mockCloser, *atomic.Int32) {
	t.Helper()

	watcher := &mockCloser{}
	var initCount atomic.Int32

	initFn := func(ctx context.Context) (*cache.Session, Symbler, io.Closer, error) {
		initCount.Add(1)
		// A real empty session: no views, no disk access, cheap to create.
		s := cache.NewSession(ctx, cache.New(nil))
		return s, nil, watcher, nil
	}

	h := NewHandler(initFn, WithConfig(DefaultConfig()))
	h.idleTimeout = idleTimeout
	return h, watcher, &initCount
}

func TestHandler_LazyInit(t *testing.T) {
	h, _, initCount := newTestHandler(t, time.Minute)

	// Session must be nil before first tool call.
	h.initMu.Lock()
	if h.session != nil {
		t.Error("session should be nil before first ensureSession call")
	}
	h.initMu.Unlock()

	// First call initializes.
	if err := h.ensureSession(context.Background()); err != nil {
		t.Fatalf("ensureSession failed: %v", err)
	}

	h.initMu.Lock()
	if h.session == nil {
		t.Error("session should be non-nil after ensureSession")
	}
	h.initMu.Unlock()

	if got := initCount.Load(); got != 1 {
		t.Errorf("initFn called %d times, want 1", got)
	}

	// Subsequent calls must not reinitialize.
	for i := 0; i < 5; i++ {
		if err := h.ensureSession(context.Background()); err != nil {
			t.Fatalf("ensureSession[%d] failed: %v", i, err)
		}
	}
	if got := initCount.Load(); got != 1 {
		t.Errorf("initFn called %d times after repeated ensureSession, want 1", got)
	}
}

func TestHandler_ConcurrentEnsureSession(t *testing.T) {
	h, _, initCount := newTestHandler(t, time.Minute)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if err := h.ensureSession(context.Background()); err != nil {
				t.Errorf("concurrent ensureSession failed: %v", err)
			}
		}()
	}
	wg.Wait()

	// initFn must be called exactly once despite concurrent callers.
	if got := initCount.Load(); got != 1 {
		t.Errorf("initFn called %d times with %d concurrent callers, want 1", got, goroutines)
	}
}

func TestHandler_IdleTimeout_ReleasesResources(t *testing.T) {
	const timeout = 60 * time.Millisecond
	h, watcher, initCount := newTestHandler(t, timeout)

	if err := h.ensureSession(context.Background()); err != nil {
		t.Fatalf("ensureSession failed: %v", err)
	}
	if initCount.Load() != 1 {
		t.Fatalf("initFn not called on first ensureSession")
	}

	// Wait for idle timer to fire (2× timeout for safety).
	time.Sleep(timeout * 2)

	h.initMu.Lock()
	sessionNil := h.session == nil
	h.initMu.Unlock()

	if !sessionNil {
		t.Error("session should be nil after idle timeout")
	}
	if !watcher.closed.Load() {
		t.Error("watcher.Close() should have been called after idle timeout")
	}
}

func TestHandler_IdleTimeout_ReinitOnNextCall(t *testing.T) {
	const timeout = 60 * time.Millisecond
	h, _, initCount := newTestHandler(t, timeout)

	// First init.
	if err := h.ensureSession(context.Background()); err != nil {
		t.Fatalf("first ensureSession failed: %v", err)
	}

	// Wait for idle timeout.
	time.Sleep(timeout * 2)

	// Re-init on next call.
	if err := h.ensureSession(context.Background()); err != nil {
		t.Fatalf("second ensureSession failed: %v", err)
	}

	h.initMu.Lock()
	sessionNil := h.session == nil
	h.initMu.Unlock()

	if sessionNil {
		t.Error("session should be non-nil after re-initialization")
	}
	if got := initCount.Load(); got != 2 {
		t.Errorf("initFn called %d times, want 2 (initial + re-init)", got)
	}
}

func TestHandler_ResetIdleTimer_PostponesShutdown(t *testing.T) {
	const timeout = 80 * time.Millisecond
	h, watcher, _ := newTestHandler(t, timeout)

	if err := h.ensureSession(context.Background()); err != nil {
		t.Fatalf("ensureSession failed: %v", err)
	}

	// Reset the timer repeatedly just before it would fire.
	for i := 0; i < 3; i++ {
		time.Sleep(timeout / 2)
		h.resetIdleTimer()
	}

	// Resources should still be alive.
	if watcher.closed.Load() {
		t.Error("watcher should not be closed while timer is being reset")
	}
	h.initMu.Lock()
	if h.session == nil {
		t.Error("session should still be alive while timer is being reset")
	}
	h.initMu.Unlock()

	// Now stop resetting and wait for expiry.
	time.Sleep(timeout * 2)

	if !watcher.closed.Load() {
		t.Error("watcher should be closed after timer expires without reset")
	}
}

func TestConfig_IdleTimeoutDefault(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.IdleTimeout != "5m" {
		t.Errorf("default IdleTimeout = %q, want \"5m\"", cfg.IdleTimeout)
	}
}

func TestConfig_IdleTimeoutJSON(t *testing.T) {
	for _, tc := range []struct {
		json string
		want time.Duration
	}{
		{`{"idle_timeout": "10s"}`, 10 * time.Second},
		{`{"idle_timeout": "500ms"}`, 500 * time.Millisecond},
		{`{"idle_timeout": "15m"}`, 15 * time.Minute},
	} {
		cfg, err := LoadConfig([]byte(tc.json))
		if err != nil {
			t.Fatalf("LoadConfig(%s) failed: %v", tc.json, err)
		}
		h := NewHandler(nil, WithConfig(cfg))
		if h.idleTimeout != tc.want {
			t.Errorf("json=%s: idleTimeout = %v, want %v", tc.json, h.idleTimeout, tc.want)
		}
	}
}
