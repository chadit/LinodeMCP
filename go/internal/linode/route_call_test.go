package linode_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// Real tools from the generated read surface: a generic primitive is only
// correct against a route it did not spell itself.
const (
	domainGetTool        = "linode_domain_get"
	domainListTool       = "linode_domain_list"
	domainRecordListTool = "linode_domain_record_list"
	// domainCreateTool declares retry_disabled; domainUpdateTool does not, and
	// the pair is what makes the policy read observable.
	domainCreateTool = "linode_domain_create"
	domainUpdateTool = "linode_domain_update"
	domainDeleteTool = "linode_domain_delete"
	// backupsCancelTool is replayable and lkeACLDeleteTool declares
	// retry_disabled, which is what makes the destroy tier's policy read
	// observable now that every DELETE the tier serves refuses a replay.
	backupsCancelTool  = "linode_instance_backups_cancel"
	phoneVerifyTool    = "linode_profile_phone_number_verify"
	managedEnableTool  = "linode_account_settings_managed_enable"
	instanceCreateTool = "linode_instance_create"
	lkeACLDeleteTool   = "linode_lke_acl_delete"
	// instanceFirewallUpdateTool is one of the two mutations whose route
	// answers with a page, and markerListTool is a collection declaring the
	// one envelope that page primitive refuses.
	instanceFirewallUpdateTool = "linode_instance_firewall_update"
	markerListTool             = "linode_object_storage_bucket_object_list"

	// domainFive is the resource every delete case here addresses.
	domainFive = "/domains/5"

	// domainCreateSubject is the prose the create tool derives its name to,
	// which is what a malformed answer is reported against.
	domainCreateSubject = "domain create"
)

// newRouteTestClient builds a client against a stub server with retries off, so
// a test asserting on one request is not served by a replay of it.
func newRouteTestClient(t *testing.T, baseURL string) *linode.Client {
	t.Helper()

	return linode.NewClient(baseURL, "test-token", nil, linode.WithMaxRetries(0))
}

// TestCallProtoRouteQueryResolvesTheRouteTheToolDeclares names no path, so a
// wrong path could only come from the contract, the one place it is written.
func TestCallProtoRouteQueryResolvesTheRouteTheToolDeclares(t *testing.T) {
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

	if err := client.CallProtoRouteQuery(t.Context(), domainGetTool, []any{5}, "", domain); err != nil {
		t.Fatalf("CallProtoRouteQuery: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want %q", gotMethod, http.MethodGet)
	}

	if !strings.HasSuffix(gotPath, domainFive) {
		t.Errorf("path = %q, want it to end with %q", gotPath, domainFive)
	}

	if domain.GetDomain() != domainExample {
		t.Errorf("decoded domain = %q, want %q", domain.GetDomain(), domainExample)
	}
}

// TestCallProtoRouteQueryReportsTheAPIError pins the failure path: tool
// handlers wrap whatever comes back in their own sentence and would otherwise
// report success.
func TestCallProtoRouteQueryReportsTheAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		if _, err := w.Write([]byte(`{"errors":[{"reason":"Not found"}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	err := client.CallProtoRouteQuery(t.Context(), domainGetTool, []any{5}, "", &linodev1.Domain{})

	apiErr, isAPIError := errors.AsType[*linode.APIError](err)
	if !isAPIError {
		t.Fatalf("error = %v, want the API error the shared decoder produces", err)
	}

	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
}

// TestCallProtoRouteQueryRefusesAWrongPathValueCount: a path built from too
// few values addresses the collection the resource sits in, which for a delete
// is every resource in it, so the request must not be sent at all.
func TestCallProtoRouteQueryRefusesAWrongPathValueCount(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("a request reached the server, want none")
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	if err := client.CallProtoRouteQuery(t.Context(), domainGetTool, nil, "", &linodev1.Domain{}); err == nil {
		t.Fatal("CallProtoRouteQuery succeeded with no path value, want a refusal")
	}
}

// TestCallProtoRouteBodySendsTheBodyOnTheDeclaredRoute: a write that reaches
// the right path with the wrong bytes creates the wrong resource, so both
// halves are checked in one call.
func TestCallProtoRouteBodySendsTheBodyOnTheDeclaredRoute(t *testing.T) {
	t.Parallel()

	var (
		gotMethod string
		gotPath   string
		gotBody   string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path

		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}

		gotBody = string(raw)

		if _, err := w.Write([]byte(`{"id":7,"domain":"` + domainExample + `"}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)
	created := &linodev1.Domain{}
	body := map[string]any{"domain": domainExample}

	if err := client.CallProtoRouteBody(t.Context(), domainCreateTool, nil, body, "", created); err != nil {
		t.Fatalf("CallProtoRouteBody: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want %q", gotMethod, http.MethodPost)
	}

	if want := "/domains"; !strings.HasSuffix(gotPath, want) {
		t.Errorf("path = %q, want it to end with %q", gotPath, want)
	}

	if want := `{"domain":"` + domainExample + `"}`; gotBody != want {
		t.Errorf("body = %q, want %q", gotBody, want)
	}

	if created.GetId() != 7 {
		t.Errorf("decoded id = %d, want 7", created.GetId())
	}
}

// TestCallProtoRouteBodyFillsThePathTemplate pins that a body-carrying call
// addresses one resource. An update that reached the collection would rewrite
// something the caller never named.
func TestCallProtoRouteBodyFillsThePathTemplate(t *testing.T) {
	t.Parallel()

	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path

		if _, err := w.Write([]byte(`{"id":5}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	err := client.CallProtoRouteBody(t.Context(), domainUpdateTool, []any{5},
		map[string]any{keyDescription: "x"}, "", &linodev1.Domain{})
	if err != nil {
		t.Fatalf("CallProtoRouteBody: %v", err)
	}

	if !strings.HasSuffix(gotPath, domainFive) {
		t.Errorf("path = %q, want it to end with %q", gotPath, domainFive)
	}
}

// TestCallProtoRouteBodyHonorsTheDeclaredRetryPolicy is the reason the policy
// is read from the contract at all: a replayed create leaves a second resource
// the caller is billed for and never hears about. The two tools differ only in
// what they declare, so a policy read that ignored the contract would make both
// counts the same.
func TestCallProtoRouteBodyHonorsTheDeclaredRetryPolicy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		tool     string
		values   []any
		attempts int
	}{
		{tool: domainCreateTool, values: nil, attempts: 1},
		{tool: domainUpdateTool, values: []any{5}, attempts: 2},
	}

	for _, testCase := range cases {
		t.Run(testCase.tool, func(t *testing.T) {
			t.Parallel()

			var attempts int

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts++

				// 429 is retryable whatever the method, so the only thing
				// deciding whether a second request arrives is the policy.
				w.WriteHeader(http.StatusTooManyRequests)

				if _, err := w.Write([]byte(`{"errors":[{"reason":"slow down"}]}`)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			client := linode.NewClient(server.URL, "test-token", nil,
				linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond))

			err := client.CallProtoRouteBody(t.Context(), testCase.tool, testCase.values,
				map[string]any{}, "", &linodev1.Domain{})
			if err == nil {
				t.Fatal("CallProtoRouteBody succeeded against a rate-limited server, want a failure")
			}

			if attempts != testCase.attempts {
				t.Errorf("attempts = %d, want %d", attempts, testCase.attempts)
			}
		})
	}
}

// TestCallProtoRouteBodyRefusesAWrongPathValueCount: a body sent to the
// collection instead of the resource is the create the caller did not ask for.
func TestCallProtoRouteBodyRefusesAWrongPathValueCount(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("a request reached the server, want none")
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	err := client.CallProtoRouteBody(t.Context(), domainUpdateTool, nil,
		map[string]any{}, "", &linodev1.Domain{})
	if err == nil {
		t.Fatal("CallProtoRouteBody succeeded with no path value, want a refusal")
	}
}

// TestCallProtoRouteBodyHoldsTheAnswerToAnObject: the subject is what makes the
// report name the call, so it is asserted whole rather than by its suffix.
func TestCallProtoRouteBodyHoldsTheAnswerToAnObject(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name      string
		subject   string
		answer    string
		wantGuard bool
	}{
		{name: "array answer", subject: domainCreateSubject, answer: jsonBodyArray, wantGuard: true},
		{name: "scalar answer", subject: domainCreateSubject, answer: `5`, wantGuard: true},
		// Bytes that are not JSON at all stay the decoder's to report, so a
		// truncated answer still names where it broke.
		{name: "unparseable answer decodes", subject: domainCreateSubject, answer: `not json`, wantGuard: false},
		{name: "empty answer decodes", subject: domainCreateSubject, answer: ``, wantGuard: false},
		{name: "no subject decodes", subject: "", answer: jsonBodyArray, wantGuard: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if _, err := w.Write([]byte(testCase.answer)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			client := newRouteTestClient(t, server.URL)

			err := client.CallProtoRouteBody(t.Context(), domainCreateTool, nil,
				map[string]any{}, testCase.subject, &linodev1.Domain{})

			guarded := errors.Is(err, linode.ErrWriteResponseNotObject)
			if err == nil || guarded != testCase.wantGuard {
				t.Errorf("error = %v (guard %v), want a refusal with guard %v", err, guarded, testCase.wantGuard)
			}

			if testCase.wantGuard && err.Error() != testCase.subject+" response must be a JSON object" {
				t.Errorf("error = %q, want the subject to name the call", err)
			}
		})
	}
}

// TestCallProtoRouteBodyRawHoldsTheAnswerToAnObject covers the explicit-nulls
// path, which reaches the same check through its own primitive.
func TestCallProtoRouteBodyRawHoldsTheAnswerToAnObject(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`[]`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	_, err := client.CallProtoRouteBodyRaw(t.Context(), domainCreateTool, nil,
		map[string]any{}, domainCreateSubject, &linodev1.Domain{})
	if !errors.Is(err, linode.ErrWriteResponseNotObject) {
		t.Errorf("error = %v, want it to wrap ErrWriteResponseNotObject", err)
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

	items, err := linode.ListProtoRoute(t.Context(), client, domainListTool, nil, "", 0, 0,
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

	if _, err := linode.ListProtoRoute(t.Context(), client, domainRecordListTool, []any{5}, "", 0, 0,
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

	if _, err := linode.ListProtoRoute(t.Context(), client, domainListTool, nil, "", 2, 50,
		func() *linodev1.Domain { return &linodev1.Domain{} }); err != nil {
		t.Fatalf("ListProtoRoute: %v", err)
	}

	for _, want := range []string{"page=2", "page_size=50"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query = %q, want it to carry %q", gotQuery, want)
		}
	}
}

// TestCallRouteSendsNoBodyOnTheDeclaredRoute: the destroy tier's whole request
// is its route, so a call that names only its tool has to reach the right
// method and path and put nothing on the wire.
func TestCallRouteSendsNoBodyOnTheDeclaredRoute(t *testing.T) {
	t.Parallel()

	var (
		gotMethod string
		gotPath   string
		gotBody   string
	)

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path

		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}

		gotBody = string(raw)
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	if err := client.CallRoute(t.Context(), domainDeleteTool, []any{5}); err != nil {
		t.Fatalf("CallRoute: %v", err)
	}

	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want %q", gotMethod, http.MethodDelete)
	}

	if !strings.HasSuffix(gotPath, domainFive) {
		t.Errorf("path = %q, want it to end with %q", gotPath, domainFive)
	}

	if gotBody != "" {
		t.Errorf("body = %q, want it empty", gotBody)
	}
}

// TestCallRouteAcceptsAnEmptyAnswer: a DELETE reports the removal by status and
// sends no JSON back, so a primitive that insisted on decoding one would fail
// every call it made.
func TestCallRouteAcceptsAnEmptyAnswer(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	if err := client.CallRoute(t.Context(), domainDeleteTool, []any{5}); err != nil {
		t.Fatalf("CallRoute: %v", err)
	}
}

// TestCallRouteReportsTheAPIFailure: the destroy flow answers with its success
// message the moment this returns nil, so a rejected delete that read as
// success would tell a caller their resource is gone while it is not.
func TestCallRouteReportsTheAPIFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		if _, err := w.Write([]byte(`{"errors":[{"reason":"Not found"}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	if err := client.CallRoute(t.Context(), domainDeleteTool, []any{5}); err == nil {
		t.Fatal("CallRoute succeeded against a 404, want a failure")
	}
}

// TestCallRouteHonorsTheDeclaredRetryPolicy: every DELETE the destroy tier
// serves refuses a replay, and the tier's replayable half is the POST actions,
// so the contract is where that is written. The two tools differ only in what
// they declare.
func TestCallRouteHonorsTheDeclaredRetryPolicy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		tool     string
		attempts int
	}{
		{tool: lkeACLDeleteTool, attempts: 1},
		{tool: backupsCancelTool, attempts: 2},
	}

	for _, testCase := range cases {
		t.Run(testCase.tool, func(t *testing.T) {
			t.Parallel()

			var attempts int

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts++

				// 429 is retryable whatever the method, so the only thing
				// deciding whether a second request arrives is the policy.
				w.WriteHeader(http.StatusTooManyRequests)

				if _, err := w.Write([]byte(`{"errors":[{"reason":"slow down"}]}`)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			client := linode.NewClient(server.URL, "test-token", nil,
				linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond))

			if err := client.CallRoute(t.Context(), testCase.tool, []any{5}); err == nil {
				t.Fatal("CallRoute succeeded against a rate-limited server, want a failure")
			}

			if attempts != testCase.attempts {
				t.Errorf("attempts = %d, want %d", attempts, testCase.attempts)
			}
		})
	}
}

// TestCallRouteRefusesAWrongPathValueCount: a delete addressed at the
// collection instead of the resource is the one mistake this tier cannot take
// back, so the refusal has to come before the request goes out.
func TestCallRouteRefusesAWrongPathValueCount(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("a request reached the server, want none")
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	if err := client.CallRoute(t.Context(), domainDeleteTool, nil); err == nil {
		t.Fatal("CallRoute succeeded with no path value, want a refusal")
	}
}

// A single-resource route can publish a query of its own: the page controls
// /databases/types/{id} takes, the object key the bucket ACL route is
// addressed by. The query the caller composed has to reach the request.
func TestCallProtoRouteQuerySendsTheQueryItWasGiven(t *testing.T) {
	t.Parallel()

	var gotQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := io.WriteString(w, `{"id":5}`); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))

	t.Cleanup(server.Close)

	client := newRouteTestClient(t, server.URL)

	domain := &linodev1.Domain{}
	if err := client.CallProtoRouteQuery(t.Context(), domainGetTool, []any{5},
		"page=2&page_size=50", domain); err != nil {
		t.Fatalf("CallProtoRouteQuery: %v", err)
	}

	if want := "page=2&page_size=50"; gotQuery != want {
		t.Errorf("query = %q, want %q", gotQuery, want)
	}

	if domain.GetId() != 5 {
		t.Errorf("domain.GetId() = %v, want %v", domain.GetId(), 5)
	}
}

// TestCallRouteBodySendsTheBodyOnTheDeclaredRoute: the acknowledged tier's
// request is a body over a route with nothing to decode back, so what reaches
// the wire is the only thing worth asserting.
func TestCallRouteBodySendsTheBodyOnTheDeclaredRoute(t *testing.T) {
	t.Parallel()

	var (
		gotMethod string
		gotPath   string
		gotBody   string
	)

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path

		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}

		gotBody = string(raw)
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)
	body := map[string]any{"otp_code": "123456"}

	if err := client.CallRouteBody(t.Context(), phoneVerifyTool, nil, body); err != nil {
		t.Fatalf("CallRouteBody: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want %q", gotMethod, http.MethodPost)
	}

	if gotPath != clientRoutePathProfilePhoneNumberVerify {
		t.Errorf("path = %q, want the declared verify route", gotPath)
	}

	if gotBody != `{"otp_code":"123456"}` {
		t.Errorf("body = %q, want the map it was handed", gotBody)
	}
}

// TestCallRouteBodyReportsAFailedCall: nothing is decoded out of the answer, so
// the status is the only thing saying whether the change happened.
func TestCallRouteBodyReportsAFailedCall(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)

		if _, err := w.Write([]byte(`{"errors":[{"reason":"bad code"}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	if err := client.CallRouteBody(t.Context(), phoneVerifyTool, nil, map[string]any{}); err == nil {
		t.Fatal("CallRouteBody succeeded against a 400, want a failure")
	}
}

// TestCallRouteBodyRefusesAWrongPathValueCount: a mutation aimed at the wrong
// resource is refused before the request goes out, the same as a delete.
func TestCallRouteBodyRefusesAWrongPathValueCount(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("a request reached the server, want none")
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	if err := client.CallRouteBody(t.Context(), domainDeleteTool, nil, map[string]any{}); err == nil {
		t.Fatal("CallRouteBody succeeded with no path value, want a refusal")
	}
}

// TestCallRouteBodyHonorsTheDeclaredRetryPolicy: a mutation the API cannot be
// asked to repeat is one the contract marks, not one each call site decides.
func TestCallRouteBodyHonorsTheDeclaredRetryPolicy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		tool     string
		attempts int
	}{
		{tool: instanceCreateTool, attempts: 1},
		{tool: phoneVerifyTool, attempts: 1},
		{tool: managedEnableTool, attempts: 2},
	}

	for _, testCase := range cases {
		t.Run(testCase.tool, func(t *testing.T) {
			t.Parallel()

			var attempts int

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts++

				// 429 is retryable whatever the method, so the only thing
				// deciding whether a second request arrives is the policy.
				w.WriteHeader(http.StatusTooManyRequests)

				if _, err := w.Write([]byte(`{"errors":[{"reason":"slow down"}]}`)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			client := linode.NewClient(server.URL, "test-token", nil,
				linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond))

			if err := client.CallRouteBody(t.Context(), testCase.tool, nil, map[string]any{}); err == nil {
				t.Fatal("CallRouteBody succeeded against a rate-limited server, want a failure")
			}

			if attempts != testCase.attempts {
				t.Errorf("attempts = %d, want %d", attempts, testCase.attempts)
			}
		})
	}
}

// TestCallProtoRouteBodyRawReportsATransportFailure pins the raw path's share
// of the failure taxonomy: a connection that never completes answers as a
// request error, not as a decode or guard failure.
func TestCallProtoRouteBodyRawReportsATransportFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()

	client := newRouteTestClient(t, server.URL)

	_, err := client.CallProtoRouteBodyRaw(t.Context(), domainCreateTool, nil,
		map[string]any{}, domainCreateSubject, &linodev1.Domain{})
	if err == nil {
		t.Fatal("raw call against a closed server succeeded, want an error")
	}
}

// TestCallProtoRouteBodyRawRetriesWhenTheContractAllows covers the raw path's
// retrying branch, which domain_update takes because it declares no
// retry_disabled.
func TestCallProtoRouteBodyRawRetriesWhenTheContractAllows(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`{"id": 5, "domain": "example.com"}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	raw, err := client.CallProtoRouteBodyRaw(t.Context(), domainUpdateTool, []any{5},
		map[string]any{}, "domain update", &linodev1.Domain{})
	if err != nil {
		t.Fatalf("CallProtoRouteBodyRaw: %v", err)
	}

	if !strings.Contains(string(raw), "example.com") {
		t.Errorf("raw body = %s, want the API answer", raw)
	}
}

// TestCallRouteObjectAnswersTheBodyAsAnObject pins the free-form read
// primitive: the body arrives as a map, no proto model in between.
func TestCallRouteObjectAnswersTheBodyAsAnObject(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`{"binlog_expire_seconds": {"type": "integer"}}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	object, err := client.CallRouteObject(t.Context(), "linode_domain_get", []any{5}, "")
	if err != nil {
		t.Fatalf("CallRouteObject: %v", err)
	}

	if _, present := object["binlog_expire_seconds"]; !present {
		t.Errorf("object = %v, want the API body's members", object)
	}
}

// TestCallRouteObjectReportsATransportFailure keeps the free-form read inside
// the same failure taxonomy as every other primitive.
func TestCallRouteObjectReportsATransportFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()

	client := newRouteTestClient(t, server.URL)

	if _, err := client.CallRouteObject(t.Context(), "linode_domain_get", []any{5}, ""); err == nil {
		t.Fatal("free-form read against a closed server succeeded, want an error")
	}
}

// The two firewall replacements are the only mutations whose route answers with
// a page. TestListProtoRouteBodyDecodesThePageAMutationAnswersWith pins that
// the elements come out of the {data} envelope rather than out of a decode into
// the tool's own response, which places none of them.
func TestListProtoRouteBodyDecodesThePageAMutationAnswersWith(t *testing.T) {
	t.Parallel()

	var (
		gotMethod string
		gotQuery  string
		gotBody   string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotQuery = r.Method, r.URL.RawQuery

		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}

		gotBody = string(raw)

		page := `{"data":[{"id":11,"label":"web"}],"page":1,"pages":1,"results":1}`
		if _, err := w.Write([]byte(page)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	items, err := linode.ListProtoRouteBody(t.Context(), client, instanceFirewallUpdateTool,
		[]any{5}, "page=2", map[string]any{"firewall_ids": []int{11}},
		func() *linodev1.Firewall { return &linodev1.Firewall{} })
	if err != nil {
		t.Fatalf("ListProtoRouteBody: %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want %q", gotMethod, http.MethodPut)
	}

	if gotQuery != "page=2" {
		t.Errorf("query = %q, want the encoded page controls", gotQuery)
	}

	if !strings.Contains(gotBody, "firewall_ids") {
		t.Errorf("body = %q, want the assignments the caller sent", gotBody)
	}

	if len(items) != 1 || items[0].GetId() != 11 || items[0].GetLabel() != "web" {
		t.Errorf("items = %v, want the one firewall the page carried", items)
	}
}

// A route answering any shape other than the standard page would decode to
// nothing here and report success, so the declared envelope is checked rather
// than assumed. linode_object_storage_bucket_object_list pages by marker.
func TestListProtoRouteBodyRefusesAnotherEnvelope(t *testing.T) {
	t.Parallel()

	client := newRouteTestClient(t, "http://127.0.0.1:1")

	_, err := linode.ListProtoRouteBody(t.Context(), client, markerListTool, []any{"us-east", "b"}, "",
		map[string]any{}, func() *linodev1.Firewall { return &linodev1.Firewall{} })
	if !errors.Is(err, linode.ErrShapeMismatch) {
		t.Errorf("error = %v, want the shape mismatch", err)
	}
}

// TestCallProtoRouteBodyQueryCarriesTheQuery pins the mutation primitive that
// takes one: a page control dropped here would send the request the caller did
// not ask for.
func TestCallProtoRouteBodyQueryCarriesTheQuery(t *testing.T) {
	t.Parallel()

	var gotQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery

		if _, err := w.Write([]byte(`{"id":5}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	err := client.CallProtoRouteBodyQuery(t.Context(), domainUpdateTool, []any{5}, "page=3",
		map[string]any{keyDescription: "x"}, "", &linodev1.Domain{})
	if err != nil {
		t.Fatalf("CallProtoRouteBodyQuery: %v", err)
	}

	if gotQuery != "page=3" {
		t.Errorf("query = %q, want the encoded controls", gotQuery)
	}
}

// Both new mutation primitives report a transport failure the way every other
// one does, so a caller hears which call could not be made.
func TestQueryingMutationPrimitivesReportATransportFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()

	client := newRouteTestClient(t, server.URL)
	body := map[string]any{}

	err := client.CallProtoRouteBodyQuery(t.Context(), domainUpdateTool, []any{5}, "page=1",
		body, "", &linodev1.Domain{})
	if err == nil {
		t.Error("queried mutation against a closed server succeeded, want an error")
	}

	_, err = linode.ListProtoRouteBody(t.Context(), client, instanceFirewallUpdateTool,
		[]any{5}, "", body, func() *linodev1.Firewall { return &linodev1.Firewall{} })
	if err == nil {
		t.Error("paged mutation against a closed server succeeded, want an error")
	}
}

// The state read a removal declares, which differs from every other read in two
// ways: it hands back the raw body so a caller can restore an explicit null, and
// it can decode a member of that body rather than the whole of it.

// lkeACLGetTool is the one read whose answer wraps its resource under a key, so
// it is what the member decode is measured against.
const lkeACLGetTool = "linode_lke_acl_get"

// TestCallProtoRouteStateAnswersTheRawBodyBesideTheResource: the restoration a
// declared null needs reads the body the decode read, not the message after it.
func TestCallProtoRouteStateAnswersTheRawBodyBesideTheResource(t *testing.T) {
	t.Parallel()

	const body = `{"id":5,"domain":"` + domainExample + `","status":"active"}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	var domain linodev1.Domain

	raw, err := client.CallProtoRouteState(t.Context(), domainGetTool, []any{5}, "", &domain)
	if err != nil {
		t.Fatalf("CallProtoRouteState: %v", err)
	}

	if string(raw) != body {
		t.Errorf("raw = %s, want the body the decode read (%s)", raw, body)
	}

	if domain.GetDomain() != domainExample {
		t.Errorf("domain = %q, want the resource decoded", domain.GetDomain())
	}
}

// TestCallProtoRouteStateDecodesTheMemberTheResourceSitsUnder: decoding the
// envelope would answer an empty message, which a plan would then hash as the
// state a delete is about to remove.
func TestCallProtoRouteStateDecodesTheMemberTheResourceSitsUnder(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"acl":{"enabled":true,"addresses":{"ipv4":["203.0.113.1/32"]}}}`)
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	var acl linodev1.LKEControlPlaneACL

	raw, err := client.CallProtoRouteState(t.Context(), lkeACLGetTool, []any{12345}, "acl", &acl)
	if err != nil {
		t.Fatalf("CallProtoRouteState: %v", err)
	}

	if !acl.GetEnabled() {
		t.Error("the wrapped resource did not reach the message")
	}

	if strings.Contains(string(raw), `"acl"`) {
		t.Errorf("raw = %s, want the member's own bytes so a null lands beside the resource", raw)
	}
}

// TestCallProtoRouteStateRefusesAnAnswerMissingItsMember: an envelope that lost
// the key the contract names would otherwise preview an empty resource.
func TestCallProtoRouteStateRefusesAnAnswerMissingItsMember(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"something_else":{}}`)
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	var acl linodev1.LKEControlPlaneACL

	_, err := client.CallProtoRouteState(t.Context(), lkeACLGetTool, []any{12345}, "acl", &acl)
	if !errors.Is(err, linode.ErrStateMemberMissing) {
		t.Errorf("err = %v, want ErrStateMemberMissing", err)
	}
}

// TestCallProtoRouteStateReportsTheAPIError: a member read fails the way every
// other read does, so a preview reports the API rather than an empty resource.
func TestCallProtoRouteStateReportsTheAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"errors":[{"reason":"boom"}]}`)
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	var acl linodev1.LKEControlPlaneACL

	if _, err := client.CallProtoRouteState(t.Context(), lkeACLGetTool, []any{12345}, "acl", &acl); err == nil {
		t.Error("a failed member read answered no error")
	}
}

// TestCallProtoRouteStateRefusesAMemberThatIsNotTheResource: a key holding a
// scalar would decode into an empty message and preview as a resource that is
// not there.
func TestCallProtoRouteStateRefusesAMemberThatIsNotTheResource(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"acl":"not-an-object"}`)
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	var acl linodev1.LKEControlPlaneACL

	if _, err := client.CallProtoRouteState(t.Context(), lkeACLGetTool, []any{12345}, "acl", &acl); err == nil {
		t.Error("a member that is not the resource decoded without complaint")
	}
}
