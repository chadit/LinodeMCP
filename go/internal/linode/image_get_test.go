package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientGetImageSuccess(t *testing.T) {
	t.Parallel()

	image := linode.Image{ID: privateImage15Fixture, Label: imageLinuxDebianFixture, Type: typeManualImage, Status: imageStatusAvailableFixture, Created: shareGroupCreatedFixture, Size: 2500}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/images/private/15", r.URL.Path, "request path should include image ID")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(image))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetImage(t.Context(), privateImage15Fixture)

	requireNoError(t, err)
	requireNotNil(t, result)
	checkEqual(t, privateImage15Fixture, result.ID)
	checkEqual(t, imageLinuxDebianFixture, result.Label)
}

func TestClientGetImageEscapesImageID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/images/linode%2Fdebian11", r.URL.EscapedPath(), "image ID should be one encoded path segment")
		checkEmpty(t, r.URL.RawQuery, "encoded path value should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.Image{ID: "linode/debian11"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetImage(t.Context(), "linode/debian11")

	requireNoError(t, err)
	requireNotNil(t, result)
	checkEqual(t, "linode/debian11", result.ID)
}

func TestClientGetImageError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetImage(t.Context(), privateImage15Fixture)

	requireError(t, err)
	checkNil(t, result)
}

func TestClientGetImageRetriesReadOnlyRoute(t *testing.T) {
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
		checkNoError(t, json.NewEncoder(w).Encode(linode.Image{ID: privateImage15Fixture}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.GetImage(t.Context(), privateImage15Fixture)

	requireNoError(t, err)
	requireNotNil(t, result)
	checkEqual(t, int32(2), calls, "read-only GET route may retry transient failures")
}
