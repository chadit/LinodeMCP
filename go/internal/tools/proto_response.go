package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/structpb"
)

// jsonNullLiteral is the wire form of an explicit null, which is what separates
// a key the API sent as null from one it never sent.
const jsonNullLiteral = "null"

// MarshalProtoToolResponse serializes a proto message with the canonical
// options (snake_case field names, default values emitted) and wraps it in an
// MCP text result. Proto-backed tools use this so their output is byte-identical
// to the Python implementation, which serializes the same message with the
// matching MessageToJson options.
func MarshalProtoToolResponse(msg proto.Message) (*mcp.CallToolResult, error) {
	data, err := MarshalProtoJSON(msg)
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(string(data)), nil
}

// MarshalProtoJSON serializes a proto message with the canonical options and
// returns the raw JSON bytes. MarshalProtoToolResponse wraps this for MCP tool
// output; the CLI version subcommand marshals the same VersionResponse message
// with it so the tool and the subcommand emit identical bytes. After the
// protojson marshal, 64-bit integer fields are widened back to JSON numbers
// (see widenInt64JSON) and the result is re-indented; json.Indent also gives
// stable whitespace where protojson randomizes the space after a colon.
func MarshalProtoJSON(msg proto.Message) ([]byte, error) {
	widened, err := marshalProtoCompact(msg)
	if err != nil {
		return nil, err
	}

	return indentProtoJSON(widened)
}

// marshalProtoCompact is MarshalProtoJSON without the final indent, so a caller
// editing the object can do it before the whitespace is fixed.
func marshalProtoCompact(msg proto.Message) ([]byte, error) {
	data, err := protojson.MarshalOptions{
		UseProtoNames:     true,
		EmitDefaultValues: true,
	}.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal proto response: %w", err)
	}

	widened, err := widenInt64JSON(data, msg.ProtoReflect().Descriptor())
	if err != nil {
		return nil, fmt.Errorf("failed to widen 64-bit fields: %w", err)
	}

	return widened, nil
}

// indentProtoJSON applies the shared whitespace, which protojson randomizes.
func indentProtoJSON(compact []byte) ([]byte, error) {
	var indented bytes.Buffer
	if err := json.Indent(&indented, compact, "", "  "); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrResponseIndent, err)
	}

	return indented.Bytes(), nil
}

// MarshalProtoToolResponseRestoringNulls serializes msg and writes back the
// named keys the raw API body sent as an explicit null.
//
// protojson emits no member for an unset optional scalar or an absent
// sub-message even with default values emitted, so a documented "gateway": null
// comes back missing and a caller cannot tell "no gateway" from "the API said
// nothing about one". member names the response field the raw body decoded
// into, empty when the response is that body itself.
func MarshalProtoToolResponseRestoringNulls(
	msg proto.Message, raw json.RawMessage, member string, names []string,
) (*mcp.CallToolResult, error) {
	compact, err := marshalProtoCompact(msg)
	if err != nil {
		return nil, err
	}

	descriptor := msg.ProtoReflect().Descriptor()

	restored, err := restoreNulls(compact, raw, descriptor, member, names)
	if err != nil {
		return nil, err
	}

	indented, err := indentProtoJSON(restored)
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(string(indented)), nil
}

// MarshalProtoListResponseRestoringNulls serializes a page and writes the named
// keys the API sent as an explicit null back into the element each came from.
//
// A page's nulls are its elements', never its own: the count and the filter
// echo are assembled from the call, so nothing at the envelope level ever comes
// back null. raws are the element bodies in arrival order, which is the order
// the page serializes them in, so element i restores from raw i. A page whose
// two lists disagree in length is refused rather than restored off by one.
func MarshalProtoListResponseRestoringNulls(
	msg proto.Message, raws []json.RawMessage, names []string,
) (*mcp.CallToolResult, error) {
	compact, err := marshalProtoCompact(msg)
	if err != nil {
		return nil, err
	}

	descriptor := msg.ProtoReflect().Descriptor()

	page := pageMember(descriptor)
	if page == nil {
		return nil, fmt.Errorf("%w: %s carries no page member", ErrNullMember, descriptor.FullName())
	}

	restored, err := restoredPage(compact, raws, descriptor, page, names)
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(string(restored)), nil
}

// pageMember is the repeated message field a list envelope holds its elements
// in, nil when the message declares none.
func pageMember(descriptor protoreflect.MessageDescriptor) protoreflect.FieldDescriptor {
	fields := descriptor.Fields()

	for index := range fields.Len() {
		field := fields.Get(index)
		if field.IsList() && field.Message() != nil {
			return field
		}
	}

	return nil
}

// restoredPage rebuilds a serialized page with each element's nulls written
// back, answering the finished bytes so the whole restoration reports through
// one failure rather than one per step.
func restoredPage(
	serialized json.RawMessage,
	raws []json.RawMessage,
	descriptor protoreflect.MessageDescriptor,
	page protoreflect.FieldDescriptor,
	names []string,
) ([]byte, error) {
	member := string(page.Name())
	fields, items := pageElements(serialized, member)

	if len(items) != len(raws) {
		return nil, fmt.Errorf("%w: %d serialized elements and %d raw ones",
			ErrPageElementCount, len(items), len(raws))
	}

	for index := range items {
		element, err := restoreObjectNulls(items[index], raws[index], page.Message(), names)
		if err != nil {
			return nil, err
		}

		items[index] = element
	}

	fields[member] = encodeArray(items)

	return indentProtoJSON(encodeObject(fields, descriptor))
}

// pageElements splits a serialized page into its members and its elements.
//
// Neither decode can fail on bytes protojson produced, so a failure leaves the
// split empty rather than taking a branch of its own: the element count above
// then reports it as the mismatch it would be, and a branch no input can reach
// would ship untested.
func pageElements(
	serialized json.RawMessage, member string,
) (map[string]json.RawMessage, []json.RawMessage) {
	fields := map[string]json.RawMessage{}

	_ = json.Unmarshal(serialized, &fields)

	var items []json.RawMessage

	_ = json.Unmarshal(fields[member], &items)

	return fields, items
}

// encodeArray writes elements back as a JSON array, keeping each element's
// bytes as the restoration left them.
func encodeArray(items []json.RawMessage) []byte {
	var out bytes.Buffer

	out.WriteByte('[')

	for index, item := range items {
		if index > 0 {
			out.WriteByte(',')
		}

		out.Write(item)
	}

	out.WriteByte(']')

	return out.Bytes()
}

// restoreNulls writes the named nulls into the serialized object, or into its
// member when the raw body decoded into one.
func restoreNulls(
	serialized, raw json.RawMessage,
	descriptor protoreflect.MessageDescriptor,
	member string,
	names []string,
) ([]byte, error) {
	if member == "" {
		return restoreObjectNulls(serialized, raw, descriptor, names)
	}

	fields, err := decodeObject(serialized)
	if err != nil {
		return nil, err
	}

	nested := descriptor.Fields().ByName(protoreflect.Name(member))
	if nested == nil || nested.Message() == nil {
		return nil, fmt.Errorf("%w: %s carries no message field %s", ErrNullMember, descriptor.FullName(), member)
	}

	inner, err := restoreObjectNulls(fields[member], raw, nested.Message(), names)
	if err != nil {
		return nil, err
	}

	fields[member] = inner

	return encodeObject(fields, descriptor), nil
}

// restoreObjectNulls rebuilds one serialized message with the named nulls the
// raw body carried. A raw body that is not an object carried no key to restore,
// so the serialized bytes stand as they are.
func restoreObjectNulls(
	serialized, raw json.RawMessage,
	descriptor protoreflect.MessageDescriptor,
	names []string,
) ([]byte, error) {
	if !isJSONObject(raw) {
		return serialized, nil
	}

	rawFields, err := decodeObject(raw)
	if err != nil {
		return nil, err
	}

	fields, err := decodeObject(serialized)
	if err != nil {
		return nil, err
	}

	var changed bool

	for _, name := range names {
		value, sent := rawFields[name]
		if _, present := fields[name]; present || !sent || !isJSONNull(value) {
			continue
		}

		fields[name] = json.RawMessage(jsonNullLiteral)
		changed = true
	}

	if !changed {
		return serialized, nil
	}

	return encodeObject(fields, descriptor), nil
}

// decodeObject reads a JSON object into its members, keeping each value's bytes.
func decodeObject(data json.RawMessage) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage

	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrResponseDecode, err)
	}

	if fields == nil {
		return nil, fmt.Errorf("%w: body is JSON null", ErrNullMember)
	}

	return fields, nil
}

// encodeObject writes the members back in declared field order, which is the
// order protojson emitted them in, so restoring a null cannot reorder the rest.
func encodeObject(fields map[string]json.RawMessage, descriptor protoreflect.MessageDescriptor) []byte {
	var out bytes.Buffer

	out.WriteByte('{')

	declared := descriptor.Fields()
	for index := range declared.Len() {
		name := string(declared.Get(index).Name())

		value, present := fields[name]
		if !present {
			continue
		}

		if out.Len() > 1 {
			out.WriteByte(',')
		}

		out.WriteString(strconv.Quote(name))
		out.WriteByte(':')
		out.Write(value)
	}

	out.WriteByte('}')

	return out.Bytes()
}

// isJSONNull reports whether a member's bytes are the JSON null literal.
func isJSONNull(value json.RawMessage) bool {
	return string(bytes.TrimSpace(value)) == jsonNullLiteral
}

// isJSONObject reports whether a body opens as a JSON object, which is the one
// shape carrying members to restore.
func isJSONObject(raw json.RawMessage) bool {
	opening := bytes.TrimLeft(raw, " \t\r\n")

	return len(opening) > 0 && opening[0] == '{'
}

// MarshalStructToolResponse serializes a free-form map through a bare
// google.protobuf.Struct and marshals it with MarshalProtoToolResponse. Read
// tools whose API response is an open-ended object (engine config descriptors,
// profile preferences, managed stats) use this so both languages emit the same
// deterministically key-sorted object; protojson sorts Struct keys on both
// sides, replacing the pre-proto split where Go alpha-sorted map keys and Python
// preserved the API's insertion order. failMessage prefixes a conversion error.
func MarshalStructToolResponse(raw map[string]any, failMessage string) (*mcp.CallToolResult, error) {
	structVal, err := structpb.NewStruct(raw)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("%s: %v", failMessage, err)), nil
	}

	return MarshalProtoToolResponse(structVal)
}
