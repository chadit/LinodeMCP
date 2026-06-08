package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientUpdateNodeBalancerNodeSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes789)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body[keyLabel], nodeLabelWeb1) {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], nodeLabelWeb1)
		}

		if !reflect.DeepEqual(body[keyAddress], nodeBalancerNodeAddress) {
			t.Errorf("body[keyAddress] = %v, want %v", body[keyAddress], nodeBalancerNodeAddress)
		}

		if !reflect.DeepEqual(body[keyMode], nodeBalancerNodeModeAccept) {
			t.Errorf("body[keyMode] = %v, want %v", body[keyMode], nodeBalancerNodeModeAccept)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:             789,
			keyLabel:          nodeLabelWeb1,
			keyAddress:        nodeBalancerNodeAddress,
			keyStatus:         nodeBalancerNodeStatusUP,
			keyMode:           nodeBalancerNodeModeAccept,
			keyNodeBalancerID: 123,
			keyConfigID:       456,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateNodeBalancerNode(t.Context(), 123, 456, 789, &linode.UpdateNodeBalancerNodeRequest{
		Label:   nodeLabelWeb1,
		Address: nodeBalancerNodeAddress,
		Mode:    nodeBalancerNodeModeAccept,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != 789 {
		t.Errorf("got.ID = %v, want %v", got.ID, 789)
	}

	if got.Label != nodeLabelWeb1 {
		t.Errorf("got.Label = %v, want %v", got.Label, nodeLabelWeb1)
	}
}

func TestClientUpdateNodeBalancerNodeValidation(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateNodeBalancerNode(t.Context(), 0, 456, 789, &linode.UpdateNodeBalancerNodeRequest{Label: nodeLabelWeb1})
	if !errors.Is(err, linode.ErrNodeBalancerIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrNodeBalancerIDPositive)
	}

	_, err = client.UpdateNodeBalancerNode(t.Context(), 123, 0, 789, &linode.UpdateNodeBalancerNodeRequest{Label: nodeLabelWeb1})
	if !errors.Is(err, linode.ErrConfigIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrConfigIDPositive)
	}

	_, err = client.UpdateNodeBalancerNode(t.Context(), 123, 456, 0, &linode.UpdateNodeBalancerNodeRequest{Label: nodeLabelWeb1})
	if !errors.Is(err, linode.ErrNodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrNodeIDPositive)
	}

	_, err = client.UpdateNodeBalancerNode(t.Context(), 123, 456, 789, nil)
	if !errors.Is(err, linode.ErrUpdateNodeBalancerNodeRequestRequired) {
		t.Fatalf("error = %v, want %v", err, linode.ErrUpdateNodeBalancerNodeRequestRequired)
	}
}

func TestClientUpdateNodeBalancerNodeAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcNodebalancers123Configs456Nodes789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes789)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errNotFound}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateNodeBalancerNode(t.Context(), 123, 456, 789, &linode.UpdateNodeBalancerNodeRequest{Label: nodeLabelWeb1})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestClientUpdateNodeBalancerNodeDoesNotRetryTransientFailures(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcNodebalancers123Configs456Nodes789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes789)
		}

		callCount.Add(1)
		http.Error(w, errTemporaryFailure, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	_, err := client.UpdateNodeBalancerNode(t.Context(), 123, 456, 789, &linode.UpdateNodeBalancerNodeRequest{Label: nodeLabelWeb1})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if callCount.Load() != int32(1) {
		t.Errorf("callCount.Load() = %v, want %v", callCount.Load(), int32(1))
	}
}
