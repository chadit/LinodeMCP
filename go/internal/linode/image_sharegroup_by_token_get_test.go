package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientGetImageShareGroupByTokenSuccess(t *testing.T) {
	t.Parallel()

	description := imageShareGroupDescription
	updated := imageShareGroupUpdated
	shareGroup := linode.ImageShareGroup{
		ID:          1,
		UUID:        imageShareGroupUUID,
		Label:       imageShareGroupLabel,
		Description: &description,
		IsSuspended: false,
		Created:     imageShareGroupCreated,
		Updated:     &updated,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/images/sharegroups/tokens/"+imageShareGroupTokenUUID+"/sharegroup", r.URL.Path, "request path should include token UUID and sharegroup suffix")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(shareGroup))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetImageShareGroupByToken(t.Context(), imageShareGroupTokenUUID)

	requireNoError(t, err)
	requireNotNil(t, result)
	checkEqual(t, imageShareGroupUUID, result.UUID)
	checkEqual(t, imageShareGroupLabel, result.Label)
}

func TestClientGetImageShareGroupByTokenEscapesPathSeparators(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/images/sharegroups/tokens/token%2F..%3Fquery%23frag/sharegroup", r.URL.EscapedPath(), "token UUID should be one encoded path segment")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroup{UUID: imageShareGroupUUID, Label: imageShareGroupLabel}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetImageShareGroupByToken(t.Context(), "token/..?query#frag")

	requireNoError(t, err)
	requireNotNil(t, result)
	checkEqual(t, imageShareGroupLabel, result.Label)
}

func TestClientGetImageShareGroupByTokenEscapesStandaloneTraversalMarker(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/images/sharegroups/tokens/%2E%2E/sharegroup", r.URL.EscapedPath(), "standalone traversal marker should be encoded")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroup{UUID: imageShareGroupUUID, Label: imageShareGroupLabel}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetImageShareGroupByToken(t.Context(), "..")

	requireNoError(t, err)
	requireNotNil(t, result)
	checkEqual(t, imageShareGroupUUID, result.UUID)
}

func TestClientGetImageShareGroupByTokenError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "not found"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetImageShareGroupByToken(t.Context(), imageShareGroupTokenUUID)

	requireError(t, err)
	checkNil(t, result)
}
