package audit_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/genlocal"
)

// errFixtureHandler stands for whatever a handler answered, so the outcome case
// can name the failure it planted rather than matching on text.
var errFixtureHandler = errors.New("handler refused the call")

const (
	// fixtureTool is the tool name reused across happy-path tests.
	// Distinct from any production tool name so a test failure
	// doesn't read as a real-tool regression.
	fixtureTool        = "fixture_tool"
	fixtureEnvironment = "fixture_env"
	fixtureProfile     = "fixture_profile"
	fixtureSession     = "sess_test_01"
	fixtureVersion     = "0.0.0-test"
)

// TestNewEventPopulatesEveryField is the substantive coverage
// assertion. The wire format claims every field is non-optional;
// this test instantiates an event and asserts each field has a
// concrete value (or null where the schema permits it). A regression
// that drops a field would surface here, not at the first sink that
// tries to read it.
func TestNewEventPopulatesEveryField(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		argLinodeID: 12345,
		"confirm":   true,
	}

	evt := audit.NewEvent(
		fixtureTool,
		audit.CapabilityDestroy,
		args,
		fixtureEnvironment,
		fixtureProfile,
		fixtureSession,
		3,
		fixtureVersion,
		false,
	)

	if evt.Ts == "" {
		t.Error("evt.Ts is empty, want a timestamp")
	}

	if evt.TsUnixNs == 0 {
		t.Errorf("evt.TsUnixNs = %v, want non-zero", evt.TsUnixNs)
	}

	if !strings.HasPrefix(evt.EventId, audit.EventIDPrefix) {
		t.Error("expected condition to be true")
	}

	if evt.Tool != fixtureTool {
		t.Errorf("evt.Tool = %v, want %v", evt.Tool, fixtureTool)
	}

	if evt.ToolCapability != string(audit.CapabilityDestroy) {
		t.Errorf("evt.ToolCapability = %v, want %v", evt.ToolCapability, audit.CapabilityDestroy)
	}

	if evt.Environment != fixtureEnvironment {
		t.Errorf("evt.Environment = %v, want %v", evt.Environment, fixtureEnvironment)
	}

	if evt.Profile != fixtureProfile {
		t.Errorf("evt.Profile = %v, want %v", evt.Profile, fixtureProfile)
	}

	if evt.Mode != string(audit.ModeNormal) {
		t.Errorf("evt.Mode = %v, want %v", evt.Mode, audit.ModeNormal)
	}

	if evt.PlanId != nil {
		t.Errorf("evt.PlanId = %v, want nil", evt.PlanId)
	}

	if !reflect.DeepEqual(evt.Args[argLinodeID], args[argLinodeID]) {
		t.Errorf("evt.Args[argLinodeID] = %v, want %v", evt.Args[argLinodeID], args[argLinodeID])
	}

	if len(evt.ArgsRedacted) != 0 {
		t.Errorf("evt.ArgsRedacted = %v, want empty", evt.ArgsRedacted)
	}

	if evt.Status != string(audit.StatusSuccess) {
		t.Errorf("evt.Status = %v, want %v", evt.Status, audit.StatusSuccess)
	}

	if evt.LatencyMs != 0 {
		t.Errorf("evt.LatencyMs = %v, want zero", evt.LatencyMs)
	}

	if evt.ResultSummary != "" {
		t.Errorf("evt.ResultSummary = %v, want empty", evt.ResultSummary)
	}

	if evt.Error != nil {
		t.Errorf("evt.Error = %v, want nil", evt.Error)
	}

	if evt.LinodemcpVersion != fixtureVersion {
		t.Errorf("evt.LinodemcpVersion = %v, want %v", evt.LinodemcpVersion, fixtureVersion)
	}

	if evt.SessionId != fixtureSession {
		t.Errorf("evt.SessionId = %v, want %v", evt.SessionId, fixtureSession)
	}

	if evt.CredentialGeneration != uint64(3) {
		t.Errorf("evt.CredentialGeneration = %v, want %v", evt.CredentialGeneration, uint64(3))
	}
}

// TestFinalizeWritesOutcomeFields locks the contract that Finalize
// produces a non-zero latency, a non-nil Error pointer when a
// message is supplied, and a nil Error pointer when the message is
// empty.
func TestFinalizeWritesOutcomeFields(t *testing.T) {
	t.Parallel()

	evt := audit.Finalize(newFixtureEvent(t), audit.StatusError,
		250*time.Millisecond, "API returned 500", "instance update failed")

	if evt.Status != string(audit.StatusError) {
		t.Errorf("evt.Status = %v, want %v", evt.Status, audit.StatusError)
	}

	if evt.LatencyMs != int64(250) {
		t.Errorf("evt.LatencyMs = %v, want %v", evt.LatencyMs, int64(250))
	}

	if evt.ResultSummary != "instance update failed" {
		t.Errorf("evt.ResultSummary = %v, want %v", evt.ResultSummary, "instance update failed")
	}

	if evt.Error == nil {
		t.Fatal("evt.Error is nil")
	}

	if *evt.Error != "API returned 500" {
		t.Errorf("*evt.Error = %v, want %v", *evt.Error, "API returned 500")
	}
}

// TestFinalizeWithEmptyErrorMessageLeavesErrorNil covers the happy
// path: Finalize with errMsg="" must produce a nil Error pointer so
// the JSON output renders `null`, not `""`.
func TestFinalizeWithEmptyErrorMessageLeavesErrorNil(t *testing.T) {
	t.Parallel()

	evt := audit.Finalize(newFixtureEvent(t), audit.StatusSuccess,
		100*time.Millisecond, "", "ok")

	if evt.Status != string(audit.StatusSuccess) {
		t.Errorf("evt.Status = %v, want %v", evt.Status, audit.StatusSuccess)
	}

	if evt.Error != nil {
		t.Errorf("evt.Error = %v, want nil", evt.Error)
	}
}

// TestSetModePopulatesPlanID locks the plan-mode contract: passing a
// non-empty plan ID stores the pointer. Passing empty clears it back
// to nil so SetMode can be used to revert.
func TestSetModePopulatesPlanID(t *testing.T) {
	t.Parallel()

	planned := audit.SetMode(newFixtureEvent(t), audit.ModeApply, "plan_01H...")

	if planned.PlanId == nil {
		t.Fatal("planned.PlanId is nil")
	}

	if *planned.PlanId != "plan_01H..." {
		t.Errorf("*planned.PlanId = %v, want %v", *planned.PlanId, "plan_01H...")
	}

	reverted := audit.SetMode(planned, audit.ModeNormal, "")

	if reverted.PlanId != nil {
		t.Errorf("reverted.PlanId = %v, want nil", reverted.PlanId)
	}
}

// TestEventTimestampSpellsTheInstantTheRecordCarries pins the two forms a ts
// member takes. A record written by an EARLIER version carries the whole-second
// form, so an instant that lands on a second has to come back without a
// fractional part; anything finer carries microseconds with their zeros kept.
// One spelling for both would read the same on this batch's own fixtures and
// move a timestamp the SQLite store reconstructs.
func TestEventTimestampSpellsTheInstantTheRecordCarries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		instant time.Time
		want    string
	}{
		{
			name:    "an instant on a whole second carries no fraction",
			instant: time.Date(2026, time.May, 19, 12, 0, 0, 0, time.UTC),
			want:    "2026-05-19T12:00:00Z",
		},
		{
			name:    "a quarter second carries its zeros",
			instant: time.Date(2026, time.May, 19, 11, 30, 0, 250000000, time.UTC),
			want:    "2026-05-19T11:30:00.250000Z",
		},
		{
			name:    "a microsecond carries every digit",
			instant: time.Date(2026, time.May, 19, 11, 30, 0, 1000, time.UTC),
			want:    "2026-05-19T11:30:00.000001Z",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := audit.EventTimestamp(testCase.instant); got != testCase.want {
				t.Errorf("EventTimestamp = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestOutcomeReadsTheStatusOffWhatTheHandlerAnswered covers both arms of the
// one place the middleware does not already know the status: an error the
// handler returned becomes the error status carrying its own sentence, and no
// error becomes success. A call audited as a success it did not have is the
// failure this exists to stop.
func TestOutcomeReadsTheStatusOffWhatTheHandlerAnswered(t *testing.T) {
	t.Parallel()

	answered := audit.Outcome(newFixtureEvent(t), 250*time.Millisecond, errFixtureHandler)

	if answered.Status != string(audit.StatusError) {
		t.Errorf("answered.Status = %v, want %v", answered.Status, audit.StatusError)
	}

	if answered.Error == nil {
		t.Fatal("answered.Error is nil")
	}

	if *answered.Error != errFixtureHandler.Error() {
		t.Errorf("*answered.Error = %v, want %v", *answered.Error, errFixtureHandler)
	}

	clean := audit.Outcome(newFixtureEvent(t), 250*time.Millisecond, nil)

	if clean.Status != string(audit.StatusSuccess) {
		t.Errorf("clean.Status = %v, want %v", clean.Status, audit.StatusSuccess)
	}

	if clean.Error != nil {
		t.Errorf("clean.Error = %v, want nil", clean.Error)
	}

	if clean.LatencyMs != int64(250) {
		t.Errorf("clean.LatencyMs = %v, want %v", clean.LatencyMs, int64(250))
	}
}

// TestFinalizeLeavesTheEventItWasHandedAlone is the half a rebuild adds that a
// write into the old record never had: the capture middleware holds the entry
// event while the outcome one is written, so a Finalize that mutated would
// change what an earlier caller still holds.
func TestFinalizeLeavesTheEventItWasHandedAlone(t *testing.T) {
	t.Parallel()

	entry := newFixtureEvent(t)

	audit.Finalize(entry, audit.StatusError, time.Second, "boom", "failed")
	audit.SetMode(entry, audit.ModeApply, "plan_01H...")

	if entry.Status != string(audit.StatusSuccess) {
		t.Errorf("entry.Status = %v, want %v", entry.Status, audit.StatusSuccess)
	}

	if entry.LatencyMs != 0 {
		t.Errorf("entry.LatencyMs = %v, want zero", entry.LatencyMs)
	}

	if entry.PlanId != nil {
		t.Errorf("entry.PlanId = %v, want nil", entry.PlanId)
	}
}

// TestRecordSerializesEmptyCollectionsAsArrays guards against a nil map or
// slice reaching the log as null. The JSONL consumers downstream expect `{}`
// and `[]`, which is the array-over-null contract the record writer keeps.
func TestRecordSerializesEmptyCollectionsAsArrays(t *testing.T) {
	t.Parallel()

	evt := audit.Event{
		EventId: "evt_test",
		Tool:    fixtureTool,
		// Args and ArgsRedacted left at zero values.
	}

	body, err := genlocal.RecordAuditEvent(&evt, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if reflect.TypeOf(parsed["args"]) != reflect.TypeFor[map[string]any]() {
		t.Errorf("type = %T, want %T", parsed["args"], map[string]any{})
	}

	if reflect.TypeOf(parsed["args_redacted"]) != reflect.TypeFor[[]any]() {
		t.Errorf("type = %T, want %T", parsed["args_redacted"], []any{})
	}
}

// TestEventIDIsCorrectLength checks the format constants. ULID body
// is 26 characters; with the evt_ prefix the total is 30. A
// regression that adjusts the alphabet or the encoder would change
// this length.
func TestEventIDIsCorrectLength(t *testing.T) {
	t.Parallel()

	id := audit.NewEventID(time.Now())

	if len(id) != 30 {
		t.Errorf("len(id) = %d, want %d", len(id), 30)
	}

	if !strings.HasPrefix(id, audit.EventIDPrefix) {
		t.Error("strings.HasPrefix(id, audit.EventIDPrefix) = false, want true")
	}
}

// TestEventIDUsesCrockfordAlphabet confirms the encoder produces
// only valid Crockford base32 characters (I, L, O, U excluded). A
// regression to a different alphabet (e.g. plain base32) would
// produce L or O characters which fail this check.
func TestEventIDUsesCrockfordAlphabet(t *testing.T) {
	t.Parallel()

	id := audit.NewEventID(time.Now())
	body := strings.TrimPrefix(id, audit.EventIDPrefix)

	for _, char := range body {
		if strings.ContainsRune("ILOU", char) {
			t.Errorf("collection should not contain %v", string(char))
		}
	}
}

// TestEventIDsAreUnique ensures two consecutive ID generations don't
// collide. The randomness portion is 80 bits, so this is effectively
// a smoke test rather than a probability check.
func TestEventIDsAreUnique(t *testing.T) {
	t.Parallel()

	id1 := audit.NewEventID(time.Now())
	id2 := audit.NewEventID(time.Now())

	if id2 == id1 {
		t.Errorf("id2 = %v, do not want %v", id2, id1)
	}
}

// newFixtureEvent is the shared helper that emits an event the
// outcome tests can mutate. Extracted so the field-population test
// (which uses its own args) doesn't share a fixture with the
// outcome tests.
func newFixtureEvent(t *testing.T) *audit.Event {
	t.Helper()

	return audit.NewEvent(
		fixtureTool,
		audit.CapabilityWrite,
		map[string]any{argLinodeID: 1},
		fixtureEnvironment,
		fixtureProfile,
		fixtureSession,
		1,
		fixtureVersion,
		false,
	)
}
