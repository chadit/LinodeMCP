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

func TestClientListInstanceVolumesSuccess(t *testing.T) {
	t.Parallel()

	volumes := []linode.Volume{
		{ID: 321, Label: "data-volume", Size: 50, Region: regionUSEast},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/instances/123/volumes", r.URL.Path, "request path should match")
		assert.Equal(t, "2", r.URL.Query().Get("page"), "page query should match")
		assert.Equal(t, "50", r.URL.Query().Get("page_size"), "page_size query should match")
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: volumes, keyPage: 2, keyPages: 3, keyResults: 1,
		}), "encoding response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	got, err := client.ListInstanceVolumes(t.Context(), 123, 2, 50)

	require.NoError(t, err, "ListInstanceVolumes should succeed on 200 response")
	require.Len(t, got, 1)
	assert.Equal(t, "data-volume", got[0].Label)
	assert.Equal(t, regionUSEast, got[0].Region)
}

func TestClientListInstanceVolumesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances/123/volumes", r.URL.Path, "request path should match")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}), "encoding error response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListInstanceVolumes(t.Context(), 123, 0, 0)

	require.Error(t, err, "ListInstanceVolumes should fail on API error")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListInstanceVolumesRejectsInvalidLinodeID(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListInstanceVolumes(t.Context(), -1, 0, 0)

	require.Error(t, err, "ListInstanceVolumes should reject invalid linode IDs before request")
	assert.False(t, called.Load(), "invalid linode ID should not reach upstream server")
	assert.ErrorIs(t, err, linode.ErrLinodeIDPositive, "error should expose invalid linode ID sentinel")
}
