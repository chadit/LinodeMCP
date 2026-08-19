// Package toolhooks holds the per-tool logic the proto contract cannot express,
// such as a check whose wording predates the contract, a body that flattens one
// argument into several, or the local state a meta tool answers from. A tool
// declares the KINDS it needs through the
// `tool_hooks` option on its *Input message, and the generated factory calls a
// function named from the tool and the kind: a declared kind with no
// implementation here fails the build, and the reverse fails
// TestContractNamesEveryImplementedHook.
//
// Every kind is named as a type below whether or not a caller declares a
// variable of it, so the signatures read as one vocabulary rather than as
// whichever ones a test happened to need a map value type for. Two conventions
// span the whole package: a normalize rewrites in place, because Python's twin
// takes the same dict its handler goes on to read and a returned map would
// leave each language deciding what happens to the one it was given; and a walk
// is best-effort, because a partial dependency picture is worth more to a caller
// deciding whether to proceed than no preview at all.
package toolhooks

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/goname"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// Normalize rewrites a tool's arguments in place before anything reads them,
// which is where a family's own reading of a value lives. It runs ahead of
// Validate so a check sees the value the request will actually carry.
//
// Rewriting in place rather than answering a new map is what keeps the two
// languages the same: Python's hook takes the same dict its handler goes on to
// read, and a returned map would leave each language deciding what happens to
// the one it was given.
type Normalize func(request *mcp.CallToolRequest)

// Validate checks a tool's arguments before anything else runs, returning the
// message the tool reports or "" to continue. Generated handlers wrap it in
// mcp.NewToolResultError, so a bad argument reads as a result the model can
// correct rather than a transport failure.
type Validate func(request *mcp.CallToolRequest) string

// Preview answers a mutating tool's dry run. It receives the call the tool would
// have made, resolved from the contract, and the body the generated builder
// assembled, so a hook never re-spells a route or rebuilds a body: what it owns
// is the prose, and for an update the fetch its prose is diffed against.
type Preview func(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error)

// FetchState reads the resource a destroy would remove. Its answer is what a
// preview reports as current_state and what a two-stage plan hashes for drift,
// so the SHAPE it returns is part of the tool's contract with a client: the
// typed client method a family already has is what every hook here calls,
// rather than a generic decode that would report a different set of fields.
//
// It takes the id the generated handler validated, which is the signature
// tools.DestructiveActionByID declares, so the emitted call site assigns the
// hook straight across and a wrong shape is a compile error.
type FetchState func(ctx context.Context, client *linode.Client, id int) (any, error)

// fetchedState answers a state read for the destroy flow, which every
// FetchState hook returns through.
//
// It hands back the client's error untouched: the destroy flow reports it under
// "Failed to fetch state for dry-run", and Python's driver prefixes that same
// sentence, so a second prefix here would put the two languages a phrase apart.
//
// The nil is the reason this is a function rather than a bare return. A typed
// pointer returned into an `any` is not nil even when the pointer is, so a
// failed read handed straight back would reach the preview as a state that
// tests non-nil and holds nothing.
func fetchedState(state any, err error) (any, error) {
	if err != nil {
		return nil, err
	}

	return state, nil
}

// DependencyWalk enriches a destroy's preview with what else the removal
// touches: the resources that cascade, the addresses released, the billing that
// stops. It receives the state FetchState already read, so a walk never re-GETs
// the resource it is describing.
//
// A walk is best-effort by convention: a sub-fetch that fails becomes a warning
// on the preview rather than an error, because a partial dependency picture is
// worth more to a caller deciding whether to proceed than no preview at all.
type DependencyWalk func(
	ctx context.Context, client *linode.Client, id int, state any,
) (tools.DryRunDetails, error)

// Execute makes a mutating tool's live call in place of the routed JSON request
// the generated handler would send. It is the one hook that replaces the call
// rather than something around it, for the routes whose request is not a JSON
// body: the support-ticket attachment posts multipart/form-data assembled from a
// local file, and a generated handler would send the file's path as JSON.
//
// It takes the arguments the generated checks already accepted, the path values
// the derived call would have been given, and the body the shared builder
// assembled, so a hook re-reads no argument and rebuilds no body. The route
// belongs to the contract either way: the client resolves it from the tool name
// the hook's own name carries.
//
// It answers with an error alone because the tier it serves assembles its
// response from the call rather than decoding one, which is also why nothing
// declares this on a tier that decodes.
//
// The dry-run branch never reaches it. A preview makes no call, so it reports
// the request the way every other mutation's does.
type Execute func(
	ctx context.Context,
	client *linode.Client,
	request *mcp.CallToolRequest,
	pathValues []any,
	body any,
) error

// Answer produces a meta tool's whole result from local state: the audit
// stores on disk, the profile builder's in-memory registry. It is the one hook
// that takes no client, because the tool it serves reaches no Linode route.
//
// It answers with the result rather than a body to serialize, since nothing
// derived from the contract stands between it and the caller: a meta tool's
// answer is whatever local state says, and the emitter has no message to
// assemble around it. That is also why a tool declaring one declares no
// success_message: the two would be separate sources for one answer.
//
// It takes cfg because local state is configured (where the audit log lives,
// which profiles exist) even where no environment is selected.
type Answer func(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
) (*mcp.CallToolResult, error)

// answeredBy passes an answer hook's result through from the reader that
// assembled it.
//
// The error is wrapped with no prose of its own: whatever the reader reports is
// what the caller reads, and Python's twin adds nothing either, so a phrase
// here would put the two languages a sentence apart.
func answeredBy(result *mcp.CallToolResult, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return result, nil
}

// FunctionName is the function one tool's hook of a kind lives under here:
// linode_domain_record_get and validate give LinodeDomainRecordGetValidate.
//
// The rule is spelled once, in the package the name refers to. cmd/toolgen
// writes the call through it and this package's own test derives what it
// expects to find through it, so the emitted call and the check on it cannot
// come apart.
func FunctionName(tool, kind string) string {
	return goname.Exported(tool) + goname.Exported(kind)
}

// previewDetails runs a walk under the caller's context, so a preview nobody is
// waiting for stops rather than describing a resource into a closed connection.
// The walks themselves make no call, which is why the check is here rather than
// inside each of them.
func previewDetails(
	ctx context.Context, tool string, walk func() tools.DryRunDetails,
) (tools.DryRunDetails, error) {
	if err := ctx.Err(); err != nil {
		return tools.DryRunDetails{}, fmt.Errorf("%s side-effect walk canceled: %w", tool, err)
	}

	return walk(), nil
}

// wrapPreview names the tool a failed preview belongs to. A preview fails when
// the call it was asked to describe has no JSON form, which reads as a defect in
// the tool rather than in the resource, so the tool is what the report names.
func wrapPreview(tool string, result *mcp.CallToolResult, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return nil, fmt.Errorf("%s preview: %w", tool, err)
	}

	return result, nil
}

// statePreview answers the dry run of a tool whose prose is diffed against the
// resource as it stands: read it, walk it, report. Every update hook here has
// that same shape, and writing it per tool put the copies a rename apart.
//
// A nil body leaves the request echo out, which is what the families whose
// preview predates that echo still answer with.
func statePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	tool, method, path string,
	body any,
	fetch func(ctx context.Context, client *linode.Client) (any, error),
	walk func(state any) tools.DryRunDetails,
) (*mcp.CallToolResult, error) {
	result, err := tools.RunDryRunPreviewWithBodyDetailed(
		ctx, request, cfg, tool, method, path, body, fetch,
		func(ctx context.Context, _ *linode.Client, state any) (tools.DryRunDetails, error) {
			return previewDetails(ctx, tool, func() tools.DryRunDetails {
				return walk(state)
			})
		},
	)

	return wrapPreview(tool, result, err)
}
