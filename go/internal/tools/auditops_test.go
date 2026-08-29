package tools_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/genlocal"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The audit export operation at the surface the generated arm calls.
//
// The handler cases beside these read the sentence a caller sees, which the
// tool declares. What they cannot see is the condition the operation reports,
// and that is the whole of what the emitted errors.Is ladder walks: an
// operation reporting the wrong one answers the wrong sentence with nothing
// failing. These read it directly, and they reach the cap, the filters and the
// store fallback, which no behavior fixture does.

// seedExportLog writes the audit log a case reads under the state home it was
// handed, and answers the directory the readers resolve to.
//
// The environment the readers and the temp file resolve from stays in each
// case's own body: a test writing it cannot run in parallel, and a helper
// hiding the write reads as one that could.
func seedExportLog(t *testing.T, stateHome string, events []*audit.Event) string {
	t.Helper()

	auditDir := filepath.Join(stateHome, "linodemcp")
	if err := os.MkdirAll(auditDir, 0o750); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), events)

	return auditDir
}

// TestAuditExportAnswersTheWindowItWrote covers the ordinary path: every seeded
// event reaches the file and the answer names where it landed.
func TestAuditExportAnswersTheWindowItWrote(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("TMPDIR", t.TempDir())

	seedExportLog(t, stateHome, []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
		auditEvent("linode_volume_list", audit.CapabilityRead, audit.StatusSuccess, 2),
	})

	answer, err := tools.AuditExport(t.Context(), &config.Config{},
		audit.ExportFormatNDJSON, time.Time{}, time.Time{}, "", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() { _ = os.Remove(answer.Path) })

	if answer.RecordCount != 2 {
		t.Errorf("answer.RecordCount = %d, want %d", answer.RecordCount, 2)
	}

	if answer.Format != audit.ExportFormatNDJSON {
		t.Errorf("answer.Format = %q, want %q", answer.Format, audit.ExportFormatNDJSON)
	}

	if len(answer.Warnings) != 0 {
		t.Errorf("answer.Warnings = %v, want none", answer.Warnings)
	}
}

// TestAuditExportCapsTheRecordsItReads covers the bound that keeps an unbounded
// range out of memory, which nothing else exercises: the tool's own schema
// never sends a cap in any fixture.
func TestAuditExportCapsTheRecordsItReads(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("TMPDIR", t.TempDir())

	seedExportLog(t, stateHome, []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
		auditEvent("linode_volume_list", audit.CapabilityRead, audit.StatusSuccess, 2),
		auditEvent("linode_domain_list", audit.CapabilityRead, audit.StatusSuccess, 3),
	})

	answer, err := tools.AuditExport(t.Context(), &config.Config{},
		audit.ExportFormatNDJSON, time.Time{}, time.Time{}, "", 1, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() { _ = os.Remove(answer.Path) })

	if answer.RecordCount != 1 {
		t.Errorf("answer.RecordCount = %d, want the requested cap of %d", answer.RecordCount, 1)
	}
}

// TestAuditExportFiltersByTheBoundsItIsHanded covers the four filters reaching
// the query, which a fixture sending no filter cannot tell apart from a query
// that dropped them.
func TestAuditExportFiltersByTheBoundsItIsHanded(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("TMPDIR", t.TempDir())

	seedExportLog(t, stateHome, []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
		auditEvent("linode_volume_list", audit.CapabilityRead, audit.StatusSuccess, 2),
		auditEvent("linode_instance_boot", audit.CapabilityWrite, audit.StatusSuccess, 3),
	})

	window := time.Date(2026, time.May, 20, 0, 0, 2, 0, time.UTC)

	answer, err := tools.AuditExport(t.Context(), &config.Config{},
		audit.ExportFormatNDJSON, window, window.Add(time.Minute), "linode_instance_*", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() { _ = os.Remove(answer.Path) })

	if answer.RecordCount != 1 {
		t.Errorf("answer.RecordCount = %d, want the one event inside both bounds", answer.RecordCount)
	}
}

// TestAuditExportDegradesToTheLogAndSaysSo covers the store fallback through
// the operation, which is where the warning a caller reads comes from.
func TestAuditExportDegradesToTheLogAndSaysSo(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("TMPDIR", t.TempDir())

	auditDir := seedExportLog(t, stateHome, []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
	})

	unreadable := filepath.Join(auditDir, "not-a-database.db")
	if err := os.WriteFile(unreadable, []byte("this is not a sqlite file"), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := &config.Config{}
	cfg.Audit.SQLite.Enabled = true
	cfg.Audit.SQLite.Path = unreadable

	answer, err := tools.AuditExport(t.Context(), cfg,
		audit.ExportFormatNDJSON, time.Time{}, time.Time{}, "", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() { _ = os.Remove(answer.Path) })

	if answer.RecordCount != 1 {
		t.Errorf("answer.RecordCount = %d, want the log's own event", answer.RecordCount)
	}

	if len(answer.Warnings) != 1 {
		t.Fatalf("answer.Warnings = %v, want the fallback to say so", answer.Warnings)
	}

	if !strings.Contains(answer.Warnings[0], unreadable) {
		t.Errorf("the warning does not name the configured store: %q", answer.Warnings[0])
	}
}

// TestAuditExportReportsAStoreThatWillNotRead covers the read condition, which
// the arm maps onto the sentence the tool declares.
func TestAuditExportReportsAStoreThatWillNotRead(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("TMPDIR", t.TempDir())

	auditDir := seedExportLog(t, stateHome, []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
	})

	if err := os.Chmod(auditDir, 0o000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() { _ = os.Chmod(auditDir, 0o750) })

	answer, err := tools.AuditExport(t.Context(), &config.Config{},
		audit.ExportFormatNDJSON, time.Time{}, time.Time{}, "", 0, false)

	if answer != nil {
		t.Errorf("answer = %v, want none beside a reported condition", answer)
	}

	if !errors.Is(err, genlocal.ErrLocalReadFailed) {
		t.Fatalf("err = %v, want %v", err, genlocal.ErrLocalReadFailed)
	}
}

// The three audit query operations at the surface the generated arm calls.
//
// The same reason the export cases above exist: a handler case reads the
// sentence a caller sees and cannot see which condition the operation reported,
// which is the whole of what the emitted ladder walks. These reach the
// configured store, the fallback and its warning, the filters and the group-by
// vocabulary, none of which a behavior fixture does.

// sealAuditDir takes every permission off the directory the readers open, and
// restores it when the case ends.
//
// The directory rather than the log file: an unreadable log reads as empty in
// both languages by design, so only a directory nothing can open reaches the
// unreadable-store condition.
func sealAuditDir(t *testing.T, auditDir string) {
	t.Helper()

	if err := os.Chmod(auditDir, 0o000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() { _ = os.Chmod(auditDir, 0o750) })
}

// TestAuditHealthAnswersTheConfiguredStore covers the nested section no
// behavior fixture reaches: with the sink on, the answer carries the store's
// own row count beside the log's.
func TestAuditHealthAnswersTheConfiguredStore(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := seedExportLog(t, stateHome, []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
	})

	dbPath := filepath.Join(auditDir, "audit.db")
	oldest := writeSQLiteAuditEvents(t, dbPath)

	cfg := &config.Config{}
	cfg.Audit.SQLite.Enabled = true
	cfg.Audit.SQLite.Path = dbPath

	answer, err := tools.AuditHealth(t.Context(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if answer.Sqlite == nil {
		t.Fatal("answer.Sqlite is nil, want the store's own section")
	}

	if answer.Sqlite.EventCount != int64(2) {
		t.Errorf("answer.Sqlite.EventCount = %v, want %v", answer.Sqlite.EventCount, int64(2))
	}

	if answer.Sqlite.OldestEventUnixNs != oldest.UnixNano() {
		t.Errorf("answer.Sqlite.OldestEventUnixNs = %v, want %v",
			answer.Sqlite.OldestEventUnixNs, oldest.UnixNano())
	}

	if !answer.ActiveLogExists {
		t.Error("answer.ActiveLogExists = false, want true")
	}

	if len(answer.Warnings) != 0 {
		t.Errorf("answer.Warnings = %v, want none", answer.Warnings)
	}
}

// TestAuditHealthDegradesToTheLogBehindAWarning covers the store's own degrade
// rule: a configured database that will not open leaves the log standing and
// says so, rather than refusing.
func TestAuditHealthDegradesToTheLogBehindAWarning(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := seedExportLog(t, stateHome, []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
	})

	dbPath := filepath.Join(auditDir, "audit.db")
	if err := os.WriteFile(dbPath, []byte("this is not a sqlite database"), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := &config.Config{}
	cfg.Audit.SQLite.Enabled = true
	cfg.Audit.SQLite.Path = dbPath

	answer, err := tools.AuditHealth(t.Context(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if answer.Sqlite != nil {
		t.Errorf("answer.Sqlite = %v, want none once the store did not answer", answer.Sqlite)
	}

	if len(answer.Warnings) != 1 {
		t.Fatalf("answer.Warnings = %v, want one line naming the store", answer.Warnings)
	}

	if !strings.Contains(answer.Warnings[0], dbPath) {
		t.Errorf("answer.Warnings[0] = %v, want it to name %v", answer.Warnings[0], dbPath)
	}
}

// TestAuditHealthReportsALogItCannotRead is the read condition, which the
// generated ladder maps onto the sentence a caller reads.
func TestAuditHealthReportsALogItCannotRead(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := seedExportLog(t, stateHome, []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
	})
	sealAuditDir(t, auditDir)

	if _, err := tools.AuditHealth(t.Context(), &config.Config{}); !errors.Is(
		err, genlocal.ErrLocalReadFailed,
	) {
		t.Errorf("err = %v, want %v", err, genlocal.ErrLocalReadFailed)
	}
}

// TestAuditRecentAnswersTheFiltersItWasHanded covers the filters the shipped
// capture never sends: the tool glob, the capability and the status.
func TestAuditRecentAnswersTheFiltersItWasHanded(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	seedExportLog(t, stateHome, []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
		auditEvent("linode_instance_boot", audit.CapabilityWrite, audit.StatusSuccess, 2),
		auditEvent("linode_volume_delete", audit.CapabilityDestroy, audit.StatusError, 3),
	})

	cases := []struct {
		name       string
		tool       string
		capability string
		status     string
		limit      int
		want       int
	}{
		{"every event", "", "", "", 0, 3},
		{"one tool glob", toolInstanceGlob, "", "", 0, 2},
		{"one capability", "", string(audit.CapabilityDestroy), "", 0, 1},
		{"one status", "", "", string(audit.StatusError), 0, 1},
		{"a limit below the count", "", "", "", 2, 2},
	}

	// One env for the whole table rather than a subtest each: the state home a
	// case reads is set here, and a test that sets one cannot hand its cases to
	// the parallel runner.
	for _, testCase := range cases {
		answer, err := tools.AuditRecent(testCase.limit, time.Time{}, time.Time{},
			testCase.tool, testCase.capability, testCase.status, false)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", testCase.name, err)
		}

		if answer.Count != testCase.want {
			t.Errorf("%s: answer.Count = %v, want %v", testCase.name, answer.Count, testCase.want)
		}
	}
}

// TestAuditRecentReportsALogItCannotRead is the one condition this operation
// declares.
func TestAuditRecentReportsALogItCannotRead(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := seedExportLog(t, stateHome, []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
	})
	sealAuditDir(t, auditDir)

	if _, err := tools.AuditRecent(0, time.Time{}, time.Time{}, "", "", "", false); !errors.Is(
		err, genlocal.ErrLocalReadFailed,
	) {
		t.Errorf("err = %v, want %v", err, genlocal.ErrLocalReadFailed)
	}
}

// TestAuditSummaryCountsTheColumnItWasHanded covers a group-by the shipped
// capture never sends, and reads the bucket the answer carries.
func TestAuditSummaryCountsTheColumnItWasHanded(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	seedExportLog(t, stateHome, []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
		auditEvent("linode_instance_boot", audit.CapabilityRead, audit.StatusSuccess, 2),
		auditEvent("linode_volume_delete", audit.CapabilityDestroy, audit.StatusError, 3),
	})

	answer, err := tools.AuditSummary(t.Context(), &config.Config{}, time.Time{},
		[]string{summaryColumnCapability}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if answer.TotalEvents != 3 {
		t.Errorf("answer.TotalEvents = %v, want %v", answer.TotalEvents, 3)
	}

	if len(answer.Rows) != 2 {
		t.Fatalf("answer.Rows = %v, want one bucket per capability", answer.Rows)
	}

	if answer.Rows[0].Groups[summaryColumnCapability] != string(audit.CapabilityRead) {
		t.Errorf("answer.Rows[0].Groups = %v, want the busiest capability first",
			answer.Rows[0].Groups)
	}

	if answer.Rows[0].Count != 2 {
		t.Errorf("answer.Rows[0].Count = %v, want %v", answer.Rows[0].Count, 2)
	}
}

// TestAuditSummaryRefusesAColumnTheVocabularyLacks is the condition answered
// before any store is opened, which is why it is a different one from a read.
func TestAuditSummaryRefusesAColumnTheVocabularyLacks(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	seedExportLog(t, stateHome, nil)

	_, err := tools.AuditSummary(t.Context(), &config.Config{}, time.Time{},
		[]string{"nope"}, false)
	if !errors.Is(err, genlocal.ErrLocalInputRejected) {
		t.Errorf("err = %v, want %v", err, genlocal.ErrLocalInputRejected)
	}
}

// TestAuditSummaryDegradesToTheLogBehindAWarning is the store's degrade rule
// again, through the reader that loads a window rather than the health one.
func TestAuditSummaryDegradesToTheLogBehindAWarning(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := seedExportLog(t, stateHome, []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
	})

	dbPath := filepath.Join(auditDir, "audit.db")
	if err := os.WriteFile(dbPath, []byte("this is not a sqlite database"), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := &config.Config{}
	cfg.Audit.SQLite.Enabled = true
	cfg.Audit.SQLite.Path = dbPath

	answer, err := tools.AuditSummary(t.Context(), cfg, time.Time{}, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if answer.TotalEvents != 1 {
		t.Errorf("answer.TotalEvents = %v, want the log's own count", answer.TotalEvents)
	}

	if len(answer.Warnings) != 1 {
		t.Fatalf("answer.Warnings = %v, want one line naming the store", answer.Warnings)
	}
}

// TestAuditReportDegradesToTheLogAndSaysSo covers the store the report resolves
// for itself, which is the one thing this operation does that the per-package
// engine tests cannot see: they are handed the path rather than resolving it.
func TestAuditReportDegradesToTheLogAndSaysSo(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := seedExportLog(t, stateHome, []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
	})

	dbPath := filepath.Join(auditDir, "audit.db")
	if err := os.WriteFile(dbPath, []byte("this is not a sqlite database"), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := &config.Config{}
	cfg.Audit.SQLite.Enabled = true
	cfg.Audit.SQLite.Path = dbPath
	cfg.Audit.Reports = map[string]config.ReportConfig{
		"window": {Output: config.ReportOutputSummary},
	}

	answer, err := tools.AuditReport(t.Context(), cfg, time.Now(), "window")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if answer.TotalEvents != 1 {
		t.Errorf("answer.TotalEvents = %v, want the log's own count", answer.TotalEvents)
	}

	if len(answer.Warnings) != 1 {
		t.Fatalf("answer.Warnings = %v, want one line naming the store", answer.Warnings)
	}
}

// TestAuditSummaryReportsALogItCannotRead is the read condition, which only a
// directory nothing can open reaches.
func TestAuditSummaryReportsALogItCannotRead(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := seedExportLog(t, stateHome, []*audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
	})
	sealAuditDir(t, auditDir)

	_, err := tools.AuditSummary(t.Context(), &config.Config{}, time.Time{}, nil, false)
	if !errors.Is(err, genlocal.ErrLocalReadFailed) {
		t.Errorf("err = %v, want %v", err, genlocal.ErrLocalReadFailed)
	}
}
