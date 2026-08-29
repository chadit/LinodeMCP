package audit_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
)

// The report engine's own tests. They live here rather than beside the tool
// because the engine does, and because coverage credit is per package: a branch
// reached only from internal/tools reads as dark in this package's profile.

// tcReportName is the report every case runs, named once because goconst counts
// a literal across the whole package.
const tcReportName = "window"

// reportClock is the reference instant a relative window is measured back from,
// standing one hour after the newest seeded event.
func reportClock() time.Time {
	return time.Date(testYear, time.May, 19, 13, 0, 0, 0, time.UTC)
}

// tcWholeWindow reaches back past the oldest seeded event, so a report carrying
// it takes all three unless one of its own filters drops one.
const tcWholeWindow = "4h"

// seedReportLog writes four events into a fresh JSONL log and answers the
// directory holding it. Three are an hour apart and the fourth is a meta event,
// which every other reader defaults out and the report asks for.
func seedReportLog(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	writeJSONLFile(t, filepath.Join(dir, audit.ActiveLogFileName), false, []*audit.Event{
		reportEvent(tcLinodeInstanceList, audit.CapabilityRead, audit.StatusSuccess, day(19, 10)),
		reportEvent(tcLinodeInstanceCreate, audit.CapabilityWrite, audit.StatusSuccess, day(19, 11)),
		reportEvent(tcLinodeInstanceDelete, audit.CapabilityDestroy, audit.StatusError, day(19, 12)),
		reportEvent("linode_audit_recent", audit.CapabilityMeta, audit.StatusSuccess, day(19, 12)),
	})

	return dir
}

// reportEvent is one seeded event carrying the environment and profile the
// post-load globs read, which makeTestEvent leaves empty.
func reportEvent(
	tool string, capability audit.Capability, status audit.Status, timestamp time.Time,
) *audit.Event {
	event := makeTestEvent(tool, capability, status, timestamp)
	event.Environment = "production"
	event.Profile = "full-access"

	return event
}

// summaryReport is a report grouping the whole seeded window by status.
func summaryReport(filter *config.ReportFilter) *config.ReportConfig {
	return &config.ReportConfig{
		Filter: *filter, Output: config.ReportOutputSummary, GroupBy: []string{colStatus},
	}
}

// listReport is a report listing the whole seeded window.
func listReport(filter *config.ReportFilter, limit int) *config.ReportConfig {
	return &config.ReportConfig{
		Filter: *filter, Output: config.ReportOutputList, Limit: limit,
	}
}

// TestReportMeasuresARelativeWindowFromTheInstantItIsHanded proves the clock
// reaches the window rather than being read inside the engine.
//
// The same store and the same report answer differently under two instants, one
// standing an hour after the events and one standing a year later, which is the
// property LOCAL_AMBIENT_CLOCK exists to keep a tenth language from losing.
func TestReportMeasuresARelativeWindowFromTheInstantItIsHanded(t *testing.T) {
	t.Parallel()

	dir := seedReportLog(t)
	definition := summaryReport(&config.ReportFilter{SinceOffset: tcWholeWindow})

	inside, err := audit.Report(t.Context(), "", dir, tcReportName, definition, reportClock())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inside.TotalEvents != 4 {
		t.Errorf("events inside the window = %d, want 4", inside.TotalEvents)
	}

	longAfter := reportClock().AddDate(1, 0, 0)

	outside, err := audit.Report(t.Context(), "", dir, tcReportName, definition, longAfter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if outside.TotalEvents != 0 {
		t.Errorf("events inside the window = %d, want 0", outside.TotalEvents)
	}
}

// TestReportSummarizesIntoTheBucketsItsDefinitionNames covers the summary
// output: the rows carry the grouped counts and the event list stays empty.
func TestReportSummarizesIntoTheBucketsItsDefinitionNames(t *testing.T) {
	t.Parallel()

	dir := seedReportLog(t)

	answer, err := audit.Report(t.Context(), "", dir, tcReportName,
		summaryReport(&config.ReportFilter{SinceOffset: tcWholeWindow}), reportClock())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if answer.Name != tcReportName || answer.Output != config.ReportOutputSummary {
		t.Errorf("answer = %q/%q, want %q/%q",
			answer.Name, answer.Output, tcReportName, config.ReportOutputSummary)
	}

	if len(answer.Rows) != 2 {
		t.Fatalf("rows = %d, want 2 (one success bucket, one error bucket)", len(answer.Rows))
	}

	if listed := answer.Events; len(listed) != 0 {
		t.Errorf("events = %d, want 0: a summary answers its rows", len(listed))
	}
}

// TestReportListsUnderItsOwnLimit covers the list output and the truncation,
// which is the only place the limit is read.
func TestReportListsUnderItsOwnLimit(t *testing.T) {
	t.Parallel()

	dir := seedReportLog(t)

	answer, err := audit.Report(t.Context(), "", dir, tcReportName,
		listReport(&config.ReportFilter{SinceOffset: tcWholeWindow}, 1), reportClock())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if answer.TotalEvents != 1 {
		t.Errorf("total = %d, want 1: the limit truncates before the count", answer.TotalEvents)
	}

	if listed := answer.Events; len(listed) != 1 {
		t.Errorf("events = %d, want 1", len(listed))
	}

	if len(answer.Rows) != 0 {
		t.Errorf("rows = %d, want 0: a list answers its events", len(answer.Rows))
	}
}

// TestReportBoundsAnAbsoluteWindowAtBothEnds covers the other window form: two
// timestamps rather than an offset, which is what a report reading a fixed
// period is written with and what the clock never touches.
func TestReportBoundsAnAbsoluteWindowAtBothEnds(t *testing.T) {
	t.Parallel()

	dir := seedReportLog(t)

	answer, err := audit.Report(t.Context(), "", dir, tcReportName,
		summaryReport(&config.ReportFilter{
			Since: "2026-05-19T10:30:00Z",
			Until: "2026-05-19T11:30:00Z",
		}), reportClock())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if answer.TotalEvents != 1 {
		t.Errorf("events = %d, want 1: only the 11:00 event falls inside both bounds",
			answer.TotalEvents)
	}
}

// TestReportKeepsOnlyTheEventsItsPostFiltersAdmit covers the four filter members
// no store query carries, each narrowing the same seeded window.
func TestReportKeepsOnlyTheEventsItsPostFiltersAdmit(t *testing.T) {
	t.Parallel()

	dir := seedReportLog(t)

	cases := []struct {
		name   string
		filter config.ReportFilter
		want   int
	}{
		{
			name:   "a capability list",
			filter: config.ReportFilter{SinceOffset: tcWholeWindow, CapabilityIn: []string{"write", "destroy"}},
			want:   2,
		},
		{
			name:   "a status list",
			filter: config.ReportFilter{SinceOffset: tcWholeWindow, StatusIn: []string{"error"}},
			want:   1,
		},
		{
			name:   "an environment glob",
			filter: config.ReportFilter{SinceOffset: tcWholeWindow, Environment: "prod*"},
			want:   4,
		},
		{
			name:   "a profile glob nothing matches",
			filter: config.ReportFilter{SinceOffset: tcWholeWindow, Profile: "no-such-*"},
			want:   0,
		},
		{
			// path.Match answers false beside its error, so a pattern it cannot
			// parse admits nothing rather than everything.
			name:   "a malformed glob",
			filter: config.ReportFilter{SinceOffset: tcWholeWindow, Environment: "[unclosed"},
			want:   0,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			answer, err := audit.Report(t.Context(), "", dir, tcReportName,
				summaryReport(&testCase.filter), reportClock())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if answer.TotalEvents != testCase.want {
				t.Errorf("events = %d, want %d", answer.TotalEvents, testCase.want)
			}
		})
	}
}

// TestReportCountsTheMetaEventsTheOtherReadersLeaveOut covers the one query
// member the report sets for itself rather than taking from a caller.
//
// The report grammar decides meta inclusion through its own capability filter,
// so its load query always asks for meta events; every other reader defaults
// them out. Nothing else in either net reaches that, which the mutation run
// said before this case existed.
func TestReportCountsTheMetaEventsTheOtherReadersLeaveOut(t *testing.T) {
	t.Parallel()

	dir := seedReportLog(t)

	answer, err := audit.Report(t.Context(), "", dir, tcReportName,
		summaryReport(&config.ReportFilter{
			SinceOffset: tcWholeWindow, Capability: string(audit.CapabilityMeta),
		}), reportClock())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if answer.TotalEvents != 1 {
		t.Errorf("meta events = %d, want 1: the report asks for them", answer.TotalEvents)
	}
}

// TestReportRefusesADefinitionTheVocabularyWillNotTake covers the three ways a
// report file can be written wrong.
//
// The configuration validator reads the offset and the bounds first, so these
// reach the engine only from a definition nothing checked. That is what makes
// them the engine's own condition rather than a failed read, and the tool layer
// tells the two apart by the sentinel alone.
func TestReportRefusesADefinitionTheVocabularyWillNotTake(t *testing.T) {
	t.Parallel()

	dir := seedReportLog(t)

	cases := []struct {
		definition *config.ReportConfig
		name       string
	}{
		{
			name:       "an offset that is not a duration",
			definition: summaryReport(&config.ReportFilter{SinceOffset: "two hours"}),
		},
		{
			name:       "a since that is not a timestamp",
			definition: summaryReport(&config.ReportFilter{Since: "yesterday"}),
		},
		{
			name:       "an until that is not a timestamp",
			definition: summaryReport(&config.ReportFilter{Until: "tomorrow"}),
		},
		{
			name: "a group_by column outside the summary vocabulary",
			definition: &config.ReportConfig{
				Output: config.ReportOutputSummary, GroupBy: []string{"nonsense"},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := audit.Report(t.Context(), "", dir, tcReportName,
				testCase.definition, reportClock())

			if !errors.Is(err, audit.ErrReportDefinition) {
				t.Errorf("err = %v, want %v", err, audit.ErrReportDefinition)
			}
		})
	}
}

// TestReportKeepsTheCauseOutOfTheConditionsWords covers the half of the sentinel
// a caller reads: the sentence is the cause's own, so the tool layer's wording
// is not doubled with the condition's.
func TestReportKeepsTheCauseOutOfTheConditionsWords(t *testing.T) {
	t.Parallel()

	dir := seedReportLog(t)

	_, err := audit.Report(t.Context(), "", dir, tcReportName,
		&config.ReportConfig{
			Output: config.ReportOutputSummary, GroupBy: []string{"nonsense"},
		}, reportClock())
	if err == nil {
		t.Fatal("err is nil, want the group_by refusal")
	}

	want := `validate report group_by: audit: unknown group_by column: "nonsense"`
	if err.Error() != want {
		t.Errorf("sentence = %q, want %q", err.Error(), want)
	}
}

// TestReportAnswersAReadFailureRatherThanADefinitionOne covers the other side of
// the sentinel: a store nothing can read is not the definition's fault, and the
// tool layer words the two differently.
func TestReportAnswersAReadFailureRatherThanADefinitionOne(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() { _ = os.Chmod(dir, 0o750) })

	_, err := audit.Report(t.Context(), "", dir, tcReportName,
		summaryReport(&config.ReportFilter{SinceOffset: tcWholeWindow}), reportClock())

	// Named positively rather than through a nil check: an assertion that only
	// says "not the definition's condition" would pass on no error at all.
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("err = %v, want the sealed directory's own refusal", err)
	}

	if errors.Is(err, audit.ErrReportDefinition) {
		t.Errorf("err = %v, want a read failure rather than the definition's condition", err)
	}
}

// TestReportCarriesTheWarningWhenItFallsBackToTheLog covers the store fallback
// through the report, which is the answer member the engine builds once with
// everything else rather than assigning in afterwards.
func TestReportCarriesTheWarningWhenItFallsBackToTheLog(t *testing.T) {
	t.Parallel()

	dir := seedReportLog(t)
	store := filepath.Join(dir, "audit.db")

	if err := os.WriteFile(store, []byte(notADatabase), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	answer, err := audit.Report(t.Context(), store, dir, tcReportName,
		summaryReport(&config.ReportFilter{SinceOffset: tcWholeWindow}), reportClock())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(answer.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one naming the store it could not read", answer.Warnings)
	}

	if answer.TotalEvents != 4 {
		t.Errorf("events = %d, want 4: the log still answers behind the warning",
			answer.TotalEvents)
	}

	// The list output builds its own answer, so it carries the warning through a
	// second constructor the summary case never reaches.
	listed, err := audit.Report(t.Context(), store, dir, tcReportName,
		listReport(&config.ReportFilter{SinceOffset: tcWholeWindow}, 0), reportClock())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(listed.Warnings) != 1 {
		t.Errorf("warnings on the list answer = %v, want one", listed.Warnings)
	}
}
