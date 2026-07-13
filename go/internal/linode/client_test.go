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

	result, err := client.ListObjectStorageBucketsByRegionProto(t.Context(), "us-east-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want %d", len(result), 1)
	}

	if result[0].GetLabel() != "my-bucket" {
		t.Errorf("result[0].GetLabel() = %v, want %v", result[0].GetLabel(), "my-bucket")
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

	_, err := client.ListObjectStorageBucketsByRegionProto(t.Context(), "us/east?1")
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
