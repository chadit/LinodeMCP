package linode_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	updateAccountEmail           = "account-updated@example.com"
	statusPending                = "pending"
	statusSuccessful             = "successful"
	accountEntityTransferToken   = "transfer-token-example"
	accountServiceTransferToken  = "service-token-example"
	accountEntityTransferDate    = "2021-02-11T16:37:03"
	accountEntityTransferExpiry  = "2021-02-12T16:37:03"
	accountTransferRegion        = "us-east"
	accountLoginUsername         = "account-login-user"
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
	imageShareGroupDescription   = "shared CI images"
	imageShareGroupUpdated       = "2025-04-15T22:44:02"
	imageShareGroupCreated       = "2025-04-14T22:44:02"
	oauthClientThumbnailURL      = "https://example.com/icon.png"
	oauthClientLabel             = "my app"
	oauthClientRedirectURI       = "https://example.com/callback"
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
		assert.Equal(t, "/profile", r.URL.Path, "request path should be /profile")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(profile), "encoding profile response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetProfile(t.Context())

	require.NoError(t, err, "GetProfile should succeed on 200 response")
	assert.Equal(t, "testuser", result.Username, "username should match the API response")
	assert.Equal(t, "test@example.com", result.Email, "email should match the API response")
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
		assert.NoError(t, json.NewEncoder(w).Encode(profile))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "tok", nil, linode.WithMaxRetries(0))
	result, err := client.GetProfile(t.Context())

	require.NoError(t, err)
	assert.Equal(t, "linodes:read_write domains:read_only", result.Scopes,
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
		assert.Equal(t, "/profile/grants", r.URL.Path,
			"request path should be /profile/grants")
		assert.Equal(t, "Bearer oauth-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "oauth-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetProfileGrants(t.Context())

	require.NoError(t, err, "GetProfileGrants should succeed on 200 response")
	require.NotNil(t, got)
	assert.Equal(t, linode.GrantPermission("read_write"), got.Global.AccountAccess,
		"global account_access must round-trip")
	assert.True(t, got.Global.AddLinodes,
		"global add_linodes must round-trip")
	require.Len(t, got.Linode, 1)
	assert.Equal(t, "web-1", got.Linode[0].Label)
	assert.Equal(t, linode.GrantPermission("read_write"), got.Linode[0].Permissions)
	require.Len(t, got.Domain, 1)
	assert.Equal(t, linode.GrantPermission("read_only"), got.Domain[0].Permissions)
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
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Grants{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "pat-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetProfileGrants(t.Context())

	require.NoError(t, err, "empty grants response must parse cleanly")
	require.NotNil(t, got)
	assert.Empty(t, got.Linode, "no Linode grants expected on PAT path")
	assert.Empty(t, got.Global.AccountAccess, "no global permission expected")
}

// TestClientGetProfileGrantsUnauthorized confirms 401 propagates as an
// APIError from GetProfileGrants, matching the GetProfile contract so
// Phase 6's loader can use the same error path for both calls.
func TestClientGetProfileGrantsUnauthorized(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"reason": "Invalid Token"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "bad-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetProfileGrants(t.Context())

	require.Error(t, err)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr,
		"GetProfileGrants must return APIError on 401")
	assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)
}

// TestClientGetProfileUnauthorized verifies that GetProfile returns an
// APIError with status 401 when the API rejects the token.
func TestClientGetProfileUnauthorized(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"reason": "Invalid Token"}},
		}), "encoding error response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "bad-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetProfile(t.Context())

	require.Error(t, err, "GetProfile should fail on 401 response")

	var apiErr *linode.APIError

	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, 401, apiErr.StatusCode, "status code should be 401 unauthorized")
}

func TestClientGetProfileAppSuccess(t *testing.T) {
	t.Parallel()

	want := linode.ProfileApp{ID: 12345, Label: "Example OAuth App", Scopes: "linodes:read_only", Website: "https://example.com"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/profile/apps/12345", r.URL.Path, "request path should include app id")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetProfileApp(t.Context(), want.ID)

	require.NoError(t, err, "GetProfileApp should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, want, *got)
}

func TestClientGetProfileAppBuildsNumericPath(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/profile/apps/12345", r.URL.EscapedPath(), "request path should include the numeric app id segment")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ProfileApp{ID: 12345}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileApp(t.Context(), 12345)

	require.NoError(t, err, "GetProfileApp should build the numeric app path")
}

func TestClientGetProfileAppAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/profile/apps/12345", r.URL.Path, "request path should include app id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetProfileApp(t.Context(), 12345)

	require.Error(t, err, "GetProfileApp should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/profile/apps/12345", r.URL.Path, "request path should include app id")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ProfileApp{ID: 12345, Label: "Example OAuth App"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.GetProfileApp(t.Context(), 12345)

	require.NoError(t, err, "GetProfileApp should succeed after retry")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, 12345, got.ID)
	assert.Equal(t, int32(2), requestCount.Load(), "read-only get should retry once then succeed")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, accountMaintenancePath, r.URL.Path, "request path should be /account/maintenance")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(maintenance))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.ListAccountMaintenance(t.Context(), 2, 25)

	require.NoError(t, err, "ListAccountMaintenance should succeed on 200 response")
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)
	assert.Equal(t, accountMaintenanceLabel, result.Data[0].Entity.Label)
	assert.Equal(t, "reboot", result.Data[0].Type)
}

func TestClientListAccountMaintenanceRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, accountMaintenancePath, r.URL.Path, "request path should be /account/maintenance")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountMaintenance]{
			Data:    []linode.AccountMaintenance{{Status: statusPending}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	result, err := client.ListAccountMaintenance(t.Context(), 0, 0)

	require.NoError(t, err, "read-only maintenance list should retry transient failures")
	require.NotNil(t, result)
	assert.Equal(t, int32(2), calls.Load(), "client should retry once")
}

func TestClientListAccountMaintenanceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, accountMaintenancePath, r.URL.Path, "request path should be /account/maintenance")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "Forbidden"}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListAccountMaintenance(t.Context(), 0, 0)

	require.Error(t, err, "ListAccountMaintenance should fail on API errors")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, maintenancePoliciesPath, r.URL.Path, "request path should be /maintenance/policies")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, http.NoBody, r.Body, "request body should be empty")
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(policies))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.ListMaintenancePolicies(t.Context(), 2, 25)

	require.NoError(t, err, "ListMaintenancePolicies should succeed on 200 response")
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)
	assert.Equal(t, maintenancePolicySlug, result.Data[0].Slug)
	assert.Equal(t, maintenancePolicyLabel, result.Data[0].Label)
	assert.True(t, result.Data[0].IsDefault)
}

func TestClientListMaintenancePoliciesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, maintenancePoliciesPath, r.URL.Path, "request path should be /maintenance/policies")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.MaintenancePolicy]{
			Data:    []linode.MaintenancePolicy{{Slug: maintenancePolicySlug}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	result, err := client.ListMaintenancePolicies(t.Context(), 0, 0)

	require.NoError(t, err, "read-only maintenance policies list should retry transient failures")
	require.NotNil(t, result)
	assert.Equal(t, int32(2), calls.Load(), "client should retry once")
}

func TestClientListMaintenancePoliciesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, maintenancePoliciesPath, r.URL.Path, "request path should be /maintenance/policies")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "Forbidden"}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListMaintenancePolicies(t.Context(), 0, 0)

	require.Error(t, err, "ListMaintenancePolicies should fail on API errors")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
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
		assert.Equal(t, "/linode/instances", r.URL.Path, "request path should be /linode/instances")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    instances,
			keyPage:    1,
			keyPages:   1,
			keyResults: 2,
		}), "encoding instances response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.ListInstances(t.Context())

	require.NoError(t, err, "ListInstances should succeed on 200 response")
	assert.Len(t, result, 2, "should return both instances")
	assert.Equal(t, "web-1", result[0].Label, "first instance label should match")
}

// TestClientGetInstanceSuccess verifies that GetInstance returns the correct
// instance when given a valid ID.
func TestClientGetInstanceSuccess(t *testing.T) {
	t.Parallel()

	instance := linode.Instance{ID: 42, Label: "my-instance", Status: "running"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances/42", r.URL.Path, "request path should include the instance ID")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(instance), "encoding instance response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.GetInstance(t.Context(), 42)

	require.NoError(t, err, "GetInstance should succeed on 200 response")
	assert.Equal(t, 42, result.ID, "instance ID should match the request")
	assert.Equal(t, "my-instance", result.Label, "instance label should match the API response")
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

	require.Error(t, err, "GetInstance should fail on 500 response")

	var apiErr *linode.APIError

	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, 500, apiErr.StatusCode, "status code should be 500 internal server error")
}

// TestClientGetProfileNetworkError verifies that GetProfile returns a
// NetworkError when the server is unreachable.
func TestClientGetProfileNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "token", nil, linode.WithMaxRetries(0))
	_, err := client.GetProfile(t.Context())

	require.Error(t, err, "GetProfile should fail when server is unreachable")

	var netErr *linode.NetworkError

	assert.ErrorAs(t, err, &netErr, "error should be a NetworkError")
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

	require.Error(t, err, "GetProfile should fail on 429 response")

	var apiErr *linode.APIError

	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, 429, apiErr.StatusCode, "status code should be 429 too many requests")
	assert.Contains(t, apiErr.Message, "retry after", "error message should include the retry-after value")
	assert.Equal(t, 30*time.Second, apiErr.RetryAfter, "RetryAfter field should carry the parsed hint")
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

	require.Error(t, err, "GetProfile should fail on 403 response")

	var apiErr *linode.APIError

	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, 403, apiErr.StatusCode, "status code should be 403 forbidden")
}

// TestClientContextCancelled verifies that GetProfile returns an error
// when the request context is already canceled before the call.
func TestClientContextCancelled(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Profile{Username: "test"}), "encoding profile response should not fail")
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.GetProfile(ctx)
	require.Error(t, err, "GetProfile should fail when context is already canceled")
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

	require.Error(t, err)
	assert.Equal(t, 6, attempts, "should attempt 1 initial + 5 retries from config")
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

	require.Error(t, err)
	assert.Equal(t, 2, attempts, "should attempt 1 initial + 1 retry from option override")
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

	require.Error(t, err)
	assert.Equal(t, 4, attempts, "should attempt 1 initial + 3 default retries")
}

// TestClientMalformedJSONResponse verifies that the client returns an error
// when the API responds with 200 OK but invalid JSON.
func TestClientMalformedJSONResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json at all`))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.GetProfile(t.Context())

	require.Error(t, err, "GetProfile should fail when response body is not valid JSON")

	var syntaxErr *json.SyntaxError

	assert.ErrorAs(t, err, &syntaxErr, "error chain should contain a json.SyntaxError")
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
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/profile", r.URL.Path, "request path should be /profile")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, updateAccountEmail, body["email"])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(updatedProfile))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	email := updateAccountEmail
	result, err := client.UpdateProfile(t.Context(), &linode.UpdateProfileRequest{
		Email: &email,
	})

	require.NoError(t, err, "UpdateProfile should succeed on 200 response")
	assert.Equal(t, updateAccountEmail, result.Email)
	assert.Equal(t, "US/Eastern", result.Timezone)
}

// TestClientUpdateProfileNetworkError verifies that UpdateProfile returns a
// NetworkError when the HTTP request fails to reach the server.
func TestClientUpdateProfileNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateProfile(t.Context(), &linode.UpdateProfileRequest{})

	require.Error(t, err, "UpdateProfile should fail when the server is unreachable")

	var netErr *linode.NetworkError

	assert.ErrorAs(t, err, &netErr, "error should be a NetworkError")
}

// TestClientUpdateProfileAPIError verifies that UpdateProfile propagates
// API errors (non-2xx) through the handleResponse error chain.
func TestClientUpdateProfileAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"field":"email","reason":"invalid email format"}]}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateProfile(t.Context(), &linode.UpdateProfileRequest{})

	require.Error(t, err, "UpdateProfile should fail on 400 response")

	var apiErr *linode.APIError

	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, 400, apiErr.StatusCode)
}

func TestClientListObjectStorageBucketsByRegionSuccess(t *testing.T) {
	t.Parallel()

	buckets := []linode.ObjectStorageBucket{
		{Label: "my-bucket", Region: "us-east-1", Hostname: "my-bucket.us-east-1.linodeobjects.com"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/object-storage/buckets/us-east-1", r.URL.Path, "request path should match regional buckets endpoint")
		assert.Empty(t, r.URL.RawQuery, "request should not include query params")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    buckets,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListObjectStorageBucketsByRegion(t.Context(), "us-east-1")

	require.NoError(t, err, "ListObjectStorageBucketsByRegion should succeed on 200 response")
	require.Len(t, result, 1, "response should include one bucket")
	assert.Equal(t, "my-bucket", result[0].Label)
}

func TestClientListObjectStorageBucketsByRegionEscapesPathParam(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/buckets/us%2Feast%3F1", r.URL.EscapedPath(), "path params should be escaped")
		assert.Empty(t, r.URL.RawQuery, "path params must not become query params")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    []linode.ObjectStorageBucket{},
			keyPage:    1,
			keyPages:   1,
			keyResults: 0,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	_, err := client.ListObjectStorageBucketsByRegion(t.Context(), "us/east?1")

	require.NoError(t, err, "escaped path param should round-trip through the client")
}

func TestClientCancelObjectStorageSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/cancel", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Empty(t, r.URL.RawQuery)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.CancelObjectStorage(t.Context())
	require.NoError(t, err, "CancelObjectStorage should succeed on 200 response")
}

func TestClientCancelObjectStorageAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/cancel", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"reason":"object storage cannot be canceled"}]}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.CancelObjectStorage(t.Context())

	require.Error(t, err, "CancelObjectStorage should fail on API error")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
}

func TestClientCancelObjectStorageDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, "/object-storage/cancel", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, "my-token", nil, linode.WithMaxRetries(3))
	err := client.CancelObjectStorage(t.Context())
	require.Error(t, err, "CancelObjectStorage should fail on 500 response")
	assert.Equal(t, int32(1), calls.Load(), "CancelObjectStorage must not retry and replay a state-changing request")
}

func TestClientAllowObjectStorageBucketAccessSuccess(t *testing.T) {
	t.Parallel()

	corsEnabled := true

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/access", r.URL.Path, "request path should match access endpoint")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "public-read", body["acl"])
		assert.Equal(t, true, body["cors_enabled"])

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.AllowObjectStorageBucketAccess(t.Context(), "us-east-1", "my-bucket", linode.AllowObjectStorageBucketAccessRequest{
		ACL:         "public-read",
		CORSEnabled: &corsEnabled,
	})

	require.NoError(t, err, "AllowObjectStorageBucketAccess should succeed on 200 response")
}

func TestClientAllowObjectStorageBucketAccessEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/buckets/us%2Feast%3F1/..%2Fbucket/access", r.URL.EscapedPath(), "path params should be escaped")
		assert.Empty(t, r.URL.RawQuery, "path params must not become query params")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.AllowObjectStorageBucketAccess(t.Context(), "us/east?1", "../bucket", linode.AllowObjectStorageBucketAccessRequest{})

	require.NoError(t, err, "escaped path params should round-trip through the client")
}

func TestClientAllowObjectStorageBucketAccessDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/access", r.URL.Path, "request path should match access endpoint")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))
	err := client.AllowObjectStorageBucketAccess(t.Context(), "us-east-1", "my-bucket", linode.AllowObjectStorageBucketAccessRequest{})

	require.Error(t, err, "AllowObjectStorageBucketAccess should fail on 500 response")
	assert.Equal(t, int32(1), calls.Load(), "AllowObjectStorageBucketAccess must not retry and replay a state-changing request")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/transfer", r.URL.Path, "request path should be /account/transfer")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(transfer))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountTransfer(t.Context())

	require.NoError(t, err, "GetAccountTransfer should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 10, result.Billable)
	assert.Equal(t, 4000, result.Quota)
	assert.Equal(t, 123, result.Used)
	require.Len(t, result.RegionTransfers, 1)
	assert.Equal(t, accountTransferRegion, result.RegionTransfers[0].ID)
	assert.Equal(t, 50, result.RegionTransfers[0].Used)
}

// TestClientGetAccountTransferAPIError verifies GetAccountTransfer propagates
// API errors through the handleResponse error chain.
func TestClientGetAccountTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/transfer", r.URL.Path, "request path should be /account/transfer")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountTransfer(t.Context())

	require.Error(t, err, "GetAccountTransfer should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/transfer", r.URL.Path, "request path should be /account/transfer")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountTransfer{Used: 123}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountTransfer(t.Context())

	require.NoError(t, err, "GetAccountTransfer should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 123, result.Used)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/settings", r.URL.Path, "request path should be /account/settings")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(settings))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountSettings(t.Context())

	require.NoError(t, err, "GetAccountSettings should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.True(t, result.BackupsEnabled)
	assert.False(t, result.Managed)
	assert.True(t, result.NetworkHelper)
	require.NotNil(t, result.LongviewSubscription)
	assert.Equal(t, longviewSubscription, *result.LongviewSubscription)
	require.NotNil(t, result.ObjectStorage)
	assert.Equal(t, objectStorage, *result.ObjectStorage)
	assert.Equal(t, "linode_default_but_legacy_config_allowed", result.InterfacesForNewLinodes)
	assert.Equal(t, maintenancePolicyMigrate, result.MaintenancePolicy)
}

// TestClientGetAccountSettingsAPIError verifies GetAccountSettings propagates
// API errors through the handleResponse error chain.
func TestClientGetAccountSettingsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/settings", r.URL.Path, "request path should be /account/settings")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountSettings(t.Context())

	require.Error(t, err, "GetAccountSettings should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/agreements", r.URL.Path, "request path should be /account/agreements")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(agreements))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountAgreements(t.Context())

	require.NoError(t, err, "GetAccountAgreements should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.True(t, result.BillingAgreement)
	assert.False(t, result.EUModel)
	assert.True(t, result.MasterServiceAgreement)
	assert.True(t, result.PrivacyPolicy)
}

// TestClientGetAccountAgreementsAPIError verifies GetAccountAgreements propagates
// API errors through the handleResponse error chain.
func TestClientGetAccountAgreementsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/agreements", r.URL.Path, "request path should be /account/agreements")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountAgreements(t.Context())

	require.Error(t, err, "GetAccountAgreements should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/notifications", r.URL.Path, "request path should be /account/notifications")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(notifications))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountNotifications(t.Context(), 2, 25)

	require.NoError(t, err, "ListAccountNotifications should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
	require.Len(t, result.Data, 1)
	assert.Equal(t, "Scheduled maintenance", result.Data[0].Label)
	assert.Equal(t, "major", result.Data[0].Severity)
	require.NotNil(t, result.Data[0].Entity)
	assert.Equal(t, "example-linode", result.Data[0].Entity.Label)
}

// TestClientListAccountNotificationsAPIError verifies ListAccountNotifications propagates API errors.
func TestClientListAccountNotificationsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/notifications", r.URL.Path, "request path should be /account/notifications")
		assert.Empty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountNotifications(t.Context(), 0, 0)

	require.Error(t, err, "ListAccountNotifications should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/notifications", r.URL.Path, "request path should be /account/notifications")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountNotification]{
			Data: []linode.AccountNotification{{Label: "Scheduled maintenance"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountNotifications(t.Context(), 0, 0)

	require.NoError(t, err, "ListAccountNotifications should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/availability/"+regionUSEast, r.URL.Path, "request path should include region")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(availability))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountAvailability(t.Context(), regionUSEast)

	require.NoError(t, err, "GetAccountAvailability should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, regionUSEast, result.Region)
	assert.Equal(t, []string{serviceLinodes, serviceNodeBalancers}, result.Available)
}

// TestClientGetAccountAvailabilityEscapesRegion verifies the client encodes path separators.
func TestClientGetAccountAvailabilityEscapesRegion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/availability/us%2Feast%3Fzone", r.URL.EscapedPath(), "request path should escape region")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountAvailability{Region: "us/east?zone"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountAvailability(t.Context(), "us/east?zone")

	require.NoError(t, err, "GetAccountAvailability should escape path parameters")
}

// TestClientGetAccountAvailabilityAPIError verifies GetAccountAvailability propagates API errors.
func TestClientGetAccountAvailabilityAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/availability/"+regionUSEast, r.URL.Path, "request path should include region")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountAvailability(t.Context(), regionUSEast)

	require.Error(t, err, "GetAccountAvailability should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/availability/"+regionUSEast, r.URL.Path, "request path should include region")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountAvailability{Region: regionUSEast}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountAvailability(t.Context(), regionUSEast)

	require.NoError(t, err, "GetAccountAvailability should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, regionUSEast, result.Region)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/availability", r.URL.Path, "request path should be /account/availability")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(availability))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountAvailability(t.Context(), 2, 25)

	require.NoError(t, err, "ListAccountAvailability should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
	require.Len(t, result.Data, 1)
	assert.Equal(t, regionUSEast, result.Data[0].Region)
	assert.Equal(t, []string{"Linodes", serviceNodeBalancers}, result.Data[0].Available)
}

// TestClientListAccountAvailabilityAPIError verifies ListAccountAvailability propagates API errors.
func TestClientListAccountAvailabilityAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/availability", r.URL.Path, "request path should be /account/availability")
		assert.Empty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountAvailability(t.Context(), 0, 0)

	require.Error(t, err, "ListAccountAvailability should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/oauth-clients", r.URL.Path, "request path should be /account/oauth-clients")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(clients), "encoding oauth clients response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountOAuthClients(t.Context(), 2, 25)

	require.NoError(t, err, "ListAccountOAuthClients should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
	require.Len(t, result.Data, 1)
	assert.Equal(t, "example-client", result.Data[0].Label)
	assert.Equal(t, "https://example.com/oauth/callback", result.Data[0].RedirectURI)
}

// TestClientListAccountOAuthClientsAPIError verifies ListAccountOAuthClients propagates API errors.
func TestClientListAccountOAuthClientsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/oauth-clients", r.URL.Path, "request path should be /account/oauth-clients")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountOAuthClients(t.Context(), 0, 0)

	require.Error(t, err, "ListAccountOAuthClients should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

// TestClientListAccountOAuthClientsRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListAccountOAuthClientsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/oauth-clients", r.URL.Path, "request path should be /account/oauth-clients")

		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "temporary upstream failure"}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.OAuthClient]{Data: []linode.OAuthClient{{ID: "2737bf16b39ab5d7b4a1", Label: "example-client"}}, Page: 1, Pages: 1, Results: 1}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountOAuthClients(t.Context(), 0, 0)

	require.NoError(t, err, "ListAccountOAuthClients should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, int32(2), attempts.Load(), "read-only list should retry one transient failure")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/betas", r.URL.Path, "request path should be /betas")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(betas))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListBetas(t.Context(), 2, 25)

	require.NoError(t, err, "ListBetas should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
	require.Len(t, result.Data, 1)
	assert.Equal(t, "example_open", result.Data[0].ID)
	assert.Equal(t, "open", result.Data[0].BetaClass)
	assert.False(t, result.Data[0].GreenlightOnly)
	assert.Equal(t, "https://example.com/beta", result.Data[0].MoreInfo)
	assert.Nil(t, result.Data[0].Ended)
}

// TestClientListBetasAPIError verifies ListBetas propagates API errors.
func TestClientListBetasAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/betas", r.URL.Path, "request path should be /betas")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListBetas(t.Context(), 0, 0)

	require.Error(t, err, "ListBetas should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

// TestClientListBetasRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListBetasRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	betas := linode.PaginatedResponse[linode.BetaProgram]{
		Data: []linode.BetaProgram{{ID: "example_open", Label: "Example Open Beta"}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/betas", r.URL.Path, "request path should be /betas")

		if requestCount.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(betas))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListBetas(t.Context(), 0, 0)

	require.NoError(t, err, "ListBetas should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, "example_open", result.Data[0].ID)
	assert.Equal(t, int32(2), requestCount.Load(), "read-only list should retry one transient failure")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/betas/"+betaExampleOpen, r.URL.Path, "request path should include beta id")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(beta))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetBeta(t.Context(), betaExampleOpen)

	require.NoError(t, err, "GetBeta should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, betaExampleOpen, result.ID)
	assert.Equal(t, labelExampleOpenBeta, result.Label)
	require.NotNil(t, result.Description)
	assert.Equal(t, description, *result.Description)
	assert.Equal(t, "open", result.BetaClass)
	assert.False(t, result.GreenlightOnly)
}

// TestClientGetBetaEscapesID verifies the client encodes path separators.
func TestClientGetBetaEscapesID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/betas/example%2Fopen%3Fquery", r.URL.EscapedPath(), "request path should escape beta id")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.BetaProgram{ID: "example/open?query"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetBeta(t.Context(), "example/open?query")

	require.NoError(t, err, "GetBeta should escape path parameters")
}

// TestClientGetBetaAPIError verifies GetBeta propagates API errors.
func TestClientGetBetaAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/betas/"+betaExampleOpen, r.URL.Path, "request path should include beta id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetBeta(t.Context(), betaExampleOpen)

	require.Error(t, err, "GetBeta should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/betas/"+betaExampleOpen, r.URL.Path, "request path should include beta id")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.BetaProgram{ID: betaExampleOpen, Label: labelExampleOpenBeta}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetBeta(t.Context(), betaExampleOpen)

	require.NoError(t, err, "GetBeta should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, betaExampleOpen, result.ID)
	assert.Equal(t, int32(2), requestCount.Load(), "read-only get should retry once then succeed")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/betas", r.URL.Path, "request path should be /account/betas")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(betas))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountBetas(t.Context(), 2, 25)

	require.NoError(t, err, "ListAccountBetas should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
	require.Len(t, result.Data, 1)
	assert.Equal(t, betaExampleOpen, result.Data[0].ID)
	assert.Equal(t, labelExampleOpenBeta, result.Data[0].Label)
	require.NotNil(t, result.Data[0].Description)
	assert.Equal(t, description, *result.Data[0].Description)
	assert.Nil(t, result.Data[0].Ended)
}

// TestClientListAccountBetasAPIError verifies ListAccountBetas propagates API errors.
func TestClientListAccountBetasAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/betas", r.URL.Path, "request path should be /account/betas")
		assert.Empty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountBetas(t.Context(), 0, 0)

	require.Error(t, err, "ListAccountBetas should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/betas", r.URL.Path, "request path should be /account/betas")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountBetaProgram]{
			Data: []linode.AccountBetaProgram{{ID: betaExampleOpen, Label: labelExampleOpenBeta}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountBetas(t.Context(), 0, 0)

	require.NoError(t, err, "ListAccountBetas should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, betaExampleOpen, result.Data[0].ID)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

// TestClientGetAccountOAuthClientSuccess verifies GetAccountOAuthClient sends the exact GET request.
func TestClientGetAccountOAuthClientSuccess(t *testing.T) {
	t.Parallel()

	want := linode.OAuthClient{ID: oauthClientID, Label: oauthClientLabel, RedirectURI: oauthClientRedirectURI, Status: oauthClientStatus, ThumbnailURL: oauthClientThumbnailURL}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/oauth-clients/"+oauthClientID, r.URL.Path, "request path should include client id")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID: oauthClientID, keyLabel: oauthClientLabel, keyRedirectURI: oauthClientRedirectURI, keyStatus: oauthClientStatus, keyThumbnailURL: oauthClientThumbnailURL, "secret": "server-secret",
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetAccountOAuthClient(t.Context(), oauthClientID)

	require.NoError(t, err, "GetAccountOAuthClient should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, want, *got)
}

func TestClientGetAccountOAuthClientEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/oauth-clients/client%2F123%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.OAuthClient{ID: oauthClientIDWithSeparators}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountOAuthClient(t.Context(), oauthClientIDWithSeparators)

	require.NoError(t, err, "GetAccountOAuthClient should escape path parameters")
}

func TestClientGetAccountOAuthClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/oauth-clients/"+oauthClientID, r.URL.Path, "request path should include client id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountOAuthClient(t.Context(), oauthClientID)

	require.Error(t, err, "GetAccountOAuthClient should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Contains(t, apiErr.Message, errForbidden)
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
		assert.NoError(t, json.NewEncoder(w).Encode(linode.OAuthClient{ID: oauthClientID, Label: oauthClientLabel}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.GetAccountOAuthClient(t.Context(), oauthClientID)

	require.NoError(t, err, "GetAccountOAuthClient should succeed after retry")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, oauthClientID, got.ID)
	assert.Equal(t, int32(2), requestCount.Load(), "read-only GET should retry once")
}

func TestClientUpdateOAuthClientSuccess(t *testing.T) {
	t.Parallel()

	public := true
	want := linode.OAuthClient{ID: oauthClientID, Label: "updated app", Public: public, RedirectURI: "https://example.com/new-callback", Status: oauthClientStatus, ThumbnailURL: oauthClientThumbnailURL}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/oauth-clients/"+oauthClientID, r.URL.Path, "request path should include client id")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var got linode.UpdateOAuthClientRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))

		if assert.NotNil(t, got.Label) {
			assert.Equal(t, want.Label, *got.Label)
		}

		if assert.NotNil(t, got.RedirectURI) {
			assert.Equal(t, want.RedirectURI, *got.RedirectURI)
		}

		if assert.NotNil(t, got.Public) {
			assert.Equal(t, public, *got.Public)
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.UpdateOAuthClientRequest{Label: &want.Label, Public: &public, RedirectURI: &want.RedirectURI}

	got, err := client.UpdateOAuthClient(t.Context(), oauthClientID, req)

	require.NoError(t, err, "UpdateOAuthClient should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, want, *got)
}

func TestClientUpdateOAuthClientEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/oauth-clients/client%2F123%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.OAuthClient{ID: oauthClientIDWithSeparators}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	label := oauthClientLabel
	req := &linode.UpdateOAuthClientRequest{Label: &label}

	_, err := client.UpdateOAuthClient(t.Context(), oauthClientIDWithSeparators, req)

	require.NoError(t, err, "UpdateOAuthClient should escape path parameters")
}

func TestClientUpdateOAuthClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/oauth-clients/"+oauthClientID, r.URL.Path, "request path should include client id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	label := oauthClientLabel
	req := &linode.UpdateOAuthClientRequest{Label: &label}

	_, err := client.UpdateOAuthClient(t.Context(), oauthClientID, req)

	require.Error(t, err, "UpdateOAuthClient should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
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

	require.Error(t, err, "UpdateOAuthClient should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating OAuth client update must not be retried")
}

func TestClientCreateOAuthClientSuccess(t *testing.T) {
	t.Parallel()

	want := linode.CreatedOAuthClient{ID: oauthClientID, Label: oauthClientLabel, RedirectURI: oauthClientRedirectURI, Secret: "secret-once"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/oauth-clients", r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var got linode.CreateOAuthClientRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, oauthClientLabel, got.Label)
		assert.Equal(t, oauthClientRedirectURI, got.RedirectURI)

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateOAuthClientRequest{Label: oauthClientLabel, RedirectURI: oauthClientRedirectURI}

	got, err := client.CreateOAuthClient(t.Context(), req)

	require.NoError(t, err, "CreateOAuthClient should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, want, *got)
}

func TestClientUpdateOAuthClientThumbnailSuccess(t *testing.T) {
	t.Parallel()

	thumbnailPNG := []byte("png-bytes")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/oauth-clients/"+oauthClientID+"/thumbnail", r.URL.Path, "request path should update client thumbnail")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, "image/png", r.Header.Get("Content-Type"))

		got, err := io.ReadAll(r.Body)
		assert.NoError(t, err, "reading thumbnail body should not fail")
		assert.Equal(t, thumbnailPNG, got, "thumbnail update should send the PNG bytes")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.UpdateOAuthClientThumbnail(t.Context(), oauthClientID, thumbnailPNG)

	require.NoError(t, err, "UpdateOAuthClientThumbnail should succeed on 200 response")
}

func TestClientUpdateOAuthClientThumbnailEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/oauth-clients/client%2F123%3Fquery/thumbnail", r.URL.EscapedPath(), "path parameter should be escaped")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.OAuthClient{ID: oauthClientIDWithSeparators}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.UpdateOAuthClientThumbnail(t.Context(), oauthClientIDWithSeparators, []byte("png-bytes"))

	require.NoError(t, err, "UpdateOAuthClientThumbnail should escape path parameters")
}

func TestClientUpdateOAuthClientThumbnailAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/oauth-clients/"+oauthClientID+"/thumbnail", r.URL.Path, "request path should update client thumbnail")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.UpdateOAuthClientThumbnail(t.Context(), oauthClientID, []byte("png-bytes"))

	require.Error(t, err, "UpdateOAuthClientThumbnail should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientUpdateOAuthClientThumbnailDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, "/account/oauth-clients/"+oauthClientID+"/thumbnail", r.URL.Path, "request path should update client thumbnail")
		http.Error(w, errTemporaryFailure, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.UpdateOAuthClientThumbnail(t.Context(), oauthClientID, []byte("png-bytes"))

	require.Error(t, err, "UpdateOAuthClientThumbnail should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating OAuth client thumbnail update must not be retried")
}

func TestClientGetOAuthClientThumbnailSuccess(t *testing.T) {
	t.Parallel()

	thumbnailPNG := []byte("png-bytes")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/oauth-clients/"+oauthClientID+"/thumbnail", r.URL.Path, "request path should get client thumbnail")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "image/png")
		_, writeErr := w.Write(thumbnailPNG)
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetOAuthClientThumbnail(t.Context(), oauthClientID)

	require.NoError(t, err, "GetOAuthClientThumbnail should succeed on 200 response")
	assert.Equal(t, thumbnailPNG, got, "GetOAuthClientThumbnail should return the PNG bytes")
}

func TestClientGetOAuthClientThumbnailEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/oauth-clients/client%2F123%3Fquery/thumbnail", r.URL.EscapedPath(), "path parameter should be escaped")
		w.Header().Set("Content-Type", "image/png")
		_, writeErr := w.Write([]byte("png-bytes"))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetOAuthClientThumbnail(t.Context(), oauthClientIDWithSeparators)

	require.NoError(t, err, "GetOAuthClientThumbnail should escape path parameters")
}

func TestClientGetOAuthClientThumbnailAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/oauth-clients/"+oauthClientID+"/thumbnail", r.URL.Path, "request path should get client thumbnail")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "Not Found"}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetOAuthClientThumbnail(t.Context(), oauthClientID)

	require.Error(t, err, "GetOAuthClientThumbnail should fail on 404 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
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
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.GetOAuthClientThumbnail(t.Context(), oauthClientID)

	require.NoError(t, err, "GetOAuthClientThumbnail should succeed after retry")
	assert.Equal(t, thumbnailPNG, got, "GetOAuthClientThumbnail should return the PNG bytes")
	assert.Equal(t, int32(2), requestCount.Load(), "read-only GetOAuthClientThumbnail should be retried on transient error")
}

func TestClientDeleteAccountOAuthClientSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, "/account/oauth-clients/"+oauthClientID, r.URL.Path, "request path should include client id")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.NoBody, r.Body, "DELETE request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountOAuthClient(t.Context(), oauthClientID)

	require.NoError(t, err, "DeleteAccountOAuthClient should succeed on 200 response")
}

func TestClientDeleteAccountOAuthClientEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, "/account/oauth-clients/client%2F123%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		assert.Empty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountOAuthClient(t.Context(), oauthClientIDWithSeparators)

	require.NoError(t, err, "DeleteAccountOAuthClient should escape path parameters")
}

func TestClientDeleteAccountOAuthClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, "/account/oauth-clients/"+oauthClientID, r.URL.Path, "request path should include client id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountOAuthClient(t.Context(), oauthClientID)

	require.Error(t, err, "DeleteAccountOAuthClient should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
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

	require.Error(t, err, "DeleteAccountOAuthClient should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "destructive OAuth client delete must not be retried")
}

func TestClientResetOAuthClientSecretSuccess(t *testing.T) {
	t.Parallel()

	want := linode.OAuthClientSecret{Secret: "new-secret-once"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/oauth-clients/"+oauthClientID+"/reset-secret", r.URL.Path, "request path should reset client secret")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.NoBody, r.Body, "reset secret request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.ResetOAuthClientSecret(t.Context(), oauthClientID)

	require.NoError(t, err, "ResetOAuthClientSecret should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, want, *got)
}

func TestClientResetOAuthClientSecretEscapesClientID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/oauth-clients/client%2F123%3Fquery/reset-secret", r.URL.EscapedPath(), "path parameter should be escaped")
		assert.Empty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.OAuthClientSecret{Secret: "new-secret"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ResetOAuthClientSecret(t.Context(), oauthClientIDWithSeparators)

	require.NoError(t, err, "ResetOAuthClientSecret should escape path parameters")
}

func TestClientResetOAuthClientSecretAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/oauth-clients/"+oauthClientID+"/reset-secret", r.URL.Path, "request path should reset client secret")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ResetOAuthClientSecret(t.Context(), oauthClientID)

	require.Error(t, err, "ResetOAuthClientSecret should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
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

	require.Error(t, err, "ResetOAuthClientSecret should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "credential rotation must not be retried")
}

// TestClientCreateOAuthClientAPIError verifies API errors propagate.
func TestClientCreateOAuthClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateOAuthClientRequest{Label: oauthClientLabel, RedirectURI: oauthClientRedirectURI}

	_, err := client.CreateOAuthClient(t.Context(), req)

	require.Error(t, err, "CreateOAuthClient should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
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

	require.Error(t, err, "CreateOAuthClient should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating OAuth client creation must not be retried")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/events", r.URL.Path, "request path should be /account/events")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(events))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountEvents(t.Context(), 2, 25)

	require.NoError(t, err, "ListAccountEvents should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
	require.Len(t, result.Data, 1)
	assert.Equal(t, 123, result.Data[0].ID)
	assert.Equal(t, "ticket_create", result.Data[0].Action)
	assert.Equal(t, "failed", result.Data[0].Status)
	require.NotNil(t, result.Data[0].Entity)
	assert.Equal(t, "ticket", result.Data[0].Entity.Type)
	require.NotNil(t, result.Data[0].Duration)
	assert.InDelta(t, duration, *result.Data[0].Duration, 0.001)
}

// TestClientListAccountEventsAPIError verifies ListAccountEvents propagates API errors.
func TestClientListAccountEventsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/events", r.URL.Path, "request path should be /account/events")
		assert.Empty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountEvents(t.Context(), 0, 0)

	require.Error(t, err, "ListAccountEvents should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/events", r.URL.Path, "request path should be /account/events")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountEvent]{
			Data: []linode.AccountEvent{{ID: 123, Action: "ticket_create"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountEvents(t.Context(), 0, 0)

	require.NoError(t, err, "ListAccountEvents should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, 123, result.Data[0].ID)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/child-accounts", r.URL.Path, "request path should be /account/child-accounts")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(childAccounts))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountChildAccounts(t.Context(), 2, 25)

	require.NoError(t, err, "ListAccountChildAccounts should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
	require.Len(t, result.Data, 1)
	assert.Equal(t, childAccountEUUID, result.Data[0].EUUID)
	assert.Equal(t, companyAcme, result.Data[0].Company)
	assert.Equal(t, "11/2024", result.Data[0].CreditCard.Expiry)
	assert.Equal(t, "0111", result.Data[0].CreditCard.LastFour)
}

// TestClientListAccountChildAccountsAPIError verifies ListAccountChildAccounts propagates API errors.
func TestClientListAccountChildAccountsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/child-accounts", r.URL.Path, "request path should be /account/child-accounts")
		assert.Empty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountChildAccounts(t.Context(), 0, 0)

	require.Error(t, err, "ListAccountChildAccounts should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/child-accounts", r.URL.Path, "request path should be /account/child-accounts")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ChildAccount]{
			Data: []linode.ChildAccount{{EUUID: childAccountEUUID, Company: companyAcme}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountChildAccounts(t.Context(), 0, 0)

	require.NoError(t, err, "ListAccountChildAccounts should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, childAccountEUUID, result.Data[0].EUUID)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

// TestClientGetAccountEventSuccess verifies GetAccountEvent sends a GET
// request to /account/events/{event_id} and decodes the response.
func TestClientGetAccountEventSuccess(t *testing.T) {
	t.Parallel()

	want := linode.AccountEvent{ID: 123, Action: "linode_create", Status: statusSuccessful, Username: "test-user"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/events/123", r.URL.Path, "request path should include event ID")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(want), "encoding event response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetAccountEvent(t.Context(), 123)

	require.NoError(t, err, "GetAccountEvent should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, 123, got.ID)
	assert.Equal(t, "linode_create", got.Action)
	assert.Equal(t, statusSuccessful, got.Status)
}

// TestClientGetAccountEventAPIError verifies GetAccountEvent propagates API errors.
func TestClientGetAccountEventAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/events/123", r.URL.Path, "request path should include event ID")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr, "writing error response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetAccountEvent(t.Context(), 123)

	require.Error(t, err, "GetAccountEvent should fail on 403 response")
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
			assert.NoError(t, writeErr, "writing transient error should not fail")

			return
		}

		assert.Equal(t, "/account/events/123", r.URL.Path, "request path should include event ID")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountEvent{ID: 123, Action: "linode_create"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	result, err := client.GetAccountEvent(t.Context(), 123)

	require.NoError(t, err, "GetAccountEvent should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 123, result.ID)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

// TestClientMarkAccountEventSeenSuccess verifies MarkAccountEventSeen sends a POST
// request to /account/events/{event_id}/seen with no body.
func TestClientMarkAccountEventSeenSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/events/123/seen", r.URL.Path, "request path should mark event seen")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		assert.Equal(t, http.NoBody, r.Body, "request should not send a body")

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		assert.NoError(t, writeErr, "writing success response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.MarkAccountEventSeen(t.Context(), 123)

	require.NoError(t, err, "MarkAccountEventSeen should succeed on 200 response")
}

// TestClientMarkAccountEventSeenAPIError verifies MarkAccountEventSeen propagates API errors.
func TestClientMarkAccountEventSeenAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/events/123/seen", r.URL.Path, "request path should mark event seen")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr, "writing error response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	err := client.MarkAccountEventSeen(t.Context(), 123)

	require.Error(t, err, "MarkAccountEventSeen should fail on 403 response")
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
		assert.NoError(t, writeErr, "writing transient error should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	err := client.MarkAccountEventSeen(t.Context(), 123)

	require.Error(t, err, "MarkAccountEventSeen should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating event seen request must not be retried")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/payment-methods", r.URL.Path, "request path should be /account/payment-methods")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(methods))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountPaymentMethods(t.Context(), 2, 25)

	require.NoError(t, err, "ListAccountPaymentMethods should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
	require.Len(t, result.Data, 1)
	assert.Equal(t, 123, result.Data[0].ID)
	assert.Equal(t, paymentMethodCreditCard, result.Data[0].Type)
	assert.True(t, result.Data[0].IsDefault)
}

// TestClientListAccountPaymentMethodsAPIError verifies ListAccountPaymentMethods propagates API errors.
func TestClientListAccountPaymentMethodsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/payment-methods", r.URL.Path, "request path should be /account/payment-methods")
		assert.Empty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountPaymentMethods(t.Context(), 0, 0)

	require.Error(t, err, "ListAccountPaymentMethods should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/payment-methods", r.URL.Path, "request path should be /account/payment-methods")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountPaymentMethod]{
			Data: []linode.AccountPaymentMethod{{ID: 123, Type: paymentMethodCreditCard, IsDefault: true}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountPaymentMethods(t.Context(), 0, 0)

	require.NoError(t, err, "ListAccountPaymentMethods should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, 123, result.Data[0].ID)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

func TestClientGetAccountPaymentMethodSuccess(t *testing.T) {
	t.Parallel()

	want := linode.AccountPaymentMethod{ID: 123, Type: paymentMethodCreditCard, IsDefault: true, Data: map[string]any{keyLastFour: "1111"}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/payment-methods/123", r.URL.Path, "request path should include payment method id")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetAccountPaymentMethod(t.Context(), "123")

	require.NoError(t, err, "GetAccountPaymentMethod should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, want, *got)
}

func TestClientGetAccountPaymentMethodEscapesID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/payment-methods/123%2F456%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountPaymentMethod{ID: 123, Type: paymentMethodCreditCard}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountPaymentMethod(t.Context(), "123/456?query")

	require.NoError(t, err, "GetAccountPaymentMethod should escape path parameters")
}

func TestClientGetAccountPaymentMethodAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/payment-methods/123", r.URL.Path, "request path should include payment method id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountPaymentMethod(t.Context(), "123")

	require.Error(t, err, "GetAccountPaymentMethod should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Contains(t, apiErr.Message, errForbidden)
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
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountPaymentMethod{ID: 123, Type: paymentMethodCreditCard}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.GetAccountPaymentMethod(t.Context(), "123")

	require.NoError(t, err, "GetAccountPaymentMethod should succeed after retry")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, 123, got.ID)
	assert.Equal(t, int32(2), requestCount.Load(), "read-only GET should retry once")
}

func TestClientDeleteAccountPaymentMethodSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, "/account/payment-methods/123", r.URL.Path, "request path should include payment method id")
		assert.Empty(t, r.URL.RawQuery, "delete request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountPaymentMethod(t.Context(), "123")

	require.NoError(t, err, "DeleteAccountPaymentMethod should succeed on 200 response")
}

func TestClientDeleteAccountPaymentMethodEscapesID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/payment-methods/123%2F456%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountPaymentMethod(t.Context(), "123/456?query")

	require.NoError(t, err, "DeleteAccountPaymentMethod should escape path parameters")
}

func TestClientDeleteAccountPaymentMethodAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, "/account/payment-methods/123", r.URL.Path, "request path should include payment method id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountPaymentMethod(t.Context(), "123")

	require.Error(t, err, "DeleteAccountPaymentMethod should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Contains(t, apiErr.Message, errForbidden)
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

	require.Error(t, err, "DeleteAccountPaymentMethod should surface transient failures")
	assert.Equal(t, int32(1), requestCount.Load(), "destructive DELETE should not be retried")
}

func TestClientMakeAccountPaymentMethodDefaultSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/payment-methods/123/make-default", r.URL.Path, "request path should include payment method id and make-default action")
		assert.Empty(t, r.URL.RawQuery, "make-default request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.NoBody, r.Body, "make-default request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.MakeAccountPaymentMethodDefault(t.Context(), "123")

	require.NoError(t, err, "MakeAccountPaymentMethodDefault should succeed on 200 response")
}

func TestClientMakeAccountPaymentMethodDefaultEscapesID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/payment-methods/123%2F456%3Fquery/make-default", r.URL.EscapedPath(), "path parameter should be escaped")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.MakeAccountPaymentMethodDefault(t.Context(), "123/456?query")

	require.NoError(t, err, "MakeAccountPaymentMethodDefault should escape path parameters")
}

func TestClientMakeAccountPaymentMethodDefaultAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/payment-methods/123/make-default", r.URL.Path, "request path should include payment method id and make-default action")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.MakeAccountPaymentMethodDefault(t.Context(), "123")

	require.Error(t, err, "MakeAccountPaymentMethodDefault should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Contains(t, apiErr.Message, errForbidden)
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

	require.Error(t, err, "MakeAccountPaymentMethodDefault should surface transient failures")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating make-default POST should not be retried")
}

func TestClientCreateAccountPaymentMethodSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.CreateAccountPaymentMethodRequest{Type: paymentMethodCreditCard, Data: map[string]any{keyToken: paymentMethodToken}, IsDefault: true}
	created := linode.AccountPaymentMethod{ID: 321, Type: paymentMethodCreditCard, IsDefault: true, Data: map[string]any{keyLastFour: "1111"}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/payment-methods", r.URL.Path, "request path should be /account/payment-methods")
		assert.Empty(t, r.URL.RawQuery, "create request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var body map[string]any

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, decodeErr)

		if decodeErr != nil {
			return
		}

		assert.Equal(t, paymentMethodCreditCard, body[keyType])
		assert.Equal(t, true, body[keyIsDefault])
		assert.Equal(t, map[string]any{keyToken: paymentMethodToken}, body[keyData])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(created))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateAccountPaymentMethod(t.Context(), request)

	require.NoError(t, err, "CreateAccountPaymentMethod should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 321, result.ID)
	assert.Equal(t, paymentMethodCreditCard, result.Type)
	assert.True(t, result.IsDefault)
}

func TestClientCreateAccountPaymentMethodAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/payment-methods", r.URL.Path, "request path should be /account/payment-methods")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.CreateAccountPaymentMethod(t.Context(), &linode.CreateAccountPaymentMethodRequest{Type: paymentMethodCreditCard, Data: map[string]any{keyToken: paymentMethodToken}, IsDefault: true})

	require.Error(t, err, "CreateAccountPaymentMethod should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, "forbidden", apiErr.Message)
}

func TestClientCreateAccountPaymentMethodDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.CreateAccountPaymentMethod(t.Context(), &linode.CreateAccountPaymentMethodRequest{Type: paymentMethodCreditCard, Data: map[string]any{keyToken: paymentMethodToken}, IsDefault: true})

	require.Error(t, err, "CreateAccountPaymentMethod should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating payment method creation must not be retried")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/invoices", r.URL.Path, "request path should be /account/invoices")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(invoices))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountInvoices(t.Context(), 2, 25)

	require.NoError(t, err, "ListAccountInvoices should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
	require.Len(t, result.Data, 1)
	assert.Equal(t, 987, result.Data[0].ID)
	assert.Equal(t, "Invoice 987", result.Data[0].Label)
	assert.InEpsilon(t, 42.50, result.Data[0].Total, 0.0001)
}

// TestClientListAccountInvoicesAPIError verifies ListAccountInvoices propagates API errors.
func TestClientListAccountInvoicesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/invoices", r.URL.Path, "request path should be /account/invoices")
		assert.Empty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountInvoices(t.Context(), 0, 0)

	require.Error(t, err, "ListAccountInvoices should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/invoices", r.URL.Path, "request path should be /account/invoices")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountInvoice]{
			Data: []linode.AccountInvoice{{ID: 987, Label: "Invoice 987"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountInvoices(t.Context(), 0, 0)

	require.NoError(t, err, "ListAccountInvoices should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, 987, result.Data[0].ID)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/payments", r.URL.Path, "request path should be /account/payments")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(payments))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountPayments(t.Context(), 2, 25)

	require.NoError(t, err, "ListAccountPayments should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
	require.Len(t, result.Data, 1)
	assert.Equal(t, 654, result.Data[0].ID)
	assert.InEpsilon(t, 20.25, result.Data[0].USD, 0.0001)
}

// TestClientListAccountPaymentsAPIError verifies ListAccountPayments propagates API errors.
func TestClientListAccountPaymentsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/payments", r.URL.Path, "request path should be /account/payments")
		assert.Empty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountPayments(t.Context(), 0, 0)

	require.Error(t, err, "ListAccountPayments should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/payments", r.URL.Path, "request path should be /account/payments")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountPayment]{
			Data: []linode.AccountPayment{{ID: 654, USD: 20.25}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountPayments(t.Context(), 0, 0)

	require.NoError(t, err, "ListAccountPayments should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, 654, result.Data[0].ID)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

func TestClientGetAccountPaymentSuccess(t *testing.T) {
	t.Parallel()

	payment := linode.AccountPayment{ID: 654, Date: "2024-02-01T00:00:00", USD: 20.25}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/payments/654", r.URL.Path, "request path should include payment ID")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(payment))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountPayment(t.Context(), 654)

	require.NoError(t, err, "GetAccountPayment should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 654, result.ID)
	assert.InEpsilon(t, 20.25, result.USD, 0.0001)
}

func TestClientGetAccountPaymentAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/payments/654", r.URL.Path, "request path should include payment ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountPayment(t.Context(), 654)

	require.Error(t, err, "GetAccountPayment should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientGetAccountPaymentRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/payments/654", r.URL.Path, "request path should include payment ID")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountPayment{ID: 654, USD: 20.25}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountPayment(t.Context(), 654)

	require.NoError(t, err, "GetAccountPayment should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 654, result.ID)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

// TestClientListAccountEntityTransfersSuccess verifies ListAccountEntityTransfers sends a GET
// request to /account/entity-transfers with pagination query parameters.
func TestClientListAccountEntityTransfersSuccess(t *testing.T) {
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/entity-transfers", r.URL.Path, "request path should be /account/entity-transfers")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(transfers))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountEntityTransfers(t.Context(), 2, 25)

	require.NoError(t, err, "ListAccountEntityTransfers should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
	require.Len(t, result.Data, 1)
	assert.Equal(t, accountEntityTransferToken, result.Data[0].Token)
	assert.Equal(t, "pending", result.Data[0].Status)
	assert.Equal(t, []int{111, 222}, result.Data[0].Entities.Linodes)
}

// TestClientListAccountEntityTransfersAPIError verifies ListAccountEntityTransfers propagates API errors.
func TestClientListAccountEntityTransfersAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/entity-transfers", r.URL.Path, "request path should be /account/entity-transfers")
		assert.Empty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountEntityTransfers(t.Context(), 0, 0)

	require.Error(t, err, "ListAccountEntityTransfers should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

// TestClientListAccountEntityTransfersRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListAccountEntityTransfersRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/entity-transfers", r.URL.Path, "request path should be /account/entity-transfers")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountEntityTransfer]{
			Data: []linode.AccountEntityTransfer{{Token: accountEntityTransferToken, Status: "pending"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountEntityTransfers(t.Context(), 0, 0)

	require.NoError(t, err, "ListAccountEntityTransfers should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, accountEntityTransferToken, result.Data[0].Token)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/service-transfers", r.URL.Path, "request path should be /account/service-transfers")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(transfers))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountServiceTransfers(t.Context(), 2, 25)

	require.NoError(t, err, "ListAccountServiceTransfers should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
	require.Len(t, result.Data, 1)
	assert.Equal(t, accountEntityTransferToken, result.Data[0].Token)
	assert.Equal(t, "pending", result.Data[0].Status)
	assert.Equal(t, []int{111, 222}, result.Data[0].Entities.Linodes)
}

func TestClientListAccountServiceTransfersAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/service-transfers", r.URL.Path, "request path should be /account/service-transfers")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountServiceTransfers(t.Context(), 0, 0)

	require.Error(t, err, "ListAccountServiceTransfers should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientListAccountServiceTransfersRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/service-transfers", r.URL.Path, "request path should be /account/service-transfers")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountEntityTransfer]{
			Data: []linode.AccountEntityTransfer{{Token: accountEntityTransferToken, Status: "pending"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountServiceTransfers(t.Context(), 0, 0)

	require.NoError(t, err, "ListAccountServiceTransfers should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, accountEntityTransferToken, result.Data[0].Token)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/service-transfers/service-token-example", r.URL.Path, "request path should include transfer token")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetAccountServiceTransfer(t.Context(), accountServiceTransferToken)

	require.NoError(t, err, "GetAccountServiceTransfer should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, accountServiceTransferToken, got.Token)
	assert.Equal(t, statusPending, got.Status)
	assert.Equal(t, []int{111, 222}, got.Entities.Linodes)
}

// TestClientGetAccountServiceTransferEscapesToken verifies the client encodes path separators.
func TestClientGetAccountServiceTransferEscapesToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/service-transfers/service%2Ftoken%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		assert.Empty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountEntityTransfer{Token: "service/token?query"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountServiceTransfer(t.Context(), "service/token?query")

	require.NoError(t, err, "GetAccountServiceTransfer should escape path parameters")
}

// TestClientGetAccountServiceTransferAPIError verifies GetAccountServiceTransfer propagates API errors.
func TestClientGetAccountServiceTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/service-transfers/service-token-example", r.URL.Path, "request path should include transfer token")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountServiceTransfer(t.Context(), accountServiceTransferToken)

	require.Error(t, err, "GetAccountServiceTransfer should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/service-transfers/service-token-example", r.URL.Path, "request path should include transfer token")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountEntityTransfer{Token: accountServiceTransferToken, Status: statusPending}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountServiceTransfer(t.Context(), accountServiceTransferToken)

	require.NoError(t, err, "GetAccountServiceTransfer should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, accountServiceTransferToken, result.Token)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

// TestClientDeleteAccountServiceTransferSuccess verifies DeleteAccountServiceTransfer sends a DELETE
// request to /account/service-transfers/{token} with no body.
func TestClientDeleteAccountServiceTransferSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, "/account/service-transfers/service-token-example", r.URL.Path, "request path should include transfer token")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.NoBody, r.Body, "DELETE request should not send a body")

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountServiceTransfer(t.Context(), accountServiceTransferToken)

	require.NoError(t, err, "DeleteAccountServiceTransfer should succeed on 200 response")
}

// TestClientDeleteAccountServiceTransferEscapesToken verifies the client encodes path separators.
func TestClientDeleteAccountServiceTransferEscapesToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, "/account/service-transfers/service%2Ftoken%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		assert.Empty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountServiceTransfer(t.Context(), "service/token?query")

	require.NoError(t, err, "DeleteAccountServiceTransfer should escape path parameters")
}

// TestClientDeleteAccountServiceTransferAPIError verifies DeleteAccountServiceTransfer propagates API errors.
func TestClientDeleteAccountServiceTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, "/account/service-transfers/service-token-example", r.URL.Path, "request path should include transfer token")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountServiceTransfer(t.Context(), accountServiceTransferToken)

	require.Error(t, err, "DeleteAccountServiceTransfer should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteAccountServiceTransfer(t.Context(), accountServiceTransferToken)

	require.Error(t, err, "DeleteAccountServiceTransfer should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating transfer cancellation must not be retried")
}

// TestClientAcceptAccountServiceTransferSuccess verifies AcceptAccountServiceTransfer sends a POST
// request to /account/service-transfers/{token}/accept with no body.
func TestClientAcceptAccountServiceTransferSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/service-transfers/service-token-example/accept", r.URL.Path, "request path should include transfer token accept action")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.NoBody, r.Body, "POST accept request should not send a body")

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.AcceptAccountServiceTransfer(t.Context(), accountServiceTransferToken)

	require.NoError(t, err, "AcceptAccountServiceTransfer should succeed on 200 response")
}

func TestClientAcceptAccountServiceTransferEscapesToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/service-transfers/service%2Ftoken%3Fquery/accept", r.URL.EscapedPath(), "path parameter should be escaped")
		assert.Empty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.AcceptAccountServiceTransfer(t.Context(), "service/token?query")

	require.NoError(t, err, "AcceptAccountServiceTransfer should escape path parameters")
}

func TestClientAcceptAccountServiceTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/service-transfers/service-token-example/accept", r.URL.Path, "request path should include transfer token accept action")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.AcceptAccountServiceTransfer(t.Context(), accountServiceTransferToken)

	require.Error(t, err, "AcceptAccountServiceTransfer should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientAcceptAccountServiceTransferDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.AcceptAccountServiceTransfer(t.Context(), accountServiceTransferToken)

	require.Error(t, err, "AcceptAccountServiceTransfer should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating service transfer acceptance must not be retried")
}

// TestClientGetAccountEntityTransferSuccess verifies GetAccountEntityTransfer sends a GET
// request to /account/entity-transfers/{token} and decodes the response.
func TestClientGetAccountEntityTransferSuccess(t *testing.T) {
	t.Parallel()

	want := linode.AccountEntityTransfer{
		Created:  accountEntityTransferDate,
		Entities: linode.AccountEntityTransferEntities{Linodes: []int{111, 222}},
		Expiry:   accountEntityTransferExpiry,
		IsSender: true,
		Status:   statusPending,
		Token:    accountEntityTransferToken,
		Updated:  accountEntityTransferDate,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/entity-transfers/transfer-token-example", r.URL.Path, "request path should include transfer token")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetAccountEntityTransfer(t.Context(), accountEntityTransferToken)

	require.NoError(t, err, "GetAccountEntityTransfer should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, accountEntityTransferToken, got.Token)
	assert.Equal(t, statusPending, got.Status)
	assert.Equal(t, []int{111, 222}, got.Entities.Linodes)
}

// TestClientGetAccountEntityTransferEscapesToken verifies the client encodes path separators.
func TestClientGetAccountEntityTransferEscapesToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/entity-transfers/transfer%2Ftoken%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		assert.Empty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountEntityTransfer{Token: "transfer/token?query"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountEntityTransfer(t.Context(), "transfer/token?query")

	require.NoError(t, err, "GetAccountEntityTransfer should escape path parameters")
}

// TestClientGetAccountEntityTransferAPIError verifies GetAccountEntityTransfer propagates API errors.
func TestClientGetAccountEntityTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/entity-transfers/transfer-token-example", r.URL.Path, "request path should include transfer token")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountEntityTransfer(t.Context(), accountEntityTransferToken)

	require.Error(t, err, "GetAccountEntityTransfer should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

// TestClientGetAccountEntityTransferRetriesTransientError verifies the read-only lookup retries transient failures.
func TestClientGetAccountEntityTransferRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/entity-transfers/transfer-token-example", r.URL.Path, "request path should include transfer token")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountEntityTransfer{Token: accountEntityTransferToken, Status: statusPending}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountEntityTransfer(t.Context(), accountEntityTransferToken)

	require.NoError(t, err, "GetAccountEntityTransfer should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, accountEntityTransferToken, result.Token)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

// TestClientAcceptAccountEntityTransferSuccess verifies AcceptAccountEntityTransfer sends a POST
// request to /account/entity-transfers/{token}/accept with no body.
func TestClientAcceptAccountEntityTransferSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/entity-transfers/transfer-token-example/accept", r.URL.Path, "request path should include transfer token accept action")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.NoBody, r.Body, "POST accept request should not send a body")

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.AcceptAccountEntityTransfer(t.Context(), accountEntityTransferToken)

	require.NoError(t, err, "AcceptAccountEntityTransfer should succeed on 200 response")
}

// TestClientAcceptAccountEntityTransferEscapesToken verifies the client encodes path separators.
func TestClientAcceptAccountEntityTransferEscapesToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/entity-transfers/transfer%2Ftoken%3Fquery/accept", r.URL.EscapedPath(), "path parameter should be escaped")
		assert.Empty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.AcceptAccountEntityTransfer(t.Context(), "transfer/token?query")

	require.NoError(t, err, "AcceptAccountEntityTransfer should escape path parameters")
}

// TestClientAcceptAccountEntityTransferAPIError verifies AcceptAccountEntityTransfer propagates API errors.
func TestClientAcceptAccountEntityTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/entity-transfers/transfer-token-example/accept", r.URL.Path, "request path should include transfer token accept action")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.AcceptAccountEntityTransfer(t.Context(), accountEntityTransferToken)

	require.Error(t, err, "AcceptAccountEntityTransfer should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

// TestClientAcceptAccountEntityTransferDoesNotRetryTransientError verifies
// transfer acceptance is not replayed after a transient failure.
func TestClientAcceptAccountEntityTransferDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.AcceptAccountEntityTransfer(t.Context(), accountEntityTransferToken)

	require.Error(t, err, "AcceptAccountEntityTransfer should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating transfer acceptance must not be retried")
}

// TestClientDeleteAccountEntityTransferSuccess verifies DeleteAccountEntityTransfer sends a DELETE
// request to /account/entity-transfers/{token} with no body.
func TestClientDeleteAccountEntityTransferSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, "/account/entity-transfers/transfer-token-example", r.URL.Path, "request path should include transfer token")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.NoBody, r.Body, "DELETE request should not send a body")

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountEntityTransfer(t.Context(), accountEntityTransferToken)

	require.NoError(t, err, "DeleteAccountEntityTransfer should succeed on 200 response")
}

// TestClientDeleteAccountEntityTransferEscapesToken verifies the client encodes path separators.
func TestClientDeleteAccountEntityTransferEscapesToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, "/account/entity-transfers/transfer%2Ftoken%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		assert.Empty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountEntityTransfer(t.Context(), "transfer/token?query")

	require.NoError(t, err, "DeleteAccountEntityTransfer should escape path parameters")
}

// TestClientDeleteAccountEntityTransferAPIError verifies DeleteAccountEntityTransfer propagates API errors.
func TestClientDeleteAccountEntityTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, "/account/entity-transfers/transfer-token-example", r.URL.Path, "request path should include transfer token")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteAccountEntityTransfer(t.Context(), accountEntityTransferToken)

	require.Error(t, err, "DeleteAccountEntityTransfer should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

// TestClientDeleteAccountEntityTransferDoesNotRetryTransientError verifies
// transfer cancellation is not replayed after a transient failure.
func TestClientDeleteAccountEntityTransferDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteAccountEntityTransfer(t.Context(), accountEntityTransferToken)

	require.Error(t, err, "DeleteAccountEntityTransfer should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating transfer cancellation must not be retried")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/invoices/12345", r.URL.Path, "request path should include invoice id")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(invoice))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountInvoice(t.Context(), accountInvoiceID)

	require.NoError(t, err, "GetAccountInvoice should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, accountInvoiceID, result.ID)
	assert.Equal(t, "Invoice #12345", result.Label)
	assert.InDelta(t, 11.00, result.Total, 0.001)
}

// TestClientGetAccountInvoiceAPIError verifies GetAccountInvoice propagates API errors.
func TestClientGetAccountInvoiceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/invoices/12345", r.URL.Path, "request path should include invoice id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountInvoice(t.Context(), accountInvoiceID)

	require.Error(t, err, "GetAccountInvoice should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/users", r.URL.Path, "request path should be /account/users")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(users))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountUsers(t.Context(), 2, 25)

	require.NoError(t, err, "ListAccountUsers should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
	require.Len(t, result.Data, 1)
	assert.Equal(t, accountLoginUsername, result.Data[0].Username)
	assert.Equal(t, accountUserEmail, result.Data[0].Email)
	assert.True(t, result.Data[0].Restricted)
	assert.True(t, result.Data[0].TFAEnabled)
}

// TestClientListAccountUsersAPIError verifies ListAccountUsers propagates API errors.
func TestClientListAccountUsersAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/users", r.URL.Path, "request path should be /account/users")
		assert.Empty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountUsers(t.Context(), 0, 0)

	require.Error(t, err, "ListAccountUsers should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/users", r.URL.Path, "request path should be /account/users")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountUser]{
			Data: []linode.AccountUser{{Username: accountLoginUsername, Email: accountUserEmail}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountUsers(t.Context(), 0, 0)

	require.NoError(t, err, "ListAccountUsers should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, accountLoginUsername, result.Data[0].Username)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/users/"+accountLoginUsername, r.URL.Path, "request path should include username")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(user))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountUser(t.Context(), accountLoginUsername)

	require.NoError(t, err, "GetAccountUser should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, accountLoginUsername, result.Username)
	assert.Equal(t, accountUserEmail, result.Email)
	assert.True(t, result.Restricted)
	assert.True(t, result.TFAEnabled)
}

// TestClientGetAccountUserEscapesUsername verifies the client encodes path separators.
func TestClientGetAccountUserEscapesUsername(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/users/user%2Fname%3Fquery", r.URL.EscapedPath())
		assert.Empty(t, r.URL.RawQuery, "escaped username must not create a query string")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountUser{Username: "user/name?query"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountUser(t.Context(), "user/name?query")

	require.NoError(t, err, "GetAccountUser should escape path parameters")
}

// TestClientGetAccountUserAPIError verifies GetAccountUser propagates API errors.
func TestClientGetAccountUserAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/users/"+accountLoginUsername, r.URL.Path, "request path should include username")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountUser(t.Context(), accountLoginUsername)

	require.Error(t, err, "GetAccountUser should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/users/"+accountLoginUsername, r.URL.Path, "request path should include username")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountUser{Username: accountLoginUsername, Email: accountUserEmail}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountUser(t.Context(), accountLoginUsername)

	require.NoError(t, err, "GetAccountUser should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, accountLoginUsername, result.Username)
	assert.Equal(t, int32(2), requestCount.Load(), "request should be retried once")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/users/"+accountLoginUsername+"/grants", r.URL.Path, "request path should include username grants")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(grants))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountUserGrants(t.Context(), accountLoginUsername)

	require.NoError(t, err, "GetAccountUserGrants should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, linode.GrantPermission("read_only"), result.Global.AccountAccess)
	require.Len(t, result.Linode, 1)
	assert.Equal(t, 123, result.Linode[0].ID)
}

// TestClientGetAccountUserGrantsEscapesUsername verifies the client encodes path separators.
func TestClientGetAccountUserGrantsEscapesUsername(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/users/user%2Fname%3Fquery/grants", r.URL.EscapedPath())
		assert.Empty(t, r.URL.RawQuery, "escaped username must not create a query string")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Grants{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountUserGrants(t.Context(), "user/name?query")

	require.NoError(t, err, "GetAccountUserGrants should escape path parameters")
}

// TestClientGetAccountUserGrantsAPIError verifies GetAccountUserGrants propagates API errors.
func TestClientGetAccountUserGrantsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/users/"+accountLoginUsername+"/grants", r.URL.Path, "request path should include username grants")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountUserGrants(t.Context(), accountLoginUsername)

	require.Error(t, err, "GetAccountUserGrants should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/users/"+accountLoginUsername+"/grants", r.URL.Path, "request path should include username grants")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Grants{Global: linode.GlobalGrants{AccountAccess: linode.GrantPermission("read_only")}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountUserGrants(t.Context(), accountLoginUsername)

	require.NoError(t, err, "GetAccountUserGrants should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, linode.GrantPermission("read_only"), result.Global.AccountAccess)
	assert.Equal(t, int32(2), requestCount.Load(), "request should be retried once")
}

// TestClientListAccountLoginsSuccess verifies ListAccountLogins sends a GET
// request to /account/logins with pagination query parameters.
func TestClientListAccountLoginsSuccess(t *testing.T) {
	t.Parallel()

	logins := linode.PaginatedResponse[linode.AccountLogin]{
		Data: []linode.AccountLogin{{
			Datetime:   accountUserPasswordCreated,
			ID:         123,
			IP:         "203.0.113.10",
			Restricted: false,
			Status:     statusSuccessful,
			Username:   accountLoginUsername,
		}},
		Page:    2,
		Pages:   3,
		Results: 60,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/logins", r.URL.Path, "request path should be /account/logins")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(logins))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountLogins(t.Context(), 2, 25)

	require.NoError(t, err, "ListAccountLogins should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
	require.Len(t, result.Data, 1)
	assert.Equal(t, 123, result.Data[0].ID)
	assert.Equal(t, accountLoginUsername, result.Data[0].Username)
	assert.Equal(t, "203.0.113.10", result.Data[0].IP)
}

// TestClientListAccountLoginsAPIError verifies ListAccountLogins propagates API errors.
func TestClientListAccountLoginsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/logins", r.URL.Path, "request path should be /account/logins")
		assert.Empty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountLogins(t.Context(), 0, 0)

	require.Error(t, err, "ListAccountLogins should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/logins", r.URL.Path, "request path should be /account/logins")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountLogin]{
			Data: []linode.AccountLogin{{ID: 123, Username: accountLoginUsername}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountLogins(t.Context(), 0, 0)

	require.NoError(t, err, "ListAccountLogins should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, 123, result.Data[0].ID)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

// TestClientGetAccountLoginSuccess verifies GetAccountLogin sends a GET
// request to /account/logins/{loginId} and decodes the response.
func TestClientGetAccountLoginSuccess(t *testing.T) {
	t.Parallel()

	login := linode.AccountLogin{
		Datetime:   accountUserPasswordCreated,
		ID:         123,
		IP:         "203.0.113.10",
		Restricted: false,
		Status:     statusSuccessful,
		Username:   accountLoginUsername,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/logins/123", r.URL.Path, "request path should include account login ID")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(login))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountLogin(t.Context(), 123)

	require.NoError(t, err, "GetAccountLogin should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 123, result.ID)
	assert.Equal(t, accountLoginUsername, result.Username)
	assert.Equal(t, "203.0.113.10", result.IP)
}

// TestClientGetAccountLoginAPIError verifies GetAccountLogin propagates API errors.
func TestClientGetAccountLoginAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/logins/123", r.URL.Path, "request path should include account login ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountLogin(t.Context(), 123)

	require.Error(t, err, "GetAccountLogin should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/logins/123", r.URL.Path, "request path should include account login ID")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountLogin{ID: 123, Username: accountLoginUsername}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountLogin(t.Context(), 123)

	require.NoError(t, err, "GetAccountLogin should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 123, result.ID)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/invoices/12345/items", r.URL.Path, "request path should include invoice id and items")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(items))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListAccountInvoiceItems(t.Context(), accountInvoiceID, 2, 25)

	require.NoError(t, err, "ListAccountInvoiceItems should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
	require.Len(t, result.Data, 1)
	assert.Equal(t, "Nanode 1GB", result.Data[0].Label)
	assert.InDelta(t, 5.00, result.Data[0].Total, 0.001)
}

// TestClientListAccountInvoiceItemsAPIError verifies ListAccountInvoiceItems propagates API errors.
func TestClientListAccountInvoiceItemsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/invoices/12345/items", r.URL.Path, "request path should include invoice id and items")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListAccountInvoiceItems(t.Context(), accountInvoiceID, 0, 0)

	require.Error(t, err, "ListAccountInvoiceItems should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

// TestClientListAccountInvoiceItemsRetriesTransientError verifies the read-only list retries transient failures.
func TestClientListAccountInvoiceItemsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/invoices/12345/items", r.URL.Path, "request path should include invoice id and items")

		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountInvoiceItem]{
			Data: []linode.AccountInvoiceItem{{Label: "Nanode 1GB", Total: 5.00}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListAccountInvoiceItems(t.Context(), accountInvoiceID, 0, 0)

	require.NoError(t, err, "ListAccountInvoiceItems should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/invoices/12345", r.URL.Path, "request path should include invoice id")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountInvoice{ID: accountInvoiceID, Label: "Invoice #12345"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountInvoice(t.Context(), accountInvoiceID)

	require.NoError(t, err, "GetAccountInvoice should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, accountInvoiceID, result.ID)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56", r.URL.Path, "request path should include child account euuid")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(childAccount))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountChildAccount(t.Context(), childAccountEUUID)

	require.NoError(t, err, "GetAccountChildAccount should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, childAccountEUUID, result.EUUID)
	assert.Equal(t, companyAcme, result.Company)
	assert.Equal(t, "11/2024", result.CreditCard.Expiry)
	assert.Equal(t, "0111", result.CreditCard.LastFour)
}

// TestClientGetAccountChildAccountEscapesEUUID verifies the client encodes path separators.
func TestClientGetAccountChildAccountEscapesEUUID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/child-accounts/child%2Faccount%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ChildAccount{EUUID: "child/account?query"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountChildAccount(t.Context(), "child/account?query")

	require.NoError(t, err, "GetAccountChildAccount should escape path parameters")
}

// TestClientGetAccountChildAccountAPIError verifies GetAccountChildAccount propagates API errors.
func TestClientGetAccountChildAccountAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56", r.URL.Path, "request path should include child account euuid")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountChildAccount(t.Context(), childAccountEUUID)

	require.Error(t, err, "GetAccountChildAccount should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56", r.URL.Path, "request path should include child account euuid")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ChildAccount{EUUID: childAccountEUUID, Company: companyAcme}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountChildAccount(t.Context(), childAccountEUUID)

	require.NoError(t, err, "GetAccountChildAccount should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, childAccountEUUID, result.EUUID)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

// TestClientCreateAccountEntityTransferSuccess verifies CreateAccountEntityTransfer
// sends a POST request to /account/entity-transfers and decodes the response.
func TestClientCreateAccountEntityTransferSuccess(t *testing.T) {
	t.Parallel()

	want := linode.AccountEntityTransfer{
		Entities: linode.AccountEntityTransferEntities{Linodes: []int{123, 456}},
		Status:   statusPending,
		Token:    "transfer-token",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/entity-transfers", r.URL.Path, "request path should be /account/entity-transfers")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got linode.CreateAccountEntityTransferRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, []int{123, 456}, got.Entities.Linodes)

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateAccountEntityTransferRequest{Entities: linode.AccountEntityTransferEntities{Linodes: []int{123, 456}}}

	got, err := client.CreateAccountEntityTransfer(t.Context(), req)

	require.NoError(t, err, "CreateAccountEntityTransfer should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, statusPending, got.Status)
	assert.Equal(t, "transfer-token", got.Token)
	assert.Equal(t, []int{123, 456}, got.Entities.Linodes)
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
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/service-transfers", r.URL.Path, "request path should be /account/service-transfers")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got linode.CreateAccountServiceTransferRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, []int{123, 456}, got.Entities.Linodes)

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateAccountServiceTransferRequest{Entities: linode.AccountEntityTransferEntities{Linodes: []int{123, 456}}}

	got, err := client.CreateAccountServiceTransfer(t.Context(), req)

	require.NoError(t, err, "CreateAccountServiceTransfer should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, statusPending, got.Status)
	assert.Equal(t, "service-transfer-token", got.Token)
	assert.Equal(t, []int{123, 456}, got.Entities.Linodes)
}

func TestClientCreateAccountServiceTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/service-transfers", r.URL.Path, "request path should be /account/service-transfers")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateAccountServiceTransferRequest{Entities: linode.AccountEntityTransferEntities{Linodes: []int{123}}}

	got, err := client.CreateAccountServiceTransfer(t.Context(), req)

	require.Error(t, err, "CreateAccountServiceTransfer should return API error")
	assert.Nil(t, got, "result should be nil on API error")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientCreateAccountServiceTransferDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	req := &linode.CreateAccountServiceTransferRequest{Entities: linode.AccountEntityTransferEntities{Linodes: []int{123}}}

	_, err := client.CreateAccountServiceTransfer(t.Context(), req)

	require.Error(t, err, "CreateAccountServiceTransfer should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating service transfer creation must not be retried")
}

// TestClientCreateAccountEntityTransferAPIError verifies API errors propagate.
func TestClientCreateAccountEntityTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/entity-transfers", r.URL.Path, "request path should be /account/entity-transfers")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateAccountEntityTransferRequest{Entities: linode.AccountEntityTransferEntities{Linodes: []int{123}}}

	_, err := client.CreateAccountEntityTransfer(t.Context(), req)

	require.Error(t, err, "CreateAccountEntityTransfer should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

// TestClientCreateAccountEntityTransferDoesNotRetryTransientError verifies
// transfer creation is not replayed after a transient failure.
func TestClientCreateAccountEntityTransferDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	req := &linode.CreateAccountEntityTransferRequest{Entities: linode.AccountEntityTransferEntities{Linodes: []int{123}}}

	_, err := client.CreateAccountEntityTransfer(t.Context(), req)

	require.Error(t, err, "CreateAccountEntityTransfer should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating transfer creation must not be retried")
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
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56/token", r.URL.Path, "request path should include child account euuid and token suffix")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.NoBody, r.Body, "token creation should not send a request body")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(proxyToken))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateAccountChildAccountToken(t.Context(), childAccountEUUID)

	require.NoError(t, err, "CreateAccountChildAccountToken should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 918, result.ID)
	assert.Equal(t, "parent1_1234_2024-05-01T00:01:01", result.Label)
	assert.Equal(t, "*", result.Scopes)
	assert.Equal(t, "abcdefghijklmnop", result.Token)
	assert.Equal(t, "2024-05-01T00:16:01", result.Expiry)
}

// TestClientCreateAccountChildAccountTokenEscapesEUUID verifies the client encodes path separators.
func TestClientCreateAccountChildAccountTokenEscapesEUUID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/child-accounts/child%2Faccount%3Fquery/token", r.URL.EscapedPath(), "path parameter should be escaped")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ProxyUserToken{Token: "proxy-token"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.CreateAccountChildAccountToken(t.Context(), "child/account?query")

	require.NoError(t, err, "CreateAccountChildAccountToken should escape path parameters")
}

// TestClientCreateAccountChildAccountTokenAPIError verifies API errors propagate.
func TestClientCreateAccountChildAccountTokenAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56/token", r.URL.Path, "request path should include child account euuid and token suffix")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.CreateAccountChildAccountToken(t.Context(), childAccountEUUID)

	require.Error(t, err, "CreateAccountChildAccountToken should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.CreateAccountChildAccountToken(t.Context(), childAccountEUUID)

	require.Error(t, err, "CreateAccountChildAccountToken should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating token creation must not be retried")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/betas/"+betaExampleOpen, r.URL.Path, "request path should include beta id")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(beta))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetAccountBeta(t.Context(), betaExampleOpen)

	require.NoError(t, err, "GetAccountBeta should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, betaExampleOpen, result.ID)
	assert.Equal(t, labelExampleOpenBeta, result.Label)
	require.NotNil(t, result.Description)
	assert.Equal(t, description, *result.Description)
}

// TestClientGetAccountBetaEscapesID verifies the client encodes path separators.
func TestClientGetAccountBetaEscapesID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/betas/example%2Fopen%3Fquery", r.URL.EscapedPath(), "request path should escape beta id")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountBetaProgram{ID: "example/open?query"}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountBeta(t.Context(), "example/open?query")

	require.NoError(t, err, "GetAccountBeta should escape path parameters")
}

// TestClientGetAccountBetaAPIError verifies GetAccountBeta propagates API errors.
func TestClientGetAccountBetaAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/betas/"+betaExampleOpen, r.URL.Path, "request path should include beta id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetAccountBeta(t.Context(), betaExampleOpen)

	require.Error(t, err, "GetAccountBeta should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
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
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/account/betas/"+betaExampleOpen, r.URL.Path, "request path should include beta id")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountBetaProgram{ID: betaExampleOpen, Label: labelExampleOpenBeta}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetAccountBeta(t.Context(), betaExampleOpen)

	require.NoError(t, err, "GetAccountBeta should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, betaExampleOpen, result.ID)
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

// TestClientEnrollAccountBetaSuccess verifies EnrollAccountBeta sends a POST
// request to /account/betas with the exact body.
func TestClientEnrollAccountBetaSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/betas", r.URL.Path, "request path should be /account/betas")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, betaExampleOpen, body["id"])

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.EnrollAccountBeta(t.Context(), &linode.EnrollAccountBetaRequest{ID: betaExampleOpen})

	require.NoError(t, err, "EnrollAccountBeta should succeed on 200 response")
}

// TestClientEnrollAccountBetaAPIError verifies EnrollAccountBeta propagates API errors.
func TestClientEnrollAccountBetaAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/betas", r.URL.Path, "request path should be /account/betas")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.EnrollAccountBeta(t.Context(), &linode.EnrollAccountBetaRequest{ID: betaExampleOpen})

	require.Error(t, err, "EnrollAccountBeta should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

// TestClientEnrollAccountBetaDoesNotRetry verifies the mutating beta enrollment
// is not replayed after a transient HTTP error.
func TestClientEnrollAccountBetaDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/betas", r.URL.Path, "request path should be /account/betas")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.EnrollAccountBeta(t.Context(), &linode.EnrollAccountBetaRequest{ID: betaExampleOpen})

	require.Error(t, err, "EnrollAccountBeta should fail on 500 response")
	assert.Equal(t, int32(1), calls.Load(), "EnrollAccountBeta must not retry and replay a mutating request")
}

// TestClientAcknowledgeAccountAgreementsSuccess verifies that
// AcknowledgeAccountAgreements sends a POST request to /account/agreements with
// the exact body and returns the agreement statuses.
func TestClientAcknowledgeAccountAgreementsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/agreements", r.URL.Path, "request path should be /account/agreements")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, true, body["billing_agreement"])
		assert.Equal(t, true, body["eu_model"])
		assert.Equal(t, true, body["master_service_agreement"])
		assert.NotContains(t, body, "privacy_policy", "omitted fields should not be sent")

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{}`))
		assert.NoError(t, writeErr)
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

	require.NoError(t, err, "AcknowledgeAccountAgreements should succeed on 200 response")
}

// TestClientAcknowledgeAccountAgreementsDoesNotRetry verifies the mutating
// agreement acknowledgement is not replayed after a transient HTTP error.
func TestClientAcknowledgeAccountAgreementsDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/agreements", r.URL.Path, "request path should be /account/agreements")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	privacyPolicy := true
	err := client.AcknowledgeAccountAgreements(t.Context(), &linode.AcknowledgeAccountAgreementsRequest{PrivacyPolicy: &privacyPolicy})

	require.Error(t, err, "AcknowledgeAccountAgreements should fail on 500 response")
	assert.Equal(t, int32(1), calls.Load(), "AcknowledgeAccountAgreements must not retry and replay a mutating request")
}

// TestClientCancelAccountSuccess verifies CancelAccount sends a POST request to
// /account/cancel with the exact body and returns the survey link.
func TestClientCancelAccountSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/cancel", r.URL.Path, "request path should be /account/cancel")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "moving providers", body["comments"])

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{"survey_link":"https://example.test/survey"}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	comments := "moving providers"
	response, err := client.CancelAccount(t.Context(), &linode.CancelAccountRequest{Comments: &comments})

	require.NoError(t, err, "CancelAccount should succeed on 200 response")
	require.NotNil(t, response, "response should not be nil")
	assert.Equal(t, "https://example.test/survey", response.SurveyLink)
}

// TestClientCancelAccountWithoutComments verifies comments are optional.
func TestClientCancelAccountWithoutComments(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/cancel", r.URL.Path, "request path should be /account/cancel")

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Empty(t, body, "omitted comments should send an empty JSON object")

		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{"survey_link":"https://example.test/survey"}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	response, err := client.CancelAccount(t.Context(), &linode.CancelAccountRequest{})

	require.NoError(t, err, "CancelAccount should succeed without comments")
	require.NotNil(t, response, "response should not be nil")
	assert.Equal(t, "https://example.test/survey", response.SurveyLink)
}

// TestClientCancelAccountDoesNotRetry verifies account cancellation is not
// replayed after a transient HTTP error.
func TestClientCancelAccountDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/cancel", r.URL.Path, "request path should be /account/cancel")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	errComments := "temporary"
	_, err := client.CancelAccount(t.Context(), &linode.CancelAccountRequest{Comments: &errComments})

	require.Error(t, err, "CancelAccount should fail on 500 response")
	assert.Equal(t, int32(1), calls.Load(), "CancelAccount must not retry and replay a destructive request")
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
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account", r.URL.Path, "request path should be /account")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "updated@example.com", body["email"])
		assert.Equal(t, "Updated", body["first_name"])
		assert.Equal(t, "123 Main St", body["address_1"])
		assert.NotContains(t, body, "address_2", "omitted fields should not be sent")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(updatedAccount))
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

	require.NoError(t, err, "UpdateAccount should succeed on 200 response")
	assert.Equal(t, "updated@example.com", result.Email)
	assert.Equal(t, "Updated", result.FirstName)
	assert.Equal(t, "123 Main St", result.Address1)
}

// TestClientUpdateAccountNetworkError verifies that UpdateAccount returns a
// NetworkError when the HTTP request fails to reach the server.
func TestClientUpdateAccountNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateAccount(t.Context(), &linode.UpdateAccountRequest{})

	require.Error(t, err, "UpdateAccount should fail when the server is unreachable")

	var netErr *linode.NetworkError

	assert.ErrorAs(t, err, &netErr, "error should be a NetworkError")
}

// TestClientUpdateAccountAPIError verifies that UpdateAccount propagates
// API errors through the handleResponse error chain.
func TestClientUpdateAccountAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"field":"email","reason":"invalid email format"}]}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateAccount(t.Context(), &linode.UpdateAccountRequest{})

	require.Error(t, err, "UpdateAccount should fail on 400 response")

	var apiErr *linode.APIError

	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
}

// TestClientUpdateAccountDoesNotRetry verifies the mutating account update is
// not replayed after a transient HTTP error.
func TestClientUpdateAccountDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account", r.URL.Path, "request path should be /account")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateAccount(t.Context(), &linode.UpdateAccountRequest{})

	require.Error(t, err, "UpdateAccount should fail on 500 response")
	assert.Equal(t, int32(1), calls.Load(), "UpdateAccount must not retry and replay a mutating request")
}

// TestClientEnableAccountManagedSuccess verifies that EnableAccountManaged sends a POST
// request to /account/settings/managed-enable with no query parameters or body.
func TestClientEnableAccountManagedSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/settings/managed-enable", r.URL.Path, "request path should be /account/settings/managed-enable")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Empty(t, body, "request should not include a body")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.EnableAccountManaged(t.Context())

	require.NoError(t, err, "EnableAccountManaged should succeed on 200 response")
}

// TestClientEnableAccountManagedNetworkError verifies that EnableAccountManaged returns a
// NetworkError when the HTTP request fails to reach the server.
func TestClientEnableAccountManagedNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(3))

	err := client.EnableAccountManaged(t.Context())

	require.Error(t, err, "EnableAccountManaged should fail when the server is unreachable")

	var netErr *linode.NetworkError

	assert.ErrorAs(t, err, &netErr, "error should be a NetworkError")
}

// TestClientEnableAccountManagedAPIError verifies that EnableAccountManaged propagates
// API errors through the handleResponse error chain.
func TestClientEnableAccountManagedAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/settings/managed-enable", r.URL.Path, "request path should be /account/settings/managed-enable")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"reason":"managed could not be enabled"}]}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.EnableAccountManaged(t.Context())

	require.Error(t, err, "EnableAccountManaged should fail on 400 response")

	var apiErr *linode.APIError

	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
}

// TestClientEnableAccountManagedDoesNotRetry verifies the mutating managed enable
// request is not replayed after a transient HTTP error.
func TestClientEnableAccountManagedDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/settings/managed-enable", r.URL.Path, "request path should be /account/settings/managed-enable")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.EnableAccountManaged(t.Context())

	require.Error(t, err, "EnableAccountManaged should fail on 500 response")
	assert.Equal(t, int32(1), calls.Load(), "EnableAccountManaged must not retry and replay a mutating request")
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
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/settings", r.URL.Path, "request path should be /account/settings")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, true, body["backups_enabled"])
		assert.Equal(t, false, body["network_helper"])
		assert.Equal(t, maintenancePolicyMigrate, body["maintenance_policy"])
		assert.NotContains(t, body, "object_storage", "omitted fields should not be sent")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(settings))
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

	require.NoError(t, err, "UpdateAccountSettings should succeed on 200 response")
	assert.True(t, result.BackupsEnabled)
	assert.False(t, result.NetworkHelper)
	assert.Equal(t, maintenancePolicyMigrate, result.MaintenancePolicy)
}

// TestClientUpdateAccountSettingsAPIError verifies that UpdateAccountSettings propagates
// API errors through the handleResponse error chain.
func TestClientUpdateAccountSettingsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/settings", r.URL.Path, "request path should be /account/settings")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"field":"maintenance_policy","reason":"invalid maintenance policy"}]}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateAccountSettings(t.Context(), &linode.UpdateAccountSettingsRequest{})

	require.Error(t, err, "UpdateAccountSettings should fail on 400 response")

	var apiErr *linode.APIError

	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
}

// TestClientUpdateAccountSettingsDoesNotRetry verifies the mutating account settings
// update is not replayed after a transient HTTP error.
func TestClientUpdateAccountSettingsDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/settings", r.URL.Path, "request path should be /account/settings")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		assert.NoError(t, err)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateAccountSettings(t.Context(), &linode.UpdateAccountSettingsRequest{})

	require.Error(t, err, "UpdateAccountSettings should fail on 500 response")
	assert.Equal(t, int32(1), calls.Load(), "UpdateAccountSettings must not retry and replay a mutating request")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/images/sharegroups/tokens", r.URL.Path, "request path should be /images/sharegroups/tokens")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:   tokens,
			"page":    2,
			"pages":   3,
			"results": 7,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListImageShareGroupTokens(t.Context(), 2, 25)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)
	assert.Equal(t, "Backend Services - Engineering", result.Data[0].Label)
	assert.Equal(t, "13428362-5458-4dad-b14b-8d0d4d648f8c", result.Data[0].TokenUUID)
	assert.Equal(t, 2, result.Page)
	assert.Equal(t, 7, result.Results)
}

func TestClientListImageShareGroupTokensError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListImageShareGroupTokens(t.Context(), 1, 25)

	require.Error(t, err)
	assert.Nil(t, result)
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/images/sharegroups", r.URL.Path, "request path should be /images/sharegroups")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    shareGroups,
			"page":    2,
			"pages":   3,
			"results": 7,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListImageShareGroups(t.Context(), 2, 25)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)
	assert.Equal(t, imageShareGroupLabel, result.Data[0].Label)
	assert.Equal(t, 2, result.Page)
	assert.Equal(t, 7, result.Results)
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/images/sharegroups/123", r.URL.Path, "request path should include share group ID")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(shareGroup))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetImageShareGroup(t.Context(), 123)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 123, result.ID)
	assert.Equal(t, imageShareGroupLabel, result.Label)
}

func TestClientGetImageShareGroupAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/images/sharegroups/123", r.URL.Path, "request path should include share group ID")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]any{{keyReason: "temporary share group failure"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetImageShareGroup(t.Context(), 123)

	require.Error(t, err)
	assert.Nil(t, result)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "temporary share group failure", apiErr.Message)
}

func TestClientGetImageShareGroupNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "token", nil, linode.WithMaxRetries(0))
	_, err := client.GetImageShareGroup(t.Context(), 123)

	require.Error(t, err, "GetImageShareGroup should fail when server is unreachable")

	var netErr *linode.NetworkError
	require.ErrorAs(t, err, &netErr, "error should be a NetworkError")
	assert.Equal(t, "GetImageShareGroup", netErr.Operation)
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/images/sharegroups/123", r.URL.Path, "request path should include share group ID")

		if requestCount.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]any{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(shareGroup))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.GetImageShareGroup(t.Context(), 123)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int32(2), requestCount.Load())
	assert.Equal(t, imageShareGroupLabel, result.Label)
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
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/images/sharegroups", r.URL.Path, "request path should be /images/sharegroups")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))

		var body map[string]any
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}

		assert.Equal(t, imageShareGroupLabel, body[keyLabel])
		assert.Equal(t, description, body[keyDescription])

		if !assert.Len(t, body["images"], 1) {
			return
		}

		image, ok := body["images"].([]any)[0].(map[string]any)
		if !assert.True(t, ok, "image payload should be an object") {
			return
		}

		assert.Equal(t, "private/7", image[keyID])
		assert.Equal(t, "Linux Debian", image["label"])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroup{
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

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, imageShareGroupLabel, result.Label)
	assert.Equal(t, 1, result.ImagesCount)
}

func TestClientCreateImageShareGroupAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "label is required"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.CreateImageShareGroup(t.Context(), &linode.CreateImageShareGroupRequest{Label: imageShareGroupLabel})

	require.Error(t, err, "CreateImageShareGroup should return API errors")
	assert.ErrorContains(t, err, "label is required")
}

func TestClientCreateImageShareGroupNetworkError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := srv.URL
	srv.Close()

	client := linode.NewClient(baseURL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.CreateImageShareGroup(t.Context(), &linode.CreateImageShareGroupRequest{Label: imageShareGroupLabel})

	require.Error(t, err, "CreateImageShareGroup should wrap network errors")

	var networkErr *linode.NetworkError
	require.ErrorAs(t, err, &networkErr, "network error should wrap as NetworkError")
	assert.Equal(t, "CreateImageShareGroup", networkErr.Operation)
}

func TestClientCreateImageShareGroupDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	_, err := client.CreateImageShareGroup(t.Context(), &linode.CreateImageShareGroupRequest{Label: imageShareGroupLabel})

	require.Error(t, err, "CreateImageShareGroup should fail on 500 response")
	assert.Equal(t, int32(1), calls.Load(), "CreateImageShareGroup must not retry and replay a mutating request")
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
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/images/upload", r.URL.Path, "request path should be /images/upload")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))

		var body map[string]any
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode") {
			return
		}

		assert.Equal(t, uploadImageLabelFixture, body[keyLabel])
		assert.Equal(t, regionUSEast, body["region"])
		assert.Equal(t, "custom upload", body[keyDescription])
		assert.Equal(t, true, body["cloud_init"])
		assert.Equal(t, []any{uploadImageTagProd, uploadImageTagWeb}, body["tags"])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "private/99", result.Image.ID)
	assert.Equal(t, uploadImageLabelFixture, result.Image.Label)
	assert.Equal(t, uploadImageTargetFixture, result.UploadTo)
}

func TestClientUploadImageAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "region is required"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UploadImage(t.Context(), &linode.UploadImageRequest{Label: uploadImageLabelFixture})

	require.Error(t, err, "UploadImage should return API errors")
	assert.ErrorContains(t, err, "region is required")
}

func TestClientUploadImageNetworkError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := srv.URL
	srv.Close()

	client := linode.NewClient(baseURL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UploadImage(t.Context(), &linode.UploadImageRequest{Label: uploadImageLabelFixture, Region: regionUSEast})

	require.Error(t, err, "UploadImage should wrap network errors")

	var networkErr *linode.NetworkError
	require.ErrorAs(t, err, &networkErr, "network error should wrap as NetworkError")
	assert.Equal(t, "UploadImage", networkErr.Operation)
}

func TestClientUploadImageDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	_, err := client.UploadImage(t.Context(), &linode.UploadImageRequest{Label: uploadImageLabelFixture, Region: regionUSEast})

	require.Error(t, err, "UploadImage should fail on 500 response")
	assert.Equal(t, int32(1), calls.Load(), "UploadImage must not retry and replay a mutating request")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/object-storage/quotas", r.URL.Path, "request path should be /object-storage/quotas")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Empty(t, body, "request should not include a body")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    quotas,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	got, err := client.ListObjectStorageQuotas(t.Context())

	require.NoError(t, err, "ListObjectStorageQuotas should succeed on 200 response")
	require.Len(t, got, 1, "one quota should be returned")
	assert.Equal(t, quotaID, got[0][keyID])
}

// TestClientListObjectStorageQuotasRetriesReadOnlyGET verifies the read-only quotas
// route can retry a transient server error without replaying a mutating request.
func TestClientListObjectStorageQuotasRetriesReadOnlyGET(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/object-storage/quotas", r.URL.Path, "request path should be /object-storage/quotas")

		if calls.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
			assert.NoError(t, err)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    []linode.ObjectStorageQuota{{keyID: "endpoint-type-1"}},
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	got, err := client.ListObjectStorageQuotas(t.Context())

	require.NoError(t, err, "ListObjectStorageQuotas should retry and then succeed")
	require.Len(t, got, 1, "one quota should be returned")
	assert.Equal(t, int32(2), calls.Load(), "GET quotas should retry once after transient failure")
}
