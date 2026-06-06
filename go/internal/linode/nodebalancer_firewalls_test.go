package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const nodeBalancerFirewallsPath = "/nodebalancers/123/firewalls"

func TestClientListNodeBalancerFirewallsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, nodeBalancerFirewallsPath, r.URL.Path, "request path should match")
		nbCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		nbCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		nbCheckEqual(t, http.NoBody, r.Body, "request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: 456, keyLabel: "nb-firewall", keyStatus: statusEnabledFixture}},
			keyPage: 1, keyPages: 1, keyResults: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListNodeBalancerFirewalls(t.Context(), 123)

	nbRequireNoError(t, err)
	nbRequireLenOne(t, got)
	nbCheckEqual(t, 456, got[0].ID)
	nbCheckEqual(t, "nb-firewall", got[0].Label)
	nbCheckEqual(t, statusEnabledFixture, got[0].Status)
}

func TestClientListNodeBalancerFirewallsRejectsInvalidNodeBalancerID(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.example.test/v4", "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListNodeBalancerFirewalls(t.Context(), 0)

	nbRequireErrorIs(t, err, linode.ErrNodeBalancerIDPositive)
	nbCheckNil(t, got)
}

func TestClientListNodeBalancerFirewallsHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, nodeBalancerFirewallsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListNodeBalancerFirewalls(t.Context(), 123)

	nbRequireError(t, err)
	nbCheckNil(t, got)

	apiErr := nbRequireAPIError(t, err)
	nbCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	nbCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientUpdateNodeBalancerFirewallsSuccess(t *testing.T) {
	t.Parallel()

	firewalls := []linode.Firewall{{ID: 456, Label: "assigned-nodebalancer-firewall", Status: statusEnabledFixture}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		nbCheckEqual(t, nodeBalancerFirewallsPath, r.URL.Path, "request path should match")
		nbCheckEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		nbCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))

		var body linode.UpdateNodeBalancerFirewallsRequest
		nbCheckNoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		nbCheckEqual(t, []int{456, 789}, body.FirewallIDs, "request body should include firewall IDs")

		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: firewalls, keyPage: 2, keyPages: 3, keyResults: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateNodeBalancerFirewalls(t.Context(), 123, 2, 25, &linode.UpdateNodeBalancerFirewallsRequest{FirewallIDs: []int{456, 789}})

	nbRequireNoError(t, err)
	nbRequireLenOne(t, got)
	nbCheckEqual(t, 456, got[0].ID)
	nbCheckEqual(t, "assigned-nodebalancer-firewall", got[0].Label)
}

func TestClientUpdateNodeBalancerFirewallsAllowsEmptyAssignments(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body linode.UpdateNodeBalancerFirewallsRequest
		nbCheckNoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		nbCheckEmpty(t, body.FirewallIDs, "empty firewall_ids should be sent to remove assignments")

		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []linode.Firewall{}, keyPage: 1, keyPages: 1, keyResults: 0}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateNodeBalancerFirewalls(t.Context(), 123, 0, 0, &linode.UpdateNodeBalancerFirewallsRequest{FirewallIDs: []int{}})

	nbRequireNoError(t, err)
	nbCheckEmpty(t, got)
}

func TestClientUpdateNodeBalancerFirewallsRejectsInvalidNodeBalancerID(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateNodeBalancerFirewalls(t.Context(), 0, 0, 0, &linode.UpdateNodeBalancerFirewallsRequest{FirewallIDs: []int{456}})

	nbRequireErrorIs(t, err, linode.ErrNodeBalancerIDPositive)
	nbCheckNil(t, got)
	nbCheckEqual(t, false, called.Load(), "invalid NodeBalancer ID should not reach upstream server")
}

func TestClientUpdateNodeBalancerFirewallsRejectsNilRequest(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateNodeBalancerFirewalls(t.Context(), 123, 0, 0, nil)

	nbRequireErrorIs(t, err, linode.ErrUpdateNodeBalancerFirewallsRequestRequired)
	nbCheckNil(t, got)
	nbCheckEqual(t, false, called.Load(), "nil request should not reach upstream server")
}

func TestClientUpdateNodeBalancerFirewallsHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		nbCheckEqual(t, nodeBalancerFirewallsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateNodeBalancerFirewalls(t.Context(), 123, 0, 0, &linode.UpdateNodeBalancerFirewallsRequest{FirewallIDs: []int{456}})

	nbRequireError(t, err)
	nbCheckNil(t, got)

	apiErr := nbRequireAPIError(t, err)
	nbCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	nbCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientUpdateNodeBalancerFirewallsDoesNotRetryTransientFailure(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		nbCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")

		hj, ok := w.(http.Hijacker)
		if !nbCheckTrue(t, ok, "response writer should support hijacking") {
			return
		}

		conn, _, err := hj.Hijack()
		if !nbCheckNoError(t, err) {
			return
		}

		nbCheckNoError(t, conn.Close())
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	got, err := client.UpdateNodeBalancerFirewalls(t.Context(), 123, 0, 0, &linode.UpdateNodeBalancerFirewallsRequest{FirewallIDs: []int{456}})

	nbRequireError(t, err)
	nbCheckNil(t, got)
	nbCheckEqual(t, int32(1), callCount.Load(), "state-changing PUT must not be replayed after a transient error")
}
