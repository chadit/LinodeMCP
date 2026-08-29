package main

import (
	"fmt"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// emitMeta writes a tool that reaches no Linode route: the version the binary
// reports, the audit stores on disk, the profile builder's registry.
//
// It shares the opening every other tier has (the cancellation check, then the
// contract's own rules) and diverges where the others open a client: there is
// nothing to call, so the answer is either the declared local operation or the
// sentence the contract declares.
func emitMeta(out *source, tool *contract) error {
	if err := checkMetaShape(tool); err != nil {
		return err
	}

	out.need(importContext, importMCP, importConfig, importProfiles,
		importTools, importToolschemas)

	emitToolFactory(out, tool)

	return emitMetaHandler(out, tool)
}

// checkMetaShape refuses the meta contracts that would emit a handler saying
// less than the contract does.
//
// A preview needs a call to describe and a destroy's state read needs a
// resource to read, so both name a step this tier has none of. The two answers
// are exclusive because the handler returns whichever it writes last, which
// would drop the other without either language reporting it.
func checkMetaShape(tool *contract) error {
	if tool.DryRun {
		return fmt.Errorf("%w: %s", errMetaDryRun, tool.Name)
	}

	if tool.Confirm && tool.ConfirmMessage == "" {
		return fmt.Errorf("%w: %s", errNoConfirmMessage, tool.Name)
	}

	// A local answer beside a success_message is refused a step earlier, by the
	// local-answer check, so reaching here with one means the answer is settled.
	if tool.answersLocally() {
		return nil
	}

	if tool.SuccessMessage == "" {
		return fmt.Errorf("%w: %s", errNoMetaAnswer, tool.Name)
	}

	if tool.MessageField == "" {
		return fmt.Errorf("%w: %s answers with %s",
			errNoMetaMessageField, tool.Name, tool.ResponseGo.FullName)
	}

	return nil
}

// emitMetaHandler writes the body behind a meta factory.
//
// The configuration is taken as a blank where nothing reads it: a sentence
// assembled from the call needs nothing configured, and the signature stays the
// same one every generated handler has so the factory closure is unchanged.
func emitMetaHandler(out *source, tool *contract) error {
	config := goConfigLocal
	if !metaReadsConfig(tool) {
		config = "_"
	}

	out.writef("func %s(ctx context.Context, request *mcp.CallToolRequest, %s *config.Config) (*mcp.CallToolResult, error) {",
		handlerName(tool.Name), config)
	out.writef("\tselect {")
	out.writef("\tcase <-ctx.Done():")
	out.writef("\t\treturn nil, ctx.Err()")
	out.writef("\tdefault:")
	out.writef("\t}")
	out.writef("")

	emitNormalize(out, tool)
	emitConstraintCheck(out, tool)

	// Asked after the rules for the reason a mutation asks it before them: a
	// gated meta tool changes local state rather than a resource, and its
	// arguments name a draft the caller already holds, so refusing an unusable
	// one first says nothing a caller did not already know.
	if tool.Confirm {
		out.writef("\tif result := tools.RequireConfirm(request, %s); result != nil {",
			goStringLiteral(tool.ConfirmMessage))
		out.writef("\t\treturn result, nil")
		out.writef("\t}")
		out.writef("")
	}

	if tool.answersLocally() {
		emitLocalAnswer(out, tool)

		return nil
	}

	return emitMetaSentence(out, tool)
}

// metaReadsConfig is whether anything in the body reads the configuration. A
// declared sentence is assembled from the call and needs nothing configured,
// and a local operation takes it only where the operation reads it.
func metaReadsConfig(tool *contract) bool {
	return tool.answersLocally() &&
		tool.LocalAnswer.Arm.reads(linodev1.LocalAmbient_LOCAL_AMBIENT_CONFIG)
}

// emitMetaSentence writes the answer of a meta tool that declares no local
// operation: the declared sentence, over the arguments it names.
func emitMetaSentence(out *source, tool *contract) error {
	answer, err := tool.formatMessage(tool.SuccessMessage)
	if err != nil {
		return err
	}

	out.need(importFmt, importGenpb)
	out.writef("\treturn tools.MarshalProtoToolResponse(&linodev1.%s{", tool.ResponseGo.TypeName)
	out.writef("\t\t%s: fmt.Sprintf(%s),", tool.MessageField, answer.call(""))
	out.writef("\t})")
	out.writef("}")
	out.writef("")

	return nil
}
