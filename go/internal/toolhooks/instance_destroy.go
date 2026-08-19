package toolhooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// runningDeleteWarning is the one sentence the delete walk adds that no
// dependency list carries: the API does not pause for a graceful shutdown.
const runningDeleteWarning = "Instance is currently running. Delete will not pause for a graceful shutdown."

// LinodeInstanceDiskDeleteDependencyWalk names the configuration profiles whose
// device slots point at the disk, since deleting it leaves those slots empty.
// Best-effort, so a failed config list becomes a warning rather than refusing
// the preview.
func LinodeInstanceDiskDeleteDependencyWalk(
	ctx context.Context, client *linode.Client, linodeID, diskID int, _ any,
) (tools.DryRunDetails, error) {
	var details tools.DryRunDetails

	configs, err := client.ListInstanceConfigs(ctx, linodeID, 1, tools.DependencyWalkPageSize)
	if err != nil {
		details.Warnings = append(details.Warnings, fmt.Sprintf("Could not list instance configs: %v", err))

		return details, nil
	}

	for i := range configs {
		config := &configs[i]
		if !configReferencesDisk(config, diskID) {
			continue
		}

		details.Dependencies = append(details.Dependencies, tools.DryRunDependency{
			Kind:   "instance_config",
			ID:     config.ID,
			Label:  config.Label,
			Action: tools.DependencyActionRemoved,
			Note:   "References this disk; its device slot is cleared when the disk is deleted.",
		})
	}

	if len(details.Dependencies) > 0 {
		details.Warnings = append(details.Warnings,
			"Config profiles reference this disk; deleting it leaves those slots empty.")
	}

	return details, nil
}

// configReferencesDisk reports whether any device slot of the config points at
// the given disk id.
func configReferencesDisk(config *linode.InstanceConfig, diskID int) bool {
	for _, device := range config.Devices {
		if device != nil && device.DiskID != nil && *device.DiskID == diskID {
			return true
		}
	}

	return false
}

// LinodeInstancePasswordResetDependencyWalk names the downtime the reset costs.
// The API powers the Linode down and back up to apply the password, which no
// descriptor says, and a running instance loses service while it happens.
func LinodeInstancePasswordResetDependencyWalk(
	ctx context.Context, _ *linode.Client, _ int, state tools.DeclaredState,
) (tools.DryRunDetails, error) {
	var details tools.DryRunDetails

	if err := ctx.Err(); err != nil {
		return details, fmt.Errorf("password-reset dependency walk canceled: %w", err)
	}

	details.SideEffects = append(details.SideEffects,
		"The instance is powered down and rebooted to apply the new root password.")

	if strings.EqualFold(state.Text("status"), instanceStatusRunning) {
		details.Warnings = append(details.Warnings,
			"Instance is currently running; the reset shuts it down and reboots it, causing downtime.")
	}

	return details, nil
}

// LinodeInstanceDeleteDependencyWalk names what a delete takes with it: volumes
// detach and keep billing, public addresses go back to the pool, firewall
// attachments drop, and the instance's own billing stops.
//
// Each sub-list is best-effort. A failed fetch becomes a warning rather than
// failing the preview, so a partial dependency picture still reaches the caller
// instead of nothing at all.
func LinodeInstanceDeleteDependencyWalk(
	ctx context.Context, client *linode.Client, linodeID int, state tools.DeclaredState,
) (tools.DryRunDetails, error) {
	var details tools.DryRunDetails

	categories := []func(context.Context, *linode.Client, int) ([]tools.DryRunDependency, string){
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

	details.BillingDelta = instanceBillingDelta(ctx, client, state.Text("type"))

	if strings.EqualFold(state.Text("status"), instanceStatusRunning) {
		details.Warnings = append(details.Warnings, runningDeleteWarning)
	}

	return details, nil
}

// instanceBillingDelta estimates the monthly cost change from deleting an
// instance of the given type, reporting the unknown sentinel when the type is
// empty or its pricing cannot be fetched.
func instanceBillingDelta(ctx context.Context, client *linode.Client, typeID string) *tools.DryRunBillingDelta {
	if typeID == "" {
		return &tools.DryRunBillingDelta{MonthlyChangeUSD: tools.BillingUnknown}
	}

	instanceType, err := client.GetType(ctx, typeID)
	if err != nil || instanceType == nil {
		return &tools.DryRunBillingDelta{
			MonthlyChangeUSD: tools.BillingUnknown,
			Note:             "Could not fetch type pricing for the estimate.",
		}
	}

	return &tools.DryRunBillingDelta{
		MonthlyChangeUSD: fmt.Sprintf("-%.2f", instanceType.Price.Monthly),
		Note:             "Instance billing stops. Attached volume billing continues.",
	}
}

// instanceVolumeDeps lists volumes attached to the instance. They survive the
// delete (detach, not destroy), so their billing continues.
func instanceVolumeDeps(
	ctx context.Context, client *linode.Client, linodeID int,
) ([]tools.DryRunDependency, string) {
	volumes, err := client.ListInstanceVolumes(ctx, linodeID, 1, tools.DependencyWalkPageSize)
	if err != nil {
		return nil, fmt.Sprintf("Could not list attached volumes: %v", err)
	}

	deps := make([]tools.DryRunDependency, 0, len(volumes))

	for i := range volumes {
		volume := &volumes[i]
		deps = append(deps, tools.DryRunDependency{
			Kind:   "volume",
			ID:     volume.ID,
			Label:  volume.Label,
			Action: tools.DependencyActionDetached,
			Note:   fmt.Sprintf("%dGB volume stays; billing continues.", volume.Size),
		})
	}

	return deps, ""
}

// instanceIPDeps lists the instance's public IPv4 addresses, which are released
// back to the pool when the instance is deleted.
func instanceIPDeps(
	ctx context.Context, client *linode.Client, linodeID int,
) ([]tools.DryRunDependency, string) {
	ips, err := client.ListInstanceIPs(ctx, linodeID)
	if err != nil {
		return nil, fmt.Sprintf("Could not list IP addresses: %v", err)
	}

	if ips == nil || ips.IPv4 == nil {
		return nil, ""
	}

	deps := make([]tools.DryRunDependency, 0, len(ips.IPv4.Public))

	for i := range ips.IPv4.Public {
		addr := &ips.IPv4.Public[i]
		deps = append(deps, tools.DryRunDependency{
			Kind:   "public_ip",
			Label:  addr.Address,
			Action: tools.DependencyActionReleased,
		})
	}

	return deps, ""
}

// instanceFirewallDeps lists firewalls this instance is attached to. The
// firewalls survive; the instance is dropped from each firewall's device list.
func instanceFirewallDeps(
	ctx context.Context, client *linode.Client, linodeID int,
) ([]tools.DryRunDependency, string) {
	firewalls, err := client.ListInstanceFirewalls(ctx, linodeID, 1, tools.DependencyWalkPageSize)
	if err != nil {
		return nil, fmt.Sprintf("Could not list firewalls: %v", err)
	}

	deps := make([]tools.DryRunDependency, 0, len(firewalls))

	for i := range firewalls {
		firewall := &firewalls[i]
		deps = append(deps, tools.DryRunDependency{
			Kind:   "firewall",
			ID:     firewall.ID,
			Label:  firewall.Label,
			Action: tools.DependencyActionRemoved,
			Note:   "Firewall stays; this instance is removed from its device list.",
		})
	}

	return deps, ""
}
