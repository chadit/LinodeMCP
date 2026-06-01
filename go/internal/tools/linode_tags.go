package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
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
	tool, handler := newToolWithHandler(
		cfg,
		"linode_tags",
		"Lists tags visible to the authenticated account.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeTagsRequest,
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeTagDeleteTool creates a tool for deleting an account tag.
func NewLinodeTagDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_tag_delete",
		"Deletes a tag from all objects on the account.",
		[]mcp.ToolOption{
			mcp.WithString(tagLabelParam, mcp.Required(), mcp.Description("Tag label to delete.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm tag deletion. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeTagDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleLinodeTagsRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := tagsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tags, listFailure := client.ListTags(ctx, page, pageSize)
	if listFailure == nil {
		return MarshalToolResponse(tags)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_tags: " + listFailure.Error()), nil
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
		Execute: func(ctx context.Context, c *linode.Client) error {
			return c.DeleteTag(ctx, tagLabel)
		},
		Success: func() any {
			return map[string]string{"deleted": tagLabel}
		},
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
	tool, handler := newToolWithHandler(
		cfg,
		"linode_tag_create",
		"Creates a Linode tag and optionally applies it to existing resources. Pass dry_run=true to preview without creating the tag.",
		[]mcp.ToolOption{
			mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
			mcp.WithString("label", mcp.Required(), mcp.Description("The tag label.")),
			mcp.WithArray(tagDomainsParam, mcp.Description("Optional domain IDs to tag.")),
			mcp.WithArray(tagLinodesParam, mcp.Description("Optional Linode IDs to tag.")),
			mcp.WithArray(tagNodeBalancersParam, mcp.Description("Optional NodeBalancer IDs to tag.")),
			mcp.WithArray(tagVolumesParam, mcp.Description("Optional volume IDs to tag.")),
			mcp.WithBoolean(paramConfirm,
				mcp.Description("Must be true to confirm tag creation. Omit when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeTagCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeTagCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := createTagRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreviewWithBody(ctx, request, cfg, "linode_tag_create", httpMethodPost, "/tags", req, nil)
	}

	if result := RequireConfirm(request, "This creates a Linode tag. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tag, err := client.CreateTag(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create tag: %v", err)), nil
	}

	response := struct {
		Message string      `json:"message"`
		Tag     *linode.Tag `json:"tag"`
	}{
		Message: fmt.Sprintf("Tag '%s' created successfully", tag.Label),
		Tag:     tag,
	}

	return MarshalToolResponse(response)
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
