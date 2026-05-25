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
	keyErrors      = "errors"
	keyReason      = "reason"

	errTemporaryFailure         = "temporary failure"
	errNotFound                 = "not found"
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
	fwLabelNew                   = "new-fw"
	paymentMethodCreditCard      = "credit_card"
	paymentMethodToken           = "tok_123"
	domainExample                = "example.com"
	remoteNameserverExample      = "ns1.example.com"
	shareGroupUUIDFixture        = "e1d0e58b-f89f-4237-84ab-b82077342359"
	shareGroupLabelFixture       = "DevOps Base Images"
	shareGroupDescriptionFixture = "shared CI images"
	shareGroupUUIDExample        = "1533863e-16a4-47b5-b829-ac0f35c13278"
	shareGroupCreatedFixture     = "2025-04-14T22:44:02"
	shareGroupUpdatedFixture     = "2025-04-15T22:44:02"
	shareGroupTokenUUIDFixture   = "sharegroup-token-uuid-fixture"
	shareGroupTokenValueFixture  = "sample-sharegroup-membership-token"
	memberLabelFixture           = "Engineering"
	memberTokenFixture           = "member-token"
	imageLinuxDebianFixture      = "Linux Debian"
	privateImage15Fixture        = "private/15"
	privateImage123Fixture       = "private/123"
	privateImage12345Fixture     = "private/12345"
	sharedImage1Fixture          = "shared/1"
	imageStatusAvailableFixture  = "available"
	typeManualImage              = "manual"
	keyCreated                   = "created"
	labelBootConfig              = "boot-config"
	configDeviceSlotSDA          = "sda"
	configKernelLatest           = "linode/latest-64bit"
	purposePublic                = "public"
	purposeVPC                   = "vpc"
	macAddressFixture            = "22:00:AB:CD:EF:01"
	pathTraversalDotDot          = ".."
	lkeVersion129                = "1.29"
	lkeVersionWithSlash          = "1.29/edge"
)
