package toolhooks_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	foldFirewallsPath = "/networking/firewalls"
	foldInstancesPath = "/linode/instances"
	foldFirewallID    = "firewall_id"
	foldRegion        = "region"
	foldImage         = "image"
	foldRegionEast    = "us-east"
	foldNanode        = "g6-nanode-1"
	foldBillingWarn   = "Billing for the instance starts immediately on creation."
	foldFirewallLabel = "web-fw"
	foldPolicyDrop    = "DROP"
)

// TestFirewallCreatePreviewNamesThePoliciesTheFoldSends pins that the sentence
// reports the same defaults the body folds into its rules object, so a caller
// who named no policy is told which one the create would carry.
func TestFirewallCreatePreviewNamesThePoliciesTheFoldSends(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args map[string]any
		want string
	}{
		"neither named": {
			args: map[string]any{keyLabel: foldFirewallLabel, keyDryRun: true},
			want: `A new Cloud Firewall "web-fw" will be created with inbound policy ACCEPT and outbound policy ACCEPT.`,
		},
		"inbound named": {
			args: map[string]any{keyLabel: foldFirewallLabel, "inbound_policy": foldPolicyDrop, keyDryRun: true},
			want: `A new Cloud Firewall "web-fw" will be created with inbound policy DROP and outbound policy ACCEPT.`,
		},
		"both named": {
			args: map[string]any{
				keyLabel: "edge", "inbound_policy": foldPolicyDrop, "outbound_policy": foldPolicyDrop, keyDryRun: true,
			},
			want: `A new Cloud Firewall "edge" will be created with inbound policy DROP and outbound policy DROP.`,
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server, fetched := nbStubAPI(t, `{}`)
			request := requestWith(testCase.args)

			result, err := toolhooks.LinodeFirewallCreatePreview(t.Context(), &request,
				configFor(server.URL), http.MethodPost, foldFirewallsPath, nil)
			if err != nil {
				t.Fatalf("preview: %v", err)
			}

			if *fetched != "" {
				t.Errorf("fetched = %q, want no request: the firewall does not exist yet", *fetched)
			}

			preview := nbDecodePreview(t, resultText(t, result))
			if len(preview.SideEffects) != 1 || preview.SideEffects[0] != testCase.want {
				t.Errorf("side_effects = %v, want [%q]", preview.SideEffects, testCase.want)
			}

			if preview.WouldExecute.Path != foldFirewallsPath {
				t.Errorf("would_execute path = %q, want %q", preview.WouldExecute.Path, foldFirewallsPath)
			}
		})
	}
}

// TestFirewallCreatePreviewCancelsWithTheContext pins that a preview nobody is
// waiting for stops rather than describing the firewall into a closed
// connection.
func TestFirewallCreatePreviewCancelsWithTheContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	request := requestWith(map[string]any{keyLabel: foldFirewallLabel, keyDryRun: true})

	result, err := toolhooks.LinodeFirewallCreatePreview(ctx, &request,
		configFor("http://127.0.0.1:1"), http.MethodPost, foldFirewallsPath, nil)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want the canceled walk reported")
	}

	if text := resultText(t, result); !strings.Contains(text, "canceled") {
		t.Errorf("error text = %q, want the cancellation named", text)
	}
}

// TestInstanceCreatePreviewDescribesTheInstanceAndItsBilling pins both halves of
// the walk: the image is named only when the caller chose one, and the billing
// warning rides on every answer.
func TestInstanceCreatePreviewDescribesTheInstanceAndItsBilling(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args map[string]any
		want string
	}{
		"no image": {
			args: map[string]any{
				foldRegion: foldRegionEast, keyType: foldNanode, foldFirewallID: float64(123), keyDryRun: true,
			},
			want: "A new g6-nanode-1 instance will be created in region us-east.",
		},
		"with image": {
			args: map[string]any{
				foldRegion: foldRegionEast, keyType: foldNanode, foldFirewallID: float64(123),
				foldImage: publicImageID, keyDryRun: true,
			},
			want: "A new g6-nanode-1 instance will be created in region us-east from image linode/debian11.",
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server, fetched := nbStubAPI(t, `{}`)
			request := requestWith(testCase.args)

			result, err := toolhooks.LinodeInstanceCreatePreview(t.Context(), &request,
				configFor(server.URL), http.MethodPost, foldInstancesPath, nil)
			if err != nil {
				t.Fatalf("preview: %v", err)
			}

			if *fetched != "" {
				t.Errorf("fetched = %q, want no request: the instance does not exist yet", *fetched)
			}

			preview := nbDecodePreview(t, resultText(t, result))
			if len(preview.SideEffects) != 1 || preview.SideEffects[0] != testCase.want {
				t.Errorf("side_effects = %v, want [%q]", preview.SideEffects, testCase.want)
			}

			if len(preview.Warnings) != 1 || preview.Warnings[0] != foldBillingWarn {
				t.Errorf("warnings = %v, want [%q]", preview.Warnings, foldBillingWarn)
			}
		})
	}
}

// TestInstanceCreatePreviewCancelsWithTheContext pins that the billing walk
// stops with the caller rather than reporting into a closed connection.
func TestInstanceCreatePreviewCancelsWithTheContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	request := requestWith(map[string]any{
		foldRegion: foldRegionEast, keyType: foldNanode, foldFirewallID: float64(123), keyDryRun: true,
	})

	result, err := toolhooks.LinodeInstanceCreatePreview(ctx, &request,
		configFor("http://127.0.0.1:1"), http.MethodPost, foldInstancesPath, nil)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want the canceled walk reported")
	}

	if text := resultText(t, result); !strings.Contains(text, "canceled") {
		t.Errorf("error text = %q, want the cancellation named", text)
	}
}
