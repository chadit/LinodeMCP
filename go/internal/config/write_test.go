package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
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

	require.Error(t, err)
	require.ErrorIs(t, err, config.ErrNilConfig)
}

// TestWriteAtomicYAMLRoundTrip writes the config to a .yml file and
// loads it back, asserting the active fields survive. This is the
// happy path: marshal, atomic-replace, re-parse.
func TestWriteAtomicYAMLRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yml")
	cfg := minimalWritableConfig()
	cfg.ActiveProfile = "compute-admin"

	require.NoError(t, config.WriteAtomic(path, cfg))

	loaded, err := config.Load(path)
	require.NoError(t, err, "round-trip Load must succeed on the written file")
	assert.Equal(t, "compute-admin", loaded.ActiveProfile)
	assert.Equal(t, "Test", loaded.Server.Name)
}

// TestWriteAtomicJSONRoundTrip checks that JSON output uses the JSON
// serializer when the target has a .json extension and that Load can
// re-read it cleanly.
func TestWriteAtomicJSONRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")
	cfg := minimalWritableConfig()
	cfg.ActiveProfile = "readonly-full"

	require.NoError(t, config.WriteAtomic(path, cfg))

	data, err := os.ReadFile(path) // #nosec G304 -- path is the test's tempdir target
	require.NoError(t, err)
	assert.Equal(t, byte('{'), data[0],
		"JSON extension must produce JSON output (starts with '{')")

	loaded, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "readonly-full", loaded.ActiveProfile)
}

// TestWriteAtomicPreservesOriginalMode confirms WriteAtomic does not
// clobber stricter file permissions an operator may have set. If the
// existing file is 0400, the rewritten file stays 0400.
func TestWriteAtomicPreservesOriginalMode(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yml")
	cfg := minimalWritableConfig()

	require.NoError(t, config.WriteAtomic(path, cfg),
		"initial write should succeed (sets default 0600)")

	require.NoError(t, os.Chmod(path, 0o400),
		"operator hardens permissions to read-only")

	// Need to bump back to 0o600 so the next write can replace the file.
	// The file mode test is about the destination's preserved mode after
	// rename, not about whether the writer can overwrite a read-only
	// file (rename on POSIX needs write permission on the directory, not
	// the destination file).
	require.NoError(t, config.WriteAtomic(path, cfg),
		"second write must succeed and preserve the 0400 mode")

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o400), info.Mode().Perm(),
		"rewritten file must keep the original 0400 permission bits")
}

// TestWriteAtomicNewFileUsesDefaultMode covers the new-file path: a
// brand-new write must land at 0600 since there is no existing mode to
// preserve.
func TestWriteAtomicNewFileUsesDefaultMode(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fresh.yml")

	require.NoError(t, config.WriteAtomic(path, minimalWritableConfig()))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"new file must default to 0600")
}

// TestWriteAtomicRejectsRoundTripInvalid verifies that a config which
// survives marshal but fails the re-parse/re-validate step does NOT
// replace the existing file. The atomic guarantee depends on this:
// the temp file is never renamed when validation fails.
func TestWriteAtomicRejectsRoundTripInvalid(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yml")
	good := minimalWritableConfig()

	require.NoError(t, config.WriteAtomic(path, good))

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
	require.Error(t, err, "validation failure must surface, not silently overwrite")

	// Confirm the existing file is unchanged: still loads cleanly with
	// the original Server.Name.
	loaded, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "Test", loaded.Server.Name,
		"failed write must not replace the good file on disk")
}

// TestWriteAtomicLeavesNoTempLeftovers verifies that even on validation
// failure the temp file is cleaned up. A directory full of orphaned
// .tmp.* files would be observable misbehavior.
func TestWriteAtomicLeavesNoTempLeftovers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	require.NoError(t, config.WriteAtomic(path, minimalWritableConfig()))

	bad := minimalWritableConfig()
	bad.Environments[envKeyDefault] = config.EnvironmentConfig{
		Label: envLabelDefault,
		Linode: config.LinodeConfig{
			APIURL: apiURLLinodeV4,
			Token:  "",
		},
	}

	require.Error(t, config.WriteAtomic(path, bad))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp.",
			"failed atomic write must not leave a .tmp.* file behind")
	}
}
