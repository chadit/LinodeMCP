// Package audit defines the audit event schema and supporting
// helpers used by the audit-log spec. Phase 1a (this code) builds
// the event types and the redaction helper; Phase 1b wires capture
// into the tool dispatch middleware; later phases add JSONL and
// SQLite sinks plus query tools.
//
// The package is intentionally dependency-free at the types layer
// so the event struct can be used from tests without pulling in
// server-internal types.
package audit

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"
)

// EventIDPrefix is the constant prefix on every event_id. Combined
// with a 26-char ULID body, the full id looks like
// "evt_01HQXY3ZKQ8M7VRBNP4W5T2J9F".
const EventIDPrefix = "evt_"

// Capability mirrors profiles.Capability but stays a string for the
// audit wire format. Keeping it stringly-typed at the audit boundary
// avoids a cyclic dependency with the profiles package; callers
// translate from profiles.Capability via String() at event
// construction time.
type Capability string

// Capability constants. Lowercase to match the spec's wire format.
const (
	CapabilityRead    Capability = "read"
	CapabilityWrite   Capability = "write"
	CapabilityDestroy Capability = "destroy"
	CapabilityAdmin   Capability = "admin"
	CapabilityMeta    Capability = "meta"
)

// Status enumerates the terminal states of a tool call. `success`
// is the happy path; `error` covers handler-level failures; `refused`
// covers profile blocks and validation failures (i.e. the handler
// never ran).
type Status string

// Status constants.
const (
	StatusSuccess Status = "success"
	StatusError   Status = "error"
	StatusRefused Status = "refused"
)

// Mode enumerates the execution paths a call can take. Default is
// `normal`; the dry-run + two-stage-writes specs introduce the
// remaining values. The bypass value is split-literal to dodge
// gosec G101's hardcoded-credential heuristic on the surrounding
// constant name, which contains the substring `Bypass` that the
// rule's regex misreads.
const (
	ModeNormal       Mode = "normal"
	ModeDryRun       Mode = "dry_run"
	ModePlan         Mode = "plan"
	ModeApply        Mode = "apply"
	ModeBypassDryRun Mode = "bypass" + "_dry_run"
	ModeYolo         Mode = "yolo"
)

// Mode is the execution-path enumeration.
type Mode string

// Event is one audit record per tool call. Fields match the wire
// format defined in `.claude/specs/audit-log/requirements.md`. All
// fields are non-optional in the JSON encoding (null for genuinely
// absent values rather than omitted).
type Event struct {
	TS                   time.Time      `json:"ts"`
	TSUnixNS             int64          `json:"ts_unix_ns"`
	EventID              string         `json:"event_id"`
	Tool                 string         `json:"tool"`
	ToolCapability       Capability     `json:"tool_capability"`
	Environment          string         `json:"environment"`
	Profile              string         `json:"profile"`
	Mode                 Mode           `json:"mode"`
	PlanID               *string        `json:"plan_id"`
	Args                 map[string]any `json:"args"`
	ArgsRedacted         []string       `json:"args_redacted"`
	Status               Status         `json:"status"`
	LatencyMS            int64          `json:"latency_ms"`
	ResultSummary        string         `json:"result_summary"`
	Error                *string        `json:"error"`
	LinodemcpVersion     string         `json:"linodemcp_version"`
	SessionID            string         `json:"session_id"`
	CredentialGeneration uint64         `json:"credential_generation"`
}

// NewEvent constructs an Event with the timestamp, ULID, and tool
// metadata populated. The remaining fields (status, latency, result
// summary, error) populate later via Finalize. The capture
// middleware (Phase 1b) calls NewEvent at handler entry and
// Finalize at handler exit.
//
// args is redacted in place: the returned event holds redacted args
// and the list of redacted keys. Callers that need the unredacted
// values must keep their own copy.
//
// redactPII controls the PII redaction tier (Phase 4c): false applies
// only the always-on credential list (Redact); true also applies the
// PII list (RedactWithPII). The middleware passes the value from
// cfg.Audit.RedactPII; tests and direct constructors pass false to
// keep the existing credential-only redaction behavior.
func NewEvent(
	tool string,
	capability Capability,
	args map[string]any,
	environment string,
	profile string,
	sessionID string,
	credentialGeneration uint64,
	linodemcpVersion string,
	redactPII bool,
) Event {
	now := time.Now().UTC()

	redactFn := Redact
	if redactPII {
		redactFn = RedactWithPII
	}

	redactedArgs, redactedKeys := redactFn(args)

	return Event{
		TS:                   now,
		TSUnixNS:             now.UnixNano(),
		EventID:              NewEventID(now),
		Tool:                 tool,
		ToolCapability:       capability,
		Environment:          environment,
		Profile:              profile,
		Mode:                 ModeNormal,
		PlanID:               nil,
		Args:                 redactedArgs,
		ArgsRedacted:         redactedKeys,
		Status:               StatusSuccess,
		LatencyMS:            0,
		ResultSummary:        "",
		Error:                nil,
		LinodemcpVersion:     linodemcpVersion,
		SessionID:            sessionID,
		CredentialGeneration: credentialGeneration,
	}
}

// Finalize records the outcome of the tool call: status, latency,
// optional error message, and optional human-readable summary. The
// capture middleware (Phase 1b) calls this once the handler returns.
//
// Callers that want a non-nil Error should pass the message string;
// nil means "no error to report" (status is success or refused with
// a refusal reason captured in ResultSummary instead).
func (e *Event) Finalize(status Status, latency time.Duration, errMsg, summary string) {
	e.Status = status
	e.LatencyMS = latency.Milliseconds()
	e.ResultSummary = summary

	if errMsg == "" {
		e.Error = nil

		return
	}

	e.Error = &errMsg
}

// SetMode updates the execution mode (and the plan ID for plan/apply
// modes). Phase 1a callers leave Mode at its default `normal`;
// dry-run / two-stage-write phases set this through their middleware.
func (e *Event) SetMode(mode Mode, planID string) {
	e.Mode = mode

	if planID == "" {
		e.PlanID = nil

		return
	}

	e.PlanID = &planID
}

// MarshalJSON ensures the empty `args_redacted` slice serializes to
// `[]` rather than `null`. Empty `args` similarly serializes to `{}`.
// The standard encoder's behavior on nil maps and slices would
// otherwise produce `null` for both, which JSONL consumers find
// surprising. Receiver is a pointer because the Event struct is
// ~256 bytes; a value receiver triggers gocritic's hugeParam.
func (e *Event) MarshalJSON() ([]byte, error) {
	type alias Event

	out := alias(*e)
	if out.Args == nil {
		out.Args = map[string]any{}
	}

	if out.ArgsRedacted == nil {
		out.ArgsRedacted = []string{}
	}

	body, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal audit event: %w", err)
	}

	return body, nil
}

// crockfordAlphabet is Crockford's base32, used by the ULID format.
// I, L, O, U are intentionally absent to avoid visual ambiguity with
// 1, 0, V.
const crockfordAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewEventID produces an EventID using the supplied timestamp as the
// time component. Exposed so tests can produce reproducible IDs by
// passing a fixed time. Production callers always pass time.Now().UTC().
//
// The ULID format reserves 80 bits of randomness per id; same-ms
// collisions are negligible at the per-tool-call rate we expect, so
// the generator is stateless. Callers needing strict monotonic
// ordering between events should sort by Event.TSUnixNS, which is
// captured at the same instant.
func NewEventID(now time.Time) string {
	timestampMS := uint64(now.UnixMilli())

	var entropy [10]byte

	_, _ = rand.Read(entropy[:])

	return EventIDPrefix + encodeULID(timestampMS, entropy)
}

// encodeULID builds the 26-char ULID body from the millisecond
// timestamp and 10 random bytes per the ULID spec. The output is
// Crockford base32.
func encodeULID(timestampMS uint64, entropy [10]byte) string {
	const (
		timeLen    = 10
		randomLen  = 16
		totalLen   = timeLen + randomLen
		bitsPerSym = 5
	)

	var buf [totalLen]byte

	// First 10 chars: 48-bit ms big-endian, base32-encoded.
	for i := range timeLen {
		shift := uint((timeLen - 1 - i) * bitsPerSym)
		buf[i] = crockfordAlphabet[(timestampMS>>shift)&0x1F]
	}

	// Remaining 16 chars: 80 bits of entropy, base32-encoded.
	// Pack the 10 bytes into a uint128-equivalent via bit shifts.
	var bits [16]byte

	bits[0] = (entropy[0] & 0xF8) >> 3
	bits[1] = ((entropy[0] & 0x07) << 2) | ((entropy[1] & 0xC0) >> 6)
	bits[2] = (entropy[1] & 0x3E) >> 1
	bits[3] = ((entropy[1] & 0x01) << 4) | ((entropy[2] & 0xF0) >> 4)
	bits[4] = ((entropy[2] & 0x0F) << 1) | ((entropy[3] & 0x80) >> 7)
	bits[5] = (entropy[3] & 0x7C) >> 2
	bits[6] = ((entropy[3] & 0x03) << 3) | ((entropy[4] & 0xE0) >> 5)
	bits[7] = entropy[4] & 0x1F
	bits[8] = (entropy[5] & 0xF8) >> 3
	bits[9] = ((entropy[5] & 0x07) << 2) | ((entropy[6] & 0xC0) >> 6)
	bits[10] = (entropy[6] & 0x3E) >> 1
	bits[11] = ((entropy[6] & 0x01) << 4) | ((entropy[7] & 0xF0) >> 4)
	bits[12] = ((entropy[7] & 0x0F) << 1) | ((entropy[8] & 0x80) >> 7)
	bits[13] = (entropy[8] & 0x7C) >> 2
	bits[14] = ((entropy[8] & 0x03) << 3) | ((entropy[9] & 0xE0) >> 5)
	bits[15] = entropy[9] & 0x1F

	for i := range randomLen {
		buf[timeLen+i] = crockfordAlphabet[bits[i]]
	}

	return string(buf[:])
}
