package tools_test

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The arms a declaration words, spelled once so each case reads as the sentence
// a caller is answered rather than as a literal repeated per row.
const (
	declaredAbsent   = "id must be a positive integer"
	declaredRefused  = "device id must be a positive integer"
	declaredUnusable = "label must be a non-empty string"
	declaredChoice   = "type must be one of: ipv4"
	// The case name three id rows share.
	caseWordedAbsent = "absent answers the worded arm"
)

// requestFor builds a call carrying one argument map.
func requestFor(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

// TestDeclaredIDArgumentAnswersTheWordedArm covers the reason reader_message
// exists on an id: the tools that answer one sentence to a missing id and to
// one no id can hold word the absent arm the way the refusal already reads.
func TestDeclaredIDArgumentAnswersTheWordedArm(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args    map[string]any
		name    string
		absent  string
		refused string
		want    string
	}{
		{
			name:   caseWordedAbsent,
			args:   map[string]any{},
			absent: declaredAbsent,
			want:   declaredAbsent,
		},
		{
			name:   "unusable still answers the reader's own",
			args:   map[string]any{keySupportTicketID: "abc"},
			absent: declaredAbsent,
			want:   "id must be a positive integer",
		},
		{
			name:    "a worded refusal renames the argument",
			args:    map[string]any{keySupportTicketID: 0},
			refused: declaredRefused,
			want:    declaredRefused,
		},
		{
			name: "an unworded declaration reads as it always did",
			args: map[string]any{},
			want: "id is required",
		},
		{
			name: "a usable id is accepted",
			args: map[string]any{keySupportTicketID: 7},
			want: "",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := requestFor(testCase.args)

			_, message := tools.DeclaredIDArgument(&request, keySupportTicketID, 0, testCase.absent, testCase.refused)
			if message != testCase.want {
				t.Errorf("DeclaredIDArgument = %q, want %q", message, testCase.want)
			}
		})
	}
}

// TestDeclaredPresentTextTellsAbsentFromUnusable covers the split a single
// PRESENT_TEXT sentence cannot make: a label nobody sent reads as missing while
// one sent as a number reads as unreadable.
func TestDeclaredPresentTextTellsAbsentFromUnusable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args map[string]any
		name string
		want string
	}{
		{name: "absent keeps the reader's own", args: map[string]any{}, want: errLabelRequired},
		{name: "non-text answers the worded arm", args: map[string]any{managedServiceLabelParam: 5}, want: declaredUnusable},
		{name: "blank answers the worded arm", args: map[string]any{managedServiceLabelParam: "  "}, want: declaredUnusable},
		{name: "usable text is accepted", args: map[string]any{managedServiceLabelParam: tagWeb}, want: ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := requestFor(testCase.args)

			got := tools.DeclaredPresentText(&request, managedServiceLabelParam, true, "", declaredUnusable)
			if got != testCase.want {
				t.Errorf("DeclaredPresentText = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestDeclaredPresentBoolWordsTheMissingFlag covers the allocate tools' split: a
// caller who sent no flag is told the flag is missing, where one who sent
// something else is told what a flag is.
func TestDeclaredPresentBoolWordsTheMissingFlag(t *testing.T) {
	t.Parallel()

	const wantPublicRequired = "public is required"

	cases := []struct {
		args map[string]any
		name string
		want string
	}{
		{name: caseWordedAbsent, args: map[string]any{}, want: wantPublicRequired},
		{name: "non-boolean keeps the reader's own", args: map[string]any{"public": "yes"}, want: "public must be a boolean"},
		{name: "false is a flag", args: map[string]any{"public": false}, want: ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := requestFor(testCase.args)

			got := tools.DeclaredPresentBool(&request, "public", true, wantPublicRequired, "")
			if got != testCase.want {
				t.Errorf("DeclaredPresentBool = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestDeclaredMemberChoiceWordsAOneMemberVocabulary covers what the derived
// sentence cannot say: a vocabulary of one reads as "must be <a>" unless the
// declaration keeps the list wording its callers have always been answered.
func TestDeclaredMemberChoiceWordsAOneMemberVocabulary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args    map[string]any
		name    string
		refused string
		want    string
	}{
		{
			name:    "outside the vocabulary answers the worded arm",
			args:    map[string]any{keyType: "ipv6-range"},
			refused: declaredChoice,
			want:    declaredChoice,
		},
		{
			name: "an unworded refusal reads as the derived sentence",
			args: map[string]any{keyType: "ipv6-range"},
			want: "type must be ipv4",
		},
		{
			name: "a named member is accepted",
			args: map[string]any{keyType: "ipv4"},
			want: "",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := requestFor(testCase.args)

			_, message := tools.DeclaredMemberChoice(
				&request, keyType, true, "type must be a non-empty string", testCase.refused, "ipv4",
			)
			if message != testCase.want {
				t.Errorf("DeclaredMemberChoice = %q, want %q", message, testCase.want)
			}
		})
	}
}

// TestDeclaredFragmentSafeArgumentRefusesTheFragmentMarker covers what separates
// the two segment members: an OAuth client id may carry a '#' and the ids this
// one addresses may not.
func TestDeclaredFragmentSafeArgumentRefusesTheFragmentMarker(t *testing.T) {
	t.Parallel()

	const (
		refusal        = "type_id must not contain '/', '?', '#', or '..'"
		wantTypeIDText = "type_id must be a non-empty string"
	)

	cases := []struct {
		args map[string]any
		name string
		want string
	}{
		{name: caseWordedAbsent, args: map[string]any{}, want: wantTypeIDText},
		{name: "non-text answers the unusable arm", args: map[string]any{databaseTypeIDParam: 5}, want: wantTypeIDText},
		{name: "a fragment marker is refused", args: map[string]any{databaseTypeIDParam: "g6#1"}, want: refusal},
		{name: "a separator is refused", args: map[string]any{databaseTypeIDParam: "g6/1"}, want: refusal},
		{name: "padding is refused", args: map[string]any{databaseTypeIDParam: " g6-1 "}, want: refusal},
		{name: "a plain slug is accepted", args: map[string]any{databaseTypeIDParam: "g6-standard-2"}, want: ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := requestFor(testCase.args)

			_, message := tools.DeclaredFragmentSafeArgument(
				&request, databaseTypeIDParam, wantTypeIDText, wantTypeIDText, refusal,
			)
			if message != testCase.want {
				t.Errorf("DeclaredFragmentSafeArgument = %q, want %q", message, testCase.want)
			}
		})
	}
}

// TestDeclaredPathSafeArgumentAcceptsTheFragmentMarker is the other half of the
// pair: the member the OAuth ids declare lets a '#' through, which is why the
// two are separate members rather than one wording.
func TestDeclaredPathSafeArgumentAcceptsTheFragmentMarker(t *testing.T) {
	t.Parallel()

	request := requestFor(map[string]any{"client_id": "client#123"})

	value, message := tools.DeclaredPathSafeArgument(&request, "client_id", "", "", "")
	if message != "" {
		t.Errorf("DeclaredPathSafeArgument refused %q: %s", "client#123", message)
	}

	if value != "client#123" {
		t.Errorf("value = %q, want %q", value, "client#123")
	}
}
