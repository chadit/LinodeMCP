package linode

import (
	"context"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// httpListPlacementGroupsProto retrieves placement groups as proto messages for
// the proto-backed list path. The page/page_size pair flows through the shared
// withPaginationQuery helper, so the request matches httpListPlacementGroups.
func (c *Client) httpListPlacementGroupsProto(ctx context.Context, page, pageSize int) ([]*linodev1.PlacementGroup, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListPlacementGroups",
		"linode_placement_group_list", "", nil, page, pageSize,
		func() *linodev1.PlacementGroup { return &linodev1.PlacementGroup{} })
}

// httpAssignPlacementGroupLinodesProto assigns Linodes to a placement group and
// decodes the response as a proto message.
func (c *Client) httpAssignPlacementGroupLinodesProto(ctx context.Context, groupID int, req *AssignPlacementGroupLinodesRequest) (*linodev1.PlacementGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_placement_group_assign", req, groupID)
	if err != nil {
		return nil, wrapRequestError("AssignPlacementGroupLinodes", err)
	}

	defer drainClose(resp)

	group := &linodev1.PlacementGroup{}
	if err := c.handleProtoResponse(resp, group); err != nil {
		return nil, err
	}

	return group, nil
}

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

// httpGetPlacementGroupProto retrieves one placement group as a proto message.
func (c *Client) httpGetPlacementGroupProto(ctx context.Context, groupID int) (*linodev1.PlacementGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_placement_group_get", nil, groupID)
	if err != nil {
		return nil, wrapRequestError("GetPlacementGroup", err)
	}

	defer drainClose(resp)

	group := &linodev1.PlacementGroup{}
	if err := c.handleProtoResponse(resp, group); err != nil {
		return nil, err
	}

	return group, nil
}

// httpCreatePlacementGroupProto creates a placement group as a proto message.
func (c *Client) httpCreatePlacementGroupProto(ctx context.Context, req *CreatePlacementGroupRequest) (*linodev1.PlacementGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_placement_group_create", req)
	if err != nil {
		return nil, wrapRequestError("CreatePlacementGroup", err)
	}

	defer drainClose(resp)

	placementGroup := &linodev1.PlacementGroup{}
	if err := c.handleProtoResponse(resp, placementGroup); err != nil {
		return nil, err
	}

	return placementGroup, nil
}

// httpUpdatePlacementGroupProto updates a placement group as a proto message.
func (c *Client) httpUpdatePlacementGroupProto(ctx context.Context, groupID int, request *UpdatePlacementGroupRequest) (*linodev1.PlacementGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_placement_group_update", request, groupID)
	if err != nil {
		return nil, wrapRequestError("UpdatePlacementGroup", err)
	}

	defer drainClose(resp)

	group := &linodev1.PlacementGroup{}
	if err := c.handleProtoResponse(resp, group); err != nil {
		return nil, err
	}

	return group, nil
}

// DeletePlacementGroup deletes a placement group by ID.
func (c *Client) httpDeletePlacementGroup(ctx context.Context, groupID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_placement_group_delete", nil, groupID)
	if err != nil {
		return wrapRequestError("DeletePlacementGroup", err)
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpUnassignPlacementGroupProto removes Linodes from a placement group and
// decodes the response as a proto message.
func (c *Client) httpUnassignPlacementGroupProto(ctx context.Context, groupID int, req *PlacementGroupUnassignRequest) (*linodev1.PlacementGroup, error) {
	if req == nil || len(req.Linodes) == 0 {
		return nil, ErrPlacementGroupUnassignLinodesRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_placement_group_unassign", req, groupID)
	if err != nil {
		return nil, wrapRequestError("UnassignPlacementGroup", err)
	}

	defer drainClose(resp)

	placementGroup := &linodev1.PlacementGroup{}
	if err := c.handleProtoResponse(resp, placementGroup); err != nil {
		return nil, err
	}

	return placementGroup, nil
}
