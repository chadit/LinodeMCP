package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chadit/LinodeMCP/internal/server"
)

// Result keys in the MCP CallToolResult wire shape. The wire uses
// camelCase (isError), so the response is walked as generic maps rather
// than decoded into Go-tagged structs; see the integration test.
const (
	keyResult  = "result"
	keyError   = "error"
	keyContent = "content"
	keyText    = "text"
	keyMessage = "message"
	keyIsError = "isError"
)

// CallResult is the parsed outcome of one tools/call. Text is the joined
// text payload from the result's content blocks (the tool's JSON for the
// data tools, a plain message for the meta tools). IsError reports
// whether the tool flagged the result as an error; the text still holds
// the error payload in that case so callers can print it.
type CallResult struct {
	Text    string
	IsError bool
}

// dispatchCall drives one tools/call through the server's HandleMessage
// chokepoint and parses the MCP result. Every safety, audit, profile,
// and two-stage behavior runs inside HandleMessage, so this function adds
// nothing but request framing and result extraction.
//
// A JSON-RPC-level error (ErrRPCError) or a missing result (ErrNoResult)
// is returned as an error; a tool-level error surfaces as a CallResult
// with IsError true, not as a Go error, so the caller can map it to exit
// code 1 and still print the payload.
func dispatchCall(
	ctx context.Context,
	srv *server.Server,
	tool string,
	args map[string]any,
) (CallResult, error) {
	message, err := buildCallMessage(tool, args)
	if err != nil {
		return CallResult{}, err
	}

	raw, err := json.Marshal(srv.HandleMessage(ctx, message))
	if err != nil {
		return CallResult{}, fmt.Errorf("marshal dispatch response: %w", err)
	}

	return parseCallResult(raw)
}

// parseCallResult walks the JSON-RPC response bytes into a CallResult.
// Split out from dispatchCall so it is unit-testable against canned
// response payloads without standing up a server.
func parseCallResult(raw []byte) (CallResult, error) {
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return CallResult{}, fmt.Errorf("unmarshal dispatch response: %w", err)
	}

	if rpcErr, isMap := decoded[keyError].(map[string]any); isMap {
		msg, _ := rpcErr[keyMessage].(string)

		return CallResult{}, fmt.Errorf("%w: %s", ErrRPCError, msg)
	}

	result, isMap := decoded[keyResult].(map[string]any)
	if !isMap {
		return CallResult{}, ErrNoResult
	}

	isError, _ := result[keyIsError].(bool)

	return CallResult{
		Text:    joinContentText(result),
		IsError: isError,
	}, nil
}

// joinContentText concatenates the text fields of a result's content
// blocks. Tool results carry a single text block in practice, but the
// MCP shape allows several, so all text blocks are joined with newlines
// to avoid silently dropping any.
func joinContentText(result map[string]any) string {
	content, isSlice := result[keyContent].([]any)
	if !isSlice {
		return ""
	}

	parts := make([]string, 0, len(content))

	for _, block := range content {
		entry, isMap := block.(map[string]any)
		if !isMap {
			continue
		}

		if text, isString := entry[keyText].(string); isString {
			parts = append(parts, text)
		}
	}

	return strings.Join(parts, "\n")
}
