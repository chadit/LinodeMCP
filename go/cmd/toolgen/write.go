package main

import (
	"fmt"
	"slices"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// tagsField is the one body field with a reader of its own across the whole
// surface: entries are trimmed, a blank one is refused, and a JSON-encoded
// array is accepted as readily as a native one. It is selected by name because
// the convention is the field's, not any one tool's, and both languages spell
// the rule once in their body builder.
const tagsField = "tags"

// toolPrefix is the prefix every tool name carries, dropped when the name has
// to read as prose rather than address a tool.
const toolPrefix = "linode_"

// responseSubject names the call a malformed response is reported against, in
// prose. Every mutation holds its response to being a JSON object, and the
// report says which call answered badly. Both languages take the subject from
// this one derivation, so they report one sentence.
func responseSubject(tool string) string {
	return strings.ReplaceAll(strings.TrimPrefix(tool, toolPrefix), "_", " ")
}

// emitWrite writes a tool that sends a body to create or change one resource.
//
// The branch order is the contract this tier exists to hold, and it differs
// between the two paths on purpose: a dry run validates before anything else,
// because there is nothing to confirm, while a live call gates on confirm
// BEFORE it validates, so a caller who has not confirmed learns nothing about
// whether their arguments would have been accepted.
func emitWrite(out *source, tool *contract) error {
	if tool.ErrorMessage == "" {
		return fmt.Errorf("%w: %s", errNoErrorMessage, tool.Name)
	}

	// A response with no message field reports no prose of its own, so the
	// sentence every other mutation declares would go nowhere.
	if tool.SuccessMessage == "" && tool.MessageField != "" {
		return fmt.Errorf("%w: %s", errNoSuccessMessage, tool.Name)
	}

	if tool.ConfirmMessage == "" {
		return fmt.Errorf("%w: %s", errNoConfirmMessage, tool.Name)
	}

	// The raw primitive a null-restoring mutation calls takes no query, so the
	// two declarations together would advertise parameters the call drops.
	if len(tool.ExplicitNulls) > 0 && len(tool.Query) > 0 {
		return fmt.Errorf("%w: %s publishes %s beside explicit_null_fields",
			errUnsupportedQuery, tool.Name, queryNames(tool.Query))
	}

	out.need(importContext, importFmt, importMCP, importConfig, importGenpb,
		importProfiles, importTools, importToolschemas)

	// The free-form body decodes into the well-known message rather than a
	// generated one, which is the only type the handler names from elsewhere.
	if tool.PayloadStruct && !tool.DecodedResponse {
		out.need(importStructpb)
	}

	emitToolFactory(out, tool)

	if err := emitWriteBody(out, tool); err != nil {
		return err
	}

	if err := emitWriteHandler(out, tool); err != nil {
		return err
	}

	return emitWritePreview(out, tool)
}

// emitToolFactory writes the tool a client sees and the tier it registers at.
// Every tier whose handler takes the whole request over shares it.
func emitToolFactory(out *source, tool *contract) {
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

// emitWriteBody writes the request-body builder: one line per BODY field, in
// the order the proto declares them, which is the order they reach the wire.
func emitWriteBody(out *source, tool *contract) error {
	out.writef("// %s builds the %s body in proto field order.", bodyName(tool.Name), tool.Method)
	out.writef("func %s(request *mcp.CallToolRequest) (*tools.WriteBody, string) {", bodyName(tool.Name))
	out.writef("\tbody := tools.NewWriteBody(request, %d)", len(tool.Body))

	for _, entry := range tool.Body {
		if entry.BodyName == "" {
			continue
		}

		out.writef("\tbody.Rename(%s, %s)",
			goStringLiteral(entry.ProtoName), goStringLiteral(entry.BodyName))
	}

	out.writef("")

	for _, entry := range tool.Body {
		// A folded argument travels inside the member it was declared into, so
		// writing it here too would post it where the API does not read it.
		if tool.Folded[entry.ProtoName] {
			continue
		}

		if _, assembled := tool.Folds[entry.ProtoName]; assembled {
			emitFold(out, tool, &entry)

			continue
		}

		setter, err := bodySetter(tool.Name, &entry)
		if err != nil {
			return err
		}

		message := entry.typedMessage()
		if message == nil {
			out.writef("\tbody.%s(%s)", setter, goStringLiteral(entry.ProtoName))

			continue
		}

		if err := emitMessageFields(out, tool.Name, setter, &entry, message); err != nil {
			return err
		}
	}

	for _, constant := range tool.Constants {
		out.writef("\tbody.Constant(%s, %s)", goStringLiteral(constant.Name), constant.Literal)
	}

	out.writef("")
	out.writef("\treturn body, body.Message()")
	out.writef("}")
	out.writef("")

	return nil
}

// bodySetter names the builder call one field is written through. A kind with no
// call fails the run by name: a field quietly left out of the body would be a
// tool that advertises an argument and drops it.
func bodySetter(tool string, entry *field) (string, error) {
	// Checked before the list branch because a map's cardinality is repeated,
	// and before the kind switch because its kind is the map entry's message.
	if entry.ObjectMap {
		// All three object shapes are the same proto declaration, so only the
		// contract separates them: the plain setter reads a null as absence,
		// which is the state a detach cannot afford to lose, and the root
		// setter hoists the members onto a body that has no member for them.
		if entry.BodyRoot {
			return "SetObjectRoot", nil
		}

		if entry.Nullable {
			return "SetObjectOrNull", nil
		}

		return "SetObject", nil
	}

	// Same map cardinality as the free-form shape above, so it is read before
	// the list branch for the same reason.
	if entry.StringMap {
		return "SetStringMap", nil
	}

	if entry.Repeated {
		return repeatedSetter(tool, entry)
	}

	// After the map branches because a map's kind is its entry's message: this
	// is the named message a body carries as one object rather than a table.
	if entry.Message != nil {
		if entry.Nullable {
			return "SetMessageOrNull", nil
		}

		return "SetMessage", nil
	}

	verb := "Set"
	if !entry.Presence {
		verb = "Put"
	}

	// Ahead of the string branch it would otherwise take: the argument is text
	// and the member is an array, so only the contract separates the two.
	if entry.CommaList {
		return "SetCommaStringList", nil
	}

	// An enum reaches the wire as the name it declares, so it is read and sent
	// as a string; the argument check is what holds it to the declared set.
	if entry.Kind == protoreflect.StringKind || entry.Kind == protoreflect.EnumKind {
		// A null is the API's clear-this-field spelling, which only a string
		// carries: an enum's null names no member of its set, and the contract
		// check refuses the declaration there.
		if entry.Nullable && entry.Kind == protoreflect.StringKind {
			return "SetStringOrNull", nil
		}

		return verb + "String", nil
	}

	if entry.Kind == protoreflect.Int32Kind || entry.Kind == protoreflect.Int64Kind {
		return verb + "Int", nil
	}

	if entry.Kind == protoreflect.BoolKind {
		return verb + "Bool", nil
	}

	return "", fmt.Errorf("%w: %s body field %s is %s",
		errUnsupportedBodyKind, tool, entry.ProtoName, entry.Kind)
}

// typedMessage is the named message one BODY field is read under, whichever
// cardinality declares it, and nil for a field read as a value.
func (f *field) typedMessage() protoreflect.MessageDescriptor {
	if f.ItemMessage != nil {
		return f.ItemMessage
	}

	return f.Message
}

// emitMessageFields writes a typed message setter: the field name, then the
// members its message declares. The spec travels to the reader inline because
// the shape is the contract's, and a reader handed nothing would accept
// whatever the caller sent.
func emitMessageFields(
	out *source, tool, setter string, entry *field, message protoreflect.MessageDescriptor,
) error {
	out.writef("\tbody.%s(%s, []tools.ItemField{",
		setter, goStringLiteral(entry.ProtoName))

	if err := emitItemFields(out, tool, entry, message, "\t\t", nil); err != nil {
		return err
	}

	out.writef("\t})")

	return nil
}

// emitItemFields writes one message's members, in declaration order, with a
// member carrying a message of its own written out under the same rules at one
// more indent.
//
// seen carries the messages this field is already inside, so a declaration that
// reaches itself fails the run by name rather than emitting until the emitter
// runs out of stack.
func emitItemFields(
	out *source, tool string, entry *field, message protoreflect.MessageDescriptor,
	indent string, seen []protoreflect.FullName,
) error {
	if slices.Contains(seen, message.FullName()) {
		return fmt.Errorf("%w: %s body field %s reaches %s again",
			errRecursiveItemMessage, tool, entry.ProtoName, message.FullName())
	}

	seen = append(slices.Clone(seen), message.FullName())
	members := message.Fields()

	for i := range members.Len() {
		member := members.Get(i)

		kind, err := itemFieldKind(tool, entry, member)
		if err != nil {
			return err
		}

		nested := itemMemberMessage(member)
		if nested == nil {
			out.writef("%s{Name: %s, Kind: tools.%s%s},",
				indent, goStringLiteral(string(member.Name())), kind, itemRequired(member))

			continue
		}

		out.writef("%s{Name: %s, Kind: tools.%s, Fields: []tools.ItemField{",
			indent, goStringLiteral(string(member.Name())), kind)

		if err := emitItemFields(out, tool, entry, nested, indent+"\t", seen); err != nil {
			return err
		}

		out.writef("%s}},", indent)
	}

	return nil
}

// itemRequired renders one member's presence rule. Required is the inverse of
// presence, the same rule that picks Put over Set for a scalar. A map or a list
// carries no proto3 `optional`, so its declaration reads as required whatever
// the caller may omit; its presence is read from the item instead, the way the
// field-level object setter reads its own.
func itemRequired(member protoreflect.FieldDescriptor) string {
	if member.HasPresence() || member.IsList() || member.IsMap() {
		return ""
	}

	return ", Required: true"
}

// itemFieldKind names the reader kind one item member is held to. A kind with
// no reader fails the run rather than reaching the wire unchecked.
func itemFieldKind(tool string, entry *field, member protoreflect.FieldDescriptor) (string, error) {
	// Both shapes below are repeated in the descriptor, a map's cardinality as
	// much as a list's, so they are read ahead of the refusal that catches
	// every other repeating member.
	if isObjectMap(member) {
		return itemObjectKind, nil
	}

	if member.IsList() && member.Kind() == protoreflect.StringKind {
		return "ItemStringList", nil
	}

	// Before the repeat refusal for the same reason: a named message is the one
	// repeating shape whose members the reader can be told about.
	if itemMemberMessage(member) != nil {
		if member.IsList() {
			return "ItemMessageList", nil
		}

		return "ItemMessage", nil
	}

	if member.IsList() || member.IsMap() {
		return "", fmt.Errorf("%w: %s body field %s item member %s repeats",
			errUnsupportedItemKind, tool, entry.ProtoName, member.Name())
	}

	// An enum reaches the wire as the name it declares, so an item member reads
	// it as a string, the same way a scalar enum field does.
	if member.Kind() == protoreflect.StringKind || member.Kind() == protoreflect.EnumKind {
		return itemStringKind, nil
	}

	if member.Kind() == protoreflect.Int32Kind || member.Kind() == protoreflect.Int64Kind {
		return itemIntKind, nil
	}

	if member.Kind() == protoreflect.BoolKind {
		return itemBoolKind, nil
	}

	return "", fmt.Errorf("%w: %s body field %s item member %s is %s",
		errUnsupportedItemKind, tool, entry.ProtoName, member.Name(), member.Kind())
}

// repeatedSetter names the call a list field is written through.
func repeatedSetter(tool string, entry *field) (string, error) {
	if entry.ProtoName == tagsField {
		return "SetTags", nil
	}

	// Checked before the kind switch because a list of free-form objects is a
	// list of messages, which no scalar branch can name.
	if entry.ObjectList {
		return "SetObjectList", nil
	}

	// After the free-form branch because both are repeated messages: the named
	// one is the only shape whose members the reader can be told about.
	if entry.ItemMessage != nil {
		return "SetMessageList", nil
	}

	if entry.Kind == protoreflect.StringKind {
		return "SetStringList", nil
	}

	if entry.Kind == protoreflect.Int32Kind || entry.Kind == protoreflect.Int64Kind {
		return "SetIntList", nil
	}

	return "", fmt.Errorf("%w: %s body field %s repeats %s",
		errUnsupportedBodyKind, tool, entry.ProtoName, entry.Kind)
}

// emitWriteHandler writes the live path: gate, validate, build, send, report.
func emitWriteHandler(out *source, tool *contract) error {
	ordered, err := tool.orderedPathFields()
	if err != nil {
		return err
	}

	failure, err := tool.formatMessage(tool.ErrorMessage)
	if err != nil {
		return err
	}

	out.writef("func %s(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {",
		handlerName(tool.Name))
	out.writef("\tif tools.IsDryRun(request) {")
	out.writef("\t\treturn %s(ctx, request, cfg)", previewName(tool.Name))
	out.writef("\t}")
	out.writef("")
	out.writef("\tif result := tools.RequireConfirm(request, %s); result != nil {",
		goStringLiteral(tool.ConfirmMessage))
	out.writef("\t\treturn result, nil")
	out.writef("\t}")
	out.writef("")

	if err := emitWriteChecks(out, tool, ordered, withEchoes); err != nil {
		return err
	}

	out.writef("\tclient, err := tools.ClientForRequest(request, cfg)")
	out.writef("\tif err != nil {")
	out.writef("\t\treturn mcp.NewToolResultError(err.Error()), nil")
	out.writef("\t}")
	out.writef("")

	if tool.PagedWrite {
		return emitPagedWriteTail(out, tool, ordered, failure)
	}

	out.writef("\tpayload := &%s{}", tool.decodeType())
	out.writef("")

	emitWriteCall(out, tool, ordered)

	out.writef("\t\treturn mcp.NewToolResultError(fmt.Sprintf(%s)), nil", failure.call("err"))
	out.writef("\t}")
	out.writef("")

	return emitWriteResult(out, tool)
}

// emitPagedWriteTail writes the call and the answer of a mutation whose route
// reports a page rather than the resource it changed: the elements are decoded
// through the list envelope and packed into the count and the repeated member
// the collection tiers fill.
//
// No prose travels with them. checkPageMembers has already held the response to
// what the page decode can fill, so there is nothing else here to assemble.
func emitPagedWriteTail(out *source, tool *contract, ordered []field, failure formatted) error {
	out.need(importLinode)

	element := "*linodev1." + tool.ElementGo.TypeName

	out.writef("\titems, err := linode.ListProtoRouteBody(ctx, client, %s, %s, %s, body,",
		goStringLiteral(tool.Name), pathValuesLiteral(ordered), writeQueryExpression(tool))
	out.writef("\t\tfunc() %s { return &linodev1.%s{} })", element, tool.ElementGo.TypeName)
	out.writef("\tif err != nil {")
	out.writef("\t\treturn mcp.NewToolResultError(fmt.Sprintf(%s)), nil", failure.call("err"))
	out.writef("\t}")
	out.writef("")
	out.writef("\treturn tools.MarshalProtoToolResponse(&linodev1.%s{%s: tools.ClampListCount(items), %s: items})",
		tool.ResponseGo.TypeName, tool.CountField, tool.EnvelopeField)
	out.writef("}")
	out.writef("")

	return nil
}

// writeQueryExpression is the query a mutation's routed call carries: the local
// its checks built, or the empty string for the mutations that publish no query
// arguments at all, which is nearly all of them.
func writeQueryExpression(tool *contract) string {
	if len(tool.Query) == 0 {
		return emptyLiteral
	}

	return "query"
}

// emitWriteCall writes the routed call, opening the failure branch its caller
// closes. A tool restoring explicit nulls keeps the raw body, since the decoded
// message no longer says which of its absent members the API sent as null.
func emitWriteCall(out *source, tool *contract, ordered []field) {
	subject := goStringLiteral(responseSubject(tool.Name))

	if len(tool.ExplicitNulls) == 0 {
		if len(tool.Query) > 0 {
			out.writef("\tif err := client.CallProtoRouteBodyQuery(ctx, %s, %s, query, body, %s, payload); err != nil {",
				goStringLiteral(tool.Name), pathValuesLiteral(ordered), subject)

			return
		}

		out.writef("\tif err := client.CallProtoRouteBody(ctx, %s, %s, body, %s, payload); err != nil {",
			goStringLiteral(tool.Name), pathValuesLiteral(ordered), subject)

		return
	}

	out.writef("\traw, err := client.CallProtoRouteBodyRaw(ctx, %s, %s, body, %s, payload)",
		goStringLiteral(tool.Name), pathValuesLiteral(ordered), subject)
	out.writef("\tif err != nil {")
}

// decodeType names the Go type a mutation's routed call decodes into, qualified
// by the package that declares it.
//
// It is the whole envelope for a tool whose answer carries members beside the
// resource, the resource itself otherwise, and the well-known Struct for a body
// the API round-trips with no schema to model it.
func (c *contract) decodeType() string {
	if c.DecodedResponse {
		return "linodev1." + c.ResponseGo.TypeName
	}

	if c.PayloadStruct {
		return "structpb." + c.PayloadGo.TypeName
	}

	return "linodev1." + c.PayloadGo.TypeName
}

// payloadVar names the value a mutation's prose reads its fields off. A tool
// that decodes its whole envelope holds the resource in a member of it, so the
// sentence reaches through that member rather than reading the envelope.
func (c *contract) payloadVar() string {
	if c.DecodedResponse && c.PayloadField != "" {
		return "payload.Get" + c.PayloadField + "()"
	}

	return "payload"
}

// emitWriteResult writes the answer a completed mutation serializes: the
// declared envelope over the decoded resource, or that resource alone for a tool
// whose declared response is the API resource itself.
func emitWriteResult(out *source, tool *contract) error {
	if tool.BareResource {
		return emitBareResult(out, tool)
	}

	if tool.DecodedResponse {
		return emitDecodedResult(out, tool)
	}

	out.writef("\treturn %s(&linodev1.%s{", marshalCall(tool), tool.ResponseGo.TypeName)

	if err := emitSuccess(out, tool); err != nil {
		return err
	}

	if err := emitWarning(out, tool); err != nil {
		return err
	}

	for index := range tool.Echoes {
		echo := &tool.Echoes[index]

		value, valueErr := echoValue(tool.Name, echo)
		if valueErr != nil {
			return valueErr
		}

		out.writef("\t\t%s: %s,", echo.GoName, value)
	}

	out.writef("\t\t%s: payload,", tool.PayloadField)
	out.writef("\t}%s", marshalTail(tool, tool.PayloadMember))
	out.writef("}")
	out.writef("")

	return nil
}

// emitDecodedResult writes the answer of a mutation whose declared envelope the
// API filled: one decode placed the resource and every value beside it, so the
// prose, the notice and any echo are written onto that message rather than
// assembled around it.
func emitDecodedResult(out *source, tool *contract) error {
	success, err := tool.formatSuccess(tool.SuccessMessage, tool.payloadVar())
	if err != nil {
		return err
	}

	out.writef("\tpayload.%s = fmt.Sprintf(%s)", tool.MessageField, success.call(""))

	if tool.WarningField != "" {
		warning, warnErr := tool.formatSuccess(tool.WarningMessage, tool.payloadVar())
		if warnErr != nil {
			return warnErr
		}

		out.writef("\tpayload.%s = %s", tool.WarningField, warningExpression(warning))
	}

	for index := range tool.Echoes {
		echo := &tool.Echoes[index]

		value, valueErr := echoValue(tool.Name, echo)
		if valueErr != nil {
			return valueErr
		}

		out.writef("\tpayload.%s = %s", echo.GoName, value)
	}

	out.writef("")
	out.writef("\treturn %s(payload)", marshalCall(tool))
	out.writef("}")
	out.writef("")

	return nil
}

// emitBareResult writes the answer of a mutation whose declared response is the
// API resource: the decoded body, with the notice written onto it rather than
// assembled around it, which is the only member such a response can be told.
func emitBareResult(out *source, tool *contract) error {
	if tool.WarningField != "" {
		warning, err := tool.formatSuccess(tool.WarningMessage, tool.payloadVar())
		if err != nil {
			return err
		}

		out.writef("\tpayload.%s = %s", tool.WarningField, warningExpression(warning))
		out.writef("")
	}

	out.writef("\treturn %s(payload%s)", marshalCall(tool), marshalArgs(tool, ""))
	out.writef("}")
	out.writef("")

	return nil
}

// marshalCall names the serializer a mutation answers through: the shared one,
// or the variant that writes back the explicit nulls the decode dropped.
func marshalCall(tool *contract) string {
	if len(tool.ExplicitNulls) == 0 {
		return "tools.MarshalProtoToolResponse"
	}

	return "tools.MarshalProtoToolResponseRestoringNulls"
}

// marshalArgs renders the raw body, the member it decoded into, and the fields
// to restore, or nothing when the tool declares none.
func marshalArgs(tool *contract, member string) string {
	if len(tool.ExplicitNulls) == 0 {
		return ""
	}

	names := make([]string, 0, len(tool.ExplicitNulls))
	for _, name := range tool.ExplicitNulls {
		names = append(names, goStringLiteral(name))
	}

	return fmt.Sprintf(", raw, %s, []string{%s}", goStringLiteral(member), strings.Join(names, ", "))
}

// marshalTail closes an envelope literal with whatever its serializer takes.
func marshalTail(tool *contract, member string) string {
	return marshalArgs(tool, member) + ")"
}

// emitSuccess writes the sentence a completed mutation reports. A response with
// no message field has nowhere to carry one, and readWriteEnvelope has already
// refused a tool that declares one anyway.
func emitSuccess(out *source, tool *contract) error {
	if tool.MessageField == "" {
		return nil
	}

	success, err := tool.formatSuccess(tool.SuccessMessage, tool.payloadVar())
	if err != nil {
		return err
	}

	out.writef("\t\t%s: fmt.Sprintf(%s),", tool.MessageField, success.call(""))

	return nil
}

// emitWarning writes the notice a response promises, filled the way the success
// sentence beside it is so both read the resource the API answered with.
func emitWarning(out *source, tool *contract) error {
	if tool.WarningField == "" {
		return nil
	}

	warning, err := tool.formatSuccess(tool.WarningMessage, tool.payloadVar())
	if err != nil {
		return err
	}

	out.writef("\t\t%s: %s,", tool.WarningField, warningExpression(warning))

	return nil
}

// warningExpression renders a notice as the literal it is when the template
// names no field, so a deliberately empty warning reaches the tree as "" rather
// than as a call with nothing to format.
func warningExpression(warning formatted) string {
	if len(warning.args) == 0 {
		return goStringLiteral(warning.format)
	}

	return "fmt.Sprintf(" + warning.call("") + ")"
}

// emitWritePreview writes the dry-run branch, which validates and builds the
// body the live path would have sent and then reports rather than sends it.
func emitWritePreview(out *source, tool *contract) error {
	ordered, err := tool.orderedPathFields()
	if err != nil {
		return err
	}

	path, err := tool.pathExpression(out)
	if err != nil {
		return err
	}

	// The reported route carries the query the live branch sends, so a preview
	// of a paged replacement names the page the call would ask for rather than
	// the collection's first one.
	if len(tool.Query) > 0 {
		path = fmt.Sprintf("tools.PathWithQuery(%s, query)", path)
	}

	out.writef("// %s answers %s's dry run: the same validation and the same body,", previewName(tool.Name), tool.Name)
	out.writef("// reported rather than sent.")
	out.writef("func %s(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {",
		previewName(tool.Name))

	if err := emitWriteChecks(out, tool, ordered, withoutEchoes); err != nil {
		return err
	}

	reported := previewBody(tool)

	if tool.declaresPreview() {
		emitDeclaredPreview(out, tool, ordered, goStringLiteral(tool.Method), path, reported)
		out.writef("}")
		out.writef("")

		return nil
	}

	out.writef("\treturn tools.RunDryRunPreviewWithBody(ctx, request, cfg, %s, %s, %s, %s, nil)",
		goStringLiteral(tool.Name), goStringLiteral(tool.Method), path, reported)
	out.writef("}")
	out.writef("")

	return nil
}

// emitWriteChecks writes the argument checks and the body build both branches
// share. Sharing them is what keeps a preview from advertising a body the live
// call would reject, or the other way round.
func emitWriteChecks(out *source, tool *contract, ordered []field, echoes bool) error {
	emitNormalize(out, tool)
	emitConstraintCheck(out, tool)

	for _, entry := range ordered {
		if err := emitRequiredPathArg(out, tool, &entry); err != nil {
			return err
		}
	}

	// Body presence runs after the path checks and before the body is built,
	// which is the order the hand-written checks these replaced answered in. The
	// open-object
	// walks read what is inside an argument whose own shape just passed, and the
	// cross-field any-of question comes last: each argument answers for itself
	// before the call answers for naming nothing.
	if err := emitBodyPresenceChecks(out, tool); err != nil {
		return err
	}

	emitObjectWalks(out, tool)
	emitAnyOfCheck(out, tool)

	if echoes {
		if err := emitEchoReads(out, tool, ordered); err != nil {
			return err
		}
	}

	emitBodyBuild(out, tool, echoes)

	// Built in both branches so a preview reports the call the live path makes:
	// the page controls the two firewall replacements publish change which
	// assignments come back, and a preview omitting them would describe a
	// different request.
	if len(tool.Query) == 0 {
		return nil
	}

	return emitQueryBuild(out, tool)
}

// previewBody is the body expression a dry run reports: the built body, or a
// copy of it with each declared member stood in for. Only the report changes;
// the live branch keeps sending what the caller supplied.
func previewBody(tool *contract) string {
	names := make([]string, 0, len(tool.Body))

	for _, entry := range tool.Body {
		if entry.Redact {
			names = append(names, goStringLiteral(entry.wireName()))
		}
	}

	chain := make([]string, 0, len(tool.PreviewStandIns)+2)
	chain = append(chain, "body")

	if len(names) > 0 {
		chain = append(chain, fmt.Sprintf(".Redacting(%s)", strings.Join(names, ", ")))
	}

	for _, entry := range tool.PreviewStandIns {
		chain = append(chain, fmt.Sprintf(".StandingIn(%s, %s, %s)",
			goStringLiteral(entry.Argument), goStringLiteral(entry.Member), goStringLiteral(entry.Text)))
	}

	return strings.Join(chain, "")
}

// bodyName is the builder a tool's two branches share.
func bodyName(tool string) string {
	return goLocalName(tool) + "Body"
}

// previewName is the dry-run branch a tool's handler delegates to.
func previewName(tool string) string {
	return "preview" + exportedToolName(tool)
}
