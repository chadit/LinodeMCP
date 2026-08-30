package linode_test

// Shared string constants for the linode package's external test files.
// Extracted to satisfy goconst across client_test.go, errors_test.go, and
// retry_test.go.
const (
	// JSON pagination keys returned by the Linode API.
	keyData             = "data"
	keyDescription      = "description"
	keyPage             = "page"
	keyResults          = "results"
	nodeLabelWeb1       = "web-1"
	keyErrors           = "errors"
	keyReason           = "reason"
	errTemporaryFailure = "temporary failure"

	// Common JSON field names used in test fixtures.
	keyID         = "id"
	keyLabel      = "label"
	keyRegion     = "region"
	domainExample = "example.com"
)

// Fixture labels and endpoint paths extracted so inline comparisons across the
// external test files don't trip goconst.
const (
	authHeaderTestToken = "Bearer test-token"

	// Well-formed JSON bodies that are not objects, served by the response-shape
	// contract tests.
	jsonBodyArray = "[]"
)

// Repeated literals extracted to satisfy goconst.
const (
	tcApplicationJSON              = "application/json"
	tcPage2PageSize50              = "page=2&page_size=50"
	tcRecovered                    = "recovered"
	tcSupportTickets123Attachments = "/support/tickets/123/attachments"
	tcTestuser                     = "testuser"
)
