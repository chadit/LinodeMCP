package linode

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	endpointInstances                       = "/linode/instances"
	endpointRegions                         = "/regions"
	endpointTypes                           = "/linode/types"
	endpointImages                          = "/images"
	endpointImageShareGroups                = "/images/sharegroups"
	endpointImageShareGroupMembershipCreate = "/images/sharegroups/tokens"
	endpointStackScripts                    = "/linode/stackscripts"
)

// ListInstances retrieves all Linode instances for the authenticated user.
func (c *Client) httpListInstances(ctx context.Context) ([]Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointInstances, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListInstances", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[Instance]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetInstance retrieves a single Linode instance by its ID.
func (c *Client) httpGetInstance(ctx context.Context, instanceID int) (*Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstances+"/%d", instanceID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstance", Err: err}
	}

	defer drainClose(resp)

	var instance Instance
	if err := c.handleResponse(resp, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

// ListRegions retrieves all available Linode regions.
func (c *Client) httpListRegions(ctx context.Context) ([]Region, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointRegions, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListRegions", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[Region]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListTypes retrieves all available Linode instance types.
func (c *Client) httpListTypes(ctx context.Context) ([]InstanceType, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointTypes, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListTypes", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[InstanceType]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListImages retrieves all available Linode images.
func (c *Client) httpListImages(ctx context.Context) ([]Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointImages, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListImages", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[Image]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListImageShareGroups retrieves owned image share groups.
func (c *Client) httpListImageShareGroups(ctx context.Context, page, pageSize int) (*PaginatedResponse[ImageShareGroup], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointImageShareGroups, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListImageShareGroups", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[ImageShareGroup]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetImageShareGroup retrieves a single image share group by ID.
func (c *Client) httpGetImageShareGroup(ctx context.Context, shareGroupID int) (*ImageShareGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointImageShareGroups+"/%d", shareGroupID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetImageShareGroup", Err: err}
	}

	defer drainClose(resp)

	var shareGroup ImageShareGroup
	if err := c.handleResponse(resp, &shareGroup); err != nil {
		return nil, err
	}

	return &shareGroup, nil
}

// ListImagesByShareGroup retrieves images shared in an owned image share group.
func (c *Client) httpListImagesByShareGroup(ctx context.Context, shareGroupID, page, pageSize int) (*PaginatedResponse[Image], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	baseEndpoint := endpointImageShareGroups + "/" + escapeImageShareGroupID(shareGroupID) + "/images"
	endpoint := withPaginationQuery(baseEndpoint, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListImagesByShareGroup", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[Image]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// ListMembersByImageShareGroup retrieves members linked to an owned image share group.
func (c *Client) httpListMembersByImageShareGroup(ctx context.Context, shareGroupID, page, pageSize int) (*PaginatedResponse[ImageShareGroupMember], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	baseEndpoint := endpointImageShareGroups + "/" + escapeImageShareGroupID(shareGroupID) + "/members"
	endpoint := withPaginationQuery(baseEndpoint, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListMembersByImageShareGroup", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[ImageShareGroupMember]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetImageShareGroupMemberToken retrieves a member token linked to an owned image share group.
func (c *Client) httpGetImageShareGroupMemberToken(ctx context.Context, shareGroupID int, tokenUUID string) (*ImageShareGroupMember, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroups + "/" + escapeImageShareGroupID(shareGroupID) + "/members/" + escapeImageShareGroupTokenUUID(tokenUUID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetImageShareGroupMemberToken", Err: err}
	}

	defer drainClose(resp)

	var member ImageShareGroupMember
	if err := c.handleResponse(resp, &member); err != nil {
		return nil, err
	}

	return &member, nil
}

// CreateImageShareGroup creates a group to share images with other users.
func (c *Client) httpCreateImageShareGroup(ctx context.Context, req *CreateImageShareGroupRequest) (*ImageShareGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointImageShareGroups, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateImageShareGroup", Err: err}
	}

	defer drainClose(resp)

	var shareGroup ImageShareGroup
	if err := c.handleResponse(resp, &shareGroup); err != nil {
		return nil, err
	}

	return &shareGroup, nil
}

// AddImageShareGroupImages adds images to an owned image share group.
func (c *Client) httpAddImageShareGroupImages(ctx context.Context, shareGroupID int, req *AddImageShareGroupImagesRequest) (*Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroups + "/" + escapeImageShareGroupID(shareGroupID) + "/images"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "AddImageShareGroupImages", Err: err}
	}

	defer drainClose(resp)

	var image Image
	if err := c.handleResponse(resp, &image); err != nil {
		return nil, err
	}

	return &image, nil
}

// DeleteImageShareGroupImage revokes access to one shared image in an owned image share group.
func (c *Client) httpDeleteImageShareGroupImage(ctx context.Context, shareGroupID, imageID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroups + "/" + escapeImageShareGroupID(shareGroupID) + "/images/" + escapeImageShareGroupID(imageID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteImageShareGroupImage", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// UpdateImageShareGroup updates an owned image share group.
func (c *Client) httpUpdateImageShareGroup(ctx context.Context, shareGroupID int, req *UpdateImageShareGroupRequest) (*ImageShareGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroups + "/" + escapeImageShareGroupID(shareGroupID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateImageShareGroup", Err: err}
	}

	defer drainClose(resp)

	var shareGroup ImageShareGroup
	if err := c.handleResponse(resp, &shareGroup); err != nil {
		return nil, err
	}

	return &shareGroup, nil
}

// UpdateImageShareGroupImage updates a shared image's label or description.
func (c *Client) httpUpdateImageShareGroupImage(ctx context.Context, shareGroupID int, imageID string, req *UpdateImageShareGroupImageRequest) (*Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroups + "/" + escapeImageShareGroupID(shareGroupID) + "/images/" + escapeImageIDSegment(imageID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateImageShareGroupImage", Err: err}
	}

	defer drainClose(resp)

	var image Image
	if err := c.handleResponse(resp, &image); err != nil {
		return nil, err
	}

	return &image, nil
}

// DeleteImageShareGroup removes an owned image share group.
func (c *Client) httpDeleteImageShareGroup(ctx context.Context, shareGroupID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointImageShareGroups+"/%d", shareGroupID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteImageShareGroup", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// ListImageShareGroupTokens retrieves image share group tokens for the user.
func (c *Client) httpListImageShareGroupTokens(ctx context.Context, page, pageSize int) (*PaginatedResponse[ImageShareGroupToken], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointImageShareGroups+"/tokens", page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListImageShareGroupTokens", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[ImageShareGroupToken]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// CreateImageShareGroupToken creates a single-use image share group membership token.
func (c *Client) httpCreateImageShareGroupToken(ctx context.Context, req *CreateImageShareGroupTokenRequest) (*ImageShareGroupToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointImageShareGroupMembershipCreate, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateImageShareGroupToken", Err: err}
	}

	defer drainClose(resp)

	var token ImageShareGroupToken
	if err := c.handleResponse(resp, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// GetImageShareGroupToken retrieves a single image share group token by UUID.
func (c *Client) httpGetImageShareGroupToken(ctx context.Context, tokenUUID string) (*ImageShareGroupToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroups + "/tokens/" + escapeImageShareGroupTokenUUID(tokenUUID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetImageShareGroupToken", Err: err}
	}

	defer drainClose(resp)

	var token ImageShareGroupToken
	if err := c.handleResponse(resp, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// ListImagesByShareGroupToken retrieves images available through an image share group token.
func (c *Client) httpListImagesByShareGroupToken(ctx context.Context, tokenUUID string, page, pageSize int) (*PaginatedResponse[Image], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	baseEndpoint := endpointImageShareGroups + "/tokens/" + escapeImageShareGroupTokenUUID(tokenUUID) + "/sharegroup/images"
	endpoint := withPaginationQuery(baseEndpoint, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListImagesByShareGroupToken", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[Image]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// UpdateImageShareGroupToken updates a single image share group membership token label.
func (c *Client) httpUpdateImageShareGroupToken(ctx context.Context, tokenUUID string, req *UpdateImageShareGroupTokenRequest) (*ImageShareGroupToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroupMembershipCreate + "/" + escapeImageShareGroupTokenUUID(tokenUUID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateImageShareGroupToken", Err: err}
	}

	defer drainClose(resp)

	var token ImageShareGroupToken
	if err := c.handleResponse(resp, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// GetImageShareGroupByToken retrieves a share group through a membership token UUID.
func (c *Client) httpGetImageShareGroupByToken(ctx context.Context, tokenUUID string) (*ImageShareGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroups + "/tokens/" + escapeImageShareGroupTokenUUID(tokenUUID) + "/sharegroup"

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetImageShareGroupByToken", Err: err}
	}

	defer drainClose(resp)

	var shareGroup ImageShareGroup
	if err := c.handleResponse(resp, &shareGroup); err != nil {
		return nil, err
	}

	return &shareGroup, nil
}

func escapeImageShareGroupTokenUUID(tokenUUID string) string {
	escapedTokenUUID := url.PathEscape(tokenUUID)
	if tokenUUID == "." || tokenUUID == ".." {
		escapedTokenUUID = strings.ReplaceAll(escapedTokenUUID, ".", "%2E")
	}

	return escapedTokenUUID
}

func escapeImageShareGroupID(shareGroupID int) string {
	return strings.ReplaceAll(url.PathEscape(strconv.Itoa(shareGroupID)), ".", "%2E")
}

func escapeImageIDSegment(imageID string) string {
	return strings.ReplaceAll(url.PathEscape(imageID), ".", "%2E")
}

// DeleteImageShareGroupToken removes one image share group membership token.
func (c *Client) httpDeleteImageShareGroupToken(ctx context.Context, tokenUUID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroups + "/tokens/" + escapeImageShareGroupTokenUUID(tokenUUID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteImageShareGroupToken", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// CreateImage creates a private image from a Linode disk.
func (c *Client) httpCreateImage(ctx context.Context, req *CreateImageRequest) (*Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointImages, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateImage", Err: err}
	}

	defer drainClose(resp)

	var image Image
	if err := c.handleResponse(resp, &image); err != nil {
		return nil, err
	}

	return &image, nil
}

// ListStackScripts retrieves StackScripts available to the authenticated user.
func (c *Client) httpListStackScripts(ctx context.Context) ([]StackScript, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointStackScripts, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListStackScripts", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[StackScript]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// CreateStackScript creates a new StackScript.
func (c *Client) httpCreateStackScript(ctx context.Context, req *CreateStackScriptRequest) (*StackScript, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointStackScripts, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateStackScript", Err: err}
	}

	defer drainClose(resp)

	var script StackScript
	if err := c.handleResponse(resp, &script); err != nil {
		return nil, err
	}

	return &script, nil
}

// BootInstance boots a Linode instance.
func (c *Client) httpBootInstance(ctx context.Context, instanceID int, configID *int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstances+"/%d/boot", instanceID)

	var payload any
	if configID != nil {
		payload = map[string]int{"config_id": *configID}
	}

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return &NetworkError{Operation: "BootInstance", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// RebootInstance reboots a Linode instance.
func (c *Client) httpRebootInstance(ctx context.Context, instanceID int, configID *int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstances+"/%d/reboot", instanceID)

	var payload any
	if configID != nil {
		payload = map[string]int{"config_id": *configID}
	}

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return &NetworkError{Operation: "RebootInstance", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// ShutdownInstance shuts down a Linode instance.
func (c *Client) httpShutdownInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstances+"/%d/shutdown", instanceID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "ShutdownInstance", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// CreateInstance creates a new Linode instance.
func (c *Client) httpCreateInstance(ctx context.Context, req *CreateInstanceRequest) (*Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointInstances, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateInstance", Err: err}
	}

	defer drainClose(resp)

	var instance Instance
	if err := c.handleResponse(resp, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

// DeleteInstance deletes a Linode instance.
func (c *Client) httpDeleteInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstances+"/%d", instanceID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteInstance", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// ResizeInstance resizes a Linode instance to a new plan.
func (c *Client) httpResizeInstance(ctx context.Context, instanceID int, req ResizeInstanceRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstances+"/%d/resize", instanceID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return &NetworkError{Operation: "ResizeInstance", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// UpdateInstance updates a Linode instance's editable fields.
func (c *Client) httpUpdateInstance(ctx context.Context, instanceID int, req *UpdateInstanceRequest) (*Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstances+"/%d", instanceID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateInstance", Err: err}
	}

	defer drainClose(resp)

	var instance Instance
	if err := c.handleResponse(resp, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}
