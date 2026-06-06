package tools_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/audit"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// auditRecentResult mirrors the tool's JSON response so the test can
// decode and assert on it.
type auditRecentResult struct {
	Count  int           `json:"count"`
	Events []audit.Event `json:"events"`
}

// TestLinodeAuditRecentDefinition pins the tool's identity: name,
// CapMeta tag, and the documented filter parameters.

func checkEqual(t *testing.T, want, got any, msgAndArgs ...any) {
	t.Helper()

	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s: got %#v, want %#v", checkMessage(msgAndArgs), got, want)
	}
}

func checkNotEqual(t *testing.T, notWant, got any, msgAndArgs ...any) {
	t.Helper()

	if reflect.DeepEqual(notWant, got) {
		t.Errorf("%s: got %#v, did not want %#v", checkMessage(msgAndArgs), got, notWant)
	}
}

func checkContains(t *testing.T, container, item any, msgAndArgs ...any) {
	t.Helper()

	if !containsValue(container, item) {
		t.Errorf("%s: %#v does not contain %#v", checkMessage(msgAndArgs), container, item)
	}
}

func checkNoConfirm(t *testing.T, container any, msgAndArgs ...any) {
	t.Helper()

	if containsValue(container, keyConfirm) {
		t.Errorf("%s: %#v unexpectedly contains %q", checkMessage(msgAndArgs), container, keyConfirm)
	}
}

func checkTrue(t *testing.T, got bool, msgAndArgs ...any) {
	t.Helper()

	if !got {
		t.Errorf("%s: got false, want true", checkMessage(msgAndArgs))
	}
}

func checkFalse(t *testing.T, got bool, msgAndArgs ...any) {
	t.Helper()

	if got {
		t.Errorf("%s: got true, want false", checkMessage(msgAndArgs))
	}
}

func checkNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()

	if err != nil {
		t.Errorf("%s: unexpected error: %v", checkMessage(msgAndArgs), err)
	}
}

func checkNotEmpty(t *testing.T, got any, msgAndArgs ...any) {
	t.Helper()

	if lengthOf(got) == 0 {
		t.Errorf("%s: got empty value", checkMessage(msgAndArgs))
	}
}

func checkEmpty(t *testing.T, got any, msgAndArgs ...any) {
	t.Helper()

	if lengthOf(got) != 0 {
		t.Errorf("%s: got non-empty value %#v", checkMessage(msgAndArgs), got)
	}
}

func requireNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()

	if err != nil {
		t.Fatalf("%s: unexpected error: %v", checkMessage(msgAndArgs), err)
	}
}

func requireNotNil(t *testing.T, got any, msgAndArgs ...any) {
	t.Helper()

	if isNil(got) {
		t.Fatalf("%s: got nil", checkMessage(msgAndArgs))
	}
}

func requireTrue(t *testing.T, got bool, msgAndArgs ...any) {
	t.Helper()

	if !got {
		t.Fatalf("%s: got false, want true", checkMessage(msgAndArgs))
	}
}

func requireLen(t *testing.T, got any, want int, msgAndArgs ...any) {
	t.Helper()

	gotLen := lengthOf(got)
	if gotLen != want {
		t.Fatalf("%s: got length %d, want %d", checkMessage(msgAndArgs), gotLen, want)
	}
}

func checkLen(t *testing.T, got any, want int, msgAndArgs ...any) {
	t.Helper()

	gotLen := lengthOf(got)
	if gotLen != want {
		t.Errorf("%s: got length %d, want %d", checkMessage(msgAndArgs), gotLen, want)
	}
}

func checkZero(t *testing.T, got any, msgAndArgs ...any) {
	t.Helper()

	if !reflect.ValueOf(got).IsZero() {
		t.Errorf("%s: got non-zero value %#v", checkMessage(msgAndArgs), got)
	}
}

func containsValue(container, item any) bool {
	if typed, ok := container.(string); ok {
		needle, ok := item.(string)

		return ok && strings.Contains(typed, needle)
	}

	reflected := reflect.ValueOf(container)
	if !reflected.IsValid() {
		return false
	}

	kind := reflected.Kind()
	if kind == reflect.Map {
		return reflected.MapIndex(reflect.ValueOf(item)).IsValid()
	}

	if kind == reflect.Slice || kind == reflect.Array {
		for idx := range reflected.Len() {
			if reflect.DeepEqual(reflected.Index(idx).Interface(), item) {
				return true
			}
		}
	}

	return false
}

func lengthOf(got any) int {
	if got == nil {
		return 0
	}

	reflected := reflect.ValueOf(got)

	kind := reflected.Kind()
	if kind == reflect.Array || kind == reflect.Chan || kind == reflect.Map || kind == reflect.Slice || kind == reflect.String {
		return reflected.Len()
	}

	return 0
}

func isNil(got any) bool {
	if got == nil {
		return true
	}

	reflected := reflect.ValueOf(got)

	kind := reflected.Kind()
	if kind == reflect.Chan || kind == reflect.Func || kind == reflect.Interface || kind == reflect.Map || kind == reflect.Pointer || kind == reflect.Slice {
		return reflected.IsNil()
	}

	return false
}

func checkMessage(msgAndArgs []any) string {
	if len(msgAndArgs) == 0 {
		return "check failed"
	}

	format, ok := msgAndArgs[0].(string)
	if !ok {
		return fmt.Sprint(msgAndArgs...)
	}

	if len(msgAndArgs) == 1 {
		return format
	}

	return fmt.Sprintf(format, msgAndArgs[1:]...)
}

func TestLinodeAuditRecentDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeAuditRecentTool(&config.Config{})

	checkEqual(t, "linode_audit_recent", tool.Name, "tool name should match")
	checkEqual(t, profiles.CapMeta, capability, "audit query is CapMeta so every profile can read it")
	requireNotNil(t, handler, "handler should not be nil")

	props := tool.InputSchema.Properties
	for _, param := range []string{"limit", keySince, "until", "tool", "capability", "status", "include_meta"} {
		checkContains(t, props, param, "schema should declare the %q filter", param)
	}

	checkNoConfirm(t, props, "a read-only query must not declare confirm")
}

// TestLinodeAuditRecentReturnsEvents drives the handler end-to-end
// against a temp audit directory (pointed at via XDG_STATE_HOME). It
// confirms the response envelope, newest-first order, and the
// default meta exclusion.
func TestLinodeAuditRecentReturnsEvents(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	auditDir := filepath.Join(stateHome, "linodemcp")
	requireNoError(t, os.MkdirAll(auditDir, 0o750), "create audit dir")

	writeAuditLog(t, filepath.Join(auditDir, "audit.log"), []audit.Event{
		auditEvent("linode_instance_list", audit.CapabilityRead, audit.StatusSuccess, 1),
		auditEvent("linode_audit_recent", audit.CapabilityMeta, audit.StatusSuccess, 2),
		auditEvent("linode_instance_delete", audit.CapabilityDestroy, audit.StatusError, 3),
	})

	_, _, handler := tools.NewLinodeAuditRecentTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	requireNoError(t, err, "handler must not error")
	requireNotNil(t, result, "result must not be nil")
	checkFalse(t, result.IsError, "default query must succeed")

	decoded := decodeAuditResult(t, result)
	checkEqual(t, 2, decoded.Count, "meta event excluded by default leaves two")
	requireLen(t, decoded.Events, 2, "two events returned")
	checkEqual(t, "linode_instance_delete", decoded.Events[0].Tool,
		"newest event (written last) must come first")

	for i := range decoded.Events {
		checkNotEqual(t, audit.CapabilityMeta, decoded.Events[i].ToolCapability,
			"meta events must be excluded without include_meta")
	}
}

// TestLinodeAuditRecentInvalidSince verifies a malformed timestamp
// surfaces as an error result rather than being silently ignored.
func TestLinodeAuditRecentInvalidSince(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeAuditRecentTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keySince: "not-a-timestamp"}))
	requireNoError(t, err, "handler returns the error in the result, not as a Go error")
	requireNotNil(t, result, "result must not be nil")
	checkTrue(t, result.IsError, "a malformed since must produce an error result")

	textContent, ok := result.Content[0].(mcp.TextContent)
	requireTrue(t, ok, "content should be TextContent")
	checkContains(t, textContent.Text, "since", "error should name the bad parameter")
}

// auditEvent builds an event at second `seq` of a fixed minute, so a
// caller passing increasing seq values gets events whose timestamps
// match their write order.
func auditEvent(tool string, capability audit.Capability, status audit.Status, seq int) audit.Event {
	ts := time.Date(2026, time.May, 20, 0, 0, seq, 0, time.UTC)

	return audit.Event{
		TS:             ts,
		TSUnixNS:       ts.UnixNano(),
		EventID:        "evt_" + tool,
		Tool:           tool,
		ToolCapability: capability,
		Status:         status,
	}
}

// writeAuditLog writes events as one JSON line each, in slice order.
func writeAuditLog(t *testing.T, path string, events []audit.Event) {
	t.Helper()

	file, err := os.Create(path) //nolint:gosec // path from test tmp dir
	requireNoError(t, err, "create %s", path)

	defer func() { checkNoError(t, file.Close(), "close %s", path) }()

	encoder := json.NewEncoder(file)
	for i := range events {
		requireNoError(t, encoder.Encode(&events[i]), "encode event %d", i)
	}
}

// decodeAuditResult extracts and JSON-decodes the tool's text result.
func decodeAuditResult(t *testing.T, result *mcp.CallToolResult) auditRecentResult {
	t.Helper()

	textContent, ok := result.Content[0].(mcp.TextContent)
	requireTrue(t, ok, "content should be TextContent")

	var decoded auditRecentResult

	requireNoError(t, json.Unmarshal([]byte(textContent.Text), &decoded), "response must be valid JSON")

	return decoded
}
