package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	// Pure-Go SQLite driver. Registers itself under the name "sqlite"
	// (NOT "sqlite3" like the CGO mattn driver), which is why sql.Open
	// below uses "sqlite". Pure-Go is mandated by the release matrix's
	// CGO_ENABLED=0 commitment for the Windows build.
	_ "modernc.org/sqlite"
)

// sqliteDriverName is the driver name modernc.org/sqlite registers.
// Spelled out as a constant because the "sqlite3" mistake (the CGO
// driver's name) is the single most common footgun with this library.
const sqliteDriverName = "sqlite"

// createSchema is the idempotent DDL run at sink open. The wire shape
// matches the spec's SQLite section: args and the redacted-key list
// are stored as JSON text columns; ts is represented only as
// ts_unix_ns (the human ISO form lives in the JSONL sink).
const createSchema = `
CREATE TABLE IF NOT EXISTS events (
    event_id TEXT PRIMARY KEY,
    ts_unix_ns INTEGER NOT NULL,
    tool TEXT NOT NULL,
    tool_capability TEXT NOT NULL,
    environment TEXT NOT NULL,
    profile TEXT NOT NULL,
    mode TEXT NOT NULL,
    plan_id TEXT,
    status TEXT NOT NULL,
    latency_ms INTEGER NOT NULL,
    result_summary TEXT,
    error TEXT,
    linodemcp_version TEXT NOT NULL,
    session_id TEXT NOT NULL,
    credential_generation INTEGER NOT NULL,
    args_json TEXT NOT NULL,
    args_redacted_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_events_ts ON events(ts_unix_ns DESC);
CREATE INDEX IF NOT EXISTS idx_events_tool ON events(tool, ts_unix_ns DESC);
CREATE INDEX IF NOT EXISTS idx_events_profile ON events(profile, ts_unix_ns DESC);
CREATE INDEX IF NOT EXISTS idx_events_status ON events(status, ts_unix_ns DESC);
CREATE INDEX IF NOT EXISTS idx_events_credential_generation ON events(credential_generation, ts_unix_ns DESC);
`

// insertEvent is the parameterized insert run per Write. Column order
// matches the bind arguments in SQLiteSink.Write.
const insertEvent = `
INSERT OR IGNORE INTO events (
    event_id, ts_unix_ns, tool, tool_capability, environment, profile,
    mode, plan_id, status, latency_ms, result_summary, error,
    linodemcp_version, session_id, credential_generation,
    args_json, args_redacted_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

// SQLiteSink writes audit events to a SQLite database. Opt-in via the
// audit.sqlite config block; when enabled it runs alongside the JSONL
// sink behind a MultiSink. Writes are synchronous, matching the JSONL
// sink; the Performance section's 100ms batching is a later
// optimization if benchmarks require it.
//
// Write failures are best-effort: they route to the configured error
// handler and are dropped, leaving the JSONL sink as the durable
// record (per the spec's "If SQLite write fails, JSONL is the durable
// record").
type SQLiteSink struct {
	db         *sql.DB
	onWriteErr func(error)
}

// NewSQLiteSink opens (creating if needed) a SQLite database at path,
// applies the busy timeout, and ensures the schema exists. The
// busy_timeout is set via the modernc _pragma DSN parameter so it
// applies to every pooled connection without a separate Exec.
//
// ctx is used only for the one-time schema creation here; per-event
// inserts get their context from the Sink.Write parameter (the
// middleware passes a cancellation-detached context).
func NewSQLiteSink(ctx context.Context, path string, busyTimeoutMS int) (*SQLiteSink, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(%d)", path, busyTimeoutMS)

	db, err := sql.Open(sqliteDriverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("audit: open sqlite %s: %w", path, err)
	}

	if _, err := db.ExecContext(ctx, createSchema); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("audit: create sqlite schema: %w", err)
	}

	return &SQLiteSink{db: db, onWriteErr: defaultSQLiteWriteErrorHandler}, nil
}

// Write implements the Sink interface. INSERT OR IGNORE makes a
// duplicate event_id a no-op rather than an error, so a fan-out that
// re-delivers the same event (or a retry) stays idempotent. Marshal
// and insert failures route to the error handler and drop the event.
func (s *SQLiteSink) Write(ctx context.Context, event *Event) {
	argsJSON, err := json.Marshal(emptyMapIfNil(event.Args))
	if err != nil {
		s.onWriteErr(fmt.Errorf("audit: marshal args: %w", err))

		return
	}

	redactedJSON, err := json.Marshal(emptySliceIfNil(event.ArgsRedacted))
	if err != nil {
		s.onWriteErr(fmt.Errorf("audit: marshal args_redacted: %w", err))

		return
	}

	if _, err := s.db.ExecContext(
		ctx,
		insertEvent,
		event.EventID, event.TSUnixNS, event.Tool, string(event.ToolCapability),
		event.Environment, event.Profile, string(event.Mode), nullableString(event.PlanID),
		string(event.Status), event.LatencyMS, event.ResultSummary, nullableString(event.Error),
		event.LinodemcpVersion, event.SessionID, event.CredentialGeneration,
		string(argsJSON), string(redactedJSON),
	); err != nil {
		s.onWriteErr(fmt.Errorf("audit: sqlite insert: %w", err))
	}
}

// Close closes the underlying database handle.
func (s *SQLiteSink) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("audit: close sqlite: %w", err)
	}

	return nil
}

// SweepRetention deletes events older than retentionDays before now
// and returns the number of rows removed. A retentionDays of 0 or
// less disables deletion (keep forever) and returns (0, nil) without
// touching the table. The cutoff is exact (now minus N days), unlike
// the JSONL sweeper's whole-day file boundaries.
func (s *SQLiteSink) SweepRetention(ctx context.Context, now time.Time, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}

	cutoff := now.UTC().AddDate(0, 0, -retentionDays).UnixNano()

	result, err := s.db.ExecContext(ctx, `DELETE FROM events WHERE ts_unix_ns < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("audit: sqlite retention delete: %w", err)
	}

	removed, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("audit: sqlite rows affected: %w", err)
	}

	return removed, nil
}

// RunRetention sweeps once immediately, then on every interval tick
// until ctx is canceled. Intended to run in its own goroutine. Sweep
// failures are logged and do not stop the loop.
func (s *SQLiteSink) RunRetention(ctx context.Context, retentionDays int, interval time.Duration, log *slog.Logger) {
	log.Info("audit sqlite retention started", "retention_days", retentionDays, "interval", interval.String())
	s.runRetentionOnce(ctx, retentionDays, log)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runRetentionOnce(ctx, retentionDays, log)
		}
	}
}

// DB exposes the underlying handle for the Phase 3d/3e query tools,
// which run their own statements against the same database.
func (s *SQLiteSink) DB() *sql.DB {
	return s.db
}

// runRetentionOnce performs a single sweep, logging the row count or a
// failure. Placed after the exported methods per funcorder.
func (s *SQLiteSink) runRetentionOnce(ctx context.Context, retentionDays int, log *slog.Logger) {
	removed, err := s.SweepRetention(ctx, time.Now(), retentionDays)
	if err != nil {
		log.Warn("audit sqlite retention sweep failed", "error", err.Error())

		return
	}

	if removed > 0 {
		log.Info("audit sqlite retention removed expired rows", "rows", removed)
	}
}

// nullableString maps a nil *string to a SQL NULL and a non-nil
// pointer to its value. database/sql does not accept a raw *string,
// so the conversion to an any (nil or string) happens here.
func nullableString(value *string) any {
	if value == nil {
		return nil
	}

	return *value
}

// emptyMapIfNil normalizes a nil args map to an empty map so the JSON
// column stores "{}" rather than "null", matching the JSONL sink's
// MarshalJSON behavior.
func emptyMapIfNil(args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}

	return args
}

// emptySliceIfNil normalizes a nil redacted-keys slice to an empty
// slice so the JSON column stores "[]" rather than "null".
func emptySliceIfNil(keys []string) []string {
	if keys == nil {
		return []string{}
	}

	return keys
}

// defaultSQLiteWriteErrorHandler logs SQLite write failures at warn.
// SQLite is the secondary sink; a drop here is survivable because the
// JSONL sink holds the durable record.
func defaultSQLiteWriteErrorHandler(err error) {
	slog.Warn("audit sqlite sink", "error", err.Error())
}
