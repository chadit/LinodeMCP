package main_test

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The refusals about a whole tool: the prose a tier reports through, the checks
// declared over its arguments, the hooks it hands work to, and the answer it
// builds.

// The route a mutation probe calls. It is a real one, since the response it
// answers with has to be a message that already has a Go type, and the two are
// declared together.
const (
	probeWritePath   = "/domains"
	probeWriteMethod = "POST"
	probeDomainArg   = "note"
)

// writeOptions is a mutation that emits, which the cases perturb one
// declaration at a time.
func writeOptions(sets ...func(*descriptorpb.MessageOptions)) *descriptorpb.MessageOptions {
	declared := make([]func(*descriptorpb.MessageOptions), 0, 7+len(sets))
	declared = append(declared,
		withRoute(probeWriteMethod, probeWritePath),
		withCapability(writeCapability),
		withResponse(probeWriteBody),
		withDescription("Creates a probe."),
		withErrorMessage("Failed to create probe: {error}"),
		withConfirmMessage("This creates a probe. Set confirm=true to proceed."),
		withSuccessMessage("Probe created successfully"),
	)

	return messageOptions(append(declared, sets...)...)
}

// writeProbe is the base mutation carrying whatever the case declares on top.
func writeProbe(message string, sets ...func(*descriptorpb.MessageOptions)) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		return goProbe(probeMessage(t, message, writeOptions(sets...), bodyString(probeDomainArg, 1)))
	}
}

// TestRefusesAMutationThatCannotReportWhatItDid covers the prose a tier is
// refused without and the prose it has nowhere to put.
func TestRefusesAMutationThatCannotReportWhatItDid(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "no confirm_message",
			refusal: "errNoConfirmMessage",
			build:   writeProbe("ProbeNoConfirmInput", withoutConfirmMessage),
		},
		{
			name:    "no success_message",
			refusal: "errNoSuccessMessage",
			build:   writeProbe("ProbeNoSuccessInput", withoutSuccessMessage),
		},
		{
			name:    "success_message on an answer that carries no sentence",
			refusal: "errUnusedSuccessMessage",
			build:   writeProbe("ProbeUnusedSuccessInput", withResponse(probeResource)),
		},
		{
			name:    "no tool_response",
			refusal: "errNoResponse",
			build:   writeProbe("ProbeNoResponseInput", withoutResponse),
		},
		{
			name:    "warning_message on an answer that carries no notice",
			refusal: "errUnusedWarningMessage",
			build:   writeProbe("ProbeUnusedWarningInput", withWarningMessage("Heads up")),
		},
	})
}

// TestRefusesADeclaredCheckNothingRuns covers the message-level checks the
// emitter would read and drop.
func TestRefusesADeclaredCheckNothingRuns(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "require_any_of naming one field",
			refusal: "errAnyOfTooFew",
			build: writeProbe("ProbeAnyOfTooFewInput",
				withAnyOf(&linodev1.RequireAnyOf{Fields: []string{probeDomainArg}})),
		},
		{
			name:    "require_any_of naming an argument the message does not send",
			refusal: "errAnyOfUnknownField",
			build: writeProbe("ProbeAnyOfUnknownInput",
				withAnyOf(&linodev1.RequireAnyOf{Fields: []string{probeDomainArg, probeAbsentName}})),
		},
		{
			name:    "require_any_of counting one answer twice",
			refusal: "errAnyOfDuplicateField",
			build: writeProbe("ProbeAnyOfDuplicateInput",
				withAnyOf(&linodev1.RequireAnyOf{Fields: []string{probeDomainArg, probeDomainArg}})),
		},
		{
			name:    "require_any_of beside the hook that owns the check",
			refusal: "errAnyOfWithValidate",
			build: writeProbe("ProbeAnyOfValidateInput", withHooks("validate"),
				withAnyOf(&linodev1.RequireAnyOf{Fields: []string{probeDomainArg, "other"}})),
		},
		{
			name:    "require_any_of on a tier that runs no body checks",
			refusal: "errAnyOfTier",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeAnyOfTierInput",
					getOptions(withAnyOf(&linodev1.RequireAnyOf{
						Fields: []string{probeIDArg, "other"},
					})),
					pathInt(probeIDArg)))
			},
		},
		{
			name:    "refuse_arguments declaring half of itself",
			refusal: "errRefuseArguments",
			build: writeProbe("ProbeRefuseHalfInput",
				withRefuse(&linodev1.RefuseArguments{Fields: []string{"gone"}})),
		},
		{
			name:    "refuse_arguments naming an argument the message declares",
			refusal: "errRefuseDeclaredField",
			build: writeProbe("ProbeRefuseDeclaredInput",
				withRefuse(&linodev1.RefuseArguments{
					Fields: []string{probeDomainArg}, Message: "domain is not accepted here",
				})),
		},
		{
			name:    "refuse_arguments beside the hook that owns the check",
			refusal: "errRefuseWithValidate",
			build: writeProbe("ProbeRefuseValidateInput", withHooks("validate"),
				withRefuse(&linodev1.RefuseArguments{
					Fields: []string{"gone"}, Message: "gone is not accepted here",
				})),
		},
		{
			name:    "refuse_unknown_arguments beside the hook that owns the check",
			refusal: "errRefuseUnknownWithValidate",
			build: writeProbe("ProbeRefuseUnknownValidateInput", withHooks("validate"),
				withRefuseUnknown(&linodev1.RefuseUnknownArguments{Message: "{name} is not accepted"})),
		},
	})
}

// TestRefusesAHookTheTierCannotRun covers the steps a tool hands work to.
func TestRefusesAHookTheTierCannotRun(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "hook kind nothing reads",
			refusal: "errUnknownHookKind",
			build:   writeProbe("ProbeUnknownHookInput", withHooks("sing")),
		},
		{
			name:    "one hook kind declared twice",
			refusal: "errRepeatedHookKind",
			build:   writeProbe("ProbeRepeatedHookInput", withHooks("preview", "preview")),
		},
		{
			name:    "plan hook on a tool advertising no plan",
			refusal: "errUnstagedHook",
			build:   writeProbe("ProbeUnstagedHookInput", withHooks("dependency_walk")),
		},
	})
}

// TestRefusesAnAnswerTheContractCannotFill covers the response-side
// declarations.
func TestRefusesAnAnswerTheContractCannotFill(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "explicit nulls naming a member the answer cannot carry",
			refusal: "errUnknownNullField",
			build:   writeProbe("ProbeUnknownNullInput", withExplicitNulls(probeAbsentName)),
		},
		{
			name:    "explicit nulls beside the fields the body already fills",
			refusal: "errNullsOverDecodedResponse",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeNullsOverDecodedInput",
					messageOptions(withRoute(probeWriteMethod, "/images/upload"),
						withCapability(writeCapability),
						withResponse("linode.mcp.v1.ImageUploadWriteResponse"),
						withDescription("Creates a probe upload."),
						withErrorMessage("Failed to upload probe: {error}"),
						withConfirmMessage("This creates a probe upload. Set confirm=true to proceed."),
						withSuccessMessage("Probe upload created successfully"),
						withResponseBody("upload_to"), withExplicitNulls("expiry")),
					bodyString("label", 1)))
			},
		},
		{
			name:    "response_body_fields naming a member the answer cannot fill",
			refusal: "errUnknownResponseBody",
			build:   writeProbe("ProbeUnknownResponseBodyInput", withResponseBody(probeAbsentName)),
		},
	})
}

// The declarations the cases above add to or drop from the base.
func withConfirmMessage(text string) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_ConfirmMessage, text)
	}
}

func withSuccessMessage(text string) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_SuccessMessage, text)
	}
}

func withWarningMessage(text string) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_WarningMessage, text)
	}
}

func withoutConfirmMessage(options *descriptorpb.MessageOptions) {
	proto.ClearExtension(options, linodev1.E_ConfirmMessage)
}

func withoutSuccessMessage(options *descriptorpb.MessageOptions) {
	proto.ClearExtension(options, linodev1.E_SuccessMessage)
}

// withoutResponse drops the message a mutation's answer is assembled from,
// which no tier but a free-form read can be emitted without.
func withoutResponse(options *descriptorpb.MessageOptions) {
	proto.ClearExtension(options, linodev1.E_ToolResponse)
}

func withHooks(kinds ...string) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_ToolHooks, kinds)
	}
}

func withAnyOf(check *linodev1.RequireAnyOf) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_RequireAnyOf, check)
	}
}

func withRefuse(check *linodev1.RefuseArguments) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_RefuseArguments, check)
	}
}

func withRefuseUnknown(check *linodev1.RefuseUnknownArguments) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_RefuseUnknownArguments, check)
	}
}

func withExplicitNulls(names ...string) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_ExplicitNullFields, names)
	}
}

func withResponseBody(names ...string) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_ResponseBodyFields, names)
	}
}
