package main

import (
	"maps"
	"slices"
	"sort"
	"strings"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The Python arm of a local answer. It renders the same four steps the Go arm
// does, through this language's own names: the ambient state, one read per
// operation input, the operation, then the sentence for whichever condition it
// reported.

// The modules a rendered local answer reads its names from.
const (
	pyLocalModule   = "linodemcp.tools.local_answer"
	pyBuilderModule = "linodemcp.tools.builderstate"
)

// The two names a builder-backed answer takes from the state module.
const (
	pyBuilderState        = "builder_state_from_context"
	pyBuilderUnconfigured = "BUILDER_UNCONFIGURED"
)

// pyHandlerNames is the names the emitted handler already holds: its own
// parameters, the locals a local answer writes, and the module's imports. An
// argument spelling one is renamed, the way the Go arm renames a read that
// would shadow a package.
//
// Keywords and builtins are not listed here: pyReservedNames already carries
// that whole vocabulary, and a local spelled like one is refused by the lint
// gate rather than by the compiler.
func pyHandlerNames() []string {
	return []string{
		"Any", "Capability", "Config", "LocalRefusal",
		"TextContent", "Tool", "arguments", goConfigLocal, "check_constraints",
		"error_response", "local_response", "message", "outcome", "schema",
		previewStateLocal,
	}
}

// pyLocalRead is the local one bound argument is read into, renamed where the
// ordinary spelling would take a name the handler already holds or one the
// language already owns.
func pyLocalRead(argument string) string {
	if slices.Contains(pyHandlerNames(), argument) || pyReserved(argument) {
		return argument + "_argument"
	}

	return argument
}

// pyLocalAnswer writes the body of a meta tool whose answer is a declared local
// operation.
func pyLocalAnswer(tool *pyTool) []string {
	answer := tool.c.LocalAnswer
	lines := make([]string, 0, 16)

	if answer.Arm.declared.GetRequires() != linodev1.LocalState_LOCAL_STATE_UNSPECIFIED {
		lines = append(lines,
			"    "+previewStateLocal+" = "+pyBuilderState+"()",
			"    if "+previewStateLocal+" is None:",
			"        return error_response("+pyBuilderUnconfigured+")",
			"",
		)
	}

	lines = append(lines, pyLocalReads(tool)...)
	lines = append(lines,
		"    outcome = "+localArmSpelling(languagePython, answer.Call.String(), answer.Direction)+
			"("+strings.Join(pyLocalCallArguments(answer), ", ")+")",
		"",
	)

	lines = append(lines, pyLocalLadder(tool)...)

	return append(lines,
		"    return local_response(",
		"        "+pyQuote(string(tool.c.ResponseGo.FullName))+",",
		"        outcome.body,",
		"    )",
	)
}

// pyLocalReads writes one read per operation input, and the reader's own
// refusal where its type can answer one.
func pyLocalReads(tool *pyTool) []string {
	answer := tool.c.LocalAnswer
	readers := localReaders()
	lines := make([]string, 0, len(answer.Arm.declared.GetInput()))

	for index, input := range answer.Arm.declared.GetInput() {
		argument := answer.Bound[index]
		reader := readers[localReaderKey{kind: input.GetKind(), optional: input.GetOptional()}]
		call := reader.pyName + "(arguments, " + pyQuote(argument) + ")"
		local := pyLocalRead(argument)

		if !reader.refuses {
			lines = append(lines, "    "+local+" = "+call)

			continue
		}

		cause := local + "_cause"
		sentence := pyLocalSentence(answer.ReaderWording[argument], cause)

		lines = append(lines,
			"    "+local+", "+cause+" = "+call,
			"    if "+cause+":",
			"        return error_response("+sentence+")",
			"",
		)
	}

	if len(answer.Arm.declared.GetInput()) > 0 && lines[len(lines)-1] != "" {
		lines = append(lines, "")
	}

	return lines
}

// pyLocalLadder writes the conditions the operation itself reports, in
// declaration order, each answering its own sentence.
func pyLocalLadder(tool *pyTool) []string {
	lines := make([]string, 0, len(tool.c.LocalAnswer.Ladder))

	for _, row := range tool.c.LocalAnswer.Ladder {
		if row.GetGuard() == linodev1.LocalGuard_LOCAL_GUARD_ARGUMENT_UNUSABLE {
			continue
		}

		sentence := pyLocalSentence(row.GetMessage(), "outcome.cause")

		lines = append(lines,
			"    if outcome.refusal is LocalRefusal."+localGuardMembers(row.GetGuard())+":",
			"        return error_response("+sentence+")",
			"",
		)
	}

	return lines
}

// pyLocalCallArguments is the operation's call list: the ambient readings it
// declared, then one local per input in the operation's own order.
//
// A declared cancellation is not among them. A Python tool handler takes the
// arguments and the configuration and nothing else, so this language's
// operations carry none to pass on, and passing one would name a local the
// handler never holds. That is the reading rendered in nothing, which is why
// LocalAmbient names the reading rather than a per-language parameter.
//
// The rest come off the vocabulary rather than being named one by one, since a
// member added to it would otherwise be dropped here alone.
func pyLocalCallArguments(answer *localAnswer) []string {
	parts := make([]string, 0, len(answer.Arm.declared.GetInput())+4)

	for _, reading := range localArmAmbientParameters(languagePython, &answer.Arm) {
		parts = append(parts, reading.handed)
	}

	if subsystem := pyLocalSubsystemArgument(answer); subsystem != "" {
		parts = append(parts, subsystem)
	}

	for _, argument := range answer.Bound {
		parts = append(parts, pyLocalRead(argument))
	}

	return parts
}

// pyLocalSubsystemArgument is what the handler hands the arm in the subsystem
// slot: the ambient state where the operation declares some, and the engine's
// own function for the operation where it declares none.
func pyLocalSubsystemArgument(answer *localAnswer) string {
	if answer.Arm.declared.GetRequires() != linodev1.LocalState_LOCAL_STATE_UNSPECIFIED {
		return previewStateLocal
	}

	return localSubsystemMethod(languagePython, answer.Call.String(), answer.Direction)
}

// pyLocalSentence renders one declared sentence as the Python expression
// producing it: the text itself where it names nothing, and the f-string
// filling it otherwise.
func pyLocalSentence(sentence, cause string) string {
	body, filled := pyLocalFilled(sentence, pyRefusalRead(cause))

	// The raw sentence rather than the f-string body: an f-string doubles a
	// brace to stand for itself, and a plain literal carries it as written.
	if !filled {
		return pyQuote(sentence)
	}

	return `f"` + body + `"`
}

// pyRefusalRead is how a refusal sentence fills one placeholder: the reported
// cause where the sentence named it, and the bound local otherwise.
func pyRefusalRead(cause string) func(string) string {
	return func(named string) string {
		if named == localCausePlaceholder {
			return cause
		}

		return pyLocalRead(named)
	}
}

// pyLocalFilled rewrites a sentence's placeholders as f-string reads, each
// resolved through read. A quoted placeholder goes through the engine's own
// quoting so the answer matches the %q the Go arm renders.
func pyLocalFilled(sentence string, read func(string) string) (string, bool) {
	var (
		text   strings.Builder
		filled bool
		rest   = sentence
	)

	text.Grow(len(sentence))

	for {
		open, closed := localBraces(rest)
		if open < 0 {
			text.WriteString(pyEscaped(rest))

			return text.String(), filled
		}

		named, quoted := strings.CutSuffix(rest[open+1:open+closed], localQuotedForm)
		text.WriteString(pyEscaped(rest[:open]))

		filling := read(named)
		if quoted {
			filling = "local_quoted(" + filling + ")"
		}

		text.WriteString("{")
		text.WriteString(filling)
		text.WriteString("}")

		filled = true
		rest = rest[open+closed+1:]
	}
}

// pyLocalAnswerImports is the import lines a module's local answers add: the
// engine's own names, and the state module's where any of them reads state.
func pyLocalAnswerImports(tools []*pyTool) []string {
	lines := make([]string, 0, 4)

	if state := pyBuilderImports(tools); state != "" {
		lines = append(lines, "from "+pyBuilderModule+" import "+state)
	}

	if arms := pyGeneratedArmImports(tools); arms != "" {
		lines = append(lines, "from "+pyPackage+"."+localOperationsFile+" import "+arms)
	}

	if served := pySubsystemFunctionImports(tools); served != "" {
		lines = append(lines, "from "+pyOperationsModule+" import "+served)
	}

	if local := pyLocalImports(tools); local != "" {
		lines = append(lines, "from "+pyLocalModule+" import "+local)
	}

	return append(lines, pyAmbientSeamImports(tools)...)
}

// pyAmbientSeamImports is the seam each reading a module's answers declare is
// read from, for the readings this language's handler does not already hold.
//
// One line per seam, since a reading rendered as an expression names something
// the module would otherwise not import at all.
func pyAmbientSeamImports(tools []*pyTool) []string {
	wanted := make(map[string]map[string]bool, len(tools))

	for _, tool := range tools {
		answer := tool.c.LocalAnswer
		if answer == nil {
			continue
		}

		for _, reading := range localArmAmbientParameters(languagePython, &answer.Arm) {
			if reading.seam != "" {
				pyRecordImport(wanted, reading.seam, reading.reader)
			}
		}
	}

	lines := make([]string, 0, len(wanted))
	for _, module := range slices.Sorted(maps.Keys(wanted)) {
		lines = append(lines,
			"from "+module+" import "+strings.Join(pySortedNames(wanted[module]), ", "))
	}

	return lines
}

// pyGeneratedArmImports is the arms one module's handlers take from the emitted
// operations module rather than from the engine.
func pyGeneratedArmImports(tools []*pyTool) string {
	names := make(map[string]bool, len(tools))

	for _, tool := range tools {
		answer := tool.c.LocalAnswer
		if answer == nil {
			continue
		}

		names[localArmSpelling(languagePython, answer.Call.String(), answer.Direction)] = true
	}

	return strings.Join(pySortedNames(names), ", ")
}

// pySubsystemFunctionImports is the subsystem functions one module's handlers
// hand to a generated arm, which is the operations that read no ambient state.
func pySubsystemFunctionImports(tools []*pyTool) string {
	names := make(map[string]bool, len(tools))

	for _, tool := range tools {
		answer := tool.c.LocalAnswer
		if answer == nil {
			continue
		}

		if answer.Arm.declared.GetRequires() == linodev1.LocalState_LOCAL_STATE_UNSPECIFIED {
			names[localSubsystemMethod(
				languagePython, answer.Call.String(), answer.Direction,
			)] = true
		}
	}

	return strings.Join(pySortedNames(names), ", ")
}

// pyLocalImports is the names one module's local answers take from the engine,
// sorted the way the linter's import sort settles a from-import.
func pyLocalImports(tools []*pyTool) string {
	names := make(map[string]bool, len(tools))
	readers := localReaders()

	for _, tool := range tools {
		answer := tool.c.LocalAnswer
		if answer == nil {
			continue
		}

		names["local_response"] = true

		if len(answer.Arm.declared.GetReports()) > 0 {
			names["LocalRefusal"] = true
		}

		for _, input := range answer.Arm.declared.GetInput() {
			names[readers[localReaderKey{
				kind: input.GetKind(), optional: input.GetOptional(),
			}].pyName] = true
		}

		if pyLocalQuotes(answer) {
			names["local_quoted"] = true
		}
	}

	return strings.Join(pySortedNames(names), ", ")
}

// pyLocalQuotes is whether any sentence asks for a value in quotes.
func pyLocalQuotes(answer *localAnswer) bool {
	return slices.ContainsFunc(answer.Ladder, func(row *linodev1.LocalRefusal) bool {
		return slices.ContainsFunc(localPlaceholders(row.GetMessage()),
			func(named localPlaceholder) bool { return named.quoted })
	})
}

// pyBuilderImports is the names one module's local answers take from the state
// module, empty where none of them reads ambient state.
func pyBuilderImports(tools []*pyTool) string {
	for _, tool := range tools {
		answer := tool.c.LocalAnswer
		if answer != nil &&
			answer.Arm.declared.GetRequires() != linodev1.LocalState_LOCAL_STATE_UNSPECIFIED {
			return strings.Join(pySortedNames(map[string]bool{
				pyBuilderUnconfigured: true, pyBuilderState: true,
			}), ", ")
		}
	}

	return ""
}

// pySortedNames orders a from-import's members the way the linter's own import
// sort settles them: constants first, then classes, then everything else, each
// group alphabetically. Measured against ruff rather than assumed, because a
// module whose members sort another way trips I001 on the generated tree, which
// the Python lint gate reads.
func pySortedNames(names map[string]bool) []string {
	found := make([]string, 0, len(names))
	for name := range names {
		found = append(found, name)
	}

	sort.Slice(found, func(left, right int) bool {
		leftRank, rightRank := pyNameRank(found[left]), pyNameRank(found[right])
		if leftRank != rightRank {
			return leftRank < rightRank
		}

		return found[left] < found[right]
	})

	return found
}

// The three groups the import sort settles a from-import's members into.
const (
	pyRankConstant = iota
	pyRankClass
	pyRankOther
)

// pyNameRank is which of the sort's three groups one member falls in.
func pyNameRank(name string) int {
	// The empty name lands in the first arm, so the second can read a byte.
	switch {
	case name == strings.ToUpper(name):
		return pyRankConstant
	case name[:1] == strings.ToUpper(name[:1]):
		return pyRankClass
	}

	return pyRankOther
}
