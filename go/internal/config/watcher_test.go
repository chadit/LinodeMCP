package config_test

import (
	"context"
	"os"
	"testing"
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
	checkNoError(t, err)

	defer watcher.Close()

	cfg := watcher.Get()
	checkNotNil(t, cfg)
	checkEqual(t, "TestServer", cfg.Server.Name)
}

// TestWatcherPicksUpReload verifies that mutating the file with a newer
// mtime triggers a reload that's visible via Get within the poll interval.
func TestWatcherPicksUpReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", validYAMLConfig())

	watcher, err := config.NewWatcher(path, pollInterval)
	checkNoError(t, err)

	defer watcher.Close()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	watcher.Start(ctx)

	// Bump the file mtime forward a full second so the next poll's
	// timestamp comparison sees the change cleanly across filesystems
	// with second-granularity mtimes (HFS+, FAT, tmpfs in some configs).
	updated := `
server:
  name: "ReloadedServer"
  logLevel: "info"
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
`
	checkNoError(t, os.WriteFile(path, []byte(updated), 0o600))

	bumpMtime(t, path)

	checkEventually(
		t,
		func() bool {
			cfg := watcher.Get()

			return cfg != nil && cfg.Server.Name == "ReloadedServer"
		},
		reloadAssertWait,
		pollInterval,
		"watcher should reload after mtime change",
	)
}

// TestWatcherKeepsLastConfigOnBadReload verifies that a syntactically bad
// reload leaves the previous Config in place and surfaces the error on the
// errors channel rather than blanking out Get.
func TestWatcherKeepsLastConfigOnBadReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", validYAMLConfig())

	watcher, err := config.NewWatcher(path, pollInterval)
	checkNoError(t, err)

	defer watcher.Close()

	original := watcher.Get()
	checkNotNil(t, original)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	watcher.Start(ctx)

	// Write garbage that fails parse + validation.
	checkNoError(t, os.WriteFile(path, []byte("not: valid: yaml: ::: "), 0o600))
	bumpMtime(t, path)

	// Give the watcher time to attempt the reload.
	select {
	case reloadErr := <-watcher.Errors():
		checkError(t, reloadErr)
	case <-time.After(reloadAssertWait):
		t.Fatal("expected a reload error on the errors channel")
	}

	// Get should still return the original (validated) config.
	current := watcher.Get()
	checkNotNil(t, current)
	checkEqual(t, original.Server.Name, current.Server.Name, "bad reload must not blank the cached config")
}

// TestWatcherOnChangeFiresAfterReload verifies that a SetOnChange callback
// runs after a successful reload and receives the new Config. This is the
// hook Phase 5 uses to wire Server.ReloadProfile into the watcher.
func TestWatcherOnChangeFiresAfterReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", validYAMLConfig())

	watcher, err := config.NewWatcher(path, pollInterval)
	checkNoError(t, err)

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
	checkNoError(t, os.WriteFile(path, []byte(updated), 0o600))
	bumpMtime(t, path)

	select {
	case cfg := <-received:
		checkNotNil(t, cfg)
		checkEqual(t, "CallbackTriggered", cfg.Server.Name, "OnChange callback must receive the post-reload Config")
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
	checkNoError(t, err)

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
	checkNoError(t, os.WriteFile(path, []byte("not: valid: yaml: ::: "), 0o600))
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
	checkNoError(t, err)

	watcher.Start(t.Context())
	watcher.Close()
	// Second close must not panic.
	checkNotPanics(t, watcher.Close)
}

// bumpMtime writes the file's mtime forward by 2 seconds so that polls on
// second-granularity filesystems detect the change reliably.
func bumpMtime(t *testing.T, path string) {
	t.Helper()

	future := time.Now().Add(2 * time.Second)
	checkNoError(t, os.Chtimes(path, future, future))
}
