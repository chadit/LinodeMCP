package linode_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	managedCredentialsPath          = "/managed/credentials"
	managedCredentialsSSHKeyPath    = "/managed/credentials/sshkey"
	managedCredentialsLabel         = "prod-password-1"
	managedCredentialsLastDecrypted = "2018-01-01T00:01:01"
	managedSSHKeyValue              = "ssh-rsa managedservices-test-key"
	managedCredentialsPassword      = "stored-password-value"
	managedCredentialsUsername      = "johndoe"
)

func TestClientListManagedCredentialsSuccess(t *testing.T) {
	t.Parallel()

	credentials := linode.PaginatedResponse[linode.ManagedCredential]{
		Data: []linode.ManagedCredential{{
			ID:            9991,
			Label:         managedCredentialsLabel,
			LastDecrypted: managedCredentialsLastDecrypted,
		}},
		Page:    2,
		Pages:   3,
		Results: 7,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedCredentialsPath, r.URL.Path, "request path should list managed credentials")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(credentials))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListManagedCredentials(t.Context(), 2, 25)

	require.NoError(t, err, "ListManagedCredentials should succeed on 200 response")
	require.NotNil(t, got)
	assert.Equal(t, 2, got.Page)
	assert.Equal(t, managedCredentialsLabel, got.Data[0].Label)
	assert.Equal(t, managedCredentialsLastDecrypted, got.Data[0].LastDecrypted)
}

func TestClientGetManagedSSHKeySuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedCredentialsSSHKeyPath, r.URL.Path, "request path should retrieve Managed SSH key")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		body, readErr := io.ReadAll(r.Body)
		assert.NoError(t, readErr)
		assert.Empty(t, body, "GET request should not send a body")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedSSHKey{SSHKey: managedSSHKeyValue}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetManagedSSHKey(t.Context())

	require.NoError(t, err, "GetManagedSSHKey should succeed on 200 response")
	require.NotNil(t, got)
	assert.Equal(t, managedSSHKeyValue, got.SSHKey)
}

func TestClientGetManagedSSHKeyAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedCredentialsSSHKeyPath, r.URL.Path, "request path should retrieve Managed SSH key")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "restricted users cannot access managed SSH keys"}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetManagedSSHKey(t.Context())

	require.Error(t, err)

	var apiErr *linode.APIError
	assert.ErrorAs(t, err, &apiErr)
}

func TestClientGetManagedSSHKeyRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		assert.Equal(t, managedCredentialsSSHKeyPath, r.URL.Path, "request path should retrieve Managed SSH key")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedSSHKey{SSHKey: managedSSHKeyValue}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetManagedSSHKey(t.Context())

	require.NoError(t, err, "read-only GetManagedSSHKey should retry transient failures")
	require.NotNil(t, got)
	assert.Equal(t, int32(2), attempts.Load(), "read-only get should retry once after a transient failure")
	assert.Equal(t, managedSSHKeyValue, got.SSHKey)
}

func TestClientListManagedCredentialsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedCredentialsPath, r.URL.Path, "request path should list managed credentials")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "restricted users cannot access managed credentials"}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.ListManagedCredentials(t.Context(), 1, 25)

	require.Error(t, err)

	var apiErr *linode.APIError
	assert.ErrorAs(t, err, &apiErr)
}

func TestClientListManagedCredentialsRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		assert.Equal(t, managedCredentialsPath, r.URL.Path, "request path should list managed credentials")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ManagedCredential]{
			Data:    []linode.ManagedCredential{{ID: 9991, Label: managedCredentialsLabel}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListManagedCredentials(t.Context(), 1, 25)

	require.NoError(t, err, "read-only ListManagedCredentials should retry transient failures")
	require.NotNil(t, got)
	assert.Equal(t, int32(2), attempts.Load(), "read-only list should retry once after a transient failure")
}

func TestClientCreateManagedCredentialSuccess(t *testing.T) {
	t.Parallel()

	username := managedCredentialsUsername
	request := &linode.CreateManagedCredentialRequest{
		Label:    managedCredentialsLabel,
		Password: managedCredentialsPassword,
		Username: &username,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, managedCredentialsPath, r.URL.Path, "request path should create managed credentials")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		var got map[string]any
		assert.NoError(t, json.Unmarshal(body, &got))
		assert.Equal(t, managedCredentialsLabel, got["label"])
		assert.Equal(t, managedCredentialsPassword, got["password"])
		assert.Equal(t, managedCredentialsUsername, got["username"])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedCredential{
			ID:            9991,
			Label:         managedCredentialsLabel,
			LastDecrypted: managedCredentialsLastDecrypted,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateManagedCredential(t.Context(), request)

	require.NoError(t, err, "CreateManagedCredential should succeed on 200 response")
	require.NotNil(t, got)
	assert.Equal(t, 9991, got.ID)
	assert.Equal(t, managedCredentialsLabel, got.Label)
	assert.Equal(t, managedCredentialsLastDecrypted, got.LastDecrypted)
}

func TestClientCreateManagedCredentialAPIError(t *testing.T) {
	t.Parallel()

	request := &linode.CreateManagedCredentialRequest{
		Label:    managedCredentialsLabel,
		Password: managedCredentialsPassword,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, managedCredentialsPath, r.URL.Path, "request path should create managed credentials")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "restricted users cannot create managed credentials"}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.CreateManagedCredential(t.Context(), request)

	require.Error(t, err)

	var apiErr *linode.APIError
	assert.ErrorAs(t, err, &apiErr)
}

func TestClientCreateManagedCredentialDoesNotRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, managedCredentialsPath, r.URL.Path, "request path should create managed credentials")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		assert.NoError(t, err)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))
	_, err := client.CreateManagedCredential(t.Context(), &linode.CreateManagedCredentialRequest{
		Label:    managedCredentialsLabel,
		Password: managedCredentialsPassword,
	})

	require.Error(t, err, "CreateManagedCredential should fail on 500 response")
	assert.Equal(t, int32(1), attempts.Load(), "CreateManagedCredential must not retry and replay a mutating request")
}

func TestClientUpdateManagedCredentialSuccess(t *testing.T) {
	t.Parallel()

	label := "prod-password-2"
	updated := linode.ManagedCredential{ID: 9991, Label: label, LastDecrypted: managedCredentialsLastDecrypted}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, managedCredentialsPath+"/9991", r.URL.Path, "request path should include credential ID")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		var got linode.UpdateManagedCredentialRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")

		if assert.NotNil(t, got.Label) {
			assert.Equal(t, label, *got.Label)
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(updated))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateManagedCredential(t.Context(), 9991, linode.UpdateManagedCredentialRequest{Label: &label})

	require.NoError(t, err, "UpdateManagedCredential should succeed on 200 response")
	require.NotNil(t, got)
	assert.Equal(t, 9991, got.ID)
	assert.Equal(t, label, got.Label)
}

func TestClientUpdateManagedCredentialAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, managedCredentialsPath+"/9991", r.URL.Path, "request path should include credential ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "restricted users cannot update managed credentials"}},
		}))
	}))
	t.Cleanup(srv.Close)

	label := managedCredentialsLabel
	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateManagedCredential(t.Context(), 9991, linode.UpdateManagedCredentialRequest{Label: &label})

	require.Error(t, err)

	var apiErr *linode.APIError
	assert.ErrorAs(t, err, &apiErr)
}

func TestClientUpdateManagedCredentialNetworkError(t *testing.T) {
	t.Parallel()

	label := managedCredentialsLabel
	client := linode.NewClient("http://127.0.0.1:1", "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateManagedCredential(t.Context(), 9991, linode.UpdateManagedCredentialRequest{Label: &label})

	require.Error(t, err)

	var netErr *linode.NetworkError
	require.ErrorAs(t, err, &netErr)
	assert.Equal(t, "UpdateManagedCredential", netErr.Operation)
}

func TestClientUpdateManagedCredentialDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, managedCredentialsPath+"/9991", r.URL.Path, "request path should include credential ID")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	t.Cleanup(srv.Close)

	label := managedCredentialsLabel
	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	_, err := client.UpdateManagedCredential(t.Context(), 9991, linode.UpdateManagedCredentialRequest{Label: &label})

	require.Error(t, err, "mutating Managed credential update should not retry transient failures")
	assert.Equal(t, int32(1), calls.Load(), "client should call update exactly once")
}
