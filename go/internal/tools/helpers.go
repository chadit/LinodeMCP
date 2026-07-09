package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync/atomic"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// Common MCP tool parameter names and descriptions used across all tools.
const (
	paramEnvironment     = "environment"
	paramEnvironmentDesc = "Linode environment to use (optional, defaults to 'default')"
	paramPage            = "page"
	paramPageSize        = "page_size"
	paramConfirm         = "confirm"
	paramDryRun          = "dry_run"
	paramDryRunDesc      = "Preview the call without making it: returns the would-be request and current resource state. Default false."
	paramLinodeID        = "linode_id"
	// paramConfirmedDryRun / paramConfirmBypassDryRun drive the Phase 3
	// bypass-dry-run gate on CapDestroy tools (see destroy.go). The model
	// asserts confirmed_dry_run after running a dry-run, or sets
	// confirm_bypass_dry_run to skip the preview explicitly.
	paramConfirmedDryRun     = "confirmed_dry_run"
	paramConfirmBypassDryRun = "confirm_bypass_dry_run"

	// paramYolo is the request flag that bypasses preview and confirm. The
	// server middleware honors it only when the active profile allows yolo.
	paramYolo = "yolo"

	// paramMode / paramPlanID drive the two-stage plan/apply flow on opted-in
	// CapDestroy tools (see twostage_destroy.go). mode:"plan" returns a plan_id
	// and a state hash; mode:"apply" with that plan_id re-checks for drift and
	// executes.
	paramMode       = "mode"
	paramModeDesc   = "Two-stage flow: \"plan\" previews and returns a plan_id; \"apply\" with plan_id re-checks drift and executes. Omit for a single-step call."
	paramPlanID     = "plan_id"
	paramPlanIDDesc = "The plan_id returned by a mode:\"plan\" call, supplied with mode:\"apply\" to execute it."

	// twoStageNote is appended to every opted-in delete tool's description so
	// the plan/apply flow shows up at the tool level, not only on the mode and
	// plan_id params. See docs/two-stage-writes.md.
	twoStageNote = " Supports two-stage writes: mode=\"plan\" returns a plan_id; mode=\"apply\" with that plan_id re-checks for drift, then executes."

	// twoStageOptInNote is the variant for a tool whose two-stage flow is off
	// until an operator enables it (e.g. instance_resize, a CapWrite tool that
	// does not opt in by capability default). See docs/two-stage-writes.md.
	twoStageOptInNote = " Supports two-stage writes when enabled in the two_stage config: mode=\"plan\" returns a plan_id; mode=\"apply\" with that plan_id re-checks for drift, then executes."
)

// liveConfigSource is the optional hot-reload provider. When set (by
// main.go via SetLiveConfigSource), prepareClient reads through it on each
// request so reloaded resilience/environment values take effect for new
// API calls. When unset, prepareClient falls back to the cfg captured at
// tool-registration time. Stored as an atomic pointer to a function so
// reads are lock-free and the global mutation is bounded to one place.
//
// Suppression must be inline on the offending declaration line so that
// newer golangci-lint releases associate it with the var.
var liveConfigSource atomic.Pointer[func() *config.Config] //nolint:gochecknoglobals // process-wide hot-reload bridge; touching every factory signature would be a 123-file refactor.

// SetLiveConfigSource registers a function that returns the latest Config.
// Pass nil to unregister. Safe for concurrent calls.
func SetLiveConfigSource(getCfg func() *config.Config) {
	if getCfg == nil {
		liveConfigSource.Store(nil)

		return
	}

	liveConfigSource.Store(&getCfg)
}

// resolveConfig returns the live config when a source is registered, else
// the snapshot the caller captured at registration time.
func resolveConfig(snapshot *config.Config) *config.Config {
	if fn := liveConfigSource.Load(); fn != nil && *fn != nil {
		if live := (*fn)(); live != nil {
			return live
		}
	}

	return snapshot
}

// prepareClient extracts the environment parameter, validates the config, and returns a ready-to-use API client.
// When a live config source is registered (see SetLiveConfigSource), the
// latest values flow through here so reloaded resilience and environment
// settings take effect on the very next tool call.
func prepareClient(request *mcp.CallToolRequest, cfg *config.Config) (*linode.Client, error) {
	cfg = resolveConfig(cfg)
	environment := request.GetString(paramEnvironment, "")

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return nil, err
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return nil, err
	}

	return linode.NewClient(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token, cfg), nil
}

// RequireConfirm checks that confirm is the literal JSON boolean true.
func RequireConfirm(request *mcp.CallToolRequest, message string) *mcp.CallToolResult {
	confirm, confirmOK := request.GetArguments()[paramConfirm].(bool)
	if !confirmOK || !confirm {
		return mcp.NewToolResultError(message)
	}

	return nil
}

// newSimpleProtoGetTool builds a no-id get tool whose input schema comes from
// the proto contract and whose response is serialized through
// MarshalProtoToolResponse.
func newSimpleProtoGetTool(
	cfg *config.Config,
	toolName, description, schemaName string,
	apiCall func(context.Context, *linode.Client) (proto.Message, error),
) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(toolName, description, toolschemas.Schema(schemaName))

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := prepareClient(&request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, err := apiCall(ctx, client)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve %s: %v", toolName, err)), nil
		}

		return MarshalProtoToolResponse(result)
	}

	return tool, handler
}

// FilterByField returns items where getField matches filter (case-insensitive).
func FilterByField[T any](items []T, filter string, getField func(T) string) []T {
	filtered := make([]T, 0, len(items))

	for i := range items {
		if strings.EqualFold(getField(items[i]), filter) {
			filtered = append(filtered, items[i])
		}
	}

	return filtered
}

// FilterByContains returns items where getField contains substr (case-insensitive).
func FilterByContains[T any](items []T, substr string, getField func(T) string) []T {
	filtered := make([]T, 0, len(items))

	lower := strings.ToLower(substr)

	for i := range items {
		if strings.Contains(strings.ToLower(getField(items[i])), lower) {
			filtered = append(filtered, items[i])
		}
	}

	return filtered
}

// listFilterParam combines a tool parameter definition with its filter logic for list tools.
type listFilterParam[T any] struct {
	paramName   string
	description string
	matchFunc   func(items []T, value string) []T
}

// fieldFilter creates a list filter parameter that matches a field exactly (case-insensitive).
func fieldFilter[T any](paramName, description string, getField func(T) string) listFilterParam[T] {
	return listFilterParam[T]{
		paramName:   paramName,
		description: description,
		matchFunc: func(items []T, value string) []T {
			return FilterByField(items, value, getField)
		},
	}
}

// containsFilter creates a list filter parameter that matches a field by substring (case-insensitive).
func containsFilter[T any](paramName, description string, getField func(T) string) listFilterParam[T] {
	return listFilterParam[T]{
		paramName:   paramName,
		description: description,
		matchFunc: func(items []T, value string) []T {
			return FilterByContains(items, value, getField)
		},
	}
}

// boolFilter creates a list filter parameter that keeps items whose bool field
// matches the parsed argument. The argument is true when it equals "true"
// case-insensitively, otherwise false, so the filter accepts the same
// "true"/"false" inputs the pre-proto image filters did.
func boolFilter[T any](paramName, description string, getField func(T) bool) listFilterParam[T] {
	return listFilterParam[T]{
		paramName:   paramName,
		description: description,
		matchFunc: func(items []T, value string) []T {
			want := strings.EqualFold(value, boolTrue)

			filtered := make([]T, 0, len(items))
			for _, item := range items {
				if getField(item) == want {
					filtered = append(filtered, item)
				}
			}

			return filtered
		},
	}
}

// protoListFilterOptions appends a WithString option for each filter param to
// opts. The three proto-list factories share this so their filter inputs stay
// byte-identical.
func protoListFilterOptions[T proto.Message](opts []mcp.ToolOption, filterParams []listFilterParam[T]) []mcp.ToolOption {
	for _, fp := range filterParams {
		opts = append(opts, mcp.WithString(fp.paramName, mcp.Description(fp.description)))
	}

	return opts
}

// finishProtoList applies the filter params to the fetched items, then
// assembles and marshals the family *ListResponse. It is the shared tail of all
// three proto-list factories (filter pipeline, count clamp, filter echo,
// MarshalProtoToolResponse), so each factory body stays short and distinct (and
// under the dupl linter's threshold). assemble builds the family list-response
// message from the filtered items, the count, and the optional filter echo
// (joined with the same ", " separator the Python side uses).
func finishProtoList[T, R proto.Message](
	request *mcp.CallToolRequest,
	items []T,
	filterParams []listFilterParam[T],
	assemble func(items []T, count int32, filter *string) R,
) (*mcp.CallToolResult, error) {
	var appliedFilters []string

	for _, fp := range filterParams {
		if value := request.GetString(fp.paramName, ""); value != "" {
			items = fp.matchFunc(items, value)
			appliedFilters = append(appliedFilters, fp.paramName+"="+value)
		}
	}

	var count int32
	if n := len(items); n <= math.MaxInt32 {
		count = int32(n)
	}

	var filter *string

	if len(appliedFilters) > 0 {
		joined := strings.Join(appliedFilters, ", ")
		filter = &joined
	}

	return MarshalProtoToolResponse(assemble(items, count, filter))
}

// newProtoListTool is the proto analog of newListTool. It builds the MCP tool
// (environment plus the filter params) and a handler that fetches the proto
// elements, applies the filters, and serializes a *ListResponse message via
// MarshalProtoToolResponse so the output matches the Python serializer
// element-for-element. assemble builds the family's list-response message from
// the filtered items, the count, and the optional filter echo (joined with the
// same ", " separator the Python side uses). Filter params reuse fieldFilter and
// containsFilter, the same constructors newListTool takes.
func newProtoListTool[T, R proto.Message](
	cfg *config.Config,
	toolName, description string,
	apiCall func(ctx context.Context, client *linode.Client) ([]T, error),
	filterParams []listFilterParam[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	options := protoListFilterOptions([]mcp.ToolOption{
		mcp.WithDescription(description),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
	}, filterParams)

	tool := mcp.NewTool(toolName, options...)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := prepareClient(&request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		items, err := apiCall(ctx, client)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve items: %v", err)), nil
		}

		return finishProtoList(&request, items, filterParams, assemble)
	}

	return tool, handler
}

// newProtoListToolRawSchema pairs the filtered list handler newProtoListTool
// builds with a raw-schema tool loaded from the generated input contract,
// discarding the schema newProtoListTool would otherwise synthesize. The
// filtered VPC and firewall list factories are structurally identical once both
// advertise a generated schema, so routing them through one builder keeps them
// below the dupl linter's threshold.
func newProtoListToolRawSchema[T, R proto.Message](
	cfg *config.Config,
	toolName, description, schemaName string,
	apiCall func(ctx context.Context, client *linode.Client) ([]T, error),
	filterParams []listFilterParam[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListTool(cfg, toolName, description, apiCall, filterParams, assemble)
	tool := mcp.NewToolWithRawSchema(toolName, description, toolschemas.Schema(schemaName))

	return tool, handler
}

// protoListPageReader validates and returns the page/page_size pair from the
// request. Each paginated family passes the same reader its non-proto handler
// already used, so the page/page_size validation behavior (and error messages)
// is unchanged. A non-empty string is a validation message; the caller returns
// it as the tool error.
type protoListPageReader func(request *mcp.CallToolRequest) (page, pageSize int, validationMessage string)

// newProtoListToolPaginated is newProtoListTool for families whose input schema
// carries page/page_size (WithNumber). It adds those two params with the exact
// names and descriptions the family's existing factory used (so the input schema
// stays byte-identical and tool-parity holds), reads them through readPage (the
// same reader the non-proto handler used, preserving validation and error text),
// and threads the validated pair into apiCall. The fetch+filter+serialize tail
// is shared with the other proto-list factories via finishProtoList.
func newProtoListToolPaginated[T, R proto.Message](
	cfg *config.Config,
	toolName, description, pageDesc, pageSizeDesc string,
	apiCall func(ctx context.Context, client *linode.Client, page, pageSize int) ([]T, error),
	readPage protoListPageReader,
	filterParams []listFilterParam[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	options := protoListFilterOptions([]mcp.ToolOption{
		mcp.WithDescription(description),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramPage, mcp.Description(pageDesc)),
		mcp.WithNumber(paramPageSize, mcp.Description(pageSizeDesc)),
	}, filterParams)

	tool := mcp.NewTool(toolName, options...)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page, pageSize, validationMessage := readPage(&request)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		client, err := prepareClient(&request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		items, err := apiCall(ctx, client, page, pageSize)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve items: %v", err)), nil
		}

		return finishProtoList(&request, items, filterParams, assemble)
	}

	return tool, handler
}

// newProtoListToolPaginatedRawSchema is newProtoListToolPaginated's raw-schema
// counterpart: it keeps the paginated fetch+filter handler but advertises the
// generated input contract instead of the synthesized schema, mirroring how
// newProtoListToolRawSchema relates to newProtoListTool.
func newProtoListToolPaginatedRawSchema[T, R proto.Message](
	cfg *config.Config,
	toolName, description, schemaName, pageDesc, pageSizeDesc string,
	apiCall func(ctx context.Context, client *linode.Client, page, pageSize int) ([]T, error),
	readPage protoListPageReader,
	filterParams []listFilterParam[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListToolPaginated(cfg, toolName, description, pageDesc, pageSizeDesc, apiCall, readPage, filterParams, assemble)
	tool := mcp.NewToolWithRawSchema(toolName, description, toolschemas.Schema(schemaName))

	return tool, handler
}

// protoListPathID describes the Required path-id param of a sub-resource list
// (e.g. vpc_id for vpc_subnet_list). option carries the exact schema option the
// family's existing factory used (so the input schema is byte-identical), and
// parse validates the raw argument the same way the existing handler did,
// returning the same error message. Keeping both as the family's own values
// preserves whether the id is a string or a number param.
type protoListPathID struct {
	option mcp.ToolOption
	parse  func(request *mcp.CallToolRequest) (int, string)
}

// newProtoListToolSubresource is newProtoListTool for sub-resource lists keyed by
// a Required path id (e.g. /vpcs/{vpc_id}/subnets). It adds the path-id param via
// pathID.option (preserving the family's exact schema and string-vs-number
// choice), validates it via pathID.parse (preserving the family's error text),
// and threads the validated id into apiCall. The fetch+filter+serialize tail is
// shared with the other proto-list factories via finishProtoList.
func newProtoListToolSubresource[T, R proto.Message](
	cfg *config.Config,
	toolName, description string,
	pathID protoListPathID,
	apiCall func(ctx context.Context, client *linode.Client, pathID int) ([]T, error),
	filterParams []listFilterParam[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return buildProtoSubresourceList(cfg, toolName, description, pathID.option, pathID.parse, apiCall, filterParams, assemble)
}

// newProtoListToolSubresourceRawSchema is newProtoListToolSubresource's
// raw-schema counterpart: it keeps the sub-resource fetch+filter handler but
// advertises the generated input contract instead of the synthesized schema,
// mirroring how newProtoListToolRawSchema relates to newProtoListTool.
func newProtoListToolSubresourceRawSchema[T, R proto.Message](
	cfg *config.Config,
	toolName, description, schemaName string,
	pathID protoListPathID,
	apiCall func(ctx context.Context, client *linode.Client, pathID int) ([]T, error),
	filterParams []listFilterParam[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListToolSubresource(cfg, toolName, description, pathID, apiCall, filterParams, assemble)
	tool := mcp.NewToolWithRawSchema(toolName, description, toolschemas.Schema(schemaName))

	return tool, handler
}

// newProtoListToolSubresourcePaginated is the combination of
// newProtoListToolSubresource and newProtoListToolPaginated: a sub-resource list
// keyed by a Required path id (e.g. /linode/instances/{linode_id}/configs) whose
// input schema also carries page/page_size. It adds the path-id param via
// pathID.option (preserving the family's exact schema and string-vs-number
// choice) and the page/page_size WithNumber params with the family's exact
// descriptions, validates the id via pathID.parse and the pagination via readPage
// (both preserving the family's error text), and threads the validated id, page,
// and pageSize into apiCall. The caller's path-aware client method formats the id
// into the endpoint exactly like the existing httpListX before adding pagination.
// The fetch+filter+serialize tail is shared with the other proto-list factories
// via finishProtoList.
func newProtoListToolSubresourcePaginated[T, R proto.Message](
	cfg *config.Config,
	toolName, description, pageDesc, pageSizeDesc string,
	pathID protoListPathID,
	readPage protoListPageReader,
	apiCall func(ctx context.Context, client *linode.Client, pathID, page, pageSize int) ([]T, error),
	filterParams []listFilterParam[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return buildProtoSubresourceListPaginated(cfg, toolName, description, pageDesc, pageSizeDesc,
		pathID.option, pathID.parse, readPage, apiCall, filterParams, assemble)
}

// newProtoListToolSubresourcePaginatedRawSchema is
// newProtoListToolSubresourcePaginated's raw-schema counterpart: it keeps the
// sub-resource paginated fetch+filter handler but advertises the generated input
// contract instead of the synthesized schema, mirroring how
// newProtoListToolRawSchema relates to newProtoListTool.
func newProtoListToolSubresourcePaginatedRawSchema[T, R proto.Message](
	cfg *config.Config,
	toolName, description, schemaName, pageDesc, pageSizeDesc string,
	pathID protoListPathID,
	readPage protoListPageReader,
	apiCall func(ctx context.Context, client *linode.Client, pathID, page, pageSize int) ([]T, error),
	filterParams []listFilterParam[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListToolSubresourcePaginated(cfg, toolName, description, pageDesc, pageSizeDesc, pathID, readPage, apiCall, filterParams, assemble)
	tool := mcp.NewToolWithRawSchema(toolName, description, toolschemas.Schema(schemaName))

	return tool, handler
}

// buildProtoSubresourceListPaginated is the shared core of the sub-resource
// paginated proto-list factories. It is generic over the path-id type P (int or
// string) so both newProtoListToolSubresourcePaginated and
// newProtoListToolSubresourceStringPaginated reuse one body: add environment +
// the path-id option + page/page_size + filter params, validate the id via parse
// and the pagination via readPage, fetch via apiCall, and serialize through
// finishProtoList.
func buildProtoSubresourceListPaginated[P comparable, T, R proto.Message](
	cfg *config.Config,
	toolName, description, pageDesc, pageSizeDesc string,
	pathOption mcp.ToolOption,
	parse func(request *mcp.CallToolRequest) (P, string),
	readPage protoListPageReader,
	apiCall func(ctx context.Context, client *linode.Client, pathID P, page, pageSize int) ([]T, error),
	filterParams []listFilterParam[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	options := protoListFilterOptions([]mcp.ToolOption{
		mcp.WithDescription(description),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		pathOption,
		mcp.WithNumber(paramPage, mcp.Description(pageDesc)),
		mcp.WithNumber(paramPageSize, mcp.Description(pageSizeDesc)),
	}, filterParams)

	tool := mcp.NewTool(toolName, options...)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, validationMessage := parse(&request)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		page, pageSize, validationMessage := readPage(&request)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		client, err := prepareClient(&request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		items, err := apiCall(ctx, client, id, page, pageSize)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve items: %v", err)), nil
		}

		return finishProtoList(&request, items, filterParams, assemble)
	}

	return tool, handler
}

// protoListPathIDString describes a Required string path-id of a sub-resource
// list whose id is genuinely non-numeric (e.g. tier for lke_tier_version_list,
// which is "standard" or "enterprise"). option carries the family's exact schema
// option and parse validates the raw argument the same way the existing handler
// did, returning the validated string and any error message. It is the string
// analog of protoListPathID, used when the path-id cannot be an int.
type protoListPathIDString struct {
	option mcp.ToolOption
	parse  func(request *mcp.CallToolRequest) (string, string)
}

// newProtoListToolSubresourceString is newProtoListToolSubresource for
// sub-resource lists keyed by a Required string path-id that is not numeric
// (e.g. /lke/tiers/{tier}/versions). It adapts the string path-id onto the shared
// buildProtoSubresourceList core so the schema, validation, and fetch+serialize
// tail all match the int variant; only the path-id type differs.
func newProtoListToolSubresourceString[T, R proto.Message](
	cfg *config.Config,
	toolName, description string,
	pathID protoListPathIDString,
	apiCall func(ctx context.Context, client *linode.Client, pathID string) ([]T, error),
	filterParams []listFilterParam[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return buildProtoSubresourceList(cfg, toolName, description, pathID.option, pathID.parse, apiCall, filterParams, assemble)
}

// newProtoListToolSubresourceStringRawSchema is
// newProtoListToolSubresourceString's raw-schema counterpart: it keeps the
// string-keyed sub-resource fetch+filter handler but advertises the generated
// input contract instead of the synthesized schema, mirroring how
// newProtoListToolSubresourceRawSchema relates to newProtoListToolSubresource.
func newProtoListToolSubresourceStringRawSchema[T, R proto.Message](
	cfg *config.Config,
	toolName, description, schemaName string,
	pathID protoListPathIDString,
	apiCall func(ctx context.Context, client *linode.Client, pathID string) ([]T, error),
	filterParams []listFilterParam[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListToolSubresourceString(cfg, toolName, description, pathID, apiCall, filterParams, assemble)
	tool := mcp.NewToolWithRawSchema(toolName, description, toolschemas.Schema(schemaName))

	return tool, handler
}

// newProtoListToolSubresourceStringPaginated is
// newProtoListToolSubresourcePaginated for sub-resource lists keyed by a Required
// string path-id that is not numeric (e.g. /images/{image_id}/sharegroups, where
// image_id is a slug like private/12345) whose input schema also carries
// page/page_size. It adapts the string path-id onto the shared
// buildProtoSubresourceListPaginated core so the schema, validation, pagination,
// and fetch+serialize tail all match the int variant; only the path-id type
// differs.
func newProtoListToolSubresourceStringPaginated[T, R proto.Message](
	cfg *config.Config,
	toolName, description, pageDesc, pageSizeDesc string,
	pathID protoListPathIDString,
	readPage protoListPageReader,
	apiCall func(ctx context.Context, client *linode.Client, pathID string, page, pageSize int) ([]T, error),
	filterParams []listFilterParam[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return buildProtoSubresourceListPaginated(cfg, toolName, description, pageDesc, pageSizeDesc,
		pathID.option, pathID.parse, readPage, apiCall, filterParams, assemble)
}

// newProtoListToolSubresourceStringPaginatedRawSchema is
// newProtoListToolSubresourceStringPaginated's raw-schema counterpart: it keeps
// the string-keyed paginated fetch+filter handler but advertises the generated
// input contract instead of the synthesized schema, mirroring how
// newProtoListToolRawSchema relates to newProtoListTool.
func newProtoListToolSubresourceStringPaginatedRawSchema[T, R proto.Message](
	cfg *config.Config,
	toolName, description, schemaName, pageDesc, pageSizeDesc string,
	pathID protoListPathIDString,
	readPage protoListPageReader,
	apiCall func(ctx context.Context, client *linode.Client, pathID string, page, pageSize int) ([]T, error),
	filterParams []listFilterParam[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListToolSubresourceStringPaginated(cfg, toolName, description, pageDesc, pageSizeDesc, pathID, readPage, apiCall, filterParams, assemble)
	tool := mcp.NewToolWithRawSchema(toolName, description, toolschemas.Schema(schemaName))

	return tool, handler
}

// buildProtoSubresourceList is the shared core of the sub-resource (non-paginated)
// proto-list factories. It is generic over the path-id type P (int or string) so
// both newProtoListToolSubresource and newProtoListToolSubresourceString reuse one
// body: add environment + the path-id option + filter params, validate the id via
// parse, fetch via apiCall, and serialize through finishProtoList.
func buildProtoSubresourceList[P comparable, T, R proto.Message](
	cfg *config.Config,
	toolName, description string,
	pathOption mcp.ToolOption,
	parse func(request *mcp.CallToolRequest) (P, string),
	apiCall func(ctx context.Context, client *linode.Client, pathID P) ([]T, error),
	filterParams []listFilterParam[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	options := protoListFilterOptions([]mcp.ToolOption{
		mcp.WithDescription(description),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		pathOption,
	}, filterParams)

	tool := mcp.NewTool(toolName, options...)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, validationMessage := parse(&request)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		client, err := prepareClient(&request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		items, err := apiCall(ctx, client, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve items: %v", err)), nil
		}

		return finishProtoList(&request, items, filterParams, assemble)
	}

	return tool, handler
}

// newProtoListToolSubresource2 is newProtoListToolSubresource for sub-resource
// lists keyed by TWO Required path ids (e.g.
// /linode/instances/{linode_id}/configs/{config_id}/interfaces). It adds both
// path-id params via parent.option and child.option (each preserving its
// family's exact schema and string-vs-number choice), validates the parent then
// the child via their parse funcs (preserving each family's error text), and
// threads both validated ids into apiCall. The caller's path-aware client method
// formats both ids into the endpoint exactly like the existing httpListX. The
// fetch+filter+serialize tail is shared with the other proto-list factories via
// finishProtoList.
func newProtoListToolSubresource2[T, R proto.Message](
	cfg *config.Config,
	toolName, description string,
	parent, child protoListPathID,
	apiCall func(ctx context.Context, client *linode.Client, parentID, childID int) ([]T, error),
	filterParams []listFilterParam[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	options := protoListFilterOptions([]mcp.ToolOption{
		mcp.WithDescription(description),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		parent.option,
		child.option,
	}, filterParams)

	tool := mcp.NewTool(toolName, options...)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parentID, childID, validationMessage := parseProtoListPathIDs2(&request, parent, child)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		client, err := prepareClient(&request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		items, err := apiCall(ctx, client, parentID, childID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve items: %v", err)), nil
		}

		return finishProtoList(&request, items, filterParams, assemble)
	}

	return tool, handler
}

// newProtoListToolSubresource2Paginated is newProtoListToolSubresource2 for
// two-path-id sub-resource lists whose input schema also carries page/page_size
// (e.g. /nodebalancers/{nodebalancer_id}/configs/{config_id}/nodes). It adds both
// path-id params plus the page/page_size WithNumber params with the family's
// exact descriptions, validates both ids via their parse funcs and the
// pagination via readPage (all preserving the family's error text), and threads
// the parent id, child id, page, and pageSize into apiCall. The fetch+filter+
// serialize tail is shared with the other proto-list factories via
// finishProtoList.
func newProtoListToolSubresource2Paginated[T, R proto.Message](
	cfg *config.Config,
	toolName, description, pageDesc, pageSizeDesc string,
	parent, child protoListPathID,
	readPage protoListPageReader,
	apiCall func(ctx context.Context, client *linode.Client, parentID, childID, page, pageSize int) ([]T, error),
	filterParams []listFilterParam[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	options := protoListFilterOptions([]mcp.ToolOption{
		mcp.WithDescription(description),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		parent.option,
		child.option,
		mcp.WithNumber(paramPage, mcp.Description(pageDesc)),
		mcp.WithNumber(paramPageSize, mcp.Description(pageSizeDesc)),
	}, filterParams)

	tool := mcp.NewTool(toolName, options...)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parentID, childID, validationMessage := parseProtoListPathIDs2(&request, parent, child)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		page, pageSize, validationMessage := readPage(&request)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		client, err := prepareClient(&request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		items, err := apiCall(ctx, client, parentID, childID, page, pageSize)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve items: %v", err)), nil
		}

		return finishProtoList(&request, items, filterParams, assemble)
	}

	return tool, handler
}

// newProtoListToolSubresource2PaginatedRawSchema is
// newProtoListToolSubresource2Paginated's raw-schema counterpart: it keeps the
// two-path-id paginated fetch+filter handler but advertises the generated input
// contract instead of the synthesized schema, mirroring how
// newProtoListToolRawSchema relates to newProtoListTool.
func newProtoListToolSubresource2PaginatedRawSchema[T, R proto.Message](
	cfg *config.Config,
	toolName, description, schemaName, pageDesc, pageSizeDesc string,
	parent, child protoListPathID,
	readPage protoListPageReader,
	apiCall func(ctx context.Context, client *linode.Client, parentID, childID, page, pageSize int) ([]T, error),
	filterParams []listFilterParam[T],
	assemble func(items []T, count int32, filter *string) R,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListToolSubresource2Paginated(cfg, toolName, description, pageDesc, pageSizeDesc, parent, child, readPage, apiCall, filterParams, assemble)
	tool := mcp.NewToolWithRawSchema(toolName, description, toolschemas.Schema(schemaName))

	return tool, handler
}

// parseProtoListPathIDs2 validates the parent then the child path-id, returning
// both validated ids or the first non-empty validation message. Both two-path-id
// factories share it so the parent-before-child validation order (and each
// family's error text) is identical across the paginated and non-paginated
// variants.
func parseProtoListPathIDs2(request *mcp.CallToolRequest, parent, child protoListPathID) (int, int, string) {
	parentID, validationMessage := parent.parse(request)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	childID, validationMessage := child.parse(request)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return parentID, childID, ""
}

// objectSliceFromToolArg normalizes an array-of-objects tool argument that may
// arrive as a native []any (the schema form) or as a JSON-encoded string (legacy
// form from a non-compliant client), decoding it into []T. An absent or empty
// value yields a nil slice; a malformed value returns a validation message that
// names the argument.
func objectSliceFromToolArg[T any](raw any, name string) ([]T, string) {
	var encoded []byte

	switch value := raw.(type) {
	case nil:
		return nil, ""
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, ""
		}

		encoded = []byte(trimmed)
	case []any:
		marshaled, err := json.Marshal(value)
		if err != nil {
			return nil, name + " must be an array of objects"
		}

		encoded = marshaled
	default:
		return nil, name + " must be an array of objects"
	}

	var result []T
	if err := json.Unmarshal(encoded, &result); err != nil {
		return nil, name + " must be an array of objects"
	}

	return result, ""
}

// objectJSONFromToolArg returns the JSON text of a single-object tool argument
// that may arrive as a native map (the schema form) or a JSON-encoded object
// string (legacy form), for callers that then decode strictly. An absent value
// yields ("", ""); a non-object value returns a validation message naming the
// argument.
func objectJSONFromToolArg(raw any, name string) (string, string) {
	switch value := raw.(type) {
	case nil:
		return "", ""
	case string:
		return strings.TrimSpace(value), ""
	case map[string]any:
		encoded, err := json.Marshal(value)
		if err != nil {
			return "", name + " must be an object"
		}

		return string(encoded), ""
	default:
		return "", name + " must be an object"
	}
}
