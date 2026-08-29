package main

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// emitAcknowledge writes a mutation the API answers with nothing to decode: a
// detach, a revoke, an acknowledgement. The change is reported by status alone,
// so the tool's answer is assembled from the call the way a destroy's is.
//
// It is the write tier in every other respect, gate order included: a dry run
// validates before anything else because there is nothing to confirm, while a
// live call gates on confirm BEFORE it validates, so a caller who has not
// confirmed learns nothing about whether their arguments would have been
// accepted.
func emitAcknowledge(out *source, tool *contract) error {
	if tool.ErrorMessage == "" {
		return fmt.Errorf("%w: %s", errNoErrorMessage, tool.Name)
	}

	if tool.SuccessMessage == "" {
		return fmt.Errorf("%w: %s", errNoSuccessMessage, tool.Name)
	}

	if tool.ConfirmMessage == "" {
		return fmt.Errorf("%w: %s", errNoConfirmMessage, tool.Name)
	}

	out.need(importContext, importFmt, importMCP, importConfig, importGenpb,
		importProfiles, importTools, importToolschemas)

	emitToolFactory(out, tool)

	if len(tool.Body) > 0 {
		if err := emitWriteBody(out, tool); err != nil {
			return err
		}
	}

	if tool.Mode {
		if err := emitTwoStage(out, tool); err != nil {
			return err
		}
	}

	if err := emitAcknowledgeHandler(out, tool); err != nil {
		return err
	}

	return emitAcknowledgePreview(out, tool)
}

// emitAcknowledgeHandler writes the live path: gate, validate, send, report.
func emitAcknowledgeHandler(out *source, tool *contract) error {
	ordered, err := tool.orderedPathFields()
	if err != nil {
		return err
	}

	failure, err := tool.formatMessage(tool.ErrorMessage)
	if err != nil {
		return err
	}

	success, err := tool.formatMessage(tool.SuccessMessage)
	if err != nil {
		return err
	}

	out.writef("func %s(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {",
		handlerName(tool.Name))

	// Ahead of the preview and the gate because a plan is neither: it reports
	// the change without making it and takes no confirm, so a call asking for
	// one that fell to either branch would be answered as something else.
	if tool.Mode {
		out.writef("\tstaged, stagedErr := %s(ctx, request, cfg)", twoStageName(tool.Name))
		out.writef("\tif staged != nil || stagedErr != nil {")
		out.writef("\t\treturn staged, stagedErr")
		out.writef("\t}")
		out.writef("")
	}

	out.writef("\tif tools.IsDryRun(request) {")
	out.writef("\t\treturn %s(ctx, request, cfg)", previewName(tool.Name))
	out.writef("\t}")
	out.writef("")
	out.writef("\tif result := tools.RequireConfirm(request, %s); result != nil {",
		goStringLiteral(tool.ConfirmMessage))
	out.writef("\t\treturn result, nil")
	out.writef("\t}")
	out.writef("")

	if err := emitAcknowledgeChecks(out, tool, ordered, withEchoes); err != nil {
		return err
	}

	out.writef("\tclient, err := tools.ClientForRequest(request, cfg)")
	out.writef("\tif err != nil {")
	out.writef("\t\treturn mcp.NewToolResultError(err.Error()), nil")
	out.writef("\t}")
	out.writef("")
	emitAcknowledgeCall(out, tool, ordered)
	out.writef("\t\treturn mcp.NewToolResultError(fmt.Sprintf(%s)), nil", failure.call("err"))
	out.writef("\t}")
	out.writef("")

	return emitAssembledAnswer(out, tool, success)
}

// emitAssembledAnswer writes the tail shared by every tier whose answer is put
// together rather than decoded: the declared sentence, the warning, the
// arguments the call echoes back, and the members a transport filled.
//
// One copy because the acknowledge tier and the body-read tier answer the same
// shape for the same reason. They differ in how the call is made, not in what
// the caller reads, so a second copy would drift on the half nobody is looking
// at.
func emitAssembledAnswer(out *source, tool *contract, success formatted) error {
	out.writef("\treturn tools.MarshalProtoToolResponse(&linodev1.%s{", tool.ResponseGo.TypeName)
	out.writef("\t\t%s: fmt.Sprintf(%s),", tool.MessageField, success.call(""))

	if warnErr := emitWarning(out, tool); warnErr != nil {
		return warnErr
	}

	for index := range tool.Echoes {
		echo := &tool.Echoes[index]

		value, err := echoValue(tool.Name, echo)
		if err != nil {
			return err
		}

		out.writef("\t\t%s: %s,", echo.GoName, value)
	}

	if tool.transported() {
		if err := emitTransportMembers(out, tool); err != nil {
			return err
		}
	}

	out.writef("\t})")
	out.writef("}")
	out.writef("")

	return nil
}

// emitAcknowledgeCall writes the request line. A tool declaring no body field
// sends none, which is what both hand-written clients put on the wire for these
// routes: an empty JSON object is a body, and adding one would change the call.
//
// A declared transport stands in for the whole line: the route it addresses
// takes something other than a JSON body, and the arguments are the ones the
// derived call would have been given, so the two are read side by side.
func emitAcknowledgeCall(out *source, tool *contract, ordered []field) {
	values := pathValuesLiteral(ordered)

	if tool.transported() {
		emitTransportCall(out, tool, values)

		return
	}

	if len(tool.Body) == 0 {
		out.writef("\tif err := client.CallRoute(ctx, %s, %s); err != nil {",
			goStringLiteral(tool.Name), values)

		return
	}

	out.writef("\tif err := client.CallRouteBody(ctx, %s, %s, body); err != nil {",
		goStringLiteral(tool.Name), values)
}

// executeBody is the body a declared transport is handed: the one the shared
// builder assembled, or nothing when the tool declares no body field. The
// transport is what turns it into the request, so it reads the built body
// rather than the arguments and the two branches stay one build apart.
func executeBody(tool *contract) string {
	if len(tool.Body) == 0 {
		return nilLiteral
	}

	return "body"
}

// withEchoes and withoutEchoes select whether the shared checks also read the
// arguments the live answer echoes back. A preview reports the call rather than
// answering with the resource, so reading them there would leave the value
// unused and the generated file would not compile.
const (
	withEchoes    = true
	withoutEchoes = false
)

// emitAcknowledgeChecks writes the argument checks, the body build when there is
// one, and the reads for arguments the answer echoes back.
func emitAcknowledgeChecks(out *source, tool *contract, ordered []field, echoes bool) error {
	emitNormalize(out, tool)
	emitConstraintCheck(out, tool)

	for _, entry := range ordered {
		if err := emitRequiredPathArg(out, tool, &entry); err != nil {
			return err
		}
	}

	// Body presence runs after the path checks and before the body is built,
	// the same order the write tier emits it in: python's acknowledge renderer
	// shares one validation seam, so a member emitted only there would leave
	// the two languages a check apart. require_any_of is not emitted here
	// because the contract refuses it off the write tier for every language.
	if err := emitBodyPresenceChecks(out, tool); err != nil {
		return err
	}

	if echoes {
		if err := emitEchoReads(out, tool, ordered); err != nil {
			return err
		}
	}

	if len(tool.Body) == 0 {
		return nil
	}

	emitBodyBuild(out, tool, echoes)

	return nil
}

// emitEchoReads writes the read for every echoed argument the path reads above
// have not already put in scope, which is how a body argument the answer carries
// back reaches the response.
func emitEchoReads(out *source, tool *contract, ordered []field) error {
	inScope := make(map[string]struct{}, len(ordered))
	for _, entry := range ordered {
		inScope[entry.ProtoName] = struct{}{}
	}

	var written bool

	for index := range tool.Echoes {
		echo := &tool.Echoes[index]

		// A nested echo names no one argument, so there is no local to read:
		// each of its members reads the request inside the literal carrying it.
		if echo.Nested != nil {
			continue
		}

		if _, held := inScope[echo.Arg.ProtoName]; held {
			continue
		}

		if err := emitEchoRead(out, tool.Name, echo); err != nil {
			return err
		}

		inScope[echo.Arg.ProtoName] = struct{}{}
		written = true
	}

	if written {
		out.writef("")
	}

	return nil
}

// emitEchoRead writes the read one echoed argument reaches the response through.
func emitEchoRead(out *source, tool string, echo *destroyEcho) error {
	if echo.Items != nil {
		emitEchoItems(out, echo)

		return nil
	}

	if echo.Arg.Repeated {
		reader, ok := echoListReader(echo.Arg.Kind)
		if !ok {
			return fmt.Errorf("%w: %s echoes %s, which repeats %s",
				errUnsupportedPathKind, tool, echo.Arg.ProtoName, echo.Arg.Kind)
		}

		out.writef("\t%s := tools.%s(request, %s)",
			echo.Arg.GoLocal, reader, goStringLiteral(echo.Arg.ProtoName))

		return nil
	}

	reader, zero, ok := echoArgReader(echo.Arg.Kind)
	if !ok {
		return fmt.Errorf("%w: %s echoes %s, which is %s",
			errUnsupportedPathKind, tool, echo.Arg.ProtoName, echo.Arg.Kind)
	}

	out.writef("\t%s := request.%s(%s, %s)",
		echo.Arg.GoLocal, reader, goStringLiteral(echo.Arg.ProtoName), zero)

	return nil
}

// echoArgReader names the request accessor one echoed argument is read through
// and the value that means the caller omitted it.
//
// It is the path reader widened by the flag, because an echo and a route segment
// answer different questions: a segment addresses a resource, so Linode carries
// only an id or a label there, while an echo reports what the caller asked for
// and a flag is as much of that as text is. Both hand handlers read an absent
// cors_enabled as false rather than leaving it out of the answer, so the flag's
// omitted value is the same false the accessor already defaults to.
func echoArgReader(kind protoreflect.Kind) (string, string, bool) {
	if kind == protoreflect.BoolKind {
		return "GetBool", falseText, true
	}

	return pathArgReader(kind)
}

// echoListReader names the call a repeated scalar echo is read through.
func echoListReader(kind protoreflect.Kind) (string, bool) {
	if kind == protoreflect.StringKind {
		return "EchoStrings", true
	}

	if kind == protoreflect.Int32Kind || kind == protoreflect.Int64Kind {
		return "EchoInt32s", true
	}

	return "", false
}

// emitEchoItems writes the slice a repeated named-message echo is rebuilt into,
// above the response literal that carries it. The response declares an item
// message of its own, so each item the caller sent is mapped member by member
// rather than handed over whole.
func emitEchoItems(out *source, echo *destroyEcho) {
	items := echo.Arg.GoLocal + "Items"

	out.writef("\t%s := tools.EchoItems(request, %s)", items, goStringLiteral(echo.Arg.ProtoName))
	out.writef("\t%s := make([]*linodev1.%s, 0, len(%s))", echo.Arg.GoLocal, echo.Items.TypeName, items)
	out.writef("")
	out.writef("\tfor _, item := range %s {", items)
	out.writef("\t\t%s = append(%s, &linodev1.%s{", echo.Arg.GoLocal, echo.Arg.GoLocal, echo.Items.TypeName)

	for _, member := range echo.Items.Members {
		out.writef("\t\t\t%s: item.%s(%s),",
			member.GoName, echoMemberReader(member.Kind), goStringLiteral(member.Argument))
	}

	out.writef("\t\t})")
	out.writef("\t}")
}

// echoMemberReader names the accessor one response item member is filled from.
// readEchoItems has already refused any kind with no accessor, so the mapping
// reaches here holding only the two a list item carries.
func echoMemberReader(kind protoreflect.Kind) string {
	if kind == protoreflect.StringKind {
		return "Text"
	}

	return "Number"
}

// echoValue is the expression one echoed argument reaches its response field
// through. An id is widened the way every other id echo is; text travels as it
// arrived.
func echoValue(tool string, echo *destroyEcho) (string, error) {
	if echo.Nested != nil {
		return nestedEchoLiteral(echo.Nested), nil
	}

	// A list is built above the literal, so its local already holds the type
	// the response declares.
	if echo.Items != nil || echo.Arg.Repeated {
		return echo.Arg.GoLocal, nil
	}

	switch echo.Arg.Kind {
	case protoreflect.Int32Kind, protoreflect.Int64Kind:
		return "tools.IDToInt32(" + echo.Arg.GoLocal + ")", nil
	case protoreflect.StringKind, protoreflect.BoolKind:
		return echo.Arg.GoLocal, nil
	case protoreflect.EnumKind, protoreflect.FloatKind,
		protoreflect.DoubleKind, protoreflect.BytesKind, protoreflect.MessageKind,
		protoreflect.GroupKind, protoreflect.Uint32Kind, protoreflect.Uint64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind, protoreflect.Fixed32Kind,
		protoreflect.Fixed64Kind, protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
	}

	return "", fmt.Errorf("%w: %s echoes %s, which is %s",
		errUnsupportedPathKind, tool, echo.Arg.ProtoName, echo.Arg.Kind)
}

// nestedEchoLiteral renders the assembled member as the composite literal the
// response carries it under. Each member reads the request where it stands
// rather than through a local, which is what keeps the preview branch, running
// the same checks with no echo to fill, from declaring values nothing uses.
func nestedEchoLiteral(nested *echoItems) string {
	parts := make([]string, 0, len(nested.Members))

	for _, member := range nested.Members {
		reader, zero, _ := echoArgReader(member.Kind)
		parts = append(parts, fmt.Sprintf("%s: request.%s(%s, %s)",
			member.GoName, reader, goStringLiteral(member.Argument), zero))
	}

	return fmt.Sprintf("&linodev1.%s{%s}", nested.TypeName, strings.Join(parts, ", "))
}

// emitAcknowledgePreview writes the dry-run branch, which runs the same checks
// and reports the call rather than making it.
func emitAcknowledgePreview(out *source, tool *contract) error {
	ordered, err := tool.orderedPathFields()
	if err != nil {
		return err
	}

	path, err := tool.pathExpression(out)
	if err != nil {
		return err
	}

	out.writef("// %s answers %s's dry run: the same checks, and the call reported", previewName(tool.Name), tool.Name)
	out.writef("// rather than made.")
	out.writef("func %s(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {",
		previewName(tool.Name))

	if err := emitAcknowledgeChecks(out, tool, ordered, withoutEchoes); err != nil {
		return err
	}

	// A declared redaction is honored here for the reason the write tier
	// honors it: the tier a tool lands on is decided by the shape of its
	// answer, and a secret in the request does not stop being one because the
	// API answers with nothing.
	body := nilLiteral
	if len(tool.Body) > 0 {
		body = previewBody(tool)
	}

	if tool.declaresPreview() {
		emitDeclaredPreview(out, tool, ordered, goStringLiteral(tool.Method), path, body)
		out.writef("}")
		out.writef("")

		return nil
	}

	out.writef("\treturn tools.RunDryRunPreviewWithBody(ctx, request, cfg, %s, %s, %s, %s, nil)",
		goStringLiteral(tool.Name), goStringLiteral(tool.Method), path, body)
	out.writef("}")
	out.writef("")

	return nil
}
