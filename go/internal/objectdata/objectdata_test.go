package objectdata_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/objectdata"
)

const transferBudget = 30 * time.Second

// writeSource puts one file on disk and answers its path.
func writeSource(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return path
}

// TestUploadSendsTheFileBytesToThePresignedURL is the byte-move proof: the
// server asserts on what it actually received, so a transfer that sent nothing,
// truncated, or re-encoded the file fails here rather than reporting success.
func TestUploadSendsTheFileBytesToThePresignedURL(t *testing.T) {
	t.Parallel()

	const content = "hello object storage"

	var (
		gotBody   string
		gotMethod string
		gotType   string
		gotLength int64
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody, gotMethod = string(body), r.Method
		gotType, gotLength = r.Header.Get("Content-Type"), r.ContentLength
		w.Header().Set("ETag", `"5eb63bbbe01eeed093cb22bb8f5acdc3"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	source, err := objectdata.Inspect(writeSource(t, "small.bin", content), "")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	result, err := objectdata.Upload(t.Context(), objectdata.UploadRequest{
		URL:         srv.URL + "/artifacts/small.bin?sig=fixture",
		ContentType: "text/plain",
		Source:      source,
		Timeout:     transferBudget,
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if gotBody != content {
		t.Errorf("server received %q, want %q", gotBody, content)
	}

	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}

	if gotType != "text/plain" {
		t.Errorf("Content-Type = %q, want text/plain", gotType)
	}

	if gotLength != int64(len(content)) {
		t.Errorf("Content-Length = %d, want %d", gotLength, len(content))
	}

	if result.SizeBytes != int64(len(content)) {
		t.Errorf("SizeBytes = %d, want %d", result.SizeBytes, len(content))
	}

	if result.ETag != "5eb63bbbe01eeed093cb22bb8f5acdc3" {
		t.Errorf("ETag = %q, want the unquoted entity tag", result.ETag)
	}
}

// TestUploadReportsARefusalWithoutTheURL pins FR13: the presigned URL is a
// bearer credential, and net/http puts it in every transport error by default.
func TestUploadReportsARefusalWithoutTheURL(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	source, err := objectdata.Inspect(writeSource(t, "small.bin", "bytes"), "")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	_, err = objectdata.Upload(t.Context(), objectdata.UploadRequest{
		URL:     srv.URL + "/artifacts/small.bin?sig=secret-token",
		Source:  source,
		Timeout: transferBudget,
	})
	if !errors.Is(err, objectdata.ErrTransfer) {
		t.Fatalf("error = %v, want ErrTransfer", err)
	}

	if reported := err.Error(); strings.Contains(reported, "secret-token") {
		t.Errorf("error text leaks the presigned URL: %s", reported)
	}
}

// TestUploadRefusesAnUnreachableHostWithoutTheURL covers the other leak path:
// a transport failure arrives wrapped in a *url.Error carrying the full URL.
func TestUploadRefusesAnUnreachableHostWithoutTheURL(t *testing.T) {
	t.Parallel()

	source, err := objectdata.Inspect(writeSource(t, "small.bin", "bytes"), "")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	// Port 0 never accepts, so the client fails before any exchange.
	_, err = objectdata.Upload(t.Context(), objectdata.UploadRequest{
		URL:     "http://127.0.0.1:0/artifacts/small.bin?sig=secret-token",
		Source:  source,
		Timeout: transferBudget,
	})
	if !errors.Is(err, objectdata.ErrTransfer) {
		t.Fatalf("error = %v, want ErrTransfer", err)
	}

	if reported := err.Error(); strings.Contains(reported, "secret-token") {
		t.Errorf("error text leaks the presigned URL: %s", reported)
	}
}

func TestInspect(t *testing.T) {
	t.Parallel()

	t.Run("reports the resolved path and size", func(t *testing.T) {
		t.Parallel()

		source, err := objectdata.Inspect(writeSource(t, "small.bin", "12345"), "")
		if err != nil {
			t.Fatalf("Inspect: %v", err)
		}

		if source.SizeBytes != 5 {
			t.Errorf("SizeBytes = %d, want 5", source.SizeBytes)
		}
	})

	t.Run("refuses a missing file", func(t *testing.T) {
		t.Parallel()

		_, err := objectdata.Inspect(filepath.Join(t.TempDir(), "absent.bin"), "")
		if !errors.Is(err, objectdata.ErrSourceMissing) {
			t.Errorf("error = %v, want ErrSourceMissing", err)
		}
	})

	t.Run("refuses a directory", func(t *testing.T) {
		t.Parallel()

		_, err := objectdata.Inspect(t.TempDir(), "")
		if !errors.Is(err, objectdata.ErrSourceNotRegular) {
			t.Errorf("error = %v, want ErrSourceNotRegular", err)
		}
	})

	t.Run("accepts a file inside the configured root", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()

		path := filepath.Join(root, "inside.bin")
		if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := objectdata.Inspect(path, root); err != nil {
			t.Errorf("Inspect: %v", err)
		}
	})

	t.Run("refuses a file outside the configured root", func(t *testing.T) {
		t.Parallel()

		_, err := objectdata.Inspect(writeSource(t, "outside.bin", "ok"), t.TempDir())
		if !errors.Is(err, objectdata.ErrSourceOutsideRoot) {
			t.Errorf("error = %v, want ErrSourceOutsideRoot", err)
		}
	})

	t.Run("refuses a symlink pointing out of the root", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		outside := writeSource(t, "target.bin", "ok")

		link := filepath.Join(root, "link.bin")
		if err := os.Symlink(outside, link); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// The link itself sits inside the root, so only resolving it first
		// catches the escape.
		_, err := objectdata.Inspect(link, root)
		if !errors.Is(err, objectdata.ErrSourceOutsideRoot) {
			t.Errorf("error = %v, want ErrSourceOutsideRoot", err)
		}
	})

	t.Run("refuses a sibling directory sharing the root's name prefix", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		root := filepath.Join(parent, "data")
		sibling := filepath.Join(parent, "data-other")

		for _, dir := range []string{root, sibling} {
			if err := os.Mkdir(dir, 0o750); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}

		path := filepath.Join(sibling, "file.bin")
		if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err := objectdata.Inspect(path, root)
		if !errors.Is(err, objectdata.ErrSourceOutsideRoot) {
			t.Errorf("error = %v, want ErrSourceOutsideRoot", err)
		}
	})
}

func TestCheckSinglePart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		size    int64
		ceiling int64
		wantErr bool
	}{
		{name: "under the ceiling", size: 1023, ceiling: 1024},
		{name: "exactly the ceiling", size: 1024, ceiling: 1024},
		{name: "over the ceiling", size: 1025, ceiling: 1024, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := objectdata.CheckSinglePart(tt.size, tt.ceiling)
			if tt.wantErr != (err != nil) {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				return
			}

			if !errors.Is(err, objectdata.ErrAboveSinglePartCeiling) {
				t.Fatalf("error = %v, want ErrAboveSinglePartCeiling", err)
			}

			// The refusal has to be actionable: both numbers, and what is
			// missing rather than a tool that does not exist.
			reported := err.Error()
			for _, want := range []string{"1025", "1024", "multipart upload is not implemented yet"} {
				if !strings.Contains(reported, want) {
					t.Errorf("error %q does not name %q", reported, want)
				}
			}
		})
	}
}

// TestInspectRefusesWhenTheConfiguredRootDoesNotExist pins that a misconfigured
// root confines everything rather than nothing: failing open would silently
// drop the confinement the operator asked for.
func TestInspectRefusesWhenTheConfiguredRootDoesNotExist(t *testing.T) {
	t.Parallel()

	_, err := objectdata.Inspect(
		writeSource(t, "file.bin", "ok"), filepath.Join(t.TempDir(), "absent-root"))
	if !errors.Is(err, objectdata.ErrSourceOutsideRoot) {
		t.Errorf("error = %v, want ErrSourceOutsideRoot", err)
	}
}

// TestUploadRefusesASourceThatVanishedAfterInspect covers the window between
// the preview's stat and the transfer's open: the file can go away in between,
// and the transfer has to say so rather than send an empty object.
func TestUploadRefusesASourceThatVanishedAfterInspect(t *testing.T) {
	t.Parallel()

	path := writeSource(t, "vanishing.bin", "bytes")

	source, err := objectdata.Inspect(path, "")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	if removeErr := os.Remove(path); removeErr != nil {
		t.Fatalf("unexpected error: %v", removeErr)
	}

	_, err = objectdata.Upload(t.Context(), objectdata.UploadRequest{
		URL:     "http://127.0.0.1:1/artifacts/key",
		Source:  source,
		Timeout: transferBudget,
	})
	if !errors.Is(err, objectdata.ErrSourceMissing) {
		t.Errorf("error = %v, want ErrSourceMissing", err)
	}
}

// TestUploadRefusesAnUnusableURL pins that a presigned URL the client cannot
// even build is reported as a transfer failure rather than a panic.
func TestUploadRefusesAnUnusableURL(t *testing.T) {
	t.Parallel()

	source, err := objectdata.Inspect(writeSource(t, "small.bin", "bytes"), "")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	_, err = objectdata.Upload(t.Context(), objectdata.UploadRequest{
		URL:     "http://[::1]:namedport/key",
		Source:  source,
		Timeout: transferBudget,
	})
	if !errors.Is(err, objectdata.ErrTransfer) {
		t.Errorf("error = %v, want ErrTransfer", err)
	}
}

// TestDownloadWritesTheServedBytesToTheDestination is the byte-move proof for
// the download half: the file on disk is compared against what the server sent,
// so a transfer that truncated or re-encoded fails here.
func TestDownloadWritesTheServedBytesToTheDestination(t *testing.T) {
	t.Parallel()

	const body = "hello object storage"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}

		w.Header().Set("ETag", `"abc123"`)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	destination := filepath.Join(t.TempDir(), "landed.bin")

	result, err := objectdata.Download(t.Context(), objectdata.DownloadRequest{
		URL:      srv.URL + "/artifacts/key?sig=fixture",
		DestPath: destination,
		Timeout:  transferBudget,
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}

	landed, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(landed) != body {
		t.Errorf("file holds %q, want %q", landed, body)
	}

	if result.SizeBytes != int64(len(body)) {
		t.Errorf("SizeBytes = %d, want %d", result.SizeBytes, len(body))
	}

	if result.ETag != "abc123" {
		t.Errorf("ETag = %q, want the unquoted entity tag", result.ETag)
	}
}

// TestDownloadLeavesNoFileWhenTheBucketRefuses pins the reason the transfer
// writes through a temporary file: a refused download must not leave a
// truncated or empty file at the path the caller named.
func TestDownloadLeavesNoFileWhenTheBucketRefuses(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	destination := filepath.Join(t.TempDir(), "landed.bin")

	_, err := objectdata.Download(t.Context(), objectdata.DownloadRequest{
		URL:      srv.URL + "/artifacts/key?sig=secret-token",
		DestPath: destination,
		Timeout:  transferBudget,
	})
	if !errors.Is(err, objectdata.ErrTransfer) {
		t.Fatalf("error = %v, want ErrTransfer", err)
	}

	if reported := err.Error(); strings.Contains(reported, "secret-token") {
		t.Errorf("error text leaks the presigned URL: %s", reported)
	}

	if _, statErr := os.Stat(destination); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("destination exists after a refused download, want no file")
	}
}

func TestResolveDestination(t *testing.T) {
	t.Parallel()

	t.Run("refuses an existing file without overwrite", func(t *testing.T) {
		t.Parallel()

		_, err := objectdata.ResolveDestination(writeSource(t, "there.bin", "x"), "", false)
		if !errors.Is(err, objectdata.ErrDestinationExists) {
			t.Errorf("error = %v, want ErrDestinationExists", err)
		}
	})

	t.Run("accepts an existing file with overwrite", func(t *testing.T) {
		t.Parallel()

		if _, err := objectdata.ResolveDestination(
			writeSource(t, "there.bin", "x"), "", true); err != nil {
			t.Errorf("ResolveDestination: %v", err)
		}
	})

	t.Run("refuses a directory", func(t *testing.T) {
		t.Parallel()

		_, err := objectdata.ResolveDestination(t.TempDir(), "", true)
		if !errors.Is(err, objectdata.ErrDestinationUnwritable) {
			t.Errorf("error = %v, want ErrDestinationUnwritable", err)
		}
	})

	t.Run("refuses a destination outside the configured root", func(t *testing.T) {
		t.Parallel()

		outside := filepath.Join(t.TempDir(), "landed.bin")

		_, err := objectdata.ResolveDestination(outside, t.TempDir(), false)
		if !errors.Is(err, objectdata.ErrSourceOutsideRoot) {
			t.Errorf("error = %v, want ErrSourceOutsideRoot", err)
		}
	})
}

// TestDownloadLeavesNoFileWhenTheBodyIsTruncated pins the temporary-file write
// against the case it exists for: the server promises more bytes than it sends,
// so the copy fails partway and the destination must stay untouched.
func TestDownloadLeavesNoFileWhenTheBodyIsTruncated(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// A Content-Length larger than the body makes the client's read fail
		// with an unexpected EOF partway through.
		w.Header().Set("Content-Length", "64")
		_, _ = w.Write([]byte("short"))
	}))
	defer srv.Close()

	destination := filepath.Join(t.TempDir(), "landed.bin")

	_, err := objectdata.Download(t.Context(), objectdata.DownloadRequest{
		URL:      srv.URL + "/artifacts/key",
		DestPath: destination,
		Timeout:  transferBudget,
	})
	if !errors.Is(err, objectdata.ErrTransfer) {
		t.Fatalf("error = %v, want ErrTransfer", err)
	}

	if _, statErr := os.Stat(destination); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("destination exists after a truncated transfer, want no file")
	}
}

// TestDownloadRefusalsThroughTheWholeCall walks the refusals Download answers
// itself, each one reached through the exported entry point rather than the
// helper behind it, so the wiring is covered along with the verdict.
func TestDownloadRefusalsThroughTheWholeCall(t *testing.T) {
	t.Parallel()

	served := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("bytes"))
	}))
	// Cleanup rather than defer: the subtests below run in parallel, so a
	// deferred close would shut the server down before they reach it.
	t.Cleanup(served.Close)

	tests := []struct {
		destination func(t *testing.T) string
		want        error
		name        string
		url         string
	}{
		{
			name: "the destination is occupied",
			destination: func(t *testing.T) string {
				t.Helper()

				return writeSource(t, "there.bin", "x")
			},
			url:  served.URL + "/artifacts/key",
			want: objectdata.ErrDestinationExists,
		},
		{
			name: "the presigned URL cannot be used",
			destination: func(t *testing.T) string {
				t.Helper()

				return filepath.Join(t.TempDir(), "landed.bin")
			},
			url:  "http://[::1]:namedport/key",
			want: objectdata.ErrTransfer,
		},
		{
			name: "the host never answers",
			destination: func(t *testing.T) string {
				t.Helper()

				return filepath.Join(t.TempDir(), "landed.bin")
			},
			// Port 0 never accepts, so the client fails before any exchange.
			url:  "http://127.0.0.1:0/artifacts/key",
			want: objectdata.ErrTransfer,
		},
		{
			name: "the destination directory does not exist",
			destination: func(t *testing.T) string {
				t.Helper()

				return filepath.Join(t.TempDir(), "absent", "landed.bin")
			},
			url:  served.URL + "/artifacts/key",
			want: objectdata.ErrDestinationUnwritable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := objectdata.Download(t.Context(), objectdata.DownloadRequest{
				URL:      tt.url,
				DestPath: tt.destination(t),
				Timeout:  transferBudget,
			})
			if !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
		})
	}
}
