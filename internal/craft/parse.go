// Package craft parses in-game Path of Exile 2 item text and advises on
// best-value crafting steps for the current patch (0.5.0 / Runes of Aldur).
//
// The parser handles the standard clipboard format. Two variants exist: the
// default copy (plain mod lines, as most players have) and the "advanced"
// copy produced when "Advanced Mod Descriptions" is enabled, which annotates
// each mod with its affix type and tier:
//
//	{ Prefix Modifier "Razor-sharp" (Tier: 3) — Damage, Physical, Attack }
//	Adds 24 to 51 Physical Damage
//
// When annotations are present we trust them for prefix/suffix/tier; otherwise
// we classify explicit mods heuristically (see classify.go).
package craft

import (
	"regexp"
	"strconv"
	"strings"
)

// AffixKind is whether a mod occupies a prefix or suffix slot.
type AffixKind string

const (
	Prefix  AffixKind = "prefix"
	Suffix  AffixKind = "suffix"
	Unknown AffixKind = "unknown"
)

// ModSource is where a mod comes from; only explicit/crafted/fractured mods
// occupy one of the six prefix/suffix slots.
type ModSource string

const (
	SrcExplicit   ModSource = "explicit"
	SrcImplicit   ModSource = "implicit"
	SrcCrafted    ModSource = "crafted"
	SrcEnchant    ModSource = "enchant"
	SrcRune       ModSource = "rune"
	SrcFractured  ModSource = "fractured"
	SrcDesecrated ModSource = "desecrated" // Abyss / Well of Souls modifiers
)

// Mod is one parsed modifier line.
type Mod struct {
	Text   string    `json:"text"`
	Source ModSource `json:"source"`
	Kind   AffixKind `json:"kind"`           // meaningful for explicit/crafted/fractured
	Tier   int       `json:"tier,omitempty"` // 0 = unknown (not in text)
	Name   string    `json:"name,omitempty"` // affix name if annotated
	Tags   []string  `json:"tags,omitempty"` // mod tags if annotated
}

// occupiesAffix reports whether the mod takes a prefix/suffix slot.
func (m Mod) occupiesAffix() bool {
	switch m.Source {
	case SrcExplicit, SrcCrafted, SrcFractured, SrcDesecrated:
		return true
	}
	return false
}

// Item is a parsed item.
type Item struct {
	ItemClass    string `json:"itemClass"`
	Rarity       string `json:"rarity"`
	Name         string `json:"name"`
	BaseType     string `json:"baseType"`
	ItemLevel    int    `json:"itemLevel"`
	Quality      int    `json:"quality"`
	Corrupted    bool   `json:"corrupted"`
	Sockets      string `json:"sockets"`
	Requirements string `json:"requirements,omitempty"`
	Mods         []Mod  `json:"mods"`

	// Annotated is true when affix type/tier came from the item text itself
	// (Advanced Mod Descriptions on) rather than from heuristic classification.
	Annotated bool `json:"annotated"`

	Prefixes   int `json:"prefixes"`
	Suffixes   int `json:"suffixes"`
	OpenPrefix int `json:"openPrefix"`
	OpenSuffix int `json:"openSuffix"`
}

var (
	sepRe        = regexp.MustCompile(`^-{3,}$`)
	sourceTagRe  = regexp.MustCompile(`\s*\((rune|implicit|crafted|enchant|fractured|scourge|desecrated)\)\s*$`)
	itemLevelRe  = regexp.MustCompile(`^Item Level:\s*(\d+)`)
	qualityRe    = regexp.MustCompile(`^Quality:\s*\+?(\d+)`)
	annotationRe = regexp.MustCompile(`^\{\s*(Prefix|Suffix|Implicit|Rune|Enchant|Crafted|Fractured)\s+Modifier(?:\s+"([^"]*)")?(?:\s+\(Tier:\s*(\d+)\))?\s*(?:[—\-]\s*(.*?))?\s*\}$`)
	// Lines that are item properties, not mods.
	propertyRe = regexp.MustCompile(`^(Quality|Physical Damage|Elemental Damage|Fire Damage|Cold Damage|Lightning Damage|Chaos Damage|Critical Hit Chance|Attacks per Second|Reload Time|Energy Shield|Armour|Evasion Rating|Spirit|Block|Sockets|Item Level|Requires|Level|Stack Size|Unidentified|Note|Grants Skill):`)
)

// Parse parses pasted PoE2 item clipboard text. It is lenient: unknown lines are
// ignored rather than failing, so partial pastes still yield a usable item.
func Parse(text string) (*Item, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")

	it := &Item{}
	var pending *Mod // annotation awaiting its value line

	// Header block: "Item Class:", "Rarity:", then name/base line(s).
	var headerNames []string
	i := 0
	for ; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if sepRe.MatchString(line) {
			i++
			break
		}
		switch {
		case strings.HasPrefix(line, "Item Class:"):
			it.ItemClass = strings.TrimSpace(strings.TrimPrefix(line, "Item Class:"))
		case strings.HasPrefix(line, "Rarity:"):
			it.Rarity = strings.TrimSpace(strings.TrimPrefix(line, "Rarity:"))
		case line != "":
			headerNames = append(headerNames, line)
		}
	}
	switch {
	case len(headerNames) >= 2: // Rare/Unique: random name + base type
		it.Name = headerNames[0]
		it.BaseType = headerNames[1]
	case len(headerNames) == 1: // Normal/Magic: single line (base, possibly w/ magic affixes)
		it.BaseType = headerNames[0]
	}

	for ; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || sepRe.MatchString(line) {
			continue
		}
		if line == "Corrupted" {
			it.Corrupted = true
			continue
		}
		if m := itemLevelRe.FindStringSubmatch(line); m != nil {
			it.ItemLevel, _ = strconv.Atoi(m[1])
			continue
		}
		if m := qualityRe.FindStringSubmatch(line); m != nil {
			it.Quality, _ = strconv.Atoi(m[1])
			continue
		}
		if strings.HasPrefix(line, "Sockets:") {
			it.Sockets = strings.TrimSpace(strings.TrimPrefix(line, "Sockets:"))
			continue
		}
		if strings.HasPrefix(line, "Requires") {
			it.Requirements = strings.TrimSpace(strings.TrimPrefix(line, "Requires:"))
			continue
		}
		// Advanced annotation line: remember it for the next mod value line.
		if m := annotationRe.FindStringSubmatch(line); m != nil {
			it.Annotated = true
			pending = &Mod{Name: m[2]}
			pending.Kind = annotationKind(m[1])
			pending.Source = annotationSource(m[1])
			if m[3] != "" {
				pending.Tier, _ = strconv.Atoi(m[3])
			}
			if m[4] != "" {
				pending.Tags = splitTags(m[4])
			}
			continue
		}
		if propertyRe.MatchString(line) {
			continue
		}

		// A mod value line.
		mod := Mod{Text: line, Source: SrcExplicit, Kind: Unknown}
		if tag := sourceTagRe.FindStringSubmatch(line); tag != nil {
			mod.Source = tagToSource(tag[1])
			mod.Text = strings.TrimSpace(sourceTagRe.ReplaceAllString(line, ""))
		}
		if pending != nil {
			// Merge annotation (authoritative for kind/tier/tags/name).
			if pending.Source != "" {
				mod.Source = pending.Source
			}
			mod.Kind = pending.Kind
			mod.Tier = pending.Tier
			mod.Name = pending.Name
			mod.Tags = pending.Tags
			pending = nil
		} else if mod.occupiesAffix() && !it.isUnique() {
			// Unique mods are fixed, not prefix/suffix affixes — don't classify.
			mod.Kind = classify(mod.Text)
		}
		it.Mods = append(it.Mods, mod)
	}

	countAffixes(it)
	return it, nil
}

// isUnique reports whether the item is a Unique (fixed mods, not affix-craftable).
func (it *Item) isUnique() bool {
	return strings.EqualFold(it.Rarity, "Unique")
}

func countAffixes(it *Item) {
	if it.isUnique() {
		return // uniques have no prefix/suffix affix slots to track
	}
	for _, m := range it.Mods {
		if !m.occupiesAffix() {
			continue
		}
		switch m.Kind {
		case Prefix:
			it.Prefixes++
		case Suffix:
			it.Suffixes++
		}
	}
	it.OpenPrefix = 3 - it.Prefixes
	it.OpenSuffix = 3 - it.Suffixes
	if it.OpenPrefix < 0 {
		it.OpenPrefix = 0
	}
	if it.OpenSuffix < 0 {
		it.OpenSuffix = 0
	}
}

func tagToSource(tag string) ModSource {
	switch tag {
	case "rune":
		return SrcRune
	case "implicit":
		return SrcImplicit
	case "crafted":
		return SrcCrafted
	case "enchant":
		return SrcEnchant
	case "fractured":
		return SrcFractured
	case "desecrated":
		return SrcDesecrated
	default:
		return SrcExplicit
	}
}

func annotationKind(label string) AffixKind {
	switch label {
	case "Prefix":
		return Prefix
	case "Suffix":
		return Suffix
	default:
		return Unknown
	}
}

func annotationSource(label string) ModSource {
	switch label {
	case "Implicit":
		return SrcImplicit
	case "Rune":
		return SrcRune
	case "Enchant":
		return SrcEnchant
	case "Crafted":
		return SrcCrafted
	case "Fractured":
		return SrcFractured
	default:
		return SrcExplicit
	}
}

func splitTags(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
