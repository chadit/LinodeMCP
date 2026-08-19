// Package linode provides a client for interacting with the Linode API v4.
package linode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/chadit/LinodeMCP/go/internal/appinfo"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linoderoute"
)

const (
	defaultTimeout     = 30 * time.Second
	defaultIdleTimeout = 30 * time.Second
	maxIdleConns       = 10
	httpBadRequest     = 400
	httpUnauthorized   = 401
	httpForbidden      = 403
	httpTooManyReqs    = 429
	httpServerError    = 500
	httpServerErrorMax = 600
	authHeaderPrefix   = "Bearer "
	contentTypeJSON    = "application/json"
	contentTypePNG     = "image/png"

	// requestTimeout is the per-request context timeout for API calls.
	requestTimeout = 30 * time.Second
)

// Option configures a Client.
type Option func(*retryConfig)

// Client is the Linode API client with built-in retry, a token-bucket rate
// limiter, and a circuit breaker that trips after sustained upstream failure.
type Client struct {
	httpClient *http.Client
	circuit    *CircuitBreaker
	limiter    *RateLimiter
	baseURL    string
	token      string
	// objectStorage rides the client because the Object Storage execute hook is
	// handed a client and no config, and widening that hook signature is an
	// emitter change. The client is already the resolved-per-call object every
	// other setting arrives through, so the data-plane budgets sit beside the
	// retry policy rather than in a second channel.
	objectStorage config.ObjectStorageConfig
	retryCfg      retryConfig
}

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(n int) Option {
	return func(rc *retryConfig) { rc.MaxRetries = n }
}

// WithBaseDelay sets the initial delay between retries.
func WithBaseDelay(d time.Duration) Option {
	return func(rc *retryConfig) { rc.BaseDelay = d }
}

// WithMaxDelay sets the upper bound on retry delay.
func WithMaxDelay(d time.Duration) Option {
	return func(rc *retryConfig) { rc.MaxDelay = d }
}

// WithBackoffFactor sets the exponential backoff multiplier.
func WithBackoffFactor(f float64) Option {
	return func(rc *retryConfig) { rc.BackoffFactor = f }
}

// WithJitter enables or disables jitter on retry delays.
func WithJitter(enabled bool) Option {
	return func(rc *retryConfig) { rc.JitterEnabled = enabled }
}

// NewClient creates a Linode API client. Retry settings layer in order:
// hardcoded defaults, cfg.Resilience values when cfg is non-nil, then
// caller-supplied options.
func NewClient(apiURL, token string, cfg *config.Config, opts ...Option) *Client {
	retryCfg := defaultRetryConfig()

	var (
		cbThreshold   int
		cbTimeout     time.Duration
		rateLimit     int
		objectStorage config.ObjectStorageConfig
	)

	if cfg != nil {
		if cfg.Resilience.MaxRetries > 0 {
			retryCfg.MaxRetries = cfg.Resilience.MaxRetries
		}

		if cfg.Resilience.BaseRetryDelay > 0 {
			retryCfg.BaseDelay = cfg.Resilience.BaseRetryDelay
		}

		if cfg.Resilience.MaxRetryDelay > 0 {
			retryCfg.MaxDelay = cfg.Resilience.MaxRetryDelay
		}

		cbThreshold = cfg.Resilience.CircuitBreakerThreshold
		cbTimeout = cfg.Resilience.CircuitBreakerTimeout
		rateLimit = cfg.Resilience.RateLimitPerMinute
		objectStorage = cfg.ObjectStorage
	}

	for _, opt := range opts {
		opt(&retryCfg)
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        maxIdleConns,
				MaxIdleConnsPerHost: maxIdleConns,
				IdleConnTimeout:     defaultIdleTimeout,
			},
		},
		baseURL:       apiURL,
		token:         token,
		retryCfg:      retryCfg,
		objectStorage: objectStorage,
		circuit:       NewCircuitBreaker(cbThreshold, cbTimeout),
		limiter:       NewRateLimiter(rateLimit),
	}
}

// ObjectStorage reports the data-plane budgets this client was built with, for
// the transfers that follow a presigned URL instead of calling a route.
func (c *Client) ObjectStorage() config.ObjectStorageConfig {
	return c.objectStorage
}

// makeRequest builds and executes an authenticated HTTP request against the
// Linode API. A non-nil payload is marshaled as JSON; nil sends no body. The
// rate limiter gates every invocation, so the bucket drains per network attempt
// (initial plus retries), not per logical operation.
func (c *Client) makeRequest(ctx context.Context, method, endpoint string, payload any) (*http.Response, error) {
	var body io.Reader

	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}

		body = bytes.NewReader(jsonData)
	}

	return c.makeRequestWithContentType(ctx, method, endpoint, body, contentTypeJSON)
}

// onSurface answers the client that addresses one API surface, which is how a
// route declaring a non-default surface reaches it without any request
// primitive learning about surfaces. The copy shares the transport, the rate
// limiter, and the circuit breaker, so a beta call spends the same budget and
// trips the same breaker as everything else.
//
// The default surface is not special-cased: BaseFor answers the configured base
// unchanged for it, so every unannotated call addresses exactly what it did
// before surfaces existed, and the branch that would say so cannot go stale.
func (c *Client) onSurface(segment string) *Client {
	surfaced := *c
	surfaced.baseURL = linoderoute.BaseFor(c.baseURL, segment)

	return &surfaced
}

// makeRouteRequest resolves a tool's method and path from the proto contract,
// so a route is declared once on its input message instead of once per client
// method here and again in the Python client. values fill the path template's
// slots in declared order. Resolution failures return before any request is
// built: a path assembled from the wrong pieces still addresses a real resource.
func (c *Client) makeRouteRequest(ctx context.Context, tool string, payload any, values ...any) (*http.Response, error) {
	method, endpoint, segment, err := routedRequest(tool, "", values)
	if err != nil {
		return nil, err
	}

	return c.onSurface(segment).makeRequest(ctx, method, endpoint, payload)
}

// makeRouteRequestQuery is makeRouteRequest for a call site that also sends an
// already-encoded query string. It calls makeRequest directly: cmd/route-dump
// only recognizes a routed primitive one hop from the request layer, so
// delegating here would lose route evidence.
func (c *Client) makeRouteRequestQuery(
	ctx context.Context,
	tool, rawQuery string,
	payload any,
	values ...any,
) (*http.Response, error) {
	method, endpoint, segment, err := routedRequest(tool, rawQuery, values)
	if err != nil {
		return nil, err
	}

	return c.onSurface(segment).makeRequest(ctx, method, endpoint, payload)
}

// makeRouteRequestContentType is makeRouteRequest for a call site that sends a
// body the JSON marshaller cannot produce, a multipart form or raw image bytes.
// Like makeRouteRequestQuery it calls the content-type request builder directly
// to stay one hop from the request layer, which is all cmd/route-dump reads as
// route evidence.
func (c *Client) makeRouteRequestContentType(
	ctx context.Context,
	tool, contentType string,
	body io.Reader,
	values ...any,
) (*http.Response, error) {
	method, endpoint, segment, err := routedRequest(tool, "", values)
	if err != nil {
		return nil, err
	}

	return c.onSurface(segment).makeRequestWithContentType(ctx, method, endpoint, body, contentType)
}

// routedRequest resolves the method and path a tool declares and attaches the
// query the call site composed. It is the only place in this package that turns
// a tool name into a request target, so every contract failure reads the same
// and stays distinguishable from a transport failure.
func routedRequest(tool, rawQuery string, values []any) (string, string, string, error) {
	method, endpoint, segment, err := linoderoute.Resolve(tool, values...)
	if err != nil {
		return "", "", "", fmt.Errorf("route request: %w", err)
	}

	return method, withRawQuery(endpoint, rawQuery), segment, nil
}

// withRawQuery attaches an already-encoded query string to a resolved path. The
// first parameter is named path, not endpoint, because cmd/route-dump reads
// "endpoint" as the mark of a request primitive and this function only joins two
// strings.
func withRawQuery(path, rawQuery string) string {
	if rawQuery == "" {
		return path
	}

	return path + "?" + rawQuery
}

func (c *Client) makeRequestWithContentType(ctx context.Context, method, endpoint string, body io.Reader, contentType string) (*http.Response, error) {
	return c.makeSurfacedRequestWithContentType(ctx, c.baseURL, method, endpoint, body, contentType)
}

func (c *Client) makeSurfacedRequestWithContentType(
	ctx context.Context,
	base, method, endpoint string,
	body io.Reader,
	contentType string,
) (*http.Response, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	rawURL := base + endpoint

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid request URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, parsedURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", authHeaderPrefix+c.token)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "LinodeMCP/"+appinfo.Version)

	start := time.Now()
	resp, err := c.httpClient.Do(req)

	if recorder := apiRecorderFromContext(ctx); recorder != nil {
		var status int
		if resp != nil {
			status = resp.StatusCode
		}

		recorder.RecordAPIRequest(ctx, metricsEndpoint(endpoint), method, status, time.Since(start).Seconds())
	}

	if err != nil {
		// Carry the method so the retry layer can tell whether replaying this
		// failed request is safe (idempotent) or risks a duplicate side effect.
		return nil, &requestError{Method: method, Err: err}
	}

	return resp, nil
}

// drainClose closes a response body. A close error is not actionable for the
// caller but can signal a transport or connection-reuse problem, so it is logged
// at warn on stderr, which keeps the stdio MCP channel clean.
func drainClose(resp *http.Response) {
	if err := resp.Body.Close(); err != nil {
		slog.Warn("failed to close response body", "error", err)
	}
}

func (c *Client) handleResponse(resp *http.Response, target any) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= httpBadRequest {
		err := c.handleErrorResponse(resp.StatusCode, body, resp)

		// Stamp the method onto the API error here, once, rather than at every
		// APIError construction site, so the retry layer can decide whether a
		// 5xx is safe to replay.
		if apiErr, ok := errors.AsType[*APIError](err); ok && resp.Request != nil {
			apiErr.Method = resp.Request.Method
		}

		return err
	}

	if target != nil {
		if err := json.Unmarshal(body, target); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// handleProtoResponse mirrors handleResponse for the proto-backed read path,
// decoding with protojson and discarding fields the message does not model,
// since the Linode API may return more fields than a message declares.
func (c *Client) handleProtoResponse(resp *http.Response, msg proto.Message) error {
	_, err := c.handleProtoResponseSubject(resp, "", msg)

	return err
}

// handleProtoResponseSubject also answers with the body it decoded, for the
// tools whose answer restores what the decode drops. subject names the call in
// the report when the API answers with something that is not a JSON object,
// which a decoder's own complaint cannot do. An empty subject skips that check,
// which is the read path, where no tool pins the wording.
func (c *Client) handleProtoResponseSubject(
	resp *http.Response, subject string, msg proto.Message,
) (json.RawMessage, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= httpBadRequest {
		apiErr := c.handleErrorResponse(resp.StatusCode, body, resp)
		if typed, ok := errors.AsType[*APIError](apiErr); ok && resp.Request != nil {
			typed.Method = resp.Request.Method
		}

		return nil, apiErr
	}

	// A free-form payload is the one shape a JSON null has an exact reading in:
	// the API sent no members, which is the empty object. protojson refuses the
	// literal itself, and on any other message a null would be a guess at what
	// the API meant, so it stays refused there.
	if isFreeForm(msg) && isNullBody(body) {
		body = []byte("{}")
	}

	// Only a body that parsed reaches the shape check: bytes that are not JSON
	// at all are the decoder's to report, and Python draws the same line, so a
	// truncated answer names its position in both languages.
	if subject != "" && json.Valid(body) && !IsObjectBody(body) {
		return nil, fmt.Errorf("%s %w", subject, ErrWriteResponseNotObject)
	}

	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(body, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal proto response: %w", err)
	}

	return body, nil
}

// isNullBody reports whether an answer is the JSON null literal and nothing
// else, which is how the API says it sent no members at all.
func isNullBody(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

// isFreeForm reports whether a decode target is the well-known Struct, the
// shape a route with no schema to model answers into.
func isFreeForm(msg proto.Message) bool {
	_, ok := msg.(*structpb.Struct)

	return ok
}

func (*Client) handleErrorResponse(statusCode int, body []byte, resp *http.Response) error {
	var apiError struct {
		Errors []struct {
			Field  string `json:"field"`
			Reason string `json:"reason"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &apiError); err == nil && len(apiError.Errors) > 0 {
		// A 429 with a populated errors[] body takes this branch, so it has to
		// carry the Retry-After hint too or the retry loop falls back to
		// exponential backoff.
		return &APIError{
			StatusCode: statusCode,
			Message:    apiError.Errors[0].Reason,
			Field:      apiError.Errors[0].Field,
			RetryAfter: parseRetryAfter(resp),
		}
	}

	switch statusCode {
	case httpUnauthorized:
		return &APIError{StatusCode: statusCode, Message: "authentication failed, check your API token"}
	case httpForbidden:
		return &APIError{StatusCode: statusCode, Message: "access forbidden, your token may lack permissions"}
	case httpTooManyReqs:
		message := "rate limit exceeded, try again later"
		retryAfter := parseRetryAfter(resp)

		if retryAfter > 0 {
			message = fmt.Sprintf("rate limit exceeded, retry after %v", retryAfter)
		}

		return &APIError{StatusCode: statusCode, Message: message, RetryAfter: retryAfter}
	case httpServerError:
		return &APIError{StatusCode: statusCode, Message: "internal server error, try again later"}
	default:
		return &APIError{StatusCode: statusCode, Message: fmt.Sprintf("API request failed with status %d", statusCode)}
	}
}

func parseRetryAfter(resp *http.Response) time.Duration {
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		return time.Duration(seconds) * time.Second
	}

	if t, err := http.ParseTime(retryAfter); err == nil {
		return time.Until(t)
	}

	return 0
}

// routedGet resolves the tool's contracted route, issues the request, and
// decodes the answer into one T, so a typed getter states only its tool,
// operation, and path values. It resolves the route and calls makeRequest
// itself because cmd/route-dump reads a tool-carrying builder only one hop
// from the request layer.
func routedGet[T any](ctx context.Context, client *Client, operation, tool string, values ...any) (*T, error) {
	method, endpoint, segment, err := routedRequest(tool, "", values)
	if err != nil {
		return nil, wrapRequestError(operation, err)
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := client.onSurface(segment).makeRequest(ctx, method, endpoint, nil)
	if err != nil {
		return nil, wrapRequestError(operation, err)
	}

	defer drainClose(resp)

	var out T
	if err := client.handleResponse(resp, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// listData unwraps a fetched paginated envelope, passing a fetch error
// through, so list methods can ride routedGet without restating the unwrap.
func listData[T any](response *PaginatedResponse[T], err error) ([]T, error) {
	if err != nil {
		return nil, err
	}

	return response.Data, nil
}
