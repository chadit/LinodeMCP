package audit_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/internal/audit"
)

// writeRotatedFile creates a rotated-log file in dir so the sweeper
// has something to find. Content is irrelevant; the sweeper keys off
// the name.
func writeRotatedFile(t *testing.T, dir, name string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("x\n"), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return path
}

// fixedSweepClock returns a clock pinned to one instant.
func fixedSweepClock(now time.Time) func() time.Time {
	return func() time.Time { return now }
}

// TestRetentionSweepRemovesExpiredKeepsRecent verifies the cutoff
// boundary: with a 14-day window and "now" fixed, files dated before
// the cutoff are deleted, the cutoff day itself is kept, recent files
// are kept, and the active audit.log is never touched.
func TestRetentionSweepRemovesExpiredKeepsRecent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	now := time.Date(2026, time.May, 19, 12, 0, 0, 0, time.UTC)

	// cutoff = 2026-05-05; strictly-before is expired.
	expiredGz := writeRotatedFile(t, dir, "audit-2026-05-04.log.gz")
	expiredPlain := writeRotatedFile(t, dir, "audit-2026-05-01.log")
	cutoffDay := writeRotatedFile(t, dir, "audit-2026-05-05.log.gz")
	recent := writeRotatedFile(t, dir, "audit-2026-05-18.log.gz")
	active := writeRotatedFile(t, dir, "audit.log")

	sweeper := audit.NewRetentionSweeper(
		dir, 14,
		audit.WithSweepClock(fixedSweepClock(now)),
	)

	removed, err := sweeper.Sweep()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if removed != 2 {
		t.Errorf("removed = %v, want %v", removed, 2)
	}

	if _, statErr := os.Stat(expiredGz); statErr == nil {
		t.Errorf("file %v should not exist", expiredGz)
	}

	if _, statErr := os.Stat(expiredPlain); statErr == nil {
		t.Errorf("file %v should not exist", expiredPlain)
	}

	if _, statErr := os.Stat(cutoffDay); statErr != nil {
		t.Errorf("file %v does not exist: %v", cutoffDay, statErr)
	}

	if _, statErr := os.Stat(recent); statErr != nil {
		t.Errorf("file %v does not exist: %v", recent, statErr)
	}

	if _, statErr := os.Stat(active); statErr != nil {
		t.Errorf("file %v does not exist: %v", active, statErr)
	}
}

// TestRetentionSweepDisabledWhenZero verifies retentionDays<=0 is a
// no-op: nothing is removed even when expired files exist.
func TestRetentionSweepDisabledWhenZero(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	now := time.Date(2026, time.May, 19, 12, 0, 0, 0, time.UTC)

	old := writeRotatedFile(t, dir, "audit-2020-01-01.log.gz")

	sweeper := audit.NewRetentionSweeper(
		dir, 0,
		audit.WithSweepClock(fixedSweepClock(now)),
	)

	removed, err := sweeper.Sweep()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if removed != 0 {
		t.Errorf("removed = %v, want %v", removed, 0)
	}

	if _, statErr := os.Stat(old); statErr != nil {
		t.Errorf("file %v does not exist: %v", old, statErr)
	}
}

// TestRetentionSweepIgnoresUnrelatedFiles verifies the sweeper only
// touches files matching audit-YYYY-MM-DD.log[.gz] and leaves
// everything else alone, even when very old or oddly named.
func TestRetentionSweepIgnoresUnrelatedFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	now := time.Date(2026, time.May, 19, 12, 0, 0, 0, time.UTC)

	keepers := []string{
		"audit.log",               // active file
		"audit-not-a-date.log",    // prefix but unparseable date
		"audit-2026-05-04.txt",    // right date, wrong suffix
		"README.md",               // unrelated
		"audit-2026-13-99.log.gz", // prefix + suffix but invalid date
	}

	paths := make([]string, 0, len(keepers))
	for _, name := range keepers {
		paths = append(paths, writeRotatedFile(t, dir, name))
	}

	sweeper := audit.NewRetentionSweeper(
		dir, 14,
		audit.WithSweepClock(fixedSweepClock(now)),
	)

	removed, err := sweeper.Sweep()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if removed != 0 {
		t.Errorf("removed = %v, want %v", removed, 0)
	}

	for _, path := range paths {
		if _, statErr := os.Stat(path); statErr != nil {
			t.Errorf("file %v does not exist: %v", path, statErr)
		}
	}
}

// TestRetentionSweepMissingDirErrors verifies a sweep against a
// non-existent directory returns an error (the caller logs it; Run
// keeps looping).
func TestRetentionSweepMissingDirErrors(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "does-not-exist")
	now := time.Date(2026, time.May, 19, 12, 0, 0, 0, time.UTC)

	sweeper := audit.NewRetentionSweeper(
		missing, 14,
		audit.WithSweepClock(fixedSweepClock(now)),
	)

	removed, err := sweeper.Sweep()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if removed != 0 {
		t.Errorf("removed = %v, want %v", removed, 0)
	}
}
