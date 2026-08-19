package linode_test

import (
	"context"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// Case names and operation labels, named because most of these strings appear
// in more than one case below.
const (
	opGetImage                  = "GetImage"
	opGetImageShareGroup        = "GetImageShareGroup"
	opGetImageShareGroupByToken = "GetImageShareGroupByToken"
	opGetInstance               = "GetInstance"
	opGetStackScript            = "GetStackScript"
	opGetType                   = "GetType"
)

// TestRoutedTransportCompute checks that each compute method below
// reports a failed connection as a *linode.NetworkError naming its own operation.
//
// The table is split in two because one function holding every case trips the
// maintainability-index check on sheer size, not on complexity.
func TestRoutedTransportCompute(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opGetImage,
			operation: opGetImage,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetImage(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      opGetImageShareGroup,
			operation: opGetImageShareGroup,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetImageShareGroup(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetImageShareGroupByToken,
			operation: opGetImageShareGroupByToken,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetImageShareGroupByToken(ctx, "alpha")

				return clientRouteError(err)
			},
		},
	})
}

// TestRoutedTransportComputeMore continues TestRoutedTransportCompute.
func TestRoutedTransportComputeMore(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      opGetInstance,
			operation: opGetInstance,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetInstance(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetStackScript,
			operation: opGetStackScript,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetStackScript(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      opGetType,
			operation: opGetType,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetType(ctx, "alpha")

				return clientRouteError(err)
			},
		},
	})
}
