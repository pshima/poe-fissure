package analysis

import (
	"testing"

	"github.com/peteshima/poe-fissure/internal/config"
	"github.com/peteshima/poe-fissure/internal/schema"
)

func TestParseMods(t *testing.T) {
	mods := []string{
		"+72 to maximum Life",
		"+34% to Fire Resistance",
		"+25 to Intelligence",
		"Adds 30 to 55 Cold Damage",
		"12% increased Cast Speed",
		"+15% to all Elemental Resistances",
	}
	st := ParseMods(mods)
	if st[StatLife] != 72 {
		t.Errorf("life = %v, want 72", st[StatLife])
	}
	if st[StatFireRes] != 34+15 {
		t.Errorf("fire res = %v, want 49", st[StatFireRes])
	}
	if st[StatColdRes] != 15 {
		t.Errorf("cold res = %v, want 15", st[StatColdRes])
	}
	if st[StatInt] != 25 {
		t.Errorf("int = %v, want 25", st[StatInt])
	}
	if st[StatEleAdded] != 42.5 {
		t.Errorf("ele added = %v, want 42.5", st[StatEleAdded])
	}
	if st[StatCastSpd] != 12 {
		t.Errorf("cast speed = %v", st[StatCastSpd])
	}
}

func TestAnalyzeRanksEmptySlotsHigh(t *testing.T) {
	c := &schema.Character{
		Name: "Frozenmulligan",
		Equipment: []schema.Item{
			{Name: "Doom Visor", BaseType: "Lacquered Helmet", InventoryID: "Helm",
				ExplicitMods: []string{"+72 to maximum Life", "+34% to Fire Resistance"}},
			// Boots, Gloves, rings, etc. are empty -> should rank high.
		},
	}
	s := NewScorer(config.Default().Weights, nil)
	reports := s.Analyze(c)
	if len(reports) == 0 {
		t.Fatal("no reports")
	}
	// The top report should be an empty slot (higher headroom than the filled Helm).
	if reports[0].Equipped != "(empty)" {
		t.Errorf("top slot should be empty, got %q (%s)", reports[0].Equipped, reports[0].Slot)
	}
	// Every report should carry at least one rationale or want.
	for _, r := range reports {
		if r.Score <= 0 {
			t.Errorf("slot %s has non-positive score %v", r.Slot, r.Score)
		}
	}
}

func TestResistGapDetection(t *testing.T) {
	c := &schema.Character{
		Equipment: []schema.Item{
			{InventoryID: "Helm", ExplicitMods: []string{"+10% to Fire Resistance"}},
		},
	}
	s := NewScorer(config.Default().Weights, []Goal{GoalResists})
	reports := s.Analyze(c)
	// Some slot should want fire resistance since total fire res (10) < cap.
	foundFireWant := false
	for _, r := range reports {
		for _, w := range r.WantStats {
			if w.Stat == StatFireRes {
				foundFireWant = true
			}
		}
	}
	if !foundFireWant {
		t.Error("expected a fire-resistance want given the gap")
	}
}
