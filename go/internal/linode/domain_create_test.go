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
)

func TestClientCreateDomainProtoDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
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
	client := linode.NewClient(srv.URL, "test-token", cfg, fastRetryOpts()...)
	soaEmail := domainCreateSOAEmail

	_, err := client.CreateDomainProto(t.Context(), &linode.CreateDomainRequest{
		Domain:   domainExample,
		Type:     domainCreateMaster,
		SOAEmail: &soaEmail,
	})
	if err == nil {
		t.Fatal("CreateDomainProto() error = nil, want transient API error")
	}

	if got := requests.Load(); got != 1 {
		t.Errorf("request count = %d, want 1", got)
	}

	_, err = client.CreateDomainProto(t.Context(), &linode.CreateDomainRequest{
		Domain:   domainExample,
		Type:     domainCreateMaster,
		SOAEmail: &soaEmail,
	})
	if !errors.Is(err, linode.ErrCircuitOpen) {
		t.Fatalf("CreateDomainProto() error = %v, want %v", err, linode.ErrCircuitOpen)
	}

	if got := requests.Load(); got != 1 {
		t.Errorf("request count after open circuit = %d, want 1", got)
	}
}

func TestClientCreateDomainProtoResponseContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantDomain int32
		wantError  bool
	}{
		{name: "object", body: `{"id":7,"future_field":"accepted"}`, wantDomain: 7},
		{name: "malformed", body: `{`, wantError: true},
		{name: "null", body: `null`, wantError: true},
		{name: "known field has incompatible type", body: `{"id":[]}`, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			client := linode.NewClient(srv.URL, "test-token", nil)
			soaEmail := domainCreateSOAEmail

			domain, err := client.CreateDomainProto(t.Context(), &linode.CreateDomainRequest{
				Domain:   domainExample,
				Type:     domainCreateMaster,
				SOAEmail: &soaEmail,
			})
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

func TestClientCreateDomainProtoReadError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(`{}`)+1))
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil)
	soaEmail := domainCreateSOAEmail

	_, err := client.CreateDomainProto(t.Context(), &linode.CreateDomainRequest{
		Domain:   domainExample,
		Type:     domainCreateMaster,
		SOAEmail: &soaEmail,
	})
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("CreateDomainProto() error = %v, want unexpected EOF", err)
	}
}
