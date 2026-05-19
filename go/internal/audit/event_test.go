package audit_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/audit"
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
	)

	assert.False(t, evt.TS.IsZero(), "TS must be populated")
	assert.NotZero(t, evt.TSUnixNS, "TSUnixNS must be populated")
	assert.True(t, strings.HasPrefix(evt.EventID, audit.EventIDPrefix),
		"EventID must carry the evt_ prefix")
	assert.Equal(t, fixtureTool, evt.Tool)
	assert.Equal(t, audit.CapabilityDestroy, evt.ToolCapability)
	assert.Equal(t, fixtureEnvironment, evt.Environment)
	assert.Equal(t, fixtureProfile, evt.Profile)
	assert.Equal(t, audit.ModeNormal, evt.Mode, "default mode must be normal")
	assert.Nil(t, evt.PlanID, "plan_id is nil unless set via SetMode")
	assert.Equal(t, args[argLinodeID], evt.Args[argLinodeID])
	assert.Empty(t, evt.ArgsRedacted, "no sensitive keys in fixture")
	assert.Equal(t, audit.StatusSuccess, evt.Status, "default status is success")
	assert.Zero(t, evt.LatencyMS, "latency populates via Finalize")
	assert.Empty(t, evt.ResultSummary)
	assert.Nil(t, evt.Error)
	assert.Equal(t, fixtureVersion, evt.LinodemcpVersion)
	assert.Equal(t, fixtureSession, evt.SessionID)
	assert.Equal(t, uint64(3), evt.CredentialGeneration)
}

// TestFinalizeWritesOutcomeFields locks the contract that Finalize
// produces a non-zero latency, a non-nil Error pointer when a
// message is supplied, and a nil Error pointer when the message is
// empty.
func TestFinalizeWritesOutcomeFields(t *testing.T) {
	t.Parallel()

	evt := newFixtureEvent(t)

	evt.Finalize(audit.StatusError, 250*time.Millisecond, "API returned 500", "instance update failed")

	assert.Equal(t, audit.StatusError, evt.Status)
	assert.Equal(t, int64(250), evt.LatencyMS)
	assert.Equal(t, "instance update failed", evt.ResultSummary)
	require.NotNil(t, evt.Error)
	assert.Equal(t, "API returned 500", *evt.Error)
}

// TestFinalizeWithEmptyErrorMessageLeavesErrorNil covers the happy
// path: Finalize with errMsg="" must produce a nil Error pointer so
// the JSON output renders `null`, not `""`.
func TestFinalizeWithEmptyErrorMessageLeavesErrorNil(t *testing.T) {
	t.Parallel()

	evt := newFixtureEvent(t)

	evt.Finalize(audit.StatusSuccess, 100*time.Millisecond, "", "ok")

	assert.Equal(t, audit.StatusSuccess, evt.Status)
	assert.Nil(t, evt.Error, "empty errMsg must produce nil Error pointer")
}

// TestSetModePopulatesPlanID locks the plan-mode contract: passing a
// non-empty plan ID stores the pointer. Passing empty clears it back
// to nil so SetMode can be used to revert.
func TestSetModePopulatesPlanID(t *testing.T) {
	t.Parallel()

	evt := newFixtureEvent(t)

	evt.SetMode(audit.ModeApply, "plan_01H...")
	require.NotNil(t, evt.PlanID)
	assert.Equal(t, "plan_01H...", *evt.PlanID)

	evt.SetMode(audit.ModeNormal, "")
	assert.Nil(t, evt.PlanID, "empty planID must clear the pointer")
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
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	assert.IsType(t, map[string]any{}, parsed["args"], "args must serialize as object")
	assert.IsType(t, []any{}, parsed["args_redacted"], "args_redacted must serialize as array")
}

// TestEventIDIsCorrectLength checks the format constants. ULID body
// is 26 characters; with the evt_ prefix the total is 30. A
// regression that adjusts the alphabet or the encoder would change
// this length.
func TestEventIDIsCorrectLength(t *testing.T) {
	t.Parallel()

	id := audit.NewEventID(time.Now())

	assert.Len(t, id, 30, "event id is evt_ (4) + 26-char ULID body")
	assert.True(t, strings.HasPrefix(id, audit.EventIDPrefix))
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
		assert.NotContains(t, "ILOU", string(char),
			"ULID body must not contain ambiguous Crockford characters")
	}
}

// TestEventIDsAreUnique ensures two consecutive ID generations don't
// collide. The randomness portion is 80 bits, so this is effectively
// a smoke test rather than a probability check.
func TestEventIDsAreUnique(t *testing.T) {
	t.Parallel()

	id1 := audit.NewEventID(time.Now())
	id2 := audit.NewEventID(time.Now())

	assert.NotEqual(t, id1, id2, "two consecutive event ids must not collide")
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
	)
}
