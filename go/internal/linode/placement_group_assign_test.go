package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientAssignPlacementGroupLinodesRoute(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcPlacementGroups528Assign {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcPlacementGroups528Assign)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		var body map[string][]int

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		if decodeErr != nil {
			t.Errorf("unexpected error: %v", decodeErr)
		}

		if !reflect.DeepEqual(body["linodes"], []int{123, 456}) {
			t.Errorf("got %v, want %v", body["linodes"], []int{123, 456})
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:                   528,
			keyLabel:                "PG_Miami_failover",
			keyRegion:               regionUSMIA,
			keyPlacementGroupType:   placementGroupTypeLocal,
			keyPlacementGroupPolicy: placementGroupPolicy,
			keyIsCompliant:          true,
			keyMembers: []map[string]any{
				{keyLinodeID: 123, keyIsCompliant: true},
				{keyLinodeID: 456, keyIsCompliant: true},
			},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	group, err := client.AssignPlacementGroupLinodes(t.Context(), 528, &linode.AssignPlacementGroupLinodesRequest{Linodes: []int{123, 456}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if group == nil {
		t.Fatal("group is nil")
	}

	if group.ID != 528 {
		t.Errorf("group.ID = %v, want %v", group.ID, 528)
	}

	if len(group.Members) != 2 {
		t.Fatalf("length differs: got %d, want %d", len(group.Members), 2)
	}

	if group.Members[0].LinodeID != 123 {
		t.Errorf("group.Members[0].LinodeID = %v, want %v", group.Members[0].LinodeID, 123)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientAssignPlacementGroupLinodesDoesNotRetryTransientPost(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcPlacementGroups528Assign {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcPlacementGroups528Assign)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(
		srv.URL,
		"test-token",
		nil,
		linode.WithMaxRetries(2),
		linode.WithBaseDelay(time.Millisecond),
		linode.WithMaxDelay(time.Millisecond),
		linode.WithJitter(false),
	)

	group, err := client.AssignPlacementGroupLinodes(t.Context(), 528, &linode.AssignPlacementGroupLinodesRequest{Linodes: []int{123}})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if group != nil {
		t.Errorf("group = %v, want nil", group)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientAssignPlacementGroupLinodesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcPlacementGroups528Assign {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcPlacementGroups528Assign)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	group, err := client.AssignPlacementGroupLinodes(t.Context(), 528, &linode.AssignPlacementGroupLinodesRequest{Linodes: []int{123}})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if group != nil {
		t.Errorf("group = %v, want nil", group)
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}
