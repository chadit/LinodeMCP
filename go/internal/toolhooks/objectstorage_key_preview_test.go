package toolhooks_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	keyUpdatePath  = "/object-storage/keys/77"
	argBucketArray = "bucket_access"
	keyStoredLabel = "my-key"
	keyNewLabel    = "renamed-key"
	// keyScopeReplaced is the sentence the update names when the caller sends a
	// replacement scope list.
	keyScopeReplaced = "The key's bucket access scopes are replaced."
)

// keyServer answers the key read the update preview fetches, and fails the test
// if anything else is asked for.
func keyServer(t *testing.T, label string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/object-storage/keys/77") {
			t.Errorf("preview fetched %s, want the key read", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{"id": 77, keyLabel: label}); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}))
}

// TestKeyUpdatePreviewNamesBothChanges covers the walk's two arms together: a
// label that differs from the one the fetch read, and a replacement scope list.
func TestKeyUpdatePreviewNamesBothChanges(t *testing.T) {
	t.Parallel()

	server := keyServer(t, keyStoredLabel)
	defer server.Close()

	request := requestWith(map[string]any{
		argKeyID: float64(77), keyLabel: keyNewLabel, keyDryRun: true,
		argBucketArray: []any{map[string]any{
			"bucket_name": sampleLabel, argRegion: sampleRegion, "permissions": "read_only",
		}},
	})
	body := map[string]any{keyLabel: keyNewLabel}

	result, err := toolhooks.LinodeObjectStorageKeyUpdatePreview(t.Context(), &request,
		configFor(server.URL), http.MethodPut, keyUpdatePath, body)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	text := resultText(t, result)

	for _, want := range []string{
		`Label changes from \"` + keyStoredLabel + `\" to \"` + keyNewLabel + `\".`,
		keyScopeReplaced,
		`"label": "` + keyStoredLabel + `"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("preview text %q lacks %q", text, want)
		}
	}
}

// TestKeyUpdatePreviewNamesALabelTheKeyAlreadyCarries covers the walk's other
// label arm: the fetched key answers with the same label, so the sentence says
// what the label is set to rather than what it changes from.
func TestKeyUpdatePreviewNamesALabelTheKeyAlreadyCarries(t *testing.T) {
	t.Parallel()

	server := keyServer(t, keyNewLabel)
	defer server.Close()

	request := requestWith(map[string]any{
		argKeyID: float64(77), keyLabel: keyNewLabel, keyDryRun: true,
	})

	result, err := toolhooks.LinodeObjectStorageKeyUpdatePreview(t.Context(), &request,
		configFor(server.URL), http.MethodPut, keyUpdatePath, map[string]any{})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	text := resultText(t, result)
	if want := `Label is set to \"` + keyNewLabel + `\".`; !strings.Contains(text, want) {
		t.Errorf("preview text %q lacks %q", text, want)
	}

	if strings.Contains(text, keyScopeReplaced) {
		t.Errorf("preview text %q names a scope replacement the caller never asked for", text)
	}
}

// TestKeyUpdatePreviewOnAScopeOnlyCall covers the remaining pair of arms: no
// label to report, and the scope list alone.
func TestKeyUpdatePreviewOnAScopeOnlyCall(t *testing.T) {
	t.Parallel()

	server := keyServer(t, keyStoredLabel)
	defer server.Close()

	request := requestWith(map[string]any{
		argKeyID: float64(77), keyDryRun: true,
		argBucketArray: []any{map[string]any{
			"bucket_name": sampleLabel, argRegion: sampleRegion, "permissions": "read_write",
		}},
	})

	result, err := toolhooks.LinodeObjectStorageKeyUpdatePreview(t.Context(), &request,
		configFor(server.URL), http.MethodPut, keyUpdatePath, map[string]any{})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	text := resultText(t, result)
	if !strings.Contains(text, keyScopeReplaced) {
		t.Errorf("preview text %q lacks %q", text, keyScopeReplaced)
	}

	if strings.Contains(text, "Label") {
		t.Errorf("preview text %q names a label change the caller never asked for", text)
	}
}
