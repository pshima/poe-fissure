package trade

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/peteshima/poe-fissure/internal/analysis"
)

func TestBuildQueryWithStatIDs(t *testing.T) {
	opts := BuildOptions{
		Category:       "armour.gloves",
		Wants:          []analysis.WantStat{{Stat: analysis.StatLife, Min: 70}, {Stat: analysis.StatFireRes, Min: 30}},
		StatIDs:        map[string]string{analysis.StatLife: "explicit.stat_life", analysis.StatFireRes: "explicit.stat_fire"},
		BudgetMax:      50,
		BudgetCurrency: "divine",
		OnlineOnly:     true,
	}
	q, unmapped := Build(opts)
	if len(unmapped) != 0 {
		t.Errorf("unexpected unmapped: %v", unmapped)
	}
	if q.Query.Status.Option != "online" {
		t.Errorf("status = %q", q.Query.Status.Option)
	}
	if len(q.Query.Stats) != 1 || len(q.Query.Stats[0].Filters) != 2 {
		t.Fatalf("expected 2 stat filters, got %+v", q.Query.Stats)
	}
	if q.Query.Filters.TradeFilters == nil {
		t.Fatal("missing trade filters")
	}
}

func TestBuildOmitsUnmappedStats(t *testing.T) {
	opts := BuildOptions{
		Wants:   []analysis.WantStat{{Stat: analysis.StatLife, Min: 70}, {Stat: analysis.StatChaosRes, Min: 10}},
		StatIDs: map[string]string{analysis.StatLife: "explicit.stat_life"},
	}
	q, unmapped := Build(opts)
	if len(unmapped) != 1 || unmapped[0].Stat != analysis.StatChaosRes {
		t.Errorf("expected chaos res unmapped, got %v", unmapped)
	}
	if len(q.Query.Stats[0].Filters) != 1 {
		t.Errorf("expected 1 mapped filter, got %d", len(q.Query.Stats[0].Filters))
	}
}

func TestURLEncodesQuery(t *testing.T) {
	q, _ := Build(BuildOptions{Category: "accessory.ring", OnlineOnly: true})
	u, err := URL("poe2", "Runes of Aldur", q)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u, "https://www.pathofexile.com/trade2/search/poe2/Runes%20of%20Aldur?q=") {
		t.Errorf("unexpected URL: %s", u)
	}
	// The q param must decode back to valid JSON matching our query.
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	raw := parsed.Query().Get("q")
	var rt Query
	if err := json.Unmarshal([]byte(raw), &rt); err != nil {
		t.Fatalf("q param is not valid JSON: %v", err)
	}
	if rt.Query.Filters.TypeFilters == nil {
		t.Error("category lost in round-trip")
	}
}
