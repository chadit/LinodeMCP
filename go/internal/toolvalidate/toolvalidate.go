// Package toolvalidate evaluates the buf.validate rules a tool's input message
// declares against the arguments of one call, so a rule is written once in the
// proto contract instead of once per language. Before this package an argument
// check lived in a hand-written hook per language, and nothing held the two
// copies to the same answer.
//
// Only message-level rules are read. A field-level buf.validate rule is
// deliberately not used anywhere in the contract: protoschema-jsonschema turns
// one into a JSON Schema keyword, which changes the schema every client already
// reads, and schema stability is a contract of its own.
package toolvalidate

import (
	"encoding/json"
	"errors"
	"regexp"

	"buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// Check answers the first rule the arguments break, in the words the rule
// declares, or "" when they break none. The answer is the tool's own sentence:
// nothing here wraps or prefixes it, because a caller reads it as the reason
// their call was refused rather than as a validation library's report.
//
// inputMessage is the full name of the tool's input message, such as
// "linode.mcp.v1.DomainCreateInput".
func Check(inputMessage string, arguments map[string]any) string {
	descriptor, found := constrained(inputMessage)
	if !found {
		return ""
	}

	message, usable := build(descriptor, arguments)
	if !usable {
		return ""
	}

	err := protovalidate.Validate(message, protovalidate.WithFailFast())

	return firstViolation(err)
}

// constrained resolves a message that declares rules, and reports whether it
// declares any. A message with none is the common case and costs one registry
// lookup, which is what keeps the seam on every generated handler affordable.
func constrained(inputMessage string) (protoreflect.MessageDescriptor, bool) {
	found, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(inputMessage))
	if err != nil {
		return nil, false
	}

	descriptor, isMessage := found.(protoreflect.MessageDescriptor)
	if !isMessage {
		return nil, false
	}

	if !proto.HasExtension(descriptor.Options(), validate.E_Message) {
		return nil, false
	}

	return descriptor, true
}

// firstViolation reads the sentence out of a validation failure. Fail-fast
// leaves at most one violation, so the first is the whole answer.
//
// The sentence is the rule's own. Every rule the contract declares carries one
// (TestEveryDeclaredRuleNamesItsSentence), so this never reports the library's
// default wording, which would put words a tool never chose in front of a
// caller.
func firstViolation(err error) string {
	failure, isValidation := errors.AsType[*protovalidate.ValidationError](err)
	if !isValidation || len(failure.Violations) == 0 {
		return ""
	}

	return failure.Violations[0].Proto.GetMessage()
}

// build assembles the input message from one call's arguments, and reports
// whether the result is worth checking rules against.
//
// What an argument the field cannot hold means depends on where the field
// travels, because that is what decides who reports it today:
//
//   - A body or query field is read by the builder for that part of the
//     request, which reports the mismatch in its own words. Those words are
//     what clients read, so this answers "not usable" and leaves the whole
//     check to it rather than pre-empting it with a rule.
//   - A path or local field is read through an accessor that answers the zero
//     value rather than failing, so the field is left absent here for the same
//     reason: a caller who sent "abc" for an id gets the tool's own "is
//     required" sentence.
//   - A string naming no member of an enum field is set to the enum's zero,
//     wherever it travels, because a string IS the shape an enum argument
//     arrives in. That is what lets a rule tell an argument naming nothing from
//     an absent one, the difference between "status must be one of ..." and no
//     complaint at all.
//   - A string offered for an integer field is read only when it spells one
//     whole number, because a reader that takes "123/456" as 123 lets a rule
//     pass on an id the path read then refuses in different words.
func build(descriptor protoreflect.MessageDescriptor, arguments map[string]any) (proto.Message, bool) {
	fields := descriptor.Fields()
	message := dynamicpb.NewMessage(descriptor)

	for name, value := range arguments {
		field := fields.ByName(protoreflect.Name(name))
		if field == nil {
			continue
		}

		var read proto.Message
		if !misspelledNumber(field, value) {
			read = readable(descriptor, name, value)
		}

		switch {
		case read != nil:
			proto.Merge(message, read)
		case unknownEnumName(field, value):
			message.Set(field, protoreflect.ValueOfEnum(0))
		case reportedElsewhere(field):
			return nil, false
		}
	}

	return message, true
}

// reportedElsewhere reports whether another builder answers a type mismatch on
// this field, which is the case for everything that reaches the wire as body or
// query.
func reportedElsewhere(field protoreflect.FieldDescriptor) bool {
	location, _ := proto.GetExtension(
		field.Options(), linodev1.E_FieldLocation,
	).(linodev1.FieldLocation)

	return location == linodev1.FieldLocation_FIELD_LOCATION_BODY ||
		location == linodev1.FieldLocation_FIELD_LOCATION_QUERY
}

// readable decodes one argument into a message of its own, or answers nil when
// the value is not the type its field declares. Decoding the argument alone is
// what lets a field be judged on its own value rather than on whether the rest
// of the call happens to decode, and going through protojson is what makes it
// read exactly as the generated schema advertises rather than through a second
// hand-written reading.
func readable(descriptor protoreflect.MessageDescriptor, name string, value any) proto.Message {
	encoded, err := json.Marshal(map[string]any{name: value})
	if err != nil {
		return nil
	}

	single := dynamicpb.NewMessage(descriptor)
	if err := protojson.Unmarshal(encoded, single); err != nil {
		return nil
	}

	return single
}

// wholeNumber is the JSON number grammar a string offered for an integer field
// has to spell end to end, because each language's reader is lenient in its own
// direction otherwise: Go's takes the 123 out of "123/456" and Python's accepts
// spellings of its own such as "1_000". Python compiles the same pattern under
// re.ASCII, without which its \d would also take Arabic-Indic digits.
var wholeNumber = regexp.MustCompile(`^-?(0|[1-9]\d*)(\.\d+)?([eE][-+]?\d+)?$`)

// misspelledNumber reports whether a value is a string offered for an integer
// field that spells anything but one whole number, which is the reading left to
// the field's own rule rather than to a JSON reader.
func misspelledNumber(field protoreflect.FieldDescriptor, value any) bool {
	text, isText := value.(string)
	if !isText {
		return false
	}

	return integerField(field) && !wholeNumber.MatchString(text)
}

// integerField reports whether a field reads a JSON number as an integer, which
// is the only place a misspelled number can be offered.
func integerField(field protoreflect.FieldDescriptor) bool {
	var integer bool

	switch field.Kind() {
	case protoreflect.Int32Kind, protoreflect.Int64Kind, protoreflect.Uint32Kind,
		protoreflect.Uint64Kind, protoreflect.Sint32Kind, protoreflect.Sint64Kind,
		protoreflect.Fixed32Kind, protoreflect.Fixed64Kind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
		integer = true
	case protoreflect.BoolKind, protoreflect.EnumKind, protoreflect.FloatKind,
		protoreflect.DoubleKind, protoreflect.StringKind, protoreflect.BytesKind,
		protoreflect.MessageKind, protoreflect.GroupKind:
		integer = false
	}

	return integer
}

// unknownEnumName reports whether a value is a string offered for an enum field
// that names none of its members, which is the one refusal above that a rule
// still gets to answer.
func unknownEnumName(field protoreflect.FieldDescriptor, value any) bool {
	if field.Kind() != protoreflect.EnumKind || field.IsList() || field.IsMap() {
		return false
	}

	name, isText := value.(string)
	if !isText {
		return false
	}

	return field.Enum().Values().ByName(protoreflect.Name(name)) == nil
}
