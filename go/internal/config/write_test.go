package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
)

// minimalWritableConfig returns a Config that passes validation so the
// write tests can focus on the round-trip and atomic-replace contract
// rather than rebuilding the full schema in each case.
func minimalWritableConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Name:      "Test",
			LogLevel:  "info",
			Transport: "stdio",
			Host:      "127.0.0.1",
			Port:      8080,
		},
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label: envLabelDefault,
				Linode: config.LinodeConfig{
					APIURL: apiURLLinodeV4,
					Token:  "tok",
				},
			},
		},
	}
}

// TestWriteAtomicNilConfigReturnsSentinel verifies the programmer-
// error guard: passing nil must return the ErrNilConfig sentinel so
// callers can match with errors.Is.
func TestWriteAtomicNilConfigReturnsSentinel(t *testing.T) {
	t.Parallel()

	err := config.WriteAtomic(filepath.Join(t.TempDir(), "out.yml"), nil)
	if !errors.Is(err, config.ErrNilConfig) {
		t.Errorf("error = %v, want %v", err, config.ErrNilConfig)
	}
}

// TestWriteAtomicYAMLRoundTrip writes the config to a .yml file and
// loads it back, asserting the active fields survive. This is the
// happy path: marshal, atomic-replace, re-parse.
func TestWriteAtomicYAMLRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yml")
	cfg := minimalWritableConfig()
	cfg.ActiveProfile = "compute-admin"

	if err := config.WriteAtomic(path, cfg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if loaded.ActiveProfile != "compute-admin" {
		t.Errorf("loaded.ActiveProfile = %v, want %v", loaded.ActiveProfile, "compute-admin")
	}

	if loaded.Server.Name != tcTest {
		t.Errorf("loaded.Server.Name = %v, want %v", loaded.Server.Name, tcTest)
	}
}

// TestWriteAtomicJSONRoundTrip checks that JSON output uses the JSON
// serializer when the target has a .json extension and that Load can
// re-read it cleanly.
func TestWriteAtomicJSONRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")
	cfg := minimalWritableConfig()
	cfg.ActiveProfile = "readonly-full"

	if err := config.WriteAtomic(path, cfg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path) // #nosec G304 -- path is the test's tempdir target
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if data[0] != byte('{') {
		t.Errorf("data[0] = %v, want %v", data[0], byte('{'))
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if loaded.ActiveProfile != "readonly-full" {
		t.Errorf("loaded.ActiveProfile = %v, want %v", loaded.ActiveProfile, "readonly-full")
	}
}

// TestWriteAtomicPreservesOriginalMode confirms WriteAtomic does not
// clobber stricter file permissions an operator may have set. If the
// existing file is 0400, the rewritten file stays 0400.
func TestWriteAtomicPreservesOriginalMode(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yml")
	cfg := minimalWritableConfig()

	if err := config.WriteAtomic(path, cfg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := os.Chmod(path, 0o400); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Need to bump back to 0o600 so the next write can replace the file.
	// The file mode test is about the destination's preserved mode after
	// rename, not about whether the writer can overwrite a read-only
	// file (rename on POSIX needs write permission on the directory, not
	// the destination file).
	if err := config.WriteAtomic(path, cfg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if info.Mode().Perm() != os.FileMode(0o400) {
		t.Errorf("info.Mode().Perm() = %v, want %v", info.Mode().Perm(), os.FileMode(0o400))
	}
}

// TestWriteAtomicNewFileUsesDefaultMode covers the new-file path: a
// brand-new write must land at 0600 since there is no existing mode to
// preserve.
func TestWriteAtomicNewFileUsesDefaultMode(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fresh.yml")

	if err := config.WriteAtomic(path, minimalWritableConfig()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if info.Mode().Perm() != os.FileMode(0o600) {
		t.Errorf("info.Mode().Perm() = %v, want %v", info.Mode().Perm(), os.FileMode(0o600))
	}
}

// TestWriteAtomicRejectsRoundTripInvalid verifies that a config which
// survives marshal but fails the re-parse/re-validate step does NOT
// replace the existing file. The atomic guarantee depends on this:
// the temp file is never renamed when validation fails.
func TestWriteAtomicRejectsRoundTripInvalid(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yml")
	good := minimalWritableConfig()

	if err := config.WriteAtomic(path, good); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Construct an invalid config: an environment with APIURL but no
	// Token survives setDefaults and trips ErrMissingToken in validate.
	// This is the cleanest "won't pass round-trip" case since the
	// Server defaults restore empty Server.Name/LogLevel before
	// validation runs.
	bad := minimalWritableConfig()
	bad.Environments[envKeyDefault] = config.EnvironmentConfig{
		Label: envLabelDefault,
		Linode: config.LinodeConfig{
			APIURL: apiURLLinodeV4,
			Token:  "",
		},
	}

	err := config.WriteAtomic(path, bad)
	if err == nil {
		t.Error("expected an error, got nil")
	}

	// Confirm the existing file is unchanged: still loads cleanly with
	// the original Server.Name.
	loaded, err := config.Load(path)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if loaded.Server.Name != tcTest {
		t.Errorf("loaded.Server.Name = %v, want %v", loaded.Server.Name, tcTest)
	}
}

// TestWriteAtomicLeavesNoTempLeftovers verifies that even on validation
// failure the temp file is cleaned up. A directory full of orphaned
// .tmp.* files would be observable misbehavior.
func TestWriteAtomicLeavesNoTempLeftovers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	if err := config.WriteAtomic(path, minimalWritableConfig()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	bad := minimalWritableConfig()
	bad.Environments[envKeyDefault] = config.EnvironmentConfig{
		Label: envLabelDefault,
		Linode: config.LinodeConfig{
			APIURL: apiURLLinodeV4,
			Token:  "",
		},
	}

	if err := config.WriteAtomic(path, bad); err == nil {
		t.Error("expected an error, got nil")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp.") {
			t.Errorf("e.Name() should not contain %v", ".tmp.")
		}
	}
}
