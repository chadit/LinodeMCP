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

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// dryRunNoCallServer returns a cfg pointed at a server that fails on ANY
// request, so a create-style dry-run (which fetches no state) is proven to
// issue zero HTTP calls.
func dryRunNoCallServer(t *testing.T) *config.Config {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("create dry_run must not issue any request; got %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
}

func TestLinodeDomainImportToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDomainImportTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without importing", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeDomainImportTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDomain:           domainExample,
			keyRemoteNameserver: remoteNameserverExample,
			keyDryRun:           true,
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

		if !reflect.DeepEqual(body["tool"], "linode_domain_import") {
			t.Errorf("got %v, want %v", body["tool"], "linode_domain_import")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], "/domains/import") {
			t.Errorf("got %v, want %v", would["path"], "/domains/import")
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}
	})

	t.Run("still validates domain", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeDomainImportTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRemoteNameserver: remoteNameserverExample,
			keyDryRun:           true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "domain is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "domain is required")
		}
	})
}

func TestLinodeDomainCloneToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDomainCloneTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without cloning", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/domains/333", linode.Domain{ID: 333, Domain: domainExample})
		_, _, handler := tools.NewLinodeDomainCloneTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDomainID: float64(333),
			keyDomain:   domainExample,
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

		if !reflect.DeepEqual(body["tool"], "linode_domain_clone") {
			t.Errorf("got %v, want %v", body["tool"], "linode_domain_clone")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], "/domains/333/clone") {
			t.Errorf("got %v, want %v", would["path"], "/domains/333/clone")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})

	t.Run("still validates domain_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeDomainCloneTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDomain: domainExample,
			keyDryRun: true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "domain_id must be a positive integer") {
			t.Errorf("error text %q does not contain %q", text.Text, "domain_id must be a positive integer")
		}
	})
}

func TestLinodeDomainCreateToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeDomainCreateTool(&config.Config{})
	if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
	}
}

func TestLinodeDomainCreateToolDryRunPreviewWithoutCreating(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeDomainCreateTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyDomain: domainExample,
		keyType:   "master",
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

	if !reflect.DeepEqual(body["tool"], "linode_domain_create") {
		t.Errorf("got %v, want %v", body["tool"], "linode_domain_create")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/domains") {
		t.Errorf("got %v, want %v", would["path"], "/domains")
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

	if !strings.Contains(effect, domainExample) {
		t.Errorf("effect does not contain %v", domainExample)
	}

	if !strings.Contains(effect, "master") {
		t.Errorf("effect does not contain %v", "master")
	}
}

func TestLinodeDomainCreateToolDryRunStillValidatesDomain(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeDomainCreateTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyType:   "master",
		keyDryRun: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "domain is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "domain is required")
	}
}

func TestLinodeDomainRecordCreateToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeDomainRecordCreateTool(&config.Config{})
	if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
	}
}

func TestLinodeDomainRecordCreateToolDryRunPreviewWithoutCreating(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeDomainRecordCreateTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyDomainID: float64(333),
		keyType:     "A",
		keyTarget:   testPublicIPv4,
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

	if !reflect.DeepEqual(body["tool"], "linode_domain_record_create") {
		t.Errorf("got %v, want %v", body["tool"], "linode_domain_record_create")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/domains/333/records") {
		t.Errorf("got %v, want %v", would["path"], "/domains/333/records")
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

	if !strings.Contains(effect, "A record") {
		t.Errorf("effect does not contain %v", "A record")
	}

	if !strings.Contains(effect, testPublicIPv4) {
		t.Errorf("effect does not contain %v", testPublicIPv4)
	}
}

func TestLinodeDomainRecordCreateToolDryRunStillValidatesDomainId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeDomainRecordCreateTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyType:   "A",
		keyTarget: testPublicIPv4,
		keyDryRun: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "domain_id is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "domain_id is required")
	}
}

func TestLinodeDomainRecordUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDomainRecordUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/domains/333/records/555",
			linode.DomainRecord{ID: 555, Type: "A"})
		_, _, handler := tools.NewLinodeDomainRecordUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDomainID: float64(333),
			keyRecordID: float64(555),
			keyTarget:   "192.0.2.2",
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

		if !reflect.DeepEqual(body["tool"], "linode_domain_record_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_domain_record_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], "/domains/333/records/555") {
			t.Errorf("got %v, want %v", would["path"], "/domains/333/records/555")
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

		if !strings.Contains(effect, "192.0.2.2") {
			t.Errorf("effect does not contain %v", "192.0.2.2")
		}
	})

	t.Run("still validates domain_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeDomainRecordUpdateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRecordID: float64(555),
			keyDryRun:   true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "domain_id is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "domain_id is required")
		}
	})
}

// TestLinodeDomainDeleteToolDryRunDependencies exercises the Phase 2 Tier A
// walk: deleting a domain destroys all its records, so the NS records are
// surfaced as cascade dependencies and a warning reports the total count.
func TestLinodeDomainDeleteToolDryRunDependencies(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunRouteServer(t, map[string]any{
		"/domains/888": linode.Domain{ID: 888, Domain: "dry.example.com"},
		"/domains/888/records": linode.PaginatedResponse[linode.DomainRecord]{
			Data: []linode.DomainRecord{
				{ID: 1, Type: "NS", Target: "ns1.linode.com"},
				{ID: 2, Type: "A", Name: "www", Target: "1.2.3.4"},
			},
		},
	})

	_, _, handler := tools.NewLinodeDomainDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyDomainID: float64(888),
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

	if !reflect.DeepEqual(body["tool"], "linode_domain_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_domain_delete")
	}

	deps, _ := body["dependencies"].([]any)
	if len(deps) != 1 {
		t.Fatalf("len(deps) = %d, want %d", len(deps), 1)
	}

	dep, gotMap := deps[0].(map[string]any)
	if !gotMap {
		t.Fatal("gotMap = false, want true")
	}

	if !reflect.DeepEqual(dep[tcKind], "ns_record") {
		t.Errorf("got %v, want %v", dep[tcKind], "ns_record")
	}

	warnings, _ := body["warnings"].([]any)
	if len(warnings) == 0 {
		t.Fatal("warnings is empty")
	}

	warning, gotString := warnings[0].(string)
	if !gotString {
		t.Fatal("gotString = false, want true")
	}

	if !strings.Contains(warning, "2 DNS record(s)") {
		t.Errorf("warning does not contain %v", "2 DNS record(s)")
	}

	if slices.Contains(*methods, http.MethodDelete) {
		t.Errorf("*methods should not contain %v", http.MethodDelete)
	}
}
