package twostage

// HashIgnoreFields returns the top-level field names stripped from a resource's
// state before it is hashed for drift detection. Cosmetic fields that change
// without a real mutation (server-side timestamps, telemetry) belong here so a
// plan does not refuse on drift the user never caused. Keyed by resource type;
// an unknown type returns nil and the whole state is hashed.
//
// This is the conservative fallback list drawn from the API response shapes: it
// strips obvious timestamp and telemetry fields. The bias is toward hashing
// more (over-detecting drift fails safe); entries are added reactively as
// false-positive drift reports come in.
func HashIgnoreFields(resourceType string) []string {
	return hashIgnoreByType()[resourceType]
}

// fieldUpdated is the server-side modification timestamp present on most Linode
// resources; it bumps without a user-caused change, so it is stripped before
// hashing for every resource type below.
const fieldUpdated = "updated"

// fieldCreated is the server-side creation timestamp; some list-derived states
// re-serialize it inconsistently, so it is stripped alongside fieldUpdated for
// the resource types that carry it.
const fieldCreated = "created"

func hashIgnoreByType() map[string][]string {
	return map[string][]string{
		// Instance.updated bumps on unrelated account activity; the watchdog and
		// last-seen telemetry move without a user mutation.
		"Instance": {fieldUpdated, "last_seen_ipv4", "watchdog_enabled", "host_uuid"},

		// Volume.updated bumps on attach/detach bookkeeping unrelated to delete.
		"Volume": {fieldUpdated, "last_seen_ipv4"},

		// LKE cluster status and timestamps churn as nodes recycle.
		"LKECluster": {fieldUpdated, fieldCreated},

		// Firewall.updated bumps when attached devices change state.
		"Firewall": {fieldUpdated},

		// NodeBalancer.transfer carries running bandwidth counters that move
		// continuously; updated bumps with them.
		"NodeBalancer": {fieldUpdated, "transfer"},

		// VPC.updated bumps as subnets and interfaces attach or detach.
		"VPC": {fieldUpdated},

		// Domain.updated bumps on record edits unrelated to deleting the zone.
		"Domain": {fieldUpdated},

		// StackScript.updated bumps on edits; deployments_total counts every
		// deploy by anyone, so it moves without the owner acting.
		"StackScript": {fieldUpdated, "deployments_total"},

		// Disk.updated bumps on imaging and resize bookkeeping.
		"Disk": {fieldUpdated},

		// VPC subnet timestamps move as interfaces attach and detach.
		"VPCSubnet": {fieldUpdated},

		// DNS record timestamps move on unrelated edits to the same record.
		"DomainRecord": {fieldUpdated},

		// LKE pool node list churns as nodes recycle; the pool itself is the
		// delete target, so the member nodes are telemetry, not user intent.
		"LKENodePool": {"nodes"},

		// DatabaseInstance.updated bumps on maintenance and config changes
		// unrelated to deleting the instance.
		"DatabaseInstance": {fieldUpdated},

		// ImageShareGroup.updated bumps as members and shared images change.
		"ImageShareGroup": {fieldUpdated},

		// A share-group token delete fetches its parent share group, whose
		// updated timestamp moves as membership churns.
		"ImageShareGroupToken": {fieldUpdated},

		// FirewallDevice.updated bumps when the attached entity changes state.
		"FirewallDevice": {fieldUpdated},

		// Kubeconfig and service-token deletes fetch the parent LKE cluster,
		// whose updated and created timestamps move as nodes recycle.
		"LKEKubeconfig":   {fieldUpdated, fieldCreated},
		"LKEServiceToken": {fieldUpdated, fieldCreated},
	}
}
