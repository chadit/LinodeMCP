package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The rule a declared preview sentence is chosen and filled by. It is pinned
// here rather than per tool because it is one rule: every tool declaring prose
// reads its wordings through this, and the fixtures pin only the wording a
// caller who supplied everything reads.

const (
	// unmarshalableMember is the body key a preview cannot describe, named here
	// because a channel has no JSON form whatever it is called.
	unmarshalableMember = "stream"
	// previewLabelFixture is the label the wordings below read.
	previewLabelFixture = "ci-token"
)

// previewRequest is a tool call carrying the given arguments.
func previewRequest(arguments map[string]any) mcp.CallToolRequest {
	var request mcp.CallToolRequest

	request.Params.Arguments = arguments

	return request
}

func TestPreviewTextReportsOnlyText(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		arguments map[string]any
		want      string
	}{
		{name: "text", arguments: map[string]any{managedServiceLabelParam: previewLabelFixture}, want: previewLabelFixture},
		{name: absentEnvironment, arguments: map[string]any{}, want: ""},
		{name: "blank", arguments: map[string]any{managedServiceLabelParam: ""}, want: ""},
		{name: "a number is not a label", arguments: map[string]any{managedServiceLabelParam: float64(7)}, want: ""},
		{name: "a bool is not a label", arguments: map[string]any{managedServiceLabelParam: true}, want: ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := previewRequest(testCase.arguments)
			if got := tools.PreviewText(&request, managedServiceLabelParam); got != testCase.want {
				t.Errorf("PreviewText = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestPreviewNumberReportsOnlyAWholeNumberThatNamesSomething(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		arguments map[string]any
		want      string
	}{
		{name: "whole number", arguments: map[string]any{keySize: float64(20)}, want: "20"},
		{name: absentEnvironment, arguments: map[string]any{}, want: ""},
		{name: "zero names no resource", arguments: map[string]any{keySize: float64(0)}, want: ""},
		{name: "text is not a number", arguments: map[string]any{keySize: "20"}, want: ""},
		{name: "a bool is not a number", arguments: map[string]any{keySize: true}, want: ""},
		{name: "a fraction is not a whole number", arguments: map[string]any{keySize: 20.5}, want: ""},
		{name: "a decoded int is a whole number", arguments: map[string]any{keySize: 20}, want: "20"},
		{name: "a wide int is a whole number", arguments: map[string]any{keySize: int64(20)}, want: "20"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := previewRequest(testCase.arguments)
			if got := tools.PreviewNumber(&request, keySize); got != testCase.want {
				t.Errorf("PreviewNumber = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestPreviewSentenceReportsTheFirstWordingItCanFill(t *testing.T) {
	t.Parallel()

	labeled := `A new personal access token "{label}" will be created.`
	plain := "A new personal access token will be created."

	cases := []struct {
		name   string
		values map[string]string
		want   string
	}{
		{
			name:   "the specific wording when every placeholder has a value",
			values: map[string]string{managedServiceLabelParam: previewLabelFixture},
			want:   `A new personal access token "ci-token" will be created.`,
		},
		{
			name:   "the fallback when one does not",
			values: map[string]string{managedServiceLabelParam: ""},
			want:   plain,
		},
		{
			name:   "the fallback when the argument never arrived",
			values: map[string]string{},
			want:   plain,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := tools.PreviewSentence(testCase.values, labeled, plain); got != testCase.want {
				t.Errorf("PreviewSentence = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestPreviewSentenceFillsEveryPlaceholderInAWording(t *testing.T) {
	t.Parallel()

	values := map[string]string{keyType: "g7-highmem-2", keyRegion: "ap-south"}
	template := "A new {type} instance will be created in region {region}."

	want := "A new g7-highmem-2 instance will be created in region ap-south."
	if got := tools.PreviewSentence(values, template); got != want {
		t.Errorf("PreviewSentence = %q, want %q", got, want)
	}
}

// TestPreviewSentenceDropsALineNoWordingCanFill: a line whose every wording
// wants a value the call did not carry is reported not at all, rather than with
// a gap where the value would have gone.
func TestPreviewSentenceDropsALineNoWordingCanFill(t *testing.T) {
	t.Parallel()

	if got := tools.PreviewSentence(map[string]string{}, "A new {type} instance was requested."); got != "" {
		t.Errorf("PreviewSentence = %q, want empty", got)
	}
}

func TestPreviewSentenceWithNoWordingReportsNothing(t *testing.T) {
	t.Parallel()

	if got := tools.PreviewSentence(map[string]string{}); got != "" {
		t.Errorf("PreviewSentence = %q, want empty", got)
	}
}

// TestRunDeclaredPreviewReportsABodyItCannotDescribe: a preview whose body has
// no JSON form cannot say what the call would send, and answering with a
// preview missing that field would be worse than saying so.
func TestRunDeclaredPreviewReportsABodyItCannotDescribe(t *testing.T) {
	t.Parallel()

	request := previewRequest(map[string]any{})

	_, err := tools.RunDeclaredPreview(t.Context(), &request, &config.Config{},
		"linode_probe_create", http.MethodPost, "/probes",
		map[string]any{unmarshalableMember: make(chan int)}, nil, &tools.DryRunDetails{})
	if _, refused := errors.AsType[*json.UnsupportedTypeError](err); !refused {
		t.Errorf("error = %v, want a body JSON has no form for to be refused", err)
	}
}

// TestRunDeclaredPreviewStopsWhenTheCallerHasGone: a declared preview makes no
// call of its own, so its context is the only thing that can tell it the answer
// is no longer wanted.
func TestRunDeclaredPreviewStopsWhenTheCallerHasGone(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	request := previewRequest(map[string]any{})

	result, err := tools.RunDeclaredPreview(ctx, &request, &config.Config{},
		"linode_probe_create", http.MethodPost, "/probes",
		map[string]any{}, nil, &tools.DryRunDetails{})
	if err != nil {
		t.Fatalf("RunDeclaredPreview: %v", err)
	}

	if !result.IsError {
		t.Errorf("result = %v, want the canceled walk reported", result)
	}
}

// TestPreviewSentenceAnswersAWordingWhoseBraceNeverCloses: the contract refuses
// such a wording, so this is the answer for a call that reached one anyway: the
// text as written rather than a panic or a truncated sentence.
func TestPreviewSentenceAnswersAWordingWhoseBraceNeverCloses(t *testing.T) {
	t.Parallel()

	broken := "A new {label instance was requested."
	if got := tools.PreviewSentence(map[string]string{}, broken); got != broken {
		t.Errorf("PreviewSentence = %q, want %q", got, broken)
	}
}

// TestPreviewMemberFlagReadsAFlagInsideTheObjectItRides: the cluster ACL sends
// its flag inside the payload, so the reader has to reach into an argument that
// may not be an object at all.
func TestPreviewMemberFlagReadsAFlagInsideTheObjectItRides(t *testing.T) {
	t.Parallel()

	cases := []struct {
		supplied any
		name     string
		want     tools.PreviewFlagState
	}{
		{
			name:     "a flag the object carries",
			supplied: map[string]any{statusEnabled: true},
			want:     tools.PreviewFlagTrue,
		},
		{
			name:     "a flag the object carries turned off",
			supplied: map[string]any{statusEnabled: false},
			want:     tools.PreviewFlagFalse,
		},
		{
			name:     "an object carrying no such member",
			supplied: map[string]any{},
			want:     tools.PreviewFlagAbsent,
		},
		{
			name:     "an argument that is not an object",
			supplied: boolStringTrue,
			want:     tools.PreviewFlagAbsent,
		},
		{
			name:     "an argument the call never carried",
			supplied: nil,
			want:     tools.PreviewFlagAbsent,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			arguments := map[string]any{}
			if testCase.supplied != nil {
				arguments[keyACL] = testCase.supplied
			}

			request := previewRequest(arguments)
			if got := tools.PreviewMemberFlag(&request, keyACL, statusEnabled); got != testCase.want {
				t.Errorf("PreviewMemberFlag = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestStateReadQueryLeavesOutWhatTheCallDidNotCarry: a declared fetch fills the
// read's query from this tool's own arguments, and an optional one the caller
// omitted is left off rather than sent empty, since an empty parameter and no
// parameter select different resources.
func TestStateReadQueryLeavesOutWhatTheCallDidNotCarry(t *testing.T) {
	t.Parallel()

	cases := []struct {
		arguments map[string]any
		name      string
		want      string
	}{
		{
			name:      "the parameter the call carries",
			arguments: map[string]any{managedContactNameParam: "photo.jpg"},
			want:      "name=photo.jpg",
		},
		{
			name:      "a parameter the call left out",
			arguments: map[string]any{},
			want:      "",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := previewRequest(testCase.arguments)

			got := tools.StateReadQuery(&request,
				map[string]string{managedContactNameParam: managedContactNameParam})
			if got != testCase.want {
				t.Errorf("StateReadQuery = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestPreviewStateReadersReportWhatTheResourceCarries: a wording naming the
// resource reads a member of it, one level down for a composite projection, and
// reads nothing at all from an answer some other fetch produced.
func TestPreviewStateReadersReportWhatTheResourceCarries(t *testing.T) {
	t.Parallel()

	state := tools.DeclaredState{
		keyLabel: nodeBalancerNodeLabelWeb1,
		keySize:  json.Number("20"),
		keyCount: json.Number("0"),
		tcInstance: tools.DeclaredState{
			argType: "g6-standard-1",
		},
	}

	cases := []struct {
		state any
		name  string
		path  string
		text  string
		whole string
	}{
		{name: "a member of the resource", state: state, path: keyLabel, text: nodeBalancerNodeLabelWeb1},
		{name: "a member one level down", state: state, path: tcInstance + "." + argType, text: "g6-standard-1"},
		{name: "a whole number", state: state, path: keySize, whole: "20"},
		{name: "a zero reads as no value", state: state, path: keyCount},
		{name: "a member the resource does not carry", state: state, path: "nobody"},
		{name: "an answer some other fetch produced", state: "not a declared state", path: keyLabel},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := tools.PreviewStateText(testCase.state, testCase.path); got != testCase.text {
				t.Errorf("PreviewStateText(%q) = %q, want %q", testCase.path, got, testCase.text)
			}

			if got := tools.PreviewStateNumber(testCase.state, testCase.path); got != testCase.whole {
				t.Errorf("PreviewStateNumber(%q) = %q, want %q", testCase.path, got, testCase.whole)
			}
		})
	}
}

// TestPreviewChangedReadsAMatchingReadingAsAbsent: the rule that turns "Label
// changes from X to Y" into "Label is set to Y" is one comparison, so it is
// pinned here rather than through every family that declares the pair.
func TestPreviewChangedReadsAMatchingReadingAsAbsent(t *testing.T) {
	t.Parallel()

	if got := tools.PreviewChanged(previewPriorValue, "new"); got != previewPriorValue {
		t.Errorf("PreviewChanged(old, new) = %q, want %q", got, previewPriorValue)
	}

	if got := tools.PreviewChanged("same", "same"); got != "" {
		t.Errorf("PreviewChanged(same, same) = %q, want %q", got, "")
	}
}

// TestPreviewGuardReadsCarriedRatherThanEmpty: a guarded line asks whether the
// call sent the argument, so a zero and an empty list still report while an
// argument nobody sent drops the line.
func TestPreviewGuardReadsCarriedRatherThanEmpty(t *testing.T) {
	t.Parallel()

	request := createRequestWithArgs(t, map[string]any{
		keyCount: float64(0),
		keyTags:  []any{},
	})

	for _, name := range []string{keyCount, keyTags} {
		if !tools.PreviewCarried(&request, name) {
			t.Errorf("PreviewCarried(%q) = false, want true", name)
		}
	}

	if tools.PreviewCarried(&request, keyLabel) {
		t.Errorf("PreviewCarried(%q) = true, want false", keyLabel)
	}

	if got := tools.PreviewCarriedNumber(&request, keyCount); got != "0" {
		t.Errorf("PreviewCarriedNumber(%q) = %q, want %q", keyCount, got, "0")
	}

	if got := tools.PreviewCarriedNumber(&request, keyLabel); got != "" {
		t.Errorf("PreviewCarriedNumber(%q) = %q, want %q", keyLabel, got, "")
	}

	if got := tools.PreviewGuarded(true, "reported"); got != "reported" {
		t.Errorf("PreviewGuarded(true) = %q, want %q", got, "reported")
	}

	if got := tools.PreviewGuarded(false, "reported"); got != "" {
		t.Errorf("PreviewGuarded(false) = %q, want %q", got, "")
	}
}

// previewPriorValue is the value a resource already holds in the change tables
// below, named so the three rows read as one subject.
const previewPriorValue = "old"

// TestPreviewDiffersReportsOnlyARealChange: a line naming only the new value
// has nothing to say when the call carries none, and nothing to say when the
// resource already holds what the call would set.
func TestPreviewDiffersReportsOnlyARealChange(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		reading  string
		argument string
		want     bool
	}{
		{name: "a value the resource does not hold", reading: previewPriorValue, argument: "new", want: true},
		{name: "a value the resource already holds", reading: "same", argument: "same"},
		{name: "no value at all", reading: previewPriorValue, argument: ""},
		{name: "a first value on a resource holding none", reading: "", argument: "new", want: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := tools.PreviewDiffers(testCase.reading, testCase.argument); got != testCase.want {
				t.Errorf("PreviewDiffers(%q, %q) = %v, want %v",
					testCase.reading, testCase.argument, got, testCase.want)
			}
		})
	}
}

// TestPreviewMatchedSelectsTheArmTheValueNames: the arms are a shortlist, so a
// value none of them names falls to the wording that answers the rest, and an
// empty one drops the line.
func TestPreviewMatchedSelectsTheArmTheValueNames(t *testing.T) {
	t.Parallel()

	arms := map[string]string{"disabled": "stops {status}"}
	values := map[string]string{keyStatus: "disabled"}

	if got := tools.PreviewMatched(values, "disabled", arms, "starts {status}"); got != "stops disabled" {
		t.Errorf("PreviewMatched(disabled) = %q, want %q", got, "stops disabled")
	}

	named := map[string]string{keyStatus: "enabled"}
	if got := tools.PreviewMatched(named, "enabled", arms, "starts {status}"); got != "starts enabled" {
		t.Errorf("PreviewMatched(enabled) = %q, want %q", got, "starts enabled")
	}

	if got := tools.PreviewMatched(named, "enabled", arms, ""); got != "" {
		t.Errorf("PreviewMatched with no otherwise = %q, want %q", got, "")
	}
}

// TestPreviewOrReportsTheValueTheFoldWouldSend: a folded argument the call
// omitted still travels, as the value its own fold declares, so the wording
// reports that rather than dropping the line over a request the tool is going
// to make anyway. The firewall create's two policies are the case.
func TestPreviewOrReportsTheValueTheFoldWouldSend(t *testing.T) {
	t.Parallel()

	if got := tools.PreviewOr("DROP", "ACCEPT"); got != "DROP" {
		t.Errorf("PreviewOr with a supplied value = %q, want the value the call carried", got)
	}

	if got := tools.PreviewOr("", "ACCEPT"); got != "ACCEPT" {
		t.Errorf("PreviewOr with no value = %q, want the fold's declared default", got)
	}
}

// TestPreviewFoldedSelectsAnArmWhateverTheAPISpelled: a resource spells its own
// vocabularies, so a status the API sends as "Running" has to reach the arm
// declared for "running". The rescue warning is the case: its wording is chosen
// by a reading rather than by an argument, and an argument keeps the exact
// comparison this one deliberately does not.
func TestPreviewFoldedSelectsAnArmWhateverTheAPISpelled(t *testing.T) {
	t.Parallel()

	arms := map[string]string{statusRunning: "reboots {label}"}
	values := map[string]string{keyLabel: "web-1"}

	for _, reading := range []string{statusRunning, "Running", "RUNNING"} {
		folded := tools.PreviewFolded(reading)

		if got := tools.PreviewMatched(values, folded, arms, ""); got != "reboots web-1" {
			t.Errorf("PreviewMatched(PreviewFolded(%q)) = %q, want %q", reading, got, "reboots web-1")
		}
	}

	// A reading no arm names still drops the line, which is what an empty
	// otherwise means: folding widens which values match, not how many.
	if got := tools.PreviewMatched(values, tools.PreviewFolded("Offline"), arms, ""); got != "" {
		t.Errorf("PreviewMatched(offline) = %q, want the line dropped", got)
	}
}

// TestPreviewElementsReadsEveryEntryShape: the entries a wording writes over
// are text or whole numbers, so a list carrying anything else drops that entry
// rather than spelling it a way the other language would not.
func TestPreviewElementsReadsEveryEntryShape(t *testing.T) {
	t.Parallel()

	request := createRequestWithArgs(t, map[string]any{
		keyTags:   []any{"prod", float64(42), true, 1.5, tcStaging},
		keyLabel:  caseNotAList,
		keyRegion: []any{},
	})

	joined := tools.PreviewJoined(&request, keyTags, ", ")
	if want := "prod, 42, staging"; joined != want {
		t.Errorf("PreviewJoined = %q, want %q", joined, want)
	}

	if got := tools.PreviewJoined(&request, keyLabel, ", "); got != "" {
		t.Errorf("PreviewJoined over a value that is no list = %q, want %q", got, "")
	}

	if got := tools.PreviewJoined(&request, keyRegion, ", "); got != "" {
		t.Errorf("PreviewJoined over an empty list = %q, want %q", got, "")
	}

	if got := tools.PreviewJoined(&request, keyCount, ", "); got != "" {
		t.Errorf("PreviewJoined over an absent list = %q, want %q", got, "")
	}
}

// TestPreviewPerElementWritesALinePerEntry: a per-entry line binds the entry
// itself and reads the tool's other arguments the way any wording does, and a
// list the call did not send writes nothing at all.
func TestPreviewPerElementWritesALinePerEntry(t *testing.T) {
	t.Parallel()

	request := createRequestWithArgs(t, map[string]any{
		keyTags: []any{float64(456), float64(789)},
	})
	values := map[string]string{keyCount: "7"}
	template := "Linode {element} joins group {count}."

	got := tools.PreviewPerElement(&request, keyTags, values, template)
	want := []string{"Linode 456 joins group 7.", "Linode 789 joins group 7."}

	if !slices.Equal(got, want) {
		t.Errorf("PreviewPerElement = %v, want %v", got, want)
	}

	if lines := tools.PreviewPerElement(&request, keyRegion, values, template); len(lines) != 0 {
		t.Errorf("PreviewPerElement over an absent list = %v, want none", lines)
	}
}
