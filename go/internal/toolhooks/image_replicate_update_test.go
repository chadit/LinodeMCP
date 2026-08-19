package toolhooks_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

// The image these two previews read, the argument naming the region list, and
// the second slug a multi-region replication lists.
const (
	previewImageID = "private/123"
	argRegions     = "regions"
	regionUSWest   = "us-west"
)

// imageFetchServer answers the image read both previews make and records the
// path it was addressed by.
func imageFetchServer(t *testing.T, fetched *string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*fetched = r.URL.EscapedPath()

		if _, err := w.Write([]byte(`{"id":"private/123","label":"golden"}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
}

// previewSideEffects decodes the sentences a preview answered with.
func previewSideEffects(t *testing.T, text string) []string {
	t.Helper()

	var preview struct {
		SideEffects []string `json:"side_effects"`
	}

	if err := json.Unmarshal([]byte(text), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}

	return preview.SideEffects
}

// TestImageReplicateNormalizeTrimsEachSlug: the slug rule reads what normalize
// leaves behind, so a padded entry has to arrive trimmed and anything that is
// not text has to survive untouched for the rule to refuse it.
func TestImageReplicateNormalizeTrimsEachSlug(t *testing.T) {
	t.Parallel()

	cases := []struct {
		supplied any
		want     any
		name     string
	}{
		{
			name:     "trims each slug",
			supplied: []any{" " + pgRegion + " ", regionUSWest},
			want:     []any{pgRegion, regionUSWest},
		},
		{
			name:     "leaves a non-text entry for the rule",
			supplied: []any{7},
			want:     []any{7},
		},
		{
			name:     "leaves a value that is no list",
			supplied: pgRegion,
			want:     pgRegion,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := requestWith(map[string]any{argRegions: testCase.supplied})

			toolhooks.LinodeImageReplicateNormalize(&request)

			if got := request.GetArguments()[argRegions]; !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("regions = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestImageReplicateNormalizeLeavesAnAbsentList: a call with no regions is the
// rule's to refuse, so normalize must not invent an empty list for it.
func TestImageReplicateNormalizeLeavesAnAbsentList(t *testing.T) {
	t.Parallel()

	request := requestWith(map[string]any{argImageID: previewImageID})

	toolhooks.LinodeImageReplicateNormalize(&request)

	if _, supplied := request.GetArguments()[argRegions]; supplied {
		t.Error("regions was added, want it left absent")
	}
}

// TestImageReplicatePreviewReadsTheImageAndListsTheRegions: the sentence is
// what says where the copies land, and the read is what says what is copied.
func TestImageReplicatePreviewReadsTheImageAndListsTheRegions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		supplied any
		name     string
		want     string
	}{
		{
			name:     "lists every slug",
			supplied: []any{pgRegion, regionUSWest},
			want:     "Image 'private/123' will be replicated to us-east, us-west.",
		},
		{
			name:     "drops an entry that is no text",
			supplied: []any{pgRegion, 7},
			want:     "Image 'private/123' will be replicated to us-east.",
		},
		{
			name:     "names no region when the list is not one",
			supplied: pgRegion,
			want:     "Image 'private/123' will be replicated to .",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var fetched string

			server := imageFetchServer(t, &fetched)
			defer server.Close()

			request := requestWith(map[string]any{
				argImageID: previewImageID,
				argRegions: testCase.supplied,
				keyDryRun:  true,
			})

			result, err := toolhooks.LinodeImageReplicatePreview(t.Context(), &request,
				configFor(server.URL), http.MethodPost, "/images/private%2F123/regions", nil)
			if err != nil {
				t.Fatalf("preview: %v", err)
			}

			if fetched != "/images/private%2F123" {
				t.Errorf("fetched = %q, want the image as one path segment", fetched)
			}

			effects := previewSideEffects(t, resultText(t, result))
			if len(effects) != 1 || effects[0] != testCase.want {
				t.Errorf("side_effects = %v, want [%q]", effects, testCase.want)
			}
		})
	}
}

// TestImageReplicatePreviewReportsAFailedFetch: a read the API refuses is
// reported rather than answered with a preview naming no state.
func TestImageReplicatePreviewReportsAFailedFetch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		if _, err := w.Write([]byte(`{"errors":[{"reason":"Not found"}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	request := requestWith(map[string]any{
		argImageID: previewImageID,
		argRegions: []any{pgRegion},
		keyDryRun:  true,
	})

	result, err := toolhooks.LinodeImageReplicatePreview(t.Context(), &request,
		configFor(server.URL), http.MethodPost, "/images/private%2F123/regions", nil)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	if !result.IsError {
		t.Errorf("result.IsError = false, want true")
	}
}
