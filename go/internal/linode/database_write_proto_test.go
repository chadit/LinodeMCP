package linode_test

import (
	"net/http"
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

	srv := jsonServer(t, databaseInstancesPath, http.MethodPost,
		`{"id":123,"label":"primary-db","status":"provisioning","engine":"mysql","version":"8.0.26"}`,
		http.StatusOK)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateDatabaseInstanceProto(t.Context(),
		&linode.CreateDatabaseInstanceRequest{Label: databaseInstanceLabel, Type: databaseInstanceType, Engine: databaseEngineID, Region: regionUSEast})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != databaseInstanceID || got.GetLabel() != databaseInstanceLabel || got.GetEngine() != databaseEngineMySQL {
		t.Errorf("got = %+v, want id 123 label primary-db engine mysql", got)
	}
}

func TestClientCreateDatabaseInstanceProtoError(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, databaseInstancesPath, http.MethodPost,
		`{"errors":[{"reason":"boom"}]}`, http.StatusInternalServerError)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	if _, err := client.CreateDatabaseInstanceProto(t.Context(),
		&linode.CreateDatabaseInstanceRequest{Label: databaseInstanceLabel, Type: databaseInstanceType, Engine: databaseEngineID, Region: regionUSEast}); err == nil {
		t.Fatal("expected an error, got nil")
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
