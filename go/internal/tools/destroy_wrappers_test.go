package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	toolDomainRecordDelete = "linode_domain_record_delete"
	toolBucketDelete       = "linode_object_storage_bucket_delete"
	domainRecordPath       = "/domains/1/records/2"
	bucketPath             = "/object-storage/buckets/us-east/my-bucket"
	confirmRequiredText    = "confirm: true is required"

	dryRunDependencyFailureText = "Failed to compute dry-run dependencies: walk failed"
)

// recordingDeleteServer answers GET with state and records the path of every
// DELETE it receives, so a wrapper test can prove which route the removal hit.
func recordingDeleteServer(t *testing.T, state map[string]any) (*config.Config, *atomic.Pointer[string]) {
	t.Helper()

	deletedPath := &atomic.Pointer[string]{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			path := r.URL.Path
			deletedPath.Store(&path)
			w.WriteHeader(http.StatusOK)

			return
		}

		writeJSON(t, w, state)
	}))
	t.Cleanup(srv.Close)

	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}, deletedPath
}

// writeJSON answers one request with the JSON encoding of body.
func writeJSON(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Errorf("encode body: %v", err)
	}
}

// domainRecordAction is the two-ID removal a domain record delete would
// declare, with its read and delete both going through the client so the
// wrapper's parsed IDs are observed on the wire.
func domainRecordAction(walk func(ctx context.Context, client *linode.Client, outerID, innerID int, state any) (tools.DryRunDetails, error)) *tools.DestructiveActionByTwoIDs {
	return &tools.DestructiveActionByTwoIDs{
		ToolName:       toolDomainRecordDelete,
		OuterIDParam:   keyDomainID,
		InnerIDParam:   keyRecordID,
		Method:         http.MethodDelete,
		PathPattern:    "/domains/%d/records/%d",
		ConfirmMessage: confirmRequiredText,
		FetchState: func(ctx context.Context, client *linode.Client, outerID, innerID int) (any, error) {
			var state map[string]any
			if err := client.CallRouteJSON(ctx, "linode_domain_record_get", []any{outerID, innerID}, &state); err != nil {
				return nil, fmt.Errorf("fetch record: %w", err)
			}

			return state, nil
		},
		Execute: func(ctx context.Context, client *linode.Client, outerID, innerID int) error {
			if err := client.CallRoute(ctx, toolDomainRecordDelete, []any{outerID, innerID}); err != nil {
				return fmt.Errorf("delete record: %w", err)
			}

			return nil
		},
		SuccessProto: func(outerID, innerID int) proto.Message {
			return &linodev1.DomainRecordDeleteResponse{
				Message:  fmt.Sprintf("Record %d removed from domain %d", innerID, outerID),
				DomainId: tools.IDToInt32(outerID),
				RecordId: tools.IDToInt32(innerID),
			}
		},
		DependencyWalk: walk,
	}
}

// bucketAction is the (region, label) removal an Object Storage bucket delete
// would declare.
func bucketAction() *tools.DestructiveActionByRegionLabel {
	return &tools.DestructiveActionByRegionLabel{
		ToolName:       toolBucketDelete,
		Method:         http.MethodDelete,
		PathPattern:    "/object-storage/buckets/%s/%s",
		ConfirmMessage: confirmRequiredText,
		FetchState: func(ctx context.Context, client *linode.Client, region, label string) (any, error) {
			var state map[string]any
			if err := client.CallRouteJSON(ctx, "linode_object_storage_bucket_get", []any{region, label}, &state); err != nil {
				return nil, fmt.Errorf("fetch bucket: %w", err)
			}

			return state, nil
		},
		Execute: func(ctx context.Context, client *linode.Client, region, label string) error {
			if err := client.CallRoute(ctx, toolBucketDelete, []any{region, label}); err != nil {
				return fmt.Errorf("delete bucket: %w", err)
			}

			return nil
		},
		SuccessProto: func(region, label string) proto.Message {
			return &linodev1.ObjectStorageBucketDeleteResponse{
				Message: "Bucket " + label + " in " + region + " removed",
				Region:  region,
				Label:   label,
			}
		},
	}
}

// TestRunDestructiveActionByTwoIDsRequiresBothIDs covers the argument gate of
// the two-ID wrapper: whichever id is missing or zero is named in the refusal,
// and nothing reaches the API.
func TestRunDestructiveActionByTwoIDsRequiresBothIDs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args map[string]any
		name string
		want string
	}{
		{name: "missing domain_id", args: map[string]any{keyRecordID: float64(2)}, want: "domain_id is required"},
		{name: "zero domain_id", args: map[string]any{keyDomainID: float64(0), keyRecordID: float64(2)}, want: "domain_id is required"},
		{name: "missing record_id", args: map[string]any{keyDomainID: float64(1)}, want: "record_id is required"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := createRequestWithArgs(t, testCase.args)

			result, err := tools.RunDestructiveActionByTwoIDs(t.Context(), &request, dryRunNoCallServer(t), domainRecordAction(nil))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := errorText(t, result); got != testCase.want {
				t.Errorf("text = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestRunDestructiveActionByTwoIDsPreviewsWithTheWalkBoundToBothIDs covers the
// dry run of a two-ID removal: the state is read at the nested path, the walk
// receives both parsed ids plus that state, and the preview names the route
// the delete would take without issuing it.
func TestRunDestructiveActionByTwoIDsPreviewsWithTheWalkBoundToBothIDs(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunGetStateServer(t, domainRecordPath, map[string]any{keyStateID: 2, keyName: hostWWW})
	request := createRequestWithArgs(t, map[string]any{keyDomainID: float64(1), keyRecordID: float64(2), keyDryRun: true})

	var walkSaw [2]int

	walk := func(_ context.Context, _ *linode.Client, outerID, innerID int, state any) (tools.DryRunDetails, error) {
		walkSaw = [2]int{outerID, innerID}

		fetched, isObject := state.(map[string]any)
		if !isObject || fetched[keyName] != hostWWW {
			t.Errorf("walk received state %v, want the fetched record", state)
		}

		return tools.DryRunDetails{Dependencies: []tools.DryRunDependency{
			{Kind: keyDomain, ID: outerID, Action: tools.DependencyActionRemoved},
		}}, nil
	}

	result, err := tools.RunDestructiveActionByTwoIDs(t.Context(), &request, cfg, domainRecordAction(walk))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("result.IsError = true, text: %s", dryRunResultText(t, result))
	}

	if walkSaw != [2]int{1, 2} {
		t.Errorf("walk received ids %v, want [1 2]", walkSaw)
	}

	body := decodeBody(t, dryRunResultText(t, result))
	assertDryRunRequest(t, body, http.MethodDelete, domainRecordPath)

	deps, isList := body["dependencies"].([]any)
	if !isList || len(deps) != 1 {
		t.Fatalf("dependencies = %v, want the walk's one entry", body["dependencies"])
	}

	dep, _ := deps[0].(map[string]any)
	if dep["kind"] != keyDomain || dep[keyStateID] != float64(1) {
		t.Errorf("dependencies[0] = %v, want domain 1", dep)
	}

	if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
		t.Errorf("methods = %v, want only the GET", *methods)
	}
}

// TestRunDestructiveActionByTwoIDsExecutesTheDeleteWithBothIDs covers the
// confirmed removal: the DELETE lands on the nested path built from both ids
// and the answer is the success body built from the same pair.
func TestRunDestructiveActionByTwoIDsExecutesTheDeleteWithBothIDs(t *testing.T) {
	t.Parallel()

	cfg, deletedPath := recordingDeleteServer(t, map[string]any{keyStateID: 2})
	request := createRequestWithArgs(t, map[string]any{
		keyDomainID: float64(1), keyRecordID: float64(2), keyConfirm: true, keyConfirmBypassDryRun: true,
	})

	result, err := tools.RunDestructiveActionByTwoIDs(t.Context(), &request, cfg, domainRecordAction(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("result.IsError = true, text: %s", dryRunResultText(t, result))
	}

	if got := deletedPath.Load(); got == nil || *got != domainRecordPath {
		t.Errorf("DELETE path = %v, want %s", got, domainRecordPath)
	}

	if text := dryRunResultText(t, result); !strings.Contains(text, "Record 2 removed from domain 1") {
		t.Errorf("text = %q, want the success message for record 2 of domain 1", text)
	}
}

// TestRunDestructiveActionByRegionLabelRequiresRegionAndLabel covers the
// bucket-path argument gate with the sentences the behavior fixtures pin.
func TestRunDestructiveActionByRegionLabelRequiresRegionAndLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args map[string]any
		name string
		want string
	}{
		{name: caseMissingRegion, args: map[string]any{keyLabel: bucketTest}, want: errRegionRequired},
		{name: caseMissingLabel, args: map[string]any{keyRegion: regionUSEast}, want: errLabelRequired},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := createRequestWithArgs(t, testCase.args)

			result, err := tools.RunDestructiveActionByRegionLabel(t.Context(), &request, dryRunNoCallServer(t), bucketAction())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := errorText(t, result); got != testCase.want {
				t.Errorf("text = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestRunDestructiveActionByRegionLabelPreviewsTheBucketState covers the dry
// run of a bucket removal: the state is read at the region/label path and the
// preview names that path as the delete route.
func TestRunDestructiveActionByRegionLabelPreviewsTheBucketState(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunGetStateServer(t, bucketPath, map[string]any{keyLabel: bucketTest, "objects": 4})
	request := createRequestWithArgs(t, map[string]any{keyRegion: regionUSEast, keyLabel: bucketTest, keyDryRun: true})

	result, err := tools.RunDestructiveActionByRegionLabel(t.Context(), &request, cfg, bucketAction())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("result.IsError = true, text: %s", dryRunResultText(t, result))
	}

	body := decodeBody(t, dryRunResultText(t, result))
	assertDryRunRequest(t, body, http.MethodDelete, bucketPath)

	state, isObject := body[keyCurrentState].(map[string]any)
	if !isObject || state["objects"] != float64(4) {
		t.Errorf("current_state = %v, want the fetched bucket", body[keyCurrentState])
	}

	if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
		t.Errorf("methods = %v, want only the GET", *methods)
	}
}

// TestRunDestructiveActionByRegionLabelExecutesTheDeleteAtTheBucketPath covers
// the confirmed bucket removal: the DELETE lands on the region/label path and
// the answer names both.
func TestRunDestructiveActionByRegionLabelExecutesTheDeleteAtTheBucketPath(t *testing.T) {
	t.Parallel()

	cfg, deletedPath := recordingDeleteServer(t, map[string]any{keyLabel: bucketTest})
	request := createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast, keyLabel: bucketTest, keyConfirm: true, keyConfirmBypassDryRun: true,
	})

	result, err := tools.RunDestructiveActionByRegionLabel(t.Context(), &request, cfg, bucketAction())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("result.IsError = true, text: %s", dryRunResultText(t, result))
	}

	if got := deletedPath.Load(); got == nil || *got != bucketPath {
		t.Errorf("DELETE path = %v, want %s", got, bucketPath)
	}

	if text := dryRunResultText(t, result); !strings.Contains(text, "Bucket my-bucket in us-east removed") {
		t.Errorf("text = %q, want the success message naming the bucket", text)
	}
}

// TestDestroyDryRunRefusesAnEnvironmentTheConfigDoesNotName covers a destroy
// preview asked for under an environment the operator never configured: the
// generated handler refuses before any read.
func TestDestroyDryRunRefusesAnEnvironmentTheConfigDoesNotName(t *testing.T) {
	t.Parallel()

	_, _, handler := gentools.NewLinodeVolumeDeleteTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyVolumeID: float64(789), keyDryRun: true, keyEnvironment: tcStaging,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := errorText(t, result); got != envNotFoundStagingText {
		t.Errorf("text = %q, want %q", got, envNotFoundStagingText)
	}
}

// TestDestroyDryRunReportsAStateTheAPIWouldNotServe covers a destroy preview
// whose read sibling answers 404: the preview fails under the fetch sentence
// rather than describing a removal of a resource nobody can see.
func TestDestroyDryRunReportsAStateTheAPIWouldNotServe(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := gentools.NewLinodeVolumeDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyVolumeID: float64(789), keyDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := errorText(t, result); !strings.HasPrefix(got, "Failed to fetch state for dry-run: ") {
		t.Errorf("text = %q, want the fetch failure sentence", got)
	}
}

// TestDestroyDryRunReportsTheDependencyWalkFailure covers a destroy preview
// whose dependency walk could not finish: the preview fails under the walk
// sentence rather than listing the dependencies it managed to find.
func TestDestroyDryRunReportsTheDependencyWalkFailure(t *testing.T) {
	t.Parallel()

	cfg, _ := dryRunGetStateServer(t, instancePath123, map[string]any{keyStateID: 123})
	request := createRequestWithArgs(t, map[string]any{keyDryRun: true})

	result, err := tools.RunDestructiveAction(t.Context(), &request, cfg, &tools.DestructiveAction{
		ToolName:   canRunDestroyTool,
		Method:     http.MethodDelete,
		Path:       instancePath123,
		FetchState: fetchInstanceObject,
		DependencyWalk: func(context.Context, *linode.Client, any) (tools.DryRunDetails, error) {
			return tools.DryRunDetails{}, errWalkFailed
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := errorText(t, result); got != dryRunDependencyFailureText {
		t.Errorf("text = %q, want %q", got, dryRunDependencyFailureText)
	}
}
