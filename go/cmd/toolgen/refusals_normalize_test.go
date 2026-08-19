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
			name:    "declaration beside a normalize hook",
			refusal: "errNormalizeWithHook",
			build: normalizeProbe("ProbeNormalizeWithHookInput", withHooks("normalize"),
				&linodev1.NormalizeField{
					Field: []string{probeDomainArg}, Transform: trimTransform,
				}),
		},
		{
			name:    "transform over no field",
			refusal: "errNormalizeNoField",
			build: normalizeProbe("ProbeNormalizeNoFieldInput", nil,
				&linodev1.NormalizeField{Transform: trimTransform}),
		},
		{
			name:    "field the message does not declare",
			refusal: "errNormalizeUnknownField",
			build: normalizeProbe("ProbeNormalizeUnknownFieldInput", nil,
				&linodev1.NormalizeField{
					Field: []string{probeAbsentName}, Transform: trimTransform,
				}),
		},
		{
			name:    "two declarations rewriting one argument",
			refusal: "errNormalizeRepeatedField",
			build: normalizeProbe("ProbeNormalizeRepeatedFieldInput", nil,
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
			build: normalizeProbe("ProbeNormalizeUnspecifiedInput", nil,
				&linodev1.NormalizeField{Field: []string{probeDomainArg}}),
		},
		{
			name:    "transform no arm renders",
			refusal: "errUnsupportedTransform",
			build: normalizeProbe("ProbeNormalizeUnrenderedInput", nil,
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
					destroyOptions(withHooks("fetch_state"),
						withNormalize(&linodev1.NormalizeField{
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
	extra func(*descriptorpb.MessageOptions),
	fields ...*linodev1.NormalizeField,
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		sets := []func(*descriptorpb.MessageOptions){withNormalize(fields...)}
		if extra != nil {
			sets = append(sets, extra)
		}

		return goProbe(probeMessage(t, message, writeOptions(sets...),
			bodyString(probeDomainArg, 1)))
	}
}

func withNormalize(fields ...*linodev1.NormalizeField) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_NormalizeFields, fields)
	}
}
