package tools_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The sentences and argument names these cases repeat, named once so the
// package's constant scan reads one occurrence rather than several.
const (
	keyObjectStorageSetting = "object_storage"
	unknownZebra            = "zebra"
	keyEngineIDArgument     = "engine_id"
	settingNotString        = "object_storage must be a string"
	linodesShape            = "linodes must be a non-empty array of distinct positive integer Linode IDs"
	linodesArray            = "linodes must be a JSON array of positive integer Linode IDs"
)

// The contract messages and tool names the refusal cases run against, named
// once because each case spells a message and the tool declared on it.
const (
	credentialUpdateInput = "linode.mcp.v1.ManagedCredentialUpdateInput"
	credentialUpdateTool  = "linode_managed_credential_update"
	contactCreateInput    = "linode.mcp.v1.ManagedContactCreateInput"
	contactCreateTool     = "linode_managed_contact_create"
	settingsUpdateInput   = "linode.mcp.v1.AccountSettingsUpdateInput"
	settingsUpdateTool    = "linode_account_settings_update"
)

// The whole-map refusals that took over from the managed, database, and
// account-settings hooks. Both come off the descriptor now, so the cases name
// real contract messages rather than a list written here.
func TestCheckArgumentRefusalsAnswersTheDeclaredRefusalFirst(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		arguments map[string]any
		message   string
		tool      string
		want      string
	}{
		"a declared refusal names every supplied member, sorted": {
			message:   credentialUpdateInput,
			tool:      credentialUpdateTool,
			arguments: map[string]any{"last_decrypted": "x", keySupportTicketID: 3},
			want:      "Read-only fields are not accepted: id, last_decrypted",
		},
		"a prose sentence stands as written": {
			message:   contactCreateInput,
			tool:      contactCreateTool,
			arguments: map[string]any{"updated": "now"},
			want:      "id and updated are read-only and cannot be set when creating a managed contact",
		},
		"a declared refusal beats an undeclared name": {
			message:   credentialUpdateInput,
			tool:      credentialUpdateTool,
			arguments: map[string]any{keySupportTicketID: 3, unknownZebra: 1},
			want:      "Read-only fields are not accepted: id",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tools.CheckArgumentRefusals(test.message, test.tool, test.arguments)
			if got != test.want {
				t.Errorf("message = %q, want %q", got, test.want)
			}
		})
	}
}

// Every tool refuses a name its input message does not declare, in the engine's
// own words. The three control names pass because the server and the destroy
// gate read them off the argument map and no message declares them, and no
// behavior fixture sends yolo, so this is the only thing holding that entry.
func TestCheckArgumentRefusalsAnswersForAnUndeclaredName(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		arguments map[string]any
		message   string
		want      string
	}{
		"every argument declared": {
			message:   settingsUpdateInput,
			arguments: map[string]any{tcBackupsEnabled: true, keyConfirm: true},
			want:      "",
		},
		"system params are declared too": {
			message:   settingsUpdateInput,
			arguments: map[string]any{keyDryRun: true, keyEnvironment: "default"},
			want:      "",
		},
		"the engine control names pass": {
			message: settingsUpdateInput,
			arguments: map[string]any{
				"confirm_bypass_dry_run": true, "confirmed_dry_run": true, "yolo": true,
			},
			want: "",
		},
		"one unknown": {
			message:   settingsUpdateInput,
			arguments: map[string]any{tcBackupsEnabled: true, keyBogus: "x"},
			want:      "Unsupported argument(s) for linode_account_settings_update: bogus",
		},
		"several unknown, sorted": {
			message:   settingsUpdateInput,
			arguments: map[string]any{unknownZebra: 1, stageAlpha: 2},
			want:      "Unsupported argument(s) for linode_account_settings_update: alpha, zebra",
		},
		"a message the registry does not carry refuses nothing": {
			message:   "linode.mcp.v1.NoSuchInput",
			arguments: map[string]any{unknownZebra: 1},
			want:      "",
		},
		"a name the registry carries as something other than a message": {
			message:   "linode.mcp.v1.FieldLocation",
			arguments: map[string]any{unknownZebra: 1},
			want:      "",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tools.CheckArgumentRefusals(test.message, settingsUpdateTool, test.arguments)
			if got != test.want {
				t.Errorf("message = %q, want %q", got, test.want)
			}
		})
	}
}

// The string member accepts a blank where its text sibling refuses one: no rule
// on account settings holds object_storage to a value, so a blank one travels
// to the route, and a reader that refused it would refuse that call here.
func TestPresentStringArgumentAcceptsABlankAndRefusesANonString(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		arguments map[string]any
		want      string
		required  bool
	}{
		"a value":                {arguments: map[string]any{keyObjectStorageSetting: statusActive}, want: ""},
		"a blank value":          {arguments: map[string]any{keyObjectStorageSetting: ""}, want: ""},
		"absent and optional":    {arguments: map[string]any{}, want: ""},
		"absent and required":    {arguments: map[string]any{}, required: true, want: settingNotString},
		"a number":               {arguments: map[string]any{keyObjectStorageSetting: 5}, want: settingNotString},
		"an explicit null value": {arguments: map[string]any{keyObjectStorageSetting: nil}, want: settingNotString},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			request := requestFor(test.arguments)
			got := tools.PresentStringArgument(&request, keyObjectStorageSetting, test.required)

			if got != test.want {
				t.Errorf("message = %q, want %q", got, test.want)
			}
		})
	}
}

// The id-list member's three arms, which are a convergence: Go answered five
// sentences for the placement membership list and Python one. A member is what
// it accepts and which refusals it tells apart, so empty, non-positive and
// repeated all reach the arm that names what the list must be.
func TestIDListArgumentTellsAbsentFromUnusableFromRefused(t *testing.T) {
	t.Parallel()

	const (
		absent   = "linodes is required"
		notList  = "linodes must be a JSON array of positive integers"
		badShape = "linodes must be a non-empty array of distinct positive integers"
	)

	tests := map[string]struct {
		arguments map[string]any
		want      string
	}{
		"nothing sent":     {arguments: map[string]any{}, want: absent},
		"sent as text":     {arguments: map[string]any{keyPlacementGroupLinodes: "123"}, want: notList},
		"sent as a number": {arguments: map[string]any{keyPlacementGroupLinodes: float64(123)}, want: notList},
		"an explicit null": {arguments: map[string]any{keyPlacementGroupLinodes: nil}, want: notList},
		"an empty list":    {arguments: map[string]any{keyPlacementGroupLinodes: []any{}}, want: badShape},
		"a zero entry":     {arguments: map[string]any{keyPlacementGroupLinodes: []any{float64(0)}}, want: badShape},
		"a negative entry": {arguments: map[string]any{keyPlacementGroupLinodes: []any{float64(-1)}}, want: badShape},
		"a fractional entry": {
			arguments: map[string]any{keyPlacementGroupLinodes: []any{123.5}}, want: badShape,
		},
		"an entry sent as text": {
			arguments: map[string]any{keyPlacementGroupLinodes: []any{"123"}}, want: badShape,
		},
		"a repeated entry": {
			arguments: map[string]any{keyPlacementGroupLinodes: []any{float64(1), float64(1)}}, want: badShape,
		},
		"one id":  {arguments: map[string]any{keyPlacementGroupLinodes: []any{float64(1)}}, want: ""},
		"two ids": {arguments: map[string]any{keyPlacementGroupLinodes: []any{float64(1), float64(2)}}, want: ""},
		"ids as whole ints": {
			arguments: map[string]any{keyPlacementGroupLinodes: []any{1, int64(2)}}, want: "",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			request := requestFor(test.arguments)
			if got := tools.IDListArgument(&request, keyPlacementGroupLinodes); got != test.want {
				t.Errorf("message = %q, want %q", got, test.want)
			}
		})
	}
}

// A declaration words each arm, which is how the two membership routes keep the
// Linode-ID phrasing their callers have always read.
func TestDeclaredIDListAnswersTheDeclaredSentences(t *testing.T) {
	t.Parallel()

	request := requestFor(map[string]any{keyPlacementGroupLinodes: []any{float64(1), float64(1)}})

	got := tools.DeclaredIDList(&request, keyPlacementGroupLinodes,
		"linodes is required",
		linodesArray,
		linodesShape)

	if got != linodesShape {
		t.Errorf("message = %q, want the declared refused sentence", got)
	}
}

// Each member has a plain spelling beside its worded one, which the emitter
// writes for a declaration that words no arm. Nothing in the contract words
// nothing today, so these drive the plain forms directly rather than leaving
// them to ship untested behind a declaration that does not exist yet.
func TestPlainReaderSpellingsAnswerTheMembersOwnSentences(t *testing.T) {
	t.Parallel()

	present := requestFor(map[string]any{})

	if got := tools.PresentStringArgument(&present, keyObjectStorageSetting, true); got != settingNotString {
		t.Errorf("PresentStringArgument = %q, want the member's own sentence", got)
	}

	if got := tools.IDListArgument(&present, keyPlacementGroupLinodes); got != "linodes is required" {
		t.Errorf("IDListArgument = %q, want the member's own sentence", got)
	}

	_, message := tools.FreeTextArgument(&present, keyEngineIDArgument)
	if message != "engine_id is required" {
		t.Errorf("FreeTextArgument = %q, want the member's own sentence", message)
	}

	sent := requestFor(map[string]any{keyEngineIDArgument: "mysql/8"})
	if value, message := tools.FreeTextArgument(&sent, keyEngineIDArgument); value != "mysql/8" || message != "" {
		t.Errorf("FreeTextArgument = (%q, %q), want the value it was handed", value, message)
	}

	blank := requestFor(map[string]any{keyEngineIDArgument: "  "})
	if _, message := tools.FreeTextArgument(&blank, keyEngineIDArgument); message != "engine_id must be a non-empty string" {
		t.Errorf("FreeTextArgument = %q, want the blank sentence", message)
	}
}
