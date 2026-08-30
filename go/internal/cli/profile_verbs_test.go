package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/cli"
	"github.com/chadit/LinodeMCP/go/internal/config"
)

// baseConfigYAML is the smallest config every verb test can load. Tests
// append profile or audit blocks to it, so one fixture body carries the
// server and environment sections instead of each test restating them.
const baseConfigYAML = `
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
  staging:
    label: "Staging"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
`

// profileCatalogYAML adds an active built-in, a disabled built-in, and a
// yolo-enabled user profile so `profile list` has one row per state.
const profileCatalogYAML = baseConfigYAML + `
active_profile: "compute-admin"
profiles_builtin_overrides:
  network-admin:
    disabled: true
profiles:
  yolo-ops:
    description: "user profile for the list test"
    allowed_tools: ["linode_volume_list", "linode_instance_list"]
    allowed_environments: ["default", "staging"]
    allow_yolo: true
`

// malformedConfigYAML fails YAML parsing, which is the "unreadable config"
// a user hits after a bad hand edit.
const malformedConfigYAML = "server: [\n  name: \"Test\"\n"

const (
	loadConfigError  = "load config from"
	userProfileYolo  = "yolo-ops"
	profileNoSuch    = "no-such"
	profileReadonly  = "readonly-full"
	profileNetworkAd = "network-admin"
	clonedProfile    = "mine"

	subverbShow    = "show"
	subverbList    = "list"
	subverbUse     = "use"
	subverbEnable  = "enable"
	subverbDisable = "disable"
	subverbClone   = "clone"
	subverbDelete  = "delete"
)

// writeConfigYAML stages contents as config.yml in a fresh tempdir and
// returns its path, so each test loads its own file rather than the
// user's real config.
func writeConfigYAML(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yml")

	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	return path
}

// wantMatch asserts got matches the pattern so a table row can be checked
// column by column without depending on exact padding widths.
func wantMatch(t *testing.T, label, got, pattern string) {
	t.Helper()

	if !regexp.MustCompile(pattern).MatchString(got) {
		t.Fatalf("%s does not match %q:\n%s", label, pattern, got)
	}
}

// TestProfileListMarksActiveAndReportsState checks the one-line summary
// per profile: the active profile carries the '*' marker, a disabled
// built-in reads DISABLED, a yolo profile reads YES, and the tools column
// counts the profile's allowed tools.
func TestProfileListMarksActiveAndReportsState(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeConfigYAML(t, profileCatalogYAML))

	var stdout, stderr bytes.Buffer

	code := cli.RunProfileCommand([]string{subverbList}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	out := stdout.String()
	wantMatch(t, "header", out, `^\*\s+name\s+yolo\s+state\s+tools\n`)
	wantMatch(t, "active row", out, `(?m)^\*\s+compute-admin\s+no\s+enabled\s+\d+$`)
	wantMatch(t, "disabled row", out, `(?m)^\s+network-admin\s+no\s+DISABLED\s+\d+$`)
	wantMatch(t, "user row", out, `(?m)^\s+yolo-ops\s+YES\s+enabled\s+2$`)
}

// TestProfileListRejectsArguments checks that `profile list extra` is a
// usage error rather than silently ignoring the stray argument.
func TestProfileListRejectsArguments(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeConfigYAML(t, baseConfigYAML))

	var stdout, stderr bytes.Buffer

	code := cli.RunProfileList([]string{"extra"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}

	wantContains(t, "stderr", stderr.String(), "takes no arguments")
}

// TestProfileReadVerbsReportUnreadableConfig checks list and show exit 1
// and name the config path when the file is missing or fails to parse,
// instead of listing an empty catalog.
func TestProfileReadVerbsReportUnreadableConfig(t *testing.T) {
	cases := []struct {
		name string
		path string
		args []string
	}{
		{name: "list on a missing file", path: missingConfigPath(t), args: []string{subverbList}},
		{name: "show on a missing file", path: missingConfigPath(t), args: []string{subverbShow, testEnvKey}},
		{name: "list on malformed yaml", path: writeConfigYAML(t, malformedConfigYAML), args: []string{subverbList}},
		{name: "show on malformed yaml", path: writeConfigYAML(t, malformedConfigYAML), args: []string{subverbShow, testEnvKey}},
	}

	for _, scenario := range cases {
		t.Run(scenario.name, func(t *testing.T) {
			t.Setenv("LINODEMCP_CONFIG_PATH", scenario.path)

			var stdout, stderr bytes.Buffer

			code := cli.RunProfileCommand(scenario.args, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}

			wantContains(t, "stderr", stderr.String(), loadConfigError)
			wantContains(t, "stderr", stderr.String(), scenario.path)
		})
	}
}

// TestProfileShowUnknownNameListsAvailableProfiles checks the recovery
// hint: an unknown name exits 1 and stderr lists every valid name.
func TestProfileShowUnknownNameListsAvailableProfiles(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeConfigYAML(t, profileCatalogYAML))

	var stdout, stderr bytes.Buffer

	code := cli.RunProfileCommand([]string{subverbShow, profileNoSuch}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}

	errOut := stderr.String()
	wantContains(t, "stderr", errOut, `profile "no-such" not found.`)
	wantContains(t, "stderr", errOut, "Available profiles:")
	wantContains(t, "stderr", errOut, "  compute-admin\n")
	wantContains(t, "stderr", errOut, "  yolo-ops\n")

	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want nothing printed for an unknown profile", stdout.String())
	}
}

// TestProfileShowPrintsEnvironmentRestriction checks the detail view
// joins a user profile's allowed environments and prints <all> for a
// built-in that has no restriction.
func TestProfileShowPrintsEnvironmentRestriction(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeConfigYAML(t, profileCatalogYAML))

	var restricted, unrestricted, stderr bytes.Buffer

	if code := cli.RunProfileCommand([]string{subverbShow, userProfileYolo}, &restricted, &stderr); code != 0 {
		t.Fatalf("show %s exit code = %d (stderr: %s), want 0", userProfileYolo, code, stderr.String())
	}

	wantContains(t, "restricted stdout", restricted.String(), "Profile: yolo-ops\n")
	wantContains(t, "restricted stdout", restricted.String(), "Allowed environments: default, staging\n")

	if code := cli.RunProfileCommand([]string{subverbShow, testProfileComputeAdmin}, &unrestricted, &stderr); code != 0 {
		t.Fatalf("show compute-admin exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	wantContains(t, "unrestricted stdout", unrestricted.String(), "Profile: compute-admin (active)\n")
	wantContains(t, "unrestricted stdout", unrestricted.String(), "Allowed environments: <all>\n")
}

// TestProfileCommandRoutesMutatorsToConfigPath drives every mutating verb
// through the dispatcher with no explicit path, the way main does, and
// checks each one lands in the file LINODEMCP_CONFIG_PATH names. The
// steps run in order because clone must precede delete.
func TestProfileCommandRoutesMutatorsToConfigPath(t *testing.T) {
	path := writeConfigYAML(t, baseConfigYAML)
	t.Setenv("LINODEMCP_CONFIG_PATH", path)

	steps := []struct {
		want string
		args []string
	}{
		{args: []string{subverbUse, profileReadonly}, want: "active profile switched to readonly-full\n"},
		{args: []string{subverbDisable, testProfileComputeAdmin}, want: "profile compute-admin disabled\n"},
		{args: []string{subverbEnable, testProfileComputeAdmin}, want: "profile compute-admin enabled\n"},
		{args: []string{subverbClone, testEnvKey, clonedProfile}, want: "profile mine cloned from default\n"},
		{args: []string{subverbDelete, clonedProfile}, want: "profile mine deleted\n"},
	}

	for _, step := range steps {
		var stdout, stderr bytes.Buffer

		code := cli.RunProfileCommand(step.args, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("%v exit code = %d (stderr: %s), want 0", step.args, code, stderr.String())
		}

		if stdout.String() != step.want {
			t.Fatalf("%v stdout = %q, want %q", step.args, stdout.String(), step.want)
		}
	}

	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}

	if reloaded.ActiveProfile != profileReadonly {
		t.Fatalf("ActiveProfile = %q, want %q", reloaded.ActiveProfile, profileReadonly)
	}

	if _, exists := reloaded.Profiles[clonedProfile]; exists {
		t.Fatal("cloned profile survived delete")
	}

	if override := reloaded.ProfilesBuiltinOverrides[testProfileComputeAdmin]; override.Disabled {
		t.Fatal("compute-admin still disabled after enable")
	}
}

// TestProfileMutatorsReportMissingConfig checks each mutating verb exits 1
// and names the path when the config file does not exist, so a typo in
// LINODEMCP_CONFIG_PATH never creates a config from nothing.
func TestProfileMutatorsReportMissingConfig(t *testing.T) {
	path := missingConfigPath(t)

	cases := []struct {
		name string
		args []string
	}{
		{name: subverbUse, args: []string{subverbUse, profileReadonly}},
		{name: subverbEnable, args: []string{subverbEnable, profileNetworkAd}},
		{name: subverbDisable, args: []string{subverbDisable, profileNetworkAd}},
		{name: subverbClone, args: []string{subverbClone, testEnvKey, clonedProfile}},
		{name: subverbDelete, args: []string{subverbDelete, clonedProfile}},
	}

	for _, scenario := range cases {
		t.Run(scenario.name, func(t *testing.T) {
			t.Setenv("LINODEMCP_CONFIG_PATH", path)

			var stdout, stderr bytes.Buffer

			code := cli.RunProfileCommand(scenario.args, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("exit code = %d (stderr: %s), want 1", code, stderr.String())
			}

			wantContains(t, "stderr", stderr.String(), loadConfigError)

			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("config path was created by a failed mutator: stat err = %v", err)
			}
		})
	}
}

// TestProfileUseReportsWriteFailure checks a mutator that loads fine but
// cannot rewrite the file exits 1 and names the path, leaving the
// original untouched. The directory is made read-only so the atomic
// temp-file write fails.
func TestProfileUseReportsWriteFailure(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions, so the write cannot be made to fail")
	}

	path := writeConfigYAML(t, baseConfigYAML)
	dir := filepath.Dir(path)

	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod read-only: %v", err)
	}

	// Restore write permission before TempDir's cleanup removes the tree.
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	var stdout, stderr bytes.Buffer

	code := cli.RunProfileUse([]string{profileReadonly}, path, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d (stdout: %s), want 1", code, stdout.String())
	}

	wantContains(t, "stderr", stderr.String(), "write config to")
	wantContains(t, "stderr", stderr.String(), path)

	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}

	if reloaded.ActiveProfile != "" {
		t.Fatalf("ActiveProfile = %q after a failed write, want unchanged", reloaded.ActiveProfile)
	}
}
