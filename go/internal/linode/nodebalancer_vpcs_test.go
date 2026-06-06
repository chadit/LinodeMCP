package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const nodeBalancerVPCsPath = "/nodebalancers/123/vpcs"

func TestClientListNodeBalancerVPCsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, nodeBalancerVPCsPath, r.URL.Path, "request path should match")
		nbCheckEqual(t, "page=2&page_size=50", r.URL.RawQuery, "request query should include pagination")
		nbCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		nbCheckEqual(t, http.NoBody, r.Body, "request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyVPCID: 456, keySubnetID: 789, keyIPv4Range: cidrV4}},
			keyPage: 2, keyPages: 3, keyResults: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListNodeBalancerVPCs(t.Context(), 123, 2, 50)

	nbRequireNoError(t, err)
	nbRequireNotNil(t, got)
	nbRequireLenOne(t, got.Data)
	nbRequireNotNil(t, got.Data[0].VPCID)
	nbCheckEqual(t, 456, *got.Data[0].VPCID)
	nbCheckEqual(t, 789, got.Data[0].SubnetID)
	nbCheckEqual(t, cidrV4, got.Data[0].IPv4Range)
	nbCheckEqual(t, 2, got.Page)
	nbCheckEqual(t, 3, got.Pages)
}

func TestClientListNodeBalancerVPCsRejectsInvalidNodeBalancerID(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.example.test/v4", "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListNodeBalancerVPCs(t.Context(), 0, 1, 25)

	nbRequireErrorIs(t, err, linode.ErrNodeBalancerIDPositive)
	nbCheckNil(t, got)
}

func TestClientListNodeBalancerVPCsHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, nodeBalancerVPCsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListNodeBalancerVPCs(t.Context(), 123, 0, 0)

	nbRequireError(t, err)
	nbCheckNil(t, got)

	apiErr := nbRequireAPIError(t, err)
	nbCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	nbCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientListNodeBalancerVPCsRetriesReadOnlyRequest(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, nodeBalancerVPCsPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyVPCID: 456, keySubnetID: 789}},
			keyPage: 1, keyPages: 1, keyResults: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListNodeBalancerVPCs(t.Context(), 123, 0, 0)

	nbRequireNoError(t, err)
	nbRequireNotNil(t, got)
	nbRequireLenOne(t, got.Data)
	nbCheckEqual(t, int32(2), calls.Load())
}
