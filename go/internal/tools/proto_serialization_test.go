package tools_test

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/types/known/anypb"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// protoSerializationWidenedJSON is the canonical form of the SQLite health
// message: every 64-bit counter arrives from protojson as a quoted string and
// comes back out as a JSON number, negatives included.
const protoSerializationWidenedJSON = `{
  "path": "",
  "event_count": 42,
  "oldest_event_unix_ns": -1234567890123,
  "db_bytes": 0
}`

// protoSerializationFreeformJSON is the same message wrapped in an Any. Every
// key in an Any body, including the "@type" discriminator protojson injects,
// is absent from the Any descriptor, so the walk treats the whole subtree as
// free-form and leaves the quoted 64-bit values alone.
const protoSerializationFreeformJSON = `{
  "@type": "type.googleapis.com/linode.mcp.v1.AuditHealthSQLite",
  "path": "",
  "event_count": "42",
  "oldest_event_unix_ns": "-1234567890123",
  "db_bytes": "0"
}`

// protoSerializationStructFailMessage is the prefix a caller hands
// MarshalStructToolResponse for the conversion failure.
const protoSerializationStructFailMessage = "failed to convert engine config"

// sqliteHealthFixture builds the health message both serialization tests use.
// The negative oldest-event stamp stands in for a row written under a skewed
// clock: the serializer has to widen it like any other 64-bit value rather than
// leave it quoted or drop the sign.
func sqliteHealthFixture() *linodev1.AuditHealthSQLite {
	return &linodev1.AuditHealthSQLite{
		EventCount:        42,
		OldestEventUnixNs: -1234567890123,
	}
}

// TestMarshalProtoJSONWidensNegativeInt64 pins the widening pass on a message
// whose 64-bit fields include a negative value. protojson quotes all five
// 64-bit kinds; this project's output contract wants them as JSON numbers, and
// the sign has to survive the rewrite.
func TestMarshalProtoJSONWidensNegativeInt64(t *testing.T) {
	t.Parallel()

	got, err := tools.MarshalProtoJSON(sqliteHealthFixture())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(got) != protoSerializationWidenedJSON {
		t.Errorf("MarshalProtoJSON() = %s, want %s", got, protoSerializationWidenedJSON)
	}
}

// TestMarshalProtoJSONLeavesUnschemedKeysQuoted covers the fallback the walk
// takes when a JSON key has no matching field on the message descriptor. An Any
// body is the reachable case: protojson flattens the wrapped message's fields
// next to an "@type" key, none of which the Any descriptor declares, so nothing
// in that subtree gets widened.
func TestMarshalProtoJSONLeavesUnschemedKeysQuoted(t *testing.T) {
	t.Parallel()

	wrapped, err := anypb.New(sqliteHealthFixture())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := tools.MarshalProtoJSON(wrapped)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(got) != protoSerializationFreeformJSON {
		t.Errorf("MarshalProtoJSON() = %s, want %s", got, protoSerializationFreeformJSON)
	}
}

// TestMarshalProtoToolResponseReportsMarshalFailure drives the tool-result
// wrapper down its error path. An Any naming a type that is not in the registry
// cannot be marshaled, and the wrapper has to surface that as an error rather
// than an empty text result a client would read as a successful call.
func TestMarshalProtoToolResponseReportsMarshalFailure(t *testing.T) {
	t.Parallel()

	unresolvable := &anypb.Any{TypeUrl: "type.googleapis.com/linode.mcp.v1.NoSuchMessage"}

	result, err := tools.MarshalProtoToolResponse(unresolvable)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

// TestMarshalStructToolResponseReportsConversionFailure covers the free-form
// serializer's guard. A value structpb cannot represent has to come back as an
// error result carrying the caller's prefix, not as a Go error: the tool call
// itself succeeded, the payload is what went wrong.
func TestMarshalStructToolResponseReportsConversionFailure(t *testing.T) {
	t.Parallel()

	result, err := tools.MarshalStructToolResponse(
		map[string]any{"stream": make(chan int)},
		protoSerializationStructFailMessage,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	// The prefix is the contract; the tail is protobuf's own wording, which
	// carries a non-breaking space precisely so callers do not match it exactly.
	wantPrefix := protoSerializationStructFailMessage + ": "
	if !strings.HasPrefix(textContent.Text, wantPrefix) {
		t.Errorf("textContent.Text = %v, want it to start with %v", textContent.Text, wantPrefix)
	}

	if !strings.Contains(textContent.Text, "chan int") {
		t.Errorf("textContent.Text = %v, want it to name the unconvertible type", textContent.Text)
	}
}
