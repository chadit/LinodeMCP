package main

import (
	"fmt"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The declared dry-run prose, resolved once into the contract model and
// rendered by each arm as a call to its own support layer.

// previewSentence is one declared line of a dry run and the wordings it can
// take: ordered most specific first, or chosen by a flag.
type previewSentence struct {
	Choice    *previewChoice
	Templates []string
	Warning   bool
}

// previewChoice is the flag a line's wording is selected by and the wording
// each of its three states reports.
type previewChoice struct {
	// Argument is the flag as the tool spells it, and Member is the open-object
	// member it sits under, "" when the flag is an argument of its own.
	Argument string
	Member   string
	Wordings previewChoiceWordings
}

// previewChoiceWordings is what a chosen line reports in each state the flag
// can be in, any of them empty for a state that reports nothing.
type previewChoiceWordings struct {
	True   string
	False  string
	Absent string
}

// all is the three arms in the order the emitters write them.
func (w previewChoiceWordings) all() []string {
	return []string{w.True, w.False, w.Absent}
}

// previewPlaceholderOpen and previewPlaceholderClose bracket an argument name
// inside a wording.
const (
	previewPlaceholderOpen  = "{"
	previewPlaceholderClose = "}"
)

// previewChoiceSeparator divides an open-object argument from the member a
// choice selects on.
const previewChoiceSeparator = "."

// readPreviewSentences resolves the prose a tool's dry run reports.
//
// Refused beside a preview hook: the hook already answers the whole dry run, so
// a declaration next to it would either be dropped or reported twice, and
// nothing here says which.
func (c *contract) readPreviewSentences(options protoreflect.ProtoMessage) error {
	if err := c.readPreviewOmitsBody(options); err != nil {
		return err
	}

	declared, _ := proto.GetExtension(options, linodev1.E_PreviewSentence).([]*linodev1.PreviewSentence)
	if len(declared) == 0 {
		return nil
	}

	if c.Hooks.Preview != "" {
		return fmt.Errorf("%w: %s", errPreviewSentenceWithHook, c.Name)
	}

	sentences := make([]previewSentence, 0, len(declared))

	for _, entry := range declared {
		sentence, err := c.readPreviewSentence(entry)
		if err != nil {
			return err
		}

		sentences = append(sentences, sentence)
	}

	c.PreviewSentences = sentences

	return nil
}

// readPreviewOmitsBody holds the no-echo flag to the tools that can act it.
func (c *contract) readPreviewOmitsBody(options protoreflect.ProtoMessage) error {
	declared, _ := proto.GetExtension(options, linodev1.E_PreviewOmitsBody).(bool)
	if !declared {
		return nil
	}

	if c.Hooks.Preview != "" {
		return fmt.Errorf("%w: %s", errPreviewOmitsBodyWithHook, c.Name)
	}

	if len(c.Body) == 0 {
		return fmt.Errorf("%w: %s", errPreviewOmitsBodyWithoutBody, c.Name)
	}

	c.PreviewOmitsBody = true

	return nil
}

// readPreviewSentence resolves one declared line: the half of the preview it
// lands in, and the wordings it chooses between.
func (c *contract) readPreviewSentence(entry *linodev1.PreviewSentence) (previewSentence, error) {
	warning, err := c.previewLine(entry.GetLine())
	if err != nil {
		return previewSentence{}, err
	}

	if entry.GetChosenBy() != nil {
		return c.readPreviewChoice(entry, warning)
	}

	templates := entry.GetTemplate()
	if len(templates) == 0 {
		return previewSentence{}, fmt.Errorf("%w: %s", errPreviewSentenceNoTemplate, c.Name)
	}

	needed := make([][]string, 0, len(templates))

	for _, template := range templates {
		names, placeholderErr := c.previewPlaceholders(template)
		if placeholderErr != nil {
			return previewSentence{}, placeholderErr
		}

		needed = append(needed, names)
	}

	if err := c.checkPreviewOrder(needed); err != nil {
		return previewSentence{}, err
	}

	return previewSentence{Templates: templates, Warning: warning}, nil
}

// readPreviewChoice resolves a line whose wording a flag selects: the flag
// itself, and the wording each of its three states reports.
//
// The ordering rule the templates get does not apply here, since a state
// selects its arm outright rather than winning a race with the arms beside it.
func (c *contract) readPreviewChoice(entry *linodev1.PreviewSentence, warning bool) (previewSentence, error) {
	if len(entry.GetTemplate()) > 0 {
		return previewSentence{}, fmt.Errorf("%w: %s", errPreviewChoiceWithTemplate, c.Name)
	}

	declared := entry.GetChosenBy()

	wordings := previewChoiceWordings{
		True:   declared.GetWhenTrue(),
		False:  declared.GetWhenFalse(),
		Absent: declared.GetWhenAbsent(),
	}

	if wordings.True == "" && wordings.False == "" && wordings.Absent == "" {
		return previewSentence{}, fmt.Errorf("%w: %s", errPreviewChoiceNoWording, c.Name)
	}

	for _, wording := range wordings.all() {
		if _, err := c.previewPlaceholders(wording); err != nil {
			return previewSentence{}, err
		}
	}

	choice, err := c.previewChoiceFlag(declared.GetArgument())
	if err != nil {
		return previewSentence{}, err
	}

	choice.Wordings = wordings

	return previewSentence{Choice: &choice, Warning: warning}, nil
}

// previewChoiceFlag holds the selector to a bool the tool carries, either an
// argument of its own or one member of an open object it sends.
func (c *contract) previewChoiceFlag(spelled string) (previewChoice, error) {
	if spelled == "" {
		return previewChoice{}, fmt.Errorf("%w: %s", errPreviewChoiceNoArgument, c.Name)
	}

	name, member, dotted := strings.Cut(spelled, previewChoiceSeparator)

	entry := c.previewArgument(name)
	if entry == nil {
		return previewChoice{}, fmt.Errorf("%w: %s selects on %s", errPreviewChoiceUnknownArgument, c.Name, spelled)
	}

	if !dotted {
		if entry.Kind != protoreflect.BoolKind || entry.Repeated {
			return previewChoice{}, fmt.Errorf("%w: %s selects on %s", errPreviewChoiceNotFlag, c.Name, name)
		}

		return previewChoice{Argument: name}, nil
	}

	if member == "" || strings.Contains(member, previewChoiceSeparator) {
		return previewChoice{}, fmt.Errorf("%w: %s selects on %s", errPreviewChoiceDeepMember, c.Name, spelled)
	}

	if !entry.ObjectMap {
		return previewChoice{}, fmt.Errorf("%w: %s selects on %s", errPreviewChoiceNotObject, c.Name, spelled)
	}

	return previewChoice{Argument: name, Member: member}, nil
}

// previewLine says which half of the preview a declared line lands in.
func (c *contract) previewLine(line linodev1.PreviewLine) (bool, error) {
	switch line {
	case linodev1.PreviewLine_PREVIEW_LINE_SIDE_EFFECT:
		return false, nil
	case linodev1.PreviewLine_PREVIEW_LINE_WARNING:
		return true, nil
	case linodev1.PreviewLine_PREVIEW_LINE_UNSPECIFIED:
		return false, fmt.Errorf("%w: %s", errPreviewSentenceNoLine, c.Name)
	}

	return false, fmt.Errorf("%w: %s declares %s", errPreviewSentenceNoLine, c.Name, line)
}

// previewPlaceholders is the arguments one wording reads, held to being values
// the tool has and may report.
func (c *contract) previewPlaceholders(template string) ([]string, error) {
	names := make([]string, 0, strings.Count(template, previewPlaceholderOpen))
	rest := template

	for {
		_, after, found := strings.Cut(rest, previewPlaceholderOpen)
		if !found {
			break
		}

		name, remainder, closed := strings.Cut(after, previewPlaceholderClose)
		if !closed {
			return nil, fmt.Errorf("%w: %s in %q", errPreviewSentenceUnclosed, c.Name, template)
		}

		if err := c.checkPreviewArgument(name, template); err != nil {
			return nil, err
		}

		names = append(names, name)
		rest = remainder
	}

	if strings.Contains(rest, previewPlaceholderClose) {
		return nil, fmt.Errorf("%w: %s in %q", errPreviewSentenceUnclosed, c.Name, template)
	}

	return names, nil
}

// checkPreviewArgument holds one placeholder to an argument whose value a
// preview can report.
func (c *contract) checkPreviewArgument(name, template string) error {
	if !slices.Contains(c.AllArguments, name) {
		return fmt.Errorf("%w: %s reports %s", errPreviewSentenceUnknownArgument, c.Name, name)
	}

	entry := c.previewArgument(name)

	switch {
	case entry == nil || entry.Repeated:
		return fmt.Errorf("%w: %s reports %s in %q", errPreviewSentenceUnreportable, c.Name, name, template)
	case !previewReportableKind(entry.Kind):
		return fmt.Errorf("%w: %s reports %s in %q", errPreviewSentenceUnreportable, c.Name, name, template)
	case entry.Redact || namesSecret(name):
		return fmt.Errorf("%w: %s reports %s", errPreviewSentenceSecretArgument, c.Name, name)
	}

	return nil
}

// previewArgument finds a named argument wherever the tool carries it, since a
// sentence names a value rather than a place in the request.
func (c *contract) previewArgument(name string) *field {
	groups := [][]field{c.Path, c.Query, c.Body, c.Local, c.Forwarded}

	for _, group := range groups {
		index := slices.IndexFunc(group, func(entry field) bool {
			return entry.ProtoName == name
		})
		if index >= 0 {
			return &group[index]
		}
	}

	return nil
}

// previewReportableKind reports whether a value of this kind has one spelling
// both languages write it with. A float or a bool does not, so declaring one
// would put the two trees a character apart on the same call. An enum does: it
// travels as its member name.
func previewReportableKind(kind protoreflect.Kind) bool {
	reportable := []protoreflect.Kind{
		protoreflect.StringKind, protoreflect.EnumKind,
		protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Uint32Kind, protoreflect.Uint64Kind,
	}

	return slices.Contains(reportable, kind)
}

// checkPreviewOrder holds the wordings to being reachable: the first one whose
// placeholders all carry a value wins, so a later wording asking for a superset
// of an earlier one's arguments could never be the one reported.
func (c *contract) checkPreviewOrder(needed [][]string) error {
	for later := 1; later < len(needed); later++ {
		for earlier := range later {
			if previewCovers(needed[later], needed[earlier]) {
				return fmt.Errorf("%w: %s", errPreviewSentenceUnreachable, c.Name)
			}
		}
	}

	return nil
}

// previewCovers reports whether every argument the earlier wording reads is one
// the later wording reads too, which is what makes the later one unreachable.
func previewCovers(later, earlier []string) bool {
	for _, name := range earlier {
		if !slices.Contains(later, name) {
			return false
		}
	}

	return true
}

// previewValueArguments is every argument the tool's declared prose reads, in
// first-mention order, which is the order each arm builds its value map in.
func (c *contract) previewValueArguments() []string {
	names := make([]string, 0)

	for _, sentence := range c.PreviewSentences {
		for _, wording := range sentence.wordings() {
			read, err := c.previewPlaceholders(wording)
			if err != nil {
				continue
			}

			for _, name := range read {
				if !slices.Contains(names, name) {
					names = append(names, name)
				}
			}
		}
	}

	return names
}

// wordings is every wording one line can report, however it selects between
// them, which is what lets one walk collect the placeholders of both shapes.
func (s *previewSentence) wordings() []string {
	if s.Choice != nil {
		return s.Choice.Wordings.all()
	}

	return s.Templates
}

// declaresPreview reports whether the contract answers the tool's dry run
// itself, which is what makes the emitted branch a declared preview rather than
// a hook call or the plain report of the request.
//
// A declared state read counts on its own: the resource a dry run reports is
// half of what a preview says, and a tool can declare that half while saying
// nothing about the change.
func (c *contract) declaresPreview() bool {
	return len(c.PreviewSentences) > 0 || c.previewReadsState()
}

// previewBodyExpression is what a declared preview reports as the request, which
// is nothing for the families whose preview predates the echo.
func (c *contract) previewBodyExpression(body string) string {
	if c.PreviewOmitsBody {
		return nilLiteral
	}

	return body
}

// emitDeclaredPreview writes the dry-run branch of a tool whose prose the
// contract declares: the values its wordings read, then the call that reports
// them beside the request, with the declared state read under it.
func emitDeclaredPreview(out *source, tool *contract, ordered []field, method, path, body string) {
	emitPreviewValues(out, tool)

	if !tool.previewReadsState() {
		out.writef("\treturn tools.RunDeclaredPreview(ctx, request, cfg, %s, %s, %s, %s, nil, &tools.DryRunDetails{",
			goStringLiteral(tool.Name), method, path, tool.previewBodyExpression(body))

		emitPreviewLines(out, tool, "SideEffects", false, "\t\t")
		emitPreviewLines(out, tool, "Warnings", true, "\t\t")

		out.writef("\t})")

		return
	}

	out.writef("\treturn tools.RunDeclaredPreview(ctx, request, cfg, %s, %s, %s, %s,",
		goStringLiteral(tool.Name), method, path, tool.previewBodyExpression(body))

	emitPreviewStateRead(out, tool, ordered)

	out.writef("\t\t&tools.DryRunDetails{")

	emitPreviewLines(out, tool, "SideEffects", false, "\t\t\t")
	emitPreviewLines(out, tool, "Warnings", true, "\t\t\t")

	out.writef("\t\t})")
}

// emitPreviewStateRead writes the fetch a declared preview reports the resource
// through: the declared read's route, decoded into the message that read
// answers with, reported as the body the API sent projected through it.
//
// The decode stays because it is what refuses a body the read's contract does
// not describe; what the preview reports is the projection, so a key the API
// sent as null survives and one it never sent is not invented.
func emitPreviewStateRead(out *source, tool *contract, ordered []field) {
	out.need(importGenpb, importLinode)

	values := pathValuesLiteral(tool.stateRouteFields(ordered))

	out.writef("\t\tfunc(ctx context.Context, client *linode.Client) (any, error) {")
	out.writef("\t\t\tstate := &linodev1.%s{}", tool.StateRead.Message.TypeName)
	out.writef("")

	out.writef("\t\t\t%s", goStateReadCall(tool, values))
	out.writef("\t\t\tif err != nil {")
	out.writef("\t\t\t\treturn nil, err")
	out.writef("\t\t\t}")
	out.writef("")
	out.writef("\t\t\treturn tools.ProjectDeclaredState(raw, state)")
	out.writef("\t\t},")
}

// goStateReadCall is the read a declared fetch performs, which takes the query
// twin for the routes a path alone does not address.
func goStateReadCall(tool *contract, values string) string {
	read, query := "CallProtoRouteState", ""

	if addressed := goStateReadQuery(tool); addressed != "" {
		read, query = "CallProtoRouteStateQuery", addressed+", "
	}

	return fmt.Sprintf("raw, err := client.%s(ctx, %s, %s, %s%s, state)",
		read, goStringLiteral(tool.StateRead.Tool), values, query,
		goStringLiteral(tool.StateRead.BodyKey))
}

// goStateReadQuery is the query a declared fetch addresses its read with, "" for
// a read a path alone addresses.
//
// The parameters are named in the read's own spelling and read off this tool's
// arguments, which is what the support layer's query builder takes.
func goStateReadQuery(tool *contract) string {
	if len(tool.StateRead.Query) == 0 {
		return ""
	}

	named := make([]string, 0, len(tool.StateRead.Query))
	for _, entry := range tool.StateRead.Query {
		named = append(named, goStringLiteral(entry.Parameter)+": "+goStringLiteral(entry.Argument))
	}

	return "tools.StateReadQuery(request, map[string]string{" + strings.Join(named, ", ") + "})"
}

// emitPreviewValues writes the map a wording reads its placeholders out of,
// built once because several wordings of one line read the same argument.
func emitPreviewValues(out *source, tool *contract) {
	names := tool.previewValueArguments()
	if len(names) == 0 {
		return
	}

	out.writef("\tpreviewValues := map[string]string{")

	for _, name := range names {
		out.writef("\t\t%s: tools.%s(request, %s),",
			goStringLiteral(name), goPreviewReader(tool.previewArgument(name)), goStringLiteral(name))
	}

	out.writef("\t}")
	out.writef("")
}

// emitPreviewLines writes one half of the declared prose at the column the
// details literal sits at, leaving the member absent when the tool declares no
// line for it.
func emitPreviewLines(out *source, tool *contract, member string, warnings bool, indent string) {
	rendered := make([]string, 0, len(tool.PreviewSentences))

	for i := range tool.PreviewSentences {
		sentence := &tool.PreviewSentences[i]
		if sentence.Warning != warnings {
			continue
		}

		rendered = append(rendered, previewSentenceExpression(tool, sentence))
	}

	if len(rendered) == 0 {
		return
	}

	out.writef("%s%s: []string{", indent, member)

	for _, expression := range rendered {
		out.writef("%s\t%s,", indent, expression)
	}

	out.writef("%s},", indent)
}

// previewSentenceExpression is one declared line as the handler reports it: the
// wording itself when there is nothing to choose between and nothing to fill in,
// and the support call that chooses and fills otherwise.
func previewSentenceExpression(tool *contract, sentence *previewSentence) string {
	if sentence.Choice != nil {
		return previewChoiceExpression(tool, sentence.Choice)
	}

	only := sentence.Templates[0]
	if len(sentence.Templates) == 1 && !strings.Contains(only, previewPlaceholderOpen) {
		return goStringLiteral(only)
	}

	quoted := make([]string, 0, len(sentence.Templates)+1)
	quoted = append(quoted, "previewValues")

	for _, template := range sentence.Templates {
		quoted = append(quoted, goStringLiteral(template))
	}

	return "tools.PreviewSentence(" + strings.Join(quoted, ", ") + ")"
}

// previewChoiceExpression is one chosen line as the handler reports it: the
// flag read in its three states, then the arm that state selects filled in.
func previewChoiceExpression(tool *contract, choice *previewChoice) string {
	wordings := choice.Wordings.all()

	parts := make([]string, 0, len(wordings)+2)
	parts = append(parts, goPreviewValues(tool), goPreviewFlagRead(choice))

	for _, wording := range wordings {
		parts = append(parts, goStringLiteral(wording))
	}

	return "tools.PreviewChosen(" + strings.Join(parts, ", ") + ")"
}

// goPreviewValues is the map a wording reads its placeholders out of, and nil
// for a tool whose declared prose reads no argument at all: a chosen line can
// be three fixed sentences, which name nothing to look up.
func goPreviewValues(tool *contract) string {
	if len(tool.previewValueArguments()) == 0 {
		return nilLiteral
	}

	return "previewValues"
}

// goPreviewFlagRead is the Go arm's read of the flag a line selects on.
func goPreviewFlagRead(choice *previewChoice) string {
	if choice.Member == "" {
		return "tools.PreviewFlag(request, " + goStringLiteral(choice.Argument) + ")"
	}

	return "tools.PreviewMemberFlag(request, " + goStringLiteral(choice.Argument) +
		", " + goStringLiteral(choice.Member) + ")"
}

// previewFillsWording reports whether any declared line needs the support layer
// to pick between ordered wordings or fill one in, which is the half of the
// prose the sentence chooser answers.
func (c *contract) previewFillsWording() bool {
	for i := range c.PreviewSentences {
		sentence := &c.PreviewSentences[i]
		if sentence.Choice != nil {
			continue
		}

		if len(sentence.Templates) > 1 || strings.Contains(sentence.Templates[0], previewPlaceholderOpen) {
			return true
		}
	}

	return false
}

// previewChoices is the flags this tool's declared lines select on, one per
// chosen line.
func (c *contract) previewChoices() []*previewChoice {
	chosen := make([]*previewChoice, 0, len(c.PreviewSentences))

	for i := range c.PreviewSentences {
		if choice := c.PreviewSentences[i].Choice; choice != nil {
			chosen = append(chosen, choice)
		}
	}

	return chosen
}

// previewReader is the support-layer reader one argument's value is read
// through, which is decided by its declared kind: a label that arrived as a
// number is not a label, and the hooks this replaces read it that way.
func previewReader(entry *field) (string, string) {
	// An enum argument arrives as the member's own name, which is text on the
	// wire and is what the hooks this replaces read it as.
	if entry != nil && (entry.Kind == protoreflect.StringKind || entry.Kind == protoreflect.EnumKind) {
		return "PreviewText", "preview_text"
	}

	return "PreviewNumber", "preview_number"
}

// goPreviewReader is the Go half of the reader table.
func goPreviewReader(entry *field) string {
	name, _ := previewReader(entry)

	return name
}

// pyPreviewReader is the Python half of the reader table.
func pyPreviewReader(entry *field) string {
	_, name := previewReader(entry)

	return name
}

// emitBodyBuild writes the shared body build and the refusal it can answer
// with.
//
// A declared preview that reports no body still runs the builder, since the
// builder is what refuses a member the call cannot send, but it keeps neither
// the body nor the verdict past the check: an assignment naming only blanks
// would not compile beside the checks above it. echoes tells the two branches
// apart, because only the dry-run branch can be the one dropping the body.
func emitBodyBuild(out *source, tool *contract, echoes bool) {
	if !echoes && tool.PreviewOmitsBody {
		out.writef("\tif _, message := %s(request); message != \"\" {", bodyName(tool.Name))
		out.writef("\t\treturn mcp.NewToolResultError(message), nil")
		out.writef("\t}")
		out.writef("")

		return
	}

	out.writef("\tbody, message := %s(request)", bodyName(tool.Name))
	out.writef("\tif message != \"\" {")
	out.writef("\t\treturn mcp.NewToolResultError(message), nil")
	out.writef("\t}")
	out.writef("")
}
