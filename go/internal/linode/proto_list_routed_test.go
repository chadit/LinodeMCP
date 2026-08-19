package linode_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/linoderoute"
)

// The routed list fetchers resolve their path from the tool's declared route
// instead of taking one their caller built. These tests pin the URL the resolver
// produces and the class a resolution failure comes back in.

// opListTaggedObjects is the operation name both tagged-object list methods
// stamp on whichever error class they report.
const opListTaggedObjects = "ListTaggedObjects"

// routedListToken is non-empty only because the client requires a token; these
// tests assert on URLs, not on auth.
const routedListToken = "routed-list-token"

// refusingServer answers no request and reports any it receives, so a test can
// prove a call failed before reaching the network.
func refusingServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var received atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		received.Add(1)

		t.Errorf("r.URL.Path = %v, want no request at all", r.URL.Path)
	}))

	t.Cleanup(srv.Close)

	return srv, &received
}

// assertRefusedBeforeSending checks a routed call the contract could not fill:
// argument class, operation name, nothing on the wire. Reporting it as a network
// failure would invite the retry loop to replay a request that was never built.
func assertRefusedBeforeSending(t *testing.T, err error, operation string, received *atomic.Int32) {
	t.Helper()

	if !errors.Is(err, linoderoute.ErrEmptyValue) {
		t.Fatalf("error = %v, want ErrEmptyValue", err)
	}

	argErr, ok := errors.AsType[*linode.ArgumentError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.ArgumentError", err)
	}

	if argErr.Operation != operation {
		t.Errorf("argErr.Operation = %v, want %v", argErr.Operation, operation)
	}

	if _, isNetwork := errors.AsType[*linode.NetworkError](err); isNetwork {
		t.Errorf("error %v is also *linode.NetworkError, want the argument class alone", err)
	}

	if received.Load() != int32(0) {
		t.Errorf("received.Load() = %v, want %v", received.Load(), int32(0))
	}
}

// Same proof for the request primitive that carries a query string: it resolves
// its route the same way but reaches the wire directly, not through a list fetcher.
func TestRoutedQueryRequestRefusesAnEmptyPathValue(t *testing.T) {
	t.Parallel()

	srv, received := refusingServer(t)
	client := linode.NewClient(srv.URL, routedListToken, nil, linode.WithMaxRetries(0))

	_, err := client.ListTaggedObjects(t.Context(), "", 2, 25)

	assertRefusedBeforeSending(t, err, opListTaggedObjects, received)
}
