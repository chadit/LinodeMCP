package linode_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// clientRouteProbeKey is the member the generic probe bodies carry when the
// decoded type has no scalar field of its own to read back.
const clientRouteProbeKey = "probe"

// Response bodies the route cases serve. Each one carries a single probe value
// so the case can prove the client decoded the payload rather than handing back
// a zero value.
const (
	clientRouteACLEnvelope                  = "{\"acl\":{\"enabled\":true}}"
	clientRouteEmptyObject                  = "{}"
	clientRouteFirewallSnapshot             = "{\"id\":4242,\"label\":\"probe-value\",\"rules\":{\"version\":7}}"
	clientRouteObjACL                       = "{\"acl\":\"probe-value\"}"
	clientRouteObjAddress                   = "{\"address\":\"probe-value\"}"
	clientRouteObjComments                  = "{\"comments\":\"probe-value\"}"
	clientRouteObjCreated                   = "{\"created\":\"probe-value\"}"
	clientRouteObjEmail                     = "{\"email\":\"probe-value\"}"
	clientRouteObjEnabledBool               = "{\"enabled\":true}"
	clientRouteObjEuuid                     = "{\"euuid\":\"probe-value\"}"
	clientRouteObjHypervisor                = "{\"hypervisor\":\"probe-value\"}"
	clientRouteObjID                        = "{\"id\":\"probe-value\"}"
	clientRouteObjInboundPolicy             = "{\"inbound_policy\":\"probe-value\"}"
	clientRouteObjInterfacesForNewLinodes   = "{\"interfaces_for_new_linodes\":\"probe-value\"}"
	clientRouteObjLabel                     = "{\"label\":\"probe-value\"}"
	clientRouteObjMacAddress                = "{\"mac_address\":\"probe-value\"}"
	clientRouteObjName                      = "{\"name\":\"probe-value\"}"
	clientRouteObjProbe                     = "{\"probe\":\"probe-value\"}"
	clientRouteObjProtocol                  = "{\"protocol\":\"probe-value\"}"
	clientRouteObjPurpose                   = "{\"purpose\":\"probe-value\"}"
	clientRouteObjRange                     = "{\"range\":\"probe-value\"}"
	clientRouteObjSSLBool                   = "{\"ssl\":true}"
	clientRouteObjStatus                    = "{\"status\":\"probe-value\"}"
	clientRouteObjType                      = "{\"type\":\"probe-value\"}"
	clientRouteObjUUID                      = "{\"uuid\":\"probe-value\"}"
	clientRouteObjUsername                  = "{\"username\":\"probe-value\"}"
	clientRouteObjVersion                   = "{\"version\":\"probe-value\"}"
	clientRouteObjZip                       = "{\"zip\":\"probe-value\"}"
	clientRoutePageAddress                  = "{\"data\":[{\"address\":\"probe-value\"},{\"address\":\"second-value\"}],\"page\":1}"
	clientRoutePageCipherSuite              = "{\"data\":[{\"cipher_suite\":\"probe-value\"},{\"cipher_suite\":\"second-value\"}],\"page\":1}"
	clientRoutePageComments                 = "{\"data\":[{\"comments\":\"probe-value\"},{\"comments\":\"second-value\"}],\"page\":1}"
	clientRoutePageCreated                  = "{\"data\":[{\"created\":\"probe-value\"},{\"created\":\"second-value\"}],\"page\":1}"
	clientRoutePageLabel                    = "{\"data\":[{\"label\":\"probe-value\"},{\"label\":\"second-value\"}],\"page\":1}"
	clientRoutePageProtocol                 = "{\"data\":[{\"protocol\":\"probe-value\"},{\"protocol\":\"second-value\"}],\"page\":1}"
	clientRoutePageType                     = "{\"data\":[{\"type\":\"probe-value\"},{\"type\":\"second-value\"}],\"page\":1}"
	clientRouteProtoArrayPurpose            = "[{\"purpose\":\"probe-value\"},{\"purpose\":\"second-value\"}]"
	clientRouteProtoArrayRegion             = "[{\"region\":\"probe-value\"},{\"region\":\"second-value\"}]"
	clientRouteProtoKeyedInterfacesQuestion = "{\"interfaces\":[{\"question\":\"probe-value\"},{\"question\":\"second-value\"}]}"
	clientRouteProtoObjACL                  = "{\"acl\":\"probe-value\"}"
	clientRouteProtoObjAction               = "{\"action\":\"probe-value\"}"
	clientRouteProtoObjActiveSince          = "{\"activeSince\":\"probe-value\"}"
	clientRouteProtoObjAddress              = "{\"address\":\"probe-value\"}"
	clientRouteProtoObjBackupsEnabledBool   = "{\"backupsEnabled\":true}"
	clientRouteProtoObjBillableInt32        = "{\"billable\":4242}"
	clientRouteProtoObjBillingAgreementBool = "{\"billingAgreement\":true}"
	clientRouteProtoObjCaCertificate        = "{\"caCertificate\":\"probe-value\"}"
	clientRouteProtoObjClass                = "{\"class\":\"probe-value\"}"
	clientRouteProtoObjClientsIncludedInt32 = "{\"clientsIncluded\":4242}"
	clientRouteProtoObjClosableBool         = "{\"closable\":true}"
	clientRouteProtoObjCreated              = "{\"created\":\"probe-value\"}"
	clientRouteProtoObjDatetime             = "{\"datetime\":\"probe-value\"}"
	clientRouteProtoObjEmail                = "{\"email\":\"probe-value\"}"
	clientRouteProtoObjEnabledBool          = "{\"enabled\":true}"
	clientRouteProtoObjEnrolled             = "{\"enrolled\":\"probe-value\"}"
	clientRouteProtoObjFirstName            = "{\"firstName\":\"probe-value\"}"
	clientRouteProtoObjID                   = "{\"id\":\"probe-value\"}"
	clientRouteProtoObjIDInt32              = "{\"id\":4242}"
	clientRouteProtoObjInboundPolicy        = "{\"inboundPolicy\":\"probe-value\"}"
	clientRouteProtoObjKubeconfig           = "{\"kubeconfig\":\"probe-value\"}"
	clientRouteProtoObjLabel                = "{\"label\":\"probe-value\"}"
	clientRouteProtoObjMessage              = "{\"message\":\"probe-value\"}"
	clientRouteProtoObjQuotaID              = "{\"quotaId\":\"probe-value\"}"
	clientRouteProtoObjRange                = "{\"range\":\"probe-value\"}"
	clientRouteProtoObjRegion               = "{\"region\":\"probe-value\"}"
	clientRouteProtoObjSSHKey               = "{\"sshKey\":\"probe-value\"}"
	clientRouteProtoObjSSLBool              = "{\"ssl\":true}"
	clientRouteProtoObjSecret               = "{\"secret\":\"probe-value\"}"
	clientRouteProtoObjSlug                 = "{\"slug\":\"probe-value\"}"
	clientRouteProtoObjSurveyLink           = "{\"surveyLink\":\"probe-value\"}"
	clientRouteProtoObjTitle                = "{\"title\":\"probe-value\"}"
	clientRouteProtoObjToken                = "{\"token\":\"probe-value\"}"
	clientRouteProtoObjTokenUUID            = "{\"tokenUuid\":\"probe-value\"}"
	clientRouteProtoObjURL                  = "{\"url\":\"probe-value\"}"
	clientRouteProtoObjUsername             = "{\"username\":\"probe-value\"}"
	clientRouteProtoPageAction              = "{\"data\":[{\"action\":\"probe-value\"},{\"action\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageActiveSince         = "{\"data\":[{\"activeSince\":\"probe-value\"},{\"activeSince\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageAddress             = "{\"data\":[{\"address\":\"probe-value\"},{\"address\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageClass               = "{\"data\":[{\"class\":\"probe-value\"},{\"class\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageCreated             = "{\"data\":[{\"created\":\"probe-value\"},{\"created\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageDate                = "{\"data\":[{\"date\":\"probe-value\"},{\"date\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageDatetime            = "{\"data\":[{\"datetime\":\"probe-value\"},{\"datetime\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageDescription         = "{\"data\":[{\"description\":\"probe-value\"},{\"description\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageDomain              = "{\"data\":[{\"domain\":\"probe-value\"},{\"domain\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageEmail               = "{\"data\":[{\"email\":\"probe-value\"},{\"email\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageEndpoint            = "{\"data\":[{\"endpoint\":\"probe-value\"},{\"endpoint\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageEnrolled            = "{\"data\":[{\"enrolled\":\"probe-value\"},{\"enrolled\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageFrom                = "{\"data\":[{\"from\":\"probe-value\"},{\"from\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageID                  = "{\"data\":[{\"id\":\"probe-value\"},{\"id\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageIpv4Range           = "{\"data\":[{\"ipv4Range\":\"probe-value\"},{\"ipv4Range\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageLabel               = "{\"data\":[{\"label\":\"probe-value\"},{\"label\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageName                = "{\"data\":[{\"name\":\"probe-value\"},{\"name\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageProtocol            = "{\"data\":[{\"protocol\":\"probe-value\"},{\"protocol\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageQuotaID             = "{\"data\":[{\"quotaId\":\"probe-value\"},{\"quotaId\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageRange               = "{\"data\":[{\"range\":\"probe-value\"},{\"range\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageReason              = "{\"data\":[{\"reason\":\"probe-value\"},{\"reason\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageRegion              = "{\"data\":[{\"region\":\"probe-value\"},{\"region\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageSlug                = "{\"data\":[{\"slug\":\"probe-value\"},{\"slug\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageStatus              = "{\"data\":[{\"status\":\"probe-value\"},{\"status\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageToken               = "{\"data\":[{\"token\":\"probe-value\"},{\"token\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageTokenUUID           = "{\"data\":[{\"tokenUuid\":\"probe-value\"},{\"tokenUuid\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageType                = "{\"data\":[{\"type\":\"probe-value\"},{\"type\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageUUID                = "{\"data\":[{\"uuid\":\"probe-value\"},{\"uuid\":\"second-value\"}],\"page\":1}"
	clientRouteProtoPageUsername            = "{\"data\":[{\"username\":\"probe-value\"},{\"username\":\"second-value\"}],\"page\":1}"
	clientRouteSecurityQuestionsEnvelope    = "{\"security_questions\":[{\"question\":\"probe-value\"},{\"question\":\"second-value\"}]}"
)

// Request paths and probe results the cases below share.
const (
	clientRouteIPv4Fixture                                      = "203.0.113.5"
	clientRoutePathAccount                                      = "/account"
	clientRoutePathAccountAgreements                            = "/account/agreements"
	clientRoutePathAccountBetas                                 = "/account/betas"
	clientRoutePathAccountChildAccountsAlpha                    = "/account/child-accounts/alpha"
	clientRoutePathAccountEvents4242                            = "/account/events/4242"
	clientRoutePathAccountOauthClients                          = "/account/oauth-clients"
	clientRoutePathAccountOauthClientsAlpha                     = "/account/oauth-clients/alpha"
	clientRoutePathAccountPaymentMethodsAlpha                   = "/account/payment-methods/alpha"
	clientRoutePathAccountServiceTransfers                      = "/account/service-transfers"
	clientRoutePathAccountServiceTransfersAlpha                 = "/account/service-transfers/alpha"
	clientRoutePathAccountSettings                              = "/account/settings"
	clientRoutePathAccountUsers                                 = "/account/users"
	clientRoutePathAccountUsersAlpha                            = "/account/users/alpha"
	clientRoutePathDatabasesMysqlInstances                      = "/databases/mysql/instances"
	clientRoutePathDatabasesMysqlInstances4242                  = "/databases/mysql/instances/4242"
	clientRoutePathDatabasesPostgresqlInstances4242             = "/databases/postgresql/instances/4242"
	clientRoutePathDomains                                      = "/domains"
	clientRoutePathDomains4242                                  = "/domains/4242"
	clientRoutePathDomains4242Records                           = "/domains/4242/records"
	clientRoutePathDomains4242Records8615                       = "/domains/4242/records/8615"
	clientRoutePathImages                                       = "/images"
	clientRoutePathImagesAlpha                                  = "/images/alpha"
	clientRoutePathImagesSharegroups                            = "/images/sharegroups"
	clientRoutePathImagesSharegroups4242                        = "/images/sharegroups/4242"
	clientRoutePathImagesSharegroups4242Images                  = "/images/sharegroups/4242/images"
	clientRoutePathImagesSharegroups4242Members                 = "/images/sharegroups/4242/members"
	clientRoutePathImagesSharegroups4242MembersAlpha            = "/images/sharegroups/4242/members/alpha"
	clientRoutePathImagesSharegroupsTokens                      = "/images/sharegroups/tokens"
	clientRoutePathImagesSharegroupsTokensAlpha                 = "/images/sharegroups/tokens/alpha"
	clientRoutePathImagesSharegroupsTokensAlphaSharegroup       = "/images/sharegroups/tokens/alpha/sharegroup"
	clientRoutePathLinodeInstances                              = "/linode/instances"
	clientRoutePathLinodeInstances4242                          = "/linode/instances/4242"
	clientRoutePathLinodeInstances4242Backups8615               = "/linode/instances/4242/backups/8615"
	clientRoutePathLinodeInstances4242Configs                   = "/linode/instances/4242/configs"
	clientRoutePathLinodeInstances4242Configs8615               = "/linode/instances/4242/configs/8615"
	clientRoutePathLinodeInstances4242Configs8615Interfaces     = "/linode/instances/4242/configs/8615/interfaces"
	clientRoutePathLinodeInstances4242Configs8615Interfaces1379 = "/linode/instances/4242/configs/8615/interfaces/1379"
	clientRoutePathLinodeInstances4242Disks                     = "/linode/instances/4242/disks"
	clientRoutePathLinodeInstances4242Disks8615                 = "/linode/instances/4242/disks/8615"
	clientRoutePathLinodeInstances4242Firewalls                 = "/linode/instances/4242/firewalls"
	clientRoutePathLinodeInstances4242Interfaces8615            = "/linode/instances/4242/interfaces/8615"
	clientRoutePathLinodeInstances4242IpsAlpha                  = "/linode/instances/4242/ips/alpha"
	clientRoutePathLinodeInstances4242Volumes                   = "/linode/instances/4242/volumes"
	clientRoutePathLinodeStackscripts                           = "/linode/stackscripts"
	clientRoutePathLinodeStackscripts4242                       = "/linode/stackscripts/4242"
	clientRoutePathLinodeTypesAlpha                             = "/linode/types/alpha"
	clientRoutePathLkeClusters                                  = "/lke/clusters"
	clientRoutePathLkeClusters4242                              = "/lke/clusters/4242"
	clientRoutePathLkeClusters4242ControlPlaneACL               = "/lke/clusters/4242/control_plane_acl"
	clientRoutePathLkeClusters4242Kubeconfig                    = "/lke/clusters/4242/kubeconfig"
	clientRoutePathLkeClusters4242NodesAlpha                    = "/lke/clusters/4242/nodes/alpha"
	clientRoutePathLkeClusters4242Pools                         = "/lke/clusters/4242/pools"
	clientRoutePathLkeClusters4242Pools8615                     = "/lke/clusters/4242/pools/8615"
	clientRoutePathLongviewClientsAlpha                         = "/longview/clients/alpha"
	clientRoutePathLongviewPlan                                 = "/longview/plan"
	clientRoutePathManagedContacts                              = "/managed/contacts"
	clientRoutePathManagedContacts4242                          = "/managed/contacts/4242"
	clientRoutePathManagedCredentials                           = "/managed/credentials"
	clientRoutePathManagedCredentials4242                       = "/managed/credentials/4242"
	clientRoutePathManagedLinodeSettings4242                    = "/managed/linode-settings/4242"
	clientRoutePathManagedServices                              = "/managed/services"
	clientRoutePathManagedServices4242                          = "/managed/services/4242"
	clientRoutePathMonitorServicesAlphaAlertDefinitions         = "/monitor/services/alpha/alert-definitions"
	clientRoutePathMonitorServicesAlphaAlertDefinitions4242     = "/monitor/services/alpha/alert-definitions/4242"
	clientRoutePathNetworkingFirewalls                          = "/networking/firewalls"
	clientRoutePathNetworkingFirewalls4242                      = "/networking/firewalls/4242"
	clientRoutePathNetworkingFirewalls4242Devices               = "/networking/firewalls/4242/devices"
	clientRoutePathNetworkingFirewalls4242Devices8615           = "/networking/firewalls/4242/devices/8615"
	clientRoutePathNetworkingFirewalls4242Rules                 = "/networking/firewalls/4242/rules"
	clientRoutePathNetworkingIps                                = "/networking/ips"
	clientRoutePathNetworkingIps20301135                        = "/networking/ips/203.0.113.5"
	clientRoutePathNetworkingIpv6Ranges                         = "/networking/ipv6/ranges"
	clientRoutePathNetworkingIpv6Ranges20010db864               = "/networking/ipv6/ranges/2001:0db8::/64"
	clientRoutePathNetworkingReservedIps                        = "/networking/reserved/ips"
	clientRoutePathNetworkingReservedIps20301135                = "/networking/reserved/ips/203.0.113.5"
	clientRoutePathNetworkingVlans                              = "/networking/vlans"
	clientRoutePathNodebalancers                                = "/nodebalancers"
	clientRoutePathNodebalancers4242                            = "/nodebalancers/4242"
	clientRoutePathNodebalancers4242Configs                     = "/nodebalancers/4242/configs"
	clientRoutePathNodebalancers4242Configs8615                 = "/nodebalancers/4242/configs/8615"
	clientRoutePathNodebalancers4242Configs8615Nodes            = "/nodebalancers/4242/configs/8615/nodes"
	clientRoutePathNodebalancers4242Configs8615Nodes1379        = "/nodebalancers/4242/configs/8615/nodes/1379"
	clientRoutePathNodebalancers4242Firewalls                   = "/nodebalancers/4242/firewalls"
	clientRoutePathObjectStorageBuckets                         = "/object-storage/buckets"
	clientRoutePathObjectStorageBucketsAlphaBravo               = "/object-storage/buckets/alpha/bravo"
	clientRoutePathObjectStorageBucketsAlphaBravoAccess         = "/object-storage/buckets/alpha/bravo/access"
	clientRoutePathObjectStorageBucketsAlphaBravoObjectACL      = "/object-storage/buckets/alpha/bravo/object-acl"
	clientRoutePathObjectStorageBucketsAlphaBravoSSL            = "/object-storage/buckets/alpha/bravo/ssl"
	clientRoutePathObjectStorageKeys                            = "/object-storage/keys"
	clientRoutePathObjectStorageKeys4242                        = "/object-storage/keys/4242"
	clientRoutePathPlacementGroups                              = "/placement/groups"
	clientRoutePathPlacementGroups4242                          = "/placement/groups/4242"
	clientRoutePathProfile                                      = "/profile"
	clientRoutePathProfileApps4242                              = "/profile/apps/4242"
	clientRoutePathProfileDevices4242                           = "/profile/devices/4242"
	clientRoutePathProfilePhoneNumber                           = "/profile/phone-number"
	clientRoutePathProfilePhoneNumberVerify                     = "/profile/phone-number/verify"
	clientRoutePathProfileSecurityQuestions                     = "/profile/security-questions"
	clientRoutePathProfileSshkeys                               = "/profile/sshkeys"
	clientRoutePathProfileSshkeys4242                           = "/profile/sshkeys/4242"
	clientRoutePathProfileTokens                                = "/profile/tokens"
	clientRoutePathProfileTokens4242                            = "/profile/tokens/4242"
	clientRoutePathRegionsAlpha                                 = "/regions/alpha"
	clientRoutePathSupportTickets                               = "/support/tickets"
	clientRoutePathSupportTickets4242Replies                    = "/support/tickets/4242/replies"
	clientRoutePathTags                                         = "/tags"
	clientRoutePathTagsAlpha                                    = "/tags/alpha"
	clientRoutePathVolumes                                      = "/volumes"
	clientRoutePathVolumes4242                                  = "/volumes/4242"
	clientRoutePathVpcs                                         = "/vpcs"
	clientRoutePathVpcs4242                                     = "/vpcs/4242"
	clientRoutePathVpcs4242Subnets                              = "/vpcs/4242/subnets"
	clientRoutePathVpcs4242Subnets8615                          = "/vpcs/4242/subnets/8615"
	clientRouteProbeValue                                       = "probe-value"
	clientRouteTwoElementProbe                                  = "2:probe-value"
)

// clientRouteCase pins one linode.Client method to the request it puts on the
// wire and the value it decodes back out of the response.
//
// Every Client method funnels through the same makeRequest/handleResponse pair,
// so covering them one at a time says little that the next method does not
// repeat. What is worth pinning is the part that differs: the verb and path a
// method sends, and whether it decodes the body it gets back. A method that
// quietly moves to another path, flips its verb, or stops decoding its payload
// fails here instead of against the real API.
type clientRouteCase struct {
	call     func(ctx context.Context, client *linode.Client) (any, error)
	want     any
	name     string
	wantVerb string
	wantPath string
	response string
}

// runClientRouteCases serves each case's response and checks the verb, path,
// and decoded probe value its call reports back.
func runClientRouteCases(t *testing.T, cases []clientRouteCase) {
	t.Helper()

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.wantVerb {
					t.Errorf("request verb = %v, want %v", r.Method, tt.wantVerb)
				}

				if r.URL.Path != tt.wantPath {
					t.Errorf("request path = %v, want %v", r.URL.Path, tt.wantPath)
				}

				if r.Header.Get("Authorization") != authHeaderTestToken {
					t.Errorf("authorization header = %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
				}

				w.Header().Set("Content-Type", tcApplicationJSON)

				if _, err := io.WriteString(w, tt.response); err != nil {
					t.Errorf("write response body: %v", err)
				}
			}))
			defer srv.Close()

			client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

			got, err := tt.call(t.Context(), client)
			if err != nil {
				t.Fatalf("call returned error: %v", err)
			}

			if got != tt.want {
				t.Errorf("decoded probe = %v, want %v", got, tt.want)
			}
		})
	}
}

// clientRouteError adds the harness's own context to a Client error so a case
// body reports the failed call instead of handing the upstream error straight
// back to the runner.
func clientRouteError(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("linode client call failed: %w", err)
}

// clientRouteProbe reads probe only once the call that produced it succeeded,
// so a failed call never reaches a field access on a nil result.
func clientRouteProbe(err error, probe func() any) (any, error) {
	if err != nil {
		return nil, clientRouteError(err)
	}

	return probe(), nil
}

// clientRouteList reduces a decoded list to "<count>:<probe of first element>",
// so a dropped element and a mis-decoded field both surface in one comparison.
func clientRouteList[T any](items []T, probe func(T) string) string {
	if len(items) == 0 {
		return "0:"
	}

	return strconv.Itoa(len(items)) + ":" + probe(items[0])
}
