package linode_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// The tools whose routes these primitives address. Naming each once keeps the
// same string from being spelled in every case, and it is also the operation
// label a routed primitive reports its failures under.
const (
	thumbnailUpdateTool = "linode_account_oauth_client_thumbnail_update"
	thumbnailGetTool    = "linode_account_oauth_client_thumbnail_get"
	attachmentTool      = "linode_support_ticket_attachment_create"

	// The ids the transport cases address, spelled once so the same literal is
	// not repeated across the sweep tables that share this package.
	transportClientAlpha = "alpha"
	transportClientOne   = "client-1"
)

// thumbnailPNG is a recognizable marker, not a real PNG: the route ships the
// bytes the caller already framed.
func thumbnailPNG() []byte {
	return []byte{0x89, 'P', 'N', 'G'}
}

// Method and path come from the route contract, but the body and its
// image/png Content-Type come from the caller and never touch the JSON
// marshaller.
func TestCallRouteRawBodySendsThePNGToTheDeclaredRoute(t *testing.T) {
	t.Parallel()

	var (
		gotMethod      string
		gotPath        string
		gotContentType string
		gotBody        []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, routedTransportToken, nil, linode.WithMaxRetries(0))

	if err := client.CallRouteRawBody(
		t.Context(), thumbnailUpdateTool, []any{transportClientOne}, "image/png", thumbnailPNG(),
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Errorf("method = %v, want %v", gotMethod, http.MethodPut)
	}

	const wantPath = "/account/oauth-clients/client-1/thumbnail"
	if gotPath != wantPath {
		t.Errorf("path = %v, want %v", gotPath, wantPath)
	}

	if gotContentType != "image/png" {
		t.Errorf("Content-Type = %v, want image/png", gotContentType)
	}

	if !bytes.Equal(gotBody, thumbnailPNG()) {
		t.Errorf("body = %v, want the PNG bytes unchanged", gotBody)
	}
}

// The hand-built endpoint this replaced trusted the caller's client ID; the
// routed twin escapes it like every other routed primitive.
func TestCallRouteRawBodyEscapesTheClientID(t *testing.T) {
	t.Parallel()

	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, routedTransportToken, nil, linode.WithMaxRetries(0))

	if err := client.CallRouteRawBody(
		t.Context(), thumbnailUpdateTool, []any{"client/1?x"}, "image/png", thumbnailPNG(),
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const wantPath = "/account/oauth-clients/client%2F1%3Fx/thumbnail"
	if gotPath != wantPath {
		t.Errorf("path = %v, want %v", gotPath, wantPath)
	}
}

// An empty client ID would address the collection rather than one client, so
// it fails before anything is sent, as an *ArgumentError so the retry layer
// cannot replay a request that never left.
func TestCallRouteRawBodyRefusesAnEmptyClientID(t *testing.T) {
	t.Parallel()

	var reached bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, routedTransportToken, nil, linode.WithMaxRetries(0))

	err := client.CallRouteRawBody(
		t.Context(), thumbnailUpdateTool, []any{""}, "image/png", thumbnailPNG(),
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	argErr, ok := errors.AsType[*linode.ArgumentError](err)
	if !ok {
		t.Fatalf("err = %T (%v), want *linode.ArgumentError", err, err)
	}

	if argErr.Operation != thumbnailUpdateTool {
		t.Errorf("argErr.Operation = %v, want %v", argErr.Operation, thumbnailUpdateTool)
	}

	if reached {
		t.Error("a request reached the server, want the contract to refuse it first")
	}
}

// The downward twin: the answer is the resource itself, negotiated under the
// declared type and handed back as it arrived.
func TestCallRouteRawBodyReadAnswersWithTheBytes(t *testing.T) {
	t.Parallel()

	var (
		gotMethod string
		gotPath   string
		gotAccept string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotAccept = r.Header.Get("Accept")

		if _, err := w.Write(thumbnailPNG()); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, routedTransportToken, nil, linode.WithMaxRetries(0))

	got, err := client.CallRouteRawBodyRead(
		t.Context(), thumbnailGetTool, []any{transportClientOne}, "image/png",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("method = %v, want %v", gotMethod, http.MethodGet)
	}

	const wantPath = "/account/oauth-clients/client-1/thumbnail"
	if gotPath != wantPath {
		t.Errorf("path = %v, want %v", gotPath, wantPath)
	}

	if gotAccept != "image/png" {
		t.Errorf("Accept = %v, want image/png", gotAccept)
	}

	if !bytes.Equal(got, thumbnailPNG()) {
		t.Errorf("body = %v, want the PNG bytes unchanged", got)
	}
}

// A failing status is read off the same bytes the answer would have been, so
// the API's own reason survives instead of being reported as empty content.
func TestCallRouteRawBodyReadReportsAnAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		if _, err := w.Write([]byte(`{"errors":[{"reason":"Not found"}]}`)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, routedTransportToken, nil, linode.WithMaxRetries(0))

	_, err := client.CallRouteRawBodyRead(
		t.Context(), thumbnailGetTool, []any{transportClientOne}, "image/png",
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("err = %T (%v), want *linode.APIError", err, err)
	}

	if apiErr.Message != "Not found" {
		t.Errorf("apiErr.Message = %v, want Not found", apiErr.Message)
	}
}

// The other content-type route: a multipart/form-data body assembled from a
// local file, with the boundary carried on the caller-supplied Content-Type.
func TestCallRouteMultipartSendsTheFileToTheDeclaredRoute(t *testing.T) {
	t.Parallel()

	var (
		gotMethod      string
		gotPath        string
		gotContentType string
		gotBody        []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)

		w.WriteHeader(http.StatusOK)

		if _, err := w.Write([]byte(`{"id":7,"filename":"note.txt"}`)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("attachment body"), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	client := linode.NewClient(srv.URL, routedTransportToken, nil, linode.WithMaxRetries(0))

	if err := client.CallRouteMultipart(
		t.Context(), attachmentTool, []any{4242}, "file", path,
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %v, want %v", gotMethod, http.MethodPost)
	}

	const wantPath = "/support/tickets/4242/attachments"
	if gotPath != wantPath {
		t.Errorf("path = %v, want %v", gotPath, wantPath)
	}

	if !strings.HasPrefix(gotContentType, "multipart/form-data; boundary=") {
		t.Errorf("Content-Type = %v, want a multipart type carrying the boundary", gotContentType)
	}

	if !strings.Contains(string(gotBody), "attachment body") {
		t.Error("body does not carry the file content")
	}

	if !strings.Contains(string(gotBody), `filename="note.txt"`) {
		t.Error("body does not carry the base filename")
	}
}

// The file read runs before any route is resolved, so nothing reaches the
// wire, but the caller still needs the operation name to know which call
// failed.
func TestCallRouteMultipartReportsAnUnreadableFile(t *testing.T) {
	t.Parallel()

	var reached bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, routedTransportToken, nil, linode.WithMaxRetries(0))

	missing := filepath.Join(t.TempDir(), "absent.txt")

	err := client.CallRouteMultipart(t.Context(), attachmentTool, []any{4242}, "file", missing)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("err = %T (%v), want *linode.NetworkError", err, err)
	}

	if netErr.Operation != attachmentTool {
		t.Errorf("netErr.Operation = %v, want %v", netErr.Operation, attachmentTool)
	}

	if reached {
		t.Error("a request reached the server, want the unreadable file to stop it first")
	}
}

// A real file on disk carries the call far enough to fail at the transport
// rather than at the read above it, which is the branch that wraps.
func TestCallRouteMultipartTransportFailure(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("attachment body"), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err := newUnreachableRoutedClient(t).CallRouteMultipart(
		t.Context(), attachmentTool, []any{4242}, "file", path,
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("err = %T (%v), want *linode.NetworkError", err, err)
	}

	if netErr.Operation != attachmentTool {
		t.Errorf("netErr.Operation = %v, want %v", netErr.Operation, attachmentTool)
	}

	if netErr.Unwrap() == nil {
		t.Error("netErr.Unwrap() = nil, want the transport error")
	}
}

// errFrameWrite is what the hostile writer below answers with, so each case
// can pin the stage that failed by the sentence wrapped around it.
var errFrameWrite = errors.New("writer refused the bytes")

// failingWriter refuses the one write whose bytes carry failOn and takes every
// other. Matching on content rather than counting writes keeps each case
// pinned to the framing stage it names, however the encoder splits its writes.
type failingWriter struct {
	failOn string
}

func (w *failingWriter) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte(w.failOn)) {
		return 0, errFrameWrite
	}

	return len(p), nil
}

// Each framing stage reports which one failed, so an operator reading the
// wrapped sentence knows whether the form header, the file's bytes, or the
// closing boundary is what the writer refused. Framing into memory cannot fail,
// which is why the destination is a parameter.
func TestFrameMultipartNamesTheStageThatFailed(t *testing.T) {
	t.Parallel()

	const content = "attachment body"

	tests := []struct {
		name   string
		failOn string
		want   string
	}{
		{
			name:   "the form header",
			failOn: "filename=",
			want:   "failed to build attachment form field: ",
		},
		{
			name:   "the file's own bytes",
			failOn: content,
			want:   "failed to write attachment content: ",
		},
		{
			// The closing boundary alone ends in a bare "--\r\n"; the opening
			// one is followed by the header block.
			name:   "the closing boundary",
			failOn: "--\r\n",
			want:   "failed to finalize attachment body: ",
		},
	}

	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := linode.FrameMultipart(&failingWriter{failOn: testCase.failOn}, "file", path)
			if !errors.Is(err, errFrameWrite) {
				t.Fatalf("err = %v, want the writer's own refusal", err)
			}

			if err.Error() != testCase.want+errFrameWrite.Error() {
				t.Errorf("err = %q, want %q", err, testCase.want+errFrameWrite.Error())
			}
		})
	}
}

// A framing that succeeds answers the content type carrying the boundary the
// body was written under, which is what the request header has to name.
func TestFrameMultipartAnswersTheBoundaryItWrote(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("attachment body"), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	framed := &bytes.Buffer{}

	contentType, err := linode.FrameMultipart(framed, "file", path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	boundary := strings.TrimPrefix(contentType, "multipart/form-data; boundary=")
	if boundary == contentType {
		t.Fatalf("content type = %q, want a multipart type carrying the boundary", contentType)
	}

	if !strings.Contains(framed.String(), "--"+boundary) {
		t.Errorf("framed body does not open on the boundary it reported")
	}
}

// An answer shorter than its declared length is a read that fails partway, and
// the caller needs the operation name to know which call went wrong.
func TestCallRouteRawBodyReadReportsATruncatedAnswer(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Hijacked so the headers land intact and only the body runs short: a
		// handler that aborts instead fails the request itself, which is a
		// different branch reporting a similar error.
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Errorf("response writer = %T, want an http.Hijacker", w)

			return
		}

		conn, _, hijackErr := hijacker.Hijack()
		if hijackErr != nil {
			t.Errorf("unexpected error: %v", hijackErr)

			return
		}

		defer func() {
			if closeErr := conn.Close(); closeErr != nil {
				t.Errorf("unexpected error: %v", closeErr)
			}
		}()

		// Promises 64 bytes and sends 5, so the read stops short of the length
		// the answer declared.
		_, _ = conn.Write([]byte(
			"HTTP/1.1 200 OK\r\nContent-Length: 64\r\nContent-Type: image/png\r\n\r\nshort",
		))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, routedTransportToken, nil, linode.WithMaxRetries(0))

	_, err := client.CallRouteRawBodyRead(
		t.Context(), thumbnailGetTool, []any{transportClientOne}, "image/png",
	)

	// Pinned to the read rather than to any network failure, so the case cannot
	// pass off the request-failed branch instead.
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("err = %v, want the body read to stop short", err)
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("err = %T (%v), want *linode.NetworkError", err, err)
	}

	if netErr.Operation != thumbnailGetTool {
		t.Errorf("netErr.Operation = %v, want %v", netErr.Operation, thumbnailGetTool)
	}

	if netErr.Unwrap() == nil {
		t.Error("netErr.Unwrap() = nil, want the read failure")
	}
}
