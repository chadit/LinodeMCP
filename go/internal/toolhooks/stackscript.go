package toolhooks

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// LinodeStackscriptUpdatePreview reads the StackScript as it stands and reports
// what the update changes about it.
func LinodeStackscriptUpdatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	stackscriptID := request.GetInt("stackscript_id", 0)
	label := request.GetString("label", "")
	script := request.GetString("script", "")
	description := request.GetString("description", "")

	return statePreview(ctx, request, cfg, "linode_stackscript_update", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return client.GetStackScript(ctx, stackscriptID)
		},
		func(state any) tools.DryRunDetails {
			return stackscriptUpdateSideEffects(state, label, script, description)
		})
}

// stackscriptUpdateSideEffects is the Tier B walk for a StackScript update,
// diffed against the fetched state so the preview says what changes rather than
// what was asked for.
func stackscriptUpdateSideEffects(state any, newLabel, newScript, newDescription string) tools.DryRunDetails {
	var (
		details   tools.DryRunDetails
		fromLabel string
	)

	// A fetch that answered with something else leaves the label empty, which
	// reads as "no previous value" rather than as no change at all.
	if script, isScript := state.(*linode.StackScript); isScript && script != nil {
		fromLabel = script.Label
	}

	tools.LabelChangeSideEffect(&details, fromLabel, newLabel)

	if newScript != "" {
		details.SideEffects = append(details.SideEffects, "The StackScript body is replaced.")
	}

	if newDescription != "" {
		details.SideEffects = append(details.SideEffects, "The StackScript description is updated.")
	}

	return details
}
