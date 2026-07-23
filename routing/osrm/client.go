// Package osrm implements the routing ports (routing.Router,
// routing.MapMatcher, routing.Matrix, routing.Snapper) against a
// self-hosted OSRM instance's HTTP API.
//
// Application code should not import this package outside of the
// composition root that constructs the client; everywhere else, code
// should depend on the routing package's interfaces so a future engine
// (e.g. routing/valhalla) can be substituted without call-site changes.
package osrm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kennankole/drop-spatial-sdk/routing"
)

const defaultProfile = "driving"

var (
	_ routing.Router     = (*Client)(nil)
	_ routing.MapMatcher = (*Client)(nil)
	_ routing.Matrix     = (*Client)(nil)
	_ routing.Snapper    = (*Client)(nil)
)

// Client is an OSRM HTTP API client. It implements routing.Router,
// routing.MapMatcher, routing.Matrix, and routing.Snapper.
type Client struct {
	baseURL    string
	profile    string
	httpClient *http.Client
	retries    int
	userAgent  string
	observer   routing.Observer
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient sets the *http.Client used for requests. Defaults to
// http.DefaultClient's timeout behavior with a 5s timeout applied.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithTimeout sets a request timeout, applied via context.WithTimeout
// around each call. Defaults to 5 seconds. Zero disables the timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if c.httpClient == nil {
			c.httpClient = &http.Client{}
		}
		c.httpClient.Timeout = d
	}
}

// WithRetries sets how many additional attempts are made after a
// transport-level failure or 5xx response, with no backoff between
// attempts (OSRM requests are expected to complete in single-digit
// milliseconds; a slow retry loop indicates the instance is genuinely
// down, not momentarily loaded). Defaults to 1. OSRM's documented "code"
// error responses (NoRoute, NoSegment, etc.) are never retried — they are
// authoritative answers, not failures.
func WithRetries(n int) Option {
	return func(c *Client) { c.retries = n }
}

// WithUserAgent sets the User-Agent header sent with every request.
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// WithObserver sets the routing.Observer notified of every call's outcome.
func WithObserver(obs routing.Observer) Option {
	return func(c *Client) { c.observer = obs }
}

// WithProfile sets the OSRM routing profile (e.g. "driving", "cycling",
// "walking"). Defaults to "driving".
func WithProfile(profile string) Option {
	return func(c *Client) { c.profile = profile }
}

// New returns an OSRM Client targeting baseURL, e.g.
// "http://osrm.internal:5000".
func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL: baseURL,
		profile: defaultProfile,
		retries: 1,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	if c.observer == nil {
		c.observer = routing.NoopObserver
	}
	return c
}

// osrmErrorResponse is the shape of an OSRM error body:
// https://project-osrm.org/docs/v5.24.0/api/#responses
type osrmErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// mapErrorCode maps OSRM's documented error codes to routing sentinel
// errors. Unknown codes map to a wrapped generic error so callers can
// still inspect the original code via errors.Unwrap / the returned string.
func mapErrorCode(code, message string) error {
	switch code {
	case "NoRoute", "NoTrips", "NoMatch":
		return fmt.Errorf("%w: %s", routing.ErrNoRoute, message)
	case "NoSegment":
		return fmt.Errorf("%w: %s", routing.ErrNoSegment, message)
	case "TooBig":
		return fmt.Errorf("%w: %s", routing.ErrTooBig, message)
	case "InvalidValue", "InvalidOptions", "InvalidQuery", "InvalidUrl":
		return fmt.Errorf("%w: %s", routing.ErrInvalidInput, message)
	default:
		return fmt.Errorf("osrm: request failed with code %q: %s", code, message)
	}
}

// doRequest performs an HTTP GET against the OSRM API, retrying on
// transport errors and 5xx responses, and decodes the JSON body into out.
// On a non-2xx response it attempts to decode an osrmErrorResponse and
// returns a mapped sentinel error.
func (c *Client) doRequest(ctx context.Context, url string, out interface{}) error {
	var lastErr error

	attempts := c.retries + 1
	for attempt := 0; attempt < attempts; attempt++ {
		err := c.doRequestOnce(ctx, url, out)
		if err == nil {
			return nil
		}
		lastErr = err

		// Only transport-level/5xx failures (wrapped as ErrUnavailable)
		// are worth retrying; OSRM's own error codes are authoritative
		// and retrying them would just waste a round trip.
		if !isUnavailable(err) {
			return err
		}
	}

	return lastErr
}

func (c *Client) doRequestOnce(ctx context.Context, url string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("osrm: build request: %w", err)
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", routing.ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%w: reading response body: %w", routing.ErrUnavailable, err)
	}

	if resp.StatusCode >= 500 {
		return fmt.Errorf("%w: http %d: %s", routing.ErrUnavailable, resp.StatusCode, string(body))
	}

	// OSRM returns 4xx with a JSON body carrying a "code" field
	// (NoRoute, NoSegment, TooBig, ...) that we map to a sentinel error.
	if resp.StatusCode >= 400 {
		var errResp osrmErrorResponse
		if jsonErr := json.Unmarshal(body, &errResp); jsonErr == nil && errResp.Code != "" {
			return mapErrorCode(errResp.Code, errResp.Message)
		}
		return fmt.Errorf("%w: http %d: %s", routing.ErrUnavailable, resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("osrm: decode response: %w", err)
	}

	return nil
}

func isUnavailable(err error) bool {
	return errors.Is(err, routing.ErrUnavailable)
}
