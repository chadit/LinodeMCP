package main

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The reader constants the Python body builder holds a value to, which are the
// item reader's own: a folded argument is held to exactly what a typed message
// member is held to.
const (
	pyItemStr    = "ITEM_STR"
	pyItemInt    = "ITEM_INT"
	pyItemBool   = "ITEM_BOOL"
	pyItemObject = "ITEM_OBJECT"
)

// pyItemKind is the constant one resolved fold kind is written as, and whether
// the kind has a Python reader at all.
func pyItemKind(kind string) (string, bool) {
	switch kind {
	case itemStringKind:
		return pyItemStr, true
	case itemIntKind:
		return pyItemInt, true
	case itemBoolKind:
		return pyItemBool, true
	case itemObjectKind:
		return pyItemObject, true
	}

	return "", false
}

// pyBody is the request-body builder: one line per BODY field, in declaration
// order. Declaration order is wire order, and Go assembles the same body the
// same way, so the two languages put identical bytes on the wire for one call.
func pyBody(tool *pyTool) ([]string, error) {
	lines := []string{
		"def " + pyBodyName(tool.c.Name) + "(",
		"    arguments: dict[str, Any],",
		") -> tuple[dict[str, Any], str | None]:",
		`    """Build the ` + tool.c.Method + ` body in proto field order."""`,
		"    body = WriteBody(arguments)",
	}

	for _, entry := range tool.bodyFields() {
		if declared := bodyNameOf(entry); declared != "" {
			lines = append(lines, "    body.rename("+pyQuote(string(entry.Name()))+
				", "+pyQuote(declared)+")")
		}
	}

	for _, entry := range tool.bodyFields() {
		name := string(entry.Name())

		// A folded argument travels inside the member it was declared into, so
		// writing it here too would post it where the API does not read it.
		if tool.c.Folded[name] {
			continue
		}

		if _, assembled := tool.c.Folds[name]; assembled {
			fold, err := pyFold(tool, entry)
			if err != nil {
				return nil, err
			}

			lines = append(lines, fold...)

			continue
		}

		setter, err := pyBodySetter(tool.c.Name, entry)
		if err != nil {
			return nil, err
		}

		message := pyTypedMessage(entry)
		if message == nil {
			lines = append(lines, "    body."+setter+"("+pyQuote(name)+")")

			continue
		}

		fields, err := pyMessageFields(tool.c.Name, setter, entry, message)
		if err != nil {
			return nil, err
		}

		lines = append(lines, fields...)
	}

	for _, constant := range tool.c.Constants {
		literal, err := pyLiteralOf(constant.Literal)
		if err != nil {
			return nil, err
		}

		lines = append(lines, "    body.constant("+pyQuote(constant.Name)+", "+literal+")")
	}

	return append(lines, "    return body.result()"), nil
}

// pyFold is the spec an assembled body member is built from, one entry per
// member. The list form is what the API reads as an array and it synthesizes a
// single element rather than merging: a caller who supplied the list gets it
// sent verbatim, since an element beside theirs attaches something they never
// asked for. The optional form leaves a member nothing filled off the wire.
func pyFold(tool *pyTool, entry protoreflect.FieldDescriptor) ([]string, error) {
	name := string(entry.Name())

	call := "fold"

	switch {
	case pyItemMessage(entry) != nil:
		call = "fold_list"
	case tool.c.FoldDecls[name].GetOmitWhenEmpty():
		call = "fold_optional"
	}

	lines := []string{"    body." + call + "(" + pyQuote(name) + ", ("}

	for _, member := range tool.c.Folds[name] {
		kind, known := pyItemKind(member.Kind)
		if !known {
			return nil, fmt.Errorf("%w: %s folds %s.%s as %s, which has no Python reader",
				errPyRender, tool.c.Name, name, member.Key, member.Kind)
		}

		parts := []string{pyQuote(member.Key), kind}
		if member.Argument != "" {
			parts = append(parts, "argument="+pyQuote(member.Argument))
		}

		if member.Literal != "" {
			literal, err := pyLiteralOf(member.Literal)
			if err != nil {
				return nil, err
			}

			parts = append(parts, "value="+literal)
		}

		lines = append(lines, "        FoldMember("+strings.Join(parts, ", ")+"),")
	}

	return append(lines, "    ))"), nil
}

// pyLiteralOf is one resolved default as Python source. The contract resolves
// the value once and stores it as Go source, so the flag spellings and the
// empty object are respelled here rather than re-read from the declaration.
func pyLiteralOf(literal string) (string, error) {
	switch literal {
	case trueText:
		return pyTrue, nil
	case falseText:
		return pyFalse, nil
	case "map[string]any{}":
		return "{}", nil
	}

	if _, err := strconv.Atoi(literal); err == nil {
		return literal, nil
	}

	text, err := strconv.Unquote(literal)
	if err != nil {
		return "", fmt.Errorf("%w: %q is no value the Python renderer can spell", errPyRender, literal)
	}

	return pyQuote(text), nil
}

// pyMessageFields is a typed message setter: the field name, then its message's
// members. The spec travels to the reader inline because the shape is the
// contract's, and a reader handed nothing would accept whatever was sent.
func pyMessageFields(
	tool, setter string, entry protoreflect.FieldDescriptor, message protoreflect.MessageDescriptor,
) ([]string, error) {
	members, err := pyItemFields(tool, entry, message, "        ", nil)
	if err != nil {
		return nil, err
	}

	lines := make([]string, 0, len(members)+2)
	lines = append(lines, "    body."+setter+"("+pyQuote(string(entry.Name()))+", (")
	lines = append(lines, members...)

	return append(lines, "    ))"), nil
}

// pyItemFields is one message's members, in declaration order, at the given
// indent. A member carrying a message of its own is written out under the same
// rules one indent deeper. seen carries the messages this field is already
// inside, so a declaration that reaches itself fails the run by name rather
// than rendering until the renderer runs out of stack.
func pyItemFields(
	tool string, entry protoreflect.FieldDescriptor,
	message protoreflect.MessageDescriptor, indent string, seen []string,
) ([]string, error) {
	if slices.Contains(seen, string(message.FullName())) {
		return nil, fmt.Errorf("%w: %s body field %s reaches %s again, which contains itself",
			errPyRender, tool, entry.Name(), message.FullName())
	}

	seen = append(seen, string(message.FullName()))

	lines := make([]string, 0, message.Fields().Len())

	for i := range message.Fields().Len() {
		member := message.Fields().Get(i)

		kind, err := pyItemFieldKind(tool, entry, member)
		if err != nil {
			return nil, err
		}

		// Required is the inverse of presence, the same rule that picks put_
		// over set_ for a scalar. A map or a list carries no proto3 optional, so
		// its declaration reads as required whatever the caller may omit.
		var required string
		if !member.HasPresence() && !pyRepeated(member) {
			required = ", required=True"
		}

		nested := pyItemMemberMessage(member)
		if nested == nil {
			lines = append(lines, indent+"ItemField("+pyQuote(string(member.Name()))+", "+kind+required+"),")

			continue
		}

		lines = append(lines, indent+"ItemField("+pyQuote(string(member.Name()))+", "+kind+", fields=(")

		inner, err := pyItemFields(tool, entry, nested, indent+"    ", seen)
		if err != nil {
			return nil, err
		}

		lines = append(lines, inner...)
		lines = append(lines, indent+")),")
	}

	return lines, nil
}

// pyItemMemberMessage is the named message one member of a typed message
// carries, else nil. It is what lets a nested member be read under the members
// its own message declares rather than refused.
func pyItemMemberMessage(member protoreflect.FieldDescriptor) protoreflect.MessageDescriptor {
	message := pyMessageOf(member)
	if message == nil || message.IsMapEntry() || message.FullName() == structFullName {
		return nil
	}

	return message
}

// pyItemFieldKind is the reader kind one item member is held to. A kind with no
// reader fails the run rather than reaching the wire unchecked.
func pyItemFieldKind(
	tool string, entry, member protoreflect.FieldDescriptor,
) (string, error) {
	// Both shapes below carry the repeated label, a map's as much as a list's,
	// so they are read ahead of the refusal that catches every other repeating
	// member.
	if pyIsObjectMap(member) {
		return "ITEM_OBJECT", nil
	}

	if pyRepeated(member) && member.Kind() == protoreflect.StringKind {
		return "ITEM_STR_LIST", nil
	}

	// Before the repeat refusal for the same reason: a named message is the one
	// repeating shape whose members the reader can be told about.
	if pyItemMemberMessage(member) != nil {
		if pyRepeated(member) {
			return "ITEM_MESSAGE_LIST", nil
		}

		return "ITEM_MESSAGE", nil
	}

	if pyRepeated(member) {
		return "", fmt.Errorf("%w: %s body field %s item member %s repeats",
			errPyRender, tool, entry.Name(), member.Name())
	}

	// An enum reaches the wire as the name it declares, so an item member reads
	// it as a string, the same way a scalar enum field does.
	if member.Kind() == protoreflect.StringKind || member.Kind() == protoreflect.EnumKind {
		return pyItemStr, nil
	}

	if member.Kind() == protoreflect.Int32Kind || member.Kind() == protoreflect.Int64Kind {
		return pyItemInt, nil
	}

	if member.Kind() == protoreflect.BoolKind {
		return pyItemBool, nil
	}

	return "", fmt.Errorf("%w: %s body field %s item member %s has no request representation",
		errPyRender, tool, entry.Name(), member.Name())
}

// pyObjectSetter is the builder call one free-form object field is written
// through. All three object shapes are the same proto declaration, so only the
// contract separates them: the plain setter reads a null as absence, which is
// the state a detach cannot afford to lose, and the root setter hoists the
// members onto a body that has no member for them.
func pyObjectSetter(entry protoreflect.FieldDescriptor) string {
	if boolOption(entry.Options(), linodev1.E_BodyRoot) {
		return "set_object_root"
	}

	if boolOption(entry.Options(), linodev1.E_BodyNullable) {
		return "set_object_or_null"
	}

	return "set_object"
}

// pyBodySetter is the builder call one field is written through. A kind with no
// call fails the run by name: a field quietly left out of the body would be a
// tool that advertises an argument and drops it.
func pyBodySetter(tool string, entry protoreflect.FieldDescriptor) (string, error) {
	// Checked before the repeated branch because a map's label is repeated, and
	// before the type switch because its type is the map entry's message.
	if pyIsObjectMap(entry) {
		return pyObjectSetter(entry), nil
	}

	// Same map label as the free-form shape above, so it is read before the
	// repeated branch for the same reason.
	if pyIsStrMap(entry) {
		return "set_str_map", nil
	}

	if pyRepeated(entry) {
		return pyRepeatedSetter(tool, entry)
	}

	// After the map branches because a map's type is its entry's message: this
	// is the named message a body carries as one object rather than a table.
	if pyBodyMessage(entry) != nil {
		if boolOption(entry.Options(), linodev1.E_BodyNullable) {
			return "set_message_or_null", nil
		}

		return "set_message", nil
	}

	return pyScalarSetter(tool, entry)
}

// pyScalarSetter is the builder call one non-repeating field is written
// through. The verb is the presence rule: a proto3 optional travels only when
// the caller supplied it, a bare scalar travels on every call.
func pyScalarSetter(tool string, entry protoreflect.FieldDescriptor) (string, error) {
	// Ahead of the string branch it would otherwise take: the argument is text
	// and the member is an array, so only the contract separates the two.
	if boolOption(entry.Options(), linodev1.E_BodyCommaList) {
		return "set_comma_str_list", nil
	}

	verb := "put"
	if entry.HasPresence() {
		verb = "set"
	}

	if entry.Kind() == protoreflect.StringKind || entry.Kind() == protoreflect.EnumKind {
		// A null is the API's clear-this-field spelling, which only a string
		// carries: an enum's null names no member of its set.
		if entry.Kind() == protoreflect.StringKind && boolOption(entry.Options(), linodev1.E_BodyNullable) {
			return "set_str_or_null", nil
		}

		return verb + "_str", nil
	}

	if entry.Kind() == protoreflect.Int32Kind || entry.Kind() == protoreflect.Int64Kind {
		return verb + "_int", nil
	}

	if entry.Kind() == protoreflect.BoolKind {
		return verb + "_bool", nil
	}

	return "", fmt.Errorf("%w: %s body field %s has no request representation",
		errPyRender, tool, entry.Name())
}

// pyRepeatedSetter is the builder call a list field is written through.
func pyRepeatedSetter(tool string, entry protoreflect.FieldDescriptor) (string, error) {
	if string(entry.Name()) == tagsField {
		return "set_tags", nil
	}

	// Checked before the type switch because a list of free-form objects is a
	// list of messages, which no scalar branch can name.
	if pyIsObjectList(entry) {
		return "set_object_list", nil
	}

	// After the free-form branch because both are repeated messages: the named
	// one is the only shape whose members the reader can be told about.
	if pyItemMessage(entry) != nil {
		return "set_message_list", nil
	}

	if entry.Kind() == protoreflect.StringKind {
		return "set_str_list", nil
	}

	if entry.Kind() == protoreflect.Int32Kind || entry.Kind() == protoreflect.Int64Kind {
		return "set_int_list", nil
	}

	return "", fmt.Errorf("%w: %s body field %s repeats a type with no request form",
		errPyRender, tool, entry.Name())
}

// pyIsStrMap reports whether a field is a map from string to string. Exclusive
// with pyIsObjectMap: a value that is always text needs no free-form decode.
func pyIsStrMap(entry protoreflect.FieldDescriptor) bool {
	message := pyMessageOf(entry)
	if message == nil || !message.IsMapEntry() {
		return false
	}

	return message.Fields().ByName("key").Kind() == protoreflect.StringKind &&
		message.Fields().ByName("value").Kind() == protoreflect.StringKind
}

// pyIsObjectMap reports whether a field is a map from string to
// google.protobuf.Value. Any other map is refused by the caller rather than
// guessed at, since no other map kind has a JSON form both languages agree on.
func pyIsObjectMap(entry protoreflect.FieldDescriptor) bool {
	message := pyMessageOf(entry)
	if message == nil || !message.IsMapEntry() {
		return false
	}

	value := message.Fields().ByName("value")
	if pyMessageOf(value) == nil {
		return false
	}

	return message.Fields().ByName("key").Kind() == protoreflect.StringKind &&
		value.Message().FullName() == structValueName
}

// pyIsObjectList reports whether a field is a repeated google.protobuf.Struct.
// Any other repeated message is refused by the caller rather than guessed at.
func pyIsObjectList(entry protoreflect.FieldDescriptor) bool {
	message := pyMessageOf(entry)
	if message == nil || message.IsMapEntry() {
		return false
	}

	return pyRepeated(entry) && message.FullName() == structFullName
}

// pyItemMessage is the named message a repeated field carries an array of, else
// nil. Deliberately exclusive with pyIsObjectList: a repeated
// google.protobuf.Struct declares no members, so only a named item message
// gives the body reader a shape to hold each entry to.
func pyItemMessage(entry protoreflect.FieldDescriptor) protoreflect.MessageDescriptor {
	message := pyMessageOf(entry)
	if message == nil || message.IsMapEntry() || !pyRepeated(entry) {
		return nil
	}

	if message.FullName() == structFullName {
		return nil
	}

	return message
}

// pyBodyMessage is the named message a singular field carries, else nil.
func pyBodyMessage(entry protoreflect.FieldDescriptor) protoreflect.MessageDescriptor {
	message := pyMessageOf(entry)
	if message == nil || message.IsMapEntry() || pyRepeated(entry) {
		return nil
	}

	if message.FullName() == structFullName {
		return nil
	}

	return message
}

// pyTypedMessage is the named message one BODY field is read under, whichever
// cardinality declares it, and nil for a field read as a value.
func pyTypedMessage(entry protoreflect.FieldDescriptor) protoreflect.MessageDescriptor {
	if message := pyItemMessage(entry); message != nil {
		return message
	}

	return pyBodyMessage(entry)
}

// pyBodyImports is what a module takes from the body builder, in import order.
// A typed list setter is handed its item spec inline, so a module holding one
// also names the spec type and every kind its items carry. Sorted the way ruff
// sorts a from-import, which it will not rewrite for the renderer.
func pyBodyImports(tools []*pyTool) ([]string, error) {
	names := map[string]bool{"WriteBody": true}

	for _, tool := range tools {
		if err := pyItemImports(tool, names); err != nil {
			return nil, err
		}
	}

	found := make([]string, 0, len(names))
	for name := range names {
		found = append(found, name)
	}

	// Constants sort ahead of the rest, which is the member order the
	// linter's import sort settles a from-import into.
	sort.Slice(found, func(left, right int) bool {
		leftConstant := found[left] == strings.ToUpper(found[left])
		rightConstant := found[right] == strings.ToUpper(found[right])

		if leftConstant != rightConstant {
			return leftConstant
		}

		return strings.ToLower(found[left]) < strings.ToLower(found[right])
	})

	return found, nil
}

// pyItemImports collects the item-spec names one tool's typed message fields
// are written with.
func pyItemImports(tool *pyTool, names map[string]bool) error {
	for _, entry := range tool.bodyFields() {
		message := pyTypedMessage(entry)
		if message == nil {
			continue
		}

		if _, assembled := tool.c.Folds[string(entry.Name())]; assembled {
			continue
		}

		names["ItemField"] = true

		if err := pyMemberKinds(tool.c.Name, entry, message, names); err != nil {
			return err
		}
	}

	for _, members := range tool.c.Folds {
		names["FoldMember"] = true

		for _, member := range members {
			kind, known := pyItemKind(member.Kind)
			if !known {
				return fmt.Errorf("%w: %s folds a member as %s, which has no Python reader",
					errPyRender, tool.c.Name, member.Kind)
			}

			names[kind] = true
		}
	}

	return nil
}

// pyMemberKinds records every kind name one message's members are written with,
// nesting included.
func pyMemberKinds(
	tool string, entry protoreflect.FieldDescriptor,
	message protoreflect.MessageDescriptor, names map[string]bool,
) error {
	for i := range message.Fields().Len() {
		member := message.Fields().Get(i)

		kind, err := pyItemFieldKind(tool, entry, member)
		if err != nil {
			return err
		}

		names[kind] = true

		if nested := pyItemMemberMessage(member); nested != nil {
			if err := pyMemberKinds(tool, entry, nested, names); err != nil {
				return err
			}
		}
	}

	return nil
}
