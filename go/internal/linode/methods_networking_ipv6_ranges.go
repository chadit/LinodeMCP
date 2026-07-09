package linode

import (
	"context"
	"net/http"
	"net/netip"
	"net/url"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// httpGetIPv6Range retrieves one IPv6 range for the authenticated user. The
// two-stage delete flow fetches drift state through this Go-struct method; the
// read tool uses httpGetIPv6RangeProto.
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

// httpGetIPv6RangeProto retrieves one IPv6 range and decodes it into the
// IPv6Range proto element for the proto-backed read path. The single-range
// detail endpoint returns is_bgp and the bound Linode IDs (route_target is a
// list-only field), so the element captures both under DiscardUnknown.
func (c *Client) httpGetIPv6RangeProto(ctx context.Context, ipv6Range string) (*linodev1.IPv6Range, error) {
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

	rangeResult := &linodev1.IPv6Range{}
	if err := c.handleProtoResponse(resp, rangeResult); err != nil {
		return nil, err
	}

	return rangeResult, nil
}

// httpDeleteIPv6Range deletes one IPv6 range for the authenticated user.
func (c *Client) httpDeleteIPv6Range(ctx context.Context, ipv6Range string) error {
	prefix, err := netip.ParsePrefix(ipv6Range)
	if err != nil || !prefix.Addr().Is6() || prefix != prefix.Masked() {
		return ErrIPv6RangeInvalid
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedRange := url.PathEscape(ipv6Range)
	endpoint := endpointNetworkingIPv6Ranges + "/" + encodedRange

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteIPv6Range", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}
