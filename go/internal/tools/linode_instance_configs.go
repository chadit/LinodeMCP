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

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	instanceConfigsPageSizeMin = 25
	instanceConfigsPageSizeMax = 500
	configUpdateNoFields       = "at least one configuration field must be provided"
)

// NewLinodeInstanceConfigListTool creates a tool for listing configuration profiles on a Linode instance.
func NewLinodeInstanceConfigListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_config_list",
		"Lists configuration profiles for a Linode instance with optional pagination.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleInstanceConfigsListRequest,
	)

	return tool, profiles.CapRead, handler
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
				mcp.Description("Must be true to confirm deletion. This action is irreversible.")),
		},
		handleInstanceConfigDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleInstanceConfigsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	page, pageSize, validationMessage := instanceConfigsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	configs, err := client.ListInstanceConfigs(ctx, linodeID, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list configuration profiles for instance %d: %v", linodeID, err)), nil
	}

	response := struct {
		Count   int                     `json:"count"`
		Configs []linode.InstanceConfig `json:"configs"`
	}{
		Count:   len(configs),
		Configs: configs,
	}

	return MarshalToolResponse(response)
}

func handleInstanceConfigDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This is irreversible. The configuration profile will be permanently deleted. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	configID, validationMessage := instanceConfigIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
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
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_config_interfaces_list",
		"Lists interfaces assigned to a specific configuration profile on a Linode instance.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("config_id", mcp.Required(),
				mcp.Description("The ID of the configuration profile")),
		},
		handleInstanceConfigInterfacesListRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleInstanceConfigInterfacesListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
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

	interfaces, err := client.ListInstanceConfigInterfaces(ctx, linodeID, configID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list interfaces for config %d on instance %d: %v", configID, linodeID, err)), nil
	}

	response := struct {
		Count      int                              `json:"count"`
		Interfaces []linode.ConfigInterfaceResponse `json:"interfaces"`
	}{
		Count:      len(interfaces),
		Interfaces: interfaces,
	}

	return MarshalToolResponse(response)
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
			mcp.WithString("devices", mcp.Required(),
				mcp.Description("JSON object mapping device slots to disk/volume IDs, e.g. {\"sda\":{\"disk_id\":123}}")),
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
				mcp.Description("Must be true to confirm configuration profile creation.")),
		},
		handleInstanceConfigCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceConfigCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This creates a configuration profile on the instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	linodeID, linodeIDOK := getPositiveIntArgument(request, "linode_id")
	if !linodeIDOK {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	createReq, errText := buildCreateConfigRequest(request)
	if errText != "" {
		return mcp.NewToolResultError(errText), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	createdConfig, err := client.CreateInstanceConfig(ctx, linodeID, &createReq)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create configuration profile for instance %d: %v", linodeID, err)), nil
	}

	response := struct {
		Message  string                 `json:"message"`
		Config   *linode.InstanceConfig `json:"config"`
		LinodeID int                    `json:"linode_id"`
	}{
		Message:  fmt.Sprintf("Configuration profile '%s' (ID: %d) created on instance %d", createdConfig.Label, createdConfig.ID, linodeID),
		Config:   createdConfig,
		LinodeID: linodeID,
	}

	return MarshalToolResponse(response)
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
			mcp.WithString("interface", mcp.Required(),
				mcp.Description("JSON object for the interface to add. Must include purpose: public, vlan, or vpc.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm configuration profile interface creation.")),
		},
		handleInstanceConfigInterfaceAddRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceConfigInterfaceAddRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This adds a network interface to the configuration profile. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	linodeID, linodeIDOK := getPositiveIntArgument(request, "linode_id")
	if !linodeIDOK {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	configID, configIDOK := getPositiveIntArgument(request, "config_id")
	if !configIDOK {
		return mcp.NewToolResultError("config_id must be a positive integer"), nil
	}

	configInterface, errText := configInterfaceFromTool(request)
	if errText != "" {
		return mcp.NewToolResultError(errText), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	createdInterface, err := client.AddInstanceConfigInterface(ctx, linodeID, configID, configInterface)
	if err != nil {
		return mcp.NewToolResultError(formatAddConfigInterfaceError(linodeID, configID, err)), nil
	}

	response := struct {
		Message   string                  `json:"message"`
		Interface *linode.ConfigInterface `json:"interface"`
		LinodeID  int                     `json:"linode_id"`
		ConfigID  int                     `json:"config_id"`
	}{
		Message:   fmt.Sprintf("Configuration profile interface added to config %d on instance %d", configID, linodeID),
		Interface: createdInterface,
		LinodeID:  linodeID,
		ConfigID:  configID,
	}

	return MarshalToolResponse(response)
}

func configInterfaceFromTool(request *mcp.CallToolRequest) (*linode.ConfigInterface, string) {
	interfaceJSON, validationMessage := stringArgument(request, "interface", true)
	if validationMessage != "" {
		return nil, validationMessage
	}

	return parseConfigInterface(interfaceJSON)
}

func parseConfigInterface(interfaceJSON string) (*linode.ConfigInterface, string) {
	var configInterface *linode.ConfigInterface
	if err := strictDecodeJSON(interfaceJSON, &configInterface); err != nil {
		return nil, fmt.Sprintf("invalid interface JSON: %v", err)
	}

	if configInterface == nil {
		return nil, "interface must be a JSON object"
	}

	if !validConfigInterfacePurpose(configInterface.Purpose) {
		return nil, "interface.purpose must be public, vlan, or vpc"
	}

	return configInterface, ""
}

func formatAddConfigInterfaceError(linodeID, configID int, err error) string {
	return "Failed to add configuration profile interface to config " + strconv.Itoa(configID) + " for instance " + strconv.Itoa(linodeID) + ": " + err.Error()
}

// NewLinodeInstanceConfigInterfacesReorderTool creates a tool for reordering configuration profile interfaces.
func NewLinodeInstanceConfigInterfacesReorderTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_config_interfaces_reorder",
		"Reorders interfaces on a Linode configuration profile. WARNING: This changes network interface ordering.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("config_id", mcp.Required(),
				mcp.Description("The ID of the configuration profile")),
			mcp.WithString("ids", mcp.Required(),
				mcp.Description("JSON array of existing configuration profile interface IDs in the desired order, e.g. [101,102,103]")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm configuration interface reorder.")),
		},
		handleInstanceConfigInterfacesReorderRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceConfigInterfacesReorderRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This reorders network interfaces on the configuration profile. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	configID, validationMessage := instanceConfigIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
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

	idsJSON, ok := value.(string)
	if !ok {
		return nil, "ids must be a string"
	}

	idsJSON = strings.TrimSpace(idsJSON)
	if idsJSON == "" {
		return nil, "ids is required"
	}

	var ids []int

	decoder := json.NewDecoder(strings.NewReader(idsJSON))
	if err := decoder.Decode(&ids); err != nil {
		return nil, "ids must be a JSON array of positive integer interface IDs"
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, "ids must be a JSON array of positive integer interface IDs"
	}

	if len(ids) == 0 {
		return nil, "ids must include at least one interface ID"
	}

	seen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, "ids must contain only positive integer interface IDs"
		}

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
			mcp.WithString("devices",
				mcp.Description("JSON object mapping device slots to disk/volume IDs")),
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
				mcp.Description("Must be true to confirm configuration profile update.")),
		},
		handleInstanceConfigUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceConfigUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This updates a configuration profile on the instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	linodeID, linodeIDOK := getPositiveIntArgument(request, "linode_id")
	if !linodeIDOK {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	configID, configIDOK := getPositiveIntArgument(request, "config_id")
	if !configIDOK {
		return mcp.NewToolResultError("config_id must be a positive integer"), nil
	}

	updateReq, errText := buildUpdateConfigRequest(request)
	if errText != "" {
		return mcp.NewToolResultError(errText), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	updatedConfig, err := client.UpdateInstanceConfig(ctx, linodeID, configID, updateReq)
	if err != nil {
		return mcp.NewToolResultError(formatUpdateConfigError(linodeID, configID, err)), nil
	}

	response := struct {
		Message  string                 `json:"message"`
		Config   *linode.InstanceConfig `json:"config"`
		LinodeID int                    `json:"linode_id"`
		ConfigID int                    `json:"config_id"`
	}{
		Message:  fmt.Sprintf("Configuration profile '%s' (ID: %d) updated on instance %d", updatedConfig.Label, updatedConfig.ID, linodeID),
		Config:   updatedConfig,
		LinodeID: linodeID,
		ConfigID: configID,
	}

	return MarshalToolResponse(response)
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
	devicesJSON, validationMessage := stringArgument(request, "devices", false)
	if validationMessage != "" {
		return validationMessage
	}

	if _, exists := request.GetArguments()["devices"]; exists {
		devices, errText := parseConfigDevices(devicesJSON)
		if errText != "" {
			return errText
		}

		req.Devices = &devices
		*fields++
	}

	helpersJSON, validationMessage := stringArgument(request, "helpers", false)
	if validationMessage != "" {
		return validationMessage
	}

	if _, exists := request.GetArguments()["helpers"]; exists {
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

	interfacesJSON, validationMessage := stringArgument(request, "interfaces", false)
	if validationMessage != "" {
		return validationMessage
	}

	if _, exists := request.GetArguments()["interfaces"]; exists {
		interfaces, errText := parseConfigInterfaces(interfacesJSON)
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

	devicesJSON, errText := stringArgument(request, "devices", true)
	if errText != "" {
		return linode.CreateConfigRequest{}, errText
	}

	devices, errText := parseConfigDevices(devicesJSON)
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

func parseConfigDevices(devicesJSON string) (map[string]*linode.ConfigDevice, string) {
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

func parseConfigInterfaces(interfacesJSON string) ([]linode.ConfigInterface, string) {
	var interfaces *[]linode.ConfigInterface
	if err := strictDecodeJSON(interfacesJSON, &interfaces); err != nil {
		return nil, fmt.Sprintf("invalid interfaces JSON: %v", err)
	}

	if interfaces == nil {
		return nil, "interfaces must be a JSON array"
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
	case "public", "vlan", "vpc":
		return true
	default:
		return false
	}
}
