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

const instanceFirewallLabelFixture = "web-firewall"

func TestClientListInstanceFirewalls(t *testing.T) {
	t.Parallel()

	firewalls := []linode.Firewall{{ID: 456, Label: instanceFirewallLabelFixture, Status: "enabled"}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/instances/123/firewalls", r.URL.Path, "request path should match")
		assert.Equal(t, "2", r.URL.Query().Get("page"), "page query should match")
		assert.Equal(t, "50", r.URL.Query().Get("page_size"), "page_size query should match")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: firewalls, keyPage: 2, keyPages: 3, keyResults: 1,
		}), "encoding response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListInstanceFirewalls(t.Context(), 123, 2, 50)

	require.NoError(t, err, "list instance firewalls should not fail")
	require.Len(t, got, 1, "one firewall should be returned")
	assert.Equal(t, instanceFirewallLabelFixture, got[0].Label, "firewall label should match")
}

func TestClientListInstanceFirewallsRejectsInvalidLinodeID(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.com/v4", "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListInstanceFirewalls(t.Context(), 0, 0, 0)

	require.ErrorIs(t, err, linode.ErrLinodeIDPositive, "invalid linode ID should be rejected before request")
	assert.Nil(t, got, "no firewalls should be returned")
}

func TestClientListInstanceFirewallsHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances/123/firewalls", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}), "encoding error response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListInstanceFirewalls(t.Context(), 123, 0, 0)

	require.Error(t, err, "HTTP error should be returned")
	assert.Nil(t, got, "no firewalls should be returned")
}
