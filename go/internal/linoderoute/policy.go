package linoderoute

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// RetryDisabled reports whether the named tool's route must not be replayed
// after a transient failure. A create whose id the API assigns is the case the
// option exists for: a replayed attempt leaves a second resource the caller
// never learns about and is billed for.
//
// It is read here rather than passed in at each call site because the fact is
// the contract's, and a routed primitive that took it as an argument would let
// one call site retry a route another one refuses to. An unknown tool answers
// false, which is the retrying policy: the route resolution that follows fails
// on the same name, so nothing reaches the API on the strength of this answer.
func RetryDisabled(tool string) bool {
	var disabled bool

	walkMessages(func(message protoreflect.MessageDescriptor) bool {
		declared, isRoute := proto.GetExtension(
			message.Options(), linodev1.E_ToolRoute,
		).(*linodev1.ToolRoute)
		if !isRoute || declared.GetTool() != tool {
			return true
		}

		disabled, _ = proto.GetExtension(
			message.Options(), linodev1.E_RetryDisabled,
		).(bool)

		return false
	})

	return disabled
}
