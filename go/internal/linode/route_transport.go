package linode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

// The contract-driven twins of the hand-written transports these replaced: a
// route whose request or answer is not JSON is still a declared route, so the
// tool names itself and the method and path come from its tool_route the same
// way every other routed primitive resolves them.

// CallRouteMultipart performs the route the named tool declares with a
// multipart/form-data body framed from a local file's CONTENTS, filed under
// partName. Nothing is decoded out of the answer: the tool's own response is
// built from the arguments it was called with.
//
// The retry policy is the contract's, the way every other mutating primitive
// takes it.
func (c *Client) CallRouteMultipart(
	ctx context.Context, tool string, pathValues []any, partName, filePath string,
) error {
	attempt := func() error {
		body := &bytes.Buffer{}

		contentType, err := FrameMultipart(body, partName, filePath)
		if err != nil {
			return &NetworkError{Operation: tool, Err: err}
		}

		attemptCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		resp, err := c.makeRouteRequestContentType(attemptCtx, tool, contentType, "", body, pathValues...)
		if err != nil {
			return wrapRequestError(tool, err)
		}

		defer drainClose(resp)

		return c.handleResponse(resp, nil)
	}

	return c.attemptUnderPolicy(ctx, tool, attempt)
}

// FrameMultipart writes one file as a multipart form into dst, answering the
// content type that carries the boundary. The form filename is the base name,
// which is what both clients have always sent.
//
// The form is framed whole rather than streamed because the retry policy may
// replay the attempt, and a stream drained by the first try has nothing left to
// send on the second.
//
// dst is a parameter rather than a buffer this opens for itself, because every
// failure below is the writer's: framing into memory cannot fail, so a caller
// that can is what makes these paths reachable at all.
func FrameMultipart(dst io.Writer, partName, filePath string) (string, error) {
	content, err := os.ReadFile(filePath) // #nosec G304 -- the path is the caller's own local file, and reading it is the tool's purpose
	if err != nil {
		return "", fmt.Errorf("failed to read attachment file: %w", err)
	}

	writer := multipart.NewWriter(dst)

	part, err := writer.CreateFormFile(partName, filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("failed to build attachment form field: %w", err)
	}

	if _, err := part.Write(content); err != nil {
		return "", fmt.Errorf("failed to write attachment content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to finalize attachment body: %w", err)
	}

	return writer.FormDataContentType(), nil
}

// CallRouteRawBody performs the route the named tool declares with payload as
// the whole request body under contentType, no JSON around it. Nothing is
// decoded out of the answer, for the reason CallRouteMultipart decodes nothing.
func (c *Client) CallRouteRawBody(
	ctx context.Context, tool string, pathValues []any, contentType string, payload []byte,
) error {
	attempt := func() error {
		attemptCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		resp, err := c.makeRouteRequestContentType(
			attemptCtx, tool, contentType, "", bytes.NewReader(payload), pathValues...,
		)
		if err != nil {
			return wrapRequestError(tool, err)
		}

		defer drainClose(resp)

		return c.handleResponse(resp, nil)
	}

	return c.attemptUnderPolicy(ctx, tool, attempt)
}

// CallRouteRawBodyRead performs the route the named tool declares and answers
// with the response body as it arrived, negotiated under accept. The route
// sends the resource itself rather than a JSON document, so there is nothing to
// decode and the bytes are the answer.
func (c *Client) CallRouteRawBodyRead(
	ctx context.Context, tool string, pathValues []any, accept string,
) ([]byte, error) {
	var payload []byte

	err := c.attemptUnderPolicy(ctx, tool, func() error {
		payload = nil

		attemptCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		resp, err := c.makeRouteRequestContentType(attemptCtx, tool, "", accept, nil, pathValues...)
		if err != nil {
			return wrapRequestError(tool, err)
		}

		defer drainClose(resp)

		payload, err = readRawBody(c, resp, tool)

		return err
	})

	return payload, err
}

// readRawBody reads the whole answer before deciding on it, because the status
// check needs the same bytes an error report would quote.
func readRawBody(client *Client, resp *http.Response, tool string) ([]byte, error) {
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &NetworkError{Operation: tool, Err: err}
	}

	if resp.StatusCode < http.StatusBadRequest {
		return payload, nil
	}

	apiErr := client.handleErrorResponse(resp.StatusCode, payload, resp)

	// Stamp the request method onto the API error so the retry layer can decide
	// whether a 5xx is safe to replay.
	if typedErr, ok := errors.AsType[*APIError](apiErr); ok && resp.Request != nil {
		typedErr.Method = resp.Request.Method
	}

	return nil, apiErr
}
