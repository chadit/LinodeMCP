package main

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// emitDestroy writes a tool that removes one resource.
//
// The whole flow lives in tools.RunDestructiveAction*, which is why this tier
// emits a configuration rather than a handler body: the plan and apply branches,
// the preview, the confirm gate, and the bypass-dry-run gate are one order that
// every destroy shares, and a per-tool body would be 44 chances to get it wrong.
//
// The one step a destroy cannot derive from its own route is the state read:
// the contract says which route a delete calls, not which one reads the thing
// it is about to delete. A removal declared beside a GET on that same route
// says it after all; the rest declare state_route or state_composite.
func emitDestroy(out *source, tool *contract) error {
	if tool.SuccessMessage == "" {
		return fmt.Errorf("%w: %s", errNoSuccessMessage, tool.Name)
	}

	if tool.ConfirmMessage == "" {
		return fmt.Errorf("%w: %s", errNoConfirmMessage, tool.Name)
	}

	if !tool.StateRead.declared() {
		return fmt.Errorf("%w: %s", errNoStateRead, tool.Name)
	}

	ordered, err := tool.orderedPathFields()
	if err != nil {
		return err
	}

	if err := checkDestroyShape(tool, ordered); err != nil {
		return err
	}

	out.need(importContext, importFmt, importMCP, importProto, importConfig,
		importGenpb, importLinode, importProfiles, importTools,
		importToolschemas, importTwostage)

	emitDestroyFactory(out, tool)

	if len(tool.Body) > 0 {
		if err := emitWriteBody(out, tool); err != nil {
			return err
		}
	}

	if err := emitDestroySuccess(out, tool, ordered); err != nil {
		return err
	}

	if singleIDDestroy(tool, ordered) {
		return emitDestroyByID(out, tool, &ordered[0])
	}

	return emitDestroyHandler(out, tool, ordered)
}

// singleIDDestroy reports whether a removal is the shape the single-id wrapper
// serves: one integer id, nothing sent, and nothing decoded back. Everything
// else reads its own ids and hands the shared flow a built path, which is what
// the hand-written multi-id destroys have always done.
func singleIDDestroy(tool *contract, ordered []field) bool {
	return len(ordered) == 1 && ordered[0].Kind == protoreflect.Int32Kind &&
		len(tool.Body) == 0 && tool.PayloadField == ""
}

// checkDestroyShape holds a removal to a route this tier can address: at least
// one id, each of a kind a URL segment carries.
func checkDestroyShape(tool *contract, ordered []field) error {
	if len(ordered) == 0 {
		return fmt.Errorf("%w: %s is addressed by %s", errUnsupportedDestroyShape,
			tool.Name, describePathShape(ordered))
	}

	for _, entry := range ordered {
		if _, _, ok := pathArgReader(entry.Kind); !ok {
			return fmt.Errorf("%w: %s is addressed by %s", errUnsupportedDestroyShape,
				tool.Name, describePathShape(ordered))
		}
	}

	// A resource comes back from a removal that sent something: the routes that
	// answer with one are the rebuilds. Decoding after a bare DELETE has no
	// consumer in either language, so it is refused rather than half-served.
	if tool.PayloadField != "" && len(tool.Body) == 0 {
		return fmt.Errorf("%w: %s decodes %s and sends no body",
			errUnsupportedDestroyShape, tool.Name, tool.PayloadMember)
	}

	return nil
}

// describePathShape words the ids a destroy is addressed by, for the report a
// shape this tier has no driver for stops the run with.
func describePathShape(ordered []field) string {
	if len(ordered) == 0 {
		return "no path id"
	}

	parts := make([]string, 0, len(ordered))
	for _, entry := range ordered {
		parts = append(parts, entry.ProtoName+" ("+entry.Kind.String()+")")
	}

	return strings.Join(parts, " and ")
}

// emitDestroyFactory writes the tool a client sees and the tier it registers at.
func emitDestroyFactory(out *source, tool *contract) {
	factory := factoryName(tool.Name)

	out.writef("// %s builds %s, which %s", factory, tool.Name, describeRoute(tool))
	out.writef("func %s(cfg *config.Config) (mcp.Tool, profiles.Capability, tools.Handler) {", factory)
	out.writef("\ttool := mcp.NewToolWithRawSchema(")
	out.writef("\t\t%s,", goStringLiteral(tool.Name))
	out.writef("\t\t%s,", goStringLiteral(tool.Description))
	out.writef("\t\ttoolschemas.Schema(%s),", goStringLiteral(tool.InputMessage))
	out.writef("\t)")
	out.writef("")
	out.writef("\thandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {")
	out.writef("\t\treturn %s(ctx, &request, cfg)", handlerName(tool.Name))
	out.writef("\t}")
	out.writef("")
	out.writef("\treturn tool, profiles.%s, handler", capabilityConstant(tool.Capability))
	out.writef("}")
	out.writef("")
}

// emitDestroySuccess writes the builder for the answer a completed delete
// carries: the declared sentence over the ids the call was addressed by, plus
// the resource for the removals whose route answers with one.
//
// A DELETE answers with an empty body, so there is usually nothing to decode and
// the response is assembled from the request instead. Keeping it a named
// function rather than a closure in the literal below is load-bearing:
// cmd/write-proto-dump reads the SuccessProto field at the call site to decide
// whether a destroy is proto-routed, and a field it cannot see at that literal
// reads as legacy.
func emitDestroySuccess(out *source, tool *contract, ordered []field) error {
	success, err := tool.formatMessage(tool.SuccessMessage)
	if err != nil {
		return err
	}

	name := successName(tool.Name)

	out.writef("// %s builds the answer %s reports %s.", name, tool.Name, destroyOutcome(tool))
	out.writef("func %s(%s) proto.Message {", name, destroySuccessParams(tool, ordered, success))
	out.writef("\treturn &linodev1.%s{", tool.ResponseGo.TypeName)
	out.writef("\t\t%s: fmt.Sprintf(%s),", tool.MessageField, success.call(""))

	for index := range tool.Echoes {
		echo := &tool.Echoes[index]

		value, err := echoValue(tool.Name, echo)
		if err != nil {
			return err
		}

		out.writef("\t\t%s: %s,", echo.GoName, value)
	}

	if tool.PayloadField != "" {
		out.writef("\t\t%s: payload,", tool.PayloadField)
	}

	out.writef("\t}")
	out.writef("}")
	out.writef("")

	return nil
}

// destroyOutcome words what a completed removal left behind, for the doc
// comment on its answer builder. A tool that answers with a resource rebuilt it
// rather than removed it, and the comment says which.
func destroyOutcome(tool *contract) string {
	if tool.PayloadField != "" {
		return "over the resource it replaced"
	}

	return "once the resource is gone"
}

// destroySuccessParams renders what the answer builder is handed: the request
// for a sentence that reads an argument off it, one local per path id, and the
// decoded resource for a removal whose route answers with one.
func destroySuccessParams(tool *contract, ordered []field, success formatted) string {
	params := make([]string, 0, len(ordered)+2)

	if readsRequest(success) {
		params = append(params, "request *mcp.CallToolRequest")
	}

	for _, entry := range ordered {
		params = append(params, entry.GoLocal+" "+pathLocalType(entry.Kind))
	}

	if tool.PayloadField != "" {
		params = append(params, "payload *linodev1."+tool.PayloadGo.TypeName)
	}

	return strings.Join(params, ", ")
}

// readsRequest reports whether a rendered sentence reads an argument straight
// off the call, which is how a BODY member reaches a builder that holds no local
// for it. A sentence naming only path ids reads none, and taking the request
// there would leave the parameter unused.
func readsRequest(rendered formatted) bool {
	for _, arg := range rendered.args {
		if strings.HasPrefix(arg, "request.") {
			return true
		}
	}

	return false
}

// pathLocalType is the Go type a path id is read into, which is the type the
// request accessor answers with rather than the proto's own.
func pathLocalType(kind protoreflect.Kind) string {
	if kind == protoreflect.Int32Kind || kind == protoreflect.Int64Kind {
		return typeWordInt
	}

	return textType
}

// emitDestroyByID writes the configuration the single-id destroy flow runs.
//
// Execute goes through the routed primitive rather than a per-family client
// method: a delete sends no body and reads nothing back, so the tool name and
// the id are the whole call, and the contract already holds the rest.
func emitDestroyByID(out *source, tool *contract, id *field) error {
	pattern, err := tool.destroyPathPattern()
	if err != nil {
		return err
	}

	out.writef("func %s(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {",
		handlerName(tool.Name))
	out.writef("\treturn tools.RunDestructiveActionWithID(ctx, request, cfg, &tools.DestructiveActionByID{")
	out.writef("\t\tToolName:       %s,", goStringLiteral(tool.Name))
	out.writef("\t\tInputMessage:   %s,", goStringLiteral(tool.InputMessage))
	out.writef("\t\tIDParam:        %s,", goStringLiteral(id.ProtoName))
	out.writef("\t\tMethod:         %s,", goStringLiteral(tool.Method))
	out.writef("\t\tPathPattern:    %s,", goStringLiteral(pattern))
	out.writef("\t\tConfirmMessage: %s,", goStringLiteral(tool.ConfirmMessage))
	out.writef("\t\tSuccessProto:   %s,", successName(tool.Name))
	emitDestroyByIDFetch(out, tool, id)
	out.writef("\t\tExecute: func(ctx context.Context, client *linode.Client, %s int) error {", id.GoLocal)
	out.writef("\t\t\treturn client.CallRoute(ctx, %s, []any{%s})", goStringLiteral(tool.Name), id.GoLocal)
	out.writef("\t\t},")

	// The id the sentence names is read here rather than taken as a parameter:
	// the wrapper parses it for the call, and a sentence only reports it.
	if err := emitDestroyFailure(out, tool, []field{*id}, readAgain); err != nil {
		return err
	}

	emitDependencyWalk(out, tool, id.GoLocal+" int", []field{*id})

	out.writef("\t\tHashIgnore:     twostage.HashIgnoreFields(%s),", goStringLiteral(tool.ResourceType))
	out.writef("\t})")
	out.writef("}")
	out.writef("")

	return nil
}

// emitDependencyWalk writes the walk the removal declares, reading the state
// the fetch it is paired with produces. A removal that declares none writes no
// closure: its preview carries the call and the state alone.
func emitDependencyWalk(out *source, tool *contract, parsed string, ordered []field) {
	// The seeded prose rides the same closure the walks do, so a removal that
	// declares sentences and no walk still reports them on its plan.
	if len(tool.DepWalks) > 0 || len(tool.PreviewSentences) > 0 || tool.BillingDecl != nil {
		emitDeclaredWalks(out, tool, parsed, ordered)
	}
}

// emitDestroyByIDFetch writes the state fetch the single-id wrapper is handed,
// which takes the id it parsed rather than closing over one.
func emitDestroyByIDFetch(out *source, tool *contract, id *field) {
	out.writef("\t\tFetchState: func(ctx context.Context, client *linode.Client, %s int) (any, error) {", id.GoLocal)
	emitStateReadBody(out, tool, "[]any{"+id.GoLocal+"}")
	out.writef("\t\t},")
}

// emitStateFetch writes what a removal's state fetch does: read the sibling it
// resolved.
func emitStateFetch(out *source, tool *contract, ordered []field) {
	if tool.StateRead.scan() {
		emitScanStateRead(out, tool, ordered)

		return
	}

	if tool.StateRead.Envelope {
		emitEnvelopeStateRead(out, tool, ordered)

		return
	}

	if tool.StateRead.collection() {
		emitCollectionStateRead(out, tool, ordered)

		return
	}

	emitStateReadBody(out, tool, pathValuesLiteral(tool.stateReadValues(ordered)))
}

// emitScanStateRead writes the whole-collection scan a declared match resolves
// to: page the named list and answer the first element every pair selects,
// with the pairs worded into the not-found sentence.
func emitScanStateRead(out *source, tool *contract, ordered []field) {
	out.need(importFmt)

	element := "linodev1." + tool.StateRead.Message.TypeName

	conditions := make([]string, 0, len(tool.StateRead.Matches))
	wording := make([]string, 0, len(tool.StateRead.Matches))
	locals := make([]string, 0, len(tool.StateRead.Matches))

	for _, match := range tool.StateRead.Matches {
		local := matchLocal(match.Argument, ordered)
		conditions = append(conditions, "item.Get"+match.GoGetter+"() == "+local)
		wording = append(wording, match.Field+"='%v'")
		locals = append(locals, local)
	}

	out.writef("			return tools.FetchCollectionScan(ctx, client, %s, %s,",
		goStringLiteral(tool.StateRead.Tool), stateFormValuesLiteral(tool, ordered))
	out.writef("				func() *%s { return &%s{} },", element, element)
	out.writef("				func(item *%s) bool { return %s },", element, strings.Join(conditions, " && "))
	out.writef("				fmt.Sprintf(%s, %s))",
		goStringLiteral(strings.Join(wording, ", ")), strings.Join(locals, ", "))
}

// stateFormValuesLiteral is the values a scan or envelope read's own slots
// take, strictly from the declared slot fill: a slotless list takes none, and
// the removal's path values must not leak into it.
func stateFormValuesLiteral(tool *contract, ordered []field) string {
	values := make([]string, 0, len(tool.StateRead.Slots))
	for _, slot := range tool.StateRead.Slots {
		values = append(values, matchLocal(slot, ordered))
	}

	return "[]any{" + strings.Join(values, ", ") + "}"
}

// matchLocal is the handler local one match argument was read into.
func matchLocal(argument string, ordered []field) string {
	for index := range ordered {
		if ordered[index].ProtoName == argument {
			return ordered[index].GoLocal
		}
	}

	return argument
}

// emitEnvelopeStateRead writes the fetch that keeps the page envelope itself
// as the state, results total included.
func emitEnvelopeStateRead(out *source, tool *contract, ordered []field) {
	emitEnvelopeFetch(out, tool, stateFormValuesLiteral(tool, ordered), "\t\t\t")
}

// emitStateReadBody writes the synthesized state fetch: the sibling read's
// route, decoded into the message that read answers with, reported as the body
// the API sent projected through that message.
//
// The decode stays because it is what refuses a body the read's contract does
// not describe; what a preview reports is the projection, so a key the API sent
// as null survives and one it never sent is not invented.
func emitStateReadBody(out *source, tool *contract, values string) {
	out.writef("\t\t\tstate := &linodev1.%s{}", tool.StateRead.Message.TypeName)
	out.writef("")
	// Through the shared call so a read a path alone does not address carries
	// its query here too: the object ACL names the bucket in its path and the
	// object in its query, and a fetch that dropped the query would preview the
	// bucket while the removal took one object out of it.
	out.writef("\t\t\t%s", goStateReadCall(tool, values))
	out.writef("\t\t\tif err != nil {")
	out.writef("\t\t\t\treturn nil, err")
	out.writef("\t\t\t}")
	out.writef("")
	out.writef("\t\t\treturn tools.ProjectDeclaredState(raw, state)")
}

// emitCollectionStateRead writes the state fetch of a removal whose API
// publishes no GET on the route it deletes: the collection one segment up is
// read with the parent's ids, and the trailing id picks the element out of it.
func emitCollectionStateRead(out *source, tool *contract, ordered []field) {
	element := "linodev1." + tool.StateRead.Message.TypeName
	parent := ordered[:len(ordered)-1]
	trailing := ordered[len(ordered)-1]

	out.writef("\t\t\treturn tools.FetchCollectionElement(ctx, client, %s, %s, %s,",
		goStringLiteral(tool.StateRead.Tool), pathValuesLiteral(parent), trailing.GoLocal)
	out.writef("\t\t\t\tfunc() *%s { return &%s{} },", element, element)
	out.writef("\t\t\t\tfunc(item *%s) bool { return item.Get%s() == %s })",
		element, tool.StateRead.Select, trailing.GoLocal)
}

// stateReadValues is the removal's path arguments in the order the state read
// fills its own template. A derived sibling shares the removal's template and so
// takes them as they come; a declared route names its own slots.
func (c *contract) stateReadValues(ordered []field) []field {
	if len(c.StateRead.Slots) == 0 {
		return ordered
	}

	return c.stateRouteFields(ordered)
}

// readAgain and alreadyRead select how a failure sentence reaches the ids it
// names: the single-id wrapper leaves the handler holding none, while a handler
// that parsed its own ids already has them in scope.
const (
	readAgain   = true
	alreadyRead = false
)

// emitDestroyFailure writes the tool's own failure sentence, or nothing when it
// declares none and the tier's shared sentence answers instead.
//
// A removal that declares error_message means to say something its family's
// callers recognize, naming the resource it could not remove. Answering the
// shared sentence over that declaration was prose drift no gate saw, the same
// drift emitListFailure closes for collections.
func emitDestroyFailure(out *source, tool *contract, ordered []field, reread bool) error {
	if tool.ErrorMessage == "" {
		return nil
	}

	failure, err := tool.formatMessage(tool.ErrorMessage)
	if err != nil {
		return err
	}

	out.need(importFmt)

	out.writef("\t\tFailure: func(err error) string {")

	if reread {
		emitPathArgReads(out, namedLocals(ordered, failure.args))
	}

	out.writef("\t\t\treturn fmt.Sprintf(%s)", failure.call("err"))
	out.writef("\t\t},")

	return nil
}

// emitDestroyHandler writes the removal whose route the single-id wrapper cannot
// address: more than one id, an id that is not a number, a body to send, or a
// resource to decode back.
//
// It reads and refuses its own ids so every slot answers the sentences the
// wrapper answers for the one it parses, then hands the shared flow a built path
// the way the hand-written multi-id destroys always have.
func emitDestroyHandler(out *source, tool *contract, ordered []field) error {
	path, err := tool.pathExpression(out)
	if err != nil {
		return err
	}

	out.writef("func %s(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {",
		handlerName(tool.Name))

	emitConstraintCheck(out, tool)

	for index := range ordered {
		emitDestroyPathArg(out, &ordered[index])
	}

	if len(tool.Body) > 0 {
		out.writef("\tbody, message := %s(request)", bodyName(tool.Name))
		out.writef("\tif message != \"\" {")
		out.writef("\t\treturn mcp.NewToolResultError(message), nil")
		out.writef("\t}")
		out.writef("")
	}

	if tool.PayloadField != "" {
		out.writef("\tvar payload linodev1.%s", tool.PayloadGo.TypeName)
		out.writef("")
	}

	out.writef("\treturn tools.RunDestructiveAction(ctx, request, cfg, &tools.DestructiveAction{")
	out.writef("\t\tToolName:       %s,", goStringLiteral(tool.Name))
	out.writef("\t\tMethod:         %s,", goStringLiteral(tool.Method))
	out.writef("\t\tPath:           %s,", path)
	out.writef("\t\tConfirmMessage: %s,", goStringLiteral(tool.ConfirmMessage))
	out.writef("\t\tFetchState: func(ctx context.Context, client *linode.Client) (any, error) {")
	emitStateFetch(out, tool, ordered)
	out.writef("\t\t},")
	out.writef("\t\tExecute: func(ctx context.Context, client *linode.Client) error {")
	out.writef("\t\t\treturn %s", destroyCall(tool, ordered))
	out.writef("\t\t},")
	out.writef("\t\tSuccess: func() proto.Message {")
	out.writef("\t\t\treturn %s(%s)", successName(tool.Name), destroySuccessArguments(tool, ordered))
	out.writef("\t\t},")

	if err := emitDestroyFailure(out, tool, ordered, alreadyRead); err != nil {
		return err
	}

	emitDestroyPreviewBody(out, tool)

	emitDependencyWalk(out, tool, "", ordered)

	out.writef("\t\tHashIgnore:     twostage.HashIgnoreFields(%s),", goStringLiteral(tool.ResourceType))
	out.writef("\t})")
	out.writef("}")
	out.writef("")

	return nil
}

// emitDestroyPathArg writes the read and the refusal for one path id. An integer
// goes through the tier's own reader so a fractional value is refused rather
// than truncated onto a resource nobody named; a label is refused when blank,
// which is all an absent one reads as.
func emitDestroyPathArg(out *source, entry *field) {
	if entry.Kind == protoreflect.Int32Kind || entry.Kind == protoreflect.Int64Kind {
		out.writef("\t%s, message := tools.DestroyID(request, %s)", entry.GoLocal, goStringLiteral(entry.ProtoName))
		out.writef("\tif message != \"\" {")
		out.writef("\t\treturn mcp.NewToolResultError(message), nil")
		out.writef("\t}")
		out.writef("")

		return
	}

	out.writef("\t%s := request.GetString(%s, %s)", entry.GoLocal, goStringLiteral(entry.ProtoName), emptyLiteral)
	out.writef("\tif %s == %s {", entry.GoLocal, emptyLiteral)
	out.writef("\t\treturn mcp.NewToolResultError(%s), nil", goStringLiteral(entry.ProtoName+" is required"))
	out.writef("\t}")
	out.writef("")
}

// emitDestroyPreviewBody names the body a preview reports beside the route, for
// the removals whose call carries one. The redacting form stands in for the
// members the contract marks, so a preview of a rebuild does not print the root
// password the live call sends.
func emitDestroyPreviewBody(out *source, tool *contract) {
	if len(tool.Body) == 0 {
		return
	}

	out.writef("\t\tPreviewBody:    %s,", previewBody(tool))
}

// destroyCall is the request a removal makes: the routed primitive with whatever
// the contract says travels with it, since the tool and its ids are the rest.
//
// A declared transport takes the call instead. The tier gate admits only the
// removal arm here, and it decodes nothing back, so the closure still answers
// with an error alone.
func destroyCall(tool *contract, ordered []field) string {
	values := pathValuesLiteral(ordered)
	name := goStringLiteral(tool.Name)

	if tool.transported() {
		return presignRemoveCall(tool, values)
	}

	if len(tool.Body) == 0 {
		return fmt.Sprintf("client.CallRoute(ctx, %s, %s)", name, values)
	}

	if tool.PayloadField == "" {
		return fmt.Sprintf("client.CallRouteBody(ctx, %s, %s, body)", name, values)
	}

	return fmt.Sprintf("client.CallProtoRouteBody(ctx, %s, %s, body, %s, &payload)",
		name, values, goStringLiteral(responseSubject(tool.Name)))
}

// presignRemoveCall renders the removal arm as one spec literal the shared
// engine runs, the pattern the other transported tiers render through.
func presignRemoveCall(tool *contract, values string) string {
	return "tools.RunPresignRemove(ctx, client, &tools.PresignSpec{\n" +
		"\t\t\t\tTool: " + goStringLiteral(tool.Name) + ",\n" +
		"\t\t\t\tSubject: " + goStringLiteral(responseSubject(tool.Name)) + ",\n" +
		"\t\t\t\tURLField: " + goStringLiteral(tool.Transport.URLField) + ",\n" +
		"\t\t\t\tPathValues: " + values + ",\n" +
		"\t\t\t\tBody: " + executeBody(tool) + ",\n" +
		"\t\t\t})"
}

// destroySuccessArguments renders the call to the answer builder, in the order
// destroySuccessParams declares its parameters.
func destroySuccessArguments(tool *contract, ordered []field) string {
	success, err := tool.formatMessage(tool.SuccessMessage)
	if err != nil {
		return ""
	}

	args := make([]string, 0, len(ordered)+2)

	if readsRequest(success) {
		args = append(args, "request")
	}

	for _, entry := range ordered {
		args = append(args, entry.GoLocal)
	}

	if tool.PayloadField != "" {
		args = append(args, "&payload")
	}

	return strings.Join(args, ", ")
}

// destroyPathPattern renders the route template as the Sprintf pattern the
// destroy driver fills, "/domains/{domain_id}" as "/domains/%d". The driver
// owns the substitution, so the pattern is what a generated call site hands it
// rather than a path it filled itself.
//
// The pattern is display-only: it feeds the dry-run preview, while the removal
// itself resolves its route from the contract. It carries the surface for that
// reason, so a preview names the request that would run rather than one on the
// default base.
func (c *contract) destroyPathPattern() (string, error) {
	filled, err := c.formatMessage(routePath(c))
	if err != nil {
		return "", err
	}

	return filled.format, nil
}

// successName is the answer builder a destroy's configuration names.
func successName(tool string) string {
	return "success" + exportedToolName(tool)
}
