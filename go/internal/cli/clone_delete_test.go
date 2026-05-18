package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/cli"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// activeClone is the user-defined name reused by the
// active-profile-delete-refusal test in two places. Extracting it
// keeps goconst quiet without inflating testconst_test.go.
const activeClone = "active-clone"

// TestRunProfileCloneCopiesBuiltinIntoUserDefined is the happy path:
// cloning a built-in into a new user-defined name persists in the
// rewritten config and the new entry carries the source's tool list.
func TestRunProfileCloneCopiesBuiltinIntoUserDefined(t *testing.T) {
	t.Parallel()

	path := writableConfig(t)

	var stdout, stderr bytes.Buffer

	exitCode := cli.RunProfileClone(
		[]string{profiles.BuiltinComputeAdmin, "my-compute"},
		path,
		&stdout,
		&stderr,
	)

	require.Equal(t, 0, exitCode, "clone must succeed: %s", stderr.String())
	assert.Contains(t, stdout.String(), "profile my-compute cloned from compute-admin")

	reloaded, err := config.Load(path)
	require.NoError(t, err)

	cloned, ok := reloaded.Profiles["my-compute"]
	require.True(t, ok, "cloned profile must appear in user profiles after reload")
	assert.NotEmpty(t, cloned.AllowedTools,
		"cloned profile must carry the source's allowed_tools")
}

// TestRunProfileCloneRefusesBuiltinDestinationName prevents the user
// from shadowing a built-in name. Built-ins are immutable in the
// catalog; a user-defined entry with the same name silently replaces
// them, which is confusing UX. Refusing at clone time makes the rule
// visible.
func TestRunProfileCloneRefusesBuiltinDestinationName(t *testing.T) {
	t.Parallel()

	path := writableConfig(t)

	var stderr bytes.Buffer

	exitCode := cli.RunProfileClone(
		[]string{profiles.BuiltinComputeAdmin, profiles.BuiltinNetworkAdmin},
		path,
		&bytes.Buffer{},
		&stderr,
	)

	require.Equal(t, 1, exitCode)
	assert.Contains(t, stderr.String(), "built-in profile name")
}

// TestRunProfileCloneRefusesEmptyDestination guards against an empty
// dst slipping through; the YAML map key would be a blank string,
// which is invalid by the schema and confusing to debug.
func TestRunProfileCloneRefusesEmptyDestination(t *testing.T) {
	t.Parallel()

	path := writableConfig(t)

	var stderr bytes.Buffer

	exitCode := cli.RunProfileClone(
		[]string{profiles.BuiltinDefault, ""},
		path,
		&bytes.Buffer{},
		&stderr,
	)

	require.Equal(t, 1, exitCode)
	assert.Contains(t, stderr.String(), "cannot be empty")
}

// TestRunProfileCloneRefusesUnknownSource locks in the source-exists
// guard: cloning from a nonexistent name produces a friendly error
// rather than writing an empty user-defined entry.
func TestRunProfileCloneRefusesUnknownSource(t *testing.T) {
	t.Parallel()

	path := writableConfig(t)

	var stderr bytes.Buffer

	exitCode := cli.RunProfileClone(
		[]string{"nonexistent-source", "my-clone"},
		path,
		&bytes.Buffer{},
		&stderr,
	)

	require.Equal(t, 1, exitCode)
	assert.Contains(t, stderr.String(), "source profile")

	// Confirm no entry was written under either name.
	reloaded, err := config.Load(path)
	require.NoError(t, err)

	_, exists := reloaded.Profiles["my-clone"]
	assert.False(t, exists, "failed clone must not leave a stub on disk")
}

// TestRunProfileCloneRefusesExistingUserDefined prevents silent
// overwrite: if dst already names a user-defined profile, the user
// must delete it first (or pick a different dst).
func TestRunProfileCloneRefusesExistingUserDefined(t *testing.T) {
	t.Parallel()

	path := writableConfig(t)

	// First clone to create the user-defined entry.
	require.Equal(t, 0, cli.RunProfileClone(
		[]string{profiles.BuiltinDefault, "my-prof"},
		path,
		&bytes.Buffer{},
		&bytes.Buffer{},
	))

	var stderr bytes.Buffer

	exitCode := cli.RunProfileClone(
		[]string{profiles.BuiltinComputeAdmin, "my-prof"},
		path,
		&bytes.Buffer{},
		&stderr,
	)

	require.Equal(t, 1, exitCode, "second clone to the same dst must be refused")
	assert.Contains(t, stderr.String(), "already exists")
}

// TestRunProfileCloneZeroArgsReturnsUsage covers the arity guard.
func TestRunProfileCloneZeroArgsReturnsUsage(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	exitCode := cli.RunProfileClone(nil, "", &bytes.Buffer{}, &stderr)

	assert.Equal(t, cli.ExitUsageError, exitCode)
	assert.Contains(t, stderr.String(), "Usage:")
}

// TestRunProfileDeleteRemovesUserDefined is the happy path: a
// user-defined profile gets removed and the rewritten file no longer
// contains it.
func TestRunProfileDeleteRemovesUserDefined(t *testing.T) {
	t.Parallel()

	path := writableConfig(t)

	// Stage a user-defined profile to delete.
	require.Equal(t, 0, cli.RunProfileClone(
		[]string{profiles.BuiltinDefault, "to-delete"},
		path,
		&bytes.Buffer{},
		&bytes.Buffer{},
	))

	var stdout, stderr bytes.Buffer

	exitCode := cli.RunProfileDelete(
		[]string{"to-delete"},
		path,
		&stdout,
		&stderr,
	)

	require.Equal(t, 0, exitCode, "delete must succeed: %s", stderr.String())
	assert.Contains(t, stdout.String(), "profile to-delete deleted")

	reloaded, err := config.Load(path)
	require.NoError(t, err)

	_, exists := reloaded.Profiles["to-delete"]
	assert.False(t, exists, "deleted profile must be gone from the config")
}

// TestRunProfileDeleteRefusesBuiltin verifies the safety guard:
// built-ins live in code, not config, so `delete` on a built-in name
// would write nothing useful. The error directs the user to `disable`
// instead.
func TestRunProfileDeleteRefusesBuiltin(t *testing.T) {
	t.Parallel()

	path := writableConfig(t)

	var stderr bytes.Buffer

	exitCode := cli.RunProfileDelete(
		[]string{profiles.BuiltinComputeAdmin},
		path,
		&bytes.Buffer{},
		&stderr,
	)

	require.Equal(t, 1, exitCode)
	assert.Contains(t, stderr.String(), "built-in")
	assert.Contains(t, stderr.String(), "disable")
}

// TestRunProfileDeleteRefusesUnknown verifies the existence guard:
// deleting a name that's neither built-in nor in the user map exits 1
// without writing.
func TestRunProfileDeleteRefusesUnknown(t *testing.T) {
	t.Parallel()

	path := writableConfig(t)

	var stderr bytes.Buffer

	exitCode := cli.RunProfileDelete(
		[]string{"nonexistent-profile"},
		path,
		&bytes.Buffer{},
		&stderr,
	)

	require.Equal(t, 1, exitCode)
	assert.Contains(t, stderr.String(), "not found")
}

// TestRunProfileDeleteRefusesActiveProfile covers the safety guard
// that mirrors disable: deleting the active profile would leave the
// server unable to start. The user must switch first.
func TestRunProfileDeleteRefusesActiveProfile(t *testing.T) {
	t.Parallel()

	path := writableConfig(t)

	// Stage and activate a user-defined profile.
	require.Equal(t, 0, cli.RunProfileClone(
		[]string{profiles.BuiltinDefault, activeClone},
		path,
		&bytes.Buffer{},
		&bytes.Buffer{},
	))
	require.Equal(t, 0, cli.RunProfileUse(
		[]string{activeClone},
		path,
		&bytes.Buffer{},
		&bytes.Buffer{},
	))

	var stderr bytes.Buffer

	exitCode := cli.RunProfileDelete(
		[]string{activeClone},
		path,
		&bytes.Buffer{},
		&stderr,
	)

	require.Equal(t, 1, exitCode)
	assert.Contains(t, stderr.String(), "active profile")

	reloaded, err := config.Load(path)
	require.NoError(t, err)

	_, exists := reloaded.Profiles[activeClone]
	assert.True(t, exists,
		"refused delete must not remove the entry from disk")
}

// TestRunProfileDeleteZeroArgsReturnsUsage covers the arity guard.
func TestRunProfileDeleteZeroArgsReturnsUsage(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	exitCode := cli.RunProfileDelete(nil, "", &bytes.Buffer{}, &stderr)

	assert.Equal(t, cli.ExitUsageError, exitCode)
	assert.Contains(t, stderr.String(), "Usage:")
}
