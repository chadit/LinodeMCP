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
	opCloneMonitorServiceAlertDefinition       = "CloneMonitorServiceAlertDefinition"
	opCloneMonitorServiceAlertDefinitionProto  = "CloneMonitorServiceAlertDefinitionProto"
	opCreateMonitorServiceAlertDefinition      = "CreateMonitorServiceAlertDefinition"
	opCreateMonitorServiceAlertDefinitionProto = "CreateMonitorServiceAlertDefinitionProto"
	opCreateMonitorServiceToken                = "CreateMonitorServiceToken"
	opDeleteMonitorServiceAlertDefinition      = "DeleteMonitorServiceAlertDefinition"
	opGetMonitorDashboard                      = "GetMonitorDashboard"
	opGetMonitorDashboardProto                 = "GetMonitorDashboardProto"
	opGetMonitorService                        = "GetMonitorService"
	opGetMonitorServiceAlertDefinition         = "GetMonitorServiceAlertDefinition"
	opGetMonitorServiceAlertDefinitionProto    = "GetMonitorServiceAlertDefinitionProto"
	opGetMonitorServiceMetrics                 = "GetMonitorServiceMetrics"
	opGetMonitorServiceProto                   = "GetMonitorServiceProto"
	opUpdateMonitorServiceAlertDefinition      = "UpdateMonitorServiceAlertDefinition"
	opUpdateMonitorServiceAlertDefinitionProto = "UpdateMonitorServiceAlertDefinitionProto"
)

// TestRoutedTransportMonitor checks that each monitor method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportMonitor(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opCloneMonitorServiceAlertDefinitionProto,
			operation: opCloneMonitorServiceAlertDefinition,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CloneMonitorServiceAlertDefinitionProto(ctx, "alpha", 4242, &linode.CloneAlertDefinitionRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateMonitorServiceAlertDefinitionProto,
			operation: opCreateMonitorServiceAlertDefinition,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateMonitorServiceAlertDefinitionProto(ctx, "alpha", &linode.CreateAlertDefinitionRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opCreateMonitorServiceToken,
			operation: opCreateMonitorServiceToken,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateMonitorServiceToken(ctx, "alpha", &linode.CreateMonitorServiceTokenRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      opDeleteMonitorServiceAlertDefinition,
			operation: opDeleteMonitorServiceAlertDefinition,
			call: func(ctx context.Context, client *linode.Client) error {
				return client.DeleteMonitorServiceAlertDefinition(ctx, "alpha", 4242)
			},
		},
		{
			name:      opGetMonitorDashboardProto,
			operation: opGetMonitorDashboard,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetMonitorDashboardProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetMonitorServiceAlertDefinition,
			operation: opGetMonitorServiceAlertDefinition,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetMonitorServiceAlertDefinition(ctx, "alpha", 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetMonitorServiceAlertDefinitionProto,
			operation: opGetMonitorServiceAlertDefinition,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetMonitorServiceAlertDefinitionProto(ctx, "alpha", 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetMonitorServiceMetrics,
			operation: opGetMonitorServiceMetrics,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetMonitorServiceMetrics(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetMonitorServiceProto,
			operation: opGetMonitorService,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetMonitorServiceProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opUpdateMonitorServiceAlertDefinitionProto,
			operation: opUpdateMonitorServiceAlertDefinition,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateMonitorServiceAlertDefinitionProto(ctx, "alpha", 4242, &linode.UpdateAlertDefinitionRequest{})

				return clientRouteError(err)
			},
		},
	})
}
