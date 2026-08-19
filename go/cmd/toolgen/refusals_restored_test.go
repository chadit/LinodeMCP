package main_test

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The refusals the retired Python suite pinned that the cases above did not
// reach. They are restored here rather than left to the accounting list,
// because a check that was covered before this port and is not covered after it
// is the one gap a rewrite is most likely to leave.

// probeWarningResponse is a real envelope carrying a warning member, which is
// what makes a missing warning_message reportable.
const probeWarningResponse = "linode.mcp.v1.OAuthClientCreateWriteResponse"

// TestRefusesWhatThePythonSuitePinned covers those checks.
func TestRefusesWhatThePythonSuitePinned(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "a response that declares a warning the tool never words",
			refusal: "errNoWarningMessage",
			build:   writeProbe("ProbeNoWarningMessageInput", withResponse(probeWarningResponse)),
		},
		{
			name:    "the plan arguments on a tier with no flow behind them",
			refusal: "errNoTwoStageDriver",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeNoTwoStageDriverInput", getOptions(),
					pathInt(probeIDArg),
					probeField("mode", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING,
						fieldOptions(withLocation(localLocation))),
					probeField("plan_id", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING,
						fieldOptions(withLocation(localLocation)))))
			},
		},
		{
			name:    "a list named in a sentence without the count form",
			refusal: "errRepeatedPlaceholder",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeRepeatedPlaceholderInput",
					writeOptions(withSuccessMessage("Created probes {tags}")),
					repeatedField("tags", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING,
						fieldOptions(withLocation(bodyLocation)))))
			},
		},
		{
			name:    "a domain argument no accessor reads",
			refusal: "errUnsupportedToolArg",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeUnsupportedToolArgInput",
					metaOptions(withHooks(), withResponse(probeWriteBody),
						withSuccessMessage("Reported {flag}")),
					probeField("flag", 1, descriptorpb.FieldDescriptorProto_TYPE_BOOL,
						fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_TOOL)))))
			},
		},
		{
			name:    "a removal with no state read to plan through",
			refusal: "errNoFetchState",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeNoFetchStateInput",
					destroyOptions(), pathInt(probeDeleteArg)))
			},
		},
		{
			name:    "a removal answering with an id it was never given",
			refusal: "errNotADeleteEnvelope",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeNotADeleteEnvelopeInput",
					destroyOptions(withPath(probePath)), pathInt(probeIDArg)))
			},
		},
		{
			name:    "a fold naming no member it can be built from",
			refusal: "errFoldNotPlaced",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				folded := bodyString(probeDomainArg, 1)
				folded.Options = fieldOptions(withLocation(bodyLocation),
					withFold(&linodev1.BodyFold{
						Source: []*linodev1.BodyFoldSource{{Argument: probeAbsentName, Key: "name"}},
					}))

				return goProbe(probeMessage(t, "ProbeFoldNotPlacedInput", writeOptions(), folded))
			},
		},
	})
}

// The route a removal probe calls and the id its answer echoes, which the
// delete envelope names.
const (
	probeDeletePath = "/domains/{domain_id}"
	probeDeleteArg  = "domain_id"
)

// destroyOptions is a removal that emits, whose answer is the id echo the
// contract already declares for domains.
func destroyOptions(sets ...func(*descriptorpb.MessageOptions)) *descriptorpb.MessageOptions {
	declared := make([]func(*descriptorpb.MessageOptions), 0, 7+len(sets))
	declared = append(declared,
		withRoute("DELETE", probeDeletePath),
		withCapability(linodev1.ToolCapability_TOOL_CAPABILITY_DESTROY),
		withResponse("linode.mcp.v1.DomainDeleteResponse"),
		withDescription("Removes a probe."),
		withErrorMessage("Failed to remove probe {domain_id}: {error}"),
		withConfirmMessage("This removes a probe. Set confirm=true to proceed."),
		withSuccessMessage("Probe {domain_id} removed successfully"),
	)

	return messageOptions(append(declared, sets...)...)
}

func withFold(fold *linodev1.BodyFold) func(*descriptorpb.FieldOptions) {
	return func(options *descriptorpb.FieldOptions) {
		proto.SetExtension(options, linodev1.E_BodyFold, fold)
	}
}
