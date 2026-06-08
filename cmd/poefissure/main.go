// Command poefissure queries a Path of Exile 2 character via the official API,
// stores snapshots locally, exports Path of Building 2 codes, and generates
// ToS-safe trade-site search URLs for gear upgrades.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/peteshima/poe-fissure/internal/analysis"
	"github.com/peteshima/poe-fissure/internal/auth"
	"github.com/peteshima/poe-fissure/internal/config"
	"github.com/peteshima/poe-fissure/internal/pob"
	"github.com/peteshima/poe-fissure/internal/poeapi"
	"github.com/peteshima/poe-fissure/internal/schema"
	"github.com/peteshima/poe-fissure/internal/storage"
	"github.com/peteshima/poe-fissure/internal/trade"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	app, err := newApp()
	if err != nil {
		fatal(err)
	}

	var cmdErr error
	switch os.Args[1] {
	case "auth":
		cmdErr = app.auth(ctx, os.Args[2:])
	case "char":
		cmdErr = app.char(ctx, os.Args[2:])
	case "pob":
		cmdErr = app.pobExport(ctx, os.Args[2:])
	case "export":
		cmdErr = app.exportItems(ctx, os.Args[2:])
	case "upgrades":
		cmdErr = app.upgrades(ctx, os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if cmdErr != nil {
		fatal(cmdErr)
	}
}

type app struct {
	cfg     config.Config
	store   auth.Store
	manager *auth.Manager
	snaps   storage.Store
}

func newApp() (*app, error) {
	cfgPath, err := config.DefaultPath()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	tokPath, err := auth.DefaultStorePath()
	if err != nil {
		return nil, err
	}
	snapDir, err := storage.DefaultDir()
	if err != nil {
		return nil, err
	}
	store := auth.Store{Path: tokPath}
	oauthCfg := auth.Config{
		ClientID:     cfg.ClientID,
		RedirectPort: cfg.RedirectPort,
		UserAgent:    cfg.UserAgent(),
	}
	return &app{
		cfg:     cfg,
		store:   store,
		manager: auth.NewManager(oauthCfg, store),
		snaps:   storage.Store{Dir: snapDir},
	}, nil
}

func (a *app) apiClient() *poeapi.Client {
	return poeapi.New(a.cfg.UserAgent(), a.manager)
}

// --- auth ---

func (a *app) auth(ctx context.Context, args []string) error {
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "login":
		oauthCfg := auth.Config{ClientID: a.cfg.ClientID, RedirectPort: a.cfg.RedirectPort, UserAgent: a.cfg.UserAgent()}
		tok, err := oauthCfg.Login(ctx)
		if err != nil {
			return err
		}
		if err := a.store.Save(tok); err != nil {
			return err
		}
		fmt.Printf("Logged in. Access token valid until %s.\n", tok.ExpiresAt.Local().Format(time.RFC1123))
		return nil
	case "status":
		tok, err := a.store.Load()
		if err != nil {
			fmt.Println("Not logged in. Run `poefissure auth login`.")
			return nil
		}
		fmt.Printf("Logged in. Scope: %s. Access token expires %s.\n", tok.Scope, tok.ExpiresAt.Local().Format(time.RFC1123))
		return nil
	case "logout":
		if err := a.store.Clear(); err != nil {
			return err
		}
		fmt.Println("Logged out.")
		return nil
	default:
		return fmt.Errorf("unknown auth subcommand %q (login|status|logout)", sub)
	}
}

// --- char ---

func (a *app) char(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: poefissure char <list|get|watch> [name]")
	}
	client := a.apiClient()
	switch args[0] {
	case "list":
		chars, err := client.ListCharacters(ctx, a.cfg.Realm)
		if err != nil {
			return err
		}
		for _, c := range chars {
			fmt.Printf("%-20s L%-3d %-12s %s\n", c.Name, c.Level, c.Class, c.League)
		}
		return nil
	case "get":
		name := a.cfg.Character
		if len(args) > 1 {
			name = args[1]
		}
		return a.fetchAndStore(ctx, client, name, true)
	case "watch":
		name := a.cfg.Character
		if len(args) > 1 {
			name = args[1]
		}
		interval := 60 * time.Second
		fmt.Printf("Watching %s every %s (Ctrl-C to stop)...\n", name, interval)
		for {
			if err := a.fetchAndStore(ctx, client, name, false); err != nil {
				fmt.Fprintf(os.Stderr, "fetch error: %v\n", err)
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(interval):
			}
		}
	default:
		return fmt.Errorf("unknown char subcommand %q (list|get|watch)", args[0])
	}
}

func (a *app) fetchAndStore(ctx context.Context, client *poeapi.Client, name string, verbose bool) error {
	c, err := client.GetCharacter(ctx, a.cfg.Realm, name)
	if err != nil {
		return err
	}
	if c == nil {
		return fmt.Errorf("character %q not found in realm %q", name, a.cfg.Realm)
	}
	path, err := a.snaps.Save(storage.Snapshot{CapturedAt: time.Now(), Realm: a.cfg.Realm, Character: c})
	if err != nil {
		return err
	}
	fmt.Printf("%s — L%d %s — %s (snapshot: %s)\n", c.Name, c.Level, c.Class, c.League, path)
	if verbose {
		printEquipment(c.EquippedBySlot())
	}
	return nil
}

func printEquipment(bySlot map[string]schema.Item) {
	if len(bySlot) == 0 {
		fmt.Println("  (no equipment returned)")
		return
	}
	for _, slot := range []string{"Weapon", "Offhand", "Helm", "BodyArmour", "Gloves", "Boots", "Ring", "Ring2", "Amulet", "Belt"} {
		it, ok := bySlot[slot]
		if !ok {
			continue
		}
		name := it.Name
		if name == "" {
			name = it.BaseType
		}
		fmt.Printf("  %-11s %s (%s)\n", slot, name, it.BaseType)
	}
}

// --- pob ---

func (a *app) pobExport(ctx context.Context, args []string) error {
	name := a.cfg.Character
	if len(args) > 0 {
		name = args[0]
	}
	c, err := a.latestOrFetch(ctx, name)
	if err != nil {
		return err
	}
	code, err := pob.BuildCode(c)
	if err != nil {
		return err
	}
	fmt.Println("Path of Building 2 import code (paste into PoB2 > Import > From Code):")
	fmt.Println(code)
	return nil
}

// --- export ---

// exportItems writes every item on the character (equipment, jewels, socketed
// skill gems) to one file each in the standard PoE clipboard item-text format.
func (a *app) exportItems(ctx context.Context, args []string) error {
	name := a.cfg.Character
	dir := "example"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dir":
			if i+1 < len(args) {
				dir = args[i+1]
				i++
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				name = args[i]
			}
		}
	}

	c, err := a.latestOrFetch(ctx, name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	groups := []struct {
		label string
		items []schema.Item
	}{
		{"equipment", c.Equipment},
		{"jewels", c.Jewels},
		{"skills", c.Skills},
	}

	n := 0
	for _, g := range groups {
		for _, it := range g.items {
			n++
			label := it.InventoryID
			if label == "" {
				label = g.label
			}
			fname := fmt.Sprintf("%02d-%s-%s.txt", n, strings.ToLower(label), itemSlug(it))
			path := filepath.Join(dir, fname)
			if err := os.WriteFile(path, []byte(pob.ItemText(it)+"\n"), 0o644); err != nil {
				return err
			}
			fmt.Printf("wrote %s\n", path)
		}
	}
	if n == 0 {
		return fmt.Errorf("no items found for %q (try `poefissure char get %s` first)", name, name)
	}
	fmt.Printf("Exported %d item(s) to %s/\n", n, dir)
	return nil
}

// itemSlug builds a filesystem-safe slug from an item's name or base type.
func itemSlug(it schema.Item) string {
	s := it.Name
	if s == "" {
		s = it.BaseType
	}
	if s == "" {
		s = it.TypeLine
	}
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "item"
	}
	return out
}

// --- upgrades ---

func (a *app) upgrades(ctx context.Context, args []string) error {
	goals := []analysis.Goal{analysis.GoalDPS, analysis.GoalSurvival, analysis.GoalResists, analysis.GoalAttributes}
	budget := a.cfg.Budget.Amount
	name := a.cfg.Character
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--goals":
			if i+1 < len(args) {
				goals = parseGoals(args[i+1])
				i++
			}
		case "--name":
			if i+1 < len(args) {
				name = args[i+1]
				i++
			}
		}
	}

	c, err := a.latestOrFetch(ctx, name)
	if err != nil {
		return err
	}
	statIDsPath, _ := trade.DefaultStatIDsPath()
	statIDs, err := trade.LoadStatIDs(statIDsPath)
	if err != nil {
		return err
	}

	scorer := analysis.NewScorer(a.cfg.Weights, goals)
	reports := scorer.Analyze(c)

	fmt.Printf("Upgrade targets for %s (budget: %.0f %s):\n\n", c.Name, budget, a.cfg.Budget.Currency)
	for _, r := range reports {
		opts := trade.BuildOptions{
			Category:       trade.SlotCategory(r.Slot),
			Wants:          r.WantStats,
			StatIDs:        statIDs,
			BudgetMax:      budget,
			BudgetCurrency: a.cfg.Budget.Currency,
			OnlineOnly:     true,
		}
		q, unmapped := trade.Build(opts)
		u, err := trade.URL(a.cfg.Realm, a.cfg.League, q)
		if err != nil {
			return err
		}
		fmt.Printf("[%-11s] %-30s score %6.1f\n", r.Slot, r.Equipped, r.Score)
		if len(r.Rationale) > 0 {
			fmt.Printf("    why: %s\n", strings.Join(r.Rationale, "; "))
		}
		if len(unmapped) > 0 {
			fmt.Printf("    note: %d target stat(s) omitted (no trade stat id mapped)\n", len(unmapped))
		}
		fmt.Printf("    %s\n\n", u)
	}
	if len(statIDs) == 0 {
		fmt.Printf("Tip: populate %s with PoE2 trade stat ids to add stat filters to these URLs.\n", statIDsPath)
	}
	return nil
}

// --- helpers ---

func (a *app) latestOrFetch(ctx context.Context, name string) (*schema.Character, error) {
	snap, err := a.snaps.Latest(name)
	if err == nil && snap != nil && snap.Character != nil {
		return snap.Character, nil
	}
	client := a.apiClient()
	c, err := client.GetCharacter(ctx, a.cfg.Realm, name)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, fmt.Errorf("character %q not found", name)
	}
	return c, nil
}

func parseGoals(s string) []analysis.Goal {
	var out []analysis.Goal
	for _, g := range strings.Split(s, ",") {
		switch strings.TrimSpace(strings.ToLower(g)) {
		case "dps", "damage":
			out = append(out, analysis.GoalDPS)
		case "survival", "survivability", "ehp", "life":
			out = append(out, analysis.GoalSurvival)
		case "resists", "resistance", "res":
			out = append(out, analysis.GoalResists)
		case "attributes", "attr":
			out = append(out, analysis.GoalAttributes)
		}
	}
	return out
}

func usage() {
	fmt.Print(`poefissure — query your PoE2 character and find upgrades

Usage:
  poefissure auth <login|status|logout>
  poefissure char <list|get [name]|watch [name]>
  poefissure pob [name]                 export a Path of Building 2 import code
  poefissure export [name] [--dir DIR]  write each item as standard PoE item text (default dir: example)
  poefissure upgrades [--goals dps,survival,resists,attributes] [--name N]

Config: ~/.config/poefissure/config.yaml (client_id, contact, character, league, ...)
`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
