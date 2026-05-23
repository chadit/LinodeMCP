package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	databaseEnginesPath       = "/databases/engines"
	databaseEngineMySQL       = "mysql"
	databaseEngineID          = "mysql/8.0.26"
	databaseEngineEscapedPath = "/databases/engines/mysql%2F8.0.26"
	databaseEngineVersion     = "8.0.26"
	databaseInstancesPath     = "/databases/instances"
	databaseMySQLConfigPath   = "/databases/mysql/config"
	databaseInstanceID        = 123
	databaseInstanceLabel     = "primary-db"
	databaseConfigTypeInteger = "integer"
)

func TestClientListDatabaseEnginesSuccess(t *testing.T) {
	t.Parallel()

	engines := []linode.DatabaseEngine{{ID: databaseEngineID, Engine: databaseEngineMySQL, Version: databaseEngineVersion}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, databaseEnginesPath, r.URL.Path, "request path should be /databases/engines")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    engines,
			keyPage:    2,
			keyPages:   3,
			keyResults: 51,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListDatabaseEngines(t.Context(), 2, 25)

	require.NoError(t, err, "ListDatabaseEngines should succeed on 200 response")
	require.Len(t, got, 1)
	assert.Equal(t, databaseEngineID, got[0].ID)
	assert.Equal(t, databaseEngineMySQL, got[0].Engine)
	assert.Equal(t, databaseEngineVersion, got[0].Version)
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

func TestClientGetDatabaseMySQLConfigSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, databaseMySQLConfigPath, r.URL.Path, "request path should be /databases/mysql/config")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"binlog_retention_period": map[string]any{
				"description": "The minimum amount of time in seconds to keep binlog entries before deletion.",
				"example":     600,
				keyType:       databaseConfigTypeInteger,
			},
			"mysql": map[string]any{
				"connect_timeout": map[string]any{
					"description": "The number of seconds that the mysqld server waits for a connect packet.",
					"example":     10,
					keyType:       databaseConfigTypeInteger,
				},
			},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetDatabaseMySQLConfig(t.Context())

	require.NoError(t, err, "GetDatabaseMySQLConfig should succeed on 200 response")
	assert.Contains(t, got, "binlog_retention_period")
	assert.Contains(t, got, "mysql")
}

func TestClientGetDatabaseMySQLConfigAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, databaseMySQLConfigPath, r.URL.Path, "request path should be /databases/mysql/config")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetDatabaseMySQLConfig(t.Context())

	require.Error(t, err)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetDatabaseMySQLConfigRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		assert.Equal(t, databaseMySQLConfigPath, r.URL.Path, "request path should be /databases/mysql/config")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"mysql": map[string]any{
				"connect_timeout": map[string]any{keyType: databaseConfigTypeInteger},
			},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetDatabaseMySQLConfig(t.Context())

	require.NoError(t, err, "read-only GetDatabaseMySQLConfig should retry transient failures")
	assert.Contains(t, got, "mysql")
	assert.Equal(t, int32(2), attempts.Load(), "transient read should be retried once")
}

func TestClientListDatabaseInstancesSuccess(t *testing.T) {
	t.Parallel()

	instances := []linode.DatabaseInstance{{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: "g6-dedicated-2", Engine: databaseEngineMySQL, Version: databaseEngineVersion, Status: oauthClientStatus}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, databaseInstancesPath, r.URL.Path, "request path should be /databases/instances")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":     instances,
			keyPage:    2,
			keyPages:   3,
			keyResults: 51,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListDatabaseInstances(t.Context(), 2, 25)

	require.NoError(t, err, "ListDatabaseInstances should succeed on 200 response")
	require.Len(t, got, 1)
	assert.Equal(t, databaseInstanceID, got[0].ID)
	assert.Equal(t, databaseInstanceLabel, got[0].Label)
	assert.Equal(t, databaseEngineMySQL, got[0].Engine)
}

func TestClientListDatabaseInstancesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, databaseInstancesPath, r.URL.Path, "request path should be /databases/instances")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.ListDatabaseInstances(t.Context(), 0, 0)

	require.Error(t, err)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListDatabaseInstancesRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		assert.Equal(t, databaseInstancesPath, r.URL.Path, "request path should be /databases/instances")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []linode.DatabaseInstance{{ID: databaseInstanceID, Label: databaseInstanceLabel}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListDatabaseInstances(t.Context(), 0, 0)

	require.NoError(t, err, "read-only ListDatabaseInstances should retry transient failures")
	require.Len(t, got, 1)
	assert.Equal(t, int32(2), attempts.Load(), "transient read should be retried once")
}

func TestClientGetDatabaseEngineSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, databaseEngineEscapedPath, r.URL.EscapedPath(), "request path should escape engine id")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.DatabaseEngine{ID: databaseEngineID, Engine: databaseEngineMySQL, Version: databaseEngineVersion}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetDatabaseEngine(t.Context(), databaseEngineID)

	require.NoError(t, err, "GetDatabaseEngine should succeed on 200 response")
	require.NotNil(t, got, "engine should not be nil")
	assert.Equal(t, databaseEngineID, got.ID)
	assert.Equal(t, databaseEngineMySQL, got.Engine)
	assert.Equal(t, databaseEngineVersion, got.Version)
}

func TestClientGetDatabaseEngineAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, databaseEngineEscapedPath, r.URL.EscapedPath(), "request path should escape engine id")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetDatabaseEngine(t.Context(), databaseEngineID)

	require.Error(t, err)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetDatabaseEngineRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		assert.Equal(t, databaseEngineEscapedPath, r.URL.EscapedPath(), "request path should escape engine id")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.DatabaseEngine{ID: databaseEngineID, Engine: databaseEngineMySQL, Version: databaseEngineVersion}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetDatabaseEngine(t.Context(), databaseEngineID)

	require.NoError(t, err, "read-only GetDatabaseEngine should retry transient failures")
	require.NotNil(t, got, "engine should not be nil")
	assert.Equal(t, int32(2), attempts.Load(), "transient read should be retried once")
}
