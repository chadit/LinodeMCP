package linode_test

import (
	"context"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// Case names and the operation each one reports under. Every string is
// named because a method name repeats across this package's tables, and a
// plain method and its proto variant share one operation.
const (
	opCloneDomain             = "CloneDomain"
	opCloneDomainProto        = "CloneDomainProto"
	opCreateDomain            = "CreateDomain"
	opCreateDomainProto       = "CreateDomainProto"
	opCreateDomainRecord      = "CreateDomainRecord"
	opCreateDomainRecordProto = "CreateDomainRecordProto"
	opDeleteDomain            = "DeleteDomain"
	opDeleteDomainRecord      = "DeleteDomainRecord"
	opGetDomain               = "GetDomain"
	opGetDomainProto          = "GetDomainProto"
	opGetDomainRecord         = "GetDomainRecord"
	opGetDomainRecordProto    = "GetDomainRecordProto"
	opGetDomainZoneFile       = "GetDomainZoneFile"
	opGetDomainZoneFileProto  = "GetDomainZoneFileProto"
	opImportDomain            = "ImportDomain"
	opImportDomainProto       = "ImportDomainProto"
	opListDomainRecords       = "ListDomainRecords"
	opUpdateDomain            = "UpdateDomain"
	opUpdateDomainProto       = "UpdateDomainProto"
	opUpdateDomainRecord      = "UpdateDomainRecord"
	opUpdateDomainRecordProto = "UpdateDomainRecordProto"
)

// TestRoutedTransportDns checks that each dns method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportDns(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opCloneDomainProto,
			operation: opCloneDomain,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CloneDomainProto(ctx, 4242, &linode.CloneDomainRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateDomainProto,
			operation: opCreateDomain,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateDomainProto(ctx, &linode.CreateDomainRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateDomainRecordProto,
			operation: opCreateDomainRecord,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateDomainRecordProto(ctx, 4242, &linode.CreateDomainRecordRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opDeleteDomain,
			operation: opDeleteDomain,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteDomain(ctx, 4242)
			},
		},
		{
			name:      opDeleteDomainRecord,
			operation: opDeleteDomainRecord,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteDomainRecord(ctx, 4242, 4242)
			},
		},
		{
			name:      opGetDomain,
			operation: opGetDomain,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDomain(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetDomainProto,
			operation: opGetDomain,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDomainProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetDomainRecord,
			operation: opGetDomainRecord,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDomainRecord(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetDomainRecordProto,
			operation: opGetDomainRecord,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDomainRecordProto(ctx, 4242, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetDomainZoneFileProto,
			operation: opGetDomainZoneFile,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetDomainZoneFileProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opImportDomainProto,
			operation: opImportDomain,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ImportDomainProto(ctx, &linode.ImportDomainRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opListDomainRecords,
			operation: opListDomainRecords,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ListDomainRecords(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateDomainProto,
			operation: opUpdateDomain,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateDomainProto(ctx, 4242, &linode.UpdateDomainRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateDomainRecordProto,
			operation: opUpdateDomainRecord,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateDomainRecordProto(ctx, 4242, 4242, &linode.UpdateDomainRecordRequest{})

				return clientRouteError(err)
			},
		},
	})
}
