package trade2

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPriceCheckSearchAndFetch(t *testing.T) {
	var searchHits, fetchHits int
	var fetchedIDs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Advertise a rate-limit policy so the limiter has something to parse.
		w.Header().Set("X-Rate-Limit-Rules", "trade")
		w.Header().Set("X-Rate-Limit-trade", "5:10:60")
		w.Header().Set("X-Rate-Limit-trade-State", "1:10:0")
		switch {
		case strings.Contains(r.URL.Path, "/api/trade2/search/"):
			searchHits++
			if got := r.Header.Get("Cookie"); got != "POESESSID=sess123" {
				t.Errorf("missing/incorrect POESESSID cookie: %q", got)
			}
			w.Write([]byte(`{"id":"q1","total":42,"result":["a","b","c"]}`))
		case strings.Contains(r.URL.Path, "/api/trade2/fetch/"):
			fetchHits++
			fetchedIDs = append(fetchedIDs, strings.TrimPrefix(r.URL.Path, "/api/trade2/fetch/"))
			w.Write([]byte(`{"result":[
				{"id":"a","listing":{"price":{"amount":3,"currency":"divine"},"account":{"name":"seller1"}},
				 "item":{"name":"Viper Thunder","baseType":"Obliterator Bow","explicitMods":["+28 to Dexterity"]}}
			]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New("test-agent", "sess123")
	c.BaseURL = srv.URL

	listings, total, err := c.PriceCheck(context.Background(), "poe2", "Runes of Aldur", []byte(`{}`), 10)
	if err != nil {
		t.Fatal(err)
	}
	if searchHits != 1 {
		t.Errorf("want 1 search, got %d", searchHits)
	}
	if fetchHits != 1 { // 3 ids fit in one MaxFetch(10) chunk
		t.Errorf("want 1 fetch, got %d", fetchHits)
	}
	if total != 42 {
		t.Errorf("want total 42, got %d", total)
	}
	if len(listings) != 1 || listings[0].Price.Amount != 3 || listings[0].Price.Currency != "divine" {
		t.Fatalf("unexpected listings: %+v", listings)
	}
	if listings[0].BaseType != "Obliterator Bow" || listings[0].Seller != "seller1" {
		t.Errorf("unexpected listing fields: %+v", listings[0])
	}
}

func TestFetchChunksByTen(t *testing.T) {
	var chunks []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/fetch/") {
			ids := strings.TrimPrefix(r.URL.Path, "/api/trade2/fetch/")
			chunks = append(chunks, ids)
			w.Write([]byte(`{"result":[]}`))
		}
	}))
	defer srv.Close()

	c := New("ua", "")
	c.BaseURL = srv.URL
	ids := make([]string, 23)
	for i := range ids {
		ids[i] = string(rune('a' + i))
	}
	if _, err := c.Fetch(context.Background(), ids, "q1", "poe2"); err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 3 { // 23 ids => 10 + 10 + 3
		t.Fatalf("want 3 chunks, got %d: %v", len(chunks), chunks)
	}
	if n := len(strings.Split(chunks[0], ",")); n != 10 {
		t.Errorf("first chunk should have 10 ids, got %d", n)
	}
}

func TestSessionExpired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := New("ua", "bad")
	c.BaseURL = srv.URL
	_, _, err := c.PriceCheck(context.Background(), "poe2", "L", []byte(`{}`), 10)
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("want ErrSessionExpired, got %v", err)
	}
}
