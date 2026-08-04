// Package toolhooks holds the per-tool logic the proto contract cannot express,
// such as a check whose wording predates the contract or a body that flattens
// one argument into several. Generated tool factories call into it by name: a
// name declared in docs/contracts/tool-hooks.txt with no implementation here
// fails the build, and the reverse fails TestManifestNamesEveryDeclaredHook.
package toolhooks

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// Validate checks a tool's arguments before anything else runs, returning the
// message the tool reports or "" to continue. Generated handlers wrap it in
// mcp.NewToolResultError, so a bad argument reads as a result the model can
// correct rather than a transport failure.
type Validate func(request *mcp.CallToolRequest) string

// DomainRecordGetValidate checks the two ids linode_domain_record_get takes.
// Its "must be a positive integer" wording diverges from the "is required" a
// required path field derives, and both sentences are pinned across languages by
// docs/contracts/message-parity-baseline.txt, so change it only deliberately.
func DomainRecordGetValidate(request *mcp.CallToolRequest) string {
	if request.GetInt("domain_id", 0) <= 0 {
		return "domain_id must be a positive integer"
	}

	if request.GetInt("record_id", 0) <= 0 {
		return "record_id must be a positive integer"
	}

	return ""
}
