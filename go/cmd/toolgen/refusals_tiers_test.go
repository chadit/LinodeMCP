package main_test

import (
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The refusals about the tier a declaration lands on: the arguments a tier can
// read, the gates it emits, the plan it runs, and the answer it builds.

// TestRefusesArgumentsTheTierCannotRead covers the argument locations and kinds
// each tier serves.
func TestRefusesArgumentsTheTierCannotRead(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "half of the page controls, which are read as a pair",
			refusal: "errUnsupportedQuery",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeUnsupportedQueryInput", writeOptions(),
					bodyString(probeDomainArg, 1),
					probeField("page", 2, descriptorpb.FieldDescriptorProto_TYPE_INT32,
						fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_QUERY)))))
			},
		},
		{
			name:    "body member with no request representation",
			refusal: "errUnsupportedBodyKind",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeUnsupportedBodyKindInput", writeOptions(),
					bodyString(probeDomainArg, 1),
					probeField("blob", 2, descriptorpb.FieldDescriptorProto_TYPE_BYTES,
						fieldOptions(withLocation(bodyLocation)))))
			},
		},
		{
			name:    "domain argument on a routed tool with no hook to read it",
			refusal: "errRoutedToolArgument",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeRoutedToolArgumentInput", writeOptions(),
					bodyString(probeDomainArg, 1),
					probeField("mode_hint", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING,
						fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_TOOL)))))
			},
		},
		{
			name:    "hoisted body beside another body member",
			refusal: "errBodyRootNotAlone",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				root := objectField(probeLocalNumber)
				root.Options = fieldOptions(withLocation(bodyLocation), withBodyRoot)

				return goProbe(probeMessage(t, "ProbeBodyRootNotAloneInput", writeOptions(),
					bodyString(probeDomainArg, 1), root))
			},
		},
	})
}

// TestRefusesAGateTheTierDoesNotEmit covers the two arguments that gate a call.
func TestRefusesAGateTheTierDoesNotEmit(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "a read that sends a body and advertises a gate",
			refusal: "errGatedBodyRead",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeGatedBodyReadInput",
					messageOptions(withRoute(probeWriteMethod, "/domains/{domain_id}/clone"),
						withCapability(readCapability), withResponse(probeResource),
						withDescription("Reads a probe through a body."),
						withErrorMessage("Failed to read probe: {error}")),
					pathInt("domain_id"), bodyString(probeDomainArg, 2),
					probeField("confirm", 3, descriptorpb.FieldDescriptorProto_TYPE_BOOL,
						fieldOptions(withLocation(localLocation)))))
			},
		},
		{
			name:    "a read advertising a preview with no gate to hold the fetch",
			refusal: "errNoReadPreview",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeNoReadPreviewInput", getOptions(),
					pathInt(probeIDArg),
					probeField("dry_run", 2, descriptorpb.FieldDescriptorProto_TYPE_BOOL,
						fieldOptions(withLocation(localLocation)))))
			},
		},
		{
			name:    "a plan argument without the other half of the flow",
			refusal: "errPartialTwoStage",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbePartialTwoStageInput", writeOptions(),
					bodyString(probeDomainArg, 1),
					probeField("mode", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING,
						fieldOptions(withLocation(localLocation)))))
			},
		},
	})
}

// TestRefusesProseThePythonArmCannotRender covers the second arm's own limits,
// which the Go arm has no equivalent of: ruff decides where emitted code
// breaks, and prose is what it cannot rewrap.
func TestRefusesProseThePythonArmCannotRender(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "a sentence too long to fit the emitted line",
			refusal: "errPyLineBudget",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return pythonProbe(t, probeRegistered(t, "ProbePyLineBudgetInput",
					getOptions(withDescription("Gets "+unbreakableWord+".")),
					pathInt(probeIDArg)))
			},
		},
		{
			name:    "a formatter the run cannot reach",
			refusal: "errPyFormat",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				run := pythonProbe(t, probeRegistered(t, "ProbePyFormatInput", getOptions(),
					pathInt(probeIDArg)))
				run.Ruff = filepath.Join(t.TempDir(), "no-such-ruff")

				return run
			},
		},
	})
}

// unbreakableWord is one token past the emitted line budget, which no wrap can
// split and no formatter can rewrap.
const unbreakableWord = "aprobenamewithnospacesatallthatrunswellpastthecolumnbudget" +
	"andkeepsgoingpastitagainsoeventhewrapcannothelpit"

// pythonProbe turns the Python arm on for one run, pointing the formatter at
// the repo's own ruff the way `make proto` does.
func pythonProbe(t *testing.T, message protoreflect.MessageDescriptor) *toolgen.ProbeRun {
	t.Helper()

	return &toolgen.ProbeRun{
		Name:    probeToolName,
		Message: message,
		PyOut:   t.TempDir(),
		Ruff:    pythonRuff,
	}
}
