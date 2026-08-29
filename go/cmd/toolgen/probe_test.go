package main_test

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The synthesized-descriptor probes.
//
// Every refusal in errors.go answers a declaration this repo's contract does
// not carry, since the contract is the surface that ships. The messages below
// are built here instead: one FileDescriptorProto per case, compiled against
// the registered contract package so its options resolve exactly as a real
// tool's do, then run through the emitter by name.
//
// The file is never registered, so the emitter's own walk over the registry
// cannot see a probe, and one run's probes cannot reach another's.

// probePackage is the package a synthesized message is declared in, which is
// the contract's own: a probe reads its options through the same extension
// types a real tool does.
const probePackage = "linode.mcp.v1"

// probeOptionsProto is the file the contract's options are declared in, taken
// as a dependency so a synthesized message can carry them.
const probeOptionsProto = "linode/mcp/v1/options.proto"

// proto3Syntax is the syntax every synthesized file declares, named because a
// descriptor field takes a pointer to it.
const proto3Syntax = "proto3"

// probeValueType is the free-form value an open object carries, and
// probeStructProto the file it is declared in.
const (
	probeValueType   = ".google.protobuf.Value"
	probeStructType  = ".google.protobuf.Struct"
	probeStructProto = "google/protobuf/struct.proto"
)

// mapEntryFlag marks a nested message as a map's entry, named because the
// descriptor field takes a pointer to it.
const mapEntryFlag = true

// probeMapMarker stands in for a map field's entry type until the message it
// belongs to is known: the entry is nested inside that message, so its name
// cannot be spelled where the field is built.
const probeMapMarker = "map<string, Value>"

// probeItemMarker stands in for a typed list's item message for the same reason
// probeMapMarker stands in for a map entry, and probeItemMember is the one
// member those entries carry.
const (
	probeItemMarker = "message<Item>"
	probeItemMember = "note"
)

// probeMessage compiles one synthesized input message and answers its
// descriptor.
func probeMessage(
	t *testing.T,
	name string,
	options *descriptorpb.MessageOptions,
	fields ...*descriptorpb.FieldDescriptorProto,
) protoreflect.MessageDescriptor {
	t.Helper()

	// Unique per case: two files claiming one path cannot both compile, and
	// the case that built a message is the one that has to be named when it
	// does not.
	path := "linode/mcp/v1/probe_" + strings.ReplaceAll(t.Name(), "/", "_") + "_" + name + ".proto"

	file := &descriptorpb.FileDescriptorProto{
		Name:       new(path),
		Package:    new(probePackage),
		Syntax:     new(proto3Syntax),
		Dependency: []string{probeOptionsProto, probeStructProto},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name:       new(name),
			Field:      fields,
			NestedType: append(mapEntries(name, fields), itemMessages(name, fields)...),
			Options:    options,
		}},
	}

	compiled, err := protodesc.NewFile(file, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatalf("compile %s: %v", path, err)
	}

	return compiled.Messages().Get(0)
}

// mapEntries is the nested entry message each marked map field needs, and
// rewrites those fields to name it. proto spells a map as a repeated message of
// key and value pairs, so a synthesized open object has to carry that shape.
func mapEntries(
	message string, fields []*descriptorpb.FieldDescriptorProto,
) []*descriptorpb.DescriptorProto {
	nested := make([]*descriptorpb.DescriptorProto, 0, len(fields))

	for _, entry := range fields {
		if entry.GetTypeName() != probeMapMarker {
			continue
		}

		name := mapEntryName(entry.GetName())
		nested = append(nested, &descriptorpb.DescriptorProto{
			Name:    new(name),
			Options: &descriptorpb.MessageOptions{MapEntry: new(mapEntryFlag)},
			Field: []*descriptorpb.FieldDescriptorProto{
				probeField("key", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, nil),
				mapValueField(),
			},
		})

		entry.TypeName = new("." + probePackage + "." + message + "." + name)
	}

	return nested
}

// itemMessages is the nested item message each marked typed-list field needs,
// and rewrites those fields to name it. A stand-in is declared over a member of
// that message, which is the shape a free-form list cannot carry.
func itemMessages(
	message string, fields []*descriptorpb.FieldDescriptorProto,
) []*descriptorpb.DescriptorProto {
	nested := make([]*descriptorpb.DescriptorProto, 0, len(fields))

	for _, entry := range fields {
		if entry.GetTypeName() != probeItemMarker {
			continue
		}

		name := itemMessageName(entry.GetName())
		nested = append(nested, &descriptorpb.DescriptorProto{
			Name: new(name),
			Field: []*descriptorpb.FieldDescriptorProto{
				probeField(probeItemMember, 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, nil),
			},
		})

		entry.TypeName = new("." + probePackage + "." + message + "." + name)
	}

	return nested
}

// itemMessageName is the message one typed list's entries are declared as,
// derived from the field so two lists in one probe cannot collide.
func itemMessageName(field string) string {
	return camelName(field) + "Item"
}

// mapValueField is the free-form value half of a map entry.
func mapValueField() *descriptorpb.FieldDescriptorProto {
	value := probeField("value", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, nil)
	value.TypeName = new(probeValueType)

	return value
}

// mapEntryName is the message proto nests a map field's entries in, which is
// the field name in camel case with Entry after it.
func mapEntryName(field string) string {
	return camelName(field) + "Entry"
}

// camelName is a field name as a nested message spells it.
func camelName(field string) string {
	parts := strings.Split(field, "_")
	for i, part := range parts {
		if part != "" {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}

	return strings.Join(parts, "")
}

// probeRegistered is probeMessage plus a message type in the global registry,
// which is how the Python arm resolves an input message by name. The type is
// registered for the whole binary, so each probe carries its own message name.
func probeRegistered(
	t *testing.T,
	name string,
	options *descriptorpb.MessageOptions,
	fields ...*descriptorpb.FieldDescriptorProto,
) protoreflect.MessageDescriptor {
	t.Helper()

	message := probeMessage(t, name, options, fields...)

	if err := protoregistry.GlobalTypes.RegisterMessage(dynamicpb.NewMessageType(message)); err != nil {
		t.Fatalf("register %s: %v", message.FullName(), err)
	}

	return message
}

// messageOptions is a MessageOptions carrying the declarations each set applies.
func messageOptions(sets ...func(*descriptorpb.MessageOptions)) *descriptorpb.MessageOptions {
	options := &descriptorpb.MessageOptions{}
	for _, set := range sets {
		set(options)
	}

	return options
}

// fieldOptions is a FieldOptions carrying the declarations each set applies.
func fieldOptions(sets ...func(*descriptorpb.FieldOptions)) *descriptorpb.FieldOptions {
	options := &descriptorpb.FieldOptions{}
	for _, set := range sets {
		set(options)
	}

	return options
}

// probeField is one synthesized argument. Numbers are assigned by position so a
// case names only what it is about.
func probeField(
	name string, number int32, kind descriptorpb.FieldDescriptorProto_Type, options *descriptorpb.FieldOptions,
) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:     new(name),
		Number:   new(number),
		Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:     kind.Enum(),
		JsonName: new(name),
		Options:  options,
	}
}

// repeatedField is probeField with the repeated label, for the checks whose
// subject is a list argument.
func repeatedField(
	name string, number int32, kind descriptorpb.FieldDescriptorProto_Type, options *descriptorpb.FieldOptions,
) *descriptorpb.FieldDescriptorProto {
	entry := probeField(name, number, kind, options)
	entry.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()

	return entry
}

// TestProbeDescriptorsCarryTheirDeclarations is the builder answering for
// itself: a synthesized message has to read back the options it was built with,
// or every refusal below would be measuring an empty declaration.
func TestProbeDescriptorsCarryTheirDeclarations(t *testing.T) {
	t.Parallel()

	message := probeMessage(t, "ProbeCarriesInput",
		messageOptions(withRoute("GET", "/probe"), withDescription("a probe")),
		probeField("region", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING,
			fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_QUERY))))

	route, carried := proto.GetExtension(message.Options(), linodev1.E_ToolRoute).(*linodev1.ToolRoute)
	if !carried || route.GetTool() == "" {
		t.Fatalf("synthesized message carries no tool_route: %#v", message.Options())
	}

	if route.GetPath() != "/probe" {
		t.Errorf("tool_route path = %q, want /probe", route.GetPath())
	}

	location, carriedLocation := proto.GetExtension(
		message.Fields().Get(0).Options(), linodev1.E_FieldLocation,
	).(linodev1.FieldLocation)
	if !carriedLocation || location != linodev1.FieldLocation_FIELD_LOCATION_QUERY {
		t.Errorf("field_location = %v, want QUERY", location)
	}
}

// withRoute names the tool and the route it calls, which is the declaration
// every emitted tier but meta is built from.
func withRoute(method, path string) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_ToolRoute, &linodev1.ToolRoute{
			Tool:   probeToolName,
			Method: method,
			Path:   path,
		})
	}
}

// probeToolName is the tool every probe declares. The emitter is handed one
// contract at a time, so the name only has to be spelled the same way twice.
const probeToolName = "linode_probe"

func withDescription(text string) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_ToolDescription, text)
	}
}

func withLocation(location linodev1.FieldLocation) func(*descriptorpb.FieldOptions) {
	return func(options *descriptorpb.FieldOptions) {
		proto.SetExtension(options, linodev1.E_FieldLocation, location)
	}
}

// refusalNamed is the error one name in errors.go is declared as, which is what
// a case is held to: the wording is prose the contract may reword, and the
// error is the thing that was raised.
func refusalNamed(t *testing.T, name string) error {
	t.Helper()

	refusal, declared := toolgen.ProbeRefusals()[name]
	if !declared {
		t.Fatalf("no refusal is declared as %s", name)
	}

	return refusal
}

// errorsFile is the emitter's refusals, relative to this package directory
// because go test runs with that as cwd.
const errorsFile = "errors.go"

// refusalTexts maps every error errors.go declares to the sentence it carries.
func refusalTexts(t *testing.T) map[string]string {
	t.Helper()

	parsed, err := parser.ParseFile(token.NewFileSet(), errorsFile, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", errorsFile, err)
	}

	texts := make(map[string]string, len(parsed.Decls))

	for _, declaration := range parsed.Decls {
		general, isVar := declaration.(*ast.GenDecl)
		if !isVar || general.Tok != token.VAR {
			continue
		}

		for _, spec := range general.Specs {
			name, text, found := refusalSpec(spec)
			if found {
				texts[name] = text
			}
		}
	}

	if len(texts) == 0 {
		t.Fatalf("%s declares no refusals", errorsFile)
	}

	return texts
}

// refusalSpec reads one `errX = errors.New("...")` line, reporting absent for
// anything else.
func refusalSpec(spec ast.Spec) (string, string, bool) {
	value, isValue := spec.(*ast.ValueSpec)
	if !isValue || len(value.Names) != 1 || len(value.Values) != 1 {
		return "", "", false
	}

	call, isCall := value.Values[0].(*ast.CallExpr)
	if !isCall || len(call.Args) != 1 {
		return "", "", false
	}

	literal, isLiteral := call.Args[0].(*ast.BasicLit)
	if !isLiteral || literal.Kind != token.STRING {
		return "", "", false
	}

	text, err := strconv.Unquote(literal.Value)
	if err != nil {
		return "", "", false
	}

	return value.Names[0].Name, text, true
}

// wantRefusal runs one probe and holds it to the refusal it is about.
func wantRefusal(t *testing.T, run *toolgen.ProbeRun, refusal string) {
	t.Helper()

	_, err := run.Emit()

	// A run that emitted answers with no error at all, which reads back here as
	// the refusal the case wanted and did not get.
	if want := refusalNamed(t, refusal); !errors.Is(err, want) {
		t.Errorf("refusal = %v, want %s (%v)", err, refusal, want)
	}
}
