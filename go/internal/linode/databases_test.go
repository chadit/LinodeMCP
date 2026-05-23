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

const (
	databaseEnginesPath = "/databases/engines"
	databaseEngineMySQL = "mysql"
)

func TestClientListDatabaseEnginesSuccess(t *testing.T) {
	t.Parallel()

	engines := []linode.DatabaseEngine{{ID: "mysql/8.0.26", Engine: databaseEngineMySQL, Version: "8.0.26"}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, databaseEnginesPath, r.URL.Path, "request path should be /databases/engines")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    engines,
			"page":    2,
			"pages":   3,
			"results": 51,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListDatabaseEngines(t.Context(), 2, 25)

	require.NoError(t, err, "ListDatabaseEngines should succeed on 200 response")
	require.Len(t, got, 1)
	assert.Equal(t, "mysql/8.0.26", got[0].ID)
	assert.Equal(t, databaseEngineMySQL, got[0].Engine)
	assert.Equal(t, "8.0.26", got[0].Version)
}

func TestClientListDatabaseEnginesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, databaseEnginesPath, r.URL.Path, "request path should be /databases/engines")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.ListDatabaseEngines(t.Context(), 0, 0)

	require.Error(t, err)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}
