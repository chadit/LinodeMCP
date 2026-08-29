package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The local answers: how a meta tool's whole result is declared rather than
// hand-written.
//
// A routed tool declares a call and the emitter writes the request, the
// refusals and the projection around it. A meta tool has no route, so it names
// an operation out of the closed set the contract declares, and gets the same
// three things written around that instead. Each member of that set carries the
// operation it names, so this file reads the table rather than holding it. What
// is not declarable is the operation itself,
// because a draft registry and an audit store are not describable in a
// descriptor, so each language serves them from one engine file.
//
// The guard the whole design rests on is enforced here rather than promised: an
// operation is handed values read through the readers its own declared input
// types name, positionally, and nothing else. There is no member of that type
// set that carries the request, the argument map or the tool's name, so an
// operation cannot be given one and cannot tell which tool reached it.

// localArm is one engine operation as its own declared member states it, with
// the answer shapes resolved once.
type localArm struct {
	// declared is the operation the member carries: the ambient state it reads,
	// the readings it takes, the inputs in call order, the conditions it can
	// report and the shape it answers with. Nothing here restates it, so the
	// member and the operation cannot come apart.
	declared *linodev1.LocalOperation
	// bodies is the message whose members the operation's plain body fills,
	// keyed by the direction it ran in. The operation itself names no message,
	// so this is where a declaring tool's response is held to what the
	// operation can actually build.
	//
	// Resolved from the declared name once per run, and a name nothing declares
	// stops the run: the shape used to be a linked-in type the compiler
	// checked, and a declaration can only carry the name.
	bodies map[linodev1.LocalDirection]protoreflect.MessageDescriptor
}

// reads is whether the operation declared one ambient reading. Held over the
// declaration rather than kept as a field per reading, since a field per
// reading is what two languages could afford and ten cannot.
func (a *localArm) reads(reading linodev1.LocalAmbient) bool {
	return slices.Contains(a.declared.GetAmbient(), reading)
}

// localReader is the function each language reads one input kind with.
type localReader struct {
	goName string
	pyName string
	// refuses is whether the reader answers a cause beside the value, which is
	// what an ARGUMENT_UNUSABLE row words.
	refuses bool
}

// localReaderKey pairs an input's type with whether presence travels with it.
type localReaderKey struct {
	kind     linodev1.LocalInputKind
	optional bool
}

// localReaders is every reader an operation input can be filled through. The
// set is closed on purpose: it is the typed-parameter guard's whole vocabulary,
// and it holds no member that reads the argument map as a map.
func localReaders() map[localReaderKey]localReader {
	return map[localReaderKey]localReader{
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_STRING}:      {goName: "LocalString", pyName: "local_string"},
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_STRING_LIST}: {goName: "LocalStringList", pyName: "local_string_list"},
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_STRING_LIST, optional: true}: {
			goName: "LocalStringListSent", pyName: "local_string_list_sent",
		},
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_INT}:  {goName: "LocalInt", pyName: "local_int"},
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_BOOL}: {goName: "LocalBool", pyName: "local_bool"},
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_BOOL, optional: true}: {
			goName: "LocalBoolSent", pyName: "local_bool_sent",
		},
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_TIMESTAMP}: {
			goName: "LocalTimestamp", pyName: "local_timestamp", refuses: true,
		},
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_CALL_LIST}: {
			goName: "LocalCallEntries", pyName: "local_call_entries",
		},
	}
}

// localEntryType is what every language calls the record a call-list input
// reaches a subsystem as. One spelling for every language, because it names the
// contract's own input kind rather than anything a language owns.
//
// It is emitted into the answers tree rather than written beside the readers,
// which is where it used to live: a subsystem type names it, and the answers
// tree is the one place both the engine and the handlers can read a name from.
const localEntryType = "CallEntry"

// localTakesCallList is whether any generated operation reads a call list,
// which is what decides whether the entry record is emitted at all. Emitting it
// unconditionally would put a type in every language's tree that nothing names.
func localTakesCallList(operations []localOperation) bool {
	for index := range operations {
		for _, input := range operations[index].arm.declared.GetInput() {
			if input.GetKind() == linodev1.LocalInputKind_LOCAL_INPUT_KIND_CALL_LIST {
				return true
			}
		}
	}

	return false
}

// localArmPrefix opens every member of the operation vocabulary, and a stem is
// what one leaves behind: LOCAL_CALL_DRAFT_CREATE stems to draft_create, which
// each language then spells its own way.
const localArmPrefix = "LOCAL_CALL_"

// localArmStem is the operation's own spelling inside its declared member.
//
// A member the prefix does not open, or one a doubled underscore leaves an
// empty word in, stems to nothing: a name the emitter cannot derive is one it
// must refuse rather than guess at.
func localArmStem(member string) (string, bool) {
	stem, opened := strings.CutPrefix(member, localArmPrefix)
	if !opened {
		return "", false
	}

	if slices.Contains(strings.Split(stem, "_"), "") {
		return "", false
	}

	return strings.ToLower(stem), true
}

// localArmNaming is how one registered language spells an operation's own
// function: what the name opens with, and whether the stem's words run together
// in upper case rather than staying underscore-separated.
type localArmNaming struct {
	language string
	prefix   string
	pascal   bool
}

// spell is the function name one language serves an operation's stem with.
// Stems reach here through localArmStem, which refuses an empty word.
func (n localArmNaming) spell(stem string) string {
	if !n.pascal {
		return n.prefix + stem
	}

	words := strings.Split(stem, "_")
	for i, word := range words {
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}

	return n.prefix + strings.Join(words, "")
}

// localArmNamings is the spelling rule each registered language serves the
// engine with.
//
// A registered language with no rule here fails by name rather than being
// skipped, the way armFor refuses a language with no renderer: deriving names
// for the languages the emitter recognized and saying nothing about the rest is
// how an engine goes one language short of its registry.
func localArmNamings() map[string]localArmNaming {
	return map[string]localArmNaming{
		languageGo:     {language: languageGo, prefix: "Run", pascal: true},
		languagePython: {language: languagePython, prefix: "run_"},
	}
}

// localArmNamingsFor answers one rule per registered language, in registry
// order.
func localArmNamingsFor(languages []string) ([]localArmNaming, error) {
	namings := localArmNamings()
	resolved := make([]localArmNaming, 0, len(languages))

	for _, language := range languages {
		naming, ruled := namings[language]
		if !ruled {
			return nil, fmt.Errorf("%w: %s", errLocalArmUnnamed, language)
		}

		resolved = append(resolved, naming)
	}

	return resolved, nil
}

// localArmSpelling is the function one language serves one side of a declared
// member with, or "" when no name derives. Every way that can happen is refused
// before a run renders, so a renderer reaching here has both a stem and a rule.
func localArmSpelling(language, member string, direction linodev1.LocalDirection) string {
	stem, derived := localSideStem(member, direction)
	if !derived {
		return ""
	}

	return localArmNamings()[language].spell(stem)
}

// localArms is the operation behind each declarable call, read off the members
// themselves.
//
// The table used to be a literal here, which put the vocabulary in the contract
// and everything the vocabulary means in this one language's source. Now each
// member carries its own local_operation, so a ten-language emitter reads the
// operations the same way a two-language one does and no engine can go one
// declaration short of the contract.
//
// A member carrying no declaration yields no entry, which checkLocalCallsServed
// then refuses by name.
func localArms() (map[linodev1.LocalCall]localArm, error) {
	members := linodev1.LocalCall(0).Descriptor().Values()
	arms := make(map[linodev1.LocalCall]localArm, members.Len())

	for i := range members.Len() {
		member := members.Get(i)
		call := linodev1.LocalCall(member.Number())

		declared, _ := proto.GetExtension(
			member.Options(), linodev1.E_LocalOperation,
		).(*linodev1.LocalOperation)
		if declared == nil {
			continue
		}

		bodies, err := localArmBodies(call, declared)
		if err != nil {
			return nil, err
		}

		arms[call] = localArm{declared: declared, bodies: bodies}
	}

	return arms, nil
}

// localArmBodies resolves the shape the operation answers with in each
// direction it declared one for.
//
// A direction shaped twice is refused rather than let the later row win: the
// declaration would be saying two things and the emitter would act on one of
// them without saying which.
func localArmBodies(
	call linodev1.LocalCall, declared *linodev1.LocalOperation,
) (map[linodev1.LocalDirection]protoreflect.MessageDescriptor, error) {
	answers := declared.GetAnswers()
	bodies := make(map[linodev1.LocalDirection]protoreflect.MessageDescriptor, len(answers))

	for _, answer := range answers {
		direction := answer.GetDirection()
		if _, shaped := bodies[direction]; shaped {
			return nil, fmt.Errorf("%w: %s shapes %s twice", errLocalArmUnshaped, call, direction)
		}

		shape, err := localArmShape(call, answer.GetShape())
		if err != nil {
			return nil, err
		}

		bodies[direction] = shape
	}

	return bodies, nil
}

// localArmShape is the message one declared shape names.
//
// Resolved against the linked-in descriptors, which is where the compiler used
// to hold a shape before the table moved into the contract. A name nothing
// declares stops the run rather than reaching the first call.
func localArmShape(
	call linodev1.LocalCall, shape string,
) (protoreflect.MessageDescriptor, error) {
	found, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(shape))
	if err != nil {
		return nil, fmt.Errorf("%w: %s answers with %q: %w", errLocalArmUnshaped, call, shape, err)
	}

	return found.Descriptor(), nil
}

// localAnswer is one tool's declared local answer, resolved against the arm it
// names so nothing downstream re-walks the declaration.
type localAnswer struct {
	// ReaderWording is the sentence each refusing reader answers with, keyed by
	// the argument it reads. Settled once the ladder is held to a row per such
	// reader, so a render reads it rather than searching for it again.
	ReaderWording map[string]string
	// Arm is the operation the tool is built on.
	Arm localArm
	// Bound is the argument filling each of the arm's inputs, in the arm's own
	// input order, which is the order the call passes them in.
	Bound []string
	// Ladder is the guard rows in declaration order, each already held to the
	// arm and to the arguments a binding fills.
	Ladder []*linodev1.LocalRefusal
	// Call is the declared member, kept for the refusals that name it.
	Call linodev1.LocalCall
	// Direction is the declared direction, unset on a one-sided operation.
	Direction linodev1.LocalDirection
}

// readLocalAnswer records the declared local answer, holding it to the arm it
// names before anything renders it.
//
// Run after the fields are read, since every binding and every sentence is
// measured against the arguments the message declares.
func (c *contract) readLocalAnswer(options protoreflect.ProtoMessage) error {
	declared, _ := proto.GetExtension(options, linodev1.E_LocalAnswer).(*linodev1.LocalAnswer)
	if declared == nil {
		return nil
	}

	if c.Tier != tierMeta {
		return fmt.Errorf("%w: %s", errLocalAnswerTier, c.Name)
	}

	if c.SuccessMessage != "" {
		return fmt.Errorf("%w: %s", errLocalAnswerTwoAnswers, c.Name)
	}

	arms, armsErr := localArms()
	if armsErr != nil {
		return armsErr
	}

	// A member with no operation is refused a step earlier, by the table check
	// every run opens with, so the only call that reaches here unserved is the
	// unset one.
	arm, served := arms[declared.GetCall()]
	if !served {
		return fmt.Errorf("%w: %s names %s", errNoLocalCall, c.Name, declared.GetCall())
	}

	answer := localAnswer{Arm: arm, Call: declared.GetCall(), Direction: declared.GetDirection()}

	if err := c.checkLocalState(declared, &arm); err != nil {
		return err
	}

	if err := c.checkLocalDirection(declared, &arm); err != nil {
		return err
	}

	bound, err := c.bindLocalInputs(declared, &arm)
	if err != nil {
		return err
	}

	answer.Bound = bound
	answer.Ladder = declared.GetRefuse()

	if err := c.checkLocalLadder(&answer); err != nil {
		return err
	}

	if err := c.checkLocalShape(&answer); err != nil {
		return err
	}

	c.LocalAnswer = &answer

	return nil
}

// checkLocalShape holds the response the tool declares to the shape its
// operation fills.
//
// This is the output half of the guard the typed parameters are the input half
// of. The operation names no message, so nothing else would notice a tool
// declaring a response the operation cannot build: a member the operation never
// fills would answer a caller a zero nothing computed, and a member it does
// fill that the response leaves out has nowhere to go. Both are refused here
// rather than met on the first call.
func (c *contract) checkLocalShape(answer *localAnswer) error {
	difference := localShapeDifference(answer.Arm.bodies[answer.Direction], c.ResponseGo.Descriptor)
	if difference == "" {
		return nil
	}

	return fmt.Errorf("%w: %s answers with %s, and %s %s",
		errLocalAnswerShape, c.Name, c.ResponseGo.FullName, answer.Call, difference)
}

// checkLocalState holds the declared ambient state to the state the operation
// reads, in both directions: an operation reading state the tool does not
// declare would resolve none, and one reading none would answer for an absence
// it never meets.
func (c *contract) checkLocalState(declared *linodev1.LocalAnswer, arm *localArm) error {
	wanted := arm.declared.GetRequires()
	got := declared.GetRequires()

	if wanted == linodev1.LocalState_LOCAL_STATE_UNSPECIFIED {
		if got != linodev1.LocalState_LOCAL_STATE_UNSPECIFIED {
			return fmt.Errorf("%w: %s requires %s", errLocalStateUnused, c.Name, got)
		}

		return nil
	}

	if got != wanted {
		return fmt.Errorf("%w: %s requires %s and %s reads %s",
			errLocalStateMismatch, c.Name, got, declared.GetCall(), wanted)
	}

	return nil
}

// checkLocalDirection refuses a direction on a one-sided operation and an
// absent one on a two-sided operation, for the reason TransferDirection is
// refused unset: the zero reads as one of the two rather than as no answer.
func (c *contract) checkLocalDirection(declared *linodev1.LocalAnswer, arm *localArm) error {
	set := declared.GetDirection() != linodev1.LocalDirection_LOCAL_DIRECTION_UNSPECIFIED
	if set == arm.declared.GetTwoSided() {
		return nil
	}

	return fmt.Errorf("%w: %s declares %s on %s",
		errLocalDirectionUnserved, c.Name, declared.GetDirection(), declared.GetCall())
}

// bindLocalInputs answers the argument filling each of the operation's inputs,
// in the operation's own input order.
//
// This is the typed-parameter guard's enforcing half. Every input has to be
// filled by a declared binding whose argument the message declares and whose
// kind the input's reader can build, so the only values that reach an operation
// are the ones a reader produced from a named argument.
func (c *contract) bindLocalInputs(declared *linodev1.LocalAnswer, arm *localArm) ([]string, error) {
	inputs := arm.declared.GetInput()
	filled := make(map[string]string, len(inputs))

	for _, binding := range declared.GetBind() {
		if err := c.readLocalBinding(binding, arm, filled); err != nil {
			return nil, err
		}
	}

	bound := make([]string, 0, len(inputs))

	for _, input := range inputs {
		argument, ok := filled[input.GetName()]
		if !ok {
			return nil, fmt.Errorf("%w: %s leaves %s of %s unbound",
				errLocalBindMissing, c.Name, input.GetName(), declared.GetCall())
		}

		bound = append(bound, argument)
	}

	return bound, nil
}

// readLocalBinding holds one binding to the operation's inputs and to the
// arguments this message declares.
func (c *contract) readLocalBinding(
	binding *linodev1.LocalBinding, arm *localArm, filled map[string]string,
) error {
	name := binding.GetInput()
	if name == "" {
		return fmt.Errorf("%w: %s", errLocalBindNoInput, c.Name)
	}

	inputs := arm.declared.GetInput()

	index := slices.IndexFunc(inputs, func(input *linodev1.LocalOperationInput) bool {
		return input.GetName() == name
	})
	if index < 0 {
		return fmt.Errorf("%w: %s binds %s", errLocalBindUnknownInput, c.Name, name)
	}

	if _, already := filled[name]; already {
		return fmt.Errorf("%w: %s binds %s twice", errLocalBindRepeated, c.Name, name)
	}

	argument := binding.GetArgument()

	local, declared := c.localArgument(argument)
	if !declared {
		return fmt.Errorf("%w: %s binds %s from %s",
			errLocalBindUnknownArgument, c.Name, name, argument)
	}

	if !localKindReadable(inputs[index].GetKind(), &local) {
		return fmt.Errorf("%w: %s fills %s from %s, which is %s",
			errLocalBindKind, c.Name, name, argument, local.Kind)
	}

	filled[name] = argument

	return nil
}

// localArgument is the tool's own argument one binding or sentence names. Only
// the local ones are in scope: a meta tool reaches no route, so every argument
// it declares is its own.
func (c *contract) localArgument(name string) (field, bool) {
	for _, entry := range c.Local {
		if entry.ProtoName == name {
			return entry, true
		}
	}

	return field{}, false
}

// localKindReadable is whether one input's reader can build its type from the
// argument's declared kind. It is what stops a list arriving where a flag is
// read and a message arriving where text is.
func localKindReadable(kind linodev1.LocalInputKind, argument *field) bool {
	// An enum argument reads as text: a schema advertises it by member name, so
	// that is the form a caller sends and a reader gets.
	text := argument.Kind == protoreflect.StringKind
	readable := map[linodev1.LocalInputKind]bool{
		linodev1.LocalInputKind_LOCAL_INPUT_KIND_STRING: !argument.Repeated &&
			(text || argument.Kind == protoreflect.EnumKind),
		linodev1.LocalInputKind_LOCAL_INPUT_KIND_TIMESTAMP:   !argument.Repeated && text,
		linodev1.LocalInputKind_LOCAL_INPUT_KIND_STRING_LIST: argument.Repeated && text,
		linodev1.LocalInputKind_LOCAL_INPUT_KIND_INT: !argument.Repeated &&
			localIntegerKind(argument.Kind),
		linodev1.LocalInputKind_LOCAL_INPUT_KIND_BOOL: !argument.Repeated &&
			argument.Kind == protoreflect.BoolKind,
		linodev1.LocalInputKind_LOCAL_INPUT_KIND_CALL_LIST: argument.Repeated &&
			argument.Kind == protoreflect.MessageKind,
	}

	return readable[kind]
}

// localIntegerKind is the integer widths a tool argument is declared in.
func localIntegerKind(kind protoreflect.Kind) bool {
	return kind == protoreflect.Int32Kind || kind == protoreflect.Int64Kind
}

// checkLocalLadder holds the guard ladder to the operation and to the arguments
// a binding fills, in both directions: no row the operation cannot report, and
// no condition it reports left unworded.
func (c *contract) checkLocalLadder(answer *localAnswer) error {
	seen := make([]localGuardRow, 0, len(answer.Ladder))

	for _, row := range answer.Ladder {
		if err := c.readLocalRefusal(row, answer, seen); err != nil {
			return err
		}

		seen = append(seen, localGuardRow{guard: row.GetGuard(), argument: row.GetArgument()})
	}

	return c.checkLocalUnworded(answer, seen)
}

// localGuardRow is one condition the ladder has already worded. The argument
// travels with the guard because a refusing reader answers for its own
// argument, so a ladder words ARGUMENT_UNUSABLE once per reading argument and
// every other guard once.
type localGuardRow struct {
	argument string
	guard    linodev1.LocalGuard
}

// readLocalRefusal holds one ladder row to the operation, to the argument its
// guard reads, and to the values its sentence can name.
func (c *contract) readLocalRefusal(
	row *linodev1.LocalRefusal, answer *localAnswer, seen []localGuardRow,
) error {
	guard := row.GetGuard()
	if guard == linodev1.LocalGuard_LOCAL_GUARD_UNSPECIFIED {
		return fmt.Errorf("%w: %s", errLocalRefuseNoGuard, c.Name)
	}

	if row.GetMessage() == "" {
		return fmt.Errorf("%w: %s refuses %s in silence", errLocalRefuseSilent, c.Name, guard)
	}

	if err := c.checkLocalGuardArm(guard, answer); err != nil {
		return err
	}

	if err := c.checkLocalGuardArgument(row, answer); err != nil {
		return err
	}

	already := localGuardRow{guard: guard, argument: row.GetArgument()}
	if slices.Contains(seen, already) {
		return fmt.Errorf("%w: %s refuses %s twice", errLocalRefuseRepeated, c.Name, guard)
	}

	return c.checkLocalPlaceholders(row, answer)
}

// checkLocalGuardArm refuses a row naming a condition the operation cannot
// report, whose sentence nothing would ever answer.
func (c *contract) checkLocalGuardArm(guard linodev1.LocalGuard, answer *localAnswer) error {
	if guard == linodev1.LocalGuard_LOCAL_GUARD_ARGUMENT_UNUSABLE {
		return nil
	}

	if slices.Contains(answer.Arm.declared.GetReports(), guard) {
		return nil
	}

	return fmt.Errorf("%w: %s refuses %s and %s does not report it",
		errLocalRefuseGuardArm, c.Name, guard, answer.Call)
}

// checkLocalGuardArgument holds a guard to the argument it reads: every guard
// that reads one names an argument a binding fills, and the two store failures
// name none, since neither reads an argument to fail on.
func (c *contract) checkLocalGuardArgument(row *linodev1.LocalRefusal, answer *localAnswer) error {
	guard := row.GetGuard()
	argument := row.GetArgument()
	reads := localGuardReadsArgument(guard)

	if reads != (argument != "") {
		return fmt.Errorf("%w: %s refuses %s naming %q",
			errLocalRefuseArgument, c.Name, guard, argument)
	}

	if !reads {
		return nil
	}

	if !slices.Contains(answer.Bound, argument) {
		return fmt.Errorf("%w: %s refuses %s on %s, which fills no input of %s",
			errLocalRefuseArgument, c.Name, guard, argument, answer.Call)
	}

	if guard != linodev1.LocalGuard_LOCAL_GUARD_ARGUMENT_UNUSABLE {
		return nil
	}

	if !answer.refusingArgument(argument) {
		return fmt.Errorf("%w: %s refuses %s on %s, whose reader cannot refuse",
			errLocalRefuseArgument, c.Name, guard, argument)
	}

	return nil
}

// localGuardReadsArgument is whether a guard answers about one named argument.
// Written as the exceptions rather than the rule: every condition names the
// argument it read, except the two that report a store the tool never named.
func localGuardReadsArgument(guard linodev1.LocalGuard) bool {
	return !slices.Contains([]linodev1.LocalGuard{
		linodev1.LocalGuard_LOCAL_GUARD_READ_FAILED,
		linodev1.LocalGuard_LOCAL_GUARD_WRITE_FAILED,
	}, guard)
}

// refusingArgument is whether the argument fills an input whose reader answers
// a cause beside its value.
func (a *localAnswer) refusingArgument(argument string) bool {
	return slices.Contains(a.refusingArguments(), argument)
}

// checkLocalUnworded refuses a declaration that leaves a condition its
// operation reports without a sentence, which would refuse a caller in silence
// at call time. Every refusing reader answers for itself the same way.
func (c *contract) checkLocalUnworded(answer *localAnswer, seen []localGuardRow) error {
	for _, guard := range answer.Arm.declared.GetReports() {
		worded := slices.ContainsFunc(seen, func(row localGuardRow) bool { return row.guard == guard })
		if !worded {
			return fmt.Errorf("%w: %s leaves %s of %s unworded",
				errLocalRefuseUnworded, c.Name, guard, answer.Call)
		}
	}

	answer.ReaderWording = make(map[string]string, len(answer.Arm.declared.GetInput()))

	for _, argument := range answer.refusingArguments() {
		wording := answer.wordingFor(argument)
		if wording == "" {
			return fmt.Errorf("%w: %s leaves the reader of %s unworded",
				errLocalRefuseUnworded, c.Name, argument)
		}

		answer.ReaderWording[argument] = wording
	}

	return nil
}

// wordingFor is the sentence the ladder gives one reading's own refusal, empty
// where it gives none.
func (a *localAnswer) wordingFor(argument string) string {
	for _, row := range a.Ladder {
		if row.GetGuard() == linodev1.LocalGuard_LOCAL_GUARD_ARGUMENT_UNUSABLE &&
			row.GetArgument() == argument {
			return row.GetMessage()
		}
	}

	return ""
}

// refusingArguments is every bound argument whose reader answers a cause beside
// its value, in the operation's own input order.
func (a *localAnswer) refusingArguments() []string {
	readers := localReaders()
	inputs := a.Arm.declared.GetInput()
	found := make([]string, 0, len(inputs))

	for index, input := range inputs {
		if readers[localReaderKey{kind: input.GetKind(), optional: input.GetOptional()}].refuses {
			found = append(found, a.Bound[index])
		}
	}

	return found
}

// localPlaceholder is one {name} a sentence spells, split from its form.
type localPlaceholder struct {
	name string
	// quoted is whether the sentence asked for the value in quotes, which is
	// the one form beyond the plain value.
	quoted bool
}

// localCausePlaceholder is the sentence's name for the cause the reader or the
// operation reported.
const localCausePlaceholder = "error"

// localQuotedForm is the suffix a sentence adds to ask for a quoted value.
const localQuotedForm = ":quoted"

// checkLocalPlaceholders refuses a sentence naming a value the handler cannot
// fill. Only a bound argument and the reported cause are in scope, which is
// what keeps a tool's own identity out of the operation: the operation reports
// a condition without words, and the sentence is filled here.
func (c *contract) checkLocalPlaceholders(row *linodev1.LocalRefusal, answer *localAnswer) error {
	for _, named := range localPlaceholders(row.GetMessage()) {
		if named.name == localCausePlaceholder {
			continue
		}

		if !slices.Contains(answer.Bound, named.name) {
			return fmt.Errorf("%w: %s names %s in %q",
				errLocalRefusePlaceholder, c.Name, named.name, row.GetMessage())
		}
	}

	return nil
}

// localBraces is where one placeholder opens and closes in a sentence, both -1
// when the rest carries no closed pair. A brace the sentence never closes is
// text rather than a placeholder, so every renderer stops reading at the same
// point.
func localBraces(sentence string) (int, int) {
	open := strings.Index(sentence, "{")
	if open < 0 {
		return -1, -1
	}

	closed := strings.Index(sentence[open:], "}")
	if closed < 0 {
		return -1, -1
	}

	return open, closed
}

// localPlaceholders is every {name} one sentence spells, in the order it
// spells them.
func localPlaceholders(sentence string) []localPlaceholder {
	named := make([]localPlaceholder, 0)
	rest := sentence

	for {
		open, closed := localBraces(rest)
		if open < 0 {
			return named
		}

		text := rest[open+1 : open+closed]
		trimmed, quoted := strings.CutSuffix(text, localQuotedForm)
		named = append(named, localPlaceholder{name: trimmed, quoted: quoted})
		rest = rest[open+closed+1:]
	}
}

// checkLocalArms holds the engine table itself to the contract and to the rule
// the whole vocabulary rests on, ahead of any rendering.
//
// It answers for the table the way checkRendererCoverage answers for the arms:
// the declarations are checked against the table everywhere else, so nothing
// else would ever notice a member no language could spell, an entry that took
// an input outside the reader set, or one named after a tool.
func checkLocalArms(arms map[linodev1.LocalCall]localArm, languages, tools []string) error {
	calls := slices.Sorted(maps.Keys(arms))

	if err := checkLocalArmNames(
		localArmMembers(calls), localSidedMembers(arms, calls), languages, tools,
	); err != nil {
		return err
	}

	if err := checkLocalAmbientRenderings(languages); err != nil {
		return err
	}

	readers := localReaders()

	for _, call := range calls {
		arm := arms[call]

		if err := checkLocalArmAmbient(call, &arm); err != nil {
			return err
		}

		if err := checkLocalArmInputs(call, &arm, readers); err != nil {
			return err
		}

		if err := checkLocalArmShapes(call, &arm); err != nil {
			return err
		}
	}

	return checkLocalCallsServed(arms)
}

// localSidedMembers is the members that run in two directions, which is what
// makes each of them spell a function per side.
func localSidedMembers(
	arms map[linodev1.LocalCall]localArm, calls []linodev1.LocalCall,
) []string {
	sided := make([]string, 0, len(calls))

	for _, call := range calls {
		if arms[call].declared.GetTwoSided() {
			sided = append(sided, call.String())
		}
	}

	return sided
}

// localArmMembers is the declared member behind each call, in call order so a
// refusal over the table reads the same way twice.
func localArmMembers(calls []linodev1.LocalCall) []string {
	members := make([]string, 0, len(calls))
	for _, call := range calls {
		members = append(members, call.String())
	}

	return members
}

// checkLocalArmShapes holds one operation's declared answer shapes to the
// directions it runs in and to the linked-in descriptors.
//
// Answered over the table rather than per tool for the reason
// checkLocalCallsServed is: an operation shaping no answer would fail whichever
// tool declared it first, and the gap would read as that tool's problem.
func checkLocalArmShapes(call linodev1.LocalCall, arm *localArm) error {
	directions := []linodev1.LocalDirection{linodev1.LocalDirection_LOCAL_DIRECTION_UNSPECIFIED}
	if arm.declared.GetTwoSided() {
		directions = []linodev1.LocalDirection{
			linodev1.LocalDirection_LOCAL_DIRECTION_ADD,
			linodev1.LocalDirection_LOCAL_DIRECTION_REMOVE,
		}
	}

	if len(arm.bodies) != len(directions) {
		return fmt.Errorf("%w: %s shapes %d answers and runs %d ways",
			errLocalArmUnshaped, call, len(arm.bodies), len(directions))
	}

	for _, direction := range directions {
		if _, shaped := arm.bodies[direction]; !shaped {
			return fmt.Errorf("%w: %s shapes no answer for %s",
				errLocalArmUnshaped, call, direction)
		}
	}

	return nil
}

// checkLocalArmInputs holds one operation's inputs to the closed reader set and
// to each other.
func checkLocalArmInputs(
	call linodev1.LocalCall, arm *localArm, readers map[localReaderKey]localReader,
) error {
	inputs := arm.declared.GetInput()
	seen := make([]string, 0, len(inputs))

	for position, input := range inputs {
		name := input.GetName()
		if name == "" {
			return fmt.Errorf("%w: %s takes an unnamed input at %d",
				errLocalArmInputUnnamed, call, position)
		}

		if _, readable := readers[localReaderKey{
			kind: input.GetKind(), optional: input.GetOptional(),
		}]; !readable {
			return fmt.Errorf("%w: %s takes %s", errLocalArmInputKind, call, name)
		}

		if slices.Contains(seen, name) {
			return fmt.Errorf("%w: %s takes %s twice", errLocalArmInputRepeated, call, name)
		}

		seen = append(seen, name)
	}

	return nil
}

// checkLocalArmAmbient holds an operation's declared readings to the vocabulary
// and to each other.
//
// A reading outside the set, or one declared twice, renders nothing: every arm
// asks the declaration whether it reads a named member, so a member no arm asks
// about would be a declaration the emitter drops without saying so.
func checkLocalArmAmbient(call linodev1.LocalCall, arm *localArm) error {
	seen := make([]linodev1.LocalAmbient, 0, len(arm.declared.GetAmbient()))

	for _, reading := range arm.declared.GetAmbient() {
		if reading == linodev1.LocalAmbient_LOCAL_AMBIENT_UNSPECIFIED {
			return fmt.Errorf("%w: %s reads %s", errLocalArmAmbient, call, reading)
		}

		if slices.Contains(seen, reading) {
			return fmt.Errorf("%w: %s reads %s twice", errLocalArmAmbient, call, reading)
		}

		seen = append(seen, reading)
	}

	return nil
}

// checkLocalArmNames derives the function every registered language serves each
// declared member with, and holds all of them to the rules the vocabulary rests
// on.
//
// Derived rather than stated: a name per language is a field per language, and
// a check reading two fields by name is a check that stays at two while the
// registry grows.
func checkLocalArmNames(members, sided, languages, tools []string) error {
	namings, err := localArmNamingsFor(languages)
	if err != nil {
		return err
	}

	stems, err := localArmStems(members, sided, tools)
	if err != nil {
		return err
	}

	return checkLocalArmSpellings(stems, namings)
}

// localArmStems is the operation's own spelling behind each declared member,
// one per side that member runs in.
//
// An operation named after a tool is refused here: one named for the tool it
// serves is per-tool code under another heading, and the whole point is that
// two tools can share one. The sides are stemmed here too, because a two-sided
// member spells a function per direction and those spellings can collide with a
// member of their own.
func localArmStems(members, sided, tools []string) ([]string, error) {
	stems := make([]string, 0, len(members))

	for _, member := range members {
		stem, derived := localArmStem(member)
		if !derived {
			return nil, fmt.Errorf("%w: %s", errLocalArmUnstemmed, member)
		}

		if tool := localToolNamed(stem, tools); tool != "" {
			return nil, fmt.Errorf("%w: %s is named after %s", errLocalArmToolNamed, member, tool)
		}

		for _, direction := range localMemberDirections(slices.Contains(sided, member)) {
			// The member stems, so every side of it does.
			side, _ := localSideStem(member, direction)
			stems = append(stems, side)
		}
	}

	return stems, nil
}

// checkLocalArmSpellings holds each language's derivation to one function per
// operation.
//
// Two members stem alike when they differ only in case, which this repo allows
// by excepting buf's upper-snake enum rule, and a language rule that erases the
// separator collides where one keeping it does not.
func checkLocalArmSpellings(stems []string, namings []localArmNaming) error {
	for _, naming := range namings {
		if shared := localSharedSpelling(stems, naming); shared != "" {
			return fmt.Errorf("%w: %s spells two operations %s",
				errLocalArmSpellingShared, naming.language, shared)
		}
	}

	return nil
}

// localSharedSpelling is the function one language would serve two operations
// with, or "" when each gets its own.
func localSharedSpelling(stems []string, naming localArmNaming) string {
	spelled := make(map[string]bool, len(stems))

	for _, stem := range stems {
		name := naming.spell(stem)
		if spelled[name] {
			return name
		}

		spelled[name] = true
	}

	return ""
}

// localToolNamed is the tool a name is named after, or "" when it is named
// after none. Case and separators drop out, so Go's LinodeAuditExportAnswer and
// Python's linode_audit_export_answer both reduce to linode_audit_export.
func localToolNamed(name string, tools []string) string {
	flat := strings.ToLower(strings.ReplaceAll(name, "_", ""))

	for _, tool := range tools {
		if strings.HasPrefix(flat, strings.ReplaceAll(tool, "_", "")) {
			return tool
		}
	}

	return ""
}

// checkLocalCallsServed holds the declared vocabulary to the table: a member
// carrying no operation would fail one tool's generation rather than the run,
// and the gap would read as a tool problem.
//
// This is the refusal a member added to LocalCall without a local_operation
// beside it raises, which is the one way the two can come apart now that
// neither restates the other.
func checkLocalCallsServed(arms map[linodev1.LocalCall]localArm) error {
	members := linodev1.LocalCall(0).Descriptor().Values()

	for i := range members.Len() {
		call := linodev1.LocalCall(members.Get(i).Number())
		if call == linodev1.LocalCall_LOCAL_CALL_UNSPECIFIED {
			continue
		}

		if _, served := arms[call]; !served {
			return fmt.Errorf("%w: %s", errUnservedLocalCall, call)
		}
	}

	return nil
}

// answersLocally is whether the tool's whole result comes from a declared local
// operation.
func (c *contract) answersLocally() bool {
	return c.LocalAnswer != nil
}
