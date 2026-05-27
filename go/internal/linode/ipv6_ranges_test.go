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

const endpointNetworkingIPv6Ranges = "/networking/ipv6/ranges"

func TestClientListIPv6RangesSuccess(t *testing.T) {
	t.Parallel()

	ranges := linode.PaginatedResponse[linode.IPv6Range]{
		Data: []linode.IPv6Range{{
			Range:  ipv6RangeFixture,
			Region: regionUSEast,
			Prefix: 124,
		}},
		Page:    2,
		Pages:   3,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointNetworkingIPv6Ranges, r.URL.Path, "request path should match")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(ranges))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListIPv6Ranges(t.Context(), 2, 25)

	require.NoError(t, err, "ListIPv6Ranges should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, ranges, *result, "response should decode IPv6 ranges")
}

func TestClientListIPv6RangesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, endpointNetworkingIPv6Ranges, r.URL.Path, "request path should match")
		http.Error(w, `{"errors":[{"reason":"forbidden"}]}`, http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.ListIPv6Ranges(t.Context(), 0, 0)

	require.Error(t, err, "ListIPv6Ranges should fail on non-200 response")
}

func TestClientCreateIPv6RangeSuccess(t *testing.T) {
	t.Parallel()

	linodeID := 12345
	createdRange := linode.IPv6Range{
		Range:       ipv6RangeFixture,
		Region:      regionUSEast,
		Prefix:      124,
		RouteTarget: ipv6RouteTarget,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, endpointNetworkingIPv6Ranges, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

		var body linode.CreateIPv6RangeRequest
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}

		if assert.NotNil(t, body.LinodeID, "linode_id should be encoded when provided") {
			assert.Equal(t, linodeID, *body.LinodeID, "linode_id should match")
		}

		assert.Equal(t, 124, body.PrefixLength, "prefix_length should match")
		assert.Equal(t, ipv6RouteTarget, body.RouteTarget, "route_target should match")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(createdRange))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	result, err := client.CreateIPv6Range(t.Context(), linode.CreateIPv6RangeRequest{
		LinodeID:     &linodeID,
		PrefixLength: 124,
		RouteTarget:  ipv6RouteTarget,
	})

	require.NoError(t, err, "CreateIPv6Range should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, createdRange, *result, "response should decode created IPv6 range")
}

func TestClientCreateIPv6RangeValidation(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.CreateIPv6Range(t.Context(), linode.CreateIPv6RangeRequest{})
	require.ErrorIs(t, err, linode.ErrIPv6RangePrefixRange, "prefix_length is required")

	var invalidLinodeID int

	_, err = client.CreateIPv6Range(t.Context(), linode.CreateIPv6RangeRequest{PrefixLength: 124, LinodeID: &invalidLinodeID})
	require.ErrorIs(t, err, linode.ErrLinodeIDPositive, "linode_id must be positive when provided")

	_, err = client.CreateIPv6Range(t.Context(), linode.CreateIPv6RangeRequest{PrefixLength: 129})
	require.ErrorIs(t, err, linode.ErrIPv6RangePrefixRange, "prefix_length must be in IPv6 range")

	_, err = client.CreateIPv6Range(t.Context(), linode.CreateIPv6RangeRequest{PrefixLength: 124, RouteTarget: "192.0.2.1"})
	require.ErrorIs(t, err, linode.ErrIPv6RangeRouteTargetInvalid, "route_target must be IPv6 when provided")
}

func TestClientCreateIPv6RangeDoesNotRetryPost(t *testing.T) {
	t.Parallel()

	var calls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++

		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		http.Error(w, `{"errors":[{"reason":"temporary"}]}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	_, err := client.CreateIPv6Range(t.Context(), linode.CreateIPv6RangeRequest{PrefixLength: 124})

	require.Error(t, err, "server error should propagate")
	assert.Equal(t, 1, calls, "non-idempotent POST must not be replayed")
}
