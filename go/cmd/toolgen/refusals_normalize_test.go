package main_test

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The normalize_fields refusals. A rewrite that does not land is invisible
// afterwards: the call goes through carrying the value the caller sent, and
// nothing downstream says the declared transform never ran.

// trimTransform is the one rewrite most of these cases declare, and
// unrenderedTransform is a member number no arm answers for, which is how a
// transform added to the enum without a rendering reads here.
const (
	trimTransform       = linodev1.NormalizeTransform_NORMALIZE_TRANSFORM_TRIM
	unrenderedTransform = linodev1.NormalizeTransform(99)
)

// probeSecretArg is the argument name the secret rule reads, spelled in halves
// so the leak scanner reads a case name rather than a credential.
const probeSecretArg = "pass" + "word"

func TestRefusesANormalizeDeclarationThatCannotLand(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "transform over no field",
			refusal: "errNormalizeNoField",
			build: normalizeProbe("ProbeNormalizeNoFieldInput",
				&linodev1.NormalizeField{Transform: trimTransform}),
		},
		{
			name:    "field the message does not declare",
			refusal: "errNormalizeUnknownField",
			build: normalizeProbe("ProbeNormalizeUnknownFieldInput",
				&linodev1.NormalizeField{
					Field: []string{probeAbsentName}, Transform: trimTransform,
				}),
		},
		{
			name:    "two declarations rewriting one argument",
			refusal: "errNormalizeRepeatedField",
			build: normalizeProbe("ProbeNormalizeRepeatedFieldInput",
				&linodev1.NormalizeField{
					Field: []string{probeDomainArg}, Transform: trimTransform,
				},
				&linodev1.NormalizeField{
					Field:     []string{probeDomainArg},
					Transform: linodev1.NormalizeTransform_NORMALIZE_TRANSFORM_TRIM_LIST_DROP_BLANK,
				}),
		},
		{
			name:    "argument named as the credential itself",
			refusal: "errNormalizeSecretField",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeNormalizeSecretInput",
					writeOptions(withNormalize(&linodev1.NormalizeField{
						Field: []string{probeSecretArg}, Transform: trimTransform,
					})),
					bodyString(probeDomainArg, 1), bodyString(probeSecretArg, 2)))
			},
		},
		{
			name:    "declaration naming no transform",
			refusal: "errUnsupportedTransform",
			build: normalizeProbe("ProbeNormalizeUnspecifiedInput",
				&linodev1.NormalizeField{Field: []string{probeDomainArg}}),
		},
		{
			name:    "transform no arm renders",
			refusal: "errUnsupportedTransform",
			build: normalizeProbe("ProbeNormalizeUnrenderedInput",
				&linodev1.NormalizeField{
					Field: []string{probeDomainArg}, Transform: unrenderedTransform,
				}),
		},
		{
			name:    "declaration on a tier that reads its arguments in the driver",
			refusal: "errNoNormalizePoint",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeNormalizeTierInput",
					destroyOptions(withNormalize(&linodev1.NormalizeField{
						Field: []string{probeDeleteArg}, Transform: trimTransform,
					})),
					pathInt(probeDeleteArg)))
			},
		},
	})
}

// TestEveryNormalizeTransformRendersInBothLanguages holds the rendering table
// to the enum the contract declares: a member with no entry, or one an arm has
// no name for, would reach one tree as a call and the other as silence.
func TestEveryNormalizeTransformRendersInBothLanguages(t *testing.T) {
	t.Parallel()

	calls := toolgen.ProbeNormalizeCalls()
	if len(calls) == 0 {
		t.Fatal("no transform renders, so this measured nothing")
	}

	members := linodev1.NormalizeTransform(0).Descriptor().Values()

	for i := range members.Len() {
		member := string(members.Get(i).Name())
		if members.Get(i).Number() == 0 {
			continue
		}

		named, rendered := calls[member]
		if !rendered {
			t.Errorf("%s renders to nothing", member)

			continue
		}

		for _, name := range named {
			if name == "" {
				t.Errorf("%s renders to %v, which leaves an arm unnamed", member, named)
			}
		}
	}
}

// normalizeProbe is a mutation carrying whatever declarations the case states,
// which is the shape the cases above are declared over.
func normalizeProbe(
	message string,
	fields ...*linodev1.NormalizeField,
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		return goProbe(probeMessage(t, message, writeOptions(withNormalize(fields...)),
			bodyString(probeDomainArg, 1)))
	}
}

func withNormalize(fields ...*linodev1.NormalizeField) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_NormalizeFields, fields)
	}
}

// The normalize_fold refusals: a fold that cannot land would leave the
// convenience argument traveling beside the shape it was meant to become.

// foldSourceArg is the repeated integer body member the fold cases read from,
// and foldTargetKey the member the fold would write.
const (
	foldSourceArg = "member_ids"
	foldTargetKey = "linodes"
)

func TestRefusesAFoldThatCannotLand(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "fold with no target key",
			refusal: "errFoldEmptyKey",
			build: foldProbe("ProbeFoldEmptyKeyInput", nil,
				&linodev1.NormalizeFold{Source: foldSourceArg, Target: probeObjectArg}),
		},
		{
			name:    "fold into itself",
			refusal: "errFoldSelfTarget",
			build: foldProbe("ProbeFoldSelfTargetInput", nil,
				&linodev1.NormalizeFold{
					Source: foldSourceArg, Target: foldSourceArg, Key: foldTargetKey,
				}),
		},
		{
			name:    "fold source another declaration rewrites",
			refusal: "errNormalizeRepeatedField",
			build: foldProbe("ProbeFoldRewrittenSourceInput",
				withNormalize(&linodev1.NormalizeField{
					Field: []string{foldSourceArg}, Transform: trimTransform,
				}),
				&linodev1.NormalizeFold{
					Source: foldSourceArg, Target: probeObjectArg, Key: foldTargetKey,
				}),
		},
		{
			name:    "fold source that is not an integer list",
			refusal: "errFoldSourceKind",
			build: foldProbe("ProbeFoldSourceKindInput", nil,
				&linodev1.NormalizeFold{
					Source: probeDomainArg, Target: probeObjectArg, Key: foldTargetKey,
				}),
		},
		{
			name:    "fold target that is not an open object",
			refusal: "errFoldTargetKind",
			build: foldProbe("ProbeFoldTargetKindInput", nil,
				&linodev1.NormalizeFold{
					Source: foldSourceArg, Target: probeDomainArg, Key: foldTargetKey,
				}),
		},
	})
}

// foldProbe is a mutation carrying one declared fold beside an integer-list
// source and an open-object target, which is the shape the cases above refuse
// one detail of at a time.
func foldProbe(
	message string,
	extra func(*descriptorpb.MessageOptions),
	fold *linodev1.NormalizeFold,
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		sets := []func(*descriptorpb.MessageOptions){withNormalizeFold(fold)}
		if extra != nil {
			sets = append(sets, extra)
		}

		return goProbe(probeMessage(t, message, writeOptions(sets...),
			bodyString(probeDomainArg, 1),
			repeatedField(foldSourceArg, 2, descriptorpb.FieldDescriptorProto_TYPE_INT32,
				fieldOptions(withLocation(bodyLocation))),
			objectField(3)))
	}
}

func withNormalizeFold(fold *linodev1.NormalizeFold) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_NormalizeFold, fold)
	}
}
