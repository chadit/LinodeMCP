package linode_test

import (
	"context"
	"encoding/json"
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
	updateAccountEmail         = "account-updated@example.com"
	statusPending              = "pending"
	accountEntityTransferToken = "transfer-token-example"
	accountEntityTransferDate  = "2021-02-11T16:37:03"
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

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
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
	assert.Equal(t, int32(1), calls, "AllowObjectStorageBucketAccess must not retry and replay a state-changing request")
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
	assert.Equal(t, "forbidden", apiErr.Message)
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
	assert.Equal(t, "forbidden", apiErr.Message)
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
	assert.Equal(t, "forbidden", apiErr.Message)
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
	assert.Equal(t, "forbidden", apiErr.Message)
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
	assert.Equal(t, "forbidden", apiErr.Message)
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
	assert.Equal(t, "forbidden", apiErr.Message)
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

	want := linode.AccountEvent{ID: 123, Action: "linode_create", Status: "successful", Username: "test-user"}

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
	assert.Equal(t, "successful", got.Status)
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
	assert.Equal(t, "forbidden", apiErr.Message)
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

// TestClientListAccountEntityTransfersSuccess verifies ListAccountEntityTransfers sends a GET
// request to /account/entity-transfers with pagination query parameters.
func TestClientListAccountEntityTransfersSuccess(t *testing.T) {
	t.Parallel()

	transfers := linode.PaginatedResponse[linode.AccountEntityTransfer]{
		Data: []linode.AccountEntityTransfer{{
			Created:  accountEntityTransferDate,
			Entities: linode.AccountEntityTransferEntities{Linodes: []int{111, 222}},
			Expiry:   "2021-02-12T16:37:03",
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
	assert.Equal(t, "forbidden", apiErr.Message)
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

// TestClientGetAccountEntityTransferSuccess verifies GetAccountEntityTransfer sends a GET
// request to /account/entity-transfers/{token} and decodes the response.
func TestClientGetAccountEntityTransferSuccess(t *testing.T) {
	t.Parallel()

	want := linode.AccountEntityTransfer{
		Created:  accountEntityTransferDate,
		Entities: linode.AccountEntityTransferEntities{Linodes: []int{111, 222}},
		Expiry:   "2021-02-12T16:37:03",
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
	assert.Equal(t, "forbidden", apiErr.Message)
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
	assert.Equal(t, "forbidden", apiErr.Message)
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
	assert.Equal(t, "forbidden", apiErr.Message)
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
	assert.Equal(t, "forbidden", apiErr.Message)
}

// TestClientListAccountLoginsSuccess verifies ListAccountLogins sends a GET
// request to /account/logins with pagination query parameters.
func TestClientListAccountLoginsSuccess(t *testing.T) {
	t.Parallel()

	logins := linode.PaginatedResponse[linode.AccountLogin]{
		Data: []linode.AccountLogin{{
			Datetime:   "2024-01-02T03:04:05",
			ID:         123,
			IP:         "203.0.113.10",
			Restricted: false,
			Status:     "successful",
			Username:   "account-login-user",
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
	assert.Equal(t, "account-login-user", result.Data[0].Username)
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
	assert.Equal(t, "forbidden", apiErr.Message)
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
			Data: []linode.AccountLogin{{ID: 123, Username: "account-login-user"}},
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
	assert.Equal(t, "forbidden", apiErr.Message)
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
	assert.Equal(t, "forbidden", apiErr.Message)
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
	assert.Equal(t, "forbidden", apiErr.Message)
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
	assert.Equal(t, "forbidden", apiErr.Message)
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
	assert.Equal(t, "forbidden", apiErr.Message)
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
	assert.Equal(t, "forbidden", apiErr.Message)
}

// TestClientEnrollAccountBetaDoesNotRetry verifies the mutating beta enrollment
// is not replayed after a transient HTTP error.
func TestClientEnrollAccountBetaDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)

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
	assert.Equal(t, int32(1), calls, "EnrollAccountBeta must not retry and replay a mutating request")
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

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)

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
	assert.Equal(t, int32(1), calls, "AcknowledgeAccountAgreements must not retry and replay a mutating request")
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

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)

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
	assert.Equal(t, int32(1), calls, "CancelAccount must not retry and replay a destructive request")
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

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)

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
	assert.Equal(t, int32(1), calls, "UpdateAccount must not retry and replay a mutating request")
}
