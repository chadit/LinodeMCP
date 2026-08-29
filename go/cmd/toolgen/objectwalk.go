package main

import (
	"fmt"
	"regexp"
	"slices"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The declaration checks for object_walk. The walker itself reads the
// declaration from the descriptor at call time, so nothing downstream would
// report a walk that names a field the message does not carry, a sentence with
// a placeholder the walk cannot fill, or a spec no key reads: the call would go
// through and the caller would be told nothing. These are what hold a
// declaration to being read.

// The placeholders a walk fills, split by where each one has a value to take.
// See ObjectWalk in options.proto for what they name.
const (
	walkFieldPlaceholder     = "{field}"
	walkKeyPlaceholder       = "{key}"
	walkKeysPlaceholder      = "{keys}"
	walkParentPlaceholder    = "{parent}"
	walkValuesPlaceholder    = "{values}"
	walkMinimumPlaceholder   = "{minimum}"
	walkMaximumPlaceholder   = "{maximum}"
	walkMaxLengthPlaceholder = "{max_length}"
)

// walkPlaceholderPattern finds what a sentence asks the walk to fill, so a
// misspelled name fails here rather than reaching a caller with its braces
// still in it.
var walkPlaceholderPattern = regexp.MustCompile(`\{[a-z_]+\}`)

// walkPlace is what a sentence at one point in a declaration may name. parented
// is false only at the top level of a map walk, which is the one place with no
// key or index above it.
type walkPlace struct {
	tool     string
	field    string
	parented bool
}

// readArgumentDeclarations resolves what a tool declares about its whole
// argument map rather than about one field: the names it refuses, the ones it
// must be given at least one of, and the walks over its open objects. They are
// read together because each of them answers for the map rather than for one
// field.
func (c *contract) readArgumentDeclarations(options protoreflect.ProtoMessage) error {
	if err := c.readRefusals(options); err != nil {
		return err
	}

	if err := c.readAnyOf(options); err != nil {
		return err
	}

	if err := c.readNormalizeFields(options); err != nil {
		return err
	}

	return c.readObjectWalks(options)
}

// readObjectWalks resolves the walks a tool declares over its open-object
// arguments, holding each to a field it can read and each sentence to
// placeholders the walk fills.
func (c *contract) readObjectWalks(options protoreflect.ProtoMessage) error {
	walks, _ := proto.GetExtension(options, linodev1.E_ObjectWalk).([]*linodev1.ObjectWalk)
	if len(walks) == 0 {
		return nil
	}

	if c.Tier != tierWrite {
		return fmt.Errorf("%w: %s", errWalkTier, c.Name)
	}

	var read []string

	for _, walk := range walks {
		names, err := c.checkWalk(walk, read)
		if err != nil {
			return err
		}

		read = append(read, names...)
	}

	c.Walks = true

	return nil
}

// checkWalk holds one declaration to the arguments it names, answering the
// names it read so a later walk naming one of them fails.
func (c *contract) checkWalk(walk *linodev1.ObjectWalk, read []string) ([]string, error) {
	names := walk.GetField()
	if len(names) == 0 {
		return nil, fmt.Errorf("%w: %s", errWalkNoField, c.Name)
	}

	if !walkSaysSomething(walk) {
		return nil, fmt.Errorf("%w: %s walks %v", errWalkSilent, c.Name, names)
	}

	for _, name := range names {
		listed, err := c.walkedField(name, read)
		if err != nil {
			return nil, err
		}

		if !listed && walk.GetElement() != "" {
			return nil, fmt.Errorf("%w: %s field %s", errWalkElementOnMap, c.Name, name)
		}

		if listed && walk.GetEmpty() != "" {
			return nil, fmt.Errorf("%w: %s field %s", errWalkEmptyOnList, c.Name, name)
		}

		place := walkPlace{tool: c.Name, field: name, parented: listed}
		if err := checkWalkSentences(walk, place); err != nil {
			return nil, err
		}

		if err := checkWalkMembers(walk.GetMembers(), place); err != nil {
			return nil, err
		}
	}

	return names, nil
}

// walkedField reports whether one walked argument is a list, and refuses a name
// the message does not carry as an open-object BODY field. Those are the two
// shapes whose members the descriptors see only the size of; a scalar or a
// typed message has its own fields, and a rule reaches those.
func (c *contract) walkedField(name string, read []string) (bool, error) {
	if slices.Contains(read, name) {
		return false, fmt.Errorf("%w: %s names %s", errWalkRepeatedField, c.Name, name)
	}

	index := slices.IndexFunc(c.Body, func(entry field) bool { return entry.ProtoName == name })
	if index < 0 {
		return false, fmt.Errorf("%w: %s names %s", errWalkUnknownField, c.Name, name)
	}

	if c.Body[index].ObjectMap {
		return false, nil
	}

	if !c.Body[index].ObjectList {
		return false, fmt.Errorf("%w: %s names %s", errWalkUnknownField, c.Name, name)
	}

	return true, nil
}

// walkSaysSomething reports whether a declaration refuses anything at all. One
// that refuses nothing is read and dropped, which is the failure this whole
// file exists to catch.
func walkSaysSomething(walk *linodev1.ObjectWalk) bool {
	if walk.GetAbsent() != "" || walk.GetUnusable() != "" || walk.GetEmpty() != "" || walk.GetElement() != "" {
		return true
	}

	return membersSaySomething(walk.GetMembers())
}

// membersSaySomething reports whether one member block refuses anything.
func membersSaySomething(members *linodev1.ObjectMembers) bool {
	if members == nil {
		return false
	}

	if members.GetUnknown() != "" || valueSaysSomething(members.GetValue()) {
		return true
	}

	if requireSaysSomething(members.GetRequire()) {
		return true
	}

	return slices.ContainsFunc(members.GetMember(), func(member *linodev1.ObjectMember) bool {
		return valueSaysSomething(member.GetValue())
	})
}

// valueSaysSomething reports whether one member spec refuses anything, its
// nested members included.
func valueSaysSomething(value *linodev1.ObjectValue) bool {
	if value == nil {
		return false
	}

	if value.GetAbsent() != "" || value.GetUnusable() != "" || value.GetRefused() != "" {
		return true
	}

	return membersSaySomething(value.GetMembers())
}

// requireSaysSomething reports whether the arms about the members together
// refuse anything.
func requireSaysSomething(require *linodev1.ObjectRequire) bool {
	return require != nil && (require.GetAnyOf() != "" || require.GetAtMostOne() != "")
}

// checkWalkSentences holds the four sentences about the argument itself to the
// placeholders they can fill. Only element sits inside a list entry, so it is
// the only one of them that may name the index.
func checkWalkSentences(walk *linodev1.ObjectWalk, place walkPlace) error {
	outer := []string{walkFieldPlaceholder}

	for _, sentence := range []string{walk.GetAbsent(), walk.GetUnusable(), walk.GetEmpty()} {
		if err := checkWalkPlaceholders(sentence, outer, place); err != nil {
			return err
		}
	}

	return checkWalkPlaceholders(walk.GetElement(), []string{walkFieldPlaceholder, walkParentPlaceholder}, place)
}

// checkWalkMembers holds one member block to a vocabulary its specs read, arms
// about keys it carries, and sentences it can fill.
func checkWalkMembers(members *linodev1.ObjectMembers, place walkPlace) error {
	if members == nil {
		return nil
	}

	known, err := walkVocabulary(members, place)
	if err != nil {
		return err
	}

	if err := checkWalkUnknown(members, known, place); err != nil {
		return err
	}

	if err := checkWalkRequire(members.GetRequire(), known, place); err != nil {
		return err
	}

	if err := checkWalkValue(members.GetValue(), place); err != nil {
		return err
	}

	for _, member := range members.GetMember() {
		if err := checkWalkValue(member.GetValue(), place); err != nil {
			return err
		}
	}

	return nil
}

// walkVocabulary is every key one block declares, refusing a key that carries
// two specs and a shared spec with no key to apply to.
func walkVocabulary(members *linodev1.ObjectMembers, place walkPlace) ([]string, error) {
	shared := members.GetKey()
	if len(shared) == 0 && members.GetValue() != nil {
		return nil, fmt.Errorf("%w: %s field %s", errWalkValueWithoutKey, place.tool, place.field)
	}

	known := slices.Clone(shared)

	for _, member := range members.GetMember() {
		if slices.Contains(known, member.GetName()) {
			return nil, fmt.Errorf("%w: %s field %s key %s",
				errWalkDuplicateKey, place.tool, place.field, member.GetName())
		}

		known = append(known, member.GetName())
	}

	return known, nil
}

// checkWalkUnknown holds the vocabulary refusal to having a vocabulary to
// measure against.
func checkWalkUnknown(members *linodev1.ObjectMembers, known []string, place walkPlace) error {
	sentence := members.GetUnknown()
	if sentence == "" {
		return nil
	}

	if len(known) == 0 {
		return fmt.Errorf("%w: %s field %s", errWalkUnknownWithoutVocabulary, place.tool, place.field)
	}

	return checkWalkPlaceholders(sentence,
		[]string{walkFieldPlaceholder, walkKeyPlaceholder, walkKeysPlaceholder, walkParentPlaceholder}, place)
}

// checkWalkRequire holds the arms about the members together to keys the
// vocabulary carries.
func checkWalkRequire(require *linodev1.ObjectRequire, known []string, place walkPlace) error {
	if require == nil {
		return nil
	}

	for _, name := range require.GetName() {
		if !slices.Contains(known, name) {
			return fmt.Errorf("%w: %s field %s requires %s",
				errWalkRequireUnknownName, place.tool, place.field, name)
		}
	}

	allowed := []string{walkFieldPlaceholder, walkParentPlaceholder}
	if err := checkWalkPlaceholders(require.GetAnyOf(), allowed, place); err != nil {
		return err
	}

	return checkWalkPlaceholders(require.GetAtMostOne(), allowed, place)
}

// checkWalkValue holds one member spec to bounds its kind reads, a vocabulary
// the contract declares, and sentences it can fill.
func checkWalkValue(value *linodev1.ObjectValue, place walkPlace) error {
	if value == nil {
		return nil
	}

	if err := checkWalkBounds(value, place); err != nil {
		return err
	}

	if err := checkWalkSpelling(value, place); err != nil {
		return err
	}

	if value.GetMembers() == nil {
		return nil
	}

	// A nested block sits under a key, so its sentences may name the key above
	// them wherever the block itself sits.
	return checkWalkMembers(value.GetMembers(), walkPlace{tool: place.tool, field: place.field, parented: true})
}

// checkWalkBounds holds each bound to the kind that reads it, since a bound the
// walker never looks at is a rule the caller is never held to.
func checkWalkBounds(value *linodev1.ObjectValue, place walkPlace) error {
	kind := value.GetKind()

	numeric := value.GetMinimum() != 0 || value.GetMaximum() != 0
	if numeric && kind != linodev1.ValueKind_VALUE_KIND_INT {
		return fmt.Errorf("%w: %s field %s declares a number bound on %s", errWalkBoundKind, place.tool, place.field, kind)
	}

	textual := value.GetMaxLength() != 0 || value.GetNonBlank() || value.GetValues() != ""
	if textual && kind != linodev1.ValueKind_VALUE_KIND_TEXT {
		return fmt.Errorf("%w: %s field %s declares a text bound on %s", errWalkBoundKind, place.tool, place.field, kind)
	}

	if value.GetMembers() != nil && kind != linodev1.ValueKind_VALUE_KIND_OBJECT {
		return fmt.Errorf("%w: %s field %s declares members on %s", errWalkMembersKind, place.tool, place.field, kind)
	}

	return walkEnumDeclared(value.GetValues(), place)
}

// walkEnumDeclared holds a declared vocabulary to an enum the contract carries,
// so a misspelled name fails here rather than leaving the value held to nothing.
func walkEnumDeclared(fullName string, place walkPlace) error {
	if fullName == "" {
		return nil
	}

	if _, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(fullName)); err != nil {
		return fmt.Errorf("%w: %s field %s names %s", errWalkUnknownEnum, place.tool, place.field, fullName)
	}

	return nil
}

// checkWalkSpelling holds a member's three sentences to what it can fill: the
// bounds it declares, and the vocabulary where it names one.
func checkWalkSpelling(value *linodev1.ObjectValue, place walkPlace) error {
	allowed := []string{walkFieldPlaceholder, walkKeyPlaceholder, walkParentPlaceholder}

	if value.GetValues() != "" {
		allowed = append(allowed, walkValuesPlaceholder)
	}

	if value.GetMinimum() != 0 {
		allowed = append(allowed, walkMinimumPlaceholder)
	}

	if value.GetMaximum() != 0 {
		allowed = append(allowed, walkMaximumPlaceholder)
	}

	if value.GetMaxLength() != 0 {
		allowed = append(allowed, walkMaxLengthPlaceholder)
	}

	for _, sentence := range []string{value.GetAbsent(), value.GetUnusable(), value.GetRefused()} {
		if err := checkWalkPlaceholders(sentence, allowed, place); err != nil {
			return err
		}
	}

	return nil
}

// checkWalkPlaceholders holds one sentence to the placeholders the walk can
// fill where it sits. A name outside the allowed set, or the index named at the
// top level of a map walk, reaches the caller with its braces still in it.
func checkWalkPlaceholders(sentence string, allowed []string, place walkPlace) error {
	if sentence == "" {
		return nil
	}

	for _, named := range walkPlaceholderPattern.FindAllString(sentence, -1) {
		if !slices.Contains(allowed, named) {
			return fmt.Errorf("%w: %s field %s says %s", errWalkPlaceholder, place.tool, place.field, named)
		}

		if named == walkParentPlaceholder && !place.parented {
			return fmt.Errorf("%w: %s field %s says %s with nothing above it",
				errWalkPlaceholder, place.tool, place.field, named)
		}
	}

	return nil
}
