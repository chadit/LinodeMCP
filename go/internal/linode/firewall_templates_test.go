package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointFirewallTemplates, r.URL.Path, "request path should match")
		assert.Equal(t, "2", r.URL.Query().Get("page"), "page query should match")
		assert.Equal(t, "50", r.URL.Query().Get("page_size"), "page_size query should match")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Empty(t, r.URL.Query()["unexpected"], "request should not include extra query parameters")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(templates))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallTemplates(t.Context(), 2, 50)

	require.NoError(t, err, "ListFirewallTemplates should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1, "result should include one template")
	assert.Equal(t, purposeVPC, result.Data[0].Slug)
	assert.Equal(t, policyDrop, result.Data[0].Rules.InboundPolicy)
	assert.Equal(t, 2, result.Page)
}

func TestClientListFirewallTemplatesHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointFirewallTemplates, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListFirewallTemplates(t.Context(), 0, 0)

	require.Error(t, err, "ListFirewallTemplates should fail on HTTP error")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListFirewallTemplatesRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			hj, ok := w.(http.Hijacker)
			if !assert.True(t, ok, "response writer should support hijacking") {
				return
			}

			conn, _, err := hj.Hijack()
			if !assert.NoError(t, err) {
				return
			}

			assert.NoError(t, conn.Close())

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointFirewallTemplates, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.FirewallTemplate]{
			Data: []linode.FirewallTemplate{{Slug: purposePublic}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListFirewallTemplates(t.Context(), 0, 0)

	require.NoError(t, err, "ListFirewallTemplates should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1, "result should include one template")
	assert.Equal(t, purposePublic, result.Data[0].Slug)
	assert.Equal(t, int32(2), requestCount.Load(), "read-only GET should retry once then succeed")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointFirewallTemplates+"/public", r.URL.Path, "request path should match")
		assert.Equal(t, "1", r.URL.Query().Get("page"), "page query should match")
		assert.Equal(t, "25", r.URL.Query().Get("page_size"), "page_size query should match")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(templates))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetFirewallTemplate(t.Context(), purposePublic, 1, 25)

	require.NoError(t, err, "GetFirewallTemplate should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1, "result should include one template")
	assert.Equal(t, purposePublic, result.Data[0].Slug)
	assert.Equal(t, policyDrop, result.Data[0].Rules.InboundPolicy)
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

			require.Error(t, err, "GetFirewallTemplate should reject invalid slug")
			require.ErrorIs(t, err, linode.ErrInvalidFirewallTemplateSlug)
			assert.False(t, called.Load(), "client should not call API for invalid slug")
		})
	}
}

func TestClientGetFirewallTemplateHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointFirewallTemplates+"/vpc", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"not found"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetFirewallTemplate(t.Context(), purposeVPC, 0, 0)

	require.Error(t, err, "GetFirewallTemplate should fail on HTTP error")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
}

func TestClientGetFirewallTemplateRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			hj, ok := w.(http.Hijacker)
			if !assert.True(t, ok, "response writer should support hijacking") {
				return
			}

			conn, _, err := hj.Hijack()
			if !assert.NoError(t, err) {
				return
			}

			assert.NoError(t, conn.Close())

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, endpointFirewallTemplates+"/vpc", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.FirewallTemplate]{
			Data: []linode.FirewallTemplate{{Slug: purposeVPC}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetFirewallTemplate(t.Context(), purposeVPC, 0, 0)

	require.NoError(t, err, "GetFirewallTemplate should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1, "result should include one template")
	assert.Equal(t, purposeVPC, result.Data[0].Slug)
	assert.Equal(t, int32(2), requestCount.Load(), "read-only GET should retry once then succeed")
}
