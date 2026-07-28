package linode_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	domainCreateSOAEmail = "admin@example.com"
	domainCreateMaster   = "master"
	domainCreateToken    = "test-token"
	domainCreateJSONNull = "null"
)

func domainCreateRequest() *linode.CreateDomainRequest {
	soaEmail := domainCreateSOAEmail

	return &linode.CreateDomainRequest{
		Domain:   domainExample,
		Type:     domainCreateMaster,
		SOAEmail: &soaEmail,
	}
}

// TestClientCreateDomainProtoDoesNotRetryTransientError pins the no-replay
// property: POST /domains is not idempotent, so a 500 must surface after one
// attempt rather than being replayed into duplicate zones. The second call
// proves the attempt still counted against the circuit breaker.
func TestClientCreateDomainProtoDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Resilience: config.ResilienceConfig{
			CircuitBreakerThreshold: 1,
			CircuitBreakerTimeout:   time.Hour,
		},
	}
	client := linode.NewClient(srv.URL, domainCreateToken, cfg, fastRetryOpts()...)

	if _, err := client.CreateDomainProto(t.Context(), domainCreateRequest()); err == nil {
		t.Fatal("CreateDomainProto() error = nil, want transient API error")
	}

	if got := requests.Load(); got != 1 {
		t.Errorf("request count = %d, want 1", got)
	}

	_, err := client.CreateDomainProto(t.Context(), domainCreateRequest())
	if !errors.Is(err, linode.ErrCircuitOpen) {
		t.Fatalf("CreateDomainProto() error = %v, want %v", err, linode.ErrCircuitOpen)
	}

	if got := requests.Load(); got != 1 {
		t.Errorf("request count after open circuit = %d, want 1", got)
	}
}

// TestClientCreateDomainProtoResponseContract pins which success bodies decode
// and which are rejected. Unknown fields are kept working (the API adds fields
// ahead of the proto), but a non-object body or an incompatible known field is
// an error rather than a silently zero-valued domain.
func TestClientCreateDomainProtoResponseContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantDomain int32
		wantError  bool
	}{
		{name: "object", body: `{"id":7,"future_field":"accepted"}`, wantDomain: 7},
		{name: "unknown fields only", body: `{"future_field":"accepted"}`},
		{name: "malformed", body: `{`, wantError: true},
		{name: "empty", body: ``, wantError: true},
		{name: "trailing json", body: `{}{}`, wantError: true},
		{name: "top-level null", body: domainCreateJSONNull, wantError: true},
		{name: "top-level array", body: `[]`, wantError: true},
		{name: "top-level string", body: `"not-an-object"`, wantError: true},
		{name: "top-level number", body: `7`, wantError: true},
		{name: "top-level boolean", body: `true`, wantError: true},
		{name: "known field has incompatible type", body: `{"id":[]}`, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tcApplicationJSON)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			client := linode.NewClient(srv.URL, domainCreateToken, nil)

			domain, err := client.CreateDomainProto(t.Context(), domainCreateRequest())
			if tt.wantError {
				if err == nil {
					t.Fatal("CreateDomainProto() error = nil, want error")
				}

				return
			}

			if err != nil {
				t.Fatalf("CreateDomainProto() error = %v", err)
			}

			if domain.GetId() != tt.wantDomain {
				t.Errorf("domain ID = %d, want %d", domain.GetId(), tt.wantDomain)
			}
		})
	}
}

func TestClientCreateDomainProtoSurfacesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":[{"field":"soa_email","reason":"invalid soa_email"}]}`))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, domainCreateToken, nil)

	_, err := client.CreateDomainProto(t.Context(), domainCreateRequest())

	apiErr, isAPIError := errors.AsType[*linode.APIError](err)
	if !isAPIError {
		t.Fatalf("CreateDomainProto() error = %v, want *linode.APIError", err)
	}

	if apiErr.Message != "invalid soa_email" {
		t.Errorf("apiErr.Message = %q, want %q", apiErr.Message, "invalid soa_email")
	}

	if apiErr.Field != "soa_email" {
		t.Errorf("apiErr.Field = %q, want %q", apiErr.Field, "soa_email")
	}
}

func TestClientCreateDomainProtoReadError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// A Content-Length longer than the body makes the client's read of the
		// body fail mid-stream, which is the transport failure the decode path
		// has to report rather than treat as an empty response.
		w.Header().Set("Content-Length", strconv.Itoa(len(`{}`)+1))
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, domainCreateToken, nil)

	_, err := client.CreateDomainProto(t.Context(), domainCreateRequest())
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("CreateDomainProto() error = %v, want unexpected EOF", err)
	}
}
