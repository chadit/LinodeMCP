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
	// vpcPath is the VPC every update case below reads and would write.
	vpcPath       = "/vpcs/5"
	vpcSubnetPath = "/vpcs/5/subnets/7"
	argIPv4       = "ipv4"
	vpcLabelOld   = "old-label"
	vpcDescEffect = "The VPC description is updated."
)

// vpcUpdateState is the VPC every update case reads before previewing.
const vpcUpdateState = `{"id":5,"label":"old-label","description":"","region":"us-east"}`

// vpcPreview is the part of a preview envelope these tests read.
type vpcPreview struct {
	WouldExecute struct {
		Body   map[string]any `json:"body"`
		Method string         `json:"method"`
		Path   string         `json:"path"`
	} `json:"would_execute"`
	CurrentState *struct {
		Label string `json:"label"`
	} `json:"current_state"`
	SideEffects []string `json:"side_effects"`
}

// decodeVPCPreview reads the envelope a VPC preview answered with.
func decodeVPCPreview(t *testing.T, text string) vpcPreview {
	t.Helper()

	var preview vpcPreview
	if err := json.Unmarshal([]byte(text), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}

	return preview
}

// assertSideEffects checks the sentences a walk produced, in order.
func assertSideEffects(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("side_effects = %v, want %v", got, want)
	}

	for i, sentence := range want {
		if got[i] != sentence {
			t.Errorf("side_effects[%d] = %q, want %q", i, got[i], sentence)
		}
	}
}

// TestVPCUpdatePreviewNamesTheChange walks every sentence the update walk can
// answer against the VPC it reads first.
func TestVPCUpdatePreviewNamesTheChange(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want []string
	}{
		{
			name: "a new label reports what it replaces",
			args: map[string]any{argLabel: labelAfter},
			want: []string{`Label changes from "old-label" to "after".`},
		},
		{
			name: "a label matching the VPC reports a set",
			args: map[string]any{argLabel: vpcLabelOld},
			want: []string{`Label is set to "old-label".`},
		},
		{
			name: "a description-only edit reports the replacement",
			args: map[string]any{keyDescription: "prod network"},
			want: []string{vpcDescEffect},
		},
		{
			name: "both changes are reported in order",
			args: map[string]any{argLabel: labelAfter, keyDescription: "prod network"},
			want: []string{`Label changes from "old-label" to "after".`, vpcDescEffect},
		},
		{
			name: "an empty edit reports nothing",
			args: map[string]any{},
			want: nil,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var fetched string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fetched = r.URL.Path

				if _, err := w.Write([]byte(vpcUpdateState)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			args := map[string]any{argVPCID: float64(5), keyDryRun: true}
			maps.Copy(args, testCase.args)

			request := requestWith(args)

			result, err := toolhooks.LinodeVPCUpdatePreview(t.Context(), &request,
				configFor(server.URL), http.MethodPut, vpcPath, testCase.args)
			if err != nil {
				t.Fatalf("preview: %v", err)
			}

			if fetched != vpcPath {
				t.Errorf("fetched = %q, want the VPC the change starts from", fetched)
			}

			preview := decodeVPCPreview(t, resultText(t, result))

			if preview.CurrentState == nil || preview.CurrentState.Label != vpcLabelOld {
				t.Errorf("current_state = %v, want the VPC the change replaces", preview.CurrentState)
			}

			assertSideEffects(t, preview.SideEffects, testCase.want)
		})
	}
}

// TestVPCUpdatePreviewReadsAnUnlabeledVPCAsASet pins the branch where the VPC
// carries no label: the change reads as a label being set rather than replaced.
func TestVPCUpdatePreviewReadsAnUnlabeledVPCAsASet(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`{"id":5,"label":""}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	request := requestWith(map[string]any{
		argVPCID: float64(5), argLabel: labelAfter, keyDryRun: true,
	})

	result, err := toolhooks.LinodeVPCUpdatePreview(t.Context(), &request,
		configFor(server.URL), http.MethodPut, vpcPath, nil)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	assertSideEffects(t, decodeVPCPreview(t, resultText(t, result)).SideEffects,
		[]string{`Label is set to "after".`})
}

// TestVPCUpdatePreviewReportsAnUnreadableVPC pins that a failed read reaches the
// caller as a tool error rather than a preview naming no starting point.
func TestVPCUpdatePreviewReportsAnUnreadableVPC(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	request := requestWith(map[string]any{
		argVPCID: float64(5), argLabel: labelAfter, keyDryRun: true,
	})

	result, err := toolhooks.LinodeVPCUpdatePreview(t.Context(), &request,
		configFor(server.URL), http.MethodPut, vpcPath, nil)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true for an unreadable VPC")
	}
}
