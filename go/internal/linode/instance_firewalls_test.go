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
	instanceFirewallLabelFixture  = "web-firewall"
	instanceFirewallStatusEnabled = "enabled"
)

func TestClientListInstanceFirewalls(t *testing.T) {
	t.Parallel()

	firewalls := []linode.Firewall{{ID: 456, Label: instanceFirewallLabelFixture, Status: instanceFirewallStatusEnabled}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointInstanceFirewallsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointInstanceFirewallsPath)
		}

		if r.URL.Query().Get("page") != "2" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page"), "2")
		}

		if r.URL.Query().Get("page_size") != "50" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page_size"), "50")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: firewalls, keyPage: 2, keyPages: 3, keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListInstanceFirewalls(t.Context(), 123, 2, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want %d", len(got), 1)
	}

	if got[0].Label != instanceFirewallLabelFixture {
		t.Errorf("got[0].Label = %v, want %v", got[0].Label, instanceFirewallLabelFixture)
	}
}

func TestClientListInstanceInterfaceFirewalls(t *testing.T) {
	t.Parallel()

	firewalls := []linode.Firewall{{ID: 789, Label: instanceFirewallLabelFixture, Status: instanceFirewallStatusEnabled}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != "/linode/instances/123/interfaces/456/firewalls" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/linode/instances/123/interfaces/456/firewalls")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.Firewall]{
			Data:    firewalls,
			Page:    1,
			Pages:   1,
			Results: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListInstanceInterfaceFirewalls(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want %d", len(got), 1)
	}

	if got[0].Label != instanceFirewallLabelFixture {
		t.Errorf("got[0].Label = %v, want %v", got[0].Label, instanceFirewallLabelFixture)
	}
}

func TestClientListInstanceInterfaceFirewallsRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.com/v4", "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListInstanceInterfaceFirewalls(t.Context(), 0, 456)
	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	got, err = client.ListInstanceInterfaceFirewalls(t.Context(), 123, 0)
	if !errors.Is(err, linode.ErrInterfaceIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrInterfaceIDPositive)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

func TestClientListInstanceInterfaceFirewallsHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/interfaces/456/firewalls" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/interfaces/456/firewalls")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListInstanceInterfaceFirewalls(t.Context(), 123, 456)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

func TestClientListInstanceFirewallsRejectsInvalidLinodeID(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.com/v4", "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListInstanceFirewalls(t.Context(), 0, 0, 0)

	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

func TestClientListInstanceFirewallsHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointInstanceFirewallsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointInstanceFirewallsPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListInstanceFirewalls(t.Context(), 123, 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

func TestClientUpdateInstanceFirewallsSuccess(t *testing.T) {
	t.Parallel()

	firewalls := []linode.Firewall{{ID: 456, Label: "assigned-instance-firewall", Status: instanceFirewallStatusEnabled}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != endpointInstanceFirewallsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointInstanceFirewallsPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		var body linode.UpdateInstanceFirewallsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body.FirewallIDs, []int{456, 789}) {
			t.Errorf("body.FirewallIDs = %v, want %v", body.FirewallIDs, []int{456, 789})
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.Firewall]{
			Data:    firewalls,
			Page:    2,
			Pages:   4,
			Results: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateInstanceFirewalls(t.Context(), 123, 2, 25, &linode.UpdateInstanceFirewallsRequest{FirewallIDs: []int{456, 789}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want %d", len(got), 1)
	}

	if got[0].ID != 456 {
		t.Errorf("got[0].ID = %v, want %v", got[0].ID, 456)
	}
}

func TestClientUpdateInstanceFirewallsAllowsEmptyAssignments(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body linode.UpdateInstanceFirewallsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(body.FirewallIDs) != 0 {
			t.Errorf("body.FirewallIDs = %v, want empty", body.FirewallIDs)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.Firewall]{Data: []linode.Firewall{}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateInstanceFirewalls(t.Context(), 123, 0, 0, &linode.UpdateInstanceFirewallsRequest{FirewallIDs: []int{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("got = %v, want empty", got)
	}
}

func TestClientUpdateInstanceFirewallsRejectsInvalidLinodeID(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateInstanceFirewalls(t.Context(), -1, 0, 0, &linode.UpdateInstanceFirewallsRequest{FirewallIDs: []int{456}})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if called.Load() {
		t.Error("called.Load() = true, want false")
	}

	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Errorf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}
}

func TestClientUpdateInstanceFirewallsRejectsNilRequest(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateInstanceFirewalls(t.Context(), 123, 0, 0, nil)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if called.Load() {
		t.Error("called.Load() = true, want false")
	}

	if !errors.Is(err, linode.ErrUpdateInstanceFirewallsRequestRequired) {
		t.Errorf("error = %v, want %v", err, linode.ErrUpdateInstanceFirewallsRequestRequired)
	}
}

func TestClientUpdateInstanceFirewallsDoesNotRetryTransientFailure(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(2))

	_, err := client.UpdateInstanceFirewalls(t.Context(), 123, 0, 0, &linode.UpdateInstanceFirewallsRequest{FirewallIDs: []int{456}})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if callCount.Load() != int32(1) {
		t.Errorf("callCount.Load() = %v, want %v", callCount.Load(), int32(1))
	}
}
