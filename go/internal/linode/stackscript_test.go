package linode_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientDeleteStackScript(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/linode/stackscripts/456", r.URL.Path, "request path should match")
		checkEqual(t, http.MethodDelete, r.Method, "request method should match")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil)
	err := client.DeleteStackScript(t.Context(), 456)

	requireNoError(t, err, "delete should succeed")
}

func TestClientDeleteStackScriptDoesNotRetryDelete(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, "/linode/stackscripts/456", r.URL.Path, "request path should match")
		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))
	err := client.DeleteStackScript(t.Context(), 456)

	requireError(t, err, "server failure should be returned")
	checkEqual(t, int32(1), calls.Load(), "DELETE must not be retried")
}
