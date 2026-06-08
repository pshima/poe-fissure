// Command poefissure-server runs poe-fissure as an HTTP API + static frontend
// host, gated by a single password. It never performs interactive OAuth: it
// refreshes a GGG token non-interactively from a refresh token minted once
// locally via `poefissure auth login`.
//
// Subcommands:
//
//	serve         (default) run the HTTP server, configured from environment
//	hash <pw>     print a bcrypt hash for APP_PASSWORD_HASH
//	gen-secret    print a random value for SESSION_SECRET
package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/peteshima/poe-fissure/internal/auth"
	"github.com/peteshima/poe-fissure/internal/config"
	"github.com/peteshima/poe-fissure/internal/craft"
	"github.com/peteshima/poe-fissure/internal/poeapi"
	"github.com/peteshima/poe-fissure/internal/server"
	"github.com/peteshima/poe-fissure/internal/storage"
	"github.com/peteshima/poe-fissure/internal/trade"
	"github.com/peteshima/poe-fissure/internal/trade2"
)

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "serve":
		if err := serve(); err != nil {
			log.Fatalf("server: %v", err)
		}
	case "hash":
		if len(os.Args) < 3 {
			log.Fatal("usage: poefissure-server hash <password>")
		}
		h, err := server.HashPassword(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(h)
	case "gen-secret":
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			log.Fatal(err)
		}
		fmt.Println(hex.EncodeToString(b))
	default:
		log.Fatalf("unknown command %q (serve|hash|gen-secret)", cmd)
	}
}

func serve() error {
	clientID := env("GGG_CLIENT_ID", "pob")
	contact := env("POE_CONTACT", "")
	realm := env("POE_REALM", "poe2")
	character := os.Getenv("POE_CHARACTER")
	league := os.Getenv("POE_LEAGUE")
	if character == "" {
		return fmt.Errorf("POE_CHARACTER is required")
	}

	passwordHash := os.Getenv("APP_PASSWORD_HASH")
	if passwordHash == "" {
		return fmt.Errorf("APP_PASSWORD_HASH is required (generate with `poefissure-server hash <password>`)")
	}
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		return fmt.Errorf("SESSION_SECRET is required (generate with `poefissure-server gen-secret`)")
	}

	tokenFile := env("TOKEN_FILE", "./data/token.json")
	snapDir := env("SNAP_DIR", "./data/snapshots")
	webDir := os.Getenv("WEB_DIR") // empty disables static serving
	port := env("PORT", "8080")
	dev := os.Getenv("APP_DEV") == "1"

	ua := config.Config{ClientID: clientID, Version: "0.1.0", Contact: contact}.UserAgent()

	// Seed the token store from GGG_REFRESH_TOKEN on first boot so the Manager
	// can mint access tokens non-interactively. An existing token file wins.
	store := auth.Store{Path: tokenFile}
	if _, err := store.Load(); err != nil {
		rt := os.Getenv("GGG_REFRESH_TOKEN")
		if rt == "" {
			return fmt.Errorf("no token store at %s and GGG_REFRESH_TOKEN unset; mint one locally with `poefissure auth login`", tokenFile)
		}
		// Zero ExpiresAt => treated as expired => refreshed on first use.
		if err := store.Save(auth.Token{RefreshToken: rt}); err != nil {
			return fmt.Errorf("seed token store: %w", err)
		}
	}

	manager := auth.NewManager(auth.Config{ClientID: clientID, UserAgent: ua}, store)
	client := poeapi.New(ua, manager)

	// On-demand trade pricing is enabled only when a POESESSID is provided.
	var tradeClient *trade2.Client
	if sess := os.Getenv("POE_SESSID"); sess != "" {
		tradeClient = trade2.New(ua, sess)
	} else {
		log.Print("POE_SESSID unset: trade pricing endpoints disabled")
	}
	// Bundled defaults (life + resistances + common stats) overlaid with any
	// user-provided file, so trade filters work out of the box.
	statIDs, err := trade.LoadStatIDsMerged(env("STAT_IDS_FILE", "./data/trade_stat_ids.json"))
	if err != nil {
		return fmt.Errorf("load trade stat ids: %w", err)
	}
	def := config.Default()

	kb, err := craft.LoadKnowledge()
	if err != nil {
		return fmt.Errorf("load crafting knowledge base: %w", err)
	}

	maxAge := time.Duration(0)
	if v := os.Getenv("CHAR_MAX_AGE_MIN"); v != "" {
		if m, err := strconv.Atoi(v); err == nil {
			maxAge = time.Duration(m) * time.Minute
		}
	}

	srv := &server.Server{
		Realm:     realm,
		Character: character,
		League:    league,
		Client:    client,
		Snaps:     storage.Store{Dir: snapDir},
		Auth: &server.Auth{
			PasswordHash: []byte(passwordHash),
			Secret:       []byte(secret),
			TTL:          7 * 24 * time.Hour,
			Secure:       !dev,
		},
		WebDir: webDir,
		MaxAge: maxAge,

		Trade:   tradeClient,
		StatIDs: statIDs,
		Weights: def.Weights,
		// No price cap by default: show the cheapest matching upgrades regardless of
		// currency (a divine-denominated cap previously restricted results to
		// divine-priced listings). Set TRADE_BUDGET_DIV to re-enable a cap.
		BudgetMax:   budgetDivines(),
		BudgetCurr:  "divine",
		MaxListings: 10,
		Craft:       kb,
	}

	addr := ":" + port
	log.Printf("poefissure-server listening on %s (character=%s realm=%s league=%q web=%q)", addr, character, realm, league, webDir)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return httpSrv.ListenAndServe()
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// budgetDivines returns the optional price cap in divines (TRADE_BUDGET_DIV);
// 0 (the default) means no cap, so the search returns the cheapest matches.
func budgetDivines() float64 {
	if v := os.Getenv("TRADE_BUDGET_DIV"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
}
