package linode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

const (
	endpointInstances                       = "/linode/instances"
	endpointRegions                         = "/regions"
	endpointRegionsAvailability             = "/regions/availability"
	endpointRegionAvailability              = endpointRegions + "/%s/availability"
	endpointKernels                         = "/linode/kernels"
	endpointTypes                           = "/linode/types"
	endpointImages                          = "/images"
	endpointImagesUpload                    = "/images/upload"
	endpointImageShareGroups                = "/images/sharegroups"
	endpointImageShareGroupMembershipCreate = "/images/sharegroups/tokens"
	endpointStackScripts                    = "/linode/stackscripts"
)

// httpListInstancesProto retrieves all Linode instances as proto messages,
// decoded directly from the API JSON for the proto-backed read path.
func (c *Client) httpListInstancesProto(ctx context.Context) ([]*linodev1.Instance, error) {
	return listProtoElements(ctx, c, "ListInstances", endpointInstances,
		func() *linodev1.Instance { return &linodev1.Instance{} })
}

// httpGetInstanceProto retrieves a single Linode instance by ID as a proto
// message, decoded directly from the API JSON for the proto-backed read path.
func (c *Client) httpGetInstanceProto(ctx context.Context, instanceID int) (*linodev1.Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstances+"/%d", instanceID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstance", Err: err}
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

// httpGetInstanceStatsByYearMonthProto retrieves monthly statistics for a Linode
// instance as a proto message. Like the daily stats endpoint, the graphs nest
// under a top-level "data" object modeled by InstanceStats.
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

	endpoint := fmt.Sprintf(endpointInstances+"/%d/stats/%d/%d", linodeID, year, month)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstanceStatsByYearMonth", Err: err}
	}

	defer drainClose(resp)

	stats := &linodev1.InstanceStats{}
	if err := c.handleProtoResponse(resp, stats); err != nil {
		return nil, err
	}

	return stats, nil
}

// httpGetInstanceTransferProto retrieves the current month's network transfer
// pool for a Linode instance as a proto message.
func (c *Client) httpGetInstanceTransferProto(ctx context.Context, linodeID int) (*linodev1.InstanceTransfer, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := fmt.Sprintf(endpointInstances+"/%s/transfer", encodedLinodeID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstanceTransfer", Err: err}
	}

	defer drainClose(resp)

	transfer := &linodev1.InstanceTransfer{}
	if err := c.handleProtoResponse(resp, transfer); err != nil {
		return nil, err
	}

	return transfer, nil
}

// httpListRegionsProto retrieves all regions as proto messages for the
// proto-backed list path, sharing the decode tail with every other proto list.
func (c *Client) httpListRegionsProto(ctx context.Context) ([]*linodev1.Region, error) {
	return listProtoElements(ctx, c, "ListRegions", endpointRegions,
		func() *linodev1.Region { return &linodev1.Region{} })
}

// GetRegion retrieves a single Linode region by its ID.
func (c *Client) httpGetRegion(ctx context.Context, regionID string) (*Region, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointRegions + "/" + url.PathEscape(regionID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetRegion", Err: err}
	}

	defer drainClose(resp)

	var region Region
	if err := c.handleResponse(resp, &region); err != nil {
		return nil, err
	}

	return &region, nil
}

// httpGetRegionProto retrieves one region as a proto message.
func (c *Client) httpGetRegionProto(ctx context.Context, regionID string) (*linodev1.Region, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointRegions + "/" + url.PathEscape(regionID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetRegion", Err: err}
	}

	defer drainClose(resp)

	region := &linodev1.Region{}
	if err := c.handleProtoResponse(resp, region); err != nil {
		return nil, err
	}

	return region, nil
}

// httpListRegionsAvailabilityProto retrieves compute type availability across
// regions as proto RegionAvailability messages for the proto-backed list path.
func (c *Client) httpListRegionsAvailabilityProto(ctx context.Context) ([]*linodev1.RegionAvailability, error) {
	return listProtoElements(ctx, c, "ListRegionsAvailability", endpointRegionsAvailability,
		func() *linodev1.RegionAvailability { return &linodev1.RegionAvailability{} })
}

// httpGetRegionAvailabilityProto retrieves compute type availability for one
// region as proto RegionAvailability messages for the proto-backed read path.
// Unlike the cross-region list (a {data:[...]} page envelope), this endpoint
// documents its 200 body as a bare top-level JSON array, so the strict bare
// fetcher decodes the array directly and rejects anything else while sharing
// the per-element decode tail, keeping the per-region get and the list
// byte-identical element-for-element.
func (c *Client) httpGetRegionAvailabilityProto(ctx context.Context, regionID string) ([]*linodev1.RegionAvailability, error) {
	endpoint := fmt.Sprintf(endpointRegionAvailability, url.PathEscape(regionID))

	return listProtoElementsBare(ctx, c, "GetRegionAvailability", endpoint,
		func() *linodev1.RegionAvailability { return &linodev1.RegionAvailability{} })
}

// httpListKernelsProto retrieves kernels as proto messages for the proto-backed
// list path. The page/page_size pair flows through withPaginationQuery, so the
// request matches httpListKernels.
func (c *Client) httpListKernelsProto(ctx context.Context, page, pageSize int) ([]*linodev1.Kernel, error) {
	return listProtoElementsPaginated(ctx, c, "ListKernels", endpointKernels, page, pageSize,
		func() *linodev1.Kernel { return &linodev1.Kernel{} })
}

// httpListTypesProto retrieves all available Linode instance types as proto
// messages, decoded directly from the API JSON for the proto-backed list path.
func (c *Client) httpListTypesProto(ctx context.Context) ([]*linodev1.InstanceType, error) {
	return listProtoElements(ctx, c, "ListTypes", endpointTypes,
		func() *linodev1.InstanceType { return &linodev1.InstanceType{} })
}

// GetType retrieves a single Linode instance type by ID.
func (c *Client) httpGetType(ctx context.Context, typeID string) (*InstanceType, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointTypes + "/" + url.PathEscape(typeID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetType", Err: err}
	}

	defer drainClose(resp)

	var instanceType InstanceType
	if err := c.handleResponse(resp, &instanceType); err != nil {
		return nil, err
	}

	return &instanceType, nil
}

// httpGetTypeProto retrieves one instance type as a proto message.
func (c *Client) httpGetTypeProto(ctx context.Context, typeID string) (*linodev1.InstanceType, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointTypes + "/" + url.PathEscape(typeID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetType", Err: err}
	}

	defer drainClose(resp)

	instanceType := &linodev1.InstanceType{}
	if err := c.handleProtoResponse(resp, instanceType); err != nil {
		return nil, err
	}

	return instanceType, nil
}

// httpGetKernelProto retrieves one kernel as a proto message.
func (c *Client) httpGetKernelProto(ctx context.Context, kernelID string) (*linodev1.Kernel, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointKernels + "/" + url.PathEscape(kernelID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetKernel", Err: err}
	}

	defer drainClose(resp)

	kernel := &linodev1.Kernel{}
	if err := c.handleProtoResponse(resp, kernel); err != nil {
		return nil, err
	}

	return kernel, nil
}

// httpListImagesProto retrieves images as proto messages for the proto-backed
// list path, decoded directly from the same /images endpoint httpListImages uses.
func (c *Client) httpListImagesProto(ctx context.Context) ([]*linodev1.Image, error) {
	return listProtoElements(ctx, c, "ListImages", endpointImages,
		func() *linodev1.Image { return &linodev1.Image{} })
}

// GetImage retrieves a single Linode image by ID.
func (c *Client) httpGetImage(ctx context.Context, imageID string) (*Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImages + "/" + escapeImageIDSegment(imageID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetImage", Err: err}
	}

	defer drainClose(resp)

	var image Image
	if err := c.handleResponse(resp, &image); err != nil {
		return nil, err
	}

	return &image, nil
}

// httpGetImageProto retrieves an image and decodes it as a proto message.
func (c *Client) httpGetImageProto(ctx context.Context, imageID string) (*linodev1.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImages + "/" + escapeImageIDSegment(imageID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetImage", Err: err}
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

	endpoint := endpointImages + "/" + escapeImageIDSegment(imageID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteImage", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpReplicateImageProto replicates an image and decodes the response as a proto
// message for the proto-backed write path.
func (c *Client) httpReplicateImageProto(ctx context.Context, imageID string, req *ReplicateImageRequest) (*linodev1.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImages + "/" + escapeImageIDSegment(imageID) + "/regions"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "ReplicateImage", Err: err}
	}

	defer drainClose(resp)

	image := &linodev1.Image{}
	if err := c.handleProtoResponse(resp, image); err != nil {
		return nil, err
	}

	return image, nil
}

// httpUpdateImageProto updates an image and decodes the response as a proto message.
func (c *Client) httpUpdateImageProto(ctx context.Context, imageID string, req *UpdateImageRequest) (*linodev1.Image, error) {
	if req == nil {
		return nil, ErrUpdateImageRequestRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImages + "/" + escapeImageIDSegment(imageID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateImage", Err: err}
	}

	defer drainClose(resp)

	image := &linodev1.Image{}
	if err := c.handleProtoResponse(resp, image); err != nil {
		return nil, err
	}

	return image, nil
}

// httpListImageShareGroupsProto retrieves owned image share groups as proto
// messages for the proto-backed list path. The page/page_size pair flows through
// withPaginationQuery, so the request matches httpListImageShareGroups.
func (c *Client) httpListImageShareGroupsProto(ctx context.Context, page, pageSize int) ([]*linodev1.ImageShareGroup, error) {
	return listProtoElementsPaginated(ctx, c, "ListImageShareGroups", endpointImageShareGroups, page, pageSize,
		func() *linodev1.ImageShareGroup { return &linodev1.ImageShareGroup{} })
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

// httpGetImageShareGroupProto retrieves one image share group as a proto message.
func (c *Client) httpGetImageShareGroupProto(ctx context.Context, shareGroupID int) (*linodev1.ImageShareGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointImageShareGroups+"/%d", shareGroupID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetImageShareGroup", Err: err}
	}

	defer drainClose(resp)

	shareGroup := &linodev1.ImageShareGroup{}
	if err := c.handleProtoResponse(resp, shareGroup); err != nil {
		return nil, err
	}

	return shareGroup, nil
}

// httpListImageShareGroupsByImageProto retrieves the share groups that contain an
// image as proto messages for the proto-backed list path. The endpoint is
// formatted with the same encoded image-id path httpListImageShareGroupsByImage
// uses, then listProtoElementsPaginated adds page/page_size via
// withPaginationQuery, so the runtime request matches exactly.
func (c *Client) httpListImageShareGroupsByImageProto(ctx context.Context, imageID string, page, pageSize int) ([]*linodev1.ImageShareGroup, error) {
	endpoint := endpointImages + "/" + escapeImageIDSegment(imageID) + "/sharegroups"

	return listProtoElementsPaginated(ctx, c, "ListImageShareGroupsByImage", endpoint, page, pageSize,
		func() *linodev1.ImageShareGroup { return &linodev1.ImageShareGroup{} })
}

// httpListImagesByShareGroupProto retrieves the images shared in an owned image
// share group as proto messages for the proto-backed list path. The endpoint is
// formatted with the same encoded share-group-id path httpListImagesByShareGroup
// uses, then listProtoElementsPaginated adds page/page_size via
// withPaginationQuery, so the runtime request matches exactly.
func (c *Client) httpListImagesByShareGroupProto(ctx context.Context, shareGroupID, page, pageSize int) ([]*linodev1.Image, error) {
	endpoint := endpointImageShareGroups + "/" + escapeImageShareGroupID(shareGroupID) + "/images"

	return listProtoElementsPaginated(ctx, c, "ListImagesByShareGroup", endpoint, page, pageSize,
		func() *linodev1.Image { return &linodev1.Image{} })
}

// httpListMembersByImageShareGroupProto retrieves members linked to an owned
// image share group as proto messages for the proto-backed list path. The
// endpoint is formatted with the same encoded share-group-id path
// httpListMembersByImageShareGroup uses, then listProtoElementsPaginated adds
// page/page_size via withPaginationQuery, so the runtime request matches exactly.
func (c *Client) httpListMembersByImageShareGroupProto(ctx context.Context, shareGroupID, page, pageSize int) ([]*linodev1.ImageShareGroupMember, error) {
	endpoint := endpointImageShareGroups + "/" + escapeImageShareGroupID(shareGroupID) + "/members"

	return listProtoElementsPaginated(ctx, c, "ListMembersByImageShareGroup", endpoint, page, pageSize,
		func() *linodev1.ImageShareGroupMember { return &linodev1.ImageShareGroupMember{} })
}

// httpGetImageShareGroupMemberTokenProto retrieves one image share group member
// token as a proto message.
func (c *Client) httpGetImageShareGroupMemberTokenProto(ctx context.Context, shareGroupID int, tokenUUID string) (*linodev1.ImageShareGroupMember, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroups + "/" + escapeImageShareGroupID(shareGroupID) + "/members/" + escapeImageShareGroupTokenUUID(tokenUUID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetImageShareGroupMemberToken", Err: err}
	}

	defer drainClose(resp)

	member := &linodev1.ImageShareGroupMember{}
	if err := c.handleProtoResponse(resp, member); err != nil {
		return nil, err
	}

	return member, nil
}

// httpUpdateImageShareGroupMemberProto updates a member token and decodes the
// response as a proto message for the proto-backed write path.
func (c *Client) httpUpdateImageShareGroupMemberProto(ctx context.Context, shareGroupID int, tokenUUID string, req *UpdateImageShareGroupMemberRequest) (*linodev1.ImageShareGroupMember, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroups + "/" + escapeImageShareGroupID(shareGroupID) + "/members/" + escapeImageShareGroupTokenUUID(tokenUUID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateImageShareGroupMember", Err: err}
	}

	defer drainClose(resp)

	member := &linodev1.ImageShareGroupMember{}
	if err := c.handleProtoResponse(resp, member); err != nil {
		return nil, err
	}

	return member, nil
}

// httpCreateImageShareGroupProto creates a share group and decodes the response
// as a proto message for the proto-backed write path.
func (c *Client) httpCreateImageShareGroupProto(ctx context.Context, req *CreateImageShareGroupRequest) (*linodev1.ImageShareGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointImageShareGroups, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateImageShareGroup", Err: err}
	}

	defer drainClose(resp)

	shareGroup := &linodev1.ImageShareGroup{}
	if err := c.handleProtoResponse(resp, shareGroup); err != nil {
		return nil, err
	}

	return shareGroup, nil
}

// httpAddImageShareGroupImagesProto adds images to a share group and decodes the
// response image as a proto message for the proto-backed write path.
func (c *Client) httpAddImageShareGroupImagesProto(ctx context.Context, shareGroupID int, req *AddImageShareGroupImagesRequest) (*linodev1.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroups + "/" + escapeImageShareGroupID(shareGroupID) + "/images"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "AddImageShareGroupImages", Err: err}
	}

	defer drainClose(resp)

	image := &linodev1.Image{}
	if err := c.handleProtoResponse(resp, image); err != nil {
		return nil, err
	}

	return image, nil
}

// httpAddImageShareGroupMembersProto adds members to a share group and decodes the
// returned parent share group as a proto message for the proto-backed write path.
func (c *Client) httpAddImageShareGroupMembersProto(ctx context.Context, shareGroupID int, req *AddImageShareGroupMembersRequest) (*linodev1.ImageShareGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroups + "/" + escapeImageShareGroupID(shareGroupID) + "/members"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "AddImageShareGroupMembers", Err: err}
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

	endpoint := endpointImageShareGroups + "/" + escapeImageShareGroupID(shareGroupID) + "/images/" + escapeImageShareGroupID(imageID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteImageShareGroupImage", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpUpdateImageShareGroupProto updates a share group and decodes the response
// as a proto message for the proto-backed write path.
func (c *Client) httpUpdateImageShareGroupProto(ctx context.Context, shareGroupID int, req *UpdateImageShareGroupRequest) (*linodev1.ImageShareGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroups + "/" + escapeImageShareGroupID(shareGroupID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateImageShareGroup", Err: err}
	}

	defer drainClose(resp)

	shareGroup := &linodev1.ImageShareGroup{}
	if err := c.handleProtoResponse(resp, shareGroup); err != nil {
		return nil, err
	}

	return shareGroup, nil
}

// httpUpdateImageShareGroupImageProto updates a shared image and decodes the
// response as a proto message for the proto-backed write path.
func (c *Client) httpUpdateImageShareGroupImageProto(ctx context.Context, shareGroupID int, imageID string, req *UpdateImageShareGroupImageRequest) (*linodev1.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroups + "/" + escapeImageShareGroupID(shareGroupID) + "/images/" + escapeImageIDSegment(imageID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateImageShareGroupImage", Err: err}
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

	endpoint := fmt.Sprintf(endpointImageShareGroups+"/%d", shareGroupID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteImageShareGroup", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpListImageShareGroupTokensProto retrieves image share group tokens for the
// user as proto messages for the proto-backed list path. The page/page_size pair
// flows through withPaginationQuery, so the request matches
// httpListImageShareGroupTokens.
func (c *Client) httpListImageShareGroupTokensProto(ctx context.Context, page, pageSize int) ([]*linodev1.ImageShareGroupToken, error) {
	return listProtoElementsPaginated(ctx, c, "ListImageShareGroupTokens", endpointImageShareGroups+"/tokens", page, pageSize,
		func() *linodev1.ImageShareGroupToken { return &linodev1.ImageShareGroupToken{} })
}

// httpCreateImageShareGroupTokenProto creates a membership token and decodes the
// response as a proto message for the proto-backed write path.
func (c *Client) httpCreateImageShareGroupTokenProto(ctx context.Context, req *CreateImageShareGroupTokenRequest) (*linodev1.ImageShareGroupToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointImageShareGroupMembershipCreate, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateImageShareGroupToken", Err: err}
	}

	defer drainClose(resp)

	token := &linodev1.ImageShareGroupToken{}
	if err := c.handleProtoResponse(resp, token); err != nil {
		return nil, err
	}

	return token, nil
}

// httpGetImageShareGroupTokenProto retrieves one image share group token as a
// proto message.
func (c *Client) httpGetImageShareGroupTokenProto(ctx context.Context, tokenUUID string) (*linodev1.ImageShareGroupToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroups + "/tokens/" + escapeImageShareGroupTokenUUID(tokenUUID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetImageShareGroupToken", Err: err}
	}

	defer drainClose(resp)

	token := &linodev1.ImageShareGroupToken{}
	if err := c.handleProtoResponse(resp, token); err != nil {
		return nil, err
	}

	return token, nil
}

// httpListImagesByShareGroupTokenProto retrieves the images available through an
// image share group token as proto messages for the proto-backed list path. The
// endpoint is formatted with the same encoded token-uuid path
// httpListImagesByShareGroupToken uses, then listProtoElementsPaginated adds
// page/page_size via withPaginationQuery, so the runtime request matches exactly.
func (c *Client) httpListImagesByShareGroupTokenProto(ctx context.Context, tokenUUID string, page, pageSize int) ([]*linodev1.Image, error) {
	endpoint := endpointImageShareGroups + "/tokens/" + escapeImageShareGroupTokenUUID(tokenUUID) + "/sharegroup/images"

	return listProtoElementsPaginated(ctx, c, "ListImagesByShareGroupToken", endpoint, page, pageSize,
		func() *linodev1.Image { return &linodev1.Image{} })
}

// httpUpdateImageShareGroupTokenProto updates a membership token label and decodes
// the response as a proto message for the proto-backed write path.
func (c *Client) httpUpdateImageShareGroupTokenProto(ctx context.Context, tokenUUID string, req *UpdateImageShareGroupTokenRequest) (*linodev1.ImageShareGroupToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroupMembershipCreate + "/" + escapeImageShareGroupTokenUUID(tokenUUID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateImageShareGroupToken", Err: err}
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

// httpGetImageShareGroupByTokenProto resolves a token to its parent share group
// as a proto message.
func (c *Client) httpGetImageShareGroupByTokenProto(ctx context.Context, tokenUUID string) (*linodev1.ImageShareGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroups + "/tokens/" + escapeImageShareGroupTokenUUID(tokenUUID) + "/sharegroup"

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetImageShareGroupByToken", Err: err}
	}

	defer drainClose(resp)

	shareGroup := &linodev1.ImageShareGroup{}
	if err := c.handleProtoResponse(resp, shareGroup); err != nil {
		return nil, err
	}

	return shareGroup, nil
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

// DeleteImageShareGroupMemberToken revokes one accepted membership token from an owned image share group.
func (c *Client) httpDeleteImageShareGroupMemberToken(ctx context.Context, shareGroupID int, tokenUUID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointImageShareGroups + "/" + escapeImageShareGroupID(shareGroupID) + "/members/" + escapeImageShareGroupTokenUUID(tokenUUID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteImageShareGroupMemberToken", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpCreateImageProto creates an image and decodes the response as a proto message.
func (c *Client) httpCreateImageProto(ctx context.Context, req *CreateImageRequest) (*linodev1.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointImages, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateImage", Err: err}
	}

	defer drainClose(resp)

	image := &linodev1.Image{}
	if err := c.handleProtoResponse(resp, image); err != nil {
		return nil, err
	}

	return image, nil
}

// httpUploadImageProto creates an image upload target and returns the one-time
// upload URL plus the created image decoded as a proto message. The endpoint body
// is {image, upload_to}; the image sub-object is protojson-decoded into the proto
// Image element (DiscardUnknown) so the output matches the Python serializer.
func (c *Client) httpUploadImageProto(ctx context.Context, req *UploadImageRequest) (*linodev1.Image, string, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointImagesUpload, req)
	if err != nil {
		return nil, "", &NetworkError{Operation: "UploadImage", Err: err}
	}

	defer drainClose(resp)

	var envelope struct {
		Image    json.RawMessage `json:"image"`
		UploadTo string          `json:"upload_to"`
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

// httpListStackScriptsProto retrieves StackScripts as proto messages for the
// proto-backed list path. The endpoint returns a {data, page, ...} page
// envelope, so listProtoElements reads the data field; the tool filters
// (is_public / mine / label_contains) are applied client-side by the factory.
func (c *Client) httpListStackScriptsProto(ctx context.Context) ([]*linodev1.StackScript, error) {
	return listProtoElements(ctx, c, "ListStackScripts", endpointStackScripts,
		func() *linodev1.StackScript { return &linodev1.StackScript{} })
}

// GetStackScript retrieves a single StackScript by ID.
func (c *Client) httpGetStackScript(ctx context.Context, stackScriptID int) (*StackScript, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointStackScripts + "/" + url.PathEscape(strconv.Itoa(stackScriptID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetStackScript", Err: err}
	}

	defer drainClose(resp)

	var script StackScript
	if err := c.handleResponse(resp, &script); err != nil {
		return nil, err
	}

	return &script, nil
}

// httpGetStackScriptProto retrieves one StackScript as a proto message.
func (c *Client) httpGetStackScriptProto(ctx context.Context, stackScriptID int) (*linodev1.StackScript, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointStackScripts + "/" + url.PathEscape(strconv.Itoa(stackScriptID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetStackScript", Err: err}
	}

	defer drainClose(resp)

	script := &linodev1.StackScript{}
	if err := c.handleProtoResponse(resp, script); err != nil {
		return nil, err
	}

	return script, nil
}

// httpCreateStackScriptProto creates a StackScript and decodes the response into
// the StackScript proto element so the write tool emits the same shape as the
// StackScript read path.
func (c *Client) httpCreateStackScriptProto(ctx context.Context, req *CreateStackScriptRequest) (*linodev1.StackScript, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointStackScripts, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateStackScript", Err: err}
	}

	defer drainClose(resp)

	script := &linodev1.StackScript{}
	if err := c.handleProtoResponse(resp, script); err != nil {
		return nil, err
	}

	return script, nil
}

// httpUpdateStackScriptProto updates a StackScript and decodes the response into
// the StackScript proto element.
func (c *Client) httpUpdateStackScriptProto(ctx context.Context, stackScriptID int, req *UpdateStackScriptRequest) (*linodev1.StackScript, error) {
	if stackScriptID <= 0 {
		return nil, ErrStackScriptIDPositive
	}

	if updateStackScriptRequestEmpty(req) {
		return nil, ErrStackScriptUpdateRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedStackScriptID := strings.ReplaceAll(url.PathEscape(strconv.Itoa(stackScriptID)), ".", "%2E")
	endpoint := endpointStackScripts + "/" + encodedStackScriptID

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateStackScript", Err: err}
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

	endpoint := fmt.Sprintf(endpointStackScripts+"/%d", stackScriptID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteStackScript", Err: err}
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

// httpCreateInstanceProto creates a Linode instance and decodes the response as
// a proto message for the proto-backed write path.
func (c *Client) httpCreateInstanceProto(ctx context.Context, req *CreateInstanceRequest) (*linodev1.Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointInstances, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateInstance", Err: err}
	}

	defer drainClose(resp)

	instance := &linodev1.Instance{}
	if err := c.handleProtoResponse(resp, instance); err != nil {
		return nil, err
	}

	return instance, nil
}

// httpUpdateInstanceProto updates a Linode instance and decodes the response as
// a proto message for the proto-backed write path.
func (c *Client) httpUpdateInstanceProto(ctx context.Context, instanceID int, req *UpdateInstanceRequest) (*linodev1.Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstances+"/%d", instanceID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateInstance", Err: err}
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
