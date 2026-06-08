package trade2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/peteshima/poe-fissure/internal/poeapi"
)

// DefaultBaseURL is the live trade host.
const DefaultBaseURL = "https://www.pathofexile.com"

// MaxFetch is the trade API's cap on listing ids per fetch request.
const MaxFetch = 10

// ErrSessionExpired indicates the POESESSID is missing or invalid (401/403). The
// caller should prompt the user to re-paste their session cookie.
var ErrSessionExpired = errors.New("trade session expired (update POESESSID)")

// Client performs on-demand, bounded trade lookups. Search and fetch have
// independent limiters because GGG advertises separate rate-limit policies for
// each endpoint. Calls are sequential; never fan out concurrent trade requests.
type Client struct {
	BaseURL   string
	UserAgent string
	POESESSID string
	HTTP      *http.Client

	search *poeapi.Limiter
	fetch  *poeapi.Limiter
}

// New builds a client. poesessid may be empty (anonymous), but PoE2 trade is
// typically login-gated, so a valid cookie is expected.
func New(userAgent, poesessid string) *Client {
	return &Client{
		BaseURL:   DefaultBaseURL,
		UserAgent: userAgent,
		POESESSID: poesessid,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		search:    poeapi.NewLimiter(),
		fetch:     poeapi.NewLimiter(),
	}
}

// PriceCheck runs one search then fetches up to max cheapest listings. With the
// query sorted by price ascending, the first results are the cheapest. Returns
// the listings and the total match count. This is the only entry point callers
// should use; it bounds the request volume to 1 search + ⌈max/10⌉ fetches.
func (c *Client) PriceCheck(ctx context.Context, realm, league string, body []byte, max int) ([]Listing, int, error) {
	sr, err := c.Search(ctx, realm, league, body)
	if err != nil {
		return nil, 0, err
	}
	ids := sr.Result
	if len(ids) > max {
		ids = ids[:max]
	}
	if len(ids) == 0 {
		return nil, sr.Total, nil
	}
	listings, err := c.Fetch(ctx, ids, sr.ID, realm)
	return listings, sr.Total, err
}

// Search POSTs a query body and returns the matching listing ids.
func (c *Client) Search(ctx context.Context, realm, league string, body []byte) (SearchResult, error) {
	u := fmt.Sprintf("%s/api/trade2/search/%s/%s", c.BaseURL, url.PathEscape(realm), url.PathEscape(league))
	data, err := c.do(ctx, http.MethodPost, u, body, c.search)
	if err != nil {
		return SearchResult{}, err
	}
	var sr SearchResult
	if err := json.Unmarshal(data, &sr); err != nil {
		return SearchResult{}, fmt.Errorf("decode trade search: %w", err)
	}
	return sr, nil
}

// Fetch retrieves listing details for ids, chunked to MaxFetch per request.
func (c *Client) Fetch(ctx context.Context, ids []string, queryID, realm string) ([]Listing, error) {
	var out []Listing
	for i := 0; i < len(ids); i += MaxFetch {
		end := i + MaxFetch
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]
		u := fmt.Sprintf("%s/api/trade2/fetch/%s?query=%s&realm=%s",
			c.BaseURL, strings.Join(chunk, ","), url.QueryEscape(queryID), url.QueryEscape(realm))
		data, err := c.do(ctx, http.MethodGet, u, nil, c.fetch)
		if err != nil {
			return out, err
		}
		var fr fetchResponse
		if err := json.Unmarshal(data, &fr); err != nil {
			return out, fmt.Errorf("decode trade fetch: %w", err)
		}
		for _, e := range fr.Result {
			mods := make([]string, 0, len(e.Item.ImplicitMods)+len(e.Item.ExplicitMods)+len(e.Item.RuneMods))
			mods = append(mods, e.Item.ImplicitMods...)
			mods = append(mods, e.Item.ExplicitMods...)
			mods = append(mods, e.Item.RuneMods...)
			out = append(out, Listing{
				Price:    Price{Amount: e.Listing.Price.Amount, Currency: e.Listing.Price.Currency},
				Seller:   e.Listing.Account.Name,
				Name:     e.Item.Name,
				BaseType: firstNonEmpty(e.Item.BaseType, e.Item.TypeLine),
				Mods:     mods,
			})
		}
	}
	return out, nil
}

// do issues a single request through the given limiter, retrying once on 429
// (the limiter records the Retry-After before the retry). 401/403 map to
// ErrSessionExpired.
func (c *Client) do(ctx context.Context, method, u string, body []byte, lim *poeapi.Limiter) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if err := lim.Wait(ctx); err != nil {
			return nil, err
		}
		var rdr io.Reader
		if body != nil {
			rdr = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, u, rdr)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if c.POESESSID != "" {
			req.Header.Set("Cookie", "POESESSID="+c.POESESSID)
		}

		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		lim.Observe(resp)
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()

		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			lastErr = fmt.Errorf("trade rate limited: %s", strings.TrimSpace(string(data)))
			continue
		case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
			return nil, ErrSessionExpired
		case resp.StatusCode < 200 || resp.StatusCode >= 300:
			return nil, fmt.Errorf("trade api status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
		}
		return data, nil
	}
	return nil, lastErr
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
