package toolhooks

import (
	"context"
	"fmt"

	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// maxVLANPageSize is the largest page the VLAN list endpoint serves. A VLAN
// paged out past it reads as not found, which is what the hand handler did.
const maxVLANPageSize = 500

// LinodeVPCSubnetDeleteDependencyWalk names the Linodes whose interfaces sit in
// the subnet: each is detached rather than deleted. The warning names the VPC
// so a caller can tell which network the subnet belonged to. Best-effort, so a
// failed VPC read leaves the label empty rather than refusing the preview.
func LinodeVPCSubnetDeleteDependencyWalk(
	ctx context.Context, client *linode.Client, vpcID, _ int, state tools.DeclaredState,
) (tools.DryRunDetails, error) {
	var details tools.DryRunDetails

	attached := state.Objects("linodes")

	for _, linodeRef := range attached {
		id, _ := linodeRef.Number("id")
		details.Dependencies = append(details.Dependencies, tools.DryRunDependency{
			Kind:   dependencyKindInstance,
			ID:     id,
			Action: tools.DependencyActionDetached,
			Note: fmt.Sprintf("%d interface(s) in this subnet are detached.",
				len(linodeRef.Objects("interfaces"))),
		})
	}

	if len(attached) == 0 {
		return details, nil
	}

	var vpcLabel string
	if vpc, err := client.GetVPC(ctx, vpcID); err == nil && vpc != nil {
		vpcLabel = vpc.Label
	}

	details.Warnings = append(details.Warnings, fmt.Sprintf(
		"%d Linode(s) have interfaces in subnet %q (VPC %q) and will be detached.",
		len(attached), state.Text("label"), vpcLabel,
	))

	return details, nil
}

// LinodeVlanDeleteFetchState resolves one VLAN by region and label. VLANs
// expose no single-resource GET, only a paginated list, so a VLAN paged out
// past the first page reads as tools.ErrVLANNotFound.
func LinodeVlanDeleteFetchState(
	ctx context.Context, client *linode.Client, regionID, label string,
) (any, error) {
	vlans, err := client.ListVLANs(ctx, 1, maxVLANPageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to list VLANs: %w", err)
	}

	for i := range vlans.Data {
		vlan := vlans.Data[i]
		if vlan.Region == regionID && vlan.Label == label {
			return vlan, nil
		}
	}

	return nil, fmt.Errorf("%w: %s in region %s", tools.ErrVLANNotFound, label, regionID)
}
