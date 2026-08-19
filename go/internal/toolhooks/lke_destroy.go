package toolhooks

import (
	"context"
	"fmt"

	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// LinodeLkeClusterDeleteDependencyWalk names what a cluster delete takes with
// it: every node pool cascades, so each is reported one by one and the nodes
// they carry are counted in a warning, since the running workloads are the part
// a caller cannot put back.
//
// A failed pool list is a warning rather than an error: the delete is still
// previewable without the pool picture, and refusing the preview would leave a
// caller with nothing to decide on.
func LinodeLkeClusterDeleteDependencyWalk(
	ctx context.Context, client *linode.Client, clusterID int, _ tools.DeclaredState,
) (tools.DryRunDetails, error) {
	var details tools.DryRunDetails

	pools, err := client.ListLKENodePools(ctx, clusterID)
	if err != nil {
		details.Warnings = append(details.Warnings, fmt.Sprintf("Could not list node pools: %v", err))

		return details, nil
	}

	var totalNodes int

	for i := range pools {
		pool := &pools[i]
		totalNodes += pool.Count

		details.Dependencies = append(details.Dependencies, tools.DryRunDependency{
			Kind:   "node_pool",
			ID:     pool.ID,
			Action: tools.DependencyActionCascadeDeleted,
			Note:   fmt.Sprintf("%d node(s) of type %s", pool.Count, pool.Type),
		})
	}

	if totalNodes > 0 {
		details.Warnings = append(details.Warnings, fmt.Sprintf(
			"Deleting this cluster destroys %d node pool(s) and %d node(s); running workloads are lost.",
			len(pools), totalNodes,
		))
	}

	return details, nil
}
