package linode_test

import (
	"context"
	"net/http"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesMonitorPart1 pins the monitor client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesMonitorPart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "CloneMonitorServiceAlertDefinitionProto",
			wantVerb: http.MethodPost,
			wantPath: "/monitor/services/alpha/alert-definitions/4242/clone",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CloneMonitorServiceAlertDefinitionProto(ctx, "alpha", 4242, &linode.CloneAlertDefinitionRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateMonitorServiceAlertDefinitionProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathMonitorServicesAlphaAlertDefinitions,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateMonitorServiceAlertDefinitionProto(ctx, "alpha", &linode.CreateAlertDefinitionRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateMonitorServiceToken",
			wantVerb: http.MethodPost,
			wantPath: "/monitor/services/alpha/token",
			response: clientRouteProtoObjToken,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateMonitorServiceToken(ctx, "alpha", &linode.CreateMonitorServiceTokenRequest{})

				return clientRouteProbe(err, func() any { return got.GetToken() })
			},
		},
		{
			name:     "DeleteMonitorServiceAlertDefinition",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathMonitorServicesAlphaAlertDefinitions4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteMonitorServiceAlertDefinition(ctx, "alpha", 4242))
			},
		},
		{
			name:     "GetMonitorDashboardProto",
			wantVerb: http.MethodGet,
			wantPath: "/monitor/dashboards/4242",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetMonitorDashboardProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetMonitorServiceAlertDefinition",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathMonitorServicesAlphaAlertDefinitions4242,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetMonitorServiceAlertDefinition(ctx, "alpha", 4242)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "GetMonitorServiceAlertDefinitionProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathMonitorServicesAlphaAlertDefinitions4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetMonitorServiceAlertDefinitionProto(ctx, "alpha", 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetMonitorServiceProto",
			wantVerb: http.MethodGet,
			wantPath: "/monitor/services/alpha",
			response: clientRouteProtoObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetMonitorServiceProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetLabel() })
			},
		},
		{
			name:     "ListMonitorAlertChannelsProto",
			wantVerb: http.MethodGet,
			wantPath: "/monitor/alert-channels",
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListMonitorAlertChannelsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.MonitorAlertChannel).GetLabel) })
			},
		},
		{
			name:     "ListMonitorAlertDefinitionsProto",
			wantVerb: http.MethodGet,
			wantPath: "/monitor/alert-definitions",
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListMonitorAlertDefinitionsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.MonitorAlertDefinition).GetLabel) })
			},
		},
		{
			name:     "ListMonitorDashboardsProto",
			wantVerb: http.MethodGet,
			wantPath: "/monitor/dashboards",
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListMonitorDashboardsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.MonitorDashboard).GetLabel) })
			},
		},
		{
			name:     "ListMonitorServiceAlertDefinitionsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathMonitorServicesAlphaAlertDefinitions,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListMonitorServiceAlertDefinitionsProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.MonitorAlertDefinition).GetLabel) })
			},
		},
	})
}

// TestClientRoutesMonitorPart2 pins the monitor client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesMonitorPart2(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "ListMonitorServiceDashboardsProto",
			wantVerb: http.MethodGet,
			wantPath: "/monitor/services/alpha/dashboards",
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListMonitorServiceDashboardsProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.MonitorDashboard).GetLabel) })
			},
		},
		{
			name:     "ListMonitorServiceMetricDefinitionsProto",
			wantVerb: http.MethodGet,
			wantPath: "/monitor/services/alpha/metric-definitions",
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListMonitorServiceMetricDefinitionsProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.MonitorMetricDefinition).GetLabel) })
			},
		},
		{
			name:     "ListMonitorServicesProto",
			wantVerb: http.MethodGet,
			wantPath: "/monitor/services",
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListMonitorServicesProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.MonitorService).GetLabel) })
			},
		},
		{
			name:     "UpdateMonitorServiceAlertDefinitionProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathMonitorServicesAlphaAlertDefinitions4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateMonitorServiceAlertDefinitionProto(ctx, "alpha", 4242, &linode.UpdateAlertDefinitionRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
	})
}
