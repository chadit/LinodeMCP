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

const regionUSMiami = "us-mia"

func TestClientReplicateImageSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/images/private%2F123/regions", r.URL.EscapedPath(), "request path should escape image ID")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var body map[string]any
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode") {
			return
		}

		assert.Equal(t, []any{regionUSMiami, regionUSEast}, body["regions"])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Image{ID: privateImage123Fixture, Label: "replicated-image"}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	image, err := client.ReplicateImage(t.Context(), privateImage123Fixture, &linode.ReplicateImageRequest{Regions: []string{regionUSMiami, regionUSEast}})

	require.NoError(t, err)
	require.NotNil(t, image)
	assert.Equal(t, privateImage123Fixture, image.ID)
}

func TestClientReplicateImageEscapesImageID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/images/private%2F123%3Fbad/regions", r.URL.EscapedPath(), "request path should escape image ID")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Image{ID: "private/123?bad"}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ReplicateImage(t.Context(), "private/123?bad", &linode.ReplicateImageRequest{Regions: []string{regionUSEast}})

	require.NoError(t, err, "ReplicateImage should escape path parameters")
}

func TestClientReplicateImageAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ReplicateImage(t.Context(), privateImage123Fixture, &linode.ReplicateImageRequest{Regions: []string{regionUSEast}})

	require.Error(t, err)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "expected APIError")
	assert.Equal(t, errTemporaryFailure, apiErr.Message)
}

func TestClientReplicateImageNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ReplicateImage(t.Context(), privateImage123Fixture, &linode.ReplicateImageRequest{Regions: []string{regionUSEast}})

	require.Error(t, err)

	var networkErr *linode.NetworkError
	require.ErrorAs(t, err, &networkErr, "expected NetworkError")
	assert.Equal(t, "ReplicateImage", networkErr.Operation)
}

func TestClientReplicateImageDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.ReplicateImage(t.Context(), privateImage123Fixture, &linode.ReplicateImageRequest{Regions: []string{regionUSEast}})

	require.Error(t, err)
	assert.Equal(t, int32(1), requestCount.Load(), "mutating replicate request must not be retried")
}
