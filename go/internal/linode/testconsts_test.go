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
	keyID     = "id"
	keyLabel  = "label"
	keySSHKey = "ssh_key"

	// Account beta fixture values.
	betaExampleOpen      = "example_open"
	labelExampleOpenBeta = "Example Open Beta"

	// Child account fixture values.
	childAccountEUUID = "A1BC2DEF-34GH-567I-J890KLMN12O34P56"
	companyAcme       = "Acme"

	// Common region IDs used in API response fixtures.
	regionUSEast         = "us-east"
	serviceLinodes       = "Linodes"
	serviceNodeBalancers = "NodeBalancers"

	// fwLabelNew is the firewall label reused across the retry-wrapper tests.
	fwLabelNew = "new-fw"
)
