"""Cross-language proto-output conformance gate.

Each fixture under ``testdata/conformance`` pairs a raw-API ``input`` with the
expected proto-canonical ``canonical`` output. This test runs the input through
``serialize_api_response`` (the exact decode + serialize path the proto-backed
handlers use) and asserts the result matches ``canonical``. The Go gate
(go/internal/tools/proto_conformance_corpus_test.go) asserts the same fixtures,
so Go and Python emit structurally identical output for every covered message.
Comparison is on parsed JSON because protojson varies whitespace by design.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import TYPE_CHECKING, Any

from linodemcp.genpb.linode.mcp.v1 import (
    account_availability_pb2,
    account_beta_program_pb2,
    account_event_pb2,
    account_pb2,
    account_service_transfer_pb2,
    account_user_pb2,
    beta_program_pb2,
    bucket_access_pb2,
    bucket_ssl_pb2,
    common_pb2,
    database_engine_pb2,
    database_instance_pb2,
    database_pb2,
    database_ssl_pb2,
    domain_pb2,
    firewall_device_pb2,
    firewall_pb2,
    image_pb2,
    image_sharegroup_member_pb2,
    image_sharegroup_pb2,
    image_sharegroup_token_pb2,
    instance_pb2,
    ip_pb2,
    kernel_pb2,
    lke_api_endpoint_pb2,
    lke_dashboard_pb2,
    lke_kubeconfig_pb2,
    lke_node_pb2,
    lke_pb2,
    lke_pool_pb2,
    lke_tier_version_pb2,
    lke_version_pb2,
    longview_pb2,
    managed_issue_pb2,
    managed_pb2,
    monitor_pb2,
    nodebalancer_config_node_pb2,
    nodebalancer_config_pb2,
    nodebalancer_pb2,
    nodebalancer_vpc_config_pb2,
    object_acl_pb2,
    object_storage_pb2,
    placement_pb2,
    profile_pb2,
    region_pb2,
    sshkey_pb2,
    stackscript_pb2,
    support_ticket_pb2,
    tag_pb2,
    type_pb2,
    vlan_pb2,
    volume_pb2,
    vpc_pb2,
)
from linodemcp.tools.proto_response import serialize_api_response

if TYPE_CHECKING:
    from google.protobuf.message import Message

# Mirror of conformanceMessages() in the Go gate. Add an entry when a message
# gains a fixture; both languages read the same corpus.
CONFORMANCE_MESSAGES: dict[str, type[Message]] = {
    "linode.mcp.v1.Account": account_pb2.Account,
    "linode.mcp.v1.AccountAgreements": account_pb2.AccountAgreements,
    "linode.mcp.v1.AccountAvailability": account_availability_pb2.AccountAvailability,
    "linode.mcp.v1.AccountAvailabilityListResponse": (
        account_availability_pb2.AccountAvailabilityListResponse
    ),
    "linode.mcp.v1.AccountBetaProgram": account_beta_program_pb2.AccountBetaProgram,
    "linode.mcp.v1.AccountBetaProgramListResponse": (
        account_beta_program_pb2.AccountBetaProgramListResponse
    ),
    "linode.mcp.v1.AccountEntityTransfer": (
        account_service_transfer_pb2.AccountEntityTransfer
    ),
    "linode.mcp.v1.AccountEvent": account_event_pb2.AccountEvent,
    "linode.mcp.v1.AccountEventListResponse": (
        account_event_pb2.AccountEventListResponse
    ),
    "linode.mcp.v1.AccountEventSeenResponse": account_pb2.AccountEventSeenResponse,
    "linode.mcp.v1.AccountInvoice": account_pb2.AccountInvoice,
    "linode.mcp.v1.AccountInvoiceItemListResponse": (
        account_pb2.AccountInvoiceItemListResponse
    ),
    "linode.mcp.v1.AccountInvoiceListResponse": account_pb2.AccountInvoiceListResponse,
    "linode.mcp.v1.AccountLogin": account_pb2.AccountLogin,
    "linode.mcp.v1.AccountLoginListResponse": account_pb2.AccountLoginListResponse,
    "linode.mcp.v1.AccountMaintenanceListResponse": (
        account_pb2.AccountMaintenanceListResponse
    ),
    "linode.mcp.v1.AccountNotificationListResponse": (
        account_pb2.AccountNotificationListResponse
    ),
    "linode.mcp.v1.AccountPayment": account_pb2.AccountPayment,
    "linode.mcp.v1.AccountPaymentListResponse": account_pb2.AccountPaymentListResponse,
    "linode.mcp.v1.AccountPaymentMethodIDResponse": (
        account_pb2.AccountPaymentMethodIDResponse
    ),
    "linode.mcp.v1.AccountPaymentMethodListResponse": (
        account_pb2.AccountPaymentMethodListResponse
    ),
    "linode.mcp.v1.AccountPaymentMethodWriteResponse": (
        account_pb2.AccountPaymentMethodWriteResponse
    ),
    "linode.mcp.v1.AccountPaymentWriteResponse": (
        account_pb2.AccountPaymentWriteResponse
    ),
    "linode.mcp.v1.AccountPromoResponse": account_pb2.AccountPromoResponse,
    "linode.mcp.v1.AccountServiceTransferActionResponse": (
        account_pb2.AccountServiceTransferActionResponse
    ),
    "linode.mcp.v1.AccountServiceTransferListResponse": (
        account_service_transfer_pb2.AccountServiceTransferListResponse
    ),
    "linode.mcp.v1.AccountSettings": account_pb2.AccountSettings,
    "linode.mcp.v1.AccountTransfer": account_pb2.AccountTransfer,
    "linode.mcp.v1.AccountUser": account_user_pb2.AccountUser,
    "linode.mcp.v1.AccountUserDeleteResponse": (
        account_user_pb2.AccountUserDeleteResponse
    ),
    "linode.mcp.v1.AccountUserGrantsWriteResponse": (
        account_user_pb2.AccountUserGrantsWriteResponse
    ),
    "linode.mcp.v1.AccountUserListResponse": account_user_pb2.AccountUserListResponse,
    "linode.mcp.v1.AccountUserWriteResponse": account_user_pb2.AccountUserWriteResponse,
    "linode.mcp.v1.BetaProgram": beta_program_pb2.BetaProgram,
    "linode.mcp.v1.BetaProgramListResponse": beta_program_pb2.BetaProgramListResponse,
    "linode.mcp.v1.BucketSSL": bucket_ssl_pb2.BucketSSL,
    "linode.mcp.v1.ChildAccountListResponse": account_pb2.ChildAccountListResponse,
    "linode.mcp.v1.ConfigInterfaceListResponse": (
        instance_pb2.ConfigInterfaceListResponse
    ),
    "linode.mcp.v1.ConfigInterfaceResponse": instance_pb2.ConfigInterfaceResponse,
    "linode.mcp.v1.ConfigInterfaceWriteResponse": (
        instance_pb2.ConfigInterfaceWriteResponse
    ),
    "linode.mcp.v1.DatabaseEngine": database_engine_pb2.DatabaseEngine,
    "linode.mcp.v1.DatabaseEngineListResponse": (
        database_engine_pb2.DatabaseEngineListResponse
    ),
    "linode.mcp.v1.DatabaseInstanceActionWriteResponse": (
        database_instance_pb2.DatabaseInstanceActionWriteResponse
    ),
    "linode.mcp.v1.DatabaseInstanceListResponse": (
        database_instance_pb2.DatabaseInstanceListResponse
    ),
    "linode.mcp.v1.DatabaseInstanceWriteResponse": (
        database_instance_pb2.DatabaseInstanceWriteResponse
    ),
    "linode.mcp.v1.DatabaseMySQLInstanceListResponse": (
        database_instance_pb2.DatabaseMySQLInstanceListResponse
    ),
    "linode.mcp.v1.DatabasePostgreSQLInstanceListResponse": (
        database_instance_pb2.DatabasePostgreSQLInstanceListResponse
    ),
    "linode.mcp.v1.DatabaseSSL": database_ssl_pb2.DatabaseSSL,
    "linode.mcp.v1.DatabaseType": database_pb2.DatabaseType,
    "linode.mcp.v1.DatabaseTypeListResponse": database_pb2.DatabaseTypeListResponse,
    "linode.mcp.v1.Domain": domain_pb2.Domain,
    "linode.mcp.v1.DomainListResponse": domain_pb2.DomainListResponse,
    "linode.mcp.v1.DomainRecord": domain_pb2.DomainRecord,
    "linode.mcp.v1.DomainRecordListResponse": domain_pb2.DomainRecordListResponse,
    "linode.mcp.v1.Firewall": firewall_pb2.Firewall,
    "linode.mcp.v1.FirewallDevice": firewall_device_pb2.FirewallDevice,
    "linode.mcp.v1.FirewallDeviceListResponse": (
        firewall_device_pb2.FirewallDeviceListResponse
    ),
    "linode.mcp.v1.FirewallListResponse": firewall_pb2.FirewallListResponse,
    "linode.mcp.v1.FirewallRuleVersion": firewall_pb2.FirewallRuleVersion,
    "linode.mcp.v1.FirewallRuleVersionListResponse": (
        firewall_pb2.FirewallRuleVersionListResponse
    ),
    "linode.mcp.v1.FirewallRules": firewall_pb2.FirewallRules,
    "linode.mcp.v1.FirewallRulesWriteResponse": firewall_pb2.FirewallRulesWriteResponse,
    "linode.mcp.v1.FirewallTemplate": firewall_pb2.FirewallTemplate,
    "linode.mcp.v1.FirewallTemplateListResponse": (
        firewall_pb2.FirewallTemplateListResponse
    ),
    "linode.mcp.v1.FirewallWriteResponse": firewall_pb2.FirewallWriteResponse,
    "linode.mcp.v1.IPAddress": ip_pb2.IPAddress,
    "linode.mcp.v1.IPAddressWriteResponse": ip_pb2.IPAddressWriteResponse,
    "linode.mcp.v1.IPv6PoolListResponse": ip_pb2.IPv6PoolListResponse,
    "linode.mcp.v1.IPv6RangeListResponse": ip_pb2.IPv6RangeListResponse,
    "linode.mcp.v1.Image": image_pb2.Image,
    "linode.mcp.v1.ImageListResponse": image_pb2.ImageListResponse,
    "linode.mcp.v1.ImageShareGroup": image_sharegroup_pb2.ImageShareGroup,
    "linode.mcp.v1.ImageShareGroupListResponse": (
        image_sharegroup_pb2.ImageShareGroupListResponse
    ),
    "linode.mcp.v1.ImageShareGroupMember": (
        image_sharegroup_member_pb2.ImageShareGroupMember
    ),
    "linode.mcp.v1.ImageShareGroupMemberListResponse": (
        image_sharegroup_member_pb2.ImageShareGroupMemberListResponse
    ),
    "linode.mcp.v1.ImageShareGroupMemberWriteResponse": (
        image_sharegroup_member_pb2.ImageShareGroupMemberWriteResponse
    ),
    "linode.mcp.v1.ImageShareGroupToken": (
        image_sharegroup_token_pb2.ImageShareGroupToken
    ),
    "linode.mcp.v1.ImageShareGroupTokenListResponse": (
        image_sharegroup_token_pb2.ImageShareGroupTokenListResponse
    ),
    "linode.mcp.v1.ImageShareGroupTokenWriteResponse": (
        image_sharegroup_token_pb2.ImageShareGroupTokenWriteResponse
    ),
    "linode.mcp.v1.ImageShareGroupWriteResponse": (
        image_sharegroup_pb2.ImageShareGroupWriteResponse
    ),
    "linode.mcp.v1.ImageUploadWriteResponse": image_pb2.ImageUploadWriteResponse,
    "linode.mcp.v1.Instance": instance_pb2.Instance,
    "linode.mcp.v1.InstanceActionWriteResponse": (
        instance_pb2.InstanceActionWriteResponse
    ),
    "linode.mcp.v1.InstanceBackup": instance_pb2.InstanceBackup,
    "linode.mcp.v1.InstanceBackupRestoreWriteResponse": (
        instance_pb2.InstanceBackupRestoreWriteResponse
    ),
    "linode.mcp.v1.InstanceBackupWriteResponse": (
        instance_pb2.InstanceBackupWriteResponse
    ),
    "linode.mcp.v1.InstanceBackupsResponse": instance_pb2.InstanceBackupsResponse,
    "linode.mcp.v1.InstanceConfigListResponse": instance_pb2.InstanceConfigListResponse,
    "linode.mcp.v1.InstanceConfigWriteResponse": (
        instance_pb2.InstanceConfigWriteResponse
    ),
    "linode.mcp.v1.InstanceDisk": instance_pb2.InstanceDisk,
    "linode.mcp.v1.InstanceDiskListResponse": instance_pb2.InstanceDiskListResponse,
    "linode.mcp.v1.InstanceDiskResizeWriteResponse": (
        instance_pb2.InstanceDiskResizeWriteResponse
    ),
    "linode.mcp.v1.InstanceDiskWriteResponse": instance_pb2.InstanceDiskWriteResponse,
    "linode.mcp.v1.InstanceIPsResponse": ip_pb2.InstanceIPsResponse,
    "linode.mcp.v1.InstanceInterface": instance_pb2.InstanceInterface,
    "linode.mcp.v1.InstanceInterfaceHistoryListResponse": (
        instance_pb2.InstanceInterfaceHistoryListResponse
    ),
    "linode.mcp.v1.InstanceInterfaceListResponse": (
        instance_pb2.InstanceInterfaceListResponse
    ),
    "linode.mcp.v1.InstanceInterfaceSettings": instance_pb2.InstanceInterfaceSettings,
    "linode.mcp.v1.InstanceListResponse": instance_pb2.InstanceListResponse,
    "linode.mcp.v1.InstanceTypeListResponse": type_pb2.InstanceTypeListResponse,
    "linode.mcp.v1.Kernel": kernel_pb2.Kernel,
    "linode.mcp.v1.KernelListResponse": kernel_pb2.KernelListResponse,
    "linode.mcp.v1.LKEAPIEndpointListResponse": (
        lke_api_endpoint_pb2.LKEAPIEndpointListResponse
    ),
    "linode.mcp.v1.LKECluster": lke_pb2.LKECluster,
    "linode.mcp.v1.LKEClusterListResponse": lke_pb2.LKEClusterListResponse,
    "linode.mcp.v1.LKEDashboard": lke_dashboard_pb2.LKEDashboard,
    "linode.mcp.v1.LKEKubeconfig": lke_kubeconfig_pb2.LKEKubeconfig,
    "linode.mcp.v1.LKENode": lke_node_pb2.LKENode,
    "linode.mcp.v1.LKENodePool": lke_pool_pb2.LKENodePool,
    "linode.mcp.v1.LKENodePoolListResponse": lke_pool_pb2.LKENodePoolListResponse,
    "linode.mcp.v1.LKENodePoolWriteResponse": lke_pool_pb2.LKENodePoolWriteResponse,
    "linode.mcp.v1.LKETierVersion": lke_tier_version_pb2.LKETierVersion,
    "linode.mcp.v1.LKETierVersionListResponse": (
        lke_tier_version_pb2.LKETierVersionListResponse
    ),
    "linode.mcp.v1.LKETypeListResponse": type_pb2.LKETypeListResponse,
    "linode.mcp.v1.LKEVersion": lke_version_pb2.LKEVersion,
    "linode.mcp.v1.LKEVersionListResponse": lke_version_pb2.LKEVersionListResponse,
    "linode.mcp.v1.LongviewClient": longview_pb2.LongviewClient,
    "linode.mcp.v1.LongviewClientCreateWriteResponse": (
        longview_pb2.LongviewClientCreateWriteResponse
    ),
    "linode.mcp.v1.LongviewClientIDResponse": longview_pb2.LongviewClientIDResponse,
    "linode.mcp.v1.LongviewClientListResponse": longview_pb2.LongviewClientListResponse,
    "linode.mcp.v1.LongviewClientWriteResponse": (
        longview_pb2.LongviewClientWriteResponse
    ),
    "linode.mcp.v1.LongviewSubscription": longview_pb2.LongviewSubscription,
    "linode.mcp.v1.LongviewSubscriptionListResponse": (
        longview_pb2.LongviewSubscriptionListResponse
    ),
    "linode.mcp.v1.LongviewSubscriptionWriteResponse": (
        longview_pb2.LongviewSubscriptionWriteResponse
    ),
    "linode.mcp.v1.LongviewTypeListResponse": longview_pb2.LongviewTypeListResponse,
    "linode.mcp.v1.MaintenancePolicyListResponse": (
        account_pb2.MaintenancePolicyListResponse
    ),
    "linode.mcp.v1.ManagedContact": managed_pb2.ManagedContact,
    "linode.mcp.v1.ManagedContactIDResponse": managed_pb2.ManagedContactIDResponse,
    "linode.mcp.v1.ManagedContactListResponse": managed_pb2.ManagedContactListResponse,
    "linode.mcp.v1.ManagedContactWriteResponse": (
        managed_pb2.ManagedContactWriteResponse
    ),
    "linode.mcp.v1.ManagedCredential": managed_pb2.ManagedCredential,
    "linode.mcp.v1.ManagedCredentialIDResponse": (
        managed_pb2.ManagedCredentialIDResponse
    ),
    "linode.mcp.v1.ManagedCredentialListResponse": (
        managed_pb2.ManagedCredentialListResponse
    ),
    "linode.mcp.v1.ManagedCredentialWriteResponse": (
        managed_pb2.ManagedCredentialWriteResponse
    ),
    "linode.mcp.v1.ManagedIssue": managed_issue_pb2.ManagedIssue,
    "linode.mcp.v1.ManagedIssueListResponse": (
        managed_issue_pb2.ManagedIssueListResponse
    ),
    "linode.mcp.v1.ManagedLinodeSettings": managed_pb2.ManagedLinodeSettings,
    "linode.mcp.v1.ManagedLinodeSettingsListResponse": (
        managed_pb2.ManagedLinodeSettingsListResponse
    ),
    "linode.mcp.v1.ManagedLinodeSettingsWriteResponse": (
        managed_pb2.ManagedLinodeSettingsWriteResponse
    ),
    "linode.mcp.v1.ManagedService": managed_pb2.ManagedService,
    "linode.mcp.v1.ManagedServiceIDResponse": managed_pb2.ManagedServiceIDResponse,
    "linode.mcp.v1.ManagedServiceListResponse": managed_pb2.ManagedServiceListResponse,
    "linode.mcp.v1.MessageResponse": common_pb2.MessageResponse,
    "linode.mcp.v1.MonitorAlertChannelListResponse": (
        monitor_pb2.MonitorAlertChannelListResponse
    ),
    "linode.mcp.v1.MonitorAlertDefinitionDeleteResponse": (
        monitor_pb2.MonitorAlertDefinitionDeleteResponse
    ),
    "linode.mcp.v1.MonitorAlertDefinitionListResponse": (
        monitor_pb2.MonitorAlertDefinitionListResponse
    ),
    "linode.mcp.v1.MonitorAlertDefinitionWriteResponse": (
        monitor_pb2.MonitorAlertDefinitionWriteResponse
    ),
    "linode.mcp.v1.MonitorDashboardListResponse": (
        monitor_pb2.MonitorDashboardListResponse
    ),
    "linode.mcp.v1.MonitorService": monitor_pb2.MonitorService,
    "linode.mcp.v1.MonitorServiceAlertDefinitionListResponse": (
        monitor_pb2.MonitorServiceAlertDefinitionListResponse
    ),
    "linode.mcp.v1.MonitorServiceDashboardListResponse": (
        monitor_pb2.MonitorServiceDashboardListResponse
    ),
    "linode.mcp.v1.MonitorServiceListResponse": monitor_pb2.MonitorServiceListResponse,
    "linode.mcp.v1.MonitorServiceMetricDefinitionListResponse": (
        monitor_pb2.MonitorServiceMetricDefinitionListResponse
    ),
    "linode.mcp.v1.MonitorServiceMetricQueryResponse": (
        monitor_pb2.MonitorServiceMetricQueryResponse
    ),
    "linode.mcp.v1.NetworkTransferPriceListResponse": (
        type_pb2.NetworkTransferPriceListResponse
    ),
    "linode.mcp.v1.NetworkingIPAssignWriteResponse": (
        ip_pb2.NetworkingIPAssignWriteResponse
    ),
    "linode.mcp.v1.NetworkingIPListResponse": ip_pb2.NetworkingIPListResponse,
    "linode.mcp.v1.NetworkingIPShareWriteResponse": (
        ip_pb2.NetworkingIPShareWriteResponse
    ),
    "linode.mcp.v1.NodeBalancer": nodebalancer_pb2.NodeBalancer,
    "linode.mcp.v1.NodeBalancerConfig": nodebalancer_config_pb2.NodeBalancerConfig,
    "linode.mcp.v1.NodeBalancerConfigListResponse": (
        nodebalancer_config_pb2.NodeBalancerConfigListResponse
    ),
    "linode.mcp.v1.NodeBalancerConfigNode": (
        nodebalancer_config_node_pb2.NodeBalancerConfigNode
    ),
    "linode.mcp.v1.NodeBalancerConfigNodeListResponse": (
        nodebalancer_config_node_pb2.NodeBalancerConfigNodeListResponse
    ),
    "linode.mcp.v1.NodeBalancerConfigNodeWriteResponse": (
        nodebalancer_config_node_pb2.NodeBalancerConfigNodeWriteResponse
    ),
    "linode.mcp.v1.NodeBalancerConfigWriteResponse": (
        nodebalancer_config_pb2.NodeBalancerConfigWriteResponse
    ),
    "linode.mcp.v1.NodeBalancerListResponse": nodebalancer_pb2.NodeBalancerListResponse,
    "linode.mcp.v1.NodeBalancerTypeListResponse": type_pb2.NodeBalancerTypeListResponse,
    "linode.mcp.v1.NodeBalancerVPCConfig": (
        nodebalancer_vpc_config_pb2.NodeBalancerVPCConfig
    ),
    "linode.mcp.v1.NodeBalancerVPCConfigListResponse": (
        nodebalancer_vpc_config_pb2.NodeBalancerVPCConfigListResponse
    ),
    "linode.mcp.v1.OAuthClient": account_pb2.OAuthClient,
    "linode.mcp.v1.OAuthClientCreateWriteResponse": (
        account_pb2.OAuthClientCreateWriteResponse
    ),
    "linode.mcp.v1.OAuthClientIDResponse": account_pb2.OAuthClientIDResponse,
    "linode.mcp.v1.OAuthClientListResponse": account_pb2.OAuthClientListResponse,
    "linode.mcp.v1.OAuthClientSecretResetWriteResponse": (
        account_pb2.OAuthClientSecretResetWriteResponse
    ),
    "linode.mcp.v1.ObjectACL": object_acl_pb2.ObjectACL,
    "linode.mcp.v1.ObjectStorageBucket": object_storage_pb2.ObjectStorageBucket,
    "linode.mcp.v1.ObjectStorageBucketAccess": (
        bucket_access_pb2.ObjectStorageBucketAccess
    ),
    "linode.mcp.v1.ObjectStorageBucketAccessWriteResponse": (
        bucket_access_pb2.ObjectStorageBucketAccessWriteResponse
    ),
    "linode.mcp.v1.ObjectStorageBucketListResponse": (
        object_storage_pb2.ObjectStorageBucketListResponse
    ),
    "linode.mcp.v1.ObjectStorageEndpointListResponse": (
        object_storage_pb2.ObjectStorageEndpointListResponse
    ),
    "linode.mcp.v1.ObjectStorageKey": object_storage_pb2.ObjectStorageKey,
    "linode.mcp.v1.ObjectStorageKeyListResponse": (
        object_storage_pb2.ObjectStorageKeyListResponse
    ),
    "linode.mcp.v1.ObjectStorageObjectListResponse": (
        object_storage_pb2.ObjectStorageObjectListResponse
    ),
    "linode.mcp.v1.ObjectStorageQuotaListResponse": (
        object_storage_pb2.ObjectStorageQuotaListResponse
    ),
    "linode.mcp.v1.ObjectStorageTypeListResponse": (
        type_pb2.ObjectStorageTypeListResponse
    ),
    "linode.mcp.v1.PersonalAccessTokenListResponse": (
        profile_pb2.PersonalAccessTokenListResponse
    ),
    "linode.mcp.v1.PersonalAccessTokenWriteResponse": (
        profile_pb2.PersonalAccessTokenWriteResponse
    ),
    "linode.mcp.v1.PlacementGroup": placement_pb2.PlacementGroup,
    "linode.mcp.v1.PlacementGroupListResponse": (
        placement_pb2.PlacementGroupListResponse
    ),
    "linode.mcp.v1.Profile": profile_pb2.Profile,
    "linode.mcp.v1.ProfileApp": profile_pb2.ProfileApp,
    "linode.mcp.v1.ProfileAppIDResponse": profile_pb2.ProfileAppIDResponse,
    "linode.mcp.v1.ProfileAppListResponse": profile_pb2.ProfileAppListResponse,
    "linode.mcp.v1.ProfileDeviceIDResponse": profile_pb2.ProfileDeviceIDResponse,
    "linode.mcp.v1.ProfileLoginListResponse": account_pb2.ProfileLoginListResponse,
    "linode.mcp.v1.ProfileTfaEnableResponse": profile_pb2.ProfileTfaEnableResponse,
    "linode.mcp.v1.ProfileTokenCreateResponse": profile_pb2.ProfileTokenCreateResponse,
    "linode.mcp.v1.ProfileTokenIDResponse": profile_pb2.ProfileTokenIDResponse,
    "linode.mcp.v1.Region": region_pb2.Region,
    "linode.mcp.v1.RegionAvailabilityListResponse": (
        region_pb2.RegionAvailabilityListResponse
    ),
    "linode.mcp.v1.RegionListResponse": region_pb2.RegionListResponse,
    "linode.mcp.v1.SSHKey": sshkey_pb2.SSHKey,
    "linode.mcp.v1.SSHKeyListResponse": sshkey_pb2.SSHKeyListResponse,
    "linode.mcp.v1.SSHKeyWriteResponse": sshkey_pb2.SSHKeyWriteResponse,
    "linode.mcp.v1.SecurityQuestionListResponse": (
        profile_pb2.SecurityQuestionListResponse
    ),
    "linode.mcp.v1.StackScript": stackscript_pb2.StackScript,
    "linode.mcp.v1.StackScriptListResponse": stackscript_pb2.StackScriptListResponse,
    "linode.mcp.v1.SupportTicket": support_ticket_pb2.SupportTicket,
    "linode.mcp.v1.SupportTicketIDResponse": support_ticket_pb2.SupportTicketIDResponse,
    "linode.mcp.v1.SupportTicketListResponse": (
        support_ticket_pb2.SupportTicketListResponse
    ),
    "linode.mcp.v1.SupportTicketReplyListResponse": (
        support_ticket_pb2.SupportTicketReplyListResponse
    ),
    "linode.mcp.v1.SupportTicketReplyWriteResponse": (
        support_ticket_pb2.SupportTicketReplyWriteResponse
    ),
    "linode.mcp.v1.SupportTicketWriteResponse": (
        support_ticket_pb2.SupportTicketWriteResponse
    ),
    "linode.mcp.v1.TagListResponse": tag_pb2.TagListResponse,
    "linode.mcp.v1.TaggedObjectListResponse": tag_pb2.TaggedObjectListResponse,
    "linode.mcp.v1.TrustedDeviceListResponse": profile_pb2.TrustedDeviceListResponse,
    "linode.mcp.v1.VLANListResponse": vlan_pb2.VLANListResponse,
    "linode.mcp.v1.VPCIPListResponse": vpc_pb2.VPCIPListResponse,
    "linode.mcp.v1.Volume": volume_pb2.Volume,
    "linode.mcp.v1.VolumeListResponse": volume_pb2.VolumeListResponse,
    "linode.mcp.v1.VolumeTypeListResponse": type_pb2.VolumeTypeListResponse,
    "linode.mcp.v1.Vpc": vpc_pb2.Vpc,
    "linode.mcp.v1.VpcListResponse": vpc_pb2.VpcListResponse,
    "linode.mcp.v1.VpcSubnet": vpc_pb2.VpcSubnet,
    "linode.mcp.v1.VpcSubnetListResponse": vpc_pb2.VpcSubnetListResponse,
}

_CORPUS_DIR = Path(__file__).resolve().parents[3] / "testdata" / "conformance"


def test_proto_output_conformance_corpus() -> None:
    fixtures = sorted(_CORPUS_DIR.glob("*.json"))
    assert fixtures, f"no conformance fixtures in {_CORPUS_DIR}"

    for fixture in fixtures:
        case: dict[str, Any] = json.loads(fixture.read_text())
        full_name: str = case["message"]
        message_cls = CONFORMANCE_MESSAGES.get(full_name)
        assert message_cls is not None, (
            f"{fixture.name}: no registered message for {full_name!r} "
            "(add it to CONFORMANCE_MESSAGES)"
        )

        got = serialize_api_response(case["input"], message_cls())
        want = case["canonical"]
        assert got == want, (
            f"{fixture.name}: canonical mismatch for {full_name}\n"
            f"got={got}\nwant={want}"
        )
