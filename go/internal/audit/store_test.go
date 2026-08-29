package audit_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
)

// storeFailure is what a store that will not answer reports. Declared here
// because both fallback cases raise it and neither can use a real store: the
// point is the route the policy takes, not what broke.
var errStoreUnreadable = errors.New("store unreadable")

// TestResolveSQLitePathAnswersEmptyWhenTheSinkIsOff covers the value that
// selects the JSONL log inside the readers, which a disabled sink and an unset
// path have to reach alike.
func TestResolveSQLitePathAnswersEmptyWhenTheSinkIsOff(t *testing.T) {
	t.Parallel()

	cases := []struct {
		cfg  *config.Config
		name string
	}{
		{name: "no configuration at all", cfg: nil},
		{name: "the sink disabled", cfg: &config.Config{}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := audit.ResolveSQLitePath(testCase.cfg); got != "" {
				t.Errorf("path = %q, want empty", got)
			}
		})
	}
}

// TestResolveSQLitePathTakesTheConfiguredPath covers the operator's own path,
// which the sink opens ahead of the default.
func TestResolveSQLitePathTakesTheConfiguredPath(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Audit.SQLite.Enabled = true
	cfg.Audit.SQLite.Path = filepath.Join(t.TempDir(), "chosen.db")

	if got := audit.ResolveSQLitePath(cfg); got != cfg.Audit.SQLite.Path {
		t.Errorf("path = %q, want %q", got, cfg.Audit.SQLite.Path)
	}
}

// TestResolveSQLitePathFallsBackToTheDefaultName covers an enabled sink with no
// path, which lands beside the JSONL log rather than nowhere.
func TestResolveSQLitePathFallsBackToTheDefaultName(t *testing.T) {
	// No t.Parallel: the default directory is read off the environment.
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	cfg := &config.Config{}
	cfg.Audit.SQLite.Enabled = true

	want := filepath.Join(audit.ResolveDefaultAuditDir(), "audit.db")
	if got := audit.ResolveSQLitePath(cfg); got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

// TestReadWithFallbackAnswersTheConfiguredStore covers the ordinary route: the
// configured store answers and the caller is told nothing.
func TestReadWithFallbackAnswersTheConfiguredStore(t *testing.T) {
	t.Parallel()

	var reached []string

	result, warnings, err := audit.ReadWithFallback("store.db",
		func(store string) (string, error) {
			reached = append(reached, store)

			return "from " + store, nil
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "from store.db" {
		t.Errorf("result = %q, want the configured store's answer", result)
	}

	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}

	if len(reached) != 1 {
		t.Errorf("reads = %v, want the configured store alone", reached)
	}
}

// TestReadWithFallbackDegradesToTheLogAndSaysSo is the whole reason the policy
// exists: an audit surface may not change data source in silence.
func TestReadWithFallbackDegradesToTheLogAndSaysSo(t *testing.T) {
	t.Parallel()

	var reached []string

	result, warnings, err := audit.ReadWithFallback("store.db",
		func(store string) (string, error) {
			reached = append(reached, store)

			if store != "" {
				return "", errStoreUnreadable
			}

			return "from the log", nil
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "from the log" {
		t.Errorf("result = %q, want the log's answer", result)
	}

	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want one", warnings)
	}

	if !strings.Contains(warnings[0], "store.db") ||
		!strings.Contains(warnings[0], errStoreUnreadable.Error()) {
		t.Errorf("warning names neither the path nor the reason: %q", warnings[0])
	}

	if len(reached) != 2 || reached[1] != "" {
		t.Errorf("reads = %v, want the store then the log", reached)
	}
}

// TestReadWithFallbackReportsAFailureWithNothingConfigured covers the case with
// nowhere left to degrade to, where the failure is the answer.
func TestReadWithFallbackReportsAFailureWithNothingConfigured(t *testing.T) {
	t.Parallel()

	var reads int

	_, warnings, err := audit.ReadWithFallback("",
		func(string) (string, error) {
			reads++

			return "", errStoreUnreadable
		})

	if !errors.Is(err, errStoreUnreadable) {
		t.Fatalf("err = %v, want %v", err, errStoreUnreadable)
	}

	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}

	if reads != 1 {
		t.Errorf("reads = %d, want the log attempted once", reads)
	}
}

// TestReadWithFallbackReportsALogThatFailsToo covers the store failing and the
// log failing behind it, where the log's own failure is what a caller reads.
func TestReadWithFallbackReportsALogThatFailsToo(t *testing.T) {
	t.Parallel()

	logFailure := os.ErrPermission

	_, warnings, err := audit.ReadWithFallback("store.db",
		func(store string) (string, error) {
			if store != "" {
				return "", errStoreUnreadable
			}

			return "", logFailure
		})

	if !errors.Is(err, logFailure) {
		t.Fatalf("err = %v, want the log's own failure %v", err, logFailure)
	}

	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none beside a reported failure", warnings)
	}
}
