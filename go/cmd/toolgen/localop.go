package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The operations whose arm the generator writes, and the subsystem surface each
// one is written against.
//
// An arm used to be four hand-written steps per language: read the declared
// inputs, call a subsystem, map its failures onto the declared conditions,
// project the answer. Those four steps are now emitted, and what the language
// writes instead is the subsystem method itself, behind a type the generator
// also emits. A wrong implementation stops the Go build at the call site and
// fails the Python conformance check, where before it was held by nothing.
//
// The migration is finished: every operation the contract declares has its arm
// written here, in every registered language. What holds it that way is the
// hand-arms gate, which scans each language's own trees and fails by name on an
// arm someone writes beside the generated ones.

// localOperation is one declared operation the generator writes the arm for.
type localOperation struct {
	arm  localArm
	call linodev1.LocalCall
}

// member is the declared member's own spelling, which every derived name
// starts from.
func (o *localOperation) member() string {
	return o.call.String()
}

// reports is the conditions the operation can report, which is its whole
// failure vocabulary. ARGUMENT_UNUSABLE is not among them: a reader answers
// that one before the operation is reached.
func (o *localOperation) reports() []linodev1.LocalGuard {
	return o.arm.declared.GetReports()
}

// localOperationSide is one direction the operation runs in and the shape it
// fills there.
//
// A generated arm fills one shape, so a two-sided operation is written as one
// arm per side rather than as one arm reading a direction. The direction a call
// runs in is what the declaring tool states, so the handler already knows it
// and the arm never has to ask.
type localOperationSide struct {
	answers   protoreflect.MessageDescriptor
	direction linodev1.LocalDirection
}

// sides is the directions the operation runs in, one for a one-sided operation
// and one per declared direction otherwise, in the vocabulary's own order.
func (o *localOperation) sides() []localOperationSide {
	directions := localMemberDirections(o.arm.declared.GetTwoSided())

	sides := make([]localOperationSide, 0, len(directions))
	for _, direction := range directions {
		sides = append(sides, localOperationSide{
			direction: direction, answers: o.arm.bodies[direction],
		})
	}

	return sides
}

// localMemberDirections is the directions a member runs in: the unset one alone
// where it runs one way, and the whole vocabulary where it runs two.
// checkLocalArmShapes holds every declaration to the same answer.
func localMemberDirections(twoSided bool) []linodev1.LocalDirection {
	if !twoSided {
		return []linodev1.LocalDirection{linodev1.LocalDirection_LOCAL_DIRECTION_UNSPECIFIED}
	}

	return []linodev1.LocalDirection{
		linodev1.LocalDirection_LOCAL_DIRECTION_ADD,
		linodev1.LocalDirection_LOCAL_DIRECTION_REMOVE,
	}
}

// localDirectionWord is the word one side closes its derived names with, empty
// on a one-sided operation. Derived from the member so a direction added to the
// vocabulary needs no second spelling.
func localDirectionWord(direction linodev1.LocalDirection) string {
	if direction == linodev1.LocalDirection_LOCAL_DIRECTION_UNSPECIFIED {
		return ""
	}

	return strings.ToLower(strings.TrimPrefix(direction.String(), localDirectionPrefix))
}

// localDirectionPrefix opens every member of the direction vocabulary.
const localDirectionPrefix = "LOCAL_DIRECTION_"

// localSideStem is the operation's own spelling for one side: the member's stem
// on a one-sided operation, and the stem closed by the direction otherwise.
func localSideStem(member string, direction linodev1.LocalDirection) (string, bool) {
	stem, derived := localArmStem(member)
	if !derived {
		return "", false
	}

	word := localDirectionWord(direction)
	if word == "" {
		return stem, true
	}

	return stem + "_" + word, true
}

// stateful is whether the subsystem is the ambient state the tool already
// fetched. An operation reading no state is served by a function the engine
// module exports instead, which is the same choice each language would make on
// its own.
func (o *localOperation) stateful() bool {
	return o.arm.declared.GetRequires() != linodev1.LocalState_LOCAL_STATE_UNSPECIFIED
}

// localAmbientParameter is one reading as one language takes it: the local it
// arrives under, the type it is carried as, where that type comes from, the
// expression the handler hands over, and where that expression comes from. All
// of them are empty where the language renders the reading in nothing.
//
// handed is separate from name because a handler does not hold every reading
// under a local of its own. A cancellation and the configuration are already
// its parameters, so it hands the local on; the clock is a seam it reads, so it
// hands the read expression instead and the subsystem still takes a value.
//
// reader and seam name where that expression comes from, and stay empty where
// the handler module already carries it.
type localAmbientParameter struct {
	name    string
	carried string
	module  string
	handed  string
	reader  string
	seam    string
}

// localAmbientRenderings is how each registered language takes each reading it
// knows, and the whole of what a generated arm hands its subsystem beyond the
// declared inputs.
//
// A reading a language has no row for is refused rather than dropped, which is
// what keeps an engine from being handed less than its declaration says. Go
// carries a cancellation as the context its handler already holds; a Python
// handler takes the arguments and the configuration and nothing else, so the
// same reading renders there in nothing.
//
// The clock is the reading no handler holds: each language reads it from its
// own seam at the call site, which is where the hand-written arms read it
// before the generator took them over.
func localAmbientRenderings() map[string]map[linodev1.LocalAmbient]localAmbientParameter {
	return map[string]map[linodev1.LocalAmbient]localAmbientParameter{
		languageGo: {
			linodev1.LocalAmbient_LOCAL_AMBIENT_CANCELLATION: {
				name: goContextLocal, carried: "context.Context", module: importContext,
				handed: goContextLocal,
			},
			linodev1.LocalAmbient_LOCAL_AMBIENT_CONFIG: {
				name: goConfigLocal, carried: "*config.Config", module: importConfig,
				handed: goConfigLocal,
			},
			linodev1.LocalAmbient_LOCAL_AMBIENT_CLOCK: {
				name: localClockLocal, carried: "time.Time", module: importTime,
				handed: "tools." + goClockReader + "(" + goContextLocal + ")",
			},
		},
		languagePython: {
			linodev1.LocalAmbient_LOCAL_AMBIENT_CANCELLATION: {},
			linodev1.LocalAmbient_LOCAL_AMBIENT_CONFIG: {
				name: goConfigLocal, carried: pyConfigType, module: pyConfigModule,
				handed: goConfigLocal,
			},
			linodev1.LocalAmbient_LOCAL_AMBIENT_CLOCK: {
				name: localClockLocal, carried: pyClockType, module: pyClockType,
				handed: pyClockReader + "()", reader: pyClockReader, seam: pyClockModule,
			},
		},
	}
}

// The Python spelling of the readings that reach its subsystems, held beside
// the table so each module and the name it carries cannot drift apart.
const (
	pyConfigModule = "linodemcp.config"
	pyConfigType   = "Config"
	pyClockModule  = "linodemcp.tools.clock"
	pyClockReader  = "now"
	// The type and its module share a spelling, which is why one constant
	// answers for both.
	pyClockType = "datetime"
)

// The clock's Go spelling, and the local both languages take it under. Each
// engine already exports the seam; the generator only says where to read it.
const (
	localClockLocal = "clock"
	goClockReader   = "ClockFromContext"
)

// localArmAmbient is the readings one arm declares, in the vocabulary's own
// order rather than the declaration's, which is the order every arm and every
// handler renders them in.
func localArmAmbient(arm *localArm) []linodev1.LocalAmbient {
	members := linodev1.LocalAmbient(0).Descriptor().Values()
	readings := make([]linodev1.LocalAmbient, 0, members.Len())

	for index := range members.Len() {
		reading := linodev1.LocalAmbient(members.Get(index).Number())
		if arm.reads(reading) {
			readings = append(readings, reading)
		}
	}

	return readings
}

// localArmAmbientParameters is the readings one arm declares as one language
// takes them, with the readings that language renders in nothing left out.
//
// Taken off the arm rather than the operation because the handler renders the
// same list for an arm the generator writes and one the engine still owns.
func localArmAmbientParameters(language string, arm *localArm) []localAmbientParameter {
	rendering := localAmbientRenderings()[language]
	parts := make([]localAmbientParameter, 0, len(arm.declared.GetAmbient()))

	for _, reading := range localArmAmbient(arm) {
		if taken := rendering[reading]; taken.name != "" {
			parts = append(parts, taken)
		}
	}

	return parts
}

// localAmbientParameters is the same list for one operation, which is what the
// arm and the subsystem type render from.
func localAmbientParameters(
	language string, operation *localOperation,
) []localAmbientParameter {
	return localArmAmbientParameters(language, &operation.arm)
}

// localSubsystemSuffix closes every subsystem type's name. Held here because
// both language arms and the naming check spell it.
const localSubsystemSuffix = "Subsystem"

// localSubsystemNaming is how a subsystem method is spelled per language: the
// operation's own stem, in that language's casing, with no prefix. The arm's
// prefix is what says "generated wrapper"; the method is the operation itself,
// so it carries none.
func localSubsystemNaming() map[string]localArmNaming {
	return map[string]localArmNaming{
		languageGo:     {language: languageGo, prefix: "", pascal: true},
		languagePython: {language: languagePython, prefix: "", pascal: false},
	}
}

// localSubsystemMethod is the method one language's subsystem serves one side
// of an operation through, or "" when no name derives. Every way that can
// happen is refused before a run renders.
func localSubsystemMethod(language, member string, direction linodev1.LocalDirection) string {
	stem, derived := localSideStem(member, direction)
	if !derived {
		return ""
	}

	naming, ruled := localSubsystemNaming()[language]
	if !ruled {
		return ""
	}

	return naming.spell(stem)
}

// localSubsystemType is what both languages call the type an operation's
// subsystem satisfies. One spelling for every language, because it names the
// operation rather than anything a language owns.
func localSubsystemType(member string) string {
	stem, derived := localArmStem(member)
	if !derived {
		return ""
	}

	return localSubsystemNaming()[languageGo].spell(stem) + localSubsystemSuffix
}

// localGuardSentinel is the name the failure vocabulary carries one condition
// under, per language: a Go error value the arm compares with errors.Is, and a
// Python exception class the arm catches. Each follows its own language's
// naming rule rather than one shared spelling, which is why the emitter derives
// both from the member instead of the contract carrying either.
func localGuardSentinel(language string, guard linodev1.LocalGuard) string {
	if language == languageGo {
		return "ErrLocal" + localGuardWords(guard)
	}

	return "Local" + localGuardWords(guard) + "Error"
}

// localGuardCause is the neutral text one condition's own value carries. It is
// derived from the member rather than taken from a tool, because the sentence a
// caller reads is declared on the tool and the operation must not know it.
func localGuardCause(guard linodev1.LocalGuard) string {
	trimmed := strings.TrimPrefix(guard.String(), localGuardPrefix)

	return strings.ToLower(strings.ReplaceAll(trimmed, "_", " "))
}

// localGuardPrefix opens every member of the condition vocabulary.
const localGuardPrefix = "LOCAL_GUARD_"

// pyOperationBase is the class every generated Python condition derives from,
// which is what lets one arm catch the whole of an operation's vocabulary and
// leave anything outside it to the interpreter.
const pyOperationBase = "LocalOperationError"

// generatedOperations answers the operations this run writes the arm for,
// which is every one the contract declares, member-sorted.
//
// There used to be a second input here, a contract file listing the operations
// each engine still wrote by hand. It emptied when the last arm was generated
// and it is gone: a file whose only legal content is nothing exempts nothing,
// and what holds the population at zero now is a gate that scans each language
// rather than a list anyone could add a line to.
func generatedOperations(
	arms map[linodev1.LocalCall]localArm,
) ([]localOperation, error) {
	generated := make([]localOperation, 0, len(arms))

	for _, call := range slices.Sorted(maps.Keys(arms)) {
		operation := localOperation{arm: arms[call], call: call}
		if err := checkGeneratedOperation(&operation); err != nil {
			return nil, err
		}

		generated = append(generated, operation)
	}

	return generated, nil
}

// checkGeneratedOperation holds one operation to what the emitted arm can
// actually write.
func checkGeneratedOperation(operation *localOperation) error {
	// A stateless operation is served by the function's own type, and one type
	// cannot carry a signature per direction. Refused rather than emitted
	// twice: Go would fail on the duplicate name and Python would shadow the
	// first alias with the second and serve one side through the other.
	if operation.arm.declared.GetTwoSided() && !operation.stateful() {
		return fmt.Errorf("%w: %s runs two ways and holds no state",
			errLocalArmSidedStateless, operation.call)
	}

	for _, side := range operation.sides() {
		if side.answers == nil {
			return fmt.Errorf("%w: %s shapes no answer for %s",
				errLocalArmUnshaped, operation.call, side.direction)
		}
	}

	if slices.Contains(operation.reports(),
		linodev1.LocalGuard_LOCAL_GUARD_ARGUMENT_UNUSABLE) {
		return fmt.Errorf("%w: %s reports %s",
			errLocalArmReaderGuard, operation.call,
			linodev1.LocalGuard_LOCAL_GUARD_ARGUMENT_UNUSABLE)
	}

	if err := checkOperationVerdicts(operation); err != nil {
		return err
	}

	return checkGeneratedOperationNames(operation)
}

// checkLocalAmbientRenderings holds the whole reading vocabulary to a stated
// rendering in every registered language.
//
// Over the vocabulary rather than over the operations that declare a reading: a
// member with no rendering is a gap whether or not an operation reaches it
// today, and the run that adds one to the contract is the run that should fail.
// A language with no row hands its subsystem less than the declaration says
// while its arm reads as though it had handed over everything.
//
// A row stating a type but no expression is refused beside it: the subsystem
// would declare a parameter the handler fills with a name nothing defines,
// which Go answers as a build failure in emitted code and Python only as a
// NameError on the first call that reaches the operation.
func checkLocalAmbientRenderings(languages []string) error {
	return checkAmbientRenderings(languages, localAmbientRenderings())
}

// checkAmbientRenderings is that check over a stated table, so a probe can
// perturb one row rather than the registry the run really reads.
func checkAmbientRenderings(
	languages []string,
	renderings map[string]map[linodev1.LocalAmbient]localAmbientParameter,
) error {
	members := linodev1.LocalAmbient(0).Descriptor().Values()

	for _, language := range languages {
		for index := range members.Len() {
			reading := linodev1.LocalAmbient(members.Get(index).Number())
			if reading == linodev1.LocalAmbient_LOCAL_AMBIENT_UNSPECIFIED {
				continue
			}

			taken, rendered := renderings[language][reading]
			if !rendered {
				return fmt.Errorf("%w: %s in %s",
					errLocalArmAmbientUnrendered, reading, language)
			}

			if taken.name != "" && taken.handed == "" {
				return fmt.Errorf("%w: %s in %s",
					errLocalArmAmbientUnhanded, reading, language)
			}
		}
	}

	return nil
}

// checkGeneratedOperationNames holds every registered language to a subsystem
// method it can spell, on every side the operation runs. A language whose
// naming rule is missing goes without a method while its arm still calls one,
// which is how an engine goes one language short of the registry.
func checkGeneratedOperationNames(operation *localOperation) error {
	if localSubsystemType(operation.member()) == "" {
		return fmt.Errorf("%w: %s", errLocalArmUnstemmed, operation.call)
	}

	for _, language := range slices.Sorted(maps.Keys(localArmNamings())) {
		for _, side := range operation.sides() {
			if localSubsystemMethod(language, operation.member(), side.direction) == "" {
				return fmt.Errorf("%w: %s in %s",
					errLocalArmUnnamed, operation.call, language)
			}
		}
	}

	return nil
}

// generatedGuards is every condition the generated operations report, in the
// vocabulary's own order, which is the order the failure types are emitted in.
//
// Read over the whole set rather than per operation: two operations reporting
// one condition share its type, and a condition no generated operation reports
// gets no type at all, since a value nothing raises and nothing catches is
// wiring that arrived early.
func generatedGuards(operations []localOperation) []linodev1.LocalGuard {
	seen := make(map[linodev1.LocalGuard]bool, len(operations))

	for index := range operations {
		for _, guard := range operations[index].reports() {
			seen[guard] = true
		}
	}

	members := linodev1.LocalGuard(0).Descriptor().Values()
	ordered := make([]linodev1.LocalGuard, 0, len(seen))

	for i := range members.Len() {
		guard := linodev1.LocalGuard(members.Get(i).Number())
		if seen[guard] {
			ordered = append(ordered, guard)
		}
	}

	return ordered
}

// localOperationsFile is the base name each language's emitted arms land under,
// beside the tool tree the handlers that call them sit in.
const localOperationsFile = "operations"

// operationsWritten is the sentence both arms open their emitted file with,
// spelled once because a reader comparing the two trees reads it twice.
func operationsWritten() []string {
	return []string{
		"The arm behind each local operation: read the declared inputs, call the",
		"subsystem, word whichever condition it reported, project the answer.",
		"",
		"Nothing here is hand-written and nothing here survives a `make proto`:",
		"change the contract, not this tree. What a language writes instead is the",
		"subsystem method each arm calls, held to the type emitted beside the",
		"answer it fills.",
	}
}

// localOperationInputs is one operation's inputs paired with the reader each is
// filled through, in the operation's own call order.
func localOperationInputs(operation *localOperation) []localOperationInput {
	declared := operation.arm.declared.GetInput()
	readers := localReaders()
	inputs := make([]localOperationInput, 0, len(declared))

	for _, input := range declared {
		inputs = append(inputs, localOperationInput{
			name: input.GetName(),
			reader: readers[localReaderKey{
				kind: input.GetKind(), optional: input.GetOptional(),
			}],
			kind:     input.GetKind(),
			optional: input.GetOptional(),
		})
	}

	return inputs
}

// localOperationInput is one declared parameter as both languages need it: the
// name the subsystem takes it under, and the type each spells it as.
type localOperationInput struct {
	name     string
	reader   localReader
	kind     linodev1.LocalInputKind
	optional bool
}
