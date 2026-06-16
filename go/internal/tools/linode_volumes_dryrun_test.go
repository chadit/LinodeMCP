package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

func TestLinodeVolumeCreateToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeVolumeCreateTool(&config.Config{})
	if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
	}
}

func TestLinodeVolumeCreateToolDryRunPreviewWithoutCreating(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("create dry_run must not issue any request; got %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVolumeCreateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLabel:  "vol-01",
		keyRegion: regionUSEast,
		keyDryRun: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_volume_create") {
		t.Errorf("got %v, want %v", body["tool"], "linode_volume_create")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/volumes") {
		t.Errorf("got %v, want %v", would["path"], "/volumes")
	}

	if body["current_state"] != nil {
		t.Errorf("value = %v, want nil", body["current_state"])
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	effect, gotString := sideEffects[0].(string)
	if !gotString {
		t.Fatal("gotString = false, want true")
	}

	if !strings.Contains(effect, "vol-01") {
		t.Errorf("effect does not contain %v", "vol-01")
	}

	warnings, _ := body["warnings"].([]any)
	if len(warnings) != 1 {
		t.Fatalf("len(warnings) = %d, want %d", len(warnings), 1)
	}
}

func TestLinodeVolumeCreateToolDryRunStillValidatesLabel(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeVolumeCreateTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast,
		keyDryRun: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "label is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "label is required")
	}
}

func TestLinodeVolumeAttachToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeVolumeAttachTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without attaching", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/volumes/333", linode.Volume{ID: 333, Label: testVolumeLabel})
		_, _, handler := tools.NewLinodeVolumeAttachTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVolumeID: float64(333),
			keyLinodeID: float64(444),
			keyDryRun:   true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_volume_attach") {
			t.Errorf("got %v, want %v", body["tool"], "linode_volume_attach")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], "/volumes/333/attach") {
			t.Errorf("got %v, want %v", would["path"], "/volumes/333/attach")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}

		sideEffects, _ := body["side_effects"].([]any)
		if len(sideEffects) != 1 {
			t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
		}

		effect, gotString := sideEffects[0].(string)
		if !gotString {
			t.Fatal("gotString = false, want true")
		}

		if !strings.Contains(effect, "444") {
			t.Errorf("effect does not contain %v", "444")
		}
	})

	t.Run("still validates volume_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVolumeAttachTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(444),
			keyDryRun:   true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "volume_id is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "volume_id is required")
		}
	})
}

func TestLinodeVolumeDetachToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeVolumeDetachTool(&config.Config{})
	if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
	}
}

func TestLinodeVolumeDetachToolDryRunPreviewWithoutDetaching(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunGetStateServer(t, "/volumes/333", linode.Volume{ID: 333, Label: testVolumeLabel})
	_, _, handler := tools.NewLinodeVolumeDetachTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyVolumeID: float64(333),
		keyDryRun:   true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_volume_detach") {
		t.Errorf("got %v, want %v", body["tool"], "linode_volume_detach")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/volumes/333/detach") {
		t.Errorf("got %v, want %v", would["path"], "/volumes/333/detach")
	}

	if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
		t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
	}

	// An unattached volume reports the detach as a no-op.
	if body["side_effects"] == nil {
		t.Fatal("expected non-empty value")
	}
}

func TestLinodeVolumeDetachToolDryRunPreviewSurfacesCurrentAttachment(t *testing.T) {
	t.Parallel()

	attachedTo := 444
	cfg, _ := dryRunGetStateServer(t, "/volumes/333",
		linode.Volume{ID: 333, Label: testVolumeLabel, LinodeID: &attachedTo})
	_, _, handler := tools.NewLinodeVolumeDetachTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyVolumeID: float64(333),
		keyDryRun:   true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	effect, gotString := sideEffects[0].(string)
	if !gotString {
		t.Fatal("gotString = false, want true")
	}

	if !strings.Contains(effect, "444") {
		t.Errorf("effect does not contain %v", "444")
	}
}

func TestLinodeVolumeDetachToolDryRunStillValidatesVolumeId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeVolumeDetachTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "volume_id is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "volume_id is required")
	}
}

func TestLinodeVolumeResizeToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeVolumeResizeTool(&config.Config{})
	if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
	}
}

func TestLinodeVolumeResizeToolDryRunPreviewWithoutResizing(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunGetStateServer(t, "/volumes/333",
		linode.Volume{ID: 333, Label: testVolumeLabel, Size: 50})
	_, _, handler := tools.NewLinodeVolumeResizeTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyVolumeID: float64(333),
		keySize:     float64(100),
		keyDryRun:   true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_volume_resize") {
		t.Errorf("got %v, want %v", body["tool"], "linode_volume_resize")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/volumes/333/resize") {
		t.Errorf("got %v, want %v", would["path"], "/volumes/333/resize")
	}

	if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
		t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	effect, gotString := sideEffects[0].(string)
	if !gotString {
		t.Fatal("gotString = false, want true")
	}

	if !strings.Contains(effect, "50 GB") {
		t.Errorf("effect does not contain %v", "50 GB")
	}

	if !strings.Contains(effect, "100 GB") {
		t.Errorf("effect does not contain %v", "100 GB")
	}

	if body["warnings"] == nil {
		t.Fatal("expected non-empty value")
	}
}

func TestLinodeVolumeResizeToolDryRunStillValidatesVolumeId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeVolumeResizeTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keySize:   float64(100),
		keyDryRun: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "volume_id is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "volume_id is required")
	}
}

func TestLinodeVolumeUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeVolumeUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/volumes/333", linode.Volume{ID: 333, Label: testVolumeLabel})
		_, _, handler := tools.NewLinodeVolumeUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVolumeID: float64(333),
			keyLabel:    testRenamedLabel,
			keyDryRun:   true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_volume_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_volume_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], tcVolumes333) {
			t.Errorf("got %v, want %v", would["path"], tcVolumes333)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}

		sideEffects, _ := body["side_effects"].([]any)
		if len(sideEffects) != 1 {
			t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
		}

		effect, gotString := sideEffects[0].(string)
		if !gotString {
			t.Fatal("gotString = false, want true")
		}

		if !strings.Contains(effect, testRenamedLabel) {
			t.Errorf("effect does not contain %v", testRenamedLabel)
		}
	})

	t.Run("still validates editable field", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVolumeUpdateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVolumeID: float64(333),
			keyDryRun:   true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "at least one of label or tags is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "at least one of label or tags is required")
		}
	})
}

// TestLinodeVolumeDeleteToolDryRunDependencies exercises the Phase 2 Tier A
// walk: a volume attached to an instance surfaces that instance as a
// detached dependency, read straight from the volume state (no extra GET).
func TestLinodeVolumeDeleteToolDryRunDependencies(t *testing.T) {
	t.Parallel()

	linodeID := 456
	attachedLabel := "attached-host"

	cfg, methods := dryRunGetStateServer(t, "/volumes/789", linode.Volume{
		ID:          789,
		Label:       testVolumeLabel,
		LinodeID:    &linodeID,
		LinodeLabel: &attachedLabel,
	})

	_, _, handler := tools.NewLinodeVolumeDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyVolumeID: float64(789),
		keyDryRun:   true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_volume_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_volume_delete")
	}

	deps, _ := body["dependencies"].([]any)
	if len(deps) != 1 {
		t.Fatalf("len(deps) = %d, want %d", len(deps), 1)
	}

	dep, ok := deps[0].(map[string]any)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	for key, want := range map[string]any{
		tcKind:                           tcInstance,
		tcAction:                         "detached",
		keySupportTicketID:               float64(456),
		monitorAlertDefinitionLabelParam: "attached-host",
	} {
		if !reflect.DeepEqual(dep[key], want) {
			t.Errorf("dep[%v] = %v, want %v", key, dep[key], want)
		}
	}

	warnings, _ := body["warnings"].([]any)
	if len(warnings) == 0 {
		t.Error("warnings is empty")
	}

	if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
		t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
	}
}
