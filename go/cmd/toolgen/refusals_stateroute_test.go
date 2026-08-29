package main_test

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The state_route refusals. A state fetch that does not land is the quietest
// failure in the destroy tier: the preview reports an empty resource, and a
// two-stage plan hashes that emptiness as the drift baseline.

// The read a state_route probe names, and the route it answers on.
const (
	probeStateReadTool = "probe_domain_get"
	probeStateReadPath = "/domains/{domain_id}"
)

func TestRefusesAStateRouteDeclarationThatCannotLand(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "declaration on a tool that neither removes nor previews",
			refusal: "errStateRouteWithNoReader",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeStateRouteNoReaderInput",
					writeOptions(withStateRoute(&linodev1.StateRoute{Tool: probeStateReadTool})),
					bodyString(probeDomainArg, 1)))
			},
		},
		{
			name:    "read the contract does not declare",
			refusal: "errStateRouteUnknownTool",
			build: stateRouteProbe("ProbeStateRouteUnknownToolInput",
				&linodev1.StateRoute{Tool: probeAbsentName}, nil),
		},
		{
			name:    "read whose route changes what it reports",
			refusal: "errStateRouteNotRead",
			build: stateRouteBesideProbe("ProbeStateRouteNotReadInput",
				&linodev1.StateRoute{Tool: probeStateReadTool},
				withRoute("POST", probeStateReadPath),
				withResponse("linode.mcp.v1.Domain")),
		},
		{
			name:    "read that declares no response to decode into",
			refusal: "errStateRouteNoResponse",
			build: stateRouteBesideProbe("ProbeStateRouteNoResponseInput",
				&linodev1.StateRoute{Tool: probeStateReadTool},
				withRoute("GET", probeStateReadPath)),
		},
		{
			name:    "read addressed by a path argument the removal does not carry",
			refusal: "errStateRouteUnknownSlot",
			build: stateRouteBesideProbe("ProbeStateRouteUnknownSlotInput",
				&linodev1.StateRoute{Tool: probeStateReadTool},
				withRoute("GET", "/domains/{domain_id}/records/{record_id}"),
				withResponse("linode.mcp.v1.Domain")),
		},
		{
			name:    "payload member the read's answer does not carry",
			refusal: "errStateRoutePayloadMember",
			build: stateRouteBesideProbe("ProbeStateRoutePayloadMemberInput",
				&linodev1.StateRoute{Tool: probeStateReadTool, PayloadMember: "absent"},
				withRoute("GET", probeStateReadPath),
				withResponse("linode.mcp.v1.Domain")),
		},
		{
			name:    "payload member beside a body key",
			refusal: "errStateRoutePayloadWithBodyKey",
			build: stateRouteBesideProbe("ProbeStateRoutePayloadWithBodyKeyInput",
				&linodev1.StateRoute{
					Tool:          probeStateReadTool,
					PayloadMember: "volume",
					BodyKey:       "volume",
				},
				withRoute("GET", probeStateReadPath),
				withResponse("linode.mcp.v1.VolumeGetResponse")),
		},
		{
			name:    "mapping a read slot the read route does not carry",
			refusal: "errStateRouteSlotUnknownRead",
			build: stateRouteBesideProbe("ProbeStateRouteSlotUnknownReadInput",
				stateRouteMapping("absent_slot", probeDeleteArg),
				withRoute("GET", probeStateReadPath),
				withResponse("linode.mcp.v1.Domain")),
		},
		{
			name:    "mapping a read slot to an argument the removal does not carry",
			refusal: "errStateRouteSlotUnknownArgument",
			build: stateRouteBesideProbe("ProbeStateRouteSlotUnknownArgumentInput",
				stateRouteMapping("owner_id", "absent_argument"),
				withRoute("GET", "/domains/{owner_id}"),
				withResponse("linode.mcp.v1.Domain")),
		},
		{
			name:    "mapping a read slot onto its own name",
			refusal: "errStateRouteSlotSameName",
			build: stateRouteBesideProbe("ProbeStateRouteSlotSameNameInput",
				stateRouteMapping(probeDeleteArg, probeDeleteArg),
				withRoute("GET", probeStateReadPath),
				withResponse("linode.mcp.v1.Domain")),
		},
		{
			name:    "read addressed by a required query parameter nothing fills",
			refusal: "errStateRouteUnfilledQuery",
			build: stateRouteQueryProbe("ProbeStateRouteUnfilledQueryInput",
				&linodev1.StateRoute{Tool: probeStateReadTool},
				queryString(probeStateQueryArg, probeStateQueryNumber)),
		},
		{
			// The read spells the slot in its path template and carries a query
			// parameter of that same name, so nothing says which one a fill means.
			name:    "read naming one slot in both its path and its query",
			refusal: "errStateRouteQueryAmbiguous",
			build: stateRouteQueryProbe("ProbeStateRouteAmbiguousQueryInput",
				&linodev1.StateRoute{Tool: probeStateReadTool},
				queryString(probeDeleteArg, probeFirstNumber)),
		},
		{
			name:    "query parameter filled from an argument named as a secret",
			refusal: "errStateRouteQuerySecret",
			build: stateRouteQueryProbe("ProbeStateRouteSecretQueryInput",
				&linodev1.StateRoute{
					Tool: probeStateReadTool,
					SlotArguments: []*linodev1.StateRouteSlot{
						{ReadSlot: probeStateQueryArg, Argument: probeSecretArg},
					},
				},
				queryString(probeStateQueryArg, probeStateQueryNumber)),
		},
	})
}

// The query parameter the state-route probes address their read with, and the
// field number it takes beside the read's own path slot.
const (
	probeStateQueryArg    = "object_key"
	probeStateQueryNumber = 2
)

// stateRouteQueryProbe is a removal whose declared read is addressed by a query
// parameter as well as by its path, which is the shape the object ACL takes.
//
// The removal carries a body argument named as a secret so the fill-from-a-
// secret case has something to name; the other two cases leave it unread.
func stateRouteQueryProbe(
	message string,
	declared *linodev1.StateRoute,
	parameter *descriptorpb.FieldDescriptorProto,
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		reads := messageOptions(
			withRoute("GET", probeStateReadPath),
			withResponse("linode.mcp.v1.Domain"),
			withCapability(linodev1.ToolCapability_TOOL_CAPABILITY_READ),
			withDescription("Reads a probe."),
		)

		// A parameter spelled like the path slot replaces it rather than sitting
		// beside it: one message cannot declare the field name twice, and the
		// route template is what names the slot either way.
		fields := []*descriptorpb.FieldDescriptorProto{parameter}
		if parameter.GetName() != probeDeleteArg {
			fields = append([]*descriptorpb.FieldDescriptorProto{pathInt(probeDeleteArg)}, fields...)
		}

		sibling := probeMessage(t, message+"Read", reads, fields...)

		run := goProbe(probeMessage(t, message,
			destroyOptions(withStateRoute(declared)),
			pathInt(probeDeleteArg), bodyString(probeSecretArg, probeStateQueryNumber)))
		run.Beside = map[string]protoreflect.MessageDescriptor{probeStateReadTool: sibling}

		return run
	}
}

// queryString is one text parameter a read route is addressed by, required in
// the schema because it carries no optional keyword.
func queryString(name string, number int32) *descriptorpb.FieldDescriptorProto {
	return probeField(name, number, descriptorpb.FieldDescriptorProto_TYPE_STRING,
		fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_QUERY)))
}

// stateRouteMapping is a declaration whose one slot entry is the subject, so
// each refusal above differs only in the pair it names.
func stateRouteMapping(slot, argument string) *linodev1.StateRoute {
	return &linodev1.StateRoute{
		Tool: probeStateReadTool,
		SlotArguments: []*linodev1.StateRouteSlot{
			{ReadSlot: slot, Argument: argument},
		},
	}
}

// stateRouteProbe is a removal declaring a state route with no sibling beside
// it, for the cases refused before the named read is ever looked up.
func stateRouteProbe(
	message string,
	declared *linodev1.StateRoute,
	extra func(*descriptorpb.MessageOptions),
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		sets := []func(*descriptorpb.MessageOptions){withStateRoute(declared)}
		if extra != nil {
			sets = append(sets, extra)
		}

		return goProbe(probeMessage(t, message, destroyOptions(sets...), pathInt(probeDeleteArg)))
	}
}

// stateRouteBesideProbe is a removal declaring a state route with the read it
// names declared beside it, so the refusal comes from what that read says
// rather than from its absence.
func stateRouteBesideProbe(
	message string,
	declared *linodev1.StateRoute,
	reads ...func(*descriptorpb.MessageOptions),
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		sibling := probeMessage(t, message+"Read",
			messageOptions(append(reads,
				withCapability(linodev1.ToolCapability_TOOL_CAPABILITY_READ),
				withDescription("Reads a probe."))...),
			pathInt(probeDeleteArg))

		run := goProbe(probeMessage(t, message,
			destroyOptions(withStateRoute(declared)), pathInt(probeDeleteArg)))
		run.Beside = map[string]protoreflect.MessageDescriptor{probeStateReadTool: sibling}

		return run
	}
}

func withStateRoute(declared *linodev1.StateRoute) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_StateRoute, declared)
	}
}

// The scan, envelope, and composite state-form refusals.

// probePageResponse is a real list envelope whose element the scan cases match
// against, and probeGetResponse a real single resource with no page in it.
const (
	probePageResponse = "linode.mcp.v1.VLANListResponse"
	probeGetResponse  = "linode.mcp.v1.VLAN"
	// compositeMemberOne and Two are the two members the composite cases land.
	compositeMemberOne = "one"
	compositeMemberTwo = "two"
)

func TestRefusesAStateFormThatCannotLand(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "match beside a single-resource declaration",
			refusal: "errScanWithResource",
			build: stateRouteBesideProbe("ProbeScanWithResourceInput",
				&linodev1.StateRoute{
					Tool: probeStateReadTool, PayloadMember: "domain",
					Match: []*linodev1.StateRouteMatch{{Field: probeBodyArg, Argument: probeDeleteArg}},
				},
				withRoute("GET", probeStateReadPath), withResponse(probePageResponse)),
		},
		{
			name:    "envelope beside a match",
			refusal: "errEnvelopeWithResource",
			build: stateRouteBesideProbe("ProbeEnvelopeWithMatchInput",
				&linodev1.StateRoute{
					Tool:          probeStateReadTool,
					EnvelopeState: true,
					Match:         []*linodev1.StateRouteMatch{{Field: probeBodyArg, Argument: probeDeleteArg}},
				},
				withRoute("GET", probeStateReadPath), withResponse(probePageResponse)),
		},
		{
			name:    "envelope beside a single-resource declaration",
			refusal: "errEnvelopeWithResource",
			build: stateRouteBesideProbe("ProbeEnvelopeWithResourceInput",
				&linodev1.StateRoute{
					Tool: probeStateReadTool, BodyKey: "domain", EnvelopeState: true,
				},
				withRoute("GET", probeStateReadPath), withResponse(probePageResponse)),
		},
		{
			name:    "scan of a read that answers no page",
			refusal: "errStateReadNotList",
			build: stateRouteBesideProbe("ProbeScanNotListInput",
				&linodev1.StateRoute{
					Tool:  probeStateReadTool,
					Match: []*linodev1.StateRouteMatch{{Field: probeBodyArg, Argument: probeDeleteArg}},
				},
				withRoute("GET", probeStateReadPath), withResponse(probeGetResponse)),
		},
		{
			name:    "match on a field the element does not declare",
			refusal: "errMatchUnknownField",
			build: stateRouteBesideProbe("ProbeMatchUnknownFieldInput",
				&linodev1.StateRoute{
					Tool:  probeStateReadTool,
					Match: []*linodev1.StateRouteMatch{{Field: probeAbsentName, Argument: probeDeleteArg}},
				},
				withRoute("GET", probeStateReadPath), withResponse(probePageResponse)),
		},
	})
}

func TestRefusesACompositeThatCannotLand(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "composite on a tool that advertises no plan",
			refusal: "errCompositeUnstaged",
			build:   compositeProbe("ProbeCompositeUnstagedInput", false, validComposite()),
		},
		{
			name:    "composite of one call",
			refusal: "errCompositeTooFew",
			build: compositeProbe("ProbeCompositeTooFewInput", true,
				withComposite(&linodev1.StateComposite{
					Calls: []*linodev1.StateCompositeCall{
						{Tool: probeStateReadTool, Member: compositeMemberOne, Fields: []string{probeBodyArg}},
					},
				})),
		},
		{
			name:    "composite landing one member twice",
			refusal: "errCompositeMember",
			build: compositeBesideProbe("ProbeCompositeMemberInput",
				&linodev1.StateComposite{
					Calls: []*linodev1.StateCompositeCall{
						{Tool: probeStateReadTool, Member: compositeMemberOne, Fields: []string{probeBodyArg}},
						{Tool: probeStateReadTool, Member: compositeMemberOne, Fields: []string{probeBodyArg}},
					},
				}),
		},
		{
			name:    "composite call keeping nothing",
			refusal: "errCompositeNoFields",
			build: compositeBesideProbe("ProbeCompositeNoFieldsInput",
				&linodev1.StateComposite{
					Calls: []*linodev1.StateCompositeCall{
						{Tool: probeStateReadTool, Member: compositeMemberOne},
						{Tool: probeStateReadTool, Member: compositeMemberTwo, Fields: []string{probeBodyArg}},
					},
				}),
		},
		{
			name:    "composite keeping a field the read does not declare",
			refusal: "errCompositeUnknownField",
			build: compositeBesideProbe("ProbeCompositeUnknownFieldInput",
				&linodev1.StateComposite{
					Calls: []*linodev1.StateCompositeCall{
						{Tool: probeStateReadTool, Member: compositeMemberOne, Fields: []string{probeAbsentName}},
						{Tool: probeStateReadTool, Member: compositeMemberTwo, Fields: []string{probeBodyArg}},
					},
				}),
		},
	})
}

// validComposite is a two-call declaration whose refusal comes from what sits
// beside it rather than from its own calls.
func validComposite() func(*descriptorpb.MessageOptions) {
	return withComposite(&linodev1.StateComposite{
		Calls: []*linodev1.StateCompositeCall{
			{Tool: probeStateReadTool, Member: compositeMemberOne, Fields: []string{probeBodyArg}},
			{Tool: probeStateReadTool, Member: compositeMemberTwo, Fields: []string{"region"}},
		},
	})
}

// compositeProbe is a mutation carrying a composite declaration, staged when
// the case needs the plan arguments in place.
func compositeProbe(
	message string, staged bool, sets ...func(*descriptorpb.MessageOptions),
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		fields := []*descriptorpb.FieldDescriptorProto{bodyString(probeDomainArg, 1)}
		if staged {
			fields = append(fields,
				probeField("mode", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING,
					fieldOptions(withLocation(localLocation))),
				probeField("plan_id", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING,
					fieldOptions(withLocation(localLocation))),
				probeField("dry_run", 4, descriptorpb.FieldDescriptorProto_TYPE_BOOL,
					fieldOptions(withLocation(localLocation))))
		}

		sets = append(sets, withResponse("linode.mcp.v1.MessageResponse"))

		return goProbe(probeMessage(t, message, writeOptions(sets...), fields...))
	}
}

// compositeBesideProbe is a staged mutation whose composite resolves against a
// real read declared beside it, so the refusal comes from the calls.
func compositeBesideProbe(
	message string, declared *linodev1.StateComposite,
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		sibling := probeMessage(t, message+"Read",
			messageOptions(withRoute("GET", "/domains"),
				withResponse(probeGetResponse),
				withCapability(linodev1.ToolCapability_TOOL_CAPABILITY_READ),
				withDescription("Reads a probe.")))

		run := compositeProbe(message, true, withComposite(declared))(t)
		run.Beside = map[string]protoreflect.MessageDescriptor{probeStateReadTool: sibling}

		return run
	}
}

func withComposite(declared *linodev1.StateComposite) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_StateComposite, declared)
	}
}
