package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientListImageShareGroupsByImageSuccess(t *testing.T) {
	t.Parallel()

	description := shareGroupDescriptionFixture
	updated := shareGroupUpdatedFixture
	shareGroups := []linode.ImageShareGroup{
		{
			ID:           1,
			UUID:         imageShareGroupUUID,
			Label:        imageShareGroupLabel,
			Description:  &description,
			IsSuspended:  false,
			Created:      shareGroupCreatedFixture,
			Updated:      &updated,
			Expiry:       nil,
			ImagesCount:  2,
			MembersCount: 3,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/images/private%2F12345/sharegroups", r.URL.EscapedPath(), "request path should include escaped image ID and sharegroups suffix")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    shareGroups,
			keyPage:    2,
			keyPages:   3,
			keyResults: 51,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListImageShareGroupsByImage(t.Context(), "private/12345", 2, 25)

	requireNoError(t, err)
	requireLenOne(t, result.Data)
	checkEqual(t, imageShareGroupLabel, result.Data[0].Label)
	checkEqual(t, 2, result.Page)
	checkEqual(t, 3, result.Pages)
	checkEqual(t, 51, result.Results)
}

func TestClientListImageShareGroupsByImageEscapesImageID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/images/private%2F12345%3Fquery%23frag/sharegroups", r.URL.EscapedPath(), "image ID should be one encoded path segment")
		checkEmpty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []linode.ImageShareGroup{}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.ListImageShareGroupsByImage(t.Context(), "private/12345?query#frag", 0, 0)

	requireNoError(t, err)
}

func TestClientListImageShareGroupsByImageEncodesTraversalMarkers(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/images/%2E%2E/sharegroups", r.URL.EscapedPath(), "standalone traversal marker should be encoded")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []linode.ImageShareGroup{}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.ListImageShareGroupsByImage(t.Context(), pathTraversalDotDot, 0, 0)

	requireNoError(t, err)
}

func TestClientListImageShareGroupsByImageError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListImageShareGroupsByImage(t.Context(), "private/12345", 0, 0)

	requireError(t, err)
	checkNil(t, result)
}

func TestClientListImageShareGroupsByImageRetriesReadOnlyRoute(t *testing.T) {
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
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []linode.ImageShareGroup{{ID: 1}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.ListImageShareGroupsByImage(t.Context(), "private/12345", 0, 0)

	requireNoError(t, err)
	requireLenOne(t, result.Data)
	checkEqual(t, int32(2), calls, "read-only GET route may retry transient failures")
}
