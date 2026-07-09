package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

const (
	tagsPageSizeMin       = 25
	tagsPageSizeMax       = 500
	tagDomainsParam       = "domains"
	tagLinodesParam       = "linodes"
	tagNodeBalancersParam = "nodebalancers"
	tagVolumesParam       = "volumes"
)

// NewLinodeTagsTool creates a tool for listing account tags.
func NewLinodeTagsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListToolPaginated(
		cfg,
		"linode_tag_list",
		"Lists tags visible to the authenticated account.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.Tag, error) {
			return client.ListTagsProto(ctx, page, pageSize)
		},
		tagsPaginationFromTool,
		nil,
		tagListResponse,
	)

	tool := mcp.NewToolWithRawSchema(
		"linode_tag_list",
		"Lists tags visible to the authenticated account.",
		toolschemas.Schema("linode.mcp.v1.TagListInput"),
	)

	return tool, profiles.CapRead, handler
}

func tagListResponse(items []*linodev1.Tag, count int32, filter *string) *linodev1.TagListResponse {
	return &linodev1.TagListResponse{Count: count, Filter: filter, Tags: items}
}

// NewLinodeTagDeleteTool creates a tool for deleting an account tag.
func NewLinodeTagDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_tag_delete",
		"Deletes a tag from all objects on the account."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.TagDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeTagDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

func handleLinodeTagDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	tagLabel, validationMessage := deleteTagLabelArgFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return RunDestructiveAction(ctx, request, cfg, &DestructiveAction{
		ToolName:       "linode_tag_delete",
		Method:         httpMethodDelete,
		Path:           endpointPathTag(tagLabel),
		ConfirmMessage: "confirm must be true to delete a tag",
		FetchState: func(ctx context.Context, c *linode.Client) (any, error) {
			return c.ListTaggedObjects(ctx, tagLabel, 0, 0)
		},
		DependencyWalk: tagDeleteDependencyWalk,
		Execute: func(ctx context.Context, c *linode.Client) error {
			return c.DeleteTag(ctx, tagLabel)
		},
		Success: func() proto.Message {
			return &linodev1.MessageResponse{
				Message: fmt.Sprintf("Tag '%s' deleted successfully", tagLabel),
			}
		},
		// ListTaggedObjects returns a paginated list, not a single object,
		// so the whole payload is hashed; the unknown "Tag" key returns nil.
		HashIgnore: twostage.HashIgnoreFields("Tag"),
	})
}

func tagLabelArgFromTool(request *mcp.CallToolRequest) (string, string) {
	args := request.GetArguments()

	tagLabel, validationMessage := requiredStringArg(args, tagLabelParam)
	if validationMessage != "" {
		return "", validationMessage
	}

	if strings.ContainsAny(tagLabel, "?#") || strings.Contains(tagLabel, "..") {
		return "", errTagLabelPathParam
	}

	return tagLabel, ""
}

func endpointPathTag(tagLabel string) string {
	return "/tags/" + url.PathEscape(tagLabel)
}

func deleteTagLabelArgFromTool(request *mcp.CallToolRequest) (string, string) {
	return tagLabelArgFromTool(request)
}

func tagsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", tagsPageSizeMin, tagsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

// NewLinodeTagCreateTool creates a tool for creating a Linode tag.
func NewLinodeTagCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_tag_create",
		"Creates a Linode tag and optionally applies it to existing resources. Pass dry_run=true to preview without creating the tag.",
		toolschemas.Schema("linode.mcp.v1.TagCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeTagCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeTagCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := createTagRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreviewWithBodyDetailed(ctx, request, cfg, "linode_tag_create", httpMethodPost, "/tags", req, nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return tagCreateSideEffects(ctx, request.GetString("label", ""))
			})
	}

	if result := RequireConfirm(request, "This creates a Linode tag. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tag, err := client.CreateTagProto(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create tag: %v", err)), nil
	}

	response := &linodev1.TagWriteResponse{
		Message: fmt.Sprintf("Tag '%s' created successfully", tag.GetLabel()),
		Tag:     tag,
	}

	return MarshalProtoToolResponse(response)
}

func createTagRequestFromTool(request *mcp.CallToolRequest) (*linode.CreateTagRequest, string) {
	label := strings.TrimSpace(request.GetString("label", ""))
	if label == "" {
		return nil, errLabelRequired
	}

	req := &linode.CreateTagRequest{Label: label}
	args := request.GetArguments()

	for _, field := range []struct {
		name string
		dest *[]int
	}{
		{name: tagDomainsParam, dest: &req.Domains},
		{name: tagLinodesParam, dest: &req.Linodes},
		{name: tagNodeBalancersParam, dest: &req.NodeBalancers},
		{name: tagVolumesParam, dest: &req.Volumes},
	} {
		raw, exists := args[field.name]
		if !exists {
			continue
		}

		ids, validationMessage := intSliceFromToolArg(raw, field.name)
		if validationMessage != "" {
			return nil, validationMessage
		}

		*field.dest = ids
	}

	return req, ""
}
