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

func TestClientListImagesByShareGroupSuccess(t *testing.T) {
	t.Parallel()

	images := []linode.Image{
		{ID: sharedImage1Fixture, Label: "shared-ubuntu", Type: "manual", Status: imageStatusAvailableFixture, Created: "2025-01-01T00:00:00", Size: 2500},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/images/sharegroups/123/images", r.URL.Path, "request path should include share group ID and images suffix")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    images,
			keyPage:    2,
			keyPages:   3,
			keyResults: 51,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListImagesByShareGroup(t.Context(), 123, 2, 25)

	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	assert.Equal(t, sharedImage1Fixture, result.Data[0].ID)
	assert.Equal(t, "shared-ubuntu", result.Data[0].Label)
	assert.Equal(t, 2, result.Page)
	assert.Equal(t, 3, result.Pages)
	assert.Equal(t, 51, result.Results)
}

func TestClientListImagesByShareGroupError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListImagesByShareGroup(t.Context(), 123, 0, 0)

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestClientListImagesByShareGroupRetriesReadOnlyRoute(t *testing.T) {
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
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []linode.Image{{ID: sharedImage1Fixture}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.ListImagesByShareGroup(t.Context(), 123, 0, 0)

	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	assert.Equal(t, int32(2), calls, "read-only GET route may retry transient failures")
}
