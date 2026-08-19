package toolhooks_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	soaExample  = "admin@example.com"
	keyDomain   = "domain"
	keyType     = "type"
	keySOAEmail = "soa_email"
	keyStatus   = "status"
	keyDryRun   = "dry_run"
)

// configFor builds a config pointing at a stub API, which is what a hook that
// fetches state needs and one that does not must never reach.
func configFor(url string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		"default": {
			Label:  "default",
			Linode: config.LinodeConfig{APIURL: url, Token: "test-token"},
		},
	}}
}

// declaredState builds the state a declared fetch hands a walk, through the
// projection itself so a walk test reads the members the same way the live
// fetch produces them.
func declaredState(t *testing.T, body string, msg proto.Message) tools.DeclaredState {
	t.Helper()

	projected, err := tools.ProjectDeclaredState(json.RawMessage(body), msg)
	if err != nil {
		t.Fatalf("ProjectDeclaredState: %v", err)
	}

	state, err := tools.DeclaredStateOf(projected)
	if err != nil {
		t.Fatalf("DeclaredStateOf: %v", err)
	}

	return state
}

// resultText is the one text item a tool result carries.
func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	if result == nil || len(result.Content) != 1 {
		t.Fatalf("result = %v, want one content item", result)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatalf("content = %T, want text", result.Content[0])
	}

	return text.Text
}

// TestLinodeDomainUpdatePreviewDiffsAgainstTheFetchedZone: an update preview
// that described the request alone would say a field "changes" when it already
// holds that value.
func TestLinodeDomainUpdatePreviewDiffsAgainstTheFetchedZone(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(
			`{"id":5,"domain":"example.com","status":"active","soa_email":"old@example.com"}`,
		)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	request := requestWith(map[string]any{
		domainIDArg: 5, keyDryRun: true,
		keyStatus: "disabled", keySOAEmail: soaExample, keyDescription: recordNameNew,
	})

	result, err := toolhooks.LinodeDomainUpdatePreview(
		t.Context(), &request, configFor(server.URL), http.MethodPut, "/domains/5", nil,
	)
	if err != nil {
		t.Fatalf("LinodeDomainUpdatePreview: %v", err)
	}

	var preview struct {
		WouldExecute struct {
			Body any `json:"body"`
		} `json:"would_execute"`
		SideEffects []string `json:"side_effects"`
	}

	if err := json.Unmarshal([]byte(resultText(t, result)), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}

	if preview.WouldExecute.Body != nil {
		t.Errorf("would_execute.body = %v, want none: this family's preview omits it",
			preview.WouldExecute.Body)
	}

	want := []string{
		`Domain status changes from "active" to "disabled".`,
		`SOA email is set to "admin@example.com".`,
		"The domain description is updated.",
	}

	if len(preview.SideEffects) != len(want) {
		t.Fatalf("side_effects = %v, want %v", preview.SideEffects, want)
	}

	for i, sentence := range want {
		if preview.SideEffects[i] != sentence {
			t.Errorf("side_effects[%d] = %q, want %q", i, preview.SideEffects[i], sentence)
		}
	}
}

// TestLinodeDomainUpdatePreviewReportsAFailedFetch: the preview is the caller's
// only look at the zone, so a fetch that failed must not read as a zone with
// nothing to change.
func TestLinodeDomainUpdatePreviewReportsAFailedFetch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		if _, err := w.Write([]byte(`{"errors":[{"reason":"Not found"}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	request := requestWith(map[string]any{domainIDArg: 5, keyDryRun: true})

	result, err := toolhooks.LinodeDomainUpdatePreview(
		t.Context(), &request, configFor(server.URL), http.MethodPut, "/domains/5", nil,
	)
	if err != nil {
		t.Fatalf("LinodeDomainUpdatePreview: %v", err)
	}

	if !result.IsError {
		t.Errorf("result = %s, want the failed fetch reported", resultText(t, result))
	}
}

// TestLinodeDomainUpdatePreviewSaysNothingAboutAnUnchangedZone: a walk that
// listed every field the caller mentioned would report a change where there is
// none, which is the reason it diffs at all.
func TestLinodeDomainUpdatePreviewSaysNothingAboutAnUnchangedZone(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(
			`{"id":5,"domain":"example.com","status":"active","soa_email":"admin@example.com"}`,
		)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	request := requestWith(map[string]any{
		domainIDArg: 5, keyDryRun: true, keyStatus: "active", keySOAEmail: soaExample,
	})

	result, err := toolhooks.LinodeDomainUpdatePreview(
		t.Context(), &request, configFor(server.URL), http.MethodPut, "/domains/5", nil,
	)
	if err != nil {
		t.Fatalf("LinodeDomainUpdatePreview: %v", err)
	}

	var preview struct {
		SideEffects []string `json:"side_effects"`
	}

	if err := json.Unmarshal([]byte(resultText(t, result)), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}

	if len(preview.SideEffects) != 0 {
		t.Errorf("side_effects = %v, want none for a zone already in that state",
			preview.SideEffects)
	}
}

// TestLinodeDomainRecordCreatePreviewReportsABodyItCannotDescribe: a preview
// whose body has no JSON form cannot say what the call would send, and
// answering with a preview missing that field would be worse than saying so.
// The refusal stays unwrappable through the tool name the report adds.
func TestLinodeDomainRecordCreatePreviewReportsABodyItCannotDescribe(t *testing.T) {
	t.Parallel()

	request := requestWith(map[string]any{keyDryRun: true, "domain_id": 5})

	_, err := toolhooks.LinodeDomainRecordCreatePreview(
		t.Context(), &request, configFor(""), http.MethodPost, "/domains/5/records",
		map[string]any{"stream": make(chan int)},
	)
	if _, refused := errors.AsType[*json.UnsupportedTypeError](err); !refused {
		t.Errorf("error = %v, want a body JSON has no form for to be refused", err)
	}
}
