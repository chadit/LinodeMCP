package config_test

import (
	"context"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/chadit/LinodeMCP/internal/config"
)

const (
	pollInterval     = 20 * time.Millisecond
	reloadAssertWait = 500 * time.Millisecond
)

// TestWatcherInitialLoad confirms that NewWatcher loads the file once and
// makes that snapshot available via Get before any polling has happened.
func TestWatcherInitialLoad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", validYAMLConfig())

	watcher, err := config.NewWatcher(path, pollInterval)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	defer watcher.Close()

	cfg := watcher.Get()
	if cfg == nil {
		t.Fatal("cfg is nil")
	}

	if cfg.Server.Name != tcTestServer {
		t.Errorf("cfg.Server.Name = %v, want %v", cfg.Server.Name, tcTestServer)
	}
}

// TestWatcherPicksUpReload verifies that mutating the file with a newer
// mtime triggers a reload that's visible via Get within the poll interval.
func TestWatcherPicksUpReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", validYAMLConfig())

	watcher, err := config.NewWatcher(path, pollInterval)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	defer watcher.Close()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	watcher.Start(ctx)

	// Bump the file mtime forward a full second so the next poll's
	// timestamp comparison sees the change cleanly across filesystems
	// with second-granularity mtimes (HFS+, FAT, tmpfs in some configs).
	updated := reloadedServerYAML
	if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	bumpMtime(t, path)

	deadline := time.NewTimer(reloadAssertWait)
	defer deadline.Stop()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		cfg := watcher.Get()
		if cfg != nil && cfg.Server.Name == tcReloadedServer {
			break
		}

		select {
		case <-deadline.C:
			t.Fatal("watcher should reload after mtime change")
		case <-ticker.C:
		}
	}
}

// TestWatcherKeepsLastConfigOnBadReload verifies that a syntactically bad
// reload leaves the previous Config in place and surfaces the error on the
// errors channel rather than blanking out Get.
func TestWatcherKeepsLastConfigOnBadReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", validYAMLConfig())

	watcher, err := config.NewWatcher(path, pollInterval)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	defer watcher.Close()

	original := watcher.Get()
	if original == nil {
		t.Error("original is nil")
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	watcher.Start(ctx)

	// Write garbage that fails parse + validation.
	if err := os.WriteFile(path, []byte("not: valid: yaml: ::: "), 0o600); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	bumpMtime(t, path)

	// Give the watcher time to attempt the reload.
	select {
	case reloadErr := <-watcher.Errors():
		if reloadErr == nil {
			t.Error("expected an error, got nil")
		}
	case <-time.After(reloadAssertWait):
		t.Fatal("expected a reload error on the errors channel")
	}

	// Get should still return the original (validated) config.
	current := watcher.Get()
	if current == nil {
		t.Fatal("current is nil")
	}

	if current.Server.Name != original.Server.Name {
		t.Errorf("current.Server.Name = %v, want %v", current.Server.Name, original.Server.Name)
	}
}

// TestWatcherOnChangeFiresAfterReload verifies that a SetOnChange callback
// runs after a successful reload and receives the new Config. This is the
// hook Phase 5 uses to wire Server.ReloadProfile into the watcher.
func TestWatcherOnChangeFiresAfterReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", validYAMLConfig())

	watcher, err := config.NewWatcher(path, pollInterval)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	defer watcher.Close()

	received := make(chan *config.Config, 1)

	watcher.SetOnChange(func(cfg *config.Config) {
		select {
		case received <- cfg:
		default:
		}
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	watcher.Start(ctx)

	updated := `
server:
  name: "CallbackTriggered"
  logLevel: "info"
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
`
	if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	bumpMtime(t, path)

	select {
	case cfg := <-received:
		if cfg == nil {
			t.Fatal("cfg is nil")
		}

		if cfg.Server.Name != "CallbackTriggered" {
			t.Errorf("cfg.Server.Name = %v, want %v", cfg.Server.Name, "CallbackTriggered")
		}
	case <-time.After(reloadAssertWait):
		t.Fatal("OnChange callback did not fire within the deadline")
	}
}

// TestWatcherOnChangeNotFiredOnBadReload confirms the callback is NOT
// invoked when a reload fails. A failed reload keeps the previous Config
// in place and must not propagate phantom updates to subscribers.
func TestWatcherOnChangeNotFiredOnBadReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", validYAMLConfig())

	watcher, err := config.NewWatcher(path, pollInterval)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	defer watcher.Close()

	fired := make(chan *config.Config, 1)

	watcher.SetOnChange(func(cfg *config.Config) {
		select {
		case fired <- cfg:
		default:
		}
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	watcher.Start(ctx)

	// Garbage config: parse will fail, lastMod stays put, callback must NOT fire.
	if err := os.WriteFile(path, []byte("not: valid: yaml: ::: "), 0o600); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	bumpMtime(t, path)

	select {
	case <-watcher.Errors():
		// Expected: bad reload surfaces on the errors channel.
	case <-time.After(reloadAssertWait):
		t.Fatal("expected reload error on errors channel")
	}

	select {
	case got := <-fired:
		t.Fatalf("OnChange callback fired for failed reload: %+v", got)
	case <-time.After(2 * pollInterval):
		// OK: callback stayed silent.
	}
}

// TestWatcherCloseStopsPolling confirms that Close releases the polling
// goroutine and that subsequent Close calls are safe (idempotent).
func TestWatcherCloseStopsPolling(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", validYAMLConfig())

	watcher, err := config.NewWatcher(path, pollInterval)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	watcher.Start(t.Context())
	watcher.Close()
	// Second close must not panic.
	watcher.Close()
}

// bumpMtime writes the file's mtime forward by 2 seconds so that polls on
// second-granularity filesystems detect the change reliably.
func bumpMtime(t *testing.T, path string) {
	t.Helper()

	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestWatcherReloadsUnderSynctest exercises the hot-reload path inside a
// synctest bubble. The bubble's fake clock fires the poll ticker without any
// real time passing, so the test runs instantly with no real sleeps. When the bubble
// function returns, synctest waits for every goroutine it started to exit and
// fails if one is still alive -- a built-in goroutine-leak check for run() that
// needs no third-party dependency.
func TestWatcherReloadsUnderSynctest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", validYAMLConfig())

	synctest.Test(t, func(t *testing.T) {
		watcher, err := config.NewWatcher(path, pollInterval)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reloaded := make(chan *config.Config, 1)

		watcher.SetOnChange(func(cfg *config.Config) {
			select {
			case reloaded <- cfg:
			default:
			}
		})

		watcher.Start(t.Context())

		updated := reloadedServerYAML
		if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Force a strictly-newer mtime. ModTime is a real wall-clock value
		// (synctest fakes time.Now but not the filesystem), so derive the new
		// stamp from the file's own mtime rather than time.Now.
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		newer := info.ModTime().Add(2 * time.Second)
		if err := os.Chtimes(path, newer, newer); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Advance the fake clock past one poll: the ticker fires, run() reloads,
		// and the callback runs -- all with zero real time elapsed.
		time.Sleep(2 * pollInterval)
		synctest.Wait()

		select {
		case cfg := <-reloaded:
			if cfg.Server.Name != tcReloadedServer {
				t.Errorf("cfg.Server.Name = %v, want %v", cfg.Server.Name, "ReloadedServer")
			}
		default:
			t.Fatal("watcher did not reload after the mtime change")
		}

		if got := watcher.Get().Server.Name; got != tcReloadedServer {
			t.Errorf("watcher.Get().Server.Name = %v, want %v", got, "ReloadedServer")
		}

		// Releasing run() before the bubble returns proves Close stops the
		// goroutine; a broken Close would leave it live and synctest would fail.
		watcher.Close()
	})
}

// TestWatcherCloseStopsPollingUnderSynctest verifies Close actually halts the
// poll loop: a file change made after Close, even with a newer mtime, must not
// be picked up. A Close that left run() spinning would reload and fail the
// assertion. The synctest bubble runs it instantly on the fake clock.
func TestWatcherCloseStopsPollingUnderSynctest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", validYAMLConfig())

	synctest.Test(t, func(t *testing.T) {
		watcher, err := config.NewWatcher(path, pollInterval)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		watcher.Start(t.Context())
		synctest.Wait() // run() is durably blocked on its poll select
		watcher.Close()
		synctest.Wait() // run() observes the stop and exits

		// Mutate the file after Close. A stopped watcher must ignore it.
		updated := reloadedServerYAML
		if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		newer := info.ModTime().Add(2 * time.Second)
		if err := os.Chtimes(path, newer, newer); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Advance the fake clock past several polls; with run() stopped nothing
		// should reload.
		time.Sleep(2 * pollInterval)
		synctest.Wait()

		if got := watcher.Get().Server.Name; got == tcReloadedServer {
			t.Errorf("watcher reloaded after Close: Server.Name = %v", got)
		}
	})
}
