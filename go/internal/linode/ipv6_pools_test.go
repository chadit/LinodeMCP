package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const endpointNetworkingIPv6Pools = "/networking/ipv6/pools"

func TestClientListIPv6PoolsSuccess(t *testing.T) {
	t.Parallel()

	pools := linode.PaginatedResponse[linode.IPv6Pool]{
		Data: []linode.IPv6Pool{{
			Range:  ipv6RangeFixture,
			Region: regionUSEast,
			Prefix: 124,
		}},
		Page:    2,
		Pages:   3,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		stdCheckEqual(t, endpointNetworkingIPv6Pools, r.URL.Path, "request path should match")
		stdCheckEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(pools))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListIPv6Pools(t.Context(), 2, 25)

	stdMustNoError(t, err, "ListIPv6Pools should succeed on 200 response")
	stdMustNotNil(t, result, "result should not be nil")
	stdCheckEqual(t, pools, *result, "response should decode IPv6 pools")
}

func TestClientListIPv6PoolsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, endpointNetworkingIPv6Pools, r.URL.Path, "request path should match")
		http.Error(w, `{"errors":[{"reason":"forbidden"}]}`, http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.ListIPv6Pools(t.Context(), 0, 0)

	stdMustError(t, err, "ListIPv6Pools should fail on non-200 response")
}
