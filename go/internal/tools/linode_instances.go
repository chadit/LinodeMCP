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

// boolTrue is used for boolean string comparison in filter functions.
const (
	boolTrue             = "true"
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
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_transfer_get",
		"Retrieves this month's network transfer statistics for a Linode instance.",
		toolschemas.Schema("linode.mcp.v1.InstanceTransferGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeInstanceTransferGetRequest(ctx, &request, cfg)
	}

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

	transfer, err := client.GetInstanceTransferProto(ctx, linodeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Linode instance transfer statistics: %v", err)), nil
	}

	return MarshalProtoToolResponse(transfer)
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
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_stats_month_get",
		"Retrieves CPU, IO, IPv4, and IPv6 statistics for a Linode instance for a specific month.",
		toolschemas.Schema("linode.mcp.v1.InstanceStatsMonthGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceStatsByYearMonthRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleInstanceStatsByYearMonthRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := requiredIDArgument(request, "linode_id")
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

	stats, err := client.GetInstanceStatsByYearMonthProto(ctx, linodeID, year, month)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve statistics for instance %d in %d-%02d: %v", linodeID, year, month, err)), nil
	}

	return MarshalProtoToolResponse(stats)
}

// requiredIDArgument parses a required positive-integer id path argument,
// returning the Option-B pair used repo-wide: "<name> is required" when the
// argument is absent, "<name> must be a positive integer" when present but not
// a positive integer (wrong type, bool, zero, or negative). Mirrors the Python
// required_int_id helper so both languages reject identical inputs with
// identical text.
func requiredIDArgument(request *mcp.CallToolRequest, name string) (int, string) {
	return requiredBoundedIDArgument(request, name, 0)
}

// requiredBoundedIDArgument is requiredIDArgument plus an upper bound: ids above
// maxValue are rejected with the same "must be a positive integer" text. Used by
// the parsers that guard against oversized ids (float64 precision loss on very
// large JSON numbers). A maxValue of 0 disables the upper bound.
func requiredBoundedIDArgument(request *mcp.CallToolRequest, name string, maxValue int) (int, string) {
	raw, exists := request.GetArguments()[name]
	if !exists {
		return 0, name + " is required"
	}

	value, ok := numberArgToInt(raw)
	if !ok || value < 1 || (maxValue > 0 && value > maxValue) {
		return 0, name + " must be a positive integer"
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
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_interface_add",
		"Adds a network interface to a Linode instance. WARNING: This changes instance network configuration.",
		toolschemas.Schema("linode.mcp.v1.InstanceInterfaceAddInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceInterfaceAddRequest(ctx, &request, cfg)
	}

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

	createdInterface, err := client.AddInstanceInterfaceProto(ctx, linodeID, interfaceReq)
	if err != nil {
		return mcp.NewToolResultError(formatAddInstanceInterfaceError(linodeID, err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.InstanceInterfaceWriteResponse{
		Message:   fmt.Sprintf("Interface added to instance %d successfully", linodeID),
		Interface: createdInterface,
	})
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
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_interface_get",
		"Retrieves a specific interface assigned to a Linode instance.",
		toolschemas.Schema("linode.mcp.v1.InstanceInterfaceGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceInterfaceGetRequest(ctx, &request, cfg)
	}

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

	instanceInterface, err := client.GetInstanceInterfaceProto(ctx, linodeID, interfaceID)
	if err != nil {
		return mcp.NewToolResultError(formatGetInstanceInterfaceError(linodeID, interfaceID, err)), nil
	}

	return MarshalProtoToolResponse(instanceInterface)
}

func instanceInterfaceIDFromTool(request *mcp.CallToolRequest) (int, string) {
	return requiredIDArgument(request, paramConfigInterfaceID)
}

func formatGetInstanceInterfaceError(linodeID, interfaceID int, err error) string {
	return "Failed to retrieve interface " + strconv.Itoa(interfaceID) + " for instance " + strconv.Itoa(linodeID) + ": " + err.Error()
}

// NewLinodeInstanceInterfaceDeleteTool creates a tool for deleting an interface from a Linode instance.
func NewLinodeInstanceInterfaceDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_interface_delete",
		"Deletes an interface from a Linode instance. WARNING: This changes instance network configuration and is irreversible.",
		toolschemas.Schema("linode.mcp.v1.InstanceInterfaceDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceInterfaceDeleteRequest(ctx, &request, cfg)
	}

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

	if result := requireDestroyConfirmation(ctx, request, "linode_instance_interface_delete", "This deletes a Linode interface and changes instance networking. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.DeleteInstanceInterface(ctx, linodeID, interfaceID); err != nil {
		return mcp.NewToolResultError(formatDeleteInstanceInterfaceError(linodeID, interfaceID, err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.InstanceInterfaceDeleteResponse{
		Message:     fmt.Sprintf("Interface %d deleted from instance %d successfully", interfaceID, linodeID),
		LinodeId:    linodeIDToInt32(linodeID),
		InterfaceId: linodeIDToInt32(interfaceID),
	})
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
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_interface_settings_update",
		"Updates interface settings for a specific Linode instance. WARNING: This changes instance network configuration.",
		toolschemas.Schema("linode.mcp.v1.InstanceInterfaceSettingsUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceInterfaceSettingsUpdateRequest(ctx, &request, cfg)
	}

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

	settings, err := client.UpdateInstanceInterfaceSettingsProto(ctx, linodeID, settingsReq)
	if err != nil {
		return mcp.NewToolResultError(formatInstanceInterfaceSettingsError("update", linodeID, err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.InstanceInterfaceSettingsWriteResponse{
		Message:  fmt.Sprintf("Interface settings for instance %d updated successfully", linodeID),
		Settings: settings,
	})
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
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_interface_update",
		"Updates a network interface on a Linode instance. WARNING: This changes instance network configuration.",
		toolschemas.Schema("linode.mcp.v1.InstanceInterfaceUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceInterfaceUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleInstanceInterfaceUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	interfaceID, validationMessage := requiredIDArgument(request, "interface_id")
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
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

	updatedInterface, err := client.UpdateInstanceInterfaceProto(ctx, linodeID, interfaceID, interfaceReq)
	if err != nil {
		return mcp.NewToolResultError(formatUpdateInstanceInterfaceError(linodeID, interfaceID, err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.InstanceInterfaceWriteResponse{
		Message:   fmt.Sprintf("Interface %d updated on instance %d successfully", interfaceID, linodeID),
		Interface: updatedInterface,
	})
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
	_, handler := newProtoListToolSubresourcePaginated(
		cfg,
		"linode_instance_interface_history_list",
		"Lists historical network interface versions for a specific Linode instance with optional pagination.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		protoListPathID{
			option: mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			parse: instanceConfigLinodeIDFromTool,
		},
		instanceFirewallsPaginationFromTool,
		func(ctx context.Context, client *linode.Client, linodeID, page, pageSize int) ([]*linodev1.InstanceInterfaceHistory, error) {
			return client.ListInstanceInterfaceHistoryProto(ctx, linodeID, page, pageSize)
		},
		nil,
		instanceInterfaceHistoryListResponse,
	)

	tool := mcp.NewToolWithRawSchema(
		"linode_instance_interface_history_list",
		"Lists historical network interface versions for a specific Linode instance with optional pagination.",
		toolschemas.Schema("linode.mcp.v1.InstanceInterfaceHistoryListInput"),
	)

	return tool, profiles.CapRead, handler
}

func instanceInterfaceHistoryListResponse(items []*linodev1.InstanceInterfaceHistory, count int32, filter *string) *linodev1.InstanceInterfaceHistoryListResponse {
	return &linodev1.InstanceInterfaceHistoryListResponse{Count: count, Filter: filter, InterfaceHistory: items}
}

// NewLinodeInterfacesUpgradeTool creates a tool for upgrading legacy config interfaces to Linode interfaces.
func NewLinodeInterfacesUpgradeTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_interface_upgrade",
		"Upgrades a Linode's legacy config interfaces to Linode interfaces. WARNING: This irreversibly changes instance network configuration.",
		toolschemas.Schema("linode.mcp.v1.InstanceInterfaceUpgradeInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeInterfacesUpgradeRequest(ctx, &request, cfg)
	}

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

	result, err := client.UpgradeLinodeInterfacesProto(ctx, linodeID, upgradeReq)
	if err != nil {
		return mcp.NewToolResultError(formatUpgradeLinodeInterfacesError(linodeID, err)), nil
	}

	result.Message = fmt.Sprintf("Linode %d interface upgrade initiated", linodeID)

	return MarshalProtoToolResponse(result)
}

func buildUpgradeLinodeInterfacesRequest(request *mcp.CallToolRequest) (*linode.UpgradeLinodeInterfacesRequest, string) {
	args := request.GetArguments()
	req := &linode.UpgradeLinodeInterfacesRequest{}

	if _, exists := args["config_id"]; exists {
		configID, validationMessage := requiredIDArgument(request, "config_id")
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
	_, handler := newProtoListToolSubresource(
		cfg,
		"linode_instance_interface_list",
		"Lists interfaces assigned to a specific Linode instance.",
		protoListPathID{
			option: mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			parse: instanceConfigLinodeIDFromTool,
		},
		func(ctx context.Context, client *linode.Client, linodeID int) ([]*linodev1.InstanceInterface, error) {
			return client.ListInstanceInterfacesProto(ctx, linodeID)
		},
		nil,
		instanceInterfaceListResponse,
	)

	tool := mcp.NewToolWithRawSchema(
		"linode_instance_interface_list",
		"Lists interfaces assigned to a specific Linode instance.",
		toolschemas.Schema("linode.mcp.v1.InstanceInterfaceListInput"),
	)

	return tool, profiles.CapRead, handler
}

func instanceInterfaceListResponse(items []*linodev1.InstanceInterface, count int32, filter *string) *linodev1.InstanceInterfaceListResponse {
	return &linodev1.InstanceInterfaceListResponse{Count: count, Filter: filter, Interfaces: items}
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
