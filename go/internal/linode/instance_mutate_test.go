package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientMutateInstanceSuccess(t *testing.T) {
	t.Parallel()

	var allowAutoDiskResize bool

	request := &linode.MutateInstanceRequest{AllowAutoDiskResize: &allowAutoDiskResize}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/linode/instances/123/mutate", r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "mutate request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var body map[string]any

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, decodeErr)

		if decodeErr != nil {
			return
		}

		assert.Equal(t, false, body["allow_auto_disk_resize"])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.MutateInstance(t.Context(), 123, request)

	require.NoError(t, err, "MutateInstance should succeed on 200 response")
}

func TestClientMutateInstanceDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/linode/instances/123/mutate", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))

	err := client.MutateInstance(t.Context(), 123, &linode.MutateInstanceRequest{})

	require.Error(t, err, "MutateInstance should return the transient error")
	assert.Equal(t, int32(1), calls.Load(), "mutating upgrade request must not be retried")
}
