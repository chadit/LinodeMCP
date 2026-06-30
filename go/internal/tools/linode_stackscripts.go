package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// NewLinodeStackScriptGetTool creates a tool for retrieving one StackScript.
func NewLinodeStackScriptGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_stackscript_get",
		"Gets one StackScript by ID.",
		toolschemas.Schema("linode.mcp.v1.StackScriptGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeStackScriptGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// stackScriptListFilters returns the is_public / mine / label_contains client-side
// filters for linode_stackscript_list. Splitting them out of the factory keeps the
// constructor body distinct from the other proto-list factories (dupl threshold).
func stackScriptListFilters() []listFilterParam[*linodev1.StackScript] {
	return []listFilterParam[*linodev1.StackScript]{
		boolFilter("is_public", "Filter by public status (true, false)",
			func(s *linodev1.StackScript) bool { return s.GetIsPublic() }),
		boolFilter("mine", "Filter by ownership - only your own StackScripts (true, false)",
			func(s *linodev1.StackScript) bool { return s.GetMine() }),
		containsFilter("label_contains", "Filter StackScripts by label containing this string (case-insensitive)",
			func(s *linodev1.StackScript) string { return s.GetLabel() }),
	}
}

// NewLinodeStackScriptListTool creates a tool for listing StackScripts.
func NewLinodeStackScriptListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListTool(
		cfg,
		"linode_stackscript_list",
		"Lists StackScripts. By default returns your own StackScripts. Can filter by public status, ownership, or label.",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.StackScript, error) {
			return client.ListStackScriptsProto(ctx)
		},
		stackScriptListFilters(),
		stackScriptListResponse,
	)

	return tool, profiles.CapRead, handler
}

func stackScriptListResponse(items []*linodev1.StackScript, count int32, filter *string) *linodev1.StackScriptListResponse {
	return &linodev1.StackScriptListResponse{Count: count, Filter: filter, Stackscripts: items}
}

func handleLinodeStackScriptGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	stackScriptID, validationMessage := stackScriptIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	script, err := client.GetStackScriptProto(ctx, stackScriptID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve StackScript: %v", err)), nil
	}

	return MarshalProtoToolResponse(script)
}

func stackScriptIDFromTool(request *mcp.CallToolRequest) (int, string) {
	raw, exists := request.GetArguments()["stackscript_id"]
	if !exists {
		return 0, "stackscript_id must be a positive integer"
	}

	stackScriptID, ok := numberArgToInt(raw)
	if !ok || stackScriptID <= 0 {
		return 0, "stackscript_id must be a positive integer"
	}

	return stackScriptID, ""
}
