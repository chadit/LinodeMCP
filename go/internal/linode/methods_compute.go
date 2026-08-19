package linode

import (
	"context"
)

// The *Proto methods below decode the API JSON straight into the generated proto
// message. Each names the same tool as its struct twin, so both resolve the one
// declared route, and the paginated list helpers add page/page_size through
// withPaginationQuery, which keeps the two runtime requests identical.

// GetInstance retrieves a single Linode instance by its ID.
func (c *Client) httpGetInstance(ctx context.Context, instanceID int) (*Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_get", nil, instanceID)
	if err != nil {
		return nil, wrapRequestError("GetInstance", err)
	}

	defer drainClose(resp)

	var instance Instance
	if err := c.handleResponse(resp, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

// httpGetType retrieves a single Linode instance type by ID.
func (c *Client) httpGetType(ctx context.Context, typeID string) (*InstanceType, error) {
	return routedGet[InstanceType](ctx, c, "GetType", "linode_type_get", typeID)
}

// GetImage retrieves a single Linode image by ID.
func (c *Client) httpGetImage(ctx context.Context, imageID string) (*Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_get", nil, imageID)
	if err != nil {
		return nil, wrapRequestError("GetImage", err)
	}

	defer drainClose(resp)

	var image Image
	if err := c.handleResponse(resp, &image); err != nil {
		return nil, err
	}

	return &image, nil
}

// GetImageShareGroup retrieves a single image share group by ID.
func (c *Client) httpGetImageShareGroup(ctx context.Context, shareGroupID int) (*ImageShareGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_sharegroup_get", nil, shareGroupID)
	if err != nil {
		return nil, wrapRequestError("GetImageShareGroup", err)
	}

	defer drainClose(resp)

	var shareGroup ImageShareGroup
	if err := c.handleResponse(resp, &shareGroup); err != nil {
		return nil, err
	}

	return &shareGroup, nil
}

// GetImageShareGroupByToken retrieves a share group through a membership token UUID.
func (c *Client) httpGetImageShareGroupByToken(ctx context.Context, tokenUUID string) (*ImageShareGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_sharegroup_by_token_get", nil, tokenUUID)
	if err != nil {
		return nil, wrapRequestError("GetImageShareGroupByToken", err)
	}

	defer drainClose(resp)

	var shareGroup ImageShareGroup
	if err := c.handleResponse(resp, &shareGroup); err != nil {
		return nil, err
	}

	return &shareGroup, nil
}

// GetStackScript retrieves a single StackScript by ID.
func (c *Client) httpGetStackScript(ctx context.Context, stackScriptID int) (*StackScript, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_stackscript_get", nil, stackScriptID)
	if err != nil {
		return nil, wrapRequestError("GetStackScript", err)
	}

	defer drainClose(resp)

	var script StackScript
	if err := c.handleResponse(resp, &script); err != nil {
		return nil, err
	}

	return &script, nil
}
