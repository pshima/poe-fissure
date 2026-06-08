package analysis

import (
	"sort"

	"github.com/peteshima/poe-fissure/internal/config"
	"github.com/peteshima/poe-fissure/internal/schema"
)

// Goal identifies a scoring objective the user opted into.
type Goal string

const (
	GoalDPS        Goal = "dps"
	GoalSurvival   Goal = "survival"
	GoalResists    Goal = "resists"
	GoalAttributes Goal = "attributes"
)

// ResCap is the standard maximum elemental resistance.
const ResCap = 75.0

// SlotReport ranks one equipment slot's upgrade opportunity and the mod profile
// to search the trade site for.
type SlotReport struct {
	Slot      string
	Equipped  string  // equipped item name/base, for display
	Score     float64 // higher = more upgrade headroom
	WantStats []WantStat
	Rationale []string
}

// WantStat is a target stat the trade query should look for on this slot.
type WantStat struct {
	Stat string
	Min  float64
}

// Scorer holds tuning derived from config.
type Scorer struct {
	Weights config.Weights
	Goals   map[Goal]bool
}

// NewScorer builds a scorer for the requested goals.
func NewScorer(w config.Weights, goals []Goal) *Scorer {
	set := make(map[Goal]bool, len(goals))
	for _, g := range goals {
		set[g] = true
	}
	if len(set) == 0 { // default: everything
		set = map[Goal]bool{GoalDPS: true, GoalSurvival: true, GoalResists: true, GoalAttributes: true}
	}
	return &Scorer{Weights: w, Goals: set}
}

// slots we consider for gear upgrades, in display order.
var consideredSlots = []string{
	"Weapon", "Offhand", "Helm", "BodyArmour", "Gloves", "Boots",
	"Ring", "Ring2", "Amulet", "Belt",
}

// Analyze produces a ranked list of slot upgrade reports for a character. The
// account-wide resistance/attribute gaps drive what each slot should search for.
func (s *Scorer) Analyze(c *schema.Character) []SlotReport {
	equipped := c.EquippedBySlot()
	total := CharacterStats(c) // item-only aggregate, used to find global gaps

	// Global gaps (only meaningful from items, surfaced as search targets).
	resGaps := map[string]float64{}
	if s.Goals[GoalResists] {
		for _, r := range []string{StatFireRes, StatColdRes, StatLightRes} {
			if g := ResCap - total[r]; g > 0 {
				resGaps[r] = g
			}
		}
	}

	var reports []SlotReport
	for _, slot := range consideredSlots {
		it, ok := equipped[slot]
		rep := SlotReport{Slot: slot}
		if ok {
			rep.Equipped = displayName(it)
		} else {
			rep.Equipped = "(empty)"
		}
		st := Stats{}
		if ok {
			st = ItemStats(it)
		}
		rep.Score, rep.WantStats, rep.Rationale = s.scoreSlot(slot, st, resGaps, ok)
		reports = append(reports, rep)
	}

	sort.SliceStable(reports, func(i, j int) bool { return reports[i].Score > reports[j].Score })
	return reports
}

// scoreSlot estimates upgrade headroom for one slot. The heuristic rewards slots
// that currently contribute little toward the active goals (more to gain) and
// emits the stat targets to search for.
func (s *Scorer) scoreSlot(slot string, st Stats, resGaps map[string]float64, equipped bool) (float64, []WantStat, []string) {
	var score float64
	var want []WantStat
	var why []string

	if !equipped {
		// An empty slot is the biggest possible upgrade.
		score += 50
		why = append(why, "slot is empty")
	}

	if s.Goals[GoalSurvival] {
		def := st[StatLife] + st[StatES] + st[StatArmour]/10 + st[StatEvasion]/10
		// Lower current defensive contribution => more headroom.
		score += s.Weights.Survivability * (1.0 + 100.0/(def+50))
		if slotCanRollLife(slot) {
			want = append(want, WantStat{Stat: StatLife, Min: roundDown(st[StatLife]+10, 10)})
			why = append(why, "add maximum life")
		}
	}

	if s.Goals[GoalResists] && slotCanRollRes(slot) {
		for stat, gap := range resGaps {
			if gap <= 0 {
				continue
			}
			// Reward slots that can help close a global resistance gap.
			score += s.Weights.Resists * gap / 2
			want = append(want, WantStat{Stat: stat, Min: minFloat(gap, 30)})
			why = append(why, "help cap "+resLabel(stat))
		}
	}

	if s.Goals[GoalAttributes] && slotCanRollRes(slot) {
		score += s.Weights.Attributes * (1.0 + 30.0/(st[StatStr]+st[StatDex]+st[StatInt]+20))
	}

	if s.Goals[GoalDPS] && slotIsOffensive(slot) {
		off := st[StatPhysInc] + st[StatPhysAdded] + st[StatEleAdded] + st[StatAttackSpd] + st[StatSpellInc] + st[StatCritChance]
		score += s.Weights.DPS * (5.0 + 200.0/(off+20))
		why = append(why, "improve damage rolls")
	}

	return score, want, why
}

func displayName(it schema.Item) string {
	if it.Name != "" {
		if it.BaseType != "" {
			return it.Name + " (" + it.BaseType + ")"
		}
		return it.Name
	}
	if it.BaseType != "" {
		return it.BaseType
	}
	return it.TypeLine
}

func slotCanRollLife(slot string) bool {
	switch slot {
	case "Helm", "BodyArmour", "Gloves", "Boots", "Ring", "Ring2", "Amulet", "Belt":
		return true
	}
	return false
}

func slotCanRollRes(slot string) bool {
	switch slot {
	case "Helm", "BodyArmour", "Gloves", "Boots", "Ring", "Ring2", "Amulet", "Belt", "Offhand":
		return true
	}
	return false
}

func slotIsOffensive(slot string) bool {
	switch slot {
	case "Weapon", "Offhand", "Ring", "Ring2", "Amulet", "Gloves":
		return true
	}
	return false
}

func resLabel(stat string) string {
	switch stat {
	case StatFireRes:
		return "fire resistance"
	case StatColdRes:
		return "cold resistance"
	case StatLightRes:
		return "lightning resistance"
	case StatChaosRes:
		return "chaos resistance"
	}
	return stat
}

func roundDown(v, step float64) float64 {
	if step <= 0 {
		return v
	}
	return float64(int(v/step)) * step
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
