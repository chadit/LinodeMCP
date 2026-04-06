// Package linode provides a client for interacting with the Linode API v4.
package linode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/chadit/LinodeMCP/internal/appinfo"
	"github.com/chadit/LinodeMCP/internal/config"
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

	// requestTimeout is the per-request context timeout for API calls.
	requestTimeout = 30 * time.Second
)

// Option configures a Client.
type Option func(*retryConfig)

// Client is the Linode API client with built-in retry logic.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	retryCfg   retryConfig
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

// NewClient creates a Linode API client.
// Retry settings layer: hardcoded defaults, then cfg.Resilience values
// (if cfg is non-nil), then caller-supplied options.
func NewClient(apiURL, token string, cfg *config.Config, opts ...Option) *Client {
	retryCfg := defaultRetryConfig()

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
		baseURL:  apiURL,
		token:    token,
		retryCfg: retryCfg,
	}
}

// makeRequest builds and executes an authenticated HTTP request against the Linode API.
// A non-nil payload is marshaled as JSON; nil sends no body.
func (c *Client) makeRequest(ctx context.Context, method, endpoint string, payload any) (*http.Response, error) {
	var body io.Reader

	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}

		body = bytes.NewReader(jsonData)
	}

	rawURL := c.baseURL + endpoint

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid request URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, parsedURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", authHeaderPrefix+c.token)
	req.Header.Set("Content-Type", contentTypeJSON)
	req.Header.Set("User-Agent", "LinodeMCP/"+appinfo.Version)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

func (c *Client) handleResponse(resp *http.Response, target any) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= httpBadRequest {
		return c.handleErrorResponse(resp.StatusCode, body, resp)
	}

	if target != nil {
		if err := json.Unmarshal(body, target); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

func (*Client) handleErrorResponse(statusCode int, body []byte, resp *http.Response) error {
	var apiError struct {
		Errors []struct {
			Field  string `json:"field"`
			Reason string `json:"reason"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &apiError); err == nil && len(apiError.Errors) > 0 {
		return &APIError{
			StatusCode: statusCode,
			Message:    apiError.Errors[0].Reason,
			Field:      apiError.Errors[0].Field,
		}
	}

	switch statusCode {
	case httpUnauthorized:
		return &APIError{StatusCode: statusCode, Message: "authentication failed, check your API token"}
	case httpForbidden:
		return &APIError{StatusCode: statusCode, Message: "access forbidden, your token may lack permissions"}
	case httpTooManyReqs:
		message := "rate limit exceeded, try again later"
		if retryAfter := parseRetryAfter(resp); retryAfter > 0 {
			message = fmt.Sprintf("rate limit exceeded, retry after %v", retryAfter)
		}

		return &APIError{StatusCode: statusCode, Message: message}
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
