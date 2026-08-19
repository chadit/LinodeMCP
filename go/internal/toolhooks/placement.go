package toolhooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	placementGroupTypeParam   = "placement_group_type"
	placementGroupPolicyParam = "placement_group_policy"
)

// LinodePlacementGroupCreateNormalize trims every text argument the create
// sends, which is what decides the bytes on the wire: Python trims the label
// alone, so a padded region reaches Linode padded there and trimmed here.
func LinodePlacementGroupCreateNormalize(request *mcp.CallToolRequest) {
	arguments := request.GetArguments()

	for _, name := range []string{"label", "region", placementGroupTypeParam, placementGroupPolicyParam} {
		if value, supplied := arguments[name].(string); supplied {
			arguments[name] = strings.TrimSpace(value)
		}
	}
}

// LinodePlacementGroupUnassignPreview reads the group so the preview reports it
// as current_state, and names each Linode the call takes out of it.
func LinodePlacementGroupUnassignPreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	groupID := request.GetInt("group_id", 0)
	linodes, _ := tools.ParsePlacementGroupLinodes(request)

	return statePreview(ctx, request, cfg, "linode_placement_group_unassign", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return fetchedState(client.GetPlacementGroup(ctx, groupID))
		},
		func(_ any) tools.DryRunDetails {
			return placementGroupMembershipSideEffects(linodes, groupID, "removed from")
		})
}

// LinodePlacementGroupAssignPreview reads the group so the preview reports it as
// current_state, and names each Linode the call puts into it.
func LinodePlacementGroupAssignPreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	groupID := request.GetInt("group_id", 0)
	linodes, _ := tools.ParsePlacementGroupLinodes(request)

	return statePreview(ctx, request, cfg, "linode_placement_group_assign", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return fetchedState(client.GetPlacementGroup(ctx, groupID))
		},
		func(_ any) tools.DryRunDetails {
			return placementGroupMembershipSideEffects(linodes, groupID, "assigned to")
		})
}

// placementGroupMembershipSideEffects names each Linode whose membership
// changes, which the request carries only as a list of ids. The verb is what
// separates the two routes: one adds the Linodes, the other takes them out.
func placementGroupMembershipSideEffects(linodes []int, groupID int, verb string) tools.DryRunDetails {
	var details tools.DryRunDetails

	for _, linodeID := range linodes {
		details.SideEffects = append(details.SideEffects,
			fmt.Sprintf("Linode %d will be %s placement group %d.", linodeID, verb, groupID))
	}

	return details
}
