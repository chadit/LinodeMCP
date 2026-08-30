package config_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
)

// requireNonRoot skips permission-denial cases when the suite runs as root,
// because root ignores mode bits and the denial the case depends on never
// happens.
func requireNonRoot(t *testing.T) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("permission denial is not observable as root")
	}
}

// TestLoadReportsPermissionDenialByName verifies a config file the process
// cannot read maps to ErrConfigPermissions, so the operator is told to fix
// the mode rather than hunt for a missing file.
func TestLoadReportsPermissionDenialByName(t *testing.T) {
	t.Parallel()
	requireNonRoot(t)

	path := writeConfigFile(t, t.TempDir(), "config.yml", validYAMLConfig())
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	_, err := config.Load(path)
	if !errors.Is(err, config.ErrConfigPermissions) {
		t.Errorf("error = %v, want %v", err, config.ErrConfigPermissions)
	}
}

// TestLoadRefusesDirectoryPath verifies a path that names a directory is
// reported as a read failure carrying the path, and is not confused with
// the missing-file or permission sentinels that have their own remedies.
func TestLoadRefusesDirectoryPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if errors.Is(err, config.ErrConfigFileNotFound) || errors.Is(err, config.ErrConfigPermissions) {
		t.Errorf("error = %v, want a plain read failure", err)
	}

	pathErr, ok := errors.AsType[*fs.PathError](err)
	if !ok {
		t.Fatalf("error = %v, want it to wrap the *fs.PathError the read produced", err)
	}

	if pathErr.Path != dir {
		t.Errorf("error names %q, want %q", pathErr.Path, dir)
	}
}

// TestLoadRejectsInvalidEnvironmentBlocks verifies each malformed
// environment entry fails validation with its own sentinel under
// ErrConfigInvalid, so the message points at the exact field to fix.
func TestLoadRejectsInvalidEnvironmentBlocks(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		yaml    string
	}{
		{
			name: "empty environment name",
			yaml: `
environments:
  "":
    label: "Unnamed"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
`,
			wantErr: config.ErrEmptyEnvironmentName,
		},
		{
			name: "token without api url",
			yaml: `
environments:
  default:
    label: "Default"
    linode:
      token: "tok"
`,
			wantErr: config.ErrMissingAPIURL,
		},
	}

	for _, tcase := range cases {
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			path := writeConfigFile(t, t.TempDir(), "config.yml", tcase.yaml)

			_, err := config.Load(path)
			if !errors.Is(err, config.ErrConfigInvalid) {
				t.Fatalf("error = %v, want %v", err, config.ErrConfigInvalid)
			}

			if !errors.Is(err, tcase.wantErr) {
				t.Errorf("error = %v, want %v", err, tcase.wantErr)
			}
		})
	}
}

// TestPathPrefersJSONUnderHomeConfigDir verifies the default config path
// without an override: config.yml under ~/.config/linodemcp until a
// config.json appears there, which then wins.
func TestPathPrefersJSONUnderHomeConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LINODEMCP_CONFIG_PATH", "")
	t.Setenv("HOME", home)

	configDir := filepath.Join(home, ".config", "linodemcp")

	if got, want := config.Path(), filepath.Join(configDir, "config.yml"); got != want {
		t.Errorf("config.Path() = %v, want %v", got, want)
	}

	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	jsonPath := writeConfigFile(t, configDir, "config.json", validJSONConfig())

	if got := config.Path(); got != jsonPath {
		t.Errorf("config.Path() = %v, want %v", got, jsonPath)
	}
}

// TestPathFallsBackToTempDirWithoutHome verifies that when no home
// directory can be resolved the default path lands under the system temp
// directory instead of an empty or relative path.
func TestPathFallsBackToTempDirWithoutHome(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", "")
	t.Setenv("HOME", "")

	want := filepath.Join(os.TempDir(), "linodemcp", "config.yml")
	if got := config.Path(); got != want {
		t.Errorf("config.Path() = %v, want %v", got, want)
	}
}

// TestApplyEnvironmentOverridesLabelsCreatedDefault verifies that when the
// Linode env vars create the default environment from scratch it gets the
// "Default" label, so the environment list shows a name rather than a blank.
func TestApplyEnvironmentOverridesLabelsCreatedDefault(t *testing.T) {
	t.Setenv("LINODEMCP_LINODE_API_URL", "https://override.api.com")
	t.Setenv("LINODEMCP_LINODE_TOKEN", "env-token")

	path := writeConfigFile(t, t.TempDir(), "config.yml", `
environments:
  prod:
    label: "Production"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	created, ok := cfg.Environments[envKeyDefault]
	if !ok {
		t.Fatal("Environments missing the default the env vars should create")
	}

	if created.Label != envLabelDefault {
		t.Errorf("created.Label = %v, want %v", created.Label, envLabelDefault)
	}

	if created.Linode.Token != "env-token" {
		t.Errorf("created.Linode.Token = %v, want %v", created.Linode.Token, "env-token")
	}
}
