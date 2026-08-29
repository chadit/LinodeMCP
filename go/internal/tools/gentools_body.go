package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// redactedKey is the single member a stood-in body value carries, so a reader
// can tell a withheld value from an empty one.
const redactedKey = "redacted"

// WriteBody is a mutating call's JSON request body under construction, held in
// the order its fields are declared in the proto rather than in whatever order
// a map iterates. Go sorts map keys when it marshals and Python keeps insertion
// order, so a body assembled as a map would put different bytes on the wire in
// each language for the same call.
//
// Presence, not truthiness, decides what a Set writes: a field the caller
// supplied travels even when its value is the type's zero, because "" and 0 are
// values the Linode API accepts and act on. A Put writes unconditionally, which
// is what a proto field without `optional` declares.
//
// The first type failure is kept and every later field is skipped, so the
// message a caller sees is the first thing wrong with the call rather than the
// last. Read it with Message before sending anything.
type WriteBody struct {
	arguments map[string]any
	renames   map[string]string
	message   string
	keys      []string
	values    []any
}

// NewWriteBody starts a body over one tool call's arguments. size is the number
// of body fields the tool declares.
func NewWriteBody(request *mcp.CallToolRequest, size int) *WriteBody {
	return &WriteBody{
		arguments: request.GetArguments(),
		keys:      make([]string, 0, size),
		values:    make([]any, 0, size),
	}
}

// Rename declares the wire key one argument is written under, for a field the
// API names differently than the tool does: linode_account_user_update takes
// new_username so the argument cannot collide with the username in its path,
// and the API reads that rename as "username".
func (b *WriteBody) Rename(argument, key string) {
	if b.renames == nil {
		b.renames = make(map[string]string, 1)
	}

	b.renames[argument] = key
}

// Message is the first type failure the body hit, or "" when every field the
// caller supplied had the type the contract declares.
func (b *WriteBody) Message() string {
	return b.message
}

// PutString writes a string field whose proto declaration has no `optional`, so
// it travels on every call.
func (b *WriteBody) PutString(name string) {
	b.always(name, "", func(raw any) (any, string) {
		value, ok := raw.(string)
		if !ok {
			return nil, name + " must be a string"
		}

		return value, ""
	})
}

// PutInt writes an integer field whose proto declaration has no `optional`.
func (b *WriteBody) PutInt(name string) {
	b.always(name, 0, func(raw any) (any, string) {
		return bodyIntValue(name, raw)
	})
}

// SetObject writes a free-form object field when the caller supplied one. A map
// field cannot carry proto3 `optional`, so presence is read from the arguments
// rather than the declaration: an omitted argument leaves the member off the
// wire entirely, where writing {} would tell the API the caller sent an empty
// object. An explicitly empty {} still travels.
func (b *WriteBody) SetObject(name string) {
	b.set(name, func(raw any) (any, string) {
		value, ok := raw.(map[string]any)
		if !ok {
			return nil, name + " must be an object"
		}

		return value, ""
	})
}

// SetObjectRoot writes a free-form object argument's own members as the whole
// request body, for the routes that read the object at the root: the profile
// preferences PUT takes {"theme":"dark"}, not {"preferences":{"theme":"dark"}}.
//
// An empty object is refused rather than sent. The members are the body, so an
// empty one is an empty body, which is the one thing a route reading the root
// cannot be asked to do.
//
// The members travel in sorted order because they have none of their own: the
// contract declares the field, not what a caller puts in it, and sorting is the
// one order both languages can reach without inventing one.
func (b *WriteBody) SetObjectRoot(name string) {
	if b.message != "" {
		return
	}

	value, ok := b.arguments[name].(map[string]any)
	if !ok || len(value) == 0 {
		b.message = name + " must be a non-empty object"

		return
	}

	for _, key := range slices.Sorted(maps.Keys(value)) {
		b.put(key, value[key])
	}
}

// SetObjectOrNull writes a free-form object field that also accepts an explicit
// JSON null, keeping the three states the API reads apart: an omitted argument
// leaves the member off the wire, an explicit null travels as null, and an
// object travels as itself. The Linode database update routes read
// `"private_network": null` as "detach from the VPC", so a null collapsed into
// absence would report a detach nothing performed.
func (b *WriteBody) SetObjectOrNull(name string) {
	b.set(name, func(raw any) (any, string) {
		if raw == nil {
			return nil, ""
		}

		value, ok := raw.(map[string]any)
		if !ok {
			return nil, name + " must be an object or null"
		}

		return value, ""
	})
}

// SetStringMap writes a string-to-string map field when the caller supplied
// one, reading presence from the arguments the way every map field does. The
// two sentences it answers are separate because the caller who sent an array
// and the caller who sent {"a": 1} have different things to fix.
func (b *WriteBody) SetStringMap(name string) {
	b.set(name, func(raw any) (any, string) {
		return bodyStringMap(name, raw)
	})
}

// SetObjectList writes a repeated free-form object field when the caller
// supplied one. An empty list is a value: it clears the field rather than
// leaving it alone, the same way an empty string list does.
func (b *WriteBody) SetObjectList(name string) {
	b.set(name, func(raw any) (any, string) {
		return bodyObjectList(name, raw)
	})
}

// ItemKind names the reader one member of a typed message is held to. Only the
// kinds the item messages declare exist, so another would be a guess rather
// than a contract.
type ItemKind int

const (
	// ItemString reads a string member.
	ItemString ItemKind = iota
	// ItemInt reads an integer member.
	ItemInt
	// ItemObject reads a free-form object member, the shape an item carries
	// when its inner members vary by kind (a node pool's autoscaler).
	ItemObject
	// ItemStringList reads a repeated string member (a node pool's tags).
	ItemStringList
	// ItemBool reads a boolean member.
	ItemBool
	// ItemMessage reads a member carrying a named message of its own, held to
	// the members that message declares (an interface's vpc ipv4 block).
	ItemMessage
	// ItemMessageList reads a member repeating a named message, each entry
	// held to the same members (a vpc ipv4 block's addresses).
	ItemMessageList
)

// ItemField is one member of a typed message, carried in the order the proto
// declares it. Required is the inverse of proto3 `optional`, the same presence
// rule the scalar setters read.
type ItemField struct {
	Name string
	// Fields is the shape a nested message is read under, empty for a kind
	// that reads a value instead.
	Fields   []ItemField
	Kind     ItemKind
	Required bool
}

// SetMessage writes a singular named-message field when the caller supplied
// one, holding it to the members its message declares. A message field carries
// no proto3 `optional`, so presence is read from the arguments the way every
// object shape reads it.
func (b *WriteBody) SetMessage(name string, fields []ItemField) {
	b.set(name, func(raw any) (any, string) {
		return messageValue(name, name+" must be "+itemKindProse(ItemMessage), fields, raw)
	})
}

// SetMessageOrNull writes a singular named-message field that also accepts an
// explicit JSON null, keeping the three states the API reads apart the way
// SetObjectOrNull does for the free-form shape. A Linode interface update reads
// `"vpc": null` as "detach this arm", so a null collapsed into absence would
// report a detach nothing performed.
func (b *WriteBody) SetMessageOrNull(name string, fields []ItemField) {
	b.set(name, func(raw any) (any, string) {
		if raw == nil {
			return nil, ""
		}

		return messageValue(name, name+" must be an object or null", fields, raw)
	})
}

// SetMessageList writes a repeated named-message field when the caller supplied
// one, holding each item to the members its message declares. An unknown member
// is refused rather than dropped, which is what the generated item schema
// already advertises with additionalProperties:false; dropping it would send a
// call the caller never asked for.
func (b *WriteBody) SetMessageList(name string, fields []ItemField) {
	b.set(name, func(raw any) (any, string) {
		return messageListValue(name, name+" must be "+objectListProse, fields, raw)
	})
}

// FoldMember is one member of an assembled body object: a flat tool argument
// that travels inside it, or a value the contract fixes.
type FoldMember struct {
	// Value is what the member travels as when no argument fills it: a fixed
	// member's whole value, or a folded argument's default. nil leaves the key
	// out, which is what an argument with no declared default means.
	Value any
	// Key is the dotted path the value lands at inside the member.
	Key string
	// Argument is the flat tool argument the value comes from, "" for a member
	// the contract fixes.
	Argument string
	// Kind is how a supplied argument is read.
	Kind ItemKind
}

// Fold writes a body member the tool always sends, assembled from the flat
// arguments the caller sends beside it. A firewall create takes inbound_policy
// and outbound_policy and the API reads both inside `rules`, so the two
// arguments travel one level down or the API never sees them.
//
// A key the caller wrote themselves is left alone and the rest fold in beside
// it, which is what keeps the flat arguments optional: a caller who spells the
// whole object out is not overruled by a default they never asked for.
func (b *WriteBody) Fold(name string, members []FoldMember) {
	if b.message != "" {
		return
	}

	supplied, message := b.foldBase(name, name+" must be an object")
	if message != "" {
		b.message = message

		return
	}

	folded, message := b.foldMembers(name, supplied, members)
	if message != "" {
		b.message = message

		return
	}

	b.put(name, folded)
}

// FoldOptional is Fold for a member the API reads as a replacement rather than
// a setting. A firewall update that mentions no policy leaves `rules` off the
// wire, because the empty object would replace the ruleset it never named.
func (b *WriteBody) FoldOptional(name string, members []FoldMember) {
	if b.message != "" {
		return
	}

	supplied, message := b.foldBase(name, name+" must be an object")
	if message != "" {
		b.message = message

		return
	}

	folded, message := b.foldMembers(name, supplied, members)
	if message != "" {
		b.message = message

		return
	}

	if len(folded) == 0 {
		return
	}

	b.put(name, folded)
}

// FoldList is Fold for a member the API reads as an array: an instance create
// is given one interface, built from the firewall id and the route flags the
// caller sent flat.
//
// A caller who supplies the list gets it sent verbatim. Synthesizing an element
// beside theirs would attach an interface they never asked for, and merging into
// the first would rewrite one they described in full.
func (b *WriteBody) FoldList(name string, members []FoldMember) {
	if b.message != "" {
		return
	}

	if raw, present := b.arguments[name]; present {
		items, ok := objectListValue(raw)
		if !ok {
			b.message = name + " must be " + objectListProse

			return
		}

		b.put(name, items)

		return
	}

	folded, message := b.foldMembers(name, map[string]any{}, members)
	if message != "" {
		b.message = message

		return
	}

	b.put(name, []map[string]any{folded})
}

// setFoldKey writes one value at a dotted path, leaving a key the caller
// already wrote alone. Each object it descends through is copied first, so a
// nested member the caller supplied is read rather than written over.
func setFoldKey(object map[string]any, path []string, value any) {
	head := path[0]

	if len(path) == 1 {
		if _, supplied := object[head]; !supplied {
			object[head] = value
		}

		return
	}

	nested := map[string]any{}
	if existing, ok := object[head].(map[string]any); ok {
		nested = maps.Clone(existing)
	}

	setFoldKey(nested, path[1:], value)

	object[head] = nested
}

// Constant writes a body member the tool always sends and takes no argument
// for. Every instance this server creates is on the current Linode Interfaces
// generation, so the create says so on every call and no caller chooses it.
func (b *WriteBody) Constant(name string, value any) {
	if b.message != "" {
		return
	}

	b.put(name, value)
}

// Default writes a body member the tool fills in when the caller named none,
// for a value the request has to carry even though the caller may omit it. The
// Object Storage upload presigns with an explicit Content-Type because the
// signature covers that header: leaving it out of the presign body and then
// sending one on the PUT signs one request and makes another.
//
// Unlike Constant it checks first, so a caller-supplied value wins. A hook
// calls it on the body the generated builder already assembled, which is why
// presence is read off the built keys rather than off the arguments: an
// argument the contract does not declare as a body member never reached here.
func (b *WriteBody) Default(name string, value any) {
	if b.message != "" || slices.Contains(b.keys, name) {
		return
	}

	b.put(name, value)
}

// SetString writes an optional string field when the caller supplied one.
func (b *WriteBody) SetString(name string) {
	b.set(name, func(raw any) (any, string) {
		value, ok := raw.(string)
		if !ok {
			return nil, name + " must be a string"
		}

		return value, ""
	})
}

// SetStringOrNull writes a string field that also accepts an explicit JSON
// null, keeping the three states the API reads apart: an omitted argument
// leaves the member off the wire, an explicit null travels as null, and a
// string travels as itself. The managed service update reads `"notes": null` as
// "erase the notes", so a null collapsed into absence would report a clear
// nothing performed.
func (b *WriteBody) SetStringOrNull(name string) {
	b.set(name, func(raw any) (any, string) {
		if raw == nil {
			return nil, ""
		}

		value, ok := raw.(string)
		if !ok {
			return nil, name + " must be a string or null"
		}

		return value, ""
	})
}

// SetInt writes an optional integer field when the caller supplied one. An
// explicit null reads as "use the default": the key is omitted rather than
// sent as 0, which is never an id the caller meant. Mirrors Python's set_int.
func (b *WriteBody) SetInt(name string) {
	if raw, present := b.arguments[name]; present && raw == nil {
		return
	}

	b.set(name, func(raw any) (any, string) {
		return bodyIntValue(name, raw)
	})
}

// PutBool writes a boolean field whose proto declaration has no `optional`.
func (b *WriteBody) PutBool(name string) {
	b.always(name, false, func(raw any) (any, string) {
		value, ok := raw.(bool)
		if !ok {
			return nil, name + " must be a boolean"
		}

		return value, ""
	})
}

// SetBool writes an optional boolean field when the caller supplied one.
func (b *WriteBody) SetBool(name string) {
	b.set(name, func(raw any) (any, string) {
		value, ok := raw.(bool)
		if !ok {
			return nil, name + " must be a boolean"
		}

		return value, ""
	})
}

// SetStringList writes a repeated string field when the caller supplied one. An
// empty list is a value: it clears the field rather than leaving it alone.
func (b *WriteBody) SetStringList(name string) {
	b.set(name, func(raw any) (any, string) {
		return bodyStringList(name, raw)
	})
}

// SetCommaStringList writes a string argument as the array of its
// comma-separated segments, each trimmed, with the blank ones dropped. A caller
// pastes one line of authorized keys and the API reads an array, so the split is
// the whole of the difference between the two.
//
// An argument whose segments are all blank travels as an empty array rather than
// leaving the member off, the same way an empty string list does: presence is
// what decides whether the member travels, and the caller supplied one.
func (b *WriteBody) SetCommaStringList(name string) {
	b.set(name, func(raw any) (any, string) {
		value, ok := raw.(string)
		if !ok {
			return nil, name + " must be a string"
		}

		return commaSeparatedEntries(value), ""
	})
}

// commaSeparatedEntries splits one line of text into the entries a body array
// carries. Kept beside the setter rather than shared with a handler so the
// generated shape has one reader whichever family declares it.
func commaSeparatedEntries(value string) []string {
	entries := []string{}

	for segment := range strings.SplitSeq(value, ",") {
		if trimmed := strings.TrimSpace(segment); trimmed != "" {
			entries = append(entries, trimmed)
		}
	}

	return entries
}

// SetIntList writes a repeated integer field when the caller supplied one. An
// empty list is a value, the same way an empty string list is.
func (b *WriteBody) SetIntList(name string) {
	b.set(name, func(raw any) (any, string) {
		return bodyIntList(name, raw)
	})
}

// SetTags writes the `tags` field when the caller supplied one. Tags have one
// reader across the whole surface: entries are trimmed, a blank one is refused,
// and a JSON-encoded array arrives as readily as a native one, because a client
// that cannot send an array still has to reach the same request body.
func (b *WriteBody) SetTags(name string) {
	b.set(name, func(raw any) (any, string) {
		return tagsValueFromToolArg(raw)
	})
}

// Redacting answers the body a dry run reports: the same fields in the same
// order, with each named member replaced by the redaction marker. A preview
// that echoed a card number back would widen where it lands, which the live
// request never needed.
func (b *WriteBody) Redacting(names ...string) *WriteBody {
	reported := b.reportedCopy()

	for index, key := range reported.keys {
		if slices.Contains(names, key) {
			reported.values[index] = map[string]any{redactedKey: true}
		}
	}

	return reported
}

// StandingIn answers the body a dry run reports for one member of a repeated
// argument's entries: the same entries in the same order, with that member
// carrying fixed text. Redacting cannot say it, since standing in for the whole
// member takes the rest of each entry with it, and the security answers are
// declared this way because the question ids beside them are what a caller
// checks before confirming.
func (b *WriteBody) StandingIn(name, member, text string) *WriteBody {
	reported := b.reportedCopy()

	index := slices.Index(reported.keys, name)
	if index < 0 {
		return reported
	}

	// A member carrying anything but entries is left as the body built it.
	entries, listed := reported.values[index].([]orderedObject)
	if !listed {
		return reported
	}

	stood := make([]orderedObject, 0, len(entries))
	for _, entry := range entries {
		stood = append(stood, entry.standingIn(member, text))
	}

	reported.values[index] = stood

	return reported
}

// MarshalJSON writes the body in declaration order.
func (b *WriteBody) MarshalJSON() ([]byte, error) {
	return MarshalOrderedJSON(b.keys, b.values)
}

// reportedCopy is the body a dry run reports before anything stands in: the
// same fields in the same order, held apart from the live one so a change to
// the report cannot reach the request.
func (b *WriteBody) reportedCopy() *WriteBody {
	return &WriteBody{
		arguments: b.arguments,
		message:   b.message,
		keys:      slices.Clone(b.keys),
		values:    slices.Clone(b.values),
	}
}

// MarshalOrderedJSON writes a JSON object holding keys and values pairwise, in
// the order given. The object is assembled by hand because encoding/json sorts
// a map's keys, which is the ordering a request body cannot afford to lose: the
// other language keeps insertion order, so a sorted body would put different
// bytes on the wire for the same call.
func MarshalOrderedJSON(keys []string, values []any) ([]byte, error) {
	var built bytes.Buffer

	built.WriteByte('{')

	for index, key := range keys {
		if index > 0 {
			built.WriteByte(',')
		}

		// One field at a time, as an object of its own: a single entry has no
		// order to sort, so encoding/json writes the name and the value exactly
		// as it would inside a whole object, braces and all. Stripping those is
		// what leaves the pair to be placed in the order the caller gave.
		pair, err := json.Marshal(map[string]any{key: values[index]})
		if err != nil {
			return nil, fmt.Errorf("marshal body field %s: %w", key, err)
		}

		built.Write(pair[1 : len(pair)-1])
	}

	built.WriteByte('}')

	return built.Bytes(), nil
}

// foldBase is the object a fold starts from: the one the caller supplied, or an
// empty one. It is copied rather than written through, so the arguments the
// rest of the call reads are the ones the caller sent.
func (b *WriteBody) foldBase(name, failure string) (map[string]any, string) {
	raw, present := b.arguments[name]
	if !present {
		return map[string]any{}, ""
	}

	object, ok := raw.(map[string]any)
	if !ok {
		return nil, failure
	}

	return maps.Clone(object), ""
}

// foldMembers writes each declared member into the object, skipping the keys
// the caller already filled.
func (b *WriteBody) foldMembers(
	name string, base map[string]any, members []FoldMember,
) (map[string]any, string) {
	for _, member := range members {
		value, travels, message := b.foldValue(name, member)
		if message != "" {
			return nil, message
		}

		if !travels {
			continue
		}

		setFoldKey(base, strings.Split(member.Key, "."), value)
	}

	return base, ""
}

// foldValue reads one member's value: what the caller sent for its argument,
// the default the contract declares, or nothing at all.
func (b *WriteBody) foldValue(name string, member FoldMember) (any, bool, string) {
	if member.Argument == "" {
		return member.Value, true, ""
	}

	raw, present := b.arguments[member.Argument]
	if !present {
		return member.Value, member.Value != nil, ""
	}

	value, ok := itemValue(member.Kind, raw)
	if !ok {
		return nil, false, name + "." + member.Key + " must be " + itemKindProse(member.Kind)
	}

	return value, true, ""
}

// always writes one field that travels whether the caller supplied it or not,
// or records the first type failure. A wrongly typed value is refused rather
// than read as the zero: no rule can speak for a field whose value the message
// cannot hold, so this is the only place the mismatch can still be reported.
func (b *WriteBody) always(name string, absent any, convert func(raw any) (any, string)) {
	if b.message != "" {
		return
	}

	raw, present := b.arguments[name]
	if !present {
		b.put(name, absent)

		return
	}

	value, message := convert(raw)
	if message != "" {
		b.message = message

		return
	}

	b.put(name, value)
}

// set writes one optional field, or records the first type failure. Skipping
// the rest after a failure is what makes the reported message the first problem
// with the call rather than whichever field happens to be declared last.
func (b *WriteBody) set(name string, convert func(raw any) (any, string)) {
	if b.message != "" {
		return
	}

	raw, present := b.arguments[name]
	if !present {
		return
	}

	value, message := convert(raw)
	if message != "" {
		b.message = message

		return
	}

	b.put(name, value)
}

// put appends one field under its wire key, keeping declaration order.
func (b *WriteBody) put(name string, value any) {
	key, renamed := b.renames[name]
	if !renamed {
		key = name
	}

	b.keys = append(b.keys, key)
	b.values = append(b.values, value)
}

// bodyIntValue accepts the int and float64 shapes a JSON-RPC argument arrives
// as, rejecting any float that is not exactly integral, which also rejects NaN
// and the infinities.
func bodyIntValue(name string, raw any) (int, string) {
	switch value := raw.(type) {
	case int:
		return value, ""
	case float64:
		whole := int(value)
		if float64(whole) == value {
			return whole, ""
		}
	case nil:
		return 0, ""
	}

	return 0, name + " must be an integer"
}

// stringListProse names the shape a string array is refused with, shared by the
// field sentence and the item one so a list reads the same wherever it sits.
const stringListProse = "an array of strings"

// bodyStringList accepts the decoded-JSON and native shapes a string array
// arrives as.
func bodyStringList(name string, raw any) ([]string, string) {
	value, ok := stringListValue(raw)
	if !ok {
		return nil, name + " must be " + stringListProse
	}

	return value, ""
}

// stringListValue reads the two shapes a string array arrives as. Shared with
// the item reader so a list inside a typed item is held to exactly what a
// top-level one is.
func stringListValue(raw any) ([]string, bool) {
	switch value := raw.(type) {
	case []string:
		return value, true
	case []any:
		found := make([]string, 0, len(value))

		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}

			found = append(found, text)
		}

		return found, true
	}

	return nil, false
}

// bodyStringMap accepts the decoded-JSON and native shapes a string-to-string
// map arrives as. A non-text value is refused rather than rendered: what Go
// would print for it is not what the caller meant to send.
func bodyStringMap(name string, raw any) (map[string]string, string) {
	if native, ok := raw.(map[string]string); ok {
		return native, ""
	}

	entries, ok := raw.(map[string]any)
	if !ok {
		return nil, name + " must be an object"
	}

	found := make(map[string]string, len(entries))

	for key, entry := range entries {
		text, ok := entry.(string)
		if !ok {
			return nil, name + " values must be strings"
		}

		found[key] = text
	}

	return found, ""
}

// objectListProse names the shape an array of objects is refused with, shared
// by the field sentence and the nested one so a list reads the same wherever it
// sits.
const objectListProse = "an array of objects"

// bodyObjectList accepts the decoded-JSON and native shapes an array of
// free-form objects arrives as.
func bodyObjectList(name string, raw any) ([]map[string]any, string) {
	value, ok := objectListValue(raw)
	if !ok {
		return nil, name + " must be " + objectListProse
	}

	return value, ""
}

// objectListValue reads the two shapes an array of objects arrives as. An entry
// that is not an object is refused rather than skipped: a grant nobody could
// read is not one the API should be told to apply. Shared with the typed reader
// so a list of messages accepts exactly what a free-form one does.
func objectListValue(raw any) ([]map[string]any, bool) {
	if native, ok := raw.([]map[string]any); ok {
		return native, true
	}

	items, ok := raw.([]any)
	if !ok {
		return nil, false
	}

	found := make([]map[string]any, 0, len(items))

	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}

		found = append(found, object)
	}

	return found, true
}

// orderedObject is one typed list item, marshaled in the order its message
// declares its members. A plain map would marshal sorted here and in insertion
// order in Python, which is the one thing a shared request body cannot afford.
type orderedObject struct {
	keys   []string
	values []any
}

// MarshalJSON writes the item in item-field declaration order.
func (o orderedObject) MarshalJSON() ([]byte, error) {
	return MarshalOrderedJSON(o.keys, o.values)
}

// standingIn is this entry as a dry run reports it, with the named member
// carrying text instead of its value. An entry the member never reached is
// answered unchanged, which is what a member the caller left out already means.
func (o orderedObject) standingIn(member, text string) orderedObject {
	stood := orderedObject{keys: slices.Clone(o.keys), values: slices.Clone(o.values)}

	index := slices.Index(stood.keys, member)
	if index >= 0 {
		stood.values[index] = text
	}

	return stood
}

// messageValue reads one named-message value, held to the members its message
// declares. failure is the sentence a value that is not an object is refused
// with, which is the one place the nullable setter's prose differs.
func messageValue(path, failure string, fields []ItemField, raw any) (any, string) {
	object, ok := raw.(map[string]any)
	if !ok {
		return nil, failure
	}

	built, message := messageObject(path, fields, object)
	if message != "" {
		return nil, message
	}

	return built, ""
}

// messageListValue reads a repeated named-message value. The outer shape is the
// object-list reader's, so a typed list accepts exactly what a free-form one
// does; what it adds is holding each entry to the members its message declares.
func messageListValue(path, failure string, fields []ItemField, raw any) (any, string) {
	items, ok := objectListValue(raw)
	if !ok {
		return nil, failure
	}

	built := make([]orderedObject, 0, len(items))

	for index, item := range items {
		object, message := messageObject(fmt.Sprintf("%s[%d]", path, index), fields, item)
		if message != "" {
			return nil, message
		}

		built = append(built, object)
	}

	return built, ""
}

// messageObject reads one object held to the members a message declares, in the
// order it declares them. path is what the sentences below address the object
// by: the field name at the top, and the member or the indexed entry each
// nested reader reached it through underneath.
//
// The first failure answers the whole call, the way the body's first type
// failure does, so the caller hears the first thing wrong rather than the last.
func messageObject(path string, fields []ItemField, item map[string]any) (orderedObject, string) {
	built := orderedObject{
		keys:   make([]string, 0, len(fields)),
		values: make([]any, 0, len(fields)),
	}

	for _, entry := range fields {
		value, travels, message := itemMember(path, entry, item)
		if message != "" {
			return orderedObject{}, message
		}

		if !travels {
			continue
		}

		built.keys = append(built.keys, entry.Name)
		built.values = append(built.values, value)
	}

	unknown, found := unknownItemKey(fields, item)
	if !found {
		return built, ""
	}

	// Quoted by hand rather than with %q, whose escaping Python's formatter
	// does not reproduce, and the two languages report one sentence.
	return orderedObject{}, path + ` has no field "` + unknown + `"`
}

// itemMember reads one declared member of one object, answering whether it
// travels and the sentence a bad one is refused with.
func itemMember(path string, entry ItemField, item map[string]any) (any, bool, string) {
	member := path + "." + entry.Name

	raw, present := item[entry.Name]
	if !present {
		if entry.Required {
			return nil, false, member + " is required"
		}

		return nil, false, ""
	}

	value, message := memberValue(member, entry, raw)
	if message != "" {
		return nil, false, message
	}

	return value, true, ""
}

// memberValue reads one supplied member under its declared kind: a member
// carrying a message of its own reads through the same readers a field-level
// message does, and every other kind reads a value.
func memberValue(path string, entry ItemField, raw any) (any, string) {
	failure := path + " must be " + itemKindProse(entry.Kind)

	switch entry.Kind {
	case ItemMessage:
		return messageValue(path, failure, entry.Fields, raw)
	case ItemMessageList:
		return messageListValue(path, failure, entry.Fields, raw)
	case ItemString, ItemInt, ItemObject, ItemStringList, ItemBool:
	}

	value, ok := itemValue(entry.Kind, raw)
	if !ok {
		return nil, failure
	}

	return value, ""
}

// itemValue reads one item member under its declared kind. A null is refused
// where a null scalar reads as zero: a member nobody sent is not one the API
// should be told to act on.
func itemValue(kind ItemKind, raw any) (any, bool) {
	switch kind {
	case ItemInt:
		value, ok := itemIntValue(raw)

		return value, ok
	case ItemObject:
		value, ok := raw.(map[string]any)

		return value, ok
	case ItemStringList:
		return stringListValue(raw)
	case ItemBool:
		value, ok := raw.(bool)

		return value, ok
	// The two nested kinds read through memberValue before a value kind is
	// asked for.
	case ItemString, ItemMessage, ItemMessageList:
	}

	value, ok := raw.(string)

	return value, ok
}

// itemIntValue accepts the int and float64 shapes a JSON-RPC number arrives as,
// rejecting any float that is not exactly integral.
func itemIntValue(raw any) (int, bool) {
	switch value := raw.(type) {
	case int:
		return value, true
	case float64:
		whole := int(value)
		if float64(whole) == value {
			return whole, true
		}
	}

	return 0, false
}

// itemKindProse names one item kind the way its refusal sentence reads.
func itemKindProse(kind ItemKind) string {
	switch kind {
	case ItemInt:
		return "an integer"
	case ItemObject:
		return "an object"
	case ItemStringList:
		return stringListProse
	case ItemBool:
		return "a boolean"
	case ItemMessage:
		return "an object"
	case ItemMessageList:
		return objectListProse
	case ItemString:
	}

	return "a string"
}

// unknownItemKey is the first member an item carries that its message does not
// declare. Key order rather than arrival order, because a Go map iterates in
// none and the two languages have to report the same sentence.
func unknownItemKey(fields []ItemField, item map[string]any) (string, bool) {
	found := make([]string, 0, len(item))

	for key := range item {
		if !slices.ContainsFunc(fields, func(entry ItemField) bool { return entry.Name == key }) {
			found = append(found, key)
		}
	}

	if len(found) == 0 {
		return "", false
	}

	slices.Sort(found)

	return found[0], true
}

// bodyIntList accepts the decoded-JSON and native shapes an integer array
// arrives as. An entry is held to the scalar integer rule, except that a null
// entry is refused where a null scalar reads as zero: a list is a set of ids,
// and an id nobody sent is not one of them.
func bodyIntList(name string, raw any) ([]int, string) {
	failure := name + " must be an array of integers"

	if native, ok := raw.([]int); ok {
		return native, ""
	}

	items, ok := raw.([]any)
	if !ok {
		return nil, failure
	}

	found := make([]int, 0, len(items))

	for _, item := range items {
		value, message := bodyIntValue(name, item)
		if message != "" || item == nil {
			return nil, failure
		}

		found = append(found, value)
	}

	return found, ""
}
