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
	databaseWriteProtoPGLabel    = "pg-db"
	databaseWriteProtoMySQLRenam = "renamed-db"
	databaseWriteProtoPGRenam    = "renamed-pg"
)

func TestClientCreateDatabaseInstanceProtoDecodes(t *testing.T) {
	t.Parallel()

	sslConnection := true
	wantRequest := &linode.CreateDatabaseInstanceRequest{
		Label:          databaseInstanceLabel,
		Type:           databaseInstanceType,
		Engine:         databaseEngineID,
		Region:         regionUSEast,
		AllowList:      []string{databaseAllowListCIDR},
		ClusterSize:    3,
		EngineConfig:   map[string]any{databaseEngineMySQL: map[string]any{"default_time_zone": "+03:00"}},
		Fork:           map[string]any{"source": "backup-123"},
		PrivateNetwork: map[string]any{databasePrivateNetworkVPCID: "vpc-123"},
		SSLConnection:  &sslConnection,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databaseInstancesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancesPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("Authorization = %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		var gotRequest linode.CreateDatabaseInstanceRequest
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Errorf("decode request body: %v", err)
		}

		if !reflect.DeepEqual(&gotRequest, wantRequest) {
			t.Errorf("request body = %#v, want %#v", &gotRequest, wantRequest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.DatabaseInstance{
			ID: databaseInstanceID, Label: databaseInstanceLabel, Status: "provisioning",
			Engine: databaseEngineMySQL, Version: databaseEngineVersion,
		}); err != nil {
			t.Errorf("encode response body: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateDatabaseInstanceProto(t.Context(), wantRequest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.GetId() != databaseInstanceID || got.GetLabel() != databaseInstanceLabel || got.GetEngine() != databaseEngineMySQL {
		t.Errorf("got = %+v, want id 123 label primary-db engine mysql", got)
	}
}

func TestClientCreateDatabaseInstanceProtoError(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != databaseInstancesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, databaseInstancesPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("encode response body: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	_, err := client.CreateDatabaseInstanceProto(t.Context(),
		&linode.CreateDatabaseInstanceRequest{Label: databaseInstanceLabel, Type: databaseInstanceType, Engine: databaseEngineID, Region: regionUSEast})

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want APIError", err)
	}

	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusInternalServerError)
	}

	if attempts.Load() != 1 {
		t.Errorf("attempts.Load() = %v, want 1", attempts.Load())
	}
}

func TestClientCreateDatabasePostgreSQLInstanceProtoDecodes(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, databasePostgreSQLInstancesPath, http.MethodPost,
		`{"id":456,"label":"pg-db","status":"provisioning","engine":"postgresql","version":"16"}`,
		http.StatusOK)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateDatabasePostgreSQLInstanceProto(t.Context(),
		&linode.CreateDatabaseInstanceRequest{Label: databaseWriteProtoPGLabel, Type: databaseInstanceType, Engine: databaseEnginePostgreSQLID, Region: regionUSEast})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != 456 || got.GetLabel() != databaseWriteProtoPGLabel || got.GetEngine() != databaseEnginePostgreSQL {
		t.Errorf("got = %+v, want id 456 label pg-db engine postgresql", got)
	}
}

func TestClientCreateDatabasePostgreSQLInstanceProtoError(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, databasePostgreSQLInstancesPath, http.MethodPost,
		`{"errors":[{"reason":"boom"}]}`, http.StatusBadRequest)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	if _, err := client.CreateDatabasePostgreSQLInstanceProto(t.Context(),
		&linode.CreateDatabaseInstanceRequest{Label: databaseWriteProtoPGLabel, Type: databaseInstanceType, Engine: databaseEnginePostgreSQLID, Region: regionUSEast}); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestClientUpdateDatabaseInstanceProtoDecodes(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, databaseInstancesPath+"/123", http.MethodPut,
		`{"id":123,"label":"renamed-db","status":"active"}`, http.StatusOK)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	label := databaseWriteProtoMySQLRenam

	got, err := client.UpdateDatabaseInstanceProto(t.Context(), databaseInstanceID,
		&linode.UpdateDatabaseInstanceRequest{Label: &label})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != databaseInstanceID || got.GetLabel() != databaseWriteProtoMySQLRenam {
		t.Errorf("got = %+v, want id 123 label renamed-db", got)
	}
}

func TestClientUpdateDatabaseInstanceProtoError(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, databaseInstancesPath+"/123", http.MethodPut,
		`{"errors":[{"reason":"boom"}]}`, http.StatusInternalServerError)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	label := databaseWriteProtoMySQLRenam

	if _, err := client.UpdateDatabaseInstanceProto(t.Context(), databaseInstanceID,
		&linode.UpdateDatabaseInstanceRequest{Label: &label}); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestClientUpdateDatabasePostgreSQLInstanceProtoDecodes(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, databasePostgreSQLInstancesPath+"/456", http.MethodPut,
		`{"id":456,"label":"renamed-pg","status":"active"}`, http.StatusOK)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	label := databaseWriteProtoPGRenam

	got, err := client.UpdateDatabasePostgreSQLInstanceProto(t.Context(), 456,
		&linode.UpdateDatabaseInstanceRequest{Label: &label})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != 456 || got.GetLabel() != databaseWriteProtoPGRenam {
		t.Errorf("got = %+v, want id 456 label renamed-pg", got)
	}
}

func TestClientUpdateDatabasePostgreSQLInstanceProtoError(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, databasePostgreSQLInstancesPath+"/456", http.MethodPut,
		`{"errors":[{"reason":"boom"}]}`, http.StatusBadRequest)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	label := databaseWriteProtoPGRenam

	if _, err := client.UpdateDatabasePostgreSQLInstanceProto(t.Context(), 456,
		&linode.UpdateDatabaseInstanceRequest{Label: &label}); err == nil {
		t.Fatal("expected an error, got nil")
	}
}
