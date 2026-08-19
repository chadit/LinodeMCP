package linode_test

// Shared string constants for the linode package's external test files.
// Extracted to satisfy goconst across client_test.go, errors_test.go,
// retry_test.go, and retry_wrappers_test.go.
const (
	// JSON pagination keys returned by the Linode API.
	keyData            = "data"
	keyDescription     = "description"
	keyPage            = "page"
	keyPages           = "pages"
	keyResults         = "results"
	keyPageSize        = "page_size"
	keyNodeBalancerID  = "nodebalancer_id"
	keyConfigID        = "config_id"
	keyPort            = "port"
	keyProtocol        = "protocol"
	keyAlgorithm       = "algorithm"
	protocolHTTPS      = "https"
	valueRoundRobin    = "roundrobin"
	keyAddress         = "address"
	nodeLabelWeb1      = "web-1"
	keyErrors          = "errors"
	keyReason          = "reason"
	keyClientsIncluded = "clients_included"
	keyPrice           = "price"
	keyHourly          = "hourly"
	keyMonthly         = "monthly"

	errTemporaryFailure   = "temporary failure"
	longviewClientCreated = "2018-01-01T00:01:01"
	longviewClientUpdated = "2018-01-02T00:01:01"
	errNotFound           = "not found"

	// Common JSON field names used in test fixtures.
	keyID          = "id"
	keyLabel       = "label"
	keyStatus      = "status"
	keySSHKey      = "ssh_key"
	keyType        = "type"
	keyServiceType = "service_type"
	keyLastFour    = "last_four"
	// Account settings fixture values.
	maintenancePolicyMigrate = "linode/migrate"

	// Child account fixture values.
	childAccountEUUID = "A1BC2DEF-34GH-567I-J890KLMN12O34P56"
	companyAcme       = "Acme"

	// Common region IDs used in API response fixtures.
	regionUSEast               = "us-east"
	vlanLabelApp               = "app-vlan"
	ipv6RangeFixture           = "2001:0db8::"
	ipv6RangeCIDR              = "2001:0db8::/64"
	ipv6RouteTarget            = "2001:db8::1"
	nodeBalancerNodeAddress    = "192.0.2.10:80"
	nodeBalancerNodeModeAccept = "accept"
	nodeBalancerNodeStatusUP   = "UP"
	keyWeight                  = "weight"

	paymentMethodCreditCard      = "credit_card"
	domainExample                = "example.com"
	shareGroupDescriptionFixture = "shared CI images"
	shareGroupUUIDExample        = "1533863e-16a4-47b5-b829-ac0f35c13278"
	shareGroupCreatedFixture     = "2025-04-14T22:44:02"
	shareGroupUpdatedFixture     = "2025-04-15T22:44:02"
	imageLinuxDebianFixture      = "Linux Debian"
	privateImage15Fixture        = "private/15"
	imageStatusAvailableFixture  = "available"
	statusEnabledFixture         = "enabled"
	typeManualImage              = "manual"
	keyCreated                   = "created"
	keyUpdated                   = "updated"
	labelBootConfig              = "boot-config"
	configDeviceSlotSDA          = "sda"
	configKernelLatest           = "linode/latest-64bit"
	purposePublic                = "public"
	purposeVPC                   = "vpc"
	policyDrop                   = "DROP"
	policyAccept                 = "ACCEPT"
	protocolTCP                  = "TCP"
	firewallRuleLabelAllowHTTPS  = "allow-https"
	pathTraversalDotDot          = ".."
)

// Fixture labels and endpoint paths extracted so inline comparisons across the
// external test files don't trip goconst.
const (
	dataVolumeLabel = "data-volume"

	endpointInstanceFirewallsPath = "/linode/instances/123/firewalls"

	authHeaderTestToken = "Bearer test-token"

	// Well-formed JSON bodies that are not objects, served by the response-shape
	// contract tests.
	jsonBodyArray = "[]"
)

// Repeated literals extracted to satisfy goconst.
const (
	tcABCDEF02                                       = "22:00:AB:CD:EF:02"
	tcAccountChildAccountsA1BC2DEF34GH567IJ890KLMN12 = "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56"
	tcAccountEvents123                               = "/account/events/123"
	tcAccountOauthClientsClient2F1233Fquery          = "/account/oauth-clients/client%2F123%3Fquery"
	tcAccountPaymentMethods123                       = "/account/payment-methods/123"
	tcAccountServiceTransfersServiceTokenExample     = "/account/service-transfers/service-token-example"
	tcAccountSettings                                = "/account/settings"
	tcAccountUsersUser2Fname3Fquery                  = "/account/users/user%2Fname%3Fquery"
	tcApplicationJSON                                = "application/json"
	tcDeployBase                                     = "deploy-base"
	tcImagesSharegroups123                           = "/images/sharegroups/123"
	tcLinodeCreate                                   = "linode_create"
	tcLinodeInstances123Configs                      = "/linode/instances/123/configs"
	tcLinodeInstances123Configs456                   = "/linode/instances/123/configs/456"
	tcLinodeInstances123Configs789Interfaces456      = "/linode/instances/123/configs/789/interfaces/456"
	tcLinodeInstances123Interfaces456                = "/linode/instances/123/interfaces/456"
	tcLinodeInstances123Volumes                      = "/linode/instances/123/volumes"
	tcLinodeStackscripts123                          = "/linode/stackscripts/123"
	tcLit                                            = "11/2024"
	tcLongviewClients789                             = "/longview/clients/789"
	tcNodebalancers123Configs456Nodes                = "/nodebalancers/123/configs/456/nodes"
	tcNodebalancers123Configs456Nodes789             = "/nodebalancers/123/configs/456/nodes/789"
	tcPGMiamiFailover                                = "PG_Miami_failover"
	tcPage2PageSize50                                = "page=2&page_size=50"
	tcProfileTokens12345                             = "/profile/tokens/12345"
	tcRecovered                                      = "recovered"
	tcSupportTickets123Attachments                   = "/support/tickets/123/attachments"
	tcTagsProd                                       = "/tags/prod"
	tcTestuser                                       = "testuser"
	tcTokenQueryFrag                                 = "token/..?query#frag"
	tcUserNameQuery                                  = "user/name?query"
)
