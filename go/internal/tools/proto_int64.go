package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// widenCtx carries the schema context of the next JSON value during the
// widening walk. Exactly one of the fields is set: msg for a message object,
// list for a repeated field's array, mapValue for a map object's value type,
// field for a scalar, and free for a subtree with no schema (Struct/Value
// content or an unknown key).
type widenCtx struct {
	msg      protoreflect.MessageDescriptor
	list     protoreflect.FieldDescriptor
	mapValue protoreflect.FieldDescriptor
	field    protoreflect.FieldDescriptor
	free     bool
}

// widenInt64JSON rewrites compact protojson output so 64-bit integer fields
// emit as JSON numbers instead of the proto3 JSON mapping's quoted strings.
// The mapping quotes them so JavaScript consumers never round values past
// 2^53, but this project's contract wants numbers to be numbers: JSON itself
// carries arbitrary-precision integers, protojson parsers accept both forms
// on decode, and the realistic values here (byte counters, latencies, counts)
// sit far below 2^53. The walk is descriptor-driven so digit strings in
// free-form Struct/Value subtrees (for example redacted audit args) are left
// untouched, and it preserves protojson's field order, which a decode into a
// Go map would not.
func widenInt64JSON(data []byte, desc protoreflect.MessageDescriptor) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var buf bytes.Buffer

	rootCtx := widenCtx{msg: desc}
	if isFreeformMessage(desc.FullName()) {
		rootCtx = widenCtx{free: true}
	}

	if err := widenValue(decoder, &buf, rootCtx); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// widenValue consumes one JSON value from decoder and writes its widened form.
func widenValue(decoder *json.Decoder, buf *bytes.Buffer, valueCtx widenCtx) error {
	tok, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("failed to read token: %w", err)
	}

	switch typed := tok.(type) {
	case json.Delim:
		if typed == '{' {
			return widenObject(decoder, buf, valueCtx)
		}

		return widenArray(decoder, buf, valueCtx)
	case string:
		if !valueCtx.free && valueCtx.field != nil && is64BitInt(valueCtx.field) && isIntegerLiteral(typed) {
			buf.WriteString(typed)

			return nil
		}

		return writeJSONString(buf, typed)
	case json.Number:
		buf.WriteString(typed.String())

		return nil
	case bool:
		buf.WriteString(strconv.FormatBool(typed))

		return nil
	default:
		buf.WriteString("null")

		return nil
	}
}

// widenObject writes the object whose opening brace was already consumed.
func widenObject(decoder *json.Decoder, buf *bytes.Buffer, objectCtx widenCtx) error {
	buf.WriteByte('{')

	first := true

	for decoder.More() {
		keyTok, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("failed to read object key: %w", err)
		}

		key, ok := keyTok.(string)
		if !ok {
			return fmt.Errorf("%w: %v", errUnexpectedKeyToken, keyTok)
		}

		if !first {
			buf.WriteByte(',')
		}

		first = false

		if err := writeJSONString(buf, key); err != nil {
			return err
		}

		buf.WriteByte(':')

		if err := widenValue(decoder, buf, objectValueCtx(objectCtx, key)); err != nil {
			return err
		}
	}

	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("failed to read object close: %w", err)
	}

	buf.WriteByte('}')

	return nil
}

// widenArray writes the array whose opening bracket was already consumed.
func widenArray(decoder *json.Decoder, buf *bytes.Buffer, arrayCtx widenCtx) error {
	buf.WriteByte('[')

	elemCtx := widenCtx{free: true}
	if arrayCtx.list != nil {
		elemCtx = singleValueCtx(arrayCtx.list)
	}

	first := true

	for decoder.More() {
		if !first {
			buf.WriteByte(',')
		}

		first = false

		if err := widenValue(decoder, buf, elemCtx); err != nil {
			return err
		}
	}

	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("failed to read array close: %w", err)
	}

	buf.WriteByte(']')

	return nil
}

// objectValueCtx resolves the context of the value stored under key in an
// object whose own context is objectCtx.
func objectValueCtx(objectCtx widenCtx, key string) widenCtx {
	if objectCtx.mapValue != nil {
		return singleValueCtx(objectCtx.mapValue)
	}

	if objectCtx.msg == nil {
		return widenCtx{free: true}
	}

	fieldDesc := objectCtx.msg.Fields().ByTextName(key)
	if fieldDesc == nil {
		return widenCtx{free: true}
	}

	if fieldDesc.IsMap() {
		return widenCtx{mapValue: fieldDesc.MapValue()}
	}

	if fieldDesc.IsList() {
		return widenCtx{list: fieldDesc}
	}

	return singleValueCtx(fieldDesc)
}

// singleValueCtx resolves the context of a single value typed by fieldDesc (a
// singular field, a list element, or a map value).
func singleValueCtx(fieldDesc protoreflect.FieldDescriptor) widenCtx {
	if fieldDesc.Kind() == protoreflect.MessageKind || fieldDesc.Kind() == protoreflect.GroupKind {
		if isFreeformMessage(fieldDesc.Message().FullName()) {
			return widenCtx{free: true}
		}

		return widenCtx{msg: fieldDesc.Message()}
	}

	return widenCtx{field: fieldDesc}
}

// isFreeformMessage reports whether name is a well-known type whose JSON form
// is free-form user data. Digit-only strings inside those subtrees are genuine
// strings, so the widening pass copies them verbatim.
func isFreeformMessage(name protoreflect.FullName) bool {
	switch name {
	case "google.protobuf.Struct", "google.protobuf.Value", "google.protobuf.ListValue":
		return true
	default:
		return false
	}
}

// is64BitInt reports whether fieldDesc is one of the five 64-bit integer
// kinds the proto3 JSON mapping quotes. A boolean chain instead of a kind
// switch keeps the exhaustive lint out of play.
func is64BitInt(fieldDesc protoreflect.FieldDescriptor) bool {
	kind := fieldDesc.Kind()

	return kind == protoreflect.Int64Kind || kind == protoreflect.Sint64Kind ||
		kind == protoreflect.Uint64Kind || kind == protoreflect.Fixed64Kind ||
		kind == protoreflect.Sfixed64Kind
}

// isIntegerLiteral reports whether text is a bare decimal integer, the only
// form protojson emits for 64-bit fields. Anything else copies back as a
// string.
func isIntegerLiteral(text string) bool {
	if text == "" {
		return false
	}

	digits := text
	if text[0] == '-' {
		digits = text[1:]
	}

	if digits == "" {
		return false
	}

	for i := range len(digits) {
		if digits[i] < '0' || digits[i] > '9' {
			return false
		}
	}

	return true
}

// writeJSONString re-encodes text as a JSON string without HTML escaping,
// matching protojson's raw emission of <, >, and &.
func writeJSONString(buf *bytes.Buffer, text string) error {
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(text); err != nil {
		return fmt.Errorf("failed to encode string: %w", err)
	}

	// Encoder.Encode appends a newline; the caller controls layout.
	buf.Truncate(buf.Len() - 1)

	return nil
}
