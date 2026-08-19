package toolhooks

import (
	"context"
	"fmt"

	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// LinodeFirewallDeleteDependencyWalk lists the devices the firewall guards, so
// a caller sees what stops being protected. Best-effort: a failed device list
// becomes a warning rather than failing the preview.
func LinodeFirewallDeleteDependencyWalk(
	ctx context.Context, client *linode.Client, firewallID int, _ any,
) (tools.DryRunDetails, error) {
	var details tools.DryRunDetails

	devices, err := client.ListFirewallDevices(ctx, firewallID, 1, tools.DependencyWalkPageSize)
	if err != nil {
		details.Warnings = append(details.Warnings, fmt.Sprintf("Could not list firewall devices: %v", err))

		return details, nil
	}

	for i := range devices.Data {
		entity := &devices.Data[i].Entity
		details.Dependencies = append(details.Dependencies, tools.DryRunDependency{
			Kind:   entity.Type,
			ID:     entity.ID,
			Label:  entity.Label,
			Action: tools.DependencyActionRemoved,
			Note:   "Loses this firewall's rules when the firewall is deleted.",
		})
	}

	if len(devices.Data) > 0 {
		details.Warnings = append(details.Warnings, fmt.Sprintf(
			"%d resource(s) currently use this firewall and will lose its rules.", len(devices.Data),
		))
	}

	return details, nil
}

// LinodeVPCDeleteDependencyWalk lists the subnets destroyed with the VPC and
// counts the Linode interfaces they detach, which is the cascade a caller
// cannot see from the request. Best-effort: a failed subnet list becomes a
// warning.
func LinodeVPCDeleteDependencyWalk(
	ctx context.Context, client *linode.Client, vpcID int, _ tools.DeclaredState,
) (tools.DryRunDetails, error) {
	var details tools.DryRunDetails

	subnets, err := client.ListVPCSubnets(ctx, vpcID)
	if err != nil {
		details.Warnings = append(details.Warnings, fmt.Sprintf("Could not list VPC subnets: %v", err))

		return details, nil
	}

	var attachedInterfaces int

	for i := range subnets {
		subnet := &subnets[i]
		attachedInterfaces += len(subnet.Linodes)
		details.Dependencies = append(details.Dependencies, tools.DryRunDependency{
			Kind:   "vpc_subnet",
			ID:     subnet.ID,
			Label:  subnet.Label,
			Action: tools.DependencyActionCascadeDeleted,
			Note:   fmt.Sprintf("%d attached Linode interface(s)", len(subnet.Linodes)),
		})
	}

	if attachedInterfaces > 0 {
		details.Warnings = append(details.Warnings, fmt.Sprintf(
			"%d Linode interface(s) across %d subnet(s) will be detached.", attachedInterfaces, len(subnets),
		))
	}

	return details, nil
}

// LinodeNodebalancerDeleteDependencyWalk lists the configs destroyed with the
// NodeBalancer, each carrying its own backend node list. Best-effort: a failed
// config list becomes a warning.
func LinodeNodebalancerDeleteDependencyWalk(
	ctx context.Context, client *linode.Client, nodeBalancerID int, _ any,
) (tools.DryRunDetails, error) {
	var details tools.DryRunDetails

	configs, err := client.ListNodeBalancerConfigs(ctx, nodeBalancerID, 0, 0)
	if err != nil {
		details.Warnings = append(details.Warnings, fmt.Sprintf("Could not list NodeBalancer configs: %v", err))

		return details, nil
	}

	for i := range configs {
		config := &configs[i]
		details.Dependencies = append(details.Dependencies, tools.DryRunDependency{
			Kind:   "nodebalancer_config",
			ID:     config.ID,
			Action: tools.DependencyActionCascadeDeleted,
			Note:   fmt.Sprintf("%s config on port %d", config.Protocol, config.Port),
		})
	}

	if len(configs) > 0 {
		details.Warnings = append(details.Warnings, fmt.Sprintf(
			"Deleting this NodeBalancer destroys %d config(s) and their backend node lists.", len(configs),
		))
	}

	return details, nil
}
