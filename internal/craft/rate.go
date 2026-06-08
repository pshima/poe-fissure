package craft

import (
	"fmt"
	"strings"
)

// Rating answers "should I craft on this item, for my build?" with a letter grade
// and the three sub-scores the user cares about: base, current mods, and how hard
// it is to finish into a great item.
type Rating struct {
	Grade          string    `json:"grade"`   // A+ .. F, or "—" when not a craft target
	Score          int       `json:"score"`   // 0-100 blended
	Verdict        string    `json:"verdict"` // one-line recommendation
	Base           int       `json:"base"`
	Mods           int       `json:"mods"`
	Difficulty     int       `json:"difficulty"`     // headroom: higher = easier to finish
	DifficultyTier string    `json:"difficultyTier"` // budget | mid | high
	Archetype      Archetype `json:"archetype"`
	Reasons        []string  `json:"reasons"`
	Sources        []string  `json:"sources"`
}

// Rate grades an item for an archetype. res supplies tier/weight facts when
// available (phase B); with NullResolver it falls back to heuristics.
func Rate(it *Item, arch Archetype, kb *Knowledge, res ModResolver) *Rating {
	r := &Rating{Archetype: arch, Sources: kb.Sources}

	if it.Corrupted {
		r.Grade, r.Verdict = "—", "Locked — corrupted, can't be crafted on (use or sell as-is)."
		r.Reasons = []string{"Corrupted items can't be modified by any crafting currency."}
		return r
	}
	if it.isUnique() {
		r.Grade, r.Verdict = "—", "Not a craft base — uniques have fixed mods (Divine for values; Verisium/Runeforging for some)."
		r.Reasons = []string{"Unique items don't take crafted prefixes/suffixes."}
		return r
	}

	r.Base, r.Mods, r.Difficulty = scoreBase(it, arch, kb, res, r), scoreMods(it, arch, kb, res, r), scoreHeadroom(it, arch, kb, r)
	r.DifficultyTier = difficultyTier(r.Difficulty)

	score := int(0.30*float64(r.Base) + 0.40*float64(r.Mods) + 0.30*float64(r.Difficulty) + 0.5)

	// NO-GO caps from the codex.
	openSlots := it.OpenPrefix + it.OpenSuffix
	if isRare(it) && openSlots == 0 && countFiller(it, arch, kb) >= 2 {
		if score > 55 {
			score = 55
		}
		r.Reasons = append(r.Reasons, "Full rare with multiple filler mods and no open slots — finishing it means annulling first (codex NO-GO).")
	}
	if fam, _, ok := weaponFamily(it); ok && fam != arch.Family {
		if score > 35 {
			score = 35
		}
		r.Reasons = append(r.Reasons, fmt.Sprintf("Wrong weapon family for your build (%s item, you play %s).", fam, arch.Family))
	}

	if score < 0 {
		score = 0
	} else if score > 100 {
		score = 100
	}
	r.Score = score
	r.Grade = gradeFor(score)
	r.Verdict = verdictFor(score)

	if !it.Annotated {
		r.Reasons = append(r.Reasons, "No tier data in the paste — enable Advanced Mod Descriptions in-game for a sharper grade.")
	}
	return r
}

// scoreBase rates the base: item level vs tier gates and weapon-family fit.
func scoreBase(it *Item, arch Archetype, kb *Knowledge, _ ModResolver, r *Rating) int {
	var s int
	switch {
	case it.ItemLevel == 0:
		s = 60
	case it.ItemLevel >= 82:
		s = 100
	case it.ItemLevel == 81:
		s = 95
	case it.ItemLevel >= 75:
		s = 80
	case it.ItemLevel >= 68:
		s = 55
	default:
		s = 30
	}
	if gate := kb.topTierIlvl(it.ItemClass); gate > 0 && it.ItemLevel > 0 {
		if it.ItemLevel >= gate {
			r.Reasons = append(r.Reasons, fmt.Sprintf("ilvl %d meets the ~%d gate for this slot's top-tier mods.", it.ItemLevel, gate))
		} else {
			r.Reasons = append(r.Reasons, fmt.Sprintf("ilvl %d is below the ~%d needed for this slot's T1 mods (caps the ceiling).", it.ItemLevel, gate))
		}
	}
	return s
}

// scoreMods rates the current mods, build-aware. Blank bases score as potential.
func scoreMods(it *Item, arch Archetype, kb *Knowledge, _ ModResolver, r *Rating) int {
	if !isRare(it) {
		r.Reasons = append(r.Reasons, "Near-blank base (Normal/Magic) — a clean canvas to craft on.")
		return 82
	}
	var desirable, filler, neutral int
	var good, bad []string
	for _, m := range it.Mods {
		if !m.occupiesAffix() {
			continue
		}
		switch kb.classifyDesirability(arch.Family, m.Text) {
		case Desirable:
			desirable++
			good = append(good, shorten(m.Text))
			if m.Tier == 1 || m.Tier == 2 {
				desirable++ // weight high-tier desirable mods extra
			}
		case FillerM:
			filler++
			bad = append(bad, shorten(m.Text))
		default:
			neutral++
		}
	}
	s := 50 + 12*desirable - 11*filler + 2*neutral
	if filler >= 3 {
		s -= 15
	}
	if s < 0 {
		s = 0
	} else if s > 100 {
		s = 100
	}
	if len(good) > 0 {
		r.Reasons = append(r.Reasons, fmt.Sprintf("%d desirable mod(s): %s.", desirable, strings.Join(dedupe(good), ", ")))
	}
	if len(bad) > 0 {
		r.Reasons = append(r.Reasons, fmt.Sprintf("%d filler mod(s) for your build: %s — expect to annul.", filler, strings.Join(dedupe(bad), ", ")))
	}
	return s
}

// scoreHeadroom rates how easy it is to finish the item (higher = easier).
func scoreHeadroom(it *Item, arch Archetype, kb *Knowledge, r *Rating) int {
	if !isRare(it) {
		return 80 // lots of room, but you still have to build it up
	}
	open := it.OpenPrefix + it.OpenSuffix
	s := 40 + 10*open - 8*countFiller(it, arch, kb) + 8*countDesirable(it, arch, kb)
	if s < 0 {
		s = 0
	} else if s > 100 {
		s = 100
	}
	r.Reasons = append(r.Reasons, fmt.Sprintf("%d open prefix / %d open suffix.", it.OpenPrefix, it.OpenSuffix))
	return s
}

func countFiller(it *Item, arch Archetype, kb *Knowledge) int {
	n := 0
	for _, m := range it.Mods {
		if m.occupiesAffix() && kb.classifyDesirability(arch.Family, m.Text) == FillerM {
			n++
		}
	}
	return n
}

func countDesirable(it *Item, arch Archetype, kb *Knowledge) int {
	n := 0
	for _, m := range it.Mods {
		if m.occupiesAffix() && kb.classifyDesirability(arch.Family, m.Text) == Desirable {
			n++
		}
	}
	return n
}

func isRare(it *Item) bool { return strings.EqualFold(it.Rarity, "Rare") }

// weaponFamily returns the item's weapon family if it is a weapon.
func weaponFamily(it *Item) (Family, string, bool) {
	if a, ok := archetypeFromBase(it.BaseType); ok {
		return a.Family, a.Weapon, true
	}
	if a, ok := archetypeFromBase(it.ItemClass); ok {
		return a.Family, a.Weapon, true
	}
	return "", "", false
}

func difficultyTier(headroom int) string {
	switch {
	case headroom >= 70:
		return "budget"
	case headroom >= 45:
		return "mid"
	default:
		return "high"
	}
}

func gradeFor(score int) string {
	switch {
	case score >= 97:
		return "A+"
	case score >= 93:
		return "A"
	case score >= 90:
		return "A-"
	case score >= 87:
		return "B+"
	case score >= 83:
		return "B"
	case score >= 80:
		return "B-"
	case score >= 77:
		return "C+"
	case score >= 73:
		return "C"
	case score >= 70:
		return "C-"
	case score >= 67:
		return "D+"
	case score >= 63:
		return "D"
	case score >= 60:
		return "D-"
	default:
		return "F"
	}
}

func verdictFor(score int) string {
	switch {
	case score >= 80:
		return "Craft on it — strong starting point."
	case score >= 65:
		return "Solid base — worth crafting if it fits your plan."
	case score >= 50:
		return "Marginal — only craft if it's cheap or fills a specific need."
	default:
		return "Don't craft — sell or discard."
	}
}

func shorten(s string) string {
	if len(s) > 48 {
		return s[:46] + "…"
	}
	return s
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
