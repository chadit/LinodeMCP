package audit

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"
	"time"
)

// DefaultAuditRetentionDays is the default rotated-log retention
// window. Rotated audit-YYYY-MM-DD.log.gz files older than this many
// days are deleted by the sweeper. Matches the spec's "Retention"
// section and the SQLite default. A value of 0 disables deletion
// (keep forever).
const DefaultAuditRetentionDays = 14

// DefaultRetentionSweepInterval is how often the background sweeper
// runs after its initial pass. Hourly matches the SQLite retention
// cadence in the spec; rotated files only appear once per UTC day so
// a finer interval would just rescan the same set.
const DefaultRetentionSweepInterval = time.Hour

// rotatedFilePrefix and the log suffixes bracket a rotated file name
// of the form audit-YYYY-MM-DD.log or audit-YYYY-MM-DD.log.gz. The
// active ActiveLogFileName ("audit.log") does not carry the date
// segment, so it never matches and is never swept.
const rotatedFilePrefix = "audit-"

// RetentionSweeper deletes rotated audit logs older than a cutoff.
// It only touches files matching the audit-YYYY-MM-DD.log[.gz]
// pattern, so the active audit.log is never at risk. File operations
// are scoped to a *os.Root opened on the audit directory, keeping
// deletions inside the directory and out of gosec's traversal path.
type RetentionSweeper struct {
	dir           string
	retentionDays int
	interval      time.Duration
	clock         func() time.Time
	log           *slog.Logger
}

// RetentionSweeperOption configures a RetentionSweeper at
// construction. Concrete options: [WithSweepInterval],
// [WithSweepClock], [WithSweepLogger].
type RetentionSweeperOption func(*RetentionSweeper)

// WithSweepInterval overrides the background sweep cadence.
func WithSweepInterval(interval time.Duration) RetentionSweeperOption {
	return func(s *RetentionSweeper) {
		if interval > 0 {
			s.interval = interval
		}
	}
}

// WithSweepClock overrides the time source so tests can pin "now"
// and exercise the cutoff boundary deterministically.
func WithSweepClock(clock func() time.Time) RetentionSweeperOption {
	return func(s *RetentionSweeper) {
		if clock != nil {
			s.clock = clock
		}
	}
}

// WithSweepLogger overrides the structured logger used for the
// start, per-removal, and failure lines.
func WithSweepLogger(logger *slog.Logger) RetentionSweeperOption {
	return func(s *RetentionSweeper) {
		if logger != nil {
			s.log = logger
		}
	}
}

// NewRetentionSweeper builds a sweeper for dir with the given
// retention window in days. A retentionDays of 0 (or negative)
// disables deletion: Sweep becomes a no-op and Run still starts but
// never removes anything.
func NewRetentionSweeper(dir string, retentionDays int, opts ...RetentionSweeperOption) *RetentionSweeper {
	sweeper := &RetentionSweeper{
		dir:           dir,
		retentionDays: retentionDays,
		interval:      DefaultRetentionSweepInterval,
		clock:         func() time.Time { return time.Now().UTC() },
		log:           slog.Default(),
	}

	for _, opt := range opts {
		opt(sweeper)
	}

	return sweeper
}

// Sweep performs one retention pass. It deletes rotated files whose
// embedded date is strictly older than (now - retentionDays),
// comparing whole UTC days. Returns the number of files removed.
//
// A retentionDays of 0 or less disables deletion and returns
// (0, nil) without touching the directory. Per-file removal failures
// are logged and skipped; the returned error is non-nil only when the
// directory itself cannot be opened or listed.
func (s *RetentionSweeper) Sweep() (int, error) {
	if s.retentionDays <= 0 {
		return 0, nil
	}

	root, err := os.OpenRoot(s.dir)
	if err != nil {
		return 0, fmt.Errorf("audit: open retention root %s: %w", s.dir, err)
	}

	defer func() { _ = root.Close() }()

	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return 0, fmt.Errorf("audit: read retention dir %s: %w", s.dir, err)
	}

	cutoff := s.cutoffDay()

	var removed int

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		day, ok := parseRotatedFileDay(entry.Name())
		if !ok {
			continue
		}

		if !day.Before(cutoff) {
			continue
		}

		if err := root.Remove(entry.Name()); err != nil {
			s.log.Warn("audit retention: remove failed", "file", entry.Name(), "error", err.Error())

			continue
		}

		s.log.Info("audit retention: removed expired log", "file", entry.Name())

		removed++
	}

	return removed, nil
}

// Run sweeps once immediately, then on every interval tick until ctx
// is canceled. Intended to run in its own goroutine. Sweep failures
// are logged and do not stop the loop.
func (s *RetentionSweeper) Run(ctx context.Context) {
	s.log.Info(
		"audit retention sweeper started",
		"dir", s.dir,
		"retention_days", s.retentionDays,
		"interval", s.interval.String(),
	)

	s.runOnce()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce()
		}
	}
}

// runOnce performs a single Sweep and logs a directory-level failure.
// Per-file failures are already logged inside Sweep.
func (s *RetentionSweeper) runOnce() {
	if _, err := s.Sweep(); err != nil {
		s.log.Warn("audit retention sweep failed", "error", err.Error())
	}
}

// cutoffDay returns the UTC day boundary: rotated files dated strictly
// before this are expired. With retentionDays=14 and today=2026-05-19,
// the cutoff is 2026-05-05, so a file dated 2026-05-04 is removed and
// 2026-05-05 is kept.
func (s *RetentionSweeper) cutoffDay() time.Time {
	now := s.clock().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	return today.AddDate(0, 0, -s.retentionDays)
}

// parseRotatedFileDay extracts the UTC day from a rotated file name of
// the form audit-YYYY-MM-DD.log or audit-YYYY-MM-DD.log.gz. Returns
// (zero, false) for names that don't match, including the active
// audit.log, so the active file is never swept.
func parseRotatedFileDay(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, rotatedFilePrefix) {
		return time.Time{}, false
	}

	rest := strings.TrimPrefix(name, rotatedFilePrefix)

	switch {
	case strings.HasSuffix(rest, ".log.gz"):
		rest = strings.TrimSuffix(rest, ".log.gz")
	case strings.HasSuffix(rest, ".log"):
		rest = strings.TrimSuffix(rest, ".log")
	default:
		return time.Time{}, false
	}

	day, err := time.ParseInLocation("2006-01-02", rest, time.UTC)
	if err != nil {
		return time.Time{}, false
	}

	return day, true
}
