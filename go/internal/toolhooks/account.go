package toolhooks

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// LinodeAccountOauthClientThumbnailUpdateExecute uploads the decoded image. The
// route consumes the raw PNG bytes under image/png, so the JSON body the
// generated handler built is not the request: the client method that frames the
// bytes is, and it is the same one the hand-written handler called.
//
// The id is read off the call rather than out of that body because the body is
// not what travels. It carries the base64 text so the schema can advertise it
// and the validate hook can decode it; what reaches the API is the bytes.
func LinodeAccountOauthClientThumbnailUpdateExecute(
	ctx context.Context,
	client *linode.Client,
	request *mcp.CallToolRequest,
	pathValues []any,
	_ any,
) error {
	clientID, ok := pathValues[0].(string)
	if !ok {
		return fmt.Errorf("%w: %T", ErrOAuthClientIDNotText, pathValues[0])
	}

	// Decoded again rather than carried: the validate hook answers a sentence,
	// not a value, and it has already accepted this text by here.
	thumbnailPNG, _ := tools.OAuthClientThumbnailPNG(request)

	// Wrapped with no prose of its own: the handler formats the tool's declared
	// error message around whatever comes back, and Python's twin adds nothing
	// either, so a phrase here would put the two languages a sentence apart.
	if err := client.UpdateOAuthClientThumbnail(ctx, clientID, thumbnailPNG); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// LinodeAccountOauthClientThumbnailGetExecute fetches the image and answers with
// it as text. The route sends raw PNG bytes under image/png, so a generated read
// would hand a PNG-headed body to protojson; the client method that reads the
// bytes is the call, and it is the same one the hand-written handler made.
//
// The base64 is the answer rather than a redaction: an avatar is what the caller
// asked for, and standing in for it would leave the tool with nothing to report.
//
// Only the members the contract declares assembled are read off what this
// answers; the client_id beside them is the handler's, from the call.
func LinodeAccountOauthClientThumbnailGetExecute(
	ctx context.Context,
	client *linode.Client,
	_ *mcp.CallToolRequest,
	pathValues []any,
) (*linodev1.OAuthClientThumbnail, error) {
	clientID, ok := pathValues[0].(string)
	if !ok {
		return nil, fmt.Errorf("%w: %T", ErrOAuthClientIDNotText, pathValues[0])
	}

	// Wrapped with no prose of its own, for the reason the update's execute hook
	// gives: the handler formats the declared sentence around whatever comes back.
	thumbnailPNG, err := client.GetOAuthClientThumbnail(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return &linodev1.OAuthClientThumbnail{
		ThumbnailPngBase64: base64.StdEncoding.EncodeToString(thumbnailPNG),
	}, nil
}

// LinodeAccountPaymentCreateNormalize trims the amount, which is the value this
// tool has always put on the wire.
func LinodeAccountPaymentCreateNormalize(request *mcp.CallToolRequest) {
	arguments := request.GetArguments()

	if usd, supplied := arguments["usd"].(string); supplied {
		arguments["usd"] = strings.TrimSpace(usd)
	}
}

// LinodeAccountServiceTransferCreateNormalize folds the linode_ids convenience
// form into entities, which is the only shape either language ever put on the
// wire. A caller-supplied entities object wins outright, since it can name
// entity types linode_ids cannot express. An unusable linode_ids value is left
// alone so validate can name it.
func LinodeAccountServiceTransferCreateNormalize(request *mcp.CallToolRequest) {
	arguments := request.GetArguments()

	entities, message := tools.ObjectMapArgument(arguments["entities"], "entities")
	if message != "" || len(entities) > 0 {
		return
	}

	raw, supplied := arguments["linode_ids"]
	if !supplied {
		return
	}

	ids, message := tools.IntSliceArgument(raw, "linode_ids")
	if message != "" {
		return
	}

	arguments["entities"] = map[string]any{"linodes": ids}

	delete(arguments, "linode_ids")
}
