package audit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/chadit/LinodeMCP/go/internal/genlocal"
)

// jsonlHealth is the JSONL half of the report while it is being gathered. The
// answer it fills is built once with every member, so the halves are carried
// rather than assigned into it.
type jsonlHealth struct {
	oldestRotatedDate string
	diskBytes         int64
	rotatedFileCount  int
	activeLogExists   bool
}

// healthParts is one read of both sinks, before the store fallback has said
// whether it degraded.
type healthParts struct {
	sqlite *genlocal.AuditHealthSQLite
	jsonl  jsonlHealth
}

// droppedEvents is always zero: the sinks write synchronously, so there is no
// bounded channel to drop from. The member stays declared so the answer's shape
// survives a future async sink that does drop.
const droppedEvents = 0

// Health is the audit stores' own status, read through the configured database
// where one answers and through the JSONL log otherwise.
//
// A configured database that will not open leaves the JSONL half standing
// behind a warning; only a log nothing can read refuses. The policy sits here
// rather than beside the tool because it is the store's own degrade rule.
func Health(
	ctx context.Context, sqlitePath, jsonlDir string,
) (*genlocal.AuditHealthResponse, error) {
	parts, warnings, err := ReadWithFallback(sqlitePath,
		func(store string) (healthParts, error) {
			return collectHealth(ctx, store, jsonlDir)
		})
	if err != nil {
		return nil, err
	}

	return genlocal.NewAuditHealthResponse(
		filepath.Join(jsonlDir, ActiveLogFileName), parts.jsonl.activeLogExists,
		parts.jsonl.rotatedFileCount, parts.jsonl.oldestRotatedDate,
		parts.jsonl.diskBytes, droppedEvents, parts.sqlite, warnings,
	), nil
}

// collectHealth reads both sinks once. jsonlDir is always inspected (the JSONL
// sink is always on); sqlitePath is inspected only when non-empty. A missing
// JSONL directory is not a failure: the zero values report "nothing written
// yet".
func collectHealth(ctx context.Context, sqlitePath, jsonlDir string) (healthParts, error) {
	var parts healthParts

	if err := collectJSONLHealth(jsonlDir, &parts.jsonl); err != nil {
		return healthParts{}, err
	}

	if sqlitePath == "" {
		return parts, nil
	}

	sqliteHealth, err := collectSQLiteHealth(ctx, sqlitePath)
	if err != nil {
		return healthParts{}, err
	}

	parts.sqlite = sqliteHealth

	return parts, nil
}

// collectJSONLHealth fills the JSONL portion of the report: whether
// the active log exists, the rotated-file count and oldest date, and
// the total bytes of all audit files. A missing directory leaves the
// zero values in place.
func collectJSONLHealth(dir string, report *jsonlHealth) error {
	root, err := openReadRoot(dir)
	if err != nil {
		if errors.Is(err, errAuditDirMissing) {
			return nil
		}

		return err
	}

	defer func() { _ = root.Close() }()

	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return fmt.Errorf("audit: read health dir %s: %w", dir, err)
	}

	var oldestDate string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		report.diskBytes += fileSize(root, name)

		if name == ActiveLogFileName {
			report.activeLogExists = true

			continue
		}

		day, ok := parseRotatedFileDay(name)
		if !ok {
			continue
		}

		report.rotatedFileCount++

		dateStr := day.Format("2006-01-02")
		if oldestDate == "" || dateStr < oldestDate {
			oldestDate = dateStr
		}
	}

	report.oldestRotatedDate = oldestDate

	return nil
}

// fileSize returns the byte size of name within root, or 0 if it can't
// be stat'd (the file is counted as contributing nothing rather than
// failing the whole report).
func fileSize(root *os.Root, name string) int64 {
	info, err := fs.Stat(root.FS(), name)
	if err != nil {
		return 0
	}

	return info.Size()
}

// collectSQLiteHealth queries the row count and oldest timestamp from
// the SQLite store and stats the database file size.
func collectSQLiteHealth(ctx context.Context, path string) (*genlocal.AuditHealthSQLite, error) {
	db, err := sql.Open(sqliteDriverName, "file:"+path)
	if err != nil {
		return nil, fmt.Errorf("audit: open sqlite %s: %w", path, err)
	}

	defer func() { _ = db.Close() }()

	var eventCount, oldestEventUnixNS int64

	row := db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(MIN(ts_unix_ns), 0) FROM events`)
	if err := row.Scan(&eventCount, &oldestEventUnixNS); err != nil {
		return nil, fmt.Errorf("audit: sqlite health query: %w", err)
	}

	return genlocal.NewAuditHealthSQLite(path, eventCount, oldestEventUnixNS, statSize(path)), nil
}

// statSize returns the byte size of the file at path, or 0 if it can't
// be stat'd.
func statSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}

	return info.Size()
}
