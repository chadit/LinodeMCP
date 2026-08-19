package tools

import (
	"encoding/json"
	"math"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// Common MCP tool parameter names and descriptions used across all tools.
const (
	paramEnvironment = "environment"
	paramPage        = "page"
	paramPageSize    = "page_size"
	paramConfirm     = "confirm"
	paramDryRun      = "dry_run"
	// Drive the bypass-dry-run gate on CapDestroy tools (see destroy.go): the
	// model asserts confirmed_dry_run after a dry-run, or sets
	// confirm_bypass_dry_run to skip the preview.
	paramConfirmedDryRun     = "confirmed_dry_run"
	paramConfirmBypassDryRun = "confirm_bypass_dry_run"

	// paramYolo bypasses preview and confirm, honored by the server middleware
	// only when the active profile allows yolo.
	paramYolo = "yolo"

	// Drive the two-stage plan/apply flow on opted-in CapDestroy tools (see
	// twostage_destroy.go): mode:"plan" returns a plan_id and a state hash,
	// mode:"apply" with that plan_id re-checks drift and executes.
	paramMode   = "mode"
	paramPlanID = "plan_id"
)

// liveConfigSource is the optional hot-reload provider. When set (by main.go
// via SetLiveConfigSource), prepareClient reads through it on each request so
// reloaded resilience/environment values apply to new API calls; when unset it
// falls back to the cfg captured at tool-registration time. The nolint below
// has to stay inline: newer golangci-lint releases only associate it with the
// var from the declaration line.
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

// prepareClient extracts the environment parameter, validates the config, and
// returns a ready-to-use API client. A registered live config source (see
// SetLiveConfigSource) takes effect on the very next tool call.
func prepareClient(request *mcp.CallToolRequest, cfg *config.Config) (*linode.Client, error) {
	cfg = resolveConfig(cfg)
	environment := request.GetString(paramEnvironment, "")

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return nil, err
	}

	if err := linodeConfigComplete(selectedEnv); err != nil {
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

// FilterByMember returns items one of whose getField entries equals wanted
// (case-insensitive). It is the comparison a list-valued field answers a single
// asked-for value with, which equality against the whole list cannot express.
func FilterByMember[T any](items []T, wanted string, getField func(T) []string) []T {
	filtered := make([]T, 0, len(items))

	for i := range items {
		if slices.ContainsFunc(getField(items[i]), func(entry string) bool {
			return strings.EqualFold(entry, wanted)
		}) {
			filtered = append(filtered, items[i])
		}
	}

	return filtered
}

// listFilterParam combines a tool parameter definition with its filter logic for list tools.
type listFilterParam[T any] struct {
	matchFunc   func(items []T, value string) []T
	paramName   string
	description string
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
	names := make([]string, 0, len(filterParams))

	for _, fp := range filterParams {
		if value := request.GetString(fp.paramName, ""); value != "" {
			items = fp.matchFunc(items, value)
			names = append(names, fp.paramName)
		}
	}

	return MarshalProtoToolResponse(
		assemble(items, ClampListCount(items), echoedFilter(request, names)),
	)
}

// ClampListCount is a page's element count as the response declares it. A
// collection longer than an int32 reports zero rather than a wrapped negative.
//
// Exported for the generated handlers of the mutations whose route answers with
// a page, which assemble that envelope themselves rather than through the list
// driver below.
func ClampListCount[T any](items []T) int32 {
	var count int32
	if n := len(items); n <= math.MaxInt32 {
		count = int32(n)
	}

	return count
}

// echoedFilter words the filter a page reports it was narrowed by, from the
// named arguments the caller actually sent. An argument left out is skipped, and
// a page narrowed by nothing echoes nothing at all.
//
// The marker shape reports the arguments its route filtered on and the page
// envelope reports the ones applied here, but a caller reads one sentence either
// way, so both languages join it the same: "name=value", ", " between.
func echoedFilter(request *mcp.CallToolRequest, names []string) *string {
	applied := make([]string, 0, len(names))

	for _, name := range names {
		if value := request.GetString(name, ""); value != "" {
			applied = append(applied, name+"="+value)
		}
	}

	if len(applied) == 0 {
		return nil
	}

	joined := strings.Join(applied, ", ")

	return &joined
}

// objectJSONFromToolArg returns the JSON text of a single-object tool argument
// that may arrive as a native map (the schema form) or a JSON-encoded object
// Standard Linode collection pagination bounds. The API applies these to every
// paginated collection, so a family with no bounds of its own uses them rather
// than declaring a private copy.
const (
	standardPageSizeMin = 25
	standardPageSizeMax = 500
)

// standardPaginationFromTool reads page/page_size under the standard Linode
// bounds. Families that predate this helper keep their own reader because their
// validation messages are pinned by the message-parity gate; new paginated
// families use this one instead of copying the pair again.
func standardPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, paramPage, 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, paramPageSize, standardPageSizeMin, standardPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

// ObjectMapArgument returns an object tool argument as a map holding exactly
// the keys the caller sent, so nothing they omitted reaches the wire. It
// accepts the native map form the schema produces and the JSON-string form some
// clients still send. An absent or blank value yields (nil, ""); anything that
// is not an object returns a validation message naming the argument.
func ObjectMapArgument(raw any, name string) (map[string]any, string) {
	switch value := raw.(type) {
	case nil:
		return nil, ""
	case map[string]any:
		return value, ""
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, ""
		}

		var decoded map[string]any
		if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
			return nil, name + " must be an object"
		}

		return decoded, ""
	default:
		return nil, name + " must be an object"
	}
}

// IDToInt32 narrows a Linode ID to the proto int32 field, returning 0 for
// the rare out-of-range value so the bounded conversion never overflows.
//
// Exported because the generated destroy tier fills its id echo through it: a
// delete answers with the id it was addressed by, and that id arrives as the
// platform int a tool argument reads into.
func IDToInt32(id int) int32 {
	if id < math.MinInt32 || id > math.MaxInt32 {
		return 0
	}

	return int32(id)
}
