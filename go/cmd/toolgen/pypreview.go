package main

import "strings"

// The Python arm's rendering of the declared dry-run prose: the same wordings
// the Go arm writes, handed to the driver rather than to a builder, since this
// arm's driver is where a preview is assembled.

// The columns a declared sentence is written at: one level inside the driver
// keyword for a bare wording, two for one the support layer chooses.
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

	lines := []string{"    preview_values = {"}

	for _, name := range names {
		reader := pyPreviewReader(tool.c.previewArgument(name))
		lines = append(lines, "        "+pyQuote(name)+": "+reader+"(arguments, "+pyQuote(name)+"),")
	}

	return append(lines, "    }", "")
}

// pyPreviewLines is one half of the declared prose as a driver keyword, and no
// lines at all when the tool declares none for that half.
func pyPreviewLines(tool *pyTool, keyword string, warnings bool) []string {
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

// pyPreviewSentence is one declared line as the driver receives it: the wording
// itself when there is nothing to choose between and nothing to fill in, and
// the support call that chooses and fills otherwise.
func pyPreviewSentence(tool *pyTool, sentence *previewSentence) []string {
	if sentence.Choice != nil {
		return pyPreviewChoice(tool, sentence.Choice)
	}

	only := sentence.Templates[0]
	if len(sentence.Templates) == 1 && !strings.Contains(only, previewPlaceholderOpen) {
		return pyCommaTerminated(pyQuoteWrapped(only, pyPreviewSentenceIndent))
	}

	lines := []string{
		"            preview_sentence(",
		"                preview_values,",
	}

	for _, template := range sentence.Templates {
		lines = append(lines, pyCommaTerminated(pyQuoteWrapped(template, pyPreviewTemplateIndent))...)
	}

	return append(lines, "            ),")
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
		lines = append(lines, pyCommaTerminated(pyQuoteWrapped(wording, pyPreviewTemplateIndent))...)
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

// pyCommaTerminated puts the separator on the last line of a rendered literal,
// which is where it belongs when the literal was split across several.
func pyCommaTerminated(lines []string) []string {
	lines[len(lines)-1] += ","

	return lines
}

// pyDeclaredPreview is the driver keywords a tool's declared prose renders to:
// each half of it, the state read it reports the resource through, and the
// choice to report no request body for the families whose preview predates the
// echo.
func pyDeclaredPreview(tool *pyTool) []string {
	if !tool.c.declaresPreview() {
		return nil
	}

	lines := pyPreviewLines(tool, "side_effects", false)
	lines = append(lines, pyPreviewLines(tool, "warnings", true)...)

	if tool.c.previewReadsState() {
		lines = append(lines, "        state_fetch=preview_state,")
	}

	if tool.c.PreviewOmitsBody {
		lines = append(lines, "        preview_request_body=False,")
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
