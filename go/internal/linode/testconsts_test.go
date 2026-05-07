package linode_test

// Shared string constants for the linode package's external test files.
// Extracted to satisfy goconst across client_test.go, errors_test.go,
// retry_test.go, and retry_wrappers_test.go.
const (
	// JSON pagination keys returned by the Linode API.
	keyData    = "data"
	keyPage    = "page"
	keyPages   = "pages"
	keyResults = "results"

	// Common JSON field names used in test fixtures.
	keyID    = "id"
	keyLabel = "label"
)
