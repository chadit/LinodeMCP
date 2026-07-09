package tools_test

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// conformanceMessages maps each corpus fixture's proto full name to a
// constructor for the matching generated message. Add an entry when a message
// gains a fixture. The Python gate (python/tests/unit/test_proto_conformance.py)
// keeps the mirror map, and both read the same testdata/conformance corpus, so
// Go and Python serialize every covered message to structurally identical
// output.
func conformanceMessages() map[string]func() proto.Message {
	registry := make(map[string]func() proto.Message)
	maps.Copy(registry, conformanceMessages0())
	maps.Copy(registry, conformanceMessages1())
	maps.Copy(registry, conformanceMessages2())
	maps.Copy(registry, conformanceMessages3())
	maps.Copy(registry, conformanceMessages4())
	maps.Copy(registry, conformanceMessages5())

	return registry
}

func conformanceMessages0() map[string]func() proto.Message {
	return map[string]func() proto.Message{
		"linode.mcp.v1.Account":                               func() proto.Message { return &linodev1.Account{} },
		"linode.mcp.v1.AccountAgreements":                     func() proto.Message { return &linodev1.AccountAgreements{} },
		"linode.mcp.v1.AccountAvailability":                   func() proto.Message { return &linodev1.AccountAvailability{} },
		"linode.mcp.v1.AccountAvailabilityListResponse":       func() proto.Message { return &linodev1.AccountAvailabilityListResponse{} },
		"linode.mcp.v1.AccountBetaEnrollResponse":             func() proto.Message { return &linodev1.AccountBetaEnrollResponse{} },
		"linode.mcp.v1.AccountBetaProgram":                    func() proto.Message { return &linodev1.AccountBetaProgram{} },
		"linode.mcp.v1.AccountBetaProgramListResponse":        func() proto.Message { return &linodev1.AccountBetaProgramListResponse{} },
		"linode.mcp.v1.AccountCancelWriteResponse":            func() proto.Message { return &linodev1.AccountCancelWriteResponse{} },
		"linode.mcp.v1.AccountChildAccountTokenWriteResponse": func() proto.Message { return &linodev1.AccountChildAccountTokenWriteResponse{} },
		"linode.mcp.v1.AccountEntityTransfer":                 func() proto.Message { return &linodev1.AccountEntityTransfer{} },
		"linode.mcp.v1.AccountEvent":                          func() proto.Message { return &linodev1.AccountEvent{} },
		"linode.mcp.v1.AccountEventListResponse":              func() proto.Message { return &linodev1.AccountEventListResponse{} },
		"linode.mcp.v1.AccountEventSeenResponse":              func() proto.Message { return &linodev1.AccountEventSeenResponse{} },
		"linode.mcp.v1.AccountInvoice":                        func() proto.Message { return &linodev1.AccountInvoice{} },
		"linode.mcp.v1.AccountInvoiceItemListResponse":        func() proto.Message { return &linodev1.AccountInvoiceItemListResponse{} },
		"linode.mcp.v1.AccountInvoiceListResponse":            func() proto.Message { return &linodev1.AccountInvoiceListResponse{} },
		"linode.mcp.v1.AccountLogin":                          func() proto.Message { return &linodev1.AccountLogin{} },
		"linode.mcp.v1.AccountLoginListResponse":              func() proto.Message { return &linodev1.AccountLoginListResponse{} },
		"linode.mcp.v1.AccountMaintenanceListResponse":        func() proto.Message { return &linodev1.AccountMaintenanceListResponse{} },
		"linode.mcp.v1.AccountNotificationListResponse":       func() proto.Message { return &linodev1.AccountNotificationListResponse{} },
		"linode.mcp.v1.AccountPayment":                        func() proto.Message { return &linodev1.AccountPayment{} },
		"linode.mcp.v1.AccountPaymentListResponse":            func() proto.Message { return &linodev1.AccountPaymentListResponse{} },
		"linode.mcp.v1.AccountPaymentMethod":                  func() proto.Message { return &linodev1.AccountPaymentMethod{} },
		"linode.mcp.v1.AccountPaymentMethodIDResponse":        func() proto.Message { return &linodev1.AccountPaymentMethodIDResponse{} },
		"linode.mcp.v1.AccountPaymentMethodListResponse":      func() proto.Message { return &linodev1.AccountPaymentMethodListResponse{} },
		"linode.mcp.v1.AccountPaymentMethodWriteResponse":     func() proto.Message { return &linodev1.AccountPaymentMethodWriteResponse{} },
		"linode.mcp.v1.AccountPaymentWriteResponse":           func() proto.Message { return &linodev1.AccountPaymentWriteResponse{} },
		"linode.mcp.v1.AccountPromoResponse":                  func() proto.Message { return &linodev1.AccountPromoResponse{} },
		"linode.mcp.v1.AccountServiceTransferActionResponse":  func() proto.Message { return &linodev1.AccountServiceTransferActionResponse{} },
		"linode.mcp.v1.AccountServiceTransferListResponse":    func() proto.Message { return &linodev1.AccountServiceTransferListResponse{} },
		"linode.mcp.v1.AccountServiceTransferWriteResponse":   func() proto.Message { return &linodev1.AccountServiceTransferWriteResponse{} },
		"linode.mcp.v1.AccountSettings":                       func() proto.Message { return &linodev1.AccountSettings{} },
		"linode.mcp.v1.AccountSettingsWriteResponse":          func() proto.Message { return &linodev1.AccountSettingsWriteResponse{} },
		"linode.mcp.v1.AccountTransfer":                       func() proto.Message { return &linodev1.AccountTransfer{} },
		"linode.mcp.v1.AccountUser":                           func() proto.Message { return &linodev1.AccountUser{} },
		"linode.mcp.v1.AccountUserDeleteResponse":             func() proto.Message { return &linodev1.AccountUserDeleteResponse{} },
		"linode.mcp.v1.AccountUserGrants":                     func() proto.Message { return &linodev1.AccountUserGrants{} },
		"linode.mcp.v1.AccountUserGrantsWriteResponse":        func() proto.Message { return &linodev1.AccountUserGrantsWriteResponse{} },
		"linode.mcp.v1.AccountUserListResponse":               func() proto.Message { return &linodev1.AccountUserListResponse{} },
		"linode.mcp.v1.AccountUserWriteResponse":              func() proto.Message { return &linodev1.AccountUserWriteResponse{} },
		"linode.mcp.v1.AccountWriteResponse":                  func() proto.Message { return &linodev1.AccountWriteResponse{} },
		"linode.mcp.v1.AuditExportResponse":                   func() proto.Message { return &linodev1.AuditExportResponse{} },
		"linode.mcp.v1.AuditHealthResponse":                   func() proto.Message { return &linodev1.AuditHealthResponse{} },
		"linode.mcp.v1.AuditRecentResponse":                   func() proto.Message { return &linodev1.AuditRecentResponse{} },
		"linode.mcp.v1.AuditReportResponse":                   func() proto.Message { return &linodev1.AuditReportResponse{} },
		"linode.mcp.v1.AuditSummaryResponse":                  func() proto.Message { return &linodev1.AuditSummaryResponse{} },
		"linode.mcp.v1.BetaProgram":                           func() proto.Message { return &linodev1.BetaProgram{} },
		"linode.mcp.v1.BetaProgramListResponse":               func() proto.Message { return &linodev1.BetaProgramListResponse{} },
		"linode.mcp.v1.BucketSSL":                             func() proto.Message { return &linodev1.BucketSSL{} },
		"linode.mcp.v1.ChildAccount":                          func() proto.Message { return &linodev1.ChildAccount{} },
		"linode.mcp.v1.ChildAccountListResponse":              func() proto.Message { return &linodev1.ChildAccountListResponse{} },
		"linode.mcp.v1.ConfigInterfaceListResponse":           func() proto.Message { return &linodev1.ConfigInterfaceListResponse{} },
		"linode.mcp.v1.ConfigInterfaceResponse":               func() proto.Message { return &linodev1.ConfigInterfaceResponse{} },
		"linode.mcp.v1.ConfigInterfaceWriteResponse":          func() proto.Message { return &linodev1.ConfigInterfaceWriteResponse{} },
		"linode.mcp.v1.DatabaseCredentials":                   func() proto.Message { return &linodev1.DatabaseCredentials{} },
		"linode.mcp.v1.DatabaseEngine":                        func() proto.Message { return &linodev1.DatabaseEngine{} },
		"linode.mcp.v1.DatabaseEngineListResponse":            func() proto.Message { return &linodev1.DatabaseEngineListResponse{} },
		"linode.mcp.v1.DatabaseInstanceActionWriteResponse":   func() proto.Message { return &linodev1.DatabaseInstanceActionWriteResponse{} },
		"linode.mcp.v1.DatabaseInstanceDeleteResponse":        func() proto.Message { return &linodev1.DatabaseInstanceDeleteResponse{} },
		"linode.mcp.v1.DatabaseInstanceListResponse":          func() proto.Message { return &linodev1.DatabaseInstanceListResponse{} },
	}
}

func conformanceMessages1() map[string]func() proto.Message {
	return map[string]func() proto.Message{
		"linode.mcp.v1.DatabaseInstanceWriteResponse":            func() proto.Message { return &linodev1.DatabaseInstanceWriteResponse{} },
		"linode.mcp.v1.DatabaseMySQLInstanceListResponse":        func() proto.Message { return &linodev1.DatabaseMySQLInstanceListResponse{} },
		"linode.mcp.v1.DatabasePostgreSQLInstanceListResponse":   func() proto.Message { return &linodev1.DatabasePostgreSQLInstanceListResponse{} },
		"linode.mcp.v1.DatabaseSSL":                              func() proto.Message { return &linodev1.DatabaseSSL{} },
		"linode.mcp.v1.DatabaseType":                             func() proto.Message { return &linodev1.DatabaseType{} },
		"linode.mcp.v1.DatabaseTypeListResponse":                 func() proto.Message { return &linodev1.DatabaseTypeListResponse{} },
		"linode.mcp.v1.Domain":                                   func() proto.Message { return &linodev1.Domain{} },
		"linode.mcp.v1.DomainDeleteResponse":                     func() proto.Message { return &linodev1.DomainDeleteResponse{} },
		"linode.mcp.v1.DomainListResponse":                       func() proto.Message { return &linodev1.DomainListResponse{} },
		"linode.mcp.v1.DomainRecord":                             func() proto.Message { return &linodev1.DomainRecord{} },
		"linode.mcp.v1.DomainRecordDeleteResponse":               func() proto.Message { return &linodev1.DomainRecordDeleteResponse{} },
		"linode.mcp.v1.DomainRecordListResponse":                 func() proto.Message { return &linodev1.DomainRecordListResponse{} },
		"linode.mcp.v1.DomainRecordWriteResponse":                func() proto.Message { return &linodev1.DomainRecordWriteResponse{} },
		"linode.mcp.v1.DomainWriteResponse":                      func() proto.Message { return &linodev1.DomainWriteResponse{} },
		"linode.mcp.v1.DomainZoneFile":                           func() proto.Message { return &linodev1.DomainZoneFile{} },
		"linode.mcp.v1.DryRunResponse":                           func() proto.Message { return &linodev1.DryRunResponse{} },
		"linode.mcp.v1.Firewall":                                 func() proto.Message { return &linodev1.Firewall{} },
		"linode.mcp.v1.FirewallDeleteResponse":                   func() proto.Message { return &linodev1.FirewallDeleteResponse{} },
		"linode.mcp.v1.FirewallDevice":                           func() proto.Message { return &linodev1.FirewallDevice{} },
		"linode.mcp.v1.FirewallDeviceDeleteResponse":             func() proto.Message { return &linodev1.FirewallDeviceDeleteResponse{} },
		"linode.mcp.v1.FirewallDeviceListResponse":               func() proto.Message { return &linodev1.FirewallDeviceListResponse{} },
		"linode.mcp.v1.FirewallDeviceWriteResponse":              func() proto.Message { return &linodev1.FirewallDeviceWriteResponse{} },
		"linode.mcp.v1.FirewallListResponse":                     func() proto.Message { return &linodev1.FirewallListResponse{} },
		"linode.mcp.v1.FirewallRuleVersion":                      func() proto.Message { return &linodev1.FirewallRuleVersion{} },
		"linode.mcp.v1.FirewallRuleVersionListResponse":          func() proto.Message { return &linodev1.FirewallRuleVersionListResponse{} },
		"linode.mcp.v1.FirewallRules":                            func() proto.Message { return &linodev1.FirewallRules{} },
		"linode.mcp.v1.FirewallRulesWriteResponse":               func() proto.Message { return &linodev1.FirewallRulesWriteResponse{} },
		"linode.mcp.v1.FirewallSettings":                         func() proto.Message { return &linodev1.FirewallSettings{} },
		"linode.mcp.v1.FirewallSettingsWriteResponse":            func() proto.Message { return &linodev1.FirewallSettingsWriteResponse{} },
		"linode.mcp.v1.FirewallTemplate":                         func() proto.Message { return &linodev1.FirewallTemplate{} },
		"linode.mcp.v1.FirewallTemplateListResponse":             func() proto.Message { return &linodev1.FirewallTemplateListResponse{} },
		"linode.mcp.v1.FirewallWriteResponse":                    func() proto.Message { return &linodev1.FirewallWriteResponse{} },
		"linode.mcp.v1.HelloResponse":                            func() proto.Message { return &linodev1.HelloResponse{} },
		"linode.mcp.v1.IPAddress":                                func() proto.Message { return &linodev1.IPAddress{} },
		"linode.mcp.v1.IPAddressWriteResponse":                   func() proto.Message { return &linodev1.IPAddressWriteResponse{} },
		"linode.mcp.v1.IPv6PoolListResponse":                     func() proto.Message { return &linodev1.IPv6PoolListResponse{} },
		"linode.mcp.v1.IPv6Range":                                func() proto.Message { return &linodev1.IPv6Range{} },
		"linode.mcp.v1.IPv6RangeDeleteResponse":                  func() proto.Message { return &linodev1.IPv6RangeDeleteResponse{} },
		"linode.mcp.v1.IPv6RangeListResponse":                    func() proto.Message { return &linodev1.IPv6RangeListResponse{} },
		"linode.mcp.v1.IPv6RangeWriteResponse":                   func() proto.Message { return &linodev1.IPv6RangeWriteResponse{} },
		"linode.mcp.v1.Image":                                    func() proto.Message { return &linodev1.Image{} },
		"linode.mcp.v1.ImageDeleteResponse":                      func() proto.Message { return &linodev1.ImageDeleteResponse{} },
		"linode.mcp.v1.ImageListResponse":                        func() proto.Message { return &linodev1.ImageListResponse{} },
		"linode.mcp.v1.ImageShareGroup":                          func() proto.Message { return &linodev1.ImageShareGroup{} },
		"linode.mcp.v1.ImageShareGroupDeleteResponse":            func() proto.Message { return &linodev1.ImageShareGroupDeleteResponse{} },
		"linode.mcp.v1.ImageShareGroupImageDeleteResponse":       func() proto.Message { return &linodev1.ImageShareGroupImageDeleteResponse{} },
		"linode.mcp.v1.ImageShareGroupListResponse":              func() proto.Message { return &linodev1.ImageShareGroupListResponse{} },
		"linode.mcp.v1.ImageShareGroupMember":                    func() proto.Message { return &linodev1.ImageShareGroupMember{} },
		"linode.mcp.v1.ImageShareGroupMemberListResponse":        func() proto.Message { return &linodev1.ImageShareGroupMemberListResponse{} },
		"linode.mcp.v1.ImageShareGroupMemberTokenDeleteResponse": func() proto.Message { return &linodev1.ImageShareGroupMemberTokenDeleteResponse{} },
		"linode.mcp.v1.ImageShareGroupMemberWriteResponse":       func() proto.Message { return &linodev1.ImageShareGroupMemberWriteResponse{} },
		"linode.mcp.v1.ImageShareGroupToken":                     func() proto.Message { return &linodev1.ImageShareGroupToken{} },
		"linode.mcp.v1.ImageShareGroupTokenDeleteResponse":       func() proto.Message { return &linodev1.ImageShareGroupTokenDeleteResponse{} },
		"linode.mcp.v1.ImageShareGroupTokenListResponse":         func() proto.Message { return &linodev1.ImageShareGroupTokenListResponse{} },
		"linode.mcp.v1.ImageShareGroupTokenWriteResponse":        func() proto.Message { return &linodev1.ImageShareGroupTokenWriteResponse{} },
		"linode.mcp.v1.ImageShareGroupWriteResponse":             func() proto.Message { return &linodev1.ImageShareGroupWriteResponse{} },
		"linode.mcp.v1.ImageUploadWriteResponse":                 func() proto.Message { return &linodev1.ImageUploadWriteResponse{} },
		"linode.mcp.v1.ImageWriteResponse":                       func() proto.Message { return &linodev1.ImageWriteResponse{} },
		"linode.mcp.v1.Instance":                                 func() proto.Message { return &linodev1.Instance{} },
		"linode.mcp.v1.InstanceActionWriteResponse":              func() proto.Message { return &linodev1.InstanceActionWriteResponse{} },
	}
}

func conformanceMessages2() map[string]func() proto.Message {
	return map[string]func() proto.Message{
		"linode.mcp.v1.InstanceBackup":                         func() proto.Message { return &linodev1.InstanceBackup{} },
		"linode.mcp.v1.InstanceBackupRestoreWriteResponse":     func() proto.Message { return &linodev1.InstanceBackupRestoreWriteResponse{} },
		"linode.mcp.v1.InstanceBackupWriteResponse":            func() proto.Message { return &linodev1.InstanceBackupWriteResponse{} },
		"linode.mcp.v1.InstanceBackupsResponse":                func() proto.Message { return &linodev1.InstanceBackupsResponse{} },
		"linode.mcp.v1.InstanceConfigDeleteResponse":           func() proto.Message { return &linodev1.InstanceConfigDeleteResponse{} },
		"linode.mcp.v1.InstanceConfigInterfaceDeleteResponse":  func() proto.Message { return &linodev1.InstanceConfigInterfaceDeleteResponse{} },
		"linode.mcp.v1.InstanceConfigInterfaceReorderResponse": func() proto.Message { return &linodev1.InstanceConfigInterfaceReorderResponse{} },
		"linode.mcp.v1.InstanceConfigListResponse":             func() proto.Message { return &linodev1.InstanceConfigListResponse{} },
		"linode.mcp.v1.InstanceConfigWriteResponse":            func() proto.Message { return &linodev1.InstanceConfigWriteResponse{} },
		"linode.mcp.v1.InstanceDeleteResponse":                 func() proto.Message { return &linodev1.InstanceDeleteResponse{} },
		"linode.mcp.v1.InstanceDisk":                           func() proto.Message { return &linodev1.InstanceDisk{} },
		"linode.mcp.v1.InstanceDiskActionResponse":             func() proto.Message { return &linodev1.InstanceDiskActionResponse{} },
		"linode.mcp.v1.InstanceDiskDeleteResponse":             func() proto.Message { return &linodev1.InstanceDiskDeleteResponse{} },
		"linode.mcp.v1.InstanceDiskListResponse":               func() proto.Message { return &linodev1.InstanceDiskListResponse{} },
		"linode.mcp.v1.InstanceDiskResizeWriteResponse":        func() proto.Message { return &linodev1.InstanceDiskResizeWriteResponse{} },
		"linode.mcp.v1.InstanceDiskWriteResponse":              func() proto.Message { return &linodev1.InstanceDiskWriteResponse{} },
		"linode.mcp.v1.InstanceIPDeleteResponse":               func() proto.Message { return &linodev1.InstanceIPDeleteResponse{} },
		"linode.mcp.v1.InstanceIPsResponse":                    func() proto.Message { return &linodev1.InstanceIPsResponse{} },
		"linode.mcp.v1.InstanceInterface":                      func() proto.Message { return &linodev1.InstanceInterface{} },
		"linode.mcp.v1.InstanceInterfaceDeleteResponse":        func() proto.Message { return &linodev1.InstanceInterfaceDeleteResponse{} },
		"linode.mcp.v1.InstanceInterfaceHistoryListResponse":   func() proto.Message { return &linodev1.InstanceInterfaceHistoryListResponse{} },
		"linode.mcp.v1.InstanceInterfaceListResponse":          func() proto.Message { return &linodev1.InstanceInterfaceListResponse{} },
		"linode.mcp.v1.InstanceInterfaceSettings":              func() proto.Message { return &linodev1.InstanceInterfaceSettings{} },
		"linode.mcp.v1.InstanceInterfaceSettingsWriteResponse": func() proto.Message { return &linodev1.InstanceInterfaceSettingsWriteResponse{} },
		"linode.mcp.v1.InstanceInterfaceUpgradeWriteResponse":  func() proto.Message { return &linodev1.InstanceInterfaceUpgradeWriteResponse{} },
		"linode.mcp.v1.InstanceInterfaceWriteResponse":         func() proto.Message { return &linodev1.InstanceInterfaceWriteResponse{} },
		"linode.mcp.v1.InstanceListResponse":                   func() proto.Message { return &linodev1.InstanceListResponse{} },
		"linode.mcp.v1.InstanceMigrateWriteResponse":           func() proto.Message { return &linodev1.InstanceMigrateWriteResponse{} },
		"linode.mcp.v1.InstancePowerActionResponse":            func() proto.Message { return &linodev1.InstancePowerActionResponse{} },
		"linode.mcp.v1.InstanceResizeWriteResponse":            func() proto.Message { return &linodev1.InstanceResizeWriteResponse{} },
		"linode.mcp.v1.InstanceStats":                          func() proto.Message { return &linodev1.InstanceStats{} },
		"linode.mcp.v1.InstanceTransfer":                       func() proto.Message { return &linodev1.InstanceTransfer{} },
		"linode.mcp.v1.InstanceTransferMonth":                  func() proto.Message { return &linodev1.InstanceTransferMonth{} },
		"linode.mcp.v1.InstanceType":                           func() proto.Message { return &linodev1.InstanceType{} },
		"linode.mcp.v1.InstanceTypeListResponse":               func() proto.Message { return &linodev1.InstanceTypeListResponse{} },
		"linode.mcp.v1.InstanceWriteResponse":                  func() proto.Message { return &linodev1.InstanceWriteResponse{} },
		"linode.mcp.v1.Kernel":                                 func() proto.Message { return &linodev1.Kernel{} },
		"linode.mcp.v1.KernelListResponse":                     func() proto.Message { return &linodev1.KernelListResponse{} },
		"linode.mcp.v1.LKEACLDeleteResponse":                   func() proto.Message { return &linodev1.LKEACLDeleteResponse{} },
		"linode.mcp.v1.LKEACLWriteResponse":                    func() proto.Message { return &linodev1.LKEACLWriteResponse{} },
		"linode.mcp.v1.LKEAPIEndpointListResponse":             func() proto.Message { return &linodev1.LKEAPIEndpointListResponse{} },
		"linode.mcp.v1.LKECluster":                             func() proto.Message { return &linodev1.LKECluster{} },
		"linode.mcp.v1.LKEClusterActionResponse":               func() proto.Message { return &linodev1.LKEClusterActionResponse{} },
		"linode.mcp.v1.LKEClusterDeleteResponse":               func() proto.Message { return &linodev1.LKEClusterDeleteResponse{} },
		"linode.mcp.v1.LKEClusterListResponse":                 func() proto.Message { return &linodev1.LKEClusterListResponse{} },
		"linode.mcp.v1.LKEClusterWriteResponse":                func() proto.Message { return &linodev1.LKEClusterWriteResponse{} },
		"linode.mcp.v1.LKEControlPlaneACL":                     func() proto.Message { return &linodev1.LKEControlPlaneACL{} },
		"linode.mcp.v1.LKEDashboard":                           func() proto.Message { return &linodev1.LKEDashboard{} },
		"linode.mcp.v1.LKEKubeconfig":                          func() proto.Message { return &linodev1.LKEKubeconfig{} },
		"linode.mcp.v1.LKEKubeconfigDeleteResponse":            func() proto.Message { return &linodev1.LKEKubeconfigDeleteResponse{} },
		"linode.mcp.v1.LKENode":                                func() proto.Message { return &linodev1.LKENode{} },
		"linode.mcp.v1.LKENodeDeleteResponse":                  func() proto.Message { return &linodev1.LKENodeDeleteResponse{} },
		"linode.mcp.v1.LKENodePool":                            func() proto.Message { return &linodev1.LKENodePool{} },
		"linode.mcp.v1.LKENodePoolDeleteResponse":              func() proto.Message { return &linodev1.LKENodePoolDeleteResponse{} },
		"linode.mcp.v1.LKENodePoolListResponse":                func() proto.Message { return &linodev1.LKENodePoolListResponse{} },
		"linode.mcp.v1.LKENodePoolWriteResponse":               func() proto.Message { return &linodev1.LKENodePoolWriteResponse{} },
		"linode.mcp.v1.LKENodeRecycleResponse":                 func() proto.Message { return &linodev1.LKENodeRecycleResponse{} },
		"linode.mcp.v1.LKEPoolRecycleResponse":                 func() proto.Message { return &linodev1.LKEPoolRecycleResponse{} },
		"linode.mcp.v1.LKEServiceTokenDeleteResponse":          func() proto.Message { return &linodev1.LKEServiceTokenDeleteResponse{} },
		"linode.mcp.v1.LKETierVersion":                         func() proto.Message { return &linodev1.LKETierVersion{} },
	}
}

func conformanceMessages3() map[string]func() proto.Message {
	return map[string]func() proto.Message{
		"linode.mcp.v1.LKETierVersionListResponse":                 func() proto.Message { return &linodev1.LKETierVersionListResponse{} },
		"linode.mcp.v1.LKETypeListResponse":                        func() proto.Message { return &linodev1.LKETypeListResponse{} },
		"linode.mcp.v1.LKEVersion":                                 func() proto.Message { return &linodev1.LKEVersion{} },
		"linode.mcp.v1.LKEVersionListResponse":                     func() proto.Message { return &linodev1.LKEVersionListResponse{} },
		"linode.mcp.v1.LongviewClient":                             func() proto.Message { return &linodev1.LongviewClient{} },
		"linode.mcp.v1.LongviewClientCreateWriteResponse":          func() proto.Message { return &linodev1.LongviewClientCreateWriteResponse{} },
		"linode.mcp.v1.LongviewClientIDResponse":                   func() proto.Message { return &linodev1.LongviewClientIDResponse{} },
		"linode.mcp.v1.LongviewClientListResponse":                 func() proto.Message { return &linodev1.LongviewClientListResponse{} },
		"linode.mcp.v1.LongviewClientWriteResponse":                func() proto.Message { return &linodev1.LongviewClientWriteResponse{} },
		"linode.mcp.v1.LongviewSubscription":                       func() proto.Message { return &linodev1.LongviewSubscription{} },
		"linode.mcp.v1.LongviewSubscriptionListResponse":           func() proto.Message { return &linodev1.LongviewSubscriptionListResponse{} },
		"linode.mcp.v1.LongviewSubscriptionWriteResponse":          func() proto.Message { return &linodev1.LongviewSubscriptionWriteResponse{} },
		"linode.mcp.v1.LongviewTypeListResponse":                   func() proto.Message { return &linodev1.LongviewTypeListResponse{} },
		"linode.mcp.v1.MaintenancePolicyListResponse":              func() proto.Message { return &linodev1.MaintenancePolicyListResponse{} },
		"linode.mcp.v1.ManagedContact":                             func() proto.Message { return &linodev1.ManagedContact{} },
		"linode.mcp.v1.ManagedContactIDResponse":                   func() proto.Message { return &linodev1.ManagedContactIDResponse{} },
		"linode.mcp.v1.ManagedContactListResponse":                 func() proto.Message { return &linodev1.ManagedContactListResponse{} },
		"linode.mcp.v1.ManagedContactWriteResponse":                func() proto.Message { return &linodev1.ManagedContactWriteResponse{} },
		"linode.mcp.v1.ManagedCredential":                          func() proto.Message { return &linodev1.ManagedCredential{} },
		"linode.mcp.v1.ManagedCredentialIDResponse":                func() proto.Message { return &linodev1.ManagedCredentialIDResponse{} },
		"linode.mcp.v1.ManagedCredentialListResponse":              func() proto.Message { return &linodev1.ManagedCredentialListResponse{} },
		"linode.mcp.v1.ManagedCredentialWriteResponse":             func() proto.Message { return &linodev1.ManagedCredentialWriteResponse{} },
		"linode.mcp.v1.ManagedIssue":                               func() proto.Message { return &linodev1.ManagedIssue{} },
		"linode.mcp.v1.ManagedIssueListResponse":                   func() proto.Message { return &linodev1.ManagedIssueListResponse{} },
		"linode.mcp.v1.ManagedLinodeSettings":                      func() proto.Message { return &linodev1.ManagedLinodeSettings{} },
		"linode.mcp.v1.ManagedLinodeSettingsListResponse":          func() proto.Message { return &linodev1.ManagedLinodeSettingsListResponse{} },
		"linode.mcp.v1.ManagedLinodeSettingsWriteResponse":         func() proto.Message { return &linodev1.ManagedLinodeSettingsWriteResponse{} },
		"linode.mcp.v1.ManagedSSHKey":                              func() proto.Message { return &linodev1.ManagedSSHKey{} },
		"linode.mcp.v1.ManagedService":                             func() proto.Message { return &linodev1.ManagedService{} },
		"linode.mcp.v1.ManagedServiceIDResponse":                   func() proto.Message { return &linodev1.ManagedServiceIDResponse{} },
		"linode.mcp.v1.ManagedServiceListResponse":                 func() proto.Message { return &linodev1.ManagedServiceListResponse{} },
		"linode.mcp.v1.ManagedServiceWriteResponse":                func() proto.Message { return &linodev1.ManagedServiceWriteResponse{} },
		"linode.mcp.v1.MessageResponse":                            func() proto.Message { return &linodev1.MessageResponse{} },
		"linode.mcp.v1.MonitorAlertChannelListResponse":            func() proto.Message { return &linodev1.MonitorAlertChannelListResponse{} },
		"linode.mcp.v1.MonitorAlertDefinitionDeleteResponse":       func() proto.Message { return &linodev1.MonitorAlertDefinitionDeleteResponse{} },
		"linode.mcp.v1.MonitorAlertDefinitionListResponse":         func() proto.Message { return &linodev1.MonitorAlertDefinitionListResponse{} },
		"linode.mcp.v1.MonitorAlertDefinitionWriteResponse":        func() proto.Message { return &linodev1.MonitorAlertDefinitionWriteResponse{} },
		"linode.mcp.v1.MonitorDashboardListResponse":               func() proto.Message { return &linodev1.MonitorDashboardListResponse{} },
		"linode.mcp.v1.MonitorService":                             func() proto.Message { return &linodev1.MonitorService{} },
		"linode.mcp.v1.MonitorServiceAlertDefinitionListResponse":  func() proto.Message { return &linodev1.MonitorServiceAlertDefinitionListResponse{} },
		"linode.mcp.v1.MonitorServiceDashboardListResponse":        func() proto.Message { return &linodev1.MonitorServiceDashboardListResponse{} },
		"linode.mcp.v1.MonitorServiceListResponse":                 func() proto.Message { return &linodev1.MonitorServiceListResponse{} },
		"linode.mcp.v1.MonitorServiceMetricDefinitionListResponse": func() proto.Message { return &linodev1.MonitorServiceMetricDefinitionListResponse{} },
		"linode.mcp.v1.MonitorServiceMetricQueryResponse":          func() proto.Message { return &linodev1.MonitorServiceMetricQueryResponse{} },
		"linode.mcp.v1.MonitorServiceTokenCreateResponse":          func() proto.Message { return &linodev1.MonitorServiceTokenCreateResponse{} },
		"linode.mcp.v1.NetworkTransferPriceListResponse":           func() proto.Message { return &linodev1.NetworkTransferPriceListResponse{} },
		"linode.mcp.v1.NetworkingIPAssignWriteResponse":            func() proto.Message { return &linodev1.NetworkingIPAssignWriteResponse{} },
		"linode.mcp.v1.NetworkingIPListResponse":                   func() proto.Message { return &linodev1.NetworkingIPListResponse{} },
		"linode.mcp.v1.NetworkingIPShareWriteResponse":             func() proto.Message { return &linodev1.NetworkingIPShareWriteResponse{} },
		"linode.mcp.v1.NodeBalancer":                               func() proto.Message { return &linodev1.NodeBalancer{} },
		"linode.mcp.v1.NodeBalancerConfig":                         func() proto.Message { return &linodev1.NodeBalancerConfig{} },
		"linode.mcp.v1.NodeBalancerConfigDeleteResponse":           func() proto.Message { return &linodev1.NodeBalancerConfigDeleteResponse{} },
		"linode.mcp.v1.NodeBalancerConfigListResponse":             func() proto.Message { return &linodev1.NodeBalancerConfigListResponse{} },
		"linode.mcp.v1.NodeBalancerConfigNode":                     func() proto.Message { return &linodev1.NodeBalancerConfigNode{} },
		"linode.mcp.v1.NodeBalancerConfigNodeDeleteResponse":       func() proto.Message { return &linodev1.NodeBalancerConfigNodeDeleteResponse{} },
		"linode.mcp.v1.NodeBalancerConfigNodeListResponse":         func() proto.Message { return &linodev1.NodeBalancerConfigNodeListResponse{} },
		"linode.mcp.v1.NodeBalancerConfigNodeWriteResponse":        func() proto.Message { return &linodev1.NodeBalancerConfigNodeWriteResponse{} },
		"linode.mcp.v1.NodeBalancerConfigWriteResponse":            func() proto.Message { return &linodev1.NodeBalancerConfigWriteResponse{} },
		"linode.mcp.v1.NodeBalancerDeleteResponse":                 func() proto.Message { return &linodev1.NodeBalancerDeleteResponse{} },
		"linode.mcp.v1.NodeBalancerListResponse":                   func() proto.Message { return &linodev1.NodeBalancerListResponse{} },
	}
}

func conformanceMessages4() map[string]func() proto.Message {
	return map[string]func() proto.Message{
		"linode.mcp.v1.NodeBalancerStats":                      func() proto.Message { return &linodev1.NodeBalancerStats{} },
		"linode.mcp.v1.NodeBalancerTypeListResponse":           func() proto.Message { return &linodev1.NodeBalancerTypeListResponse{} },
		"linode.mcp.v1.NodeBalancerVPCConfig":                  func() proto.Message { return &linodev1.NodeBalancerVPCConfig{} },
		"linode.mcp.v1.NodeBalancerVPCConfigListResponse":      func() proto.Message { return &linodev1.NodeBalancerVPCConfigListResponse{} },
		"linode.mcp.v1.NodeBalancerWriteResponse":              func() proto.Message { return &linodev1.NodeBalancerWriteResponse{} },
		"linode.mcp.v1.OAuthClient":                            func() proto.Message { return &linodev1.OAuthClient{} },
		"linode.mcp.v1.OAuthClientCreateWriteResponse":         func() proto.Message { return &linodev1.OAuthClientCreateWriteResponse{} },
		"linode.mcp.v1.OAuthClientIDResponse":                  func() proto.Message { return &linodev1.OAuthClientIDResponse{} },
		"linode.mcp.v1.OAuthClientListResponse":                func() proto.Message { return &linodev1.OAuthClientListResponse{} },
		"linode.mcp.v1.OAuthClientSecretResetWriteResponse":    func() proto.Message { return &linodev1.OAuthClientSecretResetWriteResponse{} },
		"linode.mcp.v1.OAuthClientThumbnail":                   func() proto.Message { return &linodev1.OAuthClientThumbnail{} },
		"linode.mcp.v1.OAuthClientWriteResponse":               func() proto.Message { return &linodev1.OAuthClientWriteResponse{} },
		"linode.mcp.v1.ObjectACL":                              func() proto.Message { return &linodev1.ObjectACL{} },
		"linode.mcp.v1.ObjectStorageBucket":                    func() proto.Message { return &linodev1.ObjectStorageBucket{} },
		"linode.mcp.v1.ObjectStorageBucketAccess":              func() proto.Message { return &linodev1.ObjectStorageBucketAccess{} },
		"linode.mcp.v1.ObjectStorageBucketAccessWriteResponse": func() proto.Message { return &linodev1.ObjectStorageBucketAccessWriteResponse{} },
		"linode.mcp.v1.ObjectStorageBucketDeleteResponse":      func() proto.Message { return &linodev1.ObjectStorageBucketDeleteResponse{} },
		"linode.mcp.v1.ObjectStorageBucketListResponse":        func() proto.Message { return &linodev1.ObjectStorageBucketListResponse{} },
		"linode.mcp.v1.ObjectStorageBucketWriteResponse":       func() proto.Message { return &linodev1.ObjectStorageBucketWriteResponse{} },
		"linode.mcp.v1.ObjectStorageEndpointListResponse":      func() proto.Message { return &linodev1.ObjectStorageEndpointListResponse{} },
		"linode.mcp.v1.ObjectStorageKey":                       func() proto.Message { return &linodev1.ObjectStorageKey{} },
		"linode.mcp.v1.ObjectStorageKeyDeleteResponse":         func() proto.Message { return &linodev1.ObjectStorageKeyDeleteResponse{} },
		"linode.mcp.v1.ObjectStorageKeyListResponse":           func() proto.Message { return &linodev1.ObjectStorageKeyListResponse{} },
		"linode.mcp.v1.ObjectStorageKeyWriteResponse":          func() proto.Message { return &linodev1.ObjectStorageKeyWriteResponse{} },
		"linode.mcp.v1.ObjectStorageObjectACLWriteResponse":    func() proto.Message { return &linodev1.ObjectStorageObjectACLWriteResponse{} },
		"linode.mcp.v1.ObjectStorageObjectListResponse":        func() proto.Message { return &linodev1.ObjectStorageObjectListResponse{} },
		"linode.mcp.v1.ObjectStorageQuotaListResponse":         func() proto.Message { return &linodev1.ObjectStorageQuotaListResponse{} },
		"linode.mcp.v1.ObjectStorageQuotaUsage":                func() proto.Message { return &linodev1.ObjectStorageQuotaUsage{} },
		"linode.mcp.v1.ObjectStorageSSLDeleteResponse":         func() proto.Message { return &linodev1.ObjectStorageSSLDeleteResponse{} },
		"linode.mcp.v1.ObjectStorageSSLWriteResponse":          func() proto.Message { return &linodev1.ObjectStorageSSLWriteResponse{} },
		"linode.mcp.v1.ObjectStorageTransfer":                  func() proto.Message { return &linodev1.ObjectStorageTransfer{} },
		"linode.mcp.v1.ObjectStorageTypeListResponse":          func() proto.Message { return &linodev1.ObjectStorageTypeListResponse{} },
		"linode.mcp.v1.PersonalAccessTokenListResponse":        func() proto.Message { return &linodev1.PersonalAccessTokenListResponse{} },
		"linode.mcp.v1.PersonalAccessTokenWriteResponse":       func() proto.Message { return &linodev1.PersonalAccessTokenWriteResponse{} },
		"linode.mcp.v1.PlacementGroup":                         func() proto.Message { return &linodev1.PlacementGroup{} },
		"linode.mcp.v1.PlacementGroupDeleteResponse":           func() proto.Message { return &linodev1.PlacementGroupDeleteResponse{} },
		"linode.mcp.v1.PlacementGroupListResponse":             func() proto.Message { return &linodev1.PlacementGroupListResponse{} },
		"linode.mcp.v1.PlacementGroupWriteResponse":            func() proto.Message { return &linodev1.PlacementGroupWriteResponse{} },
		"linode.mcp.v1.PlanResponse":                           func() proto.Message { return &linodev1.PlanResponse{} },
		"linode.mcp.v1.PresignedURLResponse":                   func() proto.Message { return &linodev1.PresignedURLResponse{} },
		"linode.mcp.v1.Profile":                                func() proto.Message { return &linodev1.Profile{} },
		"linode.mcp.v1.ProfileApp":                             func() proto.Message { return &linodev1.ProfileApp{} },
		"linode.mcp.v1.ProfileAppIDResponse":                   func() proto.Message { return &linodev1.ProfileAppIDResponse{} },
		"linode.mcp.v1.ProfileAppListResponse":                 func() proto.Message { return &linodev1.ProfileAppListResponse{} },
		"linode.mcp.v1.ProfileCanRunResponse":                  func() proto.Message { return &linodev1.ProfileCanRunResponse{} },
		"linode.mcp.v1.ProfileCategoryListResponse":            func() proto.Message { return &linodev1.ProfileCategoryListResponse{} },
		"linode.mcp.v1.ProfileDeviceIDResponse":                func() proto.Message { return &linodev1.ProfileDeviceIDResponse{} },
		"linode.mcp.v1.ProfileDraftAddToolsResponse":           func() proto.Message { return &linodev1.ProfileDraftAddToolsResponse{} },
		"linode.mcp.v1.ProfileDraftDiscardResponse":            func() proto.Message { return &linodev1.ProfileDraftDiscardResponse{} },
		"linode.mcp.v1.ProfileDraftRemoveToolsResponse":        func() proto.Message { return &linodev1.ProfileDraftRemoveToolsResponse{} },
		"linode.mcp.v1.ProfileDraftResponse":                   func() proto.Message { return &linodev1.ProfileDraftResponse{} },
		"linode.mcp.v1.ProfileDraftSaveResponse":               func() proto.Message { return &linodev1.ProfileDraftSaveResponse{} },
		"linode.mcp.v1.ProfileDraftSetResponse":                func() proto.Message { return &linodev1.ProfileDraftSetResponse{} },
		"linode.mcp.v1.ProfileLoginListResponse":               func() proto.Message { return &linodev1.ProfileLoginListResponse{} },
		"linode.mcp.v1.ProfilePreferencesUpdateResponse":       func() proto.Message { return &linodev1.ProfilePreferencesUpdateResponse{} },
		"linode.mcp.v1.ProfileTfaEnableConfirmResponse":        func() proto.Message { return &linodev1.ProfileTfaEnableConfirmResponse{} },
		"linode.mcp.v1.ProfileTfaEnableResponse":               func() proto.Message { return &linodev1.ProfileTfaEnableResponse{} },
		"linode.mcp.v1.ProfileTokenCreateResponse":             func() proto.Message { return &linodev1.ProfileTokenCreateResponse{} },
		"linode.mcp.v1.ProfileTokenIDResponse":                 func() proto.Message { return &linodev1.ProfileTokenIDResponse{} },
		"linode.mcp.v1.ProfileToolListResponse":                func() proto.Message { return &linodev1.ProfileToolListResponse{} },
	}
}

func conformanceMessages5() map[string]func() proto.Message {
	return map[string]func() proto.Message{
		"linode.mcp.v1.Region":                          func() proto.Message { return &linodev1.Region{} },
		"linode.mcp.v1.RegionAvailabilityListResponse":  func() proto.Message { return &linodev1.RegionAvailabilityListResponse{} },
		"linode.mcp.v1.RegionListResponse":              func() proto.Message { return &linodev1.RegionListResponse{} },
		"linode.mcp.v1.SSHKey":                          func() proto.Message { return &linodev1.SSHKey{} },
		"linode.mcp.v1.SSHKeyDeleteResponse":            func() proto.Message { return &linodev1.SSHKeyDeleteResponse{} },
		"linode.mcp.v1.SSHKeyListResponse":              func() proto.Message { return &linodev1.SSHKeyListResponse{} },
		"linode.mcp.v1.SSHKeyWriteResponse":             func() proto.Message { return &linodev1.SSHKeyWriteResponse{} },
		"linode.mcp.v1.SecurityQuestionListResponse":    func() proto.Message { return &linodev1.SecurityQuestionListResponse{} },
		"linode.mcp.v1.StackScript":                     func() proto.Message { return &linodev1.StackScript{} },
		"linode.mcp.v1.StackScriptDeleteResponse":       func() proto.Message { return &linodev1.StackScriptDeleteResponse{} },
		"linode.mcp.v1.StackScriptListResponse":         func() proto.Message { return &linodev1.StackScriptListResponse{} },
		"linode.mcp.v1.StackScriptWriteResponse":        func() proto.Message { return &linodev1.StackScriptWriteResponse{} },
		"linode.mcp.v1.SupportTicket":                   func() proto.Message { return &linodev1.SupportTicket{} },
		"linode.mcp.v1.SupportTicketIDResponse":         func() proto.Message { return &linodev1.SupportTicketIDResponse{} },
		"linode.mcp.v1.SupportTicketListResponse":       func() proto.Message { return &linodev1.SupportTicketListResponse{} },
		"linode.mcp.v1.SupportTicketReplyListResponse":  func() proto.Message { return &linodev1.SupportTicketReplyListResponse{} },
		"linode.mcp.v1.SupportTicketReplyWriteResponse": func() proto.Message { return &linodev1.SupportTicketReplyWriteResponse{} },
		"linode.mcp.v1.SupportTicketWriteResponse":      func() proto.Message { return &linodev1.SupportTicketWriteResponse{} },
		"linode.mcp.v1.TagListResponse":                 func() proto.Message { return &linodev1.TagListResponse{} },
		"linode.mcp.v1.TagWriteResponse":                func() proto.Message { return &linodev1.TagWriteResponse{} },
		"linode.mcp.v1.TaggedObjectListResponse":        func() proto.Message { return &linodev1.TaggedObjectListResponse{} },
		"linode.mcp.v1.TrustedDeviceListResponse":       func() proto.Message { return &linodev1.TrustedDeviceListResponse{} },
		"linode.mcp.v1.VLANDeleteResponse":              func() proto.Message { return &linodev1.VLANDeleteResponse{} },
		"linode.mcp.v1.VLANListResponse":                func() proto.Message { return &linodev1.VLANListResponse{} },
		"linode.mcp.v1.VPCIPListResponse":               func() proto.Message { return &linodev1.VPCIPListResponse{} },
		"linode.mcp.v1.VersionResponse":                 func() proto.Message { return &linodev1.VersionResponse{} },
		"linode.mcp.v1.Volume":                          func() proto.Message { return &linodev1.Volume{} },
		"linode.mcp.v1.VolumeDeleteResponse":            func() proto.Message { return &linodev1.VolumeDeleteResponse{} },
		"linode.mcp.v1.VolumeDetachResponse":            func() proto.Message { return &linodev1.VolumeDetachResponse{} },
		"linode.mcp.v1.VolumeGetResponse":               func() proto.Message { return &linodev1.VolumeGetResponse{} },
		"linode.mcp.v1.VolumeListResponse":              func() proto.Message { return &linodev1.VolumeListResponse{} },
		"linode.mcp.v1.VolumeTypeListResponse":          func() proto.Message { return &linodev1.VolumeTypeListResponse{} },
		"linode.mcp.v1.VolumeWriteResponse":             func() proto.Message { return &linodev1.VolumeWriteResponse{} },
		"linode.mcp.v1.Vpc":                             func() proto.Message { return &linodev1.Vpc{} },
		"linode.mcp.v1.VpcDeleteResponse":               func() proto.Message { return &linodev1.VpcDeleteResponse{} },
		"linode.mcp.v1.VpcListResponse":                 func() proto.Message { return &linodev1.VpcListResponse{} },
		"linode.mcp.v1.VpcSubnet":                       func() proto.Message { return &linodev1.VpcSubnet{} },
		"linode.mcp.v1.VpcSubnetDeleteResponse":         func() proto.Message { return &linodev1.VpcSubnetDeleteResponse{} },
		"linode.mcp.v1.VpcSubnetListResponse":           func() proto.Message { return &linodev1.VpcSubnetListResponse{} },
		"linode.mcp.v1.VpcSubnetWriteResponse":          func() proto.Message { return &linodev1.VpcSubnetWriteResponse{} },
		"linode.mcp.v1.VpcWriteResponse":                func() proto.Message { return &linodev1.VpcWriteResponse{} },
	}
}

type conformanceCase struct {
	Message   string          `json:"message"`
	Input     json.RawMessage `json:"input"`
	Canonical json.RawMessage `json:"canonical"`
}

// TestProtoOutputConformanceCorpus asserts every fixture under
// testdata/conformance: decode the raw-API input into its proto message, marshal
// with the canonical options (MarshalProtoToolResponse), and compare the parsed
// result to the fixture's canonical output. The Python gate asserts the same
// fixtures, so any divergence between the two implementations fails one side.
func TestProtoOutputConformanceCorpus(t *testing.T) {
	t.Parallel()

	const corpusDir = "../../../testdata/conformance"

	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		t.Fatalf("read corpus dir: %v", err)
	}

	registry := conformanceMessages()

	var seen int

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		seen++

		path := filepath.Join(corpusDir, entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()
			assertFixtureConforms(t, path, registry)
		})
	}

	if seen == 0 {
		t.Fatal("no conformance fixtures found")
	}
}

// assertFixtureConforms runs one corpus fixture through the canonical marshaler
// and compares the parsed output to the fixture's canonical block. Comparison is
// on parsed JSON because protojson varies output whitespace by design.
func assertFixtureConforms(t *testing.T, path string, registry map[string]func() proto.Message) {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var testCase conformanceCase
	if err := json.Unmarshal(raw, &testCase); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	ctor, ok := registry[testCase.Message]
	if !ok {
		t.Fatalf("no registered message for %q (add it to conformanceMessages)", testCase.Message)
	}

	message := ctor()
	if err := protojson.Unmarshal(testCase.Input, message); err != nil {
		t.Fatalf("decode input: %v", err)
	}

	result, err := tools.MarshalProtoToolResponse(message)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	text, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("result content is not TextContent")
	}

	var got, want any
	if err := json.Unmarshal([]byte(text.Text), &got); err != nil {
		t.Fatalf("parse marshaled output: %v", err)
	}

	if err := json.Unmarshal(testCase.Canonical, &want); err != nil {
		t.Fatalf("parse canonical: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("canonical mismatch for %s:\n got=%v\nwant=%v", testCase.Message, got, want)
	}
}
