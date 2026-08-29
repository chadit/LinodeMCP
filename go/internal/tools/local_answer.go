package tools

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/chadit/LinodeMCP/go/internal/genlocal"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
)

// The Go engine a declared local_answer runs on: the subsystem operations a
// meta tool is built from, and the readers their inputs arrive through.
//
// An operation is handed typed values and nothing else, so it can neither read
// the request nor tell which tool reached it. It answers a plain body and the
// condition it met without words, and the generated handler projects that body
// onto the message the tool's own contract declares and writes the sentence.
// Naming no message is the output half of the same guard the typed parameters
// are the input half of: two tools can share one operation and still answer
// their own declared shapes.

// The keys one call-list entry is read through. They belong to the entry's own
// shape rather than to any tool, so the reader spells them once.
const (
	localEntryTool      = "tool"
	localEntryArguments = "args"
)

// wellKnownPrefix names the messages whose body is free-form JSON rather than a
// member set the contract declares, so the nested rule stops at them.
const wellKnownPrefix = "google.protobuf."

// LocalRefusal is one condition an operation reports, spelled for the
// condition rather than for any tool so two tools sharing an operation share
// its report.
type LocalRefusal int

const (
	// LocalRefusalNone is what an operation that met no condition reports.
	LocalRefusalNone LocalRefusal = iota
	// LocalRefusalDraftMissing is no draft carrying the name the caller sent.
	LocalRefusalDraftMissing
	// LocalRefusalNotFound is nothing filed under the name the caller sent.
	LocalRefusalNotFound
	// LocalRefusalAlreadyExists is something already filed under that name.
	LocalRefusalAlreadyExists
	// LocalRefusalReadFailed is a store the operation could not read at all.
	LocalRefusalReadFailed
	// LocalRefusalWriteFailed is what the operation was asked to write and
	// could not.
	LocalRefusalWriteFailed
	// LocalRefusalInputRejected is a value the operation read and would not
	// take, for a vocabulary only the operation knows.
	LocalRefusalInputRejected
	// LocalRefusalBuiltinProfile is a name that already belongs to a profile
	// the binary carries, which nothing may be written over.
	LocalRefusalBuiltinProfile
)

// LocalOutcome is everything an operation answers: the body to project, or the
// condition it met and the cause a sentence may name.
type LocalOutcome struct {
	Body    map[string]any
	Cause   string
	Refusal LocalRefusal
}

// LocalAnswered is the outcome of an operation that ran to a body.
func LocalAnswered(body map[string]any) LocalOutcome {
	return LocalOutcome{Body: body, Cause: "", Refusal: LocalRefusalNone}
}

// LocalRefused is the outcome of an operation that met a condition, carrying
// the cause where the condition has one to report.
func LocalRefused(refusal LocalRefusal, cause string) LocalOutcome {
	return LocalOutcome{Body: nil, Cause: cause, Refusal: refusal}
}

// LocalUnreported is the outcome of an operation failing in a way its own
// declaration does not name. No tool words a sentence for it, so the outcome
// carries no body and the handler answers the disagreement between the body
// and the message rather than a body half of nothing.
func LocalUnreported(cause string) LocalOutcome {
	return LocalOutcome{Body: nil, Cause: cause, Refusal: LocalRefusalNone}
}

// LocalString reads one text input off the call. Reading through here rather
// than at each call site is what holds both languages to one coercion rule.
func LocalString(request *mcp.CallToolRequest, argument string) string {
	return request.GetString(argument, "")
}

// LocalInt reads one whole-number input off the call.
func LocalInt(request *mcp.CallToolRequest, argument string) int {
	return request.GetInt(argument, 0)
}

// LocalBool reads one flag input off the call.
func LocalBool(request *mcp.CallToolRequest, argument string) bool {
	return request.GetBool(argument, false)
}

// LocalBoolSent reads one flag input together with whether the call carried it,
// which a setter needs: false alone cannot separate a flag turned off from a
// flag the caller never mentioned.
func LocalBoolSent(request *mcp.CallToolRequest, argument string) *bool {
	if !localArgumentSent(request, argument) {
		return nil
	}

	sent := request.GetBool(argument, false)

	return &sent
}

// LocalStringList reads one list-of-text input off the call, empty rather than
// absent so a list member never projects as null.
func LocalStringList(request *mcp.CallToolRequest, argument string) []string {
	return append([]string{}, stringArrayArg(request, argument)...)
}

// LocalStringListSent reads one list-of-text input together with whether the
// call carried it, for the reason LocalBoolSent does: an empty list replaces a
// setting and an absent one leaves it alone.
func LocalStringListSent(request *mcp.CallToolRequest, argument string) *[]string {
	if !localArgumentSent(request, argument) {
		return nil
	}

	sent := LocalStringList(request, argument)

	return &sent
}

// localArgumentSent is whether the call carried the argument at all, which is
// the only thing separating "leave this alone" from "set it to nothing".
func localArgumentSent(request *mcp.CallToolRequest, argument string) bool {
	_, sent := request.GetArguments()[argument]

	return sent
}

// LocalTimestamp reads one RFC 3339 bound, answering the zero time for an
// absent one and a cause for a value it cannot read. The declaring tool's
// sentence names the argument, so this names only what the value got wrong.
func LocalTimestamp(request *mcp.CallToolRequest, argument string) (time.Time, string) {
	value := request.GetString(argument, "")
	if value == "" {
		return time.Time{}, ""
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Sprintf("expected RFC 3339, got %q: %v", value, err)
	}

	return parsed, ""
}

// LocalCallEntries reads one call-list input off the call. An entry that is
// not an object names no call and is dropped; a tool named as anything but
// text reads as empty, which is the answer the text reader gives too.
//
// The record it answers is the generated one, because a subsystem takes the
// list as a typed parameter and the answers tree is the one place both the
// engine and the handlers can name a type from.
func LocalCallEntries(request *mcp.CallToolRequest, argument string) []genlocal.CallEntry {
	raw, _ := request.GetArguments()[argument].([]any)
	entries := make([]genlocal.CallEntry, 0, len(raw))

	for _, item := range raw {
		object, isObject := item.(map[string]any)
		if !isObject {
			continue
		}

		name, _ := object[localEntryTool].(string)

		entries = append(entries, genlocal.CallEntry{
			Tool:        name,
			Environment: localEntryEnvironment(object),
		})
	}

	return entries
}

// localEntryEnvironment is the environment one entry's arguments name, empty
// where it named none or named it as something other than text. An empty
// environment and an absent one are one answer because neither says which
// environment the call would reach.
func localEntryEnvironment(entry map[string]any) string {
	arguments, carried := entry[localEntryArguments].(map[string]any)
	if !carried {
		return ""
	}

	environment, _ := arguments[paramEnvironment].(string)

	return environment
}

// LocalResponse projects what an operation answered onto the message its tool
// declares. Projecting here rather than in each generated handler is what keeps
// the answer canonical, the way MarshalProtoToolResponse does for a route.
func LocalResponse(message proto.Message, body map[string]any) (*mcp.CallToolResult, error) {
	if refusal := fillLocalBody(message, body); refusal != "" {
		return mcp.NewToolResultError(refusal), nil
	}

	return MarshalProtoToolResponse(message)
}

// LocalBodyJSON is one plain body as the canonical bytes its message
// serializes to, which is how a surface outside MCP prints what a tool answers.
// A body that disagrees with the message is reported here for the same reason
// LocalResponse reports it, and in the same words.
func LocalBodyJSON(message proto.Message, body map[string]any) ([]byte, error) {
	if refusal := fillLocalBody(message, body); refusal != "" {
		return nil, fmt.Errorf("%w: %s", ErrLocalBody, refusal)
	}

	return MarshalProtoJSON(message)
}

// fillLocalBody writes a plain body onto its message, answering the first
// disagreement between the two and "" where they agree.
//
// Both directions are held because either one answers a caller a shape the
// contract does not describe: a member the body leaves out would report a zero
// nothing computed, and one the message does not declare would be dropped in
// silence.
func fillLocalBody(message proto.Message, body map[string]any) string {
	descriptor := message.ProtoReflect().Descriptor()

	if refusal := localBodyMembers(descriptor, body); refusal != "" {
		return refusal
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Sprintf("%s: the answer does not serialize: %v", descriptor.FullName(), err)
	}

	// The nested rule reads the canonical JSON rather than the body itself,
	// because a member the engine hands over is the subsystem's own record and
	// not a plain map. Bytes json.Marshal just wrote over a map always decode
	// back into one, so nothing here can answer a second failure.
	var records map[string]any

	_ = json.Unmarshal(data, &records)

	if refusal := localNestedMembers(descriptor, records); refusal != "" {
		return refusal
	}

	if err := protojson.Unmarshal(data, message); err != nil {
		return fmt.Sprintf("%s: the answer does not fit: %v", descriptor.FullName(), err)
	}

	return ""
}

// localNestedMembers holds every record nested in a body to the message that
// record fills, which the top-level rule alone cannot see.
//
// protojson reads a member a nested record leaves out as that member's zero, so
// an incomplete record answers a value nothing computed and reports success.
// The body is read as the canonical JSON it serializes to, because a member the
// engine hands over is the subsystem's own value rather than a plain map.
func localNestedMembers(descriptor protoreflect.MessageDescriptor, body map[string]any) string {
	members := descriptor.Fields()

	for index := range members.Len() {
		member := members.Get(index)

		nested := localNestedDescriptor(member)
		if nested == nil {
			continue
		}

		if refusal := localNestedMember(member, nested, body[string(member.Name())]); refusal != "" {
			return refusal
		}
	}

	return ""
}

// localNestedDescriptor is the message one member's records fill, nil where the
// member carries none.
func localNestedDescriptor(member protoreflect.FieldDescriptor) protoreflect.MessageDescriptor {
	if member.IsMap() {
		return localRecordDescriptor(member.MapValue())
	}

	return localRecordDescriptor(member)
}

// localRecordDescriptor is the declared member set behind a value, nil for a
// scalar and nil for a well-known wrapper, whose body is free-form JSON rather
// than a member set the contract declares.
func localRecordDescriptor(member protoreflect.FieldDescriptor) protoreflect.MessageDescriptor {
	if member.Kind() != protoreflect.MessageKind && member.Kind() != protoreflect.GroupKind {
		return nil
	}

	nested := member.Message()
	if strings.HasPrefix(string(nested.FullName()), wellKnownPrefix) {
		return nil
	}

	return nested
}

// localNestedMember holds the records one member carries: a map's values, a
// list's elements, or the single record a member holds.
func localNestedMember(
	member protoreflect.FieldDescriptor, nested protoreflect.MessageDescriptor, value any,
) string {
	if member.IsMap() {
		return localNestedRecords(nested, localKeyedRecords(value))
	}

	if member.IsList() {
		held, _ := value.([]any)

		return localNestedRecords(nested, held)
	}

	if value == nil {
		return ""
	}

	return localNestedRecords(nested, []any{value})
}

// localKeyedRecords is a keyed member's values in key order, so one
// disagreement is reported the same way on every run.
func localKeyedRecords(value any) []any {
	held, _ := value.(map[string]any)

	records := make([]any, 0, len(held))
	for _, key := range slices.Sorted(maps.Keys(held)) {
		records = append(records, held[key])
	}

	return records
}

// localNestedRecords holds every record in one member to the message it fills,
// its own nested members included.
func localNestedRecords(nested protoreflect.MessageDescriptor, records []any) string {
	for _, record := range records {
		held, ok := record.(map[string]any)
		if !ok {
			continue
		}

		refusal := localBodyMembers(nested, held)
		if refusal == "" {
			refusal = localNestedMembers(nested, held)
		}

		if refusal != "" {
			return refusal
		}
	}

	return ""
}

// localBodyMembers holds a body's own members to the message's, reading the
// message in declaration order and the body in name order so one disagreement
// is reported the same way on every run and in both languages.
func localBodyMembers(descriptor protoreflect.MessageDescriptor, body map[string]any) string {
	members := descriptor.Fields()

	for index := range members.Len() {
		name := string(members.Get(index).Name())
		if _, filled := body[name]; !filled {
			return fmt.Sprintf("%s: the answer leaves %s unfilled", descriptor.FullName(), name)
		}
	}

	for _, name := range slices.Sorted(maps.Keys(body)) {
		if members.ByName(protoreflect.Name(name)) == nil {
			return fmt.Sprintf("%s: the answer names %s, which the message does not declare",
				descriptor.FullName(), name)
		}
	}

	return ""
}

// draftAnswer is one draft as the answer a draft operation fills.
func draftAnswer(draft *builder.Draft) *genlocal.ProfileDraftResponse {
	return genlocal.NewProfileDraftResponse(draft.Name, draft.Description,
		draft.AllowedTools, draft.AllowedEnvironments, draft.RequiredTokenScopes,
		draft.AllowYolo)
}
