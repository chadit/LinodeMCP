package linode

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// httpCreateSupportTicketProto opens a support ticket.
func (c *Client) httpCreateSupportTicketProto(ctx context.Context, request *CreateSupportTicketRequest) (*linodev1.SupportTicket, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_support_ticket_create", request)
	if err != nil {
		return nil, wrapRequestError("CreateSupportTicket", err)
	}

	defer drainClose(resp)

	ticket := &linodev1.SupportTicket{}
	if err := c.handleProtoResponse(resp, ticket); err != nil {
		return nil, err
	}

	return ticket, nil
}

// httpGetSupportTicketProto retrieves one support ticket as a proto message.
func (c *Client) httpGetSupportTicketProto(ctx context.Context, ticketID int) (*linodev1.SupportTicket, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_support_ticket_get", nil, ticketID)
	if err != nil {
		return nil, wrapRequestError("GetSupportTicket", err)
	}

	defer drainClose(resp)

	ticket := &linodev1.SupportTicket{}
	if err := c.handleProtoResponse(resp, ticket); err != nil {
		return nil, err
	}

	return ticket, nil
}

// httpCreateSupportTicketAttachment uploads a local file as a ticket attachment.
// This endpoint consumes multipart/form-data, not JSON, so the file goes under the
// "file" form field to match Python's make_file_request.
func (c *Client) httpCreateSupportTicketAttachment(ctx context.Context, ticketID int, request *CreateSupportTicketAttachmentRequest) (*SupportTicketAttachment, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	body, contentType, err := supportTicketAttachmentBody(request.File)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateSupportTicketAttachment", Err: err}
	}

	resp, err := c.makeRouteRequestContentType(ctx, "linode_support_ticket_attachment_create", contentType, body, ticketID)
	if err != nil {
		return nil, wrapRequestError("CreateSupportTicketAttachment", err)
	}

	defer drainClose(resp)

	var attachment SupportTicketAttachment
	if err := c.handleResponse(resp, &attachment); err != nil {
		return nil, err
	}

	return &attachment, nil
}

// supportTicketAttachmentBody builds a multipart body holding the file at path
// under the "file" field, returning it with the content type that carries the
// boundary. The form filename is the base name, matching Python's
// make_file_request.
func supportTicketAttachmentBody(path string) (*bytes.Buffer, string, error) {
	content, err := os.ReadFile(path) // #nosec G304 -- path is the user-selected local file to upload; reading it is the tool's purpose (mirrors Python make_file_request)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read attachment file: %w", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return nil, "", fmt.Errorf("failed to build attachment form field: %w", err)
	}

	if _, err := part.Write(content); err != nil {
		return nil, "", fmt.Errorf("failed to write attachment content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to finalize attachment body: %w", err)
	}

	return body, writer.FormDataContentType(), nil
}

// httpCreateSupportTicketReplyProto creates a reply on a support ticket.
func (c *Client) httpCreateSupportTicketReplyProto(ctx context.Context, ticketID int, request *CreateSupportTicketReplyRequest) (*linodev1.SupportTicketReply, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_support_ticket_reply_create", request, ticketID)
	if err != nil {
		return nil, wrapRequestError("CreateSupportTicketReply", err)
	}

	defer drainClose(resp)

	reply := &linodev1.SupportTicketReply{}
	if err := c.handleProtoResponse(resp, reply); err != nil {
		return nil, err
	}

	return reply, nil
}

// httpListSupportTicketsProto retrieves support tickets as proto messages for the
// proto-backed list path. page/page_size flow through withPaginationQuery, so the
// request matches httpListSupportTickets.
func (c *Client) httpListSupportTicketsProto(ctx context.Context, page, pageSize int) ([]*linodev1.SupportTicket, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListSupportTickets",
		"linode_support_ticket_list", "", nil, page, pageSize,
		func() *linodev1.SupportTicket { return &linodev1.SupportTicket{} })
}

// httpCloseSupportTicket closes one support ticket.
func (c *Client) httpCloseSupportTicket(ctx context.Context, ticketID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_support_ticket_close", nil, ticketID)
	if err != nil {
		return wrapRequestError("CloseSupportTicket", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpListSupportTicketRepliesProto retrieves a support ticket's replies as proto
// messages for the proto-backed list path. It names the same tool as
// httpListSupportTicketReplies, so both resolve one declared route, then
// page/page_size flow through withPaginationQuery and the request matches.
func (c *Client) httpListSupportTicketRepliesProto(ctx context.Context, ticketID, page, pageSize int) ([]*linodev1.SupportTicketReply, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListSupportTicketReplies",
		"linode_support_ticket_reply_list", "", []any{ticketID}, page, pageSize,
		func() *linodev1.SupportTicketReply { return &linodev1.SupportTicketReply{} })
}
