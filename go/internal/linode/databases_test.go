package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
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
	databaseAllInstancesPath                  = "/databases/instances"
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
	databasePrivateNetworkVPCID               = "vpc_id"
	databasePrivateNetworkSubnetID            = "subnet_id"
	databasePrivateNetworkPublicAccess        = "public_access"
)

func TestClientGetDatabaseMySQLConfigSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != databaseMySQLConfigPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseMySQLConfigPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
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
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetDatabaseMySQLConfig(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := got["binlog_retention_period"]; !ok {
		t.Errorf("got missing key %v", "binlog_retention_period")
	}

	if _, ok := got["mysql"]; !ok {
		t.Errorf("got missing key %v", "mysql")
	}
}

func TestClientGetDatabaseMySQLConfigAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databaseMySQLConfigPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseMySQLConfigPath)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetDatabaseMySQLConfig(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientGetDatabaseMySQLConfigRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.URL.Path != databaseMySQLConfigPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseMySQLConfigPath)
		}

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			"mysql": map[string]any{
				"connect_timeout": map[string]any{keyType: databaseConfigTypeInteger},
			},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.GetDatabaseMySQLConfig(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := got["mysql"]; !ok {
		t.Errorf("got missing key %v", "mysql")
	}

	if attempts.Load() != int32(2) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(2))
	}
}

func TestClientGetDatabasePostgreSQLConfigSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != databasePostgreSQLConfigPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLConfigPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			databasePostgreSQLConfigNamespace: map[string]any{
				databaseConfigMaxConnections: map[string]any{
					keyDescription: "Sets the maximum number of concurrent connections.",
					keyExample:     100,
					keyType:        databaseConfigTypeInteger,
				},
			},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetDatabasePostgreSQLConfig(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := got["pg"]; !ok {
		t.Errorf("got missing key %v", "pg")
	}
}

func TestClientGetDatabasePostgreSQLConfigAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databasePostgreSQLConfigPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLConfigPath)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetDatabasePostgreSQLConfig(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientGetDatabasePostgreSQLConfigRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.URL.Path != databasePostgreSQLConfigPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLConfigPath)
		}

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			databasePostgreSQLConfigNamespace: map[string]any{
				databaseConfigMaxConnections: map[string]any{keyType: databaseConfigTypeInteger},
			},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.GetDatabasePostgreSQLConfig(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := got["pg"]; !ok {
		t.Errorf("got missing key %v", "pg")
	}

	if attempts.Load() != int32(2) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(2))
	}
}

func TestClientGetDatabaseInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != databaseInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineMySQL, Version: databaseEngineVersion, Status: oauthClientStatus}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetDatabaseInstance(t.Context(), databaseInstanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != databaseInstanceID {
		t.Errorf("got.ID = %v, want %v", got.ID, databaseInstanceID)
	}

	if got.Label != databaseInstanceLabel {
		t.Errorf("got.Label = %v, want %v", got.Label, databaseInstanceLabel)
	}
}

func TestClientGetDatabasePostgreSQLInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != databasePostgreSQLInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstancePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEnginePostgreSQL, Version: databaseEngineVersion, Status: oauthClientStatus}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetDatabasePostgreSQLInstance(t.Context(), databaseInstanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != databaseInstanceID {
		t.Errorf("got.ID = %v, want %v", got.ID, databaseInstanceID)
	}

	if got.Label != databaseInstanceLabel {
		t.Errorf("got.Label = %v, want %v", got.Label, databaseInstanceLabel)
	}

	if got.Engine != databaseEnginePostgreSQL {
		t.Errorf("got.Engine = %v, want %v", got.Engine, databaseEnginePostgreSQL)
	}
}

func TestClientGetDatabasePostgreSQLInstanceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databasePostgreSQLInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstancePath)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetDatabasePostgreSQLInstance(t.Context(), databaseInstanceID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientGetDatabasePostgreSQLInstanceRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.URL.Path != databasePostgreSQLInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstancePath)
		}

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Engine: databaseEnginePostgreSQL}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.GetDatabasePostgreSQLInstance(t.Context(), databaseInstanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if attempts.Load() != int32(2) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(2))
	}
}

func TestClientGetDatabaseInstanceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databaseInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancePath)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetDatabaseInstance(t.Context(), databaseInstanceID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientGetDatabaseInstanceRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.URL.Path != databaseInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancePath)
		}

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.GetDatabaseInstance(t.Context(), databaseInstanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if attempts.Load() != int32(2) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(2))
	}
}

func TestClientGetDatabaseInstanceCredentialsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != databaseInstanceCredentialsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceCredentialsPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.DatabaseCredentials{Username: accountMaintenanceEntityType, Password: databaseCredentialsPassword}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetDatabaseInstanceCredentials(t.Context(), databaseInstanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Username != firewallSettingsKeyLinode {
		t.Errorf("got.Username = %v, want %v", got.Username, firewallSettingsKeyLinode)
	}

	if got.Password != databaseCredentialsPassword {
		t.Errorf("got.Password = %v, want %v", got.Password, databaseCredentialsPassword)
	}
}

func TestClientGetDatabaseInstanceCredentialsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != databaseInstanceCredentialsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceCredentialsPath)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetDatabaseInstanceCredentials(t.Context(), databaseInstanceID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientGetDatabaseInstanceCredentialsRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.URL.Path != databaseInstanceCredentialsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceCredentialsPath)
		}

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.DatabaseCredentials{Username: accountMaintenanceEntityType, Password: databaseCredentialsPassword}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.GetDatabaseInstanceCredentials(t.Context(), databaseInstanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if attempts.Load() != int32(2) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(2))
	}
}

func TestClientResetDatabaseInstanceCredentialsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databaseInstanceCredentialsResetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceCredentialsResetPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.DatabaseCredentials{Username: accountMaintenanceEntityType, Password: databaseCredentialsPassword}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ResetDatabaseInstanceCredentials(t.Context(), databaseInstanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Username != firewallSettingsKeyLinode {
		t.Errorf("got.Username = %v, want %v", got.Username, firewallSettingsKeyLinode)
	}

	if got.Password != databaseCredentialsPassword {
		t.Errorf("got.Password = %v, want %v", got.Password, databaseCredentialsPassword)
	}
}

func TestClientResetDatabaseInstanceCredentialsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databaseInstanceCredentialsResetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceCredentialsResetPath)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.ResetDatabaseInstanceCredentials(t.Context(), databaseInstanceID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientResetDatabaseInstanceCredentialsDoesNotRetryTransientPost(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databaseInstanceCredentialsResetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceCredentialsResetPath)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	_, err := client.ResetDatabaseInstanceCredentials(t.Context(), databaseInstanceID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if attempts.Load() != int32(1) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(1))
	}
}

func TestClientResetDatabasePostgreSQLInstanceCredentialsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databasePostgreSQLCredentialsResetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLCredentialsResetPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	err := client.ResetDatabasePostgreSQLInstanceCredentials(t.Context(), databaseInstanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientResetDatabasePostgreSQLInstanceCredentialsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databasePostgreSQLCredentialsResetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLCredentialsResetPath)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	err := client.ResetDatabasePostgreSQLInstanceCredentials(t.Context(), databaseInstanceID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientResetDatabasePostgreSQLInstanceCredentialsDoesNotRetryTransientPost(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databasePostgreSQLCredentialsResetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLCredentialsResetPath)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	err := client.ResetDatabasePostgreSQLInstanceCredentials(t.Context(), databaseInstanceID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if attempts.Load() != int32(1) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(1))
	}
}

func TestClientDeleteDatabasePostgreSQLInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != databasePostgreSQLInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstancePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteDatabasePostgreSQLInstance(t.Context(), databaseInstanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientDeleteDatabasePostgreSQLInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != databasePostgreSQLInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstancePath)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))

	err := client.DeleteDatabasePostgreSQLInstance(t.Context(), databaseInstanceID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if attempts.Load() != int32(1) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(1))
	}
}

func TestClientDeleteDatabaseInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != databaseInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteDatabaseInstance(t.Context(), databaseInstanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientDeleteDatabaseInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != databaseInstancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancePath)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))

	err := client.DeleteDatabaseInstance(t.Context(), databaseInstanceID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if attempts.Load() != int32(1) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(1))
	}
}

func TestClientPatchDatabaseInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databaseInstancePatchPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancePatchPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	err := client.PatchDatabaseInstance(t.Context(), databaseInstanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientPatchDatabaseInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databaseInstancePatchPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancePatchPath)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))

	err := client.PatchDatabaseInstance(t.Context(), databaseInstanceID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if attempts.Load() != int32(1) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(1))
	}
}

func TestClientSuspendDatabaseInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databaseInstanceSuspendPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceSuspendPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	err := client.SuspendDatabaseInstance(t.Context(), databaseInstanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientSuspendDatabaseInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databaseInstanceSuspendPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceSuspendPath)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))

	err := client.SuspendDatabaseInstance(t.Context(), databaseInstanceID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if attempts.Load() != int32(1) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(1))
	}
}

func TestClientSuspendDatabasePostgreSQLInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databasePostgreSQLInstanceSuspendPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstanceSuspendPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	err := client.SuspendDatabasePostgreSQLInstance(t.Context(), databaseInstanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientSuspendDatabasePostgreSQLInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databasePostgreSQLInstanceSuspendPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstanceSuspendPath)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))

	err := client.SuspendDatabasePostgreSQLInstance(t.Context(), databaseInstanceID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if attempts.Load() != int32(1) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(1))
	}
}

func TestClientResumeDatabaseInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databaseInstanceResumePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceResumePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	err := client.ResumeDatabaseInstance(t.Context(), databaseInstanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientResumeDatabaseInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databaseInstanceResumePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstanceResumePath)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))

	err := client.ResumeDatabaseInstance(t.Context(), databaseInstanceID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if attempts.Load() != int32(1) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(1))
	}
}

func TestClientResumeDatabasePostgreSQLInstanceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databasePostgreSQLInstanceResumePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstanceResumePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	err := client.ResumeDatabasePostgreSQLInstance(t.Context(), databaseInstanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientResumeDatabasePostgreSQLInstanceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databasePostgreSQLInstanceResumePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databasePostgreSQLInstanceResumePath)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))

	err := client.ResumeDatabasePostgreSQLInstance(t.Context(), databaseInstanceID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if attempts.Load() != int32(1) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(1))
	}
}
