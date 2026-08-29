package main

import (
	"fmt"
	"strings"
)

// checkTwoStage holds the plan/apply arguments to a tier that runs the flow.
//
// The two arguments are the whole declaration: a tool advertising them promises
// a caller can plan a change and apply the plan, and until this tier emitted the
// flow that promise was schema-deep. A LOCAL field the emitter does not place is
// dropped in silence, so mode and plan_id reached a generated handler as two
// arguments nothing read.
func (c *contract) checkTwoStage() error {
	if c.Mode != c.PlanID {
		return fmt.Errorf("%w: %s", errPartialTwoStage, c.Name)
	}

	// The destroy tier's driver runs the flow for every tool on it, whether or
	// not the schema advertises the arguments, so nothing here applies to it.
	if c.Tier == tierDestroy {
		return nil
	}

	if !c.Mode {
		return c.checkUnstagedWalks()
	}

	if c.Tier != tierAcknowledge {
		return fmt.Errorf("%w: %s", errNoTwoStageDriver, c.Name)
	}

	if c.CompositeDecl == nil {
		return fmt.Errorf("%w: %s", errNoStateRead, c.Name)
	}

	return nil
}

// checkUnstagedWalks refuses the mirror of the hole above: a dependency walk on
// a tool that advertises no plan is a declaration resolved, written into the
// generated file, and never reached.
func (c *contract) checkUnstagedWalks() error {
	// The deliberately-unknown estimate is the one of these an unstaged tool
	// can act: it reads no state and rides the preview rather than a walk.
	if len(c.WalkDecls) == 0 && (c.BillingDecl == nil || billingUnpriced(c.BillingDecl)) {
		return nil
	}

	return fmt.Errorf("%w: %s", errUnstagedWalk, c.Name)
}

// emitTwoStage writes the plan/apply branch a staged mutation's handler tries
// before anything else.
//
// It is a function of its own rather than lines inside the handler because the
// two paths answer different questions: the branch reports whether it handled
// the call, and everything the handler emits reports what the call answered.
// Returning the result alone lets the shared argument checks, which every other
// tier emits with the handler's own `return message, nil` tail, stand here
// unchanged: a refusal is a result, and only the fall-through is nil.
//
// The checks run here and again below rather than once ahead of both, because
// the order a live call reads them in is this tier's contract: confirm gates
// before anything says whether the arguments would have been accepted. A plan
// has no confirm to gate on, so it validates first, and the requested-gate above
// the checks is what keeps an ordinary call out of them.
func emitTwoStage(out *source, tool *contract) error {
	ordered, err := tool.orderedPathFields()
	if err != nil {
		return err
	}

	path, err := tool.pathExpression(out)
	if err != nil {
		return err
	}

	success, err := tool.formatMessage(tool.SuccessMessage)
	if err != nil {
		return err
	}

	out.need(importLinode, importProto, importTwostage)

	name := twoStageName(tool.Name)

	out.writef("// %s answers %s's plan and apply modes, and nil for the call", name, tool.Name)
	out.writef("// that asked for neither.")
	out.writef("func %s(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {",
		name)
	out.writef("\tif !tools.TwoStageRequested(ctx, request) {")
	out.writef("\t\treturn nil, nil")
	out.writef("\t}")
	out.writef("")

	if err := emitAcknowledgeChecks(out, tool, ordered, withEchoes); err != nil {
		return err
	}

	emitTwoStageAction(out, tool, ordered, path, success)

	out.writef("\tif !handled {")
	out.writef("\t\treturn nil, nil")
	out.writef("\t}")
	out.writef("")
	out.writef("\treturn staged, nil")
	out.writef("}")
	out.writef("")

	return nil
}

// emitTwoStageAction writes the action the shared flow runs: where the call
// goes, what it sends, what a plan hashes, and the answer an apply reports.
func emitTwoStageAction(out *source, tool *contract, ordered []field, path string, success formatted) {
	values := pathValuesLiteral(ordered)

	out.writef("\tstaged, handled := tools.RunTwoStageWrite(ctx, request, cfg, &tools.DestructiveAction{")
	out.writef("\t\tToolName:   %s,", goStringLiteral(tool.Name))
	out.writef("\t\tCapability: profiles.%s,", capabilityConstant(tool.Capability))
	out.writef("\t\tMethod:     %s,", goStringLiteral(tool.Method))
	out.writef("\t\tPath:       %s,", path)
	out.writef("\t\tFetchState: func(ctx context.Context, client *linode.Client) (any, error) {")
	emitTwoStageFetchBody(out, tool, ordered)
	out.writef("\t\t},")

	call, sent := twoStageCall(tool)

	out.writef("\t\tExecute: func(ctx context.Context, client *linode.Client) error {")
	out.writef("\t\t\treturn client.%s(ctx, %s, %s%s)", call, goStringLiteral(tool.Name), values, sent)
	out.writef("\t\t},")
	out.writef("\t\tSuccess: func() proto.Message {")
	out.writef("\t\t\treturn &linodev1.%s{", tool.ResponseGo.TypeName)
	out.writef("\t\t\t\t%s: fmt.Sprintf(%s),", tool.MessageField, success.call(""))

	for index := range tool.Echoes {
		echo := &tool.Echoes[index]
		// readAcknowledgeEnvelope has already held every echo to a kind
		// echoValue renders, so the error it returns cannot reach here.
		value, _ := echoValue(tool.Name, echo)
		out.writef("\t\t\t\t%s: %s,", echo.GoName, value)
	}

	out.writef("\t\t\t}")
	out.writef("\t\t},")

	emitTwoStageWalk(out, tool, ordered)

	out.writef("\t\tHashIgnore: twostage.HashIgnoreFields(%s),", goStringLiteral(tool.ResourceType))
	out.writef("\t})")
	out.writef("")
}

// twoStageCall names the routed primitive an apply sends its change through and
// the body it hands over. A tool with no body field sends none, the same way its
// live path does: an empty JSON object is a body, and adding one would change
// the call.
func twoStageCall(tool *contract) (string, string) {
	if len(tool.Body) == 0 {
		return "CallRoute", ""
	}

	return "CallRouteBody", ", body"
}

// twoStageName is the branch a staged tool's handler tries first.
func twoStageName(tool string) string {
	return "twoStage" + exportedToolName(tool)
}

// emitCompositeFetch writes the declared multi-call state read: each call in
// declaration order, keeping the declared field subset, assembled under the
// declared members.
func emitCompositeFetch(out *source, tool *contract, ordered []field) {
	out.need(importProto)
	out.writef("\t\t\treturn tools.FetchCompositeState(ctx, client, []tools.CompositeCall{")

	for index := range tool.Composite {
		call := &tool.Composite[index]
		element := "linodev1." + call.Message.TypeName

		values := make([]string, 0, len(call.Slots))
		for _, slot := range call.Slots {
			values = append(values, matchLocal(slot, ordered))
		}

		out.writef("\t\t\t\t{")
		out.writef("\t\t\t\t\tTool:   %s,", goStringLiteral(call.Tool))
		out.writef("\t\t\t\t\tMember: %s,", goStringLiteral(call.Member))
		out.writef("\t\t\t\t\tFields: []string{%s},", goNameList(call.Fields))
		out.writef("\t\t\t\t\tValues: []any{%s},", strings.Join(values, ", "))
		out.writef("\t\t\t\t\tList:   %t,", call.List)
		out.writef("\t\t\t\t\tNew:    func() proto.Message { return &%s{} },", element)
		out.writef("\t\t\t\t},")
	}

	out.writef("\t\t\t})")
}

// emitTwoStageFetchBody writes the staged fetch's body: the declared composite
// the tool carries.
func emitTwoStageFetchBody(out *source, tool *contract, ordered []field) {
	emitCompositeFetch(out, tool, ordered)
}

// emitTwoStageWalk writes the staged walk, which reads the state the declared
// composite produced.
func emitTwoStageWalk(out *source, tool *contract, ordered []field) {
	// A tool with no walks still reports its declared prose on the plan, which
	// is what the seeded lines are: the same sentences the preview reports,
	// worded against the state the plan read.
	if len(tool.DepWalks) > 0 || len(tool.PreviewSentences) > 0 || tool.BillingDecl != nil {
		emitDeclaredWalks(out, tool, "", ordered)
	}
}
