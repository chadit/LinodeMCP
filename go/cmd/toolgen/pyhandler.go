package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// pyHandler is the handler: the arguments the contract locates, then the driver
// call.
func pyHandler(tool *pyTool) ([]string, error) {
	lines := []string{
		"async def " + pyHandlerName(tool.c.Name) + "(",
		"    arguments: dict[str, Any], " + pyConfigParameter(tool) + ": Config",
		") -> list[TextContent]:",
		`    """Handle ` + tool.c.Name + ".",
		"",
	}
	lines = append(lines, pyRouteDoc(tool, pyDocIndent)...)
	lines = append(lines, pyDocClose)

	lines = append(lines, pyNormalizeLines(tool)...)

	tail, err := pyHandlerTail(tool)
	if err != nil {
		return nil, err
	}

	return append(lines, tail...), nil
}

// pyNormalizeLines is the tool's argument rewrite, which runs before anything
// reads an argument. Each call is written the way ruff format would: one line
// when it fits, otherwise one argument per line with the trailing comma ruff
// keeps.
func pyNormalizeLines(tool *pyTool) []string {
	rendering := normalizeRendering()
	lines := make([]string, 0, len(tool.c.Normalizes))

	for _, rewrite := range tool.c.Normalizes {
		call := rendering[rewrite.Transform].Python
		single := "    " + call + "(" + pyArgumentsMap + ", " + pyQuotedJoin(rewrite.Fields) + ")"

		if len(single) <= pyLineBudget {
			lines = append(lines, single)

			continue
		}

		lines = append(lines, pyNormalizeWrapped(call, rewrite.Fields)...)
	}

	if fold := tool.c.Fold; fold != nil {
		lines = append(lines, "    fold_int_list("+pyArgumentsMap+", "+
			pyQuote(fold.Source)+", "+pyQuote(fold.Target)+", "+pyQuote(fold.Key)+")")
	}

	return lines
}

// pyNormalizeWrapped is one rewrite call too long for a single line.
func pyNormalizeWrapped(call string, fields []string) []string {
	lines := []string{"    " + call + "(", "        " + pyArgumentsMap + ","}
	for _, name := range fields {
		lines = append(lines, "        "+pyQuote(name)+",")
	}

	return append(lines, "    )")
}

// pyHandlerTail is everything below the doc comment, which is the tier's own
// argument handling followed by the call it makes.
func pyHandlerTail(tool *pyTool) ([]string, error) {
	switch {
	case tool.c.Tier == tierMeta:
		return pyMeta(tool)
	case tool.c.Tier == tierWrite:
		return pyTierLines(tool, pyWriteArguments, pyWriteCall)
	case tool.c.Tier == tierBodyRead:
		return pyTierLines(tool, pyBodyReadArguments, pyBodyReadCall)
	case tool.c.Tier == tierAcknowledge:
		return pyTierLines(tool, pyAcknowledgeArguments, pyAcknowledgeCall)
	case tool.c.Tier == tierDestroy:
		return pyTierLines(tool, pyDestroyArguments, pyDestroyCall)
	case tool.gatedRead():
		return pyTierLines(tool, pyGatedReadArguments, pyDriverCall)
	default:
		return pyTierLines(tool, pyArguments, pyDriverCall)
	}
}

// pyTierLines joins one tier's argument handling to the call that follows it.
func pyTierLines(
	tool *pyTool, arguments, call func(*pyTool) ([]string, error),
) ([]string, error) {
	head, err := arguments(tool)
	if err != nil {
		return nil, err
	}

	tail, err := call(tool)
	if err != nil {
		return nil, err
	}

	return append(head, tail...), nil
}

// pyConfigParameter is the name the handler takes its configuration under,
// blanked where nothing in the body reads it, which is a meta tool answering
// its own sentence. The signature stays the one every generated handler has, so
// the registry calls all of them the same way.
func pyConfigParameter(tool *pyTool) string {
	if tool.c.Tier == tierMeta && !metaReadsConfig(tool.c) {
		return "_cfg"
	}

	return "cfg"
}

// pyMeta is a tool that reaches no route: check the rules, then answer local
// state. There is no client to open and no call to gate a verdict behind, so
// the check answers where it is read, the way an ungated read's does.
func pyMeta(tool *pyTool) ([]string, error) {
	lines := []string{
		"    message = " + pyConstraintCall(tool),
		"    if message:",
		"        return error_response(message)",
		"",
	}

	if tool.c.Confirm {
		// Asked after the rules for the reason a mutation asks it before them: a
		// gated meta tool changes local state rather than a resource, and its
		// arguments name a draft the caller already holds.
		lines = append(lines, `    if arguments.get("confirm") is not True:`, "        return error_response(")
		lines = append(lines, pyQuoteWrapped(tool.c.ConfirmMessage, pyProseIndent)...)
		lines = append(lines, "        )", "")
	}

	if tool.c.answersLocally() {
		return append(lines, pyLocalAnswer(tool)...), nil
	}

	sentence, err := pyMetaSentence(tool)
	if err != nil {
		return nil, err
	}

	return append(lines, sentence...), nil
}

// pyMetaSentence is the answer of a meta tool that declares no local
// operation: its declared sentence. The arguments it names are read above it rather than inside the
// literal, so the rendered line stays under the budget however many it names.
func pyMetaSentence(tool *pyTool) ([]string, error) {
	answer, err := pyMetaMessage(tool)
	if err != nil {
		return nil, err
	}

	return append(answer.reads,
		"    return meta_response(",
		"        "+pyQuote(string(tool.c.ResponseGo.FullName))+",",
		"        "+messageFieldName+"="+answer.rendered+",",
		"    )",
	), nil
}

// pyMetaAnswer is a meta tool's declared sentence: the reads that stand above
// the f-string, the argreader names those reads go through, and the f-string
// itself. The readers travel beside the reads so the module's import list is
// decided from the same walk that emitted them.
type pyMetaAnswer struct {
	rendered string
	reads    []string
	readers  []string
}

// pyMetaMessage is a meta tool's declared sentence as the reads it needs and
// the f-string. The arguments come off the request through the same accessor Go
// reads them with, defaults included, so a template written once renders
// identically in each language.
func pyMetaMessage(tool *pyTool) (pyMetaAnswer, error) {
	answer := pyMetaAnswer{reads: make([]string, 0), readers: make([]string, 0)}
	rest := tool.c.SuccessMessage

	var rendered strings.Builder

	rendered.Grow(len(rest))

	for {
		opened := strings.Index(rest, "{")
		if opened < 0 {
			answer.rendered = `f"` + rendered.String() + pyEscaped(rest) + `"`

			return answer, nil
		}

		closed := strings.Index(rest[opened:], "}")
		if closed < 0 {
			answer.rendered = `f"` + rendered.String() + pyEscaped(rest) + `"`

			return answer, nil
		}

		closed += opened

		name, form, fallback := pySplitPlaceholder(rest[opened+1 : closed])

		if err := pyMetaRead(tool, &answer, name, fallback); err != nil {
			return pyMetaAnswer{}, err
		}

		rendered.WriteString(pyEscaped(rest[:opened]))
		rendered.WriteString("{")
		rendered.WriteString(pyLocal(name))
		rendered.WriteString(form)
		rendered.WriteString("}")

		rest = rest[closed+1:]
	}
}

// pyMetaRead records the read one placeholder needs, and the argreader it goes
// through, on the answer under construction.
func pyMetaRead(tool *pyTool, answer *pyMetaAnswer, name, fallback string) error {
	entry, err := pyLocalArgument(tool, name)
	if err != nil {
		return err
	}

	read, err := pyLocalArgumentRead(tool, entry, fallback)
	if err != nil {
		return err
	}

	line := "    " + pyLocal(name) + " = " + read
	if !slices.Contains(answer.reads, line) {
		answer.reads = append(answer.reads, line)
	}

	if reader := pyMetaArgumentReader(entry); !slices.Contains(answer.readers, reader) {
		answer.readers = append(answer.readers, reader)
	}

	return nil
}

// pySplitPlaceholder is one placeholder as its field name, its format spec, and
// its default. A default and a width are alternatives rather than a grammar, so
// a placeholder carrying both names a field spelled with a colon in it and
// fails on the name.
func pySplitPlaceholder(body string) (string, string, string) {
	if name, fallback, found := strings.Cut(body, "|"); found {
		return name, "", fallback
	}

	if name, width, found := strings.Cut(body, ":0"); found {
		return name, ":0" + width + "d", ""
	}

	return body, "", ""
}

// pyLocalArgument is the TOOL argument a meta tool's sentence names.
func pyLocalArgument(tool *pyTool, name string) (protoreflect.FieldDescriptor, error) {
	for i := range tool.c.Local {
		if tool.c.Local[i].ProtoName == name {
			return tool.argument(name), nil
		}
	}

	return nil, fmt.Errorf("%w: %s success_message names %s, which is no TOOL argument of %s",
		errPyRender, tool.c.Name, name, tool.c.InputMessage)
}

// The two argreader functions a declared sentence reads its arguments through,
// named here because the read and the import list both spell them.
const (
	pyToolStringReader = "tool_string"
	pyToolIntReader    = "tool_int"
)

// pyMetaArgumentReader is the shared reader one TOOL argument is read through,
// "" for a kind no declared sentence can name. Text and a number are the only
// kinds covered because Go prints a flag as true and Python as True; the caller
// refuses every other kind rather than rendering it differently on each side.
func pyMetaArgumentReader(entry protoreflect.FieldDescriptor) string {
	if pyRepeated(entry) {
		return ""
	}

	kind := entry.Kind()
	if kind == protoreflect.StringKind {
		return pyToolStringReader
	}

	if kind == protoreflect.Int32Kind || kind == protoreflect.Int64Kind {
		return pyToolIntReader
	}

	return ""
}

// pyLocalArgumentRead is the read one TOOL argument reaches a sentence through.
//
// It goes through the argreader layer rather than `arguments.get`, because a
// bare get answers whatever the caller sent while Go's GetString and GetInt
// answer the declared default for a value the field cannot hold: a name sent
// as the number 5 rendered as "5" here and as the default in Go.
func pyLocalArgumentRead(
	tool *pyTool, entry protoreflect.FieldDescriptor, fallback string,
) (string, error) {
	reader := pyMetaArgumentReader(entry)
	if reader == "" {
		return "", fmt.Errorf("%w: %s success_message names %s, which has no rendering both languages share",
			errPyRender, tool.c.Name, entry.Name())
	}

	name := pyQuote(string(entry.Name()))

	if reader == pyToolStringReader {
		absent := emptyLiteral
		if fallback != "" {
			absent = pyQuote(fallback)
		}

		return reader + "(arguments, " + name + ", " + absent + ")", nil
	}

	if fallback != "" {
		if _, err := strconv.Atoi(fallback); err != nil {
			return "", fmt.Errorf("%w: %s defaults %s to %q, which is not a number",
				errPyRender, tool.c.Name, entry.Name(), fallback)
		}
	}

	if fallback == "" {
		fallback = "0"
	}

	return reader + "(arguments, " + name + ", " + fallback + ")", nil
}

// pyMetaReaderImports is the linodemcp.tools.argreader names a module's
// declared sentences read their arguments through, "" when no sentence names
// an argument.
func pyMetaReaderImports(tools []*pyTool) (string, error) {
	wanted := make([]string, 0, 2)

	for _, tool := range tools {
		if tool.c.Tier != tierMeta || tool.c.answersLocally() {
			continue
		}

		answer, err := pyMetaMessage(tool)
		if err != nil {
			return "", err
		}

		for _, reader := range answer.readers {
			if !slices.Contains(wanted, reader) {
				wanted = append(wanted, reader)
			}
		}
	}

	slices.Sort(wanted)

	return strings.Join(wanted, ", "), nil
}

// pyConstraintCall is the contract's own answers over the whole argument map,
// in order: the rules first, then the arguments the tool refuses outright, then
// whether it takes a name its message does not declare. All three are written
// for every tool rather than only the ones declaring them, so declaring one is
// a proto edit and nothing else.
func pyConstraintCall(tool *pyTool) string {
	call := "check_constraints(" + pyQuote(tool.c.InputMessage) + ", arguments)"

	if refused := tool.c.Refused.GetFields(); len(refused) > 0 {
		call += " or refused_arguments(arguments, " +
			pySentenceArgument(tool.c.Refused.GetMessage()) + ", (" + pyNameTuple(refused) + "))"
	}

	if tool.c.RefuseUnknown != nil {
		message := tool.c.RefuseUnknown.GetMessage()

		call += " or unknown_arguments(arguments, " +
			pySentenceArgument(message) + ", (" + pyNameTuple(tool.c.AllArguments) + "))"
	}

	return call
}

// pyNameTuple is a name list as the body of a Python tuple, with the trailing
// comma a one-element tuple needs to stay one.
func pyNameTuple(names []string) string {
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, pyQuote(name))
	}

	joined := strings.Join(quoted, ", ")
	if len(names) == 1 {
		joined += ","
	}

	return joined
}

// pyArguments reads the path values and refuses a call that omits one.
func pyArguments(tool *pyTool) ([]string, error) {
	lines := []string{
		"    message = " + pyConstraintCall(tool),
		"    if message:",
		"        return error_response(message)",
		"",
	}

	checks, err := pyReaderChecks(tool)
	if err != nil {
		return nil, err
	}

	lines = append(lines, checks...)

	for _, entry := range pyQueryPresentBoolFields(tool) {
		lines = append(lines,
			"    _, message = "+pyPresenceCall(entry),
			"    if message:",
			"        return error_response(message)",
			"",
		)
	}

	for _, slot := range tool.derivedSlots() {
		lines = append(lines, "    "+pyLocal(slot)+" = "+pyPathRead(tool, slot))
	}

	for _, slot := range tool.derivedSlots() {
		lines = append(lines,
			"    if not "+pyLocal(slot)+":",
			"        return error_response("+pyQuote(slot+" is required")+")",
		)
	}

	query, err := pyGetQuery(tool)
	if err != nil {
		return nil, err
	}

	return append(append(lines, query...), ""), nil
}

// pyGatedReadArguments validates and reads the ids without answering either
// yet. A gated read hands its verdict to the driver for the reason the write
// tier does: a live call must not disclose that its arguments would have been
// accepted before the caller has confirmed.
func pyGatedReadArguments(tool *pyTool) ([]string, error) {
	lines, err := pyValidation(tool)
	if err != nil {
		return nil, err
	}

	for _, slot := range tool.c.Slots {
		lines = append(lines, "    "+pyLocal(slot)+" = "+pyPathRead(tool, slot))
	}

	// `or None` because the rule reader answers "" for a call it accepted, and
	// the driver reports any error it is handed, empty string included.
	return append(lines, "    failure = message or None", ""), nil
}

// pyWriteArguments validates, then builds the body, without answering either
// yet. Both branches of a mutation share this, which is what keeps a preview
// from advertising a body the live call would reject.
func pyWriteArguments(tool *pyTool) ([]string, error) {
	lines, err := pyValidation(tool)
	if err != nil {
		return nil, err
	}

	for _, slot := range tool.c.Slots {
		lines = append(lines, "    "+pyLocal(slot)+" = "+pyPathRead(tool, slot))
	}

	lines = append(lines,
		"    body, body_message = "+pyBodyName(tool.c.Name)+"(arguments)",
		"    failure = message or body_message",
		"",
	)

	// Built in both branches so a preview reports the call the live path makes:
	// the page controls a paged replacement publishes change which assignments
	// come back, and a preview omitting them would describe a different request.
	if len(tool.c.Query) > 0 {
		query, queryErr := pyWriteQuery(tool)
		if queryErr != nil {
			return nil, queryErr
		}

		lines = append(append(lines, query...), "")
	}

	return lines, nil
}

// pyBodyReadArguments checks the arguments and builds the body, answering each
// where it is read: an ungated read's argument handling with a mutation's body
// build after it, since there is no confirm gate for a verdict to wait behind.
func pyBodyReadArguments(tool *pyTool) ([]string, error) {
	lines, err := pyArguments(tool)
	if err != nil {
		return nil, err
	}

	return append(lines,
		"    body, body_message = "+pyBodyName(tool.c.Name)+"(arguments)",
		"    if body_message:",
		"        return error_response(body_message)",
		"",
	), nil
}

// pyAcknowledgeArguments validates, reads the echoed arguments, and builds the
// body when there is one.
func pyAcknowledgeArguments(tool *pyTool) ([]string, error) {
	lines, err := pyValidation(tool)
	if err != nil {
		return nil, err
	}

	for _, slot := range tool.c.Slots {
		lines = append(lines, "    "+pyLocal(slot)+" = "+pyPathRead(tool, slot))
	}

	if !tool.buildsBody() {
		// `or None` because the rule reader answers "" for a call it accepted,
		// and the driver reports any error it is handed, empty string included.
		lines = append(lines, "    failure = message or None", "")

		return append(lines, pyTwoStageClosures(tool)...), nil
	}

	lines = append(lines,
		"    body, body_message = "+pyBodyName(tool.c.Name)+"(arguments)",
		"    failure = message or body_message",
		"",
	)

	return append(lines, pyTwoStageClosures(tool)...), nil
}

// pyTwoStageClosures hands the plan its state read and its walk, for a staged
// mutation. The state a plan hashes is not the one a preview reports: a resize
// plans against the instance and its disks together because the resize moves
// both, so the fetch here reads the declared composite, never the preview's
// state.
func pyTwoStageClosures(tool *pyTool) []string {
	if !tool.c.Mode {
		return nil
	}

	lines := []string{"    async def fetch_state(client: RetryableClient) -> Any:"}
	lines = append(lines, pyTwoStageFetchBody(tool)...)
	lines = append(lines, "")

	// The seeded prose rides the same closure the walks do, so a staged tool
	// that declares sentences and no walk still reports them on its plan.
	if len(tool.c.DepWalks) > 0 || len(tool.c.PreviewSentences) > 0 || tool.c.BillingDecl != nil {
		return append(lines, pyDeclaredWalks(tool)...)
	}

	return lines
}

// pyValidation is the argument check: the contract's own rules, then the route's
// derived ones.
func pyValidation(tool *pyTool) ([]string, error) {
	call := pyConstraintCall(tool)
	if pyNothingToCheck(tool) {
		return []string{"    message = " + call}, nil
	}

	lines := []string{"    message = " + call}

	// A reader answers its own sentence, so each one runs behind whatever the
	// rules and the readers before it already found, rather than joining the
	// chain below that only knows how to say "is required".
	for _, slot := range tool.readerSlots() {
		reader, readerErr := pyReaderCall(tool, slot)
		if readerErr != nil {
			return nil, readerErr
		}

		lines = append(lines, pyUncheckedGuard, "        _, message = "+reader)
	}

	body, err := pyBodyReaderChecks(tool)
	if err != nil {
		return nil, err
	}

	lines = append(lines, body...)

	for _, entry := range pyQueryPresentBoolFields(tool) {
		lines = append(lines, pyUncheckedGuard, "        _, message = "+pyPresenceCall(entry))
	}

	// The walks read what is inside an argument whose own shape just passed, and
	// the cross-field any-of question comes after them: each argument answers for
	// itself before the call answers for naming nothing.
	lines = append(lines, pyWalkLines(tool)...)
	lines = append(lines, pyAnyOfLines(tool)...)

	derived := tool.derivedSlots()
	if len(derived) == 0 {
		return lines, nil
	}

	// A lone slot folds into the guard: a single nested if under it is the
	// shape the linter refuses.
	if len(derived) == 1 {
		lines = append(lines,
			"    if not message and not arguments.get("+pyQuote(derived[0])+"):",
			"        message = "+pyQuote(derived[0]+" is required"),
		)

		return lines, nil
	}

	lines = append(lines, pyUncheckedGuard)
	branch := pyBranchIf

	for _, slot := range derived {
		lines = append(lines,
			"        "+branch+" not arguments.get("+pyQuote(slot)+"):",
			"            message = "+pyQuote(slot+" is required"),
		)
		branch = pyBranchElif
	}

	return lines, nil
}

// pyNothingToCheck reports whether a tool's rules are the whole check, which is
// what lets its verdict be one line.
func pyNothingToCheck(tool *pyTool) bool {
	return len(tool.c.Slots) == 0 &&
		len(pyBodyReaderFields(tool, linodev1.ArgumentReader_ARGUMENT_READER_PRESENT)) == 0 &&
		len(pyBodyReaderFields(tool, linodev1.ArgumentReader_ARGUMENT_READER_ENUM_MEMBER)) == 0 &&
		len(pyBodyReaderFields(tool, linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_TEXT)) == 0 &&
		len(pyBodyReaderFields(tool, linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_BOOL)) == 0 &&
		len(pyBodyReaderFields(tool, linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_STRING)) == 0 &&
		len(pyBodyReaderFields(tool, linodev1.ArgumentReader_ARGUMENT_READER_ID_LIST)) == 0 &&
		len(pyQueryPresentBoolFields(tool)) == 0 &&
		tool.c.AnyOf == nil && !tool.c.Walks
}

// pyBodyReaderChecks asks the body readers in declaration order, so a mutation
// reports a refused argument in the same accumulated `message` its path checks
// use.
func pyBodyReaderChecks(tool *pyTool) ([]string, error) {
	lines := make([]string, 0)

	for _, entry := range tool.bodyFields() {
		switch reader := readerOf(entry); {
		case reader == linodev1.ArgumentReader_ARGUMENT_READER_PRESENT:
			lines = append(lines, pyUncheckedGuard,
				"        _, message = required_present(arguments, "+pyQuote(string(entry.Name()))+")")
		case pyIsPresenceReader(reader):
			lines = append(lines, pyUncheckedGuard, "        _, message = "+pyPresenceCall(entry))
		case reader == linodev1.ArgumentReader_ARGUMENT_READER_ENUM_MEMBER:
			members, ok := tool.c.Members[string(entry.Name())]
			if !ok {
				return nil, fmt.Errorf("%w: %s reads %s as a member choice with no vocabulary",
					errPyRender, tool.c.Name, entry.Name())
			}

			lines = append(lines, pyUncheckedGuard)
			lines = append(lines, pyMemberChoiceLines("        ", "_, message",
				pyMemberChoiceArguments(entry, members), pyMemberChoiceHelper(entry))...)
		}
	}

	return lines, nil
}

// pyWalkLines is the open-object walk rendered into the accumulated check. Only
// the message name travels: the walker reads the declaration from the
// descriptor, so declaring a walk is a proto edit and nothing else.
func pyWalkLines(tool *pyTool) []string {
	if !tool.c.Walks {
		return nil
	}

	return []string{
		pyUncheckedGuard,
		"        message = walk_objects(" + pyQuote(tool.c.InputMessage) + ", arguments)",
	}
}

// pyAnyOfLines is the require_any_of check rendered the way ruff format would
// write it: one line when the call fits, otherwise one argument per line with
// the trailing comma ruff keeps.
func pyAnyOfLines(tool *pyTool) []string {
	if tool.c.AnyOf == nil {
		return nil
	}

	arguments := []string{pyArgumentsMap, pyQuote(tool.c.AnyOf.Message)}
	for _, name := range tool.c.AnyOf.Fields {
		arguments = append(arguments, pyQuote(name))
	}

	single := "        message = require_any_argument(" + strings.Join(arguments, ", ") + ")"
	if len(single) <= pyLineBudget {
		return []string{pyUncheckedGuard, single}
	}

	lines := []string{pyUncheckedGuard, "        message = require_any_argument("}
	for _, argument := range arguments {
		lines = append(lines, "            "+argument+",")
	}

	return append(lines, "        )")
}

// pyReaderChecks is the shared reader call for each slot declaring one,
// answering on refusal. The reader hands back the value beside the sentence, so
// this both reads the local and refuses, standing in for the derived
// read-and-check rather than sitting beside it.
func pyReaderChecks(tool *pyTool) ([]string, error) {
	lines := make([]string, 0)

	for _, slot := range tool.readerSlots() {
		call, err := pyReaderCall(tool, slot)
		if err != nil {
			return nil, err
		}

		lines = append(lines,
			"    "+pyLocal(slot)+", message = "+call,
			"    if message:",
			"        return error_response(message)",
			"",
		)
	}

	return lines, nil
}

// pyPathRead reads one path argument in its declared type. A numeric id goes
// through path_int, which answers 0 for a value that is not one so the required
// check reports the tool's own sentence; a text slot goes through path_str so a
// number the caller sent reads as absent rather than being spliced into a path.
func pyPathRead(tool *pyTool, slot string) string {
	argument := tool.pathArg(slot)
	read := "arguments.get(" + pyQuote(slot) + ", " + argument.absent + ")"

	if argument.numeric {
		return "path_int(" + read + ")"
	}

	return "path_str(" + read + ")"
}

// pyPathValuesLiteral is the map a driver fills the route template from, in
// slot order.
func pyPathValuesLiteral(tool *pyTool) string {
	entries := make([]string, 0, len(tool.c.Slots))

	for _, slot := range tool.c.Slots {
		argument := tool.pathArg(slot)
		entries = append(entries, pyQuote(slot)+": "+argument.coerce+
			"("+pyLocal(slot)+" or "+argument.absent+")")
	}

	return strings.Join(entries, ", ")
}

// pyGetQuery is the query a single-resource route publishes, built before the
// call so a range failure answers before a client is opened.
func pyGetQuery(tool *pyTool) ([]string, error) {
	if tool.c.Tier != tierGet || len(tool.c.Query) == 0 {
		return nil, nil
	}

	return pyQueryBuild(tool)
}

// pyQueryBuild is the query a read carries, answered where a bad bound is read:
// a read has no gate for a verdict to wait behind, so the range failure answers
// here and keeps a bad page number off the wire.
func pyQueryBuild(tool *pyTool) ([]string, error) {
	page, forwarded, err := pyQueryParts(tool)
	if err != nil {
		return nil, err
	}

	lines := make([]string, 0, 9)
	lines = append(lines, `    query = ""`)

	if len(page) > 0 {
		lines = append(lines[:0],
			"    try:",
			`        page = pagination_int_argument(arguments, "page", 1)`,
			"        page_size = pagination_int_argument(",
			`            arguments, "page_size", 25, 500`,
			"        )",
			"    except (TypeError, ValueError) as exc:",
			"        return error_response(str(exc))",
			"    query = pagination_query(page, page_size)",
		)
	}

	return append(lines, pyForwardedQuery(forwarded)...), nil
}

// pyWriteQuery is the query a mutation carries, with a bad bound folded into
// its verdict. A mutation hands every verdict to the driver so the confirm gate
// answers first: answering the range failure here instead would report it ahead
// of the gate on one client and behind it on the other.
func pyWriteQuery(tool *pyTool) ([]string, error) {
	page, forwarded, err := pyQueryParts(tool)
	if err != nil {
		return nil, err
	}

	lines := make([]string, 0, 9)
	lines = append(lines, `    query = ""`)

	if len(page) > 0 {
		lines = append(lines[:0],
			"    query = ''",
			"    try:",
			`        page = pagination_int_argument(arguments, "page", 1)`,
			"        page_size = pagination_int_argument(",
			`            arguments, "page_size", 25, 500`,
			"        )",
			"        query = pagination_query(page, page_size)",
			"    except (TypeError, ValueError) as exc:",
			"        failure = failure or str(exc)",
		)
	}

	return append(lines, pyForwardedQuery(forwarded)...), nil
}

// pyForwardedQuery appends the parameters the route itself filters on to the
// built query.
func pyForwardedQuery(forwarded []string) []string {
	if len(forwarded) == 0 {
		return nil
	}

	named := make([]string, 0, len(forwarded))
	for _, name := range forwarded {
		named = append(named, pyQuote(name))
	}

	return []string{"    query = with_query_arguments(arguments, query, (" + strings.Join(named, ", ") + ",))"}
}

// pyQueryParts splits a tool's query arguments into page controls and forwarded
// ones. The page controls are read as a pair because the encoder takes both: a
// route publishing one alone would send a bound nothing validated.
func pyQueryParts(tool *pyTool) ([]string, []string, error) {
	page := make([]string, 0, 2)
	forwarded := make([]string, 0, len(tool.c.Query))

	for i := range tool.c.Query {
		name := tool.c.Query[i].ProtoName
		if isPaginationParam(name) {
			page = append(page, name)

			continue
		}

		forwarded = append(forwarded, name)
	}

	if len(page) > 0 && len(page) != 2 {
		return nil, nil, fmt.Errorf("%w: %s publishes %s alone, and the page controls are read as a pair",
			errPyRender, tool.c.Name, strings.Join(page, ", "))
	}

	return page, forwarded, nil
}

// pyTwoStageFetchBody is the staged fetch's body: the composite the tool
// declares.
func pyTwoStageFetchBody(tool *pyTool) []string {
	return pyCompositeFetch(tool)
}
