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
	imageShareGroupID            = 54321
	updateImageShareGroupUpdated = "2025-04-16T22:44:02"
	updateImageShareGroupDesc    = "shared base images"
	updateImageShareGroupCreated = "2025-04-14T22:44:02"
)

func TestClientUpdateImageShareGroupSuccess(t *testing.T) {
	t.Parallel()

	label := shareGroupLabelFixture
	description := updateImageShareGroupDesc
	request := &linode.UpdateImageShareGroupRequest{Label: &label, Description: &description}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/images/sharegroups/54321", r.URL.Path, "request path should include share group ID")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))

		var body map[string]any
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}

		assert.Equal(t, label, body[keyLabel])
		assert.Equal(t, description, body[keyDescription])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroup{
			ID:           imageShareGroupID,
			UUID:         shareGroupUUIDFixture,
			Label:        label,
			Description:  &description,
			IsSuspended:  false,
			Created:      updateImageShareGroupCreated,
			Updated:      &[]string{updateImageShareGroupUpdated}[0],
			ImagesCount:  2,
			MembersCount: 3,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.UpdateImageShareGroup(t.Context(), imageShareGroupID, request)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, imageShareGroupID, result.ID)
	assert.Equal(t, label, result.Label)
}

func TestClientUpdateImageShareGroupAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "label is invalid"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateImageShareGroup(t.Context(), imageShareGroupID, &linode.UpdateImageShareGroupRequest{})

	require.Error(t, err, "UpdateImageShareGroup should return API errors")
	assert.ErrorContains(t, err, "label is invalid")
}

func TestClientUpdateImageShareGroupNetworkError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := srv.URL
	srv.Close()

	client := linode.NewClient(baseURL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateImageShareGroup(t.Context(), imageShareGroupID, &linode.UpdateImageShareGroupRequest{})

	require.Error(t, err, "UpdateImageShareGroup should wrap network errors")

	var networkErr *linode.NetworkError
	require.ErrorAs(t, err, &networkErr, "network error should wrap as NetworkError")
	assert.Equal(t, "UpdateImageShareGroup", networkErr.Operation)
}

func TestClientUpdateImageShareGroupDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	_, err := client.UpdateImageShareGroup(t.Context(), imageShareGroupID, &linode.UpdateImageShareGroupRequest{})

	require.Error(t, err, "UpdateImageShareGroup should fail on 500 response")
	assert.Equal(t, int32(1), calls.Load(), "UpdateImageShareGroup must not retry and replay a mutating request")
}
