package config

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// DefaultWatchInterval is the polling cadence for mtime-based hot-reload.
// 5 seconds balances responsiveness with file-system load.
const DefaultWatchInterval = 5 * time.Second

// Watcher hot-reloads a Config from disk by polling the file's mtime. The
// underlying Config is held behind an atomic pointer so reads are lock-free
// and consistent. New configs are validated before being swapped in; a bad
// reload leaves the previous Config in place and reports the error via the
// errors channel (consumers can drain or ignore).
//
// Tokens, API URLs, and other environment-scoped fields are NOT special-
// cased: a reload swaps the whole Config. Operators who don't want a field
// to change at runtime should keep it out of the file (use env-var
// overrides), since those are only read once at startup.
type Watcher struct {
	path     string
	interval time.Duration
	current  atomic.Pointer[Config]
	lastMod  time.Time
	errs     chan error
	stop     chan struct{}
	stopped  atomic.Bool
}

// NewWatcher loads the file once and returns a Watcher seeded with that
// Config. Call Start to begin background polling. interval <= 0 picks
// DefaultWatchInterval.
func NewWatcher(path string, interval time.Duration) (*Watcher, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, fmt.Errorf("initial config load: %w", err)
	}

	if interval <= 0 {
		interval = DefaultWatchInterval
	}

	info, statErr := os.Stat(path) // #nosec G304 -- path comes from operator config
	if statErr != nil {
		return nil, fmt.Errorf("stat config file: %w", statErr)
	}

	watcher := &Watcher{
		path:     path,
		interval: interval,
		lastMod:  info.ModTime(),
		errs:     make(chan error, watcherErrBuffer),
		stop:     make(chan struct{}),
	}
	watcher.current.Store(cfg)

	return watcher, nil
}

// watcherErrBuffer is small on purpose: hot-reload errors are advisory and
// not consumed by any blocking caller. Drop on overflow.
const watcherErrBuffer = 8

// Get returns the current Config. The pointer is stable; callers that
// stash it observe a snapshot. New reads pick up the latest reload.
func (w *Watcher) Get() *Config {
	return w.current.Load()
}

// Errors returns the channel of reload errors. Receive-only for consumers.
// Errors are dropped if the channel is full.
func (w *Watcher) Errors() <-chan error {
	return w.errs
}

// Start begins polling the file mtime in a background goroutine. Returns
// immediately. Stop the watcher with Close or by canceling ctx. Safe to
// call multiple times; only the first call starts a goroutine.
func (w *Watcher) Start(ctx context.Context) {
	if w.stopped.Load() {
		return
	}

	go w.run(ctx)
}

// Close stops the background polling goroutine. Idempotent.
func (w *Watcher) Close() {
	if w.stopped.CompareAndSwap(false, true) {
		close(w.stop)
	}
}

func (w *Watcher) run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-ticker.C:
			if err := w.checkAndReload(); err != nil {
				w.reportError(err)
			}
		}
	}
}

func (w *Watcher) checkAndReload() error {
	info, err := os.Stat(w.path) // #nosec G304 -- path is from operator config
	if err != nil {
		return fmt.Errorf("stat config: %w", err)
	}

	mod := info.ModTime()
	if !mod.After(w.lastMod) {
		return nil
	}

	cfg, err := Load(w.path)
	if err != nil {
		// Don't update lastMod: leave the bad file flagged so a subsequent
		// poll keeps re-reporting until the operator fixes it.
		return fmt.Errorf("reload config: %w", err)
	}

	w.lastMod = mod
	w.current.Store(cfg)

	return nil
}

func (w *Watcher) reportError(err error) {
	select {
	case w.errs <- err:
	default:
		// Channel full; drop. Errors are advisory.
	}
}
