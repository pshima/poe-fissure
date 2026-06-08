package trade

import (
	"testing"

	"github.com/peteshima/poe-fissure/internal/analysis"
)

func TestWeaponCategory(t *testing.T) {
	cases := map[string]string{
		"Obliterator Bow":    "weapon.bow",
		"Gemini Crossbow":    "weapon.crossbow", // "crossbow" must beat "bow"
		"Broadhead Quiver":   "armour.quiver",
		"Gelid Quarterstaff": "weapon.warstaff", // "quarterstaff" must beat "staff"
		"Reaping Staff":      "weapon.staff",
		"Attuned Wand":       "weapon.wand",
		"Omen Sceptre":       "weapon.sceptre",
		"Leather Boots":      "", // not a weapon/quiver
	}
	for base, want := range cases {
		if got := WeaponCategory(base); got != want {
			t.Errorf("WeaponCategory(%q) = %q, want %q", base, got, want)
		}
	}
}

func TestDefaultAndMergedStatIDs(t *testing.T) {
	def := DefaultStatIDs()
	if def["life"] == "" || def["fire_res"] == "" {
		t.Fatalf("default stat ids missing core entries: %v", def)
	}
	// Missing file => just defaults; no error.
	merged, err := LoadStatIDsMerged("/nonexistent/trade_stat_ids.json")
	if err != nil {
		t.Fatal(err)
	}
	if merged["life"] != def["life"] {
		t.Errorf("merged should contain defaults, got %v", merged["life"])
	}
}

func TestBuildUsesCountGroupForManyStats(t *testing.T) {
	opts := BuildOptions{
		Wants: []analysis.WantStat{
			{Stat: analysis.StatFireRes, Min: 30},
			{Stat: analysis.StatColdRes, Min: 30},
			{Stat: analysis.StatLightRes, Min: 30},
		},
		StatIDs: map[string]string{
			analysis.StatFireRes:  "explicit.stat_fire",
			analysis.StatColdRes:  "explicit.stat_cold",
			analysis.StatLightRes: "explicit.stat_light",
		},
	}
	q, _ := Build(opts)
	if len(q.Query.Stats) != 1 {
		t.Fatalf("want 1 group, got %d", len(q.Query.Stats))
	}
	g := q.Query.Stats[0]
	if g.Type != "count" || g.Value == nil || g.Value.Min == nil || *g.Value.Min != 2 {
		t.Fatalf("3 resist stats should form a count>=2 group, got type=%q value=%+v", g.Type, g.Value)
	}
}
