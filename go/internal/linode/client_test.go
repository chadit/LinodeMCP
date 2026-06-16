package linode_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
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
	updateAccountEmail           = "account-updated@example.com"
	profilePreferenceKeyTheme    = "theme"
	profilePreferenceValueDark   = "dark"
	statusPending                = "pending"
	statusSuccessful             = "successful"
	accountEntityTransferToken   = "transfer-token-example"
	accountServiceTransferToken  = "service-token-example"
	accountEntityTransferDate    = "2021-02-11T16:37:03"
	accountEntityTransferExpiry  = "2021-02-12T16:37:03"
	accountTransferRegion        = "us-east"
	accountLoginUsername         = "account-login-user"
	accountLoginIP               = "203.0.113.10"
	accountUserEmail             = "user@example.com"
	accountUserTypeDefault       = "default"
	accountUserPasswordCreated   = "2024-01-02T03:04:05"
	accountUserLastLoginDatetime = "2024-02-03T04:05:06"
	accountMaintenancePath       = "/account/maintenance"
	accountMaintenanceLabel      = "web-1"
	accountMaintenanceEntityType = "linode"
	accountMaintenanceURL        = "/v4/linode/instances/123"
	maintenancePoliciesPath      = "/maintenance/policies"
	maintenancePolicySlug        = "linode/migrate"
	maintenancePolicyLabel       = "Migrate"
	errForbidden                 = "forbidden"
	oauthClientID                = "client-123"
	oauthClientIDWithSeparators  = "client/123?query"
	oauthClientStatus            = "active"
	keyRedirectURI               = "redirect_uri"
	keyThumbnailURL              = "thumbnail_url"
	keyUserAgent                 = "user_agent"
	serverErrorReason            = "server error"
	keyLastRemoteAddr            = "last_remote_addr"
	keyLastAuthenticated         = "last_authenticated"
	imageShareGroupDescription   = "shared CI images"
	imageShareGroupUpdated       = "2025-04-15T22:44:02"
	imageShareGroupCreated       = "2025-04-14T22:44:02"
	oauthClientThumbnailURL      = "https://example.com/icon.png"
	oauthClientLabel             = "my app"
	oauthClientRedirectURI       = "https://example.com/callback"
	profileAppScopesReadOnly     = "linodes:read_only"
	profileDeviceUserAgent       = "Mozilla/5.0"
	profileDeviceRemoteAddr      = "203.0.113.1"
	profilePhoneISOCode          = "US"
	profilePhoneNumber           = "+15551234567"
	profilePhoneOTPCode          = "123456"
	profileTokenExpiryFixture    = "2024-06-01T00:01:01"
	profileTokenLabelFixture     = "ci-token"
	profileTokenScopesFixture    = "linodes:read_only"
	profileTokenSecretFixture    = "secret-token-value"
	profileTokenScopesKey        = "scopes"
)

// TestClientCreateProfileTokenSuccess verifies CreateProfileToken sends a POST
// request to /profile/tokens with the documented body and decodes the response.
func TestClientCreateProfileTokenSuccess(t *testing.T) {
	t.Parallel()

	profileToken := linode.ProfileToken{keyCreated: "2024-05-01T00:01:01", keyTFAConfirmExpiry: profileTokenExpiryFixture, keyID: float64(321), keyLabel: profileTokenLabelFixture, profileTokenScopesKey: profileTokenScopesFixture, keyToken: profileTokenSecretFixture}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcProfileTokens {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTokens)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		var got linode.CreateProfileTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if got.Expiry != profileTokenExpiryFixture {
			t.Errorf("got.Expiry = %v, want %v", got.Expiry, profileTokenExpiryFixture)
		}

		if got.Label != profileTokenLabelFixture {
			t.Errorf("got.Label = %v, want %v", got.Label, profileTokenLabelFixture)
		}

		if got.Scopes != profileTokenScopesFixture {
			t.Errorf("got.Scopes = %v, want %v", got.Scopes, profileTokenScopesFixture)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(profileToken); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateProfileToken(t.Context(), linode.CreateProfileTokenRequest{Expiry: profileTokenExpiryFixture, Label: profileTokenLabelFixture, Scopes: profileTokenScopesFixture})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	gotID, ok := (*result)[keyID].(float64)
	if !ok {
		t.Fatalf("(*result)[keyID] = %v, want a float64", (*result)[keyID])
	}

	if gotID != 321 {
		t.Errorf("(*result)[keyID] = %v, want %v", gotID, float64(321))
	}

	if !reflect.DeepEqual((*result)[keyLabel], profileTokenLabelFixture) {
		t.Errorf("(*result)[keyLabel] = %v, want %v", (*result)[keyLabel], profileTokenLabelFixture)
	}

	if !reflect.DeepEqual((*result)[profileTokenScopesKey], profileTokenScopesFixture) {
		t.Errorf("(*result)[profileTokenScopesKey] = %v, want %v", (*result)[profileTokenScopesKey], profileTokenScopesFixture)
	}

	if !reflect.DeepEqual((*result)[keyToken], profileTokenSecretFixture) {
		t.Errorf("(*result)[keyToken] = %v, want %v", (*result)[keyToken], profileTokenSecretFixture)
	}
}

// TestClientCreateProfileTokenAPIError verifies API errors propagate.
func TestClientCreateProfileTokenAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcProfileTokens {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTokens)
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

	_, err := client.CreateProfileToken(t.Context(), linode.CreateProfileTokenRequest{Label: profileTokenLabelFixture})
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

// TestClientCreateProfileTokenDoesNotRetryTransientError verifies token
// creation is not replayed after a transient failure.
func TestClientCreateProfileTokenDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.CreateProfileToken(t.Context(), linode.CreateProfileTokenRequest{Label: profileTokenLabelFixture})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

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

func TestClientGetProfilePreferencesSuccess(t *testing.T) {
	t.Parallel()

	preferences := linode.ProfilePreferences{
		"desktop_notifications": true,
		"sort_order":            "ascending",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfilePreferences {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfilePreferences)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.ContentLength != int64(0) {
			t.Errorf("r.ContentLength = %v, want %v", r.ContentLength, int64(0))
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(preferences); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	got, err := client.GetProfilePreferences(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if !reflect.DeepEqual(*got, preferences) {
		t.Errorf("*got = %v, want %v", *got, preferences)
	}
}

func TestClientGetProfilePreferencesUnauthorized(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcProfilePreferences {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfilePreferences)
		}

		w.WriteHeader(http.StatusUnauthorized)

		_, err := w.Write([]byte(`{}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "bad-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfilePreferences(t.Context())
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

func TestClientGetProfileAppSuccess(t *testing.T) {
	t.Parallel()

	want := linode.ProfileApp{ID: 12345, Label: "Example OAuth App", Scopes: profileAppScopesReadOnly, Website: "https://example.com"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileApps12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileApps12345)
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

	got, err := client.GetProfileApp(t.Context(), want.ID)
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

func TestClientGetProfileAppBuildsNumericPath(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != tcProfileApps12345 {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), tcProfileApps12345)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ProfileApp{ID: 12345}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileApp(t.Context(), 12345)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientGetProfileAppAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileApps12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileApps12345)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileApp(t.Context(), 12345)
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

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientGetProfileDeviceSuccess(t *testing.T) {
	t.Parallel()

	want := linode.ProfileDevice{keyID: float64(12345), keyUserAgent: profileDeviceUserAgent, keyLastRemoteAddr: profileDeviceRemoteAddr, keyLastAuthenticated: accountUserPasswordCreated}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileDevices12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileDevices12345)
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

	got, err := client.GetProfileDevice(t.Context(), 12345)
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

func TestClientGetProfileDeviceBuildsNumericPath(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcProfileDevices12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileDevices12345)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ProfileDevice{keyID: float64(12345)}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileDevice(t.Context(), 12345)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientGetProfileDeviceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileDevices12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileDevices12345)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileDevice(t.Context(), 12345)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error should wrap APIError: %v", err)
	}

	if !strings.Contains(apiErr.Message, errForbidden) {
		t.Errorf("apiErr.Message does not contain %v", errForbidden)
	}
}

func TestClientGetProfileDeviceRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requestCount.Add(1) == 1 {
			w.Header().Set("Content-Type", tcApplicationJSON)
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ProfileDevice{keyID: float64(12345), keyUserAgent: profileDeviceUserAgent}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.GetProfileDevice(t.Context(), 12345)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	gotID, ok := (*got)[keyID].(float64)
	if !ok {
		t.Fatalf("(*got)[keyID] = %v, want a float64", (*got)[keyID])
	}

	if gotID != 12345 {
		t.Errorf("(*got)[keyID] = %v, want %v", gotID, float64(12345))
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestClientGetProfileAppRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.Header().Set("Content-Type", tcApplicationJSON)
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

		if r.URL.Path != tcProfileApps12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileApps12345)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ProfileApp{ID: 12345, Label: "Example OAuth App"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.GetProfileApp(t.Context(), 12345)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != 12345 {
		t.Errorf("got.ID = %v, want %v", got.ID, 12345)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestClientEnableProfileTFASuccess(t *testing.T) {
	t.Parallel()

	response := linode.ProfileTFAEnableResponse{
		"secret": "JBSWY3DPEHPK3PXP",
		"expiry": "2026-01-01T00:00:00",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/profile/tfa-enable" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/tfa-enable")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Errorf("request body should be readable: %v", readErr)

			return
		}

		if len(body) != 0 {
			t.Errorf("string(body) = %v, want empty", string(body))
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.EnableProfileTFA(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(result["secret"], "JBSWY3DPEHPK3PXP") {
		t.Errorf("got %v, want %v", result["secret"], "JBSWY3DPEHPK3PXP")
	}

	if !reflect.DeepEqual(result["expiry"], "2026-01-01T00:00:00") {
		t.Errorf("got %v, want %v", result["expiry"], "2026-01-01T00:00:00")
	}
}

func TestClientEnableProfileTFANoRetryOnTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/profile/tfa-enable" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/tfa-enable")
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))

	result, err := client.EnableProfileTFA(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

func TestClientSendProfilePhoneNumberVerificationCodeSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcProfilePhoneNumber {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfilePhoneNumber)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body should be JSON: %v", err)

			return
		}

		if body["iso_code"] != profilePhoneISOCode {
			t.Errorf("got %v, want %v", body["iso_code"], profilePhoneISOCode)
		}

		if body["phone_number"] != profilePhoneNumber {
			t.Errorf("got %v, want %v", body["phone_number"], profilePhoneNumber)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.SendProfilePhoneNumberVerificationCode(t.Context(), &linode.ProfilePhoneNumberRequest{
		ISOCode:     profilePhoneISOCode,
		PhoneNumber: profilePhoneNumber,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientSendProfilePhoneNumberVerificationCodeAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcProfilePhoneNumber {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfilePhoneNumber)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.SendProfilePhoneNumberVerificationCode(t.Context(), &linode.ProfilePhoneNumberRequest{
		ISOCode:     profilePhoneISOCode,
		PhoneNumber: profilePhoneNumber,
	})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientDeleteProfilePhoneNumberSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcProfilePhoneNumber {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfilePhoneNumber)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteProfilePhoneNumber(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientDeleteProfilePhoneNumberAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcProfilePhoneNumber {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfilePhoneNumber)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteProfilePhoneNumber(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientVerifyProfilePhoneNumberSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/profile/phone-number/verify" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/phone-number/verify")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body should be JSON: %v", err)

			return
		}

		if body["otp_code"] != profilePhoneOTPCode {
			t.Errorf("got %v, want %v", body["otp_code"], profilePhoneOTPCode)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.VerifyProfilePhoneNumber(t.Context(), &linode.ProfilePhoneNumberVerifyRequest{OTPCode: profilePhoneOTPCode})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientVerifyProfilePhoneNumberAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/profile/phone-number/verify" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/phone-number/verify")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.VerifyProfilePhoneNumber(t.Context(), &linode.ProfilePhoneNumberVerifyRequest{OTPCode: profilePhoneOTPCode})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientDeleteProfileAppSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcProfileApps12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileApps12345)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteProfileApp(t.Context(), 12345)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientDeleteProfileAppAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcProfileApps12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileApps12345)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteProfileApp(t.Context(), 12345)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientDeleteProfileDeviceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != "/profile/devices/67890" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/devices/67890")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteProfileDevice(t.Context(), 67890)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientDeleteProfileDeviceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != "/profile/devices/67890" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/devices/67890")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteProfileDevice(t.Context(), 67890)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientListAccountMaintenanceSuccess(t *testing.T) {
	t.Parallel()

	maintenance := linode.PaginatedResponse[linode.AccountMaintenance]{
		Data: []linode.AccountMaintenance{{
			Entity: linode.AccountMaintenanceEntity{ID: 123, Label: accountMaintenanceLabel, Type: accountMaintenanceEntityType, URL: accountMaintenanceURL},
			Reason: "A scheduled migration is required.",
			Status: statusPending,
			Type:   "reboot",
			When:   "2026-06-01T00:00:00",
		}},
		Page:    1,
		Pages:   1,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountMaintenancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountMaintenancePath)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedIssueAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedIssueAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(maintenance); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountMaintenance(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Entity.Label != accountMaintenanceLabel {
		t.Errorf("result.Data[0].Entity.Label = %v, want %v", result.Data[0].Entity.Label, accountMaintenanceLabel)
	}

	if result.Data[0].Type != "reboot" {
		t.Errorf("result.Data[0].Type = %v, want %v", result.Data[0].Type, "reboot")
	}
}

func TestClientListAccountMaintenanceRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != accountMaintenancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountMaintenancePath)
		}

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountMaintenance]{
			Data:    []linode.AccountMaintenance{{Status: statusPending}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))

	result, err := client.ListAccountMaintenance(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}
}

func TestClientListAccountMaintenanceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountMaintenancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountMaintenancePath)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "Forbidden"}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountMaintenance(t.Context(), 0, 0)
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

func TestClientListMaintenancePoliciesSuccess(t *testing.T) {
	t.Parallel()

	policies := linode.PaginatedResponse[linode.MaintenancePolicy]{
		Data: []linode.MaintenancePolicy{{
			Slug:                  maintenancePolicySlug,
			Label:                 maintenancePolicyLabel,
			Description:           "Migrates the Linode during maintenance.",
			Type:                  "migrate",
			NotificationPeriodSec: 86400,
			IsDefault:             true,
		}},
		Page:    1,
		Pages:   1,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != maintenancePoliciesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, maintenancePoliciesPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		if r.Header.Get("Authorization") != managedIssueAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedIssueAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(policies); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	result, err := client.ListMaintenancePolicies(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Slug != maintenancePolicySlug {
		t.Errorf("result.Data[0].Slug = %v, want %v", result.Data[0].Slug, maintenancePolicySlug)
	}

	if result.Data[0].Label != maintenancePolicyLabel {
		t.Errorf("result.Data[0].Label = %v, want %v", result.Data[0].Label, maintenancePolicyLabel)
	}

	if !result.Data[0].IsDefault {
		t.Error("result.Data[0].IsDefault = false, want true")
	}
}

func TestClientListMaintenancePoliciesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != maintenancePoliciesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, maintenancePoliciesPath)
		}

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.MaintenancePolicy]{
			Data:    []linode.MaintenancePolicy{{Slug: maintenancePolicySlug}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))

	result, err := client.ListMaintenancePolicies(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}
}

func TestClientListMaintenancePoliciesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != maintenancePoliciesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, maintenancePoliciesPath)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "Forbidden"}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.ListMaintenancePolicies(t.Context(), 0, 0)
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

// TestClientListInstancesSuccess verifies that ListInstances returns the
// full slice of instances from a paginated API response.
func TestClientListInstancesSuccess(t *testing.T) {
	t.Parallel()

	instances := []linode.Instance{
		{ID: 1, Label: "web-1", Status: "running"},
		{ID: 2, Label: "db-1", Status: "stopped"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    instances,
			keyPage:    1,
			keyPages:   1,
			keyResults: 2,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	result, err := client.ListInstances(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("len(result) = %d, want %d", len(result), 2)
	}

	if result[0].Label != nodeLabelWeb1 {
		t.Errorf("result[0].Label = %v, want %v", result[0].Label, nodeLabelWeb1)
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

// TestClientListProfileSecurityQuestionsSuccess verifies ListProfileSecurityQuestions sends a GET request to /profile/security-questions.
func TestClientListProfileSecurityQuestionsSuccess(t *testing.T) {
	t.Parallel()

	questions := linode.ProfileSecurityQuestions{
		"security_questions": []map[string]any{{keyID: float64(1), "question": "What is your favorite color?"}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileSecurityQuestions {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileSecurityQuestions)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(questions); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListProfileSecurityQuestions(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	items, isList := (*result)["security_questions"].([]any)
	if !isList {
		t.Fatal("ok = false, want true")
	}

	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want %d", len(items), 1)
	}

	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !reflect.DeepEqual(item["question"], "What is your favorite color?") {
		t.Errorf("got %v, want %v", item["question"], "What is your favorite color?")
	}
}

// TestClientListProfileSecurityQuestionsAPIError verifies ListProfileSecurityQuestions propagates API errors.
func TestClientListProfileSecurityQuestionsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcProfileSecurityQuestions {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileSecurityQuestions)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListProfileSecurityQuestions(t.Context())
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

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

// TestClientListProfileSecurityQuestionsRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListProfileSecurityQuestionsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcProfileSecurityQuestions {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileSecurityQuestions)
		}

		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ProfileSecurityQuestions{"security_questions": []map[string]any{{keyID: float64(1)}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListProfileSecurityQuestions(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if attempts.Load() != int32(2) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(2))
	}
}

// TestClientListProfileDevicesSuccess verifies ListProfileDevices sends a GET request to /profile/devices.
func TestClientListProfileDevicesSuccess(t *testing.T) {
	t.Parallel()

	devices := linode.PaginatedResponse[linode.ProfileDevice]{
		Data: []linode.ProfileDevice{{keyID: float64(123), "user_agent": "Mozilla/5.0", "last_remote_addr": "192.0.2.1"}},
		Page: 2, Pages: 3, Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileDevices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileDevices)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(devices); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListProfileDevices(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if !reflect.DeepEqual(result.Data[0]["user_agent"], "Mozilla/5.0") {
		t.Errorf("got %v, want %v", result.Data[0]["user_agent"], "Mozilla/5.0")
	}

	if !reflect.DeepEqual(result.Data[0]["last_remote_addr"], "192.0.2.1") {
		t.Errorf("got %v, want %v", result.Data[0]["last_remote_addr"], "192.0.2.1")
	}
}

// TestClientListProfileDevicesAPIError verifies ListProfileDevices propagates API errors.
func TestClientListProfileDevicesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcProfileDevices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileDevices)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListProfileDevices(t.Context(), 0, 0)
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

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

// TestClientListProfileDevicesRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListProfileDevicesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcProfileDevices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileDevices)
		}

		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ProfileDevice]{Data: []linode.ProfileDevice{{keyID: float64(123)}}, Page: 1, Pages: 1, Results: 1}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListProfileDevices(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if attempts.Load() != int32(2) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(2))
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

// TestClientUpdateProfileSuccess verifies that UpdateProfile sends a PUT
// request to /profile with the correct body and returns the updated Profile.
func TestClientUpdateProfileSuccess(t *testing.T) {
	t.Parallel()

	updatedProfile := linode.Profile{
		Username:           "testuser",
		Email:              updateAccountEmail,
		UID:                1234,
		EmailNotifications: true,
		Timezone:           "US/Eastern",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/profile" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile")
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if r.Header.Get("Content-Type") != tcApplicationJSON {
			t.Errorf("got %v, want %v", r.Header.Get("Content-Type"), tcApplicationJSON)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["email"], updateAccountEmail) {
			t.Errorf("got %v, want %v", body["email"], updateAccountEmail)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(updatedProfile); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	email := updateAccountEmail

	result, err := client.UpdateProfile(t.Context(), &linode.UpdateProfileRequest{
		Email: &email,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Email != updateAccountEmail {
		t.Errorf("result.Email = %v, want %v", result.Email, updateAccountEmail)
	}

	if result.Timezone != "US/Eastern" {
		t.Errorf("result.Timezone = %v, want %v", result.Timezone, "US/Eastern")
	}
}

// TestClientUpdateProfilePreferencesSuccess verifies that UpdateProfilePreferences
// sends a PUT request to /profile/preferences with an empty JSON body.
func TestClientUpdateProfilePreferencesSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcProfilePreferences {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfilePreferences)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if r.Header.Get("Content-Type") != tcApplicationJSON {
			t.Errorf("got %v, want %v", r.Header.Get("Content-Type"), tcApplicationJSON)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(body) != 0 {
			t.Errorf("body = %v, want empty", body)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{profilePreferenceKeyTheme: profilePreferenceValueDark}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateProfilePreferences(t.Context(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(result[profilePreferenceKeyTheme], profilePreferenceValueDark) {
		t.Errorf("result[profilePreferenceKeyTheme] = %v, want %v", result[profilePreferenceKeyTheme], profilePreferenceValueDark)
	}
}

// TestClientUpdateProfileNetworkError verifies that UpdateProfile returns a
// NetworkError when the HTTP request fails to reach the server.
func TestClientUpdateProfilePreferencesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"errors":[{"field":"theme","reason":"invalid preference"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateProfilePreferences(t.Context(), linode.ProfilePreferences{profilePreferenceKeyTheme: profilePreferenceValueDark})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, 400)
	}
}

func TestClientUpdateProfilePreferencesNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateProfilePreferences(t.Context(), linode.ProfilePreferences{profilePreferenceKeyTheme: profilePreferenceValueDark})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Errorf("error = %v, want %v", err, &netErr)
	}
}

func TestClientUpdateProfilePreferencesMalformedJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)

		_, err := w.Write([]byte(`not json at all`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateProfilePreferences(t.Context(), linode.ProfilePreferences{profilePreferenceKeyTheme: profilePreferenceValueDark})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	syntaxErr, ok := errors.AsType[*json.SyntaxError](err)
	if !ok {
		t.Errorf("error = %v, want %v", err, &syntaxErr)
	}
}

func TestClientUpdateProfileNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateProfile(t.Context(), &linode.UpdateProfileRequest{})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Errorf("error = %v, want %v", err, &netErr)
	}
}

// TestClientUpdateProfileAPIError verifies that UpdateProfile propagates
// API errors (non-2xx) through the handleResponse error chain.
func TestClientUpdateProfileAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"errors":[{"field":"email","reason":"invalid email format"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateProfile(t.Context(), &linode.UpdateProfileRequest{})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, 400)
	}
}

func TestClientListObjectStorageBucketsByRegionSuccess(t *testing.T) {
	t.Parallel()

	buckets := []linode.ObjectStorageBucket{
		{Label: "my-bucket", Region: "us-east-1", Hostname: "my-bucket.us-east-1.linodeobjects.com"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/object-storage/buckets/us-east-1" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/buckets/us-east-1")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    buckets,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListObjectStorageBucketsByRegion(t.Context(), "us-east-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want %d", len(result), 1)
	}

	if result[0].Label != "my-bucket" {
		t.Errorf("result[0].Label = %v, want %v", result[0].Label, "my-bucket")
	}
}

func TestClientListObjectStorageBucketsByRegionEscapesPathParam(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/object-storage/buckets/us%2Feast%3F1" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/object-storage/buckets/us%2Feast%3F1")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    []linode.ObjectStorageBucket{},
			keyPage:    1,
			keyPages:   1,
			keyResults: 0,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListObjectStorageBucketsByRegion(t.Context(), "us/east?1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientListObjectStorageClustersRemoved(t *testing.T) {
	t.Parallel()

	_, ok := reflect.TypeFor[*linode.Client]().MethodByName("ListObjectStorageClusters")

	if ok {
		t.Error("ok = true, want false")
	}
}

func TestClientCancelObjectStorageSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcObjectStorageCancel {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcObjectStorageCancel)
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.CancelObjectStorage(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientCancelObjectStorageAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcObjectStorageCancel {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcObjectStorageCancel)
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"errors":[{"reason":"object storage cannot be canceled"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.CancelObjectStorage(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusBadRequest)
	}
}

func TestClientCancelObjectStorageDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.URL.Path != tcObjectStorageCancel {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcObjectStorageCancel)
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.CancelObjectStorage(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

func TestClientAllowObjectStorageBucketAccessSuccess(t *testing.T) {
	t.Parallel()

	corsEnabled := true

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/object-storage/buckets/us-east-1/my-bucket/access" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/buckets/us-east-1/my-bucket/access")
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if r.Header.Get("Content-Type") != tcApplicationJSON {
			t.Errorf("got %v, want %v", r.Header.Get("Content-Type"), tcApplicationJSON)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["acl"], "public-read") {
			t.Errorf("got %v, want %v", body["acl"], "public-read")
		}

		if !reflect.DeepEqual(body["cors_enabled"], true) {
			t.Errorf("got %v, want %v", body["cors_enabled"], true)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.AllowObjectStorageBucketAccess(t.Context(), "us-east-1", "my-bucket", linode.AllowObjectStorageBucketAccessRequest{
		ACL:         "public-read",
		CORSEnabled: &corsEnabled,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientAllowObjectStorageBucketAccessEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/object-storage/buckets/us%2Feast%3F1/..%2Fbucket/access" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/object-storage/buckets/us%2Feast%3F1/..%2Fbucket/access")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.AllowObjectStorageBucketAccess(t.Context(), "us/east?1", "../bucket", linode.AllowObjectStorageBucketAccessRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientAllowObjectStorageBucketAccessDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/object-storage/buckets/us-east-1/my-bucket/access" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/buckets/us-east-1/my-bucket/access")
		}

		w.WriteHeader(http.StatusInternalServerError)

		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.AllowObjectStorageBucketAccess(t.Context(), "us-east-1", "my-bucket", linode.AllowObjectStorageBucketAccessRequest{})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

// TestClientGetAccountTransferSuccess verifies GetAccountTransfer sends a GET
// request to /account/transfer and returns the account transfer response.
func TestClientGetAccountTransferSuccess(t *testing.T) {
	t.Parallel()

	transfer := linode.AccountTransfer{
		Billable: 10,
		Quota:    4000,
		Used:     123,
		RegionTransfers: []linode.AccountRegionTransfer{{
			ID:       accountTransferRegion,
			Billable: 2,
			Quota:    1000,
			Used:     50,
		}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountTransfer {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountTransfer)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(transfer); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountTransfer(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Billable != 10 {
		t.Errorf("result.Billable = %v, want %v", result.Billable, 10)
	}

	if result.Quota != 4000 {
		t.Errorf("result.Quota = %v, want %v", result.Quota, 4000)
	}

	if result.Used != 123 {
		t.Errorf("result.Used = %v, want %v", result.Used, 123)
	}

	if len(result.RegionTransfers) != 1 {
		t.Fatalf("len(result.RegionTransfers) = %d, want %d", len(result.RegionTransfers), 1)
	}

	if result.RegionTransfers[0].ID != accountTransferRegion {
		t.Errorf("result.RegionTransfers[0].ID = %v, want %v", result.RegionTransfers[0].ID, accountTransferRegion)
	}

	if result.RegionTransfers[0].Used != 50 {
		t.Errorf("result.RegionTransfers[0].Used = %v, want %v", result.RegionTransfers[0].Used, 50)
	}
}

// TestClientGetAccountTransferAPIError verifies GetAccountTransfer propagates
// API errors through the handleResponse error chain.
func TestClientGetAccountTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountTransfer {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountTransfer)
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

	_, err := client.GetAccountTransfer(t.Context())
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

// TestClientGetAccountTransferRetriesTransientError verifies read-only transfer retries.
func TestClientGetAccountTransferRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := requestCount.Add(1)
		if current == 1 {
			w.WriteHeader(http.StatusTooManyRequests)

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountTransfer {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountTransfer)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountTransfer{Used: 123}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountTransfer(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Used != 123 {
		t.Errorf("result.Used = %v, want %v", result.Used, 123)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
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

// TestClientGetAccountAgreementsSuccess verifies GetAccountAgreements sends a GET
// request to /account/agreements and returns the agreement statuses.
func TestClientGetAccountAgreementsSuccess(t *testing.T) {
	t.Parallel()

	agreements := linode.AccountAgreements{
		BillingAgreement:       true,
		EUModel:                false,
		MasterServiceAgreement: true,
		PrivacyPolicy:          true,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountAgreements {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountAgreements)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(agreements); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountAgreements(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.BillingAgreement {
		t.Error("result.BillingAgreement = false, want true")
	}

	if result.EUModel {
		t.Error("result.EUModel = true, want false")
	}

	if !result.MasterServiceAgreement {
		t.Error("result.MasterServiceAgreement = false, want true")
	}

	if !result.PrivacyPolicy {
		t.Error("result.PrivacyPolicy = false, want true")
	}
}

// TestClientGetAccountAgreementsAPIError verifies GetAccountAgreements propagates
// API errors through the handleResponse error chain.
func TestClientGetAccountAgreementsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountAgreements {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountAgreements)
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

	_, err := client.GetAccountAgreements(t.Context())
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

// TestClientListAccountNotificationsSuccess verifies ListAccountNotifications sends
// a GET request to /account/notifications with pagination query parameters.
func TestClientListAccountNotificationsSuccess(t *testing.T) {
	t.Parallel()

	when := "2026-05-22T08:00:00"
	notifications := linode.PaginatedResponse[linode.AccountNotification]{
		Data: []linode.AccountNotification{{
			Label:    "Scheduled maintenance",
			Message:  "Maintenance is scheduled for a Linode.",
			Severity: "major",
			Type:     "maintenance",
			When:     &when,
			Entity: &linode.AccountNotificationEntity{
				ID:    float64(123),
				Label: "example-linode",
				Type:  accountMaintenanceEntityType,
				URL:   "/v4/linode/instances/123",
			},
		}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountNotifications {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountNotifications)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(notifications); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountNotifications(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Label != tcScheduledMaintenance {
		t.Errorf("result.Data[0].Label = %v, want %v", result.Data[0].Label, tcScheduledMaintenance)
	}

	if result.Data[0].Severity != "major" {
		t.Errorf("result.Data[0].Severity = %v, want %v", result.Data[0].Severity, "major")
	}

	if result.Data[0].Entity == nil {
		t.Fatal("result.Data[0].Entity is nil")
	}

	if result.Data[0].Entity.Label != "example-linode" {
		t.Errorf("result.Data[0].Entity.Label = %v, want %v", result.Data[0].Entity.Label, "example-linode")
	}
}

// TestClientListAccountNotificationsAPIError verifies ListAccountNotifications propagates API errors.
func TestClientListAccountNotificationsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountNotifications {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountNotifications)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
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

	_, err := client.ListAccountNotifications(t.Context(), 0, 0)
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

// TestClientListAccountNotificationsRetriesTransientError verifies the read-only notifications lookup retries transient failures.
func TestClientListAccountNotificationsRetriesTransientError(t *testing.T) {
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

		if r.URL.Path != tcAccountNotifications {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountNotifications)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountNotification]{
			Data: []linode.AccountNotification{{Label: "Scheduled maintenance"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountNotifications(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

// TestClientGetAccountAvailabilitySuccess verifies GetAccountAvailability sends
// a GET request to /account/availability/{regionId} and decodes the response.
func TestClientGetAccountAvailabilitySuccess(t *testing.T) {
	t.Parallel()

	availability := linode.AccountAvailability{
		Available:   []string{serviceLinodes, serviceNodeBalancers},
		Region:      regionUSEast,
		Unavailable: []string{"Kubernetes", "Block Storage"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/availability/"+regionUSEast {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/availability/"+regionUSEast)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(availability); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountAvailability(t.Context(), regionUSEast)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Region != regionUSEast {
		t.Errorf("result.Region = %v, want %v", result.Region, regionUSEast)
	}

	if !reflect.DeepEqual(result.Available, []string{serviceLinodes, serviceNodeBalancers}) {
		t.Errorf("result.Available = %v, want %v", result.Available, []string{serviceLinodes, serviceNodeBalancers})
	}
}

// TestClientGetAccountAvailabilityEscapesRegion verifies the client encodes path separators.
func TestClientGetAccountAvailabilityEscapesRegion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != "/account/availability/us%2Feast%3Fzone" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/account/availability/us%2Feast%3Fzone")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountAvailability{Region: "us/east?zone"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountAvailability(t.Context(), "us/east?zone")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClientGetAccountAvailabilityAPIError verifies GetAccountAvailability propagates API errors.
func TestClientGetAccountAvailabilityAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/availability/"+regionUSEast {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/availability/"+regionUSEast)
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

	_, err := client.GetAccountAvailability(t.Context(), regionUSEast)
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

// TestClientGetAccountAvailabilityRetriesTransientError verifies the read-only regional lookup retries transient failures.
func TestClientGetAccountAvailabilityRetriesTransientError(t *testing.T) {
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

		if r.URL.Path != "/account/availability/"+regionUSEast {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/availability/"+regionUSEast)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountAvailability{Region: regionUSEast}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountAvailability(t.Context(), regionUSEast)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Region != regionUSEast {
		t.Errorf("result.Region = %v, want %v", result.Region, regionUSEast)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

// TestClientListAccountAvailabilitySuccess verifies ListAccountAvailability sends
// a GET request to /account/availability with pagination query parameters.
func TestClientListAccountAvailabilitySuccess(t *testing.T) {
	t.Parallel()

	availability := linode.PaginatedResponse[linode.AccountAvailability]{
		Data: []linode.AccountAvailability{{
			Available:   []string{serviceLinodes, serviceNodeBalancers},
			Region:      regionUSEast,
			Unavailable: []string{"Kubernetes", "Block Storage"},
		}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/availability" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/availability")
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(availability); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountAvailability(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Region != regionUSEast {
		t.Errorf("result.Data[0].Region = %v, want %v", result.Data[0].Region, regionUSEast)
	}

	if !reflect.DeepEqual(result.Data[0].Available, []string{serviceLinodes, serviceNodeBalancers}) {
		t.Errorf("result.Data[0].Available = %v, want %v", result.Data[0].Available, []string{serviceLinodes, serviceNodeBalancers})
	}
}

// TestClientListAccountAvailabilityAPIError verifies ListAccountAvailability propagates API errors.
func TestClientListAccountAvailabilityAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/availability" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/availability")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
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

	_, err := client.ListAccountAvailability(t.Context(), 0, 0)
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

// TestClientListProfileAppsSuccess verifies ListProfileApps sends a GET
// request to /profile/apps with pagination query parameters.
func TestClientListProfileAppsSuccess(t *testing.T) {
	t.Parallel()

	thumbnailURL := "https://example.com/icon.png"
	expiry := "2018-01-15T00:01:01"
	apps := linode.PaginatedResponse[linode.AuthorizedApp]{
		Data: []linode.AuthorizedApp{{ID: 123, Label: "example-app", Scopes: profileAppScopesReadOnly, Website: "example.org", Created: "2018-01-01T00:01:01", Expiry: &expiry, ThumbnailURL: &thumbnailURL}},
		Page: 2, Pages: 3, Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileApps {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileApps)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(apps); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListProfileApps(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Label != tcExampleApp {
		t.Errorf("result.Data[0].Label = %v, want %v", result.Data[0].Label, tcExampleApp)
	}

	if result.Data[0].Scopes != profileAppScopesReadOnly {
		t.Errorf("result.Data[0].Scopes = %v, want %v", result.Data[0].Scopes, profileAppScopesReadOnly)
	}

	if result.Data[0].Website != "example.org" {
		t.Errorf("result.Data[0].Website = %v, want %v", result.Data[0].Website, "example.org")
	}

	if result.Data[0].ThumbnailURL == nil {
		t.Fatal("result.Data[0].ThumbnailURL is nil")
	}

	if *result.Data[0].ThumbnailURL != thumbnailURL {
		t.Errorf("*result.Data[0].ThumbnailURL = %v, want %v", *result.Data[0].ThumbnailURL, thumbnailURL)
	}
}

// TestClientListProfileAppsAPIError verifies ListProfileApps propagates API errors.
func TestClientListProfileAppsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcProfileApps {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileApps)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListProfileApps(t.Context(), 0, 0)
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

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

// TestClientListProfileAppsRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListProfileAppsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcProfileApps {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileApps)
		}

		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "temporary upstream failure"}}}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AuthorizedApp]{Data: []linode.AuthorizedApp{{ID: 123, Label: "example-app", Scopes: profileAppScopesReadOnly}}, Page: 1, Pages: 1, Results: 1}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListProfileApps(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if attempts.Load() != int32(2) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(2))
	}
}

// TestClientListAccountOAuthClientsSuccess verifies ListAccountOAuthClients sends a GET
// request to /account/oauth-clients with pagination query parameters.
func TestClientListAccountOAuthClientsSuccess(t *testing.T) {
	t.Parallel()

	clients := linode.PaginatedResponse[linode.OAuthClient]{
		Data: []linode.OAuthClient{{ID: "2737bf16b39ab5d7b4a1", Label: "example-client", RedirectURI: "https://example.com/oauth/callback", Status: oauthClientStatus, ThumbnailURL: oauthClientThumbnailURL}},
		Page: 2, Pages: 3, Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountOauthClients {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountOauthClients)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(clients); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountOAuthClients(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Label != tcExampleClient {
		t.Errorf("result.Data[0].Label = %v, want %v", result.Data[0].Label, tcExampleClient)
	}

	if result.Data[0].RedirectURI != "https://example.com/oauth/callback" {
		t.Errorf("result.Data[0].RedirectURI = %v, want %v", result.Data[0].RedirectURI, "https://example.com/oauth/callback")
	}
}

// TestClientListAccountOAuthClientsAPIError verifies ListAccountOAuthClients propagates API errors.
func TestClientListAccountOAuthClientsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcAccountOauthClients {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountOauthClients)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountOAuthClients(t.Context(), 0, 0)
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

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

// TestClientListAccountOAuthClientsRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListAccountOAuthClientsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcAccountOauthClients {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountOauthClients)
		}

		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "temporary upstream failure"}}}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.OAuthClient]{Data: []linode.OAuthClient{{ID: "2737bf16b39ab5d7b4a1", Label: "example-client"}}, Page: 1, Pages: 1, Results: 1}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountOAuthClients(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if attempts.Load() != int32(2) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(2))
	}
}

// TestClientListBetasSuccess verifies ListBetas sends a GET request to /betas with pagination query parameters.
func TestClientListBetasSuccess(t *testing.T) {
	t.Parallel()

	description := "This beta lets users try an example feature."
	betas := linode.PaginatedResponse[linode.BetaProgram]{
		Data: []linode.BetaProgram{{
			BetaClass:      "open",
			Description:    &description,
			Ended:          nil,
			GreenlightOnly: false,
			ID:             "example_open",
			Label:          "Example Open Beta",
			MoreInfo:       "https://example.com/beta",
			Started:        accountUserPasswordCreated,
		}},
		Page:    2,
		Pages:   4,
		Results: 90,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcBetas {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcBetas)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(betas); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListBetas(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != betaExampleOpen {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, betaExampleOpen)
	}

	if result.Data[0].BetaClass != supportTicketStatusOpen {
		t.Errorf("result.Data[0].BetaClass = %v, want %v", result.Data[0].BetaClass, supportTicketStatusOpen)
	}

	if result.Data[0].GreenlightOnly {
		t.Error("result.Data[0].GreenlightOnly = true, want false")
	}

	if result.Data[0].MoreInfo != tcHTTPSExampleComBeta {
		t.Errorf("result.Data[0].MoreInfo = %v, want %v", result.Data[0].MoreInfo, tcHTTPSExampleComBeta)
	}

	if result.Data[0].Ended != nil {
		t.Errorf("result.Data[0].Ended = %v, want nil", result.Data[0].Ended)
	}
}

// TestClientListBetasAPIError verifies ListBetas propagates API errors.
func TestClientListBetasAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcBetas {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcBetas)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListBetas(t.Context(), 0, 0)
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

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

// TestClientListBetasRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListBetasRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	betas := linode.PaginatedResponse[linode.BetaProgram]{
		Data: []linode.BetaProgram{{ID: "example_open", Label: "Example Open Beta"}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcBetas {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcBetas)
		}

		if requestCount.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(betas); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListBetas(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != betaExampleOpen {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, betaExampleOpen)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

// TestClientGetBetaSuccess verifies GetBeta sends a GET request to
// /betas/{betaId} and decodes the response.
func TestClientGetBetaSuccess(t *testing.T) {
	t.Parallel()

	description := "This beta lets users try an example feature."
	beta := linode.BetaProgram{
		BetaClass:      "open",
		Description:    &description,
		Ended:          nil,
		GreenlightOnly: false,
		ID:             betaExampleOpen,
		Label:          labelExampleOpenBeta,
		MoreInfo:       "https://example.com/beta",
		Started:        accountUserPasswordCreated,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/betas/"+betaExampleOpen {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/betas/"+betaExampleOpen)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(beta); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetBeta(t.Context(), betaExampleOpen)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != betaExampleOpen {
		t.Errorf("result.ID = %v, want %v", result.ID, betaExampleOpen)
	}

	if result.Label != labelExampleOpenBeta {
		t.Errorf("result.Label = %v, want %v", result.Label, labelExampleOpenBeta)
	}

	if result.Description == nil {
		t.Fatal("result.Description is nil")
	}

	if *result.Description != description {
		t.Errorf("*result.Description = %v, want %v", *result.Description, description)
	}

	if result.BetaClass != supportTicketStatusOpen {
		t.Errorf("result.BetaClass = %v, want %v", result.BetaClass, supportTicketStatusOpen)
	}

	if result.GreenlightOnly {
		t.Error("result.GreenlightOnly = true, want false")
	}
}

// TestClientGetBetaEscapesID verifies the client encodes path separators.
func TestClientGetBetaEscapesID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/betas/example%2Fopen%3Fquery" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/betas/example%2Fopen%3Fquery")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.BetaProgram{ID: "example/open?query"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetBeta(t.Context(), "example/open?query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClientGetBetaAPIError verifies GetBeta propagates API errors.
func TestClientGetBetaAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/betas/"+betaExampleOpen {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/betas/"+betaExampleOpen)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetBeta(t.Context(), betaExampleOpen)
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

// TestClientGetBetaRetriesTransientError verifies the read-only lookup retries transient failures.
func TestClientGetBetaRetriesTransientError(t *testing.T) {
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

		if r.URL.Path != "/betas/"+betaExampleOpen {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/betas/"+betaExampleOpen)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.BetaProgram{ID: betaExampleOpen, Label: labelExampleOpenBeta}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetBeta(t.Context(), betaExampleOpen)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != betaExampleOpen {
		t.Errorf("result.ID = %v, want %v", result.ID, betaExampleOpen)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

// TestClientListAccountBetasSuccess verifies ListAccountBetas sends a GET
// request to /account/betas with pagination query parameters.
func TestClientListAccountBetasSuccess(t *testing.T) {
	t.Parallel()

	description := "This is an open public beta for an example feature."
	betas := linode.PaginatedResponse[linode.AccountBetaProgram]{
		Data: []linode.AccountBetaProgram{{
			Description: &description,
			Ended:       nil,
			Enrolled:    "2023-09-11T00:00:00",
			ID:          betaExampleOpen,
			Label:       labelExampleOpenBeta,
			Started:     "2023-07-11T00:00:00",
		}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountBetas {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountBetas)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(betas); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountBetas(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != betaExampleOpen {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, betaExampleOpen)
	}

	if result.Data[0].Label != labelExampleOpenBeta {
		t.Errorf("result.Data[0].Label = %v, want %v", result.Data[0].Label, labelExampleOpenBeta)
	}

	if result.Data[0].Description == nil {
		t.Fatal("result.Data[0].Description is nil")
	}

	if *result.Data[0].Description != description {
		t.Errorf("*result.Data[0].Description = %v, want %v", *result.Data[0].Description, description)
	}

	if result.Data[0].Ended != nil {
		t.Errorf("result.Data[0].Ended = %v, want nil", result.Data[0].Ended)
	}
}

// TestClientListAccountBetasAPIError verifies ListAccountBetas propagates API errors.
func TestClientListAccountBetasAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountBetas {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountBetas)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
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

	_, err := client.ListAccountBetas(t.Context(), 0, 0)
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

// TestClientListAccountBetasRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListAccountBetasRetriesTransientError(t *testing.T) {
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

		if r.URL.Path != tcAccountBetas {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountBetas)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountBetaProgram]{
			Data: []linode.AccountBetaProgram{{ID: betaExampleOpen, Label: labelExampleOpenBeta}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountBetas(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != betaExampleOpen {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, betaExampleOpen)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
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

func TestClientUpdateOAuthClientSuccess(t *testing.T) {
	t.Parallel()

	public := true
	want := linode.OAuthClient{ID: oauthClientID, Label: "updated app", Public: public, RedirectURI: "https://example.com/new-callback", Status: oauthClientStatus, ThumbnailURL: oauthClientThumbnailURL}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
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

		if r.Header.Get("Content-Type") != tcApplicationJSON {
			t.Errorf("got %v, want %v", r.Header.Get("Content-Type"), tcApplicationJSON)
		}

		var got linode.UpdateOAuthClientRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if got.Label == nil {
			t.Fatal("got.Label is nil")
		}

		if *got.Label != want.Label {
			t.Errorf("*got.Label = %v, want %v", *got.Label, want.Label)
		}

		if got.RedirectURI == nil {
			t.Fatal("got.RedirectURI is nil")
		}

		if *got.RedirectURI != want.RedirectURI {
			t.Errorf("*got.RedirectURI = %v, want %v", *got.RedirectURI, want.RedirectURI)
		}

		if got.Public == nil {
			t.Fatal("got.Public is nil")
		}

		if *got.Public != public {
			t.Errorf("*got.Public = %v, want %v", *got.Public, public)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(want); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.UpdateOAuthClientRequest{Label: &want.Label, Public: &public, RedirectURI: &want.RedirectURI}

	got, err := client.UpdateOAuthClient(t.Context(), oauthClientID, req)
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

func TestClientUpdateOAuthClientEscapesClientID(t *testing.T) {
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
	label := oauthClientLabel
	req := &linode.UpdateOAuthClientRequest{Label: &label}

	_, err := client.UpdateOAuthClient(t.Context(), oauthClientIDWithSeparators, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientUpdateOAuthClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
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
	label := oauthClientLabel
	req := &linode.UpdateOAuthClientRequest{Label: &label}

	_, err := client.UpdateOAuthClient(t.Context(), oauthClientID, req)
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

func TestClientUpdateOAuthClientDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	label := oauthClientLabel
	req := &linode.UpdateOAuthClientRequest{Label: &label}

	_, err := client.UpdateOAuthClient(t.Context(), oauthClientID, req)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientCreateOAuthClientSuccess(t *testing.T) {
	t.Parallel()

	want := linode.CreatedOAuthClient{ID: oauthClientID, Label: oauthClientLabel, RedirectURI: oauthClientRedirectURI, Secret: "secret-once"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountOauthClients {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountOauthClients)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if r.Header.Get("Content-Type") != tcApplicationJSON {
			t.Errorf("got %v, want %v", r.Header.Get("Content-Type"), tcApplicationJSON)
		}

		var got linode.CreateOAuthClientRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if got.Label != oauthClientLabel {
			t.Errorf("got.Label = %v, want %v", got.Label, oauthClientLabel)
		}

		if got.RedirectURI != oauthClientRedirectURI {
			t.Errorf("got.RedirectURI = %v, want %v", got.RedirectURI, oauthClientRedirectURI)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(want); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateOAuthClientRequest{Label: oauthClientLabel, RedirectURI: oauthClientRedirectURI}

	got, err := client.CreateOAuthClient(t.Context(), req)
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

func TestClientDeleteAccountOAuthClientSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
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

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountOAuthClient(t.Context(), oauthClientID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientDeleteAccountOAuthClientEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.EscapedPath() != tcAccountOauthClientsClient2F1233Fquery {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), tcAccountOauthClientsClient2F1233Fquery)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountOAuthClient(t.Context(), oauthClientIDWithSeparators)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientDeleteAccountOAuthClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
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

	err := client.DeleteAccountOAuthClient(t.Context(), oauthClientID)
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

func TestClientDeleteAccountOAuthClientDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteAccountOAuthClient(t.Context(), oauthClientID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientResetOAuthClientSecretSuccess(t *testing.T) {
	t.Parallel()

	want := linode.OAuthClientSecret{Secret: "new-secret-once"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/oauth-clients/"+oauthClientID+"/reset-secret" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/oauth-clients/"+oauthClientID+"/reset-secret")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(want); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.ResetOAuthClientSecret(t.Context(), oauthClientID)
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

func TestClientResetOAuthClientSecretEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.EscapedPath() != "/account/oauth-clients/client%2F123%3Fquery/reset-secret" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/account/oauth-clients/client%2F123%3Fquery/reset-secret")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.OAuthClientSecret{Secret: "new-secret"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ResetOAuthClientSecret(t.Context(), oauthClientIDWithSeparators)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientResetOAuthClientSecretAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/oauth-clients/"+oauthClientID+"/reset-secret" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/oauth-clients/"+oauthClientID+"/reset-secret")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ResetOAuthClientSecret(t.Context(), oauthClientID)
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

func TestClientResetOAuthClientSecretDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.ResetOAuthClientSecret(t.Context(), oauthClientID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

// TestClientCreateOAuthClientAPIError verifies API errors propagate.
func TestClientCreateOAuthClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateOAuthClientRequest{Label: oauthClientLabel, RedirectURI: oauthClientRedirectURI}

	_, err := client.CreateOAuthClient(t.Context(), req)
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

// TestClientCreateOAuthClientDoesNotRetryTransientError verifies creation is not replayed.
func TestClientCreateOAuthClientDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	req := &linode.CreateOAuthClientRequest{Label: oauthClientLabel, RedirectURI: oauthClientRedirectURI}

	_, err := client.CreateOAuthClient(t.Context(), req)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

// TestClientListAccountEventsSuccess verifies ListAccountEvents sends a GET
// request to /account/events with pagination query parameters.
func TestClientListAccountEventsSuccess(t *testing.T) {
	t.Parallel()

	duration := 300.56
	events := linode.PaginatedResponse[linode.AccountEvent]{
		Data: []linode.AccountEvent{{
			Action:   "ticket_create",
			Created:  "2018-01-01T00:01:01",
			Duration: &duration,
			Entity: &linode.AccountEventEntity{
				ID:    float64(11111),
				Label: "Problem booting my Linode",
				Type:  "ticket",
				URL:   "/v4/support/tickets/11111",
			},
			ID:      123,
			Message: "None",
			SecondaryEntity: &linode.AccountEventEntity{
				ID:    "linode/debian9",
				Label: "linode1234",
				Type:  "linode",
				URL:   "/v4/linode/instances/1234",
			},
			Seen:     true,
			Status:   "failed",
			Username: "adevi",
		}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountEvents {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountEvents)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(events); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountEvents(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != 123 {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, 123)
	}

	if result.Data[0].Action != tcTicketCreate {
		t.Errorf("result.Data[0].Action = %v, want %v", result.Data[0].Action, tcTicketCreate)
	}

	if result.Data[0].Status != "failed" {
		t.Errorf("result.Data[0].Status = %v, want %v", result.Data[0].Status, "failed")
	}

	if result.Data[0].Entity == nil {
		t.Fatal("result.Data[0].Entity is nil")
	}

	if result.Data[0].Entity.Type != "ticket" {
		t.Errorf("result.Data[0].Entity.Type = %v, want %v", result.Data[0].Entity.Type, "ticket")
	}

	if result.Data[0].Duration == nil {
		t.Fatal("result.Data[0].Duration is nil")
	}

	if math.Abs(*result.Data[0].Duration-duration) > 0.001 {
		t.Errorf("got %v, want %v", *result.Data[0].Duration, duration)
	}
}

// TestClientListAccountEventsAPIError verifies ListAccountEvents propagates API errors.
func TestClientListAccountEventsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountEvents {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountEvents)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
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

	_, err := client.ListAccountEvents(t.Context(), 0, 0)
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

// TestClientListAccountEventsRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListAccountEventsRetriesTransientError(t *testing.T) {
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

		if r.URL.Path != tcAccountEvents {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountEvents)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountEvent]{
			Data: []linode.AccountEvent{{ID: 123, Action: "ticket_create"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountEvents(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != 123 {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, 123)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

// TestClientListAccountChildAccountsSuccess verifies ListAccountChildAccounts sends a GET
// request to /account/child-accounts with pagination query parameters.
func TestClientListAccountChildAccountsSuccess(t *testing.T) {
	t.Parallel()

	childAccounts := linode.PaginatedResponse[linode.ChildAccount]{
		Data: []linode.ChildAccount{{
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
		}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountChildAccounts {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountChildAccounts)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(childAccounts); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountChildAccounts(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].EUUID != childAccountEUUID {
		t.Errorf("result.Data[0].EUUID = %v, want %v", result.Data[0].EUUID, childAccountEUUID)
	}

	if result.Data[0].Company != companyAcme {
		t.Errorf("result.Data[0].Company = %v, want %v", result.Data[0].Company, companyAcme)
	}

	if result.Data[0].CreditCard.Expiry != tcLit {
		t.Errorf("result.Data[0].CreditCard.Expiry = %v, want %v", result.Data[0].CreditCard.Expiry, tcLit)
	}

	if result.Data[0].CreditCard.LastFour != "0111" {
		t.Errorf("result.Data[0].CreditCard.LastFour = %v, want %v", result.Data[0].CreditCard.LastFour, "0111")
	}
}

// TestClientListAccountChildAccountsAPIError verifies ListAccountChildAccounts propagates API errors.
func TestClientListAccountChildAccountsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountChildAccounts {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountChildAccounts)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
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

	_, err := client.ListAccountChildAccounts(t.Context(), 0, 0)
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

// TestClientListAccountChildAccountsRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListAccountChildAccountsRetriesTransientError(t *testing.T) {
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

		if r.URL.Path != tcAccountChildAccounts {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountChildAccounts)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ChildAccount]{
			Data: []linode.ChildAccount{{EUUID: childAccountEUUID, Company: companyAcme}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountChildAccounts(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].EUUID != childAccountEUUID {
		t.Errorf("result.Data[0].EUUID = %v, want %v", result.Data[0].EUUID, childAccountEUUID)
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

// TestClientMarkAccountEventSeenSuccess verifies MarkAccountEventSeen sends a POST
// request to /account/events/{event_id}/seen with no body.
func TestClientMarkAccountEventSeenSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/events/123/seen" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/events/123/seen")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.MarkAccountEventSeen(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClientMarkAccountEventSeenAPIError verifies MarkAccountEventSeen propagates API errors.
func TestClientMarkAccountEventSeenAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/events/123/seen" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/events/123/seen")
		}

		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.MarkAccountEventSeen(t.Context(), 123)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

// TestClientMarkAccountEventSeenDoesNotRetryTransientError verifies marking an
// event as seen is not replayed after a transient failure.
func TestClientMarkAccountEventSeenDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.MarkAccountEventSeen(t.Context(), 123)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

// TestClientListAccountPaymentMethodsSuccess verifies ListAccountPaymentMethods sends a GET
// request to /account/payment-methods with pagination query parameters.
func TestClientListAccountPaymentMethodsSuccess(t *testing.T) {
	t.Parallel()

	methods := linode.PaginatedResponse[linode.AccountPaymentMethod]{
		Data: []linode.AccountPaymentMethod{{
			ID:        123,
			Type:      paymentMethodCreditCard,
			IsDefault: true,
			Data:      map[string]any{keyLastFour: "1111"},
		}},
		Page:    2,
		Pages:   4,
		Results: 80,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountPaymentMethods {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPaymentMethods)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(methods); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountPaymentMethods(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != 123 {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, 123)
	}

	if result.Data[0].Type != paymentMethodCreditCard {
		t.Errorf("result.Data[0].Type = %v, want %v", result.Data[0].Type, paymentMethodCreditCard)
	}

	if !result.Data[0].IsDefault {
		t.Error("result.Data[0].IsDefault = false, want true")
	}
}

// TestClientListAccountPaymentMethodsAPIError verifies ListAccountPaymentMethods propagates API errors.
func TestClientListAccountPaymentMethodsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountPaymentMethods {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPaymentMethods)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
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

	_, err := client.ListAccountPaymentMethods(t.Context(), 0, 0)
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

// TestClientListAccountPaymentMethodsRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListAccountPaymentMethodsRetriesTransientError(t *testing.T) {
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

		if r.URL.Path != tcAccountPaymentMethods {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPaymentMethods)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountPaymentMethod]{
			Data: []linode.AccountPaymentMethod{{ID: 123, Type: paymentMethodCreditCard, IsDefault: true}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountPaymentMethods(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != 123 {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, 123)
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

func TestClientDeleteAccountPaymentMethodSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
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

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountPaymentMethod(t.Context(), "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientDeleteAccountPaymentMethodEscapesID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/account/payment-methods/123%2F456%3Fquery" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/account/payment-methods/123%2F456%3Fquery")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountPaymentMethod(t.Context(), "123/456?query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientDeleteAccountPaymentMethodAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
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

	err := client.DeleteAccountPaymentMethod(t.Context(), "123")
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

func TestClientDeleteAccountPaymentMethodDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteAccountPaymentMethod(t.Context(), "123")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientMakeAccountPaymentMethodDefaultSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/payment-methods/123/make-default" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/payment-methods/123/make-default")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.MakeAccountPaymentMethodDefault(t.Context(), "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientMakeAccountPaymentMethodDefaultEscapesID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/account/payment-methods/123%2F456%3Fquery/make-default" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/account/payment-methods/123%2F456%3Fquery/make-default")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.MakeAccountPaymentMethodDefault(t.Context(), "123/456?query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientMakeAccountPaymentMethodDefaultAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/payment-methods/123/make-default" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/payment-methods/123/make-default")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.MakeAccountPaymentMethodDefault(t.Context(), "123")
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

func TestClientMakeAccountPaymentMethodDefaultDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.MakeAccountPaymentMethodDefault(t.Context(), "123")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientCreateAccountPaymentMethodSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.CreateAccountPaymentMethodRequest{Type: paymentMethodCreditCard, Data: map[string]any{keyToken: paymentMethodToken}, IsDefault: true}
	created := linode.AccountPaymentMethod{ID: 321, Type: paymentMethodCreditCard, IsDefault: true, Data: map[string]any{keyLastFour: "1111"}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountPaymentMethods {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPaymentMethods)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		var body map[string]any

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		if decodeErr != nil {
			t.Errorf("unexpected error: %v", decodeErr)
		}

		if decodeErr != nil {
			return
		}

		if !reflect.DeepEqual(body[keyType], paymentMethodCreditCard) {
			t.Errorf("body[keyType] = %v, want %v", body[keyType], paymentMethodCreditCard)
		}

		if !reflect.DeepEqual(body[keyIsDefault], true) {
			t.Errorf("body[keyIsDefault] = %v, want %v", body[keyIsDefault], true)
		}

		if !reflect.DeepEqual(body[keyData], map[string]any{keyToken: paymentMethodToken}) {
			t.Errorf("body[keyData] = %v, want %v", body[keyData], map[string]any{keyToken: paymentMethodToken})
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(created); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateAccountPaymentMethod(t.Context(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != 321 {
		t.Errorf("result.ID = %v, want %v", result.ID, 321)
	}

	if result.Type != paymentMethodCreditCard {
		t.Errorf("result.Type = %v, want %v", result.Type, paymentMethodCreditCard)
	}

	if !result.IsDefault {
		t.Error("result.IsDefault = false, want true")
	}
}

func TestClientCreateAccountPaymentMethodAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountPaymentMethods {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPaymentMethods)
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

	_, err := client.CreateAccountPaymentMethod(t.Context(), &linode.CreateAccountPaymentMethodRequest{Type: paymentMethodCreditCard, Data: map[string]any{keyToken: paymentMethodToken}, IsDefault: true})
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

func TestClientCreateAccountPaymentMethodDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.WriteHeader(http.StatusInternalServerError)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.CreateAccountPaymentMethod(t.Context(), &linode.CreateAccountPaymentMethodRequest{Type: paymentMethodCreditCard, Data: map[string]any{keyToken: paymentMethodToken}, IsDefault: true})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

// TestClientListAccountInvoicesSuccess verifies ListAccountInvoices sends a GET
// request to /account/invoices with pagination query parameters.
func TestClientListAccountInvoicesSuccess(t *testing.T) {
	t.Parallel()

	invoices := linode.PaginatedResponse[linode.AccountInvoice]{
		Data: []linode.AccountInvoice{{
			ID:    987,
			Date:  "2024-01-31T00:00:00",
			Label: "Invoice 987",
			Total: 42.50,
		}},
		Page:    2,
		Pages:   4,
		Results: 80,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountInvoices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountInvoices)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(invoices); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountInvoices(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != 987 {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, 987)
	}

	if result.Data[0].Label != tcInvoice987 {
		t.Errorf("result.Data[0].Label = %v, want %v", result.Data[0].Label, tcInvoice987)
	}

	if math.Abs(result.Data[0].Total-42.50) > math.Abs(42.50)*0.0001 {
		t.Errorf("got %v, want %v", result.Data[0].Total, 42.50)
	}
}

// TestClientListAccountInvoicesAPIError verifies ListAccountInvoices propagates API errors.
func TestClientListAccountInvoicesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountInvoices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountInvoices)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
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

	_, err := client.ListAccountInvoices(t.Context(), 0, 0)
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

// TestClientListAccountInvoicesRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListAccountInvoicesRetriesTransientError(t *testing.T) {
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

		if r.URL.Path != tcAccountInvoices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountInvoices)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountInvoice]{
			Data: []linode.AccountInvoice{{ID: 987, Label: "Invoice 987"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountInvoices(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != 987 {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, 987)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

// TestClientListAccountPaymentsSuccess verifies ListAccountPayments sends a GET
// request to /account/payments with pagination query parameters.
func TestClientListAccountPaymentsSuccess(t *testing.T) {
	t.Parallel()

	payments := linode.PaginatedResponse[linode.AccountPayment]{
		Data: []linode.AccountPayment{{
			ID:   654,
			Date: "2024-02-01T00:00:00",
			USD:  20.25,
		}},
		Page:    2,
		Pages:   4,
		Results: 80,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountPayments {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPayments)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(payments); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountPayments(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != 654 {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, 654)
	}

	if math.Abs(result.Data[0].USD-20.25) > math.Abs(20.25)*0.0001 {
		t.Errorf("got %v, want %v", result.Data[0].USD, 20.25)
	}
}

// TestClientListAccountPaymentsAPIError verifies ListAccountPayments propagates API errors.
func TestClientListAccountPaymentsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountPayments {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPayments)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
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

	_, err := client.ListAccountPayments(t.Context(), 0, 0)
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

// TestClientListAccountPaymentsRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListAccountPaymentsRetriesTransientError(t *testing.T) {
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

		if r.URL.Path != tcAccountPayments {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPayments)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountPayment]{
			Data: []linode.AccountPayment{{ID: 654, USD: 20.25}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountPayments(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != 654 {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, 654)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestClientGetAccountPaymentSuccess(t *testing.T) {
	t.Parallel()

	payment := linode.AccountPayment{ID: 654, Date: "2024-02-01T00:00:00", USD: 20.25}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountPayments654 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPayments654)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(payment); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountPayment(t.Context(), 654)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != 654 {
		t.Errorf("result.ID = %v, want %v", result.ID, 654)
	}

	if math.Abs(result.USD-20.25) > math.Abs(20.25)*0.0001 {
		t.Errorf("got %v, want %v", result.USD, 20.25)
	}
}

func TestClientGetAccountPaymentAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountPayments654 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPayments654)
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

	_, err := client.GetAccountPayment(t.Context(), 654)
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

func TestClientGetAccountPaymentRetriesTransientError(t *testing.T) {
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

		if r.URL.Path != tcAccountPayments654 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPayments654)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountPayment{ID: 654, USD: 20.25}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountPayment(t.Context(), 654)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != 654 {
		t.Errorf("result.ID = %v, want %v", result.ID, 654)
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

func TestClientListAccountServiceTransfersSuccess(t *testing.T) {
	t.Parallel()

	transfers := linode.PaginatedResponse[linode.AccountEntityTransfer]{
		Data: []linode.AccountEntityTransfer{{
			Created:  accountEntityTransferDate,
			Entities: linode.AccountEntityTransferEntities{Linodes: []int{111, 222}},
			Expiry:   accountEntityTransferExpiry,
			IsSender: true,
			Status:   statusPending,
			Token:    accountEntityTransferToken,
			Updated:  accountEntityTransferDate,
		}},
		Page:    2,
		Pages:   4,
		Results: 80,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountServiceTransfers {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountServiceTransfers)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(transfers); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountServiceTransfers(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Token != accountEntityTransferToken {
		t.Errorf("result.Data[0].Token = %v, want %v", result.Data[0].Token, accountEntityTransferToken)
	}

	if result.Data[0].Status != "pending" {
		t.Errorf("result.Data[0].Status = %v, want %v", result.Data[0].Status, "pending")
	}

	if !reflect.DeepEqual(result.Data[0].Entities.Linodes, []int{111, 222}) {
		t.Errorf("result.Data[0].Entities.Linodes = %v, want %v", result.Data[0].Entities.Linodes, []int{111, 222})
	}
}

func TestClientListAccountServiceTransfersAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountServiceTransfers {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountServiceTransfers)
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

	_, err := client.ListAccountServiceTransfers(t.Context(), 0, 0)
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

func TestClientListAccountServiceTransfersRetriesTransientError(t *testing.T) {
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

		if r.URL.Path != tcAccountServiceTransfers {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountServiceTransfers)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountEntityTransfer]{
			Data: []linode.AccountEntityTransfer{{Token: accountEntityTransferToken, Status: "pending"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountServiceTransfers(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Token != accountEntityTransferToken {
		t.Errorf("result.Data[0].Token = %v, want %v", result.Data[0].Token, accountEntityTransferToken)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
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

// TestClientDeleteAccountServiceTransferSuccess verifies DeleteAccountServiceTransfer sends a DELETE
// request to /account/service-transfers/{token} with no body.
func TestClientDeleteAccountServiceTransferSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
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

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountServiceTransfer(t.Context(), accountServiceTransferToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClientDeleteAccountServiceTransferEscapesToken verifies the client encodes path separators.
func TestClientDeleteAccountServiceTransferEscapesToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.EscapedPath() != "/account/service-transfers/service%2Ftoken%3Fquery" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/account/service-transfers/service%2Ftoken%3Fquery")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountServiceTransfer(t.Context(), "service/token?query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClientDeleteAccountServiceTransferAPIError verifies DeleteAccountServiceTransfer propagates API errors.
func TestClientDeleteAccountServiceTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
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

	err := client.DeleteAccountServiceTransfer(t.Context(), accountServiceTransferToken)
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

// TestClientDeleteAccountServiceTransferDoesNotRetryTransientError verifies
// service transfer cancellation is not replayed after a transient failure.
func TestClientDeleteAccountServiceTransferDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteAccountServiceTransfer(t.Context(), accountServiceTransferToken)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

// TestClientAcceptAccountServiceTransferSuccess verifies AcceptAccountServiceTransfer sends a POST
// request to /account/service-transfers/{token}/accept with no body.
func TestClientAcceptAccountServiceTransferSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/service-transfers/service-token-example/accept" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/service-transfers/service-token-example/accept")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.AcceptAccountServiceTransfer(t.Context(), accountServiceTransferToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientAcceptAccountServiceTransferEscapesToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.EscapedPath() != "/account/service-transfers/service%2Ftoken%3Fquery/accept" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/account/service-transfers/service%2Ftoken%3Fquery/accept")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.AcceptAccountServiceTransfer(t.Context(), "service/token?query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientAcceptAccountServiceTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/service-transfers/service-token-example/accept" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/service-transfers/service-token-example/accept")
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

	err := client.AcceptAccountServiceTransfer(t.Context(), accountServiceTransferToken)
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

func TestClientAcceptAccountServiceTransferDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.AcceptAccountServiceTransfer(t.Context(), accountServiceTransferToken)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
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

// TestClientGetAccountInvoiceSuccess verifies GetAccountInvoice sends a GET
// request to /account/invoices/{invoiceId} and decodes the response.
func TestClientGetAccountInvoiceSuccess(t *testing.T) {
	t.Parallel()

	invoice := linode.AccountInvoice{
		ID:    accountInvoiceID,
		Date:  "2024-01-31T00:00:00",
		Label: "Invoice #12345",
		Total: 11.00,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountInvoices12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountInvoices12345)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(invoice); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountInvoice(t.Context(), accountInvoiceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != accountInvoiceID {
		t.Errorf("result.ID = %v, want %v", result.ID, accountInvoiceID)
	}

	if result.Label != tcInvoice12345 {
		t.Errorf("result.Label = %v, want %v", result.Label, tcInvoice12345)
	}

	if math.Abs(result.Total-11.00) > 0.001 {
		t.Errorf("got %v, want %v", result.Total, 11.00)
	}
}

// TestClientGetAccountInvoiceAPIError verifies GetAccountInvoice propagates API errors.
func TestClientGetAccountInvoiceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountInvoices12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountInvoices12345)
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

	_, err := client.GetAccountInvoice(t.Context(), accountInvoiceID)
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

// TestClientListAccountUsersSuccess verifies ListAccountUsers sends a GET
// request to /account/users with pagination query parameters.
func TestClientListAccountUsersSuccess(t *testing.T) {
	t.Parallel()

	passwordCreated := accountUserPasswordCreated
	verifiedPhoneNumber := "+15555550123"
	users := linode.PaginatedResponse[linode.AccountUser]{
		Data: []linode.AccountUser{{
			Email:               accountUserEmail,
			LastLogin:           &linode.AccountUserLastLogin{LoginDatetime: accountUserLastLoginDatetime, Status: statusSuccessful},
			PasswordCreated:     &passwordCreated,
			Restricted:          true,
			SSHKeys:             []string{"ssh-rsa AAAA..."},
			TFAEnabled:          true,
			UserType:            accountUserTypeDefault,
			Username:            accountLoginUsername,
			VerifiedPhoneNumber: &verifiedPhoneNumber,
		}},
		Page:    2,
		Pages:   3,
		Results: 60,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountUsers {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountUsers)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(users); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountUsers(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Username != accountLoginUsername {
		t.Errorf("result.Data[0].Username = %v, want %v", result.Data[0].Username, accountLoginUsername)
	}

	if result.Data[0].Email != accountUserEmail {
		t.Errorf("result.Data[0].Email = %v, want %v", result.Data[0].Email, accountUserEmail)
	}

	if !result.Data[0].Restricted {
		t.Error("result.Data[0].Restricted = false, want true")
	}

	if !result.Data[0].TFAEnabled {
		t.Error("result.Data[0].TFAEnabled = false, want true")
	}
}

// TestClientListAccountUsersAPIError verifies ListAccountUsers propagates API errors.
func TestClientListAccountUsersAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountUsers {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountUsers)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountUsers(t.Context(), 0, 0)
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

// TestClientListAccountUsersRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListAccountUsersRetriesTransientError(t *testing.T) {
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

		if r.URL.Path != tcAccountUsers {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountUsers)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountUser]{
			Data: []linode.AccountUser{{Username: accountLoginUsername, Email: accountUserEmail}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountUsers(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Username != accountLoginUsername {
		t.Errorf("result.Data[0].Username = %v, want %v", result.Data[0].Username, accountLoginUsername)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
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

// TestClientListAccountLoginsSuccess verifies ListAccountLogins sends a GET
// request to /account/logins with pagination query parameters.
func TestClientListAccountLoginsSuccess(t *testing.T) {
	t.Parallel()

	logins := linode.PaginatedResponse[linode.AccountLogin]{
		Data: []linode.AccountLogin{{
			Datetime:   accountUserPasswordCreated,
			ID:         123,
			IP:         accountLoginIP,
			Restricted: false,
			Status:     statusSuccessful,
			Username:   accountLoginUsername,
		}},
		Page:    2,
		Pages:   3,
		Results: 60,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountLogins {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountLogins)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(logins); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountLogins(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != 123 {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, 123)
	}

	if result.Data[0].Username != accountLoginUsername {
		t.Errorf("result.Data[0].Username = %v, want %v", result.Data[0].Username, accountLoginUsername)
	}

	if result.Data[0].IP != accountLoginIP {
		t.Errorf("result.Data[0].IP = %v, want %v", result.Data[0].IP, accountLoginIP)
	}
}

// TestClientListAccountLoginsAPIError verifies ListAccountLogins propagates API errors.
func TestClientListAccountLoginsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountLogins {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountLogins)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
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

	_, err := client.ListAccountLogins(t.Context(), 0, 0)
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

// TestClientListAccountLoginsRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListAccountLoginsRetriesTransientError(t *testing.T) {
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

		if r.URL.Path != tcAccountLogins {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountLogins)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountLogin]{
			Data: []linode.AccountLogin{{ID: 123, Username: accountLoginUsername}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountLogins(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != 123 {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, 123)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

// TestClientGetProfileLoginSuccess verifies GetProfileLogin sends a GET
// request to /profile/logins/{loginId} and decodes the response.
func TestClientGetProfileLoginSuccess(t *testing.T) {
	t.Parallel()

	login := linode.AccountLogin{
		Datetime:   accountUserPasswordCreated,
		ID:         123,
		IP:         accountLoginIP,
		Restricted: false,
		Status:     statusSuccessful,
		Username:   accountLoginUsername,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileLogins123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileLogins123)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(login); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetProfileLogin(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != 123 {
		t.Errorf("result.ID = %v, want %v", result.ID, 123)
	}

	if result.Username != accountLoginUsername {
		t.Errorf("result.Username = %v, want %v", result.Username, accountLoginUsername)
	}

	if result.IP != accountLoginIP {
		t.Errorf("result.IP = %v, want %v", result.IP, accountLoginIP)
	}
}

// TestClientGetProfileLoginAPIError verifies GetProfileLogin propagates API errors.
func TestClientGetProfileLoginAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileLogins123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileLogins123)
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

	_, err := client.GetProfileLogin(t.Context(), 123)
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

// TestClientGetProfileLoginRetriesTransientError verifies the read-only get retries transient failures.
func TestClientGetProfileLoginRetriesTransientError(t *testing.T) {
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

		if r.URL.Path != tcProfileLogins123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileLogins123)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountLogin{ID: 123, Username: accountLoginUsername}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetProfileLogin(t.Context(), 123)
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

// TestClientGetAccountLoginSuccess verifies GetAccountLogin sends a GET
// request to /account/logins/{loginId} and decodes the response.
func TestClientGetAccountLoginSuccess(t *testing.T) {
	t.Parallel()

	login := linode.AccountLogin{
		Datetime:   accountUserPasswordCreated,
		ID:         123,
		IP:         accountLoginIP,
		Restricted: false,
		Status:     statusSuccessful,
		Username:   accountLoginUsername,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountLogins123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountLogins123)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(login); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountLogin(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != 123 {
		t.Errorf("result.ID = %v, want %v", result.ID, 123)
	}

	if result.Username != accountLoginUsername {
		t.Errorf("result.Username = %v, want %v", result.Username, accountLoginUsername)
	}

	if result.IP != accountLoginIP {
		t.Errorf("result.IP = %v, want %v", result.IP, accountLoginIP)
	}
}

// TestClientGetAccountLoginAPIError verifies GetAccountLogin propagates API errors.
func TestClientGetAccountLoginAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountLogins123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountLogins123)
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

	_, err := client.GetAccountLogin(t.Context(), 123)
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

// TestClientGetAccountLoginRetriesTransientError verifies the read-only get retries transient failures.
func TestClientGetAccountLoginRetriesTransientError(t *testing.T) {
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

		if r.URL.Path != tcAccountLogins123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountLogins123)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountLogin{ID: 123, Username: accountLoginUsername}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountLogin(t.Context(), 123)
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

// TestClientListAccountInvoiceItemsSuccess verifies ListAccountInvoiceItems sends a GET
// request to /account/invoices/{invoiceId}/items with pagination query parameters.
func TestClientListAccountInvoiceItemsSuccess(t *testing.T) {
	t.Parallel()

	items := linode.PaginatedResponse[linode.AccountInvoiceItem]{
		Data: []linode.AccountInvoiceItem{{
			Label:     "Nanode 1GB",
			Quantity:  1,
			Total:     5.00,
			Type:      "linode",
			UnitPrice: 5.00,
		}},
		Page:    2,
		Pages:   3,
		Results: 60,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountInvoices12345Items {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountInvoices12345Items)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(items); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountInvoiceItems(t.Context(), accountInvoiceID, 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Label != tcNanode1GB {
		t.Errorf("result.Data[0].Label = %v, want %v", result.Data[0].Label, tcNanode1GB)
	}

	if math.Abs(result.Data[0].Total-5.00) > 0.001 {
		t.Errorf("got %v, want %v", result.Data[0].Total, 5.00)
	}
}

// TestClientListAccountInvoiceItemsAPIError verifies ListAccountInvoiceItems propagates API errors.
func TestClientListAccountInvoiceItemsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountInvoices12345Items {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountInvoices12345Items)
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

	_, err := client.ListAccountInvoiceItems(t.Context(), accountInvoiceID, 0, 0)
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

// TestClientListAccountInvoiceItemsRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListAccountInvoiceItemsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountInvoices12345Items {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountInvoices12345Items)
		}

		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountInvoiceItem]{
			Data: []linode.AccountInvoiceItem{{Label: "Nanode 1GB", Total: 5.00}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountInvoiceItems(t.Context(), accountInvoiceID, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

// TestClientGetAccountInvoiceRetriesTransientError verifies the read-only lookup retries transient failures.
func TestClientGetAccountInvoiceRetriesTransientError(t *testing.T) {
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

		if r.URL.Path != tcAccountInvoices12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountInvoices12345)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountInvoice{ID: accountInvoiceID, Label: "Invoice #12345"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountInvoice(t.Context(), accountInvoiceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != accountInvoiceID {
		t.Errorf("result.ID = %v, want %v", result.ID, accountInvoiceID)
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

// TestClientCreateAccountServiceTransferSuccess verifies CreateAccountServiceTransfer
// sends a POST request to /account/service-transfers and decodes the response.
func TestClientCreateAccountServiceTransferSuccess(t *testing.T) {
	t.Parallel()

	want := linode.AccountEntityTransfer{
		Entities: linode.AccountEntityTransferEntities{Linodes: []int{123, 456}},
		Status:   statusPending,
		Token:    "service-transfer-token",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountServiceTransfers {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountServiceTransfers)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		var got linode.CreateAccountServiceTransferRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got.Entities.Linodes, []int{123, 456}) {
			t.Errorf("got.Entities.Linodes = %v, want %v", got.Entities.Linodes, []int{123, 456})
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(want); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateAccountServiceTransferRequest{Entities: linode.AccountEntityTransferEntities{Linodes: []int{123, 456}}}

	got, err := client.CreateAccountServiceTransfer(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Status != statusPending {
		t.Errorf("got.Status = %v, want %v", got.Status, statusPending)
	}

	if got.Token != "service-transfer-token" {
		t.Errorf("got.Token = %v, want %v", got.Token, "service-transfer-token")
	}

	if !reflect.DeepEqual(got.Entities.Linodes, []int{123, 456}) {
		t.Errorf("got.Entities.Linodes = %v, want %v", got.Entities.Linodes, []int{123, 456})
	}
}

func TestClientCreateAccountServiceTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountServiceTransfers {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountServiceTransfers)
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
	req := &linode.CreateAccountServiceTransferRequest{Entities: linode.AccountEntityTransferEntities{Linodes: []int{123}}}

	got, err := client.CreateAccountServiceTransfer(t.Context(), req)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
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

func TestClientCreateAccountServiceTransferDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	req := &linode.CreateAccountServiceTransferRequest{Entities: linode.AccountEntityTransferEntities{Linodes: []int{123}}}

	_, err := client.CreateAccountServiceTransfer(t.Context(), req)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

// TestClientCreateAccountChildAccountTokenSuccess verifies CreateAccountChildAccountToken
// sends a POST request to /account/child-accounts/{euuId}/token and decodes the response.
func TestClientCreateAccountChildAccountTokenSuccess(t *testing.T) {
	t.Parallel()

	proxyToken := linode.ProxyUserToken{
		ID:      918,
		Label:   "parent1_1234_2024-05-01T00:01:01",
		Scopes:  "*",
		Token:   "abcdefghijklmnop",
		Created: "2024-05-01T00:01:01",
		Expiry:  "2024-05-01T00:16:01",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56/token" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56/token")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(proxyToken); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateAccountChildAccountToken(t.Context(), childAccountEUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != 918 {
		t.Errorf("result.ID = %v, want %v", result.ID, 918)
	}

	if result.Label != "parent1_1234_2024-05-01T00:01:01" {
		t.Errorf("result.Label = %v, want %v", result.Label, "parent1_1234_2024-05-01T00:01:01")
	}

	if result.Scopes != "*" {
		t.Errorf("result.Scopes = %v, want %v", result.Scopes, "*")
	}

	if result.Token != "abcdefghijklmnop" {
		t.Errorf("result.Token = %v, want %v", result.Token, "abcdefghijklmnop")
	}

	if result.Expiry != "2024-05-01T00:16:01" {
		t.Errorf("result.Expiry = %v, want %v", result.Expiry, "2024-05-01T00:16:01")
	}
}

// TestClientCreateAccountChildAccountTokenEscapesEUUID verifies the client encodes path separators.
func TestClientCreateAccountChildAccountTokenEscapesEUUID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/account/child-accounts/child%2Faccount%3Fquery/token" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/account/child-accounts/child%2Faccount%3Fquery/token")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ProxyUserToken{Token: "proxy-token"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.CreateAccountChildAccountToken(t.Context(), "child/account?query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClientCreateAccountChildAccountTokenAPIError verifies API errors propagate.
func TestClientCreateAccountChildAccountTokenAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56/token" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56/token")
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

	_, err := client.CreateAccountChildAccountToken(t.Context(), childAccountEUUID)
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

// TestClientCreateAccountChildAccountTokenDoesNotRetryTransientError verifies
// token creation is not replayed after a transient failure.
func TestClientCreateAccountChildAccountTokenDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.CreateAccountChildAccountToken(t.Context(), childAccountEUUID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

// TestClientGetAccountBetaSuccess verifies GetAccountBeta sends a GET
// request to /account/betas/{betaId} and decodes the response.
func TestClientGetAccountBetaSuccess(t *testing.T) {
	t.Parallel()

	description := "This is an open public beta for an example feature."
	beta := linode.AccountBetaProgram{
		Description: &description,
		Ended:       nil,
		Enrolled:    "2023-09-11T00:00:00",
		ID:          betaExampleOpen,
		Label:       labelExampleOpenBeta,
		Started:     "2023-07-11T00:00:00",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/betas/"+betaExampleOpen {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/betas/"+betaExampleOpen)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(beta); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountBeta(t.Context(), betaExampleOpen)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != betaExampleOpen {
		t.Errorf("result.ID = %v, want %v", result.ID, betaExampleOpen)
	}

	if result.Label != labelExampleOpenBeta {
		t.Errorf("result.Label = %v, want %v", result.Label, labelExampleOpenBeta)
	}

	if result.Description == nil {
		t.Fatal("result.Description is nil")
	}

	if *result.Description != description {
		t.Errorf("*result.Description = %v, want %v", *result.Description, description)
	}
}

// TestClientGetAccountBetaEscapesID verifies the client encodes path separators.
func TestClientGetAccountBetaEscapesID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/account/betas/example%2Fopen%3Fquery" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/account/betas/example%2Fopen%3Fquery")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountBetaProgram{ID: "example/open?query"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountBeta(t.Context(), "example/open?query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClientGetAccountBetaAPIError verifies GetAccountBeta propagates API errors.
func TestClientGetAccountBetaAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/betas/"+betaExampleOpen {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/betas/"+betaExampleOpen)
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

	_, err := client.GetAccountBeta(t.Context(), betaExampleOpen)
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

// TestClientGetAccountBetaRetriesTransientError verifies the read-only lookup retries transient failures.
func TestClientGetAccountBetaRetriesTransientError(t *testing.T) {
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

		if r.URL.Path != "/account/betas/"+betaExampleOpen {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/betas/"+betaExampleOpen)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountBetaProgram{ID: betaExampleOpen, Label: labelExampleOpenBeta}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountBeta(t.Context(), betaExampleOpen)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != betaExampleOpen {
		t.Errorf("result.ID = %v, want %v", result.ID, betaExampleOpen)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

// TestClientEnrollAccountBetaSuccess verifies EnrollAccountBeta sends a POST
// request to /account/betas with the exact body.
func TestClientEnrollAccountBetaSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountBetas {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountBetas)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if r.Header.Get("Content-Type") != tcApplicationJSON {
			t.Errorf("got %v, want %v", r.Header.Get("Content-Type"), tcApplicationJSON)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["id"], betaExampleOpen) {
			t.Errorf("got %v, want %v", body["id"], betaExampleOpen)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.EnrollAccountBeta(t.Context(), &linode.EnrollAccountBetaRequest{ID: betaExampleOpen})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClientEnrollAccountBetaAPIError verifies EnrollAccountBeta propagates API errors.
func TestClientEnrollAccountBetaAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountBetas {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountBetas)
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

	err := client.EnrollAccountBeta(t.Context(), &linode.EnrollAccountBetaRequest{ID: betaExampleOpen})
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

// TestClientEnrollAccountBetaDoesNotRetry verifies the mutating beta enrollment
// is not replayed after a transient HTTP error.
func TestClientEnrollAccountBetaDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountBetas {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountBetas)
		}

		w.WriteHeader(http.StatusInternalServerError)

		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.EnrollAccountBeta(t.Context(), &linode.EnrollAccountBetaRequest{ID: betaExampleOpen})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

// TestClientAcknowledgeAccountAgreementsSuccess verifies that
// AcknowledgeAccountAgreements sends a POST request to /account/agreements with
// the exact body and returns the agreement statuses.
func TestClientAcknowledgeAccountAgreementsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountAgreements {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountAgreements)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if r.Header.Get("Content-Type") != tcApplicationJSON {
			t.Errorf("got %v, want %v", r.Header.Get("Content-Type"), tcApplicationJSON)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["billing_agreement"], true) {
			t.Errorf("got %v, want %v", body["billing_agreement"], true)
		}

		if !reflect.DeepEqual(body["eu_model"], true) {
			t.Errorf("got %v, want %v", body["eu_model"], true)
		}

		if !reflect.DeepEqual(body["master_service_agreement"], true) {
			t.Errorf("got %v, want %v", body["master_service_agreement"], true)
		}

		if _, ok := body["privacy_policy"]; ok {
			t.Errorf("body has unexpected key %v", "privacy_policy")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	billingAgreement := true
	euModel := true
	masterServiceAgreement := true

	err := client.AcknowledgeAccountAgreements(t.Context(), &linode.AcknowledgeAccountAgreementsRequest{
		BillingAgreement:       &billingAgreement,
		EUModel:                &euModel,
		MasterServiceAgreement: &masterServiceAgreement,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClientAcknowledgeAccountAgreementsDoesNotRetry verifies the mutating
// agreement acknowledgement is not replayed after a transient HTTP error.
func TestClientAcknowledgeAccountAgreementsDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountAgreements {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountAgreements)
		}

		w.WriteHeader(http.StatusInternalServerError)

		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	privacyPolicy := true

	err := client.AcknowledgeAccountAgreements(t.Context(), &linode.AcknowledgeAccountAgreementsRequest{PrivacyPolicy: &privacyPolicy})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

// TestClientCancelAccountSuccess verifies CancelAccount sends a POST request to
// /account/cancel with the exact body and returns the survey link.
func TestClientCancelAccountSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountCancel {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountCancel)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if r.Header.Get("Content-Type") != tcApplicationJSON {
			t.Errorf("got %v, want %v", r.Header.Get("Content-Type"), tcApplicationJSON)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["comments"], "moving providers") {
			t.Errorf("got %v, want %v", body["comments"], "moving providers")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, writeErr := w.Write([]byte(`{"survey_link":"https://example.test/survey"}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	comments := "moving providers"

	response, err := client.CancelAccount(t.Context(), &linode.CancelAccountRequest{Comments: &comments})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response == nil {
		t.Fatal("response is nil")
	}

	if response.SurveyLink != "https://example.test/survey" {
		t.Errorf("response.SurveyLink = %v, want %v", response.SurveyLink, "https://example.test/survey")
	}
}

// TestClientCancelAccountWithoutComments verifies comments are optional.
func TestClientCancelAccountWithoutComments(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountCancel {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountCancel)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(body) != 0 {
			t.Errorf("body = %v, want empty", body)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, writeErr := w.Write([]byte(`{"survey_link":"https://example.test/survey"}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	response, err := client.CancelAccount(t.Context(), &linode.CancelAccountRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response == nil {
		t.Fatal("response is nil")
	}

	if response.SurveyLink != "https://example.test/survey" {
		t.Errorf("response.SurveyLink = %v, want %v", response.SurveyLink, "https://example.test/survey")
	}
}

// TestClientCancelAccountDoesNotRetry verifies account cancellation is not
// replayed after a transient HTTP error.
func TestClientCancelAccountDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountCancel {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountCancel)
		}

		w.WriteHeader(http.StatusInternalServerError)

		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	errComments := "temporary"

	_, err := client.CancelAccount(t.Context(), &linode.CancelAccountRequest{Comments: &errComments})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

// TestClientUpdateAccountSuccess verifies that UpdateAccount sends a PUT
// request to /account with the exact body and returns the updated Account.
func TestClientUpdateAccountSuccess(t *testing.T) {
	t.Parallel()

	updatedAccount := linode.Account{
		FirstName: "Updated",
		LastName:  "User",
		Email:     "updated@example.com",
		Company:   "TestCo",
		Address1:  "123 Main St",
		City:      "Philadelphia",
		State:     "PA",
		Zip:       "19106",
		Country:   "US",
		Phone:     "555-0100",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/account" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account")
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if r.Header.Get("Content-Type") != tcApplicationJSON {
			t.Errorf("got %v, want %v", r.Header.Get("Content-Type"), tcApplicationJSON)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["email"], tcUpdatedExampleCom) {
			t.Errorf("got %v, want %v", body["email"], tcUpdatedExampleCom)
		}

		if !reflect.DeepEqual(body["first_name"], tcUpdated) {
			t.Errorf("got %v, want %v", body["first_name"], tcUpdated)
		}

		if !reflect.DeepEqual(body["address_1"], tcMainSt) {
			t.Errorf("got %v, want %v", body["address_1"], tcMainSt)
		}

		if _, ok := body["address_2"]; ok {
			t.Errorf("body has unexpected key %v", "address_2")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(updatedAccount); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	email := "updated@example.com"
	firstName := "Updated"
	address1 := "123 Main St"

	result, err := client.UpdateAccount(t.Context(), &linode.UpdateAccountRequest{
		Email:     &email,
		FirstName: &firstName,
		Address1:  &address1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Email != tcUpdatedExampleCom {
		t.Errorf("result.Email = %v, want %v", result.Email, tcUpdatedExampleCom)
	}

	if result.FirstName != tcUpdated {
		t.Errorf("result.FirstName = %v, want %v", result.FirstName, tcUpdated)
	}

	if result.Address1 != tcMainSt {
		t.Errorf("result.Address1 = %v, want %v", result.Address1, tcMainSt)
	}
}

// TestClientUpdateAccountNetworkError verifies that UpdateAccount returns a
// NetworkError when the HTTP request fails to reach the server.
func TestClientUpdateAccountNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateAccount(t.Context(), &linode.UpdateAccountRequest{})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Errorf("error = %v, want %v", err, &netErr)
	}
}

// TestClientUpdateAccountAPIError verifies that UpdateAccount propagates
// API errors through the handleResponse error chain.
func TestClientUpdateAccountAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"errors":[{"field":"email","reason":"invalid email format"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateAccount(t.Context(), &linode.UpdateAccountRequest{})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusBadRequest)
	}
}

// TestClientUpdateAccountDoesNotRetry verifies the mutating account update is
// not replayed after a transient HTTP error.
func TestClientUpdateAccountDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/account" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account")
		}

		w.WriteHeader(http.StatusInternalServerError)

		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateAccount(t.Context(), &linode.UpdateAccountRequest{})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

// TestClientEnableAccountManagedSuccess verifies that EnableAccountManaged sends a POST
// request to /account/settings/managed-enable with no query parameters or body.
func TestClientEnableAccountManagedSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountSettingsManagedEnable {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountSettingsManagedEnable)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if r.Header.Get("Content-Type") != tcApplicationJSON {
			t.Errorf("got %v, want %v", r.Header.Get("Content-Type"), tcApplicationJSON)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(body) != 0 {
			t.Errorf("body = %v, want empty", body)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.EnableAccountManaged(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClientEnableAccountManagedNetworkError verifies that EnableAccountManaged returns a
// NetworkError when the HTTP request fails to reach the server.
func TestClientEnableAccountManagedNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(3))

	err := client.EnableAccountManaged(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Errorf("error = %v, want %v", err, &netErr)
	}
}

// TestClientEnableAccountManagedAPIError verifies that EnableAccountManaged propagates
// API errors through the handleResponse error chain.
func TestClientEnableAccountManagedAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountSettingsManagedEnable {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountSettingsManagedEnable)
		}

		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"errors":[{"reason":"managed could not be enabled"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.EnableAccountManaged(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusBadRequest)
	}
}

// TestClientEnableAccountManagedDoesNotRetry verifies the mutating managed enable
// request is not replayed after a transient HTTP error.
func TestClientEnableAccountManagedDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountSettingsManagedEnable {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountSettingsManagedEnable)
		}

		w.WriteHeader(http.StatusInternalServerError)

		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.EnableAccountManaged(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

// TestClientUpdateAccountSettingsSuccess verifies that UpdateAccountSettings sends a PUT
// request to /account/settings with the exact body and returns the updated settings.
func TestClientUpdateAccountSettingsSuccess(t *testing.T) {
	t.Parallel()

	longview := "longview-3"
	objectStorage := "active"
	settings := linode.AccountSettings{
		BackupsEnabled:          true,
		Managed:                 true,
		NetworkHelper:           false,
		LongviewSubscription:    &longview,
		ObjectStorage:           &objectStorage,
		InterfacesForNewLinodes: "legacy_config_default_but_linode_allowed",
		MaintenancePolicy:       maintenancePolicyMigrate,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
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

		if r.Header.Get("Content-Type") != tcApplicationJSON {
			t.Errorf("got %v, want %v", r.Header.Get("Content-Type"), tcApplicationJSON)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["backups_enabled"], true) {
			t.Errorf("got %v, want %v", body["backups_enabled"], true)
		}

		if !reflect.DeepEqual(body["network_helper"], false) {
			t.Errorf("got %v, want %v", body["network_helper"], false)
		}

		if !reflect.DeepEqual(body["maintenance_policy"], maintenancePolicyMigrate) {
			t.Errorf("got %v, want %v", body["maintenance_policy"], maintenancePolicyMigrate)
		}

		if _, ok := body["object_storage"]; ok {
			t.Errorf("body has unexpected key %v", "object_storage")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(settings); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	backupsEnabled := true

	var networkHelper bool

	maintenancePolicy := maintenancePolicyMigrate

	result, err := client.UpdateAccountSettings(t.Context(), &linode.UpdateAccountSettingsRequest{
		BackupsEnabled:    &backupsEnabled,
		NetworkHelper:     &networkHelper,
		MaintenancePolicy: &maintenancePolicy,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.BackupsEnabled {
		t.Error("result.BackupsEnabled = false, want true")
	}

	if result.NetworkHelper {
		t.Error("result.NetworkHelper = true, want false")
	}

	if result.MaintenancePolicy != maintenancePolicyMigrate {
		t.Errorf("result.MaintenancePolicy = %v, want %v", result.MaintenancePolicy, maintenancePolicyMigrate)
	}
}

// TestClientUpdateAccountSettingsAPIError verifies that UpdateAccountSettings propagates
// API errors through the handleResponse error chain.
func TestClientUpdateAccountSettingsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcAccountSettings {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountSettings)
		}

		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"errors":[{"field":"maintenance_policy","reason":"invalid maintenance policy"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateAccountSettings(t.Context(), &linode.UpdateAccountSettingsRequest{})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusBadRequest)
	}
}

// TestClientUpdateAccountSettingsDoesNotRetry verifies the mutating account settings
// update is not replayed after a transient HTTP error.
func TestClientUpdateAccountSettingsDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcAccountSettings {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountSettings)
		}

		w.WriteHeader(http.StatusInternalServerError)

		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateAccountSettings(t.Context(), &linode.UpdateAccountSettingsRequest{})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

func TestClientListImageShareGroupTokensSuccess(t *testing.T) {
	t.Parallel()

	updated := "2025-08-04T11:09:09"
	expiry := "2025-09-04T10:09:09"
	tokens := []linode.ImageShareGroupToken{
		{
			TokenUUID:              "test-token-uuid",
			Status:                 oauthClientStatus,
			Label:                  "Backend Services - Engineering",
			Created:                imageShareGroupTokenCreated,
			Updated:                &updated,
			Expiry:                 &expiry,
			ValidForShareGroupUUID: "e1d0e58b-f89f-4237-84ab-b82077342359",
			ShareGroupUUID:         "e1d0e58b-f89f-4237-84ab-b82077342359",
			ShareGroupLabel:        shareGroupLabelFixture,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/images/sharegroups/tokens" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/tokens")
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+"test-token" {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+"test-token")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:   tokens,
			"page":    2,
			"pages":   3,
			"results": 7,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListImageShareGroupTokens(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Label != imageShareGroupTokenUpdateLabel {
		t.Errorf("result.Data[0].Label = %v, want %v", result.Data[0].Label, imageShareGroupTokenUpdateLabel)
	}

	if result.Data[0].TokenUUID != "test-token-uuid" {
		t.Errorf("result.Data[0].TokenUUID = %v, want %v", result.Data[0].TokenUUID, "test-token-uuid")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if result.Results != 7 {
		t.Errorf("result.Results = %v, want %v", result.Results, 7)
	}
}

func TestClientListImageShareGroupTokensError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListImageShareGroupTokens(t.Context(), 1, 25)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

func TestClientListImageShareGroupsSuccess(t *testing.T) {
	t.Parallel()

	description := shareGroupDescriptionFixture
	updated := shareGroupUpdatedFixture
	shareGroups := []linode.ImageShareGroup{
		{
			ID:           1,
			UUID:         shareGroupUUIDExample,
			Label:        imageShareGroupLabel,
			Description:  &description,
			IsSuspended:  false,
			Created:      shareGroupCreatedFixture,
			Updated:      &updated,
			ImagesCount:  2,
			MembersCount: 3,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/images/sharegroups" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups")
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+"test-token" {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+"test-token")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			"data":    shareGroups,
			"page":    2,
			"pages":   3,
			"results": 7,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListImageShareGroups(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Label != imageShareGroupLabel {
		t.Errorf("result.Data[0].Label = %v, want %v", result.Data[0].Label, imageShareGroupLabel)
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if result.Results != 7 {
		t.Errorf("result.Results = %v, want %v", result.Results, 7)
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

func TestClientCreateImageShareGroupSuccess(t *testing.T) {
	t.Parallel()

	description := shareGroupDescriptionFixture
	updated := shareGroupUpdatedFixture
	request := &linode.CreateImageShareGroupRequest{
		Label:       imageShareGroupLabel,
		Description: description,
		Images: []linode.ImageShareGroupImage{
			{ID: "private/7", Label: "Linux Debian", Description: "Official Debian Linux image"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/images/sharegroups" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+"test-token" {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+"test-token")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if !reflect.DeepEqual(body[keyLabel], imageShareGroupLabel) {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], imageShareGroupLabel)
		}

		if !reflect.DeepEqual(body[keyDescription], description) {
			t.Errorf("body[keyDescription] = %v, want %v", body[keyDescription], description)
		}

		images, isList := body["images"].([]any)
		if !isList || len(images) != 1 {
			t.Errorf(`body["images"] = %v, want one element`, body["images"])

			return
		}

		image, ok := images[0].(map[string]any)
		if !ok {
			t.Error("image payload should be an object")

			return
		}

		if !reflect.DeepEqual(image[keyID], "private/7") {
			t.Errorf("image[keyID] = %v, want %v", image[keyID], "private/7")
		}

		if !reflect.DeepEqual(image["label"], "Linux Debian") {
			t.Errorf("got %v, want %v", image["label"], "Linux Debian")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ImageShareGroup{
			ID:           1,
			UUID:         shareGroupUUIDExample,
			Label:        imageShareGroupLabel,
			Description:  &description,
			IsSuspended:  false,
			Created:      shareGroupCreatedFixture,
			Updated:      &updated,
			ImagesCount:  1,
			MembersCount: 0,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateImageShareGroup(t.Context(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Label != imageShareGroupLabel {
		t.Errorf("result.Label = %v, want %v", result.Label, imageShareGroupLabel)
	}

	if result.ImagesCount != 1 {
		t.Errorf("result.ImagesCount = %v, want %v", result.ImagesCount, 1)
	}
}

func TestClientCreateImageShareGroupAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "label is required"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.CreateImageShareGroup(t.Context(), &linode.CreateImageShareGroupRequest{Label: imageShareGroupLabel})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.Message != linode.ErrLabelRequired.Error() {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, linode.ErrLabelRequired.Error())
	}
}

func TestClientCreateImageShareGroupNetworkError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := srv.URL
	srv.Close()

	client := linode.NewClient(baseURL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.CreateImageShareGroup(t.Context(), &linode.CreateImageShareGroupRequest{Label: imageShareGroupLabel})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	networkErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &networkErr)
	}

	if networkErr.Operation != "CreateImageShareGroup" {
		t.Errorf("networkErr.Operation = %v, want %v", networkErr.Operation, "CreateImageShareGroup")
	}
}

func TestClientCreateImageShareGroupDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	_, err := client.CreateImageShareGroup(t.Context(), &linode.CreateImageShareGroupRequest{Label: imageShareGroupLabel})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

const (
	uploadImageLabelFixture  = "custom-image"
	uploadImageTagProd       = "prod"
	uploadImageTagWeb        = "web"
	uploadImageStatusFixture = "creating"
	uploadImageTargetFixture = "https://uploads.example.test/custom-image"
)

func TestClientUploadImageSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.UploadImageRequest{
		Label:       uploadImageLabelFixture,
		Region:      regionUSEast,
		Description: "custom upload",
		CloudInit:   true,
		Tags:        []string{uploadImageTagProd, uploadImageTagWeb},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/images/upload" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/upload")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+"test-token" {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+"test-token")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body should decode: %v", err)

			return
		}

		if !reflect.DeepEqual(body[keyLabel], uploadImageLabelFixture) {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], uploadImageLabelFixture)
		}

		if !reflect.DeepEqual(body["region"], regionUSEast) {
			t.Errorf("got %v, want %v", body["region"], regionUSEast)
		}

		if !reflect.DeepEqual(body[keyDescription], "custom upload") {
			t.Errorf("body[keyDescription] = %v, want %v", body[keyDescription], "custom upload")
		}

		if !reflect.DeepEqual(body["cloud_init"], true) {
			t.Errorf("got %v, want %v", body["cloud_init"], true)
		}

		if !reflect.DeepEqual(body["tags"], []any{uploadImageTagProd, uploadImageTagWeb}) {
			t.Errorf("got %v, want %v", body["tags"], []any{uploadImageTagProd, uploadImageTagWeb})
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			"image": linode.Image{
				ID:          "private/99",
				Label:       uploadImageLabelFixture,
				Description: "custom upload",
				Status:      uploadImageStatusFixture,
				Tags:        []string{uploadImageTagProd, uploadImageTagWeb},
			},
			"upload_to": uploadImageTargetFixture,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.UploadImage(t.Context(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Image.ID != "private/99" {
		t.Errorf("result.Image.ID = %v, want %v", result.Image.ID, "private/99")
	}

	if result.Image.Label != uploadImageLabelFixture {
		t.Errorf("result.Image.Label = %v, want %v", result.Image.Label, uploadImageLabelFixture)
	}

	if result.UploadTo != uploadImageTargetFixture {
		t.Errorf("result.UploadTo = %v, want %v", result.UploadTo, uploadImageTargetFixture)
	}
}

func TestClientUploadImageAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "region is required"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.UploadImage(t.Context(), &linode.UploadImageRequest{Label: uploadImageLabelFixture})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.Message != linode.ErrRegionRequired.Error() {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, linode.ErrRegionRequired.Error())
	}
}

func TestClientUploadImageNetworkError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := srv.URL
	srv.Close()

	client := linode.NewClient(baseURL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.UploadImage(t.Context(), &linode.UploadImageRequest{Label: uploadImageLabelFixture, Region: regionUSEast})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	networkErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &networkErr)
	}

	if networkErr.Operation != "UploadImage" {
		t.Errorf("networkErr.Operation = %v, want %v", networkErr.Operation, "UploadImage")
	}
}

func TestClientUploadImageDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	_, err := client.UploadImage(t.Context(), &linode.UploadImageRequest{Label: uploadImageLabelFixture, Region: regionUSEast})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

// TestClientListObjectStorageQuotasSuccess verifies that ListObjectStorageQuotas sends a
// GET request to /object-storage/quotas with no query parameters or body.
func TestClientListObjectStorageQuotasSuccess(t *testing.T) {
	t.Parallel()

	const (
		quotaIDKey = "quota_id"
		quotaID    = "endpoint-type-1"
	)

	quotas := []linode.ObjectStorageQuota{
		{keyID: quotaID, quotaIDKey: quotaID, "s3_endpoint": "us-east-1.linodeobjects.com"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/object-storage/quotas" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/quotas")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if r.Header.Get("Content-Type") != tcApplicationJSON {
			t.Errorf("got %v, want %v", r.Header.Get("Content-Type"), tcApplicationJSON)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(body) != 0 {
			t.Errorf("body = %v, want empty", body)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    quotas,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	got, err := client.ListObjectStorageQuotas(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want %d", len(got), 1)
	}

	if !reflect.DeepEqual(got[0][keyID], quotaID) {
		t.Errorf("got[0][keyID] = %v, want %v", got[0][keyID], quotaID)
	}
}

// TestClientListObjectStorageQuotasRetriesReadOnlyGET verifies the read-only quotas
// route can retry a transient server error without replaying a mutating request.
func TestClientListObjectStorageQuotasRetriesReadOnlyGET(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/object-storage/quotas" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/quotas")
		}

		if calls.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    []linode.ObjectStorageQuota{{keyID: "endpoint-type-1"}},
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	got, err := client.ListObjectStorageQuotas(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want %d", len(got), 1)
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}
}
