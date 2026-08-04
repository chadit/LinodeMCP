package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// This file is the exported seam the generated internal/gentools package
// compiles against: everything a tool factory needs was unexported here. Names
// differ from their unexported counterparts by more than capitalization because
// revive's confusing-naming rule reads PrepareClient beside prepareClient as one
// name. Keep the list short: a new entry should be a driver the emitter calls,
// not a helper one tool wants.

// Handler is the signature every tool handler has, named here so the generated
// factories do not re-spell it per tool.
type Handler = func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)

// ClientForRequest resolves the request's environment against cfg and returns a
// client for it.
func ClientForRequest(request *mcp.CallToolRequest, cfg *config.Config) (*linode.Client, error) {
	return prepareClient(request, cfg)
}

// ListFilter is one client-side filter a list tool applies to the page it
// fetched. Build one with NewContainsFilter or NewFieldFilter rather than by
// hand, so the match semantics stay the ones the Python side mirrors.
type ListFilter[T proto.Message] struct {
	// Match keeps the items the filter's argument selects.
	Match func(items []T, value string) []T
	// Param is the tool argument that carries the filter value.
	Param string
	// Description documents the argument.
	Description string
}

// NewContainsFilter builds a filter that keeps items whose field contains the
// argument, case-insensitively.
func NewContainsFilter[T proto.Message](param, description string, getField func(T) string) ListFilter[T] {
	return ListFilter[T]{
		Param:       param,
		Description: description,
		Match: func(items []T, value string) []T {
			return FilterByContains(items, value, getField)
		},
	}
}

// NewFieldFilter builds a filter that keeps items whose field equals the
// argument, case-insensitively.
func NewFieldFilter[T proto.Message](param, description string, getField func(T) string) ListFilter[T] {
	return ListFilter[T]{
		Param:       param,
		Description: description,
		Match: func(items []T, value string) []T {
			return FilterByField(items, value, getField)
		},
	}
}

// ListPathID describes the required path parameter of a sub-resource list, the
// domain_id in /domains/{domain_id}/records. Parse returns the validated id, or
// a non-empty validation message the handler reports as the tool error.
type ListPathID struct {
	Option mcp.ToolOption
	Parse  func(request *mcp.CallToolRequest) (int, string)
}

// NewGeneratedListTool builds a paginated top-level list tool that advertises
// the generated input contract named by schemaName.
func NewGeneratedListTool[T, R proto.Message](
	cfg *config.Config,
	toolName, description, schemaName string,
	fetch func(ctx context.Context, client *linode.Client, page, pageSize int) ([]T, error),
	filters []ListFilter[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, Handler) {
	return newProtoListToolPaginatedRawSchema(
		cfg, toolName, description, schemaName,
		fetch, standardPaginationFromTool, internalFilters(filters), assemble,
	)
}

// NewGeneratedSubresourceListTool is NewGeneratedListTool for a collection
// nested under a required path id.
func NewGeneratedSubresourceListTool[T, R proto.Message](
	cfg *config.Config,
	toolName, description, schemaName string,
	pathID ListPathID,
	fetch func(ctx context.Context, client *linode.Client, pathID, page, pageSize int) ([]T, error),
	filters []ListFilter[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, Handler) {
	return newProtoListToolSubresourcePaginatedRawSchema(
		cfg, toolName, description, schemaName,
		protoListPathID{option: pathID.Option, parse: pathID.Parse},
		standardPaginationFromTool, fetch, internalFilters(filters), assemble,
	)
}

// internalFilters converts ListFilter into the list machinery's twin type. The
// two carry the same values; gentools just cannot name the unexported one.
func internalFilters[T proto.Message](filters []ListFilter[T]) []listFilterParam[T] {
	converted := make([]listFilterParam[T], 0, len(filters))

	for _, filter := range filters {
		converted = append(converted, listFilterParam[T]{
			paramName:   filter.Param,
			description: filter.Description,
			matchFunc:   filter.Match,
		})
	}

	return converted
}
