package linode

import (
	"context"
	"net/netip"
)

// httpGetIPv6Range retrieves one IPv6 range for the authenticated user; the
// two-stage delete flow fetches drift state through this Go-struct method.
func (c *Client) httpGetIPv6Range(ctx context.Context, ipv6Range string) (*IPv6Range, error) {
	prefix, err := netip.ParsePrefix(ipv6Range)
	if err != nil || !prefix.Addr().Is6() || prefix != prefix.Masked() {
		return nil, ErrIPv6RangeInvalid
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_ipv6_range_get", nil, ipv6Range)
	if err != nil {
		return nil, wrapRequestError("GetIPv6Range", err)
	}

	defer drainClose(resp)

	var response IPv6Range
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}
