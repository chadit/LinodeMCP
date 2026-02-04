// Package linode provides a client for interacting with the Linode API v4.
package linode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/chadit/LinodeMCP/internal/version"
)

// Client represents a Linode API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// ClientInterface defines the interface for Linode API operations.
type ClientInterface interface {
	GetProfile(ctx context.Context) (*Profile, error)
	GetAccount(ctx context.Context) (*Account, error)
	ListInstances(ctx context.Context) ([]Instance, error)
	GetInstance(ctx context.Context, instanceID int) (*Instance, error)
	ListRegions(ctx context.Context) ([]Region, error)
	ListTypes(ctx context.Context) ([]InstanceType, error)
	ListVolumes(ctx context.Context) ([]Volume, error)
	ListImages(ctx context.Context) ([]Image, error)
}

// Profile represents a Linode user profile.
type Profile struct {
	Username           string `json:"username"`
	Email              string `json:"email"`
	Timezone           string `json:"timezone"`
	EmailNotifications bool   `json:"email_notifications"` //nolint:tagliatelle // Linode API snake_case
	Restricted         bool   `json:"restricted"`
	TwoFactorAuth      bool   `json:"two_factor_auth"` //nolint:tagliatelle // Linode API snake_case
	UID                int    `json:"uid"`
}

// Instance represents a Linode instance.
type Instance struct {
	ID              int      `json:"id"`
	Label           string   `json:"label"`
	Status          string   `json:"status"`
	Type            string   `json:"type"`
	Region          string   `json:"region"`
	Image           string   `json:"image"`
	IPv4            []string `json:"ipv4"`
	IPv6            string   `json:"ipv6"`
	Hypervisor      string   `json:"hypervisor"`
	Specs           Specs    `json:"specs"`
	Alerts          Alerts   `json:"alerts"`
	Backups         Backups  `json:"backups"`
	Created         string   `json:"created"`
	Updated         string   `json:"updated"`
	Group           string   `json:"group"`
	Tags            []string `json:"tags"`
	WatchdogEnabled bool     `json:"watchdog_enabled"` //nolint:tagliatelle // Linode API snake_case
}

// Specs represents instance hardware specifications.
type Specs struct {
	Disk     int `json:"disk"`
	Memory   int `json:"memory"`
	VCPUs    int `json:"vcpus"`
	GPUs     int `json:"gpus"`
	Transfer int `json:"transfer"`
}

// Alerts represents alert settings for an instance.
type Alerts struct {
	CPU           int `json:"cpu"`
	NetworkIn     int `json:"network_in"`     //nolint:tagliatelle // Linode API snake_case
	NetworkOut    int `json:"network_out"`    //nolint:tagliatelle // Linode API snake_case
	TransferQuota int `json:"transfer_quota"` //nolint:tagliatelle // Linode API snake_case
	IO            int `json:"io"`
}

// Backups represents backup settings.
type Backups struct {
	Enabled   bool     `json:"enabled"`
	Available bool     `json:"available"`
	Schedule  Schedule `json:"schedule"`
	Last      *Backup  `json:"last_successful"` //nolint:tagliatelle // Linode API snake_case
}

// Schedule represents backup schedule settings.
type Schedule struct {
	Day    string `json:"day"`
	Window string `json:"window"`
}

// Backup represents a backup snapshot.
type Backup struct {
	ID       int    `json:"id"`
	Label    string `json:"label"`
	Status   string `json:"status"`
	Type     string `json:"type"`
	Region   string `json:"region"`
	Created  string `json:"created"`
	Updated  string `json:"updated"`
	Finished string `json:"finished"`
}

// Account represents a Linode account.
type Account struct {
	FirstName         string   `json:"first_name"` //nolint:tagliatelle // Linode API snake_case
	LastName          string   `json:"last_name"`  //nolint:tagliatelle // Linode API snake_case
	Email             string   `json:"email"`
	Company           string   `json:"company"`
	Address1          string   `json:"address_1"` //nolint:tagliatelle // Linode API snake_case
	Address2          string   `json:"address_2"` //nolint:tagliatelle // Linode API snake_case
	City              string   `json:"city"`
	State             string   `json:"state"`
	Zip               string   `json:"zip"`
	Country           string   `json:"country"`
	Phone             string   `json:"phone"`
	Balance           float64  `json:"balance"`
	BalanceUninvoiced float64  `json:"balance_uninvoiced"` //nolint:tagliatelle // Linode API snake_case
	Capabilities      []string `json:"capabilities"`
	ActiveSince       string   `json:"active_since"` //nolint:tagliatelle // Linode API snake_case
	EUUID             string   `json:"euuid"`
	BillingSource     string   `json:"billing_source"`    //nolint:tagliatelle // Linode API snake_case
	ActivePromotions  []Promo  `json:"active_promotions"` //nolint:tagliatelle // Linode API snake_case
}

// Promo represents an active promotion on an account.
type Promo struct {
	Description              string `json:"description"`
	Summary                  string `json:"summary"`
	CreditMonthlyCap         string `json:"credit_monthly_cap"`          //nolint:tagliatelle // Linode API snake_case
	CreditRemaining          string `json:"credit_remaining"`            //nolint:tagliatelle // Linode API snake_case
	ExpireDT                 string `json:"expire_dt"`                   //nolint:tagliatelle // Linode API snake_case
	ImageURL                 string `json:"image_url"`                   //nolint:tagliatelle // Linode API snake_case
	ServiceType              string `json:"service_type"`                //nolint:tagliatelle // Linode API snake_case
	ThisMonthCreditRemaining string `json:"this_month_credit_remaining"` //nolint:tagliatelle // Linode API snake_case
}

// Region represents a Linode region (datacenter).
type Region struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Country      string   `json:"country"`
	Capabilities []string `json:"capabilities"`
	Status       string   `json:"status"`
	Resolvers    Resolver `json:"resolvers"`
	SiteType     string   `json:"site_type"` //nolint:tagliatelle // Linode API snake_case
}

// Resolver represents DNS resolvers for a region.
type Resolver struct {
	IPv4 string `json:"ipv4"`
	IPv6 string `json:"ipv6"`
}

// InstanceType represents a Linode instance type (plan).
type InstanceType struct {
	ID         string  `json:"id"`
	Label      string  `json:"label"`
	Class      string  `json:"class"`
	Disk       int     `json:"disk"`
	Memory     int     `json:"memory"`
	VCPUs      int     `json:"vcpus"`
	GPUs       int     `json:"gpus"`
	NetworkOut int     `json:"network_out"` //nolint:tagliatelle // Linode API snake_case
	Transfer   int     `json:"transfer"`
	Price      Price   `json:"price"`
	Addons     Addons  `json:"addons"`
	Successor  *string `json:"successor"`
}

// Price represents pricing for a Linode type.
type Price struct {
	Hourly  float64 `json:"hourly"`
	Monthly float64 `json:"monthly"`
}

// Addons represents add-on pricing for a Linode type.
type Addons struct {
	Backups BackupsAddon `json:"backups"`
}

// BackupsAddon represents backup add-on pricing.
type BackupsAddon struct {
	Price Price `json:"price"`
}

// Volume represents a Linode block storage volume.
type Volume struct {
	ID             int      `json:"id"`
	Label          string   `json:"label"`
	Status         string   `json:"status"`
	Size           int      `json:"size"`
	Region         string   `json:"region"`
	LinodeID       *int     `json:"linode_id"`       //nolint:tagliatelle // Linode API snake_case
	LinodeLabel    *string  `json:"linode_label"`    //nolint:tagliatelle // Linode API snake_case
	FilesystemPath string   `json:"filesystem_path"` //nolint:tagliatelle // Linode API snake_case
	Tags           []string `json:"tags"`
	Created        string   `json:"created"`
	Updated        string   `json:"updated"`
	HardwareType   string   `json:"hardware_type"` //nolint:tagliatelle // Linode API snake_case
}

// Image represents a Linode image (OS image or custom image).
type Image struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Description  string   `json:"description"`
	Type         string   `json:"type"`
	IsPublic     bool     `json:"is_public"` //nolint:tagliatelle // Linode API snake_case
	Deprecated   bool     `json:"deprecated"`
	Size         int      `json:"size"`
	Vendor       string   `json:"vendor"`
	Status       string   `json:"status"`
	Created      string   `json:"created"`
	CreatedBy    string   `json:"created_by"` //nolint:tagliatelle // Linode API snake_case
	Expiry       *string  `json:"expiry"`
	EOL          *string  `json:"eol"`
	Capabilities []string `json:"capabilities"`
	Tags         []string `json:"tags"`
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

// GetProfile retrieves the authenticated user's profile from the Linode API.
func (c *Client) GetProfile(ctx context.Context) (*Profile, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/profile", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetProfile", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var profile Profile
	if err := c.handleResponse(resp, &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

// ListInstances retrieves all Linode instances for the authenticated user.
func (c *Client) ListInstances(ctx context.Context) ([]Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/linode/instances", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListInstances", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []Instance `json:"data"`
		Page    int        `json:"page"`
		Pages   int        `json:"pages"`
		Results int        `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetInstance retrieves a single Linode instance by its ID.
func (c *Client) GetInstance(ctx context.Context, instanceID int) (*Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/linode/instances/%d", instanceID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstance", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var instance Instance
	if err := c.handleResponse(resp, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

// GetAccount retrieves the authenticated user's account information from the Linode API.
func (c *Client) GetAccount(ctx context.Context) (*Account, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/account", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccount", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var account Account
	if err := c.handleResponse(resp, &account); err != nil {
		return nil, err
	}

	return &account, nil
}

// ListRegions retrieves all available Linode regions.
func (c *Client) ListRegions(ctx context.Context) ([]Region, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/regions", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListRegions", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []Region `json:"data"`
		Page    int      `json:"page"`
		Pages   int      `json:"pages"`
		Results int      `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListTypes retrieves all available Linode instance types.
func (c *Client) ListTypes(ctx context.Context) ([]InstanceType, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/linode/types", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListTypes", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []InstanceType `json:"data"`
		Page    int            `json:"page"`
		Pages   int            `json:"pages"`
		Results int            `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListVolumes retrieves all block storage volumes for the authenticated user.
func (c *Client) ListVolumes(ctx context.Context) ([]Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/volumes", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListVolumes", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []Volume `json:"data"`
		Page    int      `json:"page"`
		Pages   int      `json:"pages"`
		Results int      `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListImages retrieves all available Linode images.
func (c *Client) ListImages(ctx context.Context) ([]Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/images", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListImages", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []Image `json:"data"`
		Page    int     `json:"page"`
		Pages   int     `json:"pages"`
		Results int     `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

//nolint:unparam // method is always GET in Stage 2 but will support POST/PUT/DELETE in future stages
func (c *Client) makeRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", authHeaderPrefix+c.token)
	req.Header.Set("Content-Type", contentTypeJSON)
	req.Header.Set("User-Agent", "LinodeMCP/"+version.Version)

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

func (c *Client) parseRetryAfter(resp *http.Response) time.Duration {
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
