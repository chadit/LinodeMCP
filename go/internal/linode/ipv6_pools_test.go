package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const endpointNetworkingIPv6Pools = "/networking/ipv6/pools"

func TestClientListIPv6PoolsSuccess(t *testing.T) {
	t.Parallel()

	pools := linode.PaginatedResponse[linode.IPv6Pool]{
		Data: []linode.IPv6Pool{{
			Range:  "2001:0db8::",
			Region: regionUSEast,
			Prefix: 124,
		}},
		Page:    2,
		Pages:   3,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointNetworkingIPv6Pools, r.URL.Path, "request path should match")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(pools))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListIPv6Pools(t.Context(), 2, 25)

	require.NoError(t, err, "ListIPv6Pools should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, pools, *result, "response should decode IPv6 pools")
}

func TestClientListIPv6PoolsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, endpointNetworkingIPv6Pools, r.URL.Path, "request path should match")
		http.Error(w, `{"errors":[{"reason":"forbidden"}]}`, http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.ListIPv6Pools(t.Context(), 0, 0)

	require.Error(t, err, "ListIPv6Pools should fail on non-200 response")
}
