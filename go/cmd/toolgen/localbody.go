package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The answer shapes a local operation fills, read off the response descriptors
// rather than spelled by hand in each engine.
//
// An operation answers a plain body, and until now each language built that
// body itself: the same member list written twice, held together by nothing.
// Here the member list is read once from the message the declaration names, and
// each language is emitted a value type carrying it plus the projection that
// turns one into the body. A member added to a message reaches both languages
// or neither.
//
// Nested members are read the same way, which is what closes the divergence one
// language spelled by hand while the other reflected over its own record.

// The type words more than one renderer spells. Held as constants because
// goconst reads the whole package, not because two spellings could drift.
const (
	typeWordAny  = "any"
	typeWordInt  = "int"
	typeWordBool = "bool"
	// typeWordStr is Python's own word for text, which three renderers spell.
	typeWordStr = "str"
)

// answerKind is what one member of a shape carries. The set is closed for the
// reason the reader set is: a member outside it would reach the generated
// surface as a value neither language could be held to.
type answerKind int

const (
	// answerString is a text member.
	answerString answerKind = iota
	// answerBool is a flag member.
	answerBool
	// answerInt is a 32-bit count, which both languages carry as their own
	// ordinary integer because that is what a length already is.
	answerInt
	// answerInt64 is a 64-bit measure.
	answerInt64
	// answerUint64 is an unsigned 64-bit counter.
	answerUint64
	// answerFree is a member the contract itself declares free-form, so the
	// value is untyped by declaration rather than by omission.
	answerFree
	// answerNested is another declared shape.
	answerNested
)

// answerMember is one member of a shape, in the message's own declaration
// order, which is the order the body is written in.
type answerMember struct {
	// name is the proto member's own name, which is the key the body carries
	// and the name fillLocalBody holds the message to.
	name string
	// shape is the nested shape's type name, set on answerNested alone.
	shape string
	kind  answerKind
	list  bool
	// mapped is whether the member is a string-keyed map rather than a single
	// value or a list of them.
	mapped bool
	// present is whether absence is something the member carries, which is what
	// lets an unset optional project as nothing rather than as a zero.
	present bool
}

// answerShape is one message the generator projects.
type answerShape struct {
	// name is what each language calls the type, taken from the message's own
	// Go spelling so nothing here invents a second naming rule.
	name string
	// full is the message it was read from, for the refusals that name it.
	full    string
	members []answerMember
	// record is whether the message declares local_record, which is what asks
	// each language for the two on-disk directions beside the projection.
	record bool
}

// nested is every shape one shape reaches directly, in member order.
func (s *answerShape) nested() []string {
	reached := make([]string, 0, len(s.members))

	for _, member := range s.members {
		if member.kind == answerNested {
			reached = append(reached, member.shape)
		}
	}

	return reached
}

// localAnswerShapes is every shape the declared operations answer with and
// every shape those reach, ordered so a nested shape is emitted before the one
// that names it.
//
// Read over the whole table rather than per tool: two tools sharing an
// operation share its shape, and a shape reached from two operations has to
// come out the same both times or the generated type would depend on which
// declaration was read first.
func localAnswerShapes(arms map[linodev1.LocalCall]localArm) ([]answerShape, error) {
	found := make(map[string]answerShape, len(arms))

	for _, call := range slices.Sorted(maps.Keys(arms)) {
		if err := readArmShapes(arms[call], found); err != nil {
			return nil, err
		}
	}

	ordered, err := orderedAnswerShapes(found)
	if err != nil {
		return nil, err
	}

	if err := checkAnswerRecords(ordered); err != nil {
		return nil, err
	}

	return ordered, checkRecordsReached(ordered)
}

// checkAnswerRecords holds a record shape to members a record direction can
// spell.
//
// A record's whole point is that its member ORDER is the message's own, so a
// record carrying another record would need that one's order held too, in a
// serializer that recursed and a reader that knew where to stop. Nothing
// declares one, so the run refuses rather than emitting a direction whose
// nested half no declaration describes.
func checkAnswerRecords(shapes []answerShape) error {
	for index := range shapes {
		shape := &shapes[index]
		if !shape.record {
			continue
		}

		for _, member := range shape.members {
			if member.kind == answerNested {
				return fmt.Errorf("%w: %s carries %s under %s",
					errLocalRecordNested, shape.full, member.shape, member.name)
			}
		}
	}

	return nil
}

// checkRecordsReached holds every message declaring local_record to the answer
// surface, so a declaration cannot sit on a message no operation reaches.
//
// The option asks each language for a serializer and a reader. A message
// outside the surface gets neither, and the declaration would read as met while
// the engine that persists the record still spelled it by hand.
func checkRecordsReached(shapes []answerShape) error {
	reached := make([]string, 0, len(shapes))

	for index := range shapes {
		if shapes[index].record {
			reached = append(reached, shapes[index].full)
		}
	}

	for _, declared := range declaredRecords() {
		if !slices.Contains(reached, declared) {
			return fmt.Errorf("%w: %s", errLocalRecordUnreached, declared)
		}
	}

	return nil
}

// declaredRecords is every message in the linked-in contract carrying
// local_record, name-sorted so one gap reports the same way on every run.
//
// The files are collected first and read afterwards, because the range holds
// the registry's read lock and reading a message's options resolves the
// extension through the same registry. Reading them inside the callback
// deadlocks, which localshape.go's own comment names as the hazard the parallel
// emitter tests reach.
func declaredRecords() []string {
	files := make([]protoreflect.FileDescriptor, 0, declaredFileBudget)

	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		files = append(files, file)

		return true
	})

	declared := make([]string, 0, 1)

	for _, file := range files {
		messages := file.Messages()
		for index := range messages.Len() {
			if messageDeclaresRecord(messages.Get(index)) {
				declared = append(declared, string(messages.Get(index).FullName()))
			}
		}
	}

	slices.Sort(declared)

	return declared
}

// declaredFileBudget is roughly how many files the linked-in contract runs to,
// which is all the collector needs to size itself once.
const declaredFileBudget = 128

// messageDeclaresRecord is whether one message carries local_record.
func messageDeclaresRecord(message protoreflect.MessageDescriptor) bool {
	options, carried := message.Options().(*descriptorpb.MessageOptions)
	if !carried {
		return false
	}

	declared, stated := proto.GetExtension(options, linodev1.E_LocalRecord).(bool)

	return stated && declared
}

// readArmShapes reads every shape one operation answers with, in the directions
// it declared.
func readArmShapes(arm localArm, found map[string]answerShape) error {
	for _, answer := range arm.declared.GetAnswers() {
		message, shaped := arm.bodies[answer.GetDirection()]
		if !shaped {
			continue
		}

		if err := readAnswerShape(message, found); err != nil {
			return err
		}
	}

	return nil
}

// readAnswerShape records one message and everything it reaches, refusing a
// shape two declarations describe differently.
func readAnswerShape(
	message protoreflect.MessageDescriptor, found map[string]answerShape,
) error {
	shape, err := buildAnswerShape(message)
	if err != nil {
		return err
	}

	if already, seen := found[shape.name]; seen {
		return sameAnswerShape(&already, &shape)
	}

	found[shape.name] = shape

	return readNestedShapes(message, found)
}

// readNestedShapes records the shapes one message's own message members reach.
func readNestedShapes(
	message protoreflect.MessageDescriptor, found map[string]answerShape,
) error {
	members := message.Fields()

	for index := range members.Len() {
		nested := answerNestedMessage(members.Get(index))
		if nested == nil {
			continue
		}

		if err := readAnswerShape(nested, found); err != nil {
			return err
		}
	}

	return nil
}

// answerNestedMessage is the message one member carries, nil where it carries
// none or carries the free-form value.
func answerNestedMessage(entry protoreflect.FieldDescriptor) protoreflect.MessageDescriptor {
	value := entry
	if entry.IsMap() {
		value = entry.MapValue()
	}

	if value.Kind() != protoreflect.MessageKind || value.Message().FullName() == answerFreeMessage {
		return nil
	}

	return value.Message()
}

// answerFreeMessage is the message the contract uses to declare a member
// free-form. Its own members describe a JSON value rather than an answer, so it
// is carried through as one instead of being projected.
const answerFreeMessage protoreflect.FullName = "google.protobuf.Value"

// sameAnswerShape refuses two messages one language would spell alike.
//
// The shapes are keyed by the name a language calls the type, so two messages
// landing on one name would emit whichever was read first and the run would not
// say which. One message read twice cannot differ: the shape is built from the
// descriptor alone, so there is nothing else here to compare.
func sameAnswerShape(already, shape *answerShape) error {
	if already.full == shape.full {
		return nil
	}

	return fmt.Errorf("%w: %s and %s are both spelled %s",
		errLocalShapeShared, already.full, shape.full, shape.name)
}

// orderedAnswerShapes lists the shapes with every nested one ahead of the shape
// naming it, name-sorted within that, so both languages emit one order and a
// language declaring types before use can read the list straight through.
func orderedAnswerShapes(found map[string]answerShape) ([]answerShape, error) {
	ordered := make([]answerShape, 0, len(found))
	placed := make(map[string]bool, len(found))

	for range len(found) {
		next := nextAnswerShapes(found, placed)
		if len(next) == 0 {
			break
		}

		for _, name := range next {
			ordered = append(ordered, found[name])
			placed[name] = true
		}
	}

	if len(ordered) != len(found) {
		return nil, fmt.Errorf("%w: %s", errLocalShapeCycle, strings.Join(unplacedShapes(found, placed), ", "))
	}

	return ordered, nil
}

// nextAnswerShapes is every unplaced shape whose own nested shapes are all
// placed, name-sorted.
func nextAnswerShapes(found map[string]answerShape, placed map[string]bool) []string {
	ready := make([]string, 0, len(found))

	for _, name := range slices.Sorted(maps.Keys(found)) {
		shape := found[name]
		if placed[name] || !answerShapeReady(&shape, placed) {
			continue
		}

		ready = append(ready, name)
	}

	return ready
}

// answerShapeReady is whether every shape one shape names is already placed.
func answerShapeReady(shape *answerShape, placed map[string]bool) bool {
	for _, name := range shape.nested() {
		if !placed[name] {
			return false
		}
	}

	return true
}

// unplacedShapes is what an ordering left behind, name-sorted, so a cycle
// refusal names every shape in it.
func unplacedShapes(found map[string]answerShape, placed map[string]bool) []string {
	left := make([]string, 0, len(found))

	for _, name := range slices.Sorted(maps.Keys(found)) {
		if !placed[name] {
			left = append(left, name)
		}
	}

	return left
}

// buildAnswerShape reads one message into the shape a language emits.
func buildAnswerShape(message protoreflect.MessageDescriptor) (answerShape, error) {
	goType, err := lookupGoMessage(message.FullName())
	if err != nil {
		return answerShape{}, err
	}

	members := message.Fields()
	shape := answerShape{
		name:    goType.TypeName,
		full:    string(message.FullName()),
		members: make([]answerMember, 0, members.Len()),
		record:  messageDeclaresRecord(message),
	}

	for index := range members.Len() {
		member, memberErr := readAnswerMember(members.Get(index))
		if memberErr != nil {
			return answerShape{}, memberErr
		}

		shape.members = append(shape.members, member)
	}

	return shape, nil
}

// readAnswerMember reads one member into what each language types it as.
func readAnswerMember(entry protoreflect.FieldDescriptor) (answerMember, error) {
	member := answerMember{
		name:    string(entry.Name()),
		list:    entry.IsList(),
		mapped:  entry.IsMap(),
		present: entry.HasOptionalKeyword(),
	}

	if entry.IsMap() && entry.MapKey().Kind() != protoreflect.StringKind {
		return answerMember{}, fmt.Errorf("%w: %s is keyed by %s",
			errLocalShapeMemberKind, entry.FullName(), entry.MapKey().Kind())
	}

	kind, shape, err := answerValueKind(entry)
	if err != nil {
		return answerMember{}, err
	}

	member.kind = kind
	member.shape = shape

	return member, nil
}

// answerValueKind is what one member's value carries, reading a map through its
// value type because the key is already held to text.
func answerValueKind(entry protoreflect.FieldDescriptor) (answerKind, string, error) {
	value := entry
	if entry.IsMap() {
		value = entry.MapValue()
	}

	if value.Kind() == protoreflect.MessageKind {
		return answerMessageKind(entry, value)
	}

	kind, carried := answerScalarKinds()[value.Kind()]
	if !carried {
		return answerString, "", fmt.Errorf("%w: %s carries %s",
			errLocalShapeMemberKind, entry.FullName(), value.Kind())
	}

	return kind, "", nil
}

// answerScalarKinds is every proto kind a shape member may carry directly.
// Closed on purpose: a width outside it would have to be narrowed or widened by
// each language on its own, which is a decision no declaration made.
func answerScalarKinds() map[protoreflect.Kind]answerKind {
	return map[protoreflect.Kind]answerKind{
		protoreflect.StringKind: answerString,
		protoreflect.BoolKind:   answerBool,
		protoreflect.Int32Kind:  answerInt,
		protoreflect.Int64Kind:  answerInt64,
		protoreflect.Uint64Kind: answerUint64,
	}
}

// answerMessageKind is what a message-typed member carries: the free-form value
// the contract declares as one, or another shape by name.
func answerMessageKind(
	entry, value protoreflect.FieldDescriptor,
) (answerKind, string, error) {
	if value.Message().FullName() == answerFreeMessage {
		return answerFree, "", nil
	}

	nested, err := lookupGoMessage(value.Message().FullName())
	if err != nil {
		return answerString, "", fmt.Errorf("%w: %s carries %s: %w",
			errLocalShapeMemberKind, entry.FullName(), value.Message().FullName(), err)
	}

	return answerNested, nested.TypeName, nil
}
