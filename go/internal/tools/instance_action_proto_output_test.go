package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The instance action write tools echo the request as proto-canonical JSON.
// These tests pin the exact output field names and values so a regression that
// silently drops a field (or reverts to the legacy struct shape) is caught. The
// message-substring assertions elsewhere miss field-name changes.

// keyOutputMessage is the "message" field key in a proto-canonical tool output.
const keyOutputMessage = "message"

// instanceActionHandler is the handler shape every tool factory returns.
type instanceActionHandler = func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)

// stubAPIConfig returns a config pointing at a test server that answers wantPath
// with 200 and an empty JSON body. The action endpoints return no resource body,
// so the empty response mirrors production.
func stubAPIConfig(t *testing.T, wantPath string) *config.Config {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, wantPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	t.Cleanup(srv.Close)

	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
}

// runActionProtoOutput invokes a handler and decodes its text output into a
// generic map, failing on any handler error or non-object output.
func runActionProtoOutput(t *testing.T, handler instanceActionHandler, args map[string]any) map[string]any {
	t.Helper()

	result, err := handler(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		text, _ := result.Content[0].(mcp.TextContent)
		t.Fatalf("result.IsError = true, want false; text=%q", text.Text)
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	var decoded map[string]any
	if unmarshalErr := json.Unmarshal([]byte(textContent.Text), &decoded); unmarshalErr != nil {
		t.Fatalf("output is not a JSON object: %v; text=%q", unmarshalErr, textContent.Text)
	}

	return decoded
}

func wantOutputString(t *testing.T, m map[string]any, key, want string) {
	t.Helper()

	got, ok := m[key].(string)
	if !ok {
		t.Errorf("output[%q] missing or not a string (have %v)", key, m[key])

		return
	}

	if got != want {
		t.Errorf("output[%q] = %q, want %q", key, got, want)
	}
}

func wantOutputNumber(t *testing.T, m map[string]any, key string, want float64) {
	t.Helper()

	got, ok := m[key].(float64)
	if !ok {
		t.Errorf("output[%q] missing or not a number (have %v)", key, m[key])

		return
	}

	if got != want {
		t.Errorf("output[%q] = %v, want %v", key, got, want)
	}
}

func wantOutputAbsent(t *testing.T, m map[string]any, key string) {
	t.Helper()

	if _, present := m[key]; present {
		t.Errorf("output[%q] present, want absent (value %v)", key, m[key])
	}
}

func TestInstanceBootProtoOutput(t *testing.T) {
	t.Parallel()

	cfg := stubAPIConfig(t, "/linode/instances/123/boot")
	_, _, handler := tools.NewLinodeInstanceBootTool(cfg)

	out := runActionProtoOutput(t, handler, map[string]any{
		keyInstanceID: float64(123), keyConfirm: true,
	})

	wantOutputString(t, out, keyOutputMessage, "Instance 123 boot initiated successfully")
	wantOutputNumber(t, out, keyInstanceID, 123)
	// The power tools echo instance_id, not linode_id.
	wantOutputAbsent(t, out, keyLinodeID)
}

func TestInstanceRebootProtoOutput(t *testing.T) {
	t.Parallel()

	cfg := stubAPIConfig(t, "/linode/instances/123/reboot")
	_, _, handler := tools.NewLinodeInstanceRebootTool(cfg)

	out := runActionProtoOutput(t, handler, map[string]any{
		keyInstanceID: float64(123), keyConfirm: true,
	})

	wantOutputString(t, out, keyOutputMessage, "Instance 123 reboot initiated successfully")
	wantOutputNumber(t, out, keyInstanceID, 123)
	wantOutputAbsent(t, out, keyLinodeID)
}

func TestInstanceShutdownProtoOutput(t *testing.T) {
	t.Parallel()

	cfg := stubAPIConfig(t, "/linode/instances/123/shutdown")
	_, _, handler := tools.NewLinodeInstanceShutdownTool(cfg)

	out := runActionProtoOutput(t, handler, map[string]any{
		keyInstanceID: float64(123), keyConfirm: true,
	})

	wantOutputString(t, out, keyOutputMessage, "Instance 123 shutdown initiated successfully")
	wantOutputNumber(t, out, keyInstanceID, 123)
	wantOutputAbsent(t, out, keyLinodeID)
}

func TestInstanceMigrateProtoOutputWithRegion(t *testing.T) {
	t.Parallel()

	cfg := stubAPIConfig(t, "/linode/instances/123/migrate")
	_, _, handler := tools.NewLinodeInstanceMigrateTool(cfg)

	out := runActionProtoOutput(t, handler, map[string]any{
		keyLinodeID: float64(123), keyRegion: regionUSEast, keyConfirm: true,
	})

	wantOutputString(t, out, keyOutputMessage, "Migration initiated for instance 123 to region us-east")
	wantOutputNumber(t, out, keyLinodeID, 123)
	wantOutputString(t, out, keyRegion, regionUSEast)
}

func TestInstanceMigrateProtoOutputNoRegion(t *testing.T) {
	t.Parallel()

	cfg := stubAPIConfig(t, "/linode/instances/123/migrate")
	_, _, handler := tools.NewLinodeInstanceMigrateTool(cfg)

	out := runActionProtoOutput(t, handler, map[string]any{
		keyLinodeID: float64(123), keyConfirm: true,
	})

	wantOutputString(t, out, keyOutputMessage, "Migration initiated for instance 123")
	wantOutputNumber(t, out, keyLinodeID, 123)
	// region is explicit-presence, omitted when the caller lets Linode pick.
	wantOutputAbsent(t, out, keyRegion)
}

func TestInstanceRescueProtoOutput(t *testing.T) {
	t.Parallel()

	cfg := stubAPIConfig(t, "/linode/instances/123/rescue")
	_, _, handler := tools.NewLinodeInstanceRescueTool(cfg)

	out := runActionProtoOutput(t, handler, map[string]any{
		keyLinodeID: float64(123), keyConfirm: true,
	})

	wantOutputString(t, out, keyOutputMessage, "Instance 123 is booting into rescue mode")
	wantOutputNumber(t, out, keyLinodeID, 123)
}

func TestInstanceResizeProtoOutput(t *testing.T) {
	t.Parallel()

	cfg := stubAPIConfig(t, "/linode/instances/123/resize")
	_, _, handler := tools.NewLinodeInstanceResizeTool(cfg)

	out := runActionProtoOutput(t, handler, map[string]any{
		keyInstanceID: float64(123), keyType: typeG6Standard1, keyConfirm: true,
	})

	wantOutputString(t, out, keyOutputMessage, "Instance 123 resize to g6-standard-1 initiated successfully")
	wantOutputNumber(t, out, keyInstanceID, 123)
	wantOutputString(t, out, "new_type", typeG6Standard1)
}

func TestInstanceBackupsEnableProtoOutput(t *testing.T) {
	t.Parallel()

	cfg := stubAPIConfig(t, "/linode/instances/123/backups/enable")
	_, _, handler := tools.NewLinodeInstanceBackupsEnableTool(cfg)

	out := runActionProtoOutput(t, handler, map[string]any{
		keyLinodeID: float64(123), keyConfirm: true,
	})

	wantOutputString(t, out, keyOutputMessage, "Backup service enabled for instance 123")
	wantOutputNumber(t, out, keyLinodeID, 123)
}

func TestInstanceBackupsCancelProtoOutput(t *testing.T) {
	t.Parallel()

	cfg := stubAPIConfig(t, "/linode/instances/123/backups/cancel")
	_, _, handler := tools.NewLinodeInstanceBackupsCancelTool(cfg)

	out := runActionProtoOutput(t, handler, map[string]any{
		keyLinodeID: float64(123), keyConfirm: true, keyConfirmedDryRun: true,
	})

	wantOutputString(t, out, keyOutputMessage,
		"Backup service canceled for instance 123. All backups have been deleted.")
	wantOutputNumber(t, out, keyLinodeID, 123)
}

func TestInstancePasswordResetProtoOutput(t *testing.T) {
	t.Parallel()

	cfg := stubAPIConfig(t, "/linode/instances/123/password")
	_, _, handler := tools.NewLinodeInstancePasswordResetTool(cfg)

	args := map[string]any{keyLinodeID: float64(123), keyConfirm: true, keyConfirmedDryRun: true}
	args[keyRootPass] = rootPassStrong

	out := runActionProtoOutput(t, handler, args)

	wantOutputString(t, out, keyOutputMessage, "Root password reset for instance 123")
	wantOutputNumber(t, out, keyLinodeID, 123)
}

func TestInstanceDiskPasswordResetProtoOutput(t *testing.T) {
	t.Parallel()

	cfg := stubAPIConfig(t, "/linode/instances/123/disks/10/password")
	_, _, handler := tools.NewLinodeInstanceDiskPasswordResetTool(cfg)

	args := map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyConfirm: true}
	args[keyDiskPassword] = rootPassStrong

	out := runActionProtoOutput(t, handler, args)

	wantOutputString(t, out, keyOutputMessage, "Password reset for disk 10 on instance 123")
	wantOutputNumber(t, out, keyLinodeID, 123)
	wantOutputNumber(t, out, keyDiskID, 10)
}
