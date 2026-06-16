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

const temporaryPlacementGroupUnassignError = "temporary placement group unassign failure"

func TestClientUnassignPlacementGroupSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.PlacementGroupUnassignRequest{Linodes: []int{123, 456}}
	updated := linode.PlacementGroup{ID: 789, Label: placementGroupTestLabel, Region: managedServiceRegion, PlacementGroupType: placementGroupTypeAntiAffinityTest, PlacementGroupPolicy: placementGroupPolicyStrictTest, IsCompliant: true}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcPlacementGroups789Unassign {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcPlacementGroups789Unassign)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		var got linode.PlacementGroupUnassignRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(&got, request) {
			t.Errorf("got %v, want %v", &got, request)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(updated); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.UnassignPlacementGroup(t.Context(), 789, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != updated.ID {
		t.Errorf("got.ID = %v, want %v", got.ID, updated.ID)
	}

	if got.Label != updated.Label {
		t.Errorf("got.Label = %v, want %v", got.Label, updated.Label)
	}
}

func TestClientUnassignPlacementGroupRequiresLinodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  *linode.PlacementGroupUnassignRequest
	}{
		{name: "nil request", req: nil},
		{name: "nil linodes", req: &linode.PlacementGroupUnassignRequest{}},
		{name: "empty linodes", req: &linode.PlacementGroupUnassignRequest{Linodes: []int{}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(0))

			got, err := client.UnassignPlacementGroup(t.Context(), 789, tt.req)

			if !errors.Is(err, linode.ErrPlacementGroupUnassignLinodesRequired) {
				t.Fatalf("error = %v, want %v", err, linode.ErrPlacementGroupUnassignLinodesRequired)
			}

			if got != nil {
				t.Errorf("got = %v, want nil", got)
			}
		})
	}
}

func TestClientUnassignPlacementGroupAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcPlacementGroups789Unassign {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcPlacementGroups789Unassign)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.UnassignPlacementGroup(t.Context(), 789, &linode.PlacementGroupUnassignRequest{Linodes: []int{123}})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientUnassignPlacementGroupDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcPlacementGroups789Unassign {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcPlacementGroups789Unassign)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPlacementGroupUnassignError}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.UnassignPlacementGroup(t.Context(), 789, &linode.PlacementGroupUnassignRequest{Linodes: []int{123}})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}
