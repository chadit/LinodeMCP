package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// expect* helpers are fatal package-local checks from linode_assertions_test.go; check* helpers are nonfatal.

func TestLinodeNodeBalancerVPCConfigGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeNodeBalancerVPCConfigGetTool(cfg)

	if tool.Name != "linode_nodebalancer_vpc_config_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_nodebalancer_vpc_config_get")
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
	for _, key := range []string{keyNodeBalancerID, keyVPCConfigID} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeNodeBalancerVPCConfigGetToolRequiredArguments(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeNodeBalancerVPCConfigGetTool(cfg)

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing nodebalancer id", args: map[string]any{keyVPCConfigID: 456}, want: "nodebalancer_id is required"},
		{name: "missing vpc config id", args: map[string]any{keyNodeBalancerID: 123}, want: "vpc_config_id is required"},
		{name: "bad vpc config id", args: map[string]any{keyNodeBalancerID: 123, keyVPCConfigID: 0}, want: "vpc_config_id must be a positive integer"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLinodeNodeBalancerVPCConfigGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/nodebalancers/123/vpcs/456" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/nodebalancers/123/vpcs/456")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyBetaID:                456,
			keyVPCID:                 789,
			keyNodeBalancerID:        123,
			"subnet_id":              321,
			"ipv4_range":             "10.100.5.100/30",
			"ipv4_range_auto_assign": false,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeNodeBalancerVPCConfigGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyNodeBalancerID: 123, keyVPCConfigID: 456})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if len(result.Content) == 0 {
		t.Error("result.Content is empty")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if out["id"] != float64(456) {
		t.Errorf("id = %v, want 456", out["id"])
	}

	if out["vpc_id"] != float64(789) {
		t.Errorf("vpc_id = %v, want 789", out["vpc_id"])
	}

	if out["nodebalancer_id"] != float64(123) {
		t.Errorf("nodebalancer_id = %v, want 123", out["nodebalancer_id"])
	}

	if out["ipv4_range"] != "10.100.5.100/30" {
		t.Errorf("ipv4_range = %v, want 10.100.5.100/30", out["ipv4_range"])
	}
}

func TestLinodeNodeBalancerVPCConfigGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/nodebalancers/123/vpcs/456" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/nodebalancers/123/vpcs/456")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeNodeBalancerVPCConfigGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyNodeBalancerID: 123, keyVPCConfigID: 456})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve VPC configuration 456 for NodeBalancer 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve VPC configuration 456 for NodeBalancer 123")
	}
}

func TestLinodeNodeBalancerVPCConfigGetToolValidationRejectsBeforeClientCall(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeNodeBalancerVPCConfigGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyNodeBalancerID: 123})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if calls.Load() != int32(0) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
	}
}
