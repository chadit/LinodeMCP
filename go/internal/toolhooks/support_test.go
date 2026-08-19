package toolhooks_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	keySupportTicketID = "ticket_id"
	keyAttachmentFile  = "file"
	attachmentPath     = "/tmp/log.txt"
)

// TestSupportTicketAttachmentCreateExecuteUploadsTheFileContents: the route
// takes multipart/form-data assembled from the file, so what reaches it is the
// bytes rather than the path the JSON body carries.
func TestSupportTicketAttachmentCreateExecuteUploadsTheFileContents(t *testing.T) {
	t.Parallel()

	upload := filepath.Join(t.TempDir(), "log.txt")
	if err := os.WriteFile(upload, []byte("the log line"), 0o600); err != nil {
		t.Fatalf("write upload: %v", err)
	}

	var sent struct {
		contentType string
		body        string
		path        string
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
		}

		sent.path, sent.contentType, sent.body = r.URL.Path, r.Header.Get("Content-Type"), string(payload)

		if _, err := w.Write([]byte(`{"id":1,"filename":"log.txt"}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	request := requestWith(map[string]any{keySupportTicketID: 123, keyAttachmentFile: upload})

	if err := toolhooks.LinodeSupportTicketAttachmentCreateExecute(
		t.Context(), clientFor(t, server.URL), &request, []any{123}, nil,
	); err != nil {
		t.Fatalf("LinodeSupportTicketAttachmentCreateExecute: %v", err)
	}

	if sent.path != "/support/tickets/123/attachments" {
		t.Errorf("path = %q, want the attachment route", sent.path)
	}

	if !strings.HasPrefix(sent.contentType, "multipart/form-data") {
		t.Errorf("Content-Type = %q, want multipart/form-data", sent.contentType)
	}

	if !strings.Contains(sent.body, "the log line") || !strings.Contains(sent.body, `filename="log.txt"`) {
		t.Errorf("body = %q, want the file's contents under its base name", sent.body)
	}
}

// TestSupportTicketAttachmentCreateExecuteReportsAFailedUpload: the file is read
// at call time, so a path that resolves to nothing fails here rather than in a
// check ahead of it.
func TestSupportTicketAttachmentCreateExecuteReportsAFailedUpload(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("a request reached the server, want none: the file could not be read")
	}))
	defer server.Close()

	request := requestWith(map[string]any{
		keySupportTicketID: 123,
		keyAttachmentFile:  filepath.Join(t.TempDir(), "absent.txt"),
	})

	err := toolhooks.LinodeSupportTicketAttachmentCreateExecute(
		t.Context(), clientFor(t, server.URL), &request, []any{123}, nil,
	)
	if err == nil {
		t.Fatal("LinodeSupportTicketAttachmentCreateExecute = nil, want the unreadable file reported")
	}
}

// TestSupportTicketAttachmentCreateExecuteRefusesAPathValueItCannotRead: the
// ticket id is resolved by the emitter, so a value of another type is a contract
// defect and is named as one rather than uploading to ticket zero.
func TestSupportTicketAttachmentCreateExecuteRefusesAPathValueItCannotRead(t *testing.T) {
	t.Parallel()

	request := requestWith(map[string]any{keySupportTicketID: 123, keyAttachmentFile: attachmentPath})

	err := toolhooks.LinodeSupportTicketAttachmentCreateExecute(
		t.Context(), clientFor(t, "http://127.0.0.1:1"), &request, []any{"123"}, nil,
	)
	if !errors.Is(err, toolhooks.ErrTicketIDNotAnInt) {
		t.Errorf("err = %v, want %v", err, toolhooks.ErrTicketIDNotAnInt)
	}
}
