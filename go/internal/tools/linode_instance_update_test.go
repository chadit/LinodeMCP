package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	instanceUpdateFixtureLabel = "web-1-renamed"
	instanceUpdatePath         = "/linode/instances/123"
	keyWatchdogEnabled         = "watchdog_enabled"
	keyAlerts                  = "alerts"
	keyGroup                   = "group"
	errInstanceIDRequiredMsg   = "instance_id is required"
	wantOneUpdateField         = "at least one update field is required"
)

func TestLinodeInstanceUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeInstanceUpdateTool(cfg)

	if tool.Name != "linode_instance_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_update")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	for _, key := range []string{
		keyInstanceID, keyLabel, keyGroup, keyTags, keyAlerts,
		keyMaintenancePolicy, keyWatchdogEnabled, keyConfirm, keyDryRun,
	} {
		if _, ok := props[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}
}

func TestLinodeInstanceUpdateToolValidation(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("validation failure must not issue any request; got %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeInstanceUpdateTool(cfg)

	cases := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseMissingConfirm,
			args:         map[string]any{keyInstanceID: 123, keyLabel: instanceUpdateFixtureLabel},
			wantContains: "Set confirm=true",
		},
		{
			name:         "missing instance_id",
			args:         map[string]any{keyConfirm: true, keyLabel: instanceUpdateFixtureLabel},
			wantContains: errInstanceIDRequiredMsg,
		},
		{
			name:         caseNoUpdateFields,
			args:         map[string]any{keyConfirm: true, keyInstanceID: 123},
			wantContains: wantOneUpdateField,
		},
		{
			name:         "tags wrong type",
			args:         map[string]any{keyConfirm: true, keyInstanceID: 123, keyTags: canRunEnvProd},
			wantContains: "tags must be an array of strings",
		},
		{
			name:         "tags entry wrong type",
			args:         map[string]any{keyConfirm: true, keyInstanceID: 123, keyTags: []any{canRunEnvProd, 7}},
			wantContains: "tags entries must be strings",
		},
		{
			name:         "alerts wrong type",
			args:         map[string]any{keyConfirm: true, keyInstanceID: 123, keyAlerts: "high"},
			wantContains: "alerts must be an object",
		},
		{
			name:         "watchdog wrong type",
			args:         map[string]any{keyConfirm: true, keyInstanceID: 123, keyWatchdogEnabled: "yes"},
			wantContains: "watchdog_enabled must be a boolean",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, isText := result.Content[0].(mcp.TextContent)
			if !isText {
				t.Fatal("isText = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantContains) {
				t.Errorf("textContent.Text = %v, does not contain %v", textContent.Text, testCase.wantContains)
			}
		})
	}
}

func TestLinodeInstanceUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	updated := linode.Instance{
		ID:              123,
		Label:           instanceUpdateFixtureLabel,
		Status:          statusRunning,
		Type:            typeG6Standard2,
		Region:          regionUSEast,
		Tags:            []string{canRunEnvProd, imageUploadTagWeb},
		WatchdogEnabled: true,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != instanceUpdatePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, instanceUpdatePath)
		}

		var requestBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if requestBody[keyLabel] != instanceUpdateFixtureLabel {
			t.Errorf("got %v, want %v", requestBody[keyLabel], instanceUpdateFixtureLabel)
		}

		if !reflect.DeepEqual(requestBody[keyTags], []any{canRunEnvProd, imageUploadTagWeb}) {
			t.Errorf("got %v, want %v", requestBody[keyTags], []any{canRunEnvProd, imageUploadTagWeb})
		}

		if requestBody[keyWatchdogEnabled] != true {
			t.Errorf("got %v, want %v", requestBody[keyWatchdogEnabled], true)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(updated); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeInstanceUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyInstanceID:      123,
		keyConfirm:         true,
		keyLabel:           instanceUpdateFixtureLabel,
		keyTags:            []any{canRunEnvProd, imageUploadTagWeb},
		keyWatchdogEnabled: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	if !strings.Contains(textContent.Text, "updated successfully") {
		t.Errorf("textContent.Text does not contain %v", "updated successfully")
	}

	if !strings.Contains(textContent.Text, instanceUpdateFixtureLabel) {
		t.Errorf("textContent.Text does not contain %v", instanceUpdateFixtureLabel)
	}
}

func TestLinodeInstanceUpdateToolDryRunPreviewWithoutUpdating(t *testing.T) {
	t.Parallel()

	current := linode.Instance{ID: 123, Label: instanceUpdateFixtureLabel, Status: statusRunning, Type: typeG6Standard2, Region: regionUSEast}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("dry_run must only issue the state GET; got %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		if r.URL.Path != instanceUpdatePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, instanceUpdatePath)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(current); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeInstanceUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyInstanceID: 123,
		keyLabel:      "renamed-again",
		keyDryRun:     true,
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

	if !reflect.DeepEqual(body["tool"], "linode_instance_update") {
		t.Errorf("got %v, want %v", body["tool"], "linode_instance_update")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], http.MethodPut) {
		t.Errorf("got %v, want %v", would["method"], http.MethodPut)
	}

	if !reflect.DeepEqual(would["path"], instanceUpdatePath) {
		t.Errorf("got %v, want %v", would["path"], instanceUpdatePath)
	}

	state, isMap := body["current_state"].(map[string]any)
	if !isMap {
		t.Fatal("current_state is not the fetched instance object")
	}

	if state[keyLabel] != instanceUpdateFixtureLabel {
		t.Errorf("got %v, want %v", state[keyLabel], instanceUpdateFixtureLabel)
	}
}

func TestLinodeInstanceUpdateToolDryRunRequiresInstanceID(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeInstanceUpdateTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLabel:  instanceUpdateFixtureLabel,
		keyDryRun: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	if !strings.Contains(textContent.Text, errInstanceIDRequiredMsg) {
		t.Errorf("textContent.Text does not contain %v", errInstanceIDRequiredMsg)
	}
}
