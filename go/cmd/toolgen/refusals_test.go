package main_test

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The negative paths: one synthesized declaration per refusal the emitter can
// raise, held to the sentence errors.go carries rather than to wording restated
// here. TestEveryRefusalIsAccountedFor reads the same source and fails when an
// error is added with no case naming it.

// The responses a probe answers with. Both are real messages, since a
// synthesized one has no Go type and the emitter refuses that first.
const (
	probeResource  = "linode.mcp.v1.Domain"
	probeWriteBody = "linode.mcp.v1.DomainWriteResponse"
)

// The route a probe calls, spelled once so the slot and the PATH argument that
// fills it cannot drift apart.
const (
	probePath     = "/probes/{probe_id}"
	probeIDArg    = "probe_id"
	probeParentID = "/probes"
)

// probeFirstNumber is the field number a probe's first argument takes.
const probeFirstNumber = 1

// probeAbsentName is the name a case uses for something the message does not
// declare, spelled once so every refusal about an unknown name reads alike.
const probeAbsentName = "nobody"

// refusalCase is one declaration and the refusal it has to raise.
type refusalCase struct {
	build   func(t *testing.T) *toolgen.ProbeRun
	name    string
	refusal string
}

// runRefusals drives each case and reports the ones that emitted anyway.
func runRefusals(t *testing.T, cases []refusalCase) {
	t.Helper()

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			wantRefusal(t, testCase.build(t), testCase.refusal)
		})
	}
}

// goProbe is a run of the Go arm alone, which is every case whose refusal comes
// from the contract build or from Go rendering.
func goProbe(message protoreflect.MessageDescriptor) *toolgen.ProbeRun {
	return &toolgen.ProbeRun{Name: probeToolName, Message: message}
}

// TestRefusesAToolDeclarationThatSaysTooLittle covers the message-level
// declarations a tool cannot be emitted without.
func TestRefusesAToolDeclarationThatSaysTooLittle(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "no tool_description",
			refusal: "errNoDescription",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeNoDescriptionInput",
					messageOptions(withRoute("GET", probePath), withCapability(readCapability),
						withResponse(probeResource), withErrorMessage("Failed: {error}")),
					pathInt(probeIDArg)))
			},
		},
		{
			name:    "no error_message",
			refusal: "errNoErrorMessage",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeNoErrorMessageInput",
					getOptions(withoutErrorMessage), pathInt(probeIDArg)))
			},
		},
		{
			name:    "routeless tool that is not meta",
			refusal: "errMetaNotEmitted",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeRoutelessInput",
					messageOptions(withMeta(), withCapability(readCapability),
						withDescription("Answers locally."), withResponse(probeResource))))
			},
		},
		{
			name:    "free-form read whose failure is worded apart from the error",
			refusal: "errNoStructFailPrefix",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeStructFailPrefixInput",
					messageOptions(withRoute("GET", probePath), withCapability(readCapability),
						withDescription("Gets a free-form probe."),
						withErrorMessage("Failed to retrieve the probe")),
					pathInt(probeIDArg)))
			},
		},
		{
			name:    "argument with no field_location",
			refusal: "errNoFieldLocation",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeNoLocationInput", getOptions(),
					pathInt(probeIDArg),
					probeField("stray", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, nil)))
			},
		},
	})
}

// TestRefusesARouteTheArgumentsCannotFill covers the path and its slots.
func TestRefusesARouteTheArgumentsCannotFill(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "more slots than PATH arguments",
			refusal: "errPathArity",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeSlotArityInput",
					getOptions(withPath("/probes/{probe_id}/parts/{part_id}")),
					pathInt(probeIDArg)))
			},
		},
		{
			name:    "PATH argument carrying neither text nor a number",
			refusal: "errUnsupportedPathKind",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbePathKindInput", getOptions(),
					probeField(probeIDArg, 1, descriptorpb.FieldDescriptorProto_TYPE_BOOL,
						fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_PATH)))))
			},
		},
	})
}

// TestRefusesAMessageTemplateNothingFills covers the placeholder forms declared
// prose is written in.
func TestRefusesAMessageTemplateNothingFills(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "placeholder naming no field",
			refusal: "errUnknownPlaceholder",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeUnknownPlaceholderInput",
					getOptions(withErrorMessage("Failed to retrieve probe {nobody}: {error}")),
					pathInt(probeIDArg)))
			},
		},
		{
			name:    "count on a field that carries one value",
			refusal: "errNotRepeated",
			build: writeProbe("ProbeNotRepeatedInput",
				withSuccessMessage("Created {note:len} probes")),
		},
	})
}

// The capability a probe declares, named because several cases spell it.
const (
	readCapability  = linodev1.ToolCapability_TOOL_CAPABILITY_READ
	writeCapability = linodev1.ToolCapability_TOOL_CAPABILITY_WRITE
)

// getOptions is a read tool that emits, which the cases perturb one declaration
// at a time.
func getOptions(sets ...func(*descriptorpb.MessageOptions)) *descriptorpb.MessageOptions {
	declared := make([]func(*descriptorpb.MessageOptions), 0, 5+len(sets))
	declared = append(declared,
		withRoute("GET", probePath),
		withCapability(readCapability),
		withResponse(probeResource),
		withDescription("Gets a probe."),
		withErrorMessage("Failed to retrieve probe {probe_id}: {error}"),
	)

	return messageOptions(append(declared, sets...)...)
}

// withoutErrorMessage drops the declaration the tiers that report failure in
// prose are refused without.
func withoutErrorMessage(options *descriptorpb.MessageOptions) {
	proto.ClearExtension(options, linodev1.E_ErrorMessage)
}

func withCapability(capability linodev1.ToolCapability) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_ToolCapability, capability)
	}
}

func withResponse(message string) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_ToolResponse, message)
	}
}

func withErrorMessage(text string) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_ErrorMessage, text)
	}
}

// withPath rewrites the route the base declared, for the cases about slots.
func withPath(path string) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		route, _ := proto.GetExtension(options, linodev1.E_ToolRoute).(*linodev1.ToolRoute)
		proto.SetExtension(options, linodev1.E_ToolRoute, &linodev1.ToolRoute{
			Tool: route.GetTool(), Method: route.GetMethod(), Path: path,
		})
	}
}

// withMeta names the tool without naming a route, which is what a tool
// answering from local state declares.
func withMeta() func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_ToolMeta, &linodev1.ToolMeta{Tool: probeToolName})
	}
}

// pathInt is the argument a route slot is filled from. It is the first field
// of every probe that declares one, so the number is fixed here.
func pathInt(name string) *descriptorpb.FieldDescriptorProto {
	return probeField(name, probeFirstNumber, descriptorpb.FieldDescriptorProto_TYPE_INT32,
		fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_PATH)))
}
