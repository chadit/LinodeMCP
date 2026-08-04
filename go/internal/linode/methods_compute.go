package linode

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The *Proto methods below decode the API JSON straight into the generated proto
// message. Each names the same tool as its struct twin, so both resolve the one
// declared route, and the paginated list helpers add page/page_size through
// withPaginationQuery, which keeps the two runtime requests identical.

// httpListInstancesProto lists Linode instances.
func (c *Client) httpListInstancesProto(ctx context.Context, page, pageSize int) ([]*linodev1.Instance, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListInstances",
		"linode_instance_list", "", nil, page, pageSize,
		func() *linodev1.Instance { return &linodev1.Instance{} })
}

// httpGetInstanceProto retrieves one Linode instance by ID.
func (c *Client) httpGetInstanceProto(ctx context.Context, instanceID int) (*linodev1.Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_get", nil, instanceID)
	if err != nil {
		return nil, wrapRequestError("GetInstance", err)
	}

	defer drainClose(resp)

	instance := &linodev1.Instance{}
	if err := c.handleProtoResponse(resp, instance); err != nil {
		return nil, err
	}

	return instance, nil
}

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

// httpGetInstanceStatsByYearMonthProto retrieves monthly statistics for a Linode
// instance. Like the daily stats endpoint, the graphs nest under a top-level
// "data" object modeled by InstanceStats.
func (c *Client) httpGetInstanceStatsByYearMonthProto(ctx context.Context, linodeID, year, month int) (*linodev1.InstanceStats, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if year < 2000 || year > 2037 {
		return nil, ErrStatsYearRange
	}

	if month < 1 || month > 12 {
		return nil, ErrStatsMonthRange
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_stats_month_get", nil, linodeID, year, month)
	if err != nil {
		return nil, wrapRequestError("GetInstanceStatsByYearMonth", err)
	}

	defer drainClose(resp)

	stats := &linodev1.InstanceStats{}
	if err := c.handleProtoResponse(resp, stats); err != nil {
		return nil, err
	}

	return stats, nil
}

// httpGetInstanceTransferProto retrieves the current month's network transfer
// pool for a Linode instance.
func (c *Client) httpGetInstanceTransferProto(ctx context.Context, linodeID int) (*linodev1.InstanceTransfer, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_transfer_get", nil, linodeID)
	if err != nil {
		return nil, wrapRequestError("GetInstanceTransfer", err)
	}

	defer drainClose(resp)

	transfer := &linodev1.InstanceTransfer{}
	if err := c.handleProtoResponse(resp, transfer); err != nil {
		return nil, err
	}

	return transfer, nil
}

// httpListRegionsProto lists regions.
func (c *Client) httpListRegionsProto(ctx context.Context, page, pageSize int) ([]*linodev1.Region, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListRegions",
		"linode_region_list", "", nil, page, pageSize,
		func() *linodev1.Region { return &linodev1.Region{} })
}

// GetRegion retrieves a single Linode region by its ID.
func (c *Client) httpGetRegion(ctx context.Context, regionID string) (*Region, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_region_get", nil, regionID)
	if err != nil {
		return nil, wrapRequestError("GetRegion", err)
	}

	defer drainClose(resp)

	var region Region
	if err := c.handleResponse(resp, &region); err != nil {
		return nil, err
	}

	return &region, nil
}

// httpGetRegionProto retrieves one region.
func (c *Client) httpGetRegionProto(ctx context.Context, regionID string) (*linodev1.Region, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_region_get", nil, regionID)
	if err != nil {
		return nil, wrapRequestError("GetRegion", err)
	}

	defer drainClose(resp)

	region := &linodev1.Region{}
	if err := c.handleProtoResponse(resp, region); err != nil {
		return nil, err
	}

	return region, nil
}

// httpListRegionsAvailabilityProto lists compute type availability across regions.
func (c *Client) httpListRegionsAvailabilityProto(ctx context.Context) ([]*linodev1.RegionAvailability, error) {
	return listProtoElementsRouted(ctx, c, "ListRegionsAvailability",
		"linode_region_availability_list", "", nil,
		func() *linodev1.RegionAvailability { return &linodev1.RegionAvailability{} })
}

// httpGetRegionAvailabilityProto lists compute type availability for one region.
// This endpoint documents its 200 body as a bare top-level JSON array, not the
// usual {data:[...]} page envelope, so the strict bare fetcher decodes the array
// directly and rejects anything else.
func (c *Client) httpGetRegionAvailabilityProto(ctx context.Context, regionID string) ([]*linodev1.RegionAvailability, error) {
	return listProtoElementsBareRouted(ctx, c, "GetRegionAvailability",
		"linode_region_availability_get", "", []any{regionID},
		func() *linodev1.RegionAvailability { return &linodev1.RegionAvailability{} })
}

// httpListKernelsProto lists kernels.
func (c *Client) httpListKernelsProto(ctx context.Context, page, pageSize int) ([]*linodev1.Kernel, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListKernels",
		"linode_kernel_list", "", nil, page, pageSize,
		func() *linodev1.Kernel { return &linodev1.Kernel{} })
}

// httpListTypesProto lists the available Linode instance types.
func (c *Client) httpListTypesProto(ctx context.Context) ([]*linodev1.InstanceType, error) {
	return listProtoElementsRouted(ctx, c, "ListTypes",
		"linode_type_list", "", nil,
		func() *linodev1.InstanceType { return &linodev1.InstanceType{} })
}

// GetType retrieves a single Linode instance type by ID.
func (c *Client) httpGetType(ctx context.Context, typeID string) (*InstanceType, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_type_get", nil, typeID)
	if err != nil {
		return nil, wrapRequestError("GetType", err)
	}

	defer drainClose(resp)

	var instanceType InstanceType
	if err := c.handleResponse(resp, &instanceType); err != nil {
		return nil, err
	}

	return &instanceType, nil
}

// httpGetTypeProto retrieves one instance type.
func (c *Client) httpGetTypeProto(ctx context.Context, typeID string) (*linodev1.InstanceType, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_type_get", nil, typeID)
	if err != nil {
		return nil, wrapRequestError("GetType", err)
	}

	defer drainClose(resp)

	instanceType := &linodev1.InstanceType{}
	if err := c.handleProtoResponse(resp, instanceType); err != nil {
		return nil, err
	}

	return instanceType, nil
}

// httpGetKernelProto retrieves one kernel.
func (c *Client) httpGetKernelProto(ctx context.Context, kernelID string) (*linodev1.Kernel, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_kernel_get", nil, kernelID)
	if err != nil {
		return nil, wrapRequestError("GetKernel", err)
	}

	defer drainClose(resp)

	kernel := &linodev1.Kernel{}
	if err := c.handleProtoResponse(resp, kernel); err != nil {
		return nil, err
	}

	return kernel, nil
}

// httpListImagesProto lists images.
func (c *Client) httpListImagesProto(ctx context.Context, page, pageSize int) ([]*linodev1.Image, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListImages",
		"linode_image_list", "", nil, page, pageSize,
		func() *linodev1.Image { return &linodev1.Image{} })
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

// httpGetImageProto retrieves one image.
func (c *Client) httpGetImageProto(ctx context.Context, imageID string) (*linodev1.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_get", nil, imageID)
	if err != nil {
		return nil, wrapRequestError("GetImage", err)
	}

	defer drainClose(resp)

	image := &linodev1.Image{}
	if err := c.handleProtoResponse(resp, image); err != nil {
		return nil, err
	}

	return image, nil
}

// DeleteImage deletes a private image.
func (c *Client) httpDeleteImage(ctx context.Context, imageID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_delete", nil, imageID)
	if err != nil {
		return wrapRequestError("DeleteImage", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpReplicateImageProto replicates an image.
func (c *Client) httpReplicateImageProto(ctx context.Context, imageID string, req *ReplicateImageRequest) (*linodev1.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_replicate", req, imageID)
	if err != nil {
		return nil, wrapRequestError("ReplicateImage", err)
	}

	defer drainClose(resp)

	image := &linodev1.Image{}
	if err := c.handleProtoResponse(resp, image); err != nil {
		return nil, err
	}

	return image, nil
}

// httpUpdateImageProto updates an image.
func (c *Client) httpUpdateImageProto(ctx context.Context, imageID string, req *UpdateImageRequest) (*linodev1.Image, error) {
	if req == nil {
		return nil, ErrUpdateImageRequestRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_update", req, imageID)
	if err != nil {
		return nil, wrapRequestError("UpdateImage", err)
	}

	defer drainClose(resp)

	image := &linodev1.Image{}
	if err := c.handleProtoResponse(resp, image); err != nil {
		return nil, err
	}

	return image, nil
}

// httpListImageShareGroupsProto lists owned image share groups.
func (c *Client) httpListImageShareGroupsProto(ctx context.Context, page, pageSize int) ([]*linodev1.ImageShareGroup, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListImageShareGroups",
		"linode_image_sharegroup_list", "", nil, page, pageSize,
		func() *linodev1.ImageShareGroup { return &linodev1.ImageShareGroup{} })
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

// httpGetImageShareGroupProto retrieves one image share group.
func (c *Client) httpGetImageShareGroupProto(ctx context.Context, shareGroupID int) (*linodev1.ImageShareGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_sharegroup_get", nil, shareGroupID)
	if err != nil {
		return nil, wrapRequestError("GetImageShareGroup", err)
	}

	defer drainClose(resp)

	shareGroup := &linodev1.ImageShareGroup{}
	if err := c.handleProtoResponse(resp, shareGroup); err != nil {
		return nil, err
	}

	return shareGroup, nil
}

// httpListImageShareGroupsByImageProto lists the share groups that contain an image.
func (c *Client) httpListImageShareGroupsByImageProto(ctx context.Context, imageID string, page, pageSize int) ([]*linodev1.ImageShareGroup, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListImageShareGroupsByImage",
		"linode_image_sharegroup_by_image_list", "", []any{imageID}, page, pageSize,
		func() *linodev1.ImageShareGroup { return &linodev1.ImageShareGroup{} })
}

// httpListImagesByShareGroupProto lists the images shared in an owned share group.
func (c *Client) httpListImagesByShareGroupProto(ctx context.Context, shareGroupID, page, pageSize int) ([]*linodev1.Image, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListImagesByShareGroup",
		"linode_image_sharegroup_image_list", "", []any{shareGroupID}, page, pageSize,
		func() *linodev1.Image { return &linodev1.Image{} })
}

// httpListMembersByImageShareGroupProto lists members linked to an owned share group.
func (c *Client) httpListMembersByImageShareGroupProto(ctx context.Context, shareGroupID, page, pageSize int) ([]*linodev1.ImageShareGroupMember, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListMembersByImageShareGroup",
		"linode_image_sharegroup_member_list", "", []any{shareGroupID}, page, pageSize,
		func() *linodev1.ImageShareGroupMember { return &linodev1.ImageShareGroupMember{} })
}

// httpGetImageShareGroupMemberTokenProto retrieves one share group member token.
func (c *Client) httpGetImageShareGroupMemberTokenProto(ctx context.Context, shareGroupID int, tokenUUID string) (*linodev1.ImageShareGroupMember, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_sharegroup_member_token_get", nil, shareGroupID, tokenUUID)
	if err != nil {
		return nil, wrapRequestError("GetImageShareGroupMemberToken", err)
	}

	defer drainClose(resp)

	member := &linodev1.ImageShareGroupMember{}
	if err := c.handleProtoResponse(resp, member); err != nil {
		return nil, err
	}

	return member, nil
}

// httpUpdateImageShareGroupMemberProto updates one share group member token.
func (c *Client) httpUpdateImageShareGroupMemberProto(ctx context.Context, shareGroupID int, tokenUUID string, req *UpdateImageShareGroupMemberRequest) (*linodev1.ImageShareGroupMember, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_sharegroup_member_token_update", req, shareGroupID, tokenUUID)
	if err != nil {
		return nil, wrapRequestError("UpdateImageShareGroupMember", err)
	}

	defer drainClose(resp)

	member := &linodev1.ImageShareGroupMember{}
	if err := c.handleProtoResponse(resp, member); err != nil {
		return nil, err
	}

	return member, nil
}

// httpCreateImageShareGroupProto creates an image share group.
func (c *Client) httpCreateImageShareGroupProto(ctx context.Context, req *CreateImageShareGroupRequest) (*linodev1.ImageShareGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_sharegroup_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateImageShareGroup", err)
	}

	defer drainClose(resp)

	shareGroup := &linodev1.ImageShareGroup{}
	if err := c.handleProtoResponse(resp, shareGroup); err != nil {
		return nil, err
	}

	return shareGroup, nil
}

// httpAddImageShareGroupImagesProto adds images to a share group.
func (c *Client) httpAddImageShareGroupImagesProto(ctx context.Context, shareGroupID int, req *AddImageShareGroupImagesRequest) (*linodev1.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_sharegroup_image_add", req, shareGroupID)
	if err != nil {
		return nil, wrapRequestError("AddImageShareGroupImages", err)
	}

	defer drainClose(resp)

	image := &linodev1.Image{}
	if err := c.handleProtoResponse(resp, image); err != nil {
		return nil, err
	}

	return image, nil
}

// httpAddImageShareGroupMembersProto adds members to a share group, returning the
// parent share group the API echoes back.
func (c *Client) httpAddImageShareGroupMembersProto(ctx context.Context, shareGroupID int, req *AddImageShareGroupMembersRequest) (*linodev1.ImageShareGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_sharegroup_member_add", req, shareGroupID)
	if err != nil {
		return nil, wrapRequestError("AddImageShareGroupMembers", err)
	}

	defer drainClose(resp)

	shareGroup := &linodev1.ImageShareGroup{}
	if err := c.handleProtoResponse(resp, shareGroup); err != nil {
		return nil, err
	}

	return shareGroup, nil
}

// DeleteImageShareGroupImage revokes access to one shared image in an owned image share group.
func (c *Client) httpDeleteImageShareGroupImage(ctx context.Context, shareGroupID, imageID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_sharegroup_image_delete", nil, shareGroupID, imageID)
	if err != nil {
		return wrapRequestError("DeleteImageShareGroupImage", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpUpdateImageShareGroupProto updates an image share group.
func (c *Client) httpUpdateImageShareGroupProto(ctx context.Context, shareGroupID int, req *UpdateImageShareGroupRequest) (*linodev1.ImageShareGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_sharegroup_update", req, shareGroupID)
	if err != nil {
		return nil, wrapRequestError("UpdateImageShareGroup", err)
	}

	defer drainClose(resp)

	shareGroup := &linodev1.ImageShareGroup{}
	if err := c.handleProtoResponse(resp, shareGroup); err != nil {
		return nil, err
	}

	return shareGroup, nil
}

// httpUpdateImageShareGroupImageProto updates one shared image in a share group.
func (c *Client) httpUpdateImageShareGroupImageProto(ctx context.Context, shareGroupID int, imageID string, req *UpdateImageShareGroupImageRequest) (*linodev1.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_sharegroup_image_update", req, shareGroupID, imageID)
	if err != nil {
		return nil, wrapRequestError("UpdateImageShareGroupImage", err)
	}

	defer drainClose(resp)

	image := &linodev1.Image{}
	if err := c.handleProtoResponse(resp, image); err != nil {
		return nil, err
	}

	return image, nil
}

// DeleteImageShareGroup removes an owned image share group.
func (c *Client) httpDeleteImageShareGroup(ctx context.Context, shareGroupID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_sharegroup_delete", nil, shareGroupID)
	if err != nil {
		return wrapRequestError("DeleteImageShareGroup", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpListImageShareGroupTokensProto lists the user's image share group tokens.
func (c *Client) httpListImageShareGroupTokensProto(ctx context.Context, page, pageSize int) ([]*linodev1.ImageShareGroupToken, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListImageShareGroupTokens",
		"linode_image_sharegroup_token_list", "", nil, page, pageSize,
		func() *linodev1.ImageShareGroupToken { return &linodev1.ImageShareGroupToken{} })
}

// httpCreateImageShareGroupTokenProto creates a membership token.
func (c *Client) httpCreateImageShareGroupTokenProto(ctx context.Context, req *CreateImageShareGroupTokenRequest) (*linodev1.ImageShareGroupToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_sharegroup_token_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateImageShareGroupToken", err)
	}

	defer drainClose(resp)

	token := &linodev1.ImageShareGroupToken{}
	if err := c.handleProtoResponse(resp, token); err != nil {
		return nil, err
	}

	return token, nil
}

// httpGetImageShareGroupTokenProto retrieves one image share group token.
func (c *Client) httpGetImageShareGroupTokenProto(ctx context.Context, tokenUUID string) (*linodev1.ImageShareGroupToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_sharegroup_token_get", nil, tokenUUID)
	if err != nil {
		return nil, wrapRequestError("GetImageShareGroupToken", err)
	}

	defer drainClose(resp)

	token := &linodev1.ImageShareGroupToken{}
	if err := c.handleProtoResponse(resp, token); err != nil {
		return nil, err
	}

	return token, nil
}

// httpListImagesByShareGroupTokenProto lists the images a membership token reaches.
func (c *Client) httpListImagesByShareGroupTokenProto(ctx context.Context, tokenUUID string, page, pageSize int) ([]*linodev1.Image, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListImagesByShareGroupToken",
		"linode_image_sharegroup_token_image_list", "", []any{tokenUUID}, page, pageSize,
		func() *linodev1.Image { return &linodev1.Image{} })
}

// httpUpdateImageShareGroupTokenProto updates a membership token label.
func (c *Client) httpUpdateImageShareGroupTokenProto(ctx context.Context, tokenUUID string, req *UpdateImageShareGroupTokenRequest) (*linodev1.ImageShareGroupToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_sharegroup_token_update", req, tokenUUID)
	if err != nil {
		return nil, wrapRequestError("UpdateImageShareGroupToken", err)
	}

	defer drainClose(resp)

	token := &linodev1.ImageShareGroupToken{}
	if err := c.handleProtoResponse(resp, token); err != nil {
		return nil, err
	}

	return token, nil
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

// httpGetImageShareGroupByTokenProto resolves a token to its parent share group.
func (c *Client) httpGetImageShareGroupByTokenProto(ctx context.Context, tokenUUID string) (*linodev1.ImageShareGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_sharegroup_by_token_get", nil, tokenUUID)
	if err != nil {
		return nil, wrapRequestError("GetImageShareGroupByToken", err)
	}

	defer drainClose(resp)

	shareGroup := &linodev1.ImageShareGroup{}
	if err := c.handleProtoResponse(resp, shareGroup); err != nil {
		return nil, err
	}

	return shareGroup, nil
}

// DeleteImageShareGroupToken removes one image share group membership token.
func (c *Client) httpDeleteImageShareGroupToken(ctx context.Context, tokenUUID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_sharegroup_token_delete", nil, tokenUUID)
	if err != nil {
		return wrapRequestError("DeleteImageShareGroupToken", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// DeleteImageShareGroupMemberToken revokes one accepted membership token from an owned image share group.
func (c *Client) httpDeleteImageShareGroupMemberToken(ctx context.Context, shareGroupID int, tokenUUID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_sharegroup_member_token_delete", nil, shareGroupID, tokenUUID)
	if err != nil {
		return wrapRequestError("DeleteImageShareGroupMemberToken", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpCreateImageProto creates an image.
func (c *Client) httpCreateImageProto(ctx context.Context, req *CreateImageRequest) (*linodev1.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateImage", err)
	}

	defer drainClose(resp)

	image := &linodev1.Image{}
	if err := c.handleProtoResponse(resp, image); err != nil {
		return nil, err
	}

	return image, nil
}

// httpUploadImageProto creates an image upload target, returning the one-time
// upload URL plus the created image. The body is {image, upload_to}, so the image
// sub-object is protojson-decoded here to match the Python serializer.
func (c *Client) httpUploadImageProto(ctx context.Context, req *UploadImageRequest) (*linodev1.Image, string, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_image_upload", req)
	if err != nil {
		return nil, "", wrapRequestError("UploadImage", err)
	}

	defer drainClose(resp)

	var envelope struct {
		UploadTo string          `json:"upload_to"`
		Image    json.RawMessage `json:"image"`
	}
	if err := c.handleResponse(resp, &envelope); err != nil {
		return nil, "", err
	}

	image := &linodev1.Image{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(envelope.Image, image); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal upload image element: %w", err)
	}

	return image, envelope.UploadTo, nil
}

// httpListStackScriptsProto lists one page of StackScripts. The tool filters
// (is_public / mine / label_contains) are applied client-side by the factory, to
// the page this returns.
func (c *Client) httpListStackScriptsProto(ctx context.Context, page, pageSize int) ([]*linodev1.StackScript, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListStackScripts",
		"linode_stackscript_list", "", nil, page, pageSize,
		func() *linodev1.StackScript { return &linodev1.StackScript{} })
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

// httpGetStackScriptProto retrieves one StackScript.
func (c *Client) httpGetStackScriptProto(ctx context.Context, stackScriptID int) (*linodev1.StackScript, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_stackscript_get", nil, stackScriptID)
	if err != nil {
		return nil, wrapRequestError("GetStackScript", err)
	}

	defer drainClose(resp)

	script := &linodev1.StackScript{}
	if err := c.handleProtoResponse(resp, script); err != nil {
		return nil, err
	}

	return script, nil
}

// httpCreateStackScriptProto creates a StackScript.
func (c *Client) httpCreateStackScriptProto(ctx context.Context, req *CreateStackScriptRequest) (*linodev1.StackScript, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_stackscript_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateStackScript", err)
	}

	defer drainClose(resp)

	script := &linodev1.StackScript{}
	if err := c.handleProtoResponse(resp, script); err != nil {
		return nil, err
	}

	return script, nil
}

// httpUpdateStackScriptProto updates a StackScript.
func (c *Client) httpUpdateStackScriptProto(ctx context.Context, stackScriptID int, req *UpdateStackScriptRequest) (*linodev1.StackScript, error) {
	if stackScriptID <= 0 {
		return nil, ErrStackScriptIDPositive
	}

	if updateStackScriptRequestEmpty(req) {
		return nil, ErrStackScriptUpdateRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_stackscript_update", req, stackScriptID)
	if err != nil {
		return nil, wrapRequestError("UpdateStackScript", err)
	}

	defer drainClose(resp)

	script := &linodev1.StackScript{}
	if err := c.handleProtoResponse(resp, script); err != nil {
		return nil, err
	}

	return script, nil
}

// DeleteStackScript deletes a StackScript.
func (c *Client) httpDeleteStackScript(ctx context.Context, stackScriptID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_stackscript_delete", nil, stackScriptID)
	if err != nil {
		return wrapRequestError("DeleteStackScript", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

func updateStackScriptRequestEmpty(req *UpdateStackScriptRequest) bool {
	if req == nil {
		return true
	}

	return req.Label == nil && req.Script == nil && len(req.Images) == 0 && req.Description == nil && req.IsPublic == nil && req.RevNote == nil
}

// BootInstance boots a Linode instance.
func (c *Client) httpBootInstance(ctx context.Context, instanceID int, configID *int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	var payload any
	if configID != nil {
		payload = map[string]int{"config_id": *configID}
	}

	resp, err := c.makeRouteRequest(ctx, "linode_instance_boot", payload, instanceID)
	if err != nil {
		return wrapRequestError("BootInstance", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// RebootInstance reboots a Linode instance.
func (c *Client) httpRebootInstance(ctx context.Context, instanceID int, configID *int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	var payload any
	if configID != nil {
		payload = map[string]int{"config_id": *configID}
	}

	resp, err := c.makeRouteRequest(ctx, "linode_instance_reboot", payload, instanceID)
	if err != nil {
		return wrapRequestError("RebootInstance", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// ShutdownInstance shuts down a Linode instance.
func (c *Client) httpShutdownInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_shutdown", nil, instanceID)
	if err != nil {
		return wrapRequestError("ShutdownInstance", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpCreateInstanceProto creates a Linode instance.
func (c *Client) httpCreateInstanceProto(ctx context.Context, req *CreateInstanceRequest) (*linodev1.Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateInstance", err)
	}

	defer drainClose(resp)

	instance := &linodev1.Instance{}
	if err := c.handleProtoResponse(resp, instance); err != nil {
		return nil, err
	}

	return instance, nil
}

// httpUpdateInstanceProto updates a Linode instance.
func (c *Client) httpUpdateInstanceProto(ctx context.Context, instanceID int, req *UpdateInstanceRequest) (*linodev1.Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_update", req, instanceID)
	if err != nil {
		return nil, wrapRequestError("UpdateInstance", err)
	}

	defer drainClose(resp)

	instance := &linodev1.Instance{}
	if err := c.handleProtoResponse(resp, instance); err != nil {
		return nil, err
	}

	return instance, nil
}

// DeleteInstance deletes a Linode instance.
func (c *Client) httpDeleteInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_delete", nil, instanceID)
	if err != nil {
		return wrapRequestError("DeleteInstance", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// ResizeInstance resizes a Linode instance to a new plan.
func (c *Client) httpResizeInstance(ctx context.Context, instanceID int, req ResizeInstanceRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_resize", req, instanceID)
	if err != nil {
		return wrapRequestError("ResizeInstance", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}
