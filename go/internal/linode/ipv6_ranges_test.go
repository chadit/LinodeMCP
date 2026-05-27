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

func TestClientGetIPv6RangeSuccess(t *testing.T) {
	t.Parallel()

	rangeResult := linode.IPv6Range{
		Range:       ipv6RangeCIDR,
		Region:      regionUSEast,
		Prefix:      64,
		RouteTarget: ipv6RouteTarget,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointNetworkingIPv6Ranges+"/2001:0db8::%2F64", r.URL.EscapedPath(), "request path should encode the IPv6 range slash")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(rangeResult))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetIPv6Range(t.Context(), ipv6RangeCIDR)

	require.NoError(t, err, "GetIPv6Range should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, rangeResult, *result, "response should decode IPv6 range")
}

func TestClientGetIPv6RangeRejectsMalformedRange(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatalf("client should reject malformed ranges before request")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	cases := []struct {
		name      string
		ipv6Range string
	}{
		{name: "missing prefix", ipv6Range: ipv6RangeFixture},
		{name: "slash only", ipv6Range: "/"},
		{name: "query separator", ipv6Range: "2001:0db8::/64?x=1"},
		{name: "traversal", ipv6Range: pathTraversalDotDot},
		{name: "ipv4 prefix", ipv6Range: "192.0.2.0/24"},
		{name: "host bits set", ipv6Range: "2001:0db8::1/64"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := client.GetIPv6Range(t.Context(), testCase.ipv6Range)

			require.ErrorIs(t, err, linode.ErrIPv6RangeInvalid, "malformed range should be rejected before request")
		})
	}
}

func TestClientDeleteIPv6RangeSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, endpointNetworkingIPv6Ranges+"/2001:0db8::%2F64", r.URL.EscapedPath(), "request path should encode the IPv6 range slash")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteIPv6Range(t.Context(), ipv6RangeCIDR)

	require.NoError(t, err, "DeleteIPv6Range should succeed on 200 response")
}

func TestClientDeleteIPv6RangeRejectsMalformedRange(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatalf("client should reject malformed ranges before request")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	for _, ipv6Range := range []string{ipv6RangeFixture, "/", "2001:0db8::/64?x=1", pathTraversalDotDot, "192.0.2.0/24", "2001:0db8::1/64"} {
		t.Run(ipv6Range, func(t *testing.T) {
			t.Parallel()

			err := client.DeleteIPv6Range(t.Context(), ipv6Range)

			require.ErrorIs(t, err, linode.ErrIPv6RangeInvalid, "malformed range should be rejected before request")
		})
	}
}

func TestClientDeleteIPv6RangeAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, endpointNetworkingIPv6Ranges+"/2001:0db8::%2F64", r.URL.EscapedPath(), "request path should match")
		http.Error(w, `{"errors":[{"reason":"forbidden"}]}`, http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteIPv6Range(t.Context(), ipv6RangeCIDR)

	require.Error(t, err, "DeleteIPv6Range should fail on non-200 response")
}

func TestClientDeleteIPv6RangeDoesNotRetryDelete(t *testing.T) {
	t.Parallel()

	var calls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++

		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		http.Error(w, `{"errors":[{"reason":"temporary"}]}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	err := client.DeleteIPv6Range(t.Context(), ipv6RangeCIDR)

	require.Error(t, err, "server error should propagate")
	assert.Equal(t, 1, calls, "destructive DELETE must not be replayed")
}

func TestClientGetIPv6RangeAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, endpointNetworkingIPv6Ranges+"/2001:0db8::%2F64", r.URL.EscapedPath(), "request path should match")
		http.Error(w, `{"errors":[{"reason":"forbidden"}]}`, http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetIPv6Range(t.Context(), ipv6RangeCIDR)

	require.Error(t, err, "GetIPv6Range should fail on non-200 response")
}
