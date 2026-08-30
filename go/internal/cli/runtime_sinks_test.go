package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/cli"
)

const (
	sqliteEnabledYAML = baseConfigYAML + `
audit:
  sqlite:
    enabled: true
`
	unknownProfileYAML = baseConfigYAML + `
active_profile: "no-such"
`
)

// TestCallContinuesWithoutAuditWhenSinkDirIsAFile checks a call still runs
// when the audit directory cannot be created because a plain file sits at
// its path: the sink failure is reported on stderr and the tool output
// still arrives with exit 0.
func TestCallContinuesWithoutAuditWhenSinkDirIsAFile(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", stateHome)

	if err := os.WriteFile(filepath.Join(stateHome, "linodemcp"), []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("plant file at audit dir path: %v", err)
	}

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand([]string{toolVersion}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	wantContains(t, "stderr", stderr.String(), "audit JSONL sink unavailable; continuing without audit")
	wantContains(t, "stdout", stdout.String(), `"version"`)
}

// TestCallOpensSQLiteSinkWhenEnabled checks audit.sqlite.enabled makes a
// call create audit.db beside the JSONL log, so a later SQLite-backed
// query has a database to read.
func TestCallOpensSQLiteSinkWhenEnabled(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("LINODEMCP_CONFIG_PATH", writeConfigYAML(t, sqliteEnabledYAML))
	t.Setenv("XDG_STATE_HOME", stateHome)

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand([]string{toolVersion}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	dbPath := filepath.Join(stateHome, "linodemcp", "audit.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("audit.db not created beside the JSONL log: %v", err)
	}

	wantNotContains(t, "stderr", stderr.String(), "SQLite sink unavailable")
}

// TestCallContinuesWithJSONLWhenSQLitePathUnwritable checks a SQLite path
// inside a directory that does not exist degrades to JSONL only: the
// call succeeds, stderr says so, and the JSONL log still records it.
func TestCallContinuesWithJSONLWhenSQLitePathUnwritable(t *testing.T) {
	stateHome := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "missing-dir", "audit.db")

	t.Setenv("LINODEMCP_CONFIG_PATH", writeConfigYAML(t, sqliteEnabledYAML+"    path: \""+dbPath+"\"\n"))
	t.Setenv("XDG_STATE_HOME", stateHome)

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand([]string{toolVersion}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	wantContains(t, "stderr", stderr.String(), "audit SQLite sink unavailable; continuing with JSONL only")

	info, err := os.Stat(auditLogPath(stateHome))
	if err != nil {
		t.Fatalf("JSONL log missing after SQLite fallback: %v", err)
	}

	if info.Size() == 0 {
		t.Fatal("JSONL log is empty; the call was not recorded")
	}
}

// TestCommandsFailOnConfigErrors checks every runtime-backed command exits
// 1 and explains why when the config is malformed or names an active
// profile that does not exist, rather than falling back to defaults the
// way a missing file does.
func TestCommandsFailOnConfigErrors(t *testing.T) {
	configs := []struct {
		name string
		yaml string
		want string
	}{
		{name: "malformed yaml", yaml: malformedConfigYAML, want: loadConfigError},
		{name: "unknown active profile", yaml: unknownProfileYAML, want: "active profile not found"},
	}

	commands := []struct {
		run  func(stdout, stderr *bytes.Buffer) int
		name string
	}{
		{name: "call version", run: func(stdout, stderr *bytes.Buffer) int {
			return cli.RunCallCommand([]string{toolVersion}, stdout, stderr)
		}},
		{name: "tools list", run: func(stdout, stderr *bytes.Buffer) int {
			return cli.RunToolsCommand(nil, stdout, stderr)
		}},
		{name: "tools show", run: func(stdout, stderr *bytes.Buffer) int {
			return cli.RunToolsCommand([]string{subverbShow, toolHello}, stdout, stderr)
		}},
		{name: "audit health", run: func(stdout, stderr *bytes.Buffer) int {
			return cli.RunAuditCommand([]string{subverbHealth}, stdout, stderr)
		}},
	}

	for _, cfg := range configs {
		t.Run(cfg.name, func(t *testing.T) {
			t.Setenv("LINODEMCP_CONFIG_PATH", writeConfigYAML(t, cfg.yaml))
			t.Setenv("XDG_STATE_HOME", t.TempDir())

			for _, command := range commands {
				var stdout, stderr bytes.Buffer

				if code := command.run(&stdout, &stderr); code != 1 {
					t.Fatalf("%s exit code = %d (stderr: %s), want 1", command.name, code, stderr.String())
				}

				wantContains(t, command.name+" stderr", stderr.String(), cfg.want)
			}
		})
	}
}
