package toolhooks_test

import (
	"maps"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	stragglerFirewallPath = "/networking/firewalls/123"
	stragglerDisabled     = "disabled"
)

// TestFirewallUpdatePreviewDiffsAgainstTheFirewall pins both sentences and the
// quoting, which is Go's %q in each language.
func TestFirewallUpdatePreviewDiffsAgainstTheFirewall(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args  map[string]any
		state string
		want  []string
	}{
		"both named": {
			state: `{"id":123,"label":"web-fw","status":"enabled"}`,
			args:  map[string]any{keyLabel: "renamed-fw", keyStatus: stragglerDisabled},
			want: []string{
				`Label changes from "web-fw" to "renamed-fw".`,
				`Firewall status changes to "disabled"; this immediately stops enforcing its rules.`,
			},
		},
		"enabling reports what starts": {
			state: `{"id":123,"label":"web-fw","status":"disabled"}`,
			args:  map[string]any{keyStatus: "enabled"},
			want: []string{
				`Firewall status changes to "enabled"; this immediately starts enforcing its rules.`,
			},
		},
		"a label the firewall never had reports a set": {
			state: `{"id":123,"status":"enabled"}`,
			args:  map[string]any{keyLabel: "renamed-fw"},
			want:  []string{`Label is set to "renamed-fw".`},
		},
		"an edit naming nothing reports nothing": {
			state: `{"id":123,"label":"web-fw","status":"enabled"}`,
			args:  map[string]any{},
			want:  nil,
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server, fetched := nbStubAPI(t, testCase.state)

			args := map[string]any{"firewall_id": float64(123), keyDryRun: true}
			maps.Copy(args, testCase.args)

			request := requestWith(args)

			result, err := toolhooks.LinodeFirewallUpdatePreview(t.Context(), &request,
				configFor(server.URL), http.MethodPut, stragglerFirewallPath, map[string]any{})
			if err != nil {
				t.Fatalf("preview: %v", err)
			}

			if *fetched != stragglerFirewallPath {
				t.Errorf("fetched = %q, want the firewall the update changes", *fetched)
			}

			preview := nbDecodePreview(t, resultText(t, result))
			if len(preview.SideEffects) != len(testCase.want) {
				t.Fatalf("side_effects = %v, want %v", preview.SideEffects, testCase.want)
			}

			for index, want := range testCase.want {
				if preview.SideEffects[index] != want {
					t.Errorf("side_effects[%d] = %q, want %q", index, preview.SideEffects[index], want)
				}
			}
		})
	}
}
