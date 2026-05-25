package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// boolTrue and boolFalse are used for boolean string comparison in filter functions.
const (
	boolTrue  = "true"
	boolFalse = "false"
)

// NewLinodeInstanceGetTool creates a tool for getting a single Linode instance by ID.
func NewLinodeInstanceGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_instance_get",
		mcp.WithDescription("Retrieves details of a single Linode instance by its ID"),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString(
			"instance_id",
			mcp.Description("The ID of the Linode instance to retrieve (required)"),
			mcp.Required(),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeInstanceGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
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

	instance, err := client.GetInstance(ctx, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Linode instance: %v", err)), nil
	}

	return MarshalToolResponse(instance)
}

// NewLinodeInstanceListTool creates a tool for listing Linode instances.
func NewLinodeInstanceListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_instance_list",
		mcp.WithDescription("Lists Linode instances with optional filtering by status"),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString(
			"status",
			mcp.Description("Filter instances by status (running, stopped, etc.)"),
		),
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

	instances, err := client.ListInstances(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Linode instances: %v", err)), nil
	}

	if statusFilter != "" {
		instances = FilterByField(instances, statusFilter, func(inst linode.Instance) string {
			return inst.Status
		})
	}

	return formatInstancesResponse(instances, statusFilter)
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
			mcp.WithString("interface", mcp.Required(),
				mcp.Description("JSON object defining exactly one interface type: public, vpc, or vlan.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm interface creation.")),
		},
		handleInstanceInterfaceAddRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceInterfaceAddRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This adds a network interface to the Linode instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
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
	interfaceJSON, validationMessage := stringArgument(request, "interface", true)
	if validationMessage != "" {
		return nil, validationMessage
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

// NewLinodeInstanceInterfaceSettingsGetTool creates a tool for retrieving Linode interface settings.
func NewLinodeInstanceInterfaceSettingsGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_interface_settings_get",
		"Retrieves interface settings for a specific Linode instance.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
		},
		handleInstanceInterfaceSettingsGetRequest,
	)

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

	settings, err := client.GetInstanceInterfaceSettings(ctx, linodeID)
	if err != nil {
		return mcp.NewToolResultError(formatInstanceInterfaceSettingsError("retrieve", linodeID, err)), nil
	}

	return MarshalToolResponse(settings)
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
			mcp.WithString("settings", mcp.Required(),
				mcp.Description("JSON object with optional default_route and/or network_helper fields.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm interface settings update.")),
		},
		handleInstanceInterfaceSettingsUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceInterfaceSettingsUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This updates interface settings for the Linode instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
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
	settingsJSON, validationMessage := stringArgument(request, "settings", true)
	if validationMessage != "" {
		return nil, validationMessage
	}

	var settingsReq *linode.UpdateInstanceInterfaceSettingsRequest
	if err := strictDecodeJSON(settingsJSON, &settingsReq); err != nil {
		return nil, fmt.Sprintf("invalid settings JSON: %v", err)
	}

	if settingsReq == nil {
		return nil, interfaceJSONObjRequired
	}

	if settingsReq.DefaultRoute == nil && settingsReq.NetworkHelper == nil {
		return nil, "at least one interface settings field is required"
	}

	return settingsReq, ""
}

func formatInstanceInterfaceSettingsError(action string, linodeID int, err error) string {
	return "Failed to " + action + " interface settings for instance " + strconv.Itoa(linodeID) + ": " + err.Error()
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

// NewLinodeInstanceInterfacesListTool creates a tool for listing interfaces assigned to a Linode instance.
func NewLinodeInstanceInterfacesListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_interfaces_list",
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

func formatInstancesResponse(instances []linode.Instance, statusFilter string) (*mcp.CallToolResult, error) {
	response := struct {
		Count     int               `json:"count"`
		Filter    string            `json:"filter,omitempty"`
		Instances []linode.Instance `json:"instances"`
	}{
		Count:     len(instances),
		Instances: instances,
	}

	if statusFilter != "" {
		response.Filter = "status=" + statusFilter
	}

	return MarshalToolResponse(response)
}
