package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const endpointFirewallTemplates = "/networking/firewalls/templates"

func TestClientListFirewallTemplatesSuccess(t *testing.T) {
	t.Parallel()

	templates := linode.PaginatedResponse[linode.FirewallTemplate]{
		Data: []linode.FirewallTemplate{{
			Slug: purposeVPC,
			Rules: linode.FirewallRules{
				InboundPolicy:  policyDrop,
				OutboundPolicy: "ACCEPT",
			},
		}},
		Page:    2,
		Pages:   3,
		Results: 5,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallTemplates {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallTemplates)
		}

		if r.URL.Query().Get("page") != "2" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page"), "2")
		}

		if r.URL.Query().Get("page_size") != "50" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page_size"), "50")
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if len(r.URL.Query()["unexpected"]) != 0 {
			t.Errorf("value = %v, want empty", r.URL.Query()["unexpected"])
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(templates); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallTemplates(t.Context(), 2, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Slug != purposeVPC {
		t.Errorf("result.Data[0].Slug = %v, want %v", result.Data[0].Slug, purposeVPC)
	}

	if result.Data[0].Rules.InboundPolicy != policyDrop {
		t.Errorf("result.Data[0].Rules.InboundPolicy = %v, want %v", result.Data[0].Rules.InboundPolicy, policyDrop)
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}
}

func TestClientListFirewallTemplatesHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallTemplates {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallTemplates)
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

	_, err := client.ListFirewallTemplates(t.Context(), 0, 0)
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

func TestClientListFirewallTemplatesRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Error("response writer should support hijacking")

				return
			}

			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Errorf("unexpected error: %v", err)

				return
			}

			if err := conn.Close(); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallTemplates {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallTemplates)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.FirewallTemplate]{
			Data: []linode.FirewallTemplate{{Slug: purposePublic}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListFirewallTemplates(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Slug != purposePublic {
		t.Errorf("result.Data[0].Slug = %v, want %v", result.Data[0].Slug, purposePublic)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestClientGetFirewallTemplateSuccess(t *testing.T) {
	t.Parallel()

	templates := linode.PaginatedResponse[linode.FirewallTemplate]{
		Data: []linode.FirewallTemplate{{
			Slug: purposePublic,
			Rules: linode.FirewallRules{
				InboundPolicy:  policyDrop,
				OutboundPolicy: "ACCEPT",
			},
		}},
		Page:    1,
		Pages:   1,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallTemplates+"/public" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallTemplates+"/public")
		}

		if r.URL.Query().Get("page") != "1" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page"), "1")
		}

		if r.URL.Query().Get("page_size") != "25" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page_size"), "25")
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(templates); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetFirewallTemplate(t.Context(), purposePublic, 1, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Slug != purposePublic {
		t.Errorf("result.Data[0].Slug = %v, want %v", result.Data[0].Slug, purposePublic)
	}

	if result.Data[0].Rules.InboundPolicy != policyDrop {
		t.Errorf("result.Data[0].Rules.InboundPolicy = %v, want %v", result.Data[0].Rules.InboundPolicy, policyDrop)
	}
}

func TestClientGetFirewallTemplateRejectsInvalidSlug(t *testing.T) {
	t.Parallel()

	invalidSlugs := []string{"", "public/vpc", "public?query=1", "public#frag", pathTraversalDotDot, "internal"}
	for _, slug := range invalidSlugs {
		t.Run(slug, func(t *testing.T) {
			t.Parallel()

			var called atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called.Store(true)

				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

			_, err := client.GetFirewallTemplate(t.Context(), slug, 0, 0)
			if !errors.Is(err, linode.ErrInvalidFirewallTemplateSlug) {
				t.Fatalf("error = %v, want %v", err, linode.ErrInvalidFirewallTemplateSlug)
			}

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

func TestClientGetFirewallTemplateHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallTemplates+"/vpc" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallTemplates+"/vpc")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusNotFound)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"not found"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetFirewallTemplate(t.Context(), purposeVPC, 0, 0)
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

func TestClientGetFirewallTemplateRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Error("response writer should support hijacking")

				return
			}

			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Errorf("unexpected error: %v", err)

				return
			}

			if err := conn.Close(); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallTemplates+"/vpc" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallTemplates+"/vpc")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.FirewallTemplate]{
			Data: []linode.FirewallTemplate{{Slug: purposeVPC}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetFirewallTemplate(t.Context(), purposeVPC, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Slug != purposeVPC {
		t.Errorf("result.Data[0].Slug = %v, want %v", result.Data[0].Slug, purposeVPC)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}
