package audit

import (
	"path/filepath"

	"github.com/chadit/LinodeMCP/go/internal/config"
)

// Which store an audit query reads, and what happens when it fails.
//
// The policy lives here rather than beside the tools because it is the audit
// subsystem's own: a query surface must not change data source without saying
// so, and it must not go dark when a secondary index is broken.

// defaultDatabaseName is what the sink opens when the operator configured no
// path of its own.
const defaultDatabaseName = "audit.db"

// ResolveSQLitePath is the database path when the SQLite sink is enabled (the
// configured path, or audit.db beside the JSONL log), and empty otherwise.
//
// Empty is what selects the JSONL fallback inside the readers, so a disabled
// sink and an unset path read the same way.
func ResolveSQLitePath(cfg *config.Config) string {
	if cfg == nil || !cfg.Audit.SQLite.Enabled {
		return ""
	}

	if cfg.Audit.SQLite.Path != "" {
		return cfg.Audit.SQLite.Path
	}

	return filepath.Join(ResolveDefaultAuditDir(), defaultDatabaseName)
}

// ReadWithFallback reads through the configured store, or through the JSONL log
// with a warning when that store will not answer.
//
// read takes the store path, where empty means JSONL. The configured path is
// attempted first whatever it holds, and a failure with nothing configured is
// the answer, since a caller with no readable store at all has nothing to
// answer from.
func ReadWithFallback[Result any](
	sqlitePath string,
	read func(string) (Result, error),
) (Result, []string, error) {
	result, err := read(sqlitePath)
	if err == nil {
		return result, nil, nil
	}

	if sqlitePath == "" {
		return result, nil, err
	}

	fallback, fallbackErr := read("")
	if fallbackErr != nil {
		return fallback, nil, fallbackErr
	}

	return fallback, []string{StoreWarning(sqlitePath, err.Error())}, nil
}

// StoreWarning is the line an answer carries when it came from the JSONL log
// instead of the configured SQLite store.
func StoreWarning(path, reason string) string {
	return "audit sqlite store at " + path + " could not be read (" + reason +
		"); answered from the JSONL log instead"
}
