package linode_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientDeleteInstanceInterfaceSendsRequest(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/linode/instances/123/interfaces/456", r.URL.Path, "request path should match")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	err := client.DeleteInstanceInterface(t.Context(), 123, 456)

	requireNoError(t, err, "DeleteInstanceInterface should succeed")
}

func TestClientDeleteInstanceInterfaceRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	err := client.DeleteInstanceInterface(t.Context(), 0, 456)
	requireErrorIs(t, err, linode.ErrLinodeIDPositive, "invalid linode ID should be rejected")

	err = client.DeleteInstanceInterface(t.Context(), 123, 0)
	requireErrorIs(t, err, linode.ErrInterfaceIDPositive, "zero interface ID should be rejected")

	err = client.DeleteInstanceInterface(t.Context(), 123, -1)
	requireErrorIs(t, err, linode.ErrInterfaceIDPositive, "negative interface ID should be rejected")

	if called.Load() {
		t.Error("invalid IDs should not issue HTTP request")
	}
}

func TestClientDeleteInstanceInterfaceDoesNotRetryDelete(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/linode/instances/123/interfaces/456", r.URL.Path, "request path should match")
		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(3))
	err := client.DeleteInstanceInterface(t.Context(), 123, 456)

	requireError(t, err, "server error should be returned")
	checkEqual(t, int32(1), calls.Load(), "DELETE interface call should not be replayed after transient server error")
}
