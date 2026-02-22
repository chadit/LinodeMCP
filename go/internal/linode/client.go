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
)

// Client represents a Linode API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

const (
	defaultTimeout     = 30 * time.Second
	defaultIdleTimeout = 30 * time.Second
	maxIdleConns       = 10
	httpBadRequest     = 400
	httpUnauthorized   = 401
	httpForbidden      = 403
	httpTooManyReqs    = 429
	httpServerError    = 500
	authHeaderPrefix   = "Bearer "
	contentTypeJSON    = "application/json"
	// requestTimeout is the per-request context timeout for API calls.
	requestTimeout = 30 * time.Second

	errMsgAuthentication = "Authentication failed. Please check your API token."
	errMsgForbidden      = "Access forbidden. Your API token may not have sufficient permissions."
	errMsgRateLimit      = "Rate limit exceeded. Please try again later."
	errMsgServerError    = "Internal server error. Please try again later."
)

// NewClient creates a new Linode API client.
func NewClient(apiURL, token string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        maxIdleConns,
				MaxIdleConnsPerHost: maxIdleConns,
				IdleConnTimeout:     defaultIdleTimeout,
			},
		},
		baseURL: apiURL,
		token:   token,
	}
}

// makeRequest builds and executes an authenticated HTTP request against the Linode API.
func (c *Client) makeRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Response, error) {
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

	resp, err := c.httpClient.Do(req) //nolint:gosec // G704: baseURL comes from operator config, not from MCP tool parameters — no SSRF risk
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// makeJSONRequest marshals payload as JSON and delegates to makeRequest. A nil payload sends no body.
func (c *Client) makeJSONRequest(ctx context.Context, method, endpoint string, payload any) (*http.Response, error) {
	var body io.Reader

	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}

		body = bytes.NewReader(jsonData)
	}

	return c.makeRequest(ctx, method, endpoint, body)
}

// handleResponse reads the full response body, checks for error status codes, and unmarshals into target.
// The caller is responsible for closing resp.Body before calling this method.
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

func (c *Client) handleErrorResponse(statusCode int, body []byte, resp *http.Response) error {
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
		return &APIError{StatusCode: statusCode, Message: errMsgAuthentication}
	case httpForbidden:
		return &APIError{StatusCode: statusCode, Message: errMsgForbidden}
	case httpTooManyReqs:
		retryAfter := c.parseRetryAfter(resp)

		message := errMsgRateLimit
		if retryAfter > 0 {
			message = fmt.Sprintf("Rate limit exceeded. Retry after %v.", retryAfter)
		}

		return &APIError{StatusCode: statusCode, Message: message}
	case httpServerError:
		return &APIError{StatusCode: statusCode, Message: errMsgServerError}
	default:
		return &APIError{StatusCode: statusCode, Message: fmt.Sprintf("API request failed with status %d", statusCode)}
	}
}

func (*Client) parseRetryAfter(resp *http.Response) time.Duration {
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
