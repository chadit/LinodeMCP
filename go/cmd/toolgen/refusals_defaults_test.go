package main_test

import (
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The refusals about a placeholder's default form. A default stands in for an
// argument the caller left out, which only a tool that addresses no request has
// to fill, so these are declared on the tier that answers from local state.

// The arguments a sentence interpolates, spelled once because a case names the
// same one in the field and in the template.
const (
	probeToolArg  = "report"
	probeCountArg = "depth"
)

// TestRefusesADefaultTheArgumentCannotHold covers the {name|default} form.
func TestRefusesADefaultTheArgumentCannotHold(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "a default that declares the state it replaces",
			refusal: "errEmptyDefault",
			build:   defaultProbe("ProbeEmptyDefaultInput", "Reported {report|}"),
		},
		{
			name:    "a default beside a count",
			refusal: "errModifiedDefault",
			build:   defaultProbe("ProbeModifiedDefaultInput", "Reported {report:len|all}"),
		},
		{
			name:    "a default the argument's kind cannot hold",
			refusal: "errUnreadableDefault",
			build:   defaultProbe("ProbeUnreadableDefaultInput", "Reported {depth|deep}"),
		},
		{
			name:    "a default on an argument that addresses the request",
			refusal: "errDefaultNotToolArg",
			build:   writeProbe("ProbeDefaultNotToolArgInput", withSuccessMessage("Created {note|a probe}")),
		},
	})
}

// defaultProbe is a tool answering from local state, whose sentence the case
// writes and whose two arguments it interpolates.
func defaultProbe(message, sentence string) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		return goProbe(probeMessage(t, message,
			metaOptions(withResponse(probeWriteBody), withSuccessMessage(sentence)),
			toolString(probeToolArg, 1), toolInt(probeCountArg, 2)))
	}
}

// toolString is one domain argument a tool reads and no request carries.
func toolString(name string, number int32) *descriptorpb.FieldDescriptorProto {
	return probeField(name, number, descriptorpb.FieldDescriptorProto_TYPE_STRING,
		fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_TOOL)))
}

// toolInt is the same, carrying a number.
func toolInt(name string, number int32) *descriptorpb.FieldDescriptorProto {
	return probeField(name, number, descriptorpb.FieldDescriptorProto_TYPE_INT32,
		fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_TOOL)))
}
