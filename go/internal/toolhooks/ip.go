package toolhooks

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// What is left here is the billing a reservation starts, which no descriptor
// carries: the address does not exist yet, so there is nothing to read and
// nothing a rule could derive the estimate from.

// reservedIPBillingUnknown stands in for a monthly change the preview cannot
// estimate, which reserved IPv4 pricing cannot be without a region price.
const reservedIPBillingUnknown = "unknown"

// LinodeNetworkingReservedIPCreatePreview reports the reservation a create would
// make. It reads no state, since the address does not exist yet, and carries the
// billing a reservation starts.
func LinodeNetworkingReservedIPCreatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	return statePreview(ctx, request, cfg, "linode_networking_reserved_ip_create", method, path, body, nil,
		func(_ any) tools.DryRunDetails {
			return tools.DryRunDetails{
				BillingDelta: &tools.DryRunBillingDelta{
					MonthlyChangeUSD: reservedIPBillingUnknown,
					Note:             "Reserved IPv4 pricing varies by region; see linode_networking_reserved_ip_type_list.",
				},
				Warnings: []string{"Reserved IP billing begins when the address is created."},
			}
		})
}
