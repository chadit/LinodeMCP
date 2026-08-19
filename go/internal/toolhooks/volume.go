package toolhooks

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// LinodeVolumeClonePreview reads the source volume so the preview can name what
// the copy is made from, which the request carries only as an id.
func LinodeVolumeClonePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	_ any,
) (*mcp.CallToolResult, error) {
	volumeID := request.GetInt("volume_id", 0)
	label := request.GetString("label", "")

	result, err := tools.RunDryRunPreviewDetailed(
		ctx, request, cfg, "linode_volume_clone", method, path,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return fetchedState(client.GetVolume(ctx, volumeID))
		},
		func(ctx context.Context, _ *linode.Client, state any) (tools.DryRunDetails, error) {
			return previewDetails(ctx, "linode_volume_clone", func() tools.DryRunDetails {
				return volumeCloneSideEffects(state, label)
			})
		},
	)

	return wrapPreview("linode_volume_clone", result, err)
}

// volumeCloneSideEffects is the Tier B preview prose for a clone. A state the
// fetch could not name still describes the copy, since the label the caller
// asked for is the part they are about to be billed for.
//
// A volume carrying no id is one the fetch did not name, whatever the reason,
// which is the reading the other language's walk has always used. Reporting
// "Volume 0" instead would put a source in front of a caller that does not
// exist.
func volumeCloneSideEffects(state any, label string) tools.DryRunDetails {
	var details tools.DryRunDetails

	details.Warnings = append(details.Warnings, "Billing for the cloned volume starts immediately on creation.")

	volume, isVolume := state.(*linode.Volume)
	if !isVolume || volume == nil || volume.ID == 0 {
		details.SideEffects = append(details.SideEffects,
			fmt.Sprintf("A new volume labeled %q will be created from the source volume.", label))

		return details
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("Volume %d (%q) will be cloned to a new volume labeled %q.", volume.ID, volume.Label, label))

	return details
}

// LinodeVolumeResizePreview reads the volume so the preview can name the size
// the resize starts from, which is what says whether it grows at all.
func LinodeVolumeResizePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	_ any,
) (*mcp.CallToolResult, error) {
	volumeID := request.GetInt("volume_id", 0)
	size := request.GetInt("size", 0)

	result, err := tools.RunDryRunPreviewDetailed(
		ctx, request, cfg, "linode_volume_resize", method, path,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return fetchedState(client.GetVolume(ctx, volumeID))
		},
		func(ctx context.Context, _ *linode.Client, state any) (tools.DryRunDetails, error) {
			return previewDetails(ctx, "linode_volume_resize", func() tools.DryRunDetails {
				return volumeResizeSideEffects(state, size)
			})
		},
	)

	return wrapPreview("linode_volume_resize", result, err)
}

// volumeResizeSideEffects is the Tier B walk for a resize, read against the size
// the volume carries now.
func volumeResizeSideEffects(state any, targetSize int) tools.DryRunDetails {
	var details tools.DryRunDetails

	var fromSize int
	if volume, isVolume := state.(*linode.Volume); isVolume && volume != nil {
		fromSize = volume.Size
	}

	effect := fmt.Sprintf("Volume resizes to %d GB.", targetSize)
	if fromSize != 0 {
		effect = fmt.Sprintf("Volume resizes from %d GB to %d GB.", fromSize, targetSize)
	}

	details.SideEffects = append(details.SideEffects, effect)
	details.Warnings = append(details.Warnings,
		"A volume can only grow; the new size must be larger than the current size.")

	return details
}

// LinodeVolumeUpdatePreview reads the volume so the preview can name the label
// being replaced rather than only the one asked for.
func LinodeVolumeUpdatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	_ any,
) (*mcp.CallToolResult, error) {
	volumeID := request.GetInt("volume_id", 0)
	label := request.GetString("label", "")
	_, hasTags := request.GetArguments()["tags"]

	result, err := tools.RunDryRunPreviewDetailed(
		ctx, request, cfg, "linode_volume_update", method, path,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return fetchedState(client.GetVolume(ctx, volumeID))
		},
		func(ctx context.Context, _ *linode.Client, state any) (tools.DryRunDetails, error) {
			return previewDetails(ctx, "linode_volume_update", func() tools.DryRunDetails {
				return volumeUpdateSideEffects(state, label, hasTags)
			})
		},
	)

	return wrapPreview("linode_volume_update", result, err)
}

// volumeUpdateSideEffects is the Tier B walk for an update, read against the
// volume as it stands now.
func volumeUpdateSideEffects(state any, newLabel string, hasTags bool) tools.DryRunDetails {
	var details tools.DryRunDetails

	var fromLabel string
	if volume, isVolume := state.(*linode.Volume); isVolume && volume != nil {
		fromLabel = volume.Label
	}

	tools.LabelChangeSideEffect(&details, fromLabel, newLabel)

	if hasTags {
		details.SideEffects = append(details.SideEffects,
			"The volume's tag set is replaced with the provided tags.")
	}

	return details
}

// LinodeVolumeDeleteDependencyWalk names the instance a delete takes the volume
// off. The attachment is already on the fetched state; the API is reached only
// when the volume record carried an instance id without its label.
func LinodeVolumeDeleteDependencyWalk(
	ctx context.Context,
	client *linode.Client,
	_ int,
	state tools.DeclaredState,
) (tools.DryRunDetails, error) {
	var details tools.DryRunDetails

	linodeID, attached := state.Number("linode_id")
	if !attached || linodeID == 0 {
		return details, nil
	}

	label := state.Text("linode_label")

	if label == "" {
		if instance, err := client.GetInstance(ctx, linodeID); err == nil && instance != nil {
			label = instance.Label
		}
	}

	details.Dependencies = append(details.Dependencies, tools.DryRunDependency{
		Kind:   "instance",
		ID:     linodeID,
		Label:  label,
		Action: "detached",
		Note:   "Volume is attached; it detaches from this instance before deletion.",
	})
	details.Warnings = append(details.Warnings,
		"Volume is currently attached to an instance; it will be detached as part of deletion.")

	return details, nil
}

// LinodeVolumeDetachPreview reads the volume and says which instance it comes
// off. The attachment is the part a caller cannot see from the request, since
// the call names only the volume.
func LinodeVolumeDetachPreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	_ any,
) (*mcp.CallToolResult, error) {
	volumeID := request.GetInt("volume_id", 0)

	result, err := tools.RunDryRunPreviewDetailed(
		ctx, request, cfg, "linode_volume_detach", method, path,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return client.GetVolume(ctx, volumeID)
		},
		func(ctx context.Context, _ *linode.Client, state any) (tools.DryRunDetails, error) {
			return previewDetails(ctx, "linode_volume_detach", func() tools.DryRunDetails {
				return volumeDetachSideEffects(state)
			})
		},
	)

	return wrapPreview("linode_volume_detach", result, err)
}

// volumeDetachSideEffects is the Tier B walk for a detach, read off the fetched
// volume: a volume attached to nothing detaches to no effect, which is worth
// saying before the call rather than after.
//
// A state carrying no attachment reads as unattached whatever the reason, which
// is what the other language's walk reports for the same answer: the fetch has
// already failed loudly if it failed at all, so what reaches here is a volume
// whose attachment the API either named or left out.
func volumeDetachSideEffects(state any) tools.DryRunDetails {
	var details tools.DryRunDetails

	volume, isVolume := state.(*linode.Volume)
	if !isVolume || volume == nil || volume.LinodeID == nil {
		details.SideEffects = append(details.SideEffects,
			"Volume is not attached to any instance; detach is a no-op.")

		return details
	}

	details.SideEffects = append(details.SideEffects, fmt.Sprintf(
		"Volume %d detaches from instance %d; its data is preserved and billing continues.",
		volume.ID, *volume.LinodeID,
	))

	return details
}
