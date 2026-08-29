package main_test

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The object_walk refusals. Each names a declaration the walker would read and
// drop, which is the one failure a declared check can have that nothing
// downstream reports: the call goes through and the caller is told nothing.

// The open object a walk reads, and the list beside it, spelled once because a
// case names the same argument in the declaration and in the walk.
const (
	probeObjectArg = "devices"
	probeListArg   = "interfaces"
)

// The one key a walk's vocabulary carries and the sentence its value refuses
// with, spelled once because most cases below declare both.
const (
	probeWalkKey             = "sda"
	probeWalkUnknownSentence = "unknown device slot {key}"
	probeWalkSentence        = "{key} must be text"
)

// TestRefusesAWalkTheArgumentsCannotAnswer covers where a walk may be declared.
func TestRefusesAWalkTheArgumentsCannotAnswer(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "walk on a tier with no argument walk",
			refusal: "errWalkTier",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeWalkTierInput",
					getOptions(withWalk(&linodev1.ObjectWalk{
						Field: []string{probeIDArg}, Unusable: "probe_id must be an object",
					})),
					pathInt(probeIDArg)))
			},
		},
		{
			name:    "walk naming no argument",
			refusal: "errWalkNoField",
			build:   walkProbe("ProbeWalkNoFieldInput", &linodev1.ObjectWalk{Unusable: "must be an object"}),
		},
		{
			name:    "walk naming an argument the message does not declare",
			refusal: "errWalkUnknownField",
			build: walkProbe("ProbeWalkUnknownFieldInput", &linodev1.ObjectWalk{
				Field: []string{probeAbsentName}, Unusable: "nobody must be an object",
			}),
		},
		{
			name:    "two walks reading one argument",
			refusal: "errWalkRepeatedField",
			build: walkProbe("ProbeWalkRepeatedFieldInput",
				&linodev1.ObjectWalk{
					Field: []string{probeObjectArg}, Unusable: "devices must be an object",
				},
				&linodev1.ObjectWalk{
					Field: []string{probeObjectArg}, Absent: "devices is required",
				}),
		},
		{
			name:    "walk that refuses nothing",
			refusal: "errWalkSilent",
			build: walkProbe("ProbeWalkSilentInput", &linodev1.ObjectWalk{
				Field: []string{probeObjectArg},
			}),
		},
		{
			name:    "element sentence on an argument with no entries",
			refusal: "errWalkElementOnMap",
			build: walkProbe("ProbeWalkElementOnMapInput", &linodev1.ObjectWalk{
				Field: []string{probeObjectArg}, Element: "devices must be an array of objects",
			}),
		},
		{
			name:    "empty sentence on a list that cannot answer it",
			refusal: "errWalkEmptyOnList",
			build: walkProbe("ProbeWalkEmptyOnListInput", &linodev1.ObjectWalk{
				Field: []string{probeListArg}, Empty: "interfaces must not be empty",
			}),
		},
	})
}

// TestRefusesAWalkMemberSpecNothingReads covers the members a walk declares
// inside the object it reads.
func TestRefusesAWalkMemberSpecNothingReads(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "shared value with no key to apply it to",
			refusal: "errWalkValueWithoutKey",
			build: walkProbe("ProbeWalkValueWithoutKeyInput", &linodev1.ObjectWalk{
				Field: []string{probeObjectArg},
				Members: &linodev1.ObjectMembers{
					Value: &linodev1.ObjectValue{
						Kind: linodev1.ValueKind_VALUE_KIND_TEXT, Unusable: probeWalkSentence,
					},
				},
			}),
		},
		{
			name:    "one key declared twice",
			refusal: "errWalkDuplicateKey",
			build: walkProbe("ProbeWalkDuplicateKeyInput", &linodev1.ObjectWalk{
				Field: []string{probeObjectArg},
				Members: &linodev1.ObjectMembers{
					Key: []string{probeWalkKey},
					Value: &linodev1.ObjectValue{
						Kind: linodev1.ValueKind_VALUE_KIND_TEXT, Unusable: probeWalkSentence,
					},
					Member: []*linodev1.ObjectMember{{
						Name: probeWalkKey,
						Value: &linodev1.ObjectValue{
							Kind: linodev1.ValueKind_VALUE_KIND_TEXT, Unusable: probeWalkSentence,
						},
					}},
				},
			}),
		},
		{
			name:    "unknown sentence over an empty vocabulary",
			refusal: "errWalkUnknownWithoutVocabulary",
			build: walkProbe("ProbeWalkUnknownVocabularyInput", &linodev1.ObjectWalk{
				Field:   []string{probeObjectArg},
				Members: &linodev1.ObjectMembers{Unknown: probeWalkUnknownSentence},
			}),
		},
		{
			name:    "require naming a key the vocabulary does not carry",
			refusal: "errWalkRequireUnknownName",
			build: walkProbe("ProbeWalkRequireUnknownInput", &linodev1.ObjectWalk{
				Field: []string{probeObjectArg},
				Members: &linodev1.ObjectMembers{
					Unknown: probeWalkUnknownSentence,
					Key:     []string{probeWalkKey},
					Require: &linodev1.ObjectRequire{Name: []string{"sdb"}, AnyOf: "sdb is required"},
				},
			}),
		},
		{
			name:    "bound on a value kind that reads none",
			refusal: "errWalkBoundKind",
			build: walkProbe("ProbeWalkBoundKindInput", &linodev1.ObjectWalk{
				Field: []string{probeObjectArg},
				Members: &linodev1.ObjectMembers{
					Key: []string{probeWalkKey},
					Value: &linodev1.ObjectValue{
						Kind:     linodev1.ValueKind_VALUE_KIND_BOOL,
						Minimum:  1,
						Unusable: "{key} must be a boolean",
					},
				},
			}),
		},
		{
			name:    "members on a value that is not an object",
			refusal: "errWalkMembersKind",
			build: walkProbe("ProbeWalkMembersKindInput", &linodev1.ObjectWalk{
				Field: []string{probeObjectArg},
				Members: &linodev1.ObjectMembers{
					Key: []string{probeWalkKey},
					Value: &linodev1.ObjectValue{
						Kind:     linodev1.ValueKind_VALUE_KIND_TEXT,
						Unusable: probeWalkSentence,
						Members:  &linodev1.ObjectMembers{Key: []string{"disk_id"}},
					},
				},
			}),
		},
		{
			name:    "vocabulary naming an enum the contract does not declare",
			refusal: "errWalkUnknownEnum",
			build: walkProbe("ProbeWalkUnknownEnumInput", &linodev1.ObjectWalk{
				Field: []string{probeObjectArg},
				Members: &linodev1.ObjectMembers{
					Key: []string{probeWalkKey},
					Value: &linodev1.ObjectValue{
						Kind:     linodev1.ValueKind_VALUE_KIND_TEXT,
						Unusable: probeWalkSentence,
						Values:   "linode.mcp.v1.NoSuchEnum",
					},
				},
			}),
		},
		{
			name:    "sentence naming a placeholder the walk cannot fill",
			refusal: "errWalkPlaceholder",
			build: walkProbe("ProbeWalkPlaceholderInput", &linodev1.ObjectWalk{
				Field: []string{probeObjectArg}, Unusable: "devices must be {nobody}",
			}),
		},
	})
}

// walkProbe is a mutation carrying an open object and a list beside it, which
// is the shape every walk case above is declared over.
func walkProbe(message string, walks ...*linodev1.ObjectWalk) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		return goProbe(probeMessage(t, message, writeOptions(withWalk(walks...)),
			bodyString(probeDomainArg, 1), objectField(probeLocalNumber),
			listField(probeListArg, 3)))
	}
}

// objectField is one open object the request body carries, which is the
// map<string, Value> shape a walk reads and a preview choice selects on.
func objectField(number int32) *descriptorpb.FieldDescriptorProto {
	entry := probeField(probeObjectArg, number, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE,
		fieldOptions(withLocation(bodyLocation)))
	entry.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	entry.TypeName = new(probeMapMarker)

	return entry
}

// listField is one array of free-form objects the request body carries, which
// is the shape a walk's element sentence answers for.
func listField(name string, number int32) *descriptorpb.FieldDescriptorProto {
	entry := probeField(name, number, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE,
		fieldOptions(withLocation(bodyLocation)))
	entry.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	entry.TypeName = new(probeStructType)

	return entry
}

func withWalk(walks ...*linodev1.ObjectWalk) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_ObjectWalk, walks)
	}
}
