package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/objectdata"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The presigned upload's dry run: the guard the live transfer runs, taken
// before any call, and the declared prose worded against what it measured.
// Python's twin is test_tool_transport.py.

const (
	transferPresignPath = "/object-storage/buckets/" + transferRegion + "/" + transferBucket + "/object-url"
	transferSizeWording = "{transport:size_bytes} bytes will be uploaded to '{name}'."
	transferSizeKey     = "transport:size_bytes"
)

// transferPreviewLines words the one declared line the cases read: the size the
// guard measured, dropped when the guard refused instead.
func transferPreviewLines(transfer tools.PresignPreview) tools.DryRunDetails {
	return tools.DryRunDetails{
		SideEffects: []string{tools.PreviewSentence(
			map[string]string{transferSizeKey: transfer.SizeBytes, keyName: transferObjectKey},
			transferSizeWording,
		)},
	}
}

func TestPreviewPresignSourceMeasuresTheSourceAndFillsTheBody(t *testing.T) {
	t.Parallel()

	request := transferRequest(map[string]any{keySourcePath: transferSource(t, transferContent)})
	body := transferBody(&request, http.MethodPut)

	transfer := tools.PreviewPresignSource(&request, nil, body, keySourcePath)

	if transfer.SizeBytes != "20" || transfer.Refusal != "" {
		t.Errorf("PreviewPresignSource = %+v, want the 20-byte source and no refusal", transfer)
	}

	rendered, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The defaults apply with no config at all, so the preview describes the
	// request the live call would sign.
	for _, want := range []string{objectdata.DefaultContentType, `"expires_in":3600`} {
		if !strings.Contains(string(rendered), want) {
			t.Errorf("filled body %s does not carry %q", rendered, want)
		}
	}
}

func TestPreviewPresignSourceRefusesWhatTheTransferWouldRefuse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		source  string
		want    string
		ceiling int64
	}{
		{
			name:    "over the ceiling",
			source:  transferSource(t, strings.Repeat("a", 2048)),
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

			request := transferRequest(map[string]any{keySourcePath: tt.source})
			cfg := &config.Config{ObjectStorage: config.ObjectStorageConfig{MaxSinglePartBytes: tt.ceiling}}

			transfer := tools.PreviewPresignSource(&request, cfg, nil, keySourcePath)

			if transfer.SizeBytes != "" || !strings.Contains(transfer.Refusal, tt.want) {
				t.Errorf("PreviewPresignSource = %+v, want a refusal naming %q and no size", transfer, tt.want)
			}
		})
	}
}

func TestRunDeclaredTransportPreviewWordsTheLineAgainstTheMeasurement(t *testing.T) {
	t.Parallel()

	request := transferRequest(map[string]any{keySourcePath: transferSource(t, transferContent)})

	result, err := tools.RunDeclaredTransportPreview(t.Context(), &request, nil, uploadTool,
		http.MethodPost, transferPresignPath, transferBody(&request, http.MethodPut), keySourcePath,
		transferPreviewLines)
	if err != nil {
		t.Fatalf("RunDeclaredTransportPreview: %v", err)
	}

	rendered := resultJSON(t, result)

	if !strings.Contains(rendered, "20 bytes will be uploaded to 'key'.") {
		t.Errorf("preview %s does not word the measured size", rendered)
	}

	if strings.Contains(rendered, `"warnings"`) {
		t.Errorf("preview %s warns about a transfer the guard accepted", rendered)
	}
}

func TestRunDeclaredTransportPreviewReportsTheRefusalAsAWarning(t *testing.T) {
	t.Parallel()

	request := transferRequest(map[string]any{keySourcePath: filepath.Join(t.TempDir(), "absent.bin")})

	result, err := tools.RunDeclaredTransportPreview(t.Context(), &request, nil, uploadTool,
		http.MethodPost, transferPresignPath, nil, keySourcePath, transferPreviewLines)
	if err != nil {
		t.Fatalf("RunDeclaredTransportPreview: %v", err)
	}

	rendered := resultJSON(t, result)

	if !strings.Contains(rendered, "no readable file") {
		t.Errorf("preview %s does not warn about the refusal", rendered)
	}

	// The size line reads a value the guard never measured, so it is dropped
	// rather than reported with a gap in it.
	if strings.Contains(rendered, "bytes will be uploaded") {
		t.Errorf("preview %s describes a transfer the guard refused", rendered)
	}
}

func TestRunDeclaredTransportPreviewStopsWhenTheCallerHasGone(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	request := transferRequest(map[string]any{keySourcePath: transferSource(t, transferContent)})

	var worded bool

	result, err := tools.RunDeclaredTransportPreview(ctx, &request, nil, uploadTool,
		http.MethodPost, transferPresignPath, nil, keySourcePath,
		func(transfer tools.PresignPreview) tools.DryRunDetails {
			worded = true

			return transferPreviewLines(transfer)
		})
	if err != nil {
		t.Fatalf("RunDeclaredTransportPreview: %v", err)
	}

	if !result.IsError || worded {
		t.Errorf("result = %+v (worded=%v), want the canceled walk reported before anything is measured",
			result, worded)
	}

	if rendered := resultJSON(t, result); !strings.Contains(rendered, "canceled") {
		t.Errorf("error %s does not name the cancellation", rendered)
	}
}

func TestRunDeclaredTransportPreviewReportsABodyItCannotDescribe(t *testing.T) {
	t.Parallel()

	request := transferRequest(map[string]any{keySourcePath: transferSource(t, transferContent)})

	_, err := tools.RunDeclaredTransportPreview(t.Context(), &request, nil, uploadTool,
		http.MethodPost, transferPresignPath, map[string]any{unmarshalableMember: make(chan int)}, keySourcePath,
		transferPreviewLines)
	if _, refused := errors.AsType[*json.UnsupportedTypeError](err); !refused {
		t.Fatalf("error = %v, want a body JSON has no form for to be refused", err)
	}
}

// resultJSON renders a tool result the way a client reads it.
func resultJSON(t *testing.T, result any) string {
	t.Helper()

	rendered, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return string(rendered)
}
