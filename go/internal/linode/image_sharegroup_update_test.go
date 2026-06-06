package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

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
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/images/sharegroups/54321", r.URL.Path, "request path should include share group ID")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"))

		var body map[string]any
		if !checkNoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}

		checkEqual(t, label, body[keyLabel])
		checkEqual(t, description, body[keyDescription])

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroup{
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

	requireNoError(t, err)
	requireNotNil(t, result)
	checkEqual(t, imageShareGroupID, result.ID)
	checkEqual(t, label, result.Label)
}

func TestClientUpdateImageShareGroupAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "label is invalid"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateImageShareGroup(t.Context(), imageShareGroupID, &linode.UpdateImageShareGroupRequest{})

	apiErr := requireAPIError(t, err, "UpdateImageShareGroup should return API errors")
	checkEqual(t, "label is invalid", apiErr.Message)
}

func TestClientUpdateImageShareGroupNetworkError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := srv.URL
	srv.Close()

	client := linode.NewClient(baseURL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateImageShareGroup(t.Context(), imageShareGroupID, &linode.UpdateImageShareGroupRequest{})

	requireError(t, err, "UpdateImageShareGroup should wrap network errors")

	networkErr := requireNetworkError(t, err, "network error should wrap as NetworkError")
	checkEqual(t, "UpdateImageShareGroup", networkErr.Operation)
}

func TestClientUpdateImageShareGroupDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	_, err := client.UpdateImageShareGroup(t.Context(), imageShareGroupID, &linode.UpdateImageShareGroupRequest{})

	requireError(t, err, "UpdateImageShareGroup should fail on 500 response")
	checkEqual(t, int32(1), calls.Load(), "UpdateImageShareGroup must not retry and replay a mutating request")
}
