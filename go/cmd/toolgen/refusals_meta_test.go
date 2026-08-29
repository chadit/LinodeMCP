package main_test

import (
	"errors"
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The refusals a tool answering from local state raises, the ones the declared
// prose forms raise, the ones the Python arm raises, and the coverage check
// every arm passes on a real run.

// The response a meta probe answers with, and the tier's capability.
const (
	probeMetaResponse = "linode.mcp.v1.AuditHealthResponse"
	metaCapability    = linodev1.ToolCapability_TOOL_CAPABILITY_META
	probeMetaSentence = "Probe is healthy"
)

// metaOptions is a tool that reaches no route and answers from local state.
func metaOptions(sets ...func(*descriptorpb.MessageOptions)) *descriptorpb.MessageOptions {
	declared := make([]func(*descriptorpb.MessageOptions), 0, 6+len(sets))
	declared = append(declared,
		withMeta(),
		withCapability(metaCapability),
		withResponse(probeMetaResponse),
		withDescription("Reports the probe's own state."),
		withSuccessMessage(probeMetaSentence),
		// Categories are required on meta tools too, unlike scopes.
		withCategories(&linodev1.ToolCategories{None: true}),
	)

	return messageOptions(append(declared, sets...)...)
}

// metaProbe is the base meta tool carrying whatever the case declares on top.
func metaProbe(
	message string, sets ...func(*descriptorpb.MessageOptions),
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		return goProbe(probeMessage(t, message, metaOptions(sets...)))
	}
}

// TestRefusesAMetaToolThatCannotAnswer covers the tier that reaches no route.
func TestRefusesAMetaToolThatCannotAnswer(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "neither a local answer nor a sentence",
			refusal: "errNoMetaAnswer",
			build:   metaProbe("ProbeMetaNoAnswerInput", withoutSuccessMessage),
		},
		{
			name:    "a sentence the response cannot report",
			refusal: "errNoMetaMessageField",
			build:   metaProbe("ProbeMetaNoMessageFieldInput", withResponse(probeResource)),
		},
	})
}

// TestRefusesACoverageGapBetweenTheArms covers the check that holds every arm
// to every option the contract declares.
func TestRefusesACoverageGapBetweenTheArms(t *testing.T) {
	t.Parallel()

	full := armClaims(t, "go")

	cases := []struct {
		name    string
		refusal string
		claims  []toolgen.ProbeClaim
	}{
		{
			name:    "an option the arm says nothing about",
			refusal: "errUnclaimedOption",
			claims:  full[:len(full)-1],
		},
		{
			name:    "an option the contract does not declare",
			refusal: "errUnknownClaim",
			claims:  append(fullClaims(full), toolgen.ProbeClaim{Option: "no_such_option", Emitted: true}),
		},
		{
			name:    "one option answered twice",
			refusal: "errRepeatedClaim",
			claims:  append(fullClaims(full), full[0]),
		},
		{
			name:    "an option the arm neither emits nor places",
			refusal: "errSilentClaim",
			claims:  silenced(full),
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := toolgen.ProbeCoverage("probe", testCase.claims)

			if want := refusalNamed(t, testCase.refusal); !errors.Is(err, want) {
				t.Errorf("refusal = %v, want %s (%v)", err, testCase.refusal, want)
			}
		})
	}
}

// TestEveryArmAnswersForEveryOption is the other half: the claims each
// registered language makes are what a run is held to, so they are read here
// rather than restated.
func TestEveryArmAnswersForEveryOption(t *testing.T) {
	t.Parallel()

	for _, language := range []string{"go", "python"} {
		t.Run(language, func(t *testing.T) {
			t.Parallel()

			if err := toolgen.ProbeCoverage(language, armClaims(t, language)); err != nil {
				t.Errorf("%s does not answer for every declared option: %v", language, err)
			}
		})
	}
}

// armClaims is one registered language's answers.
func armClaims(t *testing.T, language string) []toolgen.ProbeClaim {
	t.Helper()

	claims, err := toolgen.ProbeArmClaims(language)
	if err != nil {
		t.Fatalf("read %s claims: %v", language, err)
	}

	if len(claims) == 0 {
		t.Fatalf("%s answers for no option", language)
	}

	return claims
}

// fullClaims copies the arm's answers, since the cases below append to them.
func fullClaims(claims []toolgen.ProbeClaim) []toolgen.ProbeClaim {
	return append([]toolgen.ProbeClaim(nil), claims...)
}

// silenced is the arm's answers with the first one saying nothing: it neither
// emits the option nor names the file that acts it.
func silenced(claims []toolgen.ProbeClaim) []toolgen.ProbeClaim {
	quiet := fullClaims(claims)
	quiet[0] = toolgen.ProbeClaim{Option: quiet[0].Option, Emitted: false, Home: ""}

	return quiet
}
