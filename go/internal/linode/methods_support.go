package linode

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

const endpointSupportTickets = "/support/tickets"

// httpCreateSupportTicketProto opens a support ticket and decodes the created
// ticket into a proto message for the proto-backed write path.
func (c *Client) httpCreateSupportTicketProto(ctx context.Context, request *CreateSupportTicketRequest) (*linodev1.SupportTicket, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointSupportTickets, request)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateSupportTicket", Err: err}
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

	endpoint := endpointSupportTickets + "/" + url.PathEscape(strconv.Itoa(ticketID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetSupportTicket", Err: err}
	}

	defer drainClose(resp)

	ticket := &linodev1.SupportTicket{}
	if err := c.handleProtoResponse(resp, ticket); err != nil {
		return nil, err
	}

	return ticket, nil
}

// httpCreateSupportTicketAttachment uploads a local file as an attachment for a
// support ticket. The endpoint consumes multipart/form-data (not JSON), so the
// file is read from request.File and sent under the "file" form field, mirroring
// Python's make_file_request.
func (c *Client) httpCreateSupportTicketAttachment(ctx context.Context, ticketID int, request *CreateSupportTicketAttachmentRequest) (*SupportTicketAttachment, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	body, contentType, err := supportTicketAttachmentBody(request.File)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateSupportTicketAttachment", Err: err}
	}

	endpoint := endpointSupportTickets + "/" + url.PathEscape(strconv.Itoa(ticketID)) + "/attachments"

	resp, err := c.makeRequestWithContentType(ctx, http.MethodPost, endpoint, body, contentType)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateSupportTicketAttachment", Err: err}
	}

	defer drainClose(resp)

	var attachment SupportTicketAttachment
	if err := c.handleResponse(resp, &attachment); err != nil {
		return nil, err
	}

	return &attachment, nil
}

// supportTicketAttachmentBody reads the file at path into a multipart/form-data
// body under the "file" field and returns the body plus the content type that
// carries the boundary. The filename is the base name, matching Python's
// make_file_request (files={"file": (path.name, handle)}).
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

// httpCreateSupportTicketReplyProto creates a reply for a support ticket and
// decodes the created reply into a proto message for the proto-backed write path.
func (c *Client) httpCreateSupportTicketReplyProto(ctx context.Context, ticketID int, request *CreateSupportTicketReplyRequest) (*linodev1.SupportTicketReply, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointSupportTickets + "/" + url.PathEscape(strconv.Itoa(ticketID)) + "/replies"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, request)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateSupportTicketReply", Err: err}
	}

	defer drainClose(resp)

	reply := &linodev1.SupportTicketReply{}
	if err := c.handleProtoResponse(resp, reply); err != nil {
		return nil, err
	}

	return reply, nil
}

// httpListSupportTickets retrieves support tickets.

// httpListSupportTicketsProto retrieves support tickets as proto messages for the
// proto-backed list path. page/page_size flow through withPaginationQuery, so the
// request matches httpListSupportTickets.
func (c *Client) httpListSupportTicketsProto(ctx context.Context, page, pageSize int) ([]*linodev1.SupportTicket, error) {
	return listProtoElementsPaginated(ctx, c, "ListSupportTickets", endpointSupportTickets, page, pageSize,
		func() *linodev1.SupportTicket { return &linodev1.SupportTicket{} })
}

// httpCloseSupportTicket closes one support ticket.
func (c *Client) httpCloseSupportTicket(ctx context.Context, ticketID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointSupportTickets+"/"+url.PathEscape(strconv.Itoa(ticketID))+"/close", nil)
	if err != nil {
		return &NetworkError{Operation: "CloseSupportTicket", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all support methods use this pattern

	return c.handleResponse(resp, nil)
}

// httpListSupportTicketRepliesProto retrieves a support ticket's replies as proto
// messages for the proto-backed list path. The endpoint formats the ticket id
// exactly like httpListSupportTicketReplies, then page/page_size flow through
// withPaginationQuery, so the request matches.
func (c *Client) httpListSupportTicketRepliesProto(ctx context.Context, ticketID, page, pageSize int) ([]*linodev1.SupportTicketReply, error) {
	endpoint := endpointSupportTickets + "/" + url.PathEscape(strconv.Itoa(ticketID)) + "/replies"

	return listProtoElementsPaginated(ctx, c, "ListSupportTicketReplies", endpoint, page, pageSize,
		func() *linodev1.SupportTicketReply { return &linodev1.SupportTicketReply{} })
}
