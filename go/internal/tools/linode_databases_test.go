package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	temporaryFailure = "temporary failure"

	databaseEnginesPath                       = "/databases/engines"
	databaseTypesPath                         = "/databases/types"
	databaseTypeID                            = "g6-dedicated-1"
	databaseTypeIDParam                       = "type_id"
	databaseTypeEscapedPath                   = "/databases/types/g6-dedicated-1"
	databaseTypeIDRequiredMessage             = "type_id must be a non-empty string"
	databaseTypeIDSeparatorMessage            = "type_id must not contain separators, query, fragment, or traversal segments"
	databaseTypeLabel                         = "DBaaS - Dedicated 80GB"
	databaseEngineEscapedPath                 = "/databases/engines/mysql%2F8.0.26"
	databaseEngineID                          = "mysql/8.0.26"
	databaseEngineIDParam                     = "engine_id"
	databaseEngineIDRequiredMessage           = "engine_id must be a non-empty string"
	databaseEngineIDShapeMessage              = "engine_id must use the engine/version format"
	databaseEngineIDSeparatorMessage          = "engine_id must not contain query, fragment, or traversal segments"
	databaseEngineName                        = "mysql"
	databaseVersion                           = "8.0.26"
	databaseInstancesPath                     = "/databases/mysql/instances"
	databasePostgreSQLInstancesPath           = "/databases/postgresql/instances"
	databaseMySQLConfigPath                   = "/databases/mysql/config"
	databasePostgreSQLConfigPath              = "/databases/postgresql/config"
	databaseInstanceID                        = 123
	databaseInstanceIDParam                   = "instance_id"
	databaseInstanceIDMessage                 = "instance_id must be a positive integer"
	databaseInstancePath                      = "/databases/mysql/instances/123"
	databasePostgreSQLInstancePath            = "/databases/postgresql/instances/123"
	databasePostgreSQLInstancePatchPath       = "/databases/postgresql/instances/123/patch"
	databaseInstanceSSLPath                   = "/databases/mysql/instances/123/ssl"
	databasePostgreSQLInstanceSSLPath         = "/databases/postgresql/instances/123/ssl"
	databaseSSLCACertificate                  = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t"
	databaseInstanceCredentialsPath           = "/databases/mysql/instances/123/credentials"
	databasePostgreSQLInstanceCredentialsPath = "/databases/postgresql/instances/123/credentials"
	databaseInstanceCredentialsResetPath      = "/databases/mysql/instances/123/credentials/reset"
	databasePostgreSQLCredentialsResetPath    = "/databases/postgresql/instances/123/credentials/reset"
	databaseInstancePatchPath                 = "/databases/mysql/instances/123/patch"
	databaseInstanceSuspendPath               = "/databases/mysql/instances/123/suspend"
	databasePostgreSQLInstanceSuspendPath     = "/databases/postgresql/instances/123/suspend"
	databaseInstanceResumePath                = "/databases/mysql/instances/123/resume"
	databasePostgreSQLInstanceResumePath      = "/databases/postgresql/instances/123/resume"
	databaseInstanceLabel                     = "primary-db"
	databaseInstanceType                      = typeG6Standard2
	databaseCredentialsPassword               = "secret"
	databaseConfigMaxConnections              = "max_connections"
	caseStringInstanceID                      = "string instance id"
	caseZeroInstanceID                        = "zero instance id"
	caseNegativeInstanceID                    = "negative instance id"
	caseFractionalInstanceID                  = "fractional instance id"
	caseSlashInstanceID                       = "slash instance id"
	caseTraversalInstanceID                   = "traversal instance id"
	databaseEngineParam                       = "engine"
	databaseInvalidInstanceIDQuery            = "123?x=1"
	databaseInvalidAPIURL                     = "https://example.invalid"
	caseQueryInstanceID                       = "query instance id"
	databaseAllowListParam                    = "allow_list"
	databaseEngineConfigParam                 = "engine_config"
	databasePrivateNetworkParam               = "private_network"
	databaseUpdatesParam                      = "updates"
	databaseVersionParam                      = "version"
	databaseAllowListNotArray                 = "allow_list must be an array of strings"
	databaseEngineConfigNotObject             = "engine_config must be an object"
	databasePrivateNetworkNotObject           = "private_network must be an object"
	databaseUpdatesNotObject                  = "updates must be an object"
	databasePostgreSQLConfigNamespace         = "pg"
	databaseJSONNull                          = "null"
	databaseJSONArray                         = "[]"
	caseFalseConfirm                          = "false confirm"
	caseStringConfirm                         = "string confirm"
	caseNumericConfirm                        = "numeric confirm"
	invalidJSON                               = "not-json"
	databaseEnginePostgreSQLID                = "postgresql/16"
	databaseEnginePostgreSQL                  = "postgresql"
	databaseSSLConnectionParam                = "ssl_connection"
	databaseLabelRequiredMessage              = "label must be a non-empty string"
	caseInvalidAllowList                      = "invalid allow list"
	caseInvalidEngineConfig                   = "invalid engine config"
	caseInvalidPrivateNetwork                 = "invalid private network"
)

func TestLinodeDatabaseEngineListToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabaseEngineListTool(cfg)

	if tool.Name != "linode_database_engine_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_engine_list")
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

func TestLinodeDatabaseEngineListToolSuccess(t *testing.T) {
	t.Parallel()

	engines := []linode.DatabaseEngine{{ID: databaseEngineID, Engine: databaseEngineName, Version: databaseVersion}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != databaseEnginesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseEnginesPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    engines,
			keyPage:    2,
			keyPages:   3,
			keyResults: 51,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseEngineListTool(cfg)

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

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, databaseEngineID) {
		t.Errorf("textContent.Text does not contain %v", databaseEngineID)
	}

	if !strings.Contains(textContent.Text, databaseEngineName) {
		t.Errorf("textContent.Text does not contain %v", databaseEngineName)
	}
}

func TestLinodeDatabaseEngineListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databaseEnginesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseEnginesPath)
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
	_, _, handler := tools.NewLinodeDatabaseEngineListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve Managed Database engines") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve Managed Database engines")
	}
}

func TestLinodeDatabaseEngineListToolClientConfigurationError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseEngineListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeDatabaseEngineListToolPaginationValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseEngineListTool(cfg)

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeDatabaseTypeListToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabaseTypeListTool(cfg)

	if tool.Name != "linode_database_type_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_type_list")
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

func TestLinodeDatabaseTypeListToolSuccess(t *testing.T) {
	t.Parallel()

	types := []linode.DatabaseType{{
		ID:     databaseTypeID,
		Label:  databaseTypeLabel,
		Class:  "dedicated",
		Disk:   25600,
		Memory: 1024,
		VCPUs:  1,
		Engines: linode.DatabaseTypeEngines{
			MySQL: []linode.DatabaseTypeEngine{{Quantity: 1, Price: linode.Price{Hourly: 0.03, Monthly: 20}}},
		},
	}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != databaseTypesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseTypesPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    types,
			keyPage:    2,
			keyPages:   3,
			keyResults: 51,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseTypeListTool(cfg)

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

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, databaseTypeID) {
		t.Errorf("textContent.Text does not contain %v", databaseTypeID)
	}

	if !strings.Contains(textContent.Text, databaseTypeLabel) {
		t.Errorf("textContent.Text does not contain %v", databaseTypeLabel)
	}
}

func TestLinodeDatabaseTypeListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databaseTypesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseTypesPath)
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
	_, _, handler := tools.NewLinodeDatabaseTypeListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve Managed Database types") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve Managed Database types")
	}
}

func TestLinodeDatabaseTypeListToolClientConfigurationError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseTypeListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeDatabaseTypeListToolPaginationValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseTypeListTool(cfg)

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeDatabaseTypeGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabaseTypeGetTool(cfg)

	if tool.Name != "linode_database_type_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_type_get")
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

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, databaseTypeIDParam) {
		t.Errorf("rawSchema missing key %v", databaseTypeIDParam)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("rawSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeDatabaseTypeGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != databaseTypeEscapedPath {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), databaseTypeEscapedPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.DatabaseType{ID: databaseTypeID, Label: databaseTypeLabel}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseTypeGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseTypeIDParam: databaseTypeID, keyPage: 2, keyPageSize: 25})

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

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, databaseTypeID) {
		t.Errorf("textContent.Text does not contain %v", databaseTypeID)
	}

	if !strings.Contains(textContent.Text, databaseTypeLabel) {
		t.Errorf("textContent.Text does not contain %v", databaseTypeLabel)
	}
}

func TestLinodeDatabaseTypeGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != databaseTypeEscapedPath {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), databaseTypeEscapedPath)
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
	_, _, handler := tools.NewLinodeDatabaseTypeGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseTypeIDParam: databaseTypeID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve Managed Database type") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve Managed Database type")
	}
}

func TestLinodeDatabaseTypeGetToolTypeIdValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseTypeGetTool(cfg)

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: "missing type id", args: map[string]any{}, wantMessage: databaseTypeIDRequiredMessage},
		{name: "numeric type id", args: map[string]any{databaseTypeIDParam: 123}, wantMessage: databaseTypeIDRequiredMessage},
		{name: "blank type id", args: map[string]any{databaseTypeIDParam: ""}, wantMessage: databaseTypeIDRequiredMessage},
		{name: "slash type id", args: map[string]any{databaseTypeIDParam: "g6/dedicated-1"}, wantMessage: databaseTypeIDSeparatorMessage},
		{name: "query type id", args: map[string]any{databaseTypeIDParam: "g6-dedicated-1?x=1"}, wantMessage: databaseTypeIDSeparatorMessage},
		{name: "fragment type id", args: map[string]any{databaseTypeIDParam: "g6-dedicated-1#frag"}, wantMessage: databaseTypeIDSeparatorMessage},
		{name: "traversal type id", args: map[string]any{databaseTypeIDParam: "g6-..-1"}, wantMessage: databaseTypeIDSeparatorMessage},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeDatabaseMySQLConfigGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabaseMySQLConfigGetTool(cfg)

	if tool.Name != "linode_database_mysql_config_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_mysql_config_get")
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
	if _, ok := props[keyConfirm]; ok {
		t.Errorf("props has unexpected key %v", keyConfirm)
	}
}

func TestLinodeDatabaseMySQLConfigGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != databaseMySQLConfigPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseMySQLConfigPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			"mysql": map[string]any{
				"connect_timeout": map[string]any{
					keyDescription:     "The number of seconds that the mysqld server waits for a connect packet.",
					"example":          10,
					"requires_restart": false,
					keyType:            "integer",
				},
			},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseMySQLConfigGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

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

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "connect_timeout") {
		t.Errorf("textContent.Text does not contain %v", "connect_timeout")
	}

	if !strings.Contains(textContent.Text, "requires_restart") {
		t.Errorf("textContent.Text does not contain %v", "requires_restart")
	}
}

func TestLinodeDatabaseMySQLConfigGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databaseMySQLConfigPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseMySQLConfigPath)
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
	_, _, handler := tools.NewLinodeDatabaseMySQLConfigGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve MySQL Managed Database advanced parameters") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve MySQL Managed Database advanced parameters")
	}
}

func TestLinodeDatabaseMySQLConfigGetToolClientConfigurationError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseMySQLConfigGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeDatabasePostgreSQLConfigGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabasePostgreSQLConfigGetTool(cfg)

	if tool.Name != "linode_database_postgresql_config_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_postgresql_config_get")
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
	if _, ok := props[keyConfirm]; ok {
		t.Errorf("props has unexpected key %v", keyConfirm)
	}
}

func TestLinodeDatabasePostgreSQLConfigGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != databasePostgreSQLConfigPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLConfigPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			databasePostgreSQLConfigNamespace: map[string]any{
				databaseConfigMaxConnections: map[string]any{
					keyDescription:     "Sets the maximum number of concurrent connections.",
					"example":          100,
					"requires_restart": false,
					keyType:            "integer",
				},
			},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLConfigGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

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

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, databaseConfigMaxConnections) {
		t.Errorf("textContent.Text does not contain %v", databaseConfigMaxConnections)
	}

	if !strings.Contains(textContent.Text, "requires_restart") {
		t.Errorf("textContent.Text does not contain %v", "requires_restart")
	}
}

func TestLinodeDatabasePostgreSQLConfigGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databasePostgreSQLConfigPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLConfigPath)
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
	_, _, handler := tools.NewLinodeDatabasePostgreSQLConfigGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve PostgreSQL Managed Database advanced parameters") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve PostgreSQL Managed Database advanced parameters")
	}
}

func TestLinodeDatabasePostgreSQLConfigGetToolClientConfigurationError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLConfigGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeDatabaseInstanceListToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabaseInstanceListTool(cfg)

	if tool.Name != "linode_database_mysql_instance_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_mysql_instance_list")
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

func TestLinodeDatabaseInstanceListToolSuccess(t *testing.T) {
	t.Parallel()

	instances := []linode.DatabaseInstance{{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineName, Version: databaseVersion, Status: statusActive}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != databaseInstancesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancesPath)
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
	_, _, handler := tools.NewLinodeDatabaseInstanceListTool(cfg)

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

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, databaseInstanceLabel) {
		t.Errorf("textContent.Text does not contain %v", databaseInstanceLabel)
	}

	if !strings.Contains(textContent.Text, databaseEngineName) {
		t.Errorf("textContent.Text does not contain %v", databaseEngineName)
	}
}

func TestLinodeDatabaseInstanceListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databaseInstancesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancesPath)
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
	_, _, handler := tools.NewLinodeDatabaseInstanceListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve Managed Database instances") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve Managed Database instances")
	}
}

func TestLinodeDatabaseInstanceListToolClientConfigurationError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseInstanceListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeDatabaseInstanceListToolPaginationValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseInstanceListTool(cfg)

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeDatabasePostgreSQLInstanceListToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabasePostgreSQLInstanceListTool(cfg)

	if tool.Name != "linode_database_postgresql_instance_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_postgresql_instance_list")
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

func TestLinodeDatabasePostgreSQLInstanceListToolSuccess(t *testing.T) {
	t.Parallel()

	instances := []linode.DatabaseInstance{{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEnginePostgreSQL, Version: databaseVersion, Status: statusActive}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != databasePostgreSQLInstancesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstancesPath)
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
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceListTool(cfg)

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

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, databaseInstanceLabel) {
		t.Errorf("textContent.Text does not contain %v", databaseInstanceLabel)
	}

	if !strings.Contains(textContent.Text, databaseEnginePostgreSQL) {
		t.Errorf("textContent.Text does not contain %v", databaseEnginePostgreSQL)
	}
}

func TestLinodeDatabasePostgreSQLInstanceListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databasePostgreSQLInstancesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstancesPath)
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
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve PostgreSQL Managed Database instances") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve PostgreSQL Managed Database instances")
	}
}

func TestLinodeDatabasePostgreSQLInstanceListToolClientConfigurationError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeDatabasePostgreSQLInstanceListToolPaginationValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceListTool(cfg)

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeDatabaseInstanceGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabaseInstanceGetTool(cfg)

	if tool.Name != "linode_database_mysql_instance_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_mysql_instance_get")
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
	if _, ok := props[databaseInstanceIDParam]; !ok {
		t.Errorf("props missing key %v", databaseInstanceIDParam)
	}

	if _, ok := props[keyConfirm]; ok {
		t.Errorf("props has unexpected key %v", keyConfirm)
	}
}

func TestLinodeDatabaseInstanceGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != databaseInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineName, Version: databaseVersion, Status: statusActive}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstanceGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID})

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

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, databaseInstanceLabel) {
		t.Errorf("textContent.Text does not contain %v", databaseInstanceLabel)
	}

	if !strings.Contains(textContent.Text, databaseEngineName) {
		t.Errorf("textContent.Text does not contain %v", databaseEngineName)
	}
}

func TestLinodeDatabaseInstanceGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databaseInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancePath)
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
	_, _, handler := tools.NewLinodeDatabaseInstanceGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve MySQL Managed Database instance") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve MySQL Managed Database instance")
	}
}

func TestLinodeDatabaseInstanceGetToolClientConfigurationError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseInstanceGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeDatabaseInstanceGetToolInstanceIdValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseInstanceGetTool(cfg)

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingInstanceID, args: map[string]any{}},
		{name: caseStringInstanceID, args: map[string]any{databaseInstanceIDParam: "123"}},
		{name: caseZeroInstanceID, args: map[string]any{databaseInstanceIDParam: 0}},
		{name: caseNegativeInstanceID, args: map[string]any{databaseInstanceIDParam: -1}},
		{name: caseFractionalInstanceID, args: map[string]any{databaseInstanceIDParam: 123.4}},
		{name: caseSlashInstanceID, args: map[string]any{databaseInstanceIDParam: "/"}},
		{name: caseQueryInstanceID, args: map[string]any{databaseInstanceIDParam: databaseInvalidInstanceIDQuery}},
		{name: caseTraversalInstanceID, args: map[string]any{databaseInstanceIDParam: pathTraversalValue}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, databaseInstanceIDMessage) {
				t.Errorf("textContent.Text does not contain %v", databaseInstanceIDMessage)
			}
		})
	}
}

func TestLinodeDatabasePostgreSQLInstanceGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabasePostgreSQLInstanceGetTool(cfg)

	if tool.Name != "linode_database_postgresql_instance_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_postgresql_instance_get")
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
	if _, ok := props[databaseInstanceIDParam]; !ok {
		t.Errorf("props missing key %v", databaseInstanceIDParam)
	}

	if _, ok := props[keyConfirm]; ok {
		t.Errorf("props has unexpected key %v", keyConfirm)
	}
}

func TestLinodeDatabasePostgreSQLInstanceGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != databasePostgreSQLInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstancePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: "postgresql", Version: databaseVersion, Status: statusActive}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID})

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

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, databaseInstanceLabel) {
		t.Errorf("textContent.Text does not contain %v", databaseInstanceLabel)
	}

	if !strings.Contains(textContent.Text, "postgresql") {
		t.Errorf("textContent.Text does not contain %v", "postgresql")
	}
}

func TestLinodeDatabasePostgreSQLInstanceGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databasePostgreSQLInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstancePath)
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
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve PostgreSQL Managed Database instance") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve PostgreSQL Managed Database instance")
	}
}

func TestLinodeDatabasePostgreSQLInstanceGetToolInstanceIdValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceGetTool(cfg)

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingInstanceID, args: map[string]any{}},
		{name: caseStringInstanceID, args: map[string]any{databaseInstanceIDParam: "123"}},
		{name: caseZeroInstanceID, args: map[string]any{databaseInstanceIDParam: 0}},
		{name: caseNegativeInstanceID, args: map[string]any{databaseInstanceIDParam: -1}},
		{name: caseFractionalInstanceID, args: map[string]any{databaseInstanceIDParam: 123.4}},
		{name: caseSlashInstanceID, args: map[string]any{databaseInstanceIDParam: "/"}},
		{name: caseQueryInstanceID, args: map[string]any{databaseInstanceIDParam: databaseInvalidInstanceIDQuery}},
		{name: caseTraversalInstanceID, args: map[string]any{databaseInstanceIDParam: pathTraversalValue}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, databaseInstanceIDMessage) {
				t.Errorf("textContent.Text does not contain %v", databaseInstanceIDMessage)
			}
		})
	}
}

func TestLinodeDatabaseInstanceSSLGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabaseInstanceSSLGetTool(cfg)

	if tool.Name != "linode_database_mysql_instance_ssl_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_mysql_instance_ssl_get")
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

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, databaseInstanceIDParam) {
		t.Errorf("RawInputSchema missing key %v", databaseInstanceIDParam)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeDatabaseInstanceSSLGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != databaseInstanceSSLPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceSSLPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.DatabaseSSL{CACertificate: databaseSSLCACertificate}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstanceSSLGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID})

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

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, databaseSSLCACertificate) {
		t.Errorf("textContent.Text does not contain %v", databaseSSLCACertificate)
	}

	if !strings.Contains(textContent.Text, "ca_certificate") {
		t.Errorf("textContent.Text does not contain %v", "ca_certificate")
	}
}

func TestLinodeDatabaseInstanceSSLGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databaseInstanceSSLPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceSSLPath)
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
	_, _, handler := tools.NewLinodeDatabaseInstanceSSLGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve MySQL Managed Database SSL certificate") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve MySQL Managed Database SSL certificate")
	}
}

func TestLinodeDatabaseInstanceSSLGetToolClientConfigurationError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseInstanceSSLGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeDatabaseInstanceSSLGetToolInstanceIdValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseInstanceSSLGetTool(cfg)

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingInstanceID, args: map[string]any{}},
		{name: caseStringInstanceID, args: map[string]any{databaseInstanceIDParam: "123"}},
		{name: caseZeroInstanceID, args: map[string]any{databaseInstanceIDParam: 0}},
		{name: caseNegativeInstanceID, args: map[string]any{databaseInstanceIDParam: -1}},
		{name: caseFractionalInstanceID, args: map[string]any{databaseInstanceIDParam: 123.4}},
		{name: caseSlashInstanceID, args: map[string]any{databaseInstanceIDParam: "/"}},
		{name: caseQueryInstanceID, args: map[string]any{databaseInstanceIDParam: databaseInvalidInstanceIDQuery}},
		{name: caseTraversalInstanceID, args: map[string]any{databaseInstanceIDParam: pathTraversalValue}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, databaseInstanceIDMessage) {
				t.Errorf("textContent.Text does not contain %v", databaseInstanceIDMessage)
			}
		})
	}
}

func TestLinodeDatabasePostgreSQLInstanceSSLGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabasePostgreSQLInstanceSSLGetTool(cfg)

	if tool.Name != "linode_database_postgresql_instance_ssl_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_postgresql_instance_ssl_get")
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

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, databaseInstanceIDParam) {
		t.Errorf("RawInputSchema missing key %v", databaseInstanceIDParam)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeDatabasePostgreSQLInstanceSSLGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != databasePostgreSQLInstanceSSLPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstanceSSLPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.DatabaseSSL{CACertificate: databaseSSLCACertificate}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceSSLGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID})

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

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, databaseSSLCACertificate) {
		t.Errorf("textContent.Text does not contain %v", databaseSSLCACertificate)
	}

	if !strings.Contains(textContent.Text, "ca_certificate") {
		t.Errorf("textContent.Text does not contain %v", "ca_certificate")
	}
}

func TestLinodeDatabasePostgreSQLInstanceSSLGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databasePostgreSQLInstanceSSLPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstanceSSLPath)
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
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceSSLGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve PostgreSQL Managed Database SSL certificate") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve PostgreSQL Managed Database SSL certificate")
	}
}

func TestLinodeDatabasePostgreSQLInstanceSSLGetToolClientConfigurationError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceSSLGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeDatabasePostgreSQLInstanceSSLGetToolInstanceIdValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceSSLGetTool(cfg)

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingInstanceID, args: map[string]any{}},
		{name: caseStringInstanceID, args: map[string]any{databaseInstanceIDParam: "123"}},
		{name: caseZeroInstanceID, args: map[string]any{databaseInstanceIDParam: 0}},
		{name: caseNegativeInstanceID, args: map[string]any{databaseInstanceIDParam: -1}},
		{name: caseFractionalInstanceID, args: map[string]any{databaseInstanceIDParam: 123.4}},
		{name: caseSlashInstanceID, args: map[string]any{databaseInstanceIDParam: "/"}},
		{name: caseQueryInstanceID, args: map[string]any{databaseInstanceIDParam: databaseInvalidInstanceIDQuery}},
		{name: caseTraversalInstanceID, args: map[string]any{databaseInstanceIDParam: pathTraversalValue}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, databaseInstanceIDMessage) {
				t.Errorf("textContent.Text does not contain %v", databaseInstanceIDMessage)
			}
		})
	}
}

func TestLinodeDatabaseInstanceCredentialsGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabaseInstanceCredentialsGetTool(cfg)

	if tool.Name != "linode_database_mysql_instance_credentials_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_mysql_instance_credentials_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[databaseInstanceIDParam]; !ok {
		t.Errorf("props missing key %v", databaseInstanceIDParam)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}
}

func TestLinodeDatabaseInstanceCredentialsGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != databaseInstanceCredentialsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceCredentialsPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.DatabaseCredentials{Username: keyGrantLinode, Password: databaseCredentialsPassword}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstanceCredentialsGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true})

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

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, keyGrantLinode) {
		t.Errorf("textContent.Text does not contain %v", keyGrantLinode)
	}

	if !strings.Contains(textContent.Text, databaseCredentialsPassword) {
		t.Errorf("textContent.Text does not contain %v", databaseCredentialsPassword)
	}
}

func TestLinodeDatabaseInstanceCredentialsGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databaseInstanceCredentialsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceCredentialsPath)
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
	_, _, handler := tools.NewLinodeDatabaseInstanceCredentialsGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve MySQL Managed Database credentials") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve MySQL Managed Database credentials")
	}
}

func TestLinodeDatabaseInstanceCredentialsGetToolClientConfigurationError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseInstanceCredentialsGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeDatabaseInstanceCredentialsGetToolInstanceIdValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseInstanceCredentialsGetTool(cfg)

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingInstanceID, args: map[string]any{}},
		{name: caseStringInstanceID, args: map[string]any{databaseInstanceIDParam: "123"}},
		{name: caseZeroInstanceID, args: map[string]any{databaseInstanceIDParam: 0}},
		{name: caseNegativeInstanceID, args: map[string]any{databaseInstanceIDParam: -1}},
		{name: caseFractionalInstanceID, args: map[string]any{databaseInstanceIDParam: 123.4}},
		{name: caseSlashInstanceID, args: map[string]any{databaseInstanceIDParam: "/"}},
		{name: caseQueryInstanceID, args: map[string]any{databaseInstanceIDParam: databaseInvalidInstanceIDQuery}},
		{name: caseTraversalInstanceID, args: map[string]any{databaseInstanceIDParam: pathTraversalValue}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, databaseInstanceIDMessage) {
				t.Errorf("textContent.Text does not contain %v", databaseInstanceIDMessage)
			}
		})
	}
}

func TestLinodeDatabaseInstanceCredentialsResetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabaseInstanceCredentialsResetTool(cfg)

	if tool.Name != "linode_database_mysql_instance_credentials_reset" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_mysql_instance_credentials_reset")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[databaseInstanceIDParam]; !ok {
		t.Errorf("props missing key %v", databaseInstanceIDParam)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	if !slices.Contains(tool.InputSchema.Required, keyConfirm) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyConfirm)
	}
}

func TestLinodeDatabaseInstanceCredentialsResetToolConfirmValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: databaseInvalidAPIURL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstanceCredentialsResetTool(cfg)

	cases := []struct {
		name  string
		value any
	}{
		{name: caseMissingConfirm},
		{name: caseFalseConfirm, value: false},
		{name: caseStringConfirm, value: boolStringTrue},
		{name: caseNumericConfirm, value: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{databaseInstanceIDParam: databaseInstanceID}
			if testCase.value != nil {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "confirm=true") {
				t.Errorf("textContent.Text does not contain %v", "confirm=true")
			}
		})
	}
}

func TestLinodeDatabaseInstanceCredentialsResetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databaseInstanceCredentialsResetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceCredentialsResetPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.DatabaseCredentials{Username: keyGrantLinode, Password: databaseCredentialsPassword}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstanceCredentialsResetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "credentials reset") {
		t.Errorf("textContent.Text does not contain %v", "credentials reset")
	}

	if !strings.Contains(textContent.Text, keyGrantLinode) {
		t.Errorf("textContent.Text does not contain %v", keyGrantLinode)
	}

	if !strings.Contains(textContent.Text, databaseCredentialsPassword) {
		t.Errorf("textContent.Text does not contain %v", databaseCredentialsPassword)
	}
}

func TestLinodeDatabaseInstanceCredentialsResetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databaseInstanceCredentialsResetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceCredentialsResetPath)
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
	_, _, handler := tools.NewLinodeDatabaseInstanceCredentialsResetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to reset MySQL Managed Database credentials") {
		t.Errorf("textContent.Text does not contain %v", "Failed to reset MySQL Managed Database credentials")
	}
}

func TestLinodeDatabaseInstanceCredentialsResetToolClientConfigurationError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseInstanceCredentialsResetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeDatabaseInstanceCredentialsResetToolInstanceIdValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseInstanceCredentialsResetTool(cfg)

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingInstanceID, args: map[string]any{keyConfirm: true}},
		{name: caseStringInstanceID, args: map[string]any{databaseInstanceIDParam: "123", keyConfirm: true}},
		{name: caseZeroInstanceID, args: map[string]any{databaseInstanceIDParam: 0, keyConfirm: true}},
		{name: caseNegativeInstanceID, args: map[string]any{databaseInstanceIDParam: -1, keyConfirm: true}},
		{name: caseFractionalInstanceID, args: map[string]any{databaseInstanceIDParam: 123.4, keyConfirm: true}},
		{name: caseSlashInstanceID, args: map[string]any{databaseInstanceIDParam: "/", keyConfirm: true}},
		{name: caseQueryInstanceID, args: map[string]any{databaseInstanceIDParam: databaseInvalidInstanceIDQuery, keyConfirm: true}},
		{name: caseTraversalInstanceID, args: map[string]any{databaseInstanceIDParam: pathTraversalValue, keyConfirm: true}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, databaseInstanceIDMessage) {
				t.Errorf("textContent.Text does not contain %v", databaseInstanceIDMessage)
			}
		})
	}
}

func TestLinodeDatabasePostgreSQLInstanceCredentialsResetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabasePostgreSQLInstanceCredentialsResetTool(cfg)

	if tool.Name != "linode_database_postgresql_instance_credentials_reset" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_postgresql_instance_credentials_reset")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[databaseInstanceIDParam]; !ok {
		t.Errorf("props missing key %v", databaseInstanceIDParam)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	if !slices.Contains(tool.InputSchema.Required, keyConfirm) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyConfirm)
	}
}

func TestLinodeDatabasePostgreSQLInstanceCredentialsResetToolConfirmValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: databaseInvalidAPIURL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceCredentialsResetTool(cfg)

	cases := []struct {
		name  string
		value any
	}{
		{name: caseMissingConfirm},
		{name: caseFalseConfirm, value: false},
		{name: caseStringConfirm, value: boolStringTrue},
		{name: caseNumericConfirm, value: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{databaseInstanceIDParam: databaseInstanceID}
			if testCase.value != nil {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "confirm=true") {
				t.Errorf("textContent.Text does not contain %v", "confirm=true")
			}
		})
	}
}

func TestLinodeDatabasePostgreSQLInstanceCredentialsResetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databasePostgreSQLCredentialsResetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLCredentialsResetPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceCredentialsResetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "PostgreSQL Managed Database credentials reset") {
		t.Errorf("textContent.Text does not contain %v", "PostgreSQL Managed Database credentials reset")
	}

	if !strings.Contains(textContent.Text, "instance_id") {
		t.Errorf("textContent.Text does not contain %v", "instance_id")
	}
}

func TestLinodeDatabasePostgreSQLInstanceCredentialsResetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databasePostgreSQLCredentialsResetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLCredentialsResetPath)
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
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceCredentialsResetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to reset PostgreSQL Managed Database credentials") {
		t.Errorf("textContent.Text does not contain %v", "Failed to reset PostgreSQL Managed Database credentials")
	}
}

func TestLinodeDatabasePostgreSQLInstanceCredentialsResetToolClientConfigurationError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceCredentialsResetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeDatabasePostgreSQLInstanceCredentialsResetToolInstanceIdValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceCredentialsResetTool(cfg)

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingInstanceID, args: map[string]any{keyConfirm: true}},
		{name: caseStringInstanceID, args: map[string]any{databaseInstanceIDParam: "123", keyConfirm: true}},
		{name: caseZeroInstanceID, args: map[string]any{databaseInstanceIDParam: 0, keyConfirm: true}},
		{name: caseNegativeInstanceID, args: map[string]any{databaseInstanceIDParam: -1, keyConfirm: true}},
		{name: caseFractionalInstanceID, args: map[string]any{databaseInstanceIDParam: 123.4, keyConfirm: true}},
		{name: caseSlashInstanceID, args: map[string]any{databaseInstanceIDParam: "/", keyConfirm: true}},
		{name: caseQueryInstanceID, args: map[string]any{databaseInstanceIDParam: databaseInvalidInstanceIDQuery, keyConfirm: true}},
		{name: caseTraversalInstanceID, args: map[string]any{databaseInstanceIDParam: pathTraversalValue, keyConfirm: true}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, databaseInstanceIDMessage) {
				t.Errorf("textContent.Text does not contain %v", databaseInstanceIDMessage)
			}
		})
	}
}

func TestLinodeDatabaseInstanceCreateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabaseInstanceCreateTool(cfg)

	if tool.Name != "linode_database_mysql_instance_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_mysql_instance_create")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	if !slices.Contains(tool.InputSchema.Required, keyConfirm) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyConfirm)
	}

	if _, ok := props[keyLabel]; !ok {
		t.Errorf("props missing key %v", keyLabel)
	}

	if _, ok := props[keyType]; !ok {
		t.Errorf("props missing key %v", keyType)
	}

	if _, ok := props[databaseEngineParam]; !ok {
		t.Errorf("props missing key %v", databaseEngineParam)
	}

	if _, ok := props[keyRegion]; !ok {
		t.Errorf("props missing key %v", keyRegion)
	}
}

func TestLinodeDatabaseInstanceCreateToolConfirmValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: databaseInvalidAPIURL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstanceCreateTool(cfg)

	cases := []struct {
		name  string
		value any
	}{
		{name: caseMissingConfirm},
		{name: caseFalseConfirm, value: false},
		{name: caseStringConfirm, value: boolStringTrue},
		{name: caseNumericConfirm, value: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast}
			if testCase.value != nil {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "confirm=true") {
				t.Errorf("textContent.Text does not contain %v", "confirm=true")
			}
		})
	}
}

func TestLinodeDatabaseInstanceCreateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databaseInstancesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancesPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		for key, want := range map[string]any{
			keyLabel:                   databaseInstanceLabel,
			keyType:                    databaseInstanceType,
			databaseEngineParam:        databaseEngineID,
			keyRegion:                  regionUSEast,
			databaseSSLConnectionParam: true,
		} {
			if !reflect.DeepEqual(body[key], want) {
				t.Errorf("body[%v] = %v, want %v", key, body[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineName, Version: databaseVersion, Status: statusActive}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstanceCreateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, databaseSSLConnectionParam: true, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, databaseInstanceLabel) {
		t.Errorf("textContent.Text does not contain %v", databaseInstanceLabel)
	}

	if !strings.Contains(textContent.Text, "created") {
		t.Errorf("textContent.Text does not contain %v", "created")
	}
}

func TestLinodeDatabaseInstanceCreateToolRequiredFieldValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseInstanceCreateTool(cfg)

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingLabel, args: map[string]any{keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, keyConfirm: true}, wantMessage: databaseLabelRequiredMessage},
		{name: caseMissingType, args: map[string]any{keyLabel: databaseInstanceLabel, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, keyConfirm: true}, wantMessage: "type must be a non-empty string"},
		{name: "missing engine", args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, keyRegion: regionUSEast, keyConfirm: true}, wantMessage: "engine must be a non-empty string"},
		{name: "missing region", args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyConfirm: true}, wantMessage: "region must be a non-empty string"},
		{name: caseInvalidAllowList, args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, databaseAllowListParam: invalidJSON, keyConfirm: true}, wantMessage: databaseAllowListNotArray},
		{name: "invalid cluster size", args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, "cluster_size": "3", keyConfirm: true}, wantMessage: "cluster_size must be a positive integer"},
		{name: caseInvalidEngineConfig, args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, databaseEngineConfigParam: invalidJSON, keyConfirm: true}, wantMessage: databaseEngineConfigNotObject},
		{name: "invalid fork", args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, "fork": invalidJSON, keyConfirm: true}, wantMessage: "fork must be an object"},
		{name: caseInvalidPrivateNetwork, args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, databasePrivateNetworkParam: invalidJSON, keyConfirm: true}, wantMessage: databasePrivateNetworkNotObject},
		{name: "invalid ssl bool", args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, databaseSSLConnectionParam: boolStringTrue, keyConfirm: true}, wantMessage: "ssl_connection must be a boolean"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeDatabaseInstanceCreateToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databaseInstancesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancesPath)
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
	_, _, handler := tools.NewLinodeDatabaseInstanceCreateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to create Managed Database instance") {
		t.Errorf("textContent.Text does not contain %v", "Failed to create Managed Database instance")
	}
}

func TestLinodeDatabasePostgreSQLInstanceCreateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabasePostgreSQLInstanceCreateTool(cfg)

	if tool.Name != "linode_database_postgresql_instance_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_postgresql_instance_create")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	if !slices.Contains(tool.InputSchema.Required, keyConfirm) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyConfirm)
	}

	if _, ok := props[keyLabel]; !ok {
		t.Errorf("props missing key %v", keyLabel)
	}

	if _, ok := props[keyType]; !ok {
		t.Errorf("props missing key %v", keyType)
	}

	if _, ok := props[databaseEngineParam]; !ok {
		t.Errorf("props missing key %v", databaseEngineParam)
	}

	if _, ok := props[keyRegion]; !ok {
		t.Errorf("props missing key %v", keyRegion)
	}
}

func TestLinodeDatabasePostgreSQLInstanceCreateToolConfirmValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: databaseInvalidAPIURL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceCreateTool(cfg)

	cases := []struct {
		name  string
		value any
	}{{name: caseMissingConfirm}, {name: caseFalseConfirm, value: false}, {name: caseStringConfirm, value: boolStringTrue}, {name: caseNumericConfirm, value: 1}}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEnginePostgreSQLID, keyRegion: regionUSEast}
			if testCase.value != nil {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "confirm=true") {
				t.Errorf("textContent.Text does not contain %v", "confirm=true")
			}
		})
	}
}

func TestLinodeDatabasePostgreSQLInstanceCreateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databasePostgreSQLInstancesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstancesPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		for key, want := range map[string]any{
			keyLabel:                   databaseInstanceLabel,
			keyType:                    databaseInstanceType,
			databaseEngineParam:        databaseEnginePostgreSQLID,
			keyRegion:                  regionUSEast,
			databaseSSLConnectionParam: true,
		} {
			if !reflect.DeepEqual(body[key], want) {
				t.Errorf("body[%v] = %v, want %v", key, body[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEnginePostgreSQL, Version: databaseVersion, Status: statusActive}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceCreateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEnginePostgreSQLID, keyRegion: regionUSEast, databaseSSLConnectionParam: true, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, databaseInstanceLabel) {
		t.Errorf("textContent.Text does not contain %v", databaseInstanceLabel)
	}

	if !strings.Contains(textContent.Text, "created") {
		t.Errorf("textContent.Text does not contain %v", "created")
	}
}

func TestLinodeDatabasePostgreSQLInstanceCreateToolRequiredFieldValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceCreateTool(cfg)

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingLabel, args: map[string]any{keyType: databaseInstanceType, databaseEngineParam: databaseEnginePostgreSQLID, keyRegion: regionUSEast, keyConfirm: true}, wantMessage: databaseLabelRequiredMessage},
		{name: caseMissingType, args: map[string]any{keyLabel: databaseInstanceLabel, databaseEngineParam: databaseEnginePostgreSQLID, keyRegion: regionUSEast, keyConfirm: true}, wantMessage: "type must be a non-empty string"},
		{name: "missing engine", args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, keyRegion: regionUSEast, keyConfirm: true}, wantMessage: "engine must be a non-empty string"},
		{name: "missing region", args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEnginePostgreSQLID, keyConfirm: true}, wantMessage: "region must be a non-empty string"},
		{name: caseInvalidAllowList, args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEnginePostgreSQLID, keyRegion: regionUSEast, databaseAllowListParam: invalidJSON, keyConfirm: true}, wantMessage: databaseAllowListNotArray},
		{name: "invalid cluster size", args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEnginePostgreSQLID, keyRegion: regionUSEast, "cluster_size": "3", keyConfirm: true}, wantMessage: "cluster_size must be a positive integer"},
		{name: caseInvalidEngineConfig, args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEnginePostgreSQLID, keyRegion: regionUSEast, databaseEngineConfigParam: invalidJSON, keyConfirm: true}, wantMessage: databaseEngineConfigNotObject},
		{name: "invalid fork", args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEnginePostgreSQLID, keyRegion: regionUSEast, "fork": invalidJSON, keyConfirm: true}, wantMessage: "fork must be an object"},
		{name: caseInvalidPrivateNetwork, args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEnginePostgreSQLID, keyRegion: regionUSEast, databasePrivateNetworkParam: invalidJSON, keyConfirm: true}, wantMessage: databasePrivateNetworkNotObject},
		{name: "invalid ssl bool", args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEnginePostgreSQLID, keyRegion: regionUSEast, databaseSSLConnectionParam: boolStringTrue, keyConfirm: true}, wantMessage: "ssl_connection must be a boolean"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeDatabasePostgreSQLInstanceCreateToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databasePostgreSQLInstancesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstancesPath)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryFailure}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceCreateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEnginePostgreSQLID, keyRegion: regionUSEast, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to create PostgreSQL Managed Database instance") {
		t.Errorf("textContent.Text does not contain %v", "Failed to create PostgreSQL Managed Database instance")
	}
}

func TestLinodeDatabaseInstanceUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabaseInstanceUpdateTool(cfg)

	if tool.Name != "linode_database_mysql_instance_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_mysql_instance_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[databaseInstanceIDParam]; !ok {
		t.Errorf("props missing key %v", databaseInstanceIDParam)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	for _, key := range []string{databaseInstanceIDParam, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if _, ok := props[keyLabel]; !ok {
		t.Errorf("props missing key %v", keyLabel)
	}

	if _, ok := props[keyType]; !ok {
		t.Errorf("props missing key %v", keyType)
	}

	if _, ok := props[databaseUpdatesParam]; !ok {
		t.Errorf("props missing key %v", databaseUpdatesParam)
	}

	if _, ok := props[databaseVersionParam]; !ok {
		t.Errorf("props missing key %v", databaseVersionParam)
	}
}

func TestLinodeDatabaseInstanceUpdateToolConfirmValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: databaseInvalidAPIURL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstanceUpdateTool(cfg)

	cases := []struct {
		name  string
		value any
	}{
		{name: caseMissingConfirm},
		{name: caseFalseConfirm, value: false},
		{name: caseStringConfirm, value: boolStringTrue},
		{name: caseNumericConfirm, value: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{databaseInstanceIDParam: databaseInstanceID, keyLabel: databaseInstanceLabel}
			if testCase.value != nil {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "confirm=true") {
				t.Errorf("textContent.Text does not contain %v", "confirm=true")
			}
		})
	}
}

func TestLinodeDatabaseInstanceUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != databaseInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		for key, want := range map[string]any{
			keyLabel:                    databaseInstanceLabel,
			keyType:                     databaseInstanceType,
			databaseVersionParam:        databaseVersion,
			databaseAllowListParam:      []any{tcLit},
			databaseUpdatesParam:        map[string]any{tcFrequency: tcWeekly, tcHourOfDay: float64(1)},
			databasePrivateNetworkParam: map[string]any{tcPublicAccess: false, keyVPCID: float64(123)},
		} {
			if !reflect.DeepEqual(body[key], want) {
				t.Errorf("body[%v] = %v, want %v", key, body[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineName, Version: databaseVersion, Status: statusActive}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstanceUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		databaseInstanceIDParam:     databaseInstanceID,
		keyLabel:                    databaseInstanceLabel,
		keyType:                     databaseInstanceType,
		databaseVersionParam:        databaseVersion,
		databaseAllowListParam:      `["203.0.113.0/24"]`,
		databaseUpdatesParam:        `{"frequency":"weekly","hour_of_day":1}`,
		databasePrivateNetworkParam: `{"public_access":false,"vpc_id":123}`,
		databaseEngineConfigParam:   `{"binlog_retention_period":600}`,
		keyConfirm:                  true,
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

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, databaseInstanceLabel) {
		t.Errorf("textContent.Text does not contain %v", databaseInstanceLabel)
	}

	if !strings.Contains(textContent.Text, "updated") {
		t.Errorf("textContent.Text does not contain %v", "updated")
	}
}

// An explicit null on private_network detaches the instance from its VPC. The
// wire body must carry "private_network":null rather than rejecting the value or
// dropping the field, matching the Linode API and the Python implementation.
func TestLinodeDatabaseInstanceUpdateToolPrivateNetworkDetach(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		raw, present := body[databasePrivateNetworkParam]
		if !present {
			t.Error("private_network key missing from body, want present")
		}

		if string(raw) != databaseJSONNull {
			t.Errorf("body[private_network] = %q, want null", string(raw))
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineName, Version: databaseVersion, Status: statusActive}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstanceUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		databaseInstanceIDParam:     databaseInstanceID,
		databasePrivateNetworkParam: nil,
		keyConfirm:                  true,
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
}

// When private_network is absent the field must be omitted from the wire body
// entirely so the existing VPC binding is left untouched.
func TestLinodeDatabaseInstanceUpdateToolPrivateNetworkAbsentOmits(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if _, present := body[databasePrivateNetworkParam]; present {
			t.Errorf("body contains private_network key, want omitted")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineName, Version: databaseVersion, Status: statusActive}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstanceUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		databaseInstanceIDParam: databaseInstanceID,
		keyLabel:                databaseInstanceLabel,
		keyConfirm:              true,
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
}

// PostgreSQL update must detach on explicit null exactly like the MySQL path so
// the two engines stay at parity.
func TestLinodeDatabasePostgreSQLInstanceUpdateToolPrivateNetworkDetach(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databasePostgreSQLInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstancePath)
		}

		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		raw, present := body[databasePrivateNetworkParam]
		if !present {
			t.Error("private_network key missing from body, want present")
		}

		if string(raw) != databaseJSONNull {
			t.Errorf("body[private_network] = %q, want null", string(raw))
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEnginePostgreSQL, Version: databaseVersion, Status: statusActive}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		databaseInstanceIDParam:     databaseInstanceID,
		databasePrivateNetworkParam: nil,
		keyConfirm:                  true,
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
}

func TestLinodeDatabaseInstanceUpdateToolInputValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseInstanceUpdateTool(cfg)

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingInstanceID, args: map[string]any{keyLabel: databaseInstanceLabel, keyConfirm: true}, wantMessage: databaseInstanceIDMessage},
		{name: caseQueryInstanceID, args: map[string]any{databaseInstanceIDParam: databaseInvalidInstanceIDQuery, keyLabel: databaseInstanceLabel, keyConfirm: true}, wantMessage: databaseInstanceIDMessage},
		{name: "empty update", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}, wantMessage: "at least one update field must be provided"},
		{name: caseMissingLabel, args: map[string]any{databaseInstanceIDParam: databaseInstanceID, keyLabel: "", keyConfirm: true}, wantMessage: databaseLabelRequiredMessage},
		{name: caseInvalidAllowList, args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseAllowListParam: invalidJSON, keyConfirm: true}, wantMessage: databaseAllowListNotArray},
		{name: caseInvalidEngineConfig, args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseEngineConfigParam: invalidJSON, keyConfirm: true}, wantMessage: databaseEngineConfigNotObject},
		{name: caseInvalidPrivateNetwork, args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databasePrivateNetworkParam: invalidJSON, keyConfirm: true}, wantMessage: databasePrivateNetworkNotObject},
		{name: "invalid updates", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseUpdatesParam: invalidJSON, keyConfirm: true}, wantMessage: databaseUpdatesNotObject},
		{name: "numeric version", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseVersionParam: 8, keyConfirm: true}, wantMessage: "version must be a non-empty string"},
		{name: "non-string allow list entry", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseAllowListParam: []any{1}, keyConfirm: true}, wantMessage: databaseAllowListNotArray},
		{name: "object allow list", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseAllowListParam: jsonObjectEmpty, keyConfirm: true}, wantMessage: databaseAllowListNotArray},
		{name: "null engine config", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseEngineConfigParam: databaseJSONNull, keyConfirm: true}, wantMessage: databaseEngineConfigNotObject},
		{name: "array engine config", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseEngineConfigParam: databaseJSONArray, keyConfirm: true}, wantMessage: databaseEngineConfigNotObject},
		{name: "array private network", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databasePrivateNetworkParam: databaseJSONArray, keyConfirm: true}, wantMessage: databasePrivateNetworkNotObject},
		{name: "null updates", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseUpdatesParam: databaseJSONNull, keyConfirm: true}, wantMessage: databaseUpdatesNotObject},
		{name: "array updates", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseUpdatesParam: databaseJSONArray, keyConfirm: true}, wantMessage: databaseUpdatesNotObject},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeDatabaseInstanceUpdateToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databaseInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancePath)
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
	_, _, handler := tools.NewLinodeDatabaseInstanceUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyLabel: databaseInstanceLabel, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to update Managed Database instance") {
		t.Errorf("textContent.Text does not contain %v", "Failed to update Managed Database instance")
	}
}

func TestLinodeDatabasePostgreSQLInstanceUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabasePostgreSQLInstanceUpdateTool(cfg)

	if tool.Name != "linode_database_postgresql_instance_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_postgresql_instance_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[databaseInstanceIDParam]; !ok {
		t.Errorf("props missing key %v", databaseInstanceIDParam)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	for _, key := range []string{databaseInstanceIDParam, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if _, ok := props[keyLabel]; !ok {
		t.Errorf("props missing key %v", keyLabel)
	}

	if _, ok := props[keyType]; !ok {
		t.Errorf("props missing key %v", keyType)
	}

	if _, ok := props[databaseUpdatesParam]; !ok {
		t.Errorf("props missing key %v", databaseUpdatesParam)
	}

	if _, ok := props[databaseVersionParam]; !ok {
		t.Errorf("props missing key %v", databaseVersionParam)
	}
}

func TestLinodeDatabasePostgreSQLInstanceUpdateToolConfirmValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: databaseInvalidAPIURL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceUpdateTool(cfg)

	cases := []struct {
		name  string
		value any
	}{
		{name: caseMissingConfirm},
		{name: caseFalseConfirm, value: false},
		{name: caseStringConfirm, value: boolStringTrue},
		{name: caseNumericConfirm, value: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{databaseInstanceIDParam: databaseInstanceID, keyLabel: databaseInstanceLabel}
			if testCase.value != nil {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "confirm=true") {
				t.Errorf("textContent.Text does not contain %v", "confirm=true")
			}
		})
	}
}

func TestLinodeDatabasePostgreSQLInstanceUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != databasePostgreSQLInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstancePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		for key, want := range map[string]any{
			keyLabel:                    databaseInstanceLabel,
			keyType:                     databaseInstanceType,
			databaseVersionParam:        databaseVersion,
			databaseAllowListParam:      []any{tcLit},
			databaseUpdatesParam:        map[string]any{tcFrequency: tcWeekly, tcHourOfDay: float64(1)},
			databaseEngineConfigParam:   map[string]any{databasePostgreSQLConfigNamespace: map[string]any{"timezone": "UTC"}},
			databasePrivateNetworkParam: map[string]any{tcPublicAccess: false, keyVPCID: float64(123)},
		} {
			if !reflect.DeepEqual(body[key], want) {
				t.Errorf("body[%v] = %v, want %v", key, body[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEnginePostgreSQL, Version: databaseVersion, Status: statusActive}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		databaseInstanceIDParam:     databaseInstanceID,
		keyLabel:                    databaseInstanceLabel,
		keyType:                     databaseInstanceType,
		databaseVersionParam:        databaseVersion,
		databaseAllowListParam:      `["203.0.113.0/24"]`,
		databaseUpdatesParam:        `{"frequency":"weekly","hour_of_day":1}`,
		databasePrivateNetworkParam: `{"public_access":false,"vpc_id":123}`,
		databaseEngineConfigParam:   `{"pg":{"timezone":"UTC"}}`,
		keyConfirm:                  true,
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

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, databaseInstanceLabel) {
		t.Errorf("textContent.Text does not contain %v", databaseInstanceLabel)
	}

	if !strings.Contains(textContent.Text, "PostgreSQL Managed Database") {
		t.Errorf("textContent.Text does not contain %v", "PostgreSQL Managed Database")
	}

	if !strings.Contains(textContent.Text, "updated") {
		t.Errorf("textContent.Text does not contain %v", "updated")
	}
}

func TestLinodeDatabasePostgreSQLInstanceUpdateToolInputValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceUpdateTool(cfg)

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingInstanceID, args: map[string]any{keyLabel: databaseInstanceLabel, keyConfirm: true}, wantMessage: databaseInstanceIDMessage},
		{name: caseSlashInstanceID, args: map[string]any{databaseInstanceIDParam: "/", keyLabel: databaseInstanceLabel, keyConfirm: true}, wantMessage: databaseInstanceIDMessage},
		{name: caseQueryInstanceID, args: map[string]any{databaseInstanceIDParam: databaseInvalidInstanceIDQuery, keyLabel: databaseInstanceLabel, keyConfirm: true}, wantMessage: databaseInstanceIDMessage},
		{name: caseTraversalInstanceID, args: map[string]any{databaseInstanceIDParam: pathTraversalValue, keyLabel: databaseInstanceLabel, keyConfirm: true}, wantMessage: databaseInstanceIDMessage},
		{name: "empty update", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}, wantMessage: "at least one update field must be provided"},
		{name: caseMissingLabel, args: map[string]any{databaseInstanceIDParam: databaseInstanceID, keyLabel: "", keyConfirm: true}, wantMessage: databaseLabelRequiredMessage},
		{name: caseInvalidAllowList, args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseAllowListParam: invalidJSON, keyConfirm: true}, wantMessage: databaseAllowListNotArray},
		{name: caseInvalidEngineConfig, args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseEngineConfigParam: invalidJSON, keyConfirm: true}, wantMessage: databaseEngineConfigNotObject},
		{name: caseInvalidPrivateNetwork, args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databasePrivateNetworkParam: invalidJSON, keyConfirm: true}, wantMessage: databasePrivateNetworkNotObject},
		{name: "invalid updates", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseUpdatesParam: invalidJSON, keyConfirm: true}, wantMessage: databaseUpdatesNotObject},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeDatabasePostgreSQLInstanceUpdateToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databasePostgreSQLInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstancePath)
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
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyLabel: databaseInstanceLabel, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to update PostgreSQL Managed Database instance") {
		t.Errorf("textContent.Text does not contain %v", "Failed to update PostgreSQL Managed Database instance")
	}
}

func TestLinodeDatabaseInstanceDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabaseInstanceDeleteTool(cfg)

	if tool.Name != "linode_database_mysql_instance_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_mysql_instance_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[databaseInstanceIDParam]; !ok {
		t.Errorf("props missing key %v", databaseInstanceIDParam)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	for _, key := range []string{databaseInstanceIDParam, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeDatabaseInstanceDeleteToolConfirmValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: databaseInvalidAPIURL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstanceDeleteTool(cfg)

	cases := []struct {
		name  string
		value any
	}{
		{name: caseMissingConfirm},
		{name: caseFalseConfirm, value: false},
		{name: caseStringConfirm, value: boolStringTrue},
		{name: caseNumericConfirm, value: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{databaseInstanceIDParam: databaseInstanceID}
			if testCase.value != nil {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "confirm=true") {
				t.Errorf("textContent.Text does not contain %v", "confirm=true")
			}
		})
	}
}

func TestLinodeDatabaseInstanceDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != databaseInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstanceDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "deleted") {
		t.Errorf("textContent.Text does not contain %v", "deleted")
	}

	if !strings.Contains(textContent.Text, "123") {
		t.Errorf("textContent.Text does not contain %v", "123")
	}
}

func TestLinodeDatabaseInstanceDeleteToolInputValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseInstanceDeleteTool(cfg)

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingInstanceID, args: map[string]any{keyConfirm: true, keyConfirmedDryRun: true}},
		{name: caseStringInstanceID, args: map[string]any{databaseInstanceIDParam: "123", keyConfirm: true, keyConfirmedDryRun: true}},
		{name: caseSlashInstanceID, args: map[string]any{databaseInstanceIDParam: "/", keyConfirm: true, keyConfirmedDryRun: true}},
		{name: caseQueryInstanceID, args: map[string]any{databaseInstanceIDParam: databaseInvalidInstanceIDQuery, keyConfirm: true, keyConfirmedDryRun: true}},
		{name: caseTraversalInstanceID, args: map[string]any{databaseInstanceIDParam: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, databaseInstanceIDMessage) {
				t.Errorf("textContent.Text does not contain %v", databaseInstanceIDMessage)
			}
		})
	}
}

func TestLinodeDatabaseInstanceDeleteToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databaseInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancePath)
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
	_, _, handler := tools.NewLinodeDatabaseInstanceDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "linode_database_mysql_instance_delete failed") {
		t.Errorf("textContent.Text does not contain %v", "linode_database_mysql_instance_delete failed")
	}
}

// Dry-run coverage for MySQL Managed Database instance delete.
func TestLinodeDatabaseInstanceDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeDatabaseInstanceDeleteTool(&config.Config{})
	if _, ok := tool.InputSchema.Properties["dry_run"]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", "dry_run")
	}
}

func TestLinodeDatabaseInstanceDeleteToolDryRunPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.URL.Path != databaseInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancePath)
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Status: statusActive}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeDatabaseInstanceDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		databaseInstanceIDParam: databaseInstanceID,
		keyDryRun:               true,
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

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_database_mysql_instance_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_database_mysql_instance_delete")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], databaseInstancePath) {
		t.Errorf("got %v, want %v", would["path"], databaseInstancePath)
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodeDatabaseInstanceDeleteToolDryRunStillValidatesInstanceId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeDatabaseInstanceDeleteTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, databaseInstanceIDMessage) {
		t.Errorf("error text %q does not contain %q", text.Text, databaseInstanceIDMessage)
	}
}

func TestLinodeDatabasePostgreSQLInstanceDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabasePostgreSQLInstanceDeleteTool(cfg)

	if tool.Name != "linode_database_postgresql_instance_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_postgresql_instance_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[databaseInstanceIDParam]; !ok {
		t.Errorf("props missing key %v", databaseInstanceIDParam)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	for _, key := range []string{databaseInstanceIDParam, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeDatabasePostgreSQLInstanceDeleteToolConfirmValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: databaseInvalidAPIURL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceDeleteTool(cfg)

	cases := []struct {
		name  string
		value any
	}{
		{name: caseMissingConfirm},
		{name: caseFalseConfirm, value: false},
		{name: caseStringConfirm, value: boolStringTrue},
		{name: caseNumericConfirm, value: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{databaseInstanceIDParam: databaseInstanceID}
			if testCase.value != nil {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "confirm=true") {
				t.Errorf("textContent.Text does not contain %v", "confirm=true")
			}
		})
	}
}

func TestLinodeDatabasePostgreSQLInstanceDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != databasePostgreSQLInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstancePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "deleted") {
		t.Errorf("textContent.Text does not contain %v", "deleted")
	}

	if !strings.Contains(textContent.Text, "123") {
		t.Errorf("textContent.Text does not contain %v", "123")
	}
}

func TestLinodeDatabasePostgreSQLInstanceDeleteToolInputValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceDeleteTool(cfg)

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingInstanceID, args: map[string]any{keyConfirm: true, keyConfirmedDryRun: true}},
		{name: caseStringInstanceID, args: map[string]any{databaseInstanceIDParam: "123", keyConfirm: true, keyConfirmedDryRun: true}},
		{name: caseSlashInstanceID, args: map[string]any{databaseInstanceIDParam: "/", keyConfirm: true, keyConfirmedDryRun: true}},
		{name: caseQueryInstanceID, args: map[string]any{databaseInstanceIDParam: databaseInvalidInstanceIDQuery, keyConfirm: true, keyConfirmedDryRun: true}},
		{name: caseTraversalInstanceID, args: map[string]any{databaseInstanceIDParam: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, databaseInstanceIDMessage) {
				t.Errorf("textContent.Text does not contain %v", databaseInstanceIDMessage)
			}
		})
	}
}

func TestLinodeDatabasePostgreSQLInstanceDeleteToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databasePostgreSQLInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstancePath)
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
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "linode_database_postgresql_instance_delete failed") {
		t.Errorf("textContent.Text does not contain %v", "linode_database_postgresql_instance_delete failed")
	}
}

// Dry-run coverage for PostgreSQL Managed Database instance delete.
func TestLinodeDatabasePostgreSQLInstanceDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeDatabasePostgreSQLInstanceDeleteTool(&config.Config{})
	if _, ok := tool.InputSchema.Properties["dry_run"]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", "dry_run")
	}
}

func TestLinodeDatabasePostgreSQLInstanceDeleteToolDryRunPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.URL.Path != databasePostgreSQLInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstancePath)
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Status: statusActive}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		databaseInstanceIDParam: databaseInstanceID,
		keyDryRun:               true,
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

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_database_postgresql_instance_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_database_postgresql_instance_delete")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], databasePostgreSQLInstancePath) {
		t.Errorf("got %v, want %v", would["path"], databasePostgreSQLInstancePath)
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodeDatabasePostgreSQLInstanceDeleteToolDryRunStillValidatesInstanceId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceDeleteTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, databaseInstanceIDMessage) {
		t.Errorf("error text %q does not contain %q", text.Text, databaseInstanceIDMessage)
	}
}

func TestLinodeDatabaseInstancePatchToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabaseInstancePatchTool(cfg)

	if tool.Name != "linode_database_mysql_instance_patch" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_mysql_instance_patch")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[databaseInstanceIDParam]; !ok {
		t.Errorf("props missing key %v", databaseInstanceIDParam)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	for _, key := range []string{databaseInstanceIDParam, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeDatabaseInstancePatchToolConfirmValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: databaseInvalidAPIURL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstancePatchTool(cfg)

	cases := []struct {
		name  string
		value any
	}{
		{name: caseMissingConfirm},
		{name: caseFalseConfirm, value: false},
		{name: caseStringConfirm, value: boolStringTrue},
		{name: caseNumericConfirm, value: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{databaseInstanceIDParam: databaseInstanceID}
			if testCase.value != nil {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "confirm=true") {
				t.Errorf("textContent.Text does not contain %v", "confirm=true")
			}
		})
	}
}

func TestLinodeDatabaseInstancePatchToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databaseInstancePatchPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancePatchPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstancePatchTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "patch started") {
		t.Errorf("textContent.Text does not contain %v", "patch started")
	}

	if !strings.Contains(textContent.Text, "123") {
		t.Errorf("textContent.Text does not contain %v", "123")
	}
}

func TestLinodeDatabaseInstancePatchToolInputValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseInstancePatchTool(cfg)

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingInstanceID, args: map[string]any{keyConfirm: true}},
		{name: caseStringInstanceID, args: map[string]any{databaseInstanceIDParam: "123", keyConfirm: true}},
		{name: caseSlashInstanceID, args: map[string]any{databaseInstanceIDParam: "/", keyConfirm: true}},
		{name: caseQueryInstanceID, args: map[string]any{databaseInstanceIDParam: databaseInvalidInstanceIDQuery, keyConfirm: true}},
		{name: caseTraversalInstanceID, args: map[string]any{databaseInstanceIDParam: pathTraversalValue, keyConfirm: true}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, databaseInstanceIDMessage) {
				t.Errorf("textContent.Text does not contain %v", databaseInstanceIDMessage)
			}
		})
	}
}

func TestLinodeDatabaseInstancePatchToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databaseInstancePatchPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancePatchPath)
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
	_, _, handler := tools.NewLinodeDatabaseInstancePatchTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to patch Managed Database instance") {
		t.Errorf("textContent.Text does not contain %v", "Failed to patch Managed Database instance")
	}
}

func TestLinodeDatabaseInstanceSuspendToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabaseInstanceSuspendTool(cfg)

	if tool.Name != "linode_database_mysql_instance_suspend" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_mysql_instance_suspend")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[databaseInstanceIDParam]; !ok {
		t.Errorf("props missing key %v", databaseInstanceIDParam)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	for _, key := range []string{databaseInstanceIDParam, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeDatabaseInstanceSuspendToolConfirmValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: databaseInvalidAPIURL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstanceSuspendTool(cfg)

	cases := []struct {
		name  string
		value any
	}{
		{name: caseMissingConfirm},
		{name: caseFalseConfirm, value: false},
		{name: caseStringConfirm, value: boolStringTrue},
		{name: caseNumericConfirm, value: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{databaseInstanceIDParam: databaseInstanceID}
			if testCase.value != nil {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "confirm=true") {
				t.Errorf("textContent.Text does not contain %v", "confirm=true")
			}
		})
	}
}

func TestLinodeDatabaseInstanceSuspendToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databaseInstanceSuspendPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceSuspendPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstanceSuspendTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "suspend started") {
		t.Errorf("textContent.Text does not contain %v", "suspend started")
	}

	if !strings.Contains(textContent.Text, "123") {
		t.Errorf("textContent.Text does not contain %v", "123")
	}
}

func TestLinodeDatabaseInstanceSuspendToolInputValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseInstanceSuspendTool(cfg)

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingInstanceID, args: map[string]any{keyConfirm: true}},
		{name: caseStringInstanceID, args: map[string]any{databaseInstanceIDParam: "123", keyConfirm: true}},
		{name: caseSlashInstanceID, args: map[string]any{databaseInstanceIDParam: "/", keyConfirm: true}},
		{name: caseQueryInstanceID, args: map[string]any{databaseInstanceIDParam: databaseInvalidInstanceIDQuery, keyConfirm: true}},
		{name: caseTraversalInstanceID, args: map[string]any{databaseInstanceIDParam: pathTraversalValue, keyConfirm: true}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, databaseInstanceIDMessage) {
				t.Errorf("textContent.Text does not contain %v", databaseInstanceIDMessage)
			}
		})
	}
}

func TestLinodeDatabaseInstanceSuspendToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databaseInstanceSuspendPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceSuspendPath)
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
	_, _, handler := tools.NewLinodeDatabaseInstanceSuspendTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to suspend Managed Database instance") {
		t.Errorf("textContent.Text does not contain %v", "Failed to suspend Managed Database instance")
	}
}

func TestLinodeDatabaseInstanceResumeToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabaseInstanceResumeTool(cfg)

	if tool.Name != "linode_database_mysql_instance_resume" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_mysql_instance_resume")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[databaseInstanceIDParam]; !ok {
		t.Errorf("props missing key %v", databaseInstanceIDParam)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	for _, key := range []string{databaseInstanceIDParam, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeDatabaseInstanceResumeToolConfirmValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: databaseInvalidAPIURL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstanceResumeTool(cfg)

	cases := []struct {
		name  string
		value any
	}{
		{name: caseMissingConfirm},
		{name: caseFalseConfirm, value: false},
		{name: caseStringConfirm, value: boolStringTrue},
		{name: caseNumericConfirm, value: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{databaseInstanceIDParam: databaseInstanceID}
			if testCase.value != nil {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "confirm=true") {
				t.Errorf("textContent.Text does not contain %v", "confirm=true")
			}
		})
	}
}

func TestLinodeDatabaseInstanceResumeToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databaseInstanceResumePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceResumePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseInstanceResumeTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "resume started") {
		t.Errorf("textContent.Text does not contain %v", "resume started")
	}

	if !strings.Contains(textContent.Text, "123") {
		t.Errorf("textContent.Text does not contain %v", "123")
	}
}

func TestLinodeDatabaseInstanceResumeToolInputValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseInstanceResumeTool(cfg)

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingInstanceID, args: map[string]any{keyConfirm: true}},
		{name: caseStringInstanceID, args: map[string]any{databaseInstanceIDParam: "123", keyConfirm: true}},
		{name: caseSlashInstanceID, args: map[string]any{databaseInstanceIDParam: "/", keyConfirm: true}},
		{name: caseQueryInstanceID, args: map[string]any{databaseInstanceIDParam: databaseInvalidInstanceIDQuery, keyConfirm: true}},
		{name: caseTraversalInstanceID, args: map[string]any{databaseInstanceIDParam: pathTraversalValue, keyConfirm: true}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, databaseInstanceIDMessage) {
				t.Errorf("textContent.Text does not contain %v", databaseInstanceIDMessage)
			}
		})
	}
}

func TestLinodeDatabaseInstanceResumeToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databaseInstanceResumePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceResumePath)
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
	_, _, handler := tools.NewLinodeDatabaseInstanceResumeTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to resume Managed Database instance") {
		t.Errorf("textContent.Text does not contain %v", "Failed to resume Managed Database instance")
	}
}

func TestLinodeDatabasePostgreSQLInstanceSuspendToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabasePostgreSQLInstanceSuspendTool(cfg)

	if tool.Name != "linode_database_postgresql_instance_suspend" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_postgresql_instance_suspend")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[databaseInstanceIDParam]; !ok {
		t.Errorf("props missing key %v", databaseInstanceIDParam)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	for _, key := range []string{databaseInstanceIDParam, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeDatabasePostgreSQLInstanceSuspendToolConfirmValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: databaseInvalidAPIURL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceSuspendTool(cfg)

	cases := []struct {
		name  string
		value any
	}{
		{name: caseMissingConfirm},
		{name: caseFalseConfirm, value: false},
		{name: caseStringConfirm, value: boolStringTrue},
		{name: caseNumericConfirm, value: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{databaseInstanceIDParam: databaseInstanceID}
			if testCase.value != nil {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "confirm=true") {
				t.Errorf("textContent.Text does not contain %v", "confirm=true")
			}
		})
	}
}

func TestLinodeDatabasePostgreSQLInstanceSuspendToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databasePostgreSQLInstanceSuspendPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstanceSuspendPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceSuspendTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "suspend started") {
		t.Errorf("textContent.Text does not contain %v", "suspend started")
	}

	if !strings.Contains(textContent.Text, "123") {
		t.Errorf("textContent.Text does not contain %v", "123")
	}
}

func TestLinodeDatabasePostgreSQLInstanceSuspendToolInputValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceSuspendTool(cfg)

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingInstanceID, args: map[string]any{keyConfirm: true}},
		{name: caseStringInstanceID, args: map[string]any{databaseInstanceIDParam: "123", keyConfirm: true}},
		{name: caseSlashInstanceID, args: map[string]any{databaseInstanceIDParam: "/", keyConfirm: true}},
		{name: caseQueryInstanceID, args: map[string]any{databaseInstanceIDParam: databaseInvalidInstanceIDQuery, keyConfirm: true}},
		{name: caseTraversalInstanceID, args: map[string]any{databaseInstanceIDParam: pathTraversalValue, keyConfirm: true}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, databaseInstanceIDMessage) {
				t.Errorf("textContent.Text does not contain %v", databaseInstanceIDMessage)
			}
		})
	}
}

func TestLinodeDatabasePostgreSQLInstanceSuspendToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databasePostgreSQLInstanceSuspendPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstanceSuspendPath)
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
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceSuspendTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to suspend PostgreSQL Managed Database instance") {
		t.Errorf("textContent.Text does not contain %v", "Failed to suspend PostgreSQL Managed Database instance")
	}
}

func TestLinodeDatabasePostgreSQLInstanceResumeToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabasePostgreSQLInstanceResumeTool(cfg)

	if tool.Name != "linode_database_postgresql_instance_resume" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_postgresql_instance_resume")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[databaseInstanceIDParam]; !ok {
		t.Errorf("props missing key %v", databaseInstanceIDParam)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	for _, key := range []string{databaseInstanceIDParam, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeDatabasePostgreSQLInstanceResumeToolConfirmValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: databaseInvalidAPIURL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceResumeTool(cfg)

	cases := []struct {
		name  string
		value any
	}{
		{name: caseMissingConfirm},
		{name: caseFalseConfirm, value: false},
		{name: caseStringConfirm, value: boolStringTrue},
		{name: caseNumericConfirm, value: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{databaseInstanceIDParam: databaseInstanceID}
			if testCase.value != nil {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "confirm=true") {
				t.Errorf("textContent.Text does not contain %v", "confirm=true")
			}
		})
	}
}

func TestLinodeDatabasePostgreSQLInstanceResumeToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databasePostgreSQLInstanceResumePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstanceResumePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceResumeTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "resume started") {
		t.Errorf("textContent.Text does not contain %v", "resume started")
	}

	if !strings.Contains(textContent.Text, "123") {
		t.Errorf("textContent.Text does not contain %v", "123")
	}
}

func TestLinodeDatabasePostgreSQLInstanceResumeToolInputValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceResumeTool(cfg)

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingInstanceID, args: map[string]any{keyConfirm: true}},
		{name: caseStringInstanceID, args: map[string]any{databaseInstanceIDParam: "123", keyConfirm: true}},
		{name: caseSlashInstanceID, args: map[string]any{databaseInstanceIDParam: "/", keyConfirm: true}},
		{name: caseQueryInstanceID, args: map[string]any{databaseInstanceIDParam: databaseInvalidInstanceIDQuery, keyConfirm: true}},
		{name: caseTraversalInstanceID, args: map[string]any{databaseInstanceIDParam: pathTraversalValue, keyConfirm: true}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, databaseInstanceIDMessage) {
				t.Errorf("textContent.Text does not contain %v", databaseInstanceIDMessage)
			}
		})
	}
}

func TestLinodeDatabasePostgreSQLInstanceResumeToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databasePostgreSQLInstanceResumePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstanceResumePath)
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
	_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceResumeTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to resume PostgreSQL Managed Database instance") {
		t.Errorf("textContent.Text does not contain %v", "Failed to resume PostgreSQL Managed Database instance")
	}
}

func TestLinodeDatabaseEngineGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDatabaseEngineGetTool(cfg)

	if tool.Name != "linode_database_engine_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_database_engine_get")
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

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, databaseEngineIDParam) {
		t.Errorf("RawInputSchema missing key %v", databaseEngineIDParam)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeDatabaseEngineGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != databaseEngineEscapedPath {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), databaseEngineEscapedPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.DatabaseEngine{ID: databaseEngineID, Engine: databaseEngineName, Version: databaseVersion}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeDatabaseEngineGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseEngineIDParam: databaseEngineID})

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

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, databaseEngineID) {
		t.Errorf("textContent.Text does not contain %v", databaseEngineID)
	}

	if !strings.Contains(textContent.Text, databaseEngineName) {
		t.Errorf("textContent.Text does not contain %v", databaseEngineName)
	}
}

func TestLinodeDatabaseEngineGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != databaseEngineEscapedPath {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), databaseEngineEscapedPath)
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
	_, _, handler := tools.NewLinodeDatabaseEngineGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseEngineIDParam: databaseEngineID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve Managed Database engine") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve Managed Database engine")
	}
}

func TestLinodeDatabaseEngineGetToolClientConfigurationError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseEngineGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseEngineIDParam: databaseEngineID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeDatabaseEngineGetToolEngineIdValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDatabaseEngineGetTool(cfg)

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: "missing engine id", args: map[string]any{}, wantMessage: databaseEngineIDRequiredMessage},
		{name: "numeric engine id", args: map[string]any{databaseEngineIDParam: 123}, wantMessage: databaseEngineIDRequiredMessage},
		{name: "blank engine id", args: map[string]any{databaseEngineIDParam: ""}, wantMessage: databaseEngineIDRequiredMessage},
		{name: "query engine id", args: map[string]any{databaseEngineIDParam: "mysql?version=8"}, wantMessage: databaseEngineIDSeparatorMessage},
		{name: "fragment engine id", args: map[string]any{databaseEngineIDParam: "mysql#8.0.26"}, wantMessage: databaseEngineIDSeparatorMessage},
		{name: "traversal engine id", args: map[string]any{databaseEngineIDParam: "mysql/.."}, wantMessage: databaseEngineIDSeparatorMessage},
		{name: "leading slash engine id", args: map[string]any{databaseEngineIDParam: "/mysql/8.0.26"}, wantMessage: databaseEngineIDShapeMessage},
		{name: "trailing slash engine id", args: map[string]any{databaseEngineIDParam: "mysql/"}, wantMessage: databaseEngineIDShapeMessage},
		{name: "repeated slash engine id", args: map[string]any{databaseEngineIDParam: "mysql//8.0.26"}, wantMessage: databaseEngineIDShapeMessage},
		{name: "extra segment engine id", args: map[string]any{databaseEngineIDParam: "mysql/8.0.26/extra"}, wantMessage: databaseEngineIDShapeMessage},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}
