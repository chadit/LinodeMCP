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

// Response bodies the route cases serve. Each one carries a single probe value
// so the case can prove the client decoded the payload rather than handing back
// a zero value.
const (
	clientRouteACLEnvelope                = "{\"acl\":{\"enabled\":true}}"
	clientRouteEmptyObject                = "{}"
	clientRouteObjACL                     = "{\"acl\":\"probe-value\"}"
	clientRouteObjAddress                 = "{\"address\":\"probe-value\"}"
	clientRouteObjComments                = "{\"comments\":\"probe-value\"}"
	clientRouteObjCreated                 = "{\"created\":\"probe-value\"}"
	clientRouteObjEmail                   = "{\"email\":\"probe-value\"}"
	clientRouteObjEuuid                   = "{\"euuid\":\"probe-value\"}"
	clientRouteObjHypervisor              = "{\"hypervisor\":\"probe-value\"}"
	clientRouteObjID                      = "{\"id\":\"probe-value\"}"
	clientRouteObjInboundPolicy           = "{\"inbound_policy\":\"probe-value\"}"
	clientRouteObjInterfacesForNewLinodes = "{\"interfaces_for_new_linodes\":\"probe-value\"}"
	clientRouteObjLabel                   = "{\"label\":\"probe-value\"}"
	clientRouteObjMacAddress              = "{\"mac_address\":\"probe-value\"}"
	clientRouteObjName                    = "{\"name\":\"probe-value\"}"
	clientRouteObjProbe                   = "{\"probe\":\"probe-value\"}"
	clientRouteObjProtocol                = "{\"protocol\":\"probe-value\"}"
	clientRouteObjPurpose                 = "{\"purpose\":\"probe-value\"}"
	clientRouteObjRange                   = "{\"range\":\"probe-value\"}"
	clientRouteObjSSLBool                 = "{\"ssl\":true}"
	clientRouteObjStatus                  = "{\"status\":\"probe-value\"}"
	clientRouteObjType                    = "{\"type\":\"probe-value\"}"
	clientRouteObjUUID                    = "{\"uuid\":\"probe-value\"}"
	clientRouteObjUsername                = "{\"username\":\"probe-value\"}"
	clientRouteObjVersion                 = "{\"version\":\"probe-value\"}"
	clientRouteObjZip                     = "{\"zip\":\"probe-value\"}"
	clientRoutePageAddress                = "{\"data\":[{\"address\":\"probe-value\"},{\"address\":\"second-value\"}],\"page\":1}"
	clientRoutePageCipherSuite            = "{\"data\":[{\"cipher_suite\":\"probe-value\"},{\"cipher_suite\":\"second-value\"}],\"page\":1}"
	clientRoutePageComments               = "{\"data\":[{\"comments\":\"probe-value\"},{\"comments\":\"second-value\"}],\"page\":1}"
	clientRoutePageCreated                = "{\"data\":[{\"created\":\"probe-value\"},{\"created\":\"second-value\"}],\"page\":1}"
	clientRoutePageLabel                  = "{\"data\":[{\"label\":\"probe-value\"},{\"label\":\"second-value\"}],\"page\":1}"
	clientRoutePageProtocol               = "{\"data\":[{\"protocol\":\"probe-value\"},{\"protocol\":\"second-value\"}],\"page\":1}"
	clientRoutePageType                   = "{\"data\":[{\"type\":\"probe-value\"},{\"type\":\"second-value\"}],\"page\":1}"
	clientRouteProtoObjIDInt32            = "{\"id\":4242}"
	clientRouteProtoPageLabel             = "{\"data\":[{\"label\":\"probe-value\"},{\"label\":\"second-value\"}],\"page\":1}"
)

// Request paths and probe results the cases below share.
const (
	clientRouteIPv4Fixture                                      = "203.0.113.5"
	clientRoutePathAccount                                      = "/account"
	clientRoutePathAccountChildAccountsAlpha                    = "/account/child-accounts/alpha"
	clientRoutePathAccountEvents4242                            = "/account/events/4242"
	clientRoutePathAccountOauthClientsAlpha                     = "/account/oauth-clients/alpha"
	clientRoutePathAccountPaymentMethodsAlpha                   = "/account/payment-methods/alpha"
	clientRoutePathAccountServiceTransfersAlpha                 = "/account/service-transfers/alpha"
	clientRoutePathAccountSettings                              = "/account/settings"
	clientRoutePathAccountUsersAlpha                            = "/account/users/alpha"
	clientRoutePathDatabasesMysqlInstances4242                  = "/databases/mysql/instances/4242"
	clientRoutePathDatabasesPostgresqlInstances4242             = "/databases/postgresql/instances/4242"
	clientRoutePathDomains4242                                  = "/domains/4242"
	clientRoutePathDomains4242Records                           = "/domains/4242/records"
	clientRoutePathDomains4242Records8615                       = "/domains/4242/records/8615"
	clientRoutePathImagesAlpha                                  = "/images/alpha"
	clientRoutePathImagesSharegroups4242                        = "/images/sharegroups/4242"
	clientRoutePathImagesSharegroupsTokensAlphaSharegroup       = "/images/sharegroups/tokens/alpha/sharegroup"
	clientRoutePathLinodeInstances4242                          = "/linode/instances/4242"
	clientRoutePathLinodeInstances4242Backups8615               = "/linode/instances/4242/backups/8615"
	clientRoutePathLinodeInstances4242Configs                   = "/linode/instances/4242/configs"
	clientRoutePathLinodeInstances4242Configs8615               = "/linode/instances/4242/configs/8615"
	clientRoutePathLinodeInstances4242Configs8615Interfaces1379 = "/linode/instances/4242/configs/8615/interfaces/1379"
	clientRoutePathLinodeInstances4242Disks                     = "/linode/instances/4242/disks"
	clientRoutePathLinodeInstances4242Disks8615                 = "/linode/instances/4242/disks/8615"
	clientRoutePathLinodeInstances4242Firewalls                 = "/linode/instances/4242/firewalls"
	clientRoutePathLinodeInstances4242Interfaces8615            = "/linode/instances/4242/interfaces/8615"
	clientRoutePathLinodeInstances4242IpsAlpha                  = "/linode/instances/4242/ips/alpha"
	clientRoutePathLinodeInstances4242Volumes                   = "/linode/instances/4242/volumes"
	clientRoutePathLinodeStackscripts4242                       = "/linode/stackscripts/4242"
	clientRoutePathLinodeTypesAlpha                             = "/linode/types/alpha"
	clientRoutePathLkeClusters4242                              = "/lke/clusters/4242"
	clientRoutePathLkeClusters4242ControlPlaneACL               = "/lke/clusters/4242/control_plane_acl"
	clientRoutePathLkeClusters4242NodesAlpha                    = "/lke/clusters/4242/nodes/alpha"
	clientRoutePathLkeClusters4242Pools                         = "/lke/clusters/4242/pools"
	clientRoutePathLkeClusters4242Pools8615                     = "/lke/clusters/4242/pools/8615"
	clientRoutePathLongviewClientsAlpha                         = "/longview/clients/alpha"
	clientRoutePathLongviewPlan                                 = "/longview/plan"
	clientRoutePathManagedContacts4242                          = "/managed/contacts/4242"
	clientRoutePathManagedCredentials4242                       = "/managed/credentials/4242"
	clientRoutePathManagedLinodeSettings4242                    = "/managed/linode-settings/4242"
	clientRoutePathManagedServices4242                          = "/managed/services/4242"
	clientRoutePathMonitorServicesAlphaAlertDefinitions4242     = "/monitor/services/alpha/alert-definitions/4242"
	clientRoutePathNetworkingFirewalls4242                      = "/networking/firewalls/4242"
	clientRoutePathNetworkingFirewalls4242Devices               = "/networking/firewalls/4242/devices"
	clientRoutePathNetworkingFirewalls4242Devices8615           = "/networking/firewalls/4242/devices/8615"
	clientRoutePathNetworkingFirewalls4242Rules                 = "/networking/firewalls/4242/rules"
	clientRoutePathNetworkingIps20301135                        = "/networking/ips/203.0.113.5"
	clientRoutePathNetworkingIpv6Ranges20010db864               = "/networking/ipv6/ranges/2001:0db8::/64"
	clientRoutePathNetworkingReservedIps20301135                = "/networking/reserved/ips/203.0.113.5"
	clientRoutePathNetworkingVlans                              = "/networking/vlans"
	clientRoutePathNodebalancers4242                            = "/nodebalancers/4242"
	clientRoutePathNodebalancers4242Configs                     = "/nodebalancers/4242/configs"
	clientRoutePathNodebalancers4242Configs8615                 = "/nodebalancers/4242/configs/8615"
	clientRoutePathNodebalancers4242Configs8615Nodes            = "/nodebalancers/4242/configs/8615/nodes"
	clientRoutePathNodebalancers4242Configs8615Nodes1379        = "/nodebalancers/4242/configs/8615/nodes/1379"
	clientRoutePathNodebalancers4242Firewalls                   = "/nodebalancers/4242/firewalls"
	clientRoutePathObjectStorageBucketsAlphaBravo               = "/object-storage/buckets/alpha/bravo"
	clientRoutePathObjectStorageBucketsAlphaBravoAccess         = "/object-storage/buckets/alpha/bravo/access"
	clientRoutePathObjectStorageBucketsAlphaBravoObjectACL      = "/object-storage/buckets/alpha/bravo/object-acl"
	clientRoutePathObjectStorageBucketsAlphaBravoSSL            = "/object-storage/buckets/alpha/bravo/ssl"
	clientRoutePathObjectStorageKeys4242                        = "/object-storage/keys/4242"
	clientRoutePathPlacementGroups4242                          = "/placement/groups/4242"
	clientRoutePathProfile                                      = "/profile"
	clientRoutePathProfileApps4242                              = "/profile/apps/4242"
	clientRoutePathProfileDevices4242                           = "/profile/devices/4242"
	clientRoutePathProfilePhoneNumberVerify                     = "/profile/phone-number/verify"
	clientRoutePathProfileSshkeys4242                           = "/profile/sshkeys/4242"
	clientRoutePathProfileTokens4242                            = "/profile/tokens/4242"
	clientRoutePathVolumes4242                                  = "/volumes/4242"
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
