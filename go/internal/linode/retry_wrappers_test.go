package linode_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func writeRawTestResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()

	_, err := w.Write([]byte(body))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
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

func TestRetryWrappersDelegationPatternsGetFirewallReturnsPointer(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if r.URL.Path != "/networking/firewalls/1" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/firewalls/1")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:     1,
			keyLabel:  "my-fw",
			keyStatus: statusEnabledFixture,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	firewall, err := client.GetFirewall(t.Context(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if firewall == nil {
		t.Fatal("firewall is nil")
	}

	if firewall.ID != 1 {
		t.Errorf("firewall.ID = %v, want %v", firewall.ID, 1)
	}

	if firewall.Label != "my-fw" {
		t.Errorf("firewall.Label = %v, want %v", firewall.Label, "my-fw")
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestRetryWrappersDelegationPatternsDeleteDomainReturnsErrorOnly(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			writeRawTestResponse(t, w, `{"errors":[{"reason":"server error"}]}`)

			return
		}

		if r.URL.Path != "/domains/1" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/domains/1")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	err := client.DeleteDomain(t.Context(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestRetryWrappersDelegationPatternsGetFirewallNoRetryOn401(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusUnauthorized)
		writeRawTestResponse(t, w, `{"errors":[{"reason":"Invalid Token"}]}`)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	_, err := client.GetFirewall(t.Context(), 1)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok || !strings.Contains(apiErr.Message, "Invalid Token") {
		t.Errorf("error %v is not an APIError containing %q", err, "Invalid Token")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestRetryWrappersDelegationPatternsDeleteDomainRecordTwoIdsReturnsError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			writeRawTestResponse(t, w, `{"errors":[{"reason":"server error"}]}`)

			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	err := client.DeleteDomainRecord(t.Context(), 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestGetSSHKeyRetriesHappyPathRetriesOnTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if r.URL.Path != "/profile/sshkeys/42" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/sshkeys/42")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:     42,
			keyLabel:  "my-key",
			keySSHKey: "ssh-rsa AAAA test@example.com",
			"created": "2024-01-01T00:00:00Z",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	sshKey, err := client.GetSSHKey(t.Context(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sshKey == nil {
		t.Fatal("sshKey is nil")
	}

	if sshKey.ID != 42 {
		t.Errorf("sshKey.ID = %v, want %v", sshKey.ID, 42)
	}

	if sshKey.Label != "my-key" {
		t.Errorf("sshKey.Label = %v, want %v", sshKey.Label, "my-key")
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestGetSSHKeyRetriesNoRetryOn401(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusUnauthorized)

		_, err := w.Write([]byte(`{"errors":[{"reason":"Invalid Token"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	_, err := client.GetSSHKey(t.Context(), 42)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok || !strings.Contains(apiErr.Message, "Invalid Token") {
		t.Errorf("error %v is not an APIError containing %q", err, "Invalid Token")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestGetDomainRecordRouteHappyPathUsesExactGETDomainRecordRoute(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/domains/123/records/456" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/domains/123/records/456")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
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
			keyID:     456,
			"type":    "A",
			"name":    "www",
			"target":  "192.0.2.10",
			"ttl_sec": 300,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	record, err := client.GetDomainRecord(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record == nil {
		t.Fatal("record is nil")
	}

	if record.ID != 456 {
		t.Errorf("record.ID = %v, want %v", record.ID, 456)
	}

	if record.Type != "A" {
		t.Errorf("record.Type = %v, want %v", record.Type, "A")
	}

	if record.Name != "www" {
		t.Errorf("record.Name = %v, want %v", record.Name, "www")
	}

	if record.Target != "192.0.2.10" {
		t.Errorf("record.Target = %v, want %v", record.Target, "192.0.2.10")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestGetDomainRecordRouteTransientServerErrorRetriesReadOnlyRequest(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: 456,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	record, err := client.GetDomainRecord(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record == nil {
		t.Fatal("record is nil")
	}

	if record.ID != 456 {
		t.Errorf("record.ID = %v, want %v", record.ID, 456)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestGetDomainRecordRoutePermanentAPIErrorIsNotRetried(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusNotFound)

		_, err := w.Write([]byte(`{"errors":[{"reason":"record not found"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	record, err := client.GetDomainRecord(t.Context(), 123, 456)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if record != nil {
		t.Errorf("record = %v, want nil", record)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}
