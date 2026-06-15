package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/internal/cli"
	"github.com/chadit/LinodeMCP/internal/server"
)

const (
	toolVersion = "version"
	toolHello   = "hello"
	exitUsage   = 2
)

// TestCallVersionSucceeds drives the real call path for the version meta
// tool: exit 0 and the payload carries the version field. version needs
// no token, so this runs without a fake API.
func TestCallVersionSucceeds(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand([]string{toolVersion}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	wantContains(t, "stdout", stdout.String(), `"version"`)
}

// TestCallMatchesDirectDispatch is the core parity check: a call through
// the CLI produces the same result payload as driving the server's
// HandleMessage directly. Both paths go through the same dispatch, so the
// CLI must add nothing but framing and extraction. version is used
// because its payload is stable across two calls in the same run.
func TestCallMatchesDirectDispatch(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	if code := cli.RunCallCommand([]string{toolVersion}, &stdout, &stderr); code != 0 {
		t.Fatalf("call exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	cliPayload := decodeJSONObject(t, stdout.String())

	srv := newTestServer(t)
	directPayload := decodeJSONObject(t, directDispatchText(t, srv, toolVersion))

	if cliPayload["version"] != directPayload["version"] {
		t.Errorf(
			"version mismatch: cli=%v direct=%v",
			cliPayload["version"], directPayload["version"],
		)
	}

	if cliPayload["platform"] != directPayload["platform"] {
		t.Errorf(
			"platform mismatch: cli=%v direct=%v",
			cliPayload["platform"], directPayload["platform"],
		)
	}
}

// TestCallHelloPassesArg checks an argument reaches the tool: calling
// hello with name=Ada returns a greeting containing the name.
func TestCallHelloPassesArg(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand([]string{toolHello, "--arg", "name=Ada"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	wantContains(t, "stdout", stdout.String(), "Ada")
}

// TestCallUnknownToolExitsUsage checks that an unregistered tool name is
// caught before dispatch and exits with the usage code (2), naming the
// tool on stderr.
func TestCallUnknownToolExitsUsage(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand([]string{"linode_does_not_exist"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}

	wantContains(t, "stderr", stderr.String(), "unknown tool")
}

// TestCallNoToolExitsUsage checks that omitting the tool name prints
// usage and exits 2.
func TestCallNoToolExitsUsage(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand(nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
}

// TestCallArgAndJSONExitsUsage checks that supplying both --arg and
// --json is rejected with the usage code before dispatch.
func TestCallArgAndJSONExitsUsage(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand(
		[]string{toolHello, "--arg", "name=Ada", "--json", `{"name":"Bob"}`},
		&stdout, &stderr,
	)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
}

// TestCallBadJSONExitsUsage checks that malformed --json is a usage error.
func TestCallBadJSONExitsUsage(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand([]string{toolHello, "--json", "{bad"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
}

// TestCallBadOutputExitsUsage checks that an unrecognized --output value
// is a usage error, listing the accepted formats.
func TestCallBadOutputExitsUsage(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand([]string{toolVersion, "--output", "xml"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}

	wantContains(t, "stderr", stderr.String(), "json or table")
}

// TestCallDryRunFlagAccepted checks the safety-flag wiring: --dry-run
// parses and folds into the request without breaking the call. version
// ignores the field, so the call still succeeds; the point is that the
// flag is accepted and the request stays valid.
func TestCallDryRunFlagAccepted(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand([]string{toolVersion, "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	wantContains(t, "stdout", stdout.String(), `"version"`)
}

// TestCallBadBoolFlagExitsUsage checks that a non-boolean value for a
// boolean safety flag (--dry-run=maybe) is a usage error, caught during
// flag parsing before dispatch.
func TestCallBadBoolFlagExitsUsage(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand([]string{toolVersion, "--dry-run=maybe"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
}

// directDispatchText drives one tools/call straight through the server's
// HandleMessage and returns the result's text payload, so a test can
// compare the CLI's output against the raw dispatch. Mirrors the wire
// walking the integration test does: the MCP result uses camelCase keys,
// so the response is read as generic maps.
func directDispatchText(t *testing.T, srv *server.Server, tool string) string {
	t.Helper()

	message, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": tool, "arguments": map[string]any{}},
	})
	if err != nil {
		t.Fatalf("marshal %s request: %v", tool, err)
	}

	raw, err := json.Marshal(srv.HandleMessage(t.Context(), message))
	if err != nil {
		t.Fatalf("marshal %s response: %v", tool, err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal %s response: %v", tool, err)
	}

	result, isMap := decoded["result"].(map[string]any)
	if !isMap {
		t.Fatalf("%s response has no result: %s", tool, raw)
	}

	content, isSlice := result["content"].([]any)
	if !isSlice || len(content) == 0 {
		t.Fatalf("%s returned no content: %s", tool, raw)
	}

	first, isMap := content[0].(map[string]any)
	if !isMap {
		t.Fatalf("%s first content block is not an object: %v", tool, content[0])
	}

	text, _ := first["text"].(string)

	return text
}

// decodeJSONObject parses a JSON object payload into a generic map.
func decodeJSONObject(t *testing.T, payload string) map[string]any {
	t.Helper()

	trimmed := strings.TrimSpace(payload)

	var obj map[string]any
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		t.Fatalf("decode payload %q: %v", trimmed, err)
	}

	return obj
}
