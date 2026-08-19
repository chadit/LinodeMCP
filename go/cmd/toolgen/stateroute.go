package main

import (
	"fmt"
	"net/http"
	"slices"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The declared state read, resolved once into the contract model and rendered
// by each arm the way it already renders the sibling read it derives itself.

// readStateRoute records the read a removal declares its state fetch through.
// Resolution waits for resolveStateRoute, which is where the other declared
// tools are in scope to look the named one up.
func (c *contract) readStateRoute(options protoreflect.ProtoMessage) error {
	declared, _ := proto.GetExtension(options, linodev1.E_StateRoute).(*linodev1.StateRoute)
	if declared.GetTool() == "" {
		return nil
	}

	if c.Hooks.FetchState != "" {
		return fmt.Errorf("%w: %s", errStateRouteWithHook, c.Name)
	}

	if c.Hooks.Preview != "" {
		return fmt.Errorf("%w: %s", errStateRouteWithPreviewHook, c.Name)
	}

	if !c.readsDeclaredState() {
		return fmt.Errorf("%w: %s", errStateRouteWithNoReader, c.Name)
	}

	c.StateRouteDecl = declared

	return nil
}

// readsDeclaredState reports whether the tool has a step that acts a declared
// fetch. A removal reads state for its plan as well as its preview; every other
// tier reads it for the dry run alone, so a tool without one would carry a
// declaration nothing ever calls.
func (c *contract) readsDeclaredState() bool {
	return c.removesResource() || c.DryRun
}

// previewReadsState reports whether the tool's dry run is the one that performs
// the declared fetch, which is every tier but the removal: a removal's fetch is
// driven by the destroy flow, ahead of the preview and the plan alike.
func (c *contract) previewReadsState() bool {
	return c.StateRead.declared() && !c.removesResource()
}

// resolveStateRoute turns the declared read into the fetch each arm renders:
// the route's own path slots filled from the removal's arguments, the message
// its answer decodes into, and the nulls that read restores.
//
// The route is looked up rather than trusted so a renamed read stops the run
// here instead of answering an empty preview at call time.
func (c *contract) resolveStateRoute(declared map[string]protoreflect.MessageDescriptor) error {
	if c.StateRouteDecl == nil {
		return nil
	}

	named := c.StateRouteDecl.GetTool()

	message, ok := declared[named]
	if !ok {
		return fmt.Errorf("%w: %s reads through %s", errStateRouteUnknownTool, c.Name, named)
	}

	options := message.Options()

	route, _ := proto.GetExtension(options, linodev1.E_ToolRoute).(*linodev1.ToolRoute)
	if route.GetMethod() != http.MethodGet {
		return fmt.Errorf("%w: %s reads through %s, which is %s",
			errStateRouteNotRead, c.Name, named, route.GetMethod())
	}

	parameters := stateRouteParameters(message)

	slots, err := c.stateRouteSlots(route.GetPath(), parameters, named)
	if err != nil {
		return err
	}

	query, err := c.stateRouteQuery(message, route.GetPath(), named)
	if err != nil {
		return err
	}

	resolved, err := c.stateRouteMessage(options, named)
	if err != nil {
		return err
	}

	c.StateRead = stateRead{
		Tool:    named,
		Message: resolved,
		Slots:   slots,
		Query:   query,
		BodyKey: c.StateRouteDecl.GetBodyKey(),
		Payload: c.StateRouteDecl.GetPayloadMember(),
	}

	return nil
}

// stateRouteQuery names the arguments that fill the read's query parameters,
// each paired with the parameter it fills.
//
// A read addressed by a query parameter is addressed by it as surely as by a
// path segment: the object ACL route names the bucket in its path and the
// object in its query, so a fetch that skipped the query would answer about the
// bucket. A required parameter with nothing to fill it is therefore refused
// here rather than sent empty, which is the same stance the path slots take.
//
// The filling pool is wider than the path slots': a query value can come from
// any argument the tool validates, because the value that addresses the
// resource is not always a path argument. The object key travels in the update's
// own body.
func (c *contract) stateRouteQuery(
	message protoreflect.MessageDescriptor, path, named string,
) ([]stateReadQuery, error) {
	wanted := routeSlots(path)
	mapped := c.stateRouteQueryArguments()
	filled := make([]stateReadQuery, 0)

	fields := message.Fields()

	for i := range fields.Len() {
		field := fields.Get(i)

		if fieldLocation(field) != linodev1.FieldLocation_FIELD_LOCATION_QUERY {
			continue
		}

		parameter := string(field.Name())

		if slices.Contains(wanted, parameter) {
			return nil, fmt.Errorf("%w: %s reads through %s, which names %s in both its path and its query",
				errStateRouteQueryAmbiguous, c.Name, named, parameter)
		}

		argument, err := c.stateRouteQueryFiller(parameter, mapped[parameter], field, named)
		if err != nil {
			return nil, err
		}

		if argument == "" {
			continue
		}

		filled = append(filled, stateReadQuery{Parameter: parameter, Argument: argument})
	}

	return filled, nil
}

// stateRouteQueryFiller is the argument one query parameter is filled from, and
// "" for an optional parameter this tool carries nothing for.
//
// A required parameter with no filler is refused: the fetch would address the
// collection rather than the resource, and no fixture can see it, since a
// preview's stub is matched on the path alone.
func (c *contract) stateRouteQueryFiller(
	parameter, mapped string, field protoreflect.FieldDescriptor, named string,
) (string, error) {
	argument := mapped
	if argument == "" && slices.Contains(c.AllArguments, parameter) {
		argument = parameter
	}

	if argument == "" {
		if field.HasOptionalKeyword() {
			return "", nil
		}

		return "", fmt.Errorf("%w: %s reads through %s, which is addressed by the query parameter %s",
			errStateRouteUnfilledQuery, c.Name, named, parameter)
	}

	entry := c.previewArgument(argument)
	if entry == nil {
		return "", fmt.Errorf("%w: %s fills %s from %s",
			errStateRouteSlotUnknownArgument, c.Name, parameter, argument)
	}

	if entry.Redact || namesSecret(argument) {
		return "", fmt.Errorf("%w: %s fills %s from %s",
			errStateRouteQuerySecret, c.Name, parameter, argument)
	}

	return argument, nil
}

// stateRouteQueryArguments is the query parameters this tool spells differently,
// taken from the same mapping the path slots read.
func (c *contract) stateRouteQueryArguments() map[string]string {
	mapped := make(map[string]string, len(c.StateRouteDecl.GetSlotArguments()))

	for _, entry := range c.StateRouteDecl.GetSlotArguments() {
		mapped[entry.GetReadSlot()] = entry.GetArgument()
	}

	return mapped
}

// stateRouteSlots names the removal's own path arguments that fill the read
// route's template, in the read's slot order.
//
// Matching by name is what lets a read take a subset: the share-group token
// delete carries the token beside the group id and the parent read wants only
// the group. A slot the removal does not carry is refused, since the fetch
// would otherwise address the collection instead of the resource, unless the
// declaration names the argument that fills it.
//
// The answer is spelled in the removal's own argument names, which is what both
// renderer arms look their values up by.
func (c *contract) stateRouteSlots(path string, parameters []string, named string) ([]string, error) {
	wanted := routeSlots(path)
	carried := make(map[string]bool, len(c.Path))

	for i := range c.Path {
		carried[c.Path[i].ProtoName] = true
	}

	mapped, err := c.stateRouteSlotArguments(wanted, parameters, carried, named)
	if err != nil {
		return nil, err
	}

	filled := make([]string, 0, len(wanted))

	for _, slot := range wanted {
		if argument, declared := mapped[slot]; declared {
			filled = append(filled, argument)

			continue
		}

		if !carried[slot] {
			return nil, fmt.Errorf("%w: %s reads through %s, which is addressed by %s",
				errStateRouteUnknownSlot, c.Name, named, slot)
		}

		filled = append(filled, slot)
	}

	return filled, nil
}

// stateRouteSlotArguments is the read slots this removal spells differently,
// each held to both routes before it renames anything.
//
// A mapping can only ever name one of the removal's own validated path
// arguments, so the fetch still addresses the resource the removal does.
func (c *contract) stateRouteSlotArguments(
	wanted, parameters []string, carried map[string]bool, named string,
) (map[string]string, error) {
	declared := c.StateRouteDecl.GetSlotArguments()
	mapped := make(map[string]string, len(declared))

	for _, entry := range declared {
		slot, argument := entry.GetReadSlot(), entry.GetArgument()

		if !slices.Contains(wanted, slot) && !slices.Contains(parameters, slot) {
			return nil, fmt.Errorf("%w: %s maps %s, which %s is not addressed by",
				errStateRouteSlotUnknownRead, c.Name, slot, named)
		}

		if slot == argument {
			return nil, fmt.Errorf("%w: %s maps %s onto itself",
				errStateRouteSlotSameName, c.Name, slot)
		}

		// A path slot is filled from a path argument, so the fetch addresses the
		// resource the tool does. A query parameter is checked against the wider
		// pool where it is read, since the value addressing the resource can
		// travel in the body.
		if !carried[argument] && !slices.Contains(parameters, slot) {
			return nil, fmt.Errorf("%w: %s maps %s to %s",
				errStateRouteSlotUnknownArgument, c.Name, slot, argument)
		}

		mapped[slot] = argument
	}

	return mapped, nil
}

// stateRouteParameters is the query parameters a read route is addressed by.
func stateRouteParameters(message protoreflect.MessageDescriptor) []string {
	fields := message.Fields()
	named := make([]string, 0, fields.Len())

	for i := range fields.Len() {
		field := fields.Get(i)
		if fieldLocation(field) == linodev1.FieldLocation_FIELD_LOCATION_QUERY {
			named = append(named, string(field.Name()))
		}
	}

	return named
}

// stateRouteMessage is the message the declared read's answer decodes into.
func (c *contract) stateRouteMessage(options protoreflect.ProtoMessage, named string) (goMessage, error) {
	response := stringOption(options, linodev1.E_ToolResponse)
	if response == "" {
		return goMessage{}, fmt.Errorf("%w: %s reads through %s, which declares no response",
			errStateRouteNoResponse, c.Name, named)
	}

	answer, err := lookupGoMessage(protoreflect.FullName(response))
	if err != nil {
		return goMessage{}, err
	}

	return c.stateRoutePayload(&answer, named)
}

// stateRoutePayload is the message a declared payload member names, which is
// what the fetch decodes and projects through when the read answers a wrapper
// around the resource the API's own body is.
//
// The member is looked up rather than trusted for the reason the route is: a
// renamed member would otherwise project every key out of the state and leave a
// preview reporting an empty resource.
func (c *contract) stateRoutePayload(answer *goMessage, named string) (goMessage, error) {
	member := c.StateRouteDecl.GetPayloadMember()
	if member == "" {
		return *answer, nil
	}

	if c.StateRouteDecl.GetBodyKey() != "" {
		return goMessage{}, fmt.Errorf("%w: %s", errStateRoutePayloadWithBodyKey, c.Name)
	}

	for _, field := range answer.messageFields() {
		if field.protoName == member {
			return lookupGoMessage(field.message)
		}
	}

	return goMessage{}, fmt.Errorf("%w: %s reads through %s, whose answer %s carries no message member %s",
		errStateRoutePayloadMember, c.Name, named, answer.FullName, member)
}

// stateRouteFields is the removal's path arguments in the read route's slot
// order, which is the order the fetch fills the template in.
func (c *contract) stateRouteFields(ordered []field) []field {
	filled := make([]field, 0, len(c.StateRead.Slots))

	for _, slot := range c.StateRead.Slots {
		index := slices.IndexFunc(ordered, func(entry field) bool {
			return entry.ProtoName == slot
		})
		if index >= 0 {
			filled = append(filled, ordered[index])
		}
	}

	return filled
}
