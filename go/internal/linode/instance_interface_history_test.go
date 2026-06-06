package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientListInstanceInterfaceHistorySuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/linode/instances/123/interfaces/history", r.URL.Path, "request path should match")
		checkEqual(t, "page=2&page_size=50", r.URL.RawQuery, "request query should match")
		checkEqual(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				"interface_history_id": 3,
				"interface_id":         221,
				"linode_id":            123,
				"version":              1,
				keyCreated:             "2025-08-01T00:01:01",
				"interface_data": map[string]any{
					keyID:         1234,
					"mac_address": macAddressFixture,
				},
			}},
			keyPage:    2,
			keyPages:   4,
			keyResults: 1,
		}), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	history, err := client.ListInstanceInterfaceHistory(t.Context(), 123, 2, 50)

	requireNoError(t, err, "ListInstanceInterfaceHistory should succeed on 200 response")
	requireNotNil(t, history)
	requireLenOne(t, history.Data)
	checkEqual(t, 3, history.Data[0].InterfaceHistoryID)
	checkEqual(t, 221, history.Data[0].InterfaceID)
	checkEqual(t, 123, history.Data[0].LinodeID)
	checkEqual(t, 1, history.Data[0].Version)
	checkEqual(t, 2, history.Page)
	checkEqual(t, 4, history.Pages)
	checkEqual(t, 1, history.Results)
}

func TestClientListInstanceInterfaceHistoryInvalidLinodeID(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.com/v4", "token", nil, linode.WithMaxRetries(0))
	history, err := client.ListInstanceInterfaceHistory(t.Context(), 0, 0, 0)

	requireError(t, err, "non-positive linode ID should fail before request")
	checkNil(t, history)
	requireErrorIs(t, err, linode.ErrLinodeIDPositive)
}

func TestClientListInstanceInterfaceHistoryAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}), "encoding error response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	history, err := client.ListInstanceInterfaceHistory(t.Context(), 123, 0, 0)

	requireError(t, err, "ListInstanceInterfaceHistory should surface API errors")
	checkNil(t, history)
}

func TestClientListInstanceInterfaceHistoryRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}), "encoding transient error response should not fail")

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{"interface_history_id": 3}},
		}), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1))
	history, err := client.ListInstanceInterfaceHistory(t.Context(), 123, 0, 0)

	requireNoError(t, err, "GET history list should retry transient failures")
	requireNotNil(t, history)
	checkEqual(t, int32(2), calls.Load())
}
