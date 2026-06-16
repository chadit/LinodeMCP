package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
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
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointNetworkingIPv6Ranges {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPv6Ranges)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(ranges); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListIPv6Ranges(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !reflect.DeepEqual(*result, ranges) {
		t.Errorf("*result = %v, want %v", *result, ranges)
	}
}

func TestClientListIPv6RangesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointNetworkingIPv6Ranges {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPv6Ranges)
		}

		http.Error(w, `{"errors":[{"reason":"forbidden"}]}`, http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListIPv6Ranges(t.Context(), 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
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
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != endpointNetworkingIPv6Ranges {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointNetworkingIPv6Ranges)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var body linode.CreateIPv6RangeRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if body.LinodeID == nil {
			t.Fatal("linode_id should be encoded when provided")
		}

		if *body.LinodeID != linodeID {
			t.Errorf("*body.LinodeID = %v, want %v", *body.LinodeID, linodeID)
		}

		if body.PrefixLength != 124 {
			t.Errorf("body.PrefixLength = %v, want %v", body.PrefixLength, 124)
		}

		if body.RouteTarget != ipv6RouteTarget {
			t.Errorf("body.RouteTarget = %v, want %v", body.RouteTarget, ipv6RouteTarget)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(createdRange); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	result, err := client.CreateIPv6Range(t.Context(), linode.CreateIPv6RangeRequest{
		LinodeID:     &linodeID,
		PrefixLength: 124,
		RouteTarget:  ipv6RouteTarget,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !reflect.DeepEqual(*result, createdRange) {
		t.Errorf("*result = %v, want %v", *result, createdRange)
	}
}

func TestClientCreateIPv6RangeValidation(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.CreateIPv6Range(t.Context(), linode.CreateIPv6RangeRequest{})
	if !errors.Is(err, linode.ErrIPv6RangePrefixRange) {
		t.Fatalf("error = %v, want %v", err, linode.ErrIPv6RangePrefixRange)
	}

	var invalidLinodeID int

	_, err = client.CreateIPv6Range(t.Context(), linode.CreateIPv6RangeRequest{PrefixLength: 124, LinodeID: &invalidLinodeID})
	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}

	_, err = client.CreateIPv6Range(t.Context(), linode.CreateIPv6RangeRequest{PrefixLength: 129})
	if !errors.Is(err, linode.ErrIPv6RangePrefixRange) {
		t.Fatalf("error = %v, want %v", err, linode.ErrIPv6RangePrefixRange)
	}

	_, err = client.CreateIPv6Range(t.Context(), linode.CreateIPv6RangeRequest{PrefixLength: 124, RouteTarget: "192.0.2.1"})
	if !errors.Is(err, linode.ErrIPv6RangeRouteTargetInvalid) {
		t.Fatalf("error = %v, want %v", err, linode.ErrIPv6RangeRouteTargetInvalid)
	}
}

func TestClientCreateIPv6RangeDoesNotRetryPost(t *testing.T) {
	t.Parallel()

	var calls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		http.Error(w, `{"errors":[{"reason":"temporary"}]}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	_, err := client.CreateIPv6Range(t.Context(), linode.CreateIPv6RangeRequest{PrefixLength: 124})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls != 1 {
		t.Errorf("calls = %v, want %v", calls, 1)
	}
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
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != endpointNetworkingIPv6Ranges+"/2001:0db8::%2F64" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), endpointNetworkingIPv6Ranges+"/2001:0db8::%2F64")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(rangeResult); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetIPv6Range(t.Context(), ipv6RangeCIDR)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !reflect.DeepEqual(*result, rangeResult) {
		t.Errorf("*result = %v, want %v", *result, rangeResult)
	}
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

			if !errors.Is(err, linode.ErrIPv6RangeInvalid) {
				t.Fatalf("error = %v, want %v", err, linode.ErrIPv6RangeInvalid)
			}
		})
	}
}

func TestClientDeleteIPv6RangeSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.EscapedPath() != endpointNetworkingIPv6Ranges+"/2001:0db8::%2F64" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), endpointNetworkingIPv6Ranges+"/2001:0db8::%2F64")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteIPv6Range(t.Context(), ipv6RangeCIDR)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

			if !errors.Is(err, linode.ErrIPv6RangeInvalid) {
				t.Fatalf("error = %v, want %v", err, linode.ErrIPv6RangeInvalid)
			}
		})
	}
}

func TestClientDeleteIPv6RangeAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.EscapedPath() != endpointNetworkingIPv6Ranges+"/2001:0db8::%2F64" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), endpointNetworkingIPv6Ranges+"/2001:0db8::%2F64")
		}

		http.Error(w, `{"errors":[{"reason":"forbidden"}]}`, http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteIPv6Range(t.Context(), ipv6RangeCIDR)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestClientDeleteIPv6RangeDoesNotRetryDelete(t *testing.T) {
	t.Parallel()

	var calls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		http.Error(w, `{"errors":[{"reason":"temporary"}]}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	err := client.DeleteIPv6Range(t.Context(), ipv6RangeCIDR)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls != 1 {
		t.Errorf("calls = %v, want %v", calls, 1)
	}
}

func TestClientGetIPv6RangeAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != endpointNetworkingIPv6Ranges+"/2001:0db8::%2F64" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), endpointNetworkingIPv6Ranges+"/2001:0db8::%2F64")
		}

		http.Error(w, `{"errors":[{"reason":"forbidden"}]}`, http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetIPv6Range(t.Context(), ipv6RangeCIDR)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}
