// Package onestepgps implements domain.DeviceProvider against the OneStepGPS
// public device API. The API key lives here and here only: it is read from the
// environment on the server and never reaches the browser.
package onestepgps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fleetview/internal/domain"
)

// maxResponseBytes caps how much of an upstream response we will buffer, so a
// pathological payload cannot exhaust memory.
const maxResponseBytes = 32 << 20 // 32 MiB

// Options configures a Client.
type Options struct {
	BaseURL      string
	APIKey       string
	Timeout      time.Duration
	MaxAttempts  int
	RetryBackoff time.Duration
	SpeedUnit    string
	// HTTPClient is injectable for tests; a tuned client is built when nil.
	HTTPClient *http.Client
}

// Client is a OneStepGPS API client with bounded retries.
type Client struct {
	baseURL      string
	apiKey       string
	speedUnit    string
	maxAttempts  int
	retryBackoff time.Duration
	http         *http.Client
	log          *slog.Logger
	now          func() time.Time
}

// upstreamError carries the HTTP status so retry policy can inspect it.
type upstreamError struct {
	StatusCode int
	Body       string
}

func (e *upstreamError) Error() string {
	body := strings.TrimSpace(e.Body)
	if len(body) > 200 {
		body = body[:200] + "…"
	}
	if body == "" {
		return fmt.Sprintf("upstream returned HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("upstream returned HTTP %d: %s", e.StatusCode, body)
}

// New builds a Client. It never returns an error: an empty API key is a
// deployment problem surfaced by config validation, not by the client.
func New(opts Options, log *slog.Logger) *Client {
	if opts.Timeout <= 0 {
		opts.Timeout = 12 * time.Second
	}
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 3
	}
	if opts.RetryBackoff <= 0 {
		opts.RetryBackoff = 500 * time.Millisecond
	}
	if opts.SpeedUnit == "" {
		opts.SpeedUnit = "km/h"
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.MaxIdleConnsPerHost = 4
		transport.IdleConnTimeout = 90 * time.Second
		httpClient = &http.Client{Timeout: opts.Timeout, Transport: transport}
	}

	return &Client{
		baseURL:      opts.BaseURL,
		apiKey:       opts.APIKey,
		speedUnit:    opts.SpeedUnit,
		maxAttempts:  opts.MaxAttempts,
		retryBackoff: opts.RetryBackoff,
		http:         httpClient,
		log:          log.With("component", "onestepgps"),
		now:          time.Now,
	}
}

// Name implements domain.DeviceProvider.
func (c *Client) Name() string { return "onestepgps" }

// FetchDevices retrieves every device with its latest point, retrying
// transient failures with exponential backoff and full jitter.
func (c *Client) FetchDevices(ctx context.Context) ([]domain.Device, error) {
	var lastErr error

	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		devices, err := c.fetchOnce(ctx)
		if err == nil {
			if attempt > 1 {
				c.log.Info("upstream recovered", "attempt", attempt)
			}
			return devices, nil
		}
		lastErr = err

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !isRetryable(err) || attempt == c.maxAttempts {
			break
		}

		delay := c.backoffFor(attempt)
		c.log.Warn("upstream fetch failed, retrying",
			"attempt", attempt, "max_attempts", c.maxAttempts,
			"retry_in", delay.String(), "error", err)

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	return nil, fmt.Errorf("onestepgps: fetch failed after %d attempt(s): %w", c.maxAttempts, lastErr)
}

func (c *Client) fetchOnce(ctx context.Context) ([]domain.Device, error) {
	endpoint, err := c.endpoint()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "FleetView/1.0 (+https://github.com/fleetview)")

	start := c.now()
	resp, err := c.http.Do(req)
	if err != nil {
		// Never wrap the raw error text without care: it can contain the full
		// URL, and the URL contains the API key.
		return nil, fmt.Errorf("request %s: %w", redactURL(endpoint), redactError(err, c.apiKey))
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", redactError(err, c.apiKey))
	}

	if resp.StatusCode != http.StatusOK {
		upErr := &upstreamError{StatusCode: resp.StatusCode, Body: string(body)}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("%w: %s", domain.ErrUpstreamAuth, upErr.Error())
		}
		return nil, upErr
	}

	var payload deviceListResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	now := c.now()
	devices := make([]domain.Device, 0, len(payload.ResultList))
	skipped := 0
	for _, raw := range payload.ResultList {
		device := raw.toDomain(c.speedUnit, now)
		if device.ID == "" {
			skipped++
			continue
		}
		devices = append(devices, device)
	}
	if skipped > 0 {
		c.log.Warn("skipped devices without an id", "count", skipped)
	}

	c.log.Debug("fetched devices",
		"count", len(devices),
		"duration_ms", c.now().Sub(start).Milliseconds())
	return devices, nil
}

// endpoint builds the request URL with the credentials attached.
func (c *Client) endpoint() (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base url: %w", err)
	}
	q := u.Query()
	q.Set("latest_point", "true")
	if c.apiKey != "" {
		q.Set("api-key", c.apiKey)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// backoffFor returns an exponentially increasing delay with full jitter, which
// prevents a fleet of instances from synchronising their retries.
func (c *Client) backoffFor(attempt int) time.Duration {
	exponent := math.Pow(2, float64(attempt-1))
	capped := time.Duration(float64(c.retryBackoff) * exponent)
	if capped > 10*time.Second {
		capped = 10 * time.Second
	}
	// #nosec G404 -- jitter does not need cryptographic randomness.
	jitter := 0.5 + rand.Float64()/2 // 50%-100% of the computed delay
	return time.Duration(float64(capped) * jitter)
}

// isRetryable decides whether another attempt could plausibly succeed.
func isRetryable(err error) bool {
	if errors.Is(err, domain.ErrUpstreamAuth) {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var upErr *upstreamError
	if errors.As(err, &upErr) {
		switch {
		case upErr.StatusCode == http.StatusTooManyRequests,
			upErr.StatusCode == http.StatusRequestTimeout:
			return true
		case upErr.StatusCode >= 500:
			return true
		default:
			return false
		}
	}
	// Transport-level failures (DNS, connection reset, TLS) are worth a retry.
	return true
}

// redactURL strips the query string so credentials never reach the logs.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<invalid url>"
	}
	return u.Scheme + "://" + u.Host + u.Path
}

// redactError removes the API key from an error produced by net/http, which
// helpfully embeds the whole URL in its messages.
func redactError(err error, apiKey string) error {
	if err == nil || apiKey == "" {
		return err
	}
	msg := err.Error()
	if !strings.Contains(msg, apiKey) {
		return err
	}
	return errors.New(strings.ReplaceAll(msg, apiKey, "***REDACTED***"))
}
