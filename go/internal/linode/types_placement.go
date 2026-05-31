package linode

// PlacementGroup represents a Linode placement group.
type PlacementGroup struct {
	ID                   int                    `json:"id"`
	Label                string                 `json:"label"`
	Region               string                 `json:"region"`
	PlacementGroupType   string                 `json:"placement_group_type"`
	PlacementGroupPolicy string                 `json:"placement_group_policy"`
	IsCompliant          bool                   `json:"is_compliant"`
	Members              []PlacementGroupMember `json:"members,omitempty"`
}

// PlacementGroupMember represents one Linode instance assigned to a placement group.
type PlacementGroupMember struct {
	LinodeID    int  `json:"linode_id"`
	IsCompliant bool `json:"is_compliant"`
}
