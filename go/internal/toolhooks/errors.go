package toolhooks

import "errors"

// ErrTicketIDNotAnInt reports a path value the emitter resolved as something
// other than the ticket id the attachment client method takes, which is a
// contract defect rather than a bad call. Exported so the test that pins the
// refusal can match it by identity.
var ErrTicketIDNotAnInt = errors.New("support ticket attachment: ticket_id resolved to a non-integer path value")

// ErrOAuthClientIDNotText reports a path value the emitter resolved as
// something other than the client id the thumbnail client method takes, which
// is a contract defect rather than a bad call. Exported so the test that pins
// the refusal can match it by identity.
var ErrOAuthClientIDNotText = errors.New("oauth client thumbnail: client_id resolved to a non-string path value")

// The two reads a resize plan is built from. A plan hashes the instance and its
// disks together, so a failure in either leaves a projection that would compare
// unequal against a whole one at apply time and refuse a resource nothing
// changed. Exported so the tests that pin each refusal match it by identity.
var (
	ErrResizeInstanceRead = errors.New("get instance for resize plan")
	ErrResizeDiskList     = errors.New("list disks for resize plan")
)
