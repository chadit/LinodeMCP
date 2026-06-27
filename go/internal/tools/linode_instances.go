package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// boolTrue and boolFalse are used for boolean string comparison in filter functions.
const (
	boolTrue             = "true"
	boolFalse            = "false"
	paramStatsYear       = "year"
	paramStatsMonth      = "month"
	statsYearMin         = 2000
	statsYearMax         = 2037
	statsMonthMin        = 1
	statsMonthMax        = 12
	statsYearRangeError  = "year must be an integer between 2000 and 2037"
	statsMonthRangeError = "month must be an integer between 1 and 12"

	interfaceFieldDefaultRoute = "default_route"
	interfaceFieldPublic       = "public"
	interfaceFieldVLAN         = "vlan"
	interfaceFieldVPC          = "vpc"
)

// NewLinodeInstanceGetTool creates a tool for getting a single Linode instance by ID.
func NewLinodeInstanceGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_get",
		"Retrieves details of a single Linode instance by its ID",
		toolschemas.Schema("linode.mcp.v1.InstanceGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeInstanceGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeInstanceTransferGetTool creates a tool for getting monthly transfer statistics for a Linode instance.
func NewLinodeInstanceTransferGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_transfer_get",
		"Retrieves this month's network transfer statistics for a Linode instance.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
		},
		handleLinodeInstanceTransferGetRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeInstanceTransferGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	transfer, err := client.GetInstanceTransfer(ctx, linodeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Linode instance transfer statistics: %v", err)), nil
	}

	return MarshalToolResponse(transfer)
}

func handleLinodeInstanceGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	instanceID, err := parseInstanceID(request.GetString("instance_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	instance, err := client.GetInstanceProto(ctx, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Linode instance: %v", err)), nil
	}

	return MarshalProtoToolResponse(instance)
}

// NewLinodeInstanceListTool creates a tool for listing Linode instances.
func NewLinodeInstanceListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_list",
		"Lists Linode instances with optional filtering by status",
		toolschemas.Schema("linode.mcp.v1.InstanceListInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeInstancesRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeInstancesRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	statusFilter := request.GetString("status", "")

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	instances, err := client.ListInstancesProto(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Linode instances: %v", err)), nil
	}

	if statusFilter != "" {
		instances = FilterByField(instances, statusFilter, func(inst *linodev1.Instance) string {
			return inst.GetStatus()
		})
	}

	response := &linodev1.InstanceListResponse{
		Instances: instances,
	}

	if count := len(instances); count <= math.MaxInt32 {
		response.Count = int32(count)
	}

	if statusFilter != "" {
		filter := "status=" + statusFilter
		response.Filter = &filter
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeInstanceStatsByYearMonthTool creates a tool for retrieving monthly Linode statistics.
func NewLinodeInstanceStatsByYearMonthTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_stats_month_get",
		"Retrieves CPU, IO, IPv4, and IPv6 statistics for a Linode instance for a specific month.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber(paramStatsYear, mcp.Required(),
				mcp.Description("The statistics year, from 2000 through 2037")),
			mcp.WithNumber(paramStatsMonth, mcp.Required(),
				mcp.Description("The statistics month, from 1 through 12")),
		},
		handleInstanceStatsByYearMonthRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleInstanceStatsByYearMonthRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := requiredPositiveIntArgument(request, "linode_id", ErrLinodeIDRequired.Error(), "linode_id must be a positive integer")
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	year, validationMessage := boundedIntArgument(request, paramStatsYear, statsYearMin, statsYearMax, statsYearRangeError)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	month, validationMessage := boundedIntArgument(request, paramStatsMonth, statsMonthMin, statsMonthMax, statsMonthRangeError)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	stats, err := client.GetInstanceStatsByYearMonth(ctx, linodeID, year, month)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve statistics for instance %d in %d-%02d: %v", linodeID, year, month, err)), nil
	}

	return MarshalToolResponse(stats)
}

func requiredPositiveIntArgument(request *mcp.CallToolRequest, key, missingMessage, invalidMessage string) (int, string) {
	args := request.GetArguments()
	if _, exists := args[key]; !exists {
		return 0, missingMessage
	}

	value, validationMessage := boundedIntArgument(request, key, 1, 0, invalidMessage)
	if validationMessage != "" {
		return 0, validationMessage
	}

	return value, ""
}

func boundedIntArgument(request *mcp.CallToolRequest, key string, minValue, maxValue int, message string) (int, string) {
	args := request.GetArguments()
	if _, exists := args[key]; !exists {
		return 0, message
	}

	value, validationMessage := optionalPaginationInt(args, key, minValue, maxValue)
	if validationMessage != "" {
		return 0, message
	}

	return value, ""
}

// NewLinodeInstanceInterfaceAddTool creates a tool for adding an interface to a Linode instance.
func NewLinodeInstanceInterfaceAddTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_interface_add",
		"Adds a network interface to a Linode instance. WARNING: This changes instance network configuration.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithObject("interface", mcp.Required(),
				mcp.Description("Object defining exactly one interface type: public, vpc, or vlan.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm interface creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceInterfaceAddRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceInterfaceAddRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		if _, ifaceMessage := instanceInterfaceAddRequestFromTool(request); ifaceMessage != "" {
			return mcp.NewToolResultError(ifaceMessage), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_interface_add", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/interfaces", linodeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetInstance(ctx, linodeID) })
	}

	if result := RequireConfirm(request, "This adds a network interface to the Linode instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	interfaceReq, validationMessage := instanceInterfaceAddRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	createdInterface, err := client.AddInstanceInterface(ctx, linodeID, interfaceReq)
	if err != nil {
		return mcp.NewToolResultError(formatAddInstanceInterfaceError(linodeID, err)), nil
	}

	response := struct {
		Message   string                    `json:"message"`
		Interface *linode.InstanceInterface `json:"interface"`
		LinodeID  int                       `json:"linode_id"`
	}{
		Message:   fmt.Sprintf("Interface added to instance %d successfully", linodeID),
		Interface: createdInterface,
		LinodeID:  linodeID,
	}

	return MarshalToolResponse(response)
}

func instanceInterfaceAddRequestFromTool(request *mcp.CallToolRequest) (*linode.AddInstanceInterfaceRequest, string) {
	raw, present := request.GetArguments()["interface"]
	if !present {
		return nil, "interface is required"
	}

	var interfaceJSON string

	switch value := raw.(type) {
	case string:
		interfaceJSON = value
	case map[string]any:
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, interfaceJSONObjRequired
		}

		interfaceJSON = string(encoded)
	default:
		return nil, interfaceJSONObjRequired
	}

	var interfaceReq *linode.AddInstanceInterfaceRequest
	if err := strictDecodeJSON(interfaceJSON, &interfaceReq); err != nil {
		return nil, fmt.Sprintf("invalid interface JSON: %v", err)
	}

	if interfaceReq == nil {
		return nil, interfaceJSONObjRequired
	}

	if validationMessage := validateInstanceInterfaceAddRequest(interfaceReq); validationMessage != "" {
		return nil, validationMessage
	}

	return interfaceReq, ""
}

func validateInstanceInterfaceAddRequest(req *linode.AddInstanceInterfaceRequest) string {
	var typeCount int
	if req.Public != nil {
		typeCount++
	}

	if req.VPC != nil {
		typeCount++

		if req.VPC.SubnetID <= 0 {
			return "interface.vpc.subnet_id must be a positive integer"
		}
	}

	if req.VLAN != nil {
		typeCount++

		if strings.TrimSpace(req.VLAN.Label) == "" {
			return "interface.vlan.vlan_label is required"
		}
	}

	if typeCount != 1 {
		return "interface must define exactly one of public, vpc, or vlan"
	}

	return ""
}

func formatAddInstanceInterfaceError(linodeID int, err error) string {
	return "Failed to add interface to instance " + strconv.Itoa(linodeID) + ": " + err.Error()
}

// NewLinodeInstanceInterfaceGetTool creates a tool for retrieving one interface assigned to a Linode instance.
func NewLinodeInstanceInterfaceGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_interface_get",
		"Retrieves a specific interface assigned to a Linode instance.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber(paramConfigInterfaceID, mcp.Required(),
				mcp.Description("The ID of the Linode interface")),
		},
		handleInstanceInterfaceGetRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleInstanceInterfaceGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	interfaceID, validationMessage := instanceInterfaceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	instanceInterface, err := client.GetInstanceInterface(ctx, linodeID, interfaceID)
	if err != nil {
		return mcp.NewToolResultError(formatGetInstanceInterfaceError(linodeID, interfaceID, err)), nil
	}

	return MarshalToolResponse(instanceInterface)
}

func instanceInterfaceIDFromTool(request *mcp.CallToolRequest) (int, string) {
	args := request.GetArguments()
	if _, exists := args[paramConfigInterfaceID]; !exists {
		return 0, ErrInterfaceIDRequired.Error()
	}

	interfaceID, validationMessage := optionalPaginationInt(args, paramConfigInterfaceID, 1, 0)
	if validationMessage != "" {
		return 0, validationMessage
	}

	return interfaceID, ""
}

func formatGetInstanceInterfaceError(linodeID, interfaceID int, err error) string {
	return "Failed to retrieve interface " + strconv.Itoa(interfaceID) + " for instance " + strconv.Itoa(linodeID) + ": " + err.Error()
}

// NewLinodeInstanceInterfaceDeleteTool creates a tool for deleting an interface from a Linode instance.
func NewLinodeInstanceInterfaceDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_interface_delete",
		"Deletes an interface from a Linode instance. WARNING: This changes instance network configuration and is irreversible.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber(paramConfigInterfaceID, mcp.Required(),
				mcp.Description("The ID of the Linode interface to delete")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm interface deletion. This action is irreversible. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceInterfaceDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleInstanceInterfaceDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	interfaceID, validationMessage := instanceInterfaceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_instance_interface_delete", httpMethodDelete,
			fmt.Sprintf("/linode/instances/%d/interfaces/%d", linodeID, interfaceID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetInstanceInterface(ctx, linodeID, interfaceID)
			})
	}

	if result := RequireConfirm(request, "This deletes a Linode interface and changes instance networking. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.DeleteInstanceInterface(ctx, linodeID, interfaceID); err != nil {
		return mcp.NewToolResultError(formatDeleteInstanceInterfaceError(linodeID, interfaceID, err)), nil
	}

	response := struct {
		Message     string `json:"message"`
		LinodeID    int    `json:"linode_id"`
		InterfaceID int    `json:"interface_id"`
	}{
		Message:     fmt.Sprintf("Interface %d deleted from instance %d successfully", interfaceID, linodeID),
		LinodeID:    linodeID,
		InterfaceID: interfaceID,
	}

	return MarshalToolResponse(response)
}

func formatDeleteInstanceInterfaceError(linodeID, interfaceID int, err error) string {
	return "Failed to delete interface " + strconv.Itoa(interfaceID) + " from instance " + strconv.Itoa(linodeID) + ": " + err.Error()
}

// NewLinodeInstanceInterfaceSettingsGetTool creates a tool for retrieving Linode interface settings.
func NewLinodeInstanceInterfaceSettingsGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_interface_settings_get",
		"Retrieves interface settings for a specific Linode instance.",
		toolschemas.Schema("linode.mcp.v1.InstanceInterfaceSettingsGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceInterfaceSettingsGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleInstanceInterfaceSettingsGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	settings, err := client.GetInstanceInterfaceSettingsProto(ctx, linodeID)
	if err != nil {
		return mcp.NewToolResultError(formatInstanceInterfaceSettingsError("retrieve", linodeID, err)), nil
	}

	return MarshalProtoToolResponse(settings)
}

// NewLinodeInstanceInterfaceSettingsUpdateTool creates a tool for updating Linode interface settings.
func NewLinodeInstanceInterfaceSettingsUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_interface_settings_update",
		"Updates interface settings for a specific Linode instance. WARNING: This changes instance network configuration.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithBoolean("network_helper",
				mcp.Description("Enable or disable Network Helper.")),
			mcp.WithObject("default_route",
				mcp.Description("Default route interface IDs (ipv4_interface_id and/or ipv6_interface_id).")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm interface settings update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceInterfaceSettingsUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceInterfaceSettingsUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		if _, settingsMessage := instanceInterfaceSettingsUpdateRequestFromTool(request); settingsMessage != "" {
			return mcp.NewToolResultError(settingsMessage), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_interface_settings_update", "PUT",
			fmt.Sprintf("/linode/instances/%d/interfaces/settings", linodeID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetInstanceInterfaceSettings(ctx, linodeID)
			})
	}

	if result := RequireConfirm(request, "This updates interface settings for the Linode instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	settingsReq, validationMessage := instanceInterfaceSettingsUpdateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	settings, err := client.UpdateInstanceInterfaceSettings(ctx, linodeID, settingsReq)
	if err != nil {
		return mcp.NewToolResultError(formatInstanceInterfaceSettingsError("update", linodeID, err)), nil
	}

	response := struct {
		Message  string                            `json:"message"`
		Settings *linode.InstanceInterfaceSettings `json:"settings"`
		LinodeID int                               `json:"linode_id"`
	}{
		Message:  fmt.Sprintf("Interface settings for instance %d updated successfully", linodeID),
		Settings: settings,
		LinodeID: linodeID,
	}

	return MarshalToolResponse(response)
}

func instanceInterfaceSettingsUpdateRequestFromTool(request *mcp.CallToolRequest) (*linode.UpdateInstanceInterfaceSettingsRequest, string) {
	args := request.GetArguments()

	settingsReq := &linode.UpdateInstanceInterfaceSettingsRequest{}

	networkHelper, validationMessage := optionalBoolArg(args, "network_helper")
	if validationMessage != "" {
		return nil, validationMessage
	}

	settingsReq.NetworkHelper = networkHelper

	if raw, present := args[interfaceFieldDefaultRoute]; present && raw != nil {
		defaultRouteJSON, fieldMessage := objectJSONFromToolArg(raw, interfaceFieldDefaultRoute)
		if fieldMessage != "" {
			return nil, fieldMessage
		}

		var defaultRoute *linode.InterfaceDefaultRoute
		if err := strictDecodeJSON(defaultRouteJSON, &defaultRoute); err != nil {
			return nil, fmt.Sprintf("invalid default_route: %v", err)
		}

		settingsReq.DefaultRoute = defaultRoute
	}

	if settingsReq.DefaultRoute == nil && settingsReq.NetworkHelper == nil {
		return nil, "network_helper or default_route is required"
	}

	return settingsReq, ""
}

func formatInstanceInterfaceSettingsError(action string, linodeID int, err error) string {
	return "Failed to " + action + " interface settings for instance " + strconv.Itoa(linodeID) + ": " + err.Error()
}

// NewLinodeInstanceInterfaceUpdateTool creates a tool for updating an interface on a Linode instance.
func NewLinodeInstanceInterfaceUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_interface_update",
		"Updates a network interface on a Linode instance. WARNING: This changes instance network configuration.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("interface_id", mcp.Required(),
				mcp.Description("The ID of the Linode interface")),
			mcp.WithObject("default_route",
				mcp.Description("Default route settings for the interface.")),
			mcp.WithObject("public",
				mcp.Description("Public interface settings. Set exactly one of public, vpc, or vlan.")),
			mcp.WithObject("vlan",
				mcp.Description("VLAN interface settings. Set exactly one of public, vpc, or vlan.")),
			mcp.WithObject("vpc",
				mcp.Description("VPC interface settings. Set exactly one of public, vpc, or vlan.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm interface update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceInterfaceUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceInterfaceUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	interfaceID, interfaceIDOK := getPositiveIntArgument(request, "interface_id")
	if !interfaceIDOK {
		return mcp.NewToolResultError(linode.ErrInterfaceIDPositive.Error()), nil
	}

	if IsDryRun(request) {
		if _, ifaceMessage := instanceInterfaceUpdateRequestFromTool(request); ifaceMessage != "" {
			return mcp.NewToolResultError(ifaceMessage), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_interface_update", "PUT",
			fmt.Sprintf("/linode/instances/%d/interfaces/%d", linodeID, interfaceID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetInstanceInterface(ctx, linodeID, interfaceID)
			})
	}

	if result := RequireConfirm(request, "This updates a network interface on the Linode instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	interfaceReq, validationMessage := instanceInterfaceUpdateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	updatedInterface, err := client.UpdateInstanceInterface(ctx, linodeID, interfaceID, interfaceReq)
	if err != nil {
		return mcp.NewToolResultError(formatUpdateInstanceInterfaceError(linodeID, interfaceID, err)), nil
	}

	response := struct {
		Message     string                    `json:"message"`
		Interface   *linode.InstanceInterface `json:"interface"`
		LinodeID    int                       `json:"linode_id"`
		InterfaceID int                       `json:"interface_id"`
	}{
		Message:     fmt.Sprintf("Interface %d updated on instance %d successfully", interfaceID, linodeID),
		Interface:   updatedInterface,
		LinodeID:    linodeID,
		InterfaceID: interfaceID,
	}

	return MarshalToolResponse(response)
}

func instanceInterfaceUpdateRequestFromTool(request *mcp.CallToolRequest) (*linode.UpdateInstanceInterfaceRequest, string) {
	args := request.GetArguments()

	fields := map[string]json.RawMessage{}

	for _, name := range []string{interfaceFieldDefaultRoute, interfaceFieldPublic, interfaceFieldVLAN, interfaceFieldVPC} {
		raw, present := args[name]
		if !present || raw == nil {
			continue
		}

		fieldJSON, fieldMessage := objectJSONFromToolArg(raw, name)
		if fieldMessage != "" {
			return nil, fieldMessage
		}

		fields[name] = json.RawMessage(fieldJSON)
	}

	if len(fields) == 0 {
		return nil, "at least one of default_route, public, vpc, or vlan is required"
	}

	encoded, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Sprintf("invalid interface fields: %v", err)
	}

	var interfaceReq *linode.UpdateInstanceInterfaceRequest
	if err := strictDecodeJSON(string(encoded), &interfaceReq); err != nil {
		return nil, fmt.Sprintf("invalid interface fields: %v", err)
	}

	if interfaceReq == nil {
		return nil, interfaceJSONObjRequired
	}

	if validationMessage := validateInstanceInterfaceUpdateRequest(interfaceReq); validationMessage != "" {
		return nil, validationMessage
	}

	return interfaceReq, ""
}

func validateInstanceInterfaceUpdateRequest(req *linode.UpdateInstanceInterfaceRequest) string {
	var typeCount int
	if req.Public != nil {
		typeCount++
	}

	if req.VPC != nil {
		typeCount++

		if req.VPC.SubnetID <= 0 {
			return "vpc.subnet_id must be a positive integer"
		}
	}

	if req.VLAN != nil {
		typeCount++

		if strings.TrimSpace(req.VLAN.Label) == "" {
			return "vlan.vlan_label is required"
		}
	}

	if typeCount != 1 {
		return "exactly one of public, vpc, or vlan is required"
	}

	return ""
}

func formatUpdateInstanceInterfaceError(linodeID, interfaceID int, err error) string {
	return "Failed to update interface " + strconv.Itoa(interfaceID) + " on instance " + strconv.Itoa(linodeID) + ": " + err.Error()
}

// NewLinodeInstanceInterfaceHistoryListTool creates a tool for listing historical interface versions for a Linode instance.
func NewLinodeInstanceInterfaceHistoryListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_interface_history_list",
		"Lists historical network interface versions for a specific Linode instance with optional pagination.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleInstanceInterfaceHistoryListRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleInstanceInterfaceHistoryListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	page, pageSize, validationMessage := instanceFirewallsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	history, err := client.ListInstanceInterfaceHistory(ctx, linodeID, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(formatListInstanceInterfaceHistoryError(linodeID, err)), nil
	}

	return MarshalToolResponse(history)
}

func formatListInstanceInterfaceHistoryError(linodeID int, err error) string {
	return "Failed to list interface history for instance " + strconv.Itoa(linodeID) + ": " + err.Error()
}

// NewLinodeInterfacesUpgradeTool creates a tool for upgrading legacy config interfaces to Linode interfaces.
func NewLinodeInterfacesUpgradeTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_interface_upgrade",
		"Upgrades a Linode's legacy config interfaces to Linode interfaces. WARNING: This irreversibly changes instance network configuration.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("config_id",
				mcp.Description("Optional configuration profile ID to upgrade")),
			mcp.WithBoolean("api_dry_run",
				mcp.Description("Pass dry_run to the Linode API to validate the upgrade without applying it.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm the interface upgrade request. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeInterfacesUpgradeRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeInterfacesUpgradeRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	upgradeReq, validationMessage := buildUpgradeLinodeInterfacesRequest(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_instance_interface_upgrade", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/upgrade-interfaces", linodeID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetInstance(ctx, linodeID) })
	}

	if result := RequireConfirm(request, "This irreversibly upgrades Linode network interfaces. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, err := client.UpgradeLinodeInterfaces(ctx, linodeID, upgradeReq)
	if err != nil {
		return mcp.NewToolResultError(formatUpgradeLinodeInterfacesError(linodeID, err)), nil
	}

	return MarshalToolResponse(result)
}

func buildUpgradeLinodeInterfacesRequest(request *mcp.CallToolRequest) (*linode.UpgradeLinodeInterfacesRequest, string) {
	args := request.GetArguments()
	req := &linode.UpgradeLinodeInterfacesRequest{}

	if _, exists := args["config_id"]; exists {
		configID, validationMessage := boundedIntArgument(request, "config_id", 1, 0, "config_id must be an integer greater than or equal to 1")
		if validationMessage != "" {
			return nil, validationMessage
		}

		req.ConfigID = &configID
	}

	apiDryRun, validationMessage := optionalBoolArg(args, "api_dry_run")
	if validationMessage != "" {
		return nil, validationMessage
	}

	req.DryRun = apiDryRun

	return req, ""
}

func formatUpgradeLinodeInterfacesError(linodeID int, err error) string {
	return "Failed to upgrade interfaces for instance " + strconv.Itoa(linodeID) + ": " + err.Error()
}

// NewLinodeInstanceInterfacesListTool creates a tool for listing interfaces assigned to a Linode instance.
func NewLinodeInstanceInterfacesListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_interface_list",
		"Lists interfaces assigned to a specific Linode instance.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
		},
		handleInstanceInterfacesListRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleInstanceInterfacesListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	interfaces, err := client.ListInstanceInterfaces(ctx, linodeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list interfaces for instance %d: %v", linodeID, err)), nil
	}

	response := struct {
		Count      int                        `json:"count"`
		Interfaces []linode.InstanceInterface `json:"interfaces"`
	}{
		Count:      len(interfaces),
		Interfaces: interfaces,
	}

	return MarshalToolResponse(response)
}

func selectEnvironment(cfg *config.Config, environment string) (*config.EnvironmentConfig, error) {
	if environment != "" {
		if env, exists := cfg.Environments[environment]; exists {
			return &env, nil
		}

		return nil, fmt.Errorf("%w: %s", ErrEnvironmentNotFound, environment)
	}

	selectedEnv, err := cfg.SelectEnvironment("default")
	if err != nil {
		return nil, fmt.Errorf("failed to select default environment: %w", err)
	}

	return selectedEnv, nil
}

func validateLinodeConfig(env *config.EnvironmentConfig) error {
	if env.Linode.APIURL == "" || env.Linode.Token == "" {
		return ErrLinodeConfigIncomplete
	}

	return nil
}

// parseInstanceID validates and converts the instance ID string to an integer.
func parseInstanceID(raw string) (int, error) {
	if raw == "" {
		return 0, ErrInstanceIDRequired
	}

	instanceID, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrInvalidInstanceID, raw)
	}

	return instanceID, nil
}
