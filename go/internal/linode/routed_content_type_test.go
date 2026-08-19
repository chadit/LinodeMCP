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

// thumbnailPNG is a recognizable marker, not a real PNG: the route ships the
// bytes the caller already framed.
func thumbnailPNG() []byte {
	return []byte{0x89, 'P', 'N', 'G'}
}

// Method and path come from the route contract, but the body and its
// image/png Content-Type come from the caller and never touch the JSON
// marshaller.
func TestUpdateOAuthClientThumbnailSendsThePNGToTheDeclaredRoute(t *testing.T) {
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

	if err := client.UpdateOAuthClientThumbnail(t.Context(), "client-1", thumbnailPNG()); err != nil {
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
func TestUpdateOAuthClientThumbnailEscapesTheClientID(t *testing.T) {
	t.Parallel()

	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, routedTransportToken, nil, linode.WithMaxRetries(0))

	if err := client.UpdateOAuthClientThumbnail(t.Context(), "client/1?x", thumbnailPNG()); err != nil {
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
func TestUpdateOAuthClientThumbnailRefusesAnEmptyClientID(t *testing.T) {
	t.Parallel()

	var reached bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, routedTransportToken, nil, linode.WithMaxRetries(0))

	err := client.UpdateOAuthClientThumbnail(t.Context(), "", thumbnailPNG())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	argErr, ok := errors.AsType[*linode.ArgumentError](err)
	if !ok {
		t.Fatalf("err = %T (%v), want *linode.ArgumentError", err, err)
	}

	if argErr.Operation != "UpdateOAuthClientThumbnail" {
		t.Errorf("argErr.Operation = %v, want UpdateOAuthClientThumbnail", argErr.Operation)
	}

	if reached {
		t.Error("a request reached the server, want the contract to refuse it first")
	}
}

// The other content-type route: a multipart/form-data body assembled from a
// local file, with the boundary carried on the caller-supplied Content-Type.
func TestCreateSupportTicketAttachmentSendsTheFileToTheDeclaredRoute(t *testing.T) {
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

	attachment, err := client.CreateSupportTicketAttachment(t.Context(), 4242,
		&linode.CreateSupportTicketAttachmentRequest{File: path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if attachment == nil {
		t.Fatal("attachment = nil, want the decoded response")
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
func TestCreateSupportTicketAttachmentReportsAnUnreadableFile(t *testing.T) {
	t.Parallel()

	var reached bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, routedTransportToken, nil, linode.WithMaxRetries(0))

	missing := filepath.Join(t.TempDir(), "absent.txt")

	_, err := client.CreateSupportTicketAttachment(t.Context(), 4242,
		&linode.CreateSupportTicketAttachmentRequest{File: missing})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("err = %T (%v), want *linode.NetworkError", err, err)
	}

	if netErr.Operation != "CreateSupportTicketAttachment" {
		t.Errorf("netErr.Operation = %v, want CreateSupportTicketAttachment", netErr.Operation)
	}

	if reached {
		t.Error("a request reached the server, want the unreadable file to stop it first")
	}
}

// The shared transport sweep builds each case from an empty request, and an
// empty File path fails the read above the route call, so its attachment row
// passes off the wrong branch and never exercises this wrap. A real file on
// disk carries the call far enough to fail at the transport instead.
func TestCreateSupportTicketAttachmentTransportFailure(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("attachment body"), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err := newUnreachableRoutedClient(t).CreateSupportTicketAttachment(t.Context(), 4242,
		&linode.CreateSupportTicketAttachmentRequest{File: path})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("err = %T (%v), want *linode.NetworkError", err, err)
	}

	if netErr.Operation != "CreateSupportTicketAttachment" {
		t.Errorf("netErr.Operation = %v, want CreateSupportTicketAttachment", netErr.Operation)
	}

	if netErr.Unwrap() == nil {
		t.Error("netErr.Unwrap() = nil, want the transport error")
	}
}
