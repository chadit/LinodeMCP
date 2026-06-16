package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/cli"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// writableConfig stages a minimal config file in a tempdir and returns
// the path. The fixture matches the minimum shape that survives Load
// plus validateConfig: a server section, one environment with an API
// URL and token, and no profile overrides. Each test owns its own
// tempdir so parallel runs don't collide.
func writableConfig(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	contents := `
server:
  name: "Test"
  logLevel: "info"
  transport: "stdio"
  host: "127.0.0.1"
  port: 8080
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
`

	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	return path
}

// TestRunProfileUseSwitchesActiveProfile is the happy path: a known
// profile name is set as the active one, and the rewritten file
// reloads with the new ActiveProfile value.
func TestRunProfileUseSwitchesActiveProfile(t *testing.T) {
	t.Parallel()

	path := writableConfig(t)

	var stdout, stderr bytes.Buffer

	exitCode := cli.RunProfileUse([]string{"readonly-full"}, path, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("switching to a known built-in exit code = %d, want 0", exitCode)
	}

	wantContains(t, "stdout", stdout.String(), "active profile switched to readonly-full")

	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if reloaded.ActiveProfile != "readonly-full" {
		t.Fatalf("ActiveProfile = %q, want readonly-full", reloaded.ActiveProfile)
	}
}

// TestRunProfileUseUnknownProfileExitsOne verifies the validation
// guard: an unknown name exits 1 without writing. The existing
// ActiveProfile must remain untouched on disk.
func TestRunProfileUseUnknownProfileExitsOne(t *testing.T) {
	t.Parallel()

	path := writableConfig(t)

	var stderr bytes.Buffer

	exitCode := cli.RunProfileUse(
		[]string{"definitely-not-a-real-profile"},
		path,
		&bytes.Buffer{},
		&stderr,
	)

	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}

	wantContains(t, "stderr", stderr.String(), "not found")

	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if reloaded.ActiveProfile != "" {
		t.Fatalf("failed use wrote ActiveProfile = %q", reloaded.ActiveProfile)
	}
}

// TestRunProfileUseZeroArgsReturnsUsage covers the arity check; the
// subcommand requires exactly one positional argument.
func TestRunProfileUseZeroArgsReturnsUsage(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	exitCode := cli.RunProfileUse(nil, "", &bytes.Buffer{}, &stderr)

	if exitCode != cli.ExitUsageError {
		t.Fatalf("exit code = %d, want %d", exitCode, cli.ExitUsageError)
	}

	wantContains(t, "stderr", stderr.String(), "Usage:")
}

// TestRunProfileDisableSetsOverride exercises the disable path: the
// override map gains an entry with Disabled=true, and round-tripping
// through Load surfaces it.
func TestRunProfileDisableSetsOverride(t *testing.T) {
	t.Parallel()

	path := writableConfig(t)

	var stdout, stderr bytes.Buffer

	exitCode := cli.RunProfileDisable(
		[]string{profiles.BuiltinComputeAdmin},
		path,
		&stdout,
		&stderr,
	)

	if exitCode != 0 {
		t.Fatalf("disabling a non-active built-in must succeed: %s", stderr.String())
	}

	wantContains(t, "stdout", stdout.String(), "profile compute-admin disabled")

	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if reloaded.ProfilesBuiltinOverrides == nil {
		t.Fatalf("ProfilesBuiltinOverrides is nil")
	}

	if !reloaded.ProfilesBuiltinOverrides[profiles.BuiltinComputeAdmin].Disabled {
		t.Fatalf("override map must persist Disabled=true")
	}
}

// TestRunProfileEnableClearsOverride is the inverse: enable resets
// the disabled bit so the built-in is selectable again.
func TestRunProfileEnableClearsOverride(t *testing.T) {
	t.Parallel()

	path := writableConfig(t)

	// First disable, then enable, then assert the override flipped back.

	if exitCode := cli.RunProfileDisable(
		[]string{profiles.BuiltinComputeAdmin},
		path,
		&bytes.Buffer{},
		&bytes.Buffer{},
	); exitCode != 0 {
		t.Fatalf("initial disable exit code = %d, want 0", exitCode)
	}

	var stdout, stderr bytes.Buffer

	exitCode := cli.RunProfileEnable(
		[]string{profiles.BuiltinComputeAdmin},
		path,
		&stdout,
		&stderr,
	)

	if exitCode != 0 {
		t.Fatalf("enable must succeed: %s", stderr.String())
	}

	wantContains(t, "stdout", stdout.String(), "profile compute-admin enabled")

	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if reloaded.ProfilesBuiltinOverrides[profiles.BuiltinComputeAdmin].Disabled {
		t.Fatalf("enable must clear the Disabled bit")
	}
}

// TestRunProfileDisableRefusesActiveProfile verifies the safety
// guard: disabling the currently-active profile would leave the
// server unable to start, so the subcommand must reject it and not
// write.
func TestRunProfileDisableRefusesActiveProfile(t *testing.T) {
	t.Parallel()

	path := writableConfig(t)

	// Switch to compute-admin first.

	if exitCode := cli.RunProfileUse(
		[]string{profiles.BuiltinComputeAdmin},
		path,
		&bytes.Buffer{},
		&bytes.Buffer{},
	); exitCode != 0 {
		t.Fatalf("profile use exit code = %d, want 0", exitCode)
	}

	var stderr bytes.Buffer

	exitCode := cli.RunProfileDisable(
		[]string{profiles.BuiltinComputeAdmin},
		path,
		&bytes.Buffer{},
		&stderr,
	)

	if exitCode != 1 {
		t.Fatalf("disabling active profile exit code = %d, want 1", exitCode)
	}

	wantContains(t, "stderr", stderr.String(), "active profile")

	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	overrides := reloaded.ProfilesBuiltinOverrides

	if overrides[profiles.BuiltinComputeAdmin].Disabled {
		t.Fatalf("refused disable must not flip the bit on disk")
	}
}

// TestRunProfileEnableRefusesUserDefined verifies that enable/disable
// only apply to built-ins. The override map has no effect on
// user-defined profiles, so accepting the command would silently
// no-op; refusing it makes the misuse visible.
func TestRunProfileEnableRefusesUserDefined(t *testing.T) {
	t.Parallel()

	path := writableConfig(t)

	var stderr bytes.Buffer

	exitCode := cli.RunProfileEnable(
		[]string{"my-custom-profile"},
		path,
		&bytes.Buffer{},
		&stderr,
	)

	if exitCode != 1 {
		t.Fatalf("enable on a non-built-in exit code = %d, want 1", exitCode)
	}

	wantContains(t, "stderr", stderr.String(), "not a built-in")
}

// TestRunProfileEnableZeroArgsReturnsUsage covers the arity check on
// enable. Disable shares the implementation; one test exercises both
// guard clauses since the shared helper is what we're protecting.
func TestRunProfileEnableZeroArgsReturnsUsage(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	exitCode := cli.RunProfileEnable(nil, "", &bytes.Buffer{}, &stderr)

	if exitCode != cli.ExitUsageError {
		t.Fatalf("exit code = %d, want %d", exitCode, cli.ExitUsageError)
	}

	wantContains(t, "stderr", stderr.String(), "Usage:")
}

// TestRunProfileDisableZeroArgsReturnsUsage mirrors the enable case
// for the disable side of the shared toggle helper.
func TestRunProfileDisableZeroArgsReturnsUsage(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	exitCode := cli.RunProfileDisable(nil, "", &bytes.Buffer{}, &stderr)

	if exitCode != cli.ExitUsageError {
		t.Fatalf("exit code = %d, want %d", exitCode, cli.ExitUsageError)
	}

	wantContains(t, "stderr", stderr.String(), "Usage:")
}
