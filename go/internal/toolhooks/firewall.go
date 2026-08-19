package toolhooks

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// Cloud Firewall routes answer "must be a positive integer" for an id that is
// present but unusable, which a derived required check cannot say. The device
// route words its own id differently again, so both sentences are kept here.

// requestedPage reads the page controls a replacement's preview fetches its
// state under, so the assignments it reports come off the same page the call
// itself would ask for. The generated handler has already refused an
// out-of-range control by the time a preview hook runs.
func requestedPage(request *mcp.CallToolRequest) (int, int) {
	return request.GetInt("page", 0), request.GetInt("page_size", 0)
}

// LinodeFirewallCreatePreview names the firewall the call would create and the
// policies it would start with. The firewall does not exist yet, so the
// sentence and the request echo are the whole preview.
func LinodeFirewallCreatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	result, err := tools.RunDryRunPreviewWithBodyDetailed(
		ctx, request, cfg, "linode_firewall_create", method, path, body, nil,
		func(ctx context.Context, _ *linode.Client, _ any) (tools.DryRunDetails, error) {
			return previewDetails(ctx, "linode_firewall_create", func() tools.DryRunDetails {
				return tools.DryRunDetails{SideEffects: []string{firewallCreateSentence(request)}}
			})
		},
	)

	return wrapPreview("linode_firewall_create", result, err)
}

// firewallCreateSentence words the new firewall and the two policies it starts
// under, which fall back to the same defaults the body folds in.
func firewallCreateSentence(request *mcp.CallToolRequest) string {
	return fmt.Sprintf("A new Cloud Firewall %q will be created with inbound policy %s and outbound policy %s.",
		request.GetString("label", ""),
		request.GetString("inbound_policy", defaultFirewallPolicy),
		request.GetString("outbound_policy", defaultFirewallPolicy))
}

// defaultFirewallPolicy is the policy a firewall carries when the caller names
// none, which is the value the create body folds into its rules object.
const defaultFirewallPolicy = "ACCEPT"

// LinodeFirewallUpdatePreview reads the firewall the update changes and states
// the label and status changes against it.
func LinodeFirewallUpdatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	id := request.GetInt("firewall_id", 0)

	return statePreview(ctx, request, cfg, "linode_firewall_update", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return client.GetFirewall(ctx, id)
		},
		func(state any) tools.DryRunDetails {
			return firewallUpdateDetails(state,
				request.GetString("label", ""), request.GetString("status", ""))
		})
}

// firewallUpdateDetails reports the label change and a status change against
// the firewall as it stands.
func firewallUpdateDetails(state any, newLabel, newStatus string) tools.DryRunDetails {
	var details tools.DryRunDetails

	var fromLabel, fromStatus string

	if firewall, ok := state.(*linode.Firewall); ok && firewall != nil {
		fromLabel = firewall.Label
		fromStatus = firewall.Status
	}

	tools.LabelChangeSideEffect(&details, fromLabel, newLabel)

	if newStatus != "" && newStatus != fromStatus {
		details.SideEffects = append(details.SideEffects, fmt.Sprintf(
			"Firewall status changes to %q; this immediately %s its rules.",
			newStatus, firewallStatusVerb(newStatus),
		))
	}

	return details
}

// firewallStatusVerb maps a target status to how it affects rule enforcement.
func firewallStatusVerb(status string) string {
	if status == "disabled" {
		return "stops enforcing"
	}

	return "starts enforcing"
}
