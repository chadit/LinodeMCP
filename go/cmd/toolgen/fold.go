package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// foldMember is one member of an assembled body object, resolved down to the
// reader kind and the Go literal the emitted spec carries.
type foldMember struct {
	// Key is the dotted path the value lands at inside the member.
	Key string
	// Argument is the flat tool argument the value comes from, "" for a member
	// the contract fixes.
	Argument string
	// Literal is the emitted Go source for the value a member with no supplied
	// argument travels as, "" when the key is left out instead.
	Literal string
	// Kind is the tools.ItemKind constant the reader is held to.
	Kind string
}

// The flag spellings a declared default carries, which is what a folded member
// with no supplied argument travels as.
const (
	trueText  = "true"
	falseText = "false"
)

// bodyConstant is one top-level body member the contract fixes.
type bodyConstant struct {
	Name    string
	Literal string
}

// foldOption reads the assembly one BODY member declares, nil when it declares
// none, which is every member but the two the flat arguments fold into.
func foldOption(descriptor protoreflect.FieldDescriptor) *linodev1.BodyFold {
	fold, ok := proto.GetExtension(descriptor.Options(), linodev1.E_BodyFold).(*linodev1.BodyFold)
	if !ok || (len(fold.GetSource()) == 0 && len(fold.GetConstant()) == 0) {
		return nil
	}

	return fold
}

// deriveFolds resolves every assembled body member: which arguments fold into
// it, what each one falls back to, and which of the tool's own body fields stop
// traveling at the top level because they now travel inside one.
//
// Each refusal names the declaration rather than emitting a body that drops it,
// which is the failure this whole option exists to stop: a flat argument posted
// where the API does not read it looks like a working call and changes nothing.
func (c *contract) deriveFolds() error {
	if len(c.FoldDecls) == 0 {
		return c.readBodyConstants()
	}

	into := map[string]string{}
	c.Folds = make(map[string][]foldMember, len(c.FoldDecls))

	// Walked in body order rather than over the map, so a tool declaring two
	// folds reports the first bad one the same way on every run.
	for index := range c.Body {
		target := &c.Body[index]

		fold, declared := c.FoldDecls[target.ProtoName]
		if !declared {
			continue
		}

		members, err := c.foldSpec(target, fold, into)
		if err != nil {
			return err
		}

		c.Folds[target.ProtoName] = members
	}

	c.Folded = make(map[string]bool, len(into))
	for name := range into {
		c.Folded[name] = true
	}

	return c.readBodyConstants()
}

// foldSpec resolves one assembled member's declaration into the spec the
// emitted reader is handed.
func (c *contract) foldSpec(
	target *field, fold *linodev1.BodyFold, into map[string]string,
) ([]foldMember, error) {
	if err := c.checkFoldTarget(target); err != nil {
		return nil, err
	}

	// A list fold synthesizes its one element rather than merging, so there is
	// no empty form for the flag to leave off and it would be read and dropped.
	if fold.GetOmitWhenEmpty() && target.ItemMessage != nil {
		return nil, fmt.Errorf("%w: %s omits %s when empty, which an array member has no empty form of",
			errFoldNotPlaced, c.Name, target.ProtoName)
	}

	members := make([]foldMember, 0, len(fold.GetSource())+len(fold.GetConstant()))
	keys := make([]string, 0, cap(members))

	for _, source := range fold.GetSource() {
		member, err := c.foldSource(target, source, into)
		if err != nil {
			return nil, err
		}

		members = append(members, member)
		keys = append(keys, member.Key)
	}

	for _, constant := range fold.GetConstant() {
		member, err := c.foldConstant(target, constant)
		if err != nil {
			return nil, err
		}

		members = append(members, member)
		keys = append(keys, member.Key)
	}

	slices.Sort(keys)

	if duplicate, found := firstRepeat(keys); found {
		return nil, fmt.Errorf("%w: %s folds %s.%s twice",
			errFoldNotPlaced, c.Name, target.ProtoName, duplicate)
	}

	return members, nil
}

// checkFoldTarget holds an assembled member to the two shapes a body can carry
// one as: the free-form object a merge writes into, and the array of named
// messages a synthesized element goes into. Anything else has no reader to
// merge through, so the declaration would be read and dropped.
func (c *contract) checkFoldTarget(target *field) error {
	if target.Location != linodev1.FieldLocation_FIELD_LOCATION_BODY {
		return fmt.Errorf("%w: %s field %s is not BODY", errFoldNotPlaced, c.Name, target.ProtoName)
	}

	if target.ObjectMap || target.ItemMessage != nil {
		return nil
	}

	return fmt.Errorf("%w: %s field %s is neither a free-form object nor an array of named messages",
		errFoldNotPlaced, c.Name, target.ProtoName)
}

// foldSource resolves one flat argument into the member it fills. The argument
// has to be a BODY field of the same tool carrying a value: a repeated or
// message argument has no single value to place, and one that is itself a fold
// target would be both merged into and merged away.
func (c *contract) foldSource(
	target *field, source *linodev1.BodyFoldSource, into map[string]string,
) (foldMember, error) {
	name := source.GetArgument()

	entry, found := c.bodyField(name)
	if !found {
		return foldMember{}, fmt.Errorf("%w: %s folds %s, which it declares no BODY field for",
			errFoldNotPlaced, c.Name, name)
	}

	if entry.Repeated || entry.Message != nil || entry.ObjectMap ||
		entry.StringMap || c.FoldDecls[name] != nil {
		return foldMember{}, fmt.Errorf("%w: %s folds %s, which carries no single value",
			errFoldNotPlaced, c.Name, name)
	}

	if first, twice := into[name]; twice {
		return foldMember{}, fmt.Errorf("%w: %s folds %s into both %s and %s",
			errFoldNotPlaced, c.Name, name, first, target.ProtoName)
	}

	into[name] = target.ProtoName

	kind, ok := foldKind(entry.Kind)
	if !ok {
		return foldMember{}, fmt.Errorf("%w: %s folds %s, which is %s",
			errFoldNotPlaced, c.Name, name, entry.Kind)
	}

	literal, err := c.foldLiteral(name, kind, source.GetAbsent())
	if err != nil {
		return foldMember{}, err
	}

	return foldMember{Key: source.GetKey(), Argument: name, Literal: literal, Kind: kind}, nil
}

// foldConstant resolves one fixed member of an assembled object. Text is the
// only value a constant spells, so a member the item message declares as
// anything but a string or an enum is refused; the empty text is the empty
// object, which is what the instance create's `public` arm carries.
func (c *contract) foldConstant(target *field, constant *linodev1.BodyConstant) (foldMember, error) {
	member := memberDescriptor(target.ItemMessage, constant.GetKey())

	if constant.GetText() == "" {
		if member == nil || member.Message() == nil {
			return foldMember{}, fmt.Errorf("%w: %s fixes %s.%s with no value, and only a message member travels as the empty object",
				errFoldNotPlaced, c.Name, target.ProtoName, constant.GetKey())
		}

		return foldMember{Key: constant.GetKey(), Literal: "map[string]any{}", Kind: itemObjectKind}, nil
	}

	if member != nil && member.Kind() != protoreflect.StringKind && member.Kind() != protoreflect.EnumKind {
		return foldMember{}, fmt.Errorf("%w: %s fixes %s.%s, which is %s",
			errFoldNotPlaced, c.Name, target.ProtoName, constant.GetKey(), member.Kind())
	}

	return foldMember{
		Key: constant.GetKey(), Literal: goStringLiteral(constant.GetText()), Kind: itemStringKind,
	}, nil
}

// readBodyConstants resolves the members the tool always sends at the top of
// its body. A key naming a declared BODY field is refused: two writers for one
// member leaves it unclear which one the wire carries.
func (c *contract) readBodyConstants() error {
	for _, constant := range c.Constants {
		if constant.Literal != "" {
			continue
		}

		return fmt.Errorf("%w: %s fixes %s with no value", errFoldNotPlaced, c.Name, constant.Name)
	}

	for _, constant := range c.Constants {
		if _, declared := c.bodyField(constant.Name); declared {
			return fmt.Errorf("%w: %s fixes %s and declares a BODY field for it",
				errFoldNotPlaced, c.Name, constant.Name)
		}
	}

	return nil
}

// bodyField answers the BODY field one name refers to.
func (c *contract) bodyField(name string) (field, bool) {
	for _, entry := range c.Body {
		if entry.ProtoName == name {
			return entry, true
		}
	}

	return field{}, false
}

// foldLiteral renders the value a member travels as when the caller supplied no
// argument for it, "" for a member left out instead. A text the argument's kind
// cannot hold fails the run rather than reaching the body as something else.
func (c *contract) foldLiteral(name, kind, absent string) (string, error) {
	if absent == "" {
		return "", nil
	}

	if kind == itemBoolKind {
		if absent != trueText && absent != falseText {
			return "", fmt.Errorf("%w: %s folds %s with the default %q, which is not a flag",
				errFoldNotPlaced, c.Name, name, absent)
		}

		return absent, nil
	}

	if kind == itemIntKind {
		if _, err := strconv.Atoi(absent); err != nil {
			return "", fmt.Errorf("%w: %s folds %s with the default %q, which is not a number",
				errFoldNotPlaced, c.Name, name, absent)
		}

		return absent, nil
	}

	return goStringLiteral(absent), nil
}

// foldDefault is the value a wording reports for a folded argument the call did
// not carry: the one its own fold declares as absent, "" for an argument no
// fold defaults.
//
// The default is stated once, in the fold, and the body sends exactly this
// value; reporting anything else here would describe a request the tool does
// not make. A flag's default never reaches prose, since a wording naming a
// bool argument is refused before this is asked.
func (c *contract) foldDefault(argument string) string {
	for _, decl := range c.FoldDecls {
		for _, source := range decl.GetSource() {
			if source.GetArgument() == argument {
				return source.GetAbsent()
			}
		}
	}

	return ""
}

// foldKind names the reader one folded argument is held to, and whether its
// declared kind has one at all.
func foldKind(kind protoreflect.Kind) (string, bool) {
	if kind == protoreflect.StringKind || kind == protoreflect.EnumKind {
		return itemStringKind, true
	}

	if kind == protoreflect.Int32Kind || kind == protoreflect.Int64Kind {
		return itemIntKind, true
	}

	if kind == protoreflect.BoolKind {
		return itemBoolKind, true
	}

	return "", false
}

// memberDescriptor is the member a dotted key names inside an item message, nil
// when the message declares none or there is no message to look in.
func memberDescriptor(message protoreflect.MessageDescriptor, key string) protoreflect.FieldDescriptor {
	if message == nil {
		return nil
	}

	for segment := range strings.SplitSeq(key, ".") {
		found := message.Fields().ByName(protoreflect.Name(segment))
		if found == nil {
			return nil
		}

		if found.Message() == nil {
			return found
		}

		message = found.Message()

		if segment == lastSegment(key) {
			return found
		}
	}

	return nil
}

// lastSegment is the final part of a dotted key.
func lastSegment(key string) string {
	parts := strings.Split(key, ".")

	return parts[len(parts)-1]
}

// firstRepeat is the first value a sorted list carries twice.
func firstRepeat(sorted []string) (string, bool) {
	for index := 1; index < len(sorted); index++ {
		if sorted[index] == sorted[index-1] {
			return sorted[index], true
		}
	}

	return "", false
}

// bodyConstants reads the members a tool always sends at the top of its body,
// which no argument fills and no caller chooses.
func bodyConstants(options protoreflect.ProtoMessage) []bodyConstant {
	declared, ok := proto.GetExtension(options, linodev1.E_BodyConstant).([]*linodev1.BodyConstant)
	if !ok || len(declared) == 0 {
		return nil
	}

	found := make([]bodyConstant, 0, len(declared))
	for _, entry := range declared {
		var literal string

		if entry.GetText() != "" {
			literal = goStringLiteral(entry.GetText())
		}

		found = append(found, bodyConstant{Name: entry.GetKey(), Literal: literal})
	}

	return found
}

// emitFold writes the spec an assembled body member is built from: one entry
// per member, in the order the declaration wrote them.
//
// The list form is what the API reads as an array, and it synthesizes a single
// element rather than merging: a caller who supplied the list gets it sent
// verbatim, since an element beside theirs attaches something they never asked
// for.
func emitFold(out *source, tool *contract, entry *field) {
	call := "Fold"

	switch {
	case entry.ItemMessage != nil:
		call = "FoldList"
	case tool.FoldDecls[entry.ProtoName].GetOmitWhenEmpty():
		call = "FoldOptional"
	}

	out.writef("\tbody.%s(%s, []tools.FoldMember{", call, goStringLiteral(entry.ProtoName))

	for _, member := range tool.Folds[entry.ProtoName] {
		out.writef("\t\t{Key: %s, %sKind: tools.%s%s},",
			goStringLiteral(member.Key), foldArgument(member), member.Kind, foldValue(member))
	}

	out.writef("\t})")
}

// foldArgument renders the argument a member is filled from, nothing for a
// member the contract fixes.
func foldArgument(member foldMember) string {
	if member.Argument == "" {
		return ""
	}

	return "Argument: " + goStringLiteral(member.Argument) + ", "
}

// foldValue renders the value a member falls back to, nothing for one whose key
// is left out when the caller supplies no argument.
func foldValue(member foldMember) string {
	if member.Literal == "" {
		return ""
	}

	return ", Value: " + member.Literal
}

// The tools.ItemKind constants the emitted readers name. Written out once
// because both the typed-message walker and the fold resolver pick from them.
const (
	itemStringKind = "ItemString"
	itemIntKind    = "ItemInt"
	itemBoolKind   = "ItemBool"
	itemObjectKind = "ItemObject"
)
