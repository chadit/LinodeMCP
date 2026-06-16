package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	placementGroupTestLabel            = "pg-test"
	temporaryPlacementGroupCreateError = "temporary placement group create failure"
)

func TestClientCreatePlacementGroupSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.CreatePlacementGroupRequest{
		Label:                placementGroupTestLabel,
		Region:               managedServiceRegion,
		PlacementGroupType:   placementGroupTypeAntiAffinityTest,
		PlacementGroupPolicy: placementGroupPolicyStrictTest,
	}
	created := linode.PlacementGroup{ID: 123, Label: request.Label, Region: request.Region, PlacementGroupType: request.PlacementGroupType, PlacementGroupPolicy: request.PlacementGroupPolicy, IsCompliant: true}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcPlacementGroups {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcPlacementGroups)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		var got linode.CreatePlacementGroupRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(&got, request) {
			t.Errorf("got %v, want %v", &got, request)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(created); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreatePlacementGroup(t.Context(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != created.ID {
		t.Errorf("got.ID = %v, want %v", got.ID, created.ID)
	}

	if got.Label != created.Label {
		t.Errorf("got.Label = %v, want %v", got.Label, created.Label)
	}
}

func TestClientCreatePlacementGroupDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcPlacementGroups {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcPlacementGroups)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPlacementGroupCreateError}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.CreatePlacementGroup(t.Context(), &linode.CreatePlacementGroupRequest{Label: "pg-test", Region: managedServiceRegion, PlacementGroupType: placementGroupTypeAntiAffinityTest, PlacementGroupPolicy: placementGroupPolicyStrictTest})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}
