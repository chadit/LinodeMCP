package linode_test

// Shared string constants for the linode package's external test files.
// Extracted to satisfy goconst across client_test.go, errors_test.go,
// retry_test.go, and retry_wrappers_test.go.
const (
	// JSON pagination keys returned by the Linode API.
	keyData        = "data"
	keyDescription = "description"
	keyExample     = "example"
	keyPage        = "page"
	keyPages       = "pages"
	keyResults     = "results"

	errTemporaryFailure         = "temporary failure"
	imageShareGroupTokenCreated = "2025-08-04T10:09:09"

	// Common JSON field names used in test fixtures.
	keyID        = "id"
	keyLabel     = "label"
	keyStatus    = "status"
	keySSHKey    = "ssh_key"
	keyType      = "type"
	keyToken     = "token"
	keyDomain    = "domain"
	keyIsDefault = "is_default"
	keyLastFour  = "last_four"

	// Account settings fixture values.
	maintenancePolicyMigrate = "linode/migrate"

	// Account beta fixture values.
	betaExampleOpen      = "example_open"
	labelExampleOpenBeta = "Example Open Beta"

	// Child account fixture values.
	childAccountEUUID = "A1BC2DEF-34GH-567I-J890KLMN12O34P56"
	companyAcme       = "Acme"
	accountInvoiceID  = 12345

	// Common region IDs used in API response fixtures.
	regionUSEast         = "us-east"
	serviceLinodes       = "Linodes"
	serviceNodeBalancers = "NodeBalancers"

	// fwLabelNew is the firewall label reused across the retry-wrapper tests.
	fwLabelNew                  = "new-fw"
	paymentMethodCreditCard     = "credit_card"
	paymentMethodToken          = "tok_123"
	domainExample               = "example.com"
	remoteNameserverExample     = "ns1.example.com"
	shareGroupUUIDFixture       = "e1d0e58b-f89f-4237-84ab-b82077342359"
	shareGroupLabelFixture      = "DevOps Base Images"
	shareGroupTokenUUIDFixture  = "sharegroup-token-uuid-fixture"
	shareGroupTokenValueFixture = "sample-sharegroup-membership-token"
)
