package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
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
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointNetworkingIPv6Pools {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPv6Pools)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(pools); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListIPv6Pools(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("ListIPv6Pools should succeed on 200 response: %v", err)
	}

	if result == nil {
		t.Fatal("result should not be nil")
	}

	if !reflect.DeepEqual(*result, pools) {
		t.Errorf("*result = %v, want %v", *result, pools)
	}
}

func TestClientListIPv6PoolsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointNetworkingIPv6Pools {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPv6Pools)
		}

		http.Error(w, `{"errors":[{"reason":"forbidden"}]}`, http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListIPv6Pools(t.Context(), 0, 0)
	if err == nil {
		t.Fatal("ListIPv6Pools should fail on non-200 response")
	}
}
