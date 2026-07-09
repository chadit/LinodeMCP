package tools

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// MarshalProtoToolResponse serializes a proto message with the canonical
// options (snake_case field names, default values emitted) and wraps it in an
// MCP text result. Proto-backed tools use this so their output is byte-identical
// to the Python implementation, which serializes the same message with the
// matching MessageToJson options.
func MarshalProtoToolResponse(msg proto.Message) (*mcp.CallToolResult, error) {
	data, err := MarshalProtoJSON(msg)
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(string(data)), nil
}

// MarshalProtoJSON serializes a proto message with the canonical options and
// returns the raw JSON bytes. MarshalProtoToolResponse wraps this for MCP tool
// output; the CLI version subcommand marshals the same VersionResponse message
// with it so the tool and the subcommand emit identical bytes. After the
// protojson marshal, 64-bit integer fields are widened back to JSON numbers
// (see widenInt64JSON) and the result is re-indented; json.Indent also gives
// stable whitespace where protojson randomizes the space after a colon.
func MarshalProtoJSON(msg proto.Message) ([]byte, error) {
	data, err := protojson.MarshalOptions{
		UseProtoNames:     true,
		EmitDefaultValues: true,
	}.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal proto response: %w", err)
	}

	widened, err := widenInt64JSON(data, msg.ProtoReflect().Descriptor())
	if err != nil {
		return nil, fmt.Errorf("failed to widen 64-bit fields: %w", err)
	}

	var indented bytes.Buffer
	if err := json.Indent(&indented, widened, "", "  "); err != nil {
		return nil, fmt.Errorf("failed to indent proto response: %w", err)
	}

	return indented.Bytes(), nil
}

// MarshalStructToolResponse serializes a free-form map through a bare
// google.protobuf.Struct and marshals it with MarshalProtoToolResponse. Read
// tools whose API response is an open-ended object (engine config descriptors,
// profile preferences, managed stats) use this so both languages emit the same
// deterministically key-sorted object; protojson sorts Struct keys on both
// sides, replacing the pre-proto split where Go alpha-sorted map keys and Python
// preserved the API's insertion order. failMessage prefixes a conversion error.
func MarshalStructToolResponse(raw map[string]any, failMessage string) (*mcp.CallToolResult, error) {
	structVal, err := structpb.NewStruct(raw)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("%s: %v", failMessage, err)), nil
	}

	return MarshalProtoToolResponse(structVal)
}
