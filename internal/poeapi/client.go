// Package poeapi is a thin client for the official Path of Exile API
// (https://api.pathofexile.com). It funnels every request through a rate
// limiter and attaches the GGG-mandated User-Agent and bearer token.
package poeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// BaseURL is the official API host.
const BaseURL = "https://api.pathofexile.com"

// TokenSource supplies a valid (refreshed) bearer token on demand.
type TokenSource interface {
	AccessToken(ctx context.Context) (string, error)
}

// Client talks to the official API.
type Client struct {
	BaseURL    string
	UserAgent  string
	Tokens     TokenSource
	HTTPClient *http.Client
	Limiter    *Limiter
}

// New builds a client with sane defaults.
func New(userAgent string, tokens TokenSource) *Client {
	return &Client{
		BaseURL:    BaseURL,
		UserAgent:  userAgent,
		Tokens:     tokens,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Limiter:    NewLimiter(),
	}
}

// APIError is returned for non-2xx responses.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("poe api: status %d: %s", e.Status, e.Body)
}

// getJSON issues a GET to path (relative to BaseURL) and decodes the body into
// out. It retries once on a 429 after the limiter-imposed wait.
func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if err := c.Limiter.Wait(ctx); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Accept", "application/json")
		if c.Tokens != nil {
			tok, err := c.Tokens.AccessToken(ctx)
			if err != nil {
				return fmt.Errorf("get access token: %w", err)
			}
			req.Header.Set("Authorization", "Bearer "+tok)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return err
		}
		c.Limiter.Observe(resp)
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = &APIError{Status: resp.StatusCode, Body: strings.TrimSpace(string(body))}
			continue // limiter recorded Retry-After; loop retries after waiting
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return &APIError{Status: resp.StatusCode, Body: strings.TrimSpace(string(body))}
		}
		if out != nil {
			if err := json.Unmarshal(body, out); err != nil {
				return fmt.Errorf("decode %s: %w", path, err)
			}
		}
		return nil
	}
	return lastErr
}
