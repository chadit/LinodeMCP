package tools_test

import (
	"math"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
)

// TestPrepareClientRefusesAnEnvironmentThatIsNotAName covers the caller who
// sends a number, a flag, a list, or an object where an environment name goes.
// Reading the argument as text dropped every one of those and prepared a client
// for the default environment, so the call ran against whatever account that
// environment's token belongs to and no answer said so. Python's
// _select_environment refuses the same values under the same sentence.
//
// The config points at a server that fails the test on any request, which is
// what proves the refusal lands before the call reaches the API.
func TestPrepareClientRefusesAnEnvironmentThatIsNotAName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value any
		name  string
		want  string
	}{
		{
			name:  "number",
			value: float64(5),
			want:  "environment not found in configuration: 5",
		},
		{
			name:  "flag",
			value: true,
			want:  "environment not found in configuration: true",
		},
		{
			name:  "list",
			value: []any{canRunEnvProd, tcStaging},
			want:  `environment not found in configuration: ["prod","staging"]`,
		},
		{
			name:  "object",
			value: map[string]any{managedContactNameParam: canRunEnvProd},
			want:  `environment not found in configuration: {"name":"prod"}`,
		},
		{
			// Only a Go caller reaches this: JSON carries no NaN, so the
			// encoder never fails on a value that arrived over the wire.
			name:  "value JSON cannot spell",
			value: math.NaN(),
			want:  "environment not found in configuration: NaN",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, _, handler := gentools.NewLinodeInstanceListTool(dryRunNoCallServer(t))
			request := createRequestWithArgs(t, map[string]any{keyEnvironment: testCase.value})

			result, err := handler(t.Context(), request)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := errorText(t, result); got != testCase.want {
				t.Errorf("text = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestPrepareClientRefusesAnEnvironmentTheConfigDoesNotName pins the sentence
// the wrong-typed refusal above shares with the legitimate unknown name, since
// the two used to be worded apart from each other across the two languages.
func TestPrepareClientRefusesAnEnvironmentTheConfigDoesNotName(t *testing.T) {
	t.Parallel()

	_, _, handler := gentools.NewLinodeInstanceListTool(dryRunNoCallServer(t))
	request := createRequestWithArgs(t, map[string]any{keyEnvironment: tcStaging})

	result, err := handler(t.Context(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := errorText(t, result); got != envNotFoundStagingText {
		t.Errorf("text = %q, want %q", got, envNotFoundStagingText)
	}
}

// TestPrepareClientReadsAnUnnamedEnvironmentAsTheDefault covers the three
// spellings that name no environment at all: the argument left out, an explicit
// null, and an empty string. All three select the default, so none of them may
// take the refusal arm the values above take.
//
// The default environment here carries no URL or token, which is what makes the
// arm the call reached readable in the answer: it gets as far as judging the
// deployment's own configuration rather than the caller's argument.
func TestPrepareClientReadsAnUnnamedEnvironmentAsTheDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args map[string]any
		name string
	}{
		{name: caseReaderAbsent, args: map[string]any{}},
		{name: caseExplicitNull, args: map[string]any{keyEnvironment: nil}},
		{name: "empty string", args: map[string]any{keyEnvironment: ""}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault},
			}}

			_, _, handler := gentools.NewLinodeInstanceListTool(cfg)
			request := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), request)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			want := "linode configuration is incomplete: check your API URL and token"
			if got := errorText(t, result); got != want {
				t.Errorf("text = %q, want %q", got, want)
			}
		})
	}
}
