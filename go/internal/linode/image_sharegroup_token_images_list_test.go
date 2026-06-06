package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientListImagesByShareGroupTokenSuccess(t *testing.T) {
	t.Parallel()

	images := []linode.Image{
		{ID: "private/123", Label: "shared-ubuntu", Type: "manual", Status: imageStatusAvailableFixture, Created: "2025-01-01T00:00:00", Size: 2500},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/images/sharegroups/tokens/"+imageShareGroupTokenUUID+"/sharegroup/images", r.URL.Path, "request path should include token UUID and sharegroup images suffix")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    images,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListImagesByShareGroupToken(t.Context(), imageShareGroupTokenUUID, 2, 25)

	requireNoError(t, err)
	requireLenOne(t, result.Data)
	checkEqual(t, "private/123", result.Data[0].ID)
	checkEqual(t, "shared-ubuntu", result.Data[0].Label)
}

func TestClientListImagesByShareGroupTokenEscapesPathSeparators(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/images/sharegroups/tokens/token%2F..%3Fquery%23frag/sharegroup/images", r.URL.EscapedPath(), "token UUID should be one encoded path segment")
		checkEmpty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []linode.Image{}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListImagesByShareGroupToken(t.Context(), "token/..?query#frag", 0, 0)

	requireNoError(t, err)
	checkEmpty(t, result.Data)
}

func TestClientListImagesByShareGroupTokenEscapesStandaloneTraversalMarker(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/images/sharegroups/tokens/%2E%2E/sharegroup/images", r.URL.EscapedPath(), "standalone traversal marker should be encoded")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []linode.Image{}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListImagesByShareGroupToken(t.Context(), "..", 0, 0)

	requireNoError(t, err)
	checkEmpty(t, result.Data)
}

func TestClientListImagesByShareGroupTokenError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListImagesByShareGroupToken(t.Context(), imageShareGroupTokenUUID, 0, 0)

	requireError(t, err)
	checkNil(t, result)
}

func TestClientListImagesByShareGroupTokenRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []linode.Image{{ID: "private/123"}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.ListImagesByShareGroupToken(t.Context(), imageShareGroupTokenUUID, 0, 0)

	requireNoError(t, err)
	requireLenOne(t, result.Data)
	checkEqual(t, int32(2), calls, "read-only GET route may retry transient failures")
}
