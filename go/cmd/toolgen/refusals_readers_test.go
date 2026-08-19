package main_test

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The refusals about one argument's own declaration: where a reader can be
// read, what a body member may be renamed or composed into, and the wording an
// argument gives its refusals.

// The argument names a probe declares, spelled once because a case names the
// same field in the declaration and in the refusal it expects.
const (
	probeBodyArg  = "label"
	probeLocalArg = "environment"
)

// The field numbers those two take, which sit after the base mutation's own
// body member.
const (
	probeLocalNumber     = 2
	probeExtraBodyNumber = 3
)

// TestRefusesAReaderTheArgumentCannotAnswer covers argument_reader placement.
func TestRefusesAReaderTheArgumentCannotAnswer(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "presence reader off the body",
			refusal: "errPresentNotBody",
			build:   readerProbe("ProbePresentPlacementInput", localString(withReader(linodev1.ArgumentReader_ARGUMENT_READER_PRESENT))),
		},
		{
			name:    "required-text reader off a body string",
			refusal: "errPresentTextShape",
			build:   readerProbe("ProbePresentTextShapeInput", localString(withReader(linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_TEXT))),
		},
		{
			name:    "required-string reader off a body string",
			refusal: "errPresentStringShape",
			build:   readerProbe("ProbePresentStringShapeInput", localString(withReader(linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_STRING))),
		},
		{
			name:    "boolean-presence reader on a string",
			refusal: "errPresentBoolShape",
			build: readerProbe("ProbePresentBoolShapeInput", bodyString(probeBodyArg, 3,
				withReader(linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_BOOL))),
		},
		{
			name:    "id-list reader on one value",
			refusal: "errIDListShape",
			build: readerProbe("ProbeIDListShapeInput", bodyString(probeBodyArg, 3,
				withReader(linodev1.ArgumentReader_ARGUMENT_READER_ID_LIST))),
		},
		{
			name:    "membership reader where no raw argument is read",
			refusal: "errEnumMemberPlacement",
			build: readerProbe("ProbeMemberPlacementInput", localString(withReader(linodev1.ArgumentReader_ARGUMENT_READER_ENUM_MEMBER),
				withReaderValues("first", "second"))),
		},
		{
			name:    "membership reader on a list",
			refusal: "errEnumMemberRepeated",
			build: readerProbe("ProbeMemberRepeatedInput", repeatedField(probeBodyArg, 3,
				descriptorpb.FieldDescriptorProto_TYPE_STRING,
				fieldOptions(withLocation(bodyLocation),
					withReader(linodev1.ArgumentReader_ARGUMENT_READER_ENUM_MEMBER),
					withReaderValues("first", "second")))),
		},
		{
			name:    "membership reader on a field with no vocabulary",
			refusal: "errEnumMemberKind",
			build:   readerProbe("ProbeMemberKindInput", bodyInt(withReader(linodev1.ArgumentReader_ARGUMENT_READER_ENUM_MEMBER))),
		},
		{
			name:    "membership reader on a string that lists nothing",
			refusal: "errEnumMemberNoValues",
			build: readerProbe("ProbeMemberNoValuesInput", bodyString(probeBodyArg, 3,
				withReader(linodev1.ArgumentReader_ARGUMENT_READER_ENUM_MEMBER))),
		},
		{
			name:    "id reader on an argument that addresses nothing",
			refusal: "errReaderNotPath",
			build:   readerProbe("ProbeReaderNotPathInput", bodyInt(withReader(linodev1.ArgumentReader_ARGUMENT_READER_POSITIVE_ID))),
		},
	})
}

// TestRefusesAReaderTheSlotCannotAnswer covers the readers a path slot takes.
func TestRefusesAReaderTheSlotCannotAnswer(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "positive-id reader on a text slot",
			refusal: "errReaderNotPathInteger",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeReaderPathIntegerInput", getOptions(),
					probeField(probeIDArg, 1, descriptorpb.FieldDescriptorProto_TYPE_STRING,
						fieldOptions(withLocation(pathLocation),
							withReader(linodev1.ArgumentReader_ARGUMENT_READER_POSITIVE_ID)))))
			},
		},
		{
			name:    "text reader on a numeric slot",
			refusal: "errReaderNotPathText",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeReaderPathTextInput", getOptions(),
					probeField(probeIDArg, 1, descriptorpb.FieldDescriptorProto_TYPE_INT32,
						fieldOptions(withLocation(pathLocation),
							withReader(linodev1.ArgumentReader_ARGUMENT_READER_PATH_SAFE_TEXT)))))
			},
		},
	})
}

// TestRefusesADeclarationTheReaderLeavesUnread covers the wording and the
// vocabulary a reader is declared beside.
func TestRefusesADeclarationTheReaderLeavesUnread(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "vocabulary with no reader to hold it",
			refusal: "errReaderValuesAlone",
			build: readerProbe("ProbeValuesAloneInput", bodyString(probeBodyArg, 3,
				withReaderValues("master", "slave"))),
		},
		{
			name:    "wording with no reader to word",
			refusal: "errReaderMessageAlone",
			build: readerProbe("ProbeMessageAloneInput", bodyString(probeBodyArg, 3,
				withReaderMessage(&linodev1.ReaderMessage{Absent: "label is required"}))),
		},
		{
			name:    "wording for an arm the reader never answers",
			refusal: "errReaderMessageArm",
			build: readerProbe("ProbeMessageArmInput", bodyString(probeBodyArg, 3,
				withReader(linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_TEXT),
				withReaderMessage(&linodev1.ReaderMessage{Refused: "label was refused"}))),
		},
		{
			name:    "reader beside the hook that owns the whole check",
			refusal: "errReaderWithValidate",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeReaderWithValidateInput",
					writeOptions(withHooks("validate")),
					bodyString(probeBodyArg, 3,
						withReader(linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_TEXT))))
			},
		},
	})
}

// TestRefusesABodyDeclarationOffTheBody covers the field options that only mean
// something on a member the request body carries.
func TestRefusesABodyDeclarationOffTheBody(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "redaction on an argument no preview reports",
			refusal: "errRedactNotBody",
			build:   readerProbe("ProbeRedactPlacementInput", localString(withRedact)),
		},
		{
			name:    "wire name on an argument that reaches no body",
			refusal: "errBodyNameNotBody",
			build: readerProbe("ProbeBodyNamePlacementInput",
				localString(withBodyName("env"))),
		},
		{
			name:    "wire name repeating the argument's own name",
			refusal: "errBodyNameNoop",
			build: readerProbe("ProbeBodyNameNoopInput",
				bodyString(probeBodyArg, 3, withBodyName(probeBodyArg))),
		},
		{
			name:    "declared null on an argument that reaches no body",
			refusal: "errNullableNotBody",
			build:   readerProbe("ProbeNullablePlacementInput", localString(withNullable)),
		},
		{
			name:    "declared null on a member with no null to send",
			refusal: "errNullableNotObject",
			build:   readerProbe("ProbeNullableShapeInput", bodyInt(withNullable)),
		},
		{
			name:    "comma composition on an argument that reaches no body",
			refusal: "errCommaListNotBody",
			build:   readerProbe("ProbeCommaPlacementInput", localString(withCommaList)),
		},
		{
			name:    "comma composition on a member with no text to split",
			refusal: "errCommaListNotString",
			build:   readerProbe("ProbeCommaShapeInput", bodyInt(withCommaList)),
		},
		{
			name:    "hoisted body on a member with no keys to hoist",
			refusal: "errBodyRootNotObject",
			build:   readerProbe("ProbeBodyRootShapeInput", bodyString(probeBodyArg, 3, withBodyRoot)),
		},
	})
}

// readerProbe is a mutation carrying one extra argument, which is the shape
// every field-level refusal above is measured on.
func readerProbe(
	message string, entry *descriptorpb.FieldDescriptorProto,
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		return goProbe(probeMessage(t, message, writeOptions(),
			bodyString("domain", 1), entry))
	}
}

// The locations a probe's arguments declare.
const (
	bodyLocation  = linodev1.FieldLocation_FIELD_LOCATION_BODY
	localLocation = linodev1.FieldLocation_FIELD_LOCATION_LOCAL
	pathLocation  = linodev1.FieldLocation_FIELD_LOCATION_PATH
)

// bodyString is one text member of the request body.
func bodyString(
	name string, number int32, sets ...func(*descriptorpb.FieldOptions),
) *descriptorpb.FieldDescriptorProto {
	return probeField(name, number, descriptorpb.FieldDescriptorProto_TYPE_STRING,
		fieldOptions(append([]func(*descriptorpb.FieldOptions){withLocation(bodyLocation)}, sets...)...))
}

// bodyInt is one numeric member of the request body. It sits after the base
// mutation's own member, which is why the numbers below are fixed.
func bodyInt(sets ...func(*descriptorpb.FieldOptions)) *descriptorpb.FieldDescriptorProto {
	return probeField(probeBodyArg, probeExtraBodyNumber, descriptorpb.FieldDescriptorProto_TYPE_INT32,
		fieldOptions(append([]func(*descriptorpb.FieldOptions){withLocation(bodyLocation)}, sets...)...))
}

// localString is one argument the tool reads and the request never carries.
func localString(sets ...func(*descriptorpb.FieldOptions)) *descriptorpb.FieldDescriptorProto {
	return probeField(probeLocalArg, probeLocalNumber, descriptorpb.FieldDescriptorProto_TYPE_STRING,
		fieldOptions(append([]func(*descriptorpb.FieldOptions){withLocation(localLocation)}, sets...)...))
}

func withReader(reader linodev1.ArgumentReader) func(*descriptorpb.FieldOptions) {
	return func(options *descriptorpb.FieldOptions) {
		proto.SetExtension(options, linodev1.E_ArgumentReader, reader)
	}
}

func withReaderValues(values ...string) func(*descriptorpb.FieldOptions) {
	return func(options *descriptorpb.FieldOptions) {
		proto.SetExtension(options, linodev1.E_ReaderValues, values)
	}
}

func withReaderMessage(message *linodev1.ReaderMessage) func(*descriptorpb.FieldOptions) {
	return func(options *descriptorpb.FieldOptions) {
		proto.SetExtension(options, linodev1.E_ReaderMessage, message)
	}
}

func withRedact(options *descriptorpb.FieldOptions) {
	proto.SetExtension(options, linodev1.E_PreviewRedact, true)
}

func withNullable(options *descriptorpb.FieldOptions) {
	proto.SetExtension(options, linodev1.E_BodyNullable, true)
}

func withCommaList(options *descriptorpb.FieldOptions) {
	proto.SetExtension(options, linodev1.E_BodyCommaList, true)
}

func withBodyRoot(options *descriptorpb.FieldOptions) {
	proto.SetExtension(options, linodev1.E_BodyRoot, true)
}

func withBodyName(name string) func(*descriptorpb.FieldOptions) {
	return func(options *descriptorpb.FieldOptions) {
		proto.SetExtension(options, linodev1.E_BodyName, name)
	}
}
