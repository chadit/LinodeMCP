package main

import (
	"slices"
	"strings"
)

// The Python arm's rendering of the declared dry-run prose: the same wordings
// the Go arm writes, handed to the driver rather than to a builder, since this
// arm's driver is where a preview is assembled.

// The columns a declared sentence is written at: one level inside the driver
// keyword for a bare wording, two for one the support layer chooses.
// pyPreviewValuesLocal is the name a wording reads its placeholders out of.
const pyPreviewValuesLocal = "preview_values"

// pyPreviewNestStep is one indent level, which is what a guard moves the wording
// it wraps in by.
const pyPreviewNestStep = 4

// pyPreviewArmIndent is the column a matched line's arms are written at, two
// levels inside the driver keyword the match sits under.
const pyPreviewArmIndent = 20

const (
	pyPreviewSentenceIndent = 12
	pyPreviewTemplateIndent = 16
)

// pyPreviewValues is the map a wording reads its placeholders out of, built
// once ahead of the driver call because several wordings of one line read the
// same argument.
func pyPreviewValues(tool *pyTool) []string {
	names := tool.c.previewValueArguments()
	if len(names) == 0 {
		return nil
	}

	if subject, worded := pyPreviewClosureSubject(tool.c); worded {
		return pyPreviewStateClosures(tool, names, subject)
	}

	lines := []string{"    preview_values = {"}

	for _, name := range names {
		lines = append(lines, "        "+pyQuote(name)+": "+pyPreviewValueRead(tool, name)+",")
	}

	return append(lines, "    }", "")
}

// pyPreviewValueRead is the read one placeholder's value comes from: the
// arguments for one the call carried, the fetched resource for a member of the
// state.
func pyPreviewValueRead(tool *pyTool, name string) string {
	if name == previewElementName {
		list := tool.c.previewJoinedList()

		return "preview_joined(arguments, " + pyQuote(list.Argument) + ", " + pyQuote(list.Join) + ")"
	}

	// The one member the guard measures, so the read needs no member lookup.
	if strings.HasPrefix(name, previewTransportPrefix) {
		return pyPreviewTransferLocal + ".size_bytes"
	}

	if path, reads := strings.CutPrefix(name, previewStatePrefix); reads {
		reading := pyPreviewValueReader(tool.c, name) + "(state, " + pyQuote(path) + ")"

		if held := tool.c.previewUnchangedFor(name); held != "" {
			return "preview_changed(" + reading + ", " + pyPreviewArgumentRead(tool.c, held) + ")"
		}

		return reading
	}

	return pyPreviewArgumentRead(tool.c, name)
}

// pyPreviewArgumentRead is one argument's value as this arm reads it, taking the
// carried-number reader for a number every line reading it guards on.
func pyPreviewArgumentRead(tool *contract, name string) string {
	if tool.previewCarriedNumber(name) {
		return "preview_carried_number(arguments, " + pyQuote(name) + ")"
	}

	read := pyPreviewValueReader(tool, name) + "(arguments, " + pyQuote(name) + ")"

	// The fold's declared value, for the same reason the Go arm reports it.
	if fallback := tool.foldDefault(name); fallback != "" {
		return "preview_or(" + read + ", " + pyQuote(fallback) + ")"
	}

	return read
}

// pyPreviewGuard is the condition a guarded line is reported under: a text
// argument through its own reader, which already collapses empty onto absent,
// and anything else through presence.
func pyPreviewGuard(tool *contract, names []string) string {
	tests := make([]string, 0, len(names))

	for _, name := range names {
		if previewGuardsOnText(tool.previewArgument(name)) {
			tests = append(tests, "bool(preview_text(arguments, "+pyQuote(name)+"))")

			continue
		}

		tests = append(tests, "preview_carried(arguments, "+pyQuote(name)+")")
	}

	return strings.Join(tests, " and ")
}

// pyPreviewTransferLocal is the name this arm binds a presigned upload's
// measurement to inside the closures a transfer preview words its lines in.
const pyPreviewTransferLocal = "transfer"

// pyPreviewSubject is what a closure-worded preview reads its values off: the
// local the driver hands each half, and the function that builds the map from
// it.
type pyPreviewSubject struct {
	Local  string
	Values string
}

// pyPreviewClosureSubject is the subject a tool's prose is worded against once
// something has answered, and false for prose worded ahead of the driver call:
// the fetched resource for state prose, the guard's measurement for a transfer
// preview.
func pyPreviewClosureSubject(tool *contract) (pyPreviewSubject, bool) {
	if tool.previewReadsTransport() {
		return pyPreviewSubject{Local: pyPreviewTransferLocal, Values: "preview_transfer_values"}, true
	}

	if tool.previewReadsStateProse() {
		return pyPreviewSubject{Local: previewStateLocal, Values: "preview_state_values"}, true
	}

	return pyPreviewSubject{}, false
}

// pyPreviewStateClosures is the prose of a tool whose wordings name something
// answered after the driver call begins: the values built from that answer,
// then each declared half worded against them.
//
// They are functions rather than literals because this arm hands the driver its
// lines before the fetch or the measurement runs, and a sentence naming what
// came back has nothing to say until it has.
func pyPreviewStateClosures(tool *pyTool, names []string, subject pyPreviewSubject) []string {
	lines := make([]string, 0, len(names)+6)
	lines = append(lines,
		"    def "+subject.Values+"("+subject.Local+": Any) -> dict[str, str]:",
		"        return {")

	for _, name := range names {
		lines = append(lines, "            "+pyQuote(name)+": "+pyPreviewValueRead(tool, name)+",")
	}

	lines = append(lines, "        }", "")

	for _, half := range pyPreviewHalves() {
		lines = append(lines, pyPreviewStateHalf(tool, half, subject)...)
	}

	return lines
}

// pyPreviewHalf is one half of a declared preview: the driver keyword it is
// passed under, the closure it is worded by, and whether it holds the warnings.
type pyPreviewHalf struct {
	Keyword  string
	Closure  string
	Warnings bool
}

// pyPreviewHalves is the two halves in the order they are written.
func pyPreviewHalves() []pyPreviewHalf {
	return []pyPreviewHalf{
		{Keyword: "side_effects", Closure: "preview_effects", Warnings: false},
		{Keyword: "warnings", Closure: "preview_cautions", Warnings: true},
	}
}

// pyPreviewStateHalf is one half of the prose as a closure over the subject's
// answer, and nothing at all when the tool declares no line for that half.
func pyPreviewStateHalf(tool *pyTool, half pyPreviewHalf, subject pyPreviewSubject) []string {
	worded := pyPreviewLines(tool, half.Keyword, half.Warnings)
	if worded == nil {
		return nil
	}

	// A half of nothing but fixed wordings looks nothing up, so binding the map
	// would leave the closure carrying a name it never reads, and the unread
	// parameter itself gets the underscore the linter exempts.
	binds := slices.ContainsFunc(worded, func(line string) bool {
		return strings.Contains(line, pyPreviewValuesLocal)
	})

	parameter := subject.Local
	if !binds {
		parameter = "_" + subject.Local
	}

	lines := []string{"    def " + half.Closure + "(" + parameter + ": Any) -> tuple[str, ...]:"}

	if binds {
		lines = append(lines, "        "+pyPreviewValuesLocal+" = "+subject.Values+"("+subject.Local+")")
	}

	lines = append(lines, "        return (")

	// The half's own rendering opens with the driver keyword and closes with the
	// keyword separator, which a return statement spells differently.
	lines = append(lines, worded[1:len(worded)-1]...)

	return append(lines, "        )", "")
}

// pyPreviewLines is one half of the declared prose as a driver keyword, and no
// lines at all when the tool declares none for that half.
func pyPreviewLines(tool *pyTool, keyword string, warnings bool) []string {
	if perElement := previewPerElementLine(tool.c, warnings); perElement != nil {
		return pyPreviewPerElement(tool, keyword, perElement)
	}

	lines := []string{"        " + keyword + "=("}

	var declared bool

	for i := range tool.c.PreviewSentences {
		sentence := &tool.c.PreviewSentences[i]
		if sentence.Warning != warnings {
			continue
		}

		declared = true

		lines = append(lines, pyPreviewSentence(tool, sentence)...)
	}

	if !declared {
		return nil
	}

	return append(lines, "        ),")
}

// pyPreviewPerElement is a half written per entry of a repeated argument, which
// the driver takes as the list it is rather than as a tuple of wordings.
func pyPreviewPerElement(tool *pyTool, keyword string, sentence *previewSentence) []string {
	lines := []string{
		"        " + keyword + "=preview_per_element(",
		"            arguments,",
		"            " + pyQuote(sentence.Elements.Argument) + ",",
		"            " + pyPreviewValuesName(tool) + ",",
	}
	lines = append(lines, pyWrappedEntry(sentence.Templates[0], pyPreviewSentenceIndent)...)

	return append(lines, "        ),")
}

// pyPreviewSentence is one declared line as the driver receives it: the wording
// itself when there is nothing to choose between and nothing to fill in, and
// the support call that chooses and fills otherwise.
func pyPreviewSentence(tool *pyTool, sentence *previewSentence) []string {
	if sentence.Choice != nil {
		return pyPreviewChoice(tool, sentence.Choice)
	}

	if guard := pyPreviewLineGuard(tool.c, sentence); guard != "" {
		lines := []string{
			"            preview_guarded(",
			"                " + guard + ",",
		}
		lines = append(lines, pyIndented(pyPreviewWording(tool, sentence, pyPreviewNestStep), pyPreviewNestStep)...)

		return append(lines, "            ),")
	}

	// An unguarded bare wording stands directly in the half's collection, so a
	// split one carries the parentheses the linter asks of it there.
	if bare := pyPreviewBareTemplate(sentence); bare != "" {
		return pyWrappedGroupEntry(bare, pyPreviewSentenceIndent)
	}

	return pyPreviewWording(tool, sentence, 0)
}

// pyPreviewBareTemplate is the wording of a line with nothing to choose
// between and nothing to fill in, and empty when the line needs the support
// layer.
func pyPreviewBareTemplate(sentence *previewSentence) string {
	if sentence.Match != nil || len(sentence.Templates) != 1 {
		return ""
	}

	if strings.Contains(sentence.Templates[0], previewPlaceholderOpen) {
		return ""
	}

	return sentence.Templates[0]
}

// pyPreviewMatchArm is one arm as a dict entry, parenthesized when its wording
// does not fit beside the value it answers for.
func pyPreviewMatchArm(value, wording string, reserve int) []string {
	key := strings.Repeat(" ", pyPreviewArmIndent) + pyQuote(value) + ": "

	if single := key + pyQuote(wording) + ","; len(single)+reserve <= pyLineBudget {
		return []string{single}
	}

	lines := []string{key + "("}
	lines = append(lines, pyQuoteWrappedFor(wording, pyPreviewArmIndent+pyPreviewNestStep, reserve)...)

	return append(lines, strings.Repeat(" ", pyPreviewArmIndent)+"),")
}

// pyPreviewLineGuard is the whole condition a line is reported under: the
// arguments the call has to carry, and the change it has to be making.
func pyPreviewLineGuard(tool *contract, sentence *previewSentence) string {
	tests := make([]string, 0, 2)

	if len(sentence.Present) > 0 {
		tests = append(tests, pyPreviewGuard(tool, sentence.Present))
	}

	if pair := sentence.Changed; pair != nil {
		tests = append(tests, "preview_differs(preview_values["+pyQuote(pair.State)+
			"], preview_values["+pyQuote(pair.Argument)+"])")
	}

	return strings.Join(tests, " and ")
}

// pyPreviewMatch is one matched line as the driver receives it: the value read,
// then the arm it selects filled in.
func pyPreviewMatch(tool *pyTool, match *previewMatch, reserve int) []string {
	read := pyPreviewArgumentRead(tool.c, match.Argument)
	if match.State != "" {
		read = "preview_folded(" + pyPreviewValuesName(tool) + "[" + pyQuote(match.State) + "])"
	}

	lines := []string{
		"            preview_matched(",
		"                " + pyPreviewValuesName(tool) + ",",
		"                " + read + ",",
		"                {",
	}

	for _, value := range match.Order {
		lines = append(lines, pyPreviewMatchArm(value, match.Arms[value], reserve)...)
	}

	lines = append(lines, "                },")
	lines = append(lines, pyWrappedEntryFor(match.Otherwise, pyPreviewTemplateIndent, reserve)...)

	return append(lines, "            ),")
}

// pyPreviewWording is the wording half of one line: the wording itself when
// there is nothing to choose between and nothing to fill in, and the support
// call that chooses and fills otherwise. The reserve is the indent a guard
// later moves the whole block in by, held back so the moved lines still fit.
func pyPreviewWording(tool *pyTool, sentence *previewSentence, reserve int) []string {
	if sentence.Match != nil {
		return pyPreviewMatch(tool, sentence.Match, reserve)
	}

	if bare := pyPreviewBareTemplate(sentence); bare != "" {
		return pyWrappedEntryFor(bare, pyPreviewSentenceIndent, reserve)
	}

	lines := []string{
		"            preview_sentence(",
		"                preview_values,",
	}

	for _, template := range sentence.Templates {
		lines = append(lines, pyWrappedEntryFor(template, pyPreviewTemplateIndent, reserve)...)
	}

	return append(lines, "            ),")
}

// pyIndented is a rendered block moved in by one or more levels, for the lines
// a guard nests inside its own call.
func pyIndented(lines []string, spaces int) []string {
	pad := strings.Repeat(" ", spaces)
	moved := make([]string, 0, len(lines))

	for _, line := range lines {
		moved = append(moved, pad+line)
	}

	return moved
}

// pyPreviewChoice is one chosen line as the driver receives it: the flag read
// in its three states, then the arm that state selects filled in.
func pyPreviewChoice(tool *pyTool, choice *previewChoice) []string {
	lines := []string{
		"            preview_chosen(",
		"                " + pyPreviewValuesName(tool) + ",",
		"                " + pyPreviewFlagRead(choice) + ",",
	}

	for _, wording := range choice.Wordings.all() {
		lines = append(lines, pyWrappedEntry(wording, pyPreviewTemplateIndent)...)
	}

	return append(lines, "            ),")
}

// pyPreviewValuesName is the map a wording reads its placeholders out of, and
// an empty literal for a tool whose declared prose reads no argument at all: a
// chosen line can be three fixed sentences, which name nothing to look up.
func pyPreviewValuesName(tool *pyTool) string {
	if len(tool.c.previewValueArguments()) == 0 {
		return "{}"
	}

	return "preview_values"
}

// pyPreviewFlagRead is the Python arm's read of the flag a line selects on.
func pyPreviewFlagRead(choice *previewChoice) string {
	if choice.Member == "" {
		return "preview_flag(arguments, " + pyQuote(choice.Argument) + ")"
	}

	return "preview_member_flag(arguments, " + pyQuote(choice.Argument) +
		", " + pyQuote(choice.Member) + ")"
}

// pyGroupIndent is one Python indent level, which a parenthesized group's
// contents sit inside.
const pyGroupIndent = 4

// pyWrappedEntry is one declared wording as a call argument: wrapped with room
// kept for the separator, since the comma lands on the last line of it.
func pyWrappedEntry(text string, indent int) []string {
	return pyWrappedEntryFor(text, indent, 0)
}

// pyWrappedEntryFor is pyWrappedEntry with columns held back for the indent a
// guard later moves the block in by.
func pyWrappedEntryFor(text string, indent, reserve int) []string {
	return pyCommaTerminated(pyQuoteWrappedFor(text, indent, reserve+1))
}

// pyWrappedGroupEntry is one declared wording standing directly in a tuple or
// list: a split entry is an implicit concatenation inside a collection, which
// the linter holds to being parenthesized.
func pyWrappedGroupEntry(text string, indent int) []string {
	lines := pyQuoteWrappedFor(text, indent, 1)
	if len(lines) == 1 {
		return pyCommaTerminated(lines)
	}

	pad := strings.Repeat(" ", indent)
	out := make([]string, 0, len(lines)+2)
	out = append(out, pad+"(")
	out = append(out, pyQuoteWrapped(text, indent+pyGroupIndent)...)
	out = append(out, pad+")")

	return pyCommaTerminated(out)
}

// pyCommaTerminated puts the separator on the last line of a rendered literal,
// which is where it belongs when the literal was split across several.
func pyCommaTerminated(lines []string) []string {
	lines[len(lines)-1] += ","

	return lines
}

// pyStandInMembers is the driver keyword the declared stand-ins render to, one
// entry per member the reported body carries fixed text for. Go stands them in
// at the call site instead, since its builder answers a copy.
func pyStandInMembers(tool *pyTool) []string {
	if len(tool.c.PreviewStandIns) == 0 {
		return nil
	}

	lines := make([]string, 0, len(tool.c.PreviewStandIns)+2)
	lines = append(lines, "        stand_in_preview=(")

	for _, entry := range tool.c.PreviewStandIns {
		lines = append(lines, "            PreviewStandIn("+pyQuote(entry.Argument)+", "+
			pyQuote(entry.Member)+", "+pyQuote(entry.Text)+"),")
	}

	return append(lines, "        ),")
}

// pyDeclaredPreview is the driver keywords a tool's declared prose renders to:
// each half of it, the state read it reports the resource through, and the
// choice to report no request body for the families whose preview predates the
// echo.
func pyDeclaredPreview(tool *pyTool) []string {
	if !tool.c.declaresPreview() {
		return nil
	}

	lines := append(pyDeclaredPreviewLines(tool), pyPreviewBilling(tool)...)

	if tool.c.previewReadsState() {
		lines = append(lines, "        state_fetch=preview_state,")
	}

	if tool.c.previewReadsTransport() {
		lines = append(lines, "        preview_transfer=True,")
	}

	if tool.c.PreviewOmitsBody {
		lines = append(lines, "        preview_request_body=False,")
	}

	return lines
}

// pyBillingNoteIndent is the column the estimate's note wraps at, one step in
// from the member it belongs to.
const pyBillingNoteIndent = 16

// pyPreviewBilling is the deliberately-unknown estimate a create declares,
// stated outright because there is no price to read and so nothing to call.
func pyPreviewBilling(tool *pyTool) []string {
	if !billingUnpriced(tool.c.BillingDecl) {
		return nil
	}

	lines := []string{
		"        billing_delta={",
		"            \"monthly_change_usd\": BILLING_UNKNOWN,",
		"            \"note\": (",
	}

	lines = append(lines, pyQuoteWrappedFor(tool.c.BillingDecl.GetAbsentTypeSentence(), pyBillingNoteIndent, 0)...)

	return append(lines, "            ),", "        },")
}

// pyDeclaredPreviewLines is each half of the declared prose as the driver
// receives it: the wordings themselves, or the closure that words them once the
// read has answered.
func pyDeclaredPreviewLines(tool *pyTool) []string {
	if _, worded := pyPreviewClosureSubject(tool.c); !worded {
		lines := pyPreviewLines(tool, "side_effects", false)

		return append(lines, pyPreviewLines(tool, "warnings", true)...)
	}

	lines := make([]string, 0, len(pyPreviewHalves()))

	for _, half := range pyPreviewHalves() {
		if pyPreviewLines(tool, half.Keyword, half.Warnings) == nil {
			continue
		}

		lines = append(lines, "        "+half.Keyword+"="+half.Closure+",")
	}

	return lines
}

// pyPreviewStateRead is the fetch a declared preview reports the resource
// through, defined ahead of the driver call the way this arm defines every
// closure it hands over.
//
// The read is addressed by this tool's own path arguments in that read's slot
// order, which is what the contract already resolved them into.
func pyPreviewStateRead(tool *pyTool) []string {
	if !tool.c.previewReadsState() {
		return nil
	}

	call := "await read_route_state(client, " + pyPreviewStateIDs(tool) +
		"tool=" + pyQuote(tool.c.StateRead.Tool) + pyStateMember(tool) + pyStatePayload(tool) +
		pyStateQuery(tool) + ")"

	// A page is not one resource, so the envelope arm reads its own driver: the
	// route reader above projects a body through a single message.
	if tool.c.StateRead.Envelope {
		call = "await read_envelope_state(client, " + pyPreviewStateIDs(tool) +
			"tool=" + pyQuote(tool.c.StateRead.Tool) + pyStateQuery(tool) + ")"
	}

	return []string{
		"    async def preview_state(client: RetryableClient) -> Any:",
		"        return " + call,
		"",
	}
}

// pyPreviewStateIDs is the values the declared read's route is addressed by,
// each coerced the way this tool's own path values are, and empty for a read
// that names no slot.
func pyPreviewStateIDs(tool *pyTool) string {
	values := make([]string, 0, len(tool.c.StateRead.Slots))

	for _, slot := range tool.c.StateRead.Slots {
		argument := tool.pathArg(slot)
		values = append(values, argument.coerce+"("+pyLocal(slot)+" or "+argument.absent+")")
	}

	if len(values) == 0 {
		return ""
	}

	return strings.Join(values, ", ") + ", "
}

// pyStateQuery is the query keyword a declared fetch addresses its read with,
// empty for a read a path alone addresses.
func pyStateQuery(tool *pyTool) string {
	if len(tool.c.StateRead.Query) == 0 {
		return ""
	}

	named := make([]string, 0, len(tool.c.StateRead.Query))
	for _, entry := range tool.c.StateRead.Query {
		named = append(named, pyQuote(entry.Parameter)+": "+pyQuote(entry.Argument))
	}

	return ", query=state_read_query(arguments, {" + strings.Join(named, ", ") + "})"
}
