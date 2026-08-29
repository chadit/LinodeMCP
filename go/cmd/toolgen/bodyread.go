package main

import "fmt"

// emitBodyRead writes a tool that reads through a route taking a request body:
// the checks and the body build a mutation makes, and then the read's own
// answer, with none of the machinery a mutation carries around it.
//
// No gate and no preview, because nothing is changed by the call: the
// presigned-URL create signs what its body describes and stores nothing, so
// asking a caller to confirm a read would be a gate over an answer they can ask
// for again.
func emitBodyRead(out *source, tool *contract) error {
	if tool.ErrorMessage == "" {
		return fmt.Errorf("%w: %s", errNoErrorMessage, tool.Name)
	}

	if err := checkBodyReadShape(tool); err != nil {
		return err
	}

	if tool.Confirm || tool.DryRun {
		return fmt.Errorf("%w: %s", errGatedBodyRead, tool.Name)
	}

	out.need(importContext, importFmt, importMCP, importConfig, importGenpb,
		importProfiles, importTools, importToolschemas)

	// The free-form body decodes into the well-known message rather than a
	// generated one, which is the only type the handler names from elsewhere.
	if tool.PayloadStruct {
		out.need(importStructpb)
	}

	emitToolFactory(out, tool)

	if err := emitWriteBody(out, tool); err != nil {
		return err
	}

	return emitBodyReadHandler(out, tool)
}

// checkBodyReadShape holds this tier to the two answers it writes code for: the
// resource the call decodes into, and the envelope carrying that resource beside
// the prose and the ids the read echoes.
//
// Everything else reaches the handler with a member the answer would leave
// empty. A page and a decoded envelope are the write tier's tails, and prose
// with nothing decoded under it is an answer that reports no reading at all.
func checkBodyReadShape(tool *contract) error {
	if tool.BareResource {
		return nil
	}

	// The assembled answer is the third shape: a transport makes the call, so
	// there is no decoded resource to carry and the members it fills stand in for
	// the one it would have decoded. Held to the same prose requirement as the
	// decoded envelope below, because both report their result in a sentence.
	if tool.transported() && len(tool.Assembled) > 0 {
		if tool.PayloadField != "" {
			return fmt.Errorf("%w: %s answers with %s, which a transport fills and a decode also claims",
				errNotABodyRead, tool.Name, tool.ResponseGo.FullName)
		}

		if tool.MessageField == "" || tool.SuccessMessage == "" {
			return fmt.Errorf("%w: %s", errNoSuccessMessage, tool.Name)
		}

		return nil
	}

	if tool.PagedWrite || tool.DecodedResponse || tool.MessageField == "" || tool.PayloadField == "" {
		return fmt.Errorf("%w: %s answers with %s",
			errNotABodyRead, tool.Name, tool.ResponseGo.FullName)
	}

	if tool.SuccessMessage == "" {
		return fmt.Errorf("%w: %s", errNoSuccessMessage, tool.Name)
	}

	return nil
}

// emitBodyReadHandler writes the body behind the factory: check, build, send,
// answer with what came back.
func emitBodyReadHandler(out *source, tool *contract) error {
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

	// Every check answers where it is read, the way an ungated read answers its
	// own arguments: there is no confirm gate here for a verdict to wait behind.
	if err := emitWriteChecks(out, tool, ordered, withEchoes); err != nil {
		return err
	}

	out.writef("\tclient, err := tools.ClientForRequest(request, cfg)")
	out.writef("\tif err != nil {")
	out.writef("\t\treturn mcp.NewToolResultError(err.Error()), nil")
	out.writef("\t}")
	out.writef("")

	if tool.transported() && len(tool.Assembled) > 0 {
		return emitBodyReadAssembled(out, tool, ordered, failure)
	}

	if tool.BareResource {
		return emitBodyReadResource(out, tool, ordered, failure)
	}

	return emitBodyReadEnvelope(out, tool, ordered, failure)
}

// emitBodyReadAssembled writes the tail of a read whose call a transport makes.
// The route answers with something no decode can place: the Object Storage
// download asks for a presigned URL and then follows it, so what the caller
// reads is the transfer's own result rather than any member of the API's reply.
//
// The transport stands in for the routed call the same way it does on the
// acknowledge tier, and the answer is assembled through the shared tail.
func emitBodyReadAssembled(
	out *source, tool *contract, ordered []field, failure formatted,
) error {
	success, err := tool.formatMessage(tool.SuccessMessage)
	if err != nil {
		return err
	}

	emitTransportCall(out, tool, pathValuesLiteral(ordered))
	out.writef("\t\treturn mcp.NewToolResultError(fmt.Sprintf(%s)), nil", failure.call("err"))
	out.writef("\t}")
	out.writef("")

	return emitAssembledAnswer(out, tool, success)
}

// emitBodyReadResource writes the tail of a read whose declared answer is the
// resource the call decodes into.
func emitBodyReadResource(out *source, tool *contract, ordered []field, failure formatted) error {
	out.writef("\tresponse := &linodev1.%s{}", tool.PayloadGo.TypeName)
	out.writef("")
	emitBodyReadCall(out, tool, ordered, "response")
	out.writef("\t\treturn mcp.NewToolResultError(fmt.Sprintf(%s)), nil", failure.call("err"))
	out.writef("\t}")
	out.writef("")
	out.writef("\treturn tools.MarshalProtoToolResponse(response)")
	out.writef("}")
	out.writef("")

	return nil
}

// emitBodyReadEnvelope writes the tail of a read whose answer wraps what it
// decoded: the metric query reports the window it was asked for beside the
// samples, so the decode fills the member and the rest is assembled around it.
func emitBodyReadEnvelope(out *source, tool *contract, ordered []field, failure formatted) error {
	out.writef("\tpayload := &%s{}", tool.decodeType())
	out.writef("")
	emitBodyReadCall(out, tool, ordered, "payload")
	out.writef("\t\treturn mcp.NewToolResultError(fmt.Sprintf(%s)), nil", failure.call("err"))
	out.writef("\t}")
	out.writef("")

	return emitWriteResult(out, tool)
}

// emitBodyReadCall writes the routed call, opening the failure branch its caller
// closes. The subject is the one every routed decode carries: a body that parsed
// as something other than an object reaches protojson as an empty message, so
// without it a read reports success over an answer it never got.
func emitBodyReadCall(out *source, tool *contract, ordered []field, target string) {
	out.writef("\tif err := client.CallProtoRouteBody(ctx, %s, %s, body, %s, %s); err != nil {",
		goStringLiteral(tool.Name), pathValuesLiteral(ordered),
		goStringLiteral(responseSubject(tool.Name)), target)
}
