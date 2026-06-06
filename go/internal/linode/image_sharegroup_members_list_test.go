package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientListMembersByImageShareGroupSuccess(t *testing.T) {
	t.Parallel()

	updated := "2025-08-05T10:09:09"
	members := []linode.ImageShareGroupMember{
		{TokenUUID: imageShareGroupTokenUUID, Status: oauthClientStatus, Label: "Engineering - Backend", Created: "2025-08-04T10:07:59", Updated: &updated},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/images/sharegroups/123/members", r.URL.Path, "request path should include share group ID and members suffix")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    members,
			keyPage:    2,
			keyPages:   3,
			keyResults: 51,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListMembersByImageShareGroup(t.Context(), 123, 2, 25)

	requireNoError(t, err)
	requireLenOne(t, result.Data)
	checkEqual(t, imageShareGroupTokenUUID, result.Data[0].TokenUUID)
	checkEqual(t, oauthClientStatus, result.Data[0].Status)
	checkEqual(t, "Engineering - Backend", result.Data[0].Label)
	checkEqual(t, 2, result.Page)
	checkEqual(t, 3, result.Pages)
	checkEqual(t, 51, result.Results)
}

func TestClientListMembersByImageShareGroupError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListMembersByImageShareGroup(t.Context(), 123, 0, 0)

	requireError(t, err)
	checkNil(t, result)
}

func TestClientListMembersByImageShareGroupRetriesReadOnlyRoute(t *testing.T) {
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
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []linode.ImageShareGroupMember{{TokenUUID: imageShareGroupTokenUUID}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.ListMembersByImageShareGroup(t.Context(), 123, 0, 0)

	requireNoError(t, err)
	requireLenOne(t, result.Data)
	checkEqual(t, int32(2), calls, "read-only GET route may retry transient failures")
}
