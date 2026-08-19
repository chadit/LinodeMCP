package tools_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// Case names the package has no constant for yet.
const (
	caseAbsent       = "absent"
	caseExplicitNull = "explicit null"
)

// The sentence every unusable public flag answers, and the any-of sentence the
// OAuth client update declares.
const (
	errPublicBoolean = keyInterfacePublic + " must be a boolean"
	errOAuthAnyOf    = "at least one of label, redirect_uri, or public is required"
)

// TestPresentTextArgumentRequiredRefusesEveryUnusableShape pins the widening the
// retired hooks relied on: a text field sent as a number or as spaces reads the
// same as one never sent, because none of them carry a usable value.
func TestPresentTextArgumentRequiredRefusesEveryUnusableShape(t *testing.T) {
	t.Parallel()

	cases := []struct {
		arguments map[string]any
		name      string
		want      string
	}{
		{name: caseAbsent, arguments: map[string]any{}, want: errLabelRequired},
		{name: caseExplicitNull, arguments: map[string]any{keyLabel: nil}, want: errLabelRequired},
		{name: caseNumber, arguments: map[string]any{keyLabel: 7}, want: errLabelRequired},
		{name: "a boolean", arguments: map[string]any{keyLabel: true}, want: errLabelRequired},
		{name: caseEmpty, arguments: map[string]any{keyLabel: ""}, want: errLabelRequired},
		{name: caseBlank, arguments: map[string]any{keyLabel: blankString}, want: errLabelRequired},
		{name: "usable text", arguments: map[string]any{keyLabel: "oauth-app"}, want: ""},
		{name: "text the caller padded", arguments: map[string]any{keyLabel: "  oauth-app  "}, want: ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := tools.PresentTextArgument(bodyRequest(testCase.arguments), keyLabel, true)
			if got != testCase.want {
				t.Errorf("PresentTextArgument = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestPresentTextArgumentOptionalRefusesOnlyWhatWasSent covers the `optional`
// field, where saying nothing is a legal request and only a supplied value has
// to be usable.
func TestPresentTextArgumentOptionalRefusesOnlyWhatWasSent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		arguments map[string]any
		name      string
		want      string
	}{
		{name: caseAbsent, arguments: map[string]any{}, want: ""},
		{name: caseExplicitNull, arguments: map[string]any{keyLabel: nil}, want: errLabelRequired},
		{name: caseNumber, arguments: map[string]any{keyLabel: 7}, want: errLabelRequired},
		{name: caseEmpty, arguments: map[string]any{keyLabel: ""}, want: errLabelRequired},
		{name: caseBlank, arguments: map[string]any{keyLabel: blankString}, want: errLabelRequired},
		{name: "usable text", arguments: map[string]any{keyLabel: "renamed-app"}, want: ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := tools.PresentTextArgument(bodyRequest(testCase.arguments), keyLabel, false)
			if got != testCase.want {
				t.Errorf("PresentTextArgument = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestPresentBoolArgumentRequiredReadsPresenceNotTruth pins why the flag is read
// off the argument map: an implicit-presence bool decodes absent and false
// alike, and false is a real answer the caller is entitled to give.
func TestPresentBoolArgumentRequiredReadsPresenceNotTruth(t *testing.T) {
	t.Parallel()

	cases := []struct {
		arguments map[string]any
		name      string
		want      string
	}{
		{name: caseAbsent, arguments: map[string]any{}, want: errPublicBoolean},
		{name: caseExplicitNull, arguments: map[string]any{keyInterfacePublic: nil}, want: errPublicBoolean},
		{name: "the word yes", arguments: map[string]any{keyInterfacePublic: phase2NonBoolean}, want: errPublicBoolean},
		{name: caseNumber, arguments: map[string]any{keyInterfacePublic: 1}, want: errPublicBoolean},
		{name: caseFalse, arguments: map[string]any{keyInterfacePublic: false}, want: ""},
		{name: "true", arguments: map[string]any{keyInterfacePublic: true}, want: ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := tools.PresentBoolArgument(bodyRequest(testCase.arguments), keyInterfacePublic, true)
			if got != testCase.want {
				t.Errorf("PresentBoolArgument = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestPresentBoolArgumentOptionalRefusesOnlyWhatWasSent covers the `optional`
// flag, where an absent flag leaves the setting alone.
func TestPresentBoolArgumentOptionalRefusesOnlyWhatWasSent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		arguments map[string]any
		name      string
		want      string
	}{
		{name: caseAbsent, arguments: map[string]any{}, want: ""},
		{name: caseExplicitNull, arguments: map[string]any{keyInterfacePublic: nil}, want: errPublicBoolean},
		{name: "the word yes", arguments: map[string]any{keyInterfacePublic: phase2NonBoolean}, want: errPublicBoolean},
		{name: caseFalse, arguments: map[string]any{keyInterfacePublic: false}, want: ""},
		{name: "true", arguments: map[string]any{keyInterfacePublic: true}, want: ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := tools.PresentBoolArgument(bodyRequest(testCase.arguments), keyInterfacePublic, false)
			if got != testCase.want {
				t.Errorf("PresentBoolArgument = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestRequireAnyArgumentAsksOnlyWhetherOneArrived pins that presence alone
// satisfies the any-of rule: an update sends a field's zero to clear it, so a
// null or an empty list is a change the caller meant to make.
func TestRequireAnyArgumentAsksOnlyWhetherOneArrived(t *testing.T) {
	t.Parallel()

	cases := []struct {
		arguments map[string]any
		name      string
		want      string
	}{
		{name: "no field named", arguments: map[string]any{}, want: errOAuthAnyOf},
		{
			name:      "only fields outside the list",
			arguments: map[string]any{keyConfirm: true, keyDryRun: true},
			want:      errOAuthAnyOf,
		},
		{name: "the first name", arguments: map[string]any{keyLabel: "renamed-app"}, want: ""},
		{name: "the last name", arguments: map[string]any{keyInterfacePublic: false}, want: ""},
		{name: "a name sent as null", arguments: map[string]any{keyRedirectURI: nil}, want: ""},
		{name: "a name sent as an empty list", arguments: map[string]any{keyRedirectURI: []any{}}, want: ""},
		{name: "a name sent unusable", arguments: map[string]any{keyLabel: 7}, want: ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := tools.RequireAnyArgument(
				bodyRequest(testCase.arguments), errOAuthAnyOf,
				keyLabel, keyRedirectURI, keyInterfacePublic,
			)
			if got != testCase.want {
				t.Errorf("RequireAnyArgument = %q, want %q", got, testCase.want)
			}
		})
	}
}
