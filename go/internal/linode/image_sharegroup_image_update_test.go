package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const updatedSharedImageLabelFixture = "Updated Shared Debian"

func TestClientUpdateImageShareGroupImageSuccess(t *testing.T) {
	t.Parallel()

	label := updatedSharedImageLabelFixture
	description := "Updated shared image description"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/images/sharegroups/123/images/shared%2F1", r.URL.EscapedPath(), "request path should include escaped shared image ID")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"))

		var body map[string]any
		checkNoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		checkEqual(t, label, body["label"], "label should be sent")
		checkEqual(t, description, body["description"], "description should be sent")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.Image{ID: "shared/1", Label: label, Description: description}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.UpdateImageShareGroupImage(t.Context(), 123, "shared/1", &linode.UpdateImageShareGroupImageRequest{
		Label:       &label,
		Description: &description,
	})

	requireNoError(t, err)
	requireNotNil(t, result)
	checkEqual(t, "shared/1", result.ID)
	checkEqual(t, label, result.Label)
}

func TestClientUpdateImageShareGroupImageEscapesImageID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/images/sharegroups/123/images/shared%2F%2E%2E%3Fquery%23frag", r.URL.EscapedPath(), "image ID should be one escaped path segment")
		checkEmpty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.Image{ID: "shared/..?query#frag"}))
	}))
	t.Cleanup(srv.Close)

	label := updatedSharedImageLabelFixture

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.UpdateImageShareGroupImage(t.Context(), 123, "shared/..?query#frag", &linode.UpdateImageShareGroupImageRequest{Label: &label})

	requireNoError(t, err)
	requireNotNil(t, result)
}

func TestClientUpdateImageShareGroupImageDoesNotRetryMutation(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)

		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "try later"}}}))
	}))
	t.Cleanup(srv.Close)

	label := updatedSharedImageLabelFixture

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	result, err := client.UpdateImageShareGroupImage(t.Context(), 123, "shared/1", &linode.UpdateImageShareGroupImageRequest{Label: &label})

	requireError(t, err)
	checkNil(t, result)
	checkEqual(t, int32(1), requestCount.Load(), "mutating PUT should not be replayed by retry wrapper")
}
