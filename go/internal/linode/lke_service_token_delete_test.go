package linode_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientDeleteLKEServiceTokenSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lke/clusters/123/servicetoken", r.URL.Path, "request path should match documented service token route")
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")

		body, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err, "reading request body should not fail") {
			return
		}

		assert.Empty(t, body, "DELETE service token request should not send a body")

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	err := client.DeleteLKEServiceToken(t.Context(), 123)

	require.NoError(t, err, "DeleteLKEServiceToken should succeed on 200 response")
}

func TestClientDeleteLKEServiceTokenDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, "/lke/clusters/123/servicetoken", r.URL.Path, "request path should match documented service token route")
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		http.Error(w, "temporary failure", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	err := client.DeleteLKEServiceToken(t.Context(), 123)

	require.Error(t, err, "DeleteLKEServiceToken should return the transient error")
	assert.Equal(t, int32(1), calls.Load(), "DELETE service token must not be retried")
}
