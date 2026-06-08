// Package analysis turns item mods into comparable stat bundles and scores
// candidate items against an equipped item for a set of build goals. The math is
// a transparent heuristic, not a PoB-accurate simulation: it ranks which slots
// and mods most improve the build so the trade layer can search for them.
package analysis

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/peteshima/poe-fissure/internal/schema"
)

// Stat keys used throughout scoring.
const (
	StatLife       = "life"
	StatES         = "energy_shield"
	StatMana       = "mana"
	StatArmour     = "armour"
	StatEvasion    = "evasion"
	StatFireRes    = "fire_res"
	StatColdRes    = "cold_res"
	StatLightRes   = "lightning_res"
	StatChaosRes   = "chaos_res"
	StatStr        = "strength"
	StatDex        = "dexterity"
	StatInt        = "intelligence"
	StatPhysInc    = "phys_increased"
	StatPhysAdded  = "phys_added"
	StatEleAdded   = "ele_added"
	StatAttackSpd  = "attack_speed"
	StatCastSpd    = "cast_speed"
	StatCritChance = "crit_chance"
	StatCritMulti  = "crit_multi"
	StatSpellInc   = "spell_increased"
)

// Stats is a bag of additive stat contributions parsed from item mods.
type Stats map[string]float64

// Add merges other into s.
func (s Stats) Add(other Stats) {
	for k, v := range other {
		s[k] += v
	}
}

var (
	reLife       = regexp.MustCompile(`(?i)\+?(\d+) to maximum Life`)
	reES         = regexp.MustCompile(`(?i)\+?(\d+) to maximum Energy Shield`)
	reMana       = regexp.MustCompile(`(?i)\+?(\d+) to maximum Mana`)
	reArmour     = regexp.MustCompile(`(?i)\+?(\d+) to Armour\b`)
	reEvasion    = regexp.MustCompile(`(?i)\+?(\d+) to Evasion Rating`)
	reRes        = regexp.MustCompile(`(?i)\+?(\d+)% to (Fire|Cold|Lightning|Chaos) Resistance`)
	reAllEleRes  = regexp.MustCompile(`(?i)\+?(\d+)% to all Elemental Resistances`)
	reAttr       = regexp.MustCompile(`(?i)\+?(\d+) to (Strength|Dexterity|Intelligence)`)
	reAllAttr    = regexp.MustCompile(`(?i)\+?(\d+) to all Attributes`)
	rePhysInc    = regexp.MustCompile(`(?i)(\d+)% increased.*Physical Damage`)
	rePhysAdded  = regexp.MustCompile(`(?i)Adds (\d+) to (\d+) Physical Damage`)
	reEleAdded   = regexp.MustCompile(`(?i)Adds (\d+) to (\d+) (Fire|Cold|Lightning) Damage`)
	reAtkSpd     = regexp.MustCompile(`(?i)(\d+)% increased Attack Speed`)
	reCastSpd    = regexp.MustCompile(`(?i)(\d+)% increased Cast Speed`)
	reCritChance = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)% increased Critical (?:Strike|Hit) Chance`)
	reCritMulti  = regexp.MustCompile(`(?i)\+?(\d+)% to Critical (?:Strike|Hit) (?:Multiplier|Damage Bonus)`)
	reSpellInc   = regexp.MustCompile(`(?i)(\d+)% increased Spell Damage`)
)

func num(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// ParseMods extracts a Stats bundle from a list of mod lines.
func ParseMods(mods []string) Stats {
	st := Stats{}
	for _, m := range mods {
		if g := reLife.FindStringSubmatch(m); g != nil {
			st[StatLife] += num(g[1])
		}
		if g := reES.FindStringSubmatch(m); g != nil {
			st[StatES] += num(g[1])
		}
		if g := reMana.FindStringSubmatch(m); g != nil {
			st[StatMana] += num(g[1])
		}
		if g := reArmour.FindStringSubmatch(m); g != nil {
			st[StatArmour] += num(g[1])
		}
		if g := reEvasion.FindStringSubmatch(m); g != nil {
			st[StatEvasion] += num(g[1])
		}
		if g := reRes.FindStringSubmatch(m); g != nil {
			switch strings.ToLower(g[2]) {
			case "fire":
				st[StatFireRes] += num(g[1])
			case "cold":
				st[StatColdRes] += num(g[1])
			case "lightning":
				st[StatLightRes] += num(g[1])
			case "chaos":
				st[StatChaosRes] += num(g[1])
			}
		}
		if g := reAllEleRes.FindStringSubmatch(m); g != nil {
			v := num(g[1])
			st[StatFireRes] += v
			st[StatColdRes] += v
			st[StatLightRes] += v
		}
		if g := reAttr.FindStringSubmatch(m); g != nil {
			switch strings.ToLower(g[2]) {
			case "strength":
				st[StatStr] += num(g[1])
			case "dexterity":
				st[StatDex] += num(g[1])
			case "intelligence":
				st[StatInt] += num(g[1])
			}
		}
		if g := reAllAttr.FindStringSubmatch(m); g != nil {
			v := num(g[1])
			st[StatStr] += v
			st[StatDex] += v
			st[StatInt] += v
		}
		if g := rePhysInc.FindStringSubmatch(m); g != nil {
			st[StatPhysInc] += num(g[1])
		}
		if g := rePhysAdded.FindStringSubmatch(m); g != nil {
			st[StatPhysAdded] += (num(g[1]) + num(g[2])) / 2
		}
		if g := reEleAdded.FindStringSubmatch(m); g != nil {
			st[StatEleAdded] += (num(g[1]) + num(g[2])) / 2
		}
		if g := reAtkSpd.FindStringSubmatch(m); g != nil {
			st[StatAttackSpd] += num(g[1])
		}
		if g := reCastSpd.FindStringSubmatch(m); g != nil {
			st[StatCastSpd] += num(g[1])
		}
		if g := reCritChance.FindStringSubmatch(m); g != nil {
			st[StatCritChance] += num(g[1])
		}
		if g := reCritMulti.FindStringSubmatch(m); g != nil {
			st[StatCritMulti] += num(g[1])
		}
		if g := reSpellInc.FindStringSubmatch(m); g != nil {
			st[StatSpellInc] += num(g[1])
		}
	}
	return st
}

// ItemStats parses every mod tier on an item into one bundle.
func ItemStats(it schema.Item) Stats {
	st := Stats{}
	st.Add(ParseMods(it.ImplicitMods))
	st.Add(ParseMods(it.ExplicitMods))
	st.Add(ParseMods(it.CraftedMods))
	st.Add(ParseMods(it.EnchantMods))
	st.Add(ParseMods(it.RuneMods))
	st.Add(ParseMods(it.FracturedMods))
	return st
}

// CharacterStats aggregates stats across all equipped items. This is an
// item-only view (no passive tree / base values) used for relative comparison
// and gap detection, not absolute character sheet values.
func CharacterStats(c *schema.Character) Stats {
	st := Stats{}
	for _, it := range c.Equipment {
		st.Add(ItemStats(it))
	}
	return st
}
