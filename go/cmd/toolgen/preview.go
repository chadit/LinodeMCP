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
	Choice *previewChoice
	// Match is the value a line's wording is selected by, nil for a line that
	// selects on nothing.
	Match *previewMatch
	// Changed is the pair a line naming only the new value is reported for,
	// nil for a line every call reports.
	Changed *previewUnchanged
	// Elements is the repeated argument a line is written over, nil for a line
	// written once.
	Elements  *previewElements
	Templates []string
	// Present names the arguments the call must carry for this line to be
	// reported, empty for a line every call reports.
	Present []string
	Warning bool
}

// previewUnchanged is one state reading a wording reports only where it differs
// from the argument the call would set.
type previewUnchanged struct {
	// State is the placeholder name a wording spells, prefix included, so it
	// looks up in the same map the readers fill.
	State    string
	Argument string
}

// previewMatch is the value a line's wording is selected by and the wording
// each value it names reports. Exactly one of Argument and State is set; a
// State reading is compared case-folded, so its arms are held folded too.
type previewMatch struct {
	Argument  string
	State     string
	Arms      map[string]string
	Otherwise string
	// Order is the arm values in declaration order, so the emitted map literal
	// is written the same way on every run.
	Order []string
}

// wordings is every wording a match can report, which is what lets one walk
// collect the placeholders of all of them.
func (m *previewMatch) wordings() []string {
	all := make([]string, 0, len(m.Order)+1)
	for _, value := range m.Order {
		all = append(all, m.Arms[value])
	}

	return append(all, m.Otherwise)
}

// previewElements is the repeated argument one line is written over, and
// whether it writes a line per entry or one line naming them all.
type previewElements struct {
	Argument string
	Join     string
}

// previewStandIn is one member of a repeated argument's entries the reported
// body carries fixed text for. Argument is the wire key, since the reported
// body is walked by the names it was built under rather than by argument name.
type previewStandIn struct {
	Argument string
	Member   string
	Text     string
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

// previewStateLocal is the local a preview's own wordings read their state
// through, which is what the emitted walk names something else.
const previewStateLocal = "state"

// previewStatePrefix marks a placeholder that reads the resource the preview
// fetched rather than an argument the call carried. The rest of the name is the
// member's own path through the declared read's message.
const previewStatePrefix = "state:"

// previewTransportPrefix marks a placeholder read off what the tool's presigned
// upload measured before the call, spelled after the transport's own member:
// `{transport:size_bytes}`. Declaring one is what makes the dry run inspect the
// source, fill the presign body the way the live call does, and warn about a
// source the guard refuses.
const previewTransportPrefix = "transport:"

// previewTransportLocal is the name the Go arm binds the measurement to inside
// the walk a transfer preview words its lines in.
const previewTransportLocal = "transfer"

// previewElementName is the placeholder a per-element line's entry fills. It
// matches tools.PreviewElementName, which is what the support layer looks it up
// under.
const previewElementName = "element"

// readPreviewSentences resolves the prose a tool's dry run reports.
func (c *contract) readPreviewSentences(options protoreflect.ProtoMessage) error {
	if err := c.readPreviewOmitsBody(options); err != nil {
		return err
	}

	if err := c.readPreviewUnchanged(options); err != nil {
		return err
	}

	if err := c.readPreviewStandIns(options); err != nil {
		return err
	}

	declared, _ := proto.GetExtension(options, linodev1.E_PreviewSentence).([]*linodev1.PreviewSentence)
	if len(declared) == 0 {
		return nil
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

// readPreviewStandIns resolves the members of a repeated argument's entries the
// reported body carries fixed text for.
func (c *contract) readPreviewStandIns(options protoreflect.ProtoMessage) error {
	declared, _ := proto.GetExtension(options, linodev1.E_PreviewStandIn).([]*linodev1.PreviewStandIn)
	if len(declared) == 0 {
		return nil
	}

	stood := make([]previewStandIn, 0, len(declared))

	for _, entry := range declared {
		one, err := c.readPreviewStandIn(entry)
		if err != nil {
			return err
		}

		if slices.Contains(stood, one) {
			return fmt.Errorf("%w: %s stands in for %s.%s",
				errPreviewStandInRepeated, c.Name, one.Argument, one.Member)
		}

		stood = append(stood, one)
	}

	c.PreviewStandIns = stood

	return nil
}

// readPreviewStandIn resolves one stand-in: the repeated argument, the member
// its entries declare, and the text reported in that member's place.
func (c *contract) readPreviewStandIn(entry *linodev1.PreviewStandIn) (previewStandIn, error) {
	name := entry.GetArgument()

	index := slices.IndexFunc(c.Body, func(argument field) bool { return argument.ProtoName == name })
	if index < 0 || c.Body[index].ItemMessage == nil {
		return previewStandIn{}, fmt.Errorf("%w: %s stands in over %s",
			errPreviewStandInNotItemList, c.Name, name)
	}

	member := entry.GetMember()
	if c.Body[index].ItemMessage.Fields().ByName(protoreflect.Name(member)) == nil {
		return previewStandIn{}, fmt.Errorf("%w: %s stands in over %s.%s",
			errPreviewStandInUnknownMember, c.Name, name, member)
	}

	if entry.GetText() == "" {
		return previewStandIn{}, fmt.Errorf("%w: %s stands in over %s.%s",
			errPreviewStandInNoText, c.Name, name, member)
	}

	return previewStandIn{
		Argument: c.Body[index].wireName(), Member: member, Text: entry.GetText(),
	}, nil
}

// readPreviewOmitsBody holds the no-echo flag to the tools that can act it.
func (c *contract) readPreviewOmitsBody(options protoreflect.ProtoMessage) error {
	declared, _ := proto.GetExtension(options, linodev1.E_PreviewOmitsBody).(bool)
	if !declared {
		return nil
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
		// Held before the arm resolves, since the arm never reads a guard and a
		// declaration carrying both would otherwise pass with one ignored.
		if len(entry.GetWhenPresent()) > 0 {
			return previewSentence{}, fmt.Errorf("%w: %s", errPreviewPresentWithChoice, c.Name)
		}

		if entry.GetMatchedBy() != nil {
			return previewSentence{}, fmt.Errorf("%w: %s", errPreviewMatchWithChoice, c.Name)
		}

		return c.readPreviewChoice(entry, warning)
	}

	if entry.GetMatchedBy() != nil {
		return c.readPreviewMatch(entry, warning)
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

	if orderErr := c.checkPreviewOrder(needed); orderErr != nil {
		return previewSentence{}, orderErr
	}

	present, presentErr := c.readPreviewPresent(entry)
	if presentErr != nil {
		return previewSentence{}, presentErr
	}

	changed, declared, changedErr := c.readPreviewChanged(entry)
	if changedErr != nil {
		return previewSentence{}, changedErr
	}

	elements, writesEach, elementErr := c.readPreviewElements(entry, templates)
	if elementErr != nil {
		return previewSentence{}, elementErr
	}

	return previewSentence{
		Templates: templates, Present: present, Elements: previewElementList(elements, writesEach),
		Changed: previewChangedPair(changed, declared), Warning: warning,
	}, nil
}

// readPreviewElements resolves the repeated argument a line is written over,
// and holds the entry placeholder to a line that declares one.
func (c *contract) readPreviewElements(
	entry *linodev1.PreviewSentence, templates []string,
) (previewElements, bool, error) {
	declared := entry.GetPerElement()

	reads := slices.ContainsFunc(templates, func(template string) bool {
		return strings.Contains(template, previewPlaceholderOpen+previewElementName+previewPlaceholderClose)
	})

	if declared == nil {
		if reads {
			return previewElements{}, false, fmt.Errorf("%w: %s", errPreviewElementWithoutList, c.Name)
		}

		return previewElements{}, false, nil
	}

	if !reads {
		return previewElements{}, false, fmt.Errorf("%w: %s", errPreviewElementUnread, c.Name)
	}

	name := declared.GetArgument()

	field := c.previewArgument(name)
	if field == nil || !field.Repeated {
		return previewElements{}, false, fmt.Errorf("%w: %s writes over %s", errPreviewElementNotRepeated, c.Name, name)
	}

	if !previewReportableKind(field.Kind) {
		return previewElements{}, false, fmt.Errorf("%w: %s writes over %s", errPreviewElementUnreportable, c.Name, name)
	}

	if field.Redact || namesSecret(name) {
		return previewElements{}, false, fmt.Errorf("%w: %s writes over %s", errPreviewSentenceSecretArgument, c.Name, name)
	}

	return previewElements{Argument: name, Join: declared.GetJoin()}, true, nil
}

// previewElementList is the list a line carries, and nil for a line written
// once.
func previewElementList(list previewElements, declared bool) *previewElements {
	if !declared {
		return nil
	}

	return &list
}

// previewChangedPair is the pair a line carries, and nil for a line that
// declared none.
func previewChangedPair(pair previewUnchanged, declared bool) *previewUnchanged {
	if !declared {
		return nil
	}

	return &pair
}

// readPreviewMatch resolves a line whose wording a value selects: the argument
// that selects, and the wording each value it names reports.
//
// The ordering rule the templates get does not apply, since a value picks its
// arm outright rather than winning a race with the arms beside it.
func (c *contract) readPreviewMatch(entry *linodev1.PreviewSentence, warning bool) (previewSentence, error) {
	if len(entry.GetTemplate()) > 0 {
		return previewSentence{}, fmt.Errorf("%w: %s", errPreviewMatchWithTemplate, c.Name)
	}

	declared := entry.GetMatchedBy()

	match, err := c.readPreviewMatchArms(declared)
	if err != nil {
		return previewSentence{}, err
	}

	present, err := c.readPreviewPresent(entry)
	if err != nil {
		return previewSentence{}, err
	}

	changed, pairs, err := c.readPreviewChanged(entry)
	if err != nil {
		return previewSentence{}, err
	}

	return previewSentence{
		Match: &match, Present: present,
		Changed: previewChangedPair(changed, pairs), Warning: warning,
	}, nil
}

// readPreviewMatchArms resolves the value a match selects on and the wordings
// its arms report.
func (c *contract) readPreviewMatchArms(declared *linodev1.PreviewMatch) (previewMatch, error) {
	match, err := c.previewMatchSelector(declared)
	if err != nil {
		return previewMatch{}, err
	}

	for _, arm := range declared.GetArm() {
		value, wording := arm.GetEquals(), arm.GetWording()
		if value == "" || wording == "" {
			return previewMatch{}, fmt.Errorf("%w: %s", errPreviewMatchArmEmpty, c.Name)
		}

		// A state reading is compared case-folded, so the arms are held folded
		// and two that fold alike are the repeat the exact rule already names.
		if match.State != "" {
			value = strings.ToLower(value)
		}

		if _, named := match.Arms[value]; named {
			return previewMatch{}, fmt.Errorf("%w: %s names %s twice", errPreviewMatchArmRepeated, c.Name, value)
		}

		match.Arms[value] = wording
		match.Order = append(match.Order, value)
	}

	if len(match.Order) == 0 {
		return previewMatch{}, fmt.Errorf("%w: %s", errPreviewMatchNoArm, c.Name)
	}

	for _, wording := range match.wordings() {
		if _, err := c.previewPlaceholders(wording); err != nil {
			return previewMatch{}, err
		}
	}

	return match, nil
}

// previewMatchSelector resolves what a matched line selects on: an argument the
// call carries, or a reading of the resource the preview fetched.
func (c *contract) previewMatchSelector(declared *linodev1.PreviewMatch) (previewMatch, error) {
	argument, state := declared.GetArgument(), declared.GetState()

	empty := previewMatch{
		Arms:      make(map[string]string, len(declared.GetArm())),
		Otherwise: declared.GetOtherwise(),
	}

	if (argument == "") == (state == "") {
		return previewMatch{}, fmt.Errorf("%w: %s", errPreviewMatchSelector, c.Name)
	}

	// The reading itself is held to the declared read in the resolve pass, the
	// way every other state placeholder is: nothing is in scope to look one up
	// while the declarations are still being read.
	if state != "" {
		empty.State = previewStatePrefix + state

		return empty, nil
	}

	if err := c.checkPreviewArgument(argument, argument); err != nil {
		return previewMatch{}, fmt.Errorf("%w: %s selects on %s", errPreviewMatchArgument, c.Name, argument)
	}

	empty.Argument = argument

	return empty, nil
}

// readPreviewChanged resolves the pair a line naming only the new value is
// reported for.
func (c *contract) readPreviewChanged(entry *linodev1.PreviewSentence) (previewUnchanged, bool, error) {
	declared := entry.GetWhenChanged()
	if declared == nil {
		return previewUnchanged{}, false, nil
	}

	argument := declared.GetArgument()
	if err := c.checkPreviewArgument(argument, argument); err != nil {
		return previewUnchanged{}, false, fmt.Errorf("%w: %s holds %s",
			errPreviewChangedUnknownArgument, c.Name, argument)
	}

	return previewUnchanged{
		State:    previewStatePrefix + declared.GetState(),
		Argument: argument,
	}, true, nil
}

// readPreviewPresent holds a line's guard to arguments the tool carries and may
// report, which is the same bar a wording's placeholder answers to.
func (c *contract) readPreviewPresent(entry *linodev1.PreviewSentence) ([]string, error) {
	guarded := entry.GetWhenPresent()
	if len(guarded) == 0 {
		return nil, nil
	}

	for _, name := range guarded {
		if err := c.checkPreviewGuardArgument(name); err != nil {
			return nil, err
		}
	}

	return guarded, nil
}

// checkPreviewGuardArgument holds a guard to an argument the tool carries.
//
// The reportability bar a placeholder answers to does not apply: a guard asks
// whether the call sent the argument and never spells its value, which is what
// lets a line be guarded on a list or an open object neither language could
// agree on the wording of. A credential is still refused, since naming one even
// as a condition says whether it was sent.
func (c *contract) checkPreviewGuardArgument(name string) error {
	entry := c.previewArgument(name)
	if entry == nil {
		return fmt.Errorf("%w: %s guards on %s", errPreviewSentenceUnknownArgument, c.Name, name)
	}

	if entry.Redact || namesSecret(name) {
		return fmt.Errorf("%w: %s guards on %s", errPreviewSentenceSecretArgument, c.Name, name)
	}

	return nil
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

		if err := c.checkPreviewPlaceholder(name, template); err != nil {
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

// checkPreviewPlaceholder holds one placeholder to something a preview can
// report: a transport measurement, or a value checkPreviewArgument accepts.
func (c *contract) checkPreviewPlaceholder(name, template string) error {
	if member, reads := strings.CutPrefix(name, previewTransportPrefix); reads {
		return c.checkPreviewTransportMember(member)
	}

	return c.checkPreviewArgument(name, template)
}

// checkPreviewTransportMember holds a transport placeholder to the one reading
// a dry run takes before the call: the size of a presigned upload's source. A
// tool with no such transport has nothing to measure, and any other member is
// filled by the transfer itself, which a preview never makes.
func (c *contract) checkPreviewTransportMember(member string) error {
	if !c.transported() || c.Transport.Kind != transportPresign || !c.Transport.Up {
		return fmt.Errorf("%w: %s", errPreviewTransportNotUpload, c.Name)
	}

	if member != c.Transport.SizeField {
		return fmt.Errorf("%w: %s reads %s", errPreviewTransportMember, c.Name, member)
	}

	return nil
}

// checkPreviewArgument holds one placeholder to a value a preview can report:
// an argument the call carried, or a member of the resource it read.
func (c *contract) checkPreviewArgument(name, template string) error {
	// A state placeholder is held against the read's own message once that read
	// is resolved, which is after every tool's options have been read.
	if strings.HasPrefix(name, previewStatePrefix) {
		return nil
	}

	// The entry a per-element line is written over, which is the one name a
	// wording reads that no argument carries. A line declaring none is refused
	// below, since nothing would fill it.
	if name == previewElementName {
		return nil
	}

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

// readPreviewUnchanged records the readings a wording reports only where they
// differ from the argument the call would set.
//
// The pair is held against the prose in resolvePreviewState, once the read it
// names a member of has been resolved.
func (c *contract) readPreviewUnchanged(options protoreflect.ProtoMessage) error {
	declared, _ := proto.GetExtension(options, linodev1.E_PreviewUnchanged).([]*linodev1.PreviewUnchanged)
	if len(declared) == 0 {
		return nil
	}

	pairs := make([]previewUnchanged, 0, len(declared))

	for _, entry := range declared {
		argument := entry.GetArgument()
		if err := c.checkPreviewArgument(argument, argument); err != nil {
			return fmt.Errorf("%w: %s holds %s", errPreviewUnchangedUnknownArgument, c.Name, argument)
		}

		pairs = append(pairs, previewUnchanged{
			State:    previewStatePrefix + entry.GetState(),
			Argument: argument,
		})
	}

	c.PreviewUnchanged = pairs

	return nil
}

// checkPreviewUnchanged holds each pair to a reading some wording actually
// names, since a rule no wording reads would never change an answer.
func (c *contract) checkPreviewUnchanged() error {
	read := c.previewValueArguments()

	for _, pair := range c.PreviewUnchanged {
		if !slices.Contains(read, pair.State) {
			return fmt.Errorf("%w: %s names %s", errPreviewUnchangedUnread, c.Name, pair.State)
		}
	}

	return nil
}

// checkPreviewElementHalves holds a per-entry line to being the only line in
// its half. One call answers with a list and the other with a string, and
// nothing declared says how the two would join.
func (c *contract) checkPreviewElementHalves() error {
	for _, warnings := range []bool{false, true} {
		if previewPerElementLine(c, warnings) == nil {
			continue
		}

		var lines int

		for i := range c.PreviewSentences {
			if c.PreviewSentences[i].Warning == warnings {
				lines++
			}
		}

		if lines > 1 {
			return fmt.Errorf("%w: %s", errPreviewElementBesideLine, c.Name)
		}
	}

	return nil
}

// checkPreviewChanged holds each when_changed pair to a reading the declared
// read answers with, and refuses one the tool also steps a wording aside on.
func (c *contract) checkPreviewChanged() error {
	for i := range c.PreviewSentences {
		pair := c.PreviewSentences[i].Changed
		if pair == nil {
			continue
		}

		if c.previewUnchangedFor(pair.State) != "" {
			return fmt.Errorf("%w: %s names %s", errPreviewChangedWithUnchanged, c.Name, pair.State)
		}

		path := strings.TrimPrefix(pair.State, previewStatePrefix)
		if err := c.checkPreviewStatePath(path, pair.State); err != nil {
			return err
		}
	}

	return nil
}

// previewUnchangedFor is the argument a state reading is held against, and ""
// for a reading the tool reports however it compares.
func (c *contract) previewUnchangedFor(state string) string {
	for _, pair := range c.PreviewUnchanged {
		if pair.State == state {
			return pair.Argument
		}
	}

	return ""
}

// checkPreviewNumberGuards holds a guarded number to being read one way. A
// number named in one line's when_present reports zero as a value, so reading it
// unguarded elsewhere would report the same argument two ways.
func (c *contract) checkPreviewNumberGuards() error {
	for _, name := range c.previewGuardedNumbers() {
		for i := range c.PreviewSentences {
			sentence := &c.PreviewSentences[i]
			if slices.Contains(sentence.Present, name) {
				continue
			}

			if c.previewSentenceReads(sentence, name) {
				return fmt.Errorf("%w: %s reads %s", errPreviewNumberGuardMixed, c.Name, name)
			}
		}
	}

	return nil
}

// previewGuardedNumbers is every whole-number argument some line guards on
// being carried, which is what lets its zero report as a value.
func (c *contract) previewGuardedNumbers() []string {
	guarded := make([]string, 0)

	for i := range c.PreviewSentences {
		for _, name := range c.PreviewSentences[i].Present {
			entry := c.previewArgument(name)
			if entry == nil || goPreviewReader(entry) == previewTextReader {
				continue
			}

			if !slices.Contains(guarded, name) {
				guarded = append(guarded, name)
			}
		}
	}

	return guarded
}

// previewSentenceReads reports whether any wording of one line names an
// argument.
func (c *contract) previewSentenceReads(sentence *previewSentence, name string) bool {
	for _, wording := range sentence.wordings() {
		read, err := c.previewPlaceholders(wording)
		if err != nil {
			continue
		}

		if slices.Contains(read, name) {
			return true
		}
	}

	return false
}

// previewCarriedNumber reports whether an argument is a number every line
// reading it has already guarded on being carried, so its zero is a value.
func (c *contract) previewCarriedNumber(name string) bool {
	return slices.Contains(c.previewGuardedNumbers(), name)
}

// resolvePreviewState holds every state placeholder the tool's prose reads to a// resolvePreviewState holds every state placeholder the tool's prose reads to a
// member of the resource its declared read answers with.
//
// It runs after the read is resolved, since the message a placeholder names its
// member of is not known until then.
func (c *contract) resolvePreviewState() error {
	if err := c.checkPreviewElementHalves(); err != nil {
		return err
	}

	if c.previewReadsTransport() && c.previewReadsState() {
		return fmt.Errorf("%w: %s", errPreviewTransportWithState, c.Name)
	}

	if err := c.checkPreviewUnchanged(); err != nil {
		return err
	}

	if err := c.checkPreviewChanged(); err != nil {
		return err
	}

	if err := c.checkPreviewNumberGuards(); err != nil {
		return err
	}

	for _, name := range c.previewValueArguments() {
		path, reads := strings.CutPrefix(name, previewStatePrefix)
		if !reads {
			continue
		}

		if err := c.checkPreviewStatePath(path, name); err != nil {
			return err
		}
	}

	return nil
}

// checkPreviewStatePath holds one state placeholder to a member of the resource
// the tool's declared read answers with.
//
// A tool with no declared read is refused rather than emitted against a nil
// state: the prose would report a gap on every call, which reads as a resource
// carrying nothing rather than as the missing declaration it is.
func (c *contract) checkPreviewStatePath(path, template string) error {
	if !c.previewReadsState() {
		return fmt.Errorf("%w: %s reports %s%s with no declared state read",
			errPreviewSentenceNoState, c.Name, previewStatePrefix, path)
	}

	head, member, dotted := strings.Cut(path, previewChoiceSeparator)

	message := &c.StateRead.Message

	if dotted {
		if member == "" || strings.Contains(member, previewChoiceSeparator) {
			return fmt.Errorf("%w: %s reports %s%s", errPreviewStateDeepMember, c.Name, previewStatePrefix, path)
		}

		inner, err := previewStateMember(message, head)
		if err != nil {
			return fmt.Errorf("%w: %s reports %s%s in %q", err, c.Name, previewStatePrefix, path, template)
		}

		message, head = &inner, member
	}

	if head == "" {
		return fmt.Errorf("%w: %s reports %s%s", errPreviewStateDeepMember, c.Name, previewStatePrefix, path)
	}

	kind, declared := message.kindOf(head)

	switch {
	case !declared || message.isList(head):
		return fmt.Errorf("%w: %s reports %s%s in %q",
			errPreviewStateUnknownMember, c.Name, previewStatePrefix, path, template)
	case !previewReportableKind(kind):
		return fmt.Errorf("%w: %s reports %s%s in %q",
			errPreviewSentenceUnreportable, c.Name, previewStatePrefix, path, template)
	case namesSecret(head):
		return fmt.Errorf("%w: %s reports %s%s",
			errPreviewSentenceSecretArgument, c.Name, previewStatePrefix, path)
	}

	return nil
}

// previewStateMember is the message one singular member of a state carries,
// which is as far as a two-level path reaches.
func previewStateMember(message *goMessage, name string) (goMessage, error) {
	for _, member := range message.messageFields() {
		if member.protoName == name {
			return lookupGoMessage(member.message)
		}
	}

	return goMessage{}, errPreviewStateUnknownMember
}

// previewStateReader is the support-layer reader one state member is read
// through, decided by its declared kind the way an argument's reader is.
func (c *contract) previewStateReader(path string) (string, string) {
	head, member, dotted := strings.Cut(path, previewChoiceSeparator)

	message := &c.StateRead.Message
	if dotted {
		inner, err := previewStateMember(message, head)
		if err != nil {
			return "PreviewStateText", "preview_state_text"
		}

		message, head = &inner, member
	}

	kind, _ := message.kindOf(head)
	if kind == protoreflect.StringKind || kind == protoreflect.EnumKind {
		return "PreviewStateText", "preview_state_text"
	}

	return "PreviewStateNumber", "preview_state_number"
}

// previewReadsStateProse reports whether any declared line names the resource
// the fetch read, which is what makes the prose worded after the read rather
// than before it.
func (c *contract) previewReadsStateProse() bool {
	for _, name := range c.previewValueArguments() {
		if strings.HasPrefix(name, previewStatePrefix) {
			return true
		}
	}

	return false
}

// previewReadsTransport reports whether any wording reads what the tool's
// presigned upload would send, which is what selects the transfer preview: the
// values are worded inside the walk the measurement is taken in.
func (c *contract) previewReadsTransport() bool {
	for _, name := range c.previewValueArguments() {
		if strings.HasPrefix(name, previewTransportPrefix) {
			return true
		}
	}

	return false
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
		if list := sentence.Elements; list != nil && list.Join != "" {
			if !slices.Contains(names, previewElementName) {
				names = append(names, previewElementName)
			}
		}

		if pair := sentence.Changed; pair != nil {
			for _, name := range []string{pair.State, pair.Argument} {
				if !slices.Contains(names, name) {
					names = append(names, name)
				}
			}
		}

		// The reading a match selects on is filled like any other, which is
		// what puts the whole line inside the walk the read answers into.
		if match := sentence.Match; match != nil && match.State != "" &&
			!slices.Contains(names, match.State) {
			names = append(names, match.State)
		}

		for _, wording := range sentence.wordings() {
			read, err := c.previewPlaceholders(wording)
			if err != nil {
				continue
			}

			names = previewAppendValues(names, read)
		}
	}

	return names
}

// previewAppendValues adds the names a wording reads that the map does not
// already carry.
//
// A per-entry line binds the entry itself rather than looking it up, so only a
// joined line puts one in the map, which its own walk has already added.
func previewAppendValues(names, read []string) []string {
	for _, name := range read {
		if name == previewElementName || slices.Contains(names, name) {
			continue
		}

		names = append(names, name)
	}

	return names
}

// wordings is every wording one line can report, however it selects between
// them, which is what lets one walk collect the placeholders of both shapes.
func (s *previewSentence) wordings() []string {
	if s.Choice != nil {
		return s.Choice.Wordings.all()
	}

	if s.Match != nil {
		return s.Match.wordings()
	}

	return s.Templates
}

// declaresPreview reports whether the contract answers the tool's dry run
// itself, which is what makes the emitted branch a declared preview rather than
// the plain report of the request.
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
	if tool.previewReadsTransport() {
		emitDeclaredTransportPreview(out, tool, method, path, body)

		return
	}

	if tool.previewReadsStateProse() {
		emitDeclaredStatePreview(out, tool, ordered, method, path, body)

		return
	}

	emitPreviewValues(out, tool, "\t")

	if !tool.previewReadsState() {
		out.writef("\treturn tools.RunDeclaredPreview(ctx, request, cfg, %s, %s, %s, %s, nil, &tools.DryRunDetails{",
			goStringLiteral(tool.Name), method, path, tool.previewBodyExpression(body))

		emitPreviewLines(out, tool, "SideEffects", false, "\t\t")
		emitPreviewLines(out, tool, "Warnings", true, "\t\t")
		emitPreviewBilling(out, tool, "\t\t")

		out.writef("\t})")

		return
	}

	out.writef("\treturn tools.RunDeclaredPreview(ctx, request, cfg, %s, %s, %s, %s,",
		goStringLiteral(tool.Name), method, path, tool.previewBodyExpression(body))

	emitPreviewStateRead(out, tool, ordered)

	out.writef("\t\t&tools.DryRunDetails{")

	emitPreviewLines(out, tool, "SideEffects", false, "\t\t\t")
	emitPreviewLines(out, tool, "Warnings", true, "\t\t\t")
	emitPreviewBilling(out, tool, "\t\t\t")

	out.writef("\t\t})")
}

// emitDeclaredTransportPreview writes the dry-run branch of a tool whose prose
// reads what its presigned upload would send: the wordings are filled inside
// the walk the source is measured in, so a preview nobody is waiting for opens
// nothing.
func emitDeclaredTransportPreview(out *source, tool *contract, method, path, body string) {
	out.writef("\treturn tools.RunDeclaredTransportPreview(ctx, request, cfg, %s, %s, %s, %s, %s,",
		goStringLiteral(tool.Name), method, path, tool.previewBodyExpression(body),
		goStringLiteral(tool.Transport.LocalPathArgument))
	out.writef("\t\tfunc(%s tools.PresignPreview) tools.DryRunDetails {", previewTransportLocal)

	emitPreviewValues(out, tool, "\t\t\t")

	out.writef("\t\t\treturn tools.DryRunDetails{")

	emitPreviewLines(out, tool, "SideEffects", false, "\t\t\t\t")
	emitPreviewLines(out, tool, "Warnings", true, "\t\t\t\t")
	emitPreviewBilling(out, tool, "\t\t\t\t")

	out.writef("\t\t\t}")
	out.writef("\t\t})")
}

// emitDeclaredStatePreview writes the dry-run branch of a tool whose prose names
// the resource it fetched, so the wordings are filled inside the walk the read
// answers into rather than ahead of it.
func emitDeclaredStatePreview(out *source, tool *contract, ordered []field, method, path, body string) {
	out.writef("\treturn tools.RunDeclaredStatePreview(ctx, request, cfg, %s, %s, %s, %s,",
		goStringLiteral(tool.Name), method, path, tool.previewBodyExpression(body))

	emitPreviewStateRead(out, tool, ordered)

	out.writef("\t\tfunc(state any) tools.DryRunDetails {")

	emitPreviewValues(out, tool, "\t\t\t")

	out.writef("\t\t\treturn tools.DryRunDetails{")

	emitPreviewLines(out, tool, "SideEffects", false, "\t\t\t\t")
	emitPreviewLines(out, tool, "Warnings", true, "\t\t\t\t")
	emitPreviewBilling(out, tool, "\t\t\t\t")

	out.writef("\t\t\t}")
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

	// A page is not one resource: the envelope arm answers the page itself, so
	// the decode below, which holds a body to one message, cannot describe it.
	if tool.StateRead.Envelope {
		out.writef("\t\tfunc(ctx context.Context, client *linode.Client) (any, error) {")
		emitEnvelopeFetch(out, tool, values, "\t\t\t")
		out.writef("\t\t},")

		return
	}

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

// emitEnvelopeFetch writes the read whose whole page is the state: the declared
// list, the page controls this tool publishes, and the element each entry is
// projected through.
func emitEnvelopeFetch(out *source, tool *contract, values, indent string) {
	out.need(importGenpb, importLinode)

	element := "linodev1." + tool.StateRead.Message.TypeName

	query := goStateReadQuery(tool)
	if query == "" {
		query = goStringLiteral("")
	}

	out.writef("%sreturn tools.FetchEnvelopeState(ctx, client, %s, %s, %s,",
		indent, goStringLiteral(tool.StateRead.Tool), values, query)
	out.writef("%s\tfunc() *%s { return &%s{} })", indent, element, element)
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
func emitPreviewValues(out *source, tool *contract, indent string) {
	emitPreviewValuesFrom(out, tool, indent, previewStateLocal)
}

// emitPreviewValuesFrom is emitPreviewValues over a named state: the preview
// reads the resource it fetched, and a staged plan reads the member of its
// composite the prose is declared against, so one wording serves both.
func emitPreviewValuesFrom(out *source, tool *contract, indent, state string) {
	names := tool.previewValueArguments()
	if len(names) == 0 {
		return
	}

	out.writef("%spreviewValues := map[string]string{", indent)

	for _, name := range names {
		out.writef("%s\t%s: %s,", indent, goStringLiteral(name), goPreviewValueRead(tool, name, state))
	}

	out.writef("%s}", indent)
	out.writef("")
}

// goPreviewValueRead is the read one placeholder's value comes from: the
// request for an argument the call carried, the fetched resource for a member
// of the state.
func goPreviewValueRead(tool *contract, name, state string) string {
	if name == previewElementName {
		list := tool.previewJoinedList()

		return "tools.PreviewJoined(request, " + goStringLiteral(list.Argument) +
			", " + goStringLiteral(list.Join) + ")"
	}

	// The one member the guard measures, so the read needs no member lookup.
	if strings.HasPrefix(name, previewTransportPrefix) {
		return previewTransportLocal + ".SizeBytes"
	}

	if path, reads := strings.CutPrefix(name, previewStatePrefix); reads {
		reader, _ := tool.previewStateReader(path)
		reading := "tools." + reader + "(" + state + ", " + goStringLiteral(path) + ")"

		if held := tool.previewUnchangedFor(name); held != "" {
			return "tools.PreviewChanged(" + reading + ", " + goPreviewArgumentRead(tool, held) + ")"
		}

		return reading
	}

	return goPreviewArgumentRead(tool, name)
}

// goPreviewArgumentRead is one argument's value as the Go arm reads it, taking
// the carried-number reader for a number every line reading it guards on.
func goPreviewArgumentRead(tool *contract, name string) string {
	if tool.previewCarriedNumber(name) {
		return "tools.PreviewCarriedNumber(request, " + goStringLiteral(name) + ")"
	}

	read := "tools." + goPreviewReader(tool.previewArgument(name)) + "(request, " + goStringLiteral(name) + ")"

	// A folded argument the call omitted still travels, as the value its own
	// fold declares, so the wording reports that rather than dropping the line
	// over a request the tool is going to make anyway.
	if fallback := tool.foldDefault(name); fallback != "" {
		return "tools.PreviewOr(" + read + ", " + goStringLiteral(fallback) + ")"
	}

	return read
}

// previewGuardsOnText reports whether a guard asks its argument for a non-empty
// value rather than for presence.
//
// Only a singular text argument answers that way, because its reader already
// collapses an empty string onto an absent one. A LIST of text does not: an
// empty list is a value a caller can mean, and asking a list for a string reads
// as absent however many entries it carries.
func previewGuardsOnText(entry *field) bool {
	return entry != nil && !entry.Repeated && !entry.ObjectMap &&
		(entry.Kind == protoreflect.StringKind || entry.Kind == protoreflect.EnumKind)
}

// previewJoinedList is the list a joined line names, which is the one every
// {element} placeholder on a joined line reads.
func (c *contract) previewJoinedList() *previewElements {
	for i := range c.PreviewSentences {
		if list := c.PreviewSentences[i].Elements; list != nil && list.Join != "" {
			return list
		}
	}

	return &previewElements{}
}

// previewPerElementLine is the line a half writes per entry, and nil for a half
// written once. A half mixing the two is refused, so at most one can be here.
func previewPerElementLine(tool *contract, warnings bool) *previewSentence {
	for i := range tool.PreviewSentences {
		sentence := &tool.PreviewSentences[i]
		if sentence.Warning != warnings {
			continue
		}

		if list := sentence.Elements; list != nil && list.Join == "" {
			return sentence
		}
	}

	return nil
}

// goPreviewLineGuard is the whole condition a line is reported under: the
// arguments the call has to carry, and the change it has to be making. "" for a
// line every call reports.
func goPreviewLineGuard(tool *contract, sentence *previewSentence) string {
	tests := make([]string, 0, 2)

	if len(sentence.Present) > 0 {
		tests = append(tests, goPreviewGuard(tool, sentence.Present))
	}

	if pair := sentence.Changed; pair != nil {
		tests = append(tests, "tools.PreviewDiffers(previewValues["+goStringLiteral(pair.State)+
			"], previewValues["+goStringLiteral(pair.Argument)+"])")
	}

	return strings.Join(tests, " && ")
}

// goPreviewMatchExpression is one matched line as the handler reports it: the
// value read, then the arm it selects filled in.
func goPreviewMatchExpression(tool *contract, match *previewMatch) string {
	arms := make([]string, 0, len(match.Order))
	for _, value := range match.Order {
		arms = append(arms, goStringLiteral(value)+": "+goStringLiteral(match.Arms[value]))
	}

	read := goPreviewArgumentRead(tool, match.Argument)
	if match.State != "" {
		read = "tools.PreviewFolded(previewValues[" + goStringLiteral(match.State) + "])"
	}

	return "tools.PreviewMatched(" + goPreviewValues(tool) + ", " + read +
		", map[string]string{" + strings.Join(arms, ", ") + "}, " +
		goStringLiteral(match.Otherwise) + ")"
}

// goPreviewGuard is the condition a guarded line is reported under: a text
// argument through its own reader, which already collapses empty onto absent,
// and anything else through presence.
func goPreviewGuard(tool *contract, names []string) string {
	tests := make([]string, 0, len(names))

	for _, name := range names {
		if previewGuardsOnText(tool.previewArgument(name)) {
			tests = append(tests, "tools.PreviewText(request, "+goStringLiteral(name)+") != \"\"")

			continue
		}

		tests = append(tests, "tools.PreviewCarried(request, "+goStringLiteral(name)+")")
	}

	return strings.Join(tests, " && ")
}

// emitPreviewBilling writes the deliberately-unknown estimate a create
// declares: the sentinel and the note, stated outright because there is no
// price to read and so nothing to call.
func emitPreviewBilling(out *source, tool *contract, indent string) {
	if !billingUnpriced(tool.BillingDecl) {
		return
	}

	out.writef("%sBillingDelta: &tools.DryRunBillingDelta{", indent)
	out.writef("%s\tMonthlyChangeUSD: tools.BillingUnknown,", indent)
	out.writef("%s\tNote:             %s,", indent, goStringLiteral(tool.BillingDecl.GetAbsentTypeSentence()))
	out.writef("%s},", indent)
}

// emitPreviewLines writes one half of the declared prose at the column the
// details literal sits at, leaving the member absent when the tool declares no
// line for it.
func emitPreviewLines(out *source, tool *contract, member string, warnings bool, indent string) {
	if perElement := previewPerElementLine(tool, warnings); perElement != nil {
		out.writef("%s%s: tools.PreviewPerElement(request, %s, %s, %s),", indent, member,
			goStringLiteral(perElement.Elements.Argument), goPreviewValues(tool),
			goStringLiteral(perElement.Templates[0]))

		return
	}

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

// previewSentenceReads reports whether a line names any value: a wording with a
// placeholder, a guard, a flag, or a match. A line that names none is its own
// text wherever it is reported.
func previewSentenceReads(sentence *previewSentence) bool {
	if sentence.Choice != nil || sentence.Match != nil ||
		len(sentence.Present) > 0 || sentence.Changed != nil || sentence.Elements != nil {
		return true
	}

	for _, wording := range sentence.wordings() {
		if strings.Contains(wording, previewPlaceholderOpen) {
			return true
		}
	}

	return false
}

// previewSentenceExpression is one declared line as the handler reports it: the
// wording itself when there is nothing to choose between and nothing to fill in,
// and the support call that chooses and fills otherwise.
func previewSentenceExpression(tool *contract, sentence *previewSentence) string {
	if sentence.Choice != nil {
		return previewChoiceExpression(tool, sentence.Choice)
	}

	if guard := goPreviewLineGuard(tool, sentence); guard != "" {
		return "tools.PreviewGuarded(" + guard + ", " + previewWordingExpression(tool, sentence) + ")"
	}

	return previewWordingExpression(tool, sentence)
}

// previewWordingExpression is the wording half of one line: the wording itself
// when there is nothing to choose between and nothing to fill in, and the
// support call that chooses and fills otherwise.
func previewWordingExpression(tool *contract, sentence *previewSentence) string {
	if sentence.Match != nil {
		return goPreviewMatchExpression(tool, sentence.Match)
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
// number is not a label, and the hand-written bodies this replaced read it that
// way.
// previewTextReader is the Go reader a text or enum argument is read through,
// named so a check can ask which reader a kind takes without spelling it twice.
const previewTextReader = "PreviewText"

func previewReader(entry *field) (string, string) {
	// An enum argument arrives as the member's own name, which is text on the
	// wire and is what the hand-written bodies this replaced read it as.
	if entry != nil && (entry.Kind == protoreflect.StringKind || entry.Kind == protoreflect.EnumKind) {
		return previewTextReader, "preview_text"
	}

	return "PreviewNumber", "preview_number"
}

// goPreviewReader is the Go half of the reader table.
func goPreviewReader(entry *field) string {
	name, _ := previewReader(entry)

	return name
}

// pyPreviewValueReader is the Python reader one placeholder's value is read
// through, whichever of the two places it comes from.
func pyPreviewValueReader(tool *contract, name string) string {
	if path, reads := strings.CutPrefix(name, previewStatePrefix); reads {
		_, reader := tool.previewStateReader(path)

		return reader
	}

	return pyPreviewReader(tool.previewArgument(name))
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
// apart, because the dry-run branch is one of the two that can drop the body.
//
// The other is a live call whose transport does not send the derived JSON at
// all: a multipart form or a raw image body advertises what it carries through
// the schema and puts the bytes the argument names on the wire instead.
func emitBodyBuild(out *source, tool *contract, echoes bool) {
	if (!echoes && tool.PreviewOmitsBody) || (echoes && tool.transportDropsBody()) {
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
