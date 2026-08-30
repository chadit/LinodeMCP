package linode_test

import (
	"context"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestRoutedTransportAccountPart2 checks that each method below reports a failed
// connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportAccountPart2(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      "CallRouteRawBodyRead",
			operation: thumbnailGetTool,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CallRouteRawBodyRead(ctx, thumbnailGetTool, []any{transportClientAlpha}, "image/png")

				return clientRouteError(err)
			},
		},
	})
}

// TestRoutedTransportAccountPart3 checks that each method below reports a failed
// connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportAccountPart3(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      "CallRouteRawBody",
			operation: thumbnailUpdateTool,
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.CallRouteRawBody(
					ctx, thumbnailUpdateTool, []any{transportClientAlpha}, "image/png", []byte("png"),
				))
			},
		},
	})
}
