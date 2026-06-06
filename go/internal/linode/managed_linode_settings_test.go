package linode_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	managedLinodeSettingsID       = 234
	managedLinodeSettingsPath     = "/managed/linode-settings/234"
	managedLinodeSettingsListPath = "/managed/linode-settings"
	managedLinodeSettingsLabel    = "linode123"
	managedLinodeSettingsGroup    = "linodes"
	managedLinodeSettingsIP       = "203.0.113.1"
	managedLinodeSettingsSSHUser  = "linode"
	managedLinodeSettingsSSHPort  = 22
)

func TestClientListManagedLinodeSettingsSuccess(t *testing.T) {
	t.Parallel()

	port := 2222
	user := accountMaintenanceEntityType
	settings := linode.PaginatedResponse[linode.ManagedLinodeSettings]{
		Data: []linode.ManagedLinodeSettings{{
			ID:    123,
			Label: managedLinodeSettingsLabel,
			Group: managedLinodeSettingsGroup,
			SSH: linode.ManagedLinodeSettingsSSH{
				Access: true,
				IP:     managedLinodeSettingsIP,
				Port:   &port,
				User:   &user,
			},
		}},
		Page:    2,
		Pages:   3,
		Results: 7,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedLinodeSettingsListPath, r.URL.Path, "request path should match managed Linode settings endpoint")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(settings))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListManagedLinodeSettings(t.Context(), 2, 25)

	requireNoError(t, err, "ListManagedLinodeSettings should succeed")
	requireNotNil(t, result, "settings should be returned")
	requireLenOne(t, result.Data)
	checkEqual(t, 123, result.Data[0].ID)
	checkEqual(t, managedLinodeSettingsLabel, result.Data[0].Label)
	checkEqual(t, managedLinodeSettingsIP, result.Data[0].SSH.IP)
	requireNotNil(t, result.Data[0].SSH.Port)
	checkEqual(t, port, *result.Data[0].SSH.Port)
}

func TestClientListManagedLinodeSettingsRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedLinodeSettingsListPath, r.URL.Path, "request path should match managed Linode settings endpoint")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ManagedLinodeSettings]{
			Data:    []linode.ManagedLinodeSettings{{ID: 123, Label: managedLinodeSettingsLabel}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(1))

	result, err := client.ListManagedLinodeSettings(t.Context(), 1, 0)

	requireNoError(t, err, "read-only list should retry transient failures")
	requireNotNil(t, result, "settings should be returned after retry")
	checkEqual(t, int32(2), calls.Load(), "request should retry once")
}

func TestClientListManagedLinodeSettingsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedLinodeSettingsListPath, r.URL.Path, "request path should match managed Linode settings endpoint")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListManagedLinodeSettings(t.Context(), 1, 0)

	requireError(t, err, "ListManagedLinodeSettings should fail on API error")
	checkNil(t, result, "settings should not be returned")
	apiErr := requireAPIError(t, err)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetManagedLinodeSettingsSuccess(t *testing.T) {
	t.Parallel()

	sshUser := managedLinodeSettingsSSHUser
	sshPort := managedLinodeSettingsSSHPort
	settings := linode.ManagedLinodeSettings{
		ID:    managedLinodeSettingsID,
		Label: managedLinodeSettingsLabel,
		Group: managedLinodeSettingsGroup,
		SSH: linode.ManagedLinodeSettingsSSH{
			Access: true,
			IP:     managedLinodeSettingsIP,
			Port:   &sshPort,
			User:   &sshUser,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedLinodeSettingsPath, r.URL.EscapedPath(), "request path should include encoded Linode ID")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		body, err := io.ReadAll(r.Body)
		checkNoError(t, err, "reading request body should not fail")
		checkEmpty(t, body, "request body should be empty")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(settings))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetManagedLinodeSettings(t.Context(), managedLinodeSettingsID)

	requireNoError(t, err, "GetManagedLinodeSettings should succeed")
	requireNotNil(t, result, "settings should be returned")
	checkEqual(t, managedLinodeSettingsID, result.ID)
	checkEqual(t, managedLinodeSettingsLabel, result.Label)
	checkEqual(t, managedLinodeSettingsGroup, result.Group)
	checkTrue(t, result.SSH.Access)
	checkEqual(t, managedLinodeSettingsIP, result.SSH.IP)
	requireNotNil(t, result.SSH.Port)
	checkEqual(t, managedLinodeSettingsSSHPort, *result.SSH.Port)
	requireNotNil(t, result.SSH.User)
	checkEqual(t, managedLinodeSettingsSSHUser, *result.SSH.User)
}

func TestClientGetManagedLinodeSettingsRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	settings := linode.ManagedLinodeSettings{ID: managedLinodeSettingsID, Label: managedLinodeSettingsLabel}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedLinodeSettingsPath, r.URL.EscapedPath(), "request path should include encoded Linode ID")

		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: "temporary failure"}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(settings))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(
		srv.URL,
		"my-token",
		nil,
		linode.WithMaxRetries(1),
		linode.WithBaseDelay(time.Millisecond),
		linode.WithJitter(false),
	)
	result, err := client.GetManagedLinodeSettings(t.Context(), managedLinodeSettingsID)

	requireNoError(t, err, "GetManagedLinodeSettings should retry transient failures")
	requireNotNil(t, result, "settings should be returned after retry")
	checkEqual(t, int32(2), calls.Load(), "one retry should be attempted")
	checkEqual(t, managedLinodeSettingsID, result.ID)
}

func TestClientGetManagedLinodeSettingsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedLinodeSettingsPath, r.URL.EscapedPath(), "request path should include encoded Linode ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetManagedLinodeSettings(t.Context(), managedLinodeSettingsID)

	requireError(t, err, "API error should be returned")
	checkNil(t, result, "settings should be nil on API error")
	apiErr := requireAPIError(t, err)
	checkEqual(t, http.StatusNotFound, apiErr.StatusCode)
}

func TestClientUpdateManagedLinodeSettingsSuccess(t *testing.T) {
	t.Parallel()

	port := managedLinodeSettingsSSHPort
	user := managedLinodeSettingsSSHUser
	settings := linode.ManagedLinodeSettings{
		ID:    managedLinodeSettingsID,
		Label: managedLinodeSettingsLabel,
		Group: managedLinodeSettingsGroup,
		SSH: linode.ManagedLinodeSettingsSSH{
			Access: true,
			IP:     managedLinodeSettingsIP,
			Port:   &port,
			User:   &user,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, managedLinodeSettingsPath, r.URL.EscapedPath(), "request path should include encoded Linode ID")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got linode.UpdateManagedLinodeSettingsRequest
		checkNoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")

		if checkNotNil(t, got.SSH, "ssh update should be sent") {
			if checkNotNil(t, got.SSH.Access, "ssh access should be sent") {
				checkTrue(t, *got.SSH.Access)
			}

			if checkNotNil(t, got.SSH.IP, "ssh ip should be sent") {
				checkEqual(t, managedLinodeSettingsIP, *got.SSH.IP)
			}

			if checkNotNil(t, got.SSH.Port, "ssh port should be sent") {
				checkEqual(t, port, *got.SSH.Port)
			}

			if checkNotNil(t, got.SSH.User, "ssh user should be sent") {
				checkEqual(t, user, *got.SSH.User)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(settings))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.UpdateManagedLinodeSettings(t.Context(), managedLinodeSettingsID, linode.UpdateManagedLinodeSettingsRequest{
		SSH: &linode.UpdateManagedLinodeSettingsSSH{Access: &settings.SSH.Access, IP: &settings.SSH.IP, Port: settings.SSH.Port, User: settings.SSH.User},
	})

	requireNoError(t, err, "UpdateManagedLinodeSettings should succeed on 200 response")
	requireNotNil(t, result)
	checkEqual(t, managedLinodeSettingsID, result.ID)
	checkEqual(t, managedLinodeSettingsLabel, result.Label)
	checkEqual(t, managedLinodeSettingsIP, result.SSH.IP)
}

func TestClientUpdateManagedLinodeSettingsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, managedLinodeSettingsPath, r.URL.EscapedPath(), "request path should include encoded Linode ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	access := true
	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.UpdateManagedLinodeSettings(t.Context(), managedLinodeSettingsID, linode.UpdateManagedLinodeSettingsRequest{SSH: &linode.UpdateManagedLinodeSettingsSSH{Access: &access}})

	requireError(t, err, "UpdateManagedLinodeSettings should fail on API error")
	checkNil(t, result, "settings should not be returned")
	apiErr := requireAPIError(t, err)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientUpdateManagedLinodeSettingsDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, managedLinodeSettingsPath, r.URL.EscapedPath(), "request path should include encoded Linode ID")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))
	}))
	t.Cleanup(srv.Close)

	access := true
	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	_, err := client.UpdateManagedLinodeSettings(t.Context(), managedLinodeSettingsID, linode.UpdateManagedLinodeSettingsRequest{SSH: &linode.UpdateManagedLinodeSettingsSSH{Access: &access}})

	requireError(t, err, "mutating Managed Linode settings update should not retry transient failures")
	checkEqual(t, int32(1), calls.Load(), "client should call update exactly once")
}
