package linode_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	protoTestLabelBoot = "boot"
	protoTestRootPass  = "Str0ngP@ssw0rd!"
)

// jsonServer returns a test server that always answers with status and body.
func jsonServer(t *testing.T, wantPath, wantMethod, body string, status int) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, wantPath)
		}

		if r.Method != wantMethod {
			t.Errorf("r.Method = %v, want %v", r.Method, wantMethod)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write body: %v", err)
		}
	}))
}

func TestClientCreateInstanceConfigProtoDecodes(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, "/linode/instances/123/configs", http.MethodPost,
		`{"id":789,"label":"boot","kernel":"linode/latest-64bit"}`, http.StatusOK)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateInstanceConfigProto(t.Context(), 123, &linode.CreateConfigRequest{Label: protoTestLabelBoot})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != 789 || got.GetLabel() != protoTestLabelBoot || got.GetKernel() != configKernelLatest {
		t.Errorf("got = %+v, want id 789 label boot kernel linode/latest-64bit", got)
	}
}

func TestClientCreateInstanceConfigProtoError(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, "/linode/instances/123/configs", http.MethodPost,
		`{"errors":[{"reason":"boom"}]}`, http.StatusInternalServerError)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	if _, err := client.CreateInstanceConfigProto(t.Context(), 123, &linode.CreateConfigRequest{Label: "boot"}); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestClientUpdateInstanceConfigProtoDecodes(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, "/linode/instances/123/configs/789", http.MethodPut,
		`{"id":789,"label":"updated"}`, http.StatusOK)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	label := keyUpdated

	got, err := client.UpdateInstanceConfigProto(t.Context(), 123, 789, &linode.UpdateConfigRequest{Label: &label})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != 789 || got.GetLabel() != keyUpdated {
		t.Errorf("got = %+v, want id 789 label updated", got)
	}
}

func TestClientAddInstanceConfigInterfaceProtoDecodes(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, "/linode/instances/123/configs/789/interfaces", http.MethodPost,
		`{"id":202,"purpose":"vpc","primary":true}`, http.StatusOK)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.AddInstanceConfigInterfaceProto(t.Context(), 123, 789, &linode.ConfigInterface{Purpose: "vpc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != 202 || got.GetPurpose() != "vpc" || !got.GetPrimary() {
		t.Errorf("got = %+v, want id 202 purpose vpc primary true", got)
	}
}

func TestClientUpdateInstanceConfigInterfaceProtoError(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, "/linode/instances/123/configs/789/interfaces/202", http.MethodPut,
		`{"errors":[{"reason":"boom"}]}`, http.StatusBadRequest)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	if _, err := client.UpdateInstanceConfigInterfaceProto(t.Context(), 123, 789, 202, &linode.UpdateConfigInterfaceRequest{}); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestClientCreateInstanceDiskProtoDecodes(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, "/linode/instances/123/disks", http.MethodPost,
		`{"id":50,"label":"My Disk","size":1024,"filesystem":"ext4","status":"ready"}`, http.StatusOK)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateInstanceDiskProto(t.Context(), 123, &linode.CreateDiskRequest{Label: "My Disk", Size: 1024})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != 50 || got.GetLabel() != "My Disk" || got.GetSize() != 1024 {
		t.Errorf("got = %+v, want id 50 label 'My Disk' size 1024", got)
	}
}

func TestClientUpdateInstanceDiskProtoDecodes(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, "/linode/instances/123/disks/50", http.MethodPut,
		`{"id":50,"label":"renamed"}`, http.StatusOK)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateInstanceDiskProto(t.Context(), 123, 50, linode.UpdateDiskRequest{Label: "renamed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != 50 || got.GetLabel() != "renamed" {
		t.Errorf("got = %+v, want id 50 label renamed", got)
	}
}

func TestClientCloneInstanceDiskProtoDecodes(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, "/linode/instances/123/disks/50/clone", http.MethodPost,
		`{"id":99,"label":"My Disk (clone)"}`, http.StatusOK)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CloneInstanceDiskProto(t.Context(), 123, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != 99 {
		t.Errorf("got.GetId() = %v, want 99", got.GetId())
	}
}

func TestClientCreateInstanceBackupProtoDecodes(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, "/linode/instances/123/backups", http.MethodPost,
		`{"id":4001,"label":"snap","status":"pending","type":"snapshot"}`, http.StatusOK)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateInstanceBackupProto(t.Context(), 123, "snap")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != 4001 || got.GetType() != "snapshot" {
		t.Errorf("got = %+v, want id 4001 type snapshot", got)
	}
}

func TestClientRebuildInstanceProtoDecodes(t *testing.T) {
	t.Parallel()

	srv := jsonServer(t, "/linode/instances/123/rebuild", http.MethodPost,
		`{"id":123,"label":"my-linode","status":"rebuilding","region":"us-east"}`, http.StatusOK)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.RebuildInstanceProto(t.Context(), 123, &linode.RebuildInstanceRequest{Image: "linode/ubuntu24.04", RootPass: protoTestRootPass})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != 123 || got.GetStatus() != "rebuilding" || got.GetRegion() != managedServiceRegion {
		t.Errorf("got = %+v, want id 123 status rebuilding region us-east", got)
	}
}

func TestClientRebuildInstanceProtoRetriesTransient(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			// A 429 is always retryable (even for a POST), so the retry
			// wrapper must retry and the second attempt succeeds.
			w.WriteHeader(http.StatusTooManyRequests)

			if _, err := w.Write([]byte(`{"errors":[{"reason":"slow down"}]}`)); err != nil {
				t.Errorf("write body: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(`{"id":123,"status":"rebuilding"}`)); err != nil {
			t.Errorf("write body: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))

	got, err := client.RebuildInstanceProto(t.Context(), 123, &linode.RebuildInstanceRequest{Image: "linode/ubuntu24.04", RootPass: "Str0ngP@ssw0rd!"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != 123 {
		t.Errorf("got.GetId() = %v, want 123", got.GetId())
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want 2 (one transient failure then a retry)", calls.Load())
	}
}
