package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

const (
	instanceConfigsPageSizeMin    = 25
	instanceConfigsPageSizeMax    = 500
	configUpdateNoFields          = "at least one configuration field must be provided"
	configInterfaceUpdateNoFields = "at least one interface update field must be provided"
	interfaceJSONObjRequired      = "interface must be a JSON object"
	paramConfigInterfaceID        = "interface_id"

	configInterfacePurposeVLAN = "vlan"
	configInterfacePurposeVPC  = "vpc"
)

// NewLinodeInstanceConfigListTool creates a tool for listing configuration profiles on a Linode instance.
func NewLinodeInstanceConfigListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolSubresourcePaginated(
		cfg,
		"linode_instance_config_list",
		"Lists configuration profiles for a Linode instance with optional pagination.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		protoListPathID{
			option: mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			parse: instanceConfigLinodeIDFromTool,
		},
		instanceConfigsPaginationFromTool,
		func(ctx context.Context, client *linode.Client, linodeID, page, pageSize int) ([]*linodev1.InstanceConfig, error) {
			return client.ListInstanceConfigsProto(ctx, linodeID, page, pageSize)
		},
		nil,
		instanceConfigListResponse,
	)

	return tool, profiles.CapRead, handler
}

func instanceConfigListResponse(items []*linodev1.InstanceConfig, count int32, filter *string) *linodev1.InstanceConfigListResponse {
	return &linodev1.InstanceConfigListResponse{Count: count, Filter: filter, Configs: items}
}

// NewLinodeInstanceConfigDeleteTool creates a tool for deleting a configuration profile from a Linode instance.
func NewLinodeInstanceConfigDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_config_delete",
		"Deletes a configuration profile from a Linode instance. WARNING: This is irreversible.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("config_id", mcp.Required(),
				mcp.Description("The ID of the configuration profile to delete")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm deletion. This action is irreversible. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceConfigDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleInstanceConfigDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	configID, validationMessage := instanceConfigIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_instance_config_delete", httpMethodDelete,
			fmt.Sprintf("/linode/instances/%d/configs/%d", linodeID, configID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetInstanceConfig(ctx, linodeID, configID)
			})
	}

	if result := RequireConfirm(request, "This is irreversible. The configuration profile will be permanently deleted. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.DeleteInstanceConfig(ctx, linodeID, configID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to remove configuration profile %d from instance %d: %v", configID, linodeID, err)), nil
	}

	response := struct {
		Message  string `json:"message"`
		LinodeID int    `json:"linode_id"`
		ConfigID int    `json:"config_id"`
	}{
		Message:  fmt.Sprintf("Configuration profile %d deleted from instance %d successfully", configID, linodeID),
		LinodeID: linodeID,
		ConfigID: configID,
	}

	return MarshalToolResponse(response)
}

func instanceConfigLinodeIDFromTool(request *mcp.CallToolRequest) (int, string) {
	args := request.GetArguments()
	if _, exists := args["linode_id"]; !exists {
		return 0, ErrLinodeIDRequired.Error()
	}

	linodeID, validationMessage := optionalPaginationInt(args, "linode_id", 1, 0)
	if validationMessage != "" {
		return 0, validationMessage
	}

	return linodeID, ""
}

func instanceConfigIDFromTool(request *mcp.CallToolRequest) (int, string) {
	args := request.GetArguments()
	if _, exists := args["config_id"]; !exists {
		return 0, ErrConfigIDRequired.Error()
	}

	configID, validationMessage := optionalPaginationInt(args, "config_id", 1, 0)
	if validationMessage != "" {
		return 0, validationMessage
	}

	return configID, ""
}

func instanceConfigsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", instanceConfigsPageSizeMin, instanceConfigsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

// NewLinodeInstanceConfigGetTool creates a tool for retrieving a specific configuration profile on a Linode instance.
func NewLinodeInstanceConfigGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_config_get",
		"Retrieves details of a specific configuration profile on a Linode instance.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("config_id", mcp.Required(),
				mcp.Description("The ID of the configuration profile to retrieve")),
		},
		handleInstanceConfigGetRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleInstanceConfigGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	configID, validationMessage := configIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	configProfile, err := client.GetInstanceConfig(ctx, linodeID, configID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve config %d for instance %d: %v", configID, linodeID, err)), nil
	}

	return MarshalToolResponse(configProfile)
}

func configIDFromTool(request *mcp.CallToolRequest) (int, string) {
	args := request.GetArguments()
	if _, exists := args["config_id"]; !exists {
		return 0, ErrConfigIDRequired.Error()
	}

	configID, validationMessage := optionalPaginationInt(args, "config_id", 1, 0)
	if validationMessage != "" {
		return 0, validationMessage
	}

	return configID, ""
}

// NewLinodeInstanceConfigInterfacesListTool creates a tool for listing interfaces on a Linode configuration profile.
func NewLinodeInstanceConfigInterfacesListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolSubresource2(
		cfg,
		"linode_instance_config_interface_list",
		"Lists interfaces assigned to a specific configuration profile on a Linode instance.",
		protoListPathID{
			option: mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			parse: instanceConfigLinodeIDFromTool,
		},
		protoListPathID{
			option: mcp.WithNumber("config_id", mcp.Required(),
				mcp.Description("The ID of the configuration profile")),
			parse: configIDFromTool,
		},
		func(ctx context.Context, client *linode.Client, linodeID, configID int) ([]*linodev1.ConfigInterfaceResponse, error) {
			return client.ListInstanceConfigInterfacesProto(ctx, linodeID, configID)
		},
		nil,
		configInterfaceListResponse,
	)

	return tool, profiles.CapRead, handler
}

func configInterfaceListResponse(items []*linodev1.ConfigInterfaceResponse, count int32, filter *string) *linodev1.ConfigInterfaceListResponse {
	return &linodev1.ConfigInterfaceListResponse{Count: count, Filter: filter, Interfaces: items}
}

// NewLinodeInstanceConfigCreateTool creates a tool for creating a Linode configuration profile.
func NewLinodeInstanceConfigCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_config_create",
		"Creates a configuration profile on a Linode instance. WARNING: This changes instance boot configuration.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithString("label", mcp.Required(),
				mcp.Description("Label for the configuration profile")),
			mcp.WithObject("devices", mcp.Required(),
				mcp.Description("Object mapping device slots to disk/volume IDs, e.g. {\"sda\":{\"disk_id\":123}}")),
			mcp.WithString("kernel",
				mcp.Description("Kernel ID to boot, e.g. linode/latest-64bit")),
			mcp.WithString("comments",
				mcp.Description("Optional comments for the configuration profile")),
			mcp.WithNumber("memory_limit",
				mcp.Description("Optional memory limit in MB")),
			mcp.WithString("root_device",
				mcp.Description("Root device to boot, e.g. /dev/sda")),
			mcp.WithString("run_level",
				mcp.Description("Run level: default, single, or binbash")),
			mcp.WithString("virt_mode",
				mcp.Description("Virtualization mode: paravirt or fullvirt")),
			mcp.WithString("helpers",
				mcp.Description("Optional helpers JSON object")),
			mcp.WithString("interfaces",
				mcp.Description("Optional interfaces JSON array")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm configuration profile creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceConfigCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceConfigCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, linodeIDOK := getPositiveIntArgument(request, "linode_id")
	if !linodeIDOK {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	if IsDryRun(request) {
		if _, errText := buildCreateConfigRequest(request); errText != "" {
			return mcp.NewToolResultError(errText), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_instance_config_create", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/configs", linodeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetInstance(ctx, linodeID) },
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return instanceConfigCreateSideEffects(ctx, request.GetString("label", ""), linodeID)
			})
	}

	if result := RequireConfirm(request, "This creates a configuration profile on the instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	createReq, errText := buildCreateConfigRequest(request)
	if errText != "" {
		return mcp.NewToolResultError(errText), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	createdConfig, err := client.CreateInstanceConfigProto(ctx, linodeID, &createReq)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create configuration profile for instance %d: %v", linodeID, err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.InstanceConfigWriteResponse{
		Message: fmt.Sprintf("Configuration profile '%s' (ID: %d) created on instance %d", createdConfig.GetLabel(), createdConfig.GetId(), linodeID),
		Config:  createdConfig,
	})
}

// NewLinodeInstanceConfigInterfaceAddTool creates a tool for appending a network interface to a Linode configuration profile.
func NewLinodeInstanceConfigInterfaceAddTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_config_interface_add",
		"Adds a network interface to a Linode configuration profile. WARNING: This changes instance network configuration and requires a reboot to take effect.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("config_id", mcp.Required(),
				mcp.Description("The ID of the configuration profile")),
			mcp.WithString("purpose", mcp.Required(),
				mcp.Enum("public", "vlan", "vpc"),
				mcp.Description("The interface purpose (required).")),
			mcp.WithString("label",
				mcp.Description("Interface label. Required for vlan interfaces.")),
			mcp.WithString("ipam_address",
				mcp.Description("Private CIDR address for vlan interfaces.")),
			mcp.WithBoolean("primary",
				mcp.Description("Whether this is the primary non-vlan interface.")),
			mcp.WithNumber("subnet_id",
				mcp.Description("The VPC subnet ID. Required for vpc interfaces.")),
			mcp.WithArray("ip_ranges",
				mcp.Description("IPv4 CIDR VPC subnet ranges routed to this interface.")),
			mcp.WithObject("ipv4",
				mcp.Description("VPC IPv4 configuration for this interface.")),
			mcp.WithObject("ipv6",
				mcp.Description("VPC IPv6 configuration for this interface.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm configuration profile interface creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceConfigInterfaceAddRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceConfigInterfaceAddRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, linodeIDOK := getPositiveIntArgument(request, "linode_id")
	if !linodeIDOK {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	configID, configIDOK := getPositiveIntArgument(request, "config_id")
	if !configIDOK {
		return mcp.NewToolResultError("config_id must be a positive integer"), nil
	}

	if IsDryRun(request) {
		if _, errText := configInterfaceFromTool(request); errText != "" {
			return mcp.NewToolResultError(errText), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_config_interface_add", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/configs/%d/interfaces", linodeID, configID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetInstanceConfig(ctx, linodeID, configID)
			})
	}

	if result := RequireConfirm(request, "This adds a network interface to the configuration profile. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	configInterface, errText := configInterfaceFromTool(request)
	if errText != "" {
		return mcp.NewToolResultError(errText), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	createdInterface, err := client.AddInstanceConfigInterfaceProto(ctx, linodeID, configID, configInterface)
	if err != nil {
		return mcp.NewToolResultError(formatAddConfigInterfaceError(linodeID, configID, err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.ConfigInterfaceWriteResponse{
		Message:   fmt.Sprintf("Configuration profile interface added to config %d on instance %d", configID, linodeID),
		Interface: createdInterface,
	})
}

func configInterfaceFromTool(request *mcp.CallToolRequest) (*linode.ConfigInterface, string) {
	purpose, validationMessage := stringArgument(request, "purpose", true)
	if validationMessage != "" {
		return nil, validationMessage
	}

	if !validConfigInterfacePurpose(purpose) {
		return nil, "purpose must be public, vlan, or vpc"
	}

	configInterface := &linode.ConfigInterface{Purpose: purpose}
	if validationMessage := applyConfigInterfaceAddFields(request, configInterface); validationMessage != "" {
		return nil, validationMessage
	}

	return configInterface, ""
}

func applyConfigInterfaceAddFields(request *mcp.CallToolRequest, configInterface *linode.ConfigInterface) string {
	args := request.GetArguments()

	if validationMessage := applyConfigInterfaceStringFields(request, args, configInterface); validationMessage != "" {
		return validationMessage
	}

	return applyConfigInterfaceNetworkFields(request, args, configInterface)
}

func applyConfigInterfaceStringFields(request *mcp.CallToolRequest, args map[string]any, configInterface *linode.ConfigInterface) string {
	if _, exists := args["label"]; exists {
		label, validationMessage := stringArgument(request, "label", false)
		if validationMessage != "" {
			return validationMessage
		}

		configInterface.Label = &label
	}

	if configInterface.Purpose == configInterfacePurposeVLAN && (configInterface.Label == nil || *configInterface.Label == "") {
		return "label is required for vlan interfaces"
	}

	if _, exists := args["ipam_address"]; exists {
		ipamAddress, validationMessage := stringArgument(request, "ipam_address", false)
		if validationMessage != "" {
			return validationMessage
		}

		configInterface.IPAMAddress = &ipamAddress
	}

	return ""
}

func applyConfigInterfaceNetworkFields(request *mcp.CallToolRequest, args map[string]any, configInterface *linode.ConfigInterface) string {
	if _, exists := args["subnet_id"]; exists {
		subnetID, ok := getPositiveIntArgument(request, "subnet_id")
		if !ok {
			return "subnet_id must be a positive integer"
		}

		configInterface.SubnetID = &subnetID
	}

	if configInterface.Purpose == configInterfacePurposeVPC && configInterface.SubnetID == nil {
		return "subnet_id is required for vpc interfaces"
	}

	if rawPrimary, exists := args["primary"]; exists {
		primary, validationMessage := boolToolArg(rawPrimary, "primary")
		if validationMessage != "" {
			return validationMessage
		}

		configInterface.Primary = &primary
	}

	if _, exists := args["ip_ranges"]; exists {
		ipRanges, validationMessage := stringSliceFromToolArg(args["ip_ranges"], "ip_ranges")
		if validationMessage != "" {
			return validationMessage
		}

		configInterface.IPRanges = ipRanges
	}

	if validationMessage := applyConfigInterfaceObject(args, "ipv4", &configInterface.IPv4); validationMessage != "" {
		return validationMessage
	}

	return applyConfigInterfaceObject(args, "ipv6", &configInterface.IPv6)
}

// applyConfigInterfaceObject reads a JSON object argument (ipv4/ipv6) and stores
// its JSON verbatim so the legacy config-interface endpoint receives the VPC IP
// fields the caller sent without this tool needing to track the shape. The value
// must be an object; a scalar or array is rejected so the request body stays
// valid JSON.
func applyConfigInterfaceObject(args map[string]any, name string, target *json.RawMessage) string {
	raw, exists := args[name]
	if !exists {
		return ""
	}

	object, isObject := raw.(map[string]any)
	if !isObject {
		return name + " must be an object"
	}

	encoded, err := json.Marshal(object)
	if err != nil {
		return name + " must be an object"
	}

	*target = json.RawMessage(encoded)

	return ""
}

func formatAddConfigInterfaceError(linodeID, configID int, err error) string {
	return "Failed to add configuration profile interface to config " + strconv.Itoa(configID) + " for instance " + strconv.Itoa(linodeID) + ": " + err.Error()
}

// NewLinodeInstanceConfigInterfaceUpdateTool creates a tool for updating a configuration profile interface.
func NewLinodeInstanceConfigInterfaceUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_config_interface_update",
		"Updates a network interface on a Linode configuration profile. WARNING: This changes instance network configuration and requires a reboot to take effect.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("config_id", mcp.Required(),
				mcp.Description("The ID of the configuration profile")),
			mcp.WithNumber(paramConfigInterfaceID, mcp.Required(),
				mcp.Description("The ID of the configuration profile interface")),
			mcp.WithArray("ip_ranges",
				mcp.Description("IPv4 ranges routed to this interface.")),
			mcp.WithObject("ipv4",
				mcp.Description("IPv4 configuration for this interface.")),
			mcp.WithBoolean("primary",
				mcp.Description("Whether this is the primary interface.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm configuration profile interface update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceConfigInterfaceUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceConfigInterfaceUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, linodeIDOK := getPositiveIntArgument(request, "linode_id")
	if !linodeIDOK {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	configID, configIDOK := getPositiveIntArgument(request, "config_id")
	if !configIDOK {
		return mcp.NewToolResultError("config_id must be a positive integer"), nil
	}

	interfaceID, interfaceIDOK := getPositiveIntArgument(request, paramConfigInterfaceID)
	if !interfaceIDOK {
		return mcp.NewToolResultError("interface_id must be a positive integer"), nil
	}

	if IsDryRun(request) {
		if _, errText := updateConfigInterfaceFromTool(request); errText != "" {
			return mcp.NewToolResultError(errText), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_config_interface_update", "PUT",
			fmt.Sprintf("/linode/instances/%d/configs/%d/interfaces/%d", linodeID, configID, interfaceID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetInstanceConfigInterface(ctx, linodeID, configID, interfaceID)
			})
	}

	if result := RequireConfirm(request, "This updates a network interface on the configuration profile. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	updateReq, errText := updateConfigInterfaceFromTool(request)
	if errText != "" {
		return mcp.NewToolResultError(errText), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	updatedInterface, err := client.UpdateInstanceConfigInterfaceProto(ctx, linodeID, configID, interfaceID, updateReq)
	if err != nil {
		return mcp.NewToolResultError(formatUpdateConfigInterfaceError(linodeID, configID, interfaceID, err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.ConfigInterfaceWriteResponse{
		Message:   fmt.Sprintf("Configuration profile interface %d updated on config %d for instance %d", interfaceID, configID, linodeID),
		Interface: updatedInterface,
	})
}

func updateConfigInterfaceFromTool(request *mcp.CallToolRequest) (*linode.UpdateConfigInterfaceRequest, string) {
	args := request.GetArguments()
	configInterface := &linode.UpdateConfigInterfaceRequest{}

	if _, exists := args["ip_ranges"]; exists {
		ipRanges, validationMessage := stringSliceFromToolArg(args["ip_ranges"], "ip_ranges")
		if validationMessage != "" {
			return nil, validationMessage
		}

		configInterface.IPRanges = ipRanges
	}

	if validationMessage := applyConfigInterfaceObject(args, "ipv4", &configInterface.IPv4); validationMessage != "" {
		return nil, validationMessage
	}

	if rawPrimary, exists := args["primary"]; exists {
		primary, validationMessage := boolToolArg(rawPrimary, "primary")
		if validationMessage != "" {
			return nil, validationMessage
		}

		configInterface.Primary = &primary
	}

	if configInterface.Primary == nil && configInterface.IPv4 == nil && configInterface.IPRanges == nil {
		return nil, configInterfaceUpdateNoFields
	}

	return configInterface, ""
}

func formatUpdateConfigInterfaceError(linodeID, configID, interfaceID int, err error) string {
	return "Failed to update configuration profile interface " + strconv.Itoa(interfaceID) + " in config " + strconv.Itoa(configID) + " for instance " + strconv.Itoa(linodeID) + ": " + err.Error()
}

// NewLinodeInstanceConfigInterfacesReorderTool creates a tool for reordering configuration profile interfaces.
func NewLinodeInstanceConfigInterfacesReorderTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_config_interface_reorder",
		"Reorders interfaces on a Linode configuration profile. WARNING: This changes network interface ordering.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("config_id", mcp.Required(),
				mcp.Description("The ID of the configuration profile")),
			mcp.WithArray("ids", mcp.Required(),
				mcp.Description("Existing configuration profile interface IDs in the desired order, e.g. [101,102,103]")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm configuration interface reorder. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceConfigInterfacesReorderRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceConfigInterfacesReorderRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	configID, validationMessage := instanceConfigIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		if _, reorderMessage := buildReorderConfigInterfacesRequest(request); reorderMessage != "" {
			return mcp.NewToolResultError(reorderMessage), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_config_interface_reorder", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/configs/%d/interfaces/order", linodeID, configID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetInstanceConfig(ctx, linodeID, configID)
			})
	}

	if result := RequireConfirm(request, "This reorders network interfaces on the configuration profile. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	reorderReq, validationMessage := buildReorderConfigInterfacesRequest(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.ReorderInstanceConfigInterfaces(ctx, linodeID, configID, reorderReq); err != nil {
		return mcp.NewToolResultError(formatReorderConfigInterfacesError(linodeID, configID, err)), nil
	}

	response := struct {
		Message  string `json:"message"`
		LinodeID int    `json:"linode_id"`
		ConfigID int    `json:"config_id"`
		IDs      []int  `json:"ids"`
	}{
		Message:  fmt.Sprintf("Configuration profile %d interfaces reordered on instance %d", configID, linodeID),
		LinodeID: linodeID,
		ConfigID: configID,
		IDs:      reorderReq.IDs,
	}

	return MarshalToolResponse(response)
}

func buildReorderConfigInterfacesRequest(request *mcp.CallToolRequest) (*linode.ReorderConfigInterfacesRequest, string) {
	args := request.GetArguments()

	value, exists := args["ids"]
	if !exists {
		return nil, "ids is required"
	}

	ids, validationMessage := intSliceFromToolArg(value, "ids")
	if validationMessage != "" {
		return nil, validationMessage
	}

	seen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			return nil, "ids must not contain duplicate interface IDs"
		}

		seen[id] = struct{}{}
	}

	return &linode.ReorderConfigInterfacesRequest{IDs: ids}, ""
}

func formatReorderConfigInterfacesError(linodeID, configID int, err error) string {
	return "Failed to reorder interfaces for configuration profile " + strconv.Itoa(configID) + " on instance " + strconv.Itoa(linodeID) + ": " + err.Error()
}

// NewLinodeInstanceConfigInterfaceGetTool creates a tool for retrieving a configuration profile interface.
func NewLinodeInstanceConfigInterfaceGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_config_interface_get",
		"Retrieves a network interface from a Linode configuration profile.",
		toolschemas.Schema("linode.mcp.v1.InstanceConfigInterfaceGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceConfigInterfaceGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleInstanceConfigInterfaceGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, linodeIDOK := getPositiveIntArgument(request, "linode_id")
	if !linodeIDOK {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	configID, configIDOK := getPositiveIntArgument(request, "config_id")
	if !configIDOK {
		return mcp.NewToolResultError(linode.ErrConfigIDPositive.Error()), nil
	}

	interfaceID, interfaceIDOK := getPositiveIntArgument(request, "interface_id")
	if !interfaceIDOK {
		return mcp.NewToolResultError(linode.ErrInterfaceIDPositive.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	configInterface, err := client.GetInstanceConfigInterfaceProto(ctx, linodeID, configID, interfaceID)
	if err != nil {
		return mcp.NewToolResultError(formatGetConfigInterfaceError(linodeID, configID, interfaceID, err)), nil
	}

	return MarshalProtoToolResponse(configInterface)
}

func formatGetConfigInterfaceError(linodeID, configID, interfaceID int, err error) string {
	return "Failed to retrieve configuration profile interface " + strconv.Itoa(interfaceID) + " from config " + strconv.Itoa(configID) + " for instance " + strconv.Itoa(linodeID) + ": " + err.Error()
}

// NewLinodeInstanceConfigInterfaceDeleteTool creates a tool for deleting a configuration profile interface.
func NewLinodeInstanceConfigInterfaceDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_config_interface_delete",
		"Deletes a network interface from a Linode configuration profile. WARNING: This changes instance network configuration.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("config_id", mcp.Required(),
				mcp.Description("The ID of the configuration profile")),
			mcp.WithNumber("interface_id", mcp.Required(),
				mcp.Description("The ID of the configuration profile interface to delete")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm configuration profile interface deletion. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceConfigInterfaceDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleInstanceConfigInterfaceDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, linodeIDOK := getPositiveIntArgument(request, "linode_id")
	if !linodeIDOK {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	configID, configIDOK := getPositiveIntArgument(request, "config_id")
	if !configIDOK {
		return mcp.NewToolResultError(linode.ErrConfigIDPositive.Error()), nil
	}

	interfaceID, interfaceIDOK := getPositiveIntArgument(request, "interface_id")
	if !interfaceIDOK {
		return mcp.NewToolResultError(linode.ErrInterfaceIDPositive.Error()), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_instance_config_interface_delete", httpMethodDelete,
			fmt.Sprintf("/linode/instances/%d/configs/%d/interfaces/%d", linodeID, configID, interfaceID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetInstanceConfigInterface(ctx, linodeID, configID, interfaceID)
			})
	}

	if result := RequireConfirm(request, "This removes a network interface from the configuration profile. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.DeleteInstanceConfigInterface(ctx, linodeID, configID, interfaceID); err != nil {
		return mcp.NewToolResultError(formatDeleteConfigInterfaceError(linodeID, configID, interfaceID, err)), nil
	}

	response := struct {
		Message     string `json:"message"`
		LinodeID    int    `json:"linode_id"`
		ConfigID    int    `json:"config_id"`
		InterfaceID int    `json:"interface_id"`
	}{
		Message:     fmt.Sprintf("Configuration profile interface %d removed from config %d on instance %d", interfaceID, configID, linodeID),
		LinodeID:    linodeID,
		ConfigID:    configID,
		InterfaceID: interfaceID,
	}

	return MarshalToolResponse(response)
}

func formatDeleteConfigInterfaceError(linodeID, configID, interfaceID int, err error) string {
	return "Failed to remove configuration profile interface " + strconv.Itoa(interfaceID) + " from config " + strconv.Itoa(configID) + " for instance " + strconv.Itoa(linodeID) + ": " + err.Error()
}

// NewLinodeInstanceConfigUpdateTool creates a tool for updating a Linode configuration profile.
func NewLinodeInstanceConfigUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_config_update",
		"Updates a configuration profile on a Linode instance. WARNING: This changes instance boot configuration.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("config_id", mcp.Required(),
				mcp.Description("The ID of the configuration profile")),
			mcp.WithString("label",
				mcp.Description("Updated label for the configuration profile")),
			mcp.WithObject("devices",
				mcp.Description("Object mapping device slots to disk/volume IDs")),
			mcp.WithString("kernel",
				mcp.Description("Kernel ID to boot, e.g. linode/latest-64bit")),
			mcp.WithString("comments",
				mcp.Description("Optional comments for the configuration profile")),
			mcp.WithNumber("memory_limit",
				mcp.Description("Optional memory limit in MB")),
			mcp.WithString("root_device",
				mcp.Description("Root device to boot, e.g. /dev/sda")),
			mcp.WithString("run_level",
				mcp.Description("Run level: default, single, or binbash")),
			mcp.WithString("virt_mode",
				mcp.Description("Virtualization mode: paravirt or fullvirt")),
			mcp.WithObject("helpers",
				mcp.Description("Optional helpers object")),
			mcp.WithArray("interfaces",
				mcp.Description("Optional array of interface objects")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm configuration profile update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceConfigUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceConfigUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, linodeIDOK := getPositiveIntArgument(request, "linode_id")
	if !linodeIDOK {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	configID, configIDOK := getPositiveIntArgument(request, "config_id")
	if !configIDOK {
		return mcp.NewToolResultError("config_id must be a positive integer"), nil
	}

	if IsDryRun(request) {
		if _, errText := buildUpdateConfigRequest(request); errText != "" {
			return mcp.NewToolResultError(errText), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_config_update", "PUT",
			fmt.Sprintf("/linode/instances/%d/configs/%d", linodeID, configID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetInstanceConfig(ctx, linodeID, configID)
			})
	}

	if result := RequireConfirm(request, "This updates a configuration profile on the instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	updateReq, errText := buildUpdateConfigRequest(request)
	if errText != "" {
		return mcp.NewToolResultError(errText), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	updatedConfig, err := client.UpdateInstanceConfigProto(ctx, linodeID, configID, updateReq)
	if err != nil {
		return mcp.NewToolResultError(formatUpdateConfigError(linodeID, configID, err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.InstanceConfigWriteResponse{
		Message: fmt.Sprintf("Configuration profile '%s' (ID: %d) updated on instance %d", updatedConfig.GetLabel(), updatedConfig.GetId(), linodeID),
		Config:  updatedConfig,
	})
}

func formatUpdateConfigError(linodeID, configID int, err error) string {
	return "Failed to update configuration profile " + strconv.Itoa(configID) + " for instance " + strconv.Itoa(linodeID) + ": " + err.Error()
}

func buildUpdateConfigRequest(request *mcp.CallToolRequest) (*linode.UpdateConfigRequest, string) {
	req := &linode.UpdateConfigRequest{}

	var fields int

	if errText := applyUpdateConfigStringOptions(request, req, &fields); errText != "" {
		return nil, errText
	}

	if errText := applyUpdateConfigJSONOptions(request, req, &fields); errText != "" {
		return nil, errText
	}

	if fields == 0 {
		return nil, configUpdateNoFields
	}

	return req, ""
}

func applyUpdateConfigStringOptions(request *mcp.CallToolRequest, req *linode.UpdateConfigRequest, fields *int) string {
	if validationMessage := applyUpdateConfigLabel(request, req, fields); validationMessage != "" {
		return validationMessage
	}

	if validationMessage := applyUpdateConfigPlainString(request, "kernel", &req.Kernel, fields); validationMessage != "" {
		return validationMessage
	}

	if validationMessage := applyUpdateConfigPlainString(request, "comments", &req.Comments, fields); validationMessage != "" {
		return validationMessage
	}

	if validationMessage := applyUpdateConfigMemoryLimit(request, req, fields); validationMessage != "" {
		return validationMessage
	}

	if validationMessage := applyUpdateConfigPlainString(request, "root_device", &req.RootDevice, fields); validationMessage != "" {
		return validationMessage
	}

	if validationMessage := applyUpdateConfigEnum(request, "run_level", []string{"default", "single", "binbash"}, &req.RunLevel, fields); validationMessage != "" {
		return validationMessage
	}

	return applyUpdateConfigEnum(request, "virt_mode", []string{"paravirt", "fullvirt"}, &req.VirtMode, fields)
}

func applyUpdateConfigLabel(request *mcp.CallToolRequest, req *linode.UpdateConfigRequest, fields *int) string {
	label, validationMessage := stringArgument(request, "label", false)
	if validationMessage != "" {
		return validationMessage
	}

	if _, exists := request.GetArguments()["label"]; !exists {
		return ""
	}

	label = strings.TrimSpace(label)
	if label == "" {
		return errLabelRequired
	}

	req.Label = &label
	*fields++

	return ""
}

func applyUpdateConfigPlainString(request *mcp.CallToolRequest, name string, target **string, fields *int) string {
	value, validationMessage := stringArgument(request, name, false)
	if validationMessage != "" {
		return validationMessage
	}

	if _, exists := request.GetArguments()[name]; !exists {
		return ""
	}

	*target = &value
	*fields++

	return ""
}

func applyUpdateConfigMemoryLimit(request *mcp.CallToolRequest, req *linode.UpdateConfigRequest, fields *int) string {
	args := request.GetArguments()

	memoryLimit, validationMessage := optionalPaginationInt(args, "memory_limit", 1, 0)
	if validationMessage != "" {
		return validationMessage
	}

	if _, exists := args["memory_limit"]; !exists {
		return ""
	}

	req.MemoryLimit = &memoryLimit
	*fields++

	return ""
}

func applyUpdateConfigEnum(request *mcp.CallToolRequest, name string, allowed []string, target **string, fields *int) string {
	value, validationMessage := stringArgument(request, name, false)
	if validationMessage != "" {
		return validationMessage
	}

	if _, exists := request.GetArguments()[name]; !exists {
		return ""
	}

	if slices.Contains(allowed, value) {
		*target = &value
		*fields++

		return ""
	}

	return name + " must be " + strings.Join(allowed, ", ")
}

func applyUpdateConfigJSONOptions(request *mcp.CallToolRequest, req *linode.UpdateConfigRequest, fields *int) string {
	if rawDevices, exists := request.GetArguments()["devices"]; exists {
		devices, errText := parseConfigDevices(rawDevices)
		if errText != "" {
			return errText
		}

		req.Devices = &devices
		*fields++
	}

	if rawHelpers, exists := request.GetArguments()["helpers"]; exists {
		helpersJSON, validationMessage := objectJSONFromToolArg(rawHelpers, "helpers")
		if validationMessage != "" {
			return validationMessage
		}

		var helpers *linode.ConfigHelpers
		if err := strictDecodeJSON(helpersJSON, &helpers); err != nil {
			return fmt.Sprintf("invalid helpers JSON: %v", err)
		}

		if helpers == nil {
			return "helpers must be a JSON object"
		}

		req.Helpers = helpers
		*fields++
	}

	if rawInterfaces, exists := request.GetArguments()["interfaces"]; exists {
		interfaces, errText := parseConfigInterfaces(rawInterfaces)
		if errText != "" {
			return errText
		}

		req.Interfaces = &interfaces
		*fields++
	}

	return ""
}

func getPositiveIntArgument(request *mcp.CallToolRequest, name string) (int, bool) {
	args, argumentsOK := request.Params.Arguments.(map[string]any)
	if !argumentsOK {
		return 0, false
	}

	raw, argumentFound := args[name]
	if !argumentFound {
		return 0, false
	}

	switch value := raw.(type) {
	case int:
		return value, value > 0
	case int64:
		if value <= 0 || value > math.MaxInt {
			return 0, false
		}

		return int(value), true
	case float64:
		if value <= 0 || math.Trunc(value) != value || value > float64(math.MaxInt) {
			return 0, false
		}

		return int(value), true
	default:
		return 0, false
	}
}

func buildCreateConfigRequest(request *mcp.CallToolRequest) (linode.CreateConfigRequest, string) {
	label, errText := stringArgument(request, "label", true)
	if errText != "" {
		return linode.CreateConfigRequest{}, errText
	}

	label = strings.TrimSpace(label)
	if label == "" {
		return linode.CreateConfigRequest{}, "label is required"
	}

	devices, errText := parseConfigDevices(request.GetArguments()["devices"])
	if errText != "" {
		return linode.CreateConfigRequest{}, errText
	}

	req := linode.CreateConfigRequest{Label: label, Devices: devices}
	if errText := applyConfigStringOptions(request, &req); errText != "" {
		return linode.CreateConfigRequest{}, errText
	}

	if errText := applyConfigJSONOptions(request, &req); errText != "" {
		return linode.CreateConfigRequest{}, errText
	}

	return req, ""
}

func stringArgument(request *mcp.CallToolRequest, name string, required bool) (string, string) {
	args := request.GetArguments()

	raw, exists := args[name]
	if !exists {
		if required {
			return "", name + " is required"
		}

		return "", ""
	}

	value, ok := raw.(string)
	if !ok {
		return "", name + " must be a string"
	}

	return value, ""
}

// parseConfigDevices accepts the devices argument as a native object (the schema
// form) or a JSON-encoded object string (legacy form), decoding it strictly so
// unknown fields are rejected.
func parseConfigDevices(raw any) (map[string]*linode.ConfigDevice, string) {
	devicesJSON, validationMessage := objectJSONFromToolArg(raw, "devices")
	if validationMessage != "" {
		return nil, validationMessage
	}

	if devicesJSON == "" {
		return nil, "devices is required"
	}

	var devices map[string]*linode.ConfigDevice
	if err := strictDecodeJSON(devicesJSON, &devices); err != nil {
		return nil, fmt.Sprintf("invalid devices JSON: %v", err)
	}

	if devices == nil {
		return nil, "devices must be a JSON object"
	}

	if len(devices) == 0 {
		return nil, "devices must include at least one device slot"
	}

	for slot, device := range devices {
		if !validConfigDeviceSlot(slot) {
			return nil, fmt.Sprintf("device slot %s must be one of sda through sdh", slot)
		}

		if device == nil {
			return nil, fmt.Sprintf("device %s must be an object", slot)
		}

		if device.DiskID == nil && device.VolumeID == nil {
			return nil, fmt.Sprintf("device %s requires disk_id or volume_id", slot)
		}

		if device.DiskID != nil && *device.DiskID <= 0 {
			return nil, fmt.Sprintf("device %s disk_id must be greater than 0", slot)
		}

		if device.VolumeID != nil && *device.VolumeID <= 0 {
			return nil, fmt.Sprintf("device %s volume_id must be greater than 0", slot)
		}

		if device.DiskID != nil && device.VolumeID != nil {
			return nil, fmt.Sprintf("device %s can use disk_id or volume_id, not both", slot)
		}
	}

	return devices, ""
}

func validConfigDeviceSlot(slot string) bool {
	switch slot {
	case "sda", "sdb", "sdc", "sdd", "sde", "sdf", "sdg", "sdh":
		return true
	default:
		return false
	}
}

func applyConfigStringOptions(request *mcp.CallToolRequest, req *linode.CreateConfigRequest) string {
	kernel, validationMessage := stringArgument(request, "kernel", false)
	if validationMessage != "" {
		return validationMessage
	}

	if kernel != "" {
		req.Kernel = kernel
	}

	comments, validationMessage := stringArgument(request, "comments", false)
	if validationMessage != "" {
		return validationMessage
	}

	if comments != "" {
		req.Comments = comments
	}

	args := request.GetArguments()

	memoryLimit, validationMessage := optionalPaginationInt(args, "memory_limit", 1, 0)
	if validationMessage != "" {
		return validationMessage
	}

	if memoryLimit != 0 {
		req.MemoryLimit = memoryLimit
	}

	rootDevice, validationMessage := stringArgument(request, "root_device", false)
	if validationMessage != "" {
		return validationMessage
	}

	if rootDevice != "" {
		req.RootDevice = rootDevice
	}

	runLevel, validationMessage := stringArgument(request, "run_level", false)
	if validationMessage != "" {
		return validationMessage
	}

	if runLevel != "" {
		if runLevel != "default" && runLevel != "single" && runLevel != "binbash" {
			return "run_level must be default, single, or binbash"
		}

		req.RunLevel = runLevel
	}

	virtMode, validationMessage := stringArgument(request, "virt_mode", false)
	if validationMessage != "" {
		return validationMessage
	}

	if virtMode != "" {
		if virtMode != "paravirt" && virtMode != "fullvirt" {
			return "virt_mode must be paravirt or fullvirt"
		}

		req.VirtMode = virtMode
	}

	return ""
}

func applyConfigJSONOptions(request *mcp.CallToolRequest, req *linode.CreateConfigRequest) string {
	helpersJSON, validationMessage := stringArgument(request, "helpers", false)
	if validationMessage != "" {
		return validationMessage
	}

	if helpersJSON != "" {
		var helpers *linode.ConfigHelpers
		if err := strictDecodeJSON(helpersJSON, &helpers); err != nil {
			return fmt.Sprintf("invalid helpers JSON: %v", err)
		}

		if helpers == nil {
			return "helpers must be a JSON object"
		}

		req.Helpers = helpers
	}

	interfacesJSON, validationMessage := stringArgument(request, "interfaces", false)
	if validationMessage != "" {
		return validationMessage
	}

	if interfacesJSON != "" {
		interfaces, errText := parseConfigInterfaces(interfacesJSON)
		if errText != "" {
			return errText
		}

		req.Interfaces = interfaces
	}

	return ""
}

func strictDecodeJSON(input string, target any) error {
	decoder := json.NewDecoder(bytes.NewReader([]byte(input)))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}

	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err != nil {
			return fmt.Errorf("decode trailing JSON: %w", err)
		}

		return errUnexpectedTrailingJSON
	}

	return nil
}

const errInterfacesNotArray = "interfaces must be an array of objects"

// parseConfigInterfaces accepts the interfaces argument as a native array of
// objects (the schema form) or a JSON-encoded string (legacy form), decoding it
// strictly so unknown or read-only fields are rejected.
func parseConfigInterfaces(raw any) ([]linode.ConfigInterface, string) {
	var encoded string

	switch value := raw.(type) {
	case string:
		encoded = value
	case []any:
		marshaled, err := json.Marshal(value)
		if err != nil {
			return nil, errInterfacesNotArray
		}

		encoded = string(marshaled)
	default:
		return nil, errInterfacesNotArray
	}

	var interfaces *[]linode.ConfigInterface
	if err := strictDecodeJSON(encoded, &interfaces); err != nil {
		return nil, fmt.Sprintf("invalid interfaces JSON: %v", err)
	}

	if interfaces == nil {
		return nil, errInterfacesNotArray
	}

	for index, iface := range *interfaces {
		if !validConfigInterfacePurpose(iface.Purpose) {
			return nil, fmt.Sprintf("interfaces[%d].purpose must be public, vlan, or vpc", index)
		}
	}

	return *interfaces, ""
}

func validConfigInterfacePurpose(purpose string) bool {
	switch purpose {
	case interfaceFieldPublic, configInterfacePurposeVLAN, configInterfacePurposeVPC:
		return true
	default:
		return false
	}
}
