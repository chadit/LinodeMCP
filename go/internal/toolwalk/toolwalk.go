// Package toolwalk evaluates the open-object walks a tool's input message
// declares against the arguments of one call, so the check is written once in
// the proto contract instead of once per language.
//
// An open object is a map or a repeated Struct field: the device slots on a
// config write, the boot helpers beside them, the phone numbers on a Managed
// contact, the SSH block on Managed settings, the default firewall assignments,
// the grant sections on a user's grants. Every value such a field can hold is a
// google.protobuf.Value, so the message accepts them all and a buf.validate rule
// sees a size rather than the members the API reads. That is why these were the
// last checks still written out by hand in both languages.
//
// The declaration is read from the descriptor at call time, the same way
// toolvalidate reads rules, so declaring a walk is a proto edit and the
// generated handler only names the seam.
package toolwalk

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The placeholders a declared sentence is filled from. See ObjectWalk in
// options.proto for what each one names.
const (
	fieldPlaceholder     = "{field}"
	keyPlaceholder       = "{key}"
	keysPlaceholder      = "{keys}"
	parentPlaceholder    = "{parent}"
	valuesPlaceholder    = "{values}"
	minimumPlaceholder   = "{minimum}"
	maximumPlaceholder   = "{maximum}"
	maxLengthPlaceholder = "{max_length}"
)

// scope is what a sentence at one point in a walk can name: the argument being
// walked, the key the refusal is about, and the key or index of the object the
// key sits in.
type scope struct {
	field  string
	key    string
	parent string
	keys   []string
}

// Check answers the first walk the arguments break, in the words the walk
// declares, or "" when they break none. The answer is the tool's own sentence:
// nothing here wraps or prefixes it, because a caller reads it as the reason
// their call was refused.
//
// inputMessage is the full name of the tool's input message, such as
// "linode.mcp.v1.InstanceConfigCreateInput".
func Check(inputMessage string, arguments map[string]any) string {
	descriptor, walks := declared(inputMessage)
	if len(walks) == 0 {
		return ""
	}

	for _, walk := range walks {
		if message := runWalk(descriptor, walk, arguments); message != "" {
			return message
		}
	}

	return ""
}

// declared resolves a message and the walks it declares. A message with none is
// the common case and costs one registry lookup, which is what keeps the seam
// affordable on every handler that carries it.
func declared(inputMessage string) (protoreflect.MessageDescriptor, []*linodev1.ObjectWalk) {
	found, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(inputMessage))
	if err != nil {
		return nil, nil
	}

	descriptor, isMessage := found.(protoreflect.MessageDescriptor)
	if !isMessage {
		return nil, nil
	}

	walks, _ := proto.GetExtension(descriptor.Options(), linodev1.E_ObjectWalk).([]*linodev1.ObjectWalk)

	return descriptor, walks
}

// runWalk applies one declaration to each argument it names, in declared order.
func runWalk(descriptor protoreflect.MessageDescriptor, walk *linodev1.ObjectWalk, arguments map[string]any) string {
	for _, name := range walk.GetField() {
		if message := walkArgument(walk, descriptor, name, arguments); message != "" {
			return message
		}
	}

	return ""
}

// walkArgument answers for one argument: whether the caller sent it, whether it
// has the shape the field declares, and then what is inside it.
//
// An explicit null reads as absent, which is how both hand walks read one: a
// caller who sent devices: null named no device slot.
func walkArgument(walk *linodev1.ObjectWalk, descriptor protoreflect.MessageDescriptor, name string, arguments map[string]any) string {
	where := scope{field: name}

	raw, present := arguments[name]
	if !present || raw == nil {
		return fill(walk.GetAbsent(), where)
	}

	// The field says which of the two open shapes this is. A name the message
	// does not carry never reaches here, because the emitter refuses a walk
	// that declares one.
	field := descriptor.Fields().ByName(protoreflect.Name(name))
	if field != nil && field.IsMap() {
		return walkObject(walk, where, raw)
	}

	return walkList(walk, where, raw)
}

// walkObject answers for a map argument and the members inside it.
func walkObject(walk *linodev1.ObjectWalk, where scope, raw any) string {
	object, isObject := raw.(map[string]any)
	if !isObject {
		return fill(walk.GetUnusable(), where)
	}

	if len(object) == 0 && walk.GetEmpty() != "" {
		return fill(walk.GetEmpty(), where)
	}

	return walkMembers(walk.GetMembers(), object, where)
}

// walkList answers for a repeated Struct argument, entry by entry. The index
// stands where a map walk's key does, since a list entry has no other name.
func walkList(walk *linodev1.ObjectWalk, where scope, raw any) string {
	entries, isList := raw.([]any)
	if !isList {
		return fill(walk.GetUnusable(), where)
	}

	for index, entry := range entries {
		inside := scope{field: where.field, parent: strconv.Itoa(index)}

		if message := walkEntry(walk, inside, entry); message != "" {
			return message
		}
	}

	return ""
}

// walkEntry answers for one list entry.
func walkEntry(walk *linodev1.ObjectWalk, where scope, entry any) string {
	object, isObject := entry.(map[string]any)
	if !isObject {
		return fill(walk.GetElement(), where)
	}

	return walkMembers(walk.GetMembers(), object, where)
}

// walkMembers answers for one object: the keys it carries that the vocabulary
// does not name, then each declared member, then the arms about the members
// together.
func walkMembers(members *linodev1.ObjectMembers, object map[string]any, where scope) string {
	if message := unknownKeys(members, object, where); message != "" {
		return message
	}

	for _, key := range members.GetKey() {
		if message := checkValue(members.GetValue(), object, key, where); message != "" {
			return message
		}
	}

	for _, member := range members.GetMember() {
		if message := checkValue(member.GetValue(), object, member.GetName(), where); message != "" {
			return message
		}
	}

	return requireArms(members.GetRequire(), object, where)
}

// unknownKeys answers for every key outside the vocabulary at once, so a
// sentence naming {keys} reports all of them and one naming {key} reports the
// first in sorted order.
func unknownKeys(members *linodev1.ObjectMembers, object map[string]any, where scope) string {
	sentence := members.GetUnknown()
	if sentence == "" {
		return ""
	}

	known := vocabulary(members)

	var extra []string

	for key := range object {
		if !slices.Contains(known, key) {
			extra = append(extra, key)
		}
	}

	if len(extra) == 0 {
		return ""
	}

	sort.Strings(extra)

	return fill(sentence, scope{field: where.field, parent: where.parent, key: extra[0], keys: extra})
}

// vocabulary is every key an object may carry, the shared-value keys first and
// the members with a value of their own after them.
func vocabulary(members *linodev1.ObjectMembers) []string {
	known := slices.Clone(members.GetKey())
	for _, member := range members.GetMember() {
		known = append(known, member.GetName())
	}

	return known
}

// requireArms answers for the members together: the object naming none of them,
// then naming more than one.
func requireArms(require *linodev1.ObjectRequire, object map[string]any, where scope) string {
	var named int

	for _, name := range require.GetName() {
		if _, present := object[name]; present {
			named++
		}
	}

	if named == 0 && require.GetAnyOf() != "" {
		return fill(require.GetAnyOf(), where)
	}

	if named > 1 && require.GetAtMostOne() != "" {
		return fill(require.GetAtMostOne(), where)
	}

	return ""
}

// checkValue answers for one member of an object: whether it is there, whether
// it has the declared kind, and then whether it is inside the bounds or the
// vocabulary that kind carries.
func checkValue(value *linodev1.ObjectValue, object map[string]any, key string, where scope) string {
	here := scope{field: where.field, key: key, parent: where.parent}

	raw, present := object[key]
	if !present {
		return fillValue(value.GetAbsent(), here, value)
	}

	if raw == nil {
		if value.GetNullable() {
			return ""
		}

		return fillValue(value.GetUnusable(), here, value)
	}

	return checkKind(value, raw, here)
}

// checkKind answers for a member the caller did send, under the kind it
// declares.
func checkKind(value *linodev1.ObjectValue, raw any, where scope) string {
	switch value.GetKind() {
	case linodev1.ValueKind_VALUE_KIND_INT:
		return checkInt(value, raw, where)
	case linodev1.ValueKind_VALUE_KIND_BOOL:
		if _, isBool := raw.(bool); !isBool {
			return fillValue(value.GetUnusable(), where, value)
		}
	case linodev1.ValueKind_VALUE_KIND_TEXT:
		return checkText(value, raw, where)
	case linodev1.ValueKind_VALUE_KIND_OBJECT:
		return checkObject(value, raw, where)
	case linodev1.ValueKind_VALUE_KIND_UNSPECIFIED:
		return ""
	}

	return ""
}

// checkInt answers for a whole-number member and the bounds it declares. A
// fraction is refused rather than truncated: the values these members carry are
// ids and ports, and 22.5 names neither.
func checkInt(value *linodev1.ObjectValue, raw any, where scope) string {
	number, whole := wholeNumber(raw)
	if !whole {
		return fillValue(value.GetUnusable(), where, value)
	}

	if value.GetMinimum() != 0 && number < value.GetMinimum() {
		return refusal(value, where)
	}

	if value.GetMaximum() != 0 && number > value.GetMaximum() {
		return refusal(value, where)
	}

	return ""
}

// checkText answers for a text member: the blank it may not be, the length the
// API caps it at, and the vocabulary it may have to name.
func checkText(value *linodev1.ObjectValue, raw any, where scope) string {
	text, isText := raw.(string)
	if !isText {
		return fillValue(value.GetUnusable(), where, value)
	}

	if value.GetNonBlank() && strings.TrimSpace(text) == "" {
		return refusal(value, where)
	}

	if value.GetMaxLength() != 0 && int64(utf8.RuneCountInString(text)) > value.GetMaxLength() {
		return refusal(value, where)
	}

	if names := enumNames(value.GetValues()); len(names) > 0 && !slices.Contains(names, text) {
		return refusal(value, where)
	}

	return ""
}

// checkObject answers for a member the walk reads one level further in.
func checkObject(value *linodev1.ObjectValue, raw any, where scope) string {
	object, isObject := raw.(map[string]any)
	if !isObject {
		return fillValue(value.GetUnusable(), where, value)
	}

	return walkMembers(value.GetMembers(), object, scope{field: where.field, parent: where.key})
}

// refusal is the sentence a value of the right kind and the wrong content gets,
// which is the member's own where it words one and its unusable sentence where
// one sentence covers every way it can be wrong.
func refusal(value *linodev1.ObjectValue, where scope) string {
	if value.GetRefused() != "" {
		return fillValue(value.GetRefused(), where, value)
	}

	return fillValue(value.GetUnusable(), where, value)
}

// wholeNumber reads the shapes a JSON number arrives as, refusing a bool, which
// Go's type switch would otherwise never reach and Python's int check would
// take as 1.
func wholeNumber(raw any) (int64, bool) {
	switch value := raw.(type) {
	case int:
		return int64(value), true
	case int64:
		return value, true
	case float64:
		whole := int64(value)

		return whole, float64(whole) == value
	}

	return 0, false
}

// enumNames is one declared vocabulary: the member names of the named enum,
// zero sentinel excluded, in enum-number order. Read from the descriptor so the
// vocabulary has one home even as the enum grows.
func enumNames(fullName string) []string {
	found, err := protoregistry.GlobalTypes.FindEnumByName(protoreflect.FullName(fullName))
	if err != nil {
		return nil
	}

	values := found.Descriptor().Values()
	names := make([]string, 0, values.Len())

	for index := range values.Len() {
		if values.Get(index).Number() != 0 {
			names = append(names, string(values.Get(index).Name()))
		}
	}

	return names
}

// fill writes a scope into a declared sentence. An empty sentence stays empty,
// which is how a walk says it accepts what it just looked at.
func fill(sentence string, where scope) string {
	if sentence == "" {
		return ""
	}

	filled := strings.ReplaceAll(sentence, fieldPlaceholder, where.field)
	filled = strings.ReplaceAll(filled, keysPlaceholder, strings.Join(where.keys, ", "))
	filled = strings.ReplaceAll(filled, keyPlaceholder, where.key)

	return strings.ReplaceAll(filled, parentPlaceholder, where.parent)
}

// fillValue writes a scope and the bounds a member declares into its sentence,
// so a sentence stating a bound does not carry a second copy of the number.
func fillValue(sentence string, where scope, value *linodev1.ObjectValue) string {
	if sentence == "" {
		return ""
	}

	filled := fill(sentence, where)
	filled = strings.ReplaceAll(filled, valuesPlaceholder, strings.Join(enumNames(value.GetValues()), ", "))
	filled = strings.ReplaceAll(filled, minimumPlaceholder, strconv.FormatInt(value.GetMinimum(), 10))
	filled = strings.ReplaceAll(filled, maximumPlaceholder, strconv.FormatInt(value.GetMaximum(), 10))

	return strings.ReplaceAll(filled, maxLengthPlaceholder, strconv.FormatInt(value.GetMaxLength(), 10))
}
