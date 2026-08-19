package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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
