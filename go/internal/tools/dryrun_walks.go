package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// Phase 2 dependency-walk shared vocabulary. Action names match the spec's
// dependencies[].action enum; billingUnknown is the sentinel returned when
// a cost estimate cannot be computed.
const (
	dependencyActionDetached       = "detached"
	dependencyActionReleased       = "released"
	dependencyActionRemoved        = "removed"
	dependencyActionCascadeDeleted = "cascade_deleted"

	// dependencyKindInstance is the dependency kind for a Linode instance,
	// shared by walks that surface an attached/affected instance.
	dependencyKindInstance = "instance"

	billingUnknown = "unknown"

	// dependencyWalkPageSize bounds a single dependency-list fetch. A
	// resource with more dependents than this is rare; the walk notes a
	// possible truncation rather than paging exhaustively during a preview.
	dependencyWalkPageSize = 100

	statusRunning        = "running"
	runningDeleteWarning = "Instance is currently running. Delete will not pause for a graceful shutdown."
)

// instanceBillingDelta estimates the monthly cost change from deleting an
// instance of the given type. Best-effort: returns the "unknown" sentinel
// when the type is empty or its pricing cannot be fetched.
func instanceBillingDelta(ctx context.Context, client *linode.Client, typeID string) *DryRunBillingDelta {
	if typeID == "" {
		return &DryRunBillingDelta{MonthlyChangeUSD: billingUnknown}
	}

	instanceType, err := client.GetType(ctx, typeID)
	if err != nil || instanceType == nil {
		return &DryRunBillingDelta{
			MonthlyChangeUSD: billingUnknown,
			Note:             "Could not fetch type pricing for the estimate.",
		}
	}

	return &DryRunBillingDelta{
		MonthlyChangeUSD: fmt.Sprintf("-%.2f", instanceType.Price.Monthly),
		Note:             "Instance billing stops. Attached volume billing continues.",
	}
}

// instanceVolumeDeps lists volumes attached to the instance. They survive
// the delete (detach, not destroy), so their billing continues.
func instanceVolumeDeps(ctx context.Context, client *linode.Client, linodeID int) ([]DryRunDependency, string) {
	volumes, err := client.ListInstanceVolumes(ctx, linodeID, 1, dependencyWalkPageSize)
	if err != nil {
		return nil, fmt.Sprintf("Could not list attached volumes: %v", err)
	}

	deps := make([]DryRunDependency, 0, len(volumes))
	for i := range volumes {
		volume := &volumes[i]
		deps = append(deps, DryRunDependency{
			Kind:   "volume",
			ID:     volume.ID,
			Label:  volume.Label,
			Action: dependencyActionDetached,
			Note:   fmt.Sprintf("%dGB volume stays; billing continues.", volume.Size),
		})
	}

	return deps, ""
}

// instanceIPDeps lists the instance's public IPv4 addresses. Ephemeral
// addresses return to the pool when the instance is deleted; reserved
// addresses detach while their reservation and billing continue.
func instanceIPDeps(ctx context.Context, client *linode.Client, linodeID int) ([]DryRunDependency, string) {
	ips, err := client.ListInstanceIPs(ctx, linodeID)
	if err != nil {
		return nil, fmt.Sprintf("Could not list IP addresses: %v", err)
	}

	if ips == nil || ips.IPv4 == nil {
		return nil, ""
	}

	deps := make([]DryRunDependency, 0, len(ips.IPv4.Public)+len(ips.IPv4.Reserved))
	for i := range ips.IPv4.Public {
		addr := &ips.IPv4.Public[i]
		deps = append(deps, DryRunDependency{
			Kind:   "public_ip",
			Label:  addr.Address,
			Action: dependencyActionReleased,
		})
	}

	for i := range ips.IPv4.Reserved {
		addr := &ips.IPv4.Reserved[i]
		deps = append(deps, DryRunDependency{
			Kind:   "public_ip",
			Label:  addr.Address,
			Action: dependencyActionDetached,
			Note:   "Reserved IP is detached from the instance; reservation and billing continue.",
		})
	}

	return deps, ""
}

// instanceFirewallDeps lists firewalls this instance is attached to. The
// firewalls survive; the instance is dropped from each firewall's devices.
func instanceFirewallDeps(ctx context.Context, client *linode.Client, linodeID int) ([]DryRunDependency, string) {
	firewalls, err := client.ListInstanceFirewalls(ctx, linodeID, 1, dependencyWalkPageSize)
	if err != nil {
		return nil, fmt.Sprintf("Could not list firewalls: %v", err)
	}

	deps := make([]DryRunDependency, 0, len(firewalls))
	for i := range firewalls {
		firewall := &firewalls[i]
		deps = append(deps, DryRunDependency{
			Kind:   "firewall",
			ID:     firewall.ID,
			Label:  firewall.Label,
			Action: dependencyActionRemoved,
			Note:   "Firewall stays; this instance is removed from its device list.",
		})
	}

	return deps, ""
}

// instanceDeleteDependencyWalk is the Tier A walk for linode_instance_delete.
// It surfaces the resources a delete affects (attached volumes detach,
// ephemeral public IPs release, reserved public IPs detach, firewall
// attachments drop), estimates the monthly billing change, and warns when the
// instance is running. Each sub-list is best-effort: a fetch error becomes a
// warning rather than failing the whole preview, so a partial dependency
// picture still reaches the model.
func instanceDeleteDependencyWalk(ctx context.Context, client *linode.Client, linodeID int, state any) (DryRunDetails, error) {
	var details DryRunDetails

	categories := []func(context.Context, *linode.Client, int) ([]DryRunDependency, string){
		instanceVolumeDeps,
		instanceIPDeps,
		instanceFirewallDeps,
	}

	for _, category := range categories {
		deps, warning := category(ctx, client, linodeID)
		details.Dependencies = append(details.Dependencies, deps...)

		if warning != "" {
			details.Warnings = append(details.Warnings, warning)
		}
	}

	if instance, ok := state.(*linode.Instance); ok && instance != nil {
		details.BillingDelta = instanceBillingDelta(ctx, client, instance.Type)

		if strings.EqualFold(instance.Status, statusRunning) {
			details.Warnings = append(details.Warnings, runningDeleteWarning)
		}
	}

	return details, nil
}

// instanceRebuildSideEffectsWalk is the Tier A walk for linode_instance_rebuild.
// A rebuild erases every disk and recreates the boot disk from the new image,
// so each existing disk is reported as a side effect and the current image is
// named in a warning. Best-effort: a failed disk list becomes a warning.
func instanceRebuildSideEffectsWalk(ctx context.Context, client *linode.Client, state any) (DryRunDetails, error) {
	var details DryRunDetails

	instance, ok := state.(*linode.Instance)
	if !ok || instance == nil {
		return details, nil
	}

	disks, err := client.ListInstanceDisks(ctx, instance.ID)
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
	if instance.Image != "" {
		warning = fmt.Sprintf(
			"Rebuild replaces the current image %q, destroys all data, and resets the root password.", instance.Image,
		)
	}

	details.Warnings = append(details.Warnings, warning)

	return details, nil
}

// instancePasswordResetSideEffectsWalk is the Tier A walk for
// linode_instance_password_reset. The reset powers the instance down and
// reboots it; that downtime is the only side effect, with an extra warning
// when the instance is currently running.
func instancePasswordResetSideEffectsWalk(ctx context.Context, _ *linode.Client, _ int, state any) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("password-reset side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		"The instance is powered down and rebooted to apply the new root password.")

	if instance, ok := state.(*linode.Instance); ok && instance != nil && strings.EqualFold(instance.Status, statusRunning) {
		details.Warnings = append(details.Warnings,
			"Instance is currently running; the reset shuts it down and reboots it, causing downtime.")
	}

	return details, nil
}

// instanceRescueSideEffectsWalk is the Tier A walk for linode_instance_rescue.
// Rescue mode reboots the instance and replaces its normal boot configuration
// with a recovery environment until the operator reboots out of it; that is
// reported as a side effect, with a downtime warning when it is running.
func instanceRescueSideEffectsWalk(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("rescue side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		"The instance reboots into rescue mode; its normal boot configuration is bypassed until you reboot out of rescue mode.")

	if instance, ok := state.(*linode.Instance); ok && instance != nil && strings.EqualFold(instance.Status, statusRunning) {
		details.Warnings = append(details.Warnings,
			"Instance is currently running; entering rescue mode reboots it, causing downtime.")
	}

	return details, nil
}

// backupRestoreSideEffects is the Tier A walk for linode_instance_backup_restore.
// The side effect depends on the overwrite flag: with overwrite the target
// instance's existing disks and configs are destroyed and replaced, otherwise
// the backup is restored alongside what is already there.
func backupRestoreSideEffects(ctx context.Context, targetLinodeID int, overwrite bool) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("backup-restore side-effect walk canceled: %w", err)
	}

	if overwrite {
		details.SideEffects = append(details.SideEffects, fmt.Sprintf(
			"All existing disks and configs on target instance %d are destroyed and replaced by the backup.", targetLinodeID,
		))
		details.Warnings = append(details.Warnings, fmt.Sprintf(
			"overwrite=true: existing data on target instance %d is permanently lost.", targetLinodeID,
		))

		return details, nil
	}

	details.SideEffects = append(details.SideEffects, fmt.Sprintf(
		"The backup is restored onto target instance %d; the restore fails if its disks or configs collide.", targetLinodeID,
	))

	return details, nil
}

// volumeAttachSideEffects is the Tier B walk for linode_volume_attach. It
// describes the attachment the call would make. The instance is an argument,
// so it is passed in rather than read from the fetched volume state.
func volumeAttachSideEffects(ctx context.Context, volumeID, linodeID int) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("attach side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects, fmt.Sprintf(
		"Volume %d attaches to instance %d; the volume must be in the same region as the instance.",
		volumeID, linodeID,
	))

	return details, nil
}

// volumeDetachSideEffects is the Tier B walk for linode_volume_detach. It reads
// the volume's current attachment from the fetched state and reports the
// detach (data is preserved, billing continues).
func volumeDetachSideEffects(ctx context.Context, state any) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("detach side-effect walk canceled: %w", err)
	}

	volume, ok := state.(*linode.Volume)
	if !ok || volume == nil {
		return details, nil
	}

	if volume.LinodeID == nil {
		details.SideEffects = append(details.SideEffects,
			"Volume is not attached to any instance; detach is a no-op.")

		return details, nil
	}

	details.SideEffects = append(details.SideEffects, fmt.Sprintf(
		"Volume %d detaches from instance %d; its data is preserved and billing continues.",
		volume.ID, *volume.LinodeID,
	))

	return details, nil
}

// instanceResizeSideEffects is the Tier B walk for linode_instance_resize. It
// names the type change (from the fetched state to the requested type) and
// warns about the reboot and billing change.
func instanceResizeSideEffects(ctx context.Context, fromType, targetType string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("resize side-effect walk canceled: %w", err)
	}

	effect := fmt.Sprintf(
		"Instance resizes to type %s; it reboots and is unavailable during the resize.", targetType,
	)
	if fromType != "" {
		effect = fmt.Sprintf(
			"Instance resizes from type %s to %s; it reboots and is unavailable during the resize.",
			fromType, targetType,
		)
	}

	details.SideEffects = append(details.SideEffects, effect)
	details.Warnings = append(details.Warnings,
		"Resizing changes the monthly price to match the new type.")

	return details, nil
}

// instanceMigrateSideEffects is the Tier B walk for linode_instance_migrate. It
// names the region change (from the fetched state to the requested region) and
// notes the downtime during migration.
func instanceMigrateSideEffects(ctx context.Context, state any, targetRegion string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("migrate side-effect walk canceled: %w", err)
	}

	var fromRegion string
	if instance, ok := state.(*linode.Instance); ok && instance != nil {
		fromRegion = instance.Region
	}

	effect := "Instance migrates; it is unavailable during the migration."

	switch {
	case targetRegion != "" && fromRegion != "":
		effect = fmt.Sprintf(
			"Instance migrates from region %s to %s; it is unavailable during the migration.",
			fromRegion, targetRegion,
		)
	case targetRegion != "":
		effect = fmt.Sprintf(
			"Instance migrates to region %s; it is unavailable during the migration.", targetRegion,
		)
	}

	details.SideEffects = append(details.SideEffects, effect)

	return details, nil
}

// instanceMutateSideEffects is the Tier B walk for linode_instance_mutate. It
// names the current type (from the fetched state) being upgraded to the latest
// generation and notes the reboot.
func instanceMutateSideEffects(ctx context.Context, state any) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("mutate side-effect walk canceled: %w", err)
	}

	var fromType string
	if instance, ok := state.(*linode.Instance); ok && instance != nil {
		fromType = instance.Type
	}

	effect := "Instance upgrades to the latest generation of its type; it reboots during the upgrade."
	if fromType != "" {
		effect = fmt.Sprintf(
			"Instance type %s upgrades to the latest generation; it reboots during the upgrade.", fromType,
		)
	}

	details.SideEffects = append(details.SideEffects, effect)

	return details, nil
}

// volumeResizeSideEffects is the Tier B walk for linode_volume_resize. It names
// the size change (from the fetched state to the requested size) and warns that
// a volume can only grow.
func volumeResizeSideEffects(ctx context.Context, state any, targetSize int) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("volume-resize side-effect walk canceled: %w", err)
	}

	var fromSize int
	if volume, ok := state.(*linode.Volume); ok && volume != nil {
		fromSize = volume.Size
	}

	effect := fmt.Sprintf("Volume resizes to %d GB.", targetSize)
	if fromSize != 0 {
		effect = fmt.Sprintf("Volume resizes from %d GB to %d GB.", fromSize, targetSize)
	}

	details.SideEffects = append(details.SideEffects, effect)
	details.Warnings = append(details.Warnings,
		"A volume can only grow; the new size must be larger than the current size.")

	return details, nil
}

// instanceDiskCloneSideEffects is the Tier B walk for
// linode_instance_disk_clone. It reports the new disk a clone creates on the
// same instance and the additional storage it consumes.
func instanceDiskCloneSideEffects(ctx context.Context, state any) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("disk-clone side-effect walk canceled: %w", err)
	}

	if disk, ok := state.(*linode.InstanceDisk); ok && disk != nil {
		details.SideEffects = append(details.SideEffects, fmt.Sprintf(
			"Disk %q (%d MB) is cloned to a new disk on the same instance, consuming %d MB of additional storage.",
			disk.Label, disk.Size, disk.Size,
		))

		return details, nil
	}

	details.SideEffects = append(details.SideEffects,
		"A copy of the disk is created on the same instance, consuming additional storage.")

	return details, nil
}

// instanceDiskResizeSideEffects is the Tier B walk for
// linode_instance_disk_resize. It names the size change (from the fetched
// state to the requested size, in MB) and notes the power-off requirement.
func instanceDiskResizeSideEffects(ctx context.Context, state any, targetSize int) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("disk-resize side-effect walk canceled: %w", err)
	}

	var fromSize int
	if disk, ok := state.(*linode.InstanceDisk); ok && disk != nil {
		fromSize = disk.Size
	}

	effect := fmt.Sprintf("Disk resizes to %d MB.", targetSize)
	if fromSize != 0 {
		effect = fmt.Sprintf("Disk resizes from %d MB to %d MB.", fromSize, targetSize)
	}

	details.SideEffects = append(details.SideEffects, effect)
	details.Warnings = append(details.Warnings,
		"The instance must be powered off to resize a disk.")

	return details, nil
}

// labelChangeSideEffect appends a "label changes from X to Y" (or "label set
// to Y") side effect when newLabel is non-empty, comparing against the current
// label from the fetched state.
func labelChangeSideEffect(details *DryRunDetails, fromLabel, newLabel string) {
	if newLabel == "" {
		return
	}

	if fromLabel != "" && fromLabel != newLabel {
		details.SideEffects = append(details.SideEffects,
			fmt.Sprintf("Label changes from %q to %q.", fromLabel, newLabel))

		return
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("Label is set to %q.", newLabel))
}

// volumeUpdateSideEffects is the Tier B walk for linode_volume_update. It
// reports the label change (against the fetched state) and notes when the tag
// set is replaced.
func volumeUpdateSideEffects(ctx context.Context, state any, newLabel string, hasTags bool) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("volume-update side-effect walk canceled: %w", err)
	}

	var fromLabel string
	if volume, ok := state.(*linode.Volume); ok && volume != nil {
		fromLabel = volume.Label
	}

	labelChangeSideEffect(&details, fromLabel, newLabel)

	if hasTags {
		details.SideEffects = append(details.SideEffects,
			"The volume's tag set is replaced with the provided tags.")
	}

	return details, nil
}

// firewallUpdateSideEffects is the Tier B walk for linode_firewall_update. It
// reports the label change and a status change (enabled/disabled) against the
// fetched state.
func firewallUpdateSideEffects(ctx context.Context, state any, newLabel, newStatus string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("firewall-update side-effect walk canceled: %w", err)
	}

	var (
		fromLabel  string
		fromStatus string
	)

	if firewall, ok := state.(*linode.Firewall); ok && firewall != nil {
		fromLabel = firewall.Label
		fromStatus = firewall.Status
	}

	labelChangeSideEffect(&details, fromLabel, newLabel)

	if newStatus != "" && newStatus != fromStatus {
		details.SideEffects = append(details.SideEffects, fmt.Sprintf(
			"Firewall status changes to %q; this immediately %s its rules.",
			newStatus, firewallStatusVerb(newStatus),
		))
	}

	return details, nil
}

// firewallStatusVerb maps a target firewall status to how it affects rule
// enforcement, for the status-change side effect.
func firewallStatusVerb(status string) string {
	if status == "disabled" {
		return "stops enforcing"
	}

	return "starts enforcing"
}

// vpcUpdateSideEffects is the Tier B walk for linode_vpc_update. It reports the
// label change against the fetched state and notes a description change.
func vpcUpdateSideEffects(ctx context.Context, state any, newLabel, newDescription string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("vpc-update side-effect walk canceled: %w", err)
	}

	var fromLabel string
	if vpc, ok := state.(*linode.VPC); ok && vpc != nil {
		fromLabel = vpc.Label
	}

	labelChangeSideEffect(&details, fromLabel, newLabel)

	if newDescription != "" {
		details.SideEffects = append(details.SideEffects,
			"The VPC description is updated.")
	}

	return details, nil
}

// nodebalancerUpdateSideEffects is the Tier B walk for
// linode_nodebalancer_update. It reports the label change and a
// connection-throttle change against the fetched state.
func nodebalancerUpdateSideEffects(ctx context.Context, state any, newLabel string, newThrottle int) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("nodebalancer-update side-effect walk canceled: %w", err)
	}

	var fromLabel string
	if nb, ok := state.(*linode.NodeBalancer); ok && nb != nil {
		fromLabel = nb.Label
	}

	labelChangeSideEffect(&details, fromLabel, newLabel)

	if newThrottle != notProvided {
		details.SideEffects = append(details.SideEffects, fmt.Sprintf(
			"Connection throttle is set to %d connections per second per client IP.", newThrottle,
		))
	}

	return details, nil
}

// domainUpdateSideEffects is the Tier B walk for linode_domain_update. It
// reports status and SOA-email changes against the fetched state and notes a
// description change.
func domainUpdateSideEffects(ctx context.Context, state any, newStatus, newSOA, newDescription string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("domain-update side-effect walk canceled: %w", err)
	}

	var (
		fromStatus string
		fromSOA    string
	)

	if domain, ok := state.(*linode.Domain); ok && domain != nil {
		fromStatus = domain.Status
		fromSOA = domain.SOAEmail
	}

	if newStatus != "" && newStatus != fromStatus {
		details.SideEffects = append(details.SideEffects,
			fmt.Sprintf("Domain status changes from %q to %q.", fromStatus, newStatus))
	}

	if newSOA != "" && newSOA != fromSOA {
		details.SideEffects = append(details.SideEffects,
			fmt.Sprintf("SOA email is set to %q.", newSOA))
	}

	if newDescription != "" {
		details.SideEffects = append(details.SideEffects, "The domain description is updated.")
	}

	return details, nil
}

// lkeClusterUpdateSideEffects is the Tier B walk for linode_lke_cluster_update.
// It reports the label change and a Kubernetes version change (a node upgrade)
// against the fetched state.
func lkeClusterUpdateSideEffects(ctx context.Context, state any, newLabel, newK8sVersion string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("lke-cluster-update side-effect walk canceled: %w", err)
	}

	var (
		fromLabel   string
		fromVersion string
	)

	if cluster, ok := state.(*linode.LKECluster); ok && cluster != nil {
		fromLabel = cluster.Label
		fromVersion = cluster.K8sVersion
	}

	labelChangeSideEffect(&details, fromLabel, newLabel)

	if newK8sVersion != "" && newK8sVersion != fromVersion {
		details.SideEffects = append(details.SideEffects, fmt.Sprintf(
			"Kubernetes version changes from %q to %q; the control plane and nodes upgrade.",
			fromVersion, newK8sVersion,
		))
	}

	return details, nil
}

// domainRecordUpdateSideEffects is the Tier B walk for
// linode_domain_record_update. It reports the record name and target changes
// against the fetched state.
func domainRecordUpdateSideEffects(ctx context.Context, state any, newName, newTarget string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("domain-record-update side-effect walk canceled: %w", err)
	}

	var (
		fromName   string
		fromTarget string
	)

	if record, ok := state.(*linode.DomainRecord); ok && record != nil {
		fromName = record.Name
		fromTarget = record.Target
	}

	if newName != "" && newName != fromName {
		details.SideEffects = append(details.SideEffects,
			fmt.Sprintf("Record name changes from %q to %q.", fromName, newName))
	}

	if newTarget != "" && newTarget != fromTarget {
		details.SideEffects = append(details.SideEffects,
			fmt.Sprintf("Record target changes from %q to %q.", fromTarget, newTarget))
	}

	return details, nil
}

// sshKeyUpdateSideEffects is the Tier B walk for linode_sshkey_update. It
// reports the label change against the fetched state.
func sshKeyUpdateSideEffects(ctx context.Context, state any, newLabel string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("sshkey-update side-effect walk canceled: %w", err)
	}

	var fromLabel string
	if key, ok := state.(*linode.SSHKey); ok && key != nil {
		fromLabel = key.Label
	}

	labelChangeSideEffect(&details, fromLabel, newLabel)

	return details, nil
}

// stackScriptUpdateSideEffects is the Tier B walk for
// linode_stackscript_update. It reports the label change and notes when the
// script body or description is updated.
func stackScriptUpdateSideEffects(ctx context.Context, state any, newLabel, newScript, newDescription string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("stackscript-update side-effect walk canceled: %w", err)
	}

	var fromLabel string
	if script, ok := state.(*linode.StackScript); ok && script != nil {
		fromLabel = script.Label
	}

	labelChangeSideEffect(&details, fromLabel, newLabel)

	if newScript != "" {
		details.SideEffects = append(details.SideEffects, "The StackScript body is replaced.")
	}

	if newDescription != "" {
		details.SideEffects = append(details.SideEffects, "The StackScript description is updated.")
	}

	return details, nil
}

// lkePoolUpdateSideEffects is the Tier B walk for linode_lke_pool_update. It
// reports a node-count change (against the fetched pool) and an autoscaler
// reconfiguration.
func lkePoolUpdateSideEffects(ctx context.Context, state any, newCount int, countProvided, autoscalerProvided bool) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("lke-pool-update side-effect walk canceled: %w", err)
	}

	if countProvided {
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

	if autoscalerProvided {
		details.SideEffects = append(details.SideEffects,
			"The pool autoscaler configuration is updated.")
	}

	return details, nil
}

// lkeACLUpdateSideEffects is the Tier B walk for linode_lke_acl_update. It
// reports whether the control-plane ACL is being enabled or disabled (gating
// Kubernetes API reachability).
func lkeACLUpdateSideEffects(ctx context.Context, enabled bool) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("lke-acl-update side-effect walk canceled: %w", err)
	}

	effect := "The control-plane ACL is disabled; the Kubernetes API becomes reachable from any address."
	if enabled {
		effect = "The control-plane ACL is enabled; only the listed addresses may reach the Kubernetes API."
	}

	details.SideEffects = append(details.SideEffects, effect)

	return details, nil
}

// networkingIPUpdateRDNSSideEffects is the Tier B walk for
// linode_networking_ip_update. It reports the reverse-DNS change or its
// removal.
func networkingIPUpdateRDNSSideEffects(ctx context.Context, rdns string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("rdns-update side-effect walk canceled: %w", err)
	}

	effect := "Reverse DNS (rDNS) is cleared."
	if rdns != "" {
		effect = fmt.Sprintf("Reverse DNS (rDNS) is set to %q.", rdns)
	}

	details.SideEffects = append(details.SideEffects, effect)

	return details, nil
}

// objectStorageKeyUpdateSideEffects is the Tier B walk for
// linode_object_storage_key_update. It reports the label change against the
// fetched key (credential-safe: the GET never returns the secret) and notes
// when bucket access scopes are replaced.
func objectStorageKeyUpdateSideEffects(ctx context.Context, state any, newLabel, newBucketAccess string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("key-update side-effect walk canceled: %w", err)
	}

	var fromLabel string
	if key, ok := state.(*linode.ObjectStorageKey); ok && key != nil {
		fromLabel = key.Label
	}

	labelChangeSideEffect(&details, fromLabel, newLabel)

	if newBucketAccess != "" {
		details.SideEffects = append(details.SideEffects,
			"The key's bucket access scopes are replaced.")
	}

	return details, nil
}

// bucketAccessUpdateSideEffects is the Tier B walk for
// linode_object_storage_bucket_access_update. It reports the ACL change and a
// CORS enable/disable toggle from the request args.
func bucketAccessUpdateSideEffects(ctx context.Context, acl string, corsProvided, corsEnabled bool) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("bucket-access-update side-effect walk canceled: %w", err)
	}

	if acl != "" {
		details.SideEffects = append(details.SideEffects,
			fmt.Sprintf("Bucket access control is set to %q.", acl))
	}

	if corsProvided {
		state := "disabled"
		if corsEnabled {
			state = "enabled"
		}

		details.SideEffects = append(details.SideEffects,
			fmt.Sprintf("CORS is %s for the bucket.", state))
	}

	return details, nil
}

// objectACLUpdateSideEffects is the Tier B walk for
// linode_object_storage_object_acl_update. It reports the new access-control
// level the object is set to.
func objectACLUpdateSideEffects(ctx context.Context, acl string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("object-acl-update side-effect walk canceled: %w", err)
	}

	if acl != "" {
		details.SideEffects = append(details.SideEffects,
			fmt.Sprintf("Object access control is set to %q.", acl))
	}

	return details, nil
}

// volumeCreateSideEffects is the Tier B preview for linode_volume_create. It
// describes the volume that would be created (arg-only, no fetch) and warns
// that billing begins on creation.
func volumeCreateSideEffects(ctx context.Context, label, region string, size, linodeID int) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("volume-create side-effect walk canceled: %w", err)
	}

	effect := fmt.Sprintf("A new %d GB volume %q will be created", size, label)
	if region != "" {
		effect += " in region " + region
	}

	details.SideEffects = append(details.SideEffects, effect+".")

	if linodeID != 0 {
		details.SideEffects = append(details.SideEffects,
			fmt.Sprintf("The volume is attached to instance %d on creation.", linodeID))
	}

	details.Warnings = append(details.Warnings, "Billing for the volume starts immediately on creation.")

	return details, nil
}

// volumeCloneSideEffects is the Tier B preview for linode_volume_clone. It
// reports the cloned volume label and warns that cloning creates a billable volume.
func volumeCloneSideEffects(ctx context.Context, state any, label string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("volume-clone side-effect walk canceled: %w", err)
	}

	volume, ok := state.(*linode.Volume)
	if !ok || volume == nil {
		details.SideEffects = append(details.SideEffects,
			fmt.Sprintf("A new volume labeled %q will be created from the source volume.", label))
		details.Warnings = append(details.Warnings, "Billing for the cloned volume starts immediately on creation.")

		return details, nil
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("Volume %d (%q) will be cloned to a new volume labeled %q.", volume.ID, volume.Label, label))
	details.Warnings = append(details.Warnings, "Billing for the cloned volume starts immediately on creation.")

	return details, nil
}

// firewallCreateSideEffects is the Tier B preview for linode_firewall_create.
// It reports the default inbound/outbound policies the new firewall would
// carry (arg-only, no fetch).
func firewallCreateSideEffects(ctx context.Context, label, inboundPolicy, outboundPolicy string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("firewall-create side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("A new Cloud Firewall %q will be created with inbound policy %s and outbound policy %s.",
			label, inboundPolicy, outboundPolicy))

	return details, nil
}

// nodebalancerCreateSideEffects is the Tier B preview for
// linode_nodebalancer_create. It describes the load balancer that would be
// created (arg-only, no fetch) and warns that billing begins on creation.
func nodebalancerCreateSideEffects(ctx context.Context, label, region string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("nodebalancer-create side-effect walk canceled: %w", err)
	}

	effect := fmt.Sprintf("A new NodeBalancer will be created in region %s.", region)
	if label != "" {
		effect = fmt.Sprintf("A new NodeBalancer %q will be created in region %s.", label, region)
	}

	details.SideEffects = append(details.SideEffects, effect)
	details.Warnings = append(details.Warnings, "Billing for the NodeBalancer starts immediately on creation.")

	return details, nil
}

// vpcCreateSideEffects is the Tier B preview for linode_vpc_create. It names
// the VPC and region (arg-only, no fetch).
func vpcCreateSideEffects(ctx context.Context, label, region string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("vpc-create side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("A new VPC %q will be created in region %s.", label, region))

	return details, nil
}

// domainCreateSideEffects is the Tier B preview for linode_domain_create. It
// reports the domain type and name (arg-only, no fetch).
func domainCreateSideEffects(ctx context.Context, domainType, domain string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("domain-create side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("A new %s DNS domain %q will be created.", domainType, domain))

	return details, nil
}

// sshKeyCreateSideEffects is the Tier B preview for linode_sshkey_create. The
// public key itself is never echoed; only the label is surfaced (arg-only, no
// fetch).
func sshKeyCreateSideEffects(ctx context.Context, label string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("sshkey-create side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("A new SSH key %q will be added to your profile.", label))

	return details, nil
}

// instanceCreateSideEffects is the Tier B preview for linode_instance_create.
// It describes the instance that would be created (arg-only, no fetch) and
// warns that billing begins on creation.
func instanceCreateSideEffects(ctx context.Context, instanceType, region, image string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("instance-create side-effect walk canceled: %w", err)
	}

	effect := fmt.Sprintf("A new %s instance will be created in region %s", instanceType, region)
	if image != "" {
		effect += " from image " + image
	}

	details.SideEffects = append(details.SideEffects, effect+".")
	details.Warnings = append(details.Warnings, "Billing for the instance starts immediately on creation.")

	return details, nil
}

// lkeClusterCreateSideEffects is the Tier B preview for
// linode_lke_cluster_create. It describes the cluster that would be created
// (arg-only, no fetch) and warns that node-pool billing begins on creation.
func lkeClusterCreateSideEffects(ctx context.Context, label, region, k8sVersion string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("lke-cluster-create side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("A new LKE cluster %q will be created in region %s running Kubernetes %s.",
			label, region, k8sVersion))
	details.Warnings = append(details.Warnings,
		"Billing for the cluster's node pools starts immediately on creation.")

	return details, nil
}

// imageCreateSideEffects is the Tier B preview for linode_image_create. It
// names the source disk and optional label (arg-only, no fetch).
func imageCreateSideEffects(ctx context.Context, diskID int, label string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("image-create side-effect walk canceled: %w", err)
	}

	effect := fmt.Sprintf("A new image will be captured from disk %d", diskID)
	if label != "" {
		effect += fmt.Sprintf(" and labeled %q", label)
	}

	details.SideEffects = append(details.SideEffects, effect+".")

	return details, nil
}

// bucketCreateSideEffects is the Tier B preview for
// linode_object_storage_bucket_create. It names the bucket and region
// (arg-only, no fetch) and warns that billing begins on creation.
func bucketCreateSideEffects(ctx context.Context, label, region string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("bucket-create side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("A new Object Storage bucket %q will be created in %s.", label, region))
	details.Warnings = append(details.Warnings, "Billing for Object Storage starts immediately on creation.")

	return details, nil
}

// objectStorageKeyCreateSideEffects is the Tier B preview for
// linode_object_storage_key_create. It names the key (arg-only, no fetch) and
// warns that the secret is shown only once at creation.
func objectStorageKeyCreateSideEffects(ctx context.Context, label string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("object-storage-key-create side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("A new Object Storage access key %q will be created.", label))
	details.Warnings = append(details.Warnings, "The secret key is returned only once, at creation time.")

	return details, nil
}

// domainRecordCreateSideEffects is the Tier B preview for
// linode_domain_record_create. It names the record type, host, and target
// (arg-only, no fetch).
func domainRecordCreateSideEffects(ctx context.Context, recordType, name, target string, domainID int) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("domain-record-create side-effect walk canceled: %w", err)
	}

	effect := fmt.Sprintf("A new %s record will be created in domain %d", recordType, domainID)
	if name != "" {
		effect += fmt.Sprintf(" for host %q", name)
	}

	if target != "" {
		effect += fmt.Sprintf(" targeting %q", target)
	}

	details.SideEffects = append(details.SideEffects, effect+".")

	return details, nil
}

// vpcSubnetCreateSideEffects is the Tier B preview for
// linode_vpc_subnet_create. It names the subnet, its IPv4 range, and the
// parent VPC (arg-only, no fetch).
func vpcSubnetCreateSideEffects(ctx context.Context, label, ipv4 string, vpcID int) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("vpc-subnet-create side-effect walk canceled: %w", err)
	}

	effect := fmt.Sprintf("A new subnet %q will be created in VPC %d", label, vpcID)
	if ipv4 != "" {
		effect += " with IPv4 range " + ipv4
	}

	details.SideEffects = append(details.SideEffects, effect+".")

	return details, nil
}

// ipv6RangeCreateSideEffects is the Tier B preview for
// linode_ipv6_range_create. It states the prefix length and where the range
// would be routed (arg-only, no fetch).
func ipv6RangeCreateSideEffects(ctx context.Context, prefixLength, linodeID int, routeTarget string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("ipv6-range-create side-effect walk canceled: %w", err)
	}

	var suffix string
	if routeTarget != "" {
		suffix = " and routed to " + routeTarget
	}

	if suffix == "" && linodeID != 0 {
		suffix = fmt.Sprintf(" and routed to instance %d", linodeID)
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("A new IPv6 range with prefix length /%d will be allocated%s.", prefixLength, suffix))

	return details, nil
}

// firewallDeviceCreateSideEffects is the Tier B preview for
// linode_firewall_device_create. It names the entity being attached to the
// firewall (arg-only, no fetch).
func firewallDeviceCreateSideEffects(ctx context.Context, deviceType string, deviceID, firewallID int) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("firewall-device-create side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("The %s %d will be attached to firewall %d.", deviceType, deviceID, firewallID))

	return details, nil
}

// instanceDiskCreateSideEffects is the Tier B preview for
// linode_instance_disk_create. It names the disk and its size (arg-only; the
// fetched instance state is unused here).
func instanceDiskCreateSideEffects(ctx context.Context, label string, size, linodeID int) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("instance-disk-create side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("A new %d MB disk %q will be created on instance %d.", size, label, linodeID))

	return details, nil
}

// instanceConfigCreateSideEffects is the Tier B preview for
// linode_instance_config_create. It names the configuration profile being
// added (arg-only; the fetched instance state is unused here).
func instanceConfigCreateSideEffects(ctx context.Context, label string, linodeID int) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("instance-config-create side-effect walk canceled: %w", err)
	}

	effect := fmt.Sprintf("A new configuration profile will be created on instance %d", linodeID)
	if label != "" {
		effect += fmt.Sprintf(" labeled %q", label)
	}

	details.SideEffects = append(details.SideEffects, effect+".")

	return details, nil
}

// placementGroupCreateSideEffects is the Tier B preview for
// linode_placement_group_create. It names the group, its type/policy, and the
// region (arg-only, no fetch).
func placementGroupCreateSideEffects(ctx context.Context, label, region, pgType, policy string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("placement-group-create side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("A new placement group %q (%s, %s policy) will be created in region %s.",
			label, pgType, policy, region))

	return details, nil
}

// placementGroupUpdateSideEffects is the Tier B preview for
// linode_placement_group_update. The update tool only changes the label
// (arg-only, no fetch).
func placementGroupUpdateSideEffects(ctx context.Context, label string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("placement-group-update side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("The placement group's label is set to %q.", label))

	return details, nil
}

// placementGroupMembershipSideEffects is the Tier B preview shared by
// linode_placement_group_assign and linode_placement_group_unassign. It names
// the Linodes whose membership changes; action is "assigned to" or "removed
// from" (arg-only, no fetch).
func placementGroupMembershipSideEffects(ctx context.Context, linodes []int, groupID int, action string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("placement-group membership side-effect walk canceled: %w", err)
	}

	for _, linodeID := range linodes {
		details.SideEffects = append(details.SideEffects,
			fmt.Sprintf("Linode %d will be %s placement group %d.", linodeID, action, groupID))
	}

	return details, nil
}

// profileTokenCreateSideEffects is the Tier B preview for
// linode_profile_token_create. It names the token and warns that the secret is
// shown only once (credential-sensitive; arg-only, no fetch).
func profileTokenCreateSideEffects(ctx context.Context, label string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("profile-token-create side-effect walk canceled: %w", err)
	}

	effect := "A new personal access token will be created."
	if label != "" {
		effect = fmt.Sprintf("A new personal access token %q will be created.", label)
	}

	details.SideEffects = append(details.SideEffects, effect)
	details.Warnings = append(details.Warnings, "The token secret is returned only once, at creation time.")

	return details, nil
}

// profileTFADisableSideEffects is the Tier B preview for
// linode_profile_tfa_disable. Disabling two-factor auth is a security
// downgrade, so it carries a warning (arg-only, no fetch).
func profileTFADisableSideEffects(ctx context.Context) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("profile-tfa-disable side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects, "Two-factor authentication is disabled for this profile.")
	details.Warnings = append(details.Warnings, "Disabling two-factor authentication reduces account security.")

	return details, nil
}

// profileTFAEnableSideEffects is the Tier B preview for
// linode_profile_tfa_enable. It generates a 2FA secret that must still be
// confirmed before 2FA is active (arg-only, no fetch).
func profileTFAEnableSideEffects(ctx context.Context) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("profile-tfa-enable side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		"A new two-factor authentication secret is generated; it must be confirmed before two-factor authentication becomes active.")

	return details, nil
}

// profileTFAEnableConfirmSideEffects is the Tier B preview for
// linode_profile_tfa_enable_confirm. Confirming the secret turns 2FA on
// (arg-only, no fetch).
func profileTFAEnableConfirmSideEffects(ctx context.Context) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("profile-tfa-enable-confirm side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects, "Two-factor authentication is enabled for this profile.")

	return details, nil
}

// profileTokenUpdateSideEffects is the Tier B preview for
// linode_profile_token_update. The update tool changes the token's label
// (arg-only, no fetch).
func profileTokenUpdateSideEffects(ctx context.Context, label string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("profile-token-update side-effect walk canceled: %w", err)
	}

	effect := "The personal access token is updated."
	if label != "" {
		effect = fmt.Sprintf("The personal access token's label is set to %q.", label)
	}

	details.SideEffects = append(details.SideEffects, effect)

	return details, nil
}

// tagCreateSideEffects is the Tier B preview for linode_tag_create (arg-only).
func tagCreateSideEffects(ctx context.Context, label string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("tag-create side-effect walk canceled: %w", err)
	}

	effect := "A new tag will be created."
	if label != "" {
		effect = fmt.Sprintf("A new tag %q will be created.", label)
	}

	details.SideEffects = append(details.SideEffects, effect)

	return details, nil
}

// supportTicketCreateSideEffects is the Tier B preview for
// linode_support_ticket_create (arg-only).
func supportTicketCreateSideEffects(ctx context.Context, summary string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("support-ticket-create side-effect walk canceled: %w", err)
	}

	effect := "A new support ticket will be opened."
	if summary != "" {
		effect = fmt.Sprintf("A new support ticket %q will be opened.", summary)
	}

	details.SideEffects = append(details.SideEffects, effect)

	return details, nil
}

// supportTicketReplyCreateSideEffects is the Tier B preview for
// linode_support_ticket_reply_create (arg-only).
func supportTicketReplyCreateSideEffects(ctx context.Context, ticketID int) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("support-ticket-reply-create side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("A reply will be posted to support ticket %d.", ticketID))

	return details, nil
}

// supportTicketAttachmentCreateSideEffects is the Tier B preview for
// linode_support_ticket_attachment_create (arg-only).
func supportTicketAttachmentCreateSideEffects(ctx context.Context, ticketID int) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("support-ticket-attachment-create side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("A file attachment will be uploaded to support ticket %d.", ticketID))

	return details, nil
}

// supportTicketCloseSideEffects is the Tier B preview for
// linode_support_ticket_close (arg-only).
func supportTicketCloseSideEffects(ctx context.Context, ticketID int) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("support-ticket-close side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("Support ticket %d will be closed.", ticketID))

	return details, nil
}

// profilePreferencesUpdateSideEffects is the Tier B preview for
// linode_profile_preferences_update (arg-only).
func profilePreferencesUpdateSideEffects(ctx context.Context) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("profile-preferences-update side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		"The OAuth client's profile preferences are replaced with the supplied values.")

	return details, nil
}

// profileSecurityQuestionsAnswerSideEffects is the Tier B preview for
// linode_profile_security_question_answer. The answers are security material,
// so the side effect describes the action without echoing them (arg-only).
func profileSecurityQuestionsAnswerSideEffects(ctx context.Context) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("profile-security-questions-answer side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		"The profile's security question answers are saved.")

	return details, nil
}

// accountAgreementsAcknowledgeSideEffects is the Tier B preview for
// linode_account_agreement_acknowledge (arg-only).
func accountAgreementsAcknowledgeSideEffects(ctx context.Context) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("account-agreements-acknowledge side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		"The selected account agreements are acknowledged for this account.")

	return details, nil
}

// accountBetaEnrollSideEffects is the Tier B preview for
// linode_account_beta_enroll (arg-only).
func accountBetaEnrollSideEffects(ctx context.Context, betaID string) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("account-beta-enroll side-effect walk canceled: %w", err)
	}

	effect := "The account is enrolled in the requested beta program."
	if betaID != "" {
		effect = fmt.Sprintf("The account is enrolled in beta program %q.", betaID)
	}

	details.SideEffects = append(details.SideEffects, effect)

	return details, nil
}

// accountCancelSideEffects is the Tier B preview for linode_account_cancel.
// Cancellation is the most destructive account-level action, so the preview
// carries a hard irreversibility warning (arg-only).
func accountCancelSideEffects(ctx context.Context) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("account-cancel side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		"The Linode account is closed and all of its resources are removed.")
	details.Warnings = append(details.Warnings,
		"Account cancellation is permanent and irreversible; every resource on the account is destroyed and access is lost.")

	return details, nil
}

// accountChildAccountTokenCreateSideEffects is the Tier B preview for
// linode_account_child_account_token_create. The proxy token itself is never
// surfaced; the side effect only states that a token is minted (arg-only).
func accountChildAccountTokenCreateSideEffects(ctx context.Context) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("account-child-account-token-create side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		"A proxy user token is created for the selected child account.")

	return details, nil
}

// accountEventSeenSideEffects is the Tier B preview for
// linode_account_event_seen. The Linode API marks the given event and every
// earlier event as seen, so the preview surfaces that wider effect (arg-only).
func accountEventSeenSideEffects(ctx context.Context) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("account-event-seen side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		"The specified account event and all earlier events are marked as seen.")

	return details, nil
}

// profilePhoneNumberSendSideEffects is the Tier B preview for
// linode_profile_phone_number_send. The phone number is PII, so the side
// effect avoids echoing it (arg-only).
func profilePhoneNumberSendSideEffects(ctx context.Context) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("profile-phone-number-send side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		"A verification code is sent to the supplied phone number.")

	return details, nil
}

// profilePhoneNumberVerifySideEffects is the Tier B preview for
// linode_profile_phone_number_verify (arg-only).
func profilePhoneNumberVerifySideEffects(ctx context.Context) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("profile-phone-number-verify side-effect walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		"The phone number is verified and added to the profile.")

	return details, nil
}

// tagDeleteDependencyWalk is the Tier A walk for linode_tag_delete. Deleting a
// tag removes it from every object that carries it; those objects are not
// deleted, so each is surfaced as a "removed" dependency. State-only: the
// tagged objects come from the destroy FetchState (ListTaggedObjects).
func tagDeleteDependencyWalk(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("tag-delete dependency walk canceled: %w", err)
	}

	resp, ok := state.(*linode.PaginatedResponse[linode.TaggedObject])
	if !ok || resp == nil {
		return details, nil
	}

	for i := range resp.Data {
		object := resp.Data[i]

		kind, _ := object["type"].(string)
		if kind == "" {
			kind = "resource"
		}

		dependency := DryRunDependency{
			Kind:   kind,
			Action: dependencyActionRemoved,
			Note:   "Loses this tag; the resource itself is not deleted.",
		}

		if data, dataOK := object["data"].(map[string]any); dataOK {
			dependency.ID = data["id"]
		}

		details.Dependencies = append(details.Dependencies, dependency)
	}

	if len(resp.Data) > 0 {
		details.Warnings = append(details.Warnings, fmt.Sprintf(
			"Deleting this tag removes it from %d tagged object(s); the objects are not deleted.", len(resp.Data),
		))
	}

	return details, nil
}

// placementGroupDeleteDependencyWalk is the Tier A walk for
// linode_placement_group_delete. Deleting a placement group detaches its
// member Linodes; the instances themselves are not deleted, so each member is
// surfaced as a detached dependency. State-only: members come from the destroy
// FetchState, no extra GET.
func placementGroupDeleteDependencyWalk(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("placement-group dependency walk canceled: %w", err)
	}

	group, ok := state.(*linode.PlacementGroup)
	if !ok || group == nil {
		return details, nil
	}

	for i := range group.Members {
		details.Dependencies = append(details.Dependencies, DryRunDependency{
			Kind:   dependencyKindInstance,
			ID:     group.Members[i].LinodeID,
			Action: dependencyActionDetached,
			Note:   "Linode is removed from the placement group; the instance is not deleted.",
		})
	}

	if len(group.Members) > 0 {
		details.Warnings = append(details.Warnings, fmt.Sprintf(
			"Deleting this placement group detaches %d Linode(s); the instances are not deleted.", len(group.Members),
		))
	}

	return details, nil
}

// volumeDeleteDependencyWalk is the Tier A walk for linode_volume_delete. The
// only cross-resource dependency is the instance the volume is attached to:
// an attached volume detaches from its instance before it is destroyed. The
// attachment is already on the volume state from the destroy FetchState; the
// walk only calls the API to resolve the instance label when the volume
// record did not carry one.
func volumeDeleteDependencyWalk(ctx context.Context, client *linode.Client, _ int, state any) (DryRunDetails, error) {
	var details DryRunDetails

	volume, ok := state.(*linode.Volume)
	if !ok || volume == nil || volume.LinodeID == nil || *volume.LinodeID == 0 {
		return details, nil
	}

	linodeID := *volume.LinodeID

	var label string
	if volume.LinodeLabel != nil {
		label = *volume.LinodeLabel
	}

	if label == "" {
		if instance, err := client.GetInstance(ctx, linodeID); err == nil && instance != nil {
			label = instance.Label
		}
	}

	details.Dependencies = append(details.Dependencies, DryRunDependency{
		Kind:   dependencyKindInstance,
		ID:     linodeID,
		Label:  label,
		Action: dependencyActionDetached,
		Note:   "Volume is attached; it detaches from this instance before deletion.",
	})
	details.Warnings = append(details.Warnings,
		"Volume is currently attached to an instance; it will be detached as part of deletion.")

	return details, nil
}

// lkeClusterDeleteDependencyWalk is the Tier A walk for
// linode_lke_cluster_delete. Deleting a cluster cascades to its node pools
// and their nodes, so each pool is surfaced as a cascade_deleted dependency
// and a warning summarizes the total node count (running workloads are lost).
// Best-effort: a failed pool list becomes a warning, not a hard error.
func lkeClusterDeleteDependencyWalk(ctx context.Context, client *linode.Client, clusterID int, _ any) (DryRunDetails, error) {
	var details DryRunDetails

	pools, err := client.ListLKENodePools(ctx, clusterID)
	if err != nil {
		details.Warnings = append(details.Warnings, fmt.Sprintf("Could not list node pools: %v", err))

		return details, nil
	}

	var totalNodes int

	for i := range pools {
		pool := &pools[i]
		totalNodes += pool.Count
		details.Dependencies = append(details.Dependencies, DryRunDependency{
			Kind:   "node_pool",
			ID:     pool.ID,
			Action: dependencyActionCascadeDeleted,
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

// lkePoolDeleteDependencyWalk is the Tier A walk for linode_lke_pool_delete.
// The pool state (from FetchState) carries its nodes; each node's backing
// Linode is destroyed with the pool, so nodes are surfaced as cascade_deleted
// dependencies and a warning summarizes the node count (workloads are lost).
func lkePoolDeleteDependencyWalk(ctx context.Context, _ *linode.Client, _, _ int, state any) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("pool dependency walk canceled: %w", err)
	}

	pool, ok := state.(*linode.LKENodePool)
	if !ok || pool == nil {
		return details, nil
	}

	for i := range pool.Nodes {
		node := &pool.Nodes[i]
		details.Dependencies = append(details.Dependencies, DryRunDependency{
			Kind:   dependencyKindInstance,
			ID:     node.InstanceID,
			Label:  node.ID,
			Action: dependencyActionCascadeDeleted,
			Note:   "Backing Linode for this pool node.",
		})
	}

	if pool.Count > 0 {
		details.Warnings = append(details.Warnings, fmt.Sprintf(
			"Deleting this pool destroys %d node(s) and their backing Linodes; running workloads are lost.", pool.Count,
		))
	}

	return details, nil
}

// lkeNodeDeleteDependencyWalk is the Tier A walk for linode_lke_node_delete.
// The node state (from FetchState) names the backing Linode, which is
// destroyed with the node, so it is surfaced as a cascade_deleted dependency.
func lkeNodeDeleteDependencyWalk(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
	var details DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("node dependency walk canceled: %w", err)
	}

	node, ok := state.(*linode.LKENode)
	if !ok || node == nil {
		return details, nil
	}

	if node.InstanceID > 0 {
		details.Dependencies = append(details.Dependencies, DryRunDependency{
			Kind:   dependencyKindInstance,
			ID:     node.InstanceID,
			Label:  node.ID,
			Action: dependencyActionCascadeDeleted,
			Note:   "Backing Linode for this node.",
		})
	}

	details.Warnings = append(details.Warnings,
		"Deleting this node removes it from its pool; the pool node count drops by one and scheduled workloads reschedule.")

	return details, nil
}

// firewallDeleteDependencyWalk is the Tier A walk for linode_firewall_delete.
// The resources attached to a firewall (Linodes, NodeBalancers) survive the
// delete but lose this firewall's rules, so each attached device is surfaced
// as a removed dependency. Best-effort: a failed device list becomes a
// warning, not a hard error.
func firewallDeleteDependencyWalk(ctx context.Context, client *linode.Client, firewallID int, _ any) (DryRunDetails, error) {
	var details DryRunDetails

	devices, err := client.ListFirewallDevices(ctx, firewallID, 1, dependencyWalkPageSize)
	if err != nil {
		details.Warnings = append(details.Warnings, fmt.Sprintf("Could not list firewall devices: %v", err))

		return details, nil
	}

	if devices == nil {
		return details, nil
	}

	for i := range devices.Data {
		entity := &devices.Data[i].Entity
		details.Dependencies = append(details.Dependencies, DryRunDependency{
			Kind:   entity.Type,
			ID:     entity.ID,
			Label:  entity.Label,
			Action: dependencyActionRemoved,
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

// domainDeleteDependencyWalk is the Tier A walk for linode_domain_delete.
// Deleting a domain destroys all its DNS records; the walk surfaces the NS
// records (the delegation that breaks) and warns with the total record count.
// Best-effort: a failed record list becomes a warning, not a hard error.
func domainDeleteDependencyWalk(ctx context.Context, client *linode.Client, domainID int, _ any) (DryRunDetails, error) {
	var details DryRunDetails

	records, err := client.ListDomainRecords(ctx, domainID)
	if err != nil {
		details.Warnings = append(details.Warnings, fmt.Sprintf("Could not list domain records: %v", err))

		return details, nil
	}

	var nsCount int

	for i := range records {
		record := &records[i]
		if !strings.EqualFold(record.Type, "NS") {
			continue
		}

		nsCount++

		details.Dependencies = append(details.Dependencies, DryRunDependency{
			Kind:   "ns_record",
			Label:  record.Target,
			Action: dependencyActionCascadeDeleted,
			Note:   "NS record for " + record.Name,
		})
	}

	if len(records) > 0 {
		details.Warnings = append(details.Warnings, fmt.Sprintf(
			"Deleting this domain destroys %d DNS record(s), including %d NS record(s).", len(records), nsCount,
		))
	}

	return details, nil
}

// nodebalancerDeleteDependencyWalk is the Tier A walk for
// linode_nodebalancer_delete. Each config (and its backend node list) is
// destroyed with the NodeBalancer, so configs are surfaced as cascade_deleted
// dependencies. Best-effort: a failed config list becomes a warning.
func nodebalancerDeleteDependencyWalk(ctx context.Context, client *linode.Client, nodeBalancerID int, _ any) (DryRunDetails, error) {
	var details DryRunDetails

	configs, err := client.ListNodeBalancerConfigs(ctx, nodeBalancerID, 0, 0)
	if err != nil {
		details.Warnings = append(details.Warnings, fmt.Sprintf("Could not list NodeBalancer configs: %v", err))

		return details, nil
	}

	for i := range configs {
		config := &configs[i]
		details.Dependencies = append(details.Dependencies, DryRunDependency{
			Kind:   "nodebalancer_config",
			ID:     config.ID,
			Action: dependencyActionCascadeDeleted,
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

// nodebalancerConfigDeleteDependencyWalk is the Tier A walk for
// linode_nodebalancer_config_delete. Deleting a config destroys its backend
// node list, so each node is surfaced as a cascade_deleted dependency.
// Best-effort: a failed node list becomes a warning.
func nodebalancerConfigDeleteDependencyWalk(ctx context.Context, client *linode.Client, nodeBalancerID, configID int) DryRunDetails {
	var details DryRunDetails

	nodes, err := client.ListNodeBalancerConfigNodes(ctx, nodeBalancerID, configID, 1, dependencyWalkPageSize)
	if err != nil {
		details.Warnings = append(details.Warnings, fmt.Sprintf("Could not list config backend nodes: %v", err))

		return details
	}

	if nodes == nil {
		return details
	}

	for i := range nodes.Data {
		node := &nodes.Data[i]
		details.Dependencies = append(details.Dependencies, DryRunDependency{
			Kind:   "nodebalancer_node",
			ID:     node.ID,
			Label:  node.Label,
			Action: dependencyActionCascadeDeleted,
			Note:   fmt.Sprintf("backend %s (%s)", node.Address, node.Mode),
		})
	}

	if len(nodes.Data) > 0 {
		details.Warnings = append(details.Warnings, fmt.Sprintf(
			"Deleting this config removes %d backend node(s) from the rotation.", len(nodes.Data),
		))
	}

	return details
}

// vpcDeleteDependencyWalk is the Tier A walk for linode_vpc_delete. Each
// subnet is destroyed with the VPC, and any Linode interfaces in a subnet are
// detached, so subnets are surfaced as cascade_deleted dependencies with their
// attached-interface count. Best-effort: a failed subnet list becomes a warning.
func vpcDeleteDependencyWalk(ctx context.Context, client *linode.Client, vpcID int, _ any) (DryRunDetails, error) {
	var details DryRunDetails

	subnets, err := client.ListVPCSubnets(ctx, vpcID)
	if err != nil {
		details.Warnings = append(details.Warnings, fmt.Sprintf("Could not list VPC subnets: %v", err))

		return details, nil
	}

	var attachedInterfaces int

	for i := range subnets {
		subnet := &subnets[i]
		attachedInterfaces += len(subnet.Linodes)
		details.Dependencies = append(details.Dependencies, DryRunDependency{
			Kind:   "vpc_subnet",
			ID:     subnet.ID,
			Label:  subnet.Label,
			Action: dependencyActionCascadeDeleted,
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

// vpcSubnetDeleteDependencyWalk is the Tier A walk for
// linode_vpc_subnet_delete. The subnet state (from FetchState) carries the
// Linodes with interfaces in this subnet; each is surfaced as a detached
// dependency. The parent VPC is fetched once to label the warning.
func vpcSubnetDeleteDependencyWalk(ctx context.Context, client *linode.Client, vpcID, _ int, state any) (DryRunDetails, error) {
	var details DryRunDetails

	subnet, ok := state.(*linode.VPCSubnet)
	if !ok || subnet == nil {
		return details, nil
	}

	for i := range subnet.Linodes {
		linodeRef := &subnet.Linodes[i]
		details.Dependencies = append(details.Dependencies, DryRunDependency{
			Kind:   dependencyKindInstance,
			ID:     linodeRef.ID,
			Action: dependencyActionDetached,
			Note:   fmt.Sprintf("%d interface(s) in this subnet are detached.", len(linodeRef.Interfaces)),
		})
	}

	if len(subnet.Linodes) == 0 {
		return details, nil
	}

	var vpcLabel string
	if vpc, err := client.GetVPC(ctx, vpcID); err == nil && vpc != nil {
		vpcLabel = vpc.Label
	}

	details.Warnings = append(details.Warnings, fmt.Sprintf(
		"%d Linode(s) have interfaces in subnet %q (VPC %q) and will be detached.",
		len(subnet.Linodes), subnet.Label, vpcLabel,
	))

	return details, nil
}

// configReferencesDisk reports whether any device slot of the config points
// at the given disk ID.
func configReferencesDisk(config *linode.InstanceConfig, diskID int) bool {
	for _, device := range config.Devices {
		if device != nil && device.DiskID != nil && *device.DiskID == diskID {
			return true
		}
	}

	return false
}

// instanceDiskDeleteDependencyWalk is the Tier A walk for
// linode_instance_disk_delete. It lists the instance's config profiles and
// surfaces those that reference the disk, since deleting the disk leaves the
// referencing config's slot empty. Best-effort: a failed config list becomes
// a warning, not a hard error.
func instanceDiskDeleteDependencyWalk(ctx context.Context, client *linode.Client, linodeID, diskID int, _ any) (DryRunDetails, error) {
	var details DryRunDetails

	configs, err := client.ListInstanceConfigs(ctx, linodeID, 1, dependencyWalkPageSize)
	if err != nil {
		details.Warnings = append(details.Warnings, fmt.Sprintf("Could not list instance configs: %v", err))

		return details, nil
	}

	for i := range configs {
		config := &configs[i]
		if !configReferencesDisk(config, diskID) {
			continue
		}

		details.Dependencies = append(details.Dependencies, DryRunDependency{
			Kind:   "instance_config",
			ID:     config.ID,
			Label:  config.Label,
			Action: dependencyActionRemoved,
			Note:   "References this disk; its device slot is cleared when the disk is deleted.",
		})
	}

	if len(details.Dependencies) > 0 {
		details.Warnings = append(details.Warnings,
			"Config profiles reference this disk; deleting it leaves those slots empty.")
	}

	return details, nil
}
