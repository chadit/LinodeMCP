package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientListLKETierVersionsUsesTierPath(t *testing.T) {
	t.Parallel()

	versions := []linode.LKETierVersion{{ID: "1.33", Tier: "standard"}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should match")
		assert.Equal(t, "/lke/tiers/standard/versions", r.URL.Path, "request path should include tier")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: versions, keyPage: 1, keyPages: 1, keyResults: 1,
		}), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil)
	got, err := client.ListLKETierVersions(t.Context(), "standard")

	require.NoError(t, err, "listing tier versions should not fail")
	assert.Equal(t, versions, got, "tier versions should match response")
}

func TestClientListLKETierVersionsEscapesTierPathSegment(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/tiers/standard/enterprise/versions", r.URL.Path, "decoded path exposes separator")
		assert.Equal(t, "/lke/tiers/standard%2Fenterprise/versions", r.URL.EscapedPath(), "escaped path should keep tier in one segment")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []linode.LKETierVersion{}, keyPage: 1, keyPages: 1, keyResults: 0,
		}), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil)
	_, err := client.ListLKETierVersions(t.Context(), "standard/enterprise")

	require.NoError(t, err, "escaped tier request should not fail")
}
