package linode_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// Real tools from the generated read surface: a generic primitive is only
// correct against a route it did not spell itself.
const (
	domainGetTool        = "linode_domain_get"
	domainListTool       = "linode_domain_list"
	domainRecordListTool = "linode_domain_record_list"
)

// newRouteTestClient builds a client against a stub server with retries off, so
// a test asserting on one request is not served by a replay of it.
func newRouteTestClient(t *testing.T, baseURL string) *linode.Client {
	t.Helper()

	return linode.NewClient(baseURL, "test-token", nil, linode.WithMaxRetries(0))
}

// TestCallProtoRouteResolvesTheRouteTheToolDeclares names no path, so a wrong
// path could only come from the contract, the one place it is written.
func TestCallProtoRouteResolvesTheRouteTheToolDeclares(t *testing.T) {
	t.Parallel()

	var (
		gotMethod string
		gotPath   string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path

		if _, err := w.Write([]byte(`{"id":5,"domain":"` + domainExample + `"}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)
	domain := &linodev1.Domain{}

	if err := client.CallProtoRoute(t.Context(), domainGetTool, []any{5}, domain); err != nil {
		t.Fatalf("CallProtoRoute: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want %q", gotMethod, http.MethodGet)
	}

	if want := "/domains/5"; !strings.HasSuffix(gotPath, want) {
		t.Errorf("path = %q, want it to end with %q", gotPath, want)
	}

	if domain.GetDomain() != domainExample {
		t.Errorf("decoded domain = %q, want %q", domain.GetDomain(), domainExample)
	}
}

// TestCallProtoRouteReportsTheAPIError pins the failure path: tool handlers wrap
// whatever comes back in their own sentence and would otherwise report success.
func TestCallProtoRouteReportsTheAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		if _, err := w.Write([]byte(`{"errors":[{"reason":"Not found"}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	err := client.CallProtoRoute(t.Context(), domainGetTool, []any{5}, &linodev1.Domain{})

	apiErr, isAPIError := errors.AsType[*linode.APIError](err)
	if !isAPIError {
		t.Fatalf("error = %v, want the API error the shared decoder produces", err)
	}

	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
}

// TestCallProtoRouteRefusesAWrongPathValueCount: a path built from too few
// values addresses the collection the resource sits in, which for a delete is
// every resource in it, so the request must not be sent at all.
func TestCallProtoRouteRefusesAWrongPathValueCount(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("a request reached the server, want none")
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	if err := client.CallProtoRoute(t.Context(), domainGetTool, nil, &linodev1.Domain{}); err == nil {
		t.Fatal("CallProtoRoute succeeded with no path value, want a refusal")
	}
}

func TestListProtoRouteFetchesTheDeclaredCollection(t *testing.T) {
	t.Parallel()

	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path

		if _, err := w.Write([]byte(`{"data":[{"id":1,"domain":"a.com"},{"id":2,"domain":"b.com"}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	items, err := linode.ListProtoRoute(t.Context(), client, domainListTool, nil, 0, 0,
		func() *linodev1.Domain { return &linodev1.Domain{} })
	if err != nil {
		t.Fatalf("ListProtoRoute: %v", err)
	}

	if want := "/domains"; !strings.HasSuffix(gotPath, want) {
		t.Errorf("path = %q, want it to end with %q", gotPath, want)
	}

	if len(items) != 2 || items[1].GetDomain() != "b.com" {
		t.Fatalf("items = %v, want the two decoded domains", items)
	}
}

// TestListProtoRouteFillsASubresourcePath pins that one primitive serves nested
// collections too, so list tiers differ only in whether a path value travels.
func TestListProtoRouteFillsASubresourcePath(t *testing.T) {
	t.Parallel()

	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path

		if _, err := w.Write([]byte(`{"data":[]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	if _, err := linode.ListProtoRoute(t.Context(), client, domainRecordListTool, []any{5}, 0, 0,
		func() *linodev1.DomainRecord { return &linodev1.DomainRecord{} }); err != nil {
		t.Fatalf("ListProtoRoute: %v", err)
	}

	if want := "/domains/5/records"; !strings.HasSuffix(gotPath, want) {
		t.Errorf("path = %q, want it to end with %q", gotPath, want)
	}
}

// TestListProtoRouteSendsThePageControls: a page the request never asks for
// means every caller silently reads page one.
func TestListProtoRouteSendsThePageControls(t *testing.T) {
	t.Parallel()

	var gotQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery

		if _, err := w.Write([]byte(`{"data":[]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	if _, err := linode.ListProtoRoute(t.Context(), client, domainListTool, nil, 2, 50,
		func() *linodev1.Domain { return &linodev1.Domain{} }); err != nil {
		t.Fatalf("ListProtoRoute: %v", err)
	}

	for _, want := range []string{"page=2", "page_size=50"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query = %q, want it to carry %q", gotQuery, want)
		}
	}
}
