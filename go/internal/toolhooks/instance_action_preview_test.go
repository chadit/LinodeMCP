package toolhooks_test

import (
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	actionInstancePath   = "/linode/instances/123"
	actionMigratePath    = actionInstancePath + "/migrate"
	actionMutatePath     = actionInstancePath + "/mutate"
	actionRestorePath    = actionInstancePath + "/backups/456/restore"
	actionFromRegion     = "us-east"
	actionTargetRegion   = "us-west"
	actionFromType       = "g6-standard-1"
	actionBackupLabel    = "nightly"
	argBackupID          = "backup_id"
	argTargetLinodeID    = "target_linode_id"
	argOverwrite         = "overwrite"
	actionMigrateBothWay = "Instance migrates from region us-east to us-west; " +
		"it is unavailable during the migration."
)

// previewSentences reads the side effects and warnings one preview reported, so
// a case asserts on the prose the hook owns rather than on rendered JSON.
func previewSentences(t *testing.T, envelope map[string]any) ([]any, []any) {
	t.Helper()

	effects, isList := envelope["side_effects"].([]any)
	if !isList {
		t.Fatalf("side_effects = %v, want a list", envelope["side_effects"])
	}

	warnings, isList := envelope["warnings"].([]any)
	if !isList {
		t.Fatalf("warnings = %v, want a list", envelope["warnings"])
	}

	return effects, warnings
}

// wantOnly holds a preview's prose to exactly the sentences a case expects,
// which is what keeps a hook from quietly gaining or losing one.
func wantOnly(t *testing.T, got []any, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %v, want %q", i, got[i], want[i])
		}
	}
}

// TestLinodeInstanceMigratePreviewWordsEveryDestination covers the three
// sentences the move can be described by: both regions known, only the target
// known, and neither, which is a caller letting Linode pick from an instance the
// read answered nothing about.
func TestLinodeInstanceMigratePreviewWordsEveryDestination(t *testing.T) {
	t.Parallel()

	cases := []struct {
		state  map[string]any
		name   string
		region string
		want   string
	}{
		{
			name:   "both regions known",
			state:  map[string]any{argRegion: actionFromRegion},
			region: actionTargetRegion,
			want:   actionMigrateBothWay,
		},
		{
			name:   "target alone",
			state:  map[string]any{},
			region: actionTargetRegion,
			want: "Instance migrates to region us-west; " +
				"it is unavailable during the migration.",
		},
		{
			name:   "Linode picks the destination",
			state:  map[string]any{argRegion: actionFromRegion},
			region: "",
			want:   "Instance migrates; it is unavailable during the migration.",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server := simpleWriteStateServer(t, testCase.state)
			arguments := map[string]any{argLinodeID: float64(123), keyDryRun: true}

			if testCase.region != "" {
				arguments[argRegion] = testCase.region
			}

			request := requestWith(arguments)

			result, err := toolhooks.LinodeInstanceMigratePreview(
				t.Context(), &request, configFor(server.URL), http.MethodPost,
				actionMigratePath, map[string]any{argRegion: testCase.region},
			)
			if err != nil {
				t.Fatalf("LinodeInstanceMigratePreview: %v", err)
			}

			envelope := simpleWritePreview(t, result)
			effects, warnings := previewSentences(t, envelope)

			wantOnly(t, effects, []string{testCase.want})
			wantOnly(t, warnings, nil)
		})
	}
}

// TestLinodeInstanceMigratePreviewReportsTheInstance pins the state half: the
// preview reads the Linode the move is about, not just the arguments.
func TestLinodeInstanceMigratePreviewReportsTheInstance(t *testing.T) {
	t.Parallel()

	server := simpleWriteStateServer(t, map[string]any{
		argSimpleLabel: simpleWriteHostLabel, argRegion: actionFromRegion,
	})
	request := requestWith(map[string]any{
		argLinodeID: float64(123), argRegion: actionTargetRegion, keyDryRun: true,
	})

	result, err := toolhooks.LinodeInstanceMigratePreview(
		t.Context(), &request, configFor(server.URL), http.MethodPost,
		actionMigratePath, map[string]any{argRegion: actionTargetRegion},
	)
	if err != nil {
		t.Fatalf("LinodeInstanceMigratePreview: %v", err)
	}

	envelope := simpleWritePreview(t, result)
	if got := simpleWriteState(t, envelope)[argSimpleLabel]; got != simpleWriteHostLabel {
		t.Errorf("current_state label = %v, want %v", got, simpleWriteHostLabel)
	}
}

// TestLinodeInstanceMutatePreviewCarriesBothHalves covers the union this hook
// landed as: Go's preview named the type and Python's named the downtime, so the
// warning stands whether or not the read answered with a type.
func TestLinodeInstanceMutatePreviewCarriesBothHalves(t *testing.T) {
	t.Parallel()

	cases := []struct {
		state map[string]any
		name  string
		want  string
	}{
		{
			name:  "type known",
			state: map[string]any{"type": actionFromType},
			want: "Instance type g6-standard-1 upgrades to the latest generation; " +
				"it reboots during the upgrade.",
		},
		{
			name:  "type unknown",
			state: map[string]any{},
			want: "Instance upgrades to the latest generation of its type; " +
				"it reboots during the upgrade.",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server := simpleWriteStateServer(t, testCase.state)
			request := requestWith(map[string]any{argLinodeID: float64(123), keyDryRun: true})

			result, err := toolhooks.LinodeInstanceMutatePreview(
				t.Context(), &request, configFor(server.URL), http.MethodPost,
				actionMutatePath, map[string]any{},
			)
			if err != nil {
				t.Fatalf("LinodeInstanceMutatePreview: %v", err)
			}

			envelope := simpleWritePreview(t, result)
			effects, warnings := previewSentences(t, envelope)

			wantOnly(t, effects, []string{testCase.want})
			wantOnly(t, warnings, []string{"The Linode may be unavailable during the upgrade."})
		})
	}
}
