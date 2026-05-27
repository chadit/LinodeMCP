package linode

import (
	"context"
	"net/http"
	"net/netip"
	"net/url"
)

// httpGetIPv6Range retrieves one IPv6 range for the authenticated user.
func (c *Client) httpGetIPv6Range(ctx context.Context, ipv6Range string) (*IPv6Range, error) {
	prefix, err := netip.ParsePrefix(ipv6Range)
	if err != nil || !prefix.Addr().Is6() || prefix != prefix.Masked() {
		return nil, ErrIPv6RangeInvalid
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedRange := url.PathEscape(ipv6Range)
	endpoint := endpointNetworkingIPv6Ranges + "/" + encodedRange

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetIPv6Range", Err: err}
	}

	defer drainClose(resp)

	var response IPv6Range
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}
