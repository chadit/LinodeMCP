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
	unsupportedSettings     = "Unsupported account settings field(s): {fields}"
	settingNotString        = "object_storage must be a string"
	linodesShape            = "linodes must be a non-empty array of distinct positive integer Linode IDs"
	linodesArray            = "linodes must be a JSON array of positive integer Linode IDs"
)

// The two whole-map refusals and the string reader that took over from the
// managed, database, and account-settings hooks. The cases are the ones those
// hooks were held to, so the sentences a caller reads have not moved.
func TestRefusedArgumentsNamesEverySuppliedMemberSorted(t *testing.T) {
	t.Parallel()

	const readOnly = "Read-only fields are not accepted: {fields}"

	tests := map[string]struct {
		arguments map[string]any
		want      string
	}{
		"none supplied": {arguments: map[string]any{managedServiceLabelParam: tagWeb}, want: ""},
		"one supplied": {
			arguments: map[string]any{keySupportTicketID: 3},
			want:      "Read-only fields are not accepted: id",
		},
		"both supplied, sorted": {
			arguments: map[string]any{"last_decrypted": "x", keySupportTicketID: 3},
			want:      "Read-only fields are not accepted: id, last_decrypted",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			request := requestFor(test.arguments)
			got := tools.RefusedArguments(&request, readOnly, keySupportTicketID, "last_decrypted")

			if got != test.want {
				t.Errorf("message = %q, want %q", got, test.want)
			}
		})
	}
}

// A sentence naming the members in prose takes neither placeholder, which is
// how the contact create keeps the one wording it has always answered.
func TestRefusedArgumentsLeavesAProseSentenceAlone(t *testing.T) {
	t.Parallel()

	const sentence = "id and updated are read-only and cannot be set when creating a managed contact"

	request := requestFor(map[string]any{"updated": "now"})
	if got := tools.RefusedArguments(&request, sentence, "id", "updated"); got != sentence {
		t.Errorf("message = %q, want %q", got, sentence)
	}
}

// The database routes name the first undeclared argument and the account
// settings name them all, which is the whole difference between the two
// placeholders.
func TestUnknownArgumentsFillsTheDeclaredPlaceholder(t *testing.T) {
	t.Parallel()

	declared := []string{keyEnvironment, keyConfirm, keyDryRun, tcBackupsEnabled, keyObjectStorageSetting}

	tests := map[string]struct {
		arguments map[string]any
		sentence  string
		want      string
	}{
		"every argument declared": {
			arguments: map[string]any{tcBackupsEnabled: true, keyConfirm: true},
			sentence:  unsupportedSettings,
			want:      "",
		},
		"system params are declared too": {
			arguments: map[string]any{keyDryRun: true, keyEnvironment: "default"},
			sentence:  unsupportedSettings,
			want:      "",
		},
		"one unknown": {
			arguments: map[string]any{tcBackupsEnabled: true, "bogus": "x"},
			sentence:  unsupportedSettings,
			want:      "Unsupported account settings field(s): bogus",
		},
		"several unknown, sorted": {
			arguments: map[string]any{unknownZebra: 1, stageAlpha: 2},
			sentence:  unsupportedSettings,
			want:      "Unsupported account settings field(s): alpha, zebra",
		},
		"the first unknown alone": {
			arguments: map[string]any{unknownZebra: 1, stageAlpha: 2},
			sentence:  "unsupported argument: {field}",
			want:      "unsupported argument: alpha",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			request := requestFor(test.arguments)
			got := tools.UnknownArguments(&request, test.sentence, declared...)

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
