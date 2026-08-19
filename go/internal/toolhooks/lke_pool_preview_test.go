package toolhooks_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	lkePoolUpdatePath = "/lke/clusters/123/pools/10"
	lkePoolCreatePath = "/lke/clusters/123/pools"
	lkeClusterPath    = "/lke/clusters/123"
	argCount          = "count"
	argAutoscaler     = "autoscaler"
	poolAtTwo         = `{"id":10,"cluster_id":123,"count":2}`
)

// TestLkePoolUpdatePreviewNamesTheCountChange: the count a resize starts from is
// the one number the arguments cannot supply, so the sentence names it only when
// the pool read answered with one.
func TestLkePoolUpdatePreviewNamesTheCountChange(t *testing.T) {
	t.Parallel()

	cases := []struct {
		arguments map[string]any
		name      string
		pool      string
		want      []string
	}{
		{
			name:      "resize from a read count",
			pool:      poolAtTwo,
			arguments: map[string]any{argClusterID: 123, argPoolID: 10, argCount: 5, keyDryRun: true},
			want:      []string{"Node pool resizes from 2 to 5 node(s)."},
		},
		{
			name:      "a pool at zero says only where it lands",
			pool:      `{"id":10,"cluster_id":123,"count":0}`,
			arguments: map[string]any{argClusterID: 123, argPoolID: 10, argCount: 5, keyDryRun: true},
			want:      []string{"Node pool is set to 5 node(s)."},
		},
		{
			name:      "an unchanged count says only where it lands",
			pool:      `{"id":10,"cluster_id":123,"count":5}`,
			arguments: map[string]any{argClusterID: 123, argPoolID: 10, argCount: 5, keyDryRun: true},
			want:      []string{"Node pool is set to 5 node(s)."},
		},
		{
			name: "an explicit null counts as supplied",
			pool: poolAtTwo,
			arguments: map[string]any{
				argClusterID: 123, argPoolID: 10, argCount: nil, keyDryRun: true,
			},
			want: []string{"Node pool resizes from 2 to 0 node(s)."},
		},
		{
			name: "the autoscaler sentence joins the resize",
			pool: poolAtTwo,
			arguments: map[string]any{
				argClusterID: 123, argPoolID: 10, argCount: 5,
				argAutoscaler: map[string]any{"enabled": true},
				keyDryRun:     true,
			},
			want: []string{
				"Node pool resizes from 2 to 5 node(s).",
				"The pool autoscaler configuration is updated.",
			},
		},
		{
			name:      "the autoscaler alone",
			pool:      poolAtTwo,
			arguments: map[string]any{argClusterID: 123, argPoolID: 10, argAutoscaler: map[string]any{}, keyDryRun: true},
			want:      []string{"The pool autoscaler configuration is updated."},
		},
		{
			name:      "neither argument leaves no sentence",
			pool:      poolAtTwo,
			arguments: map[string]any{argClusterID: 123, argPoolID: 10, argTags: []any{"web"}, keyDryRun: true},
			want:      nil,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var fetched string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fetched = r.Method + " " + r.URL.Path

				if _, err := w.Write([]byte(testCase.pool)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			request := requestWith(testCase.arguments)

			result, err := toolhooks.LinodeLkePoolUpdatePreview(t.Context(), &request,
				configFor(server.URL), http.MethodPut, lkePoolUpdatePath, nil)
			if err != nil {
				t.Fatalf("preview: %v", err)
			}

			if want := http.MethodGet + " " + lkePoolUpdatePath; fetched != want {
				t.Errorf("fetched = %q, want %q", fetched, want)
			}

			var preview struct {
				SideEffects []string `json:"side_effects"`
			}

			if err := json.Unmarshal([]byte(resultText(t, result)), &preview); err != nil {
				t.Fatalf("decode preview: %v", err)
			}

			if len(preview.SideEffects) != len(testCase.want) {
				t.Fatalf("side_effects = %v, want %v", preview.SideEffects, testCase.want)
			}

			for index, effect := range testCase.want {
				if preview.SideEffects[index] != effect {
					t.Errorf("side_effects[%d] = %q, want %q", index, preview.SideEffects[index], effect)
				}
			}
		})
	}
}

// TestLkePoolUpdatePreviewReportsAFailedFetch: the pool read is what the resize
// sentence is diffed against, so a read the API refuses is reported rather than
// answered with a preview that names a count nobody read.
func TestLkePoolUpdatePreviewReportsAFailedFetch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		if _, err := w.Write([]byte(`{"errors":[{"reason":"Not found"}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	request := requestWith(map[string]any{
		argClusterID: 123, argPoolID: 10, argCount: 5, keyDryRun: true,
	})

	result, err := toolhooks.LinodeLkePoolUpdatePreview(t.Context(), &request,
		configFor(server.URL), http.MethodPut, lkePoolUpdatePath, nil)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	if !result.IsError {
		t.Errorf("result.IsError = false, want true")
	}
}
