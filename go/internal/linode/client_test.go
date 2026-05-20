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

const updateAccountEmail = "account-updated@example.com"

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
