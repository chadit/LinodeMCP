package linode

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
)

// httpCreateSupportTicketAttachment uploads a local file as a ticket attachment.
// This endpoint consumes multipart/form-data, not JSON, so the file goes under the
// "file" form field to match the Python client's multipart upload.
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
// boundary. The form filename is the base name, matching the Python client.
func supportTicketAttachmentBody(path string) (*bytes.Buffer, string, error) {
	content, err := os.ReadFile(path) // #nosec G304 -- path is the user-selected local file to upload; reading it is the tool's purpose (mirrors the Python client's attachment upload)
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
