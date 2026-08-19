package toolhooks

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// Every NodeBalancer route answers "must be a positive integer" for an id that
// is present but unusable, which a derived required check cannot say.

// LinodeNodebalancerFirewallUpdatePreview reads the assignments the replacement
// overwrites. Go read them off the collection's first page whatever the caller
// asked for, and Python read the NodeBalancer instead of its firewalls.
func LinodeNodebalancerFirewallUpdatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	balancerID := request.GetInt("nodebalancer_id", 0)

	return statePreview(ctx, request, cfg, "linode_nodebalancer_firewall_update", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			page, pageSize := requestedPage(request)

			return tools.ProtoStateList(client.ListNodeBalancerFirewallsProto(ctx, balancerID, page, pageSize))
		},
		func(_ any) tools.DryRunDetails { return tools.DryRunDetails{} })
}

// LinodeNodebalancerConfigCreatePreview reads the NodeBalancer's existing
// configs, so a caller sees which ports are already taken before adding one.
// Go carried the fetch and Python carried none; the fetch is the union.
func LinodeNodebalancerConfigCreatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	nodeBalancerID := request.GetInt("nodebalancer_id", 0)

	return statePreview(ctx, request, cfg, "linode_nodebalancer_config_create", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			configs, err := client.ListNodeBalancerConfigs(ctx, nodeBalancerID, 0, 0)

			// An empty page reaches Python as [] and Go as a nil slice, which
			// serialize differently; the two previews have to read alike.
			if configs == nil {
				configs = []linode.NodeBalancerConfig{}
			}

			return fetchedState(configs, err)
		},
		func(_ any) tools.DryRunDetails {
			return tools.DryRunDetails{}
		})
}

// LinodeNodebalancerUpdatePreview reads the NodeBalancer so the label a change
// starts from can be named; the request carries only the new values.
func LinodeNodebalancerUpdatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	nodeBalancerID := request.GetInt("nodebalancer_id", 0)
	arguments := request.GetArguments()
	newLabel, _ := arguments["label"].(string)
	_, throttleSupplied := arguments["client_conn_throttle"]
	newThrottle := request.GetInt("client_conn_throttle", 0)

	return statePreview(ctx, request, cfg, "linode_nodebalancer_update", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return fetchedState(client.GetNodeBalancer(ctx, nodeBalancerID))
		},
		func(state any) tools.DryRunDetails {
			return nodeBalancerUpdateEffects(state, newLabel, newThrottle, throttleSupplied)
		})
}

// nodeBalancerUpdateEffects names the label change and the throttle the update
// sets, read against the NodeBalancer as it stands.
func nodeBalancerUpdateEffects(state any, newLabel string, newThrottle int, throttleSupplied bool) tools.DryRunDetails {
	var (
		details   tools.DryRunDetails
		fromLabel string
	)

	if balancer, ok := state.(*linode.NodeBalancer); ok && balancer != nil {
		fromLabel = balancer.Label
	}

	tools.LabelChangeSideEffect(&details, fromLabel, newLabel)

	if throttleSupplied {
		details.SideEffects = append(details.SideEffects, fmt.Sprintf(
			"Connection throttle is set to %d connections per second per client IP.", newThrottle,
		))
	}

	return details
}

// LinodeNodebalancerConfigDeleteDependencyWalk names the backend nodes the
// delete takes out of rotation, since the config owns them and they go with it.
// Best-effort, so a failed node list becomes a warning rather than refusing the
// preview.
func LinodeNodebalancerConfigDeleteDependencyWalk(
	ctx context.Context, client *linode.Client, nodeBalancerID, configID int, _ any,
) (tools.DryRunDetails, error) {
	var details tools.DryRunDetails

	nodes, err := client.ListNodeBalancerConfigNodes(ctx, nodeBalancerID, configID, 1, tools.DependencyWalkPageSize)
	if err != nil {
		details.Warnings = append(details.Warnings, fmt.Sprintf("Could not list config backend nodes: %v", err))

		return details, nil
	}

	for i := range nodes.Data {
		node := &nodes.Data[i]
		details.Dependencies = append(details.Dependencies, tools.DryRunDependency{
			Kind:   "nodebalancer_node",
			ID:     node.ID,
			Label:  node.Label,
			Action: tools.DependencyActionCascadeDeleted,
			Note:   fmt.Sprintf("backend %s (%s)", node.Address, node.Mode),
		})
	}

	if len(nodes.Data) > 0 {
		details.Warnings = append(details.Warnings, fmt.Sprintf(
			"Deleting this config removes %d backend node(s) from the rotation.", len(nodes.Data),
		))
	}

	return details, nil
}
