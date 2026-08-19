package toolhooks

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// Every LKE route under a cluster answers "cluster_id must be a positive
// integer" when the id is present but unusable, which a derived required check
// cannot say, so each of these hooks keeps the sentence its hand-written
// handler answered.

// lkeCluster reads the cluster id every LKE sub-route is addressed by.
func lkeCluster(request *mcp.CallToolRequest) (int, string) {
	return tools.RequiredIDArgument(request, "cluster_id")
}

// LinodeLkeClusterUpdatePreview reads the cluster so the label and Kubernetes
// version a change starts from can be named; the request carries only the new
// values.
func LinodeLkeClusterUpdatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	clusterID, _ := lkeCluster(request)
	newLabel := request.GetString("label", "")
	newK8sVersion := request.GetString("k8s_version", "")

	return statePreview(ctx, request, cfg, "linode_lke_cluster_update", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return client.GetLKECluster(ctx, clusterID)
		},
		func(state any) tools.DryRunDetails {
			return lkeClusterUpdateEffects(state, newLabel, newK8sVersion)
		})
}

// lkeClusterUpdateEffects names the label change and the Kubernetes version
// change, which upgrades the control plane and every node.
func lkeClusterUpdateEffects(state any, newLabel, newK8sVersion string) tools.DryRunDetails {
	var (
		details     tools.DryRunDetails
		fromLabel   string
		fromVersion string
	)

	if cluster, ok := state.(*linode.LKECluster); ok && cluster != nil {
		fromLabel = cluster.Label
		fromVersion = cluster.K8sVersion
	}

	tools.LabelChangeSideEffect(&details, fromLabel, newLabel)

	if newK8sVersion != "" && newK8sVersion != fromVersion {
		details.SideEffects = append(details.SideEffects, fmt.Sprintf(
			"Kubernetes version changes from %q to %q; the control plane and nodes upgrade.",
			fromVersion, newK8sVersion,
		))
	}

	return details
}
