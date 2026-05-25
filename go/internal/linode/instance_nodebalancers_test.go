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

func TestListInstanceNodeBalancers(t *testing.T) {
	t.Parallel()

	nodeBalancers := []linode.NodeBalancer{{ID: 456, Label: "app-lb", Region: regionUSEast}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/instances/123/nodebalancers", r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query params")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: nodeBalancers, keyPage: 1, keyPages: 1, keyResults: 1,
		}), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListInstanceNodeBalancers(t.Context(), 123)

	require.NoError(t, err, "ListInstanceNodeBalancers should not fail")
	require.Len(t, result, 1, "result should include one NodeBalancer")
	assert.Equal(t, 456, result[0].ID, "NodeBalancer ID should match")
	assert.Equal(t, "app-lb", result[0].Label, "NodeBalancer label should match")
}

func TestListInstanceNodeBalancersRejectsInvalidLinodeID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		linodeID int
	}{
		{name: "zero linode ID", linodeID: 0},
		{name: "negative linode ID", linodeID: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := linode.NewClient("https://api.example.test/v4", "test-token", nil, linode.WithMaxRetries(0))
			result, err := client.ListInstanceNodeBalancers(t.Context(), tt.linodeID)

			require.Error(t, err, "invalid linode ID should fail")
			assert.Nil(t, result, "result should be nil on validation failure")
			assert.ErrorIs(t, err, linode.ErrLinodeIDPositive, "error should require a positive linode ID")
		})
	}
}
