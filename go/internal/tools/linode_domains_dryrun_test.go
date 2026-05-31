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
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without importing", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeDomainImportTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDomain:           domainExample,
			keyRemoteNameserver: remoteNameserverExample,
			keyDryRun:           true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_domain_import", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/domains/import", would["path"])
		assert.Nil(t, body["current_state"])
	})

	t.Run("still validates domain", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeDomainImportTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRemoteNameserver: remoteNameserverExample,
			keyDryRun:           true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "domain is required")
	})
}

func TestLinodeDomainCloneToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDomainCloneTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
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
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_domain_clone", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/domains/333/clone", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates domain_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeDomainCloneTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDomain: domainExample,
			keyDryRun: true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "domain_id must be a positive integer")
	})
}

func TestLinodeDomainCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDomainCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeDomainCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDomain: domainExample,
			keyType:   "master",
			keyDryRun: true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_domain_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/domains", would["path"])
		assert.Nil(t, body["current_state"])

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "create surfaces the new-domain side effect")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, domainExample, "side effect should name the new domain")
		assert.Contains(t, effect, "master", "side effect should state the domain type")
	})

	t.Run("still validates domain", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeDomainCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyType:   "master",
			keyDryRun: true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "domain is required")
	})
}

func TestLinodeDomainRecordCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDomainRecordCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeDomainRecordCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDomainID: float64(333),
			keyType:     "A",
			keyTarget:   testPublicIPv4,
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_domain_record_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/domains/333/records", would["path"])
		assert.Nil(t, body["current_state"])

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "create surfaces the new-record side effect")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, "A record", "side effect should name the record type")
		assert.Contains(t, effect, testPublicIPv4, "side effect should name the target")
	})

	t.Run("still validates domain_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeDomainRecordCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyType:   "A",
			keyTarget: testPublicIPv4,
			keyDryRun: true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "domain_id is required")
	})
}

func TestLinodeDomainRecordUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDomainRecordUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
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
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_domain_record_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, "/domains/333/records/555", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "update surfaces the target change")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, "192.0.2.2", "side effect names the new target")
	})

	t.Run("still validates domain_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeDomainRecordUpdateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRecordID: float64(555),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "domain_id is required")
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
	require.NoError(t, err)
	require.False(t, result.IsError)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	assert.Equal(t, "linode_domain_delete", body["tool"])

	deps, _ := body["dependencies"].([]any)
	require.Len(t, deps, 1, "only NS records are surfaced as dependencies")

	dep, gotMap := deps[0].(map[string]any)
	require.True(t, gotMap)
	assert.Equal(t, "ns_record", dep["kind"])

	warnings, _ := body["warnings"].([]any)
	require.NotEmpty(t, warnings)

	warning, gotString := warnings[0].(string)
	require.True(t, gotString)
	assert.Contains(t, warning, "2 DNS record(s)")

	assert.NotContains(t, *methods, http.MethodDelete, "dry_run must not issue a DELETE")
}
