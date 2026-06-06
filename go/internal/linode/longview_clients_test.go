package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	longviewClientAPIKey       = "longview-api-key-secret"
	longviewClientInstallCode  = "longview-install-code-secret"
	longviewClientLabel        = "client789"
	longviewClientUpdatedLabel = "renamed-client"
	longviewClientsPath        = "/longview/clients"
)

func TestClientListLongviewClientsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, longviewClientsPath, r.URL.Path, "request path should match")
		longviewCheckEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		longviewCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				"api_key":      longviewClientAPIKey,
				"apps":         map[string]bool{"apache": true, databaseEngineMySQL: true, "nginx": false},
				keyCreated:     longviewClientCreated,
				keyID:          789,
				"install_code": longviewClientInstallCode,
				keyLabel:       longviewClientLabel,
				keyUpdated:     longviewClientUpdated,
			}},
			keyPage:    2,
			keyPages:   3,
			keyResults: 75,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListLongviewClients(t.Context(), 2, 25)

	longviewRequireNoError(t, err)
	longviewRequireNotNil(t, got)
	longviewRequireLenOne(t, got.Data)
	longviewCheckEqual(t, longviewClientLabel, got.Data[0].Label)
	longviewCheckTrue(t, got.Data[0].Apps.Apache)
	longviewCheckTrue(t, got.Data[0].Apps.MySQL)
	longviewCheckFalse(t, got.Data[0].Apps.Nginx)
}

func TestClientListLongviewClientsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, longviewClientsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListLongviewClients(t.Context(), 0, 0)

	longviewRequireError(t, err)
	longviewCheckNil(t, got)

	apiErr := longviewRequireAPIError(t, err)
	longviewCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	longviewCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientListLongviewClientsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		longviewCheckEqual(t, longviewClientsPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyID: 789, keyLabel: longviewClientLabel}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListLongviewClients(t.Context(), 0, 0)

	longviewRequireNoError(t, err)
	longviewRequireNotNil(t, got)
	longviewRequireLenOne(t, got.Data)
	longviewCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
}

func TestClientUpdateLongviewClientSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		longviewCheckEqual(t, longviewClientsPath+"/789", r.URL.EscapedPath(), "request path should match")
		longviewCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))

		var body map[string]any
		longviewCheckNoError(t, json.NewDecoder(r.Body).Decode(&body))
		longviewCheckEqual(t, map[string]any{keyLabel: longviewClientUpdatedLabel}, body, "request body should only include editable label")

		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: 789, keyLabel: longviewClientUpdatedLabel}))
	}))
	defer srv.Close()

	label := longviewClientUpdatedLabel
	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateLongviewClient(t.Context(), 789, &linode.UpdateLongviewClientRequest{Label: &label})

	longviewRequireNoError(t, err)
	longviewRequireNotNil(t, got)
	longviewCheckEqual(t, 789, got.ID)
	longviewCheckEqual(t, label, got.Label)
}

func TestClientUpdateLongviewClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		longviewCheckEqual(t, longviewClientsPath+"/789", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	label := longviewClientUpdatedLabel
	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateLongviewClient(t.Context(), 789, &linode.UpdateLongviewClientRequest{Label: &label})

	longviewRequireError(t, err)
	longviewCheckNil(t, got)

	apiErr := longviewRequireAPIError(t, err)
	longviewCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	longviewCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientUpdateLongviewClientDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		longviewCheckEqual(t, longviewClientsPath+"/789", r.URL.Path, "request path should match")
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	label := longviewClientUpdatedLabel
	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	_, err := client.UpdateLongviewClient(t.Context(), 789, &linode.UpdateLongviewClientRequest{Label: &label})

	longviewRequireError(t, err, "UpdateLongviewClient should fail on 503 response")
	longviewCheckEqual(t, int32(1), calls.Load(), "UpdateLongviewClient must not retry and replay a mutating request")
}

func TestClientDeleteLongviewClientSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		longviewCheckEqual(t, longviewClientsPath+"/789", r.URL.EscapedPath(), "request path should match")
		longviewCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		longviewCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteLongviewClient(t.Context(), 789)

	longviewRequireNoError(t, err)
}

func TestClientDeleteLongviewClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		longviewCheckEqual(t, longviewClientsPath+"/789", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteLongviewClient(t.Context(), 789)

	longviewRequireError(t, err)

	apiErr := longviewRequireAPIError(t, err)
	longviewCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	longviewCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientDeleteLongviewClientDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		longviewCheckEqual(t, longviewClientsPath+"/789", r.URL.Path, "request path should match")
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	err := client.DeleteLongviewClient(t.Context(), 789)

	longviewRequireError(t, err, "DeleteLongviewClient should fail on 503 response")
	longviewCheckEqual(t, int32(1), calls.Load(), "DeleteLongviewClient must not retry and replay a destructive request")
}
