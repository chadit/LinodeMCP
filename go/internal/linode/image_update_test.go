package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const updatedImageLabelFixture = "Updated Debian 12"

func TestClientUpdateImageSuccess(t *testing.T) {
	t.Parallel()

	label := updatedImageLabelFixture
	description := "Updated image description"
	tags := []string{uploadImageTagProd, uploadImageTagWeb}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/images/private%2F12345", r.URL.EscapedPath(), "request path should include escaped image ID")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"))

		var body map[string]any
		checkNoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		checkEqual(t, label, body["label"], "label should be sent")
		checkEqual(t, description, body["description"], "description should be sent")
		checkEqual(t, []any{uploadImageTagProd, uploadImageTagWeb}, body["tags"], "tags should be sent")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.Image{ID: "private/12345", Label: label, Description: description, Tags: tags}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.UpdateImage(t.Context(), "private/12345", &linode.UpdateImageRequest{
		Label:       &label,
		Description: &description,
		Tags:        &tags,
	})

	requireNoError(t, err)
	requireNotNil(t, result)
	checkEqual(t, "private/12345", result.ID)
	checkEqual(t, label, result.Label)
	checkEqual(t, tags, result.Tags)
}

func TestClientUpdateImageEscapesImageID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/images/private%2F%2E%2E%3Fquery%23frag", r.URL.EscapedPath(), "image ID should be one escaped path segment")
		checkEmpty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.Image{ID: "private/..?query#frag"}))
	}))
	t.Cleanup(srv.Close)

	label := updatedImageLabelFixture

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.UpdateImage(t.Context(), "private/..?query#frag", &linode.UpdateImageRequest{Label: &label})

	requireNoError(t, err)
	requireNotNil(t, result)
}

func TestClientUpdateImageRejectsNilRequest(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestCount.Add(1)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.UpdateImage(t.Context(), "private/12345", nil)

	requireError(t, err)
	checkNil(t, result)
	requireErrorIs(t, err, linode.ErrUpdateImageRequestRequired, "error should identify missing update image request")
	checkEqual(t, int32(0), requestCount.Load(), "nil request should be rejected before HTTP call")
}

func TestClientUpdateImageDoesNotRetryMutation(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)

		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "try later"}}}))
	}))
	t.Cleanup(srv.Close)

	label := updatedImageLabelFixture

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	result, err := client.UpdateImage(t.Context(), "private/12345", &linode.UpdateImageRequest{Label: &label})

	requireError(t, err)
	checkNil(t, result)
	checkEqual(t, int32(1), requestCount.Load(), "mutating PUT should not be replayed by retry wrapper")
}
