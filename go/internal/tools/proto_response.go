package tools

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// MarshalProtoToolResponse serializes a proto message with the canonical
// options (snake_case field names, default values emitted) and wraps it in an
// MCP text result. Proto-backed tools use this so their output is byte-identical
// to the Python implementation, which serializes the same message with the
// matching MessageToJson options.
func MarshalProtoToolResponse(msg proto.Message) (*mcp.CallToolResult, error) {
	data, err := protojson.MarshalOptions{
		UseProtoNames:     true,
		EmitDefaultValues: true,
		Indent:            "  ",
	}.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal proto response: %w", err)
	}

	return mcp.NewToolResultText(string(data)), nil
}
