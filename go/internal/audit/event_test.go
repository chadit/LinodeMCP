package audit_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/audit"
)

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

	if evt.TS.IsZero() {
		t.Error("evt.TS.IsZero() = true, want false")
	}

	if evt.TSUnixNS == 0 {
		t.Errorf("evt.TSUnixNS = %v, want non-zero", evt.TSUnixNS)
	}

	if !strings.HasPrefix(evt.EventID, audit.EventIDPrefix) {
		t.Error("expected condition to be true")
	}

	if evt.Tool != fixtureTool {
		t.Errorf("evt.Tool = %v, want %v", evt.Tool, fixtureTool)
	}

	if evt.ToolCapability != audit.CapabilityDestroy {
		t.Errorf("evt.ToolCapability = %v, want %v", evt.ToolCapability, audit.CapabilityDestroy)
	}

	if evt.Environment != fixtureEnvironment {
		t.Errorf("evt.Environment = %v, want %v", evt.Environment, fixtureEnvironment)
	}

	if evt.Profile != fixtureProfile {
		t.Errorf("evt.Profile = %v, want %v", evt.Profile, fixtureProfile)
	}

	if evt.Mode != audit.ModeNormal {
		t.Errorf("evt.Mode = %v, want %v", evt.Mode, audit.ModeNormal)
	}

	if evt.PlanID != nil {
		t.Errorf("evt.PlanID = %v, want nil", evt.PlanID)
	}

	if !reflect.DeepEqual(evt.Args[argLinodeID], args[argLinodeID]) {
		t.Errorf("evt.Args[argLinodeID] = %v, want %v", evt.Args[argLinodeID], args[argLinodeID])
	}

	if len(evt.ArgsRedacted) != 0 {
		t.Errorf("evt.ArgsRedacted = %v, want empty", evt.ArgsRedacted)
	}

	if evt.Status != audit.StatusSuccess {
		t.Errorf("evt.Status = %v, want %v", evt.Status, audit.StatusSuccess)
	}

	if evt.LatencyMS != 0 {
		t.Errorf("evt.LatencyMS = %v, want zero", evt.LatencyMS)
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

	if evt.SessionID != fixtureSession {
		t.Errorf("evt.SessionID = %v, want %v", evt.SessionID, fixtureSession)
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

	evt := newFixtureEvent(t)

	evt.Finalize(audit.StatusError, 250*time.Millisecond, "API returned 500", "instance update failed")

	if evt.Status != audit.StatusError {
		t.Errorf("evt.Status = %v, want %v", evt.Status, audit.StatusError)
	}

	if evt.LatencyMS != int64(250) {
		t.Errorf("evt.LatencyMS = %v, want %v", evt.LatencyMS, int64(250))
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

	evt := newFixtureEvent(t)

	evt.Finalize(audit.StatusSuccess, 100*time.Millisecond, "", "ok")

	if evt.Status != audit.StatusSuccess {
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

	evt := newFixtureEvent(t)

	evt.SetMode(audit.ModeApply, "plan_01H...")

	if evt.PlanID == nil {
		t.Fatal("evt.PlanID is nil")
	}

	if *evt.PlanID != "plan_01H..." {
		t.Errorf("*evt.PlanID = %v, want %v", *evt.PlanID, "plan_01H...")
	}

	evt.SetMode(audit.ModeNormal, "")

	if evt.PlanID != nil {
		t.Errorf("evt.PlanID = %v, want nil", evt.PlanID)
	}
}

// TestMarshalJSONSerializesEmptyCollectionsAsArrays guards against
// the standard encoder's nil-map / nil-slice fallback to `null`. The
// JSONL consumers downstream of this expect `{}` and `[]` so the
// alias-and-substitute pattern in MarshalJSON has to actually fire.
func TestMarshalJSONSerializesEmptyCollectionsAsArrays(t *testing.T) {
	t.Parallel()

	evt := audit.Event{
		EventID: "evt_test",
		Tool:    fixtureTool,
		// Args and ArgsRedacted left at zero values.
	}

	body, err := evt.MarshalJSON()
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
func newFixtureEvent(t *testing.T) audit.Event {
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
