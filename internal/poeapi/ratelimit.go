package poeapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// rule is one parsed rate-limit policy entry: at most Hits requests per Period,
// with a Restrict-second penalty if exceeded.
type rule struct {
	Hits     int
	Period   time.Duration
	Restrict time.Duration
}

// parseRules parses an "X-Rate-Limit-<rule>" header value of the form
// "hits:period:restrict" (comma-separated for multiple tiers).
func parseRules(v string) []rule {
	var out []rule
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		f := strings.Split(part, ":")
		if len(f) != 3 {
			continue
		}
		h, _ := strconv.Atoi(f[0])
		p, _ := strconv.Atoi(f[1])
		r, _ := strconv.Atoi(f[2])
		out = append(out, rule{Hits: h, Period: time.Duration(p) * time.Second, Restrict: time.Duration(r) * time.Second})
	}
	return out
}

// Limiter enforces GGG's dynamic rate limits. It paces requests to stay under
// the tightest advertised policy and honours active restrictions / Retry-After.
type Limiter struct {
	mu          sync.Mutex
	minInterval time.Duration // derived from the tightest rule
	last        time.Time
	blockedTill time.Time
}

// NewLimiter returns a limiter with a conservative default pace until the first
// response teaches it the real policy.
func NewLimiter() *Limiter {
	return &Limiter{minInterval: time.Second}
}

// Wait blocks until it is safe to issue the next request, respecting context.
func (l *Limiter) Wait(ctx context.Context) error {
	l.mu.Lock()
	now := time.Now()
	var until time.Time
	if l.blockedTill.After(now) {
		until = l.blockedTill
	}
	if next := l.last.Add(l.minInterval); next.After(until) {
		until = next
	}
	l.mu.Unlock()

	d := time.Until(until)
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Observe updates limiter state from a response's rate-limit headers. It widens
// the pacing interval to match the tightest policy and records any active
// restriction (or Retry-After on a 429).
func (l *Limiter) Observe(resp *http.Response) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.last = time.Now()

	rulesHdr := resp.Header.Get("X-Rate-Limit-Rules")
	for _, name := range strings.Split(rulesHdr, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		for _, ru := range parseRules(resp.Header.Get("X-Rate-Limit-" + name)) {
			if ru.Hits > 0 && ru.Period > 0 {
				// Keep a small safety margin (10%) under the advertised rate.
				iv := ru.Period / time.Duration(ru.Hits)
				iv += iv / 10
				if iv > l.minInterval {
					l.minInterval = iv
				}
			}
		}
		// State header: "current:period:restrictRemaining".
		st := parseRules(resp.Header.Get("X-Rate-Limit-" + name + "-State"))
		for _, s := range st {
			if s.Restrict > 0 {
				if till := l.last.Add(s.Restrict); till.After(l.blockedTill) {
					l.blockedTill = till
				}
			}
		}
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(strings.TrimSpace(ra)); err == nil {
				if till := l.last.Add(time.Duration(secs) * time.Second); till.After(l.blockedTill) {
					l.blockedTill = till
				}
			}
		}
	}
}
