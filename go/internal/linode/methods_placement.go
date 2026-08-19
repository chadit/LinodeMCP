package linode

import (
	"context"
)

// GetPlacementGroup retrieves a single placement group by ID.
func (c *Client) httpGetPlacementGroup(ctx context.Context, groupID int) (*PlacementGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_placement_group_get", nil, groupID)
	if err != nil {
		return nil, wrapRequestError("GetPlacementGroup", err)
	}

	defer drainClose(resp)

	var group PlacementGroup
	if err := c.handleResponse(resp, &group); err != nil {
		return nil, err
	}

	return &group, nil
}
