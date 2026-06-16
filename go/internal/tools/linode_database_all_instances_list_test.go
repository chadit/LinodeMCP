package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// databaseAllInstancesPath is the cross-engine list endpoint, distinct from
// the per-engine /databases/mysql/instances and /databases/postgresql/instances.
const databaseAllInstancesPath = "/databases/instances"

func TestLinodeDatabaseAllInstancesListToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabaseAllInstancesListTool(cfg)

	if tool.Name != "linode_database_instance_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_instance_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyPage]; !ok {
		t.Errorf("props missing key %v", keyPage)
	}

	if _, ok := props[keyPageSize]; !ok {
		t.Errorf("props missing key %v", keyPageSize)
	}

	if _, ok := props[keyConfirm]; ok {
		t.Errorf("props has unexpected key %v", keyConfirm)
	}
}

func TestLinodeDatabaseAllInstancesListToolSuccess(t *testing.T) {
	t.Parallel()

	instances := []linode.DatabaseInstance{{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineName, Version: databaseVersion, Status: statusActive}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != databaseAllInstancesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseAllInstancesPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    instances,
			keyPage:    2,
			keyPages:   3,
			keyResults: 51,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseAllInstancesListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	if !strings.Contains(textContent.Text, databaseInstanceLabel) {
		t.Errorf("textContent.Text does not contain %v", databaseInstanceLabel)
	}

	if !strings.Contains(textContent.Text, databaseEngineName) {
		t.Errorf("textContent.Text does not contain %v", databaseEngineName)
	}
}

func TestLinodeDatabaseAllInstancesListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databaseAllInstancesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseAllInstancesPath)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: temporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseAllInstancesListTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve Managed Database instances across engines") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve Managed Database instances across engines")
	}
}

func TestLinodeDatabaseAllInstancesListToolRejectsInvalidPagination(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("validation failure must not issue any request; got %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseAllInstancesListTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyPageSize: 7}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}
