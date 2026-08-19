package toolhooks

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// LinodeInstanceDiskResizePreview reads the disk so the size it grows or
// shrinks from is reported beside the size it is headed for.
func LinodeInstanceDiskResizePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)
	diskID := request.GetInt("disk_id", 0)
	size := request.GetInt("size", 0)

	return statePreview(ctx, request, cfg, "linode_instance_disk_resize", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return fetchedState(client.GetInstanceDisk(ctx, linodeID, diskID))
		},
		func(state any) tools.DryRunDetails {
			return tools.DryRunDetails{
				SideEffects: []string{instanceDiskResizeEffect(state, size)},
				Warnings:    []string{"The instance must be powered off to resize a disk."},
			}
		})
}

// instanceDiskResizeEffect words the change. A read that answered with nothing
// leaves the starting size out rather than naming a zero.
func instanceDiskResizeEffect(state any, targetSize int) string {
	var fromSize int

	if disk, isDisk := state.(*linode.InstanceDisk); isDisk && disk != nil {
		fromSize = disk.Size
	}

	if fromSize == 0 {
		return fmt.Sprintf("Disk resizes to %d MB.", targetSize)
	}

	return fmt.Sprintf("Disk resizes from %d MB to %d MB.", fromSize, targetSize)
}
