package linode_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	supportTicketAttachmentFile     = "attachment-content"
	supportTicketAttachmentFilename = "diagnostics.txt"
	supportTicketAttachmentError    = "temporary attachment failure"
)

// writeTempAttachment writes the attachment content under the test temp dir and
// returns its absolute path. The client uploads the file as multipart/form-data,
// so it must read a real local file.
func writeTempAttachment(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), supportTicketAttachmentFilename)
	if err := os.WriteFile(path, []byte(supportTicketAttachmentFile), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return path
}

func TestCallRouteMultipartUploadsTheFileContents(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcSupportTickets123Attachments {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets123Attachments)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if contentType := r.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "multipart/form-data") {
			t.Errorf("Content-Type = %q, want multipart/form-data", contentType)
		}

		part, header, formErr := r.FormFile("file")
		if formErr != nil {
			t.Errorf("unexpected error: %v", formErr)

			return
		}

		defer func() {
			if closeErr := part.Close(); closeErr != nil {
				t.Errorf("unexpected error: %v", closeErr)
			}
		}()

		if header.Filename != supportTicketAttachmentFilename {
			t.Errorf("uploaded filename = %v, want %v", header.Filename, supportTicketAttachmentFilename)
		}

		content, readErr := io.ReadAll(part)
		if readErr != nil {
			t.Errorf("unexpected error: %v", readErr)
		}

		if string(content) != supportTicketAttachmentFile {
			t.Errorf("uploaded content = %q, want %q", content, supportTicketAttachmentFile)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{"id": 654}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	// Nothing is read out of the answer: the tool reports the ticket it was
	// addressed by, and the API confirms the upload by status.
	if err := client.CallRouteMultipart(
		t.Context(), attachmentTool, []any{123}, "file", writeTempAttachment(t),
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallRouteMultipartReportsAnAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcSupportTickets123Attachments {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets123Attachments)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.CallRouteMultipart(
		t.Context(), attachmentTool, []any{123}, "file", writeTempAttachment(t),
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestCallRouteMultipartHonorsTheDeclaredRetryPolicy(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcSupportTickets123Attachments {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets123Attachments)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: supportTicketAttachmentError}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.CallRouteMultipart(
		t.Context(), attachmentTool, []any{123}, "file", writeTempAttachment(t),
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}
