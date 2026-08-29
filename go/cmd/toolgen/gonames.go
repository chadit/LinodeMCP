package main

import (
	"fmt"
	"go/token"
	"reflect"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/goname"
)

// goMessage is the Go type protoc-gen-go generated for one proto message, read
// off the compiled type rather than derived from the proto name.
//
// Deriving it would mean reimplementing protoc-gen-go's name mangling, which
// breaks silently on a field like id_2 or a message with an underscore, and
// only shows up when the emitted source stops compiling.
type goMessage struct {
	// Descriptor is the message's own descriptor, kept from the lookup that
	// resolved the type rather than resolved again. A second lookup would be
	// one more read of the process-global registry, which a parallel caller
	// cannot take while another goroutine holds it across a range.
	Descriptor protoreflect.MessageDescriptor
	// FullName is the proto full name, "linode.mcp.v1.Domain".
	FullName protoreflect.FullName
	// TypeName is the Go type name, "Domain".
	TypeName string
	// goNames maps a proto field name to its Go struct field name.
	goNames map[string]string
	// kinds maps a proto field name to its declared kind, which is what picks
	// the verb a message template renders the field under.
	kinds map[string]protoreflect.Kind
	// lists holds the repeated field names, which a message template can name
	// only for their count.
	lists map[string]bool
	// repeated lists the repeated message fields, in declaration order.
	repeated []repeatedField
	// singular lists the message fields that hold one message, in declaration
	// order. A mutation's answer carries exactly one, the resource it changed.
	singular []repeatedField
	// scalars lists the fields holding no message, in declaration order. A
	// delete's answer is made of them: the prose, and the ids it echoes back.
	scalars []scalarField
}

// scalarField is one non-message field of a message.
type scalarField struct {
	// protoName is the proto field name, "domain_id".
	protoName string
	// goName is the Go struct field name, "DomainId".
	goName string
	// argument is the tool argument an assembled answer fills this member
	// from, "" when the member's own name is that argument.
	argument string
	// repeated is true for a list field, which no id echo can be.
	repeated bool
	// local is true when the member declares FIELD_LOCATION_LOCAL, which on a
	// response says the MCP layer assembles it rather than the API sending it.
	local bool
	// kind is the member's declared kind, which is what says whether a declared
	// transport's answer can fill it.
	kind protoreflect.Kind
}

// argumentName is the input field an assembled answer resolves this member
// against, which is the member's own name unless the response aliases it.
func (s scalarField) argumentName() string {
	if s.argument != "" {
		return s.argument
	}

	return s.protoName
}

// repeatedField is one repeated message field of a response envelope.
type repeatedField struct {
	// goName is the Go struct field name, "Domains".
	goName string
	// protoName is the wire name the serialized answer carries it under.
	protoName string
	// message is the element's proto full name.
	message protoreflect.FullName
	// local is true when the member declares FIELD_LOCATION_LOCAL, which on a
	// response says the MCP layer assembles it rather than the API sending it.
	local bool
}

// lookupGoMessage resolves the Go type generated for a proto message.
func lookupGoMessage(name protoreflect.FullName) (goMessage, error) {
	messageType, err := protoregistry.GlobalTypes.FindMessageByName(name)
	if err != nil {
		return goMessage{}, fmt.Errorf("%w: %s: %w", errNoGoType, name, err)
	}

	pointer := reflect.TypeOf(messageType.New().Interface())
	if pointer == nil || pointer.Kind() != reflect.Pointer {
		return goMessage{}, fmt.Errorf("%w: %s is not a pointer type", errNoGoType, name)
	}

	structType := pointer.Elem()
	if structType.Kind() != reflect.Struct {
		return goMessage{}, fmt.Errorf("%w: %s is not a struct type", errNoGoType, name)
	}

	structFields := reflect.VisibleFields(structType)
	descriptorFields := messageType.Descriptor().Fields()

	found := goMessage{
		Descriptor: messageType.Descriptor(),
		FullName:   name,
		TypeName:   structType.Name(),
		goNames:    make(map[string]string, len(structFields)),
		kinds:      make(map[string]protoreflect.Kind, len(structFields)),
		lists:      make(map[string]bool, len(structFields)),
	}

	for _, structField := range structFields {
		protoName := protoTagName(structField.Tag.Get("protobuf"))
		if protoName == "" {
			continue
		}

		found.goNames[protoName] = structField.Name

		if err := found.record(descriptorFields, protoName, structField.Name); err != nil {
			return goMessage{}, err
		}
	}

	return found, nil
}

// record files one field under the parts of the message this emitter reads: its
// kind, and whether it holds many messages or one.
func (m *goMessage) record(fields protoreflect.FieldDescriptors, protoName, goName string) error {
	descriptor := fields.ByName(protoreflect.Name(protoName))
	if descriptor == nil {
		return nil
	}

	m.kinds[protoName] = descriptor.Kind()
	m.lists[protoName] = descriptor.IsList()

	argument, err := echoArgument(m.FullName, descriptor)
	if err != nil {
		return err
	}

	if descriptor.Message() == nil {
		m.scalars = append(m.scalars, scalarField{
			protoName: protoName,
			goName:    goName,
			argument:  argument,
			repeated:  descriptor.IsList(),
			local:     fieldLocation(descriptor) == linodev1.FieldLocation_FIELD_LOCATION_LOCAL,
			kind:      descriptor.Kind(),
		})

		return nil
	}

	entry := repeatedField{
		goName:    goName,
		protoName: protoName,
		message:   descriptor.Message().FullName(),
		local:     fieldLocation(descriptor) == linodev1.FieldLocation_FIELD_LOCATION_LOCAL,
	}

	if descriptor.IsList() {
		m.repeated = append(m.repeated, entry)

		return nil
	}

	m.singular = append(m.singular, entry)

	return nil
}

// echoArgument reads the tool argument one response member is filled from, ""
// when the member's own name is that argument.
//
// The two refusals are what keep the declaration from being read and ignored: a
// member that repeats or carries a message is resolved by a different reader
// entirely, and an alias spelling the member's own name resolves exactly where
// the absence of one does.
func echoArgument(
	message protoreflect.FullName, descriptor protoreflect.FieldDescriptor,
) (string, error) {
	argument, ok := proto.GetExtension(
		descriptor.Options(), linodev1.E_EchoArgument,
	).(string)
	if !ok || argument == "" {
		return "", nil
	}

	if descriptor.IsList() || descriptor.Message() != nil {
		return "", fmt.Errorf("%w: %s.%s", errEchoArgumentShape, message, descriptor.Name())
	}

	if argument == string(descriptor.Name()) {
		return "", fmt.Errorf("%w: %s.%s", errEchoArgumentNoop, message, descriptor.Name())
	}

	return argument, nil
}

// goNameOf returns the Go struct field name for a proto field, or "" when the
// message has no such field.
func (m *goMessage) goNameOf(protoName string) string {
	return m.goNames[protoName]
}

// kindOf returns a proto field's declared kind, and whether the message has one.
func (m *goMessage) kindOf(protoName string) (protoreflect.Kind, bool) {
	kind, found := m.kinds[protoName]

	return kind, found
}

// isList reports whether a proto field repeats.
func (m *goMessage) isList(protoName string) bool {
	return m.lists[protoName]
}

// repeatedFields lists the message's repeated message fields.
func (m *goMessage) repeatedFields() []repeatedField {
	return m.repeated
}

// messageFields lists the fields holding one message each.
func (m *goMessage) messageFields() []repeatedField {
	return m.singular
}

// scalarFields lists the message's non-message fields in declaration order.
func (m *goMessage) scalarFields() []scalarField {
	return m.scalars
}

// protoTagName reads the proto field name out of a generated struct tag, which
// reads "bytes,2,opt,name=soa_email,json=soaEmail,proto3".
func protoTagName(tag string) string {
	for part := range strings.SplitSeq(tag, ",") {
		if name, found := strings.CutPrefix(part, "name="); found {
			return name
		}
	}

	return ""
}

// goLocalName turns a proto field name into the local variable a handler reads
// it into: domain_id becomes domainID.
//
// Initialisms apply only where a whole underscore-separated word matches. These
// names are never exported, so they need not match protoc-gen-go.
func goLocalName(protoName string) string {
	words := strings.Split(protoName, "_")

	var built strings.Builder

	built.Grow(len(protoName))

	for i, word := range words {
		if word == "" {
			continue
		}

		if i == 0 {
			built.WriteString(word)

			continue
		}

		if initialism, known := goname.Initialism(word); known {
			built.WriteString(initialism)

			continue
		}

		built.WriteString(strings.ToUpper(word[:1]))
		built.WriteString(word[1:])
	}

	return avoidKeyword(built.String())
}

// avoidKeyword renames a local that would shadow a reserved word. A single-word
// field such as the `range` in /networking/ipv6/ranges/{range} produces a local
// spelled exactly like the keyword, which is a parse error rather than a lint
// finding, so the suffix is not optional.
func avoidKeyword(local string) string {
	if token.IsKeyword(local) {
		return local + "Value"
	}

	return local
}

// goStringLiteral renders text as a Go double-quoted string literal.
func goStringLiteral(text string) string {
	return fmt.Sprintf("%q", text)
}
