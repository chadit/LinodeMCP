package audit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// HealthReport is the audit subsystem's self-reported status, returned
// by the linode_audit_health tool. DroppedEvents is always zero today:
// the sinks write synchronously, so there is no bounded channel to
// drop from. The field exists so the wire shape is stable if a future
// async sink adds drop accounting.
type HealthReport struct {
	JSONLPath         string        `json:"jsonl_path"`
	ActiveLogExists   bool          `json:"active_log_exists"`
	RotatedFileCount  int           `json:"rotated_file_count"`
	OldestRotatedDate string        `json:"oldest_rotated_date"`
	DiskBytes         int64         `json:"disk_bytes"`
	DroppedEvents     int64         `json:"dropped_events"`
	SQLite            *SQLiteHealth `json:"sqlite"`
}

// SQLiteHealth is the SQLite-sink portion of the health report,
// present only when the SQLite sink is enabled.
type SQLiteHealth struct {
	Path              string `json:"path"`
	EventCount        int64  `json:"event_count"`
	OldestEventUnixNS int64  `json:"oldest_event_unix_ns"`
	DBBytes           int64  `json:"db_bytes"`
}

// CollectHealth gathers the audit subsystem status. jsonlDir is always
// inspected (the JSONL sink is always on); sqlitePath is inspected
// only when non-empty (SQLite sink enabled). A missing JSONL directory
// is not an error: the zero-valued JSONL fields report "nothing
// written yet".
func CollectHealth(ctx context.Context, sqlitePath, jsonlDir string) (HealthReport, error) {
	report := HealthReport{
		JSONLPath: filepath.Join(jsonlDir, ActiveLogFileName),
	}

	if err := collectJSONLHealth(jsonlDir, &report); err != nil {
		return HealthReport{}, err
	}

	if sqlitePath != "" {
		sqliteHealth, err := collectSQLiteHealth(ctx, sqlitePath)
		if err != nil {
			return HealthReport{}, err
		}

		report.SQLite = sqliteHealth
	}

	return report, nil
}

// collectJSONLHealth fills the JSONL portion of the report: whether
// the active log exists, the rotated-file count and oldest date, and
// the total bytes of all audit files. A missing directory leaves the
// zero values in place.
func collectJSONLHealth(dir string, report *HealthReport) error {
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
		report.DiskBytes += fileSize(root, name)

		if name == ActiveLogFileName {
			report.ActiveLogExists = true

			continue
		}

		day, ok := parseRotatedFileDay(name)
		if !ok {
			continue
		}

		report.RotatedFileCount++

		dateStr := day.Format("2006-01-02")
		if oldestDate == "" || dateStr < oldestDate {
			oldestDate = dateStr
		}
	}

	report.OldestRotatedDate = oldestDate

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
func collectSQLiteHealth(ctx context.Context, path string) (*SQLiteHealth, error) {
	db, err := sql.Open(sqliteDriverName, "file:"+path)
	if err != nil {
		return nil, fmt.Errorf("audit: open sqlite %s: %w", path, err)
	}

	defer func() { _ = db.Close() }()

	health := &SQLiteHealth{Path: path}

	row := db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(MIN(ts_unix_ns), 0) FROM events`)
	if err := row.Scan(&health.EventCount, &health.OldestEventUnixNS); err != nil {
		return nil, fmt.Errorf("audit: sqlite health query: %w", err)
	}

	health.DBBytes = statSize(path)

	return health, nil
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
