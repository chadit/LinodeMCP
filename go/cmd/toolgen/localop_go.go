package main

import (
	"fmt"
	"sort"
	"strings"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The Go arm of a generated operation: the failure vocabulary and the subsystem
// type beside the answer, and the arm itself beside the handler that calls it.
//
// The split follows what each package may reach. A subsystem is engine code, so
// the type it satisfies has to sit where the engine can import it, which is the
// answers tree. The arm reaches the engine's outcome as well as the answer, so
// it sits with the handlers, which already read both.

// importGenlocal is the answers tree, which the emitted arms read the answer
// types, the failure vocabulary and the subsystem types out of.
const importGenlocal = "github.com/chadit/LinodeMCP/go/internal/genlocal"

// goOperationsFile is the one file the Go operations arm writes.
const goOperationsFile = localOperationsFile + generatedSuffix

// goSubsystemLocal is the name every emitted arm takes its subsystem under. It
// is deliberately not the operation's own word: the arm must read as one shape
// whichever operation it serves.
const goSubsystemLocal = "subsystem"

// goOperationInputType is what one declared input kind is carried as, and
// whether that spelling names a type the answers tree itself declares.
//
// answers is what separates the two places the same row is read from: the
// subsystem type is written INSIDE the answers tree and names the type plain,
// while the arm sits beside the handlers and has to qualify it.
type goOperationInputType struct {
	carried string
	answers bool
}

// goOperationInputTypes is what each declared input kind is carried as, and
// what the optional form of it is where one exists. The table is data rather
// than a branch per kind so a kind added to the contract is one row.
func goOperationInputTypes() map[localReaderKey]goOperationInputType {
	return map[localReaderKey]goOperationInputType{
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_STRING}:      {carried: textType},
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_STRING_LIST}: {carried: "[]" + textType},
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_STRING_LIST, optional: true}: {
			carried: "*[]" + textType,
		},
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_INT}:  {carried: typeWordInt},
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_BOOL}: {carried: typeWordBool},
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_BOOL, optional: true}: {
			carried: "*" + typeWordBool,
		},
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_TIMESTAMP}: {carried: "time.Time"},
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_CALL_LIST}: {
			carried: "[]" + localEntryType, answers: true,
		},
	}
}

// goOperationInput is the type one declared input reaches the subsystem as, as
// the tree named by qualifier spells it: empty inside the answers tree, and the
// answers package elsewhere.
//
// A kind with no row is still refused rather than guessed at. The reader set
// itself stays closed: this adds no member to it, only the spelling the answers
// tree carries the one member it had no type for.
func goOperationInput(
	call linodev1.LocalCall, input *localOperationInput,
	types map[localReaderKey]goOperationInputType, qualifier string,
) (string, error) {
	carried, typed := types[localReaderKey{
		kind: input.kind, optional: input.optional,
	}]
	if !typed {
		return "", fmt.Errorf("%w: %s takes %s as %s",
			errLocalArmInputUntyped, call, input.name, input.kind)
	}

	return goOperationInputSpelling(carried, qualifier), nil
}

// goOperationInputSpelling qualifies a type the answers tree declares, leaving
// every other spelling alone. The qualifier reaches inside the list marker
// because the type it names is the element, not the list.
func goOperationInputSpelling(carried goOperationInputType, qualifier string) string {
	if !carried.answers || qualifier == "" {
		return carried.carried
	}

	marker, element := strings.CutPrefix(carried.carried, "[]")
	if !element {
		return qualifier + "." + carried.carried
	}

	return "[]" + qualifier + "." + marker
}

// checkGoOperationRendering holds one operation to what this arm writes. The
// readings are checked once for every language before this runs, so what is
// left is the inputs the answers tree carries no type for.
func checkGoOperationRendering(
	operation *localOperation, types map[localReaderKey]goOperationInputType,
) error {
	for _, input := range localOperationInputs(operation) {
		if _, err := goOperationInput(operation.call, &input, types, ""); err != nil {
			return err
		}
	}

	return nil
}

// goOperationSurface writes the failure vocabulary and the subsystem type each
// generated operation is served through, which land in the answers tree beside
// the shapes they answer with.
func goOperationSurface(operations []localOperation) (string, bool, error) {
	if len(operations) == 0 {
		return "", false, nil
	}

	var out strings.Builder

	out.Grow(len(operations) * goAnswerShapeBudget)

	guards := generatedGuards(operations)
	for _, guard := range guards {
		out.WriteString("\n// ")
		out.WriteString(localGuardSentinel(languageGo, guard))
		out.WriteString(" is the condition ")
		out.WriteString(guard.String())
		out.WriteString(" names.\n")
		out.WriteString("// A subsystem reports it by answering an error that wraps this one.\n")
		out.WriteString("var ")
		out.WriteString(localGuardSentinel(languageGo, guard))
		out.WriteString(" = errors.New(")
		out.WriteString(goStringLiteral(localGuardCause(guard)))
		out.WriteString(")\n")
	}

	if len(guards) > 0 {
		out.WriteString(goOperationCause())
	}

	if localTakesCallList(operations) {
		out.WriteString(goOperationEntry())
	}

	out.WriteString(goVerdictSurface(operations))

	for index := range operations {
		text, err := goSubsystemType(&operations[index])
		if err != nil {
			return "", false, err
		}

		out.WriteString(text)
	}

	return out.String(), len(guards) > 0, nil
}

// goOperationCause writes the way a Go subsystem reports a condition with the
// cause beside it.
//
// The Python arm needs none of this: its emitted condition is a class whose
// constructor already takes a cause. A Go sentinel is one value shared by every
// reporter, so carrying a cause needs a wrapper that answers the cause and
// unwraps to the condition the arm's ladder compares against.
func goOperationCause() string {
	return `
// LocalCause is one condition reported with the cause beside it, since a
// sentinel shared by every reporter cannot carry one itself.
type LocalCause struct {
	condition error
	cause     string
}

// NewLocalCause reports one condition carrying the cause a sentence may name. A
// subsystem with nothing to add answers the condition itself. The concrete type
// is what a caller returns, so the value it hands back is the wrap rather than
// an error from elsewhere it still owes one to.
func NewLocalCause(condition error, cause string) *LocalCause {
	return &LocalCause{condition: condition, cause: cause}
}

// Error is the cause alone. The sentence a caller reads is the tool's, written
// around this, so the condition's own word would only be doubled into it.
func (c *LocalCause) Error() string { return c.cause }

// Unwrap is the condition, which is what an arm's errors.Is ladder walks to.
func (c *LocalCause) Unwrap() error { return c.condition }
`
}

// goOperationEntry writes the record a call-list input reaches a subsystem as.
//
// The reader that fills it sits above the answers tree and cannot be named from
// inside it, so the record it answers is written here instead. The two members
// are the whole of what an entry carries into an operation: everything else the
// caller wrote beside them stays in the argument map the reader read it from.
func goOperationEntry() string {
	return `
// ` + localEntryType + ` is one entry of a call-list input: the tool a caller says it
// intends to run, and the environment that call would target. Reading only
// those two off the entry is what keeps the whole argument map out of an
// operation's reach.
type ` + localEntryType + ` struct {
	Tool        string
	Environment string
}
`
}

// goSubsystemType writes the type one operation's subsystem satisfies: an
// interface carrying a method per side where the operation reads ambient state,
// and the function's own type where it reads none.
func goSubsystemType(operation *localOperation) (string, error) {
	name := localSubsystemType(operation.member())

	var out strings.Builder

	out.Grow(len(operation.sides()) * goAnswerShapeBudget)
	out.WriteString("\n// ")
	out.WriteString(name)
	out.WriteString(" is what serves ")
	out.WriteString(operation.member())
	out.WriteString(".\n")

	if !operation.stateful() {
		// A stateless operation runs one way, which checkGeneratedOperation
		// refuses anything else for, so the one side is the whole type.
		signature, err := goSubsystemSignature(operation, &operation.sides()[0])
		if err != nil {
			return "", err
		}

		out.WriteString("type ")
		out.WriteString(name)
		out.WriteString(" func")
		out.WriteString(signature)
		out.WriteString("\n")

		return out.String(), nil
	}

	out.WriteString("type ")
	out.WriteString(name)
	out.WriteString(" interface {\n")

	for _, side := range operation.sides() {
		signature, err := goSubsystemSignature(operation, &side)
		if err != nil {
			return "", err
		}

		method := localSubsystemMethod(languageGo, operation.member(), side.direction)

		out.WriteString("\t// ")
		out.WriteString(method)
		out.WriteString(" is ")
		out.WriteString(localSideProse(&side))
		out.WriteString(".\n")
		out.WriteString("\t")
		out.WriteString(method)
		out.WriteString(signature)
		out.WriteString("\n")
	}

	out.WriteString("}\n")

	return out.String(), nil
}

// localSideProse is how one side names itself in a comment: the operation
// itself where it runs one way, and the direction it runs where it runs two.
func localSideProse(side *localOperationSide) string {
	if side.direction == linodev1.LocalDirection_LOCAL_DIRECTION_UNSPECIFIED {
		return "the operation itself"
	}

	return "the operation running " + side.direction.String()
}

// localSideClause is what a sentence naming the operation adds for one side,
// empty where it runs one way. Kept short because the Python arm writes it into
// a docstring the column budget holds.
func localSideClause(side *localOperationSide) string {
	if side.direction == linodev1.LocalDirection_LOCAL_DIRECTION_UNSPECIFIED {
		return ""
	}

	return " for " + side.direction.String()
}

// goAmbientParameters is the readings one operation declares as this language
// takes them, ahead of everything the call carries.
func goAmbientParameters(operation *localOperation) []goParameter {
	taken := localAmbientParameters(languageGo, operation)
	parts := make([]goParameter, 0, len(taken))

	for _, reading := range taken {
		parts = append(parts, goParameter{name: reading.name, carried: reading.carried})
	}

	return parts
}

// goOperationImports is what the emitted trees take for the readings and the
// input types the generated operations render.
func goOperationImports(operations []localOperation) []string {
	paths := make(map[string]bool, len(operations))

	for index := range operations {
		operation := &operations[index]
		for _, reading := range localAmbientParameters(languageGo, operation) {
			paths[reading.module] = true
		}

		for _, input := range localOperationInputs(operation) {
			if input.kind == linodev1.LocalInputKind_LOCAL_INPUT_KIND_TIMESTAMP {
				paths[importTime] = true
			}
		}
	}

	wanted := make([]string, 0, len(paths))
	for path := range paths {
		wanted = append(wanted, path)
	}

	sort.Strings(wanted)

	return wanted
}

// goSubsystemSignature is one side's parameter list and its returns: the
// readings the operation declares, then the declared inputs in call order,
// answering the shape that side fills, and an error beside it only where the
// operation declares a condition to report.
func goSubsystemSignature(operation *localOperation, side *localOperationSide) (string, error) {
	parts := goAmbientParameters(operation)

	for _, input := range localOperationInputs(operation) {
		carried, err := goOperationInput(operation.call, &input, goOperationInputTypes(), "")
		if err != nil {
			return "", err
		}

		parts = append(parts, goParameter{name: goAnswerParameter(input.name), carried: carried})
	}

	shape, err := goOperationShape(side)
	if err != nil {
		return "", err
	}

	answered := "*" + shape
	if len(operation.reports()) > 0 {
		answered = "(*" + shape + ", error)"
	}

	return "(" + goJoinParameters(parts) + ") " + answered, nil
}

// goParameter is one emitted parameter: the local it arrives under and the
// type it is carried as.
type goParameter struct {
	name    string
	carried string
}

// goJoinParameters writes a parameter list with adjacent members of one type
// sharing that type, which is the form the formatter settles them into.
func goJoinParameters(parts []goParameter) string {
	written := make([]string, 0, len(parts))
	names := make([]string, 0, len(parts))

	var carried string

	for _, part := range parts {
		if part.carried != carried && len(names) > 0 {
			written = append(written, strings.Join(names, ", ")+" "+carried)
			names = names[:0]
		}

		carried = part.carried

		names = append(names, part.name)
	}

	if len(names) > 0 {
		written = append(written, strings.Join(names, ", ")+" "+carried)
	}

	return strings.Join(written, ", ")
}

// goOperationShape is the answer type one side fills.
func goOperationShape(side *localOperationSide) (string, error) {
	shape, err := lookupGoMessage(side.answers.FullName())
	if err != nil {
		return "", err
	}

	return shape.TypeName, nil
}

// renderGoOperations writes the arm behind every generated operation.
func renderGoOperations(operations []localOperation) (emittedFile, error) {
	out := newSource()

	out.need(importTools, importGenlocal)
	out.need(goOperationImports(operations)...)
	out.writef("// %s", strings.Join(operationsWritten(), "\n// "))
	out.writef("")

	for index := range operations {
		operation := &operations[index]
		for _, side := range operation.sides() {
			if err := emitGoOperation(out, operation, &side); err != nil {
				return emittedFile{}, err
			}
		}
	}

	formatted, err := formatSource(out.render(generatedPackage), goOperationsFile)
	if err != nil {
		return emittedFile{}, err
	}

	return emittedFile{Name: goOperationsFile, Text: formatted}, nil
}

// emitGoOperation writes one side's arm: call the subsystem, word whichever
// condition it reported, project the answer.
func emitGoOperation(out *source, operation *localOperation, side *localOperationSide) error {
	if err := checkGoOperationRendering(operation, goOperationInputTypes()); err != nil {
		return err
	}

	shape, err := goOperationShape(side)
	if err != nil {
		return err
	}

	arm := localArmSpelling(languageGo, operation.member(), side.direction)
	project := answersPackage + "." + goAnswerProject(shape)

	out.writef("// %s is the arm %s declares%s.",
		arm, operation.member(), localSideClause(side))
	out.writef("func %s(%s) tools.LocalOutcome {", arm, goOperationParameters(operation))

	call := goSubsystemCall(operation, side)

	if len(operation.reports()) == 0 {
		out.writef("\treturn tools.LocalAnswered(%s(%s))", project, call)
		out.writef("}")
		out.writef("")

		return nil
	}

	out.need(importErrors)
	out.writef("\tanswer, err := %s", call)
	out.writef("")
	emitGoOperationLadder(out, operation)
	out.writef("\treturn tools.LocalAnswered(%s(answer))", project)
	out.writef("}")
	out.writef("")

	return nil
}

// emitGoOperationLadder writes the conditions the operation declares, in
// declaration order, then the one it did not.
//
// The last arm is what keeps a failure the declaration does not name from
// reaching the projection: the answer beside it is nothing, and projecting
// nothing would read a value the subsystem never built.
func emitGoOperationLadder(out *source, operation *localOperation) {
	out.writef("\tswitch {")

	for _, guard := range operation.reports() {
		out.writef("\tcase errors.Is(err, %s.%s):",
			answersPackage, localGuardSentinel(languageGo, guard))
		out.writef("\t\treturn tools.LocalRefused(tools.%s, err.Error())",
			goLocalRefusalName(guard))
	}

	out.writef("\tcase err != nil:")
	out.writef("\t\treturn tools.LocalUnreported(err.Error())")
	out.writef("\t}")
	out.writef("")
}

// goOperationParameters is the arm's own parameter list: the readings the
// operation declares, the subsystem, then one local per declared input in the
// operation's call order. The handler hands them over in that same order.
func goOperationParameters(operation *localOperation) string {
	parts := append(goAmbientParameters(operation), goParameter{
		name:    goSubsystemLocal,
		carried: answersPackage + "." + localSubsystemType(operation.member()),
	})

	for _, input := range localOperationInputs(operation) {
		// The rendering already refused an untyped input, so the lookup here
		// answers the same row that check read.
		parts = append(parts, goParameter{
			name: goAnswerParameter(input.name),
			carried: goOperationInputSpelling(goOperationInputTypes()[localReaderKey{
				kind: input.kind, optional: input.optional,
			}], answersPackage),
		})
	}

	return goJoinParameters(parts)
}

// goSubsystemCall is the call one side's arm makes: the subsystem's method for
// that side where the operation reads ambient state, and the subsystem itself
// where it does not.
func goSubsystemCall(operation *localOperation, side *localOperationSide) string {
	ambient := goAmbientParameters(operation)
	arguments := make([]string, 0, len(operation.arm.declared.GetInput())+len(ambient))

	for _, reading := range ambient {
		arguments = append(arguments, reading.name)
	}

	for _, input := range localOperationInputs(operation) {
		arguments = append(arguments, goAnswerParameter(input.name))
	}

	called := goSubsystemLocal
	if operation.stateful() {
		called += "." + localSubsystemMethod(languageGo, operation.member(), side.direction)
	}

	return called + "(" + strings.Join(arguments, ", ") + ")"
}
