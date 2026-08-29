package main

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The declared dependency walks: what else a call touches, read here and held
// to shapes both arms can render. Resolution against the other declared tools
// waits for the resolve pass, where element messages are in scope.

// readDependencyWalks records the declared walks and the billing delta,
// holding each to the structural rules that need no other tool in scope.
func (c *contract) readDependencyWalks(options protoreflect.ProtoMessage) error {
	declared, _ := proto.GetExtension(options, linodev1.E_DependencyWalk).([]*linodev1.DependencyWalk)
	billing, _ := proto.GetExtension(options, linodev1.E_BillingDelta).(*linodev1.BillingDelta)

	if len(declared) == 0 && billing == nil {
		return nil
	}

	for _, walk := range declared {
		if err := c.checkWalkShape(walk); err != nil {
			return err
		}
	}

	if billing != nil {
		if err := c.checkBillingShape(billing); err != nil {
			return err
		}
	}

	c.WalkDecls = declared
	c.BillingDecl = billing

	return nil
}

// checkWalkShape holds one walk to the structural rules: at most one source,
// an emission to read it into, and the failure sentence a route read needs.
// A sourceless walk carries warnings alone; giving it an emission or a filter
// is iteration nothing feeds.
func (c *contract) checkWalkShape(walk *linodev1.DependencyWalk) error {
	var sources int

	for _, source := range []string{walk.GetListTool(), walk.GetStateMember(), walk.GetStateScalar()} {
		if source != "" {
			sources++
		}
	}

	if sources > 1 {
		return fmt.Errorf("%w: %s", errWalkSourceCount, c.Name)
	}

	if sources == 0 && (walk.GetEmit() != nil || walk.GetFilter() != nil || len(walk.GetWarnings()) == 0) {
		return fmt.Errorf("%w: %s", errWalkSourceCount, c.Name)
	}

	if sources == 1 && walk.GetEmit() == nil {
		return fmt.Errorf("%w: %s", errWalkNoEmit, c.Name)
	}

	if err := c.checkWalkEmit(walk.GetEmit()); err != nil {
		return err
	}

	if (walk.GetListTool() == "") != (walk.GetListErrorWarning() == "") {
		return fmt.Errorf("%w: %s", errWalkErrorWarning, c.Name)
	}

	if walk.GetListMember() != "" && walk.GetListTool() == "" {
		return fmt.Errorf("%w: %s", errWalkSourceCount, c.Name)
	}

	if len(walk.GetSlotArguments()) > 0 && walk.GetListTool() == "" {
		return fmt.Errorf("%w: %s", errWalkSourceCount, c.Name)
	}

	if err := c.checkWalkFilter(walk.GetFilter()); err != nil {
		return err
	}

	for _, warning := range walk.GetWarnings() {
		if err := c.checkWalkWarning(warning); err != nil {
			return err
		}
	}

	return nil
}

// checkWalkEmit holds one emission to naming its kind one way and carrying a
// line worth saying.
func (c *contract) checkWalkEmit(emit *linodev1.WalkEmit) error {
	if emit == nil {
		return nil
	}

	if emit.GetTarget() == linodev1.WalkTarget_WALK_TARGET_SIDE_EFFECTS {
		return c.checkWalkSideEffects(emit)
	}

	if (emit.GetKind() == "") == (emit.GetKindField() == "") {
		return fmt.Errorf("%w: %s", errWalkEmitKind, c.Name)
	}

	if emit.GetKindFallback() != "" && emit.GetKindField() == "" {
		return fmt.Errorf("%w: %s", errWalkEmitKind, c.Name)
	}

	if emit.GetLabelFallback() != "" && emit.GetLabelField() == "" {
		return fmt.Errorf("%w: %s", errWalkEmitLabel, c.Name)
	}

	if emit.GetNote() == "" && emit.GetAction() == "" {
		return fmt.Errorf("%w: %s", errWalkEmitEmpty, c.Name)
	}

	return nil
}

// checkWalkSideEffects holds a side-effect emission to bare prose: the
// engines say only its note there, so a kind, action, or field reference
// would be wiring with no effect.
func (c *contract) checkWalkSideEffects(emit *linodev1.WalkEmit) error {
	if emit.GetNote() == "" {
		return fmt.Errorf("%w: %s", errWalkEmitEmpty, c.Name)
	}

	if emit.GetKind() != "" || emit.GetKindField() != "" || emit.GetKindFallback() != "" ||
		emit.GetIdField() != "" || emit.GetLabelField() != "" ||
		emit.GetLabelFallback() != "" || emit.GetAction() != "" {
		return fmt.Errorf("%w: %s", errWalkEmitKind, c.Name)
	}

	return nil
}

// checkWalkFilter holds a filter to one comparison source.
func (c *contract) checkWalkFilter(filter *linodev1.WalkFilter) error {
	if filter == nil {
		return nil
	}

	if filter.GetField() == "" || (filter.GetValue() == "") == (filter.GetArgument() == "") {
		return fmt.Errorf("%w: %s", errWalkFilterShape, c.Name)
	}

	if filter.GetArgument() != "" && c.previewArgument(filter.GetArgument()) == nil {
		return fmt.Errorf("%w: %s filters from %s",
			errStateRouteSlotUnknownArgument, c.Name, filter.GetArgument())
	}

	return nil
}

// checkWalkWarning holds one warning to a single predicate: always, a state
// equality, the truncation notice, any-emitted, a positive aggregate, or one
// side of the presence pair.
func (c *contract) checkWalkWarning(warning *linodev1.WalkWarning) error {
	if warning.GetTemplate() == "" {
		return fmt.Errorf("%w: %s", errWalkWarningShape, c.Name)
	}

	if (warning.GetWhenField() == "") != (warning.GetWhenEquals() == "") {
		return fmt.Errorf("%w: %s", errWalkWarningShape, c.Name)
	}

	var predicates int

	for _, holds := range []bool{
		warning.GetWhenField() != "",
		warning.GetWhenTruncated(),
		warning.GetWhenAny(),
		warning.GetWhenPositive() != "",
		warning.GetWhenPresent() != "",
		warning.GetWhenAbsent() != "",
	} {
		if holds {
			predicates++
		}
	}

	if predicates > 1 {
		return fmt.Errorf("%w: %s", errWalkWarningShape, c.Name)
	}

	return nil
}

// checkBillingShape holds the billing estimate to naming its whole flow.
func (c *contract) checkBillingShape(billing *linodev1.BillingDelta) error {
	if billingUnpriced(billing) {
		// The note is the whole answer, so the fields a priced estimate needs
		// would each be wiring nothing reads.
		if billing.GetAbsentTypeSentence() == "" || billing.GetSentence() != "" ||
			billing.GetUnknownSentence() != "" || billing.GetAmount() != "" {
			return fmt.Errorf("%w: %s", errBillingShape, c.Name)
		}

		return nil
	}

	if billing.GetPriceTool() == "" || billing.GetTypeField() == "" ||
		billing.GetSentence() == "" || billing.GetUnknownSentence() == "" ||
		billing.GetAmount() == "" {
		return fmt.Errorf("%w: %s", errBillingShape, c.Name)
	}

	return nil
}

// billingUnpriced reports whether the estimate is deliberately unknown: a call
// with nothing to read and no price to read it from, where the note is the
// whole answer.
func billingUnpriced(billing *linodev1.BillingDelta) bool {
	return billing != nil && billing.GetPriceTool() == "" && billing.GetTypeField() == ""
}

// resolvedWalk is one walk with its element message in hand, which is what
// the emission renders getters and template fields against.
type resolvedWalk struct {
	Decl    *linodev1.DependencyWalk
	Element goMessage
	// Slots names the arguments filling a route walk's list, in slot order;
	// empty for the state-sourced walks.
	Slots []string
	// Response is the message a member walk decodes through; zero when the
	// walk reads a page envelope or the state.
	Response goMessage
	// EnrichMessage and EnrichSlots resolve the declared enrichment read;
	// zero when the walk declares none.
	EnrichMessage goMessage
	EnrichSlots   []walkEnrichSlot
}

// walkEnrichSlot names where one enrichment read slot fills from: the tool's
// own validated argument, or a member only the fetch read.
type walkEnrichSlot struct {
	Argument   string
	StateField string
}

// resolveDependencyWalks resolves each declared walk against the other tools
// and the declared state, and validates every template placeholder.
func (c *contract) resolveDependencyWalks(declared map[string]protoreflect.MessageDescriptor) error {
	if len(c.WalkDecls) == 0 && (c.BillingDecl == nil || billingUnpriced(c.BillingDecl)) {
		return nil
	}

	if len(c.WalkDecls) > 0 && !c.StateRead.declared() && len(c.Composite) == 0 {
		return fmt.Errorf("%w: %s", errWalkNoState, c.Name)
	}

	walks := make([]resolvedWalk, 0, len(c.WalkDecls))

	for _, decl := range c.WalkDecls {
		walk, err := c.resolveWalk(decl, declared)
		if err != nil {
			return err
		}

		walks = append(walks, walk)
	}

	c.DepWalks = walks

	if err := c.checkWalkSentences(); err != nil {
		return err
	}

	return c.resolveBilling(declared)
}

// checkWalkSentences holds the preview sentences beside a declared walk to
// constant side-effect lines: the closure seeds the dry run with them ahead
// of the walk's own, and nothing there can fill a placeholder or a choice.
func (c *contract) checkWalkSentences() error {
	for index := range c.PreviewSentences {
		sentence := &c.PreviewSentences[index]
		if sentence.Warning {
			return fmt.Errorf("%w: %s", errWalkSentence, c.Name)
		}

		if sentence.Choice != nil || len(sentence.Templates) != 1 ||
			strings.Contains(sentence.Templates[0], previewPlaceholderOpen) {
			return fmt.Errorf("%w: %s", errWalkSentence, c.Name)
		}
	}

	return nil
}

// resolveWalk resolves one walk's element source and its enrichment.
func (c *contract) resolveWalk(
	decl *linodev1.DependencyWalk, declared map[string]protoreflect.MessageDescriptor,
) (resolvedWalk, error) {
	walk, err := c.resolveWalkSource(decl, declared)
	if err != nil {
		return resolvedWalk{}, err
	}

	err = c.resolveWalkEnrich(&walk, declared)

	return walk, err
}

// resolveWalkSource resolves where the walk's elements come from. A
// sourceless walk reads its warning predicates off the state itself.
func (c *contract) resolveWalkSource(
	decl *linodev1.DependencyWalk, declared map[string]protoreflect.MessageDescriptor,
) (resolvedWalk, error) {
	if decl.GetListTool() != "" {
		return c.resolveRouteWalk(decl, declared)
	}

	if decl.GetStateScalar() != "" {
		return c.resolveScalarWalk(decl)
	}

	if decl.GetStateMember() != "" {
		return c.resolveStateWalk(decl)
	}

	walk := resolvedWalk{Decl: decl, Element: c.stateMessage()}
	err := c.checkWalkNames(&walk)

	return walk, err
}

// resolveWalkEnrich resolves the enrichment read: its route, the kept
// fields, and each slot's source pool.
func (c *contract) resolveWalkEnrich(
	walk *resolvedWalk, declared map[string]protoreflect.MessageDescriptor,
) error {
	enrich := walk.Decl.GetEnrich()
	if enrich == nil {
		return nil
	}

	descriptor, route, err := c.compositeReadRoute(enrich.GetTool(), declared)
	if err != nil {
		return err
	}

	message, isList, err := compositeReadShape(descriptor)
	if err != nil {
		return err
	}

	if isList {
		return fmt.Errorf("%w: %s decorates from %s",
			errWalkEnrichShape, c.Name, enrich.GetTool())
	}

	for _, field := range enrich.GetFields() {
		if message.goNameOf(field) == "" {
			return fmt.Errorf("%w: %s keeps %s", errDepWalkUnknownField, c.Name, field)
		}
	}

	slots, err := c.enrichSlots(enrich, route)
	if err != nil {
		return err
	}

	walk.EnrichMessage = message
	walk.EnrichSlots = slots

	return nil
}

// enrichSlots fills each read slot from the arguments first, then from the
// declared state, the second pool WalkEnrich admits because the id an
// enrichment needs is often one the fetch read.
func (c *contract) enrichSlots(
	enrich *linodev1.WalkEnrich, route *linodev1.ToolRoute,
) ([]walkEnrichSlot, error) {
	mapped := make(map[string]string, len(enrich.GetSlotArguments()))
	for _, slot := range enrich.GetSlotArguments() {
		mapped[slot.GetReadSlot()] = slot.GetArgument()
	}

	state := c.stateMessage()
	wanted := routeSlots(route.GetPath())
	slots := make([]walkEnrichSlot, 0, len(wanted))

	for _, slot := range wanted {
		name := mapped[slot]
		if name == "" {
			name = slot
		}

		if c.previewArgument(name) != nil {
			slots = append(slots, walkEnrichSlot{Argument: name})

			continue
		}

		if state.goNameOf(name) != "" {
			slots = append(slots, walkEnrichSlot{StateField: name})

			continue
		}

		return nil, fmt.Errorf("%w: %s fills %s from %s",
			errStateRouteSlotUnknownArgument, c.Name, slot, name)
	}

	return slots, nil
}

// resolveRouteWalk resolves a walk over a list read of its own: the page's
// element, or the nested member a non-envelope answer carries.
func (c *contract) resolveRouteWalk(
	decl *linodev1.DependencyWalk, declared map[string]protoreflect.MessageDescriptor,
) (resolvedWalk, error) {
	message, route, err := c.compositeReadRoute(decl.GetListTool(), declared)
	if err != nil {
		return resolvedWalk{}, err
	}

	if decl.GetListMember() != "" {
		return c.resolveMemberWalk(decl, message, route)
	}

	element, isList, err := compositeReadShape(message)
	if err != nil {
		return resolvedWalk{}, err
	}

	if !isList {
		return resolvedWalk{}, fmt.Errorf("%w: %s walks %s",
			errStateReadNotList, c.Name, decl.GetListTool())
	}

	slots, err := c.compositeSlots(&linodev1.StateCompositeCall{
		Tool:          decl.GetListTool(),
		SlotArguments: decl.GetSlotArguments(),
	}, route)
	if err != nil {
		return resolvedWalk{}, err
	}

	walk := resolvedWalk{Decl: decl, Element: element, Slots: slots}
	err = c.checkWalkNames(&walk)

	return walk, err
}

// resolveMemberWalk resolves the nested-member form: the response message
// walked down the dotted path to a repeated message member.
func (c *contract) resolveMemberWalk(
	decl *linodev1.DependencyWalk,
	message protoreflect.MessageDescriptor,
	route *linodev1.ToolRoute,
) (resolvedWalk, error) {
	answered := protoreflect.FullName(stringOption(message.Options(), linodev1.E_ToolResponse))

	response, err := lookupGoMessage(answered)
	if err != nil {
		return resolvedWalk{}, err
	}

	element, err := nestedRepeatedElement(&response, decl.GetListMember())
	if err != nil {
		return resolvedWalk{}, fmt.Errorf("%w: %s walks %s of %s",
			errWalkStateMember, c.Name, decl.GetListMember(), decl.GetListTool())
	}

	slots, err := c.compositeSlots(&linodev1.StateCompositeCall{
		Tool:          decl.GetListTool(),
		SlotArguments: decl.GetSlotArguments(),
	}, route)
	if err != nil {
		return resolvedWalk{}, err
	}

	walk := resolvedWalk{Decl: decl, Element: element, Response: response, Slots: slots}
	err = c.checkWalkNames(&walk)

	return walk, err
}

// nestedRepeatedElement follows a dotted path to a repeated message member.
func nestedRepeatedElement(message *goMessage, path string) (goMessage, error) {
	head, rest, dotted := strings.Cut(path, ".")

	if dotted {
		for _, member := range message.messageFields() {
			if member.protoName != head {
				continue
			}

			inner, err := lookupGoMessage(member.message)
			if err != nil {
				return goMessage{}, err
			}

			return nestedRepeatedElement(&inner, rest)
		}

		return goMessage{}, fmt.Errorf("%w: %s", errDepWalkUnknownField, head)
	}

	for _, repeated := range message.repeatedFields() {
		if repeated.protoName == head {
			return lookupGoMessage(repeated.message)
		}
	}

	return goMessage{}, fmt.Errorf("%w: %s", errDepWalkUnknownField, path)
}

// resolveStateWalk resolves a walk over a repeated member of the declared
// state.
func (c *contract) resolveStateWalk(decl *linodev1.DependencyWalk) (resolvedWalk, error) {
	element, isList, err := c.stateMemberElement(decl.GetStateMember())
	if err != nil {
		return resolvedWalk{}, err
	}

	if !isList {
		return resolvedWalk{}, fmt.Errorf("%w: %s walks %s",
			errWalkStateMember, c.Name, decl.GetStateMember())
	}

	walk := resolvedWalk{Decl: decl, Element: element}
	err = c.checkWalkNames(&walk)

	return walk, err
}

// resolveScalarWalk resolves the single-element walk a state scalar names. The
// element the emission reads is the state itself.
func (c *contract) resolveScalarWalk(decl *linodev1.DependencyWalk) (resolvedWalk, error) {
	state := c.stateMessage()
	if state.goNameOf(decl.GetStateScalar()) == "" {
		return resolvedWalk{}, fmt.Errorf("%w: %s reads %s",
			errWalkStateMember, c.Name, decl.GetStateScalar())
	}

	walk := resolvedWalk{Decl: decl, Element: state}
	nameErr := c.checkWalkNames(&walk)

	return walk, nameErr
}

// stateMessage is the message the declared state projects through.
func (c *contract) stateMessage() goMessage {
	return c.StateRead.Message
}

// stateMemberElement is the element message a repeated state member carries.
func (c *contract) stateMemberElement(member string) (goMessage, bool, error) {
	state := c.stateMessage()

	// An envelope state carries its elements under "data", projected through
	// the list's element message the read already resolved.
	if c.StateRead.Envelope {
		return state, member == "data", nil
	}

	for _, repeated := range state.repeatedFields() {
		if repeated.protoName != member {
			continue
		}

		element, err := lookupGoMessage(repeated.message)
		if err != nil {
			return goMessage{}, false, err
		}

		return element, true, nil
	}

	return goMessage{}, false, nil
}

// checkWalkNames holds the walk's field references and templates to names its
// element and this tool's own vocabulary declare.
func (c *contract) checkWalkNames(walk *resolvedWalk) error {
	emit := walk.Decl.GetEmit()

	for _, field := range []string{
		emit.GetKindField(), emit.GetLabelField(), emit.GetIdField(),
	} {
		if field == "" {
			continue
		}

		if err := c.checkWalkPath(walk, field); err != nil {
			return err
		}
	}

	if filter := walk.Decl.GetFilter(); filter != nil {
		if err := c.checkWalkPath(walk, filter.GetField()); err != nil {
			return err
		}
	}

	for _, template := range []string{emit.GetNote(), emit.GetLabelFallback()} {
		if err := c.checkWalkTemplate(walk, template); err != nil {
			return err
		}
	}

	for _, warning := range walk.Decl.GetWarnings() {
		if err := c.checkWalkTemplate(walk, warning.GetTemplate()); err != nil {
			return err
		}

		if err := c.checkWarningPredicate(walk, warning); err != nil {
			return err
		}
	}

	return nil
}

// checkWarningPredicate holds a warning's condition to names the state and
// the aggregate vocabulary declare.
func (c *contract) checkWarningPredicate(
	walk *resolvedWalk, warning *linodev1.WalkWarning,
) error {
	state := c.stateMessage()

	for _, field := range []string{
		warning.GetWhenField(), warning.GetWhenPresent(), warning.GetWhenAbsent(),
	} {
		if field != "" && state.goNameOf(field) == "" {
			return fmt.Errorf("%w: %s conditions on %s", errDepWalkUnknownField, c.Name, field)
		}
	}

	if name := warning.GetWhenPositive(); name != "" {
		return c.checkWalkAggregate(walk, name)
	}

	return nil
}

// walkAggregateNamed reports whether the name is one the renderer computes
// for every walk.
func walkAggregateNamed(name string) bool {
	return name == "count" || name == "total" || name == "max_results"
}

// checkWalkAggregate holds one named guard to the same vocabulary the
// templates read.
func (c *contract) checkWalkAggregate(walk *resolvedWalk, name string) error {
	switch {
	case walkAggregateNamed(name):
		return nil
	case strings.HasPrefix(name, "sum:"), strings.HasPrefix(name, "state:"):
		return c.checkWalkPlaceholder(walk, name)
	default:
		return fmt.Errorf("%w: %s guards on %s", errDepWalkPlaceholder, c.Name, name)
	}
}

// checkWalkPath holds one possibly-dotted field reference to the element: a
// plain member, one step into a nested member ("data.id"), or one level into
// an open object's values ("devices.*.disk_id").
func (c *contract) checkWalkPath(walk *resolvedWalk, path string) error {
	head, _, dotted := strings.Cut(path, ".")
	if walk.Element.goNameOf(head) == "" {
		return fmt.Errorf("%w: %s reads %s", errDepWalkUnknownField, c.Name, path)
	}

	_ = dotted

	return nil
}

// walkPlaceholder matches one {placeholder} in a template.
var depWalkPlaceholder = regexp.MustCompile(`\{([^{}]+)\}`)

// checkWalkTemplate holds every placeholder to the declared vocabulary: the
// element's own fields, {arg:name}, {len:member}, {enrich:field},
// {state:field}, and the aggregates the renderer computes.
func (c *contract) checkWalkTemplate(walk *resolvedWalk, template string) error {
	for _, match := range depWalkPlaceholder.FindAllStringSubmatch(template, -1) {
		if err := c.checkWalkPlaceholder(walk, match[1]); err != nil {
			return err
		}
	}

	return nil
}

// checkWalkPlaceholder resolves one placeholder against its pool.
func (c *contract) checkWalkPlaceholder(walk *resolvedWalk, name string) error {
	switch {
	case walkAggregateNamed(name) || name == "error":
		return nil
	case strings.HasPrefix(name, "sum:"):
		// The operand is a numeric field, or a nested list under sum:len:.
		name = strings.TrimPrefix(strings.TrimPrefix(name, "sum:"), "len:")
		if walk.Element.goNameOf(name) == "" {
			return fmt.Errorf("%w: %s sums %s", errDepWalkPlaceholder, c.Name, name)
		}

		return nil
	case strings.HasPrefix(name, "len:"):
		name = strings.TrimPrefix(name, "len:")
		if walk.Element.goNameOf(name) == "" {
			return fmt.Errorf("%w: %s measures %s", errDepWalkPlaceholder, c.Name, name)
		}

		return nil
	case strings.HasPrefix(name, "arg:"):
		name = strings.TrimPrefix(name, "arg:")
		if c.previewArgument(name) == nil {
			return fmt.Errorf("%w: %s interpolates %s", errDepWalkPlaceholder, c.Name, name)
		}

		return nil
	case strings.HasPrefix(name, "state:"):
		name = strings.TrimPrefix(name, "state:")

		state := c.stateMessage()
		if state.goNameOf(name) == "" {
			return fmt.Errorf("%w: %s reads state %s", errDepWalkPlaceholder, c.Name, name)
		}

		return nil
	case strings.HasPrefix(name, "enrich:"):
		return c.checkEnrichField(walk, strings.TrimPrefix(name, "enrich:"))
	default:
		if walk.Element.goNameOf(name) == "" {
			return fmt.Errorf("%w: %s words %s", errDepWalkPlaceholder, c.Name, name)
		}

		return nil
	}
}

// checkEnrichField holds one {enrich:field} to the declared enrichment.
func (c *contract) checkEnrichField(walk *resolvedWalk, name string) error {
	enrich := walk.Decl.GetEnrich()
	if enrich == nil || !slices.Contains(enrich.GetFields(), name) {
		return fmt.Errorf("%w: %s decorates with %s", errDepWalkPlaceholder, c.Name, name)
	}

	return nil
}

// resolveBilling holds the billing estimate to a priceable flow.
func (c *contract) resolveBilling(declared map[string]protoreflect.MessageDescriptor) error {
	if c.BillingDecl == nil {
		return nil
	}

	// Nothing to resolve for the deliberately-unknown form: it names no price
	// tool to look up and reads no state to price.
	if billingUnpriced(c.BillingDecl) {
		return nil
	}

	message, route, err := c.compositeReadRoute(c.BillingDecl.GetPriceTool(), declared)
	if err != nil {
		return err
	}

	// The engine fills the read's one slot with the type itself.
	if len(routeSlots(route.GetPath())) != 1 {
		return fmt.Errorf("%w: %s", errBillingShape, c.Name)
	}

	priced := protoreflect.FullName(stringOption(message.Options(), linodev1.E_ToolResponse))

	response, err := lookupGoMessage(priced)
	if err != nil {
		return err
	}

	c.BillingRead = response

	state := c.stateMessage()
	if state.goNameOf(c.BillingDecl.GetTypeField()) == "" {
		return fmt.Errorf("%w: %s prices %s",
			errDepWalkUnknownField, c.Name, c.BillingDecl.GetTypeField())
	}

	return nil
}
