package linode_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	accountTransferRegion = "us-east"
	errForbidden          = "forbidden"
	oauthClientID         = "client-123"
	oauthClientStatus     = "active"
	// profileGetTool is the read the retry, breaker and limiter tests probe
	// the client with: a real declared route with no path values to fill.
	profileGetTool = "linode_profile_get"
)

// readProfile is that probe: a routed JSON read of the profile, decoded into
// the hand-written type the scope validator reads it into.
func readProfile(ctx context.Context, client *linode.Client) (*linode.Profile, error) {
	var profile linode.Profile
	if err := client.CallRouteJSON(ctx, profileGetTool, nil, &profile); err != nil {
		return nil, fmt.Errorf("read profile: %w", err)
	}

	return &profile, nil
}

// fastRetryOpts returns Option values with minimal delays for testing.
func fastRetryOpts() []linode.Option {
	return []linode.Option{
		linode.WithMaxRetries(3),
		linode.WithBaseDelay(1 * time.Millisecond),
		linode.WithMaxDelay(10 * time.Millisecond),
		linode.WithBackoffFactor(2.0),
		linode.WithJitter(false),
	}
}

// TestClientCallRouteJSONDecodesTheDeclaredRoute verifies the JSON read
// primitive reaches the route its tool declares, sends the token, and decodes
// the 200 body into the hand-written type it was handed.
func TestClientCallRouteJSONDecodesTheDeclaredRoute(t *testing.T) {
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

	result, err := readProfile(t.Context(), client)
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

// TestClientCallRouteJSONUnauthorized verifies that the JSON read returns an
// APIError with status 401 when the API rejects the token.
func TestClientCallRouteJSONUnauthorized(t *testing.T) {
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

	_, err := readProfile(t.Context(), client)
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

// TestClientCallRouteJSONNetworkError verifies that the JSON read returns a
// NetworkError when the server is unreachable.
func TestClientCallRouteJSONNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "token", nil, linode.WithMaxRetries(0))

	_, err := readProfile(t.Context(), client)
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

	_, err := readProfile(t.Context(), client)
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

	_, err := readProfile(t.Context(), client)
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

// TestClientContextCancelled verifies that a routed read returns an error
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

	_, err := readProfile(ctx, client)
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

	_, err := readProfile(t.Context(), client)
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

	_, err := readProfile(t.Context(), client)
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

	_, err := readProfile(t.Context(), client)
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

	_, err := readProfile(t.Context(), client)
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

// The route the thumbnail update declares carries retry_disabled: replaying a
// PUT would resend the whole image over a replacement that may already have
// landed.
func TestCallRouteRawBodyHonorsTheDeclaredRetryPolicy(t *testing.T) {
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

	err := client.CallRouteRawBody(t.Context(), thumbnailUpdateTool,
		[]any{oauthClientID}, "image/png", []byte("png-bytes"))
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

// The read declares no such policy, so a transient failure is replayed and the
// second attempt's bytes are the answer.
func TestCallRouteRawBodyReadRetriesATransientFailure(t *testing.T) {
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

		if _, writeErr := w.Write(thumbnailPNG); writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	got, err := client.CallRouteRawBodyRead(t.Context(), thumbnailGetTool,
		[]any{oauthClientID}, "image/png")
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

func TestClientListAccountEntityTransfersRemoved(t *testing.T) {
	t.Parallel()

	_, ok := reflect.TypeFor[*linode.Client]().MethodByName("ListAccountEntityTransfers")
	if ok {
		t.Error("ok = true, want false")
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

// TestNewClientCarriesTheObjectStorageSettings pins the seam the Object Storage
// upload depends on: its transfer is handed a client and no config, so the
// data-plane budgets have to arrive on the client or the engine cannot read
// them.
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
