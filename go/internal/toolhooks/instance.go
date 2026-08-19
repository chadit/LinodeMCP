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

// instanceStatusRunning is the status a Linode reports while it is up, which is
// what turns a reboot into downtime worth warning about.
const instanceStatusRunning = "running"

// LinodeInstanceFirewallUpdatePreview reads the assignments the replacement
// overwrites, on the page the call itself would ask for. Go read them and said
// nothing; Python said the sentence and read nothing.
func LinodeInstanceFirewallUpdatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)

	return statePreview(ctx, request, cfg, "linode_instance_firewall_update", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			page, pageSize := requestedPage(request)

			return tools.ProtoStateList(client.ListInstanceFirewallsProto(ctx, linodeID, page, pageSize))
		},
		func(_ any) tools.DryRunDetails {
			return tools.DryRunDetails{
				SideEffects: []string{fmt.Sprintf("Firewall assignments for Linode %d will be replaced.", linodeID)},
			}
		})
}

// LinodeInstanceRescuePreview reads the instance the reboot would interrupt.
// The walk takes the state rather than the id because a running Linode goes
// down for the rescue boot and one that is already off does not.
func LinodeInstanceRescuePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)

	return statePreview(ctx, request, cfg, "linode_instance_rescue", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return fetchedState(client.GetInstance(ctx, linodeID))
		},
		func(state any) tools.DryRunDetails {
			details := tools.DryRunDetails{SideEffects: []string{
				"The instance reboots into rescue mode; its normal boot configuration is bypassed until you reboot out of rescue mode.",
			}}

			if instance, ok := state.(*linode.Instance); ok && instance != nil &&
				strings.EqualFold(instance.Status, instanceStatusRunning) {
				details.Warnings = append(details.Warnings,
					"Instance is currently running; entering rescue mode reboots it, causing downtime.")
			}

			return details
		})
}

// LinodeInstanceMigratePreview reads the Linode the migration would move, so the
// region it leaves is reported beside the one it is headed for.
func LinodeInstanceMigratePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)
	targetRegion := request.GetString("region", "")

	return statePreview(ctx, request, cfg, "linode_instance_migrate", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return fetchedState(client.GetInstance(ctx, linodeID))
		},
		func(state any) tools.DryRunDetails {
			return tools.DryRunDetails{
				SideEffects: []string{instanceMigrateEffect(state, targetRegion)},
			}
		})
}

// instanceMigrateEffect words the move. An omitted region is Linode picking the
// destination, and a read that answered with nothing leaves the origin out
// rather than naming an empty one.
func instanceMigrateEffect(state any, targetRegion string) string {
	var fromRegion string

	if instance, isInstance := state.(*linode.Instance); isInstance && instance != nil {
		fromRegion = instance.Region
	}

	if targetRegion == "" {
		return "Instance migrates; it is unavailable during the migration."
	}

	if fromRegion == "" {
		return fmt.Sprintf("Instance migrates to region %s; it is unavailable during the migration.", targetRegion)
	}

	return fmt.Sprintf(
		"Instance migrates from region %s to %s; it is unavailable during the migration.",
		fromRegion, targetRegion,
	)
}

// LinodeInstanceMutatePreview reads the Linode whose type the upgrade replaces.
// Go's preview named the type and Python's named the downtime, so this one
// carries both.
func LinodeInstanceMutatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)

	return statePreview(ctx, request, cfg, "linode_instance_mutate", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return fetchedState(client.GetInstance(ctx, linodeID))
		},
		func(state any) tools.DryRunDetails {
			return tools.DryRunDetails{
				SideEffects: []string{instanceMutateEffect(state)},
				Warnings:    []string{"The Linode may be unavailable during the upgrade."},
			}
		})
}

// instanceMutateEffect words the upgrade, naming the type it starts from when
// the read answered with one.
func instanceMutateEffect(state any) string {
	var fromType string

	if instance, isInstance := state.(*linode.Instance); isInstance && instance != nil {
		fromType = instance.Type
	}

	if fromType == "" {
		return "Instance upgrades to the latest generation of its type; it reboots during the upgrade."
	}

	return fmt.Sprintf(
		"Instance type %s upgrades to the latest generation; it reboots during the upgrade.", fromType,
	)
}

// LinodeInstanceDiskClonePreview reads the disk being copied, since its size is
// the storage the clone consumes.
func LinodeInstanceDiskClonePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)
	diskID := request.GetInt("disk_id", 0)

	return statePreview(ctx, request, cfg, "linode_instance_disk_clone", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return fetchedState(client.GetInstanceDisk(ctx, linodeID, diskID))
		},
		instanceDiskCloneEffects)
}

// instanceDiskCloneEffects names the copy a clone leaves behind and the storage
// it takes, read off the disk fetched beside it.
func instanceDiskCloneEffects(state any) tools.DryRunDetails {
	var (
		label string
		size  int
	)

	if disk, isDisk := state.(*linode.InstanceDisk); isDisk && disk != nil {
		label = disk.Label
		size = disk.Size
	}

	return tools.DryRunDetails{SideEffects: []string{fmt.Sprintf(
		"Disk %q (%d MB) is cloned to a new disk on the same instance, consuming %d MB of additional storage.",
		label, size, size,
	)}}
}

// LinodeInstanceResizePreview reads the Linode whose plan the resize replaces,
// so the type it leaves is reported beside the one it is headed for.
func LinodeInstanceResizePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	instanceID := request.GetInt("instance_id", 0)
	targetType := request.GetString("type", "")

	return statePreview(ctx, request, cfg, "linode_instance_resize", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return fetchedState(client.GetInstance(ctx, instanceID))
		},
		func(state any) tools.DryRunDetails {
			return instanceResizeDetails(resizeStateType(state), targetType)
		})
}

// LinodeInstanceResizeFetchState builds the projection a resize plan hashes:
// the instance's current type plus each disk's id, size, and filesystem.
//
// It is not the instance the preview reports, because a resize moves the disks
// too and allow_auto_disk_resize resizes them as part of the plan change. The
// projection holds only what a real change would move, so a cosmetic instance
// field cannot refuse an apply and the tool needs no hash-ignore list.
func LinodeInstanceResizeFetchState(ctx context.Context, client *linode.Client, instanceID int) (any, error) {
	instance, err := client.GetInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrResizeInstanceRead, err)
	}

	disks, err := client.ListInstanceDisks(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrResizeDiskList, err)
	}

	snapshot := make([]instanceResizeDiskInfo, len(disks))
	for i := range disks {
		snapshot[i] = instanceResizeDiskInfo{
			ID:         disks[i].ID,
			Size:       disks[i].Size,
			Filesystem: disks[i].Filesystem,
		}
	}

	return &instanceResizeState{Type: instance.Type, Disks: snapshot}, nil
}

// LinodeInstanceResizeDependencyWalk names the plan change over the state the
// fetch already read, so a plan reads like the preview beside it.
func LinodeInstanceResizeDependencyWalk(
	ctx context.Context, _ *linode.Client, request *mcp.CallToolRequest, state any,
) (tools.DryRunDetails, error) {
	targetType := request.GetString("type", "")

	return previewDetails(ctx, "linode_instance_resize", func() tools.DryRunDetails {
		return instanceResizeDetails(resizeStateType(state), targetType)
	})
}

// instanceResizeState is the drift-relevant projection a resize plan hashes.
type instanceResizeState struct {
	Type  string                   `json:"type"`
	Disks []instanceResizeDiskInfo `json:"disks"`
}

// instanceResizeDiskInfo is one disk's drift-relevant fields.
type instanceResizeDiskInfo struct {
	Filesystem string `json:"filesystem"`
	ID         int    `json:"id"`
	Size       int    `json:"size"`
}

// resizeStateType reads the from-type out of whichever state shape reaches it:
// the composite projection on the plan path, the bare instance on the preview.
func resizeStateType(state any) string {
	switch typed := state.(type) {
	case *instanceResizeState:
		if typed != nil {
			return typed.Type
		}
	case *linode.Instance:
		if typed != nil {
			return typed.Type
		}
	}

	return ""
}

// instanceResizeDetails words the type change and the two costs it carries: the
// reboot the move needs, and the price the new plan bills at.
func instanceResizeDetails(fromType, targetType string) tools.DryRunDetails {
	effect := fmt.Sprintf(
		"Instance resizes to type %s; it reboots and is unavailable during the resize.", targetType,
	)
	if fromType != "" {
		effect = fmt.Sprintf(
			"Instance resizes from type %s to %s; it reboots and is unavailable during the resize.",
			fromType, targetType,
		)
	}

	return tools.DryRunDetails{
		SideEffects: []string{effect},
		Warnings:    []string{"Resizing changes the monthly price to match the new type."},
	}
}

// LinodeInstanceCreatePreview describes the instance the call would create and
// warns that billing starts with it. Nothing exists to read yet, so the
// sentences and the request echo are the whole preview.
func LinodeInstanceCreatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	result, err := tools.RunDryRunPreviewWithBodyDetailed(
		ctx, request, cfg, "linode_instance_create", method, path, body, nil,
		func(ctx context.Context, _ *linode.Client, _ any) (tools.DryRunDetails, error) {
			return previewDetails(ctx, "linode_instance_create", func() tools.DryRunDetails {
				return instanceCreateDetails(request)
			})
		},
	)

	return wrapPreview("linode_instance_create", result, err)
}

// instanceCreateDetails words the instance being made, naming the image only
// when the caller chose one, and the billing the create starts.
func instanceCreateDetails(request *mcp.CallToolRequest) tools.DryRunDetails {
	effect := fmt.Sprintf("A new %s instance will be created in region %s",
		request.GetString("type", ""), request.GetString("region", ""))

	if image := request.GetString("image", ""); image != "" {
		effect += " from image " + image
	}

	return tools.DryRunDetails{
		SideEffects: []string{effect + "."},
		Warnings:    []string{"Billing for the instance starts immediately on creation."},
	}
}
