package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeVolumeCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeVolumeCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
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
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, true, body[keyDryRun])
		assert.Equal(t, "linode_volume_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/volumes", would["path"])
		assert.Nil(t, body["current_state"], "create has no existing resource to preview")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "create surfaces the new-volume side effect")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, "vol-01", "side effect should name the new volume")

		warnings, _ := body["warnings"].([]any)
		require.Len(t, warnings, 1, "create warns that billing starts immediately")
	})

	t.Run("still validates label", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVolumeCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion: regionUSEast,
			keyDryRun: true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "label is required")
	})
}

func TestLinodeVolumeAttachToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeVolumeAttachTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
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
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_volume_attach", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/volumes/333/attach", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "attach surfaces the attachment side effect")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, "444", "side effect should name the target instance")
	})

	t.Run("still validates volume_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVolumeAttachTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(444),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "volume_id is required")
	})
}

func TestLinodeVolumeDetachToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeVolumeDetachTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without detaching", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/volumes/333", linode.Volume{ID: 333, Label: testVolumeLabel})
		_, _, handler := tools.NewLinodeVolumeDetachTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVolumeID: float64(333),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_volume_detach", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/volumes/333/detach", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		// An unattached volume reports the detach as a no-op.
		assert.NotEmpty(t, body["side_effects"])
	})

	t.Run("preview surfaces current attachment", func(t *testing.T) {
		t.Parallel()

		attachedTo := 444
		cfg, _ := dryRunGetStateServer(t, "/volumes/333",
			linode.Volume{ID: 333, Label: testVolumeLabel, LinodeID: &attachedTo})
		_, _, handler := tools.NewLinodeVolumeDetachTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVolumeID: float64(333),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1)

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, "444", "side effect should name the current instance")
	})

	t.Run("still validates volume_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVolumeDetachTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "volume_id is required")
	})
}

func TestLinodeVolumeResizeToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeVolumeResizeTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without resizing", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/volumes/333",
			linode.Volume{ID: 333, Label: testVolumeLabel, Size: 50})
		_, _, handler := tools.NewLinodeVolumeResizeTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVolumeID: float64(333),
			keySize:     float64(100),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_volume_resize", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/volumes/333/resize", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "resize surfaces the size change")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, "50 GB", "side effect names the current size")
		assert.Contains(t, effect, "100 GB", "side effect names the target size")
		assert.NotEmpty(t, body["warnings"], "resize warns a volume can only grow")
	})

	t.Run("still validates volume_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVolumeResizeTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keySize:   float64(100),
			keyDryRun: true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "volume_id is required")
	})
}

func TestLinodeVolumeUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeVolumeUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
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
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_volume_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, "/volumes/333", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "update surfaces the label change")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, testRenamedLabel, "side effect names the new label")
	})

	t.Run("still validates editable field", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVolumeUpdateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVolumeID: float64(333),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "at least one of label or tags is required")
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
	require.NoError(t, err)
	require.False(t, result.IsError)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	assert.Equal(t, "linode_volume_delete", body["tool"])

	deps, _ := body["dependencies"].([]any)
	require.Len(t, deps, 1, "the attached instance should be the one dependency")

	dep, ok := deps[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "instance", dep["kind"])
	assert.Equal(t, "detached", dep["action"])
	assert.InDelta(t, 456, dep["id"], 0)
	assert.Equal(t, "attached-host", dep["label"])

	warnings, _ := body["warnings"].([]any)
	assert.NotEmpty(t, warnings, "an attached volume should warn about detachment")

	assert.Equal(t, []string{http.MethodGet}, *methods,
		"the walk reads the label from the volume state; no extra GET")
}
