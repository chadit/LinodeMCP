package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/cli"
)

// malformedConfigPath writes a config file that is not valid YAML and
// returns its path, so a command run against it hits the real config
// error rather than the missing-file fallback.
func malformedConfigPath(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("server: [unclosed\n"), 0o600); err != nil {
		t.Fatalf("write malformed config: %v", err)
	}

	return path
}

// TestRunTUICommandReportsMalformedConfig checks the TUI refuses to start
// on a config file that does not parse: it exits 1 and names the file in
// the diagnostic instead of falling back to the in-memory default the way
// a missing file does. Nothing is drawn to the terminal stream, because
// the failure happens before the program is built.
func TestRunTUICommandReportsMalformedConfig(t *testing.T) {
	path := malformedConfigPath(t)
	t.Setenv("LINODEMCP_CONFIG_PATH", path)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var out, errOut bytes.Buffer

	code := cli.RunTUICommand(&out, &errOut)
	if code != 1 {
		t.Fatalf("exit code = %d (stderr: %s), want 1", code, errOut.String())
	}

	wantContains(t, "stderr", errOut.String(), "load config from")
	wantContains(t, "stderr", errOut.String(), path)
	wantContains(t, "stderr", errOut.String(), "malformed")

	if out.Len() != 0 {
		t.Errorf("TUI drew to the terminal before failing on config: %q", out.String())
	}
}

// TestRunTUICommandFailsWithoutControllingTerminal checks a session with
// no terminal to read keys from exits 1 with a "tui error" diagnostic
// rather than hanging. Bubble Tea falls back to /dev/tty when stdin is not
// a terminal, so this only runs where that fallback also fails; in an
// interactive shell the program would wait on the developer's keyboard,
// so the test skips there.
func TestRunTUICommandFailsWithoutControllingTerminal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the /dev/tty probe has no Windows equivalent")
	}

	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		_ = tty.Close()

		t.Skip("a controlling terminal is attached; the TUI would read from it")
	}

	t.Setenv("LINODEMCP_CONFIG_PATH", missingConfigPath(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var out, errOut bytes.Buffer

	code := cli.RunTUICommand(&out, &errOut)
	if code != 1 {
		t.Fatalf("exit code = %d (stderr: %s), want 1", code, errOut.String())
	}

	wantContains(t, "stderr", errOut.String(), "tui error:")
	wantContains(t, "stderr", errOut.String(), "TTY")
}
