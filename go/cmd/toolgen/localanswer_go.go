package main

import (
	"slices"
	"strings"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The Go arm of a local answer: resolve the ambient state, read each of the
// operation's inputs through the reader its type names, run the operation, then
// word whichever condition it reported.
//
// Every sentence is written here rather than passed down, which is why the
// operation takes no prose and answers a condition instead. That is also what
// keeps the tool's own identity out of it: nothing in the call carries a name.

// The names a generated handler takes its two ambient readings under, spelled
// once because the handler, the emitted arm and the subsystem type all render
// them.
const (
	goConfigLocal  = "cfg"
	goContextLocal = "ctx"
)

// goHandlerNames is the identifiers the emitted handler already holds: its own
// parameters, the locals a local answer writes, and the package names the
// module imports. A read whose argument spells one of them is renamed rather
// than allowed to shadow it, because the very next line calls through the name
// it took.
func goHandlerNames() []string {
	return []string{
		goConfigLocal, "config", "context", "ctx", "fmt", "linodev1", "linode",
		"linoderoute", "mcp", "outcome", "profiles", "proto", "refusal",
		"request", previewStateLocal, "structpb", "tools", "toolschemas", "twostage",
	}
}

// goLocalRead is the local one bound argument is read into. It is the ordinary
// local spelling unless that would shadow something the handler already holds,
// which is the state linode_profile_draft_add_tools reaches by binding an
// argument named after the tools package.
func goLocalRead(argument string) string {
	local := goLocalName(argument)
	if slices.Contains(goHandlerNames(), local) {
		return local + "Argument"
	}

	return local
}

// emitLocalAnswer writes the body behind a meta factory whose answer is a
// declared local operation.
func emitLocalAnswer(out *source, tool *contract) {
	answer := tool.LocalAnswer

	out.need(importTools)

	if answer.Arm.declared.GetRequires() != linodev1.LocalState_LOCAL_STATE_UNSPECIFIED {
		out.need(importMCP)
		out.writef("\t%s, refusal := tools.BuilderStateOrRefusal(ctx)", previewStateLocal)
		out.writef("\tif refusal != nil {")
		out.writef("\t\treturn refusal, nil")
		out.writef("\t}")
		out.writef("")
	}

	emitLocalReads(out, tool)

	out.writef("\toutcome := %s(%s)",
		goLocalArmCall(answer), strings.Join(goLocalCallArguments(answer), ", "))
	out.writef("")

	emitLocalLadder(out, tool)

	// The operation names no message, so the tool's own response is written
	// here. checkLocalShape has already held the two to one shape.
	out.need(importGenpb)
	out.writef("\treturn tools.LocalResponse(&linodev1.%s{}, outcome.Body)", tool.ResponseGo.TypeName)
	out.writef("}")
	out.writef("")
}

// emitLocalReads writes one read per operation input, and the reader's own
// refusal where its type can answer one.
func emitLocalReads(out *source, tool *contract) {
	answer := tool.LocalAnswer
	readers := localReaders()

	for index, input := range answer.Arm.declared.GetInput() {
		argument := answer.Bound[index]
		reader := readers[localReaderKey{kind: input.GetKind(), optional: input.GetOptional()}]
		local := goLocalRead(argument)

		if !reader.refuses {
			out.writef("\t%s := tools.%s(request, %s)", local, reader.goName, goStringLiteral(argument))

			continue
		}

		cause := local + "Cause"
		out.writef("\t%s, %s := tools.%s(request, %s)",
			local, cause, reader.goName, goStringLiteral(argument))
		emitLocalReaderRefusal(out, tool, argument, cause)
	}

	if len(answer.Arm.declared.GetInput()) > 0 {
		out.writef("")
	}
}

// emitLocalReaderRefusal words the one refusal a reading answers for itself.
// The ladder is held to a row per refusing reader before this runs, so the
// wording is read rather than checked for again.
func emitLocalReaderRefusal(out *source, tool *contract, argument, cause string) {
	sentence := goLocalSentence(tool.LocalAnswer.ReaderWording[argument], cause)

	out.need(importMCP)
	out.writef("\tif %s != \"\" {", cause)
	out.writef("\t\treturn mcp.NewToolResultError(%s), nil", sentence)
	out.writef("\t}")
	out.writef("")

	if strings.HasPrefix(sentence, "fmt.") {
		out.need(importFmt)
	}
}

// emitLocalLadder writes the conditions the operation itself reports, in
// declaration order, each answering its own sentence.
func emitLocalLadder(out *source, tool *contract) {
	for _, row := range tool.LocalAnswer.Ladder {
		if row.GetGuard() == linodev1.LocalGuard_LOCAL_GUARD_ARGUMENT_UNUSABLE {
			continue
		}

		sentence := goLocalSentence(row.GetMessage(), "outcome.Cause")

		out.need(importMCP)
		out.writef("\tif outcome.Refusal == tools.%s {", goLocalRefusalName(row.GetGuard()))
		out.writef("\t\treturn mcp.NewToolResultError(%s), nil", sentence)
		out.writef("\t}")
		out.writef("")

		if strings.HasPrefix(sentence, "fmt.") {
			out.need(importFmt)
		}
	}
}

// goLocalArmCall is the arm the handler calls, which sits in this package
// beside the handler itself.
//
// The tool's own declared direction picks the arm, so a two-sided operation is
// reached through the side it runs rather than through one arm reading a
// direction the handler would have to pass.
func goLocalArmCall(answer *localAnswer) string {
	return localArmSpelling(languageGo, answer.Call.String(), answer.Direction)
}

// goLocalCallArguments is the operation's call list: the ambient readings it
// declared, the subsystem, then one local per input in the operation's own
// order.
//
// The readings come off the vocabulary rather than being named one by one,
// because a member added to it would otherwise be silently dropped here while
// the arm and the subsystem both declare it.
func goLocalCallArguments(answer *localAnswer) []string {
	parts := make([]string, 0, len(answer.Arm.declared.GetInput())+4)

	for _, reading := range localArmAmbientParameters(languageGo, &answer.Arm) {
		parts = append(parts, reading.handed)
	}

	if subsystem := goLocalSubsystemArgument(answer); subsystem != "" {
		parts = append(parts, subsystem)
	}

	for _, argument := range answer.Bound {
		parts = append(parts, goLocalRead(argument))
	}

	return parts
}

// goLocalSubsystemArgument is what the handler hands the arm in the subsystem
// slot: the ambient state where the operation declares some, and the engine's
// own function for the operation where it declares none.
func goLocalSubsystemArgument(answer *localAnswer) string {
	if answer.Arm.declared.GetRequires() != linodev1.LocalState_LOCAL_STATE_UNSPECIFIED {
		return previewStateLocal
	}

	return "tools." + localSubsystemMethod(languageGo, answer.Call.String(), answer.Direction)
}

// goLocalSentence renders one declared sentence as the Go expression producing
// it: the text itself where it names nothing, and the Sprintf filling it
// otherwise. Only a bound argument and the reported cause are in scope.
func goLocalSentence(sentence, cause string) string {
	format, args := goLocalFilled(sentence, goRefusalRead(cause))

	// The raw sentence rather than the format string: a literal percent is
	// doubled to reach Sprintf intact, and nothing formats a plain literal.
	if len(args) == 0 {
		return goStringLiteral(sentence)
	}

	return "fmt.Sprintf(" + strings.Join(append([]string{goStringLiteral(format)}, args...), ", ") + ")"
}

// goRefusalRead is how a refusal sentence fills one placeholder: the reported
// cause where the sentence named it, and the bound local otherwise.
func goRefusalRead(cause string) func(string) string {
	return func(named string) string {
		if named == localCausePlaceholder {
			return cause
		}

		return goLocalRead(named)
	}
}

// goLocalFilled splits a sentence into its format string and the expressions
// filling it, each resolved through read. A quoted placeholder takes %q, which
// is the form the audit report's unknown-name refusal answers in.
func goLocalFilled(sentence string, read func(string) string) (string, []string) {
	var (
		text strings.Builder
		args []string
		rest = sentence
	)

	text.Grow(len(sentence))

	for {
		open, closed := localBraces(rest)
		if open < 0 {
			text.WriteString(goPercentEscaped(rest))

			return text.String(), args
		}

		named, quoted := strings.CutSuffix(rest[open+1:open+closed], localQuotedForm)
		text.WriteString(goPercentEscaped(rest[:open]))
		text.WriteString(goPlaceholderVerb(quoted))

		args = append(args, read(named))

		rest = rest[open+closed+1:]
	}
}

// The verbs a declared sentence's placeholders print under: the quoted form
// that answers in quotes, and the plain one every other value takes.
const (
	goQuotedVerb = "%q"
	goPlainVerb  = "%v"
)

// goPercentEscaped is literal text inside a format string: a percent stands for
// itself rather than opening a verb.
func goPercentEscaped(text string) string {
	return strings.ReplaceAll(text, "%", "%%")
}

// goPlaceholderVerb is the verb one placeholder prints under.
func goPlaceholderVerb(quoted bool) string {
	if quoted {
		return goQuotedVerb
	}

	return goPlainVerb
}

// goLocalRefusalName is the engine's own name for one condition, which is what
// the operation reports and the handler compares against.
func goLocalRefusalName(guard linodev1.LocalGuard) string {
	return "LocalRefusal" + localGuardWords(guard)
}

// localGuardWords is a guard member's name in the Go spelling, derived from the
// declared member so a member added to the contract cannot reach the engine
// under a name nobody wrote.
func localGuardWords(guard linodev1.LocalGuard) string {
	return localMemberPascal(strings.TrimPrefix(guard.String(), localGuardPrefix))
}

// localMemberPascal is one declared member's stem run together in the Go
// spelling. Shared by every vocabulary the emitter names a type or a value
// after, so two of them cannot spell one convention two ways.
func localMemberPascal(stem string) string {
	var built strings.Builder

	built.Grow(len(stem))

	// Every part carries at least one letter: the members are underscore-joined
	// words, which is the spelling protoc accepts and buf's enum naming holds.
	for word := range strings.SplitSeq(stem, "_") {
		built.WriteString(strings.ToUpper(word[:1]))
		built.WriteString(strings.ToLower(word[1:]))
	}

	return built.String()
}

// localGuardMembers is a guard member's name in the Python spelling, which is
// the declared member with its prefix removed.
func localGuardMembers(guard linodev1.LocalGuard) string {
	return strings.TrimPrefix(guard.String(), localGuardPrefix)
}
