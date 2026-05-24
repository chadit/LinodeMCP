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

func TestClientGetImageSuccess(t *testing.T) {
	t.Parallel()

	image := linode.Image{ID: privateImage15Fixture, Label: imageLinuxDebianFixture, Type: typeManualImage, Status: imageStatusAvailableFixture, Created: shareGroupCreatedFixture, Size: 2500}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/images/private/15", r.URL.Path, "request path should include image ID")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(image))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetImage(t.Context(), privateImage15Fixture)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, privateImage15Fixture, result.ID)
	assert.Equal(t, imageLinuxDebianFixture, result.Label)
}

func TestClientGetImageEscapesImageID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/images/linode%2Fdebian11", r.URL.EscapedPath(), "image ID should be one encoded path segment")
		assert.Empty(t, r.URL.RawQuery, "encoded path value should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Image{ID: "linode/debian11"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetImage(t.Context(), "linode/debian11")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "linode/debian11", result.ID)
}

func TestClientGetImageError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetImage(t.Context(), privateImage15Fixture)

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestClientGetImageRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Image{ID: privateImage15Fixture}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.GetImage(t.Context(), privateImage15Fixture)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int32(2), calls, "read-only GET route may retry transient failures")
}
