package tools_test

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

// Both drivers ask the contract for the tool's rules before they read anything
// of their own, which is the half of the seam no emitted call site can show:
// the emitter writes the check into the handlers it writes the body of, and
// these two tiers reach it through the driver instead.
//
// The message named below carries rules and the driver's own arguments do not
// match the tool it belongs to, deliberately: what these pin is that the driver
// consults the contract and answers its sentence, and a rule whose sentence the
// driver could also have produced on its own would show nothing.
const (
	ruledMessage  = "linode.mcp.v1.DomainCreateInput"
	ruledSentence = "domain is required"
)

// A destroy answers a broken rule before it reads the id it is addressed by, so
// a caller hears what is wrong with the call rather than what is missing from
// it.
func TestDestructiveActionAnswersTheContractBeforeReadingItsID(t *testing.T) {
	t.Parallel()

	request := createRequestWithArgs(t, map[string]any{keyType: filterTypeMaster})

	result, err := tools.RunDestructiveActionWithID(
		t.Context(), &request, newTestConfig("http://127.0.0.1:1"),
		&tools.DestructiveActionByID{
			ToolName:       "linode_seam_probe_delete",
			InputMessage:   ruledMessage,
			IDParam:        keyDomainID,
			Method:         "DELETE",
			PathPattern:    "/domains/%d",
			ConfirmMessage: "This removes the probe. Set confirm=true to proceed.",
			SuccessProto: func(int) proto.Message {
				t.Error("the success body was built, want the rule to answer first")

				return &linodev1.DomainDeleteResponse{}
			},
			FetchState: func(context.Context, *linode.Client, int) (any, error) {
				t.Error("state was fetched, want the rule to answer first")

				return &linodev1.Domain{}, nil
			},
			Execute: func(context.Context, *linode.Client, int) error {
				t.Error("the delete ran, want the rule to answer first")

				return nil
			},
			HashIgnore: twostage.HashIgnoreFields("domain"),
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := resultText(t, result); got != ruledSentence {
		t.Errorf("result = %v, want %v", got, ruledSentence)
	}
}

// A collection answers a broken rule before it fetches a page, so a call the
// contract refuses is never made.
func TestGeneratedListAnswersTheContractBeforeFetching(t *testing.T) {
	t.Parallel()

	_, handler := tools.NewGeneratedListTool(
		newTestConfig("http://127.0.0.1:1"),
		"linode_seam_probe_list",
		"A list whose input message declares rules.",
		ruledMessage,
		nil,
		nil,
		func(context.Context, *linode.Client, *mcp.CallToolRequest, int, int) ([]*linodev1.Domain, error) {
			t.Error("fetch ran, want the rule to answer first")

			return nil, nil
		},
		nil,
		func(items []*linodev1.Domain, count int32, filter *string) *linodev1.DomainListResponse {
			return &linodev1.DomainListResponse{Count: count, Filter: filter, Domains: items}
		},
	)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyType: filterTypeMaster}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := resultText(t, result); got != ruledSentence {
		t.Errorf("result = %v, want %v", got, ruledSentence)
	}
}
