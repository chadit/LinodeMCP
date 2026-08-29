package main_test

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The dependency_walk refusals: every structural rule that needs no other
// tool in scope, each refused at contract build.

func TestRefusesAWalkThatCannotLand(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "walk with no iteration source",
			refusal: refusalWalkSourceCount,
			build: depWalkProbe("ProbeWalkNoSourceInput",
				&linodev1.DependencyWalk{Emit: validEmit()}),
		},
		{
			name:    "walk with two iteration sources",
			refusal: refusalWalkSourceCount,
			build: depWalkProbe("ProbeWalkTwoSourcesInput",
				&linodev1.DependencyWalk{
					ListTool: probeStateReadTool, StateMember: probeWalkMember, Emit: validEmit(),
					ListErrorWarning: "Could not list: {error}",
				}),
		},
		{
			name:    "walk reading elements into no emission",
			refusal: "errWalkNoEmit",
			build: depWalkProbe("ProbeWalkNoEmitInput",
				&linodev1.DependencyWalk{StateMember: probeWalkMember}),
		},
		{
			name:    "route walk without its failure sentence",
			refusal: "errWalkErrorWarning",
			build: depWalkProbe("ProbeWalkNoErrorWarningInput",
				&linodev1.DependencyWalk{ListTool: probeStateReadTool, Emit: validEmit()}),
		},
		{
			name:    "emission naming its kind both ways",
			refusal: "errWalkEmitKind",
			build: depWalkProbe("ProbeWalkKindBothInput",
				&linodev1.DependencyWalk{
					StateMember: probeWalkMember,
					Emit: &linodev1.WalkEmit{
						Kind: probeWalkKind, KindField: "type", Action: probeWalkAction, Note: "n",
					},
				}),
		},
		{
			name:    "emission with no line worth saying",
			refusal: "errWalkEmitEmpty",
			build: depWalkProbe("ProbeWalkEmitEmptyInput",
				&linodev1.DependencyWalk{
					StateMember: probeWalkMember,
					Emit:        &linodev1.WalkEmit{Kind: probeWalkKind},
				}),
		},
		{
			name:    "filter with no field",
			refusal: "errWalkFilterShape",
			build: depWalkProbe("ProbeWalkFilterShapeInput",
				&linodev1.DependencyWalk{
					StateMember: probeWalkMember,
					Filter:      &linodev1.WalkFilter{Value: "NS"},
					Emit:        validEmit(),
				}),
		},
		{
			name:    "warning with two predicates",
			refusal: "errWalkWarningShape",
			build: depWalkProbe("ProbeWalkWarningShapeInput",
				&linodev1.DependencyWalk{
					StateMember: probeWalkMember,
					Emit:        validEmit(),
					Warnings: []*linodev1.WalkWarning{
						{Template: "t", WhenTruncated: true, WhenAny: true},
					},
				}),
		},
		{
			name:    "sourceless walk with nothing to say",
			refusal: refusalWalkSourceCount,
			build: depWalkProbe("ProbeWalkSourcelessSilentInput",
				&linodev1.DependencyWalk{}),
		},
		{
			name:    "side-effect line carrying a kind",
			refusal: "errWalkEmitKind",
			build: depWalkProbe("ProbeWalkSideEffectKindInput",
				&linodev1.DependencyWalk{
					StateMember: probeWalkMember,
					Emit: &linodev1.WalkEmit{
						Target: linodev1.WalkTarget_WALK_TARGET_SIDE_EFFECTS,
						Kind:   probeWalkKind, Note: "n",
					},
				}),
		},
		{
			name:    "side-effect line with no prose",
			refusal: "errWalkEmitEmpty",
			build: depWalkProbe("ProbeWalkSideEffectSilentInput",
				&linodev1.DependencyWalk{
					StateMember: probeWalkMember,
					Emit: &linodev1.WalkEmit{
						Target: linodev1.WalkTarget_WALK_TARGET_SIDE_EFFECTS,
					},
				}),
		},
		{
			name:    "label fallback with nothing to fall back from",
			refusal: "errWalkEmitLabel",
			build: depWalkProbe("ProbeWalkLabelFallbackInput",
				&linodev1.DependencyWalk{
					StateMember: probeWalkMember,
					Emit: &linodev1.WalkEmit{
						Kind: probeWalkKind, Action: probeWalkAction,
						Note: "n", LabelFallback: "{enrich:label}",
					},
				}),
		},
		{
			name:    "warning pairing an old predicate with a new one",
			refusal: "errWalkWarningShape",
			build: depWalkProbe("ProbeWalkPredicatePairInput",
				&linodev1.DependencyWalk{
					StateMember: probeWalkMember,
					Emit:        validEmit(),
					Warnings: []*linodev1.WalkWarning{
						{Template: "t", WhenAny: true, WhenPositive: "count"},
					},
				}),
		},
		{
			name:    "member selector with no list to select from",
			refusal: "errWalkSourceCount",
			build: depWalkProbe("ProbeWalkMemberNoListInput",
				&linodev1.DependencyWalk{
					StateMember: probeWalkMember, ListMember: "ipv4.public",
					Emit: validEmit(),
				}),
		},
		{
			name:    "slot spellings with no list to fill",
			refusal: "errWalkSourceCount",
			build: depWalkProbe("ProbeWalkSlotsNoListInput",
				&linodev1.DependencyWalk{
					StateMember: probeWalkMember,
					SlotArguments: []*linodev1.StateRouteSlot{
						{ReadSlot: "linode_id", Argument: "instance_id"},
					},
					Emit: validEmit(),
				}),
		},
		{
			name:    "billing estimate missing a sentence",
			refusal: "errBillingShape",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeBillingShapeInput",
					destroyOptions(withBilling(&linodev1.BillingDelta{
						PriceTool: probeStateReadTool, TypeField: "type",
						Sentence: "-{monthly:.2f}",
					})),
					pathInt(probeDeleteArg)))
			},
		},
	})
}

// TestRefusesAWalkTheResolveCannotLand covers the rules that need the other
// declared tools or the state in scope.
func TestRefusesAWalkTheResolveCannotLand(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "state walk with no declared read to feed it",
			refusal: "errWalkNoState",
			build: depWalkProbe("ProbeWalkNoStateInput",
				validWalk()),
		},
		{
			name:    "walk over a member the state does not carry",
			refusal: "errWalkStateMember",
			build: statefulWalkProbe("ProbeWalkBadMemberInput",
				&linodev1.DependencyWalk{StateMember: "nope", Emit: validEmit()}),
		},
		{
			name:    "emission reading a field the element does not declare",
			refusal: refusalWalkField,
			build: statefulWalkProbe("ProbeWalkBadFieldInput",
				&linodev1.DependencyWalk{
					StateMember: probeWalkMember,
					Emit: &linodev1.WalkEmit{
						KindField: probeAbsentName, Action: probeWalkAction, Note: "n",
					},
				}),
		},
		{
			name:    "template naming a placeholder nothing fills",
			refusal: refusalWalkPlaceholder,
			build: statefulWalkProbe("ProbeWalkBadPlaceholderInput",
				&linodev1.DependencyWalk{
					StateMember: probeWalkMember,
					Emit: &linodev1.WalkEmit{
						Kind: probeWalkKind, Action: probeWalkAction, Note: "{nope}",
					},
				}),
		},
		{
			name:    "guard on an aggregate nothing computes",
			refusal: refusalWalkPlaceholder,
			build: statefulWalkProbe("ProbeWalkBadGuardInput",
				&linodev1.DependencyWalk{
					StateMember: probeWalkMember,
					Emit:        validEmit(),
					Warnings: []*linodev1.WalkWarning{
						{Template: "t", WhenPositive: probeAbsentName},
					},
				}),
		},
		{
			name:    "guard summing a field the element does not declare",
			refusal: refusalWalkPlaceholder,
			build: statefulWalkProbe("ProbeWalkBadSumGuardInput",
				&linodev1.DependencyWalk{
					StateMember: probeWalkMember,
					Emit:        validEmit(),
					Warnings: []*linodev1.WalkWarning{
						{Template: "t", WhenPositive: "sum:" + probeAbsentName},
					},
				}),
		},
		{
			name:    "presence predicate on a member the state does not carry",
			refusal: refusalWalkField,
			build: statefulWalkProbe("ProbeWalkBadPresenceInput",
				&linodev1.DependencyWalk{
					StateMember: probeWalkMember,
					Emit:        validEmit(),
					Warnings: []*linodev1.WalkWarning{
						{Template: "t", WhenPresent: probeAbsentName},
					},
				}),
		},
		{
			name:    "enrichment reading a list",
			refusal: "errWalkEnrichShape",
			build: enrichWalkProbe("ProbeWalkEnrichListInput", probePageResponse, probeEnrichPath,
				&linodev1.WalkEnrich{Tool: probeEnrichTool, Fields: []string{probeBodyArg}}),
		},
		{
			name:    "enrichment keeping a field the read does not declare",
			refusal: refusalWalkField,
			build: enrichWalkProbe("ProbeWalkEnrichFieldInput", probeGetResponse, probeEnrichPath,
				&linodev1.WalkEnrich{Tool: probeEnrichTool, Fields: []string{probeAbsentName}}),
		},
		{
			name:    "walk beside a warning-line preview sentence",
			refusal: "errWalkSentence",
			build:   sentenceWalkProbe("ProbeWalkSentenceShapeInput"),
		},
		{
			name:    "enrichment slot nothing fills",
			refusal: "errStateRouteSlotUnknownArgument",
			build: enrichWalkProbe("ProbeWalkEnrichSlotInput", probeGetResponse,
				"/vlans/{nobody_id}",
				&linodev1.WalkEnrich{Tool: probeEnrichTool, Fields: []string{probeBodyArg}}),
		},
	})
}

// statefulWalkProbe is a removal whose state is a declared read of a group
// with members, which is the shape the resolve-time cases refuse one detail
// of at a time.
func statefulWalkProbe(
	message string, walks ...*linodev1.DependencyWalk,
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		sibling := probeMessage(t, message+"Read",
			messageOptions(withRoute("GET", probeStateReadPath),
				withResponse("linode.mcp.v1.PlacementGroup"),
				withCapability(linodev1.ToolCapability_TOOL_CAPABILITY_READ),
				withDescription("Reads a probe.")),
			pathInt(probeDeleteArg))

		run := goProbe(probeMessage(t, message,
			destroyOptions(withStateRoute(&linodev1.StateRoute{Tool: probeStateReadTool}),
				withWalks(walks...)),
			pathInt(probeDeleteArg)))
		run.Beside = map[string]protoreflect.MessageDescriptor{probeStateReadTool: sibling}

		return run
	}
}

// sentenceWalkProbe is a stateful walk beside a warning-line preview
// sentence, which the closure seed has nowhere to say.
func sentenceWalkProbe(message string) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		sibling := probeMessage(t, message+"Read",
			messageOptions(withRoute("GET", probeStateReadPath),
				withResponse("linode.mcp.v1.PlacementGroup"),
				withCapability(linodev1.ToolCapability_TOOL_CAPABILITY_READ),
				withDescription("Reads a probe.")),
			pathInt(probeDeleteArg))

		run := goProbe(probeMessage(t, message,
			destroyOptions(withStateRoute(&linodev1.StateRoute{Tool: probeStateReadTool}),
				withWalks(validWalk()),
				withPreview(&linodev1.PreviewSentence{
					Template: []string{"never said"},
					Line:     linodev1.PreviewLine_PREVIEW_LINE_WARNING,
				})),
			pathInt(probeDeleteArg)))
		run.Beside = map[string]protoreflect.MessageDescriptor{probeStateReadTool: sibling}

		return run
	}
}

// probeWalkMember and probeWalkKind are the member and kind the cases share,
// and the refusal names the tables repeat.
const (
	probeWalkMember = "members"
	probeWalkKind   = "instance"
	probeWalkAction = "detached"

	refusalWalkSourceCount = "errWalkSourceCount"
	refusalWalkField       = "errDepWalkUnknownField"
	refusalWalkPlaceholder = "errDepWalkPlaceholder"

	probeEnrichTool = "probe_vlan_get"
	probeEnrichPath = "/vlans/{domain_id}"
)

// enrichWalkProbe is a stateful walk whose enrichment reads a second
// sibling, the shape the enrichment cases refuse one detail of at a time.
func enrichWalkProbe(
	message, response, path string, enrich *linodev1.WalkEnrich,
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		walk := validWalk()
		walk.Enrich = enrich

		run := statefulWalkProbe(message, walk)(t)
		run.Beside[probeEnrichTool] = probeMessage(t, message+"Enrich",
			messageOptions(withRoute("GET", path),
				withResponse(response),
				withCapability(linodev1.ToolCapability_TOOL_CAPABILITY_READ),
				withDescription("Reads a probe.")),
			pathInt(probeDeleteArg))

		return run
	}
}

// validWalk is a walk every structural rule accepts, for the cases whose
// refusal comes from what sits beside it.
func validWalk() *linodev1.DependencyWalk {
	return &linodev1.DependencyWalk{StateMember: probeWalkMember, Emit: validEmit()}
}

// validEmit is the smallest emission the rules accept.
func validEmit() *linodev1.WalkEmit {
	return &linodev1.WalkEmit{Kind: probeWalkKind, Action: probeWalkAction, Note: "n"}
}

// walkProbe is a removal carrying the declared walks the case states.
func depWalkProbe(
	message string,
	walks ...*linodev1.DependencyWalk,
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		return goProbe(probeMessage(t, message,
			destroyOptions(withWalks(walks...)), pathInt(probeDeleteArg)))
	}
}

func withWalks(walks ...*linodev1.DependencyWalk) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_DependencyWalk, walks)
	}
}

func withBilling(billing *linodev1.BillingDelta) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_BillingDelta, billing)
	}
}
