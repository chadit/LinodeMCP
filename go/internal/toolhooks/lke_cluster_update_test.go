package toolhooks_test

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	// clusterUpdatePath is the cluster the preview reads and would write.
	clusterUpdatePath = "/lke/clusters/123"
	argK8sVersion     = "k8s_version"
	labelBefore       = "before"
	k8sVersionNext    = "1.32"
	labelChangeEffect = `Label changes from "before" to "after".`
	upgradeEffect     = `Kubernetes version changes from "1.30" to "1.32"; the control plane and nodes upgrade.`
)

// clusterUpdateState is the cluster every case below reads before previewing.
const clusterUpdateState = `{"id":123,"label":"before","k8s_version":"1.30"}`

// TestLkeClusterUpdatePreviewNamesTheChange walks every sentence pair the hook
// can answer against the cluster it reads first.
func TestLkeClusterUpdatePreviewNamesTheChange(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want []string
	}{
		{
			name: "a new label reports what it replaces",
			args: map[string]any{argLabel: labelAfter},
			want: []string{labelChangeEffect},
		},
		{
			name: "a label matching the cluster reports a set",
			args: map[string]any{argLabel: labelBefore},
			want: []string{`Label is set to "before".`},
		},
		{
			name: "a newer version reports the upgrade it starts from",
			args: map[string]any{argK8sVersion: k8sVersionNext},
			want: []string{upgradeEffect},
		},
		{
			name: "the version the cluster already runs reports nothing",
			args: map[string]any{argK8sVersion: "1.30"},
			want: nil,
		},
		{
			name: "a tags-only edit reports nothing",
			args: map[string]any{argTags: []any{tagLabelProd}},
			want: nil,
		},
		{
			name: "both changes are reported in order",
			args: map[string]any{argLabel: labelAfter, argK8sVersion: k8sVersionNext},
			want: []string{labelChangeEffect, upgradeEffect},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var fetched string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fetched = r.URL.Path

				if _, err := w.Write([]byte(clusterUpdateState)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			args := map[string]any{argClusterID: float64(123), keyDryRun: true}
			maps.Copy(args, testCase.args)

			request := requestWith(args)

			result, err := toolhooks.LinodeLkeClusterUpdatePreview(t.Context(), &request,
				configFor(server.URL), http.MethodPut, clusterUpdatePath, testCase.args)
			if err != nil {
				t.Fatalf("preview: %v", err)
			}

			if fetched != clusterUpdatePath {
				t.Errorf("fetched = %q, want the cluster the change starts from", fetched)
			}

			assertClusterUpdatePreview(t, resultText(t, result), testCase.want)
		})
	}
}

// assertClusterUpdatePreview checks the preview envelope and the exact
// sentences the walk produced, in order.
func assertClusterUpdatePreview(t *testing.T, text string, want []string) {
	t.Helper()

	var preview struct {
		WouldExecute struct {
			Method string `json:"method"`
			Path   string `json:"path"`
		} `json:"would_execute"`
		CurrentState struct {
			Label string `json:"label"`
		} `json:"current_state"`
		SideEffects []string `json:"side_effects"`
	}

	if err := json.Unmarshal([]byte(text), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}

	if preview.CurrentState.Label != labelBefore {
		t.Errorf("current_state.label = %q, want the label the change replaces", preview.CurrentState.Label)
	}

	if preview.WouldExecute.Method != http.MethodPut || preview.WouldExecute.Path != clusterUpdatePath {
		t.Errorf("would_execute = %s %s, want PUT %s",
			preview.WouldExecute.Method, preview.WouldExecute.Path, clusterUpdatePath)
	}

	if len(preview.SideEffects) != len(want) {
		t.Fatalf("side_effects = %v, want %v", preview.SideEffects, want)
	}

	for i, sentence := range want {
		if preview.SideEffects[i] != sentence {
			t.Errorf("side_effects[%d] = %q, want %q", i, preview.SideEffects[i], sentence)
		}
	}
}

// TestLkeClusterUpdatePreviewReportsAnUnreadableCluster pins that a failed read
// reaches the caller as a tool error rather than a preview naming no starting
// point.
func TestLkeClusterUpdatePreviewReportsAnUnreadableCluster(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	request := requestWith(map[string]any{
		argClusterID: float64(123),
		argLabel:     labelAfter,
		keyDryRun:    true,
	})

	result, err := toolhooks.LinodeLkeClusterUpdatePreview(t.Context(), &request,
		configFor(server.URL), http.MethodPut, clusterUpdatePath, nil)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true for an unreadable cluster")
	}
}
