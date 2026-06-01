package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeVolumeCloneTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeVolumeCloneTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_volume_clone", tool.Name)
		assert.NotEmpty(t, tool.Description)
		require.NotNil(t, handler)
		assert.Contains(t, tool.Description, "WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyVolumeID)
		assert.Contains(t, props, keyLabel)
		assert.Contains(t, props, keyConfirm)
		assert.Contains(t, props, keyDryRun)
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseRequiresConfirm, args: map[string]any{keyVolumeID: float64(333), keyLabel: labelDataVol}, wantContains: errConfirmEqualsTrue},
		{name: "confirm false", args: map[string]any{keyVolumeID: float64(333), keyLabel: labelDataVol, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: "confirm string", args: map[string]any{keyVolumeID: float64(333), keyLabel: labelDataVol, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: "confirm numeric", args: map[string]any{keyVolumeID: float64(333), keyLabel: labelDataVol, keyConfirm: float64(1)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingVolumeID, args: map[string]any{keyLabel: labelDataVol, keyConfirm: true}, wantContains: errVolumeIDRequired},
		{name: "negative volume id", args: map[string]any{keyVolumeID: float64(-1), keyLabel: labelDataVol, keyConfirm: true}, wantContains: errVolumeIDPositive},
		{name: "non-integer volume id", args: map[string]any{keyVolumeID: float64(333.5), keyLabel: labelDataVol, keyConfirm: true}, wantContains: errVolumeIDPositive},
		{name: "slash volume id", args: map[string]any{keyVolumeID: "333/clone", keyLabel: labelDataVol, keyConfirm: true}, wantContains: errVolumeIDPositive},
		{name: "query volume id", args: map[string]any{keyVolumeID: "333?debug=true", keyLabel: labelDataVol, keyConfirm: true}, wantContains: errVolumeIDPositive},
		{name: "traversal volume id", args: map[string]any{keyVolumeID: pathTraversalValue, keyLabel: labelDataVol, keyConfirm: true}, wantContains: errVolumeIDPositive},
		{name: caseMissingLabel, args: map[string]any{keyVolumeID: float64(333), keyConfirm: true}, wantContains: errLabelRequired},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.IsError)
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful clone", func(t *testing.T) {
		t.Parallel()

		volume := linode.Volume{ID: 444, Label: labelDataVol, Region: regionUSEast, Status: imageUploadStatusFixture}

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)
			assert.Equal(t, "/volumes/333/clone", r.URL.Path)
			assert.Equal(t, http.MethodPost, r.Method)

			var body map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, labelDataVol, body[keyLabel])

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(volume))
		}))
		t.Cleanup(srv.Close)

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeVolumeCloneTool(successCfg)

		result, err := successHandler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVolumeID: float64(333),
			keyLabel:    labelDataVol,
			keyConfirm:  true,
		}))

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError)
		assert.Equal(t, int32(1), requestCount.Load())

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "cloned successfully")
		assert.Contains(t, textContent.Text, labelDataVol)
	})
}

func TestLinodeVolumeCloneToolDryRun(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		assert.Equal(t, http.MethodGet, r.Method, "dry_run path must only issue GET")
		assert.Equal(t, "/volumes/333", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Volume{ID: 333, Label: testVolumeLabel, Region: regionUSEast}))
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVolumeCloneTool(cfg)

	t.Run("dry_run validates empty label before preview", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			requestCount.Add(1)
			t.Fatalf("dry_run with invalid label must not call the client")
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeVolumeCloneTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVolumeID: float64(333),
			keyLabel:    "",
			keyDryRun:   true,
		}))

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, errLabelRequired)
		assert.Equal(t, int32(0), requestCount.Load())
	})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyVolumeID: float64(333),
		keyLabel:    labelDataVol,
		keyDryRun:   true,
	}))

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError)
	require.Equal(t, []string{http.MethodGet}, methodsSeen)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	assert.Equal(t, true, body[keyDryRun])
	assert.Equal(t, "linode_volume_clone", body["tool"])

	would, _ := body["would_execute"].(map[string]any)
	assert.Equal(t, "POST", would["method"])
	assert.Equal(t, "/volumes/333/clone", would["path"])
}
