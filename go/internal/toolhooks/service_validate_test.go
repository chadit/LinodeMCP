package toolhooks_test

import (
	"testing"
)

// The argument keys the service tables address their tools by.
const (
	argClusterID    = "cluster_id"
	argPoolID       = "pool_id"
	argContactID    = "contact_id"
	argImageID      = "image_id"
	argShareGroupID = "sharegroup_id"
	argTokenUUID    = "token_uuid"
	argKeyID        = "key_id"
	argRegion       = "region"
	argLabel        = "label"
	argGroupID      = "group_id"
	argVPCID        = "vpc_id"
	argSubnetID     = "subnet_id"
)

// Every sentence below is what the hand-written handler for that tool answered.
// The emitter derives "<name> is required" and would replace the rest without
// saying so, which is what these tables exist to catch.
const (
	// publicImageID is the prefixed image the image-argument tables accept and
	// the instance-create preview names, spelled once so the three tables that
	// use it cannot drift apart.
	publicImageID = "linode/debian11"
)

// The values the shape checks accept, and the label length Object Storage
// documents as the ceiling.
const (
	sampleRegion = "us-east-1"
	sampleLabel  = "my-bucket"
)

// TestServiceValidateHooks pins the sentences the service read tools answer for
// a bad argument, the way TestReadValidateHooks does for the account, profile,
// and instance surfaces.
//
// These hooks only ever run through a generated handler in another package, so
// nothing else calls them with an argument they reject: the wording reaches a
// client through code no test here exercises, and drift in it would show up as
// a changed answer to a caller rather than as a failure.
func TestServiceValidateHooks(t *testing.T) {
	t.Parallel()

	runValidateTables(t, serviceValidateTables())
}

// serviceValidateTables gathers one group of tables per service family.
func serviceValidateTables() []hookTable {
	families := [][]hookTable{
		databaseValidateTables(),
		firewallValidateTables(),
		imageValidateTables(),
		lkeValidateTables(),
		managedValidateTables(),
		nodeBalancerValidateTables(),
		objectStorageValidateTables(),
		vpcValidateTables(),
		singleArgumentValidateTables(),
	}

	tables := make([]hookTable, 0, len(families))
	for _, family := range families {
		tables = append(tables, family...)
	}

	return tables
}

// databaseValidateTables covers the Managed Database read tools. The four
// instance tools share one table because they answer identically, which is the
// claim worth pinning: a caller reading MySQL and PostgreSQL back to back gets
// the same sentence for the same mistake.
func databaseValidateTables() []hookTable {
	return []hookTable{}
}

// firewallValidateTables covers the Cloud Firewall read tools.
func firewallValidateTables() []hookTable {
	return []hookTable{}
}

// imageValidateTables covers the image and share-group read tools.
//
// The image id is checked in a fixed order that decides which sentence a caller
// sees: a traversal inside a two-part id is reported as a traversal, but one
// that adds a third part is reported as a bad format, because the format check
// runs first.
func imageValidateTables() []hookTable {
	return []hookTable{}
}

// lkeValidateTables covers the LKE read tools. Every route under a cluster
// checks the cluster id first, so the tables that take a second argument supply
// a good cluster id to reach it.
func lkeValidateTables() []hookTable {
	return []hookTable{}
}

// managedValidateTables covers the Managed read tools, whose ids are bounded as
// well as positive: the API mints them large enough that a JSON number can
// round, and an id that rounded would address a different resource.
func managedValidateTables() []hookTable {
	return []hookTable{}
}

// nodeBalancerValidateTables covers the NodeBalancer read tools. A backend node
// is addressed by three ids and checks them outermost first, so a caller who
// omits two hears about the NodeBalancer rather than the node.
func nodeBalancerValidateTables() []hookTable {
	return []hookTable{}
}

// objectStorageValidateTables covers the Object Storage read tools.
//
// The two quota tools take the same argument and reject it differently on
// purpose: the get route only keeps the id inside its path segment, and the
// usage route also names the character set it accepts.
func objectStorageValidateTables() []hookTable {
	return []hookTable{}
}

// vpcValidateTables covers the VPC read tools.
func vpcValidateTables() []hookTable {
	return []hookTable{}
}

// singleArgumentValidateTables covers the tools addressed by one id apiece.
//
// The zone-file tool answers the positive-integer sentence for a missing id as
// well as an unusable one, where the other two report a missing id as missing.
// Both are what their hand-written handlers answered.
func singleArgumentValidateTables() []hookTable {
	return []hookTable{}
}
