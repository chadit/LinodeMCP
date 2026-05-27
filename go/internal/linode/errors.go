package linode

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// ErrCircuitOpen is returned when the circuit breaker is open and rejecting
// requests. Callers can check this sentinel to distinguish "we never tried"
// from "we tried and the upstream failed".
var ErrCircuitOpen = errors.New("circuit breaker open")

// ErrRateLimitWaitCanceled is returned when a caller's context is canceled
// while a goroutine is blocked waiting for a token. The breaker shouldn't
// count this; it's a caller-side decision, not an upstream-health signal.
var ErrRateLimitWaitCanceled = errors.New("rate limit wait canceled")

// ErrUpdateImageRequestRequired is returned when UpdateImage is called without a request body.
var ErrUpdateImageRequestRequired = errors.New("update image request is required")

// ErrLinodeIDPositive is returned when a Linode ID argument is not positive.
var ErrLinodeIDPositive = errors.New("linode_id must be a positive integer")

// ErrIPv6RangePrefixRange is returned when an IPv6 range prefix length is outside the IPv6 CIDR range.
var ErrIPv6RangePrefixRange = errors.New("prefix_length must be an integer between 1 and 128")

// ErrIPv6RangeRouteTargetInvalid is returned when an IPv6 range route target is not a valid IPv6 address.
var ErrIPv6RangeRouteTargetInvalid = errors.New("route_target must be a valid IPv6 address")

// ErrRegionRequired is returned when a region argument is empty.
var ErrRegionRequired = errors.New("region is required")

// ErrIPAddressRequired is returned when an IP address argument is empty.
var ErrIPAddressRequired = errors.New("IP address is required")

// ErrRDNSRequired is returned when a reverse DNS update has no rdns value.
var ErrRDNSRequired = errors.New("rdns is required")

// ErrIPAddressInvalid is returned when an IP address argument is not a valid IPv4 or IPv6 address.
var ErrIPAddressInvalid = errors.New("IP address must be a valid IPv4 or IPv6 address")

// ErrIPv4AddressInvalid is returned when an IPv4-only route receives a non-IPv4 address.
var ErrIPv4AddressInvalid = errors.New("IP address must be a valid IPv4 address")

// ErrIPAssignmentsRequired is returned when an IP assignment request has no assignments.
var ErrIPAssignmentsRequired = errors.New("at least one IP assignment is required")

// ErrStackScriptIDPositive is returned when a StackScript ID argument is not positive.
var ErrStackScriptIDPositive = errors.New("stackscript_id must be a positive integer")

// ErrStackScriptUpdateRequired is returned when a StackScript update request has no editable fields.
var ErrStackScriptUpdateRequired = errors.New("at least one editable field is required")

// ErrTransferYearPositive is returned when a transfer year argument is not positive.
var ErrTransferYearPositive = errors.New("year must be a positive integer")

// ErrTransferMonthRange is returned when a transfer month argument is outside 1-12.
var ErrTransferMonthRange = errors.New("month must be an integer between 1 and 12")

// ErrConfigIDPositive is returned when a config ID argument is not positive.
var ErrConfigIDPositive = errors.New("config_id must be a positive integer")

// ErrStatsYearRange is returned when a statistics year path argument is out of range.
var ErrStatsYearRange = errors.New("year must be an integer between 2000 and 2037")

// ErrStatsMonthRange is returned when a statistics month path argument is out of range.
var ErrStatsMonthRange = errors.New("month must be an integer between 1 and 12")

// ErrDiskIDPositive is returned when a disk ID argument is not positive.
var ErrDiskIDPositive = errors.New("disk_id must be a positive integer")

// ErrInterfaceIDPositive is returned when an interface ID argument is not positive.
var ErrInterfaceIDPositive = errors.New("interface_id must be a positive integer")

// ErrCreateConfigRequestRequired is returned when CreateInstanceConfig is called without a request body.
var ErrCreateConfigRequestRequired = errors.New("create config request is required")

// ErrUpdateConfigRequestRequired is returned when UpdateInstanceConfig is called without a request body.
var ErrUpdateConfigRequestRequired = errors.New("update config request is required")

// ErrAddConfigInterfaceRequestRequired is returned when AddInstanceConfigInterface is called without a request body.
var ErrAddConfigInterfaceRequestRequired = errors.New("add config interface request is required")

// ErrAddInstanceInterfaceRequestRequired is returned when AddInstanceInterface is called without a request body.
var ErrAddInstanceInterfaceRequestRequired = errors.New("add instance interface request is required")

// ErrUpdateInterfaceSettingsRequestRequired is returned when UpdateInstanceInterfaceSettings is called without a request body.
var ErrUpdateInterfaceSettingsRequestRequired = errors.New("interface settings update request is required")

// ErrUpdateInstanceInterfaceRequestRequired is returned when UpdateInstanceInterface is called without a request body.
var ErrUpdateInstanceInterfaceRequestRequired = errors.New("update instance interface request is required")

// ErrUpdateConfigInterfaceRequestRequired is returned when UpdateInstanceConfigInterface is called without a request body.
var ErrUpdateConfigInterfaceRequestRequired = errors.New("update config interface request is required")

// ErrReorderConfigInterfacesRequestRequired is returned when ReorderInstanceConfigInterfaces is called without a request body.
var ErrReorderConfigInterfacesRequestRequired = errors.New("reorder config interfaces request is required")

// ErrUpdateInstanceFirewallsRequestRequired is returned when UpdateInstanceFirewalls is called without a request body.
var ErrUpdateInstanceFirewallsRequestRequired = errors.New("firewall_ids is required")

// ErrInvalidFirewallTemplateSlug is returned when a firewall template slug is not documented.
var ErrInvalidFirewallTemplateSlug = errors.New("firewall template slug must be one of public or vpc")

// ErrFirewallDeviceIDPositive is returned when a firewall device ID is not positive.
var ErrFirewallDeviceIDPositive = errors.New("device id must be a positive integer")

// ErrFirewallDeviceTypeRequired is returned when a firewall device type is missing.
var ErrFirewallDeviceTypeRequired = errors.New("device type is required")

// ErrInvalidFirewallDeviceType is returned when a firewall device type is not documented.
var ErrInvalidFirewallDeviceType = errors.New("device type must be one of linode, nodebalancer, or linode_interface")

// ErrFirewallIDPositive is returned when a firewall ID argument is not positive.
var ErrFirewallIDPositive = errors.New("firewall_id must be a positive integer")

// ErrFirewallRulesRequired is returned when a firewall rules update request is missing.
var ErrFirewallRulesRequired = errors.New("firewall rules request is required")

// ErrFirewallRuleVersionPositive is returned when a firewall rule version is not positive.
var ErrFirewallRuleVersionPositive = errors.New("version must be a positive integer")

// APIError represents an error returned by the Linode API.
// RetryAfter carries the server's Retry-After hint when present so the retry
// loop can honor it instead of computing its own backoff.
type APIError struct {
	StatusCode int           `json:"status_code"`
	Message    string        `json:"message"`
	Field      string        `json:"field,omitempty"`
	RetryAfter time.Duration `json:"retry_after,omitempty"`
	// Method is the HTTP method of the request that produced this error. It
	// drives retry safety for 5xx responses: a server error on a
	// non-idempotent request (POST) may have been applied before the error
	// surfaced, so it must not be replayed. Not part of the API payload.
	Method string `json:"-"`
}

func (e *APIError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("Linode API error (status %d): %s (field: %s)", e.StatusCode, e.Message, e.Field)
	}

	return fmt.Sprintf("Linode API error (status %d): %s", e.StatusCode, e.Message)
}

// IsAuthenticationError returns true if the status code is 401 Unauthorized.
func (e *APIError) IsAuthenticationError() bool { return e.StatusCode == httpUnauthorized }

// IsRateLimitError returns true if the status code is 429 Too Many Requests.
func (e *APIError) IsRateLimitError() bool { return e.StatusCode == httpTooManyReqs }

// IsForbiddenError returns true if the status code is 403 Forbidden.
func (e *APIError) IsForbiddenError() bool { return e.StatusCode == httpForbidden }

// IsServerError returns true if the status code indicates a server error (5xx).
func (e *APIError) IsServerError() bool {
	return e.StatusCode >= httpServerError && e.StatusCode < httpServerErrorMax
}

// NetworkError represents a network-related error.
type NetworkError struct {
	Operation string
	Err       error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error during %s: %v", e.Operation, e.Err)
}

func (e *NetworkError) Unwrap() error { return e.Err }

// isNetworkError returns true if the error is a network-related error.
func isNetworkError(err error) bool {
	if _, ok := errors.AsType[*NetworkError](err); ok {
		return true
	}

	if _, ok := errors.AsType[net.Error](err); ok {
		return true
	}

	_, ok := errors.AsType[*url.Error](err)

	return ok
}

// isTimeoutError returns true if the error is a timeout error.
func isTimeoutError(err error) bool {
	if netErr, ok := errors.AsType[net.Error](err); ok {
		return netErr.Timeout()
	}

	if urlErr, ok := errors.AsType[*url.Error](err); ok {
		if innerNetErr, ok := errors.AsType[net.Error](urlErr.Err); ok {
			return innerNetErr.Timeout()
		}
	}

	return false
}

// RetryableError represents an error that can be retried.
type RetryableError struct {
	Err        error
	RetryAfter time.Duration
}

func (e *RetryableError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("retryable error (retry after %v): %v", e.RetryAfter, e.Err)
	}

	return fmt.Sprintf("retryable error: %v", e.Err)
}

func (e *RetryableError) Unwrap() error { return e.Err }

// requestError wraps a transport-level failure (timeout, connection reset,
// DNS, refused) with the HTTP method of the request that produced it. The
// method drives retry safety: a transport failure on a non-idempotent request
// (POST) may have reached and been processed by the server before the error
// surfaced locally, so replaying it could duplicate the side effect.
type requestError struct {
	Method string
	Err    error
}

func (e *requestError) Error() string { return "request failed: " + e.Err.Error() }

func (e *requestError) Unwrap() error { return e.Err }

// isIdempotentMethod reports whether replaying a failed request of this HTTP
// method is safe. GET, HEAD, PUT, DELETE, and OPTIONS are idempotent by HTTP
// semantics: a retry converges to the same end state. POST (and PATCH) are
// not, so a failure that might already have been applied must not be retried.
// An unknown or empty method is treated as non-idempotent to stay on the safe
// side.
func isIdempotentMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions:
		return true
	default:
		return false
	}
}

// isRetryable reports whether err is a transient failure worth retrying.
// A rate-limit (429) is always safe to replay because the request was rejected
// before processing. A 5xx or a transport failure may have been applied
// server-side, so those are retried only when the underlying request was
// idempotent.
func isRetryable(err error) bool {
	if _, ok := errors.AsType[*RetryableError](err); ok {
		return true
	}

	if apiErr, ok := errors.AsType[*APIError](err); ok {
		if apiErr.IsRateLimitError() {
			return true
		}

		return apiErr.IsServerError() && isIdempotentMethod(apiErr.Method)
	}

	if isNetworkError(err) || isTimeoutError(err) {
		reqErr, ok := errors.AsType[*requestError](err)

		return ok && isIdempotentMethod(reqErr.Method)
	}

	return false
}
