package linode_test

import (
	"context"
	"net/http"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesManagedPart1 pins the Managed services client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesManagedPart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "CreateManagedContactProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathManagedContacts,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateManagedContactProto(ctx, &linode.CreateManagedContactRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateManagedServiceProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathManagedServices,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateManagedServiceProto(ctx, &linode.CreateManagedServiceRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "DeleteManagedContact",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathManagedContacts4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteManagedContact(ctx, 4242))
			},
		},
		{
			name:     "DeleteManagedService",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathManagedServices4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteManagedService(ctx, 4242))
			},
		},
		{
			name:     "DisableManagedService",
			wantVerb: http.MethodPost,
			wantPath: "/managed/services/4242/disable",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DisableManagedService(ctx, 4242))
			},
		},
		{
			name:     "EnableManagedService",
			wantVerb: http.MethodPost,
			wantPath: "/managed/services/4242/enable",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.EnableManagedService(ctx, 4242))
			},
		},
		{
			name:     "GetManagedContact",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathManagedContacts4242,
			response: clientRouteObjName,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetManagedContact(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Name })
			},
		},
		{
			name:     "GetManagedContactProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathManagedContacts4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetManagedContactProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetManagedIssueProto",
			wantVerb: http.MethodGet,
			wantPath: "/managed/issues/4242",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetManagedIssueProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetManagedLinodeSettings",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathManagedLinodeSettings4242,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetManagedLinodeSettings(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "GetManagedLinodeSettingsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathManagedLinodeSettings4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetManagedLinodeSettingsProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetManagedService",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathManagedServices4242,
			response: clientRouteObjStatus,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetManagedService(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Status })
			},
		},
	})
}

// TestClientRoutesManagedPart2 pins the Managed services client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesManagedPart2(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetManagedServiceProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathManagedServices4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetManagedServiceProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetManagedStats",
			wantVerb: http.MethodGet,
			wantPath: "/managed/stats",
			response: clientRouteObjProbe,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetManagedStats(ctx)

				return clientRouteProbe(err, func() any { return got[clientRouteProbeKey] })
			},
		},
		{
			name:     "ListManagedContactsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathManagedContacts,
			response: clientRouteProtoPageName,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListManagedContactsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.ManagedContact).GetName) })
			},
		},
		{
			name:     "ListManagedIssuesProto",
			wantVerb: http.MethodGet,
			wantPath: "/managed/issues",
			response: clientRouteProtoPageCreated,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListManagedIssuesProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.ManagedIssue).GetCreated) })
			},
		},
		{
			name:     "ListManagedLinodeSettingsProto",
			wantVerb: http.MethodGet,
			wantPath: "/managed/linode-settings",
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListManagedLinodeSettingsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.ManagedLinodeSettings).GetLabel) })
			},
		},
		{
			name:     "ListManagedServicesProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathManagedServices,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListManagedServicesProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.ManagedService).GetLabel) })
			},
		},
		{
			name:     "UpdateManagedContactProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathManagedContacts4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateManagedContactProto(ctx, 4242, linode.UpdateManagedContactRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdateManagedLinodeSettingsProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathManagedLinodeSettings4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateManagedLinodeSettingsProto(ctx, 4242, linode.UpdateManagedLinodeSettingsRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdateManagedServiceProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathManagedServices4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateManagedServiceProto(ctx, 4242, &linode.UpdateManagedServiceRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
	})
}
