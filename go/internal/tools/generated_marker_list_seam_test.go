package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	markerSeamSchema = "linode.mcp.v1.ObjectStorageBucketObjectListInput"
	markerSeamTool   = "linode_object_storage_bucket_object_list"
	markerSeamNext   = "images/b.png"
	markerSeamBucket = "photos"
	markerSeamRegion = regionUSEast1
	// The narrowing argument this route takes beside the prefix.
	keyDelimiter = "delimiter"
	// The path value a case that never reads it hands back.
	seamUnusedPathValue = "unused"
)

// markerSeamArgs is a call that names the bucket and narrows it, which is the
// shape every case here varies from.
func markerSeamArgs() map[string]any {
	return map[string]any{
		keyRegion:                markerSeamRegion,
		managedServiceLabelParam: markerSeamBucket,
		keyReservedIPPrefix:      objectPrefixImages,
	}
}

// newMarkerSeamTool builds the driver over a fetch that answers the given page,
// so the assertions are about what the driver reports rather than about HTTP.
func newMarkerSeamTool(
	t *testing.T, items []*linodev1.ObjectStorageObject, cursor linode.ListMarkerPage, echoes []string,
) tools.Handler {
	t.Helper()

	_, handler := tools.NewGeneratedMarkerListTool(
		newTestConfig("http://127.0.0.1:1"),
		markerSeamTool,
		"Lists objects in an Object Storage bucket.",
		markerSeamSchema,
		nil,
		nil,
		[]tools.ListPathValue{
			func(request *mcp.CallToolRequest) (any, string) {
				return request.GetString(keyRegion, ""), ""
			},
			func(request *mcp.CallToolRequest) (any, string) {
				return request.GetString(managedServiceLabelParam, ""), ""
			},
		},
		func(context.Context, *linode.Client, *mcp.CallToolRequest, []any,
		) ([]*linodev1.ObjectStorageObject, linode.ListMarkerPage, error) {
			return items, cursor, nil
		},
		echoes,
		func(items []*linodev1.ObjectStorageObject, count int32, filter *string,
			cursor linode.ListMarkerPage,
		) *linodev1.ObjectStorageObjectListResponse {
			return &linodev1.ObjectStorageObjectListResponse{
				Count:       count,
				Filter:      filter,
				IsTruncated: cursor.IsTruncated,
				NextMarker:  tools.TextOrAbsent(cursor.NextMarker),
				Objects:     items,
			}
		},
	)

	return handler
}

// markerSeamBody runs one call and decodes the answer.
func markerSeamBody(t *testing.T, handler tools.Handler, args map[string]any) map[string]any {
	t.Helper()

	result, err := handler(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(resultText(t, result)), &body); err != nil {
		t.Fatalf("decode answer: %v", err)
	}

	return body
}

// The cursor reaches the answer, which is the whole reason this tier exists: a
// truncated page whose marker went missing reads as the end of the collection.
func TestGeneratedMarkerListReportsTheCursorItFetched(t *testing.T) {
	t.Parallel()

	handler := newMarkerSeamTool(t,
		[]*linodev1.ObjectStorageObject{{Name: "images/a.png"}, {Name: markerSeamNext}},
		linode.ListMarkerPage{IsTruncated: true, NextMarker: markerSeamNext},
		[]string{keyReservedIPPrefix, keyDelimiter})

	body := markerSeamBody(t, handler, markerSeamArgs())

	if body["is_truncated"] != true {
		t.Errorf("is_truncated = %v, want true", body["is_truncated"])
	}

	if body["next_marker"] != markerSeamNext {
		t.Errorf("next_marker = %v, want %v", body["next_marker"], markerSeamNext)
	}

	if body["count"] != float64(2) {
		t.Errorf("count = %v, want 2", body["count"])
	}
}

// A complete listing leaves the marker out rather than reporting an empty one,
// which would have a caller ask for a page that does not exist.
func TestGeneratedMarkerListOmitsAMarkerTheRouteDidNotHandOut(t *testing.T) {
	t.Parallel()

	handler := newMarkerSeamTool(t,
		[]*linodev1.ObjectStorageObject{{Name: "only.png"}},
		linode.ListMarkerPage{},
		[]string{keyReservedIPPrefix, keyDelimiter})

	body := markerSeamBody(t, handler, markerSeamArgs())

	if _, reported := body["next_marker"]; reported {
		t.Errorf("next_marker = %v, want it left out", body["next_marker"])
	}

	if body["is_truncated"] != false {
		t.Errorf("is_truncated = %v, want false", body["is_truncated"])
	}
}

// The echo names what the route was asked to narrow by. A cursor argument is a
// position rather than a filter, so it is not on the list and cannot appear.
func TestGeneratedMarkerListEchoesOnlyTheNarrowingArguments(t *testing.T) {
	t.Parallel()

	handler := newMarkerSeamTool(t, nil, linode.ListMarkerPage{},
		[]string{keyReservedIPPrefix, keyDelimiter})

	args := markerSeamArgs()
	args[keyDelimiter] = "/"
	args["marker"] = "images/a.png"
	args["page_size"] = "100"

	body := markerSeamBody(t, handler, args)

	if want := "prefix=images/, delimiter=/"; body["filter"] != want {
		t.Errorf("filter = %v, want %v", body["filter"], want)
	}
}

// A page narrowed by nothing echoes nothing, so the member stays out of the
// answer rather than reporting an empty filter.
func TestGeneratedMarkerListEchoesNothingWhenNothingWasAsked(t *testing.T) {
	t.Parallel()

	handler := newMarkerSeamTool(t, nil, linode.ListMarkerPage{},
		[]string{keyReservedIPPrefix, keyDelimiter})

	body := markerSeamBody(t, handler,
		map[string]any{keyRegion: markerSeamRegion, managedServiceLabelParam: markerSeamBucket})

	if _, reported := body["filter"]; reported {
		t.Errorf("filter = %v, want it left out", body["filter"])
	}
}

// A tool advertising no echoes at all still answers, which is the shape a
// marker route with no narrowing arguments would take.
func TestGeneratedMarkerListAnswersWithoutAnyDeclaredEchoes(t *testing.T) {
	t.Parallel()

	handler := newMarkerSeamTool(t,
		[]*linodev1.ObjectStorageObject{{Name: "only.png"}}, linode.ListMarkerPage{}, nil)

	body := markerSeamBody(t, handler, markerSeamArgs())

	if _, reported := body["filter"]; reported {
		t.Errorf("filter = %v, want it left out", body["filter"])
	}

	if body["count"] != float64(1) {
		t.Errorf("count = %v, want 1", body["count"])
	}
}

// A path value the driver cannot read answers before the fetch, the same way
// the page-envelope tiers do.
func TestGeneratedMarkerListReportsAFailedFetchWithTheDeclaredSentence(t *testing.T) {
	t.Parallel()

	const failure = "Failed to list contents of bucket 'photos'"

	_, handler := tools.NewGeneratedMarkerListTool(
		newTestConfig("http://127.0.0.1:1"),
		markerSeamTool,
		"Lists objects in an Object Storage bucket.",
		markerSeamSchema,
		func(*mcp.CallToolRequest, error) string { return failure },
		nil,
		nil,
		func(context.Context, *linode.Client, *mcp.CallToolRequest, []any,
		) ([]*linodev1.ObjectStorageObject, linode.ListMarkerPage, error) {
			return nil, linode.ListMarkerPage{}, errors.New("upstream refused")
		},
		nil,
		func(items []*linodev1.ObjectStorageObject, count int32, _ *string,
			_ linode.ListMarkerPage,
		) *linodev1.ObjectStorageObjectListResponse {
			return &linodev1.ObjectStorageObjectListResponse{Count: count, Objects: items}
		},
	)

	result, err := handler(t.Context(), createRequestWithArgs(t, markerSeamArgs()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := resultText(t, result); got != failure {
		t.Errorf("result = %v, want %v", got, failure)
	}
}

// The marker driver refuses an argument the message does not declare, the same
// way the emitted handlers do. The check lives in the driver because emitList
// writes a driver call and no handler body, so nothing emitted could carry it.
func TestGeneratedMarkerListRefusesAnArgumentTheMessageDoesNotDeclare(t *testing.T) {
	t.Parallel()

	const refusal = "Unsupported argument(s) for linode_object_storage_bucket_object_list: bogus_field"

	_, handler := tools.NewGeneratedMarkerListTool(
		newTestConfig("http://127.0.0.1:1"),
		markerSeamTool,
		"Lists objects in an Object Storage bucket.",
		markerSeamSchema,
		nil,
		nil,
		nil,
		func(context.Context, *linode.Client, *mcp.CallToolRequest, []any,
		) ([]*linodev1.ObjectStorageObject, linode.ListMarkerPage, error) {
			t.Error("fetch ran, want the refusal to answer first")

			return nil, linode.ListMarkerPage{}, nil
		},
		nil,
		func(items []*linodev1.ObjectStorageObject, count int32, _ *string,
			_ linode.ListMarkerPage,
		) *linodev1.ObjectStorageObjectListResponse {
			return &linodev1.ObjectStorageObjectListResponse{Count: count, Objects: items}
		},
	)

	args := markerSeamArgs()
	args["bogus_field"] = "x"

	result, err := handler(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := resultText(t, result); got != refusal {
		t.Errorf("result = %v, want %v", got, refusal)
	}
}

// The declared check owns the whole argument read, so it answers before any
// path value is looked at and before the route is called.
func TestGeneratedMarkerListRunsTheDeclaredCheckBeforeAnyPathValue(t *testing.T) {
	t.Parallel()

	const refusal = "region must be a valid region or cluster ID"

	var readPath bool

	_, handler := tools.NewGeneratedMarkerListTool(
		newTestConfig("http://127.0.0.1:1"),
		markerSeamTool,
		"Lists objects in an Object Storage bucket.",
		markerSeamSchema,
		nil,
		func(*mcp.CallToolRequest) string { return refusal },
		[]tools.ListPathValue{
			func(*mcp.CallToolRequest) (any, string) {
				readPath = true

				return seamUnusedPathValue, ""
			},
		},
		func(context.Context, *linode.Client, *mcp.CallToolRequest, []any,
		) ([]*linodev1.ObjectStorageObject, linode.ListMarkerPage, error) {
			t.Error("fetch ran, want the declared check to answer first")

			return nil, linode.ListMarkerPage{}, nil
		},
		nil,
		func(items []*linodev1.ObjectStorageObject, count int32, _ *string,
			_ linode.ListMarkerPage,
		) *linodev1.ObjectStorageObjectListResponse {
			return &linodev1.ObjectStorageObjectListResponse{Count: count, Objects: items}
		},
	)

	result, err := handler(t.Context(), createRequestWithArgs(t, markerSeamArgs()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := resultText(t, result); got != refusal {
		t.Errorf("result = %v, want %v", got, refusal)
	}

	if readPath {
		t.Error("a path value was read, want the declared check to answer before any")
	}
}

// The driver asks the contract for the tool's rules before it reads anything of
// its own. The message named here carries rules the driver's own arguments do
// not satisfy, deliberately: what this pins is that the contract is consulted,
// which no emitted call site can show.
func TestGeneratedMarkerListAnswersTheContractBeforeReadingItsPath(t *testing.T) {
	t.Parallel()

	var readPath bool

	_, handler := tools.NewGeneratedMarkerListTool(
		newTestConfig("http://127.0.0.1:1"),
		markerSeamTool,
		"Lists objects in an Object Storage bucket.",
		ruledMessage,
		nil,
		nil,
		[]tools.ListPathValue{
			func(*mcp.CallToolRequest) (any, string) {
				readPath = true

				return seamUnusedPathValue, ""
			},
		},
		func(context.Context, *linode.Client, *mcp.CallToolRequest, []any,
		) ([]*linodev1.ObjectStorageObject, linode.ListMarkerPage, error) {
			t.Error("fetch ran, want the contract to answer first")

			return nil, linode.ListMarkerPage{}, nil
		},
		nil,
		func(items []*linodev1.ObjectStorageObject, count int32, _ *string,
			_ linode.ListMarkerPage,
		) *linodev1.ObjectStorageObjectListResponse {
			return &linodev1.ObjectStorageObjectListResponse{Count: count, Objects: items}
		},
	)

	result, err := handler(t.Context(), createRequestWithArgs(t, markerSeamArgs()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := resultText(t, result); got != ruledSentence {
		t.Errorf("result = %v, want %v", got, ruledSentence)
	}

	if readPath {
		t.Error("a path value was read, want the contract to answer before any")
	}
}

// A client that cannot be built answers before the fetch, so a caller hears
// which environment is missing rather than a failed listing.
func TestGeneratedMarkerListReportsAnUnresolvableEnvironment(t *testing.T) {
	t.Parallel()

	handler := newMarkerSeamTool(t, nil, linode.ListMarkerPage{}, nil)

	args := markerSeamArgs()
	args[keyEnvironment] = tcStaging

	result, err := handler(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := resultText(t, result); !strings.Contains(got, "staging") {
		t.Errorf("result = %v, want it to name the missing environment", got)
	}
}
