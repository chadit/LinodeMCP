package toolhooks_test

import (
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	rescuePath           = "/linode/instances/123/rescue"
	settingsPath         = "/linode/instances/123/interfaces/settings"
	argStatus            = "status"
	statusOffline        = "offline"
	rescueDefaultRouteV4 = "ipv4_interface_id"
	rescueEffect         = "The instance reboots into rescue mode; its normal boot " +
		"configuration is bypassed until you reboot out of rescue mode."
	rescueRunningWarning = "Instance is currently running; entering rescue mode " +
		"reboots it, causing downtime."
	settingsEffect = "Interface settings for Linode 123 will be updated."
)

// TestLinodeInstanceRescuePreviewWarnsOnlyWhenRunning covers both arms of the
// walk: a Linode that is up loses service to the rescue boot, and one that is
// already down has nothing to warn about.
func TestLinodeInstanceRescuePreviewWarnsOnlyWhenRunning(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status string
		want   []string
	}{
		{name: "running", status: "running", want: []string{rescueRunningWarning}},
		{name: "mixed case still running", status: "Running", want: []string{rescueRunningWarning}},
		{name: "offline", status: statusOffline, want: nil},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server := simpleWriteStateServer(t, map[string]any{argStatus: testCase.status})
			request := requestWith(map[string]any{argLinodeID: float64(123), keyDryRun: true})

			result, err := toolhooks.LinodeInstanceRescuePreview(
				t.Context(), &request, configFor(server.URL), http.MethodPost,
				rescuePath, map[string]any{},
			)
			if err != nil {
				t.Fatalf("LinodeInstanceRescuePreview: %v", err)
			}

			envelope := simpleWritePreview(t, result)
			effects, warnings := previewSentences(t, envelope)

			wantOnly(t, effects, []string{rescueEffect})
			wantOnly(t, warnings, testCase.want)
		})
	}
}

// TestLinodeInstanceRescuePreviewReportsTheInstance pins the state half: the
// preview reads the Linode the reboot is about rather than answering from the
// arguments alone.
func TestLinodeInstanceRescuePreviewReportsTheInstance(t *testing.T) {
	t.Parallel()

	server := simpleWriteStateServer(t, map[string]any{
		argSimpleLabel: simpleWriteHostLabel, argStatus: statusOffline,
	})
	request := requestWith(map[string]any{argLinodeID: float64(123), keyDryRun: true})

	result, err := toolhooks.LinodeInstanceRescuePreview(
		t.Context(), &request, configFor(server.URL), http.MethodPost,
		rescuePath, map[string]any{},
	)
	if err != nil {
		t.Fatalf("LinodeInstanceRescuePreview: %v", err)
	}

	envelope := simpleWritePreview(t, result)
	if got := simpleWriteState(t, envelope)[argSimpleLabel]; got != simpleWriteHostLabel {
		t.Errorf("current_state label = %v, want %v", got, simpleWriteHostLabel)
	}
}
