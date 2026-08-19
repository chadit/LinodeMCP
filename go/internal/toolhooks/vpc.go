package toolhooks

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// LinodeVPCUpdatePreview reads the VPC so the label a change starts from can be
// named; the request carries only the new values.
func LinodeVPCUpdatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	vpcID := request.GetInt("vpc_id", 0)
	newLabel := request.GetString("label", "")
	newDescription := request.GetString("description", "")

	return statePreview(ctx, request, cfg, "linode_vpc_update", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return fetchedState(client.GetVPC(ctx, vpcID))
		},
		func(state any) tools.DryRunDetails {
			return vpcUpdateEffects(state, newLabel, newDescription)
		})
}

// vpcUpdateEffects names the label change and, when a new description travels,
// that the description is replaced. The description text itself stays out of
// the sentence because it is free-form prose a caller already sent.
func vpcUpdateEffects(state any, newLabel, newDescription string) tools.DryRunDetails {
	var (
		details   tools.DryRunDetails
		fromLabel string
	)

	if vpc, isVPC := state.(*linode.VPC); isVPC && vpc != nil {
		fromLabel = vpc.Label
	}

	tools.LabelChangeSideEffect(&details, fromLabel, newLabel)

	if newDescription != "" {
		details.SideEffects = append(details.SideEffects, "The VPC description is updated.")
	}

	return details
}
