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

const (
	linodeTypeID                   = "g6-standard-2"
	linodeTypeEscapedPath          = "/linode/types/g6-standard-2"
	linodeTypeIDWithSeparators     = "g6/standard?plan=2"
	linodeTypeEscapedSeparatorPath = "/linode/types/g6%2Fstandard%3Fplan=2"
)

func TestClientGetTypeSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, linodeTypeEscapedPath, r.URL.EscapedPath(), "request path should include type id")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.InstanceType{ID: linodeTypeID, Label: "Linode 4GB"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetType(t.Context(), linodeTypeID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, linodeTypeID, got.ID)
	assert.Equal(t, "Linode 4GB", got.Label)
}

func TestClientGetTypeEscapesPathSeparators(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, linodeTypeEscapedSeparatorPath, r.URL.EscapedPath(), "request path should escape type id")
		assert.Empty(t, r.URL.RawQuery, "encoded query separator should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.InstanceType{ID: linodeTypeIDWithSeparators}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetType(t.Context(), linodeTypeIDWithSeparators)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, linodeTypeIDWithSeparators, got.ID)
}

func TestClientGetTypeAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, linodeTypeEscapedPath, r.URL.EscapedPath(), "request path should include type id")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetType(t.Context(), linodeTypeID)

	require.Error(t, err)
	assert.Nil(t, got)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetTypeRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		assert.Equal(t, linodeTypeEscapedPath, r.URL.EscapedPath(), "request path should include type id")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.InstanceType{ID: linodeTypeID}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetType(t.Context(), linodeTypeID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int32(2), attempts.Load(), "read-only GET route may retry transient failures")
}
