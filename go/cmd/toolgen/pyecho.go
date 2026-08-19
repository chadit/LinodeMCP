package main

import (
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// pyEchoArguments is the map an acknowledged answer's echo is filled from, one
// entry per echoed field.
//
// A path id reaches it through the same expression the route does, so a tool
// cannot address one resource and report another. A body argument is read
// straight off the call, since that is what it was sent as.
//
// Keyed by the argument rather than by the response member, which is what the
// destroy tier hands its ids in under: with the two keyed alike, one runtime
// reader resolves both, and an aliased member still reaches its value.
func pyEchoArguments(tool *pyTool) string {
	entries := make([]string, 0, tool.declared.Fields().Len())

	for i := range tool.declared.Fields().Len() {
		entry := tool.declared.Fields().Get(i)
		if string(entry.Name()) == messageFieldName || pyIsPayloadMember(entry) {
			continue
		}

		argument := echoArgumentOf(entry)

		value := pyPathEcho(tool, argument)
		if value == "" {
			value = pyEchoBodyValue(tool, entry, pyEchoAbsent(entry))
		}

		entries = append(entries, pyQuote(argument)+": "+value)
	}

	return "{" + strings.Join(entries, ", ") + "}"
}

// pyWriteEchoArguments is the map a mutation's echoed members are filled from.
// A path id reaches it through the same expression the route does; a body
// argument is read straight off the call, since that is what it was sent as.
func pyWriteEchoArguments(tool *pyTool) string {
	entries := make([]string, 0, len(tool.echoes))

	for _, name := range tool.echoes {
		member := tool.declared.Fields().ByName(protoreflect.Name(name))
		argument := echoArgumentOf(member)

		value := pyPathEcho(tool, argument)
		if value == "" {
			value = pyEchoBodyValue(tool, member, pyBodyEchoAbsent(tool, argument))
		}

		entries = append(entries, pyQuote(argument)+": "+value)
	}

	return "{" + strings.Join(entries, ", ") + "}"
}

// pyAssembledEchoArguments is the members an assembled read reports back, keyed
// by the member itself. The write tier keys its echo map by argument because
// its driver maps one to the other; here the driver places the members
// directly, so a response naming an argument under another name resolves
// through the alias once, at this call site.
func pyAssembledEchoArguments(tool *pyTool) string {
	entries := make([]string, 0, len(tool.echoes))

	for _, name := range tool.echoes {
		member := tool.declared.Fields().ByName(protoreflect.Name(name))
		argument := echoArgumentOf(member)
		path := tool.pathArg(argument)

		entries = append(entries, pyQuote(name)+": "+path.coerce+
			"("+pyLocal(argument)+" or "+path.absent+")")
	}

	return "{" + strings.Join(entries, ", ") + "}"
}

// pyPathEcho is the expression one echoed path id is filled from, "" when the
// argument is not a path field.
func pyPathEcho(tool *pyTool, argument string) string {
	for i := range tool.c.Path {
		if tool.c.Path[i].ProtoName != argument {
			continue
		}

		path := tool.pathArg(argument)

		return path.coerce + "(" + pyLocal(argument) + " or " + path.absent + ")"
	}

	return ""
}

// pyIsPayloadMember reports whether a response member carries the resource the
// API sent back. A repeated message is not one: the decoded resource lands in a
// singular member, so a list beside it is assembled from the call. Neither is a
// member declared LOCAL, which says the route sent no body to decode into it.
func pyIsPayloadMember(entry protoreflect.FieldDescriptor) bool {
	return pyMessageOf(entry) != nil && !pyRepeated(entry) && !pyIsLocalMember(entry)
}

// pyIsLocalMember reports whether a response member says the MCP layer
// assembles it. FIELD_LOCATION_LOCAL means a value that never crosses the
// Linode boundary, so on a response it is a member the route answers with
// nothing for and it is rebuilt from the call.
func pyIsLocalMember(entry protoreflect.FieldDescriptor) bool {
	location, ok := proto.GetExtension(entry.Options(), linodev1.E_FieldLocation).(linodev1.FieldLocation)

	return ok && location == linodev1.FieldLocation_FIELD_LOCATION_LOCAL
}

// pyEchoBodyValue is the expression one echoed body member is filled from. A
// list is rebuilt from the body the reader accepted rather than from the raw
// call, since that is the value the request carried.
func pyEchoBodyValue(tool *pyTool, entry protoreflect.FieldDescriptor, absent string) string {
	if message := pyMessageOf(entry); message != nil {
		if !pyRepeated(entry) {
			return pyEchoMessage(message)
		}

		return pyEchoItems(tool, string(entry.Name()), message)
	}

	argument := echoArgumentOf(entry)
	if pyRepeated(entry) {
		return "body.get(" + pyQuote(pyEchoWireName(tool, argument)) + ", [])"
	}

	return "arguments.get(" + pyQuote(argument) + ", " + absent + ")"
}

// pyEchoMessage is the assembled object a single named-message echo reports.
// Each member reads the call under its own name, in the order the response
// declares them.
func pyEchoMessage(declared protoreflect.MessageDescriptor) string {
	members := make([]string, 0, declared.Fields().Len())

	for i := range declared.Fields().Len() {
		member := declared.Fields().Get(i)
		members = append(members, pyQuote(string(member.Name()))+
			": arguments.get("+pyQuote(string(member.Name()))+", "+pyEchoAbsent(member)+")")
	}

	return "{" + strings.Join(members, ", ") + "}"
}

// pyEchoItems is the mapped list a repeated named-message echo reports. The
// response declares an item message of its own, so each item the body reader
// accepted is rebuilt member by member in the order the response declares them.
func pyEchoItems(tool *pyTool, name string, declared protoreflect.MessageDescriptor) string {
	members := make([]string, 0, declared.Fields().Len())

	for i := range declared.Fields().Len() {
		member := declared.Fields().Get(i)
		members = append(members, pyQuote(string(member.Name()))+
			": item.get("+pyQuote(string(member.Name()))+", "+pyEchoAbsent(member)+")")
	}

	return "[{" + strings.Join(members, ", ") + "} for item in body.get(" +
		pyQuote(pyEchoWireName(tool, name)) + ", [])]"
}

// pyEchoWireName is the body key an echoed list was written under, rename
// included.
func pyEchoWireName(tool *pyTool, name string) string {
	for _, entry := range tool.bodyFields() {
		if string(entry.Name()) == name {
			return wireNameOf(entry)
		}
	}

	return name
}

// pyEchoAbsent is what an echoed body argument reads as when the caller omitted
// it. Both hand handlers read an absent cors_enabled as false rather than
// leaving it out of the answer, so a flag omitted reports false the way an
// omitted string reports "".
func pyEchoAbsent(entry protoreflect.FieldDescriptor) string {
	if pyRepeated(entry) {
		return "[]"
	}

	if entry.Kind() == protoreflect.Int32Kind || entry.Kind() == protoreflect.Int64Kind {
		return "0"
	}

	if entry.Kind() == protoreflect.BoolKind {
		return pyFalse
	}

	return emptyLiteral
}

// pyBodyEchoAbsent is what an echoed body argument reads as when the caller
// omitted it, read off the argument the mutation declared rather than the
// member reporting it.
func pyBodyEchoAbsent(tool *pyTool, name string) string {
	for _, entry := range tool.bodyFields() {
		if string(entry.Name()) != name {
			continue
		}

		if entry.Kind() == protoreflect.Int32Kind || entry.Kind() == protoreflect.Int64Kind {
			return "0"
		}
	}

	return `""`
}
