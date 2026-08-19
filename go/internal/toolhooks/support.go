package toolhooks

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// LinodeSupportTicketAttachmentCreateExecute uploads the named file. The route
// consumes multipart/form-data assembled from the file's CONTENTS, so the JSON
// body the generated handler built is not the request: the client method that
// reads the file and frames the form is, and it is the same one the hand-written
// handler called.
//
// The path is read off the call rather than out of that body because the body is
// not what travels. It carries the path so the schema can advertise it and the
// rules can check it; what reaches the API is the bytes it names.
func LinodeSupportTicketAttachmentCreateExecute(
	ctx context.Context,
	client *linode.Client,
	request *mcp.CallToolRequest,
	pathValues []any,
	_ any,
) error {
	ticketID, ok := pathValues[0].(int)
	if !ok {
		return fmt.Errorf("%w: %T", ErrTicketIDNotAnInt, pathValues[0])
	}

	upload := &linode.CreateSupportTicketAttachmentRequest{File: request.GetString("file", "")}

	// Wrapped with no prose of its own: the handler formats the tool's declared
	// error message around whatever comes back, and Python's twin adds nothing
	// either, so a phrase here would put the two languages a sentence apart.
	if _, err := client.CreateSupportTicketAttachment(ctx, ticketID, upload); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}
