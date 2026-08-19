package tools_test

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The two parts of the generated list seam no emitted tool reaches yet: the
// filter over a boolean field, and the declared argument check that replaces
// the derived required-value ones. Both are driven straight through the seam,
// since a tool exercising them arrives when its family migrates.

const (
	seamHookMessage  = "tier must be standard or enterprise"
	seamPublicImage  = "linode/ubuntu"
	seamPrivateImage = "private/1"
)

// boolFilterPage is the collection the flag filter narrows.
func boolFilterPage() []*linodev1.Image {
	return []*linodev1.Image{
		{Id: seamPrivateImage, IsPublic: false},
		{Id: seamPublicImage, IsPublic: true},
	}
}

// A flag filter compares against the "true" or "false" a caller sends, because
// every filter argument arrives as text.
func TestNewBoolFilterMatchesTheFlagTheCallerSent(t *testing.T) {
	t.Parallel()

	filter := tools.NewBoolFilter("is_public", "Filter by public status",
		func(item *linodev1.Image) bool { return item.GetIsPublic() })

	if filter.Param != "is_public" {
		t.Errorf("filter.Param = %v, want %v", filter.Param, "is_public")
	}

	tests := []struct {
		value string
		want  string
	}{
		{value: boolStringTrue, want: seamPublicImage},
		{value: caseFalse, want: seamPrivateImage},
		{value: "TRUE", want: seamPublicImage},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			t.Parallel()

			kept := filter.Match(boolFilterPage(), test.value)

			if len(kept) != 1 {
				t.Fatalf("len(kept) = %v, want %v", len(kept), 1)
			}

			if kept[0].GetId() != test.want {
				t.Errorf("kept[0].GetId() = %v, want %v", kept[0].GetId(), test.want)
			}
		})
	}
}

// A tool that declares its own check answers that check's sentence, and the
// path values are never read: the hook owns the whole argument check, which is
// the point of declaring one.
func TestGeneratedSubresourceListRunsTheDeclaredCheckInsteadOfTheDerivedOnes(t *testing.T) {
	t.Parallel()

	var readPath bool

	_, handler := tools.NewGeneratedSubresourceListTool(
		newTestConfig("http://127.0.0.1:1"),
		"linode_seam_probe_list",
		"A list whose contract declares its own argument check.",
		"linode.mcp.v1.DomainListInput",
		nil,
		func(*mcp.CallToolRequest) string { return seamHookMessage },
		[]tools.ListPathValue{
			func(*mcp.CallToolRequest) (any, string) {
				readPath = true

				return "unused", ""
			},
		},
		func(context.Context, *linode.Client, *mcp.CallToolRequest, []any, int, int) ([]*linodev1.Domain, error) {
			t.Error("fetch ran, want the declared check to answer first")

			return nil, nil
		},
		nil,
		func(items []*linodev1.Domain, count int32, filter *string) *linodev1.DomainListResponse {
			return &linodev1.DomainListResponse{Count: count, Filter: filter, Domains: items}
		},
	)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := resultText(t, result); got != seamHookMessage {
		t.Errorf("result = %v, want %v", got, seamHookMessage)
	}

	if readPath {
		t.Error("a path value was read, want the declared check to answer before any")
	}
}

// Without a declared check the path readers run in order, and the first one
// that reports a message answers before anything is fetched.
func TestGeneratedSubresourceListStopsAtTheFirstUnreadablePathValue(t *testing.T) {
	t.Parallel()

	const missing = "config_id is required"

	var secondRead bool

	_, handler := tools.NewGeneratedSubresourceListTool(
		newTestConfig("http://127.0.0.1:1"),
		"linode_seam_probe_list",
		"A list nested two deep.",
		"linode.mcp.v1.DomainListInput",
		nil,
		nil,
		[]tools.ListPathValue{
			func(*mcp.CallToolRequest) (any, string) { return nil, missing },
			func(*mcp.CallToolRequest) (any, string) {
				secondRead = true

				return 1, ""
			},
		},
		func(context.Context, *linode.Client, *mcp.CallToolRequest, []any, int, int) ([]*linodev1.Domain, error) {
			t.Error("fetch ran, want the missing path value to answer first")

			return nil, nil
		},
		nil,
		func(items []*linodev1.Domain, count int32, filter *string) *linodev1.DomainListResponse {
			return &linodev1.DomainListResponse{Count: count, Filter: filter, Domains: items}
		},
	)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := resultText(t, result); got != missing {
		t.Errorf("result = %v, want %v", got, missing)
	}

	if secondRead {
		t.Error("the second path value was read, want the first failure to answer")
	}
}
