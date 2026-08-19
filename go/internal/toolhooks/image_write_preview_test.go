package toolhooks_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

// TestImageCreatePreviewNamesTheDiskAndLabel: a caller reading this preview
// knows only what it says, since nothing exists yet to fetch, so both the disk
// and the label it lands under have to be in the sentence.
func TestImageCreatePreviewNamesTheDiskAndLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		arguments  map[string]any
		name       string
		wantEffect string
	}{
		{
			name:       "with a label",
			arguments:  map[string]any{"disk_id": 456, keyLabel: "nightly", keyDryRun: true},
			wantEffect: `A new image will be captured from disk 456 and labeled "nightly".`,
		},
		{
			name:       "without a label",
			arguments:  map[string]any{"disk_id": 456, keyDryRun: true},
			wantEffect: "A new image will be captured from disk 456.",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				t.Errorf("preview called %s %s, want no call", r.Method, r.URL.Path)
			}))
			defer server.Close()

			request := requestWith(testCase.arguments)

			result, err := toolhooks.LinodeImageCreatePreview(t.Context(), &request,
				configFor(server.URL), http.MethodPost, "/images", nil)
			if err != nil {
				t.Fatalf("preview: %v", err)
			}

			var preview struct {
				CurrentState any      `json:"current_state"`
				SideEffects  []string `json:"side_effects"`
			}

			if err := json.Unmarshal([]byte(resultText(t, result)), &preview); err != nil {
				t.Fatalf("decode preview: %v", err)
			}

			if len(preview.SideEffects) != 1 || preview.SideEffects[0] != testCase.wantEffect {
				t.Errorf("side_effects = %v, want [%q]", preview.SideEffects, testCase.wantEffect)
			}

			if preview.CurrentState != nil {
				t.Errorf("current_state = %v, want nil", preview.CurrentState)
			}
		})
	}
}
