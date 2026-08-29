package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The Python arm of a generated operation. It writes the same three things the
// Go arm does, through this language's own idioms: an exception tree for the
// failure vocabulary, a Protocol or a callable alias for the subsystem type,
// and a try/except arm in place of the errors.Is ladder.
//
// It writes one thing the Go arm does not. Python has no compile step, so a
// subsystem missing the method its arm calls is an AttributeError at call time
// rather than a build failure. The conformance module walks the whole set once
// and names each gap, which is what a hand-written test then reads.

// The modules the emitted Python operations sit in and read from.
const (
	pyAnswersPackage = "linodemcp.genlocal"
	// pyOperationsModule is where each language's stateless operations live.
	// One module per language rather than a row per operation: a row per
	// operation is the naming defect the arm table already shed.
	pyOperationsModule = "linodemcp.tools.operations"
	// pyBuilderStateClass is the type the builder state is served through,
	// which the conformance walk reads the methods off.
	pyBuilderStateClass = "BuilderState"
)

// The two halves of the operations module's own name, which the conformance
// walk imports as a module rather than by member: it reads the functions off
// whatever serves them, and a module is what serves a stateless operation.
const (
	pyOperationsPackage = "linodemcp.tools"
	pyOperationsName    = "operations"
)

// The two files the Python operations arm writes.
const (
	pyOperationsFile         = localOperationsFile + pySuffix
	pyConformanceContentFile = "conformance" + pySuffix
)

// pySubsystemLocal is the name every emitted arm takes its subsystem under, the
// Go arm's own word so the two trees read alike.
const pySubsystemLocal = goSubsystemLocal

// pyOperationInputTypes is what each declared input kind is annotated as, and
// the optional form where one exists.
func pyOperationInputTypes() map[localReaderKey]string {
	return map[localReaderKey]string{
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_STRING}:                      typeWordStr,
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_STRING_LIST}:                 "list[" + typeWordStr + "]",
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_STRING_LIST, optional: true}: "list[" + typeWordStr + "] | None",
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_INT}:                         typeWordInt,
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_BOOL}:                        typeWordBool,
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_BOOL, optional: true}:        typeWordBool + " | None",
		// A bound the call left out reads as None here where Go reads it as the
		// zero time, so the annotation says so rather than the reader lying.
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_TIMESTAMP}: pyTimestampType,
		{kind: linodev1.LocalInputKind_LOCAL_INPUT_KIND_CALL_LIST}: "list[" + localEntryType + "]",
	}
}

// pyOperationInput is the annotation one declared input reaches the subsystem
// under. A kind with no row is refused for the reason the Go arm refuses it.
func pyOperationInput(
	call linodev1.LocalCall, input *localOperationInput, types map[localReaderKey]string,
) (string, error) {
	carried, typed := types[localReaderKey{
		kind: input.kind, optional: input.optional,
	}]
	if !typed {
		return "", fmt.Errorf("%w: %s takes %s as %s",
			errLocalArmInputUntyped, call, input.name, input.kind)
	}

	return carried, nil
}

// pyOperationSurface writes the failure vocabulary and the subsystem type each
// generated operation is served through, which land in the answers module
// beside the shapes they answer with.
func pyOperationSurface(operations []localOperation) ([]string, error) {
	if len(operations) == 0 {
		return nil, nil
	}

	lines := make([]string, 0, len(operations)*shapeLineBudget)

	guards := generatedGuards(operations)
	if len(guards) > 0 {
		lines = append(lines, pyOperationBaseClass()...)
	}

	for _, guard := range guards {
		lines = append(lines, pyGuardClass(guard)...)
	}

	if localTakesCallList(operations) {
		lines = append(lines, pyOperationEntry()...)
	}

	lines = append(lines, pyVerdictSurface(operations)...)

	for index := range operations {
		text, err := pySubsystemType(&operations[index])
		if err != nil {
			return nil, err
		}

		lines = append(lines, text...)
	}

	return lines, nil
}

// pyOperationBaseClass writes the class every generated condition derives from.
func pyOperationBaseClass() []string {
	return []string{
		"",
		"",
		"class " + pyOperationBase + "(Exception):",
		`    """A condition a local operation reports.`,
		"",
		"    Every generated condition derives from this one, which is what lets an",
		"    arm catch the whole of an operation's vocabulary and leave anything",
		"    outside it to the interpreter.",
		pyDocClose,
	}
}

// pyGuardClass writes the exception one condition is raised as, carrying the
// same neutral text the Go value answers so a sentence naming the cause reads
// alike in both languages.
func pyGuardClass(guard linodev1.LocalGuard) []string {
	name := localGuardSentinel(languagePython, guard)

	return []string{
		"",
		"",
		"class " + name + "(" + pyOperationBase + "):",
		`    """The condition ` + guard.String() + ` names."""`,
		"",
		"    def __init__(self, cause: str = " + pyQuote(localGuardCause(guard)) + ") -> None:",
		`        """Report the condition, carrying the cause a sentence may name."""`,
		"        super().__init__(cause)",
	}
}

// pyOperationEntry writes the record a call-list input reaches a subsystem as.
//
// The reader that fills it sits above the answers module and cannot be named
// from inside it, so the record it answers is written here instead.
func pyOperationEntry() []string {
	return []string{
		"",
		"",
		"@dataclass(frozen=True)",
		"class " + localEntryType + ":",
		`    """One entry of a call-list input.`,
		"",
		"    A tool a caller says it intends to run, and the environment that call",
		"    would target. Reading only those two off the entry is what keeps the",
		"    whole argument map out of an operation's reach.",
		pyDocClose,
		"",
		"    tool: " + typeWordStr,
		"    environment: " + typeWordStr,
	}
}

// pySubsystemType writes the type one operation's subsystem satisfies: a
// Protocol carrying a method per side where the operation reads ambient state,
// and the callable's own alias where it reads none.
func pySubsystemType(operation *localOperation) ([]string, error) {
	parts, err := pySubsystemParameters(operation)
	if err != nil {
		return nil, err
	}

	name := localSubsystemType(operation.member())

	if !operation.stateful() {
		// A stateless operation runs one way, which checkGeneratedOperation
		// refuses anything else for, so the one side is the whole alias.
		shape, shapeErr := goOperationShape(&operation.sides()[0])
		if shapeErr != nil {
			return nil, shapeErr
		}

		carried := make([]string, 0, len(parts))
		for _, part := range parts {
			carried = append(carried, part.annotation)
		}

		return []string{
			"",
			"",
			"type " + name + " = Callable[[" + strings.Join(carried, ", ") + "], " + shape + "]",
		}, nil
	}

	return pySubsystemProtocol(operation, name, parts)
}

// pySubsystemProtocol writes the Protocol one stateful operation's subsystem
// satisfies, and the sentence saying which conditions its methods raise.
func pySubsystemProtocol(
	operation *localOperation, name string, parts []pySubsystemParameter,
) ([]string, error) {
	taken := make([]string, 0, len(parts)+1)
	taken = append(taken, "self")

	for _, part := range parts {
		taken = append(taken, part.name+": "+part.annotation)
	}

	lines := []string{"", "", "class " + name + "(Protocol):"}
	lines = append(lines, pySubsystemProse(operation)...)

	for _, side := range operation.sides() {
		shape, err := goOperationShape(&side)
		if err != nil {
			return nil, err
		}

		method := localSubsystemMethod(languagePython, operation.member(), side.direction)
		lines = append(lines, "",
			"    def "+method+"("+strings.Join(taken, ", ")+") -> "+shape+": ...")
	}

	return lines, nil
}

// pySubsystemProse is the Protocol's own docstring. The conditions the methods
// report go here rather than on each one, because a Protocol method carries the
// ellipsis body the type checker asks for and a docstring beside it is the
// placeholder the linter then removes.
func pySubsystemProse(operation *localOperation) []string {
	opening := `    """What serves ` + operation.member() + `.`

	raised := pySubsystemRaises(operation)
	if raised == "" {
		return []string{opening + `"""`}
	}

	lines := []string{opening, ""}
	lines = append(lines, pyProseWrapped("Every method raises "+raised+".")...)

	return append(lines, pyDocClose)
}

// pyDocstringIndent is where a class docstring's own prose starts, which is one
// indent level inside the class body.
const pyDocstringIndent = 4

// pySubsystemRaises is the conditions one operation's method reports, joined
// the way the docstring names them, empty where it reports none.
func pySubsystemRaises(operation *localOperation) string {
	raised := make([]string, 0, len(operation.reports()))
	for _, guard := range operation.reports() {
		raised = append(raised, localGuardSentinel(languagePython, guard))
	}

	return strings.Join(raised, " or ")
}

// pySubsystemParameter is one declared input as Python takes it.
type pySubsystemParameter struct {
	name       string
	annotation string
}

// pyAmbientParameters is the readings one operation declares as this language
// takes them, ahead of everything the call carries. A cancellation is not among
// them: this language renders that reading in nothing.
func pyAmbientParameters(operation *localOperation) []pySubsystemParameter {
	taken := localAmbientParameters(languagePython, operation)
	parts := make([]pySubsystemParameter, 0, len(taken))

	for _, reading := range taken {
		parts = append(parts, pySubsystemParameter{
			name: reading.name, annotation: reading.carried,
		})
	}

	return parts
}

// pyTimestampType is what a declared bound is annotated as, which is what its
// reader answers rather than what the contract's kind is named for.
const pyTimestampType = "datetime | None"

// pyOperationTypeImports is the modules the emitted surface names in
// annotations alone, each with the members it takes, sorted the way the
// linter's import block reads them.
func pyOperationTypeImports(operations []localOperation) map[string][]string {
	wanted := make(map[string]map[string]bool, len(operations))

	for index := range operations {
		operation := &operations[index]
		for _, reading := range localAmbientParameters(languagePython, operation) {
			pyRecordImport(wanted, reading.module, reading.carried)
		}

		for _, input := range localOperationInputs(operation) {
			if input.kind == linodev1.LocalInputKind_LOCAL_INPUT_KIND_TIMESTAMP {
				pyRecordImport(wanted, "datetime", "datetime")
			}
		}
	}

	taken := make(map[string][]string, len(wanted))
	for module, names := range wanted {
		taken[module] = pySortedNames(names)
	}

	return taken
}

// pyRecordImport files one name under the module it comes from.
func pyRecordImport(wanted map[string]map[string]bool, module, name string) {
	if wanted[module] == nil {
		wanted[module] = map[string]bool{}
	}

	wanted[module][name] = true
}

// pySubsystemParameters is the whole call list one operation's method takes:
// the readings it declares, then its declared inputs in call order.
func pySubsystemParameters(operation *localOperation) ([]pySubsystemParameter, error) {
	inputs, err := pyInputParameters(operation, pyOperationInputTypes())
	if err != nil {
		return nil, err
	}

	return append(pyAmbientParameters(operation), inputs...), nil
}

// pyInputParameters is one operation's declared inputs alone, which is what the
// arm takes beyond the subsystem it is handed.
func pyInputParameters(
	operation *localOperation, types map[localReaderKey]string,
) ([]pySubsystemParameter, error) {
	parts := make([]pySubsystemParameter, 0, len(operation.arm.declared.GetInput()))

	for _, input := range localOperationInputs(operation) {
		carried, err := pyOperationInput(operation.call, &input, types)
		if err != nil {
			return nil, err
		}

		parts = append(parts, pySubsystemParameter{
			name: pyLocalRead(input.name), annotation: carried,
		})
	}

	return parts, nil
}

// renderPyOperations writes the arm behind every generated operation.
func renderPyOperations(operations []localOperation) ([]string, error) {
	body := make([]string, 0, len(operations)*shapeLineBudget)

	for index := range operations {
		operation := &operations[index]
		for _, side := range operation.sides() {
			arm, err := pyOperationArm(operation, &side)
			if err != nil {
				return nil, err
			}

			body = append(body, arm...)
		}
	}

	lines := []string{pyHeader, `"""` + operationsWritten()[0]}
	lines = append(lines, operationsWritten()[1:]...)
	lines = append(lines, `"""`, "", "from __future__ import annotations", "")
	lines = append(lines, pyOperationImports(operations)...)

	return append(lines, body...), nil
}

// pyOperationArm writes one side's arm: call the subsystem, word whichever
// condition it reported, project the answer.
func pyOperationArm(operation *localOperation, side *localOperationSide) ([]string, error) {
	shape, err := goOperationShape(side)
	if err != nil {
		return nil, err
	}

	parts, err := pySubsystemParameters(operation)
	if err != nil {
		return nil, err
	}

	inputs, err := pyInputParameters(operation, pyOperationInputTypes())
	if err != nil {
		return nil, err
	}

	// The readings, the subsystem, then the inputs: the same order the handler
	// hands them over in, and the same order the Go arm takes them.
	taken := make([]string, 0, len(parts)+1)
	for _, part := range pyAmbientParameters(operation) {
		taken = append(taken, part.name+": "+part.annotation)
	}

	taken = append(taken, pySubsystemLocal+": "+localSubsystemType(operation.member()))

	for _, part := range inputs {
		taken = append(taken, part.name+": "+part.annotation)
	}

	lines := make([]string, 0, shapeLineBudget)
	lines = append(lines,
		"",
		"",
		"def "+localArmSpelling(languagePython, operation.member(), side.direction)+
			"("+strings.Join(taken, ", ")+") -> LocalOutcome:",
		`    """The arm `+operation.member()+` declares`+localSideClause(side)+`."""`)

	return append(lines, pyOperationBody(operation, side, shape, parts)...), nil
}

// pyOperationBody is the arm's own steps, which are the call alone where the
// operation reports nothing and the call under an except ladder where it does.
func pyOperationBody(
	operation *localOperation, side *localOperationSide,
	shape string, parts []pySubsystemParameter,
) []string {
	project := pyAnswerProject(shape)
	call := pySubsystemCall(operation, side, parts)

	if len(operation.reports()) == 0 {
		return []string{"    return local_answered(" + project + "(" + call + "))"}
	}

	lines := []string{"    try:", "        answer = " + call}

	for _, guard := range operation.reports() {
		lines = append(lines,
			"    except "+localGuardSentinel(languagePython, guard)+" as exc:",
			"        return local_refused(LocalRefusal."+localGuardMembers(guard)+", str(exc))")
	}

	lines = append(lines,
		"    except "+pyOperationBase+" as exc:",
		"        return local_unreported(str(exc))",
		"",
		"    return local_answered("+project+"(answer))")

	return lines
}

// pySubsystemCall is the call one side's arm makes: the subsystem's method for
// that side where the operation reads ambient state, and the subsystem itself
// where it does not.
func pySubsystemCall(
	operation *localOperation, side *localOperationSide, parts []pySubsystemParameter,
) string {
	arguments := make([]string, 0, len(parts))
	for _, part := range parts {
		arguments = append(arguments, part.name)
	}

	called := pySubsystemLocal
	if operation.stateful() {
		called += "." + localSubsystemMethod(languagePython, operation.member(), side.direction)
	}

	return called + "(" + strings.Join(arguments, ", ") + ")"
}

// pyOperationImports is the arms module's import block: the projections and the
// conditions it names at run time, and the subsystem types and the outcome it
// names in annotations alone, which the linter holds to the type-checking
// block.
func pyOperationImports(operations []localOperation) []string {
	answers := make(map[string]bool, len(operations)*2)
	engine := map[string]bool{"local_answered": true}
	hinted := make(map[string]bool, len(operations))

	for index := range operations {
		operation := &operations[index]
		hinted[localSubsystemType(operation.member())] = true

		if localTakesCallList([]localOperation{*operation}) {
			hinted[localEntryType] = true
		}

		for _, side := range operation.sides() {
			shape, err := goOperationShape(&side)
			if err == nil {
				answers[pyAnswerProject(shape)] = true
			}
		}

		if len(operation.reports()) == 0 {
			continue
		}

		answers[pyOperationBase] = true
		engine["LocalRefusal"] = true
		engine["local_refused"] = true
		engine["local_unreported"] = true

		for _, guard := range operation.reports() {
			answers[localGuardSentinel(languagePython, guard)] = true
		}
	}

	return pyOperationImportBlock(answers, engine, hinted, pyOperationTypeImports(operations))
}

// pyOperationImportBlock assembles the module's import statements from the names
// each one carries, with everything named in an annotation alone under the
// type-checking block the linter holds those to.
func pyOperationImportBlock(answers, engine, hinted map[string]bool, typed map[string][]string) []string {
	lines := []string{"from typing import TYPE_CHECKING", ""}
	lines = append(lines, pyFromImport(pyAnswersPackage, pySortedNames(answers))...)
	lines = append(lines, pyFromImport(pyLocalModule, pySortedNames(engine))...)
	lines = append(lines, "", "if TYPE_CHECKING:")
	lines = append(lines, pyTypedImportLines(typed, "    ")...)

	for _, line := range pyFromImport(pyAnswersPackage, pySortedNames(hinted)) {
		lines = append(lines, "    "+line)
	}

	return append(lines,
		"    from "+pyLocalModule+" import LocalOutcome",
		"")
}

// pyTypedImportLines writes the annotation-only imports at one indent, standard
// library first and the rest after a blank line, which is the order and the
// spacing the linter's own import sort settles them into.
func pyTypedImportLines(typed map[string][]string, indent string) []string {
	standard, external := pyImportGroups(typed)
	lines := make([]string, 0, len(typed)+1)

	for _, module := range standard {
		lines = append(lines, indent+"from "+module+" import "+strings.Join(typed[module], ", "))
	}

	if len(standard) > 0 && len(external) > 0 {
		lines = append(lines, "")
	}

	for _, module := range external {
		lines = append(lines, indent+"from "+module+" import "+strings.Join(typed[module], ", "))
	}

	return lines
}

// pyImportGroups splits the annotation-only modules into the two groups the
// import sort keeps: this language's own library, then everything else. A
// module naming the distribution is the repo's own, which is the only other
// kind an emitted tree reaches.
func pyImportGroups(typed map[string][]string) ([]string, []string) {
	standard := make([]string, 0, len(typed))
	external := make([]string, 0, len(typed))

	for _, module := range slices.Sorted(maps.Keys(typed)) {
		if strings.HasPrefix(module, pyDistribution+".") {
			external = append(external, module)

			continue
		}

		standard = append(standard, module)
	}

	return standard, external
}

// pyDistribution is the package every module this repo owns sits under.
const pyDistribution = "linodemcp"

// pyFromImport writes one from-import, parenthesized so the formatter settles
// the wrapping rather than this arm guessing at the column.
func pyFromImport(module string, names []string) []string {
	lines := make([]string, 0, len(names)+2)
	lines = append(lines, "from "+module+" import (")

	for _, name := range names {
		lines = append(lines, "    "+name+",")
	}

	return append(lines, ")")
}

// renderPyConformance writes the module that holds every subsystem to the
// surface its arm calls.
//
// The Go build makes this check at the call site and needs no file. Python has
// no compile step, so the same gap is a runtime AttributeError, and this is
// what turns it into a named complaint a test can read.
func renderPyConformance(operations []localOperation) []string {
	lines := make([]string, 0, len(operations)+shapeLineBudget)
	lines = append(lines,
		pyHeader,
		`"""Every local operation's subsystem held to the surface its arm calls.`,
		"",
		"Python has no compile step, so a missing or misshapen subsystem method is",
		"an AttributeError at call time rather than a build failure. This walks the",
		"whole set once and names each gap, which is what the Go build already does",
		"at the call site.",
		`"""`,
		"",
		"from __future__ import annotations",
		"",
		"import inspect",
		"",
		"from "+pyOperationsPackage+" import "+pyOperationsName,
		"from "+pyBuilderModule+" import "+pyBuilderStateClass,
		"",
		"# Every declared operation as the walk reads it: the member, what serves",
		"# it, the name it is served under, and the inputs it declares in call",
		"# order.",
		"_OPERATIONS: tuple[tuple[str, object, str, tuple[str, ...]], ...] = (",
	)

	for index := range operations {
		operation := &operations[index]
		for _, side := range operation.sides() {
			lines = append(lines, pyConformanceRow(operation, &side))
		}
	}

	lines = append(lines, ")", "")

	return append(lines, pyConformanceWalk()...)
}

// pyConformanceRow is one side of one operation as the walk reads it.
func pyConformanceRow(operation *localOperation, side *localOperationSide) string {
	served := pyBuilderStateClass
	if !operation.stateful() {
		served = pyOperationsName
	}

	return "    (" + pyQuote(operation.member()) + ", " + served + ", " +
		pyQuote(localSubsystemMethod(languagePython, operation.member(), side.direction)) +
		", " + pyDeclaredInputs(operation) + "),"
}

// pyDeclaredInputs is the whole call list one operation's method takes, as a
// tuple literal: the readings this language renders, then the declared inputs.
// A single name takes the trailing comma a one-member tuple needs, and a longer
// list takes none, since a trailing comma is what makes the formatter break a
// short literal across lines.
func pyDeclaredInputs(operation *localOperation) string {
	declared := make([]string, 0, len(operation.arm.declared.GetInput()))
	for _, reading := range pyAmbientParameters(operation) {
		declared = append(declared, pyQuote(reading.name))
	}

	for _, input := range localOperationInputs(operation) {
		declared = append(declared, pyQuote(pyLocalRead(input.name)))
	}

	if len(declared) == 1 {
		return "(" + declared[0] + ",)"
	}

	return "(" + strings.Join(declared, ", ") + ")"
}

// pyConformanceWalk is the check itself, which is the same text on every run
// because the operations it reads are the table above.
func pyConformanceWalk() []string {
	return []string{
		"",
		"def check_local_operations() -> list[str]:",
		`    """Each gap between a declared operation and what serves it."""`,
		"    gaps: list[str] = []",
		"    for member, served, method, declared in _OPERATIONS:",
		"        gaps.extend(_check_operation(member, served, method, declared))",
		"    return gaps",
		"",
		"",
		"def _check_operation(",
		"    member: str, served: object, method: str, declared: tuple[str, ...]",
		") -> list[str]:",
		`    """One operation's gap, or nothing where its subsystem matches."""`,
		"    where = f\"{member}: {getattr(served, '__name__', served)}\"",
		"    found = getattr(served, method, None)",
		"    if found is None:",
		`        return [f"{where} defines no {method}"]`,
		"    if not callable(found):",
		`        return [f"{where}.{method} is not callable"]`,
		"    taken = tuple(",
		"        name",
		"        for name in inspect.signature(found).parameters",
		`        if name != "self"`,
		"    )",
		"    if taken != declared:",
		`        return [f"{where}.{method} takes {taken}, declared {declared}"]`,
		"    return []",
	}
}
