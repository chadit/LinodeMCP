package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	previewToolName = "linode_instance_update"
	previewGetTool  = "linode_instance_get"

	// envNotFoundStagingText is the refusal a caller reads back when the
	// environment argument names one the config does not carry. The dry-run,
	// plan, and apply paths all share it, so the tests pin one spelling.
	envNotFoundStagingText = "environment not found in configuration: staging"
	dryRunFetchFailureText = "Failed to fetch state for dry-run: fetch failed"
	dryRunWalkFailureText  = "Failed to compute dry-run side effects: walk failed"

	keyCurrentState = "current_state"
	keySideEffects  = "side_effects"
	keyWarnings     = "warnings"

	// The object a caller sends where the environment name belongs. Named for
	// that role rather than borrowed from the managed-contact and profile-label
	// constants that happen to hold the same two strings.
	envObjectKey   = "name"
	envObjectValue = "prod"
)

// errWalkFailed stands in for a side-effect walk that could not finish.
var errWalkFailed = errors.New("walk failed")

// stateFetcher is the read a preview performs before it describes the call.
type stateFetcher = func(ctx context.Context, client *linode.Client) (any, error)

// fetchInstanceObject reads GET /linode/instances/123 through the prepared
// client, so the preview's state is observed on the wire rather than made up
// by the closure.
func fetchInstanceObject(ctx context.Context, client *linode.Client) (any, error) {
	var state map[string]any
	if err := client.CallRouteJSON(ctx, previewGetTool, []any{123}, &state); err != nil {
		return nil, fmt.Errorf("fetch instance: %w", err)
	}

	return state, nil
}

// fetchInstanceList reads the same route but keeps whatever array the API
// answered, the shape a collection preview reports as its state.
func fetchInstanceList(ctx context.Context, client *linode.Client) (any, error) {
	var state []any
	if err := client.CallRouteJSON(ctx, previewGetTool, []any{123}, &state); err != nil {
		return nil, fmt.Errorf("fetch instance list: %w", err)
	}

	return state, nil
}

// failingFetch answers the fetch failure a read sibling reports when the API
// refuses the GET.
func failingFetch(context.Context, *linode.Client) (any, error) {
	return nil, errStateFetch
}

// previewEntryPoint is one of the shared dry-run branches a generated write
// handler delegates to. The refusal tables run every branch through the same
// inputs so the sentences cannot drift between them.
type previewEntryPoint struct {
	run  func(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config, fetch stateFetcher) (*mcp.CallToolResult, error)
	name string
}

func previewEntryPoints() []previewEntryPoint {
	body := map[string]any{keyLabel: testRenamedLabel}

	return []previewEntryPoint{
		{
			name: "RunDryRunPreview",
			run: func(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config, fetch stateFetcher) (*mcp.CallToolResult, error) {
				return tools.RunDryRunPreview(ctx, request, cfg, previewToolName, http.MethodPut, instancePath123, fetch)
			},
		},
		{
			name: "RunDryRunPreviewWithBody",
			run: func(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config, fetch stateFetcher) (*mcp.CallToolResult, error) {
				return tools.RunDryRunPreviewWithBody(ctx, request, cfg, previewToolName, http.MethodPut, instancePath123, body, fetch)
			},
		},
		{
			name: "RunDryRunPreviewWithBodyDetailed",
			run: func(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config, fetch stateFetcher) (*mcp.CallToolResult, error) {
				return tools.RunDryRunPreviewWithBodyDetailed(ctx, request, cfg, previewToolName, http.MethodPut, instancePath123, body, fetch, nil)
			},
		},
	}
}

// errorText returns the text of a result the tool refused with, and fails the
// caller when the result was not an error.
func errorText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	if !result.IsError {
		t.Fatalf("result.IsError = false, want true; text: %s", dryRunResultText(t, result))
	}

	return dryRunResultText(t, result)
}

// TestRunDryRunPreviewReportsANullStateWhenThereIsNothingToFetch covers a create
// preview: no resource exists yet, so the branch makes no API call at all and
// still reports the route, the environment, and an explicit null state.
func TestRunDryRunPreviewReportsANullStateWhenThereIsNothingToFetch(t *testing.T) {
	t.Parallel()

	cfg := dryRunNoCallServer(t)
	request := createRequestWithArgs(t, map[string]any{keyDryRun: true, keyEnvironment: envKeyDefault})

	result, err := tools.RunDryRunPreview(t.Context(), &request, cfg, "linode_instance_create", http.MethodPost, "/linode/instances", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("result.IsError = true, text: %s", dryRunResultText(t, result))
	}

	body := decodeBody(t, dryRunResultText(t, result))
	assertDryRunRequest(t, body, http.MethodPost, "/linode/instances")

	state, present := body[keyCurrentState]
	if !present || state != nil {
		t.Errorf("current_state = %v (present %v), want an explicit null", state, present)
	}

	if body[canRunKeyEnv] != envKeyDefault {
		t.Errorf("environment = %v, want %v", body[canRunKeyEnv], envKeyDefault)
	}

	if body[canRunKeyTool] != "linode_instance_create" {
		t.Errorf("tool = %v, want linode_instance_create", body[canRunKeyTool])
	}
}

// TestRunDryRunPreviewRefusesAnEnvironmentTheConfigDoesNotName covers the
// caller who asks for a preview under an environment the operator never
// configured: every shared branch refuses before it reads anything, on the arm
// that would have built a client and on the one that never does.
func TestRunDryRunPreviewRefusesAnEnvironmentTheConfigDoesNotName(t *testing.T) {
	t.Parallel()

	// The create arm builds no client, which is where a name the config never
	// carried used to reach a successful preview.
	arms := []struct {
		name     string
		fetching bool
	}{
		{name: "with a state fetch", fetching: true},
		{name: "with no state fetch", fetching: false},
	}

	for _, entry := range previewEntryPoints() {
		for _, arm := range arms {
			t.Run(entry.name+"/"+arm.name, func(t *testing.T) {
				t.Parallel()

				cfg := dryRunNoCallServer(t)
				request := createRequestWithArgs(t, map[string]any{keyDryRun: true, keyEnvironment: tcStaging})

				var fetch stateFetcher

				if arm.fetching {
					fetch = func(context.Context, *linode.Client) (any, error) {
						t.Error("fetch ran even though no client could be prepared")

						return nil, errStateFetch
					}
				}

				result, err := entry.run(t.Context(), &request, cfg, fetch)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if got := errorText(t, result); got != envNotFoundStagingText {
					t.Errorf("text = %q, want %q", got, envNotFoundStagingText)
				}
			})
		}
	}
}

// TestRunDryRunPreviewRefusesAnEnvironmentThatIsNotAName covers a create
// preview, the one path with no state fetch. Nothing there builds a client, so
// prepareClient's own refusal never runs, and reading the argument as text
// answered a successful preview with the environment dropped. Python's
// _derived_preview refuses the same value under the same sentence.
func TestRunDryRunPreviewRefusesAnEnvironmentThatIsNotAName(t *testing.T) {
	t.Parallel()

	const want = `environment not found in configuration: {"name":"prod"}`

	envObject := map[string]any{envObjectKey: envObjectValue}

	for _, entry := range previewEntryPoints() {
		t.Run(entry.name, func(t *testing.T) {
			t.Parallel()

			cfg := dryRunNoCallServer(t)
			request := createRequestWithArgs(t, map[string]any{
				keyDryRun:      true,
				keyEnvironment: envObject,
			})

			result, err := entry.run(t.Context(), &request, cfg, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := errorText(t, result); got != want {
				t.Errorf("text = %q, want %q", got, want)
			}
		})
	}
}

// TestRunDryRunPreviewReportsTheStateFetchFailure covers the read sibling
// answering an error: every shared branch reports it under the same sentence
// rather than previewing against a state it never saw.
func TestRunDryRunPreviewReportsTheStateFetchFailure(t *testing.T) {
	t.Parallel()

	for _, entry := range previewEntryPoints() {
		t.Run(entry.name, func(t *testing.T) {
			t.Parallel()

			cfg := dryRunNoCallServer(t)
			request := createRequestWithArgs(t, map[string]any{keyDryRun: true})

			result, err := entry.run(t.Context(), &request, cfg, failingFetch)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := errorText(t, result); got != dryRunFetchFailureText {
				t.Errorf("text = %q, want %q", got, dryRunFetchFailureText)
			}
		})
	}
}

// TestRunDryRunPreviewCarriesANonObjectStateVerbatim covers a fetch whose body
// is a collection rather than one resource: the preview reports the array as
// the current state instead of refusing or flattening it.
func TestRunDryRunPreviewCarriesANonObjectStateVerbatim(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunGetStateServer(t, instancePath123, []any{
		map[string]any{keyStateID: 1},
		map[string]any{keyStateID: 2},
	})
	request := createRequestWithArgs(t, map[string]any{keyDryRun: true})

	result, err := tools.RunDryRunPreview(t.Context(), &request, cfg, previewToolName, http.MethodPut, instancePath123, fetchInstanceList)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("result.IsError = true, text: %s", dryRunResultText(t, result))
	}

	body := decodeBody(t, dryRunResultText(t, result))

	want := []any{
		map[string]any{keyStateID: float64(1)},
		map[string]any{keyStateID: float64(2)},
	}
	if !reflect.DeepEqual(body[keyCurrentState], want) {
		t.Errorf("current_state = %v, want %v", body[keyCurrentState], want)
	}

	if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
		t.Errorf("methods = %v, want only the GET", *methods)
	}
}

// TestRunDryRunPreviewDetailedAttachesTheWalkFindings covers the Phase 2 hook:
// the walk receives the fetched state and the same client the fetch used, and
// what it reports lands in the preview beside the state.
func TestRunDryRunPreviewDetailedAttachesTheWalkFindings(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunGetStateServer(t, instancePath123, map[string]any{
		keyStateID: 123, keyLabel: firewallDeviceLabelFixture,
	})
	request := createRequestWithArgs(t, map[string]any{keyDryRun: true})

	walk := func(_ context.Context, client *linode.Client, state any) (tools.DryRunDetails, error) {
		if client == nil {
			t.Error("walk received no client")
		}

		fetched, isObject := state.(map[string]any)
		if !isObject || fetched[keyLabel] != firewallDeviceLabelFixture {
			t.Errorf("walk received state %v, want the fetched instance", state)
		}

		return tools.DryRunDetails{
			SideEffects:  []string{"Reboots the instance"},
			Warnings:     []string{"Downtime expected"},
			BillingDelta: &tools.DryRunBillingDelta{MonthlyChangeUSD: "+10.00", Note: "larger plan"},
		}, nil
	}

	result, err := tools.RunDryRunPreviewDetailed(t.Context(), &request, cfg, previewToolName, http.MethodPut, instancePath123, fetchInstanceObject, walk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("result.IsError = true, text: %s", dryRunResultText(t, result))
	}

	body := decodeBody(t, dryRunResultText(t, result))

	if !reflect.DeepEqual(body[keySideEffects], []any{"Reboots the instance"}) {
		t.Errorf("side_effects = %v, want the walk's sentence", body[keySideEffects])
	}

	if !reflect.DeepEqual(body[keyWarnings], []any{"Downtime expected"}) {
		t.Errorf("warnings = %v, want the walk's warning", body[keyWarnings])
	}

	delta, isObject := body["billing_delta"].(map[string]any)
	if !isObject || delta["monthly_change_usd"] != "+10.00" || delta["note"] != "larger plan" {
		t.Errorf("billing_delta = %v, want the walk's estimate", body["billing_delta"])
	}

	if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
		t.Errorf("methods = %v, want only the GET", *methods)
	}
}

// TestRunDryRunPreviewDetailedReportsTheWalkFailure covers a walk that could
// not finish: the preview fails under its own sentence rather than shipping a
// partial side-effect list the operator would take as complete.
func TestRunDryRunPreviewDetailedReportsTheWalkFailure(t *testing.T) {
	t.Parallel()

	cfg, _ := dryRunGetStateServer(t, instancePath123, map[string]any{keyStateID: 123})
	request := createRequestWithArgs(t, map[string]any{keyDryRun: true})

	walk := func(context.Context, *linode.Client, any) (tools.DryRunDetails, error) {
		return tools.DryRunDetails{}, errWalkFailed
	}

	result, err := tools.RunDryRunPreviewDetailed(t.Context(), &request, cfg, previewToolName, http.MethodPut, instancePath123, fetchInstanceObject, walk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := errorText(t, result); got != dryRunWalkFailureText {
		t.Errorf("text = %q, want %q", got, dryRunWalkFailureText)
	}
}

// TestRunDryRunPreviewWithBodyReportsTheFetchedStateBesideTheBody covers an
// update preview: the current resource is read through the client and the
// sanitized request body rides along under would_execute.
func TestRunDryRunPreviewWithBodyReportsTheFetchedStateBesideTheBody(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunGetStateServer(t, instancePath123, map[string]any{
		keyStateID: 123, keyLabel: firewallDeviceLabelFixture,
	})
	request := createRequestWithArgs(t, map[string]any{keyDryRun: true})

	result, err := tools.RunDryRunPreviewWithBody(t.Context(), &request, cfg, previewToolName, http.MethodPut, instancePath123,
		map[string]any{keyLabel: testRenamedLabel}, fetchInstanceObject)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("result.IsError = true, text: %s", dryRunResultText(t, result))
	}

	body := decodeBody(t, dryRunResultText(t, result))
	assertDryRunRequest(t, body, http.MethodPut, instancePath123)

	would, _ := body["would_execute"].(map[string]any)

	sent, isObject := would["body"].(map[string]any)
	if !isObject || sent[keyLabel] != testRenamedLabel {
		t.Errorf("would_execute.body = %v, want the renamed label", would["body"])
	}

	state, isObject := body[keyCurrentState].(map[string]any)
	if !isObject || state[keyLabel] != firewallDeviceLabelFixture {
		t.Errorf("current_state = %v, want the fetched instance", body[keyCurrentState])
	}

	if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
		t.Errorf("methods = %v, want only the GET", *methods)
	}
}

// TestRunDryRunPreviewWithBodyDetailedWithoutAWalkMatchesTheBodyPreview pins the
// documented equivalence: a nil walk reproduces RunDryRunPreviewWithBody's
// answer byte for byte, so a generated handler may switch between the two
// without changing what a caller reads.
func TestRunDryRunPreviewWithBodyDetailedWithoutAWalkMatchesTheBodyPreview(t *testing.T) {
	t.Parallel()

	state := map[string]any{keyStateID: 123, keyLabel: firewallDeviceLabelFixture}
	body := map[string]any{keyLabel: testRenamedLabel}

	plainCfg, _ := dryRunGetStateServer(t, instancePath123, state)
	detailedCfg, _ := dryRunGetStateServer(t, instancePath123, state)
	request := createRequestWithArgs(t, map[string]any{keyDryRun: true, keyEnvironment: envKeyDefault})

	plain, err := tools.RunDryRunPreviewWithBody(t.Context(), &request, plainCfg, previewToolName, http.MethodPut, instancePath123, body, fetchInstanceObject)
	if err != nil {
		t.Fatalf("plain preview: %v", err)
	}

	detailed, err := tools.RunDryRunPreviewWithBodyDetailed(t.Context(), &request, detailedCfg, previewToolName, http.MethodPut, instancePath123, body, fetchInstanceObject, nil)
	if err != nil {
		t.Fatalf("detailed preview: %v", err)
	}

	if plain.IsError || detailed.IsError {
		t.Fatalf("IsError plain=%v detailed=%v, want neither", plain.IsError, detailed.IsError)
	}

	plainBody := decodeBody(t, dryRunResultText(t, plain))
	detailedBody := decodeBody(t, dryRunResultText(t, detailed))

	if !reflect.DeepEqual(plainBody, detailedBody) {
		t.Errorf("detailed preview = %v, want %v", detailedBody, plainBody)
	}

	if plainBody[canRunKeyEnv] != envKeyDefault {
		t.Errorf("environment = %v, want %v", plainBody[canRunKeyEnv], envKeyDefault)
	}
}

// TestBuildDryRunResponseDetailedRejectsADependencyIDItCannotEncode covers a
// walk reporting a dependency whose id the wire cannot carry: the preview fails
// rather than listing a dependency with no usable identifier.
func TestBuildDryRunResponseDetailedRejectsADependencyIDItCannotEncode(t *testing.T) {
	t.Parallel()

	result, err := tools.BuildDryRunResponseDetailed(
		canRunDestroyTool, "", http.MethodDelete, instancePath123,
		map[string]any{keyStateID: 123},
		&tools.DryRunDetails{Dependencies: []tools.DryRunDependency{
			{Kind: "volume", ID: json.Number("1e1000"), Action: tools.DependencyActionDetached},
		}},
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %+v, want nil", result)
	}
}
