package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
	"github.com/chadit/LinodeMCP/go/internal/toolvalidate"
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

// ArgumentLen counts the entries a list argument carries, which is what a
// message template's {name:len} form renders. A caller who sent no list, or
// sent something that is not one, has sent nothing to count.
func ArgumentLen(request *mcp.CallToolRequest, name string) int {
	entries, listed := request.GetArguments()[name].([]any)
	if !listed {
		return 0
	}

	return len(entries)
}

// StandardPageQuery reads the page controls a route publishes and encodes them,
// answering a non-empty message when either is out of range. A read tier tool
// gets it because some single-resource routes publish the same controls a
// collection does; the list tiers read theirs through the driver instead.
// A control the caller left out reads back as zero and is dropped rather than
// sent, which is what the hand-written reads these replace put on the wire: the
// route's own default is not the same number as the reader's.
func StandardPageQuery(request *mcp.CallToolRequest) (string, string) {
	page, pageSize, message := standardPaginationFromTool(request)
	if message != "" {
		return "", message
	}

	values := url.Values{}

	if page > 0 {
		values.Set(paramPage, strconv.Itoa(page))
	}

	if pageSize > 0 {
		values.Set(paramPageSize, strconv.Itoa(pageSize))
	}

	return values.Encode(), ""
}

// PathWithQuery is the route a preview reports: the endpoint, with the encoded
// query behind it when the caller supplied anything the route takes one for.
//
// A preview reporting the bare path over a call that carries page controls
// would describe a different request from the one the live branch makes, which
// is the one thing a dry run cannot afford to do. linodemcp.tools.drivers
// spells it the same way, so both clients report one route.
func PathWithQuery(path, query string) string {
	if query == "" {
		return path
	}

	return path + "?" + query
}

// WithQueryArguments appends the named arguments to an already-encoded query,
// skipping the ones the caller left out. These are the parameters the route
// itself filters on, so an absent one must not reach it as an empty value: an
// empty prefix and no prefix select different objects.
func WithQueryArguments(request *mcp.CallToolRequest, query string, names ...string) string {
	values := url.Values{}
	arguments := request.GetArguments()

	for _, name := range names {
		raw, supplied := arguments[name]
		if !supplied {
			continue
		}

		if text := queryArgumentText(raw); text != "" {
			values.Set(name, text)
		}
	}

	encoded := values.Encode()

	if query == "" || encoded == "" {
		return query + encoded
	}

	return query + "&" + encoded
}

// StateReadQuery is the query a declared fetch addresses its read with: each of
// that read's own query parameters paired with the argument this tool fills it
// from.
//
// The two names are usually the same, and the pairing is what lets them differ
// the way a path slot's mapping does. A parameter whose argument the call did
// not carry is left off, which is what an optional one means.
func StateReadQuery(request *mcp.CallToolRequest, filled map[string]string) string {
	values := url.Values{}
	arguments := request.GetArguments()

	for parameter, argument := range filled {
		raw, supplied := arguments[argument]
		if !supplied {
			continue
		}

		if text := queryArgumentText(raw); text != "" {
			values.Set(parameter, text)
		}
	}

	return values.Encode()
}

// queryArgumentText renders one query argument as the API reads it. A flag is
// sent only when it is true, which is how skip_ipv6_rdns has always traveled:
// sending "false" asks the route for something it does not offer.
func queryArgumentText(raw any) string {
	switch value := raw.(type) {
	case bool:
		if value {
			return "true"
		}

		return ""
	case string:
		return value
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	}

	return ""
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

// NewMemberFilter builds a filter that keeps items one of whose repeated-field
// entries equals the argument, case-insensitively. A region answers a single
// asked-for capability out of the list it publishes.
func NewMemberFilter[T proto.Message](param, description string, getField func(T) []string) ListFilter[T] {
	return ListFilter[T]{
		Param:       param,
		Description: description,
		Match: func(items []T, value string) []T {
			return FilterByMember(items, value, getField)
		},
	}
}

// NewBoolFilter builds a filter over a boolean field, which the caller selects
// by sending "true" or "false" because every filter argument arrives as text.
func NewBoolFilter[T proto.Message](param, description string, getField func(T) bool) ListFilter[T] {
	return ListFilter[T]{
		Param:       param,
		Description: description,
		Match:       boolFilter(param, description, getField).matchFunc,
	}
}

// ListPathValue reads one of a sub-resource list's required path parameters,
// the domain_id in /domains/{domain_id}/records. It returns the value the route
// template's slot is filled with, or a non-empty message the handler reports as
// the tool error instead of calling the API.
//
// The value is any rather than int because a nested collection is addressed by
// whatever its path field declares: a numeric id, a bucket label, or an enum
// slug such as the tier in /lke/tiers/{tier}/versions.
type ListPathValue func(request *mcp.CallToolRequest) (any, string)

// ListValidate is a tool's declared argument check, which replaces the
// required-value checks a generated list would otherwise derive.
type ListValidate func(request *mcp.CallToolRequest) string

// ListFailureText words a collection's failed fetch from the tool's own
// declared sentence. It takes the request because such a sentence names the
// ids the call was addressed by, which is the whole reason a family declares
// one instead of answering the shared list sentence.
type ListFailureText func(request *mcp.CallToolRequest, err error) string

// NewGeneratedListTool builds a paginated top-level list tool that advertises
// the generated input contract named by schemaName. validate is nil unless the
// tool declares a check of its own.
func NewGeneratedListTool[T, R proto.Message](
	cfg *config.Config,
	toolName, description, schemaName string,
	failure ListFailureText,
	validate ListValidate,
	fetch func(ctx context.Context, client *linode.Client, request *mcp.CallToolRequest, page, pageSize int) ([]T, error),
	filters []ListFilter[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, Handler) {
	nested := func(
		ctx context.Context, client *linode.Client, request *mcp.CallToolRequest,
		_ []any, page, pageSize int,
	) ([]T, error) {
		return fetch(ctx, client, request, page, pageSize)
	}

	return NewGeneratedSubresourceListTool(
		cfg, toolName, description, schemaName, failure, validate, nil, nested, filters, assemble,
	)
}

// NewGeneratedSubresourceListTool is NewGeneratedListTool for a collection
// nested under path parameters. readPath reads them in route-template order,
// which is the order fetch fills the template's slots from, so a two-id route
// cannot address one resource while reporting another.
//
// failure words a failed fetch from the tool's own declared sentence. It is a
// closure rather than a sentence because the tool's template names the ids the
// call was addressed by, which only the handler has read. A nil one falls back
// to the sentence the whole list surface has always answered, which is what a
// contract declaring no error_message asks for.
func NewGeneratedSubresourceListTool[T, R proto.Message](
	cfg *config.Config,
	toolName, description, schemaName string,
	failure ListFailureText,
	validate ListValidate,
	readPath []ListPathValue,
	fetch func(
		ctx context.Context, client *linode.Client, request *mcp.CallToolRequest,
		values []any, page, pageSize int,
	) ([]T, error),
	filters []ListFilter[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, Handler) {
	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// A collection reaches its rules and its argument refusals here rather
		// than through an emitted call because the driver already holds the
		// message name both are derived from, so the emitter has nothing left
		// to write.
		if message := toolvalidate.Check(schemaName, request.GetArguments()); message != "" {
			return mcp.NewToolResultError(message), nil
		}

		if message := CheckArgumentRefusals(schemaName, toolName, request.GetArguments()); message != "" {
			return mcp.NewToolResultError(message), nil
		}

		values, message := readListPath(&request, validate, readPath)
		if message != "" {
			return mcp.NewToolResultError(message), nil
		}

		page, pageSize, message := standardPaginationFromTool(&request)
		if message != "" {
			return mcp.NewToolResultError(message), nil
		}

		client, err := prepareClient(&request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		items, err := fetch(ctx, client, &request, values, page, pageSize)
		if err != nil {
			return mcp.NewToolResultError(listFailure(failure, &request, err)), nil
		}

		return finishProtoList(&request, items, internalFilters(filters), assemble)
	}

	tool := mcp.NewToolWithRawSchema(toolName, description, toolschemas.Schema(schemaName))

	return tool, handler
}

// collectionStatePage is the page a synthesized collection fetch reads. One
// call at the standard maximum is what the route can hand over at once, and the
// collections this serves are small nested sets: the certificates trusted on one
// identity-provider configuration. An element past that page reports as missing
// rather than being previewed as something else.
const collectionStatePage = 1

// FetchCollectionElement reads the resource a removal is about to take away out
// of the collection it belongs to, for the removals whose API publishes no GET
// on the route they delete.
//
// values fill the collection's own route, which is the removal's path with its
// trailing id dropped; match picks the element that id names. The answer is the
// element rather than the page, so a preview and a plan hash read the same shape
// they would from an exact-path sibling.
func FetchCollectionElement[T proto.Message](
	ctx context.Context,
	client *linode.Client,
	tool string,
	values []any,
	id any,
	newElem func() T,
	match func(T) bool,
) (any, error) {
	items, raws, err := linode.ListProtoRouteRaw(
		ctx, client, tool, values, "", collectionStatePage, standardPageSizeMax, newElem,
	)

	return collectionElement(items, raws, err, tool, id, match)
}

// collectionElement picks the element match names out of a fetched page, or
// hands the fetch's own failure back. It takes the fetch's results the way
// ProtoStateList does, which is what keeps a failed fetch reporting the route's
// own sentence rather than one this seam wrapped around it.
//
// The element is reported from the body it decoded from, so a resource read out
// of a page carries the same state a resource read on its own route does.
func collectionElement[T proto.Message](
	items []T, raws []json.RawMessage, err error, tool string, id any, match func(T) bool,
) (any, error) {
	if err != nil {
		return nil, err
	}

	// One raw body per element, both built from the same page, so the index a
	// match lands on names the body that element decoded from.
	for index, item := range items {
		if match(item) {
			return ProjectDeclaredState(raws[index], item)
		}
	}

	return nil, fmt.Errorf("%w: %s carries no id '%v'", ErrCollectionElement, tool, id)
}

// NewGeneratedNullListTool builds a collection whose elements carry documented
// explicit nulls: the fetch keeps the body each element decoded from, and the
// answer writes the named keys back into the element they came from.
//
// It takes no filters, which is what keeps element i of the answer element i of
// the fetch. A narrowed page would leave the two lists a different length, and
// the restoration is by position because that is the only pairing a page body
// offers.
func NewGeneratedNullListTool[T, R proto.Message](
	cfg *config.Config,
	toolName, description, schemaName string,
	failure ListFailureText,
	validate ListValidate,
	readPath []ListPathValue,
	fetch func(
		ctx context.Context, client *linode.Client, request *mcp.CallToolRequest,
		values []any, page, pageSize int,
	) ([]T, []json.RawMessage, error),
	nulls []string,
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, Handler) {
	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if message := toolvalidate.Check(schemaName, request.GetArguments()); message != "" {
			return mcp.NewToolResultError(message), nil
		}

		if message := CheckArgumentRefusals(schemaName, toolName, request.GetArguments()); message != "" {
			return mcp.NewToolResultError(message), nil
		}

		values, message := readListPath(&request, validate, readPath)
		if message != "" {
			return mcp.NewToolResultError(message), nil
		}

		page, pageSize, message := standardPaginationFromTool(&request)
		if message != "" {
			return mcp.NewToolResultError(message), nil
		}

		client, err := prepareClient(&request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		items, raws, err := fetch(ctx, client, &request, values, page, pageSize)
		if err != nil {
			return mcp.NewToolResultError(listFailure(failure, &request, err)), nil
		}

		return MarshalProtoListResponseRestoringNulls(
			assemble(items, ClampListCount(items), nil), raws, nulls,
		)
	}

	tool := mcp.NewToolWithRawSchema(toolName, description, toolschemas.Schema(schemaName))

	return tool, handler
}

// NewGeneratedMarkerListTool builds a collection whose route pages by a cursor
// rather than by number: the body answers {data, is_truncated, next_marker} and
// the caller resumes from the marker it was handed.
//
// echoes names the forwarded query arguments the answer reports it was narrowed
// by. The cursor arguments are left out on purpose: the marker and the page
// bound say where a page starts and how big it is, not which objects it holds,
// so echoing them would report a position as a filter.
//
// assemble takes the cursor alongside the items because the response carries
// both, and a truncated page whose marker went missing reads to a caller as the
// end of the collection.
func NewGeneratedMarkerListTool[T, R proto.Message](
	cfg *config.Config,
	toolName, description, schemaName string,
	failure ListFailureText,
	validate ListValidate,
	readPath []ListPathValue,
	fetch func(
		ctx context.Context, client *linode.Client, request *mcp.CallToolRequest, values []any,
	) ([]T, linode.ListMarkerPage, error),
	echoes []string,
	assemble func(items []T, count int32, filter *string, cursor linode.ListMarkerPage) R,
) (mcp.Tool, Handler) {
	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if message := toolvalidate.Check(schemaName, request.GetArguments()); message != "" {
			return mcp.NewToolResultError(message), nil
		}

		if message := CheckArgumentRefusals(schemaName, toolName, request.GetArguments()); message != "" {
			return mcp.NewToolResultError(message), nil
		}

		values, message := readListPath(&request, validate, readPath)
		if message != "" {
			return mcp.NewToolResultError(message), nil
		}

		client, err := prepareClient(&request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		items, cursor, err := fetch(ctx, client, &request, values)
		if err != nil {
			return mcp.NewToolResultError(listFailure(failure, &request, err)), nil
		}

		return MarshalProtoToolResponse(
			assemble(items, ClampListCount(items), echoedFilter(&request, echoes), cursor),
		)
	}

	tool := mcp.NewToolWithRawSchema(toolName, description, toolschemas.Schema(schemaName))

	return tool, handler
}

// TextOrAbsent is an optional response member filled only when the API sent
// something. A marker-paged answer leaves next_marker out on a complete listing
// rather than reporting an empty cursor a caller might resume from.
func TextOrAbsent(text string) *string {
	if text == "" {
		return nil
	}

	return &text
}

// ListFailure is the sentence every collection reports a failed fetch with when
// its contract declares none, which is nearly all of them: naming the tool a
// page came from adds nothing a caller does not already know.
const ListFailure = "Failed to retrieve items"

// listFailure words one failed fetch, from the tool's own sentence or the
// shared one.
func listFailure(failure ListFailureText, request *mcp.CallToolRequest, err error) string {
	if failure == nil {
		return fmt.Sprintf("%s: %v", ListFailure, err)
	}

	return failure(request, err)
}

// readListPath resolves a list's path values, running the tool's own check in
// place of the derived ones when it declares one. Reading every value before
// the first fetch is what keeps a route from being filled halfway.
func readListPath(
	request *mcp.CallToolRequest, validate ListValidate, readPath []ListPathValue,
) ([]any, string) {
	if validate != nil {
		if message := validate(request); message != "" {
			return nil, message
		}
	}

	values := make([]any, 0, len(readPath))

	for _, read := range readPath {
		value, message := read(request)
		if message != "" {
			return nil, message
		}

		values = append(values, value)
	}

	return values, ""
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
