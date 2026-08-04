package linode_test

import (
	"encoding/json"
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

// An empty tag label would collapse "/tags/{tag_label}" to "/tags/", which lists
// every tag on the account instead of one tag's objects.
func TestRoutedProtoListRefusesAnEmptyPathValue(t *testing.T) {
	t.Parallel()

	srv, received := refusingServer(t)
	client := linode.NewClient(srv.URL, routedListToken, nil, linode.WithMaxRetries(0))

	_, err := client.ListTaggedObjectsProto(t.Context(), "", 2, 25)

	assertRefusedBeforeSending(t, err, opListTaggedObjects, received)
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

// Page controls have to arrive in the query of the resolved path, not appended to
// a path that already carries one, and the tag label has to be escaped exactly once.
func TestRoutedPaginatedProtoListSendsPathAndPageControls(t *testing.T) {
	t.Parallel()

	const (
		wantPath  = "/tags/prod%2Fweb"
		wantQuery = "page=2&page_size=25"
	)

	var (
		gotPath  string
		gotQuery string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotQuery = r.URL.RawQuery

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{"data": []any{}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, routedListToken, nil, linode.WithMaxRetries(0))

	if _, err := client.ListTaggedObjectsProto(t.Context(), "prod/web", 2, 25); err != nil {
		t.Fatalf("ListTaggedObjectsProto() error = %v, want no error", err)
	}

	if gotPath != wantPath {
		t.Errorf("path = %v, want %v", gotPath, wantPath)
	}

	if gotQuery != wantQuery {
		t.Errorf("query = %v, want %v", gotQuery, wantQuery)
	}
}

// The other half: a routed list with no filters and no page controls sends the
// bare declared path. It reads the request target rather than the parsed path and
// query, because a trailing "?" on an empty query parses back to an empty RawQuery
// and would pass unnoticed there while every unpaginated routed list sent a URL
// the unmigrated call sites do not.
func TestRoutedProtoListOmitsAnEmptyQuery(t *testing.T) {
	t.Parallel()

	const wantTarget = "/profile/security-questions"

	var gotTarget string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTarget = r.RequestURI

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{"security_questions": []any{}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, routedListToken, nil, linode.WithMaxRetries(0))

	if _, err := client.ListProfileSecurityQuestionsProto(t.Context()); err != nil {
		t.Fatalf("ListProfileSecurityQuestionsProto() error = %v, want no error", err)
	}

	if gotTarget != wantTarget {
		t.Errorf("request target = %v, want %v", gotTarget, wantTarget)
	}
}

// Swapped ids would still address a real resource, so slot order is worth pinning
// on the one migrated route with more than one slot.
func TestRoutedBareProtoListFillsSlotsInDeclaredOrder(t *testing.T) {
	t.Parallel()

	const wantPath = "/linode/instances/123/configs/456/interfaces"

	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode([]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, routedListToken, nil, linode.WithMaxRetries(0))

	if _, err := client.ListInstanceConfigInterfacesProto(t.Context(), 123, 456); err != nil {
		t.Fatalf("ListInstanceConfigInterfacesProto() error = %v, want no error", err)
	}

	if gotPath != wantPath {
		t.Errorf("path = %v, want %v", gotPath, wantPath)
	}
}
