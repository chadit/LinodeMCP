package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const monitorServiceTokenPath = "/monitor/services/dbaas/token"

func monitorServiceTokenCreateRequest() *linode.CreateMonitorServiceTokenRequest {
	return &linode.CreateMonitorServiceTokenRequest{EntityIDs: []int{10, 20}}
}

func TestClientCreateMonitorServiceTokenSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		monitorCheckEqual(t, monitorServiceTokenPath, r.URL.Path, "request path should match")
		monitorCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		monitorCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))

		var body map[string]any
		if !monitorCheckNoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}

		monitorCheckEqual(t, []any{float64(10), float64(20)}, body["entity_ids"])

		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyToken: "monitor-token"}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateMonitorServiceToken(t.Context(), monitorServiceTypeDatabase, monitorServiceTokenCreateRequest())

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, "monitor-token", got.Token)
}

func TestClientCreateMonitorServiceTokenEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, "/monitor/services/dbaas%2Fpostgres/token", r.URL.EscapedPath(), "request path should be escaped")
		w.Header().Set("Content-Type", "application/json")
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyToken: "monitor-token"}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateMonitorServiceToken(t.Context(), monitorServiceTypeWithSlash, monitorServiceTokenCreateRequest())

	monitorRequireNoError(t, err)
	monitorRequireNotNil(t, got)
	monitorCheckEqual(t, "monitor-token", got.Token)
}

func TestClientCreateMonitorServiceTokenAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		monitorCheckEqual(t, monitorServiceTokenPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		monitorCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateMonitorServiceToken(t.Context(), monitorServiceTypeDatabase, monitorServiceTokenCreateRequest())

	monitorRequireError(t, err)
	monitorCheckNil(t, got)

	apiErr := monitorRequireAPIError(t, err)
	monitorCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	monitorCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientCreateMonitorServiceTokenDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		monitorCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		monitorCheckEqual(t, monitorServiceTokenPath, r.URL.Path, "request path should match")
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	got, err := client.CreateMonitorServiceToken(t.Context(), monitorServiceTypeDatabase, monitorServiceTokenCreateRequest())

	monitorRequireError(t, err)
	monitorCheckNil(t, got)
	monitorCheckEqual(t, int32(1), calls.Load(), "token creation must not retry after transient failure")
}
