package toolhooks

import (
	"context"
	"fmt"

	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// dependencyKindInstance is the kind every dependency naming a Linode carries.
// The walks below all report one, so the spelling is written once.
const dependencyKindInstance = "instance"

// LinodePlacementGroupDeleteDependencyWalk names what a placement-group delete
// takes with it: each member Linode is detached, and the instances themselves
// are not deleted. The members come from the declared fetch's state, so no extra
// call is made.
func LinodePlacementGroupDeleteDependencyWalk(
	ctx context.Context, _ *linode.Client, _ int, state tools.DeclaredState,
) (tools.DryRunDetails, error) {
	var details tools.DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("placement-group dependency walk canceled: %w", err)
	}

	members := state.Objects("members")

	for _, member := range members {
		id, _ := member.Number("linode_id")
		details.Dependencies = append(details.Dependencies, tools.DryRunDependency{
			Kind:   dependencyKindInstance,
			ID:     id,
			Action: tools.DependencyActionDetached,
			Note:   "Linode is removed from the placement group; the instance is not deleted.",
		})
	}

	if len(members) > 0 {
		details.Warnings = append(details.Warnings, fmt.Sprintf(
			"Deleting this placement group detaches %d Linode(s); the instances are not deleted.", len(members),
		))
	}

	return details, nil
}

// LinodeLkePoolDeleteDependencyWalk names what a pool delete takes with it: the
// fetched pool carries its nodes, each node's backing Linode is destroyed with
// it, and a warning counts them because the running workloads are the part a
// caller cannot put back.
func LinodeLkePoolDeleteDependencyWalk(
	ctx context.Context, _ *linode.Client, _, _ int, state tools.DeclaredState,
) (tools.DryRunDetails, error) {
	var details tools.DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("pool dependency walk canceled: %w", err)
	}

	for _, node := range state.Objects("nodes") {
		id, _ := node.Number("instance_id")
		details.Dependencies = append(details.Dependencies, tools.DryRunDependency{
			Kind:   dependencyKindInstance,
			ID:     id,
			Label:  node.Text("id"),
			Action: tools.DependencyActionCascadeDeleted,
			Note:   "Backing Linode for this pool node.",
		})
	}

	if count, sent := state.Number("count"); sent && count > 0 {
		details.Warnings = append(details.Warnings, fmt.Sprintf(
			"Deleting this pool destroys %d node(s) and their backing Linodes; running workloads are lost.", count,
		))
	}

	return details, nil
}

// LinodeLkeNodeDeleteDependencyWalk names the backing Linode the fetched node
// carries, which is destroyed with it, plus the pool-level effect a caller
// cannot see from the request.
func LinodeLkeNodeDeleteDependencyWalk(
	ctx context.Context, _ *linode.Client, _ int, _ string, state tools.DeclaredState,
) (tools.DryRunDetails, error) {
	var details tools.DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("node dependency walk canceled: %w", err)
	}

	if instance, sent := state.Number("instance_id"); sent && instance > 0 {
		details.Dependencies = append(details.Dependencies, tools.DryRunDependency{
			Kind:   dependencyKindInstance,
			ID:     instance,
			Label:  state.Text("id"),
			Action: tools.DependencyActionCascadeDeleted,
			Note:   "Backing Linode for this node.",
		})
	}

	details.Warnings = append(details.Warnings,
		"Deleting this node removes it from its pool; the pool node count drops by one and scheduled workloads reschedule.")

	return details, nil
}

// LinodeTagDeleteFetchState lists the objects carrying the tag a delete would
// remove. The page is what the preview reports and what the walk names one by
// one, so the tagged objects are read once for both.
func LinodeTagDeleteFetchState(ctx context.Context, client *linode.Client, tagLabel string) (any, error) {
	return fetchedState(client.ListTaggedObjects(ctx, tagLabel, 0, 0))
}

// LinodeTagDeleteDependencyWalk names each object that loses the tag; none of
// them is deleted. The itemized list is the fetched page, while the count comes
// from the envelope's total, so a truncated first page cannot understate the
// blast radius. A second warning says how many were itemized.
func LinodeTagDeleteDependencyWalk(
	ctx context.Context, _ *linode.Client, _ string, state any,
) (tools.DryRunDetails, error) {
	var details tools.DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("tag-delete dependency walk canceled: %w", err)
	}

	resp, ok := state.(*linode.PaginatedResponse[linode.TaggedObject])
	if !ok || resp == nil {
		return details, nil
	}

	for i := range resp.Data {
		details.Dependencies = append(details.Dependencies, taggedObjectDependency(resp.Data[i]))
	}

	tagDeleteWarnings(&details, resp.Results)

	return details, nil
}

// taggedObjectDependency reports one tagged object as the dependency it is: the
// object keeps existing, it only loses the tag.
func taggedObjectDependency(object linode.TaggedObject) tools.DryRunDependency {
	kind, _ := object["type"].(string)
	if kind == "" {
		kind = "resource"
	}

	dependency := tools.DryRunDependency{
		Kind:   kind,
		Action: tools.DependencyActionRemoved,
		Note:   "Loses this tag; the resource itself is not deleted.",
	}

	if data, dataOK := object["data"].(map[string]any); dataOK {
		dependency.ID = data["id"]
	}

	return dependency
}

// tagDeleteWarnings counts the blast radius from the page envelope's total, so a
// first page that does not carry every tagged object still reports how many
// there are.
func tagDeleteWarnings(details *tools.DryRunDetails, results int) {
	itemized := len(details.Dependencies)
	if itemized == 0 {
		return
	}

	total := max(results, itemized)

	details.Warnings = append(details.Warnings, fmt.Sprintf(
		"Deleting this tag removes it from %d tagged object(s); the objects are not deleted.", total,
	))

	if total > itemized {
		details.Warnings = append(details.Warnings, fmt.Sprintf(
			"Only the first %d tagged object(s) are itemized in this preview.", itemized,
		))
	}
}

// LinodeInstanceRebuildDependencyWalk names what a rebuild erases: every disk is
// recreated from the new image, and the current image is named in a warning
// because that is what the caller is replacing. Best-effort, so a failed disk
// list becomes a warning rather than refusing the preview.
func LinodeInstanceRebuildDependencyWalk(
	ctx context.Context, client *linode.Client, linodeID int, state tools.DeclaredState,
) (tools.DryRunDetails, error) {
	var details tools.DryRunDetails

	disks, err := client.ListInstanceDisks(ctx, linodeID)
	if err != nil {
		details.Warnings = append(details.Warnings, fmt.Sprintf("Could not list instance disks: %v", err))
	}

	for i := range disks {
		disk := &disks[i]
		details.SideEffects = append(details.SideEffects, fmt.Sprintf(
			"Disk %q (%d MB, %s) is erased and recreated from the new image.", disk.Label, disk.Size, disk.Filesystem,
		))
	}

	warning := "Rebuild destroys all data on the instance and resets the root password."
	if image := state.Text("image"); image != "" {
		warning = fmt.Sprintf(
			"Rebuild replaces the current image %q, destroys all data, and resets the root password.", image,
		)
	}

	details.Warnings = append(details.Warnings, warning)

	return details, nil
}
