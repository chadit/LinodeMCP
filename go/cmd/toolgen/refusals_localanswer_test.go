package main_test

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The refusals a declared local answer raises, and the two trees one renders.
//
// No tool in the contract declares local_answer yet, so every case here
// synthesizes the declaration the same way every other refusal case does. That
// is also what makes the render cases worth having: they are the only place the
// two arms run over this option at all.

// The arguments a local-answer probe declares, and the numbers they take.
const (
	probeDraftArg    = "name"
	probePatternsArg = "tools"
	probeSinceArg    = "since"
	probeDraftNumber = 1
	probeListNumber  = 2
)

// The operation inputs a probe binds, named once so a case that perturbs a
// binding still spells the input the table declares.
const (
	probeDraftInput    = "draft"
	probePatternsInput = "patterns"
	probeGroupByInput  = "group_by"
	probeMetaArg       = "include_meta"
)

// The sentences a probe answers, spelled once so a case about a placeholder and
// a case about a guard read the same wording.
const (
	probeMissingSentence = "draft not found: {name}"
	probeReadSentence    = "failed to read audit log: {error}"
	probeSinceSentence   = "invalid 'since' timestamp: {error}"
)

// localAnswerOptions is a meta tool answering from a declared local operation
// rather than from a hook.
func localAnswerOptions(
	answer *linodev1.LocalAnswer, sets ...func(*descriptorpb.MessageOptions),
) *descriptorpb.MessageOptions {
	declared := make([]func(*descriptorpb.MessageOptions), 0, 6+len(sets))
	declared = append(declared,
		withMeta(),
		withCapability(metaCapability),
		withResponse(probeShapeFor(answer)),
		withDescription("Answers the probe from local state."),
		withCategories(&linodev1.ToolCategories{None: true}),
		withLocalAnswer(answer),
	)

	return messageOptions(append(declared, sets...)...)
}

func withLocalAnswer(answer *linodev1.LocalAnswer) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_LocalAnswer, answer)
	}
}

// probeShapes is the message each declared operation fills, keyed by the call
// and, where the operation is two-sided, by the direction it ran in.
//
// Stated as frozen literals rather than read off the emitter's own arm table,
// so a case renders only while the two agree. A call left out answers with the
// meta probe's own response, which the shape check then refuses by name.
func probeShapes() map[string]string {
	return map[string]string{
		"LOCAL_CALL_BUILD_INFO":                         "linode.mcp.v1.VersionResponse",
		"LOCAL_CALL_AUDIT_HEALTH":                       probeMetaResponse,
		"LOCAL_CALL_AUDIT_RECENT":                       "linode.mcp.v1.AuditRecentResponse",
		"LOCAL_CALL_AUDIT_SUMMARY":                      "linode.mcp.v1.AuditSummaryResponse",
		"LOCAL_CALL_AUDIT_EXPORT":                       "linode.mcp.v1.AuditExportResponse",
		"LOCAL_CALL_AUDIT_REPORT":                       "linode.mcp.v1.AuditReportResponse",
		"LOCAL_CALL_DRAFT_SET":                          "linode.mcp.v1.ProfileDraftSetResponse",
		"LOCAL_CALL_DRAFT_TOOLS|LOCAL_DIRECTION_ADD":    probeAddToolsResponse,
		"LOCAL_CALL_DRAFT_TOOLS|LOCAL_DIRECTION_REMOVE": "linode.mcp.v1.ProfileDraftRemoveToolsResponse",
		"LOCAL_CALL_CATALOG_CAN_RUN":                    probeCanRunResponse,
	}
}

// The two shapes a render case reads back off the emitted bytes, spelled here
// because the case names the Go type the handler builds.
const (
	probeAddToolsResponse = "linode.mcp.v1.ProfileDraftAddToolsResponse"
	probeCanRunResponse   = "linode.mcp.v1.ProfileCanRunResponse"
	probeDraftResponse    = "linode.mcp.v1.ProfileDraftResponse"

	// probeUnshaped is the table refusal every answer-shape case names.
	probeUnshaped = "errLocalArmUnshaped"
)

// probeShapeFor is the response one declaration's operation fills.
func probeShapeFor(answer *linodev1.LocalAnswer) string {
	keyed := answer.GetCall().String()
	if answer.GetDirection() != linodev1.LocalDirection_LOCAL_DIRECTION_UNSPECIFIED {
		keyed += "|" + answer.GetDirection().String()
	}

	if shape, stated := probeShapes()[keyed]; stated {
		return shape
	}

	return probeMetaResponse
}

// draftToolsAnswer is the two-sided draft operation as a well-formed
// declaration, which the cases below perturb one field at a time.
func draftToolsAnswer() *linodev1.LocalAnswer {
	return &linodev1.LocalAnswer{
		Call:      linodev1.LocalCall_LOCAL_CALL_DRAFT_TOOLS,
		Requires:  linodev1.LocalState_LOCAL_STATE_BUILDER,
		Direction: linodev1.LocalDirection_LOCAL_DIRECTION_ADD,
		Bind: []*linodev1.LocalBinding{
			{Input: probeDraftInput, Argument: probeDraftArg},
			{Input: probePatternsInput, Argument: probePatternsArg},
		},
		Refuse: []*linodev1.LocalRefusal{{
			Guard:    linodev1.LocalGuard_LOCAL_GUARD_DRAFT_MISSING,
			Argument: probeDraftArg,
			Message:  probeMissingSentence,
		}},
	}
}

// draftProbe is a well-formed draft-tools tool carrying whatever the case
// declares on top of it.
func draftProbe(
	message string, answer *linodev1.LocalAnswer, sets ...func(*descriptorpb.MessageOptions),
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		return goProbe(probeMessage(t, message, localAnswerOptions(answer, sets...),
			localToolString(probeDraftArg),
			localToolStringList(probePatternsArg)))
	}
}

// localToolString is one text argument a meta tool reads and no request
// carries. It is always the probe's first, which is why the number is fixed
// here rather than passed.
func localToolString(name string) *descriptorpb.FieldDescriptorProto {
	return probeField(name, probeDraftNumber, descriptorpb.FieldDescriptorProto_TYPE_STRING,
		fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_TOOL)))
}

// localToolStringList is one list argument a meta tool reads, always the
// probe's second.
func localToolStringList(name string) *descriptorpb.FieldDescriptorProto {
	return repeatedField(name, probeListNumber, descriptorpb.FieldDescriptorProto_TYPE_STRING,
		fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_TOOL)))
}

// TestRefusesALocalAnswerOnTheWrongTool covers the declarations that would give
// one tool two answers or put a local operation where a route already is.
func TestRefusesALocalAnswerOnTheWrongTool(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "a local answer on a tool that reaches a route",
			refusal: "errLocalAnswerTier",
			build:   writeProbe("ProbeLocalRoutedInput", withLocalAnswer(draftToolsAnswer())),
		},
		{
			name:    "a local answer beside a declared sentence",
			refusal: "errLocalAnswerTwoAnswers",
			build: draftProbe("ProbeLocalTwoAnswersInput", draftToolsAnswer(),
				withSuccessMessage("Probe is healthy")),
		},
		{
			name:    "a meta tool answering no way at all",
			refusal: "errNoMetaAnswer",
			build:   metaProbe("ProbeLocalNoAnswerAtAllInput", withoutSuccessMessage),
		},
	})
}

// TestRefusesALocalCallNothingServes covers the operation itself.
func TestRefusesALocalCallNothingServes(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "no call at all",
			refusal: "errNoLocalCall",
			build: draftProbe("ProbeLocalNoCallInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Call = linodev1.LocalCall_LOCAL_CALL_UNSPECIFIED
			})),
		},
		{
			name:    "state on an operation that reads none",
			refusal: "errLocalStateUnused",
			build: draftProbe("ProbeLocalStateUnusedInput", &linodev1.LocalAnswer{
				Call:     linodev1.LocalCall_LOCAL_CALL_BUILD_INFO,
				Requires: linodev1.LocalState_LOCAL_STATE_BUILDER,
			}),
		},
		{
			name:    "no state on an operation that reads some",
			refusal: "errLocalStateMismatch",
			build: draftProbe("ProbeLocalStateMissingInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Requires = linodev1.LocalState_LOCAL_STATE_UNSPECIFIED
			})),
		},
		{
			name:    "no direction on a two-sided operation",
			refusal: "errLocalDirectionUnserved",
			build: draftProbe("ProbeLocalNoDirectionInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Direction = linodev1.LocalDirection_LOCAL_DIRECTION_UNSPECIFIED
			})),
		},
		{
			name:    "a direction on a one-sided operation",
			refusal: "errLocalDirectionUnserved",
			build: draftProbe("ProbeLocalOneSidedDirectionInput", &linodev1.LocalAnswer{
				Call:      linodev1.LocalCall_LOCAL_CALL_BUILD_INFO,
				Direction: linodev1.LocalDirection_LOCAL_DIRECTION_ADD,
			}),
		},
	})
}

// TestRefusesAResponseTheOperationCannotFill is the guard's output half: an
// operation answers a plain body naming no message, so a tool declaring a
// response the operation cannot build is refused here or nowhere.
func TestRefusesAResponseTheOperationCannotFill(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			// The operation fills a tool list, and a discard answers whether
			// one draft was there. Nothing at call time would notice.
			name:    "a response the operation fills members outside of",
			refusal: "errLocalAnswerShape",
			build: draftProbe("ProbeLocalWrongShapeInput", draftToolsAnswer(),
				withResponse("linode.mcp.v1.ProfileDraftDiscardResponse")),
		},
		{
			// The other side of the same operation: one arm cannot build both
			// tools' answers, which is the case that made this check necessary.
			name:    "the other side's response over one direction",
			refusal: "errLocalAnswerShape",
			build: draftProbe("ProbeLocalCrossedSideInput", draftToolsAnswer(),
				withResponse("linode.mcp.v1.ProfileDraftRemoveToolsResponse")),
		},
	})
}

// TestTheShapeCheckReadsMembersNotNames pins what the comparison behind that
// refusal actually compares. Two messages carrying one member set under one set
// of kinds are the same shape whatever they are called, which is what lets two
// tools declare their own responses over one shared operation.
func TestTheShapeCheckReadsMembersNotNames(t *testing.T) {
	t.Parallel()

	cases := []struct {
		filled   proto.Message
		declared proto.Message
		name     string
		want     string
	}{
		{
			name:     "one message against itself",
			filled:   &linodev1.ProfileDraftResponse{},
			declared: &linodev1.ProfileDraftResponse{},
			want:     "",
		},
		{
			// The contract carries a member of its own type, so the walk has to
			// end on the pair it is already comparing.
			name:     "a message that reaches itself",
			filled:   &linodev1.FirewallDeviceEntity{},
			declared: &linodev1.FirewallDeviceEntity{},
			want:     "",
		},
		{
			// Two shapes carrying free-form maps, which report the kind each
			// key has to carry rather than the map itself.
			name:     "two names over one member set of maps",
			filled:   &linodev1.DatabaseMySQLInstanceCreateInput{},
			declared: &linodev1.DatabasePostgreSQLInstanceCreateInput{},
			want:     "",
		},
		{
			// Two differently named messages carrying {instance_id, message}.
			name:     "two names over one member set",
			filled:   &linodev1.InstanceDeleteResponse{},
			declared: &linodev1.InstancePowerActionResponse{},
			want:     "",
		},
		{
			name:     "a member the response does not declare",
			filled:   &linodev1.VolumeDeleteResponse{},
			declared: &linodev1.ImageShareGroupDeleteResponse{},
			want:     "fills volume_id, which it does not declare",
		},
		{
			name:     "a member the operation does not fill",
			filled:   &linodev1.ImageShareGroupDeleteResponse{},
			declared: &linodev1.VolumeDeleteResponse{},
			want:     "fills no volume_id",
		},
		{
			// One name, two forms: a list of text against a single flag.
			name:     "one member under two forms",
			filled:   &linodev1.FirewallAddresses{},
			declared: &linodev1.InterfaceDefaultRoute{},
			want:     "fills ipv4 as repeated string and it declares bool",
		},
		{
			// Same member names and forms, different messages inside, which
			// only walking into them finds.
			name:     "a nested member of another shape",
			filled:   &linodev1.InstanceConfigWriteResponse{},
			declared: &linodev1.NodeBalancerConfigWriteResponse{},
			want:     "fills label, which it does not declare",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := toolgen.ProbeLocalShapeDifference(testCase.filled, testCase.declared)
			if got != testCase.want {
				t.Errorf("difference = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestRefusesABindingNoOperationCanTake is the typed-parameter guard's own set:
// every value an operation gets comes through a binding it declares, of a kind
// its reader can build, so there is no declaration that hands one anything else.
func TestRefusesABindingNoOperationCanTake(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "a binding filling nothing",
			refusal: "errLocalBindNoInput",
			build: draftProbe("ProbeLocalBindNoInputInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Bind[0].Input = ""
			})),
		},
		{
			name:    "a binding filling an input the operation does not take",
			refusal: "errLocalBindUnknownInput",
			build: draftProbe("ProbeLocalBindUnknownInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Bind[0].Input = probeAbsentName
			})),
		},
		{
			name:    "a binding reading an argument the message does not declare",
			refusal: "errLocalBindUnknownArgument",
			build: draftProbe("ProbeLocalBindUnknownArgInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Bind[0].Argument = probeAbsentName
			})),
		},
		{
			name:    "one input filled twice",
			refusal: "errLocalBindRepeated",
			build: draftProbe("ProbeLocalBindRepeatedInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Bind[1].Input = probeDraftInput
			})),
		},
		{
			name:    "an input no binding fills",
			refusal: "errLocalBindMissing",
			build: draftProbe("ProbeLocalBindMissingInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Bind = answer.GetBind()[:1]
			})),
		},
		{
			name:    "a list filling an input read as text",
			refusal: "errLocalBindKind",
			build: draftProbe("ProbeLocalBindListForTextInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Bind[0].Argument = probePatternsArg
			})),
		},
		{
			name:    "text filling an input read as a list",
			refusal: "errLocalBindKind",
			build: draftProbe("ProbeLocalBindTextForListInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Bind[1].Argument = probeDraftArg
			})),
		},
	})
}

// TestRefusesAGuardLadderThatCannotAnswer covers the sentences.
func TestRefusesAGuardLadderThatCannotAnswer(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "a row answering no condition",
			refusal: "errLocalRefuseNoGuard",
			build: draftProbe("ProbeLocalRefuseNoGuardInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Refuse[0].Guard = linodev1.LocalGuard_LOCAL_GUARD_UNSPECIFIED
			})),
		},
		{
			name:    "a guard with no sentence",
			refusal: "errLocalRefuseSilent",
			build: draftProbe("ProbeLocalRefuseSilentInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Refuse[0].Message = ""
			})),
		},
		{
			name:    "a condition the operation cannot report",
			refusal: "errLocalRefuseGuardArm",
			build: draftProbe("ProbeLocalRefuseGuardArmInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Refuse[0].Guard = linodev1.LocalGuard_LOCAL_GUARD_BUILTIN_PROFILE
			})),
		},
		{
			name:    "one condition worded twice",
			refusal: "errLocalRefuseRepeated",
			build: draftProbe("ProbeLocalRefuseRepeatedInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Refuse = append(answer.GetRefuse(), &linodev1.LocalRefusal{
					Guard:    linodev1.LocalGuard_LOCAL_GUARD_DRAFT_MISSING,
					Argument: probeDraftArg,
					Message:  probeMissingSentence,
				})
			})),
		},
		{
			name:    "a condition the operation reports left unworded",
			refusal: "errLocalRefuseUnworded",
			build: draftProbe("ProbeLocalRefuseUnwordedInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Refuse = nil
			})),
		},
		{
			// Worded on the operation that does report a store failure, so the
			// case measures the argument rather than the guard.
			name:    "a store failure naming an argument it never reads",
			refusal: probeArgumentRefusal,
			build: summaryProbeWith("ProbeLocalRefuseStoreArgInput", func(answer *linodev1.LocalAnswer) {
				answer.Refuse[2].Argument = probeSinceArg
			}),
		},
		{
			name:    "a guard reading an argument no binding fills",
			refusal: probeArgumentRefusal,
			build: draftProbe("ProbeLocalRefuseUnboundArgInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Refuse[0].Argument = probeAbsentName
			})),
		},
		{
			name:    "a reader refusal on an argument whose reader cannot refuse",
			refusal: probeArgumentRefusal,
			build: draftProbe("ProbeLocalRefuseUnrefusableInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Refuse[0].Guard = linodev1.LocalGuard_LOCAL_GUARD_ARGUMENT_UNUSABLE
			})),
		},
		{
			// The reader answers for itself, so a timestamp with no row would
			// refuse a caller without saying why.
			name:    "a reading whose own refusal the ladder leaves unworded",
			refusal: "errLocalRefuseUnworded",
			build: summaryProbeWith("ProbeLocalReaderUnwordedInput", func(answer *linodev1.LocalAnswer) {
				answer.Refuse = answer.GetRefuse()[1:]
			}),
		},
		{
			name:    "a sentence naming a value nothing fills",
			refusal: "errLocalRefusePlaceholder",
			build: draftProbe("ProbeLocalRefusePlaceholderInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Refuse[0].Message = "draft not found: {" + probeAbsentName + "}"
			})),
		},
	})
}

// probeArgumentRefusal is the refusal three ladder cases are held to: a guard
// naming an argument its own shape does not allow.
const probeArgumentRefusal = "errLocalRefuseArgument"

// localAnswerWith is the well-formed draft-tools declaration with one field
// changed, which is how each case above says what it is about.
func localAnswerWith(change func(*linodev1.LocalAnswer)) *linodev1.LocalAnswer {
	answer := draftToolsAnswer()
	change(answer)

	return answer
}

// TestRefusesAnEngineTableTheContractCannotUse holds the arm table itself to
// the rules the whole vocabulary rests on. The real table is read off the
// contract's own members, so a synthesized one is the only way to reach these.
func TestRefusesAnEngineTableTheContractCannotUse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		refusal string
		arms    []toolgen.ProbeLocalArm
	}{
		{
			name:    "an operation the table has no entry for",
			refusal: "errUnservedLocalCall",
			arms:    []toolgen.ProbeLocalArm{{Call: probeDraftToolsMember, Missing: true}},
		},
		{
			// The whole typed-parameter guard, stated as a table rule: a type
			// outside the reader set has no reader, so nothing could fill it.
			name:    "an input outside the closed reader set",
			refusal: "errLocalArmInputKind",
			arms: []toolgen.ProbeLocalArm{{
				Call:   probeDraftToolsMember,
				Inputs: []toolgen.ProbeLocalInput{{Name: probeDraftInput, Kind: "arguments"}},
			}},
		},
		{
			name:    "a presence-carrying input no reader answers for",
			refusal: "errLocalArmInputKind",
			arms: []toolgen.ProbeLocalArm{{
				Call:   probeDraftToolsMember,
				Inputs: []toolgen.ProbeLocalInput{{Name: probeDraftInput, Kind: probeStringKind, Optional: true}},
			}},
		},
		{
			name:    "an input a declaration left unnamed",
			refusal: "errLocalArmInputUnnamed",
			arms: []toolgen.ProbeLocalArm{{
				Call:   probeDraftToolsMember,
				Inputs: []toolgen.ProbeLocalInput{{Kind: probeStringKind}},
			}},
		},
		{
			// A reading no arm asks about renders nothing, so a declaration
			// naming one would be dropped rather than acted on.
			name:    "an ambient reading outside the vocabulary",
			refusal: probeAmbientRefusal,
			arms: []toolgen.ProbeLocalArm{{
				Call:    probeDraftToolsMember,
				Ambient: []string{probeReadingUndeclared},
			}},
		},
		{
			name:    "one ambient reading declared twice",
			refusal: probeAmbientRefusal,
			arms: []toolgen.ProbeLocalArm{{
				Call:    probeDraftToolsMember,
				Ambient: []string{probeReadingConfig, probeReadingConfig},
			}},
		},
		{
			name:    "one input declared twice",
			refusal: "errLocalArmInputRepeated",
			arms: []toolgen.ProbeLocalArm{{
				Call: probeDraftToolsMember,
				Inputs: []toolgen.ProbeLocalInput{
					{Name: probeDraftInput, Kind: probeStringKind},
					{Name: probeDraftInput, Kind: probeStringKind},
				},
			}},
		},
		{
			// The output half of the guard, stated as a table rule: an
			// operation names no message, so the table is the only place its
			// answer has a shape at all.
			name:    "an operation shaping no answer",
			refusal: probeUnshaped,
			arms: []toolgen.ProbeLocalArm{{
				Call: probeDraftToolsMember,
			}},
		},
		{
			name:    "a two-sided operation shaping one side only",
			refusal: probeUnshaped,
			arms: []toolgen.ProbeLocalArm{{
				Call:     probeDraftToolsMember,
				TwoSided: true,
				Shapes:   map[string]proto.Message{probeDirectionAdd: &linodev1.ProfileDraftAddToolsResponse{}},
			}},
		},
		{
			name:    "a two-sided operation shaping a direction it does not run",
			refusal: probeUnshaped,
			arms: []toolgen.ProbeLocalArm{{
				Call:     probeDraftToolsMember,
				TwoSided: true,
				Shapes: map[string]proto.Message{
					probeDirectionAdd:   &linodev1.ProfileDraftAddToolsResponse{},
					probeDirectionUnset: &linodev1.ProfileDraftAddToolsResponse{},
				},
			}},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := toolgen.ProbeLocalArms(testCase.arms, []string{probeNamedTool})

			if want := refusalNamed(t, testCase.refusal); !errors.Is(err, want) {
				t.Errorf("refusal = %v, want %s (%v)", err, testCase.refusal, want)
			}
		})
	}
}

// probeDraftToolsMember is the call the table cases perturb, probeNamedTool is
// the tool an operation must not be named after, and probeStringKind is the
// reader set's spelling for text.
// The direction members a probe files a shape under.
const (
	probeDirectionUnset  = "LOCAL_DIRECTION_UNSPECIFIED"
	probeDirectionAdd    = "LOCAL_DIRECTION_ADD"
	probeDirectionRemove = "LOCAL_DIRECTION_REMOVE"
	// probeDirectionNone is the direction a one-sided operation runs, which
	// carries no member name at all.
	probeDirectionNone = ""
)

const (
	probeDraftToolsMember = "LOCAL_CALL_DRAFT_TOOLS"
	probeNamedTool        = "linode_profile_draft_add_tools"
	probeStringKind       = "string"
)

// Every operation the contract declares, spelled once for the whole test
// package: the cases that pin a derivation and the cases that state a
// hand-written split both name them, and goconst reads the package whole.
const (
	memberBuildInfo         = "LOCAL_CALL_BUILD_INFO"
	memberAuditHealth       = "LOCAL_CALL_AUDIT_HEALTH"
	memberAuditRecent       = "LOCAL_CALL_AUDIT_RECENT"
	memberAuditSummary      = "LOCAL_CALL_AUDIT_SUMMARY"
	memberAuditExport       = "LOCAL_CALL_AUDIT_EXPORT"
	memberAuditReport       = "LOCAL_CALL_AUDIT_REPORT"
	memberDraftCreate       = "LOCAL_CALL_DRAFT_CREATE"
	memberDraftRead         = "LOCAL_CALL_DRAFT_READ"
	memberDraftDiscard      = "LOCAL_CALL_DRAFT_DISCARD"
	memberDraftSet          = "LOCAL_CALL_DRAFT_SET"
	memberDraftSave         = "LOCAL_CALL_DRAFT_SAVE"
	memberCatalogTools      = "LOCAL_CALL_CATALOG_TOOLS"
	memberCatalogCategories = "LOCAL_CALL_CATALOG_CATEGORIES"
	memberCatalogCanRun     = "LOCAL_CALL_CATALOG_CAN_RUN"
)

// The ambient readings the table cases state. probeReadingUndeclared used to be
// the clock, which B9 declared under D-LG0-3; it names a session instead, since
// the case needs a reading the vocabulary does not carry and the clock is now
// one it does.
const (
	probeAmbientRefusal    = "errLocalArmAmbient"
	probeReadingConfig     = "config"
	probeReadingUndeclared = "session"
)

// TestRefusesAnAnswerShapeTheContractCannotResolve holds the shapes a
// declaration names.
//
// The table used to carry linked-in types the compiler checked. A declaration
// can only carry the name, so resolving it is where a shape that names nothing,
// or a direction shaped twice, has to be caught instead.
func TestRefusesAnAnswerShapeTheContractCannotResolve(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		answers []toolgen.ProbeLocalArmAnswer
	}{
		{
			name:    "a shape no message declares",
			answers: []toolgen.ProbeLocalArmAnswer{{Shape: "linode.mcp.v1.ProfileDraftNoSuchResponse"}},
		},
		{
			name:    "an answer naming no shape at all",
			answers: []toolgen.ProbeLocalArmAnswer{{}},
		},
		{
			// Left to the later row, the declaration would be saying two things
			// and the emitter would act on one of them without saying which.
			name: "one direction shaped twice",
			answers: []toolgen.ProbeLocalArmAnswer{
				{Direction: probeDirectionAdd, Shape: probeAddToolsResponse},
				{Direction: probeDirectionAdd, Shape: probeAddToolsResponse},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := toolgen.ProbeLocalArmBodies(probeDraftToolsMember, testCase.answers)

			if want := refusalNamed(t, probeUnshaped); !errors.Is(err, want) {
				t.Errorf("refusal = %v, want %s (%v)", err, probeUnshaped, want)
			}
		})
	}
}

// TestTheEngineTableAnswersForEveryDeclaredCall is the other half: the shipped
// table is what a run is held to, so it is read here rather than restated. A
// member added to the contract with no operation behind it fails this by name.
func TestTheEngineTableAnswersForEveryDeclaredCall(t *testing.T) {
	t.Parallel()

	if err := toolgen.ProbeLocalArms(nil, []string{probeNamedTool}); err != nil {
		t.Errorf("the engine table does not answer for every declared call: %v", err)
	}
}

// The languages the naming cases state: the two the registry carries, and one
// registered with a renderer whose spelling rule was never written.
const (
	probeLanguageGo      = "go"
	probeLanguagePython  = "python"
	probeLanguageUnruled = "rust"
)

// probeConfigLocal is the local both engines take the configuration under,
// which the forbidden-name case and the reading-spelling case both name.
const probeConfigLocal = "cfg"

// TestRefusesANamingNoEngineCouldServe holds the derivation itself. Names are
// derived from the declared member rather than stated per language, so these
// are the ways a run can reach a member no engine defines a function for.
func TestRefusesANamingNoEngineCouldServe(t *testing.T) {
	t.Parallel()

	registered := []string{probeLanguageGo, probeLanguagePython}

	cases := []struct {
		name      string
		refusal   string
		members   []string
		sided     []string
		languages []string
	}{
		{
			name:      "a registered language with no spelling rule",
			refusal:   "errLocalArmUnnamed",
			members:   []string{probeDraftToolsMember},
			languages: []string{probeLanguageGo, probeLanguagePython, probeLanguageUnruled},
		},
		{
			name:      "a member the vocabulary's own prefix does not open",
			refusal:   "errLocalArmUnstemmed",
			members:   []string{"DRAFT_TOOLS"},
			languages: registered,
		},
		{
			name:      "a member a doubled underscore leaves an empty word in",
			refusal:   "errLocalArmUnstemmed",
			members:   []string{"LOCAL_CALL_DRAFT__TOOLS"},
			languages: registered,
		},
		{
			// The rule the vocabulary rests on, now read off the member rather
			// than off a name someone typed beside it.
			name:      "a member named after the tool it serves",
			refusal:   "errLocalArmToolNamed",
			members:   []string{"LOCAL_CALL_LINODE_PROFILE_DRAFT_ADD_TOOLS"},
			languages: registered,
		},
		{
			// This repo excepts buf's upper-snake enum rule, so two members
			// differing only in case are declarable and stem alike.
			name:      "two members that stem to one function",
			refusal:   "errLocalArmSpellingShared",
			members:   []string{probeDraftToolsMember, "LOCAL_CALL_draft_tools"},
			languages: registered,
		},
		{
			// A two-sided member spells a function per direction, so its sides
			// can land on a member of their own. Go refuses the duplicate
			// declaration and Python lets the second one shadow the first, so
			// the emitter is what has to say no.
			name:      "a side that stems to a member's own function",
			refusal:   "errLocalArmSpellingShared",
			members:   []string{probeDraftToolsMember, "LOCAL_CALL_DRAFT_TOOLS_ADD"},
			sided:     []string{probeDraftToolsMember},
			languages: registered,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := toolgen.ProbeLocalArmNames(
				testCase.members,
				testCase.sided,
				testCase.languages,
				[]string{probeNamedTool},
			)

			if want := refusalNamed(t, testCase.refusal); !errors.Is(err, want) {
				t.Errorf("refusal = %v, want %s (%v)", err, testCase.refusal, want)
			}
		})
	}
}

// TestEveryOperationDerivesTheFunctionItsArmIsCalledThrough pins the derivation
// to the sixteen functions a handler really calls: the seven each engine still
// defines by hand, and the nine the emitted operations tree defines. Sixteen
// rather than fifteen because the two-sided operation is reached through one
// function per direction, which is what lets a generated arm fill one shape.
//
// Spelled out rather than computed: a derivation checked against itself would
// pass while renaming every call the emitter writes, and the two trees are what
// the emitted call has to land on.
func TestEveryOperationDerivesTheFunctionItsArmIsCalledThrough(t *testing.T) {
	t.Parallel()

	engines := []struct {
		member    string
		direction string
		goName    string
		pyName    string
	}{
		{memberBuildInfo, probeDirectionNone, "RunBuildInfo", "run_build_info"},
		{memberAuditHealth, probeDirectionNone, "RunAuditHealth", "run_audit_health"},
		{memberAuditRecent, probeDirectionNone, "RunAuditRecent", "run_audit_recent"},
		{memberAuditSummary, probeDirectionNone, "RunAuditSummary", "run_audit_summary"},
		{memberAuditExport, probeDirectionNone, "RunAuditExport", "run_audit_export"},
		{memberAuditReport, probeDirectionNone, "RunAuditReport", "run_audit_report"},
		{memberDraftCreate, probeDirectionNone, "RunDraftCreate", "run_draft_create"},
		{memberDraftRead, probeDirectionNone, "RunDraftRead", "run_draft_read"},
		{memberDraftDiscard, probeDirectionNone, "RunDraftDiscard", "run_draft_discard"},
		{memberDraftSet, probeDirectionNone, "RunDraftSet", "run_draft_set"},
		{probeDraftToolsMember, probeDirectionAdd, "RunDraftToolsAdd", "run_draft_tools_add"},
		{
			probeDraftToolsMember, probeDirectionRemove,
			"RunDraftToolsRemove", "run_draft_tools_remove",
		},
		{memberDraftSave, probeDirectionNone, "RunDraftSave", "run_draft_save"},
		{memberCatalogTools, probeDirectionNone, "RunCatalogTools", "run_catalog_tools"},
		{
			memberCatalogCategories, probeDirectionNone,
			"RunCatalogCategories", "run_catalog_categories",
		},
		{memberCatalogCanRun, probeDirectionNone, "RunCatalogCanRun", "run_catalog_can_run"},
	}

	// A member no name derives from spells nothing rather than guessing, which
	// is the answer the run refuses on ahead of any rendering.
	if got := toolgen.ProbeLocalArmSpelling(
		probeLanguageGo, "DRAFT_TOOLS", probeDirectionNone,
	); got != "" {
		t.Errorf("undeclared member spelling = %q, want empty", got)
	}

	for _, engine := range engines {
		t.Run(engine.member+engine.direction, func(t *testing.T) {
			t.Parallel()

			if got := toolgen.ProbeLocalArmSpelling(
				probeLanguageGo, engine.member, engine.direction,
			); got != engine.goName {
				t.Errorf("go spelling = %q, want %q", got, engine.goName)
			}

			if got := toolgen.ProbeLocalArmSpelling(
				probeLanguagePython, engine.member, engine.direction,
			); got != engine.pyName {
				t.Errorf("python spelling = %q, want %q", got, engine.pyName)
			}
		})
	}
}

// The two trees a well-formed declaration renders. Nothing in the contract
// declares local_answer yet, so these are the only place either arm runs over
// the option, and they are what stands in for a golden file until AV3 moves the
// first tool onto it.

// summaryAnswer is a one-sided operation reading a timestamp, which is the
// declaration that exercises a reader's own refusal beside the operation's.
func summaryAnswer() *linodev1.LocalAnswer {
	return &linodev1.LocalAnswer{
		Call: linodev1.LocalCall_LOCAL_CALL_AUDIT_SUMMARY,
		Bind: []*linodev1.LocalBinding{
			{Input: probeSinceArg, Argument: probeSinceArg},
			{Input: probeGroupByInput, Argument: probeGroupByInput},
			{Input: probeMetaArg, Argument: probeMetaArg},
		},
		Refuse: []*linodev1.LocalRefusal{
			{
				Guard:    linodev1.LocalGuard_LOCAL_GUARD_ARGUMENT_UNUSABLE,
				Argument: probeSinceArg,
				Message:  probeSinceSentence,
			},
			{
				Guard:    linodev1.LocalGuard_LOCAL_GUARD_INPUT_REJECTED,
				Argument: probeGroupByInput,
				Message:  "invalid group_by: {error}",
			},
			{Guard: linodev1.LocalGuard_LOCAL_GUARD_READ_FAILED, Message: probeReadSentence},
		},
	}
}

// summaryProbeWith is the audit-summary declaration with one field changed,
// for the cases whose subject is a condition draft_tools cannot report.
func summaryProbeWith(
	message string, change func(*linodev1.LocalAnswer),
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		answer := summaryAnswer()
		change(answer)

		return summaryProbeFor(t, message, answer)
	}
}

// summaryProbe is the audit-summary declaration and the arguments it binds.
func summaryProbe(t *testing.T, message string) *toolgen.ProbeRun {
	t.Helper()

	return summaryProbeFor(t, message, summaryAnswer())
}

// summaryProbeFor is one audit-summary declaration over the arguments it binds.
func summaryProbeFor(
	t *testing.T, message string, answer *linodev1.LocalAnswer,
) *toolgen.ProbeRun {
	t.Helper()

	return goProbe(probeMessage(t, message, localAnswerOptions(answer),
		localToolString(probeSinceArg),
		localToolStringList(probeGroupByInput),
		probeField(probeMetaArg, 3, descriptorpb.FieldDescriptorProto_TYPE_BOOL,
			fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_TOOL)))))
}

// TestRendersALocalAnswerInGo pins the shape the Go arm writes: the ambient
// state, one typed read per input, the operation, then a sentence per condition.
func TestRendersALocalAnswerInGo(t *testing.T) {
	t.Parallel()

	text := renderedGo(t, draftProbe("ProbeLocalRenderGoInput", draftToolsAnswer())(t))

	for _, want := range []string{
		"state, refusal := tools.BuilderStateOrRefusal(ctx)",
		`name := tools.LocalString(request, "name")`,
		// Renamed on purpose: the argument this tool declares is spelled like
		// the package every generated handler calls through, and the very next
		// line calls through it.
		`toolsArgument := tools.LocalStringList(request, "tools")`,
		"outcome := RunDraftToolsAdd(state, name, toolsArgument)",
		"if outcome.Refusal == tools.LocalRefusalDraftMissing {",
		`return mcp.NewToolResultError(fmt.Sprintf("draft not found: %v", name)), nil`,
		"return tools.LocalResponse(&linodev1.ProfileDraftAddToolsResponse{}, outcome.Body)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered Go does not carry %q:\n%s", want, text)
		}
	}
}

// TestRendersALocalReaderRefusalInGo pins the one refusal a reading answers for
// itself, beside the ones the operation reports.
func TestRendersALocalReaderRefusalInGo(t *testing.T) {
	t.Parallel()

	text := renderedGo(t, summaryProbe(t, "ProbeLocalRenderReaderInput"))

	for _, want := range []string{
		`since, sinceCause := tools.LocalTimestamp(request, "since")`,
		`if sinceCause != "" {`,
		`return mcp.NewToolResultError(fmt.Sprintf("invalid 'since' timestamp: %v", sinceCause)), nil`,
		"outcome := RunAuditSummary(ctx, cfg, tools.AuditSummary, since, groupBy, includeMeta)",
		"if outcome.Refusal == tools.LocalRefusalInputRejected {",
		`return mcp.NewToolResultError(fmt.Sprintf("failed to read audit log: %v", outcome.Cause)), nil`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered Go does not carry %q:\n%s", want, text)
		}
	}

	// The state a summary does not read is not resolved for it either.
	if strings.Contains(text, "BuilderStateOrRefusal") {
		t.Errorf("rendered Go resolves state the operation never reads:\n%s", text)
	}
}

// TestPyLocalBag is the typed-parameter guard read off the rendered bytes
// rather than off the checks: whatever the declaration says, the call carries
// only the locals the readers produced, and never the request, the argument map
// or the tool's own name.
//
// Named this short on purpose. A probe's file path is built from the test's own
// name, and the rendered module's docstring carries that path, so a longer name
// here trips the Python line budget before the case measures anything.
func TestPyLocalBag(t *testing.T) {
	t.Parallel()

	goText := renderedGo(t, draftProbe("ProbeLocalRenderNoBagInput", draftToolsAnswer())(t))
	pyText := renderedPython(t, probePyBag)

	for _, call := range []string{
		callLine(t, goText, "outcome := RunDraftToolsAdd("),
		callLine(t, pyText, "outcome = run_draft_tools_add("),
	} {
		for _, forbidden := range []string{"request", "arguments", probeToolName, probeConfigLocal} {
			if strings.Contains(call, forbidden) {
				t.Errorf("the operation call carries %q: %s", forbidden, call)
			}
		}
	}
}

// TestPyLocal pins the shape the Python arm writes, which is the same four
// steps through this language's own names. Named as short as TestPyLocalBag is,
// and for the same reason.
func TestPyLocal(t *testing.T) {
	t.Parallel()

	text := renderedPython(t, probePyShape)

	for _, want := range []string{
		"from linodemcp.tools.builderstate import (\n    BUILDER_UNCONFIGURED,\n" +
			"    builder_state_from_context,\n)",
		// The member order the linter's own import sort settles on, which the
		// generated tree is read by ruff under.
		"from linodemcp.gentools.operations import run_draft_tools_add",
		"from linodemcp.tools.local_answer import (\n    LocalRefusal,\n" +
			"    local_response,\n    local_string,\n    local_string_list,\n)",
		"    state = builder_state_from_context()",
		"    if state is None:",
		"        return error_response(BUILDER_UNCONFIGURED)",
		`    name = local_string(arguments, "name")`,
		`    tools = local_string_list(arguments, "tools")`,
		"    outcome = run_draft_tools_add(state, name, tools)",
		// Python holds no name spelled "tools" in this module, so the read
		// keeps the argument's own spelling where the Go arm renames it.
		"    if outcome.refusal is LocalRefusal.DRAFT_MISSING:",
		`        return error_response(f"draft not found: {name}")`,
		"    return local_response(",
		`        "` + probeAddToolsResponse + `",`,
		"        outcome.body,",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered Python does not carry %q:\n%s", want, text)
		}
	}

	// The arm renders through ruff format, which leaves import order alone, so
	// the order this arm chooses is only ever read by the lint gate. Reading it
	// here is what caught the members sorting the wrong way.
	checkedByRuff(t, text)
}

// checkedByRuff holds one rendered module to the linter the Python gate runs
// over the generated tree.
func checkedByRuff(t *testing.T, text string) {
	t.Helper()

	command := exec.CommandContext(t.Context(), pythonRuff, "check",
		"--stdin-filename", pythonPackageDir+"/probe.py", "-")
	command.Stdin = strings.NewReader(text)

	if out, err := command.CombinedOutput(); err != nil {
		t.Errorf("the rendered module does not pass the Python lint gate: %v\n%s", err, out)
	}
}

// The suffix the Go arm's files carry, and the function that appears in the
// rendered module rather than in the registry beside it.
const (
	goModuleSuffix = ".gen.go"
	goProbeHandler = "func handleLinodeProbe("
)

// TestPyLocalShade is the Python half of the shadowing rule the Go case above
// pins: an argument spelling a name the handler already holds is read into a
// renamed local rather than allowed to take it. No tool declares one today,
// which is why the case synthesizes it. Named short for the docstring budget.
func TestPyLocalShade(t *testing.T) {
	t.Parallel()

	answer := draftToolsAnswer()
	answer.Bind[0].Argument = probeShadowArg
	answer.Refuse[0].Argument = probeShadowArg
	answer.Refuse[0].Message = "draft not found: {" + probeShadowArg + "}"

	run := pythonProbe(t, probeRegistered(t, probePyShade, localAnswerOptions(answer),
		localToolString(probeShadowArg),
		localToolStringList(probePatternsArg)))

	files, err := run.Emit()
	if err != nil {
		t.Fatalf("emit: %v", err)
	}

	text := pythonModule(t, files)

	for _, want := range []string{
		`    state_argument = local_string(arguments, "state")`,
		"    outcome = run_draft_tools_add(state, state_argument, tools)",
		`        return error_response(f"draft not found: {state_argument}")`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered Python does not carry %q:\n%s", want, text)
		}
	}
}

// probeShadowArg is the argument the shadowing case declares, and probePyShade
// is the message it synthesizes.
const (
	probeShadowArg = "state"
	probePyShade   = "ProbeSh"
)

// renderedGo is the Go module one probe renders, failing the case rather than
// returning an error, since every case here declares a well-formed answer.
func renderedGo(t *testing.T, run *toolgen.ProbeRun) string {
	t.Helper()

	files, err := run.Emit()
	if err != nil {
		t.Fatalf("emit: %v", err)
	}

	// The registry is rendered beside the module and carries the same suffix,
	// so the handler is what says which of the two this is. Map order is not
	// settled, so picking by suffix alone reads whichever came first.
	for name, text := range files {
		if strings.HasSuffix(name, goModuleSuffix) && strings.Contains(text, goProbeHandler) {
			return text
		}
	}

	t.Fatal("the run rendered no Go module")

	return ""
}

// The messages the Python render cases synthesize, one each because a
// synthesized message registers under its full name and two cases cannot claim
// one. Both are short because the probe's file path is built from the message
// and the test's name together, and the rendered docstring carries that path.
const (
	probePyShape = "ProbeShape"
	probePyBag   = "ProbeBag"
)

// renderedPython is the Python module one probe renders, through the repo's own
// formatter the way `make proto` runs it.
func renderedPython(t *testing.T, message string) string {
	t.Helper()

	run := pythonProbe(t, probeRegistered(t, message, localAnswerOptions(draftToolsAnswer()),
		localToolString(probeDraftArg),
		localToolStringList(probePatternsArg)))

	files, err := run.Emit()
	if err != nil {
		t.Fatalf("emit: %v", err)
	}

	return pythonModule(t, files)
}

// pythonModule is the rendered module rather than the registry beside it. Map
// order is not settled, so the handler is what says which of the two this is.
func pythonModule(t *testing.T, files map[string]string) string {
	t.Helper()

	for name, text := range files {
		if strings.HasPrefix(name, "python/") && strings.Contains(text, pyProbeHandler) {
			return text
		}
	}

	t.Fatal("the run rendered no Python module")

	return ""
}

// pyProbeHandler is the function that appears in the rendered Python module
// rather than in the registry beside it.
const pyProbeHandler = "async def handle_linode_probe("

// callLine is the one rendered line opening the operation's call, which is
// where a bag would have to appear if one could reach an operation at all.
func callLine(t *testing.T, text, opening string) string {
	t.Helper()

	for line := range strings.SplitSeq(text, "\n") {
		if strings.Contains(line, opening) {
			return line
		}
	}

	t.Fatalf("no rendered line opens %q:\n%s", opening, text)

	return ""
}

// The reader kinds and declaration forms the two cases above leave untouched,
// each rendered once so no reader or form ships unread. Go only: the Python arm
// renders through the repo's formatter, and the two cases above already prove
// the two arms write the same four steps.
func TestRendersEveryLocalReaderKind(t *testing.T) {
	t.Parallel()

	cases := []struct {
		build   func(t *testing.T) *toolgen.ProbeRun
		name    string
		message string
		wanted  []string
	}{
		{
			name:    "the other side of a two-sided operation",
			message: "ProbeLocalRemoveInput",
			build: draftProbe("ProbeLocalRemoveInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Direction = linodev1.LocalDirection_LOCAL_DIRECTION_REMOVE
			})),
			wanted: []string{"outcome := RunDraftToolsRemove(state, name, toolsArgument)"},
		},
		{
			name:    "a sentence answering in quotes",
			message: "ProbeLocalQuotedInput",
			build:   reportProbe("ProbeLocalQuotedInput"),
			wanted: []string{
				`report := tools.LocalString(request, "report")`,
				// The clock is the reading no handler holds under a local, so
				// the call site is where this language reads its own seam.
				"outcome := RunAuditReport(ctx, cfg, tools.ClockFromContext(ctx), tools.AuditReport, report)",
				"if outcome.Refusal == tools.LocalRefusalNotFound {",
				`mcp.NewToolResultError(fmt.Sprintf("unknown report: %q", report)), nil`,
			},
		},
		{
			name:    "the presence-carrying readers a setter binds",
			message: "ProbeLocalSetterInput",
			build:   setterProbe("ProbeLocalSetterInput"),
			wanted: []string{
				`allowedEnvironments := tools.LocalStringListSent(request, "allowed_environments")`,
				`allowYolo := tools.LocalBoolSent(request, "allow_yolo")`,
				"outcome := RunDraftSet(state, name, allowedEnvironments, " +
					"requiredScopes, allowYolo)",
			},
		},
		{
			// A brace the sentence never closes is text, not a placeholder, so
			// the refusal renders as the literal a caller would read.
			name:    "a sentence whose brace never closes",
			message: "ProbeLocalUnclosedInput",
			build: draftProbe("ProbeLocalUnclosedInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Refuse[0].Message = "draft not found: {name"
			})),
			wanted: []string{`mcp.NewToolResultError("draft not found: {name"), nil`},
		},
		{
			// A percent in a sentence that also names a value has to reach
			// Sprintf doubled, or it opens a verb nothing fills.
			name:    "a sentence carrying a literal percent",
			message: "ProbeLocalPercentInput",
			build: draftProbe("ProbeLocalPercentInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Refuse[0].Message = "draft not found (100% checked): {name}"
			})),
			wanted: []string{
				`fmt.Sprintf("draft not found (100%% checked): %v", name)`,
			},
		},
		{
			// The same sentence with nothing to fill stays a plain literal, so
			// the percent is not doubled there.
			name:    "a plain sentence carrying a literal percent",
			message: "ProbeLocalPlainPercentInput",
			build: draftProbe("ProbeLocalPlainPercentInput", localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Refuse[0].Message = "draft not found (100% checked)"
			})),
			wanted: []string{`mcp.NewToolResultError("draft not found (100% checked)"), nil`},
		},
		{
			name:    "the typed entries a pre-check binds",
			message: "ProbeLocalCallsInput",
			build:   canRunProbe("ProbeLocalCallsInput"),
			wanted: []string{
				`calls := tools.LocalCallEntries(request, "calls")`,
				"outcome := RunCatalogCanRun(state, calls)",
				"return tools.LocalResponse(&linodev1.ProfileCanRunResponse{}, outcome.Body)",
			},
		},
		{
			// The call list is the only place the arm table's ambient
			// parameters show: audit-recent reads the log alone, so it takes
			// no context where the two store queries do.
			name:    "the number and flag readers a query binds",
			message: "ProbeLocalQueryInput",
			build:   recentProbe("ProbeLocalQueryInput"),
			wanted: []string{
				`limit := tools.LocalInt(request, "limit")`,
				`includeMeta := tools.LocalBool(request, "include_meta")`,
				"outcome := RunAuditRecent(tools.AuditRecent, limit, since, until, " +
					"tool, capability, status, includeMeta)",
			},
		},
		{
			// The mirror of the case above, and the one argument Go reads
			// under its plain spelling while Python has to rename it.
			name:    "a store query that takes the context and the configuration",
			message: "ProbeLocalExportInput",
			build:   exportProbe("ProbeLocalExportInput"),
			wanted: []string{
				`format := tools.LocalString(request, "format")`,
				"outcome := RunAuditExport(ctx, cfg, tools.AuditExport, format, since, until, " +
					"tool, maxRecords, includeMeta)",
				"if outcome.Refusal == tools.LocalRefusalWriteFailed {",
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			text := renderedGo(t, testCase.build(t))
			for _, want := range testCase.wanted {
				if !strings.Contains(text, want) {
					t.Errorf("rendered Go does not carry %q:\n%s", want, text)
				}
			}
		})
	}
}

// reportProbe is the audit-report declaration, which is the one that answers a
// name in quotes.
func reportProbe(message string) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		return goProbe(probeMessage(t, message, localAnswerOptions(reportAnswer()),
			localToolString(probeReportInput)))
	}
}

// reportAnswer is the audit-report declaration: ambient config and clock, one
// text input, and a sentence that answers a name in quotes.
func reportAnswer() *linodev1.LocalAnswer {
	return &linodev1.LocalAnswer{
		Call: linodev1.LocalCall_LOCAL_CALL_AUDIT_REPORT,
		Bind: []*linodev1.LocalBinding{{Input: probeReportInput, Argument: probeReportInput}},
		Refuse: []*linodev1.LocalRefusal{
			{
				Guard:    linodev1.LocalGuard_LOCAL_GUARD_NOT_FOUND,
				Argument: probeReportInput,
				Message:  "unknown report: {" + probeReportInput + ":quoted}",
			},
			{
				Guard:    linodev1.LocalGuard_LOCAL_GUARD_INPUT_REJECTED,
				Argument: probeReportInput,
				Message:  "failed to run report: {error}",
			},
			{Guard: linodev1.LocalGuard_LOCAL_GUARD_READ_FAILED, Message: probeReadSentence},
		},
	}
}

// setterProbe is the draft-set declaration, whose three optional settings are
// the only place a presence-carrying reader is bound.
func setterProbe(message string) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		return goProbe(probeMessage(t, message, localAnswerOptions(&linodev1.LocalAnswer{
			Call:     linodev1.LocalCall_LOCAL_CALL_DRAFT_SET,
			Requires: linodev1.LocalState_LOCAL_STATE_BUILDER,
			Bind: []*linodev1.LocalBinding{
				{Input: probeDraftInput, Argument: probeDraftArg},
				{Input: "environments", Argument: "allowed_environments"},
				{Input: "scopes", Argument: "required_scopes"},
				{Input: "yolo", Argument: "allow_yolo"},
			},
			Refuse: []*linodev1.LocalRefusal{{
				Guard:    linodev1.LocalGuard_LOCAL_GUARD_DRAFT_MISSING,
				Argument: probeDraftArg,
				Message:  probeMissingSentence,
			}},
		}),
			localToolString(probeDraftArg),
			localToolStringList("allowed_environments"),
			repeatedField("required_scopes", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING,
				fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_TOOL))),
			probeField("allow_yolo", 4, descriptorpb.FieldDescriptorProto_TYPE_BOOL,
				fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_TOOL)))))
	}
}

// exportAnswer is the audit-export declaration. It is the one that binds an
// argument the Python language already owns, and the one whose operation
// reports a write failure beside a read failure.
func exportAnswer() *linodev1.LocalAnswer {
	return &linodev1.LocalAnswer{
		Call: linodev1.LocalCall_LOCAL_CALL_AUDIT_EXPORT,
		Bind: []*linodev1.LocalBinding{
			{Input: probeFormatArg, Argument: probeFormatArg},
			{Input: probeSinceArg, Argument: probeSinceArg},
			{Input: probeUntilArg, Argument: probeUntilArg},
			{Input: probeToolGlobArg, Argument: probeToolGlobArg},
			{Input: probeMaxRecordsArg, Argument: probeMaxRecordsArg},
			{Input: probeMetaArg, Argument: probeMetaArg},
		},
		Refuse: []*linodev1.LocalRefusal{
			{
				Guard:    linodev1.LocalGuard_LOCAL_GUARD_ARGUMENT_UNUSABLE,
				Argument: probeSinceArg,
				Message:  probeSinceSentence,
			},
			{
				Guard:    linodev1.LocalGuard_LOCAL_GUARD_ARGUMENT_UNUSABLE,
				Argument: probeUntilArg,
				Message:  "invalid 'until' timestamp: {error}",
			},
			{Guard: linodev1.LocalGuard_LOCAL_GUARD_READ_FAILED, Message: probeReadSentence},
			{
				Guard:   linodev1.LocalGuard_LOCAL_GUARD_WRITE_FAILED,
				Message: "failed to write export file: {error}",
			},
		},
	}
}

// The audit-export arguments the two render cases bind, spelled here because
// one of them is the whole subject: `format` is a Python builtin.
const (
	probeFormatArg     = "format"
	probeUntilArg      = "until"
	probeToolGlobArg   = "tool"
	probeMaxRecordsArg = "max_records"
)

// exportArguments is one argument per audit-export input, each of the kind its
// reader takes.
func exportArguments() []*descriptorpb.FieldDescriptorProto {
	text := func(name string, number int32) *descriptorpb.FieldDescriptorProto {
		return probeField(name, number, descriptorpb.FieldDescriptorProto_TYPE_STRING,
			fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_TOOL)))
	}

	return []*descriptorpb.FieldDescriptorProto{
		text(probeFormatArg, 1),
		text(probeSinceArg, 2),
		text(probeUntilArg, 3),
		text(probeToolGlobArg, 4),
		probeField(probeMaxRecordsArg, 5, descriptorpb.FieldDescriptorProto_TYPE_INT32,
			fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_TOOL))),
		probeField(probeMetaArg, 6, descriptorpb.FieldDescriptorProto_TYPE_BOOL,
			fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_TOOL))),
	}
}

// exportProbe is the audit-export declaration as a Go render.
func exportProbe(message string) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		return goProbe(probeMessage(t, message,
			localAnswerOptions(exportAnswer()), exportArguments()...))
	}
}

// recentProbe is the audit-recent declaration, which binds every reader kind a
// query argument comes in.
func recentProbe(message string) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		inputs := probeRecentInputs()

		binds := make([]*linodev1.LocalBinding, 0, len(inputs))
		for _, input := range inputs {
			binds = append(binds, &linodev1.LocalBinding{Input: input, Argument: input})
		}

		return goProbe(probeMessage(t, message, localAnswerOptions(&linodev1.LocalAnswer{
			Call: linodev1.LocalCall_LOCAL_CALL_AUDIT_RECENT,
			Bind: binds,
			Refuse: []*linodev1.LocalRefusal{
				{
					Guard:    linodev1.LocalGuard_LOCAL_GUARD_ARGUMENT_UNUSABLE,
					Argument: probeSinceArg,
					Message:  probeSinceSentence,
				},
				{
					Guard:    linodev1.LocalGuard_LOCAL_GUARD_ARGUMENT_UNUSABLE,
					Argument: "until",
					Message:  "invalid 'until' timestamp: {error}",
				},
				{Guard: linodev1.LocalGuard_LOCAL_GUARD_READ_FAILED, Message: probeReadSentence},
			},
		}), recentArguments()...))
	}
}

// probeRecentInputs is the audit-recent operation's inputs, which the probe
// binds one argument of the same name to each of.
func probeRecentInputs() []string {
	return []string{
		"limit", probeSinceArg, "until", probeToolGlobArg, "capability", "status", probeMetaArg,
	}
}

// probeReportInput is the audit-report operation's one input, and
// probeCallsInput is the pre-check's.
const (
	probeReportInput = "report"
	probeCallsInput  = "calls"
)

// canRunProbe is the pre-check declaration, whose one input is a list of typed
// entries rather than a scalar or a list of text.
func canRunProbe(message string) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		entries := probeField(probeCallsInput, 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE,
			fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_TOOL)))
		entries.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
		entries.TypeName = new(probeItemMarker)

		return goProbe(probeMessage(t, message, localAnswerOptions(&linodev1.LocalAnswer{
			Call:     linodev1.LocalCall_LOCAL_CALL_CATALOG_CAN_RUN,
			Requires: linodev1.LocalState_LOCAL_STATE_BUILDER,
			Bind: []*linodev1.LocalBinding{
				{Input: probeCallsInput, Argument: probeCallsInput},
			},
		}), entries))
	}
}

// recentArguments is one argument per audit-recent input, each of the kind its
// reader takes.
func recentArguments() []*descriptorpb.FieldDescriptorProto {
	toolArg := func(
		name string, number int32, kind descriptorpb.FieldDescriptorProto_Type,
	) *descriptorpb.FieldDescriptorProto {
		return probeField(name, number, kind,
			fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_TOOL)))
	}

	return []*descriptorpb.FieldDescriptorProto{
		toolArg("limit", 1, descriptorpb.FieldDescriptorProto_TYPE_INT32),
		toolArg(probeSinceArg, 2, descriptorpb.FieldDescriptorProto_TYPE_STRING),
		toolArg("until", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING),
		toolArg("tool", 4, descriptorpb.FieldDescriptorProto_TYPE_STRING),
		toolArg("capability", 5, descriptorpb.FieldDescriptorProto_TYPE_STRING),
		toolArg("status", 6, descriptorpb.FieldDescriptorProto_TYPE_STRING),
		toolArg(probeMetaArg, 7, descriptorpb.FieldDescriptorProto_TYPE_BOOL),
	}
}

// The Python arm over the declaration forms the two cases above leave out: the
// other direction of a two-sided operation, the readers a store query binds,
// and a sentence that answers in quotes. Run as a plain loop rather than
// subtests because a probe's file path is built from the test's name, and a
// subtest's name would push the rendered docstring past the line budget.
func TestPyLocalKinds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		message string
		answer  *linodev1.LocalAnswer
		fields  []*descriptorpb.FieldDescriptorProto
		wanted  []string
	}{
		{
			// The sentence names nothing, which is the other form a refusal
			// renders in: a plain literal rather than an f-string.
			name:    "the other side of a two-sided operation",
			message: "ProbeRem",
			answer: localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Direction = linodev1.LocalDirection_LOCAL_DIRECTION_REMOVE
				answer.Refuse[0].Message = "draft not found"
			}),
			fields: []*descriptorpb.FieldDescriptorProto{
				localToolString(probeDraftArg), localToolStringList(probePatternsArg),
			},
			wanted: []string{
				"run_draft_tools_remove(state, name, tools)",
				`        return error_response("draft not found")`,
			},
		},
		{
			name:    "a sentence answering in quotes over ambient config and the clock",
			message: "ProbeRep",
			answer:  reportAnswer(),
			fields:  []*descriptorpb.FieldDescriptorProto{localToolString(probeReportInput)},
			wanted: []string{
				// The seam import is the half a reading rendered as an
				// expression adds: without it the call names nothing.
				"from linodemcp.tools.clock import now",
				"    outcome = run_audit_report(cfg, now(), audit_report, report)",
				"    if outcome.refusal is LocalRefusal.NOT_FOUND:",
				`        return error_response(f"unknown report: {local_quoted(report)}")`,
				`        return error_response(f"failed to read audit log: {outcome.cause}")`,
			},
		},
		{
			name:    "a sentence whose brace never closes",
			message: "ProbeUnc",
			answer: localAnswerWith(func(answer *linodev1.LocalAnswer) {
				answer.Refuse[0].Message = "draft not found: {name"
			}),
			fields: []*descriptorpb.FieldDescriptorProto{
				localToolString(probeDraftArg), localToolStringList(probePatternsArg),
			},
			wanted: []string{`        return error_response("draft not found: {name")`},
		},
		{
			// `format` is a Python builtin, so the read has to be renamed or
			// the generated module fails the lint gate on A001. Go renames
			// nothing here, which is why this case is Python's alone.
			name:    "an argument the language already owns",
			message: "ProbeExp",
			answer:  exportAnswer(),
			fields:  exportArguments(),
			wanted: []string{
				`    format_argument = local_string(arguments, "format")`,
				// The renamed local reaches the call under its new spelling,
				// behind the subsystem the handler hands over.
				"        audit_export,\n        format_argument,\n        since,",
				`        return error_response(f"failed to write export file: {outcome.cause}")`,
			},
		},
		{
			name:    "the readers a store query binds",
			message: "ProbeSum",
			answer:  summaryAnswer(),
			fields: []*descriptorpb.FieldDescriptorProto{
				localToolString(probeSinceArg), localToolStringList(probeGroupByInput),
				probeField(probeMetaArg, 3, descriptorpb.FieldDescriptorProto_TYPE_BOOL,
					fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_TOOL))),
			},
			wanted: []string{
				`    since, since_cause = local_timestamp(arguments, "since")`,
				"    if since_cause:",
				`        return error_response(f"invalid 'since' timestamp: {since_cause}")`,
				`    include_meta = local_bool(arguments, "include_meta")`,
				"    outcome = run_audit_summary(cfg, audit_summary, since, group_by, include_meta)",
				"    if outcome.refusal is LocalRefusal.INPUT_REJECTED:",
			},
		},
	}

	for _, testCase := range cases {
		run := pythonProbe(t, probeRegistered(t, testCase.message,
			localAnswerOptions(testCase.answer), testCase.fields...))

		files, err := run.Emit()
		if err != nil {
			t.Fatalf("%s: emit: %v", testCase.name, err)
		}

		text := pythonModule(t, files)
		for _, want := range testCase.wanted {
			if !strings.Contains(text, want) {
				t.Errorf("%s: rendered Python does not carry %q:\n%s", testCase.name, want, text)
			}
		}
	}
}
