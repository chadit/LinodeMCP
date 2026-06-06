package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientGetLongviewClientSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, "/longview/clients/789", r.URL.Path, "request path should include Longview client ID")
		longviewCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		longviewCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID:          789,
			keyLabel:       longviewClientLabel,
			"api_key":      "secret-api-key",
			"install_code": "secret-install-code",
			"apps": map[string]bool{
				"apache":            true,
				databaseEngineMySQL: true,
				"nginx":             false,
			},
			keyCreated: longviewClientCreated,
			keyUpdated: longviewClientUpdated,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetLongviewClient(t.Context(), "789")

	longviewRequireNoError(t, err, "GetLongviewClient should succeed on 200 response")
	longviewRequireNotNil(t, result, "result should not be nil")
	longviewCheckEqual(t, 789, result.ID)
	longviewCheckEqual(t, longviewClientLabel, result.Label)
	longviewCheckTrue(t, result.Apps.Apache)
	longviewCheckTrue(t, result.Apps.MySQL)
	longviewCheckFalse(t, result.Apps.Nginx)
	longviewCheckEqual(t, longviewClientCreated, result.Created)
	longviewCheckEqual(t, longviewClientUpdated, result.Updated)
}

func TestClientGetLongviewClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, "/longview/clients/789", r.URL.Path, "request path should include Longview client ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetLongviewClient(t.Context(), "789")

	longviewRequireError(t, err, "GetLongviewClient should fail on 403 response")

	apiErr := longviewRequireAPIError(t, err, "error should wrap APIError")
	longviewCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetLongviewClientEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, "/longview/clients/123%2F..", r.URL.EscapedPath(), "client ID should be escaped")
		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(linode.LongviewClient{ID: 123, Label: "client123"}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetLongviewClient(t.Context(), "123/..")

	longviewRequireNoError(t, err, "client should escape path separators before sending request")
}

func TestClientGetLongviewClientRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, "/longview/clients/789", r.URL.Path, "request path should include Longview client ID")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(linode.LongviewClient{ID: 789, Label: longviewClientLabel}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	result, err := client.GetLongviewClient(t.Context(), "789")

	longviewRequireNoError(t, err, "GetLongviewClient should retry transient failures")
	longviewRequireNotNil(t, result, "result should not be nil")
	longviewCheckEqual(t, int32(2), calls.Load(), "transient error should be retried once")
	longviewCheckEqual(t, 789, result.ID)
}
