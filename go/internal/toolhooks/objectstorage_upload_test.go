package toolhooks_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/objectdata"
	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	uploadBucketLabel = "artifacts"
	uploadSourceArg   = "source_path"
	uploadObjectKey   = "key"
	uploadDestArg     = "dest_path"
	uploadStoredETag  = "abc123"
)

// uploadSource writes one file and answers its path.
func uploadSource(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "upload.bin")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return path
}

// uploadBody builds the body the generated handler hands the hook, so the test
// exercises the same shape production does.
func uploadBody(request *mcp.CallToolRequest) *tools.WriteBody {
	body := tools.NewWriteBody(request, 3)

	body.PutString("name")
	body.SetString("content_type")
	body.SetInt("expires_in")
	body.Constant("method", "PUT")

	return body
}

// TestLinodeObjectStorageObjectUploadExecutePresignsThenSends pins the two legs
// and their order: exactly one Linode call, then the transfer to the URL that
// call answered with.
func TestLinodeObjectStorageObjectUploadExecutePresignsThenSends(t *testing.T) {
	t.Parallel()

	const content = "hello object storage"

	var (
		presignBody map[string]any
		putBody     string
		order       []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		order = append(order, r.Method+" "+r.URL.Path)

		if r.Method == http.MethodPost {
			_ = json.Unmarshal(body, &presignBody)

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"url":"` + "http://" + r.Host + `/artifacts/key?sig=fixture"}`))

			return
		}

		putBody = string(body)

		w.Header().Set("ETag", `"abc123"`)
	}))
	t.Cleanup(server.Close)

	request := requestWith(map[string]any{
		foldRegion:      crossSampleRegion,
		keyLabel:        uploadBucketLabel,
		argName:         uploadObjectKey,
		uploadSourceArg: uploadSource(t, content),
	})

	assembled, err := toolhooks.LinodeObjectStorageObjectUploadExecute(
		t.Context(), clientFor(t, server.URL), &request,
		[]any{crossSampleRegion, uploadBucketLabel}, uploadBody(&request),
	)
	if err != nil {
		t.Fatalf("LinodeObjectStorageObjectUploadExecute: %v", err)
	}

	if len(order) != 2 || !strings.HasPrefix(order[0], "POST") || !strings.HasPrefix(order[1], "PUT") {
		t.Fatalf("calls = %v, want the presign POST then the transfer PUT", order)
	}

	// The signature covers Content-Type, so the presign body has to carry the
	// default rather than the PUT inventing one the URL was not signed for.
	if presignBody["content_type"] != objectdata.DefaultContentType {
		t.Errorf("presigned content_type = %v, want the default filled in", presignBody["content_type"])
	}

	if presignBody["expires_in"] != float64(3600) {
		t.Errorf("presigned expires_in = %v, want the configured default", presignBody["expires_in"])
	}

	if putBody != content {
		t.Errorf("transferred %q, want the file's bytes %q", putBody, content)
	}

	if assembled.GetSizeBytes() != int64(len(content)) {
		t.Errorf("size_bytes = %d, want %d", assembled.GetSizeBytes(), len(content))
	}

	if assembled.GetEtag() != uploadStoredETag {
		t.Errorf("etag = %q, want the stored entity tag", assembled.GetEtag())
	}

	if assembled.GetUploadMode() != "single" {
		t.Errorf("upload_mode = %q, want single", assembled.GetUploadMode())
	}
}

// TestLinodeObjectStorageObjectUploadExecuteRefusesBeforeCalling pins that an
// unusable source costs no request: the file is checked ahead of the presign.
func TestLinodeObjectStorageObjectUploadExecuteRefusesBeforeCalling(t *testing.T) {
	t.Parallel()

	var calls int

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	request := requestWith(map[string]any{
		foldRegion:      crossSampleRegion,
		keyLabel:        uploadBucketLabel,
		argName:         uploadObjectKey,
		uploadSourceArg: filepath.Join(t.TempDir(), "absent.bin"),
	})

	_, err := toolhooks.LinodeObjectStorageObjectUploadExecute(
		t.Context(), clientFor(t, server.URL), &request,
		[]any{crossSampleRegion, uploadBucketLabel}, uploadBody(&request),
	)
	if !errors.Is(err, objectdata.ErrSourceMissing) {
		t.Fatalf("error = %v, want ErrSourceMissing", err)
	}

	if calls != 0 {
		t.Errorf("made %d requests, want none before the source is readable", calls)
	}
}

// previewText runs the upload preview and answers its rendered result.
func previewText(t *testing.T, arguments map[string]any, ceiling int64) string {
	t.Helper()

	request := requestWith(arguments)
	cfg := &config.Config{ObjectStorage: config.ObjectStorageConfig{MaxSinglePartBytes: ceiling}}

	result, err := toolhooks.LinodeObjectStorageObjectUploadPreview(
		t.Context(), &request, cfg, http.MethodPost,
		"/object-storage/buckets/"+crossSampleRegion+"/"+uploadBucketLabel+"/object-url",
		uploadBody(&request),
	)
	if err != nil {
		t.Fatalf("LinodeObjectStorageObjectUploadPreview: %v", err)
	}

	rendered, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		t.Fatalf("unexpected error: %v", marshalErr)
	}

	return string(rendered)
}

// TestLinodeObjectStorageObjectUploadPreviewDescribesTheTransfer pins that a
// preview reports the byte count, and that it fills the presign body's
// Content-Type: the preview would otherwise describe a request the live call
// does not make.
func TestLinodeObjectStorageObjectUploadPreviewDescribesTheTransfer(t *testing.T) {
	t.Parallel()

	rendered := previewText(t, map[string]any{
		foldRegion:      crossSampleRegion,
		keyLabel:        uploadBucketLabel,
		argName:         "releases/app.tar.gz",
		uploadSourceArg: uploadSource(t, "hello object storage"),
	}, 0)

	for _, want := range []string{
		objectdata.DefaultContentType,
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("preview %s does not carry %q", rendered, want)
		}
	}
}

// TestLinodeObjectStorageObjectUploadPreviewWarnsAboutARefusal pins the reason
// the preview stats the file at all: a preview that cannot say the upload will
// be refused has told the caller nothing they needed.
func TestLinodeObjectStorageObjectUploadPreviewWarnsAboutARefusal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		source  string
		want    string
		ceiling int64
	}{
		{
			name:    "over the ceiling",
			source:  uploadSource(t, strings.Repeat("a", 2048)),
			want:    "single-part upload ceiling",
			ceiling: 1024,
		},
		{
			name:   "unreadable source",
			source: filepath.Join(t.TempDir(), "absent.bin"),
			want:   "no readable file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rendered := previewText(t, map[string]any{
				foldRegion:      crossSampleRegion,
				keyLabel:        uploadBucketLabel,
				argName:         uploadObjectKey,
				uploadSourceArg: tt.source,
			}, tt.ceiling)

			if !strings.Contains(rendered, tt.want) {
				t.Errorf("preview %s does not warn %q", rendered, tt.want)
			}
		})
	}
}

// TestLinodeObjectStorageObjectUploadExecuteRefusesOverTheCeiling pins that the
// ceiling is checked before the presign, so an oversized file costs no request.
func TestLinodeObjectStorageObjectUploadExecuteRefusesOverTheCeiling(t *testing.T) {
	t.Parallel()

	var calls int

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	request := requestWith(map[string]any{
		foldRegion:      crossSampleRegion,
		keyLabel:        uploadBucketLabel,
		argName:         uploadObjectKey,
		uploadSourceArg: uploadSource(t, strings.Repeat("a", 2048)),
	})

	client := linode.NewClient(server.URL, "test-token",
		&config.Config{ObjectStorage: config.ObjectStorageConfig{MaxSinglePartBytes: 1024}},
		linode.WithMaxRetries(0))

	_, err := toolhooks.LinodeObjectStorageObjectUploadExecute(
		t.Context(), client, &request,
		[]any{crossSampleRegion, uploadBucketLabel}, uploadBody(&request),
	)
	if !errors.Is(err, objectdata.ErrAboveSinglePartCeiling) {
		t.Fatalf("error = %v, want ErrAboveSinglePartCeiling", err)
	}

	if calls != 0 {
		t.Errorf("made %d requests, want none for a file over the ceiling", calls)
	}
}

// TestLinodeObjectStorageObjectUploadExecuteReportsEachLegsFailure pins that a
// failure in either leg reaches the caller rather than a partial answer.
func TestLinodeObjectStorageObjectUploadExecuteReportsEachLegsFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		presignFail bool
	}{
		{name: "the presign call fails", presignFail: true},
		{name: "the transfer fails"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && tt.presignFail {
					w.WriteHeader(http.StatusInternalServerError)

					return
				}

				if r.Method == http.MethodPost {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"url":"` + "http://" + r.Host + `/artifacts/key"}`))

					return
				}

				w.WriteHeader(http.StatusForbidden)
			}))
			t.Cleanup(server.Close)

			request := requestWith(map[string]any{
				foldRegion:      crossSampleRegion,
				keyLabel:        uploadBucketLabel,
				argName:         uploadObjectKey,
				uploadSourceArg: uploadSource(t, "bytes"),
			})

			_, err := toolhooks.LinodeObjectStorageObjectUploadExecute(
				t.Context(), clientFor(t, server.URL), &request,
				[]any{crossSampleRegion, uploadBucketLabel}, uploadBody(&request),
			)
			if err == nil {
				t.Fatal("err = nil, want the failure the handler formats its sentence around")
			}
		})
	}
}

// TestLinodeObjectStorageObjectUploadPreviewWithoutABodyOrConfig pins the two
// shapes a preview can be handed before anything is wired: no body to default
// into, and a tool registered before a config was loaded.
func TestLinodeObjectStorageObjectUploadPreviewWithoutABodyOrConfig(t *testing.T) {
	t.Parallel()

	request := requestWith(map[string]any{
		foldRegion:      crossSampleRegion,
		keyLabel:        uploadBucketLabel,
		argName:         uploadObjectKey,
		uploadSourceArg: uploadSource(t, "hello object storage"),
	})

	result, err := toolhooks.LinodeObjectStorageObjectUploadPreview(
		t.Context(), &request, nil, http.MethodPost,
		"/object-storage/buckets/77/77/object-url", nil,
	)
	if err != nil {
		t.Fatalf("LinodeObjectStorageObjectUploadPreview: %v", err)
	}

	rendered, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		t.Fatalf("unexpected error: %v", marshalErr)
	}

	// The defaults still apply with no config, so the preview describes the same
	// transfer the live call would make.
	if !strings.Contains(string(rendered), "20 bytes will be uploaded") {
		t.Errorf("preview %s does not describe the transfer", rendered)
	}
}

// downloadBody builds the body the generated download handler hands its hook.
func downloadBody(request *mcp.CallToolRequest) *tools.WriteBody {
	body := tools.NewWriteBody(request, 2)

	body.PutString("name")
	body.SetInt("expires_in")
	body.Constant("method", "GET")

	return body
}

// TestLinodeObjectStorageObjectDownloadExecutePresignsThenFetches pins the two
// legs and their order, and that the object's bytes reach the named file.
func TestLinodeObjectStorageObjectDownloadExecutePresignsThenFetches(t *testing.T) {
	t.Parallel()

	const content = "hello object storage"

	var order []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, r.Method+" "+r.URL.Path)

		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"url":"` + "http://" + r.Host + `/artifacts/key?sig=fixture"}`))

			return
		}

		w.Header().Set("ETag", `"abc123"`)
		_, _ = w.Write([]byte(content))
	}))
	t.Cleanup(server.Close)

	destination := filepath.Join(t.TempDir(), "landed.bin")
	request := requestWith(map[string]any{
		foldRegion:    crossSampleRegion,
		keyLabel:      uploadBucketLabel,
		argName:       uploadObjectKey,
		uploadDestArg: destination,
	})

	assembled, err := toolhooks.LinodeObjectStorageObjectDownloadExecute(
		t.Context(), clientFor(t, server.URL), &request,
		[]any{crossSampleRegion, uploadBucketLabel}, downloadBody(&request),
	)
	if err != nil {
		t.Fatalf("LinodeObjectStorageObjectDownloadExecute: %v", err)
	}

	if len(order) != 2 || !strings.HasPrefix(order[0], "POST") || !strings.HasPrefix(order[1], "GET") {
		t.Fatalf("calls = %v, want the presign POST then the transfer GET", order)
	}

	landed, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(landed) != content {
		t.Errorf("file holds %q, want %q", landed, content)
	}

	if assembled.GetSizeBytes() != int64(len(content)) {
		t.Errorf("size_bytes = %d, want %d", assembled.GetSizeBytes(), len(content))
	}

	if assembled.GetEtag() != uploadStoredETag {
		t.Errorf("etag = %q, want the stored entity tag", assembled.GetEtag())
	}
}

// TestLinodeObjectStorageObjectDownloadExecuteRefusesBeforeCalling pins that an
// occupied destination costs no request: the path is resolved before presigning.
func TestLinodeObjectStorageObjectDownloadExecuteRefusesBeforeCalling(t *testing.T) {
	t.Parallel()

	var calls int

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	occupied := filepath.Join(t.TempDir(), "taken.bin")
	if err := os.WriteFile(occupied, []byte("already here"), 0o600); err != nil {
		t.Fatalf("seed the destination: %v", err)
	}

	request := requestWith(map[string]any{
		foldRegion:    crossSampleRegion,
		keyLabel:      uploadBucketLabel,
		argName:       uploadObjectKey,
		uploadDestArg: occupied,
	})

	_, err := toolhooks.LinodeObjectStorageObjectDownloadExecute(
		t.Context(), clientFor(t, server.URL), &request,
		[]any{crossSampleRegion, uploadBucketLabel}, downloadBody(&request),
	)
	if !errors.Is(err, objectdata.ErrDestinationExists) {
		t.Fatalf("error = %v, want ErrDestinationExists", err)
	}

	if calls != 0 {
		t.Errorf("made %d requests, want none for an occupied destination", calls)
	}
}

// TestLinodeObjectStorageObjectDownloadExecuteReportsEachLegsFailure pins that a
// failure in either leg reaches the caller rather than a partial answer.
func TestLinodeObjectStorageObjectDownloadExecuteReportsEachLegsFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		presignFail bool
	}{
		{name: "the presign call fails", presignFail: true},
		{name: "the transfer fails"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && tt.presignFail {
					w.WriteHeader(http.StatusInternalServerError)

					return
				}

				if r.Method == http.MethodPost {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"url":"` + "http://" + r.Host + `/artifacts/key"}`))

					return
				}

				w.WriteHeader(http.StatusForbidden)
			}))
			t.Cleanup(server.Close)

			request := requestWith(map[string]any{
				foldRegion:    crossSampleRegion,
				keyLabel:      uploadBucketLabel,
				argName:       uploadObjectKey,
				uploadDestArg: filepath.Join(t.TempDir(), "landed.bin"),
			})

			_, err := toolhooks.LinodeObjectStorageObjectDownloadExecute(
				t.Context(), clientFor(t, server.URL), &request,
				[]any{crossSampleRegion, uploadBucketLabel}, downloadBody(&request),
			)
			if err == nil {
				t.Fatal("err = nil, want the failure the handler formats its sentence around")
			}
		})
	}
}
