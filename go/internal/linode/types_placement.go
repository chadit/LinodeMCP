package linode

// PlacementGroup represents a Linode placement group.
type PlacementGroup struct {
	ID                   int                       `json:"id"`
	Label                string                    `json:"label"`
	Region               string                    `json:"region"`
	PlacementGroupType   string                    `json:"placement_group_type"`
	PlacementGroupPolicy string                    `json:"placement_group_policy"`
	IsCompliant          bool                      `json:"is_compliant"`
	Members              []PlacementGroupMember    `json:"members,omitempty"`
	Migrations           *PlacementGroupMigrations `json:"migrations,omitempty"`
}

// PlacementGroupMember represents one Linode instance assigned to a placement group.
type PlacementGroupMember struct {
	LinodeID    int  `json:"linode_id"`
	IsCompliant bool `json:"is_compliant"`
}

// PlacementGroupMigrations represents placement group migration state.
type PlacementGroupMigrations struct {
	Inbound  []PlacementGroupMigration `json:"inbound"`
	Outbound []PlacementGroupMigration `json:"outbound"`
}

// PlacementGroupMigration represents a compute instance migrating into or out of a placement group.
type PlacementGroupMigration struct {
	LinodeID int `json:"linode_id"`
}
