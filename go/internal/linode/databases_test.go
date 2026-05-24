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
	databaseEnginesPath                  = "/databases/engines"
	databaseEngineMySQL                  = "mysql"
	databaseEngineID                     = "mysql/8.0.26"
	databaseEngineEscapedPath            = "/databases/engines/mysql%2F8.0.26"
	databaseEngineVersion                = "8.0.26"
	databaseInstancesPath                = "/databases/mysql/instances"
	databasePostgreSQLInstancesPath      = "/databases/postgresql/instances"
	databaseMySQLConfigPath              = "/databases/mysql/config"
	databasePostgreSQLConfigPath         = "/databases/postgresql/config"
	databaseInstanceID                   = 123
	databaseInstancePath                 = "/databases/mysql/instances/123"
	databaseInstanceSSLPath              = "/databases/mysql/instances/123/ssl"
	databaseSSLCACertificate             = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t"
	databaseInstanceCredentialsPath      = "/databases/mysql/instances/123/credentials"
	databaseInstanceCredentialsResetPath = "/databases/mysql/instances/123/credentials/reset"
	databaseInstancePatchPath            = "/databases/mysql/instances/123/patch"
	databaseInstanceSuspendPath          = "/databases/mysql/instances/123/suspend"
	databaseInstanceResumePath           = "/databases/mysql/instances/123/resume"
	databaseInstanceLabel                = "primary-db"
	databaseInstanceType                 = "g6-dedicated-2"
	databaseCredentialsPassword          = "secret"
	databaseConfigTypeInteger            = "integer"
	databaseConfigMaxConnections         = "max_connections"
	databaseAllowListCIDR                = "203.0.113.0/24"
	databaseEnginePostgreSQLID           = "postgresql/16"
	databaseEnginePostgreSQL             = "postgresql"
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
				keyDescription: "The minimum amount of time in seconds to keep binlog entries before deletion.",
				keyExample:     600,
				keyType:        databaseConfigTypeInteger,
			},
			"mysql": map[string]any{
				"connect_timeout": map[string]any{
					keyDescription: "The number of seconds that the mysqld server waits for a connect packet.",
					keyExample:     10,
					keyType:        databaseConfigTypeInteger,
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

func TestClientGetDatabasePostgreSQLConfigSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, databasePostgreSQLConfigPath, r.URL.Path, "request path should be /databases/postgresql/config")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"pg": map[string]any{
				databaseConfigMaxConnections: map[string]any{
					keyDescription: "Sets the maximum number of concurrent connections.",
					keyExample:     100,
					keyType:        databaseConfigTypeInteger,
				},
			},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetDatabasePostgreSQLConfig(t.Context())

	require.NoError(t, err, "GetDatabasePostgreSQLConfig should succeed on 200 response")
	assert.Contains(t, got, "pg")
}

func TestClientGetDatabasePostgreSQLConfigAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, databasePostgreSQLConfigPath, r.URL.Path, "request path should be /databases/postgresql/config")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetDatabasePostgreSQLConfig(t.Context())

	require.Error(t, err)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetDatabasePostgreSQLConfigRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		assert.Equal(t, databasePostgreSQLConfigPath, r.URL.Path, "request path should be /databases/postgresql/config")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"pg": map[string]any{
				databaseConfigMaxConnections: map[string]any{keyType: databaseConfigTypeInteger},
			},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetDatabasePostgreSQLConfig(t.Context())

	require.NoError(t, err, "read-only GetDatabasePostgreSQLConfig should retry transient failures")
	assert.Contains(t, got, "pg")
	assert.Equal(t, int32(2), attempts.Load(), "transient read should be retried once")
}

func TestClientListDatabaseInstancesSuccess(t *testing.T) {
	t.Parallel()

	instances := []linode.DatabaseInstance{{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineMySQL, Version: databaseEngineVersion, Status: oauthClientStatus}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, databaseInstancesPath, r.URL.Path, "request path should be /databases/mysql/instances")
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
		assert.Equal(t, databaseInstancesPath, r.URL.Path, "request path should be /databases/mysql/instances")
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

		assert.Equal(t, databaseInstancesPath, r.URL.Path, "request path should be /databases/mysql/instances")

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

func TestClientListDatabasePostgreSQLInstancesSuccess(t *testing.T) {
	t.Parallel()

	instances := []linode.DatabaseInstance{{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEnginePostgreSQL, Version: databaseEngineVersion, Status: oauthClientStatus}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, databasePostgreSQLInstancesPath, r.URL.Path, "request path should be /databases/postgresql/instances")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    instances,
			keyPage:    2,
			keyPages:   3,
			keyResults: 51,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListDatabasePostgreSQLInstances(t.Context(), 2, 25)

	require.NoError(t, err, "ListDatabasePostgreSQLInstances should succeed on 200 response")
	require.Len(t, got, 1)
	assert.Equal(t, databaseInstanceID, got[0].ID)
	assert.Equal(t, databaseInstanceLabel, got[0].Label)
	assert.Equal(t, databaseEnginePostgreSQL, got[0].Engine)
}

func TestClientListDatabasePostgreSQLInstancesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, databasePostgreSQLInstancesPath, r.URL.Path, "request path should be /databases/postgresql/instances")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.ListDatabasePostgreSQLInstances(t.Context(), 0, 0)

	require.Error(t, err)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListDatabasePostgreSQLInstancesRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		assert.Equal(t, databasePostgreSQLInstancesPath, r.URL.Path, "request path should be /databases/postgresql/instances")

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
	got, err := client.ListDatabasePostgreSQLInstances(t.Context(), 0, 0)

	require.NoError(t, err, "read-only ListDatabasePostgreSQLInstances should retry transient failures")
	require.Len(t, got, 1)
	assert.Equal(t, int32(2), attempts.Load(), "transient read should be retried once")
}

func TestClientGetDatabaseInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, databaseInstancePath, r.URL.Path, "request path should include instance id")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineMySQL, Version: databaseEngineVersion, Status: oauthClientStatus}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetDatabaseInstance(t.Context(), databaseInstanceID)

	require.NoError(t, err, "GetDatabaseInstance should succeed on 200 response")
	require.NotNil(t, got)
	assert.Equal(t, databaseInstanceID, got.ID)
	assert.Equal(t, databaseInstanceLabel, got.Label)
}

func TestClientGetDatabaseInstanceSSLSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, databaseInstanceSSLPath, r.URL.Path, "request path should include instance ssl path")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.DatabaseSSL{CACertificate: databaseSSLCACertificate}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetDatabaseInstanceSSL(t.Context(), databaseInstanceID)

	require.NoError(t, err, "GetDatabaseInstanceSSL should succeed on 200 response")
	require.NotNil(t, got)
	assert.Equal(t, databaseSSLCACertificate, got.CACertificate)
}

func TestClientGetDatabaseInstanceSSLAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, databaseInstanceSSLPath, r.URL.Path, "request path should include instance ssl path")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetDatabaseInstanceSSL(t.Context(), databaseInstanceID)

	require.Error(t, err)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetDatabaseInstanceSSLRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		assert.Equal(t, databaseInstanceSSLPath, r.URL.Path, "request path should include instance ssl path")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.DatabaseSSL{CACertificate: databaseSSLCACertificate}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetDatabaseInstanceSSL(t.Context(), databaseInstanceID)

	require.NoError(t, err, "read-only GetDatabaseInstanceSSL should retry transient failures")
	require.NotNil(t, got)
	assert.Equal(t, databaseSSLCACertificate, got.CACertificate)
	assert.Equal(t, int32(2), attempts.Load(), "transient read should be retried once")
}

func TestClientGetDatabaseInstanceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, databaseInstancePath, r.URL.Path, "request path should include instance id")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetDatabaseInstance(t.Context(), databaseInstanceID)

	require.Error(t, err)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetDatabaseInstanceRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		assert.Equal(t, databaseInstancePath, r.URL.Path, "request path should include instance id")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetDatabaseInstance(t.Context(), databaseInstanceID)

	require.NoError(t, err, "read-only GetDatabaseInstance should retry transient failures")
	require.NotNil(t, got)
	assert.Equal(t, int32(2), attempts.Load(), "transient read should be retried once")
}

func TestClientGetDatabaseInstanceCredentialsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, databaseInstanceCredentialsPath, r.URL.Path, "request path should include instance credentials path")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.DatabaseCredentials{Username: accountMaintenanceEntityType, Password: databaseCredentialsPassword}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetDatabaseInstanceCredentials(t.Context(), databaseInstanceID)

	require.NoError(t, err, "GetDatabaseInstanceCredentials should succeed on 200 response")
	require.NotNil(t, got)
	assert.Equal(t, "linode", got.Username)
	assert.Equal(t, "secret", got.Password)
}

func TestClientGetDatabaseInstanceCredentialsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, databaseInstanceCredentialsPath, r.URL.Path, "request path should include instance credentials path")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetDatabaseInstanceCredentials(t.Context(), databaseInstanceID)

	require.Error(t, err)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetDatabaseInstanceCredentialsRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		assert.Equal(t, databaseInstanceCredentialsPath, r.URL.Path, "request path should include instance credentials path")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.DatabaseCredentials{Username: accountMaintenanceEntityType, Password: databaseCredentialsPassword}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetDatabaseInstanceCredentials(t.Context(), databaseInstanceID)

	require.NoError(t, err, "read-only GetDatabaseInstanceCredentials should retry transient failures")
	require.NotNil(t, got)
	assert.Equal(t, int32(2), attempts.Load(), "transient read should be retried once")
}

func TestClientResetDatabaseInstanceCredentialsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, databaseInstanceCredentialsResetPath, r.URL.Path, "request path should include credentials reset path")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.NoBody, r.Body, "request body should be empty")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.DatabaseCredentials{Username: accountMaintenanceEntityType, Password: databaseCredentialsPassword}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ResetDatabaseInstanceCredentials(t.Context(), databaseInstanceID)

	require.NoError(t, err, "ResetDatabaseInstanceCredentials should succeed on 200 response")
	require.NotNil(t, got)
	assert.Equal(t, "linode", got.Username)
	assert.Equal(t, "secret", got.Password)
}

func TestClientResetDatabaseInstanceCredentialsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, databaseInstanceCredentialsResetPath, r.URL.Path, "request path should include credentials reset path")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.ResetDatabaseInstanceCredentials(t.Context(), databaseInstanceID)

	require.Error(t, err)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientResetDatabaseInstanceCredentialsDoesNotRetryTransientPost(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, databaseInstanceCredentialsResetPath, r.URL.Path, "request path should include credentials reset path")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	_, err := client.ResetDatabaseInstanceCredentials(t.Context(), databaseInstanceID)

	require.Error(t, err)
	assert.Equal(t, int32(1), attempts.Load(), "credential reset POST must not be retried")
}

func TestClientCreateDatabaseInstanceSuccess(t *testing.T) {
	t.Parallel()

	privateNetwork := true
	sslConnection := true
	expectedReq := linode.CreateDatabaseInstanceRequest{
		Label:          databaseInstanceLabel,
		Type:           databaseInstanceType,
		Engine:         databaseEngineID,
		Region:         regionUSEast,
		AllowList:      []string{databaseAllowListCIDR},
		ClusterSize:    3,
		EngineConfig:   map[string]any{databaseConfigMaxConnections: float64(100)},
		PrivateNetwork: &privateNetwork,
		SSLConnection:  &sslConnection,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, databaseInstancesPath, r.URL.Path, "request path should be /databases/mysql/instances")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		var gotReq linode.CreateDatabaseInstanceRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&gotReq))
		assert.Equal(t, expectedReq, gotReq)

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineMySQL, Version: databaseEngineVersion, Status: oauthClientStatus}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateDatabaseInstance(t.Context(), &expectedReq)

	require.NoError(t, err, "CreateDatabaseInstance should succeed on 200 response")
	require.NotNil(t, got)
	assert.Equal(t, databaseInstanceID, got.ID)
	assert.Equal(t, databaseInstanceLabel, got.Label)
}

func TestClientCreateDatabaseInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, databaseInstancesPath, r.URL.Path, "request path should be /databases/mysql/instances")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	_, err := client.CreateDatabaseInstance(t.Context(), &linode.CreateDatabaseInstanceRequest{Label: databaseInstanceLabel, Type: databaseInstanceType, Engine: databaseEngineID, Region: regionUSEast})

	require.Error(t, err)
	assert.Equal(t, int32(1), attempts.Load(), "non-idempotent create POST must not be retried")
}

func TestClientCreateDatabasePostgreSQLInstanceSuccess(t *testing.T) {
	t.Parallel()

	privateNetwork := true
	sslConnection := true
	expectedReq := linode.CreateDatabaseInstanceRequest{
		Label:          databaseInstanceLabel,
		Type:           databaseInstanceType,
		Engine:         databaseEnginePostgreSQLID,
		Region:         regionUSEast,
		AllowList:      []string{databaseAllowListCIDR},
		ClusterSize:    3,
		EngineConfig:   map[string]any{databaseConfigMaxConnections: float64(100)},
		PrivateNetwork: &privateNetwork,
		SSLConnection:  &sslConnection,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, databasePostgreSQLInstancesPath, r.URL.Path, "request path should be /databases/postgresql/instances")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		var gotReq linode.CreateDatabaseInstanceRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&gotReq))
		assert.Equal(t, expectedReq, gotReq)

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEnginePostgreSQL, Version: databaseEngineVersion, Status: oauthClientStatus}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateDatabasePostgreSQLInstance(t.Context(), &expectedReq)

	require.NoError(t, err, "CreateDatabasePostgreSQLInstance should succeed on 200 response")
	require.NotNil(t, got)
	assert.Equal(t, databaseInstanceID, got.ID)
	assert.Equal(t, databaseInstanceLabel, got.Label)
}

func TestClientCreateDatabasePostgreSQLInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, databasePostgreSQLInstancesPath, r.URL.Path, "request path should be /databases/postgresql/instances")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	_, err := client.CreateDatabasePostgreSQLInstance(t.Context(), &linode.CreateDatabaseInstanceRequest{Label: databaseInstanceLabel, Type: databaseInstanceType, Engine: databaseEnginePostgreSQLID, Region: regionUSEast})

	require.Error(t, err)
	assert.Equal(t, int32(1), attempts.Load(), "non-idempotent PostgreSQL create POST must not be retried")
}

func TestClientUpdateDatabaseInstanceSuccess(t *testing.T) {
	t.Parallel()

	label := databaseInstanceLabel
	databaseType := databaseInstanceType
	version := databaseEngineVersion
	allowList := []string{databaseAllowListCIDR}
	expectedReq := linode.UpdateDatabaseInstanceRequest{
		AllowList:      &allowList,
		EngineConfig:   map[string]any{"binlog_retention_period": float64(600)},
		Label:          &label,
		PrivateNetwork: map[string]any{"public_access": false, "vpc_id": float64(123), "subnet_id": float64(456)},
		Type:           &databaseType,
		Updates:        map[string]any{"frequency": "weekly", "hour_of_day": float64(1)},
		Version:        &version,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, databaseInstancePath, r.URL.Path, "request path should include instance id")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		var gotReq linode.UpdateDatabaseInstanceRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&gotReq))
		assert.Equal(t, expectedReq, gotReq)

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineMySQL, Version: databaseEngineVersion, Status: oauthClientStatus}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateDatabaseInstance(t.Context(), databaseInstanceID, &expectedReq)

	require.NoError(t, err, "UpdateDatabaseInstance should succeed on 200 response")
	require.NotNil(t, got)
	assert.Equal(t, databaseInstanceID, got.ID)
	assert.Equal(t, databaseInstanceLabel, got.Label)
}

func TestClientUpdateDatabaseInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	label := databaseInstanceLabel

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, databaseInstancePath, r.URL.Path, "request path should include instance id")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	_, err := client.UpdateDatabaseInstance(t.Context(), databaseInstanceID, &linode.UpdateDatabaseInstanceRequest{Label: &label})

	require.Error(t, err)
	assert.Equal(t, int32(1), attempts.Load(), "side-effecting update PUT must not be retried")
}

func TestClientDeleteDatabaseInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, databaseInstancePath, r.URL.Path, "request path should include instance id")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.NoBody, r.Body, "delete request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteDatabaseInstance(t.Context(), databaseInstanceID)

	require.NoError(t, err, "DeleteDatabaseInstance should succeed on 200 response")
}

func TestClientDeleteDatabaseInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, databaseInstancePath, r.URL.Path, "request path should include instance id")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	err := client.DeleteDatabaseInstance(t.Context(), databaseInstanceID)

	require.Error(t, err)
	assert.Equal(t, int32(1), attempts.Load(), "destructive database DELETE must not be retried")
}

func TestClientPatchDatabaseInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, databaseInstancePatchPath, r.URL.Path, "request path should include instance id and patch suffix")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.NoBody, r.Body, "patch request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.PatchDatabaseInstance(t.Context(), databaseInstanceID)

	require.NoError(t, err, "PatchDatabaseInstance should succeed on 200 response")
}

func TestClientPatchDatabaseInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, databaseInstancePatchPath, r.URL.Path, "request path should include instance id and patch suffix")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	err := client.PatchDatabaseInstance(t.Context(), databaseInstanceID)

	require.Error(t, err)
	assert.Equal(t, int32(1), attempts.Load(), "side-effecting patch POST must not be retried")
}

func TestClientSuspendDatabaseInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, databaseInstanceSuspendPath, r.URL.Path, "request path should include instance id and suspend suffix")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.NoBody, r.Body, "suspend request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.SuspendDatabaseInstance(t.Context(), databaseInstanceID)

	require.NoError(t, err, "SuspendDatabaseInstance should succeed on 200 response")
}

func TestClientSuspendDatabaseInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, databaseInstanceSuspendPath, r.URL.Path, "request path should include instance id and suspend suffix")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	err := client.SuspendDatabaseInstance(t.Context(), databaseInstanceID)

	require.Error(t, err)
	assert.Equal(t, int32(1), attempts.Load(), "side-effecting suspend POST must not be retried")
}

func TestClientResumeDatabaseInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, databaseInstanceResumePath, r.URL.Path, "request path should include instance id and resume suffix")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.NoBody, r.Body, "resume request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.ResumeDatabaseInstance(t.Context(), databaseInstanceID)

	require.NoError(t, err, "ResumeDatabaseInstance should succeed on 200 response")
}

func TestClientResumeDatabaseInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, databaseInstanceResumePath, r.URL.Path, "request path should include instance id and resume suffix")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	err := client.ResumeDatabaseInstance(t.Context(), databaseInstanceID)

	require.Error(t, err)
	assert.Equal(t, int32(1), attempts.Load(), "side-effecting resume POST must not be retried")
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
