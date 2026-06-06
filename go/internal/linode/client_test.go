package linode_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
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
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/profile/tokens", r.URL.Path, "request path should be /profile/tokens")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got linode.CreateProfileTokenRequest
		checkNoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should be valid JSON")
		checkEqual(t, profileTokenExpiryFixture, got.Expiry)
		checkEqual(t, profileTokenLabelFixture, got.Label)
		checkEqual(t, profileTokenScopesFixture, got.Scopes)

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(profileToken))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.CreateProfileToken(t.Context(), linode.CreateProfileTokenRequest{Expiry: profileTokenExpiryFixture, Label: profileTokenLabelFixture, Scopes: profileTokenScopesFixture})

	mustNoError(t, err, "CreateProfileToken should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkInEpsilon(t, float64(321), (*result)[keyID], 0.001)
	checkEqual(t, profileTokenLabelFixture, (*result)[keyLabel])
	checkEqual(t, profileTokenScopesFixture, (*result)[profileTokenScopesKey])
	checkEqual(t, profileTokenSecretFixture, (*result)[keyToken])
}

// TestClientCreateProfileTokenAPIError verifies API errors propagate.
func TestClientCreateProfileTokenAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/profile/tokens", r.URL.Path, "request path should be /profile/tokens")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	_, err := client.CreateProfileToken(t.Context(), linode.CreateProfileTokenRequest{Label: profileTokenLabelFixture})

	mustError(t, err, "CreateProfileToken should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	_, err := client.CreateProfileToken(t.Context(), linode.CreateProfileTokenRequest{Label: profileTokenLabelFixture})

	mustError(t, err, "CreateProfileToken should return the transient error")
	checkEqual(t, int32(1), requestCount.Load(), "personal access token creation must not be retried")
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
		checkEqual(t, "/profile", r.URL.Path, "request path should be /profile")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(profile), "encoding profile response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetProfile(t.Context())

	mustNoError(t, err, "GetProfile should succeed on 200 response")
	checkEqual(t, "testuser", result.Username, "username should match the API response")
	checkEqual(t, "test@example.com", result.Email, "email should match the API response")
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
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(profile))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "tok", nil, linode.WithMaxRetries(0))
	result, err := client.GetProfile(t.Context())

	mustNoError(t, err)
	checkEqual(t, "linodes:read_write domains:read_only", result.Scopes,
		"PAT scopes from /profile must round-trip into Profile.Scopes")
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
		checkEqual(t, "/profile/grants", r.URL.Path,
			"request path should be /profile/grants")
		checkEqual(t, "Bearer oauth-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "oauth-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetProfileGrants(t.Context())

	mustNoError(t, err, "GetProfileGrants should succeed on 200 response")
	mustNotNil(t, got)
	checkEqual(t, linode.GrantPermission("read_write"), got.Global.AccountAccess,
		"global account_access must round-trip")
	checkTrue(t, got.Global.AddLinodes,
		"global add_linodes must round-trip")
	mustLen(t, got.Linode, 1)
	checkEqual(t, "web-1", got.Linode[0].Label)
	checkEqual(t, linode.GrantPermission("read_write"), got.Linode[0].Permissions)
	mustLen(t, got.Domain, 1)
	checkEqual(t, linode.GrantPermission("read_only"), got.Domain[0].Permissions)
}

func TestClientGetProfilePreferencesSuccess(t *testing.T) {
	t.Parallel()

	preferences := linode.ProfilePreferences{
		"desktop_notifications": true,
		"sort_order":            "ascending",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/preferences", r.URL.Path, "request path should match profile preferences route")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, int64(0), r.ContentLength, "GET request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(preferences), "encoding preferences response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	got, err := client.GetProfilePreferences(t.Context())

	mustNoError(t, err, "GetProfilePreferences should succeed on 200 response")
	mustNotNil(t, got)
	checkEqual(t, preferences, *got)
}

func TestClientGetProfilePreferencesUnauthorized(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/profile/preferences", r.URL.Path, "request path should match profile preferences route")
		w.WriteHeader(http.StatusUnauthorized)
		_, err := w.Write([]byte(`{}`))
		checkNoError(t, err, "writing error response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "bad-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetProfilePreferences(t.Context())

	mustError(t, err, "GetProfilePreferences should fail on 401 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "GetProfilePreferences must return APIError on 401")
	checkEqual(t, http.StatusUnauthorized, apiErr.StatusCode)
}

// TestClientGetProfileGrantsPATEmpty verifies that a PAT (which doesn't use
// OAuth grants) returning an empty grants payload still parses cleanly.
// The Linode API returns 200 with zero-valued fields for this case; the
// Phase 6 loader detects "use PAT scopes path" by checking
// Profile.Scopes != "" before consulting Grants.
func TestClientGetProfileGrantsPATEmpty(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.Grants{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "pat-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetProfileGrants(t.Context())

	mustNoError(t, err, "empty grants response must parse cleanly")
	mustNotNil(t, got)
	checkEmpty(t, got.Linode, "no Linode grants expected on PAT path")
	checkEmpty(t, got.Global.AccountAccess, "no global permission expected")
}

// TestClientGetProfileGrantsUnauthorized confirms 401 propagates as an
// APIError from GetProfileGrants, matching the GetProfile contract so
// Phase 6's loader can use the same error path for both calls.
func TestClientGetProfileGrantsUnauthorized(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"reason": "Invalid Token"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "bad-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetProfileGrants(t.Context())

	mustError(t, err)

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr,
		"GetProfileGrants must return APIError on 401")
	checkEqual(t, http.StatusUnauthorized, apiErr.StatusCode)
}

// TestClientGetProfileUnauthorized verifies that GetProfile returns an
// APIError with status 401 when the API rejects the token.
func TestClientGetProfileUnauthorized(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"reason": "Invalid Token"}},
		}), "encoding error response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "bad-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetProfile(t.Context())

	mustError(t, err, "GetProfile should fail on 401 response")

	var apiErr *linode.APIError

	mustErrorAs(t, err, &apiErr, "error should be an APIError")
	checkEqual(t, 401, apiErr.StatusCode, "status code should be 401 unauthorized")
}

func TestClientGetProfileAppSuccess(t *testing.T) {
	t.Parallel()

	want := linode.ProfileApp{ID: 12345, Label: "Example OAuth App", Scopes: profileAppScopesReadOnly, Website: "https://example.com"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/apps/12345", r.URL.Path, "request path should include app id")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetProfileApp(t.Context(), want.ID)

	mustNoError(t, err, "GetProfileApp should succeed on 200 response")
	mustNotNil(t, got, "result should not be nil")
	checkEqual(t, want, *got)
}

func TestClientGetProfileAppBuildsNumericPath(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/profile/apps/12345", r.URL.EscapedPath(), "request path should include the numeric app id segment")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ProfileApp{ID: 12345}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileApp(t.Context(), 12345)

	mustNoError(t, err, "GetProfileApp should build the numeric app path")
}

func TestClientGetProfileAppAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/apps/12345", r.URL.Path, "request path should include app id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileApp(t.Context(), 12345)

	mustError(t, err, "GetProfileApp should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
}

func TestClientGetProfileDeviceSuccess(t *testing.T) {
	t.Parallel()

	want := linode.ProfileDevice{keyID: float64(12345), keyUserAgent: profileDeviceUserAgent, keyLastRemoteAddr: profileDeviceRemoteAddr, keyLastAuthenticated: accountUserPasswordCreated}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "method should be GET")
		checkEqual(t, "/profile/devices/12345", r.URL.Path, "path should target trusted device")
		checkEmpty(t, r.URL.RawQuery, "query should be empty")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetProfileDevice(t.Context(), 12345)

	mustNoError(t, err, "GetProfileDevice should succeed on 200 response")
	mustNotNil(t, got, "result should not be nil")
	checkEqual(t, want, *got)
}

func TestClientGetProfileDeviceBuildsNumericPath(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/profile/devices/12345", r.URL.Path, "path should include escaped device id")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ProfileDevice{keyID: float64(12345)}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileDevice(t.Context(), 12345)

	mustNoError(t, err, "GetProfileDevice should build the numeric device path")
}

func TestClientGetProfileDeviceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "method should be GET")
		checkEqual(t, "/profile/devices/12345", r.URL.Path, "path should target trusted device")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileDevice(t.Context(), 12345)

	mustError(t, err, "GetProfileDevice should fail on 403 response")

	var apiErr *linode.APIError
	if !errorAsValue(err, &apiErr) {
		t.Fatalf("error should wrap APIError: %v", err)
	}

	checkContains(t, apiErr.Message, errForbidden)
}

func TestClientGetProfileDeviceRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requestCount.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ProfileDevice{keyID: float64(12345), keyUserAgent: profileDeviceUserAgent}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.GetProfileDevice(t.Context(), 12345)

	mustNoError(t, err, "GetProfileDevice should succeed after retry")
	mustNotNil(t, got, "result should not be nil")
	checkInEpsilon(t, float64(12345), (*got)[keyID], 0)
	checkEqual(t, int32(2), requestCount.Load(), "read-only GET should retry once")
}

func TestClientGetProfileAppRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/apps/12345", r.URL.Path, "request path should include app id")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ProfileApp{ID: 12345, Label: "Example OAuth App"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.GetProfileApp(t.Context(), 12345)

	mustNoError(t, err, "GetProfileApp should succeed after retry")
	mustNotNil(t, got, "result should not be nil")
	checkEqual(t, 12345, got.ID)
	checkEqual(t, int32(2), requestCount.Load(), "read-only get should retry once then succeed")
}

func TestClientEnableProfileTFASuccess(t *testing.T) {
	t.Parallel()

	response := linode.ProfileTFAEnableResponse{
		"secret": "JBSWY3DPEHPK3PXP",
		"expiry": "2026-01-01T00:00:00",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/profile/tfa-enable", r.URL.Path, "request path should be /profile/tfa-enable")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "authorization header should use bearer token")

		body, readErr := io.ReadAll(r.Body)
		if !checkNoError(t, readErr, "request body should be readable") {
			return
		}

		checkEmpty(t, string(body), "request body should be empty")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.EnableProfileTFA(t.Context())

	mustNoError(t, err, "EnableProfileTFA should succeed on 200 response")
	checkEqual(t, "JBSWY3DPEHPK3PXP", result["secret"], "secret should match the API response")
	checkEqual(t, "2026-01-01T00:00:00", result["expiry"], "expiry should match the API response")
}

func TestClientEnableProfileTFANoRetryOnTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/profile/tfa-enable", r.URL.Path, "request path should be /profile/tfa-enable")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))
	result, err := client.EnableProfileTFA(t.Context())

	mustError(t, err, "EnableProfileTFA should surface the transient API error")
	checkNil(t, result, "result should be nil on failure")
	checkEqual(t, int32(1), calls.Load(), "non-idempotent TFA secret generation must not be retried")
}

func TestClientSendProfilePhoneNumberVerificationCodeSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/profile/phone-number", r.URL.Path, "request path should be /profile/phone-number")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		var body map[string]string
		if !checkNoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should be JSON") {
			return
		}

		checkEqual(t, profilePhoneISOCode, body["iso_code"])
		checkEqual(t, profilePhoneNumber, body["phone_number"])

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.SendProfilePhoneNumberVerificationCode(t.Context(), &linode.ProfilePhoneNumberRequest{
		ISOCode:     profilePhoneISOCode,
		PhoneNumber: profilePhoneNumber,
	})

	mustNoError(t, err, "SendProfilePhoneNumberVerificationCode should succeed on 200 response")
}

func TestClientSendProfilePhoneNumberVerificationCodeAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/profile/phone-number", r.URL.Path, "request path should be /profile/phone-number")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.SendProfilePhoneNumberVerificationCode(t.Context(), &linode.ProfilePhoneNumberRequest{
		ISOCode:     profilePhoneISOCode,
		PhoneNumber: profilePhoneNumber,
	})

	mustError(t, err, "SendProfilePhoneNumberVerificationCode should fail on server error")
	checkEqual(t, int32(1), requestCount.Load(), "non-idempotent POST must not be retried")
}

func TestClientDeleteProfilePhoneNumberSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/profile/phone-number", r.URL.Path, "request path should be /profile/phone-number")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteProfilePhoneNumber(t.Context())

	mustNoError(t, err, "DeleteProfilePhoneNumber should succeed on 200 response")
}

func TestClientDeleteProfilePhoneNumberAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/profile/phone-number", r.URL.Path, "request path should be /profile/phone-number")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteProfilePhoneNumber(t.Context())

	mustError(t, err, "DeleteProfilePhoneNumber should fail on server error")
	checkEqual(t, int32(1), requestCount.Load(), "destructive DELETE must not be retried")
}

func TestClientVerifyProfilePhoneNumberSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/profile/phone-number/verify", r.URL.Path, "request path should be /profile/phone-number/verify")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		var body map[string]string
		if !checkNoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should be JSON") {
			return
		}

		checkEqual(t, profilePhoneOTPCode, body["otp_code"])

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.VerifyProfilePhoneNumber(t.Context(), &linode.ProfilePhoneNumberVerifyRequest{OTPCode: profilePhoneOTPCode})

	mustNoError(t, err, "VerifyProfilePhoneNumber should succeed on 200 response")
}

func TestClientVerifyProfilePhoneNumberAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/profile/phone-number/verify", r.URL.Path, "request path should be /profile/phone-number/verify")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.VerifyProfilePhoneNumber(t.Context(), &linode.ProfilePhoneNumberVerifyRequest{OTPCode: profilePhoneOTPCode})

	mustError(t, err, "VerifyProfilePhoneNumber should fail on server error")
	checkEqual(t, int32(1), requestCount.Load(), "non-idempotent POST must not be retried")
}

func TestClientDeleteProfileAppSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/profile/apps/12345", r.URL.Path, "request path should include app id")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteProfileApp(t.Context(), 12345)

	mustNoError(t, err, "DeleteProfileApp should succeed on 200 response")
}

func TestClientDeleteProfileAppAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/profile/apps/12345", r.URL.Path, "request path should include app id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteProfileApp(t.Context(), 12345)

	mustError(t, err, "DeleteProfileApp should fail on server error")
	checkEqual(t, int32(1), requestCount.Load(), "destructive delete must not be retried")
}

func TestClientDeleteProfileDeviceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/profile/devices/67890", r.URL.Path, "request path should include device id")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteProfileDevice(t.Context(), 67890)

	mustNoError(t, err, "DeleteProfileDevice should succeed on 200 response")
}

func TestClientDeleteProfileDeviceAPIErrorDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/profile/devices/67890", r.URL.Path, "request path should include device id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteProfileDevice(t.Context(), 67890)

	mustError(t, err, "DeleteProfileDevice should fail on server error")
	checkEqual(t, int32(1), requestCount.Load(), "destructive delete must not be retried")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, accountMaintenancePath, r.URL.Path, "request path should be /account/maintenance")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(maintenance))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.ListAccountMaintenance(t.Context(), 2, 25)

	mustNoError(t, err, "ListAccountMaintenance should succeed on 200 response")
	mustNotNil(t, result)
	mustLen(t, result.Data, 1)
	checkEqual(t, accountMaintenanceLabel, result.Data[0].Entity.Label)
	checkEqual(t, "reboot", result.Data[0].Type)
}

func TestClientListAccountMaintenanceRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, accountMaintenancePath, r.URL.Path, "request path should be /account/maintenance")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountMaintenance]{
			Data:    []linode.AccountMaintenance{{Status: statusPending}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	result, err := client.ListAccountMaintenance(t.Context(), 0, 0)

	mustNoError(t, err, "read-only maintenance list should retry transient failures")
	mustNotNil(t, result)
	checkEqual(t, int32(2), calls.Load(), "client should retry once")
}

func TestClientListAccountMaintenanceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, accountMaintenancePath, r.URL.Path, "request path should be /account/maintenance")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "Forbidden"}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListAccountMaintenance(t.Context(), 0, 0)

	mustError(t, err, "ListAccountMaintenance should fail on API errors")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, maintenancePoliciesPath, r.URL.Path, "request path should be /maintenance/policies")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, http.NoBody, r.Body, "request body should be empty")
		checkEqual(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(policies))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.ListMaintenancePolicies(t.Context(), 2, 25)

	mustNoError(t, err, "ListMaintenancePolicies should succeed on 200 response")
	mustNotNil(t, result)
	mustLen(t, result.Data, 1)
	checkEqual(t, maintenancePolicySlug, result.Data[0].Slug)
	checkEqual(t, maintenancePolicyLabel, result.Data[0].Label)
	checkTrue(t, result.Data[0].IsDefault)
}

func TestClientListMaintenancePoliciesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, maintenancePoliciesPath, r.URL.Path, "request path should be /maintenance/policies")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.MaintenancePolicy]{
			Data:    []linode.MaintenancePolicy{{Slug: maintenancePolicySlug}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	result, err := client.ListMaintenancePolicies(t.Context(), 0, 0)

	mustNoError(t, err, "read-only maintenance policies list should retry transient failures")
	mustNotNil(t, result)
	checkEqual(t, int32(2), calls.Load(), "client should retry once")
}

func TestClientListMaintenancePoliciesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, maintenancePoliciesPath, r.URL.Path, "request path should be /maintenance/policies")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "Forbidden"}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListMaintenancePolicies(t.Context(), 0, 0)

	mustError(t, err, "ListMaintenancePolicies should fail on API errors")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
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
		checkEqual(t, "/linode/instances", r.URL.Path, "request path should be /linode/instances")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    instances,
			keyPage:    1,
			keyPages:   1,
			keyResults: 2,
		}), "encoding instances response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.ListInstances(t.Context())

	mustNoError(t, err, "ListInstances should succeed on 200 response")
	checkLen(t, result, 2, "should return both instances")
	checkEqual(t, "web-1", result[0].Label, "first instance label should match")
}

// TestClientGetInstanceSuccess verifies that GetInstance returns the correct
// instance when given a valid ID.
func TestClientGetInstanceSuccess(t *testing.T) {
	t.Parallel()

	instance := linode.Instance{ID: 42, Label: "my-instance", Status: "running"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/linode/instances/42", r.URL.Path, "request path should include the instance ID")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(instance), "encoding instance response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.GetInstance(t.Context(), 42)

	mustNoError(t, err, "GetInstance should succeed on 200 response")
	checkEqual(t, 42, result.ID, "instance ID should match the request")
	checkEqual(t, "my-instance", result.Label, "instance label should match the API response")
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

	mustError(t, err, "GetInstance should fail on 500 response")

	var apiErr *linode.APIError

	mustErrorAs(t, err, &apiErr, "error should be an APIError")
	checkEqual(t, 500, apiErr.StatusCode, "status code should be 500 internal server error")
}

// TestClientListProfileSecurityQuestionsSuccess verifies ListProfileSecurityQuestions sends a GET request to /profile/security-questions.
func TestClientListProfileSecurityQuestionsSuccess(t *testing.T) {
	t.Parallel()

	questions := linode.ProfileSecurityQuestions{
		"security_questions": []map[string]any{{keyID: float64(1), "question": "What is your favorite color?"}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/security-questions", r.URL.Path, "request path should be /profile/security-questions")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(questions), "encoding profile security questions response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListProfileSecurityQuestions(t.Context())

	mustNoError(t, err, "ListProfileSecurityQuestions should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	items, ok := (*result)["security_questions"].([]any)
	mustTrue(t, ok, "security_questions should decode as a JSON array")
	mustLen(t, items, 1)
	item, ok := items[0].(map[string]any)
	mustTrue(t, ok, "security question entry should decode as an object")
	checkEqual(t, "What is your favorite color?", item["question"])
}

// TestClientListProfileSecurityQuestionsAPIError verifies ListProfileSecurityQuestions propagates API errors.
func TestClientListProfileSecurityQuestionsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/profile/security-questions", r.URL.Path, "request path should be /profile/security-questions")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListProfileSecurityQuestions(t.Context())

	mustError(t, err, "ListProfileSecurityQuestions should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
}

// TestClientListProfileSecurityQuestionsRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListProfileSecurityQuestionsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/profile/security-questions", r.URL.Path, "request path should be /profile/security-questions")

		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ProfileSecurityQuestions{"security_questions": []map[string]any{{keyID: float64(1)}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListProfileSecurityQuestions(t.Context())

	mustNoError(t, err, "ListProfileSecurityQuestions should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, int32(2), attempts.Load(), "read-only list should retry one transient failure")
}

// TestClientListProfileDevicesSuccess verifies ListProfileDevices sends a GET request to /profile/devices.
func TestClientListProfileDevicesSuccess(t *testing.T) {
	t.Parallel()

	devices := linode.PaginatedResponse[linode.ProfileDevice]{
		Data: []linode.ProfileDevice{{keyID: float64(123), "user_agent": "Mozilla/5.0", "last_remote_addr": "192.0.2.1"}},
		Page: 2, Pages: 3, Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/devices", r.URL.Path, "request path should be /profile/devices")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(devices), "encoding profile devices response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListProfileDevices(t.Context(), 2, 25)

	mustNoError(t, err, "ListProfileDevices should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 2, result.Page)
	mustLen(t, result.Data, 1)
	checkEqual(t, "Mozilla/5.0", result.Data[0]["user_agent"])
	checkEqual(t, "192.0.2.1", result.Data[0]["last_remote_addr"])
}

// TestClientListProfileDevicesAPIError verifies ListProfileDevices propagates API errors.
func TestClientListProfileDevicesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/profile/devices", r.URL.Path, "request path should be /profile/devices")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListProfileDevices(t.Context(), 0, 0)

	mustError(t, err, "ListProfileDevices should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
}

// TestClientListProfileDevicesRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListProfileDevicesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/profile/devices", r.URL.Path, "request path should be /profile/devices")

		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ProfileDevice]{Data: []linode.ProfileDevice{{keyID: float64(123)}}, Page: 1, Pages: 1, Results: 1}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListProfileDevices(t.Context(), 0, 0)

	mustNoError(t, err, "ListProfileDevices should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	mustLen(t, result.Data, 1)
	checkEqual(t, int32(2), attempts.Load(), "read-only list should retry one transient failure")
}

// TestClientGetProfileNetworkError verifies that GetProfile returns a
// NetworkError when the server is unreachable.
func TestClientGetProfileNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "token", nil, linode.WithMaxRetries(0))
	_, err := client.GetProfile(t.Context())

	mustError(t, err, "GetProfile should fail when server is unreachable")

	var netErr *linode.NetworkError

	checkErrorAs(t, err, &netErr, "error should be a NetworkError")
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

	mustError(t, err, "GetProfile should fail on 429 response")

	var apiErr *linode.APIError

	mustErrorAs(t, err, &apiErr, "error should be an APIError")
	checkEqual(t, 429, apiErr.StatusCode, "status code should be 429 too many requests")
	checkContains(t, apiErr.Message, "retry after", "error message should include the retry-after value")
	checkEqual(t, 30*time.Second, apiErr.RetryAfter, "RetryAfter field should carry the parsed hint")
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

	mustError(t, err, "GetProfile should fail on 403 response")

	var apiErr *linode.APIError

	mustErrorAs(t, err, &apiErr, "error should be an APIError")
	checkEqual(t, 403, apiErr.StatusCode, "status code should be 403 forbidden")
}

// TestClientContextCancelled verifies that GetProfile returns an error
// when the request context is already canceled before the call.
func TestClientContextCancelled(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		checkNoError(t, json.NewEncoder(w).Encode(linode.Profile{Username: "test"}), "encoding profile response should not fail")
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.GetProfile(ctx)
	mustError(t, err, "GetProfile should fail when context is already canceled")
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

	mustError(t, err)
	checkEqual(t, 6, attempts, "should attempt 1 initial + 5 retries from config")
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

	mustError(t, err)
	checkEqual(t, 2, attempts, "should attempt 1 initial + 1 retry from option override")
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

	mustError(t, err)
	checkEqual(t, 4, attempts, "should attempt 1 initial + 3 default retries")
}

// TestClientMalformedJSONResponse verifies that the client returns an error
// when the API responds with 200 OK but invalid JSON.
func TestClientMalformedJSONResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`not json at all`))
		checkNoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.GetProfile(t.Context())

	mustError(t, err, "GetProfile should fail when response body is not valid JSON")

	var syntaxErr *json.SyntaxError

	checkErrorAs(t, err, &syntaxErr, "error chain should contain a json.SyntaxError")
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
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/profile", r.URL.Path, "request path should be /profile")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		checkNoError(t, json.NewDecoder(r.Body).Decode(&body))
		checkEqual(t, updateAccountEmail, body["email"])

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(updatedProfile))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	email := updateAccountEmail
	result, err := client.UpdateProfile(t.Context(), &linode.UpdateProfileRequest{
		Email: &email,
	})

	mustNoError(t, err, "UpdateProfile should succeed on 200 response")
	checkEqual(t, updateAccountEmail, result.Email)
	checkEqual(t, "US/Eastern", result.Timezone)
}

// TestClientUpdateProfilePreferencesSuccess verifies that UpdateProfilePreferences
// sends a PUT request to /profile/preferences with an empty JSON body.
func TestClientUpdateProfilePreferencesSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/profile/preferences", r.URL.Path, "request path should be /profile/preferences")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		checkNoError(t, json.NewDecoder(r.Body).Decode(&body))
		checkEmpty(t, body, "request body should be an empty JSON object")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{profilePreferenceKeyTheme: profilePreferenceValueDark}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.UpdateProfilePreferences(t.Context(), nil)

	mustNoError(t, err, "UpdateProfilePreferences should succeed on 200 response")
	checkEqual(t, profilePreferenceValueDark, result[profilePreferenceKeyTheme])
}

// TestClientUpdateProfileNetworkError verifies that UpdateProfile returns a
// NetworkError when the HTTP request fails to reach the server.
func TestClientUpdateProfilePreferencesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"field":"theme","reason":"invalid preference"}]}`))
		checkNoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateProfilePreferences(t.Context(), linode.ProfilePreferences{profilePreferenceKeyTheme: profilePreferenceValueDark})

	mustError(t, err, "UpdateProfilePreferences should fail on 400 response")

	var apiErr *linode.APIError

	mustErrorAs(t, err, &apiErr, "error should be an APIError")
	checkEqual(t, 400, apiErr.StatusCode)
}

func TestClientUpdateProfilePreferencesNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateProfilePreferences(t.Context(), linode.ProfilePreferences{profilePreferenceKeyTheme: profilePreferenceValueDark})

	mustError(t, err, "UpdateProfilePreferences should fail when the server is unreachable")

	var netErr *linode.NetworkError

	checkErrorAs(t, err, &netErr, "error should be a NetworkError")
}

func TestClientUpdateProfilePreferencesMalformedJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`not json at all`))
		checkNoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateProfilePreferences(t.Context(), linode.ProfilePreferences{profilePreferenceKeyTheme: profilePreferenceValueDark})

	mustError(t, err, "UpdateProfilePreferences should fail when response body is not valid JSON")

	var syntaxErr *json.SyntaxError

	checkErrorAs(t, err, &syntaxErr, "error chain should contain a json.SyntaxError")
}

func TestClientUpdateProfileNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateProfile(t.Context(), &linode.UpdateProfileRequest{})

	mustError(t, err, "UpdateProfile should fail when the server is unreachable")

	var netErr *linode.NetworkError

	checkErrorAs(t, err, &netErr, "error should be a NetworkError")
}

// TestClientUpdateProfileAPIError verifies that UpdateProfile propagates
// API errors (non-2xx) through the handleResponse error chain.
func TestClientUpdateProfileAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"field":"email","reason":"invalid email format"}]}`))
		checkNoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateProfile(t.Context(), &linode.UpdateProfileRequest{})

	mustError(t, err, "UpdateProfile should fail on 400 response")

	var apiErr *linode.APIError

	mustErrorAs(t, err, &apiErr, "error should be an APIError")
	checkEqual(t, 400, apiErr.StatusCode)
}

func TestClientListObjectStorageBucketsByRegionSuccess(t *testing.T) {
	t.Parallel()

	buckets := []linode.ObjectStorageBucket{
		{Label: "my-bucket", Region: "us-east-1", Hostname: "my-bucket.us-east-1.linodeobjects.com"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/object-storage/buckets/us-east-1", r.URL.Path, "request path should match regional buckets endpoint")
		checkEmpty(t, r.URL.RawQuery, "request should not include query params")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    buckets,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListObjectStorageBucketsByRegion(t.Context(), "us-east-1")

	mustNoError(t, err, "ListObjectStorageBucketsByRegion should succeed on 200 response")
	mustLen(t, result, 1, "response should include one bucket")
	checkEqual(t, "my-bucket", result[0].Label)
}

func TestClientListObjectStorageBucketsByRegionEscapesPathParam(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/object-storage/buckets/us%2Feast%3F1", r.URL.EscapedPath(), "path params should be escaped")
		checkEmpty(t, r.URL.RawQuery, "path params must not become query params")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    []linode.ObjectStorageBucket{},
			keyPage:    1,
			keyPages:   1,
			keyResults: 0,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	_, err := client.ListObjectStorageBucketsByRegion(t.Context(), "us/east?1")

	mustNoError(t, err, "escaped path param should round-trip through the client")
}

func TestClientListObjectStorageClustersRemoved(t *testing.T) {
	t.Parallel()

	_, ok := reflect.TypeFor[*linode.Client]().MethodByName("ListObjectStorageClusters")

	checkFalse(t, ok, "deprecated Object Storage clusters route must not be exposed by the Go client")
}

func TestClientCancelObjectStorageSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/object-storage/cancel", r.URL.Path)
		checkEqual(t, http.MethodPost, r.Method)
		checkEmpty(t, r.URL.RawQuery)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.CancelObjectStorage(t.Context())
	mustNoError(t, err, "CancelObjectStorage should succeed on 200 response")
}

func TestClientCancelObjectStorageAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/object-storage/cancel", r.URL.Path)
		checkEqual(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"reason":"object storage cannot be canceled"}]}`))
		checkNoError(t, err)
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.CancelObjectStorage(t.Context())

	mustError(t, err, "CancelObjectStorage should fail on API error")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should be an APIError")
	checkEqual(t, http.StatusBadRequest, apiErr.StatusCode)
}

func TestClientCancelObjectStorageDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, "/object-storage/cancel", r.URL.Path)
		checkEqual(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, "my-token", nil, linode.WithMaxRetries(3))
	err := client.CancelObjectStorage(t.Context())
	mustError(t, err, "CancelObjectStorage should fail on 500 response")
	checkEqual(t, int32(1), calls.Load(), "CancelObjectStorage must not retry and replay a state-changing request")
}

func TestClientAllowObjectStorageBucketAccessSuccess(t *testing.T) {
	t.Parallel()

	corsEnabled := true

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/object-storage/buckets/us-east-1/my-bucket/access", r.URL.Path, "request path should match access endpoint")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		checkNoError(t, json.NewDecoder(r.Body).Decode(&body))
		checkEqual(t, "public-read", body["acl"])
		checkEqual(t, true, body["cors_enabled"])

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.AllowObjectStorageBucketAccess(t.Context(), "us-east-1", "my-bucket", linode.AllowObjectStorageBucketAccessRequest{
		ACL:         "public-read",
		CORSEnabled: &corsEnabled,
	})

	mustNoError(t, err, "AllowObjectStorageBucketAccess should succeed on 200 response")
}

func TestClientAllowObjectStorageBucketAccessEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/object-storage/buckets/us%2Feast%3F1/..%2Fbucket/access", r.URL.EscapedPath(), "path params should be escaped")
		checkEmpty(t, r.URL.RawQuery, "path params must not become query params")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.AllowObjectStorageBucketAccess(t.Context(), "us/east?1", "../bucket", linode.AllowObjectStorageBucketAccessRequest{})

	mustNoError(t, err, "escaped path params should round-trip through the client")
}

func TestClientAllowObjectStorageBucketAccessDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/object-storage/buckets/us-east-1/my-bucket/access", r.URL.Path, "request path should match access endpoint")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		checkNoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))
	err := client.AllowObjectStorageBucketAccess(t.Context(), "us-east-1", "my-bucket", linode.AllowObjectStorageBucketAccessRequest{})

	mustError(t, err, "AllowObjectStorageBucketAccess should fail on 500 response")
	checkEqual(t, int32(1), calls.Load(), "AllowObjectStorageBucketAccess must not retry and replay a state-changing request")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/transfer", r.URL.Path, "request path should be /account/transfer")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(transfer))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountTransfer(t.Context())

	mustNoError(t, err, "GetAccountTransfer should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 10, result.Billable)
	checkEqual(t, 4000, result.Quota)
	checkEqual(t, 123, result.Used)
	mustLen(t, result.RegionTransfers, 1)
	checkEqual(t, accountTransferRegion, result.RegionTransfers[0].ID)
	checkEqual(t, 50, result.RegionTransfers[0].Used)
}

// TestClientGetAccountTransferAPIError verifies GetAccountTransfer propagates
// API errors through the handleResponse error chain.
func TestClientGetAccountTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/transfer", r.URL.Path, "request path should be /account/transfer")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountTransfer(t.Context())

	mustError(t, err, "GetAccountTransfer should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/transfer", r.URL.Path, "request path should be /account/transfer")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountTransfer{Used: 123}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountTransfer(t.Context())

	mustNoError(t, err, "GetAccountTransfer should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 123, result.Used)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/settings", r.URL.Path, "request path should be /account/settings")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(settings))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountSettings(t.Context())

	mustNoError(t, err, "GetAccountSettings should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkTrue(t, result.BackupsEnabled)
	checkFalse(t, result.Managed)
	checkTrue(t, result.NetworkHelper)
	mustNotNil(t, result.LongviewSubscription)
	checkEqual(t, longviewSubscription, *result.LongviewSubscription)
	mustNotNil(t, result.ObjectStorage)
	checkEqual(t, objectStorage, *result.ObjectStorage)
	checkEqual(t, "linode_default_but_legacy_config_allowed", result.InterfacesForNewLinodes)
	checkEqual(t, maintenancePolicyMigrate, result.MaintenancePolicy)
}

// TestClientGetAccountSettingsAPIError verifies GetAccountSettings propagates
// API errors through the handleResponse error chain.
func TestClientGetAccountSettingsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/settings", r.URL.Path, "request path should be /account/settings")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountSettings(t.Context())

	mustError(t, err, "GetAccountSettings should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/agreements", r.URL.Path, "request path should be /account/agreements")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(agreements))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountAgreements(t.Context())

	mustNoError(t, err, "GetAccountAgreements should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkTrue(t, result.BillingAgreement)
	checkFalse(t, result.EUModel)
	checkTrue(t, result.MasterServiceAgreement)
	checkTrue(t, result.PrivacyPolicy)
}

// TestClientGetAccountAgreementsAPIError verifies GetAccountAgreements propagates
// API errors through the handleResponse error chain.
func TestClientGetAccountAgreementsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/agreements", r.URL.Path, "request path should be /account/agreements")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountAgreements(t.Context())

	mustError(t, err, "GetAccountAgreements should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/notifications", r.URL.Path, "request path should be /account/notifications")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(notifications))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountNotifications(t.Context(), 2, 25)

	mustNoError(t, err, "ListAccountNotifications should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 2, result.Page)
	mustLen(t, result.Data, 1)
	checkEqual(t, "Scheduled maintenance", result.Data[0].Label)
	checkEqual(t, "major", result.Data[0].Severity)
	mustNotNil(t, result.Data[0].Entity)
	checkEqual(t, "example-linode", result.Data[0].Entity.Label)
}

// TestClientListAccountNotificationsAPIError verifies ListAccountNotifications propagates API errors.
func TestClientListAccountNotificationsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/notifications", r.URL.Path, "request path should be /account/notifications")
		checkEmpty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountNotifications(t.Context(), 0, 0)

	mustError(t, err, "ListAccountNotifications should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/notifications", r.URL.Path, "request path should be /account/notifications")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountNotification]{
			Data: []linode.AccountNotification{{Label: "Scheduled maintenance"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountNotifications(t.Context(), 0, 0)

	mustNoError(t, err, "ListAccountNotifications should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/availability/"+regionUSEast, r.URL.Path, "request path should include region")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(availability))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountAvailability(t.Context(), regionUSEast)

	mustNoError(t, err, "GetAccountAvailability should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, regionUSEast, result.Region)
	checkEqual(t, []string{serviceLinodes, serviceNodeBalancers}, result.Available)
}

// TestClientGetAccountAvailabilityEscapesRegion verifies the client encodes path separators.
func TestClientGetAccountAvailabilityEscapesRegion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/availability/us%2Feast%3Fzone", r.URL.EscapedPath(), "request path should escape region")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountAvailability{Region: "us/east?zone"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountAvailability(t.Context(), "us/east?zone")

	mustNoError(t, err, "GetAccountAvailability should escape path parameters")
}

// TestClientGetAccountAvailabilityAPIError verifies GetAccountAvailability propagates API errors.
func TestClientGetAccountAvailabilityAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/availability/"+regionUSEast, r.URL.Path, "request path should include region")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountAvailability(t.Context(), regionUSEast)

	mustError(t, err, "GetAccountAvailability should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/availability/"+regionUSEast, r.URL.Path, "request path should include region")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountAvailability{Region: regionUSEast}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountAvailability(t.Context(), regionUSEast)

	mustNoError(t, err, "GetAccountAvailability should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, regionUSEast, result.Region)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

// TestClientListAccountAvailabilitySuccess verifies ListAccountAvailability sends
// a GET request to /account/availability with pagination query parameters.
func TestClientListAccountAvailabilitySuccess(t *testing.T) {
	t.Parallel()

	availability := linode.PaginatedResponse[linode.AccountAvailability]{
		Data: []linode.AccountAvailability{{
			Available:   []string{"Linodes", serviceNodeBalancers},
			Region:      regionUSEast,
			Unavailable: []string{"Kubernetes", "Block Storage"},
		}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/availability", r.URL.Path, "request path should be /account/availability")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(availability))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountAvailability(t.Context(), 2, 25)

	mustNoError(t, err, "ListAccountAvailability should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 2, result.Page)
	mustLen(t, result.Data, 1)
	checkEqual(t, regionUSEast, result.Data[0].Region)
	checkEqual(t, []string{"Linodes", serviceNodeBalancers}, result.Data[0].Available)
}

// TestClientListAccountAvailabilityAPIError verifies ListAccountAvailability propagates API errors.
func TestClientListAccountAvailabilityAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/availability", r.URL.Path, "request path should be /account/availability")
		checkEmpty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountAvailability(t.Context(), 0, 0)

	mustError(t, err, "ListAccountAvailability should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/apps", r.URL.Path, "request path should be /profile/apps")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(apps), "encoding profile apps response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListProfileApps(t.Context(), 2, 25)

	mustNoError(t, err, "ListProfileApps should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 2, result.Page)
	mustLen(t, result.Data, 1)
	checkEqual(t, "example-app", result.Data[0].Label)
	checkEqual(t, profileAppScopesReadOnly, result.Data[0].Scopes)
	checkEqual(t, "example.org", result.Data[0].Website)
	mustNotNil(t, result.Data[0].ThumbnailURL)
	checkEqual(t, thumbnailURL, *result.Data[0].ThumbnailURL)
}

// TestClientListProfileAppsAPIError verifies ListProfileApps propagates API errors.
func TestClientListProfileAppsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/profile/apps", r.URL.Path, "request path should be /profile/apps")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListProfileApps(t.Context(), 0, 0)

	mustError(t, err, "ListProfileApps should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
}

// TestClientListProfileAppsRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListProfileAppsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/profile/apps", r.URL.Path, "request path should be /profile/apps")

		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "temporary upstream failure"}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AuthorizedApp]{Data: []linode.AuthorizedApp{{ID: 123, Label: "example-app", Scopes: profileAppScopesReadOnly}}, Page: 1, Pages: 1, Results: 1}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListProfileApps(t.Context(), 0, 0)

	mustNoError(t, err, "ListProfileApps should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	mustLen(t, result.Data, 1)
	checkEqual(t, int32(2), attempts.Load(), "read-only list should retry one transient failure")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/oauth-clients", r.URL.Path, "request path should be /account/oauth-clients")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(clients), "encoding oauth clients response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountOAuthClients(t.Context(), 2, 25)

	mustNoError(t, err, "ListAccountOAuthClients should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 2, result.Page)
	mustLen(t, result.Data, 1)
	checkEqual(t, "example-client", result.Data[0].Label)
	checkEqual(t, "https://example.com/oauth/callback", result.Data[0].RedirectURI)
}

// TestClientListAccountOAuthClientsAPIError verifies ListAccountOAuthClients propagates API errors.
func TestClientListAccountOAuthClientsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/account/oauth-clients", r.URL.Path, "request path should be /account/oauth-clients")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountOAuthClients(t.Context(), 0, 0)

	mustError(t, err, "ListAccountOAuthClients should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
}

// TestClientListAccountOAuthClientsRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListAccountOAuthClientsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/account/oauth-clients", r.URL.Path, "request path should be /account/oauth-clients")

		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "temporary upstream failure"}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.OAuthClient]{Data: []linode.OAuthClient{{ID: "2737bf16b39ab5d7b4a1", Label: "example-client"}}, Page: 1, Pages: 1, Results: 1}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountOAuthClients(t.Context(), 0, 0)

	mustNoError(t, err, "ListAccountOAuthClients should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	mustLen(t, result.Data, 1)
	checkEqual(t, int32(2), attempts.Load(), "read-only list should retry one transient failure")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/betas", r.URL.Path, "request path should be /betas")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(betas))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListBetas(t.Context(), 2, 25)

	mustNoError(t, err, "ListBetas should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 2, result.Page)
	mustLen(t, result.Data, 1)
	checkEqual(t, "example_open", result.Data[0].ID)
	checkEqual(t, "open", result.Data[0].BetaClass)
	checkFalse(t, result.Data[0].GreenlightOnly)
	checkEqual(t, "https://example.com/beta", result.Data[0].MoreInfo)
	checkNil(t, result.Data[0].Ended)
}

// TestClientListBetasAPIError verifies ListBetas propagates API errors.
func TestClientListBetasAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/betas", r.URL.Path, "request path should be /betas")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListBetas(t.Context(), 0, 0)

	mustError(t, err, "ListBetas should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
}

// TestClientListBetasRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListBetasRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	betas := linode.PaginatedResponse[linode.BetaProgram]{
		Data: []linode.BetaProgram{{ID: "example_open", Label: "Example Open Beta"}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/betas", r.URL.Path, "request path should be /betas")

		if requestCount.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(betas))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListBetas(t.Context(), 0, 0)

	mustNoError(t, err, "ListBetas should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	mustLen(t, result.Data, 1)
	checkEqual(t, "example_open", result.Data[0].ID)
	checkEqual(t, int32(2), requestCount.Load(), "read-only list should retry one transient failure")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/betas/"+betaExampleOpen, r.URL.Path, "request path should include beta id")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(beta))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetBeta(t.Context(), betaExampleOpen)

	mustNoError(t, err, "GetBeta should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, betaExampleOpen, result.ID)
	checkEqual(t, labelExampleOpenBeta, result.Label)
	mustNotNil(t, result.Description)
	checkEqual(t, description, *result.Description)
	checkEqual(t, "open", result.BetaClass)
	checkFalse(t, result.GreenlightOnly)
}

// TestClientGetBetaEscapesID verifies the client encodes path separators.
func TestClientGetBetaEscapesID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/betas/example%2Fopen%3Fquery", r.URL.EscapedPath(), "request path should escape beta id")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.BetaProgram{ID: "example/open?query"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetBeta(t.Context(), "example/open?query")

	mustNoError(t, err, "GetBeta should escape path parameters")
}

// TestClientGetBetaAPIError verifies GetBeta propagates API errors.
func TestClientGetBetaAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/betas/"+betaExampleOpen, r.URL.Path, "request path should include beta id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetBeta(t.Context(), betaExampleOpen)

	mustError(t, err, "GetBeta should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/betas/"+betaExampleOpen, r.URL.Path, "request path should include beta id")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.BetaProgram{ID: betaExampleOpen, Label: labelExampleOpenBeta}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetBeta(t.Context(), betaExampleOpen)

	mustNoError(t, err, "GetBeta should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, betaExampleOpen, result.ID)
	checkEqual(t, int32(2), requestCount.Load(), "read-only get should retry once then succeed")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/betas", r.URL.Path, "request path should be /account/betas")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(betas))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountBetas(t.Context(), 2, 25)

	mustNoError(t, err, "ListAccountBetas should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 2, result.Page)
	mustLen(t, result.Data, 1)
	checkEqual(t, betaExampleOpen, result.Data[0].ID)
	checkEqual(t, labelExampleOpenBeta, result.Data[0].Label)
	mustNotNil(t, result.Data[0].Description)
	checkEqual(t, description, *result.Data[0].Description)
	checkNil(t, result.Data[0].Ended)
}

// TestClientListAccountBetasAPIError verifies ListAccountBetas propagates API errors.
func TestClientListAccountBetasAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/betas", r.URL.Path, "request path should be /account/betas")
		checkEmpty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountBetas(t.Context(), 0, 0)

	mustError(t, err, "ListAccountBetas should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/betas", r.URL.Path, "request path should be /account/betas")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountBetaProgram]{
			Data: []linode.AccountBetaProgram{{ID: betaExampleOpen, Label: labelExampleOpenBeta}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountBetas(t.Context(), 0, 0)

	mustNoError(t, err, "ListAccountBetas should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	mustLen(t, result.Data, 1)
	checkEqual(t, betaExampleOpen, result.Data[0].ID)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

// TestClientGetAccountOAuthClientSuccess verifies GetAccountOAuthClient sends the exact GET request.
func TestClientGetAccountOAuthClientSuccess(t *testing.T) {
	t.Parallel()

	want := linode.OAuthClient{ID: oauthClientID, Label: oauthClientLabel, RedirectURI: oauthClientRedirectURI, Status: oauthClientStatus, ThumbnailURL: oauthClientThumbnailURL}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/oauth-clients/"+oauthClientID, r.URL.Path, "request path should include client id")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID: oauthClientID, keyLabel: oauthClientLabel, keyRedirectURI: oauthClientRedirectURI, keyStatus: oauthClientStatus, keyThumbnailURL: oauthClientThumbnailURL, "secret": "server-secret",
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetAccountOAuthClient(t.Context(), oauthClientID)

	mustNoError(t, err, "GetAccountOAuthClient should succeed on 200 response")
	mustNotNil(t, got, "result should not be nil")
	checkEqual(t, want, *got)
}

func TestClientGetAccountOAuthClientEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/account/oauth-clients/client%2F123%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.OAuthClient{ID: oauthClientIDWithSeparators}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountOAuthClient(t.Context(), oauthClientIDWithSeparators)

	mustNoError(t, err, "GetAccountOAuthClient should escape path parameters")
}

func TestClientGetAccountOAuthClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/oauth-clients/"+oauthClientID, r.URL.Path, "request path should include client id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountOAuthClient(t.Context(), oauthClientID)

	mustError(t, err, "GetAccountOAuthClient should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkContains(t, apiErr.Message, errForbidden)
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

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.OAuthClient{ID: oauthClientID, Label: oauthClientLabel}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.GetAccountOAuthClient(t.Context(), oauthClientID)

	mustNoError(t, err, "GetAccountOAuthClient should succeed after retry")
	mustNotNil(t, got, "result should not be nil")
	checkEqual(t, oauthClientID, got.ID)
	checkEqual(t, int32(2), requestCount.Load(), "read-only GET should retry once")
}

func TestClientUpdateOAuthClientSuccess(t *testing.T) {
	t.Parallel()

	public := true
	want := linode.OAuthClient{ID: oauthClientID, Label: "updated app", Public: public, RedirectURI: "https://example.com/new-callback", Status: oauthClientStatus, ThumbnailURL: oauthClientThumbnailURL}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/account/oauth-clients/"+oauthClientID, r.URL.Path, "request path should include client id")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, "application/json", r.Header.Get("Content-Type"))

		var got linode.UpdateOAuthClientRequest
		checkNoError(t, json.NewDecoder(r.Body).Decode(&got))

		if checkNotNil(t, got.Label) {
			checkEqual(t, want.Label, *got.Label)
		}

		if checkNotNil(t, got.RedirectURI) {
			checkEqual(t, want.RedirectURI, *got.RedirectURI)
		}

		if checkNotNil(t, got.Public) {
			checkEqual(t, public, *got.Public)
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.UpdateOAuthClientRequest{Label: &want.Label, Public: &public, RedirectURI: &want.RedirectURI}

	got, err := client.UpdateOAuthClient(t.Context(), oauthClientID, req)

	mustNoError(t, err, "UpdateOAuthClient should succeed on 200 response")
	mustNotNil(t, got, "result should not be nil")
	checkEqual(t, want, *got)
}

func TestClientUpdateOAuthClientEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/account/oauth-clients/client%2F123%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.OAuthClient{ID: oauthClientIDWithSeparators}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	label := oauthClientLabel
	req := &linode.UpdateOAuthClientRequest{Label: &label}

	_, err := client.UpdateOAuthClient(t.Context(), oauthClientIDWithSeparators, req)

	mustNoError(t, err, "UpdateOAuthClient should escape path parameters")
}

func TestClientUpdateOAuthClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/account/oauth-clients/"+oauthClientID, r.URL.Path, "request path should include client id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	label := oauthClientLabel
	req := &linode.UpdateOAuthClientRequest{Label: &label}

	_, err := client.UpdateOAuthClient(t.Context(), oauthClientID, req)

	mustError(t, err, "UpdateOAuthClient should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
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

	mustError(t, err, "UpdateOAuthClient should return the transient error")
	checkEqual(t, int32(1), requestCount.Load(), "mutating OAuth client update must not be retried")
}

func TestClientCreateOAuthClientSuccess(t *testing.T) {
	t.Parallel()

	want := linode.CreatedOAuthClient{ID: oauthClientID, Label: oauthClientLabel, RedirectURI: oauthClientRedirectURI, Secret: "secret-once"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/oauth-clients", r.URL.Path, "request path should match")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, "application/json", r.Header.Get("Content-Type"))

		var got linode.CreateOAuthClientRequest
		checkNoError(t, json.NewDecoder(r.Body).Decode(&got))
		checkEqual(t, oauthClientLabel, got.Label)
		checkEqual(t, oauthClientRedirectURI, got.RedirectURI)

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateOAuthClientRequest{Label: oauthClientLabel, RedirectURI: oauthClientRedirectURI}

	got, err := client.CreateOAuthClient(t.Context(), req)

	mustNoError(t, err, "CreateOAuthClient should succeed on 200 response")
	mustNotNil(t, got, "result should not be nil")
	checkEqual(t, want, *got)
}

func TestClientUpdateOAuthClientThumbnailSuccess(t *testing.T) {
	t.Parallel()

	thumbnailPNG := []byte("png-bytes")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/account/oauth-clients/"+oauthClientID+"/thumbnail", r.URL.Path, "request path should update client thumbnail")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, "image/png", r.Header.Get("Content-Type"))

		got, err := io.ReadAll(r.Body)
		checkNoError(t, err, "reading thumbnail body should not fail")
		checkEqual(t, thumbnailPNG, got, "thumbnail update should send the PNG bytes")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.UpdateOAuthClientThumbnail(t.Context(), oauthClientID, thumbnailPNG)

	mustNoError(t, err, "UpdateOAuthClientThumbnail should succeed on 200 response")
}

func TestClientUpdateOAuthClientThumbnailEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/account/oauth-clients/client%2F123%3Fquery/thumbnail", r.URL.EscapedPath(), "path parameter should be escaped")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.OAuthClient{ID: oauthClientIDWithSeparators}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.UpdateOAuthClientThumbnail(t.Context(), oauthClientIDWithSeparators, []byte("png-bytes"))

	mustNoError(t, err, "UpdateOAuthClientThumbnail should escape path parameters")
}

func TestClientUpdateOAuthClientThumbnailAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/account/oauth-clients/"+oauthClientID+"/thumbnail", r.URL.Path, "request path should update client thumbnail")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.UpdateOAuthClientThumbnail(t.Context(), oauthClientID, []byte("png-bytes"))

	mustError(t, err, "UpdateOAuthClientThumbnail should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientUpdateOAuthClientThumbnailDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		checkEqual(t, "/account/oauth-clients/"+oauthClientID+"/thumbnail", r.URL.Path, "request path should update client thumbnail")
		http.Error(w, errTemporaryFailure, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.UpdateOAuthClientThumbnail(t.Context(), oauthClientID, []byte("png-bytes"))

	mustError(t, err, "UpdateOAuthClientThumbnail should return the transient error")
	checkEqual(t, int32(1), requestCount.Load(), "mutating OAuth client thumbnail update must not be retried")
}

func TestClientGetOAuthClientThumbnailSuccess(t *testing.T) {
	t.Parallel()

	thumbnailPNG := []byte("png-bytes")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/oauth-clients/"+oauthClientID+"/thumbnail", r.URL.Path, "request path should get client thumbnail")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "image/png")
		_, writeErr := w.Write(thumbnailPNG)
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetOAuthClientThumbnail(t.Context(), oauthClientID)

	mustNoError(t, err, "GetOAuthClientThumbnail should succeed on 200 response")
	checkEqual(t, thumbnailPNG, got, "GetOAuthClientThumbnail should return the PNG bytes")
}

func TestClientGetOAuthClientThumbnailEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/account/oauth-clients/client%2F123%3Fquery/thumbnail", r.URL.EscapedPath(), "path parameter should be escaped")
		w.Header().Set("Content-Type", "image/png")
		_, writeErr := w.Write([]byte("png-bytes"))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetOAuthClientThumbnail(t.Context(), oauthClientIDWithSeparators)

	mustNoError(t, err, "GetOAuthClientThumbnail should escape path parameters")
}

func TestClientGetOAuthClientThumbnailAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/oauth-clients/"+oauthClientID+"/thumbnail", r.URL.Path, "request path should get client thumbnail")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "Not Found"}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetOAuthClientThumbnail(t.Context(), oauthClientID)

	mustError(t, err, "GetOAuthClientThumbnail should fail on 404 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should be an APIError")
	checkEqual(t, http.StatusNotFound, apiErr.StatusCode)
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
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.GetOAuthClientThumbnail(t.Context(), oauthClientID)

	mustNoError(t, err, "GetOAuthClientThumbnail should succeed after retry")
	checkEqual(t, thumbnailPNG, got, "GetOAuthClientThumbnail should return the PNG bytes")
	checkEqual(t, int32(2), requestCount.Load(), "read-only GetOAuthClientThumbnail should be retried on transient error")
}

func TestClientDeleteAccountOAuthClientSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/account/oauth-clients/"+oauthClientID, r.URL.Path, "request path should include client id")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, http.NoBody, r.Body, "DELETE request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountOAuthClient(t.Context(), oauthClientID)

	mustNoError(t, err, "DeleteAccountOAuthClient should succeed on 200 response")
}

func TestClientDeleteAccountOAuthClientEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/account/oauth-clients/client%2F123%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		checkEmpty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountOAuthClient(t.Context(), oauthClientIDWithSeparators)

	mustNoError(t, err, "DeleteAccountOAuthClient should escape path parameters")
}

func TestClientDeleteAccountOAuthClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/account/oauth-clients/"+oauthClientID, r.URL.Path, "request path should include client id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountOAuthClient(t.Context(), oauthClientID)

	mustError(t, err, "DeleteAccountOAuthClient should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
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

	mustError(t, err, "DeleteAccountOAuthClient should return the transient error")
	checkEqual(t, int32(1), requestCount.Load(), "destructive OAuth client delete must not be retried")
}

func TestClientResetOAuthClientSecretSuccess(t *testing.T) {
	t.Parallel()

	want := linode.OAuthClientSecret{Secret: "new-secret-once"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/oauth-clients/"+oauthClientID+"/reset-secret", r.URL.Path, "request path should reset client secret")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, http.NoBody, r.Body, "reset secret request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.ResetOAuthClientSecret(t.Context(), oauthClientID)

	mustNoError(t, err, "ResetOAuthClientSecret should succeed on 200 response")
	mustNotNil(t, got, "result should not be nil")
	checkEqual(t, want, *got)
}

func TestClientResetOAuthClientSecretEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/oauth-clients/client%2F123%3Fquery/reset-secret", r.URL.EscapedPath(), "path parameter should be escaped")
		checkEmpty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.OAuthClientSecret{Secret: "new-secret"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ResetOAuthClientSecret(t.Context(), oauthClientIDWithSeparators)

	mustNoError(t, err, "ResetOAuthClientSecret should escape path parameters")
}

func TestClientResetOAuthClientSecretAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/oauth-clients/"+oauthClientID+"/reset-secret", r.URL.Path, "request path should reset client secret")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ResetOAuthClientSecret(t.Context(), oauthClientID)

	mustError(t, err, "ResetOAuthClientSecret should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
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

	mustError(t, err, "ResetOAuthClientSecret should return the transient error")
	checkEqual(t, int32(1), requestCount.Load(), "credential rotation must not be retried")
}

// TestClientCreateOAuthClientAPIError verifies API errors propagate.
func TestClientCreateOAuthClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateOAuthClientRequest{Label: oauthClientLabel, RedirectURI: oauthClientRedirectURI}

	_, err := client.CreateOAuthClient(t.Context(), req)

	mustError(t, err, "CreateOAuthClient should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
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

	mustError(t, err, "CreateOAuthClient should return the transient error")
	checkEqual(t, int32(1), requestCount.Load(), "mutating OAuth client creation must not be retried")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/events", r.URL.Path, "request path should be /account/events")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(events))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountEvents(t.Context(), 2, 25)

	mustNoError(t, err, "ListAccountEvents should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 2, result.Page)
	mustLen(t, result.Data, 1)
	checkEqual(t, 123, result.Data[0].ID)
	checkEqual(t, "ticket_create", result.Data[0].Action)
	checkEqual(t, "failed", result.Data[0].Status)
	mustNotNil(t, result.Data[0].Entity)
	checkEqual(t, "ticket", result.Data[0].Entity.Type)
	mustNotNil(t, result.Data[0].Duration)
	checkInDelta(t, duration, *result.Data[0].Duration, 0.001)
}

// TestClientListAccountEventsAPIError verifies ListAccountEvents propagates API errors.
func TestClientListAccountEventsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/events", r.URL.Path, "request path should be /account/events")
		checkEmpty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountEvents(t.Context(), 0, 0)

	mustError(t, err, "ListAccountEvents should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/events", r.URL.Path, "request path should be /account/events")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountEvent]{
			Data: []linode.AccountEvent{{ID: 123, Action: "ticket_create"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountEvents(t.Context(), 0, 0)

	mustNoError(t, err, "ListAccountEvents should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	mustLen(t, result.Data, 1)
	checkEqual(t, 123, result.Data[0].ID)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/child-accounts", r.URL.Path, "request path should be /account/child-accounts")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(childAccounts))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountChildAccounts(t.Context(), 2, 25)

	mustNoError(t, err, "ListAccountChildAccounts should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 2, result.Page)
	mustLen(t, result.Data, 1)
	checkEqual(t, childAccountEUUID, result.Data[0].EUUID)
	checkEqual(t, companyAcme, result.Data[0].Company)
	checkEqual(t, "11/2024", result.Data[0].CreditCard.Expiry)
	checkEqual(t, "0111", result.Data[0].CreditCard.LastFour)
}

// TestClientListAccountChildAccountsAPIError verifies ListAccountChildAccounts propagates API errors.
func TestClientListAccountChildAccountsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/child-accounts", r.URL.Path, "request path should be /account/child-accounts")
		checkEmpty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountChildAccounts(t.Context(), 0, 0)

	mustError(t, err, "ListAccountChildAccounts should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/child-accounts", r.URL.Path, "request path should be /account/child-accounts")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ChildAccount]{
			Data: []linode.ChildAccount{{EUUID: childAccountEUUID, Company: companyAcme}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountChildAccounts(t.Context(), 0, 0)

	mustNoError(t, err, "ListAccountChildAccounts should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	mustLen(t, result.Data, 1)
	checkEqual(t, childAccountEUUID, result.Data[0].EUUID)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

// TestClientGetAccountEventSuccess verifies GetAccountEvent sends a GET
// request to /account/events/{event_id} and decodes the response.
func TestClientGetAccountEventSuccess(t *testing.T) {
	t.Parallel()

	want := linode.AccountEvent{ID: 123, Action: "linode_create", Status: statusSuccessful, Username: "test-user"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/account/events/123", r.URL.Path, "request path should include event ID")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(want), "encoding event response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetAccountEvent(t.Context(), 123)

	mustNoError(t, err, "GetAccountEvent should succeed on 200 response")
	mustNotNil(t, got, "result should not be nil")
	checkEqual(t, 123, got.ID)
	checkEqual(t, "linode_create", got.Action)
	checkEqual(t, statusSuccessful, got.Status)
}

// TestClientGetAccountEventAPIError verifies GetAccountEvent propagates API errors.
func TestClientGetAccountEventAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/account/events/123", r.URL.Path, "request path should include event ID")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr, "writing error response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetAccountEvent(t.Context(), 123)

	mustError(t, err, "GetAccountEvent should fail on 403 response")
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
			checkNoError(t, writeErr, "writing transient error should not fail")

			return
		}

		checkEqual(t, "/account/events/123", r.URL.Path, "request path should include event ID")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountEvent{ID: 123, Action: "linode_create"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	result, err := client.GetAccountEvent(t.Context(), 123)

	mustNoError(t, err, "GetAccountEvent should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 123, result.ID)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

// TestClientMarkAccountEventSeenSuccess verifies MarkAccountEventSeen sends a POST
// request to /account/events/{event_id}/seen with no body.
func TestClientMarkAccountEventSeenSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/events/123/seen", r.URL.Path, "request path should mark event seen")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		checkEqual(t, http.NoBody, r.Body, "request should not send a body")

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		checkNoError(t, writeErr, "writing success response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.MarkAccountEventSeen(t.Context(), 123)

	mustNoError(t, err, "MarkAccountEventSeen should succeed on 200 response")
}

// TestClientMarkAccountEventSeenAPIError verifies MarkAccountEventSeen propagates API errors.
func TestClientMarkAccountEventSeenAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/events/123/seen", r.URL.Path, "request path should mark event seen")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr, "writing error response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.MarkAccountEventSeen(t.Context(), 123)

	mustError(t, err, "MarkAccountEventSeen should fail on 403 response")
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
		checkNoError(t, writeErr, "writing transient error should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	err := client.MarkAccountEventSeen(t.Context(), 123)

	mustError(t, err, "MarkAccountEventSeen should return the transient error")
	checkEqual(t, int32(1), requestCount.Load(), "mutating event seen request must not be retried")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/payment-methods", r.URL.Path, "request path should be /account/payment-methods")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(methods))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountPaymentMethods(t.Context(), 2, 25)

	mustNoError(t, err, "ListAccountPaymentMethods should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 2, result.Page)
	mustLen(t, result.Data, 1)
	checkEqual(t, 123, result.Data[0].ID)
	checkEqual(t, paymentMethodCreditCard, result.Data[0].Type)
	checkTrue(t, result.Data[0].IsDefault)
}

// TestClientListAccountPaymentMethodsAPIError verifies ListAccountPaymentMethods propagates API errors.
func TestClientListAccountPaymentMethodsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/payment-methods", r.URL.Path, "request path should be /account/payment-methods")
		checkEmpty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountPaymentMethods(t.Context(), 0, 0)

	mustError(t, err, "ListAccountPaymentMethods should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/payment-methods", r.URL.Path, "request path should be /account/payment-methods")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountPaymentMethod]{
			Data: []linode.AccountPaymentMethod{{ID: 123, Type: paymentMethodCreditCard, IsDefault: true}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountPaymentMethods(t.Context(), 0, 0)

	mustNoError(t, err, "ListAccountPaymentMethods should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	mustLen(t, result.Data, 1)
	checkEqual(t, 123, result.Data[0].ID)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

func TestClientGetAccountPaymentMethodSuccess(t *testing.T) {
	t.Parallel()

	want := linode.AccountPaymentMethod{ID: 123, Type: paymentMethodCreditCard, IsDefault: true, Data: map[string]any{keyLastFour: "1111"}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/payment-methods/123", r.URL.Path, "request path should include payment method id")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetAccountPaymentMethod(t.Context(), "123")

	mustNoError(t, err, "GetAccountPaymentMethod should succeed on 200 response")
	mustNotNil(t, got, "result should not be nil")
	checkEqual(t, want, *got)
}

func TestClientGetAccountPaymentMethodEscapesID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/account/payment-methods/123%2F456%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountPaymentMethod{ID: 123, Type: paymentMethodCreditCard}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountPaymentMethod(t.Context(), "123/456?query")

	mustNoError(t, err, "GetAccountPaymentMethod should escape path parameters")
}

func TestClientGetAccountPaymentMethodAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/payment-methods/123", r.URL.Path, "request path should include payment method id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountPaymentMethod(t.Context(), "123")

	mustError(t, err, "GetAccountPaymentMethod should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkContains(t, apiErr.Message, errForbidden)
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

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountPaymentMethod{ID: 123, Type: paymentMethodCreditCard}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.GetAccountPaymentMethod(t.Context(), "123")

	mustNoError(t, err, "GetAccountPaymentMethod should succeed after retry")
	mustNotNil(t, got, "result should not be nil")
	checkEqual(t, 123, got.ID)
	checkEqual(t, int32(2), requestCount.Load(), "read-only GET should retry once")
}

func TestClientDeleteAccountPaymentMethodSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/account/payment-methods/123", r.URL.Path, "request path should include payment method id")
		checkEmpty(t, r.URL.RawQuery, "delete request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountPaymentMethod(t.Context(), "123")

	mustNoError(t, err, "DeleteAccountPaymentMethod should succeed on 200 response")
}

func TestClientDeleteAccountPaymentMethodEscapesID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/account/payment-methods/123%2F456%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountPaymentMethod(t.Context(), "123/456?query")

	mustNoError(t, err, "DeleteAccountPaymentMethod should escape path parameters")
}

func TestClientDeleteAccountPaymentMethodAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/account/payment-methods/123", r.URL.Path, "request path should include payment method id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountPaymentMethod(t.Context(), "123")

	mustError(t, err, "DeleteAccountPaymentMethod should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkContains(t, apiErr.Message, errForbidden)
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

	mustError(t, err, "DeleteAccountPaymentMethod should surface transient failures")
	checkEqual(t, int32(1), requestCount.Load(), "destructive DELETE should not be retried")
}

func TestClientMakeAccountPaymentMethodDefaultSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/payment-methods/123/make-default", r.URL.Path, "request path should include payment method id and make-default action")
		checkEmpty(t, r.URL.RawQuery, "make-default request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, http.NoBody, r.Body, "make-default request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.MakeAccountPaymentMethodDefault(t.Context(), "123")

	mustNoError(t, err, "MakeAccountPaymentMethodDefault should succeed on 200 response")
}

func TestClientMakeAccountPaymentMethodDefaultEscapesID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/account/payment-methods/123%2F456%3Fquery/make-default", r.URL.EscapedPath(), "path parameter should be escaped")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.MakeAccountPaymentMethodDefault(t.Context(), "123/456?query")

	mustNoError(t, err, "MakeAccountPaymentMethodDefault should escape path parameters")
}

func TestClientMakeAccountPaymentMethodDefaultAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/payment-methods/123/make-default", r.URL.Path, "request path should include payment method id and make-default action")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.MakeAccountPaymentMethodDefault(t.Context(), "123")

	mustError(t, err, "MakeAccountPaymentMethodDefault should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkContains(t, apiErr.Message, errForbidden)
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

	mustError(t, err, "MakeAccountPaymentMethodDefault should surface transient failures")
	checkEqual(t, int32(1), requestCount.Load(), "mutating make-default POST should not be retried")
}

func TestClientCreateAccountPaymentMethodSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.CreateAccountPaymentMethodRequest{Type: paymentMethodCreditCard, Data: map[string]any{keyToken: paymentMethodToken}, IsDefault: true}
	created := linode.AccountPaymentMethod{ID: 321, Type: paymentMethodCreditCard, IsDefault: true, Data: map[string]any{keyLastFour: "1111"}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/payment-methods", r.URL.Path, "request path should be /account/payment-methods")
		checkEmpty(t, r.URL.RawQuery, "create request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		var body map[string]any

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		checkNoError(t, decodeErr)

		if decodeErr != nil {
			return
		}

		checkEqual(t, paymentMethodCreditCard, body[keyType])
		checkEqual(t, true, body[keyIsDefault])
		checkEqual(t, map[string]any{keyToken: paymentMethodToken}, body[keyData])

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(created))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateAccountPaymentMethod(t.Context(), request)

	mustNoError(t, err, "CreateAccountPaymentMethod should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 321, result.ID)
	checkEqual(t, paymentMethodCreditCard, result.Type)
	checkTrue(t, result.IsDefault)
}

func TestClientCreateAccountPaymentMethodAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/payment-methods", r.URL.Path, "request path should be /account/payment-methods")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.CreateAccountPaymentMethod(t.Context(), &linode.CreateAccountPaymentMethodRequest{Type: paymentMethodCreditCard, Data: map[string]any{keyToken: paymentMethodToken}, IsDefault: true})

	mustError(t, err, "CreateAccountPaymentMethod should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, "forbidden", apiErr.Message)
}

func TestClientCreateAccountPaymentMethodDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.CreateAccountPaymentMethod(t.Context(), &linode.CreateAccountPaymentMethodRequest{Type: paymentMethodCreditCard, Data: map[string]any{keyToken: paymentMethodToken}, IsDefault: true})

	mustError(t, err, "CreateAccountPaymentMethod should return the transient error")
	checkEqual(t, int32(1), requestCount.Load(), "mutating payment method creation must not be retried")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/invoices", r.URL.Path, "request path should be /account/invoices")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(invoices))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountInvoices(t.Context(), 2, 25)

	mustNoError(t, err, "ListAccountInvoices should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 2, result.Page)
	mustLen(t, result.Data, 1)
	checkEqual(t, 987, result.Data[0].ID)
	checkEqual(t, "Invoice 987", result.Data[0].Label)
	checkInEpsilon(t, 42.50, result.Data[0].Total, 0.0001)
}

// TestClientListAccountInvoicesAPIError verifies ListAccountInvoices propagates API errors.
func TestClientListAccountInvoicesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/invoices", r.URL.Path, "request path should be /account/invoices")
		checkEmpty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountInvoices(t.Context(), 0, 0)

	mustError(t, err, "ListAccountInvoices should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/invoices", r.URL.Path, "request path should be /account/invoices")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountInvoice]{
			Data: []linode.AccountInvoice{{ID: 987, Label: "Invoice 987"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountInvoices(t.Context(), 0, 0)

	mustNoError(t, err, "ListAccountInvoices should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	mustLen(t, result.Data, 1)
	checkEqual(t, 987, result.Data[0].ID)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/payments", r.URL.Path, "request path should be /account/payments")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(payments))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountPayments(t.Context(), 2, 25)

	mustNoError(t, err, "ListAccountPayments should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 2, result.Page)
	mustLen(t, result.Data, 1)
	checkEqual(t, 654, result.Data[0].ID)
	checkInEpsilon(t, 20.25, result.Data[0].USD, 0.0001)
}

// TestClientListAccountPaymentsAPIError verifies ListAccountPayments propagates API errors.
func TestClientListAccountPaymentsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/payments", r.URL.Path, "request path should be /account/payments")
		checkEmpty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountPayments(t.Context(), 0, 0)

	mustError(t, err, "ListAccountPayments should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/payments", r.URL.Path, "request path should be /account/payments")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountPayment]{
			Data: []linode.AccountPayment{{ID: 654, USD: 20.25}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountPayments(t.Context(), 0, 0)

	mustNoError(t, err, "ListAccountPayments should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	mustLen(t, result.Data, 1)
	checkEqual(t, 654, result.Data[0].ID)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

func TestClientGetAccountPaymentSuccess(t *testing.T) {
	t.Parallel()

	payment := linode.AccountPayment{ID: 654, Date: "2024-02-01T00:00:00", USD: 20.25}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/payments/654", r.URL.Path, "request path should include payment ID")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(payment))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountPayment(t.Context(), 654)

	mustNoError(t, err, "GetAccountPayment should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 654, result.ID)
	checkInEpsilon(t, 20.25, result.USD, 0.0001)
}

func TestClientGetAccountPaymentAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/payments/654", r.URL.Path, "request path should include payment ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountPayment(t.Context(), 654)

	mustError(t, err, "GetAccountPayment should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
}

func TestClientGetAccountPaymentRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/payments/654", r.URL.Path, "request path should include payment ID")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountPayment{ID: 654, USD: 20.25}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountPayment(t.Context(), 654)

	mustNoError(t, err, "GetAccountPayment should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 654, result.ID)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

func TestClientListAccountEntityTransfersRemoved(t *testing.T) {
	t.Parallel()

	_, ok := reflect.TypeFor[*linode.Client]().MethodByName("ListAccountEntityTransfers")
	checkFalse(t, ok, "deprecated account entity transfer list client method should be removed")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/service-transfers", r.URL.Path, "request path should be /account/service-transfers")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(transfers))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountServiceTransfers(t.Context(), 2, 25)

	mustNoError(t, err, "ListAccountServiceTransfers should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 2, result.Page)
	mustLen(t, result.Data, 1)
	checkEqual(t, accountEntityTransferToken, result.Data[0].Token)
	checkEqual(t, "pending", result.Data[0].Status)
	checkEqual(t, []int{111, 222}, result.Data[0].Entities.Linodes)
}

func TestClientListAccountServiceTransfersAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/service-transfers", r.URL.Path, "request path should be /account/service-transfers")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountServiceTransfers(t.Context(), 0, 0)

	mustError(t, err, "ListAccountServiceTransfers should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
}

func TestClientListAccountServiceTransfersRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/service-transfers", r.URL.Path, "request path should be /account/service-transfers")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountEntityTransfer]{
			Data: []linode.AccountEntityTransfer{{Token: accountEntityTransferToken, Status: "pending"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountServiceTransfers(t.Context(), 0, 0)

	mustNoError(t, err, "ListAccountServiceTransfers should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	mustLen(t, result.Data, 1)
	checkEqual(t, accountEntityTransferToken, result.Data[0].Token)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/service-transfers/service-token-example", r.URL.Path, "request path should include transfer token")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetAccountServiceTransfer(t.Context(), accountServiceTransferToken)

	mustNoError(t, err, "GetAccountServiceTransfer should succeed on 200 response")
	mustNotNil(t, got, "result should not be nil")
	checkEqual(t, accountServiceTransferToken, got.Token)
	checkEqual(t, statusPending, got.Status)
	checkEqual(t, []int{111, 222}, got.Entities.Linodes)
}

// TestClientGetAccountServiceTransferEscapesToken verifies the client encodes path separators.
func TestClientGetAccountServiceTransferEscapesToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/account/service-transfers/service%2Ftoken%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		checkEmpty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountEntityTransfer{Token: "service/token?query"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountServiceTransfer(t.Context(), "service/token?query")

	mustNoError(t, err, "GetAccountServiceTransfer should escape path parameters")
}

// TestClientGetAccountServiceTransferAPIError verifies GetAccountServiceTransfer propagates API errors.
func TestClientGetAccountServiceTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/service-transfers/service-token-example", r.URL.Path, "request path should include transfer token")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountServiceTransfer(t.Context(), accountServiceTransferToken)

	mustError(t, err, "GetAccountServiceTransfer should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/service-transfers/service-token-example", r.URL.Path, "request path should include transfer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountEntityTransfer{Token: accountServiceTransferToken, Status: statusPending}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountServiceTransfer(t.Context(), accountServiceTransferToken)

	mustNoError(t, err, "GetAccountServiceTransfer should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, accountServiceTransferToken, result.Token)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

// TestClientDeleteAccountServiceTransferSuccess verifies DeleteAccountServiceTransfer sends a DELETE
// request to /account/service-transfers/{token} with no body.
func TestClientDeleteAccountServiceTransferSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/account/service-transfers/service-token-example", r.URL.Path, "request path should include transfer token")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, http.NoBody, r.Body, "DELETE request should not send a body")

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountServiceTransfer(t.Context(), accountServiceTransferToken)

	mustNoError(t, err, "DeleteAccountServiceTransfer should succeed on 200 response")
}

// TestClientDeleteAccountServiceTransferEscapesToken verifies the client encodes path separators.
func TestClientDeleteAccountServiceTransferEscapesToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/account/service-transfers/service%2Ftoken%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		checkEmpty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountServiceTransfer(t.Context(), "service/token?query")

	mustNoError(t, err, "DeleteAccountServiceTransfer should escape path parameters")
}

// TestClientDeleteAccountServiceTransferAPIError verifies DeleteAccountServiceTransfer propagates API errors.
func TestClientDeleteAccountServiceTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, "/account/service-transfers/service-token-example", r.URL.Path, "request path should include transfer token")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountServiceTransfer(t.Context(), accountServiceTransferToken)

	mustError(t, err, "DeleteAccountServiceTransfer should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteAccountServiceTransfer(t.Context(), accountServiceTransferToken)

	mustError(t, err, "DeleteAccountServiceTransfer should return the transient error")
	checkEqual(t, int32(1), requestCount.Load(), "mutating transfer cancellation must not be retried")
}

// TestClientAcceptAccountServiceTransferSuccess verifies AcceptAccountServiceTransfer sends a POST
// request to /account/service-transfers/{token}/accept with no body.
func TestClientAcceptAccountServiceTransferSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/service-transfers/service-token-example/accept", r.URL.Path, "request path should include transfer token accept action")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, http.NoBody, r.Body, "POST accept request should not send a body")

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.AcceptAccountServiceTransfer(t.Context(), accountServiceTransferToken)

	mustNoError(t, err, "AcceptAccountServiceTransfer should succeed on 200 response")
}

func TestClientAcceptAccountServiceTransferEscapesToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/service-transfers/service%2Ftoken%3Fquery/accept", r.URL.EscapedPath(), "path parameter should be escaped")
		checkEmpty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.AcceptAccountServiceTransfer(t.Context(), "service/token?query")

	mustNoError(t, err, "AcceptAccountServiceTransfer should escape path parameters")
}

func TestClientAcceptAccountServiceTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/service-transfers/service-token-example/accept", r.URL.Path, "request path should include transfer token accept action")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.AcceptAccountServiceTransfer(t.Context(), accountServiceTransferToken)

	mustError(t, err, "AcceptAccountServiceTransfer should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
}

func TestClientAcceptAccountServiceTransferDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.AcceptAccountServiceTransfer(t.Context(), accountServiceTransferToken)

	mustError(t, err, "AcceptAccountServiceTransfer should return the transient error")
	checkEqual(t, int32(1), requestCount.Load(), "mutating service transfer acceptance must not be retried")
}

func TestClientAccountEntityTransferAcceptRouteRemoved(t *testing.T) {
	t.Parallel()

	_, exists := reflect.TypeFor[*linode.Client]().MethodByName("AcceptAccountEntityTransfer")
	checkFalse(t, exists, "deprecated account entity-transfer accept client method should not be exposed")
}

func TestClientDeleteAccountEntityTransferDeprecatedRouteRemoved(t *testing.T) {
	t.Parallel()

	_, exists := reflect.TypeFor[*linode.Client]().MethodByName("DeleteAccountEntityTransfer")
	checkFalse(t, exists, "deprecated account entity-transfer delete client method should not be exposed")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/invoices/12345", r.URL.Path, "request path should include invoice id")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(invoice))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountInvoice(t.Context(), accountInvoiceID)

	mustNoError(t, err, "GetAccountInvoice should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, accountInvoiceID, result.ID)
	checkEqual(t, "Invoice #12345", result.Label)
	checkInDelta(t, 11.00, result.Total, 0.001)
}

// TestClientGetAccountInvoiceAPIError verifies GetAccountInvoice propagates API errors.
func TestClientGetAccountInvoiceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/invoices/12345", r.URL.Path, "request path should include invoice id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountInvoice(t.Context(), accountInvoiceID)

	mustError(t, err, "GetAccountInvoice should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/users", r.URL.Path, "request path should be /account/users")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(users))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountUsers(t.Context(), 2, 25)

	mustNoError(t, err, "ListAccountUsers should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 2, result.Page)
	mustLen(t, result.Data, 1)
	checkEqual(t, accountLoginUsername, result.Data[0].Username)
	checkEqual(t, accountUserEmail, result.Data[0].Email)
	checkTrue(t, result.Data[0].Restricted)
	checkTrue(t, result.Data[0].TFAEnabled)
}

// TestClientListAccountUsersAPIError verifies ListAccountUsers propagates API errors.
func TestClientListAccountUsersAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/users", r.URL.Path, "request path should be /account/users")
		checkEmpty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountUsers(t.Context(), 0, 0)

	mustError(t, err, "ListAccountUsers should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/users", r.URL.Path, "request path should be /account/users")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountUser]{
			Data: []linode.AccountUser{{Username: accountLoginUsername, Email: accountUserEmail}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountUsers(t.Context(), 0, 0)

	mustNoError(t, err, "ListAccountUsers should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	mustLen(t, result.Data, 1)
	checkEqual(t, accountLoginUsername, result.Data[0].Username)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/users/"+accountLoginUsername, r.URL.Path, "request path should include username")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(user))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountUser(t.Context(), accountLoginUsername)

	mustNoError(t, err, "GetAccountUser should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, accountLoginUsername, result.Username)
	checkEqual(t, accountUserEmail, result.Email)
	checkTrue(t, result.Restricted)
	checkTrue(t, result.TFAEnabled)
}

// TestClientGetAccountUserEscapesUsername verifies the client encodes path separators.
func TestClientGetAccountUserEscapesUsername(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/account/users/user%2Fname%3Fquery", r.URL.EscapedPath())
		checkEmpty(t, r.URL.RawQuery, "escaped username must not create a query string")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountUser{Username: "user/name?query"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountUser(t.Context(), "user/name?query")

	mustNoError(t, err, "GetAccountUser should escape path parameters")
}

// TestClientGetAccountUserAPIError verifies GetAccountUser propagates API errors.
func TestClientGetAccountUserAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/users/"+accountLoginUsername, r.URL.Path, "request path should include username")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountUser(t.Context(), accountLoginUsername)

	mustError(t, err, "GetAccountUser should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/users/"+accountLoginUsername, r.URL.Path, "request path should include username")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountUser{Username: accountLoginUsername, Email: accountUserEmail}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountUser(t.Context(), accountLoginUsername)

	mustNoError(t, err, "GetAccountUser should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, accountLoginUsername, result.Username)
	checkEqual(t, int32(2), requestCount.Load(), "request should be retried once")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/users/"+accountLoginUsername+"/grants", r.URL.Path, "request path should include username grants")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(grants))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountUserGrants(t.Context(), accountLoginUsername)

	mustNoError(t, err, "GetAccountUserGrants should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, linode.GrantPermission("read_only"), result.Global.AccountAccess)
	mustLen(t, result.Linode, 1)
	checkEqual(t, 123, result.Linode[0].ID)
}

// TestClientGetAccountUserGrantsEscapesUsername verifies the client encodes path separators.
func TestClientGetAccountUserGrantsEscapesUsername(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/account/users/user%2Fname%3Fquery/grants", r.URL.EscapedPath())
		checkEmpty(t, r.URL.RawQuery, "escaped username must not create a query string")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.Grants{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountUserGrants(t.Context(), "user/name?query")

	mustNoError(t, err, "GetAccountUserGrants should escape path parameters")
}

// TestClientGetAccountUserGrantsAPIError verifies GetAccountUserGrants propagates API errors.
func TestClientGetAccountUserGrantsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/users/"+accountLoginUsername+"/grants", r.URL.Path, "request path should include username grants")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountUserGrants(t.Context(), accountLoginUsername)

	mustError(t, err, "GetAccountUserGrants should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/users/"+accountLoginUsername+"/grants", r.URL.Path, "request path should include username grants")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.Grants{Global: linode.GlobalGrants{AccountAccess: linode.GrantPermission("read_only")}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountUserGrants(t.Context(), accountLoginUsername)

	mustNoError(t, err, "GetAccountUserGrants should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, linode.GrantPermission("read_only"), result.Global.AccountAccess)
	checkEqual(t, int32(2), requestCount.Load(), "request should be retried once")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/logins", r.URL.Path, "request path should be /account/logins")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(logins))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountLogins(t.Context(), 2, 25)

	mustNoError(t, err, "ListAccountLogins should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 2, result.Page)
	mustLen(t, result.Data, 1)
	checkEqual(t, 123, result.Data[0].ID)
	checkEqual(t, accountLoginUsername, result.Data[0].Username)
	checkEqual(t, accountLoginIP, result.Data[0].IP)
}

// TestClientListAccountLoginsAPIError verifies ListAccountLogins propagates API errors.
func TestClientListAccountLoginsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/logins", r.URL.Path, "request path should be /account/logins")
		checkEmpty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountLogins(t.Context(), 0, 0)

	mustError(t, err, "ListAccountLogins should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/logins", r.URL.Path, "request path should be /account/logins")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountLogin]{
			Data: []linode.AccountLogin{{ID: 123, Username: accountLoginUsername}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountLogins(t.Context(), 0, 0)

	mustNoError(t, err, "ListAccountLogins should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	mustLen(t, result.Data, 1)
	checkEqual(t, 123, result.Data[0].ID)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/logins/123", r.URL.Path, "request path should include profile login ID")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(login))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetProfileLogin(t.Context(), 123)

	mustNoError(t, err, "GetProfileLogin should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 123, result.ID)
	checkEqual(t, accountLoginUsername, result.Username)
	checkEqual(t, accountLoginIP, result.IP)
}

// TestClientGetProfileLoginAPIError verifies GetProfileLogin propagates API errors.
func TestClientGetProfileLoginAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/logins/123", r.URL.Path, "request path should include profile login ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileLogin(t.Context(), 123)

	mustError(t, err, "GetProfileLogin should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/profile/logins/123", r.URL.Path, "request path should include profile login ID")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountLogin{ID: 123, Username: accountLoginUsername}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetProfileLogin(t.Context(), 123)

	mustNoError(t, err, "GetProfileLogin should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 123, result.ID)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/logins/123", r.URL.Path, "request path should include account login ID")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(login))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountLogin(t.Context(), 123)

	mustNoError(t, err, "GetAccountLogin should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 123, result.ID)
	checkEqual(t, accountLoginUsername, result.Username)
	checkEqual(t, accountLoginIP, result.IP)
}

// TestClientGetAccountLoginAPIError verifies GetAccountLogin propagates API errors.
func TestClientGetAccountLoginAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/logins/123", r.URL.Path, "request path should include account login ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountLogin(t.Context(), 123)

	mustError(t, err, "GetAccountLogin should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/logins/123", r.URL.Path, "request path should include account login ID")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountLogin{ID: 123, Username: accountLoginUsername}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountLogin(t.Context(), 123)

	mustNoError(t, err, "GetAccountLogin should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 123, result.ID)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/invoices/12345/items", r.URL.Path, "request path should include invoice id and items")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(items))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountInvoiceItems(t.Context(), accountInvoiceID, 2, 25)

	mustNoError(t, err, "ListAccountInvoiceItems should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 2, result.Page)
	mustLen(t, result.Data, 1)
	checkEqual(t, "Nanode 1GB", result.Data[0].Label)
	checkInDelta(t, 5.00, result.Data[0].Total, 0.001)
}

// TestClientListAccountInvoiceItemsAPIError verifies ListAccountInvoiceItems propagates API errors.
func TestClientListAccountInvoiceItemsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/invoices/12345/items", r.URL.Path, "request path should include invoice id and items")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountInvoiceItems(t.Context(), accountInvoiceID, 0, 0)

	mustError(t, err, "ListAccountInvoiceItems should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
}

// TestClientListAccountInvoiceItemsRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListAccountInvoiceItemsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/invoices/12345/items", r.URL.Path, "request path should include invoice id and items")

		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountInvoiceItem]{
			Data: []linode.AccountInvoiceItem{{Label: "Nanode 1GB", Total: 5.00}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountInvoiceItems(t.Context(), accountInvoiceID, 0, 0)

	mustNoError(t, err, "ListAccountInvoiceItems should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	mustLen(t, result.Data, 1)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/invoices/12345", r.URL.Path, "request path should include invoice id")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountInvoice{ID: accountInvoiceID, Label: "Invoice #12345"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountInvoice(t.Context(), accountInvoiceID)

	mustNoError(t, err, "GetAccountInvoice should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, accountInvoiceID, result.ID)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56", r.URL.Path, "request path should include child account euuid")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(childAccount))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountChildAccount(t.Context(), childAccountEUUID)

	mustNoError(t, err, "GetAccountChildAccount should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, childAccountEUUID, result.EUUID)
	checkEqual(t, companyAcme, result.Company)
	checkEqual(t, "11/2024", result.CreditCard.Expiry)
	checkEqual(t, "0111", result.CreditCard.LastFour)
}

// TestClientGetAccountChildAccountEscapesEUUID verifies the client encodes path separators.
func TestClientGetAccountChildAccountEscapesEUUID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/account/child-accounts/child%2Faccount%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ChildAccount{EUUID: "child/account?query"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountChildAccount(t.Context(), "child/account?query")

	mustNoError(t, err, "GetAccountChildAccount should escape path parameters")
}

// TestClientGetAccountChildAccountAPIError verifies GetAccountChildAccount propagates API errors.
func TestClientGetAccountChildAccountAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56", r.URL.Path, "request path should include child account euuid")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountChildAccount(t.Context(), childAccountEUUID)

	mustError(t, err, "GetAccountChildAccount should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56", r.URL.Path, "request path should include child account euuid")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ChildAccount{EUUID: childAccountEUUID, Company: companyAcme}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountChildAccount(t.Context(), childAccountEUUID)

	mustNoError(t, err, "GetAccountChildAccount should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, childAccountEUUID, result.EUUID)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/service-transfers", r.URL.Path, "request path should be /account/service-transfers")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got linode.CreateAccountServiceTransferRequest
		checkNoError(t, json.NewDecoder(r.Body).Decode(&got))
		checkEqual(t, []int{123, 456}, got.Entities.Linodes)

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateAccountServiceTransferRequest{Entities: linode.AccountEntityTransferEntities{Linodes: []int{123, 456}}}

	got, err := client.CreateAccountServiceTransfer(t.Context(), req)

	mustNoError(t, err, "CreateAccountServiceTransfer should succeed on 200 response")
	mustNotNil(t, got, "result should not be nil")
	checkEqual(t, statusPending, got.Status)
	checkEqual(t, "service-transfer-token", got.Token)
	checkEqual(t, []int{123, 456}, got.Entities.Linodes)
}

func TestClientCreateAccountServiceTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/service-transfers", r.URL.Path, "request path should be /account/service-transfers")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateAccountServiceTransferRequest{Entities: linode.AccountEntityTransferEntities{Linodes: []int{123}}}

	got, err := client.CreateAccountServiceTransfer(t.Context(), req)

	mustError(t, err, "CreateAccountServiceTransfer should return API error")
	checkNil(t, got, "result should be nil on API error")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
}

func TestClientCreateAccountServiceTransferDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	req := &linode.CreateAccountServiceTransferRequest{Entities: linode.AccountEntityTransferEntities{Linodes: []int{123}}}

	_, err := client.CreateAccountServiceTransfer(t.Context(), req)

	mustError(t, err, "CreateAccountServiceTransfer should return the transient error")
	checkEqual(t, int32(1), requestCount.Load(), "mutating service transfer creation must not be retried")
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
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56/token", r.URL.Path, "request path should include child account euuid and token suffix")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, http.NoBody, r.Body, "token creation should not send a request body")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(proxyToken))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateAccountChildAccountToken(t.Context(), childAccountEUUID)

	mustNoError(t, err, "CreateAccountChildAccountToken should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, 918, result.ID)
	checkEqual(t, "parent1_1234_2024-05-01T00:01:01", result.Label)
	checkEqual(t, "*", result.Scopes)
	checkEqual(t, "abcdefghijklmnop", result.Token)
	checkEqual(t, "2024-05-01T00:16:01", result.Expiry)
}

// TestClientCreateAccountChildAccountTokenEscapesEUUID verifies the client encodes path separators.
func TestClientCreateAccountChildAccountTokenEscapesEUUID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/account/child-accounts/child%2Faccount%3Fquery/token", r.URL.EscapedPath(), "path parameter should be escaped")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ProxyUserToken{Token: "proxy-token"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.CreateAccountChildAccountToken(t.Context(), "child/account?query")

	mustNoError(t, err, "CreateAccountChildAccountToken should escape path parameters")
}

// TestClientCreateAccountChildAccountTokenAPIError verifies API errors propagate.
func TestClientCreateAccountChildAccountTokenAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56/token", r.URL.Path, "request path should include child account euuid and token suffix")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.CreateAccountChildAccountToken(t.Context(), childAccountEUUID)

	mustError(t, err, "CreateAccountChildAccountToken should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.CreateAccountChildAccountToken(t.Context(), childAccountEUUID)

	mustError(t, err, "CreateAccountChildAccountToken should return the transient error")
	checkEqual(t, int32(1), requestCount.Load(), "mutating token creation must not be retried")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/betas/"+betaExampleOpen, r.URL.Path, "request path should include beta id")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(beta))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountBeta(t.Context(), betaExampleOpen)

	mustNoError(t, err, "GetAccountBeta should succeed on 200 response")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, betaExampleOpen, result.ID)
	checkEqual(t, labelExampleOpenBeta, result.Label)
	mustNotNil(t, result.Description)
	checkEqual(t, description, *result.Description)
}

// TestClientGetAccountBetaEscapesID verifies the client encodes path separators.
func TestClientGetAccountBetaEscapesID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/account/betas/example%2Fopen%3Fquery", r.URL.EscapedPath(), "request path should escape beta id")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountBetaProgram{ID: "example/open?query"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountBeta(t.Context(), "example/open?query")

	mustNoError(t, err, "GetAccountBeta should escape path parameters")
}

// TestClientGetAccountBetaAPIError verifies GetAccountBeta propagates API errors.
func TestClientGetAccountBetaAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/betas/"+betaExampleOpen, r.URL.Path, "request path should include beta id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountBeta(t.Context(), betaExampleOpen)

	mustError(t, err, "GetAccountBeta should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
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
			checkNoError(t, writeErr)

			return
		}

		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/account/betas/"+betaExampleOpen, r.URL.Path, "request path should include beta id")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountBetaProgram{ID: betaExampleOpen, Label: labelExampleOpenBeta}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountBeta(t.Context(), betaExampleOpen)

	mustNoError(t, err, "GetAccountBeta should succeed after retry")
	mustNotNil(t, result, "result should not be nil")
	checkEqual(t, betaExampleOpen, result.ID)
	checkEqual(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

// TestClientEnrollAccountBetaSuccess verifies EnrollAccountBeta sends a POST
// request to /account/betas with the exact body.
func TestClientEnrollAccountBetaSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/betas", r.URL.Path, "request path should be /account/betas")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		checkNoError(t, json.NewDecoder(r.Body).Decode(&body))
		checkEqual(t, betaExampleOpen, body["id"])

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.EnrollAccountBeta(t.Context(), &linode.EnrollAccountBetaRequest{ID: betaExampleOpen})

	mustNoError(t, err, "EnrollAccountBeta should succeed on 200 response")
}

// TestClientEnrollAccountBetaAPIError verifies EnrollAccountBeta propagates API errors.
func TestClientEnrollAccountBetaAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/betas", r.URL.Path, "request path should be /account/betas")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.EnrollAccountBeta(t.Context(), &linode.EnrollAccountBetaRequest{ID: betaExampleOpen})

	mustError(t, err, "EnrollAccountBeta should fail on 403 response")

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr, "error should wrap APIError")
	mustNotNil(t, apiErr, "APIError should be present")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
	checkEqual(t, errForbidden, apiErr.Message)
}

// TestClientEnrollAccountBetaDoesNotRetry verifies the mutating beta enrollment
// is not replayed after a transient HTTP error.
func TestClientEnrollAccountBetaDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/betas", r.URL.Path, "request path should be /account/betas")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		checkNoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.EnrollAccountBeta(t.Context(), &linode.EnrollAccountBetaRequest{ID: betaExampleOpen})

	mustError(t, err, "EnrollAccountBeta should fail on 500 response")
	checkEqual(t, int32(1), calls.Load(), "EnrollAccountBeta must not retry and replay a mutating request")
}

// TestClientAcknowledgeAccountAgreementsSuccess verifies that
// AcknowledgeAccountAgreements sends a POST request to /account/agreements with
// the exact body and returns the agreement statuses.
func TestClientAcknowledgeAccountAgreementsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/agreements", r.URL.Path, "request path should be /account/agreements")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		checkNoError(t, json.NewDecoder(r.Body).Decode(&body))
		checkEqual(t, true, body["billing_agreement"])
		checkEqual(t, true, body["eu_model"])
		checkEqual(t, true, body["master_service_agreement"])
		checkNotContains(t, body, "privacy_policy", "omitted fields should not be sent")

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		checkNoError(t, writeErr)
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

	mustNoError(t, err, "AcknowledgeAccountAgreements should succeed on 200 response")
}

// TestClientAcknowledgeAccountAgreementsDoesNotRetry verifies the mutating
// agreement acknowledgement is not replayed after a transient HTTP error.
func TestClientAcknowledgeAccountAgreementsDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/agreements", r.URL.Path, "request path should be /account/agreements")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		checkNoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	privacyPolicy := true
	err := client.AcknowledgeAccountAgreements(t.Context(), &linode.AcknowledgeAccountAgreementsRequest{PrivacyPolicy: &privacyPolicy})

	mustError(t, err, "AcknowledgeAccountAgreements should fail on 500 response")
	checkEqual(t, int32(1), calls.Load(), "AcknowledgeAccountAgreements must not retry and replay a mutating request")
}

// TestClientCancelAccountSuccess verifies CancelAccount sends a POST request to
// /account/cancel with the exact body and returns the survey link.
func TestClientCancelAccountSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/cancel", r.URL.Path, "request path should be /account/cancel")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		checkNoError(t, json.NewDecoder(r.Body).Decode(&body))
		checkEqual(t, "moving providers", body["comments"])

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{"survey_link":"https://example.test/survey"}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	comments := "moving providers"
	response, err := client.CancelAccount(t.Context(), &linode.CancelAccountRequest{Comments: &comments})

	mustNoError(t, err, "CancelAccount should succeed on 200 response")
	mustNotNil(t, response, "response should not be nil")
	checkEqual(t, "https://example.test/survey", response.SurveyLink)
}

// TestClientCancelAccountWithoutComments verifies comments are optional.
func TestClientCancelAccountWithoutComments(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/cancel", r.URL.Path, "request path should be /account/cancel")

		var body map[string]any
		checkNoError(t, json.NewDecoder(r.Body).Decode(&body))
		checkEmpty(t, body, "omitted comments should send an empty JSON object")

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{"survey_link":"https://example.test/survey"}`))
		checkNoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	response, err := client.CancelAccount(t.Context(), &linode.CancelAccountRequest{})

	mustNoError(t, err, "CancelAccount should succeed without comments")
	mustNotNil(t, response, "response should not be nil")
	checkEqual(t, "https://example.test/survey", response.SurveyLink)
}

// TestClientCancelAccountDoesNotRetry verifies account cancellation is not
// replayed after a transient HTTP error.
func TestClientCancelAccountDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/cancel", r.URL.Path, "request path should be /account/cancel")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		checkNoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	errComments := "temporary"
	_, err := client.CancelAccount(t.Context(), &linode.CancelAccountRequest{Comments: &errComments})

	mustError(t, err, "CancelAccount should fail on 500 response")
	checkEqual(t, int32(1), calls.Load(), "CancelAccount must not retry and replay a destructive request")
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
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/account", r.URL.Path, "request path should be /account")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		checkNoError(t, json.NewDecoder(r.Body).Decode(&body))
		checkEqual(t, "updated@example.com", body["email"])
		checkEqual(t, "Updated", body["first_name"])
		checkEqual(t, "123 Main St", body["address_1"])
		checkNotContains(t, body, "address_2", "omitted fields should not be sent")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(updatedAccount))
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

	mustNoError(t, err, "UpdateAccount should succeed on 200 response")
	checkEqual(t, "updated@example.com", result.Email)
	checkEqual(t, "Updated", result.FirstName)
	checkEqual(t, "123 Main St", result.Address1)
}

// TestClientUpdateAccountNetworkError verifies that UpdateAccount returns a
// NetworkError when the HTTP request fails to reach the server.
func TestClientUpdateAccountNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateAccount(t.Context(), &linode.UpdateAccountRequest{})

	mustError(t, err, "UpdateAccount should fail when the server is unreachable")

	var netErr *linode.NetworkError

	checkErrorAs(t, err, &netErr, "error should be a NetworkError")
}

// TestClientUpdateAccountAPIError verifies that UpdateAccount propagates
// API errors through the handleResponse error chain.
func TestClientUpdateAccountAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"field":"email","reason":"invalid email format"}]}`))
		checkNoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateAccount(t.Context(), &linode.UpdateAccountRequest{})

	mustError(t, err, "UpdateAccount should fail on 400 response")

	var apiErr *linode.APIError

	mustErrorAs(t, err, &apiErr, "error should be an APIError")
	checkEqual(t, http.StatusBadRequest, apiErr.StatusCode)
}

// TestClientUpdateAccountDoesNotRetry verifies the mutating account update is
// not replayed after a transient HTTP error.
func TestClientUpdateAccountDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/account", r.URL.Path, "request path should be /account")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		checkNoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateAccount(t.Context(), &linode.UpdateAccountRequest{})

	mustError(t, err, "UpdateAccount should fail on 500 response")
	checkEqual(t, int32(1), calls.Load(), "UpdateAccount must not retry and replay a mutating request")
}

// TestClientEnableAccountManagedSuccess verifies that EnableAccountManaged sends a POST
// request to /account/settings/managed-enable with no query parameters or body.
func TestClientEnableAccountManagedSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/settings/managed-enable", r.URL.Path, "request path should be /account/settings/managed-enable")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		checkNoError(t, err)
		checkEmpty(t, body, "request should not include a body")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.EnableAccountManaged(t.Context())

	mustNoError(t, err, "EnableAccountManaged should succeed on 200 response")
}

// TestClientEnableAccountManagedNetworkError verifies that EnableAccountManaged returns a
// NetworkError when the HTTP request fails to reach the server.
func TestClientEnableAccountManagedNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(3))

	err := client.EnableAccountManaged(t.Context())

	mustError(t, err, "EnableAccountManaged should fail when the server is unreachable")

	var netErr *linode.NetworkError

	checkErrorAs(t, err, &netErr, "error should be a NetworkError")
}

// TestClientEnableAccountManagedAPIError verifies that EnableAccountManaged propagates
// API errors through the handleResponse error chain.
func TestClientEnableAccountManagedAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/settings/managed-enable", r.URL.Path, "request path should be /account/settings/managed-enable")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"reason":"managed could not be enabled"}]}`))
		checkNoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.EnableAccountManaged(t.Context())

	mustError(t, err, "EnableAccountManaged should fail on 400 response")

	var apiErr *linode.APIError

	mustErrorAs(t, err, &apiErr, "error should be an APIError")
	checkEqual(t, http.StatusBadRequest, apiErr.StatusCode)
}

// TestClientEnableAccountManagedDoesNotRetry verifies the mutating managed enable
// request is not replayed after a transient HTTP error.
func TestClientEnableAccountManagedDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/settings/managed-enable", r.URL.Path, "request path should be /account/settings/managed-enable")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		checkNoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.EnableAccountManaged(t.Context())

	mustError(t, err, "EnableAccountManaged should fail on 500 response")
	checkEqual(t, int32(1), calls.Load(), "EnableAccountManaged must not retry and replay a mutating request")
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
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/account/settings", r.URL.Path, "request path should be /account/settings")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		checkNoError(t, json.NewDecoder(r.Body).Decode(&body))
		checkEqual(t, true, body["backups_enabled"])
		checkEqual(t, false, body["network_helper"])
		checkEqual(t, maintenancePolicyMigrate, body["maintenance_policy"])
		checkNotContains(t, body, "object_storage", "omitted fields should not be sent")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(settings))
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

	mustNoError(t, err, "UpdateAccountSettings should succeed on 200 response")
	checkTrue(t, result.BackupsEnabled)
	checkFalse(t, result.NetworkHelper)
	checkEqual(t, maintenancePolicyMigrate, result.MaintenancePolicy)
}

// TestClientUpdateAccountSettingsAPIError verifies that UpdateAccountSettings propagates
// API errors through the handleResponse error chain.
func TestClientUpdateAccountSettingsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/account/settings", r.URL.Path, "request path should be /account/settings")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"field":"maintenance_policy","reason":"invalid maintenance policy"}]}`))
		checkNoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateAccountSettings(t.Context(), &linode.UpdateAccountSettingsRequest{})

	mustError(t, err, "UpdateAccountSettings should fail on 400 response")

	var apiErr *linode.APIError

	mustErrorAs(t, err, &apiErr, "error should be an APIError")
	checkEqual(t, http.StatusBadRequest, apiErr.StatusCode)
}

// TestClientUpdateAccountSettingsDoesNotRetry verifies the mutating account settings
// update is not replayed after a transient HTTP error.
func TestClientUpdateAccountSettingsDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/account/settings", r.URL.Path, "request path should be /account/settings")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		checkNoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateAccountSettings(t.Context(), &linode.UpdateAccountSettingsRequest{})

	mustError(t, err, "UpdateAccountSettings should fail on 500 response")
	checkEqual(t, int32(1), calls.Load(), "UpdateAccountSettings must not retry and replay a mutating request")
}

func TestClientListImageShareGroupTokensSuccess(t *testing.T) {
	t.Parallel()

	updated := "2025-08-04T11:09:09"
	expiry := "2025-09-04T10:09:09"
	tokens := []linode.ImageShareGroupToken{
		{
			TokenUUID:              "13428362-5458-4dad-b14b-8d0d4d648f8c",
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/images/sharegroups/tokens", r.URL.Path, "request path should be /images/sharegroups/tokens")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:   tokens,
			"page":    2,
			"pages":   3,
			"results": 7,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListImageShareGroupTokens(t.Context(), 2, 25)

	mustNoError(t, err)
	mustNotNil(t, result)
	mustLen(t, result.Data, 1)
	checkEqual(t, "Backend Services - Engineering", result.Data[0].Label)
	checkEqual(t, "13428362-5458-4dad-b14b-8d0d4d648f8c", result.Data[0].TokenUUID)
	checkEqual(t, 2, result.Page)
	checkEqual(t, 7, result.Results)
}

func TestClientListImageShareGroupTokensError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListImageShareGroupTokens(t.Context(), 1, 25)

	mustError(t, err)
	checkNil(t, result)
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/images/sharegroups", r.URL.Path, "request path should be /images/sharegroups")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    shareGroups,
			"page":    2,
			"pages":   3,
			"results": 7,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListImageShareGroups(t.Context(), 2, 25)

	mustNoError(t, err)
	mustNotNil(t, result)
	mustLen(t, result.Data, 1)
	checkEqual(t, imageShareGroupLabel, result.Data[0].Label)
	checkEqual(t, 2, result.Page)
	checkEqual(t, 7, result.Results)
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/images/sharegroups/123", r.URL.Path, "request path should include share group ID")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(shareGroup))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetImageShareGroup(t.Context(), 123)

	mustNoError(t, err)
	mustNotNil(t, result)
	checkEqual(t, 123, result.ID)
	checkEqual(t, imageShareGroupLabel, result.Label)
}

func TestClientGetImageShareGroupAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/images/sharegroups/123", r.URL.Path, "request path should include share group ID")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]any{{keyReason: "temporary share group failure"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetImageShareGroup(t.Context(), 123)

	mustError(t, err)
	checkNil(t, result)

	var apiErr *linode.APIError
	mustErrorAs(t, err, &apiErr)
	checkEqual(t, "temporary share group failure", apiErr.Message)
}

func TestClientGetImageShareGroupNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "token", nil, linode.WithMaxRetries(0))
	_, err := client.GetImageShareGroup(t.Context(), 123)

	mustError(t, err, "GetImageShareGroup should fail when server is unreachable")

	var netErr *linode.NetworkError
	mustErrorAs(t, err, &netErr, "error should be a NetworkError")
	checkEqual(t, "GetImageShareGroup", netErr.Operation)
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/images/sharegroups/123", r.URL.Path, "request path should include share group ID")

		if requestCount.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]any{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(shareGroup))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.GetImageShareGroup(t.Context(), 123)

	mustNoError(t, err)
	mustNotNil(t, result)
	checkEqual(t, int32(2), requestCount.Load())
	checkEqual(t, imageShareGroupLabel, result.Label)
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
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/images/sharegroups", r.URL.Path, "request path should be /images/sharegroups")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"))

		var body map[string]any
		if !checkNoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}

		checkEqual(t, imageShareGroupLabel, body[keyLabel])
		checkEqual(t, description, body[keyDescription])

		if !checkLen(t, body["images"], 1) {
			return
		}

		image, ok := body["images"].([]any)[0].(map[string]any)
		if !checkTrue(t, ok, "image payload should be an object") {
			return
		}

		checkEqual(t, "private/7", image[keyID])
		checkEqual(t, "Linux Debian", image["label"])

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroup{
			ID:           1,
			UUID:         shareGroupUUIDExample,
			Label:        imageShareGroupLabel,
			Description:  &description,
			IsSuspended:  false,
			Created:      shareGroupCreatedFixture,
			Updated:      &updated,
			ImagesCount:  1,
			MembersCount: 0,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.CreateImageShareGroup(t.Context(), request)

	mustNoError(t, err)
	mustNotNil(t, result)
	checkEqual(t, imageShareGroupLabel, result.Label)
	checkEqual(t, 1, result.ImagesCount)
}

func TestClientCreateImageShareGroupAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "label is required"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.CreateImageShareGroup(t.Context(), &linode.CreateImageShareGroupRequest{Label: imageShareGroupLabel})

	mustError(t, err, "CreateImageShareGroup should return API errors")
	checkErrorContains(t, err, "label is required")
}

func TestClientCreateImageShareGroupNetworkError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := srv.URL
	srv.Close()

	client := linode.NewClient(baseURL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.CreateImageShareGroup(t.Context(), &linode.CreateImageShareGroupRequest{Label: imageShareGroupLabel})

	mustError(t, err, "CreateImageShareGroup should wrap network errors")

	var networkErr *linode.NetworkError
	mustErrorAs(t, err, &networkErr, "network error should wrap as NetworkError")
	checkEqual(t, "CreateImageShareGroup", networkErr.Operation)
}

func TestClientCreateImageShareGroupDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	_, err := client.CreateImageShareGroup(t.Context(), &linode.CreateImageShareGroupRequest{Label: imageShareGroupLabel})

	mustError(t, err, "CreateImageShareGroup should fail on 500 response")
	checkEqual(t, int32(1), calls.Load(), "CreateImageShareGroup must not retry and replay a mutating request")
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
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/images/upload", r.URL.Path, "request path should be /images/upload")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer "+"test-token", r.Header.Get("Authorization"))

		var body map[string]any
		if !checkNoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode") {
			return
		}

		checkEqual(t, uploadImageLabelFixture, body[keyLabel])
		checkEqual(t, regionUSEast, body["region"])
		checkEqual(t, "custom upload", body[keyDescription])
		checkEqual(t, true, body["cloud_init"])
		checkEqual(t, []any{uploadImageTagProd, uploadImageTagWeb}, body["tags"])

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			"image": linode.Image{
				ID:          "private/99",
				Label:       uploadImageLabelFixture,
				Description: "custom upload",
				Status:      uploadImageStatusFixture,
				Tags:        []string{uploadImageTagProd, uploadImageTagWeb},
			},
			"upload_to": uploadImageTargetFixture,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.UploadImage(t.Context(), request)

	mustNoError(t, err)
	mustNotNil(t, result)
	checkEqual(t, "private/99", result.Image.ID)
	checkEqual(t, uploadImageLabelFixture, result.Image.Label)
	checkEqual(t, uploadImageTargetFixture, result.UploadTo)
}

func TestClientUploadImageAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "region is required"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UploadImage(t.Context(), &linode.UploadImageRequest{Label: uploadImageLabelFixture})

	mustError(t, err, "UploadImage should return API errors")
	checkErrorContains(t, err, "region is required")
}

func TestClientUploadImageNetworkError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := srv.URL
	srv.Close()

	client := linode.NewClient(baseURL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UploadImage(t.Context(), &linode.UploadImageRequest{Label: uploadImageLabelFixture, Region: regionUSEast})

	mustError(t, err, "UploadImage should wrap network errors")

	var networkErr *linode.NetworkError
	mustErrorAs(t, err, &networkErr, "network error should wrap as NetworkError")
	checkEqual(t, "UploadImage", networkErr.Operation)
}

func TestClientUploadImageDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	_, err := client.UploadImage(t.Context(), &linode.UploadImageRequest{Label: uploadImageLabelFixture, Region: regionUSEast})

	mustError(t, err, "UploadImage should fail on 500 response")
	checkEqual(t, int32(1), calls.Load(), "UploadImage must not retry and replay a mutating request")
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/object-storage/quotas", r.URL.Path, "request path should be /object-storage/quotas")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		checkEqual(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		checkNoError(t, err)
		checkEmpty(t, body, "request should not include a body")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    quotas,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	got, err := client.ListObjectStorageQuotas(t.Context())

	mustNoError(t, err, "ListObjectStorageQuotas should succeed on 200 response")
	mustLen(t, got, 1, "one quota should be returned")
	checkEqual(t, quotaID, got[0][keyID])
}

// TestClientListObjectStorageQuotasRetriesReadOnlyGET verifies the read-only quotas
// route can retry a transient server error without replaying a mutating request.
func TestClientListObjectStorageQuotasRetriesReadOnlyGET(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/object-storage/quotas", r.URL.Path, "request path should be /object-storage/quotas")

		if calls.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
			checkNoError(t, err)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    []linode.ObjectStorageQuota{{keyID: "endpoint-type-1"}},
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	got, err := client.ListObjectStorageQuotas(t.Context())

	mustNoError(t, err, "ListObjectStorageQuotas should retry and then succeed")
	mustLen(t, got, 1, "one quota should be returned")
	checkEqual(t, int32(2), calls.Load(), "GET quotas should retry once after transient failure")
}

func helperMessage(args []any) string {
	if len(args) == 0 {
		return ""
	}

	format, ok := args[0].(string)
	if ok {
		if len(args) > 1 {
			return ": " + fmt.Sprintf(format, args[1:]...)
		}

		return ": " + format
	}

	return ": " + fmt.Sprint(args...)
}

func fail(tb testing.TB, msg string, args ...any) {
	tb.Helper()

	tb.Fatalf("%s%s", msg, helperMessage(args))
}

func isNilValue(value any) bool {
	if value == nil {
		return true
	}

	actualValue := reflect.ValueOf(value)
	kind := actualValue.Kind()

	canBeNil := kind == reflect.Chan || kind == reflect.Func || kind == reflect.Interface || kind == reflect.Map || kind == reflect.Pointer || kind == reflect.Slice
	if !canBeNil {
		return false
	}

	return actualValue.IsNil()
}

func toFloat64(value any) (float64, bool) {
	switch actual := value.(type) {
	case int:
		return float64(actual), true
	case int8:
		return float64(actual), true
	case int16:
		return float64(actual), true
	case int32:
		return float64(actual), true
	case int64:
		return float64(actual), true
	case time.Duration:
		return float64(actual), true
	case uint:
		return float64(actual), true
	case uint8:
		return float64(actual), true
	case uint16:
		return float64(actual), true
	case uint32:
		return float64(actual), true
	case uint64:
		return float64(actual), true
	case uintptr:
		return float64(actual), true
	case float32:
		return float64(actual), true
	case float64:
		return actual, true
	default:
		return 0, false
	}
}

func containsValue(container, item any) bool {
	if text, ok := container.(string); ok {
		needle, ok := item.(string)

		return ok && strings.Contains(text, needle)
	}

	if isNilValue(container) {
		return false
	}

	containerValue := reflect.ValueOf(container)
	kind := containerValue.Kind()

	if kind == reflect.Map {
		key := reflect.ValueOf(item)
		if !key.IsValid() || !key.Type().AssignableTo(containerValue.Type().Key()) {
			return false
		}

		return containerValue.MapIndex(key).IsValid()
	}

	if kind != reflect.Array && kind != reflect.Slice {
		return false
	}

	for index := range containerValue.Len() {
		if reflect.DeepEqual(containerValue.Index(index).Interface(), item) {
			return true
		}
	}

	return false
}

func errorAsValue(err error, target any) bool {
	asError := errors.As

	return asError(err, target)
}

func mustNoError(tb testing.TB, err error, args ...any) {
	tb.Helper()

	if err != nil {
		fail(tb, fmt.Sprintf("unexpected error: %v", err), args...)
	}
}

func mustError(tb testing.TB, err error, args ...any) {
	tb.Helper()

	if err == nil {
		fail(tb, "expected error", args...)
	}
}

func checkErrorIs(tb testing.TB, err, target error, args ...any) {
	tb.Helper()

	if !errors.Is(err, target) {
		tb.Errorf("expected error %v to match %v%s", err, target, helperMessage(args))
	}
}

func mustErrorIs(tb testing.TB, err, target error, args ...any) {
	tb.Helper()

	if !errors.Is(err, target) {
		fail(tb, fmt.Sprintf("expected error %v to match %v", err, target), args...)
	}
}

func checkErrorAs(tb testing.TB, err error, target any, args ...any) {
	tb.Helper()

	if !errorAsValue(err, target) {
		tb.Errorf("expected error %v to match target %T%s", err, target, helperMessage(args))
	}
}

func mustErrorAs(tb testing.TB, err error, target any, args ...any) {
	tb.Helper()

	if !errorAsValue(err, target) {
		fail(tb, fmt.Sprintf("expected error %v to match target %T", err, target), args...)
	}
}

func checkErrorContains(tb testing.TB, err error, substring string, args ...any) {
	tb.Helper()

	if err == nil || !strings.Contains(fmt.Sprint(err), substring) {
		tb.Errorf("expected error %v to contain %q%s", err, substring, helperMessage(args))
	}
}

func mustErrorContains(tb testing.TB, err error, substring string, args ...any) {
	tb.Helper()

	if err == nil || !strings.Contains(fmt.Sprint(err), substring) {
		fail(tb, fmt.Sprintf("expected error %v to contain %q", err, substring), args...)
	}
}

func mustNil(tb testing.TB, actual any, args ...any) {
	tb.Helper()

	if !isNilValue(actual) {
		fail(tb, fmt.Sprintf("expected nil, got %#v", actual), args...)
	}
}

func checkNotNil(tb testing.TB, actual any, args ...any) bool {
	tb.Helper()

	if isNilValue(actual) {
		tb.Errorf("expected non-nil value%s", helperMessage(args))

		return false
	}

	return true
}

func mustNotNil(tb testing.TB, actual any, args ...any) {
	tb.Helper()

	if isNilValue(actual) {
		fail(tb, "expected non-nil value", args...)
	}
}

func checkLen(tb testing.TB, actual any, expected int, args ...any) bool {
	tb.Helper()

	actualLength, ok := valueLen(actual)
	if !ok || actualLength != expected {
		tb.Errorf("expected length %d, got %d%s", expected, actualLength, helperMessage(args))

		return false
	}

	return true
}

// mustLen uses the package-level valueLen helper from image_assertions_test.go.
func mustLen(tb testing.TB, actual any, expectedAndArgs ...any) {
	tb.Helper()

	if len(expectedAndArgs) == 0 {
		fail(tb, "missing expected length")
	}

	expected, ok := expectedAndArgs[0].(int)
	if !ok {
		fail(tb, fmt.Sprintf("expected length argument must be int, got %T", expectedAndArgs[0]))
	}

	if !checkLen(tb, actual, expected, expectedAndArgs[1:]...) {
		tb.FailNow()
	}
}

func mustTrue(tb testing.TB, actual bool, args ...any) {
	tb.Helper()

	if !actual {
		fail(tb, "expected true", args...)
	}
}

func checkFalse(tb testing.TB, actual bool, args ...any) {
	tb.Helper()

	if actual {
		tb.Errorf("expected false%s", helperMessage(args))
	}
}

func checkContains(tb testing.TB, container, item any, args ...any) {
	tb.Helper()

	if !containsValue(container, item) {
		tb.Errorf("expected %#v to contain %#v%s", container, item, helperMessage(args))
	}
}

func checkNotContains(tb testing.TB, container, item any, args ...any) {
	tb.Helper()

	if containsValue(container, item) {
		tb.Errorf("expected %#v not to contain %#v%s", container, item, helperMessage(args))
	}
}

func checkInEpsilon(tb testing.TB, expected, actual any, epsilon float64, args ...any) {
	tb.Helper()

	expectedFloat, expectedOK := toFloat64(expected)
	actualFloat, actualOK := toFloat64(actual)

	if !expectedOK || !actualOK || math.Abs(expectedFloat-actualFloat) > math.Abs(expectedFloat)*epsilon {
		tb.Errorf("expected %#v and %#v to be within epsilon %v%s", expected, actual, epsilon, helperMessage(args))
	}
}

func checkInDelta(tb testing.TB, expected, actual any, delta float64, args ...any) {
	tb.Helper()

	expectedFloat, expectedOK := toFloat64(expected)
	actualFloat, actualOK := toFloat64(actual)

	if !expectedOK || !actualOK || math.Abs(expectedFloat-actualFloat) > delta {
		tb.Errorf("expected %#v and %#v to be within delta %v%s", expected, actual, delta, helperMessage(args))
	}
}

func checkLess(tb testing.TB, actual, ceiling any, args ...any) {
	tb.Helper()

	actualFloat, actualOK := toFloat64(actual)
	ceilingFloat, ceilingOK := toFloat64(ceiling)

	if !actualOK || !ceilingOK || actualFloat >= ceilingFloat {
		tb.Errorf("expected %#v to be less than %#v%s", actual, ceiling, helperMessage(args))
	}
}

func checkGreaterOrEqual(tb testing.TB, actual, floor any, args ...any) {
	tb.Helper()

	actualFloat, actualOK := toFloat64(actual)
	floorFloat, floorOK := toFloat64(floor)

	if !actualOK || !floorOK || actualFloat < floorFloat {
		tb.Errorf("expected %#v to be greater than or equal to %#v%s", actual, floor, helperMessage(args))
	}
}

func mustGreaterOrEqual(tb testing.TB, actual, floor any, args ...any) {
	tb.Helper()

	actualFloat, actualOK := toFloat64(actual)
	floorFloat, floorOK := toFloat64(floor)

	if !actualOK || !floorOK || actualFloat < floorFloat {
		fail(tb, fmt.Sprintf("expected %#v to be greater than or equal to %#v", actual, floor), args...)
	}
}

func mustNotPanics(tb testing.TB, callback func(), args ...any) {
	tb.Helper()

	defer func() {
		if recovered := recover(); recovered != nil {
			fail(tb, fmt.Sprintf("unexpected panic: %#v", recovered), args...)
		}
	}()

	callback()
}
