package main

import (
	"fmt"
)

// The Go arm of the declared transports: each arm renders one call, and the
// members it fills are copied into the tool's own answer. Mirrors the Python
// arm in transport_py.go, which renders the same declaration as a spec the
// driver runs.

// transferLocal names the local a transported handler holds its transfer's
// result in. One name for every arm that answers with anything, since what it
// carries is the transfer's own measurement rather than a decoded message.
const transferLocal = "transferred"

// emitTransportCall writes the declared transport's call and opens the failure
// branch its caller closes. An arm answering nothing tests the error inline, the
// way the routed call it stands in for does.
func emitTransportCall(out *source, tool *contract, values string) {
	out.need(importTools)

	switch tool.Transport.Kind {
	case transportMultipart:
		out.writef("\tif err := client.CallRouteMultipart(ctx, %s, %s, %s, %s); err != nil {",
			goStringLiteral(tool.Name), values,
			goStringLiteral(tool.Transport.PartName),
			stringArgumentRead(tool.Transport.FileArgument))
	case transportRawBody:
		emitRawBodyCall(out, tool, values)
	case transportPresign:
		emitPresignCall(out, tool, values)
	case transportNone:
	}
}

// emitRawBodyCall writes a bare-bytes transfer: the argument decoded and sent
// going up, the answer read and held for the response coming down.
func emitRawBodyCall(out *source, tool *contract, values string) {
	spec := tool.Transport

	if spec.Up {
		out.writef("\tif err := client.CallRouteRawBody(ctx, %s, %s, %s, tools.Base64Argument(request, %s)); err != nil {",
			goStringLiteral(tool.Name), values,
			goStringLiteral(spec.ContentType),
			goStringLiteral(spec.SourceArgument))

		return
	}

	out.writef("\t%s, err := client.CallRouteRawBodyRead(ctx, %s, %s, %s)",
		transferLocal, goStringLiteral(tool.Name), values, goStringLiteral(spec.ContentType))
	out.writef("\tif err != nil {")
}

// emitPresignCall writes the minted-URL transfer as one spec literal the shared
// engine runs, the pattern the declared walks render through.
func emitPresignCall(out *source, tool *contract, values string) {
	spec := tool.Transport

	out.writef("\t%s, err := tools.RunPresignTransfer(ctx, client, &tools.PresignSpec{", transferLocal)
	out.writef("\t\tTool: %s,", goStringLiteral(tool.Name))
	out.writef("\t\tSubject: %s,", goStringLiteral(responseSubject(tool.Name)))

	if spec.Up {
		out.writef("\t\tUp: true,")
	}

	out.writef("\t\tURLField: %s,", goStringLiteral(spec.URLField))
	out.writef("\t\tLocalPath: %s,", stringArgumentRead(spec.LocalPathArgument))

	out.writef("\t\t%s", presignGuardArgument(spec))
	out.writef("\t\tPathValues: %s,", values)
	out.writef("\t\tBody: %s,", executeBody(tool))
	out.writef("\t})")
	out.writef("\tif err != nil {")
}

// presignGuardArgument is the one argument a direction's own guard reads: the
// media type an upload signs under, the permission a download needs for an
// occupied destination.
func presignGuardArgument(spec *transportSpec) string {
	if spec.Up {
		return "ContentType: " + stringArgumentRead(spec.ContentTypeArgument) + ","
	}

	return "Overwrite: request.GetBool(" + goStringLiteral(spec.OverwriteArgument) + ", false),"
}

// stringArgumentRead is the read a transport makes for one of its arguments,
// which is the request's own reader rather than a local: every name a transport
// carries is one the schema advertises and the rules have already checked.
func stringArgumentRead(argument string) string {
	return fmt.Sprintf("request.GetString(%s, \"\")", goStringLiteral(argument))
}

// emitTransportMembers copies what the transfer measured into the answer, in
// the response's own field order. Only the declared members are written, and
// the contract has already held that list to exactly what the arm fills.
func emitTransportMembers(out *source, tool *contract) error {
	for _, member := range tool.Assembled {
		value, err := transportMemberValue(tool, member.ProtoName)
		if err != nil {
			return err
		}

		out.writef("\t\t%s: %s,", member.GoName, value)
	}

	return nil
}

// transportMemberValue answers the expression filling one response member.
func transportMemberValue(tool *contract, protoName string) (string, error) {
	spec := tool.Transport

	switch protoName {
	case spec.AnswerField:
		return "tools.Base64Text(" + transferLocal + ")", nil
	case spec.SizeField:
		return transferLocal + ".SizeBytes", nil
	case spec.ETagField:
		return transferLocal + ".ETag", nil
	}

	for _, constant := range spec.Constants {
		if constant.GetField() == protoName {
			return goStringLiteral(constant.GetText()), nil
		}
	}

	return "", fmt.Errorf("%w: %s member %s", errTransportMembers, tool.Name, protoName)
}
