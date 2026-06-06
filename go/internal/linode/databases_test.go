package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	databaseEnginesPath                       = "/databases/engines"
	databaseTypesPath                         = "/databases/types"
	databaseTypeID                            = "g6-dedicated-1"
	databaseTypeEscapedPath                   = "/databases/types/g6-dedicated-1"
	databaseTypeIDWithSeparators              = "g6/dedicated?plan=1"
	databaseTypeEscapedSeparatorPath          = "/databases/types/g6%2Fdedicated%3Fplan=1"
	databaseTypeLabel                         = "DBaaS - Dedicated 80GB"
	databaseEngineMySQL                       = "mysql"
	databaseEngineID                          = "mysql/8.0.26"
	databaseEngineEscapedPath                 = "/databases/engines/mysql%2F8.0.26"
	databaseEngineVersion                     = "8.0.26"
	databaseInstancesPath                     = "/databases/mysql/instances"
	databasePostgreSQLInstancesPath           = "/databases/postgresql/instances"
	databaseMySQLConfigPath                   = "/databases/mysql/config"
	databasePostgreSQLConfigPath              = "/databases/postgresql/config"
	databaseInstanceID                        = 123
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
	databaseInstanceType                      = "g6-dedicated-2"
	databaseCredentialsPassword               = "secret"
	databaseConfigTypeInteger                 = "integer"
	databaseConfigMaxConnections              = "max_connections"
	databaseAllowListCIDR                     = "203.0.113.0/24"
	databaseEnginePostgreSQLID                = "postgresql/16"
	databaseEnginePostgreSQL                  = "postgresql"
	databasePostgreSQLConfigNamespace         = "pg"
)

func TestClientListDatabaseEnginesSuccess(t *testing.T) {
	t.Parallel()

	engines := []linode.DatabaseEngine{{ID: databaseEngineID, Engine: databaseEngineMySQL, Version: databaseEngineVersion}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, databaseEnginesPath, r.URL.Path, "request path should be /databases/engines")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    engines,
			keyPage:    2,
			keyPages:   3,
			keyResults: 51,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListDatabaseEngines(t.Context(), 2, 25)

	mustNoError(t, err, "ListDatabaseEngines should succeed on 200 response")
	mustLen(t, got, 1)
	checkEqual(t, databaseEngineID, got[0].ID)
	checkEqual(t, databaseEngineMySQL, got[0].Engine)
	checkEqual(t, databaseEngineVersion, got[0].Version)
}

func TestClientListDatabaseEnginesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, databaseEnginesPath, r.URL.Path, "request path should be /databases/engines")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.ListDatabaseEngines(t.Context(), 0, 0)

	mustError(t, err)

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListDatabaseTypesSuccess(t *testing.T) {
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, databaseTypesPath, r.URL.Path, "request path should be /databases/types")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    types,
			keyPage:    2,
			keyPages:   3,
			keyResults: 51,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListDatabaseTypes(t.Context(), 2, 25)

	mustNoError(t, err, "ListDatabaseTypes should succeed on 200 response")
	mustLen(t, got, 1)
	checkEqual(t, databaseTypeID, got[0].ID)
	checkEqual(t, databaseTypeLabel, got[0].Label)
	checkEqual(t, 1, got[0].Engines.MySQL[0].Quantity)
}

func TestClientListDatabaseTypesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, databaseTypesPath, r.URL.Path, "request path should be /databases/types")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.ListDatabaseTypes(t.Context(), 0, 0)

	mustError(t, err)

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListDatabaseTypesRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	types := []linode.DatabaseType{{ID: databaseTypeID, Label: databaseTypeLabel}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		checkEqual(t, databaseTypesPath, r.URL.Path, "request path should be /databases/types")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    types,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListDatabaseTypes(t.Context(), 0, 0)

	mustNoError(t, err, "read-only ListDatabaseTypes should retry transient failures")
	mustLen(t, got, 1)
	checkEqual(t, databaseTypeID, got[0].ID)
	checkEqual(t, int32(2), attempts.Load(), "transient read should be retried once")
}

func TestClientGetDatabaseTypeSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, databaseTypeEscapedPath, r.URL.EscapedPath(), "request path should escape type id")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseType{ID: databaseTypeID, Label: databaseTypeLabel}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetDatabaseType(t.Context(), databaseTypeID, 2, 25)

	mustNoError(t, err, "GetDatabaseType should succeed on 200 response")
	mustNotNil(t, got, "database type should not be nil")
	checkEqual(t, databaseTypeID, got.ID)
	checkEqual(t, databaseTypeLabel, got.Label)
}

func TestClientGetDatabaseTypeEscapesPathSeparators(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, databaseTypeEscapedSeparatorPath, r.URL.EscapedPath(), "request path should escape separators in type id")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseType{ID: databaseTypeIDWithSeparators, Label: databaseTypeLabel}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetDatabaseType(t.Context(), databaseTypeIDWithSeparators, 0, 0)

	mustNoError(t, err)
	mustNotNil(t, got)
	checkEqual(t, databaseTypeIDWithSeparators, got.ID)
}

func TestClientGetDatabaseTypeAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, databaseTypeEscapedPath, r.URL.EscapedPath(), "request path should escape type id")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetDatabaseType(t.Context(), databaseTypeID, 0, 0)

	mustError(t, err)

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetDatabaseTypeRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		checkEqual(t, databaseTypeEscapedPath, r.URL.EscapedPath(), "request path should escape type id")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseType{ID: databaseTypeID, Label: databaseTypeLabel}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetDatabaseType(t.Context(), databaseTypeID, 0, 0)

	mustNoError(t, err)
	mustNotNil(t, got)
	checkEqual(t, int32(2), attempts.Load(), "transient read failures should be retried")
}

func TestClientGetDatabaseMySQLConfigSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, databaseMySQLConfigPath, r.URL.Path, "request path should be /databases/mysql/config")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
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

	mustNoError(t, err, "GetDatabaseMySQLConfig should succeed on 200 response")
	checkContains(t, got, "binlog_retention_period")
	checkContains(t, got, "mysql")
}

func TestClientGetDatabaseMySQLConfigAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, databaseMySQLConfigPath, r.URL.Path, "request path should be /databases/mysql/config")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetDatabaseMySQLConfig(t.Context())

	mustError(t, err)

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetDatabaseMySQLConfigRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		checkEqual(t, databaseMySQLConfigPath, r.URL.Path, "request path should be /databases/mysql/config")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			"mysql": map[string]any{
				"connect_timeout": map[string]any{keyType: databaseConfigTypeInteger},
			},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetDatabaseMySQLConfig(t.Context())

	mustNoError(t, err, "read-only GetDatabaseMySQLConfig should retry transient failures")
	checkContains(t, got, "mysql")
	checkEqual(t, int32(2), attempts.Load(), "transient read should be retried once")
}

func TestClientGetDatabasePostgreSQLConfigSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, databasePostgreSQLConfigPath, r.URL.Path, "request path should be /databases/postgresql/config")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			databasePostgreSQLConfigNamespace: map[string]any{
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

	mustNoError(t, err, "GetDatabasePostgreSQLConfig should succeed on 200 response")
	checkContains(t, got, "pg")
}

func TestClientGetDatabasePostgreSQLConfigAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, databasePostgreSQLConfigPath, r.URL.Path, "request path should be /databases/postgresql/config")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetDatabasePostgreSQLConfig(t.Context())

	mustError(t, err)

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetDatabasePostgreSQLConfigRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		checkEqual(t, databasePostgreSQLConfigPath, r.URL.Path, "request path should be /databases/postgresql/config")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			databasePostgreSQLConfigNamespace: map[string]any{
				databaseConfigMaxConnections: map[string]any{keyType: databaseConfigTypeInteger},
			},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetDatabasePostgreSQLConfig(t.Context())

	mustNoError(t, err, "read-only GetDatabasePostgreSQLConfig should retry transient failures")
	checkContains(t, got, "pg")
	checkEqual(t, int32(2), attempts.Load(), "transient read should be retried once")
}

func TestClientListDatabaseInstancesSuccess(t *testing.T) {
	t.Parallel()

	instances := []linode.DatabaseInstance{{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineMySQL, Version: databaseEngineVersion, Status: oauthClientStatus}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, databaseInstancesPath, r.URL.Path, "request path should be /databases/mysql/instances")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":     instances,
			keyPage:    2,
			keyPages:   3,
			keyResults: 51,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListDatabaseInstances(t.Context(), 2, 25)

	mustNoError(t, err, "ListDatabaseInstances should succeed on 200 response")
	mustLen(t, got, 1)
	checkEqual(t, databaseInstanceID, got[0].ID)
	checkEqual(t, databaseInstanceLabel, got[0].Label)
	checkEqual(t, databaseEngineMySQL, got[0].Engine)
}

func TestClientListDatabaseInstancesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, databaseInstancesPath, r.URL.Path, "request path should be /databases/mysql/instances")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.ListDatabaseInstances(t.Context(), 0, 0)

	mustError(t, err)

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListDatabaseInstancesRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		checkEqual(t, databaseInstancesPath, r.URL.Path, "request path should be /databases/mysql/instances")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []linode.DatabaseInstance{{ID: databaseInstanceID, Label: databaseInstanceLabel}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListDatabaseInstances(t.Context(), 0, 0)

	mustNoError(t, err, "read-only ListDatabaseInstances should retry transient failures")
	mustLen(t, got, 1)
	checkEqual(t, int32(2), attempts.Load(), "transient read should be retried once")
}

func TestClientListDatabasePostgreSQLInstancesSuccess(t *testing.T) {
	t.Parallel()

	instances := []linode.DatabaseInstance{{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEnginePostgreSQL, Version: databaseEngineVersion, Status: oauthClientStatus}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, databasePostgreSQLInstancesPath, r.URL.Path, "request path should be /databases/postgresql/instances")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    instances,
			keyPage:    2,
			keyPages:   3,
			keyResults: 51,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListDatabasePostgreSQLInstances(t.Context(), 2, 25)

	mustNoError(t, err, "ListDatabasePostgreSQLInstances should succeed on 200 response")
	mustLen(t, got, 1)
	checkEqual(t, databaseInstanceID, got[0].ID)
	checkEqual(t, databaseInstanceLabel, got[0].Label)
	checkEqual(t, databaseEnginePostgreSQL, got[0].Engine)
}

func TestClientListDatabasePostgreSQLInstancesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, databasePostgreSQLInstancesPath, r.URL.Path, "request path should be /databases/postgresql/instances")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.ListDatabasePostgreSQLInstances(t.Context(), 0, 0)

	mustError(t, err)

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListDatabasePostgreSQLInstancesRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		checkEqual(t, databasePostgreSQLInstancesPath, r.URL.Path, "request path should be /databases/postgresql/instances")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []linode.DatabaseInstance{{ID: databaseInstanceID, Label: databaseInstanceLabel}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListDatabasePostgreSQLInstances(t.Context(), 0, 0)

	mustNoError(t, err, "read-only ListDatabasePostgreSQLInstances should retry transient failures")
	mustLen(t, got, 1)
	checkEqual(t, int32(2), attempts.Load(), "transient read should be retried once")
}

func TestClientGetDatabaseInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, databaseInstancePath, r.URL.Path, "request path should include instance id")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineMySQL, Version: databaseEngineVersion, Status: oauthClientStatus}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetDatabaseInstance(t.Context(), databaseInstanceID)

	mustNoError(t, err, "GetDatabaseInstance should succeed on 200 response")
	mustNotNil(t, got)
	checkEqual(t, databaseInstanceID, got.ID)
	checkEqual(t, databaseInstanceLabel, got.Label)
}

func TestClientGetDatabasePostgreSQLInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, databasePostgreSQLInstancePath, r.URL.Path, "request path should include PostgreSQL instance id")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEnginePostgreSQL, Version: databaseEngineVersion, Status: oauthClientStatus}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetDatabasePostgreSQLInstance(t.Context(), databaseInstanceID)

	mustNoError(t, err, "GetDatabasePostgreSQLInstance should succeed on 200 response")
	mustNotNil(t, got)
	checkEqual(t, databaseInstanceID, got.ID)
	checkEqual(t, databaseInstanceLabel, got.Label)
	checkEqual(t, databaseEnginePostgreSQL, got.Engine)
}

func TestClientGetDatabasePostgreSQLInstanceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, databasePostgreSQLInstancePath, r.URL.Path, "request path should include PostgreSQL instance id")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetDatabasePostgreSQLInstance(t.Context(), databaseInstanceID)

	mustError(t, err)

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetDatabasePostgreSQLInstanceRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		checkEqual(t, databasePostgreSQLInstancePath, r.URL.Path, "request path should include PostgreSQL instance id")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Engine: databaseEnginePostgreSQL}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetDatabasePostgreSQLInstance(t.Context(), databaseInstanceID)

	mustNoError(t, err, "read-only GetDatabasePostgreSQLInstance should retry transient failures")
	mustNotNil(t, got)
	checkEqual(t, int32(2), attempts.Load(), "transient read should be retried once")
}

func TestClientGetDatabaseInstanceSSLSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, databaseInstanceSSLPath, r.URL.Path, "request path should include instance ssl path")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseSSL{CACertificate: databaseSSLCACertificate}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetDatabaseInstanceSSL(t.Context(), databaseInstanceID)

	mustNoError(t, err, "GetDatabaseInstanceSSL should succeed on 200 response")
	mustNotNil(t, got)
	checkEqual(t, databaseSSLCACertificate, got.CACertificate)
}

func TestClientGetDatabaseInstanceSSLAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, databaseInstanceSSLPath, r.URL.Path, "request path should include instance ssl path")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetDatabaseInstanceSSL(t.Context(), databaseInstanceID)

	mustError(t, err)

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetDatabaseInstanceSSLRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		checkEqual(t, databaseInstanceSSLPath, r.URL.Path, "request path should include instance ssl path")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseSSL{CACertificate: databaseSSLCACertificate}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetDatabaseInstanceSSL(t.Context(), databaseInstanceID)

	mustNoError(t, err, "read-only GetDatabaseInstanceSSL should retry transient failures")
	mustNotNil(t, got)
	checkEqual(t, databaseSSLCACertificate, got.CACertificate)
	checkEqual(t, int32(2), attempts.Load(), "transient read should be retried once")
}

func TestClientGetDatabasePostgreSQLInstanceSSLSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, databasePostgreSQLInstanceSSLPath, r.URL.Path, "request path should include PostgreSQL instance ssl path")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseSSL{CACertificate: databaseSSLCACertificate}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetDatabasePostgreSQLInstanceSSL(t.Context(), databaseInstanceID)

	mustNoError(t, err, "GetDatabasePostgreSQLInstanceSSL should succeed on 200 response")
	mustNotNil(t, got)
	checkEqual(t, databaseSSLCACertificate, got.CACertificate)
}

func TestClientGetDatabasePostgreSQLInstanceSSLAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, databasePostgreSQLInstanceSSLPath, r.URL.Path, "request path should include PostgreSQL instance ssl path")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetDatabasePostgreSQLInstanceSSL(t.Context(), databaseInstanceID)

	mustError(t, err)

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetDatabasePostgreSQLInstanceSSLRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		checkEqual(t, databasePostgreSQLInstanceSSLPath, r.URL.Path, "request path should include PostgreSQL instance ssl path")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseSSL{CACertificate: databaseSSLCACertificate}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetDatabasePostgreSQLInstanceSSL(t.Context(), databaseInstanceID)

	mustNoError(t, err, "read-only GetDatabasePostgreSQLInstanceSSL should retry transient failures")
	mustNotNil(t, got)
	checkEqual(t, databaseSSLCACertificate, got.CACertificate)
	checkEqual(t, int32(2), attempts.Load(), "transient read should be retried once")
}

func TestClientGetDatabaseInstanceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, databaseInstancePath, r.URL.Path, "request path should include instance id")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetDatabaseInstance(t.Context(), databaseInstanceID)

	mustError(t, err)

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetDatabaseInstanceRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		checkEqual(t, databaseInstancePath, r.URL.Path, "request path should include instance id")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetDatabaseInstance(t.Context(), databaseInstanceID)

	mustNoError(t, err, "read-only GetDatabaseInstance should retry transient failures")
	mustNotNil(t, got)
	checkEqual(t, int32(2), attempts.Load(), "transient read should be retried once")
}

func TestClientGetDatabaseInstanceCredentialsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, databaseInstanceCredentialsPath, r.URL.Path, "request path should include instance credentials path")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseCredentials{Username: accountMaintenanceEntityType, Password: databaseCredentialsPassword}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetDatabaseInstanceCredentials(t.Context(), databaseInstanceID)

	mustNoError(t, err, "GetDatabaseInstanceCredentials should succeed on 200 response")
	mustNotNil(t, got)
	checkEqual(t, "linode", got.Username)
	checkEqual(t, "secret", got.Password)
}

func TestClientGetDatabaseInstanceCredentialsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, databaseInstanceCredentialsPath, r.URL.Path, "request path should include instance credentials path")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetDatabaseInstanceCredentials(t.Context(), databaseInstanceID)

	mustError(t, err)

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetDatabaseInstanceCredentialsRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, databaseInstanceCredentialsPath, r.URL.Path, "request path should include instance credentials path")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseCredentials{Username: accountMaintenanceEntityType, Password: databaseCredentialsPassword}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetDatabaseInstanceCredentials(t.Context(), databaseInstanceID)

	mustNoError(t, err, "read-only GetDatabaseInstanceCredentials should retry transient failures")
	mustNotNil(t, got)
	checkEqual(t, int32(2), attempts.Load(), "transient read should be retried once")
}

func TestClientResetDatabaseInstanceCredentialsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databaseInstanceCredentialsResetPath, r.URL.Path, "request path should include credentials reset path")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		checkEqual(t, http.NoBody, r.Body, "request body should be empty")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseCredentials{Username: accountMaintenanceEntityType, Password: databaseCredentialsPassword}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ResetDatabaseInstanceCredentials(t.Context(), databaseInstanceID)

	mustNoError(t, err, "ResetDatabaseInstanceCredentials should succeed on 200 response")
	mustNotNil(t, got)
	checkEqual(t, "linode", got.Username)
	checkEqual(t, "secret", got.Password)
}

func TestClientResetDatabaseInstanceCredentialsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databaseInstanceCredentialsResetPath, r.URL.Path, "request path should include credentials reset path")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.ResetDatabaseInstanceCredentials(t.Context(), databaseInstanceID)

	mustError(t, err)

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientResetDatabaseInstanceCredentialsDoesNotRetryTransientPost(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databaseInstanceCredentialsResetPath, r.URL.Path, "request path should include credentials reset path")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	_, err := client.ResetDatabaseInstanceCredentials(t.Context(), databaseInstanceID)

	mustError(t, err)
	checkEqual(t, int32(1), attempts.Load(), "credential reset POST must not be retried")
}

func TestClientResetDatabasePostgreSQLInstanceCredentialsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databasePostgreSQLCredentialsResetPath, r.URL.Path, "request path should include PostgreSQL credentials reset path")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		checkEqual(t, http.NoBody, r.Body, "request body should be empty")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.ResetDatabasePostgreSQLInstanceCredentials(t.Context(), databaseInstanceID)

	mustNoError(t, err, "ResetDatabasePostgreSQLInstanceCredentials should succeed on 200 response")
}

func TestClientResetDatabasePostgreSQLInstanceCredentialsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databasePostgreSQLCredentialsResetPath, r.URL.Path, "request path should include PostgreSQL credentials reset path")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.ResetDatabasePostgreSQLInstanceCredentials(t.Context(), databaseInstanceID)

	mustError(t, err)

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientResetDatabasePostgreSQLInstanceCredentialsDoesNotRetryTransientPost(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databasePostgreSQLCredentialsResetPath, r.URL.Path, "request path should include PostgreSQL credentials reset path")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	err := client.ResetDatabasePostgreSQLInstanceCredentials(t.Context(), databaseInstanceID)

	mustError(t, err)
	checkEqual(t, int32(1), attempts.Load(), "PostgreSQL credential reset POST must not be retried")
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
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databaseInstancesPath, r.URL.Path, "request path should be /databases/mysql/instances")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))

		var gotReq linode.CreateDatabaseInstanceRequest
		checkNoError(t, json.NewDecoder(r.Body).Decode(&gotReq))
		checkEqual(t, expectedReq, gotReq)

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineMySQL, Version: databaseEngineVersion, Status: oauthClientStatus}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateDatabaseInstance(t.Context(), &expectedReq)

	mustNoError(t, err, "CreateDatabaseInstance should succeed on 200 response")
	mustNotNil(t, got)
	checkEqual(t, databaseInstanceID, got.ID)
	checkEqual(t, databaseInstanceLabel, got.Label)
}

func TestClientCreateDatabaseInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databaseInstancesPath, r.URL.Path, "request path should be /databases/mysql/instances")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	_, err := client.CreateDatabaseInstance(t.Context(), &linode.CreateDatabaseInstanceRequest{Label: databaseInstanceLabel, Type: databaseInstanceType, Engine: databaseEngineID, Region: regionUSEast})

	mustError(t, err)
	checkEqual(t, int32(1), attempts.Load(), "non-idempotent create POST must not be retried")
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
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databasePostgreSQLInstancesPath, r.URL.Path, "request path should be /databases/postgresql/instances")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))

		var gotReq linode.CreateDatabaseInstanceRequest
		checkNoError(t, json.NewDecoder(r.Body).Decode(&gotReq))
		checkEqual(t, expectedReq, gotReq)

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEnginePostgreSQL, Version: databaseEngineVersion, Status: oauthClientStatus}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateDatabasePostgreSQLInstance(t.Context(), &expectedReq)

	mustNoError(t, err, "CreateDatabasePostgreSQLInstance should succeed on 200 response")
	mustNotNil(t, got)
	checkEqual(t, databaseInstanceID, got.ID)
	checkEqual(t, databaseInstanceLabel, got.Label)
}

func TestClientCreateDatabasePostgreSQLInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databasePostgreSQLInstancesPath, r.URL.Path, "request path should be /databases/postgresql/instances")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	_, err := client.CreateDatabasePostgreSQLInstance(t.Context(), &linode.CreateDatabaseInstanceRequest{Label: databaseInstanceLabel, Type: databaseInstanceType, Engine: databaseEnginePostgreSQLID, Region: regionUSEast})

	mustError(t, err)
	checkEqual(t, int32(1), attempts.Load(), "non-idempotent PostgreSQL create POST must not be retried")
}

func TestClientDeleteDatabasePostgreSQLInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, databasePostgreSQLInstancePath, r.URL.Path, "request path should include instance id")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		checkEqual(t, http.NoBody, r.Body, "delete request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteDatabasePostgreSQLInstance(t.Context(), databaseInstanceID)

	mustNoError(t, err, "DeleteDatabasePostgreSQLInstance should succeed on 200 response")
}

func TestClientDeleteDatabasePostgreSQLInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, databasePostgreSQLInstancePath, r.URL.Path, "request path should include instance id")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	err := client.DeleteDatabasePostgreSQLInstance(t.Context(), databaseInstanceID)

	mustError(t, err)
	checkEqual(t, int32(1), attempts.Load(), "non-idempotent PostgreSQL delete DELETE must not be retried")
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
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, databaseInstancePath, r.URL.Path, "request path should include instance id")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))

		var gotReq linode.UpdateDatabaseInstanceRequest
		checkNoError(t, json.NewDecoder(r.Body).Decode(&gotReq))
		checkEqual(t, expectedReq, gotReq)

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineMySQL, Version: databaseEngineVersion, Status: oauthClientStatus}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateDatabaseInstance(t.Context(), databaseInstanceID, &expectedReq)

	mustNoError(t, err, "UpdateDatabaseInstance should succeed on 200 response")
	mustNotNil(t, got)
	checkEqual(t, databaseInstanceID, got.ID)
	checkEqual(t, databaseInstanceLabel, got.Label)
}

func TestClientUpdateDatabaseInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	label := databaseInstanceLabel

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, databaseInstancePath, r.URL.Path, "request path should include instance id")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	_, err := client.UpdateDatabaseInstance(t.Context(), databaseInstanceID, &linode.UpdateDatabaseInstanceRequest{Label: &label})

	mustError(t, err)
	checkEqual(t, int32(1), attempts.Load(), "side-effecting update PUT must not be retried")
}

func TestClientUpdateDatabasePostgreSQLInstanceSuccess(t *testing.T) {
	t.Parallel()

	label := databaseInstanceLabel
	databaseType := databaseInstanceType
	version := databaseEngineVersion
	allowList := []string{databaseAllowListCIDR}
	expectedReq := linode.UpdateDatabaseInstanceRequest{
		AllowList:      &allowList,
		EngineConfig:   map[string]any{databasePostgreSQLConfigNamespace: map[string]any{"timezone": "UTC"}},
		Label:          &label,
		PrivateNetwork: map[string]any{"public_access": false, "vpc_id": float64(123)},
		Type:           &databaseType,
		Updates:        map[string]any{"frequency": "weekly", "hour_of_day": float64(1)},
		Version:        &version,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, databasePostgreSQLInstancePath, r.URL.Path, "request path should include instance id")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))

		var gotReq linode.UpdateDatabaseInstanceRequest
		checkNoError(t, json.NewDecoder(r.Body).Decode(&gotReq))
		checkEqual(t, expectedReq, gotReq)

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEnginePostgreSQL, Version: databaseEngineVersion, Status: oauthClientStatus}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateDatabasePostgreSQLInstance(t.Context(), databaseInstanceID, &expectedReq)

	mustNoError(t, err, "UpdateDatabasePostgreSQLInstance should succeed on 200 response")
	mustNotNil(t, got)
	checkEqual(t, databaseInstanceID, got.ID)
	checkEqual(t, databaseInstanceLabel, got.Label)
	checkEqual(t, databaseEnginePostgreSQL, got.Engine)
}

func TestClientUpdateDatabasePostgreSQLInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	label := databaseInstanceLabel

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, databasePostgreSQLInstancePath, r.URL.Path, "request path should include instance id")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	_, err := client.UpdateDatabasePostgreSQLInstance(t.Context(), databaseInstanceID, &linode.UpdateDatabaseInstanceRequest{Label: &label})

	mustError(t, err)
	checkEqual(t, int32(1), attempts.Load(), "side-effecting PostgreSQL update PUT must not be retried")
}

func TestClientDeleteDatabaseInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, databaseInstancePath, r.URL.Path, "request path should include instance id")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		checkEqual(t, http.NoBody, r.Body, "delete request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteDatabaseInstance(t.Context(), databaseInstanceID)

	mustNoError(t, err, "DeleteDatabaseInstance should succeed on 200 response")
}

func TestClientDeleteDatabaseInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, databaseInstancePath, r.URL.Path, "request path should include instance id")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	err := client.DeleteDatabaseInstance(t.Context(), databaseInstanceID)

	mustError(t, err)
	checkEqual(t, int32(1), attempts.Load(), "destructive database DELETE must not be retried")
}

func TestClientPatchDatabaseInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databaseInstancePatchPath, r.URL.Path, "request path should include instance id and patch suffix")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		checkEqual(t, http.NoBody, r.Body, "patch request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.PatchDatabaseInstance(t.Context(), databaseInstanceID)

	mustNoError(t, err, "PatchDatabaseInstance should succeed on 200 response")
}

func TestClientPatchDatabaseInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databaseInstancePatchPath, r.URL.Path, "request path should include instance id and patch suffix")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	err := client.PatchDatabaseInstance(t.Context(), databaseInstanceID)

	mustError(t, err)
	checkEqual(t, int32(1), attempts.Load(), "side-effecting patch POST must not be retried")
}

func TestClientSuspendDatabaseInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databaseInstanceSuspendPath, r.URL.Path, "request path should include instance id and suspend suffix")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		checkEqual(t, http.NoBody, r.Body, "suspend request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.SuspendDatabaseInstance(t.Context(), databaseInstanceID)

	mustNoError(t, err, "SuspendDatabaseInstance should succeed on 200 response")
}

func TestClientSuspendDatabaseInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databaseInstanceSuspendPath, r.URL.Path, "request path should include instance id and suspend suffix")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	err := client.SuspendDatabaseInstance(t.Context(), databaseInstanceID)

	mustError(t, err)
	checkEqual(t, int32(1), attempts.Load(), "side-effecting suspend POST must not be retried")
}

func TestClientSuspendDatabasePostgreSQLInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databasePostgreSQLInstanceSuspendPath, r.URL.Path, "request path should include PostgreSQL instance id and suspend suffix")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		checkEqual(t, http.NoBody, r.Body, "suspend request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.SuspendDatabasePostgreSQLInstance(t.Context(), databaseInstanceID)

	mustNoError(t, err, "SuspendDatabasePostgreSQLInstance should succeed on 200 response")
}

func TestClientSuspendDatabasePostgreSQLInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databasePostgreSQLInstanceSuspendPath, r.URL.Path, "request path should include PostgreSQL instance id and suspend suffix")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	err := client.SuspendDatabasePostgreSQLInstance(t.Context(), databaseInstanceID)

	mustError(t, err)
	checkEqual(t, int32(1), attempts.Load(), "side-effecting PostgreSQL suspend POST must not be retried")
}

func TestClientResumeDatabaseInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databaseInstanceResumePath, r.URL.Path, "request path should include instance id and resume suffix")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		checkEqual(t, http.NoBody, r.Body, "resume request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.ResumeDatabaseInstance(t.Context(), databaseInstanceID)

	mustNoError(t, err, "ResumeDatabaseInstance should succeed on 200 response")
}

func TestClientResumeDatabaseInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databaseInstanceResumePath, r.URL.Path, "request path should include instance id and resume suffix")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	err := client.ResumeDatabaseInstance(t.Context(), databaseInstanceID)

	mustError(t, err)
	checkEqual(t, int32(1), attempts.Load(), "side-effecting resume POST must not be retried")
}

func TestClientResumeDatabasePostgreSQLInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databasePostgreSQLInstanceResumePath, r.URL.Path, "request path should include PostgreSQL instance id and resume suffix")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		checkEqual(t, http.NoBody, r.Body, "resume request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.ResumeDatabasePostgreSQLInstance(t.Context(), databaseInstanceID)

	mustNoError(t, err, "ResumeDatabasePostgreSQLInstance should succeed on 200 response")
}

func TestClientResumeDatabasePostgreSQLInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, databasePostgreSQLInstanceResumePath, r.URL.Path, "request path should include PostgreSQL instance id and resume suffix")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	err := client.ResumeDatabasePostgreSQLInstance(t.Context(), databaseInstanceID)

	mustError(t, err)
	checkEqual(t, int32(1), attempts.Load(), "side-effecting PostgreSQL resume POST must not be retried")
}

func TestClientGetDatabaseEngineSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, databaseEngineEscapedPath, r.URL.EscapedPath(), "request path should escape engine id")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseEngine{ID: databaseEngineID, Engine: databaseEngineMySQL, Version: databaseEngineVersion}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetDatabaseEngine(t.Context(), databaseEngineID)

	mustNoError(t, err, "GetDatabaseEngine should succeed on 200 response")
	mustNotNil(t, got, "engine should not be nil")
	checkEqual(t, databaseEngineID, got.ID)
	checkEqual(t, databaseEngineMySQL, got.Engine)
	checkEqual(t, databaseEngineVersion, got.Version)
}

func TestClientGetDatabaseEngineAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, databaseEngineEscapedPath, r.URL.EscapedPath(), "request path should escape engine id")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetDatabaseEngine(t.Context(), databaseEngineID)

	mustError(t, err)

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetDatabaseEngineRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		checkEqual(t, databaseEngineEscapedPath, r.URL.EscapedPath(), "request path should escape engine id")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.DatabaseEngine{ID: databaseEngineID, Engine: databaseEngineMySQL, Version: databaseEngineVersion}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetDatabaseEngine(t.Context(), databaseEngineID)

	mustNoError(t, err, "read-only GetDatabaseEngine should retry transient failures")
	mustNotNil(t, got, "engine should not be nil")
	checkEqual(t, int32(2), attempts.Load(), "transient read should be retried once")
}
