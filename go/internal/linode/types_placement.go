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

// CreatePlacementGroupRequest represents the request body for creating a placement group.
type CreatePlacementGroupRequest struct {
	Label                string `json:"label"`
	Region               string `json:"region"`
	PlacementGroupType   string `json:"placement_group_type"`
	PlacementGroupPolicy string `json:"placement_group_policy"`
}

// UpdatePlacementGroupRequest contains editable fields for PUT /placement/groups/{group_id}.
type UpdatePlacementGroupRequest struct {
	Label string `json:"label,omitempty"`
}

// AssignPlacementGroupLinodesRequest represents the request body for assigning Linodes to a placement group.
type AssignPlacementGroupLinodesRequest struct {
	Linodes []int `json:"linodes"`
}

// PlacementGroupUnassignRequest represents the request body for unassigning Linodes from a placement group.
type PlacementGroupUnassignRequest struct {
	Linodes []int `json:"linodes,omitempty"`
}
