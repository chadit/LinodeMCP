package toolhooks_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

// The OAuth client thumbnail is the one acknowledge-tier step no declaration
// reaches: its image travels as raw PNG bytes rather than as the JSON body the
// emitter derives, so both directions of that transport are checked here.

const (
	ciOAuthClientID    = "abc123"
	keyThumbnailBase64 = "thumbnail_png_base64"
	// helloBase64 decodes to "hello", which is what the wire test looks for.
	helloBase64 = "aGVsbG8="
)

// TestLinodeAccountOauthClientThumbnailUpdateExecuteSendsTheRawImage: the route
// takes the decoded PNG under image/png, so what reaches it is the bytes rather
// than the base64 text the JSON body would have carried.
func TestLinodeAccountOauthClientThumbnailUpdateExecuteSendsTheRawImage(t *testing.T) {
	t.Parallel()

	var sent struct {
		contentType string
		body        string
		path        string
		method      string
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
		}

		sent.method, sent.path = r.Method, r.URL.Path
		sent.contentType, sent.body = r.Header.Get("Content-Type"), string(payload)

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	request := requestWith(map[string]any{
		argClientID: ciOAuthClientID, keyThumbnailBase64: helloBase64,
	})

	if err := toolhooks.LinodeAccountOauthClientThumbnailUpdateExecute(
		t.Context(), clientFor(t, server.URL), &request,
		[]any{ciOAuthClientID}, map[string]any{keyThumbnailBase64: helloBase64},
	); err != nil {
		t.Fatalf("LinodeAccountOauthClientThumbnailUpdateExecute: %v", err)
	}

	if sent.method != http.MethodPut || sent.path != "/account/oauth-clients/abc123/thumbnail" {
		t.Errorf("%s %s, want PUT the thumbnail route", sent.method, sent.path)
	}

	if sent.contentType != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", sent.contentType)
	}

	if sent.body != "hello" {
		t.Errorf("body = %q, want the decoded image rather than the base64 text", sent.body)
	}
}

// TestLinodeAccountOauthClientThumbnailUpdateExecuteRefusesANonTextID: a path
// value of another type is a contract and generator disagreement, so the hook
// says so by identity rather than making a call it cannot address.
func TestLinodeAccountOauthClientThumbnailUpdateExecuteRefusesANonTextID(t *testing.T) {
	t.Parallel()

	request := requestWith(map[string]any{
		argClientID: ciOAuthClientID, keyThumbnailBase64: helloBase64,
	})

	err := toolhooks.LinodeAccountOauthClientThumbnailUpdateExecute(
		t.Context(), clientFor(t, silentServer(t).URL), &request, []any{7}, nil,
	)
	if !errors.Is(err, toolhooks.ErrOAuthClientIDNotText) {
		t.Errorf("err = %v, want ErrOAuthClientIDNotText", err)
	}
}

// TestLinodeAccountOauthClientThumbnailUpdateExecuteReportsTheAPIFailure: the
// hook adds no prose of its own, so the caller reads the tool's declared
// sentence wrapped around whatever the API said.
func TestLinodeAccountOauthClientThumbnailUpdateExecuteReportsTheAPIFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)

	request := requestWith(map[string]any{
		argClientID: ciOAuthClientID, keyThumbnailBase64: helloBase64,
	})

	if err := toolhooks.LinodeAccountOauthClientThumbnailUpdateExecute(
		t.Context(), clientFor(t, server.URL), &request,
		[]any{ciOAuthClientID}, nil,
	); err == nil {
		t.Error("err = nil, want the API failure the handler formats its sentence around")
	}
}

// TestLinodeAccountOauthClientThumbnailGetExecuteEncodesTheRawImage: the route
// answers with image bytes, so what the hook hands back is the base64 text the
// response member carries rather than anything a decode could have placed.
func TestLinodeAccountOauthClientThumbnailGetExecuteEncodesTheRawImage(t *testing.T) {
	t.Parallel()

	var asked struct {
		method string
		path   string
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.method, asked.path = r.Method, r.URL.Path

		w.Header().Set("Content-Type", "image/png")

		if _, err := w.Write([]byte("hello")); err != nil {
			t.Errorf("write image: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	request := requestWith(map[string]any{argClientID: ciOAuthClientID})

	assembled, err := toolhooks.LinodeAccountOauthClientThumbnailGetExecute(
		t.Context(), clientFor(t, server.URL), &request, []any{ciOAuthClientID},
	)
	if err != nil {
		t.Fatalf("LinodeAccountOauthClientThumbnailGetExecute: %v", err)
	}

	if asked.method != http.MethodGet || asked.path != "/account/oauth-clients/abc123/thumbnail" {
		t.Errorf("%s %s, want GET the thumbnail route", asked.method, asked.path)
	}

	if got := assembled.GetThumbnailPngBase64(); got != helloBase64 {
		t.Errorf("encoded = %q, want %q", got, helloBase64)
	}

	// The handler fills client_id from the call, so the hook leaving it blank is
	// the contract rather than a gap.
	if got := assembled.GetClientId(); got != "" {
		t.Errorf("client_id = %q, want the handler to fill it", got)
	}
}

// TestLinodeAccountOauthClientThumbnailGetExecuteRefusesANonTextID: a path value
// of another type is a contract and generator disagreement, so the hook says so
// by identity rather than making a call it cannot address.
func TestLinodeAccountOauthClientThumbnailGetExecuteRefusesANonTextID(t *testing.T) {
	t.Parallel()

	request := requestWith(map[string]any{argClientID: ciOAuthClientID})

	_, err := toolhooks.LinodeAccountOauthClientThumbnailGetExecute(
		t.Context(), clientFor(t, silentServer(t).URL), &request, []any{7},
	)
	if !errors.Is(err, toolhooks.ErrOAuthClientIDNotText) {
		t.Errorf("err = %v, want ErrOAuthClientIDNotText", err)
	}
}

// TestLinodeAccountOauthClientThumbnailGetExecuteReportsTheAPIFailure: the hook
// adds no prose of its own, so the caller reads the tool's declared sentence
// wrapped around whatever the API said.
func TestLinodeAccountOauthClientThumbnailGetExecuteReportsTheAPIFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	request := requestWith(map[string]any{argClientID: ciOAuthClientID})

	assembled, err := toolhooks.LinodeAccountOauthClientThumbnailGetExecute(
		t.Context(), clientFor(t, server.URL), &request, []any{ciOAuthClientID},
	)
	if err == nil {
		t.Error("err = nil, want the API failure the handler formats its sentence around")
	}

	if assembled != nil {
		t.Errorf("assembled = %v, want no answer beside a failure", assembled)
	}
}
