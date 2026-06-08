package craft

import "strings"

// classify guesses whether an explicit mod is a prefix or suffix from its text,
// for the common case where the item text lacks affix annotations. It covers the
// mods that matter most for gearing; anything unrecognised returns Unknown so the
// advisor can flag it rather than miscount. When the player enables Advanced Mod
// Descriptions, the parser uses the real annotations and this is not consulted.
//
// Rule of thumb in PoE2: added/“increased local” power and the defensive/offensive
// pools (life, mana, ES, armour, evasion, flat & %-physical/elemental/spell damage,
// gem levels) are PREFIXES; utility “of …” mods (resistances, attributes, attack/
// cast speed, accuracy, crit, movement/leech/on-kill) are SUFFIXES.
func classify(text string) AffixKind {
	s := strings.ToLower(text)

	// Suffixes first — several contain words that would otherwise look prefixy.
	for _, sub := range suffixHints {
		if strings.Contains(s, sub) {
			return Suffix
		}
	}
	for _, sub := range prefixHints {
		if strings.Contains(s, sub) {
			return Prefix
		}
	}
	return Unknown
}

var suffixHints = []string{
	"resistance",
	"to strength",
	"to dexterity",
	"to intelligence",
	"to all attributes",
	"attack speed",
	"cast speed",
	"accuracy rating",
	"critical",
	"movement speed",
	"life leech",
	"mana leech",
	"leeched",
	"per enemy killed",
	"life per enemy",
	"mana per enemy",
	"increased light radius",
	"reduced attribute requirements",
	"thorns",
	"buildup", // freeze/ignite/shock/stun buildup are suffixes
	"ailment",
}

var prefixHints = []string{
	"increased physical damage",
	"increased elemental damage",
	"increased fire damage",
	"increased cold damage",
	"increased lightning damage",
	"increased chaos damage",
	"increased spell damage",
	"adds", // "Adds # to # X Damage" — flat damage is a prefix
	"maximum life",
	"maximum mana",
	"maximum energy shield",
	"increased energy shield",
	"to armour",
	"increased armour",
	"to evasion",
	"increased evasion",
	"to level of all",
	"increased mana",
	"flat life regeneration",
	"life regeneration rate",
	"as extra", // "Gain #% of Damage as Extra X Damage" is a prefix
}
