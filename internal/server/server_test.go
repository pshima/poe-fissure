package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/peteshima/poe-fissure/internal/config"
	"github.com/peteshima/poe-fissure/internal/schema"
	"github.com/peteshima/poe-fissure/internal/storage"
	"github.com/peteshima/poe-fissure/internal/trade2"
)

type fakeFetcher struct{ c *schema.Character }

func (f fakeFetcher) GetCharacter(ctx context.Context, realm, name string) (*schema.Character, error) {
	return f.c, nil
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatal(err)
	}
	char := &schema.Character{
		Name: "Frozenmulligan", Class: "Ranger", Level: 90, League: "Runes of Aldur",
		Equipment: []schema.Item{{
			InventoryID: "Weapon", Name: "Viper Thunder", BaseType: "Obliterator Bow",
			Rarity: "Rare", ItemLevel: 79, ExplicitMods: []string{"+28 to Dexterity"},
		}},
	}
	return &Server{
		Realm: "poe2", Character: "Frozenmulligan", League: "Runes of Aldur",
		Client: fakeFetcher{c: char},
		Snaps:  storage.Store{Dir: t.TempDir()},
		Auth:   &Auth{PasswordHash: []byte(hash), Secret: []byte("test-secret"), TTL: time.Hour, Secure: false},
	}
}

func TestAuthSignVerify(t *testing.T) {
	a := &Auth{Secret: []byte("s"), TTL: time.Hour}
	tok := a.sign(time.Now().Add(time.Hour).Unix())
	if !a.verify(tok) {
		t.Fatal("valid token failed verification")
	}
	if a.verify(tok + "x") {
		t.Fatal("tampered token verified")
	}
	if a.verify(a.sign(time.Now().Add(-time.Hour).Unix())) {
		t.Fatal("expired token verified")
	}
}

func TestLoginAndItems(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()

	// Unauthenticated request is rejected.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/character/items", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 unauthenticated, got %d", rec.Code)
	}

	// Wrong password rejected.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"password":"nope"}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 wrong password, got %d", rec.Code)
	}

	// Correct password issues a session cookie.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"password":"hunter2"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 login, got %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie set")
	}

	// Authenticated items request returns the equipped bow as standard text.
	req := httptest.NewRequest(http.MethodGet, "/api/character/items", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 items, got %d: %s", rec.Code, rec.Body.String())
	}
	var items []itemText
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Slot != "Weapon" || !strings.Contains(items[0].Text, "Obliterator Bow") {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestUpgradesEndpoint(t *testing.T) {
	// Mock trade2 API: any search returns ids; any fetch returns one priced listing.
	tradeAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/search/") {
			w.Write([]byte(`{"id":"q1","total":7,"result":["a"]}`))
			return
		}
		w.Write([]byte(`{"result":[{"id":"a","listing":{"price":{"amount":2,"currency":"divine"},"account":{"name":"s"}},"item":{"name":"Boots","baseType":"Leather Boots"}}]}`))
	}))
	defer tradeAPI.Close()

	srv := newTestServer(t)
	tc := trade2.New("ua", "sess")
	tc.BaseURL = tradeAPI.URL
	srv.Trade = tc
	srv.Weights = config.Default().Weights
	h := srv.Handler()

	// Log in.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"password":"hunter2"}`)))
	cookies := rec.Result().Cookies()

	// Price the top upgrade slot.
	req := httptest.NewRequest(http.MethodGet, "/api/upgrades?all=false", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var ups []UpgradeResult
	if err := json.Unmarshal(rec.Body.Bytes(), &ups); err != nil {
		t.Fatal(err)
	}
	if len(ups) != 1 {
		t.Fatalf("want 1 priced slot, got %d", len(ups))
	}
	if ups[0].Cheapest == nil || ups[0].Cheapest.Amount != 2 || ups[0].Total != 7 {
		t.Fatalf("unexpected upgrade result: %+v", ups[0])
	}
}

func TestPricingDisabledWithoutTrade(t *testing.T) {
	srv := newTestServer(t) // no Trade set
	h := srv.Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"password":"hunter2"}`)))
	cookies := rec.Result().Cookies()

	req := httptest.NewRequest(http.MethodGet, "/api/upgrades", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 when trade disabled, got %d", rec.Code)
	}
}
