// Package server exposes poe-fissure's functionality as a JSON HTTP API behind a
// single-password session gate, plus static hosting for the React frontend. It
// reuses the existing internal packages (auth/poeapi/storage/pob/analysis/trade)
// — handlers are the only place network-bound work is triggered, and never on a
// timer (trade calls in particular are strictly on-demand).
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peteshima/poe-fissure/internal/analysis"
	"github.com/peteshima/poe-fissure/internal/config"
	"github.com/peteshima/poe-fissure/internal/craft"
	"github.com/peteshima/poe-fissure/internal/pob"
	"github.com/peteshima/poe-fissure/internal/schema"
	"github.com/peteshima/poe-fissure/internal/storage"
	"github.com/peteshima/poe-fissure/internal/trade"
	"github.com/peteshima/poe-fissure/internal/trade2"
)

// CharacterFetcher is the slice of poeapi.Client the server depends on.
type CharacterFetcher interface {
	GetCharacter(ctx context.Context, realm, name string) (*schema.Character, error)
}

// Server wires the HTTP API to the character fetcher, snapshot store, and auth.
type Server struct {
	Realm     string
	Character string
	League    string
	Client    CharacterFetcher
	Snaps     storage.Store
	Auth      *Auth
	// WebDir is the built React app (web/dist); empty disables static serving.
	WebDir string
	// MaxAge is how old a snapshot may be before GET /api/character refetches.
	MaxAge time.Duration

	// Trade is the on-demand trade2 price client; nil disables price endpoints.
	Trade *trade2.Client
	// StatIDs maps internal stat keys to trade2 stat ids (empty => no stat filters).
	StatIDs trade.StatIDs
	// Weights / Budget tune the upgrade scorer and price cap.
	Weights     config.Weights
	BudgetMax   float64
	BudgetCurr  string
	MaxListings int // cheapest listings to fetch per check (default 10)

	// Craft is the crafting knowledge base for the advisor endpoints.
	Craft *craft.Knowledge
}

// Handler builds the full HTTP handler (API + static frontend).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/login", s.Auth.handleLogin)
	mux.HandleFunc("POST /api/logout", s.Auth.handleLogout)
	mux.HandleFunc("GET /api/session", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": s.Auth.authed(r)})
	})

	mux.HandleFunc("GET /api/character", s.Auth.require(s.handleCharacter))
	mux.HandleFunc("POST /api/character/refresh", s.Auth.require(s.handleRefresh))
	mux.HandleFunc("GET /api/character/items", s.Auth.require(s.handleItems))

	mux.HandleFunc("GET /api/upgrades", s.Auth.require(s.handleUpgrades))
	mux.HandleFunc("GET /api/price", s.Auth.require(s.handlePrice))

	mux.HandleFunc("POST /api/craft/parse", s.Auth.require(s.handleCraftParse))
	mux.HandleFunc("POST /api/craft/advise", s.Auth.require(s.handleCraftAdvise))

	if s.WebDir != "" {
		mux.Handle("/", s.staticHandler())
	}
	return mux
}

// --- character handlers ---

func (s *Server) handleCharacter(w http.ResponseWriter, r *http.Request) {
	c, err := s.latestOrFetch(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	c, err := s.fetchAndStore(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// itemText is one equipped item rendered as standard PoE clipboard text.
type itemText struct {
	Slot     string `json:"slot"`
	Name     string `json:"name"`
	BaseType string `json:"baseType"`
	Text     string `json:"text"`
}

func (s *Server) handleItems(w http.ResponseWriter, r *http.Request) {
	c, err := s.latestOrFetch(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	bySlot := c.EquippedBySlot()
	order := []string{"Weapon", "Offhand", "Helm", "BodyArmour", "Gloves", "Boots", "Ring", "Ring2", "Amulet", "Belt"}
	seen := map[string]bool{}
	out := make([]itemText, 0, len(c.Equipment))
	add := func(slot string, it schema.Item) {
		name := it.Name
		if name == "" {
			name = it.BaseType
		}
		out = append(out, itemText{Slot: slot, Name: name, BaseType: it.BaseType, Text: pob.ItemText(it)})
	}
	for _, slot := range order {
		if it, ok := bySlot[slot]; ok {
			add(slot, it)
			seen[slot] = true
		}
	}
	// Any slots not in the canonical order (e.g. flasks, extra) appended after.
	for _, it := range c.Equipment {
		if it.InventoryID == "" || seen[it.InventoryID] {
			continue
		}
		add(it.InventoryID, it)
		seen[it.InventoryID] = true
	}
	writeJSON(w, http.StatusOK, out)
}

// --- character fetching ---

func (s *Server) latestOrFetch(ctx context.Context) (*schema.Character, error) {
	snap, err := s.Snaps.Latest(s.Character)
	if err == nil && snap != nil && snap.Character != nil {
		if s.MaxAge <= 0 || time.Since(snap.CapturedAt) < s.MaxAge {
			return snap.Character, nil
		}
	}
	return s.fetchAndStore(ctx)
}

func (s *Server) fetchAndStore(ctx context.Context) (*schema.Character, error) {
	c, err := s.Client.GetCharacter(ctx, s.Realm, s.Character)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, &fetchError{msg: "character not found: " + s.Character}
	}
	if _, err := s.Snaps.Save(storage.Snapshot{CapturedAt: time.Now(), Realm: s.Realm, Character: c}); err != nil {
		return nil, err
	}
	return c, nil
}

type fetchError struct{ msg string }

func (e *fetchError) Error() string { return e.msg }

// --- trade pricing (on-demand only) ---

// UpgradeResult is one slot's upgrade opportunity with live cheapest pricing.
type UpgradeResult struct {
	Slot          string           `json:"slot"`
	Equipped      string           `json:"equipped"`
	Score         float64          `json:"score"`
	Rationale     []string         `json:"rationale"`
	TradeURL      string           `json:"tradeUrl"`
	Cheapest      *trade2.Price    `json:"cheapest"`
	Listings      []trade2.Listing `json:"listings"`
	Total         int              `json:"total"`
	UnmappedStats int              `json:"unmappedStats"`
}

// handleUpgrades ranks slots by upgrade headroom and prices the top slot (or
// every slot, sequentially, when ?all=true). Each priced slot costs ~2 trade
// requests; this only ever runs when the user opens/triggers the page.
func (s *Server) handleUpgrades(w http.ResponseWriter, r *http.Request) {
	if s.Trade == nil {
		writeError(w, http.StatusServiceUnavailable, "trade pricing not configured (set POE_SESSID)")
		return
	}
	c, err := s.latestOrFetch(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	reports := analysis.NewScorer(s.Weights, nil).Analyze(c)
	bySlot := c.EquippedBySlot()
	limit := 1
	if r.URL.Query().Get("all") == "true" {
		limit = len(reports)
	}
	out := make([]UpgradeResult, 0, limit)
	for i := 0; i < len(reports) && i < limit; i++ {
		res, err := s.priceForReport(r.Context(), reports[i], bySlot[reports[i].Slot])
		if err != nil {
			if errors.Is(err, trade2.ErrSessionExpired) {
				writeSessionExpired(w)
				return
			}
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		out = append(out, res)
	}
	writeJSON(w, http.StatusOK, out)
}

// handlePrice prices a single named slot (?slot=Gloves).
func (s *Server) handlePrice(w http.ResponseWriter, r *http.Request) {
	if s.Trade == nil {
		writeError(w, http.StatusServiceUnavailable, "trade pricing not configured (set POE_SESSID)")
		return
	}
	slot := r.URL.Query().Get("slot")
	if slot == "" {
		writeError(w, http.StatusBadRequest, "missing ?slot")
		return
	}
	c, err := s.latestOrFetch(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	reports := analysis.NewScorer(s.Weights, nil).Analyze(c)
	var rep *analysis.SlotReport
	for i := range reports {
		if reports[i].Slot == slot {
			rep = &reports[i]
			break
		}
	}
	if rep == nil {
		writeError(w, http.StatusNotFound, "unknown slot: "+slot)
		return
	}
	res, err := s.priceForReport(r.Context(), *rep, c.EquippedBySlot()[slot])
	if err != nil {
		if errors.Is(err, trade2.ErrSessionExpired) {
			writeSessionExpired(w)
			return
		}
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// priceForReport builds the "at least as good as equipped" query for a slot and
// fetches the cheapest matching live listings. equipped is the currently-worn
// item in that slot (used to pick the right weapon/quiver category).
func (s *Server) priceForReport(ctx context.Context, rep analysis.SlotReport, equipped schema.Item) (UpgradeResult, error) {
	category := trade.SlotCategory(rep.Slot)
	if wc := trade.WeaponCategory(equipped.BaseType); wc != "" {
		category = wc // weapon/offhand: match the equipped item's actual class
	}
	q, unmapped := trade.Build(trade.BuildOptions{
		Category:       category,
		Wants:          rep.WantStats,
		StatIDs:        s.StatIDs,
		BudgetMax:      s.BudgetMax,
		BudgetCurrency: s.BudgetCurr,
		OnlineOnly:     true,
	})
	tradeURL, _ := trade.URL(s.Realm, s.League, q)
	res := UpgradeResult{
		Slot: rep.Slot, Equipped: rep.Equipped, Score: rep.Score,
		Rationale: rep.Rationale, TradeURL: tradeURL, UnmappedStats: len(unmapped),
	}
	body, err := q.JSON()
	if err != nil {
		return res, err
	}
	listings, total, err := s.Trade.PriceCheck(ctx, s.Realm, s.League, body, s.maxListings())
	if err != nil {
		return res, err
	}
	res.Listings = listings
	res.Total = total
	if len(listings) > 0 {
		p := listings[0].Price
		res.Cheapest = &p
	}
	return res, nil
}

func (s *Server) maxListings() int {
	if s.MaxListings <= 0 {
		return 10
	}
	return s.MaxListings
}

func writeSessionExpired(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, map[string]string{
		"error": trade2.ErrSessionExpired.Error(),
		"code":  "poesessid_invalid",
	})
}

// --- crafting advisor ---

func (s *Server) handleCraftParse(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<18)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	it, err := craft.Parse(body.Text)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"item":        it,
		"suggestions": s.Craft.SuggestionsFor(it.ItemClass),
		"rating":      craft.Rate(it, s.archetype(), s.Craft, craft.NullResolver()),
	})
}

// archetype derives the player's build archetype from the latest cached snapshot
// (no network call); falls back to a generic attack build if none is stored yet.
func (s *Server) archetype() craft.Archetype {
	if snap, err := s.Snaps.Latest(s.Character); err == nil && snap != nil && snap.Character != nil {
		return craft.DeriveArchetype(snap.Character)
	}
	return craft.Archetype{Family: craft.FamilyAttack, Weapon: "unknown"}
}

func (s *Server) handleCraftAdvise(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text       string          `json:"text"`
		TargetText string          `json:"targetText"`
		Kind       craft.AffixKind `json:"kind"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<18)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	it, err := craft.Parse(body.Text)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	plan := craft.Advise(it, craft.Goal{TargetText: body.TargetText, Kind: body.Kind}, s.Craft)
	writeJSON(w, http.StatusOK, map[string]any{"item": it, "plan": plan})
}

// --- static frontend (SPA) ---

func (s *Server) staticHandler() http.Handler {
	fs := http.FileServer(http.Dir(s.WebDir))
	index := filepath.Join(s.WebDir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve the file if it exists; otherwise fall back to index.html so the
		// client-side router can handle the route.
		clean := filepath.Clean(r.URL.Path)
		if clean == "/" {
			http.ServeFile(w, r, index)
			return
		}
		p := filepath.Join(s.WebDir, strings.TrimPrefix(clean, "/"))
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, index)
	})
}

// --- json helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
