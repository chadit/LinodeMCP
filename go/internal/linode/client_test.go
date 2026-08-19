package linode_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	statusPending               = "pending"
	statusSuccessful            = "successful"
	accountServiceTransferToken = "service-token-example"
	accountEntityTransferDate   = "2021-02-11T16:37:03"
	accountEntityTransferExpiry = "2021-02-12T16:37:03"
	accountTransferRegion       = "us-east"
	accountLoginUsername        = "account-login-user"
	accountUserEmail            = "user@example.com"
	accountUserTypeDefault      = "default"
	accountUserPasswordCreated  = "2024-01-02T03:04:05"
	accountMaintenanceURL       = "/v4/linode/instances/123"
	errForbidden                = "forbidden"
	oauthClientID               = "client-123"
	oauthClientIDWithSeparators = "client/123?query"
	oauthClientStatus           = "active"
	keyRedirectURI              = "redirect_uri"
	keyThumbnailURL             = "thumbnail_url"
	imageShareGroupDescription  = "shared CI images"
	imageShareGroupUpdated      = "2025-04-15T22:44:02"
	imageShareGroupCreated      = "2025-04-14T22:44:02"
	oauthClientThumbnailURL     = "https://example.com/icon.png"
	oauthClientLabel            = "my app"
	oauthClientRedirectURI      = "https://example.com/callback"
	profileTokenScopesKey       = "scopes"
)

// TestClientGetProfileSuccess verifies that GetProfile returns a fully
// populated Profile when the API responds with 200 OK and valid JSON.
func TestClientGetProfileSuccess(t *testing.T) {
	t.Parallel()

	profile := linode.Profile{
		Username: "testuser",
		Email:    "test@example.com",
		UID:      1234,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profile" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile")
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(profile); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetProfile(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Username != tcTestuser {
		t.Errorf("result.Username = %v, want %v", result.Username, tcTestuser)
	}

	if result.Email != "test@example.com" {
		t.Errorf("result.Email = %v, want %v", result.Email, "test@example.com")
	}
}

// TestClientGetProfileWithScopes verifies that a personal access token
// response with the Scopes field populated round-trips through GetProfile.
// Phase 6 reads Profile.Scopes for PATs; OAuth tokens leave it empty and
// the loader uses GetProfileGrants instead.
func TestClientGetProfileWithScopes(t *testing.T) {
	t.Parallel()

	profile := linode.Profile{
		Username: "patuser",
		Email:    "pat@example.com",
		Scopes:   "linodes:read_write domains:read_only",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(profile); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "tok", nil, linode.WithMaxRetries(0))

	result, err := client.GetProfile(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Scopes != "linodes:read_write domains:read_only" {
		t.Errorf("result.Scopes = %v, want %v", result.Scopes, "linodes:read_write domains:read_only")
	}
}

// TestClientGetProfileGrantsSuccess covers the OAuth path: /profile/grants
// returns a structured Grants response listing global flags plus per-
// resource grant slices. The Phase 6 loader walks the same shape to
// figure out what the OAuth token is permitted to do.
func TestClientGetProfileGrantsSuccess(t *testing.T) {
	t.Parallel()

	want := linode.Grants{
		Global: linode.GlobalGrants{
			AccountAccess: "read_write",
			AddLinodes:    true,
			AddDomains:    false,
		},
		Linode: []linode.Grant{
			{ID: 42, Label: "web-1", Permissions: "read_write"},
		},
		Domain: []linode.Grant{
			{ID: 7, Label: "example.com", Permissions: "read_only"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profile/grants" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/grants")
		}

		if r.Header.Get("Authorization") != "Bearer oauth-token" {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer oauth-token")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(want); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "oauth-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetProfileGrants(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Global.AccountAccess != linode.GrantPermission("read_write") {
		t.Errorf("got.Global.AccountAccess = %v, want %v", got.Global.AccountAccess, linode.GrantPermission("read_write"))
	}

	if !got.Global.AddLinodes {
		t.Error("got.Global.AddLinodes = false, want true")
	}

	if len(got.Linode) != 1 {
		t.Fatalf("len(got.Linode) = %d, want %d", len(got.Linode), 1)
	}

	if got.Linode[0].Label != nodeLabelWeb1 {
		t.Errorf("got.Linode[0].Label = %v, want %v", got.Linode[0].Label, nodeLabelWeb1)
	}

	if got.Linode[0].Permissions != linode.GrantPermission("read_write") {
		t.Errorf("got.Linode[0].Permissions = %v, want %v", got.Linode[0].Permissions, linode.GrantPermission("read_write"))
	}

	if len(got.Domain) != 1 {
		t.Fatalf("len(got.Domain) = %d, want %d", len(got.Domain), 1)
	}

	if got.Domain[0].Permissions != linode.GrantPermission("read_only") {
		t.Errorf("got.Domain[0].Permissions = %v, want %v", got.Domain[0].Permissions, linode.GrantPermission("read_only"))
	}
}

// TestClientGetProfileGrantsPATEmpty verifies that a PAT (which doesn't use
// OAuth grants) returning an empty grants payload still parses cleanly.
// The Linode API returns 200 with zero-valued fields for this case; the
// Phase 6 loader detects "use PAT scopes path" by checking
// Profile.Scopes != "" before consulting Grants.
func TestClientGetProfileGrantsPATEmpty(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Grants{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "pat-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetProfileGrants(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if len(got.Linode) != 0 {
		t.Errorf("got.Linode = %v, want empty", got.Linode)
	}

	if got.Global.AccountAccess != "" {
		t.Errorf("got.Global.AccountAccess = %v, want empty", got.Global.AccountAccess)
	}
}

// TestClientGetProfileGrantsUnauthorized confirms 401 propagates as an
// APIError from GetProfileGrants, matching the GetProfile contract so
// Phase 6's loader can use the same error path for both calls.
func TestClientGetProfileGrantsUnauthorized(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)

		if err := json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"reason": "Invalid Token"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "bad-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileGrants(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusUnauthorized)
	}
}

// TestClientGetProfileUnauthorized verifies that GetProfile returns an
// APIError with status 401 when the API rejects the token.
func TestClientGetProfileUnauthorized(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)

		if err := json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"reason": "Invalid Token"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "bad-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfile(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, 401)
	}
}

// TestClientGetInstanceSuccess verifies that GetInstance returns the correct
// instance when given a valid ID.
func TestClientGetInstanceSuccess(t *testing.T) {
	t.Parallel()

	instance := linode.Instance{ID: 42, Label: "my-instance", Status: "running"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/42" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/42")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(instance); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	result, err := client.GetInstance(t.Context(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ID != 42 {
		t.Errorf("result.ID = %v, want %v", result.ID, 42)
	}

	if result.Label != "my-instance" {
		t.Errorf("result.Label = %v, want %v", result.Label, "my-instance")
	}
}

// TestClientGetInstanceServerError verifies that GetInstance returns an
// APIError with status 500 when the server responds with an internal error.
func TestClientGetInstanceServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.GetInstance(t.Context(), 1)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, 500)
	}
}

// TestClientGetProfileNetworkError verifies that GetProfile returns a
// NetworkError when the server is unreachable.
func TestClientGetProfileNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfile(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Errorf("error = %v, want %v", err, &netErr)
	}
}

// TestClientHandleResponseRateLimitWithRetryAfter verifies that the client
// returns an APIError with status 429 and includes the Retry-After value
// in the error message when the API rate-limits the request.
func TestClientHandleResponseRateLimitWithRetryAfter(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfile(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, 429)
	}

	if !strings.Contains(apiErr.Message, "retry after") {
		t.Errorf("apiErr.Message does not contain %v", "retry after")
	}

	if apiErr.RetryAfter != 30*time.Second {
		t.Errorf("apiErr.RetryAfter = %v, want %v", apiErr.RetryAfter, 30*time.Second)
	}
}

// TestClientHandleResponseForbiddenNoBody verifies that the client returns
// an APIError with status 403 when the API returns a forbidden response
// with an empty JSON body.
func TestClientHandleResponseForbiddenNoBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfile(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, 403)
	}
}

// TestClientContextCancelled verifies that GetProfile returns an error
// when the request context is already canceled before the call.
func TestClientContextCancelled(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(linode.Profile{Username: "test"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfile(ctx)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

// TestNewClientConfigOverridesDefaults verifies that NewClient applies
// config.Resilience values over hardcoded defaults, and that caller
// options override both.
func TestNewClientConfigOverridesDefaults(t *testing.T) {
	t.Parallel()

	// Track how many attempts the server sees. Config sets MaxRetries=5,
	// so we expect 6 total attempts (1 initial + 5 retries).
	var attempts int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++

		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Resilience: config.ResilienceConfig{
			MaxRetries:     5,
			BaseRetryDelay: 1 * time.Millisecond,
			MaxRetryDelay:  5 * time.Millisecond,
		},
	}

	client := linode.NewClient(srv.URL, "token", cfg, linode.WithJitter(false))

	_, err := client.GetProfile(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if attempts != 6 {
		t.Errorf("attempts = %v, want %v", attempts, 6)
	}
}

// TestNewClientOptionsOverrideConfig verifies that caller-supplied options
// take precedence over config.Resilience values.
func TestNewClientOptionsOverrideConfig(t *testing.T) {
	t.Parallel()

	var attempts int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++

		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	// Config says 5 retries, but option overrides to 1.
	cfg := &config.Config{
		Resilience: config.ResilienceConfig{
			MaxRetries:     5,
			BaseRetryDelay: 1 * time.Millisecond,
			MaxRetryDelay:  5 * time.Millisecond,
		},
	}

	client := linode.NewClient(
		srv.URL, "token", cfg,
		linode.WithMaxRetries(1),
		linode.WithJitter(false),
	)

	_, err := client.GetProfile(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if attempts != 2 {
		t.Errorf("attempts = %v, want %v", attempts, 2)
	}
}

// TestNewClientNilConfigUsesDefaults verifies that passing nil config
// uses hardcoded defaults (3 retries).
func TestNewClientNilConfigUsesDefaults(t *testing.T) {
	t.Parallel()

	var attempts int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++

		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := linode.NewClient(
		srv.URL, "token", nil,
		linode.WithBaseDelay(1*time.Millisecond),
		linode.WithMaxDelay(5*time.Millisecond),
		linode.WithJitter(false),
	)

	_, err := client.GetProfile(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if attempts != 4 {
		t.Errorf("attempts = %v, want %v", attempts, 4)
	}
}

// TestClientMalformedJSONResponse verifies that the client returns an error
// when the API responds with 200 OK but invalid JSON.
func TestClientMalformedJSONResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)

		_, err := w.Write([]byte(`not json at all`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfile(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	syntaxErr, ok := errors.AsType[*json.SyntaxError](err)
	if !ok {
		t.Errorf("error = %v, want %v", err, &syntaxErr)
	}
}

func TestClientListObjectStorageClustersRemoved(t *testing.T) {
	t.Parallel()

	_, ok := reflect.TypeFor[*linode.Client]().MethodByName("ListObjectStorageClusters")

	if ok {
		t.Error("ok = true, want false")
	}
}

// TestClientGetAccountSettingsSuccess verifies GetAccountSettings sends a GET
// request to /account/settings and returns the account settings response.
func TestClientGetAccountSettingsSuccess(t *testing.T) {
	t.Parallel()

	longviewSubscription := "longview-3"
	objectStorage := "active"
	settings := linode.AccountSettings{
		BackupsEnabled:          true,
		Managed:                 false,
		NetworkHelper:           true,
		LongviewSubscription:    &longviewSubscription,
		ObjectStorage:           &objectStorage,
		InterfacesForNewLinodes: "linode_default_but_legacy_config_allowed",
		MaintenancePolicy:       maintenancePolicyMigrate,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountSettings {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountSettings)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(settings); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountSettings(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.BackupsEnabled {
		t.Error("result.BackupsEnabled = false, want true")
	}

	if result.Managed {
		t.Error("result.Managed = true, want false")
	}

	if !result.NetworkHelper {
		t.Error("result.NetworkHelper = false, want true")
	}

	if result.LongviewSubscription == nil {
		t.Fatal("result.LongviewSubscription is nil")
	}

	if *result.LongviewSubscription != longviewSubscription {
		t.Errorf("*result.LongviewSubscription = %v, want %v", *result.LongviewSubscription, longviewSubscription)
	}

	if result.ObjectStorage == nil {
		t.Fatal("result.ObjectStorage is nil")
	}

	if *result.ObjectStorage != objectStorage {
		t.Errorf("*result.ObjectStorage = %v, want %v", *result.ObjectStorage, objectStorage)
	}

	if result.InterfacesForNewLinodes != "linode_default_but_legacy_config_allowed" {
		t.Errorf("result.InterfacesForNewLinodes = %v, want %v", result.InterfacesForNewLinodes, "linode_default_but_legacy_config_allowed")
	}

	if result.MaintenancePolicy != maintenancePolicyMigrate {
		t.Errorf("result.MaintenancePolicy = %v, want %v", result.MaintenancePolicy, maintenancePolicyMigrate)
	}
}

// TestClientGetAccountSettingsAPIError verifies GetAccountSettings propagates
// API errors through the handleResponse error chain.
func TestClientGetAccountSettingsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountSettings {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountSettings)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountSettings(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr == nil {
		t.Fatal("apiErr is nil")
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

// TestClientGetAccountOAuthClientSuccess verifies GetAccountOAuthClient sends the exact GET request.
func TestClientGetAccountOAuthClientSuccess(t *testing.T) {
	t.Parallel()

	want := linode.OAuthClient{ID: oauthClientID, Label: oauthClientLabel, RedirectURI: oauthClientRedirectURI, Status: oauthClientStatus, ThumbnailURL: oauthClientThumbnailURL}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/oauth-clients/"+oauthClientID {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/oauth-clients/"+oauthClientID)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: oauthClientID, keyLabel: oauthClientLabel, keyRedirectURI: oauthClientRedirectURI, keyStatus: oauthClientStatus, keyThumbnailURL: oauthClientThumbnailURL, "secret": "server-secret",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetAccountOAuthClient(t.Context(), oauthClientID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if !reflect.DeepEqual(*got, want) {
		t.Errorf("*got = %v, want %v", *got, want)
	}
}

func TestClientGetAccountOAuthClientEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != tcAccountOauthClientsClient2F1233Fquery {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), tcAccountOauthClientsClient2F1233Fquery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.OAuthClient{ID: oauthClientIDWithSeparators}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountOAuthClient(t.Context(), oauthClientIDWithSeparators)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientGetAccountOAuthClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/oauth-clients/"+oauthClientID {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/oauth-clients/"+oauthClientID)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountOAuthClient(t.Context(), oauthClientID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if !strings.Contains(apiErr.Message, errForbidden) {
		t.Errorf("apiErr.Message does not contain %v", errForbidden)
	}
}

func TestClientGetAccountOAuthClientRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempt := requestCount.Add(1)
		if attempt == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.OAuthClient{ID: oauthClientID, Label: oauthClientLabel}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.GetAccountOAuthClient(t.Context(), oauthClientID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != oauthClientID {
		t.Errorf("got.ID = %v, want %v", got.ID, oauthClientID)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestClientUpdateOAuthClientThumbnailSuccess(t *testing.T) {
	t.Parallel()

	thumbnailPNG := []byte("png-bytes")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/account/oauth-clients/"+oauthClientID+"/thumbnail" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/oauth-clients/"+oauthClientID+"/thumbnail")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if r.Header.Get("Content-Type") != "image/png" {
			t.Errorf("got %v, want %v", r.Header.Get("Content-Type"), "image/png")
		}

		got, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got, thumbnailPNG) {
			t.Errorf("got = %v, want %v", got, thumbnailPNG)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.UpdateOAuthClientThumbnail(t.Context(), oauthClientID, thumbnailPNG)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientUpdateOAuthClientThumbnailEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/account/oauth-clients/client%2F123%3Fquery/thumbnail" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/account/oauth-clients/client%2F123%3Fquery/thumbnail")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.OAuthClient{ID: oauthClientIDWithSeparators}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.UpdateOAuthClientThumbnail(t.Context(), oauthClientIDWithSeparators, []byte("png-bytes"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientUpdateOAuthClientThumbnailAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/account/oauth-clients/"+oauthClientID+"/thumbnail" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/oauth-clients/"+oauthClientID+"/thumbnail")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.UpdateOAuthClientThumbnail(t.Context(), oauthClientID, []byte("png-bytes"))
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientUpdateOAuthClientThumbnailDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.URL.Path != "/account/oauth-clients/"+oauthClientID+"/thumbnail" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/oauth-clients/"+oauthClientID+"/thumbnail")
		}

		http.Error(w, errTemporaryFailure, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.UpdateOAuthClientThumbnail(t.Context(), oauthClientID, []byte("png-bytes"))
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientGetOAuthClientThumbnailSuccess(t *testing.T) {
	t.Parallel()

	thumbnailPNG := []byte("png-bytes")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/oauth-clients/"+oauthClientID+"/thumbnail" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/oauth-clients/"+oauthClientID+"/thumbnail")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", "image/png")

		_, writeErr := w.Write(thumbnailPNG)
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetOAuthClientThumbnail(t.Context(), oauthClientID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(got, thumbnailPNG) {
		t.Errorf("got = %v, want %v", got, thumbnailPNG)
	}
}

func TestClientGetOAuthClientThumbnailEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/account/oauth-clients/client%2F123%3Fquery/thumbnail" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/account/oauth-clients/client%2F123%3Fquery/thumbnail")
		}

		w.Header().Set("Content-Type", "image/png")

		_, writeErr := w.Write([]byte("png-bytes"))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetOAuthClientThumbnail(t.Context(), oauthClientIDWithSeparators)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientGetOAuthClientThumbnailAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/oauth-clients/"+oauthClientID+"/thumbnail" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/oauth-clients/"+oauthClientID+"/thumbnail")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusNotFound)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "Not Found"}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetOAuthClientThumbnail(t.Context(), oauthClientID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusNotFound)
	}
}

func TestClientGetOAuthClientThumbnailRetriesOnTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	thumbnailPNG := []byte("png-bytes")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			http.Error(w, errTemporaryFailure, http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "image/png")

		_, writeErr := w.Write(thumbnailPNG)
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.GetOAuthClientThumbnail(t.Context(), oauthClientID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(got, thumbnailPNG) {
		t.Errorf("got = %v, want %v", got, thumbnailPNG)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

// TestClientGetAccountEventSuccess verifies GetAccountEvent sends a GET
// request to /account/events/{event_id} and decodes the response.
func TestClientGetAccountEventSuccess(t *testing.T) {
	t.Parallel()

	want := linode.AccountEvent{ID: 123, Action: "linode_create", Status: statusSuccessful, Username: "test-user"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcAccountEvents123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountEvents123)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(want); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetAccountEvent(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != 123 {
		t.Errorf("got.ID = %v, want %v", got.ID, 123)
	}

	if got.Action != tcLinodeCreate {
		t.Errorf("got.Action = %v, want %v", got.Action, tcLinodeCreate)
	}

	if got.Status != statusSuccessful {
		t.Errorf("got.Status = %v, want %v", got.Status, statusSuccessful)
	}
}

// TestClientGetAccountEventAPIError verifies GetAccountEvent propagates API errors.
func TestClientGetAccountEventAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcAccountEvents123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountEvents123)
		}

		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountEvent(t.Context(), 123)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

// TestClientGetAccountEventRetriesTransientError verifies the read-only event lookup retries transient failures.
func TestClientGetAccountEventRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := requestCount.Add(1)
		if current == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)

			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			if writeErr != nil {
				t.Errorf("unexpected error: %v", writeErr)
			}

			return
		}

		if r.URL.Path != tcAccountEvents123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountEvents123)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountEvent{ID: 123, Action: "linode_create"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountEvent(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != 123 {
		t.Errorf("result.ID = %v, want %v", result.ID, 123)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestClientGetAccountPaymentMethodSuccess(t *testing.T) {
	t.Parallel()

	want := linode.AccountPaymentMethod{ID: 123, Type: paymentMethodCreditCard, IsDefault: true, Data: map[string]any{keyLastFour: "1111"}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountPaymentMethods123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPaymentMethods123)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(want); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetAccountPaymentMethod(t.Context(), "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if !reflect.DeepEqual(*got, want) {
		t.Errorf("*got = %v, want %v", *got, want)
	}
}

func TestClientGetAccountPaymentMethodEscapesID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/account/payment-methods/123%2F456%3Fquery" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/account/payment-methods/123%2F456%3Fquery")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountPaymentMethod{ID: 123, Type: paymentMethodCreditCard}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountPaymentMethod(t.Context(), "123/456?query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientGetAccountPaymentMethodAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountPaymentMethods123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPaymentMethods123)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountPaymentMethod(t.Context(), "123")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if !strings.Contains(apiErr.Message, errForbidden) {
		t.Errorf("apiErr.Message does not contain %v", errForbidden)
	}
}

func TestClientGetAccountPaymentMethodRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempt := requestCount.Add(1)
		if attempt == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountPaymentMethod{ID: 123, Type: paymentMethodCreditCard}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.GetAccountPaymentMethod(t.Context(), "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != 123 {
		t.Errorf("got.ID = %v, want %v", got.ID, 123)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestClientListAccountEntityTransfersRemoved(t *testing.T) {
	t.Parallel()

	_, ok := reflect.TypeFor[*linode.Client]().MethodByName("ListAccountEntityTransfers")
	if ok {
		t.Error("ok = true, want false")
	}
}

// TestClientGetAccountServiceTransferSuccess verifies GetAccountServiceTransfer sends a GET
// request to /account/service-transfers/{token} and decodes the response.
func TestClientGetAccountServiceTransferSuccess(t *testing.T) {
	t.Parallel()

	want := linode.AccountEntityTransfer{
		Created:  accountEntityTransferDate,
		Entities: linode.AccountEntityTransferEntities{Linodes: []int{111, 222}},
		Expiry:   accountEntityTransferExpiry,
		IsSender: true,
		Status:   statusPending,
		Token:    accountServiceTransferToken,
		Updated:  accountEntityTransferDate,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountServiceTransfersServiceTokenExample {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountServiceTransfersServiceTokenExample)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(want); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetAccountServiceTransfer(t.Context(), accountServiceTransferToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Token != accountServiceTransferToken {
		t.Errorf("got.Token = %v, want %v", got.Token, accountServiceTransferToken)
	}

	if got.Status != statusPending {
		t.Errorf("got.Status = %v, want %v", got.Status, statusPending)
	}

	if !reflect.DeepEqual(got.Entities.Linodes, []int{111, 222}) {
		t.Errorf("got.Entities.Linodes = %v, want %v", got.Entities.Linodes, []int{111, 222})
	}
}

// TestClientGetAccountServiceTransferEscapesToken verifies the client encodes path separators.
func TestClientGetAccountServiceTransferEscapesToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/account/service-transfers/service%2Ftoken%3Fquery" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/account/service-transfers/service%2Ftoken%3Fquery")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountEntityTransfer{Token: "service/token?query"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountServiceTransfer(t.Context(), "service/token?query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClientGetAccountServiceTransferAPIError verifies GetAccountServiceTransfer propagates API errors.
func TestClientGetAccountServiceTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountServiceTransfersServiceTokenExample {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountServiceTransfersServiceTokenExample)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountServiceTransfer(t.Context(), accountServiceTransferToken)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr == nil {
		t.Fatal("apiErr is nil")
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

// TestClientGetAccountServiceTransferRetriesTransientError verifies the read-only lookup retries transient failures.
func TestClientGetAccountServiceTransferRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			if writeErr != nil {
				t.Errorf("unexpected error: %v", writeErr)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountServiceTransfersServiceTokenExample {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountServiceTransfersServiceTokenExample)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountEntityTransfer{Token: accountServiceTransferToken, Status: statusPending}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountServiceTransfer(t.Context(), accountServiceTransferToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Token != accountServiceTransferToken {
		t.Errorf("result.Token = %v, want %v", result.Token, accountServiceTransferToken)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestClientAccountEntityTransferAcceptRouteRemoved(t *testing.T) {
	t.Parallel()

	_, exists := reflect.TypeFor[*linode.Client]().MethodByName("AcceptAccountEntityTransfer")
	if exists {
		t.Error("exists = true, want false")
	}
}

func TestClientDeleteAccountEntityTransferDeprecatedRouteRemoved(t *testing.T) {
	t.Parallel()

	_, exists := reflect.TypeFor[*linode.Client]().MethodByName("DeleteAccountEntityTransfer")
	if exists {
		t.Error("exists = true, want false")
	}
}

// TestClientGetAccountUserSuccess verifies GetAccountUser sends a GET
// request to /account/users/{username} and returns the account user response.
func TestClientGetAccountUserSuccess(t *testing.T) {
	t.Parallel()

	passwordCreated := accountUserPasswordCreated
	user := linode.AccountUser{
		Email:           accountUserEmail,
		PasswordCreated: &passwordCreated,
		Restricted:      true,
		TFAEnabled:      true,
		UserType:        accountUserTypeDefault,
		Username:        accountLoginUsername,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/users/"+accountLoginUsername {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/users/"+accountLoginUsername)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(user); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountUser(t.Context(), accountLoginUsername)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Username != accountLoginUsername {
		t.Errorf("result.Username = %v, want %v", result.Username, accountLoginUsername)
	}

	if result.Email != accountUserEmail {
		t.Errorf("result.Email = %v, want %v", result.Email, accountUserEmail)
	}

	if !result.Restricted {
		t.Error("result.Restricted = false, want true")
	}

	if !result.TFAEnabled {
		t.Error("result.TFAEnabled = false, want true")
	}
}

// TestClientGetAccountUserEscapesUsername verifies the client encodes path separators.
func TestClientGetAccountUserEscapesUsername(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != tcAccountUsersUser2Fname3Fquery {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), tcAccountUsersUser2Fname3Fquery)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountUser{Username: "user/name?query"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountUser(t.Context(), tcUserNameQuery)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClientGetAccountUserAPIError verifies GetAccountUser propagates API errors.
func TestClientGetAccountUserAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/users/"+accountLoginUsername {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/users/"+accountLoginUsername)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountUser(t.Context(), accountLoginUsername)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr == nil {
		t.Fatal("apiErr is nil")
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

// TestClientGetAccountUserRetriesTransientError verifies the read-only lookup retries transient failures.
func TestClientGetAccountUserRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			if writeErr != nil {
				t.Errorf("unexpected error: %v", writeErr)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/users/"+accountLoginUsername {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/users/"+accountLoginUsername)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountUser{Username: accountLoginUsername, Email: accountUserEmail}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountUser(t.Context(), accountLoginUsername)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Username != accountLoginUsername {
		t.Errorf("result.Username = %v, want %v", result.Username, accountLoginUsername)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

// TestClientGetAccountUserGrantsSuccess verifies GetAccountUserGrants sends a GET
// request to /account/users/{username}/grants and returns the grants response.
func TestClientGetAccountUserGrantsSuccess(t *testing.T) {
	t.Parallel()

	grants := linode.Grants{
		Global: linode.GlobalGrants{AccountAccess: linode.GrantPermission("read_only")},
		Linode: []linode.Grant{{ID: 123, Permissions: linode.GrantPermission("read_write")}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/users/"+accountLoginUsername+"/grants" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/users/"+accountLoginUsername+"/grants")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(grants); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountUserGrants(t.Context(), accountLoginUsername)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Global.AccountAccess != linode.GrantPermission("read_only") {
		t.Errorf("result.Global.AccountAccess = %v, want %v", result.Global.AccountAccess, linode.GrantPermission("read_only"))
	}

	if len(result.Linode) != 1 {
		t.Fatalf("len(result.Linode) = %d, want %d", len(result.Linode), 1)
	}

	if result.Linode[0].ID != 123 {
		t.Errorf("result.Linode[0].ID = %v, want %v", result.Linode[0].ID, 123)
	}
}

// TestClientGetAccountUserGrantsEscapesUsername verifies the client encodes path separators.
func TestClientGetAccountUserGrantsEscapesUsername(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/account/users/user%2Fname%3Fquery/grants" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/account/users/user%2Fname%3Fquery/grants")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Grants{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountUserGrants(t.Context(), tcUserNameQuery)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClientGetAccountUserGrantsAPIError verifies GetAccountUserGrants propagates API errors.
func TestClientGetAccountUserGrantsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/users/"+accountLoginUsername+"/grants" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/users/"+accountLoginUsername+"/grants")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountUserGrants(t.Context(), accountLoginUsername)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr == nil {
		t.Fatal("apiErr is nil")
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

// TestClientGetAccountUserGrantsRetriesTransientError verifies the read-only grants lookup retries transient failures.
func TestClientGetAccountUserGrantsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			if writeErr != nil {
				t.Errorf("unexpected error: %v", writeErr)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/users/"+accountLoginUsername+"/grants" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/users/"+accountLoginUsername+"/grants")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Grants{Global: linode.GlobalGrants{AccountAccess: linode.GrantPermission("read_only")}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountUserGrants(t.Context(), accountLoginUsername)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Global.AccountAccess != linode.GrantPermission("read_only") {
		t.Errorf("result.Global.AccountAccess = %v, want %v", result.Global.AccountAccess, linode.GrantPermission("read_only"))
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

// TestClientGetAccountChildAccountSuccess verifies GetAccountChildAccount sends a GET
// request to /account/child-accounts/{euuId} and decodes the response.
func TestClientGetAccountChildAccountSuccess(t *testing.T) {
	t.Parallel()

	childAccount := linode.ChildAccount{
		EUUID:         childAccountEUUID,
		Company:       companyAcme,
		Email:         "jkowalski@example.com",
		FirstName:     "John",
		LastName:      "Smith",
		BillingSource: "external",
		CreditCard: linode.ChildAccountCreditCard{
			Expiry:   "11/2024",
			LastFour: "0111",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountChildAccountsA1BC2DEF34GH567IJ890KLMN12 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountChildAccountsA1BC2DEF34GH567IJ890KLMN12)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(childAccount); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountChildAccount(t.Context(), childAccountEUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.EUUID != childAccountEUUID {
		t.Errorf("result.EUUID = %v, want %v", result.EUUID, childAccountEUUID)
	}

	if result.Company != companyAcme {
		t.Errorf("result.Company = %v, want %v", result.Company, companyAcme)
	}

	if result.CreditCard.Expiry != tcLit {
		t.Errorf("result.CreditCard.Expiry = %v, want %v", result.CreditCard.Expiry, tcLit)
	}

	if result.CreditCard.LastFour != "0111" {
		t.Errorf("result.CreditCard.LastFour = %v, want %v", result.CreditCard.LastFour, "0111")
	}
}

// TestClientGetAccountChildAccountEscapesEUUID verifies the client encodes path separators.
func TestClientGetAccountChildAccountEscapesEUUID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/account/child-accounts/child%2Faccount%3Fquery" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/account/child-accounts/child%2Faccount%3Fquery")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ChildAccount{EUUID: "child/account?query"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountChildAccount(t.Context(), "child/account?query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClientGetAccountChildAccountAPIError verifies GetAccountChildAccount propagates API errors.
func TestClientGetAccountChildAccountAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountChildAccountsA1BC2DEF34GH567IJ890KLMN12 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountChildAccountsA1BC2DEF34GH567IJ890KLMN12)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountChildAccount(t.Context(), childAccountEUUID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr == nil {
		t.Fatal("apiErr is nil")
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

// TestClientGetAccountChildAccountRetriesTransientError verifies the read-only lookup retries transient failures.
func TestClientGetAccountChildAccountRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			if writeErr != nil {
				t.Errorf("unexpected error: %v", writeErr)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountChildAccountsA1BC2DEF34GH567IJ890KLMN12 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountChildAccountsA1BC2DEF34GH567IJ890KLMN12)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ChildAccount{EUUID: childAccountEUUID, Company: companyAcme}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountChildAccount(t.Context(), childAccountEUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.EUUID != childAccountEUUID {
		t.Errorf("result.EUUID = %v, want %v", result.EUUID, childAccountEUUID)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestClientGetImageShareGroupSuccess(t *testing.T) {
	t.Parallel()

	description := shareGroupDescriptionFixture
	updated := shareGroupUpdatedFixture
	shareGroup := linode.ImageShareGroup{
		ID:           123,
		UUID:         shareGroupUUIDExample,
		Label:        imageShareGroupLabel,
		Description:  &description,
		IsSuspended:  false,
		Created:      shareGroupCreatedFixture,
		Updated:      &updated,
		ImagesCount:  2,
		MembersCount: 3,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcImagesSharegroups123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcImagesSharegroups123)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+"test-token" {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+"test-token")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(shareGroup); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetImageShareGroup(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != 123 {
		t.Errorf("result.ID = %v, want %v", result.ID, 123)
	}

	if result.Label != imageShareGroupLabel {
		t.Errorf("result.Label = %v, want %v", result.Label, imageShareGroupLabel)
	}
}

func TestClientGetImageShareGroupAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcImagesSharegroups123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcImagesSharegroups123)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]any{{keyReason: "temporary share group failure"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetImageShareGroup(t.Context(), 123)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.Message != "temporary share group failure" {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, "temporary share group failure")
	}
}

func TestClientGetImageShareGroupNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "token", nil, linode.WithMaxRetries(0))

	_, err := client.GetImageShareGroup(t.Context(), 123)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &netErr)
	}

	if netErr.Operation != "GetImageShareGroup" {
		t.Errorf("netErr.Operation = %v, want %v", netErr.Operation, "GetImageShareGroup")
	}
}

func TestClientGetImageShareGroupRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	description := shareGroupDescriptionFixture
	updated := shareGroupUpdatedFixture
	shareGroup := linode.ImageShareGroup{
		ID:           123,
		UUID:         shareGroupUUIDExample,
		Label:        imageShareGroupLabel,
		Description:  &description,
		IsSuspended:  false,
		Created:      shareGroupCreatedFixture,
		Updated:      &updated,
		ImagesCount:  2,
		MembersCount: 3,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcImagesSharegroups123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcImagesSharegroups123)
		}

		if requestCount.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]any{{keyReason: errTemporaryFailure}},
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(shareGroup); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	result, err := client.GetImageShareGroup(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}

	if result.Label != imageShareGroupLabel {
		t.Errorf("result.Label = %v, want %v", result.Label, imageShareGroupLabel)
	}
}

// TestNewClientCarriesTheObjectStorageSettings pins the seam the Object Storage
// upload depends on: its execute hook is handed a client and no config, so the
// data-plane budgets have to arrive on the client or the hook cannot read them.
func TestNewClientCarriesTheObjectStorageSettings(t *testing.T) {
	t.Parallel()

	settings := config.ObjectStorageConfig{
		FilesystemRoot:     "/srv/exports",
		MaxSinglePartBytes: 1024,
		TransferTimeout:    time.Minute,
		PresignTTLSeconds:  60,
	}

	client := linode.NewClient("https://api.linode.test/v4", "token",
		&config.Config{ObjectStorage: settings})

	if got := client.ObjectStorage(); got != settings {
		t.Errorf("ObjectStorage() = %+v, want %+v", got, settings)
	}
}

// TestNewClientWithoutAConfigHasNoObjectStorageSettings pins that a client built
// with no config answers zeroes rather than inventing budgets; the hook fills
// the defaults so both languages resolve them in one place.
func TestNewClientWithoutAConfigHasNoObjectStorageSettings(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.test/v4", "token", nil)

	if got := client.ObjectStorage(); got != (config.ObjectStorageConfig{}) {
		t.Errorf("ObjectStorage() = %+v, want the zero value", got)
	}
}
