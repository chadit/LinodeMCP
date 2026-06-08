package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeVolumeCloneToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeVolumeCloneTool(cfg)

	t.Parallel()

	if tool.Name != "linode_volume_clone" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_volume_clone")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyVolumeID]; !ok {
		t.Errorf("props missing key %v", keyVolumeID)
	}

	if _, ok := props[keyLabel]; !ok {
		t.Errorf("props missing key %v", keyLabel)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	if _, ok := props[keyDryRun]; !ok {
		t.Errorf("props missing key %v", keyDryRun)
	}
}

func TestLinodeVolumeCloneToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVolumeCloneTool(cfg)

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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeVolumeCloneToolSuccessfulClone(t *testing.T) {
	t.Parallel()

	volume := linode.Volume{ID: 444, Label: labelDataVol, Region: regionUSEast, Status: imageUploadStatusFixture}

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.URL.Path != "/volumes/333/clone" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/volumes/333/clone")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body[keyLabel], labelDataVol) {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], labelDataVol)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(volume); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "cloned successfully") {
		t.Errorf("textContent.Text does not contain %v", "cloned successfully")
	}

	if !strings.Contains(textContent.Text, labelDataVol) {
		t.Errorf("textContent.Text does not contain %v", labelDataVol)
	}
}

func TestLinodeVolumeCloneToolDryRun(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcVolumes333 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcVolumes333)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.Volume{ID: 333, Label: testVolumeLabel, Region: regionUSEast}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errLabelRequired) {
			t.Errorf("error text %q does not contain %q", text.Text, errLabelRequired)
		}

		if requestCount.Load() != int32(0) {
			t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(0))
		}
	})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyVolumeID: float64(333),
		keyLabel:    labelDataVol,
		keyDryRun:   true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Fatalf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_volume_clone") {
		t.Errorf("got %v, want %v", body["tool"], "linode_volume_clone")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/volumes/333/clone") {
		t.Errorf("got %v, want %v", would["path"], "/volumes/333/clone")
	}
}
