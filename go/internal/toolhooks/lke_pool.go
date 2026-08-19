package toolhooks

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// argPoolCount is the node count both the presence check and the sentence read.
const argPoolCount = "count"

// LinodeLkePoolUpdatePreview reads the node pool so a caller sees the count the
// resize starts from, which is the one number the arguments cannot supply.
func LinodeLkePoolUpdatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	clusterID := request.GetInt("cluster_id", 0)
	poolID := request.GetInt("pool_id", 0)

	arguments := request.GetArguments()
	_, countSupplied := arguments[argPoolCount]
	_, autoscalerSupplied := arguments["autoscaler"]
	newCount := request.GetInt(argPoolCount, 0)

	return statePreview(ctx, request, cfg, "linode_lke_pool_update", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return client.GetLKENodePool(ctx, clusterID, poolID)
		},
		func(state any) tools.DryRunDetails {
			return lkePoolUpdateEffects(state, newCount, countSupplied, autoscalerSupplied)
		})
}

// lkePoolUpdateEffects names the count change and the autoscaler change a pool
// update performs.
//
// A fetched count of zero is left out of the resize sentence: the read that
// failed and the pool that really holds no nodes are the same value here, so
// the sentence that names a starting count is reserved for a count that was
// read.
func lkePoolUpdateEffects(
	state any, newCount int, countSupplied, autoscalerSupplied bool,
) tools.DryRunDetails {
	var details tools.DryRunDetails

	if countSupplied {
		var fromCount int
		if pool, ok := state.(*linode.LKENodePool); ok && pool != nil {
			fromCount = pool.Count
		}

		effect := fmt.Sprintf("Node pool is set to %d node(s).", newCount)
		if fromCount != 0 && fromCount != newCount {
			effect = fmt.Sprintf("Node pool resizes from %d to %d node(s).", fromCount, newCount)
		}

		details.SideEffects = append(details.SideEffects, effect)
	}

	if autoscalerSupplied {
		details.SideEffects = append(details.SideEffects,
			"The pool autoscaler configuration is updated.")
	}

	return details
}
