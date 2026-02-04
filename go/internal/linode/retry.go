package linode

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"
)

// RetryConfig holds configuration for retry behavior.
type RetryConfig struct {
	MaxRetries    int
	BaseDelay     time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
	JitterEnabled bool
}

const (
	defaultMaxRetries    = 3
	defaultMaxDelay      = 30 * time.Second
	defaultBackoffFactor = 2.0
	jitterPercent        = 0.1
)

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:    defaultMaxRetries,
		BaseDelay:     time.Second,
		MaxDelay:      defaultMaxDelay,
		BackoffFactor: defaultBackoffFactor,
		JitterEnabled: true,
	}
}

// RetryableClient wraps the basic Client with retry functionality.
type RetryableClient struct {
	*Client

	retryConfig RetryConfig
}

// NewRetryableClient creates a RetryableClient with the given retry configuration.
func NewRetryableClient(apiURL, token string, retryConfig RetryConfig) *RetryableClient {
	return &RetryableClient{
		Client:      NewClient(apiURL, token),
		retryConfig: retryConfig,
	}
}

// NewRetryableClientWithDefaults creates a RetryableClient with default retry settings.
func NewRetryableClientWithDefaults(apiURL, token string) *RetryableClient {
	return NewRetryableClient(apiURL, token, DefaultRetryConfig())
}

// GetProfile retrieves the user profile with automatic retry on transient failures.
func (rc *RetryableClient) GetProfile(ctx context.Context) (*Profile, error) {
	var profile *Profile

	err := rc.executeWithRetry(ctx, "GetProfile", func() error {
		var err error

		profile, err = rc.Client.GetProfile(ctx)

		return err
	})

	return profile, err
}

// ListInstances retrieves all instances with automatic retry on transient failures.
func (rc *RetryableClient) ListInstances(ctx context.Context) ([]Instance, error) {
	var instances []Instance

	err := rc.executeWithRetry(ctx, "ListInstances", func() error {
		var err error

		instances, err = rc.Client.ListInstances(ctx)

		return err
	})

	return instances, err
}

// GetInstance retrieves a single instance by ID with automatic retry on transient failures.
func (rc *RetryableClient) GetInstance(ctx context.Context, instanceID int) (*Instance, error) {
	var instance *Instance

	err := rc.executeWithRetry(ctx, "GetInstance", func() error {
		var err error

		instance, err = rc.Client.GetInstance(ctx, instanceID)

		return err
	})

	return instance, err
}

// GetAccount retrieves the account information with automatic retry on transient failures.
func (rc *RetryableClient) GetAccount(ctx context.Context) (*Account, error) {
	var account *Account

	err := rc.executeWithRetry(ctx, "GetAccount", func() error {
		var err error

		account, err = rc.Client.GetAccount(ctx)

		return err
	})

	return account, err
}

// ListRegions retrieves all regions with automatic retry on transient failures.
func (rc *RetryableClient) ListRegions(ctx context.Context) ([]Region, error) {
	var regions []Region

	err := rc.executeWithRetry(ctx, "ListRegions", func() error {
		var err error

		regions, err = rc.Client.ListRegions(ctx)

		return err
	})

	return regions, err
}

// ListTypes retrieves all Linode types with automatic retry on transient failures.
func (rc *RetryableClient) ListTypes(ctx context.Context) ([]InstanceType, error) {
	var types []InstanceType

	err := rc.executeWithRetry(ctx, "ListTypes", func() error {
		var err error

		types, err = rc.Client.ListTypes(ctx)

		return err
	})

	return types, err
}

// ListVolumes retrieves all volumes with automatic retry on transient failures.
func (rc *RetryableClient) ListVolumes(ctx context.Context) ([]Volume, error) {
	var volumes []Volume

	err := rc.executeWithRetry(ctx, "ListVolumes", func() error {
		var err error

		volumes, err = rc.Client.ListVolumes(ctx)

		return err
	})

	return volumes, err
}

// ListImages retrieves all images with automatic retry on transient failures.
func (rc *RetryableClient) ListImages(ctx context.Context) ([]Image, error) {
	var images []Image

	err := rc.executeWithRetry(ctx, "ListImages", func() error {
		var err error

		images, err = rc.Client.ListImages(ctx)

		return err
	})

	return images, err
}

func (rc *RetryableClient) executeWithRetry(ctx context.Context, _ string, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= rc.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := rc.calculateDelay(attempt)
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		if attempt == rc.retryConfig.MaxRetries {
			break
		}

		if !rc.shouldRetry(err) {
			return err
		}
	}

	return lastErr
}

func (rc *RetryableClient) calculateDelay(attempt int) time.Duration {
	delay := float64(rc.retryConfig.BaseDelay) * math.Pow(rc.retryConfig.BackoffFactor, float64(attempt-1))

	if rc.retryConfig.JitterEnabled {
		jitterMax := big.NewInt(int64(delay * jitterPercent))
		if jitterMax.Int64() > 0 {
			jitterBig, _ := rand.Int(rand.Reader, jitterMax)
			jitter := float64(jitterBig.Int64())
			delay += jitter
		}
	}

	maxDelay := float64(rc.retryConfig.MaxDelay)
	if delay > maxDelay {
		delay = maxDelay
	}

	return time.Duration(delay)
}

func (rc *RetryableClient) shouldRetry(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if apiErr.IsRateLimitError() || apiErr.IsServerError() {
			return true
		}

		if apiErr.IsAuthenticationError() || apiErr.IsForbiddenError() {
			return false
		}
	}

	if IsNetworkError(err) || IsTimeoutError(err) {
		return true
	}

	return false
}
