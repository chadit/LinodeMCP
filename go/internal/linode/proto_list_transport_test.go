package linode_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// protoListTransportToken is the token the unreachable-host client carries. No
// request ever leaves, so the value only has to be non-empty.
const protoListTransportToken = "proto-list-transport-token"

// The four proto list fetchers each keep their own copy of the request-and-wrap
// preamble, one per envelope shape. A dropped wrap in any of them would surface
// to callers as a bare *url.Error with no operation name on it, so each shape
// gets its own transport-failure test.

// TestClientListObjectStorageBucketsProtoNetworkError covers the plain
// {data:[...]} page-envelope fetcher.
func TestClientListObjectStorageBucketsProtoNetworkError(t *testing.T) {
	t.Parallel()

	_, err := newUnreachableProtoListClient(t).ListObjectStorageBucketsProto(t.Context())

	assertProtoListNetworkError(t, err, "ListObjectStorageBuckets")
}

// TestClientListPlacementGroupsProtoNetworkError covers the paginated
// page-envelope fetcher, which builds its URL through withPaginationQuery
// before the request.
func TestClientListPlacementGroupsProtoNetworkError(t *testing.T) {
	t.Parallel()

	_, err := newUnreachableProtoListClient(t).ListPlacementGroupsProto(t.Context(), 1, 100)

	assertProtoListNetworkError(t, err, "ListPlacementGroups")
}

// TestClientListProfileSecurityQuestionsProtoNetworkError covers the fetcher for
// envelopes that name their own member instead of "data".
func TestClientListProfileSecurityQuestionsProtoNetworkError(t *testing.T) {
	t.Parallel()

	_, err := newUnreachableProtoListClient(t).ListProfileSecurityQuestionsProto(t.Context())

	assertProtoListNetworkError(t, err, "ListProfileSecurityQuestions")
}

// TestClientGetRegionAvailabilityProtoNetworkError covers the fetcher for
// endpoints whose body is a bare top-level JSON array.
func TestClientGetRegionAvailabilityProtoNetworkError(t *testing.T) {
	t.Parallel()

	_, err := newUnreachableProtoListClient(t).GetRegionAvailabilityProto(t.Context(), "us-east")

	assertProtoListNetworkError(t, err, "GetRegionAvailability")
}

// assertProtoListNetworkError checks that err is a *linode.NetworkError naming
// operation and still carrying the transport error underneath.
func assertProtoListNetworkError(t *testing.T, err error, operation string) {
	t.Helper()

	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("err = %T, want *linode.NetworkError", err)
	}

	if netErr.Operation != operation {
		t.Errorf("netErr.Operation = %v, want %v", netErr.Operation, operation)
	}

	if netErr.Unwrap() == nil {
		t.Error("netErr.Unwrap() = nil, want the transport error")
	}
}

// newUnreachableProtoListClient returns a client pointed at a server that has
// already shut down, so its next connection attempt fails at the transport
// layer. Retries are off so the failure surfaces on the first try.
func newUnreachableProtoListClient(t *testing.T) *linode.Client {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := srv.URL
	srv.Close()

	return linode.NewClient(baseURL, protoListTransportToken, nil, linode.WithMaxRetries(0))
}
