package poeapi

import (
	"net/http"
	"testing"
	"time"
)

func TestParseRules(t *testing.T) {
	rs := parseRules("60:60:120,10:5:30")
	if len(rs) != 2 {
		t.Fatalf("got %d rules", len(rs))
	}
	if rs[0].Hits != 60 || rs[0].Period != 60*time.Second || rs[0].Restrict != 120*time.Second {
		t.Errorf("rule0 = %+v", rs[0])
	}
	if rs[1].Hits != 10 {
		t.Errorf("rule1 = %+v", rs[1])
	}
}

func TestObserveWidensInterval(t *testing.T) {
	l := NewLimiter()
	// 30 requests per 60s => ~2s per request.
	l.Observe(&http.Response{Header: headers(map[string]string{
		"X-Rate-Limit-Rules":   "account",
		"X-Rate-Limit-account": "30:60:60",
	})})
	if l.minInterval < 2*time.Second {
		t.Errorf("minInterval = %v, want >= 2s", l.minInterval)
	}
}

func TestObserveRetryAfterBlocks(t *testing.T) {
	l := NewLimiter()
	resp := &http.Response{StatusCode: http.StatusTooManyRequests, Header: headers(map[string]string{
		"Retry-After": "5",
	})}
	l.Observe(resp)
	if time.Until(l.blockedTill) < 4*time.Second {
		t.Errorf("blockedTill not set from Retry-After")
	}
}

func headers(m map[string]string) http.Header {
	h := http.Header{}
	for k, v := range m {
		h.Set(k, v)
	}
	return h
}
