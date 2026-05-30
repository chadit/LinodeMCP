package tools

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	nodeBalancerNodeErrAddressRequired     = "address is required"
	nodeBalancerConfigPortMax              = 65535
	nodeBalancerConfigKeyPort              = "port"
	nodeBalancerConfigKeyProtocol          = "protocol"
	nodeBalancerConfigKeySSLCert           = "ssl_cert"
	nodeBalancerConfigKeySSLKey            = "ssl_key"
	nodeBalancerConfigProtocolHTTP         = "http"
	nodeBalancerConfigProtocolHTTPS        = "https"
	nodeBalancerConfigProtocolTCP          = "tcp"
	nodeBalancerConfigAlgorithmRoundRobin  = "roundrobin"
	nodeBalancerConfigAlgorithmLeastConn   = "leastconn"
	nodeBalancerConfigAlgorithmSource      = "source"
	nodeBalancerConfigStickinessNone       = "none"
	nodeBalancerConfigStickinessTable      = "table"
	nodeBalancerConfigStickinessHTTPCookie = "http_cookie"
	nodeBalancerConfigCheckNone            = "none"
	nodeBalancerConfigCheckConnection      = "connection"
	nodeBalancerConfigCheckHTTP            = "http"
	nodeBalancerConfigCheckHTTPBody        = "http_body"
	nodeBalancerConfigCipherRecommended    = "recommended"
	nodeBalancerConfigCipherLegacy         = "legacy"
	nodeBalancerConfigNodesPageSizeMin     = 25
	nodeBalancerConfigNodesPageSizeMax     = 500
)

// NewLinodeNodeBalancerTypesTool creates a tool for listing available NodeBalancer types.
func NewLinodeNodeBalancerTypesTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_nodebalancer_types",
		"Lists available NodeBalancer types.",
		nil,
		handleLinodeNodeBalancerTypesRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeNodeBalancerTypesRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	types, listFailureMessage := listNodeBalancerTypes(ctx, client)
	if listFailureMessage != "" {
		return mcp.NewToolResultError("Failed to retrieve linode_nodebalancer_types: " + listFailureMessage), nil
	}

	return MarshalToolResponse(types)
}

func listNodeBalancerTypes(ctx context.Context, client *linode.Client) (*linode.PaginatedResponse[linode.NodeBalancerType], string) {
	types, err := client.ListNodeBalancerTypes(ctx)
	if err != nil {
		return nil, err.Error()
	}

	return types, ""
}

// NewLinodeNodeBalancerListTool creates a tool for listing NodeBalancers.
func NewLinodeNodeBalancerListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newListTool(
		cfg,
		"linode_nodebalancer_list",
		"Lists all NodeBalancers on your account. Can filter by region or label.",
		func(ctx context.Context, client *linode.Client) ([]linode.NodeBalancer, error) {
			return client.ListNodeBalancers(ctx)
		},
		[]listFilterParam[linode.NodeBalancer]{
			fieldFilter("region", "Filter by region ID (e.g., us-east, eu-west)",
				func(n linode.NodeBalancer) string { return n.Region }),
			containsFilter("label_contains", "Filter NodeBalancers by label containing this string (case-insensitive)",
				func(n linode.NodeBalancer) string { return n.Label }),
		},
		"nodebalancers",
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeInstanceNodeBalancerListTool creates a tool for listing NodeBalancers assigned to a Linode instance.
func NewLinodeInstanceNodeBalancerListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_nodebalancer_list",
		"Lists NodeBalancers assigned to a Linode instance.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
		},
		handleLinodeInstanceNodeBalancerListRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeNodeBalancerConfigListTool creates a tool for listing configs on a NodeBalancer.
func NewLinodeNodeBalancerConfigListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_nodebalancer_config_list",
		"Lists configs for a specific NodeBalancer by its ID.",
		[]mcp.ToolOption{
			mcp.WithNumber("nodebalancer_id", mcp.Required(),
				mcp.Description("The ID of the NodeBalancer whose configs should be listed")),
		},
		handleLinodeNodeBalancerConfigListRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeNodeBalancerConfigNodesListTool creates a tool for listing nodes on a NodeBalancer config.
func NewLinodeNodeBalancerConfigNodesListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_nodebalancer_config_nodes_list",
		"Lists backend nodes for a specific NodeBalancer config.",
		[]mcp.ToolOption{
			mcp.WithNumber("nodebalancer_id", mcp.Required(),
				mcp.Description("The ID of the NodeBalancer whose config nodes should be listed")),
			mcp.WithNumber("config_id", mcp.Required(),
				mcp.Description("The ID of the NodeBalancer config whose nodes should be listed")),
			mcp.WithNumber("page", mcp.Description("Page number to retrieve")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page, from 25 through 500")),
		},
		handleLinodeNodeBalancerConfigNodesListRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeNodeBalancerConfigGetTool creates a tool for getting one config on a NodeBalancer.
func NewLinodeNodeBalancerConfigGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_nodebalancer_config_get",
		"Gets one config for a specific NodeBalancer by IDs.",
		[]mcp.ToolOption{
			mcp.WithNumber("nodebalancer_id", mcp.Required(),
				mcp.Description("The ID of the NodeBalancer whose config should be retrieved")),
			mcp.WithNumber("config_id", mcp.Required(),
				mcp.Description("The ID of the NodeBalancer config to retrieve")),
		},
		handleLinodeNodeBalancerConfigGetRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeNodeBalancerConfigNodeGetTool creates a tool for getting one node on a NodeBalancer config.
func NewLinodeNodeBalancerConfigNodeGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_nodebalancer_config_node_get",
		"Gets a backend node for a specific NodeBalancer config.",
		[]mcp.ToolOption{
			mcp.WithNumber("nodebalancer_id", mcp.Required(),
				mcp.Description("The ID of the NodeBalancer whose config node should be retrieved")),
			mcp.WithNumber("config_id", mcp.Required(),
				mcp.Description("The ID of the NodeBalancer config whose node should be retrieved")),
			mcp.WithNumber("node_id", mcp.Required(),
				mcp.Description("The ID of the NodeBalancer config node to retrieve")),
		},
		handleLinodeNodeBalancerConfigNodeGetRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeNodeBalancerConfigCreateTool creates a tool for creating a config on a NodeBalancer.
func NewLinodeNodeBalancerConfigCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_nodebalancer_config_create",
		"Creates a config for a specific NodeBalancer by its ID. Pass dry_run=true to preview without creating.",
		[]mcp.ToolOption{
			mcp.WithNumber("nodebalancer_id", mcp.Required(),
				mcp.Description("The ID of the NodeBalancer that should receive a new config")),
			mcp.WithNumber(nodeBalancerConfigKeyPort, mcp.Required(),
				mcp.Description("The TCP port this config listens on, from 1 through 65535")),
			mcp.WithString(nodeBalancerConfigKeyProtocol,
				mcp.Description("Optional protocol: http, https, or tcp")),
			mcp.WithString("algorithm",
				mcp.Description("Optional balancing algorithm: roundrobin, leastconn, or source")),
			mcp.WithString("stickiness",
				mcp.Description("Optional session stickiness: none, table, or http_cookie")),
			mcp.WithString("check",
				mcp.Description("Optional health check mode: none, connection, http, or http_body")),
			mcp.WithNumber("check_interval", mcp.Description("Optional health check interval in seconds")),
			mcp.WithNumber("check_timeout", mcp.Description("Optional health check timeout in seconds")),
			mcp.WithNumber("check_attempts", mcp.Description("Optional health check attempt count")),
			mcp.WithString("check_path", mcp.Description("Optional HTTP health check path")),
			mcp.WithString("check_body", mcp.Description("Optional expected HTTP health check body")),
			mcp.WithBoolean("check_passive", mcp.Description("Optionally enable passive health checks")),
			mcp.WithString("cipher_suite", mcp.Description("Optional HTTPS cipher suite")),
			mcp.WithString(nodeBalancerConfigKeySSLCert, mcp.Description("Optional HTTPS certificate PEM")),
			mcp.WithString(nodeBalancerConfigKeySSLKey, mcp.Description("Optional HTTPS private key PEM")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm NodeBalancer config creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeNodeBalancerConfigCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

// NewLinodeNodeBalancerNodeCreateTool creates a tool for creating a node on a NodeBalancer config.
func NewLinodeNodeBalancerNodeCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_nodebalancer_node_create",
		"Creates a backend node for a specific NodeBalancer config. Pass dry_run=true to preview without creating.",
		[]mcp.ToolOption{
			mcp.WithNumber("nodebalancer_id", mcp.Required(),
				mcp.Description("The ID of the NodeBalancer that owns the config")),
			mcp.WithNumber("config_id", mcp.Required(),
				mcp.Description("The ID of the NodeBalancer config that should receive a new node")),
			mcp.WithString("label", mcp.Required(),
				mcp.Description("Label for the backend node")),
			mcp.WithString("address", mcp.Required(),
				mcp.Description("Backend node address, including port, for example 192.0.2.10:80")),
			mcp.WithNumber("weight", mcp.Description("Optional traffic weight for this node")),
			mcp.WithString("mode", mcp.Description("Optional node mode: accept, reject, drain, or backup")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm NodeBalancer node creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeNodeBalancerNodeCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

// NewLinodeNodeBalancerGetTool creates a tool for getting a single NodeBalancer.
func NewLinodeNodeBalancerGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_nodebalancer_get",
		mcp.WithDescription("Gets detailed information about a specific NodeBalancer by its ID."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber(
			"nodebalancer_id",
			mcp.Required(),
			mcp.Description("The ID of the NodeBalancer to retrieve"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNodeBalancerGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeInstanceNodeBalancerListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	nodeBalancers, err := client.ListInstanceNodeBalancers(ctx, linodeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list NodeBalancers for instance %d: %v", linodeID, err)), nil
	}

	response := struct {
		Count         int                   `json:"count"`
		NodeBalancers []linode.NodeBalancer `json:"nodebalancers"`
	}{
		Count:         len(nodeBalancers),
		NodeBalancers: nodeBalancers,
	}

	return MarshalToolResponse(response)
}

func handleLinodeNodeBalancerConfigListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	nodeBalancerID, validationMessage := nodeBalancerIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	configs, err := client.ListNodeBalancerConfigs(ctx, nodeBalancerID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list configs for NodeBalancer %d: %v", nodeBalancerID, err)), nil
	}

	response := struct {
		Count   int                         `json:"count"`
		Configs []linode.NodeBalancerConfig `json:"configs"`
	}{
		Count:   len(configs),
		Configs: configs,
	}

	return MarshalToolResponse(response)
}

func handleLinodeNodeBalancerConfigNodesListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	nodeBalancerID, validationMessage := nodeBalancerIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	configID, validationMessage := nodeBalancerConfigIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	page, validationMessage := optionalPaginationInt(request.GetArguments(), "page", 1, 0)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	pageSize, validationMessage := optionalPaginationInt(request.GetArguments(), "page_size", nodeBalancerConfigNodesPageSizeMin, nodeBalancerConfigNodesPageSizeMax)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	nodes, err := client.ListNodeBalancerConfigNodes(ctx, nodeBalancerID, configID, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list nodes for NodeBalancer %d config %d: %v", nodeBalancerID, configID, err)), nil
	}

	return MarshalToolResponse(nodes)
}

func handleLinodeNodeBalancerConfigGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	nodeBalancerID, validationMessage := nodeBalancerIDFromTool(request)
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

	nodeBalancerConfig, err := client.GetNodeBalancerConfig(ctx, nodeBalancerID, configID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve config %d for NodeBalancer %d: %v", configID, nodeBalancerID, err)), nil
	}

	return MarshalToolResponse(nodeBalancerConfig)
}

func handleLinodeNodeBalancerConfigNodeGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	nodeBalancerID, validationMessage := nodeBalancerIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	configID, validationMessage := nodeBalancerConfigIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	nodeID, validationMessage := nodeBalancerConfigNodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	node, err := client.GetNodeBalancerConfigNode(ctx, nodeBalancerID, configID, nodeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve node %d for NodeBalancer %d config %d: %v", nodeID, nodeBalancerID, configID, err)), nil
	}

	return MarshalToolResponse(node)
}

func handleLinodeNodeBalancerConfigCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	nodeBalancerID, validationMessage := nodeBalancerIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	req, validationMessage := nodeBalancerConfigCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return handleLinodeNodeBalancerConfigCreateDryRun(ctx, request, cfg, nodeBalancerID)
	}

	if result := RequireConfirm(request, "This creates a NodeBalancer config. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	nodeBalancerConfig, err := client.CreateNodeBalancerConfig(ctx, nodeBalancerID, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create config for NodeBalancer %d: %v", nodeBalancerID, err)), nil
	}

	if nodeBalancerConfig == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create config for NodeBalancer %d: empty response", nodeBalancerID)), nil
	}

	response := struct {
		Message string                     `json:"message"`
		Config  *linode.NodeBalancerConfig `json:"config"`
	}{
		Message: fmt.Sprintf("NodeBalancer config %d created successfully for NodeBalancer %d", nodeBalancerConfig.ID, nodeBalancerID),
		Config:  nodeBalancerConfig,
	}

	return MarshalToolResponse(response)
}

func handleLinodeNodeBalancerConfigCreateDryRun(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config, nodeBalancerID int) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	configs, err := client.ListNodeBalancerConfigs(ctx, nodeBalancerID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch NodeBalancer configs for dry-run: %v", err)), nil
	}

	currentState := struct {
		NodeBalancerID int                         `json:"nodebalancer_id"`
		Configs        []linode.NodeBalancerConfig `json:"configs"`
	}{
		NodeBalancerID: nodeBalancerID,
		Configs:        configs,
	}

	return BuildDryRunResponse(
		"linode_nodebalancer_config_create",
		request.GetString(paramEnvironment, ""),
		"POST",
		fmt.Sprintf("/nodebalancers/%d/configs", nodeBalancerID),
		currentState,
	)
}

func nodeBalancerConfigCreateRequestFromTool(request *mcp.CallToolRequest) (linode.CreateNodeBalancerConfigRequest, string) {
	args := request.GetArguments()
	if _, exists := args[nodeBalancerConfigKeyPort]; !exists {
		return linode.CreateNodeBalancerConfigRequest{}, "port is required"
	}

	port, validationMessage := optionalPaginationInt(args, nodeBalancerConfigKeyPort, 1, nodeBalancerConfigPortMax)
	if validationMessage != "" {
		return linode.CreateNodeBalancerConfigRequest{}, validationMessage
	}

	req := linode.CreateNodeBalancerConfigRequest{Port: port}

	var message string
	if req.Protocol, message = optionalNodeBalancerConfigChoice(request, nodeBalancerConfigKeyProtocol, []string{nodeBalancerConfigProtocolHTTP, nodeBalancerConfigProtocolHTTPS, nodeBalancerConfigProtocolTCP}); message != "" {
		return linode.CreateNodeBalancerConfigRequest{}, message
	}

	if req.Algorithm, message = optionalNodeBalancerConfigChoice(request, "algorithm", []string{nodeBalancerConfigAlgorithmRoundRobin, nodeBalancerConfigAlgorithmLeastConn, nodeBalancerConfigAlgorithmSource}); message != "" {
		return linode.CreateNodeBalancerConfigRequest{}, message
	}

	if req.Stickiness, message = optionalNodeBalancerConfigChoice(request, "stickiness", []string{nodeBalancerConfigStickinessNone, nodeBalancerConfigStickinessTable, nodeBalancerConfigStickinessHTTPCookie}); message != "" {
		return linode.CreateNodeBalancerConfigRequest{}, message
	}

	if req.Check, message = optionalNodeBalancerConfigChoice(request, "check", []string{nodeBalancerConfigCheckNone, nodeBalancerConfigCheckConnection, nodeBalancerConfigCheckHTTP, nodeBalancerConfigCheckHTTPBody}); message != "" {
		return linode.CreateNodeBalancerConfigRequest{}, message
	}

	if req.CipherSuite, message = optionalNodeBalancerConfigChoice(request, "cipher_suite", []string{nodeBalancerConfigCipherRecommended, nodeBalancerConfigCipherLegacy}); message != "" {
		return linode.CreateNodeBalancerConfigRequest{}, message
	}

	if req.CheckInterval, message = optionalNodeBalancerConfigInt(args, "check_interval"); message != "" {
		return linode.CreateNodeBalancerConfigRequest{}, message
	}

	if req.CheckTimeout, message = optionalNodeBalancerConfigInt(args, "check_timeout"); message != "" {
		return linode.CreateNodeBalancerConfigRequest{}, message
	}

	if req.CheckAttempts, message = optionalNodeBalancerConfigInt(args, "check_attempts"); message != "" {
		return linode.CreateNodeBalancerConfigRequest{}, message
	}

	req.CheckPath = request.GetString("check_path", "")
	req.CheckBody = request.GetString("check_body", "")

	req.SSLCert = request.GetString(nodeBalancerConfigKeySSLCert, "")
	req.SSLKey = request.GetString(nodeBalancerConfigKeySSLKey, "")

	if req.Protocol == nodeBalancerConfigProtocolHTTPS && (req.SSLCert == "" || req.SSLKey == "") {
		return linode.CreateNodeBalancerConfigRequest{}, "ssl_cert and ssl_key are required when protocol is https"
	}

	if raw, exists := args["check_passive"]; exists {
		v, ok := raw.(bool)
		if !ok {
			return linode.CreateNodeBalancerConfigRequest{}, "check_passive must be a boolean"
		}

		req.CheckPassive = &v
	}

	return req, ""
}

func optionalNodeBalancerConfigInt(args map[string]any, key string) (int, string) {
	if _, exists := args[key]; !exists {
		return 0, ""
	}

	return optionalPaginationInt(args, key, 1, 0)
}

func optionalNodeBalancerConfigChoice(request *mcp.CallToolRequest, key string, allowed []string) (string, string) {
	value := request.GetString(key, "")
	if value == "" {
		return "", ""
	}

	if slices.Contains(allowed, value) {
		return value, ""
	}

	return "", fmt.Sprintf("%s must be one of: %s", key, strings.Join(allowed, ", "))
}

func nodeBalancerConfigIDFromTool(request *mcp.CallToolRequest) (int, string) {
	args := request.GetArguments()
	if _, exists := args["config_id"]; !exists {
		return 0, "config_id is required"
	}

	configID, validationMessage := optionalPaginationInt(args, "config_id", 1, 0)
	if validationMessage != "" {
		return 0, validationMessage
	}

	return configID, ""
}

func nodeBalancerConfigNodeIDFromTool(request *mcp.CallToolRequest) (int, string) {
	args := request.GetArguments()
	if _, exists := args["node_id"]; !exists {
		return 0, "node_id is required"
	}

	nodeID, validationMessage := optionalPaginationInt(args, "node_id", 1, 0)
	if validationMessage != "" {
		return 0, validationMessage
	}

	return nodeID, ""
}

func nodeBalancerIDFromTool(request *mcp.CallToolRequest) (int, string) {
	args := request.GetArguments()
	if _, exists := args["nodebalancer_id"]; !exists {
		return 0, "nodebalancer_id is required"
	}

	nodeBalancerID, validationMessage := optionalPaginationInt(args, "nodebalancer_id", 1, 0)
	if validationMessage != "" {
		return 0, validationMessage
	}

	return nodeBalancerID, ""
}

func handleLinodeNodeBalancerNodeCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	nodeBalancerID, validationMessage := nodeBalancerIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	configID, validationMessage := nodeBalancerConfigIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	req, validationMessage := nodeBalancerNodeCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return BuildDryRunResponse(
			"linode_nodebalancer_node_create",
			request.GetString(paramEnvironment, ""),
			"POST",
			fmt.Sprintf("/nodebalancers/%d/configs/%d/nodes", nodeBalancerID, configID),
			map[string]any{"nodebalancer_id": nodeBalancerID, "config_id": configID},
		)
	}

	if result := RequireConfirm(request, "This creates a NodeBalancer node. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	node, err := client.CreateNodeBalancerNode(ctx, nodeBalancerID, configID, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create node for NodeBalancer %d config %d: %v", nodeBalancerID, configID, err)), nil
	}

	if node == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create node for NodeBalancer %d config %d: empty response", nodeBalancerID, configID)), nil
	}

	response := struct {
		Message string                   `json:"message"`
		Node    *linode.NodeBalancerNode `json:"node"`
	}{
		Message: fmt.Sprintf("NodeBalancer node %d created successfully for NodeBalancer %d config %d", node.ID, nodeBalancerID, configID),
		Node:    node,
	}

	return MarshalToolResponse(response)
}

func nodeBalancerNodeCreateRequestFromTool(request *mcp.CallToolRequest) (linode.CreateNodeBalancerNodeRequest, string) {
	label := strings.TrimSpace(request.GetString("label", ""))
	if label == "" {
		return linode.CreateNodeBalancerNodeRequest{}, errLabelRequired
	}

	address := strings.TrimSpace(request.GetString("address", ""))
	if address == "" {
		return linode.CreateNodeBalancerNodeRequest{}, nodeBalancerNodeErrAddressRequired
	}

	args := request.GetArguments()
	req := linode.CreateNodeBalancerNodeRequest{Label: label, Address: address}

	if _, exists := args["weight"]; exists {
		weight, message := optionalPaginationInt(args, "weight", 1, 0)
		if message != "" {
			return linode.CreateNodeBalancerNodeRequest{}, message
		}

		req.Weight = weight
	}

	if req.Mode = request.GetString("mode", ""); req.Mode != "" {
		if !slices.Contains([]string{"accept", "reject", "drain", "backup"}, req.Mode) {
			return linode.CreateNodeBalancerNodeRequest{}, "mode must be one of: accept, reject, drain, backup"
		}
	}

	return req, ""
}

func handleLinodeNodeBalancerGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	nodeBalancerID := request.GetInt("nodebalancer_id", 0)

	if nodeBalancerID == 0 {
		return mcp.NewToolResultError("nodebalancer_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	nodeBalancer, err := client.GetNodeBalancer(ctx, nodeBalancerID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve NodeBalancer %d: %v", nodeBalancerID, err)), nil
	}

	return MarshalToolResponse(nodeBalancer)
}
