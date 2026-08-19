package toolhooks

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// LinodeAccountUserCreatePreview names the user being added and the restriction
// it starts with, neither of which a declared sentence can interpolate. Python
// carried the sentence and the request echo, Go carried neither; both are here
// now.
func LinodeAccountUserCreatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	result, err := tools.RunDryRunPreviewWithBodyDetailed(
		ctx, request, cfg, "linode_account_user_create", method, path, body, nil,
		func(ctx context.Context, _ *linode.Client, _ any) (tools.DryRunDetails, error) {
			return previewDetails(ctx, "linode_account_user_create", func() tools.DryRunDetails {
				return tools.DryRunDetails{
					SideEffects: []string{accountUserCreateSentence(request)},
				}
			})
		},
	)

	return wrapPreview("linode_account_user_create", result, err)
}

// accountUserCreateSentence renders the booleans and quotes the way the echoed
// request body renders them, so the prose and the body a caller reads together
// do not spell the same value two ways.
func accountUserCreateSentence(request *mcp.CallToolRequest) string {
	return fmt.Sprintf("A new account user %q will be created with restricted=%t.",
		request.GetString("username", ""), request.GetBool("restricted", false))
}
