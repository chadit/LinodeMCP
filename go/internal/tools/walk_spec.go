package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// The declared dependency-walk engine: the emitter renders each walk as a
// WalkSpec literal and this interpreter runs them, so both languages word one
// blast radius from one declaration. Mirrors Python's run_dependency_walks.

// WalkSpec is one declared walk, resolved to values the runtime can act on.
type WalkSpec struct {
	// New builds the list element a route walk decodes through; nil for the
	// state-fed walks.
	New func() proto.Message
	// Filter keeps only matching elements, nil for none.
	Filter *WalkSpecFilter
	// Enrich decorates templates with a best-effort read, nil for none.
	Enrich *WalkSpecEnrich
	// ListTool names the list a route walk reads; empty for state walks.
	ListTool string
	// ListMember is the nested repeated member a non-envelope list carries
	// its elements under, dotted up to two levels; empty reads the
	// envelope's data.
	ListMember string
	// StateMember names the repeated state member a state walk iterates.
	StateMember string
	// StateScalar names the state field whose positive value is the single
	// element a scalar walk emits.
	StateScalar string
	// ListErrorWarning words a failed list, with {error} for the failure.
	ListErrorWarning string
	Emit             WalkSpecEmit
	Warnings         []WalkSpecWarning
	// Values fill the list route's slots, in slot order.
	Values []any
}

// WalkSpecFilter keeps elements whose field matches a literal or an argument.
type WalkSpecFilter struct {
	Field    string
	Value    string
	Argument string
	Fold     bool
}

// WalkSpecEmit is what each element becomes.
type WalkSpecEmit struct {
	Kind         string
	KindField    string
	KindFallback string
	IDField      string
	LabelField   string
	// LabelFallback words the label when LabelField renders empty, a
	// template over the same pools the note reads.
	LabelFallback string
	Action        string
	Note          string
	SideEffects   bool
}

// WalkSpecWarning is one sentence about the whole, said under one predicate.
type WalkSpecWarning struct {
	Template      string
	WhenField     string
	WhenEquals    string
	WhenPositive  string
	WhenPresent   string
	WhenAbsent    string
	WhenTruncated bool
	WhenAny       bool
}

// WalkSpecEnrich is one best-effort read decorating the templates.
type WalkSpecEnrich struct {
	New    func() proto.Message
	Tool   string
	Fields []string
	Values []any
}

// walkPlaceholder matches one {placeholder} in a template.
var walkPlaceholder = regexp.MustCompile(`\{([^{}]+)\}`)

// RunDependencyWalks runs each spec in order over the declared state and
// assembles one DryRunDetails, which is what a dry run reports and a plan
// carries beside its hash.
func RunDependencyWalks(
	ctx context.Context,
	client *linode.Client,
	specs []WalkSpec,
	state DeclaredState,
	arguments map[string]any,
) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("dependency walk canceled: %w", err)
	}

	for index := range specs {
		specs[index].run(ctx, client, &details, state, arguments)
	}

	return details, nil
}

// run executes one walk: gather elements, filter, emit, then the warnings.
func (spec *WalkSpec) run(
	ctx context.Context,
	client *linode.Client,
	details *DryRunDetails,
	state DeclaredState,
	arguments map[string]any,
) {
	elements, total := spec.elements(ctx, client, details, state)
	kept := spec.filtered(elements, arguments)
	pools := spec.pools(ctx, client, state, arguments, kept)

	var emitted int

	for _, element := range kept {
		if spec.emitElement(details, element, pools) {
			emitted++
		}
	}

	spec.sayWarnings(details, state, pools, kept, emitted, total, len(elements))
}

// elements gathers the walk's elements and the envelope total when one rides
// along. A failed list becomes the declared warning rather than an error.
func (spec *WalkSpec) elements(
	ctx context.Context,
	client *linode.Client,
	details *DryRunDetails,
	state DeclaredState,
) ([]DeclaredState, int) {
	if spec.ListTool != "" && spec.ListMember != "" {
		return spec.memberElements(ctx, client, details)
	}

	if spec.ListTool != "" {
		return spec.listElements(ctx, client, details)
	}

	if spec.StateScalar != "" {
		if number, carried := state.Number(spec.StateScalar); carried && number > 0 {
			return []DeclaredState{state}, 0
		}

		return nil, 0
	}

	elements := state.Objects(spec.StateMember)
	total := len(elements)

	if results, carried := state.Number("results"); carried {
		total = results
	}

	return elements, total
}

// listElements reads the walk's own list, best-effort.
func (spec *WalkSpec) listElements(
	ctx context.Context, client *linode.Client, details *DryRunDetails,
) ([]DeclaredState, int) {
	// A walk's own list is read at the route's page defaults: the walk names
	// the collection, and no page control of the tool's reaches it.
	state, err := FetchEnvelopeState(ctx, client, spec.ListTool, spec.Values, "", spec.New)
	if err != nil {
		// The failure lands before the declared warnings and the walk
		// continues over no elements, so a state-fed warning still speaks.
		details.Warnings = append(details.Warnings,
			strings.ReplaceAll(spec.ListErrorWarning, "{error}", err.Error()))

		return nil, 0
	}

	// envelopeState always answers a DeclaredState, so no second guard here.
	declared, _ := state.(DeclaredState)

	elements := declared.Objects("data")
	total := len(elements)

	if results, carried := declared.Number("results"); carried {
		total = results
	}

	return elements, total
}

// memberElements reads the nested-member list a non-envelope route answers:
// the instance IP read keeps ipv4.public and nothing else.
func (spec *WalkSpec) memberElements(
	ctx context.Context, client *linode.Client, details *DryRunDetails,
) ([]DeclaredState, int) {
	message := spec.New()

	raw, err := client.CallProtoRouteState(ctx, spec.ListTool, spec.Values, "", message)
	if err != nil {
		details.Warnings = append(details.Warnings,
			strings.ReplaceAll(spec.ListErrorWarning, "{error}", err.Error()))

		return nil, 0
	}

	projected, projectErr := ProjectDeclaredState(raw, message)

	var elements []DeclaredState

	if declared, isState := projected.(DeclaredState); projectErr == nil && isState {
		elements = nestedObjects(declared, spec.ListMember)
	}

	return elements, len(elements)
}

// nestedObjects walks a dotted member path down to its repeated elements.
func nestedObjects(declared DeclaredState, path string) []DeclaredState {
	head, rest, dotted := strings.Cut(path, ".")
	if dotted {
		return nestedObjects(declared.Object(head), rest)
	}

	return declared.Objects(head)
}

// BillingSpec prices what a removal stops paying for.
type BillingSpec struct {
	// New builds the pricing read's response message.
	New func() proto.Message
	// PriceTool is the pricing read, its one slot filled with the type.
	PriceTool string
	// TypeField is the declared state member holding the priced type.
	TypeField string
	// Amount words the fetched price, with {monthly:.2f} for the number.
	Amount string
	// Sentence is the note beside a priced amount.
	Sentence string
	// UnknownSentence is the note beside a failed price fetch.
	UnknownSentence string
	// AbsentTypeSentence is the note when the state carries no type.
	AbsentTypeSentence string
}

// RunBillingDelta words the monthly consequence beside the walks' answer.
func RunBillingDelta(
	ctx context.Context,
	client *linode.Client,
	spec *BillingSpec,
	state DeclaredState,
	details *DryRunDetails,
) {
	typed := state.Text(spec.TypeField)
	if typed == "" {
		details.BillingDelta = &DryRunBillingDelta{
			MonthlyChangeUSD: BillingUnknown, Note: spec.AbsentTypeSentence,
		}

		return
	}

	monthly, priced := spec.monthlyPrice(ctx, client, typed)
	if !priced {
		details.BillingDelta = &DryRunBillingDelta{
			MonthlyChangeUSD: BillingUnknown, Note: spec.UnknownSentence,
		}

		return
	}

	amount := strings.ReplaceAll(spec.Amount, "{monthly:.2f}", fmt.Sprintf("%.2f", monthly))
	details.BillingDelta = &DryRunBillingDelta{MonthlyChangeUSD: amount, Note: spec.Sentence}
}

// monthlyPrice reads the type's monthly price, best-effort.
func (spec *BillingSpec) monthlyPrice(
	ctx context.Context, client *linode.Client, typed string,
) (float64, bool) {
	message := spec.New()

	raw, err := client.CallProtoRouteState(ctx, spec.PriceTool, []any{typed}, "", message)
	if err != nil {
		return 0, false
	}

	projected, projectErr := ProjectDeclaredState(raw, message)

	var (
		monthly float64
		priced  bool
	)

	if declared, isState := projected.(DeclaredState); projectErr == nil && isState {
		monthly, priced = priceMonthly(declared)
	}

	return monthly, priced
}

// priceMonthly reads price.monthly off the projected type.
func priceMonthly(declared DeclaredState) (float64, bool) {
	number, carried := declared.Object("price")["monthly"].(json.Number)
	if !carried {
		return 0, false
	}

	monthly, err := number.Float64()

	return monthly, err == nil
}

// filtered keeps the elements the declared filter matches.
func (spec *WalkSpec) filtered(elements []DeclaredState, arguments map[string]any) []DeclaredState {
	if spec.Filter == nil {
		return elements
	}

	wanted := spec.Filter.Value
	if spec.Filter.Argument != "" {
		wanted = fmt.Sprintf("%v", arguments[spec.Filter.Argument])
	}

	kept := make([]DeclaredState, 0, len(elements))

	for _, element := range elements {
		if walkFieldMatches(element, spec.Filter.Field, wanted, spec.Filter.Fold) {
			kept = append(kept, element)
		}
	}

	return kept
}

// walkFieldMatches compares one possibly-dotted element field, one level into
// an open object's values for the "parent.*.field" form.
func walkFieldMatches(element DeclaredState, path, wanted string, fold bool) bool {
	head, rest, dotted := strings.Cut(path, ".")
	if !dotted {
		return walkValueEquals(element[head], wanted, fold)
	}

	if member, isStar := strings.CutPrefix(rest, "*."); isStar {
		for _, value := range element.Object(head) {
			slot, isObject := value.(map[string]any)
			if isObject && walkValueEquals(slot[member], wanted, fold) {
				return true
			}
		}

		return false
	}

	return walkValueEquals(element.Object(head)[rest], wanted, fold)
}

// walkValueEquals compares one element value against the wanted text.
func walkValueEquals(value any, wanted string, fold bool) bool {
	rendered := walkRender(value)
	if fold {
		return strings.EqualFold(rendered, wanted)
	}

	return rendered == wanted
}

// emitElement words one element into the declared target. A note whose element
// placeholders all render empty drops the line, the rule every declared
// sentence follows; the dependency itself still lands.
func (spec *WalkSpec) emitElement(
	details *DryRunDetails, element DeclaredState, pools templatePools,
) bool {
	if spec.Emit.SideEffects {
		if line, said := fillTemplate(spec.Emit.Note, element, pools); said {
			details.SideEffects = append(details.SideEffects, line)

			return true
		}

		return false
	}

	kind := spec.Emit.Kind
	if spec.Emit.KindField != "" {
		if kind = walkRender(walkFieldValue(element, spec.Emit.KindField)); kind == "" {
			kind = spec.Emit.KindFallback
		}
	}

	dependency := DryRunDependency{Kind: kind, Action: spec.Emit.Action}

	if spec.Emit.IDField != "" {
		dependency.ID = walkFieldValue(element, spec.Emit.IDField)
	}

	if spec.Emit.LabelField != "" {
		dependency.Label = walkRender(walkFieldValue(element, spec.Emit.LabelField))
	}

	if dependency.Label == "" && spec.Emit.LabelFallback != "" {
		if line, said := fillTemplate(spec.Emit.LabelFallback, element, pools); said {
			dependency.Label = line
		}
	}

	if line, said := fillTemplate(spec.Emit.Note, element, pools); said {
		dependency.Note = line
	}

	details.Dependencies = append(details.Dependencies, dependency)

	return true
}

// walkFieldValue reads one possibly-dotted element value.
func walkFieldValue(element DeclaredState, path string) any {
	head, rest, dotted := strings.Cut(path, ".")
	if !dotted {
		return element[head]
	}

	return element.Object(head)[rest]
}

// sayWarnings words the whole-walk sentences whose predicates hold.
func (spec *WalkSpec) sayWarnings(
	details *DryRunDetails,
	state DeclaredState,
	pools templatePools,
	kept []DeclaredState,
	emitted, total, fetched int,
) {
	pools.aggregates = map[string]string{
		"count":       strconv.Itoa(emitted),
		"total":       strconv.Itoa(fetched),
		"max_results": strconv.Itoa(max(total, emitted)),
	}

	for _, warning := range spec.Warnings {
		fillSums(pools.aggregates, warning.Template+" {"+warning.WhenPositive+"}", kept)

		if !warningHolds(&warning, state, pools.aggregates) {
			continue
		}

		if line, said := fillTemplate(warning.Template, state, pools); said {
			details.Warnings = append(details.Warnings, line)
		}
	}
}

// fillSums computes every {sum:...} the text names over the kept elements: a
// numeric field summed, or a nested list's length summed for the sum:len form.
func fillSums(aggregates map[string]string, text string, kept []DeclaredState) {
	for _, match := range walkPlaceholder.FindAllStringSubmatch(text, -1) {
		name, isSum := strings.CutPrefix(match[1], "sum:")
		if !isSum || aggregates["sum:"+name] != "" {
			continue
		}

		var total int

		for _, element := range kept {
			total += sumOperand(element, name)
		}

		aggregates["sum:"+name] = strconv.Itoa(total)
	}
}

// sumOperand is one element's contribution to a {sum:...} aggregate: a
// nested list's length under the len form, the numeric field otherwise.
func sumOperand(element DeclaredState, name string) int {
	if member, isLen := strings.CutPrefix(name, "len:"); isLen {
		return len(element.Objects(member))
	}

	number, _ := element.Number(name)

	return number
}

// warningHolds evaluates one warning's predicate.
func warningHolds(warning *WalkSpecWarning, state DeclaredState, aggregates map[string]string) bool {
	switch {
	case warning.WhenField != "":
		return strings.EqualFold(state.Text(warning.WhenField), warning.WhenEquals)
	case warning.WhenPositive != "":
		return aggregatePositive(warning.WhenPositive, state, aggregates)
	case warning.WhenPresent != "":
		return walkRender(state[warning.WhenPresent]) != ""
	case warning.WhenAbsent != "":
		return walkRender(state[warning.WhenAbsent]) == ""
	case warning.WhenTruncated:
		count, _ := strconv.Atoi(aggregates["count"])
		total, _ := strconv.Atoi(aggregates["max_results"])

		return total > count
	case warning.WhenAny:
		return aggregates["count"] != "0"
	default:
		return true
	}
}

// aggregatePositive reads one named aggregate as the guard value.
func aggregatePositive(name string, state DeclaredState, aggregates map[string]string) bool {
	if member, isState := strings.CutPrefix(name, "state:"); isState {
		number, _ := state.Number(member)

		return number > 0
	}

	value, _ := strconv.Atoi(aggregates[name])

	return value > 0
}

// templatePools is everything a template can interpolate beside the element.
type templatePools struct {
	state      DeclaredState
	arguments  map[string]any
	enriched   DeclaredState
	aggregates map[string]string
}

// pools assembles the interpolation pools, performing the enrichment read.
func (spec *WalkSpec) pools(
	ctx context.Context,
	client *linode.Client,
	state DeclaredState,
	arguments map[string]any,
	kept []DeclaredState,
) templatePools {
	pools := templatePools{state: state, arguments: arguments, enriched: DeclaredState{}}

	// The read runs only when the walk kept an element, the laziness the
	// hand bodies had by skipping the call outright over nothing.
	if spec.Enrich == nil || len(kept) == 0 {
		return pools
	}

	message := spec.Enrich.New()

	raw, err := client.CallProtoRouteState(ctx, spec.Enrich.Tool, spec.Enrich.Values, "", message)
	if err != nil {
		return pools
	}

	// Decoration only: a projection that answers anything but a DeclaredState
	// leaves the pool empty, the same as a failed read.
	projected, projectErr := ProjectDeclaredState(raw, message)

	if declared, isState := projected.(DeclaredState); projectErr == nil && isState {
		pools.enriched = declared
	}

	return pools
}

// fillTemplate interpolates one template. It reports false when every element
// placeholder rendered empty, which drops the line.
func fillTemplate(template string, element DeclaredState, pools templatePools) (string, bool) {
	if template == "" {
		return "", false
	}

	var elementPlaceholders, filled int

	line := walkPlaceholder.ReplaceAllStringFunc(template, func(match string) string {
		name := match[1 : len(match)-1]
		value, isElement := fillPlaceholder(name, element, pools)

		if isElement {
			elementPlaceholders++

			if value != "" {
				filled++
			}
		}

		return value
	})

	if elementPlaceholders > 0 && filled == 0 {
		return "", false
	}

	return line, true
}

// fillPlaceholder renders one placeholder, reporting whether it read the
// element (the drop rule counts only those).
func fillPlaceholder(name string, element DeclaredState, pools templatePools) (string, bool) {
	switch {
	// A computed aggregate wins only where one was computed, which is what
	// resolves the collision rule by position: warnings fill after the
	// aggregates land, notes before, so a note's {count} is the element's own.
	case pools.aggregates[name] != "":
		return pools.aggregates[name], false
	case strings.HasPrefix(name, "arg:"):
		return walkRender(pools.arguments[strings.TrimPrefix(name, "arg:")]), false
	case strings.HasPrefix(name, "state:"):
		return walkRender(pools.state[strings.TrimPrefix(name, "state:")]), false
	case strings.HasPrefix(name, "enrich:"):
		return walkRender(pools.enriched[strings.TrimPrefix(name, "enrich:")]), false
	case strings.HasPrefix(name, "len:"):
		return strconv.Itoa(len(element.Objects(strings.TrimPrefix(name, "len:")))), true
	case strings.HasPrefix(name, "sum:"):
		return pools.aggregates[name], false
	default:
		return walkRender(walkFieldValue(element, name)), true
	}
}

// walkRender words one value the way both languages spell it.
func walkRender(value any) string {
	if value == nil {
		return ""
	}

	return fmt.Sprintf("%v", value)
}
