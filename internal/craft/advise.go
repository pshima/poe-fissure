package craft

import (
	"fmt"
	"strings"
)

// Goal is what the user wants to add/achieve on the item.
type Goal struct {
	// TargetText is the desired modifier in plain words (e.g. "increased
	// Physical Damage"). Kind is inferred from it when not given.
	TargetText string    `json:"targetText"`
	Kind       AffixKind `json:"kind,omitempty"`
}

// Step is one ordered instruction.
type Step struct {
	Action     string   `json:"action"`
	Currencies []string `json:"currencies,omitempty"`
	Note       string   `json:"note,omitempty"`
}

// Plan is the advisor's output for an item + goal.
type Plan struct {
	Summary     string    `json:"summary"`
	Craftable   bool      `json:"craftable"`
	Reason      string    `json:"reason,omitempty"`
	Goal        string    `json:"goal,omitempty"`
	TargetKind  AffixKind `json:"targetKind,omitempty"`
	Steps       []Step    `json:"steps,omitempty"`
	Risks       []string  `json:"risks,omitempty"`
	Suggestions []string  `json:"suggestions,omitempty"`
	Sources     []string  `json:"sources,omitempty"`
}

// Advise produces a best-value crafting plan for the item toward the goal.
func Advise(it *Item, goal Goal, kb *Knowledge) *Plan {
	p := &Plan{
		Summary:     describe(it),
		Suggestions: kb.targetsFor(it.ItemClass),
		Sources:     kb.Sources,
	}

	switch {
	case it.Corrupted:
		p.Craftable = false
		p.Reason = "This item is Corrupted — crafting currency can no longer modify it. It's locked as-is."
		return p
	case it.isUnique():
		p.Craftable = false
		p.Reason = "Unique items have fixed modifiers — they can't take crafted prefixes/suffixes. You can re-roll their numeric values with a Divine Orb, and some uniques can be enhanced via Verisium / Runeforging (0.5)."
		p.Steps = []Step{{
			Action:     "Re-roll this unique's numeric values toward higher rolls.",
			Currencies: []string{kb.currency("divine").Name},
			Note:       "Divine only changes values within each mod's existing range; it won't change which mods the unique has.",
		}}
		return p
	case !strings.EqualFold(it.Rarity, "Rare"):
		p.Craftable = true
		target := strings.TrimSpace(goal.TargetText)
		p.Goal = target
		if ess, ok := kb.essenceFor(target); ok {
			p.Reason = fmt.Sprintf("Item is %s. Use an Essence to make it Rare while guaranteeing %q, then fill the rest.", it.Rarity, target)
			p.Steps = append(p.Steps, Step{
				Action:     fmt.Sprintf("Upgrade to Rare and guarantee %q with %s.", target, ess.Essence),
				Currencies: []string{ess.Essence},
				Note:       fmt.Sprintf("An Essence on a Magic/Normal base makes it Rare and guarantees %s. Use the Greater tier on an ilvl 75+ base for high tiers. (Essence names can shift between patches — confirm on PoE2DB.)", ess.Guarantees),
			})
		} else {
			p.Reason = "Item is " + it.Rarity + ". Make it Rare first, then add affixes."
			p.Steps = append(p.Steps, Step{
				Action:     "Turn the base into a Rare, guaranteeing a key mod with an Essence.",
				Currencies: []string{"Essence (matching your target)", kb.currency("regal").Name, kb.currency("alchemy").Name},
				Note:       "Pick an Essence whose guaranteed mod matches your target; otherwise Regal a good Magic item. Start from an ilvl 75+ base for top tiers.",
			})
		}
		p.Steps = append(p.Steps, Step{
			Action:     "Fill the remaining open prefixes/suffixes toward your other priorities.",
			Currencies: []string{kb.currency("exalted").Name, kb.omen("sinistral_exaltation").Name + " / " + kb.omen("dextral_exaltation").Name},
			Note:       "Use Sinistral (prefix) / Dextral (suffix) Exaltation to control the side, then Divine for value rolls once the mods are set.",
		})
		return p
	}

	// Rare, not corrupted: craftable.
	p.Craftable = true

	target := strings.TrimSpace(goal.TargetText)
	if target == "" {
		p.Reason = "Pick a target modifier to craft (see suggestions for this item type), then I'll give exact steps."
		return p
	}
	p.Goal = target

	kind := goal.Kind
	if kind == "" || kind == Unknown {
		kind = classify(target)
	}
	p.TargetKind = kind
	if kind == Unknown {
		p.Reason = fmt.Sprintf("I can't tell if %q is a prefix or suffix without affix annotations. Enable Advanced Mod Descriptions and re-paste, or tell me which it is.", target)
		return p
	}

	open := it.OpenPrefix
	slotWord := "prefix"
	annulOmen := "sinistral_annulment"
	exaltOmen := "sinistral_exaltation"
	if kind == Suffix {
		open = it.OpenSuffix
		slotWord = "suffix"
		annulOmen = "dextral_annulment"
		exaltOmen = "dextral_exaltation"
	}

	// If the target slot type is full, remove one of that type first.
	if open == 0 {
		p.Steps = append(p.Steps, Step{
			Action:     fmt.Sprintf("No open %s slot — remove an unwanted %s first.", slotWord, slotWord),
			Currencies: []string{kb.currency("annulment").Name, kb.omen(annulOmen).Name},
			Note:       fmt.Sprintf("The omen forces the annul to hit a %s. It's still random among your %ses, so protect keepers with a %s first if you can.", slotWord, slotWord, kb.currency("fracturing").Name),
		})
		p.Risks = append(p.Risks, fmt.Sprintf("Annulment can remove a good %s — only do this if you have an expendable %s, or fracture your keepers first.", slotWord, slotWord))
	}

	// Targeted add. Note: Omen of Homogenising Exaltation (the old tag-lock trick)
	// is NOT obtainable in the Runes of Aldur league, so we never recommend it. On
	// jewellery, catalyst quality + Omen of Catalysing Exaltation is the in-league
	// tag-bias tool; everywhere else, force the slot type and repeat.
	if isJewellery(it.ItemClass) && hasTagOverlap(it, target) {
		p.Steps = append(p.Steps, Step{
			Action:     fmt.Sprintf("Bias the slam toward %q using catalyst tags (jewellery).", target),
			Currencies: []string{"Catalysts (matching tag)", kb.currency("exalted").Name, kb.omen("catalysing_exaltation").Name},
			Note:       "Apply catalysts of the target's tag, then Exalt under Catalysing Exaltation — it multiplies tagged-mod weight ~5x (20% quality) to ~7.5x (40%). Breach Rings reach 50%. The league replacement for Homogenising.",
		})
	} else {
		p.Steps = append(p.Steps, Step{
			Action:     fmt.Sprintf("Add a %s, forcing the slot type, until %q lands.", slotWord, target),
			Currencies: []string{kb.currency("exalted").Name, kb.omen(exaltOmen).Name},
			Note:       fmt.Sprintf("The omen guarantees a %s; the specific mod is still random within the %s pool, so repeat as needed. (Homogenising Exaltation isn't available this league.)", slotWord, slotWord),
		})
	}

	// Deterministic alternative — name the specific essence when we know it.
	detCurrency := "Perfect Essence / Alloy"
	if ess, ok := kb.essenceFor(target); ok {
		detCurrency = ess.Essence + " (or a matching Alloy)"
	}
	p.Steps = append(p.Steps, Step{
		Action:     "Deterministic alternative: guarantee the mod instead of slamming.",
		Currencies: []string{detCurrency},
		Note:       "Guarantees the mod but REMOVES a random existing mod (0.5: max one essence/alloy mod per item). Fill the opposite slot type first so the removal can't hit a keeper.",
	})

	// Finish.
	p.Steps = append(p.Steps, Step{
		Action:     "Once the mods you want are present, optimise their rolls.",
		Currencies: []string{kb.currency("divine").Name},
		Note:       "Divine re-rolls values within each mod's tier range.",
	})

	return p
}

// isJewellery reports whether the item class is a ring/amulet/belt (where the
// catalyst-bias technique applies).
func isJewellery(itemClass string) bool {
	return containsAny(strings.ToLower(itemClass), "ring", "amulet", "belt")
}

// describe summarises the item's craftable state.
func describe(it *Item) string {
	name := it.BaseType
	if it.Name != "" {
		name = it.Name + " (" + it.BaseType + ")"
	}
	head := fmt.Sprintf("%s %s %s", it.Rarity, it.ItemClass, name)
	if it.ItemLevel > 0 {
		head += fmt.Sprintf(", item level %d", it.ItemLevel)
	}
	if it.Corrupted {
		return head + " — Corrupted (locked)."
	}
	if it.isUnique() {
		return head + " — Unique (fixed mods)."
	}
	return fmt.Sprintf("%s. %d prefix / %d suffix used — %d open prefix, %d open suffix.",
		head, it.Prefixes, it.Suffixes, it.OpenPrefix, it.OpenSuffix)
}

// tagKeywords are the damage/defence/utility families used to judge whether a
// target mod shares a "tag" with an existing mod (drives the Homogenising route
// when affix annotations aren't available).
var tagKeywords = []string{
	"physical", "fire", "cold", "lightning", "chaos", "elemental",
	"attack", "spell", "projectile", "minion", "critical",
	"life", "mana", "energy shield", "armour", "evasion",
	"accuracy", "attribute", "resistance", "speed",
}

func tagsOf(text string) map[string]bool {
	s := strings.ToLower(text)
	out := map[string]bool{}
	for _, k := range tagKeywords {
		if strings.Contains(s, k) {
			out[k] = true
		}
	}
	return out
}

// hasTagOverlap reports whether the target mod shares a tag-family with any mod
// already on the item (using annotation tags when present, else text keywords).
func hasTagOverlap(it *Item, target string) bool {
	want := tagsOf(target)
	if len(want) == 0 {
		return false
	}
	for _, m := range it.Mods {
		// Prefer real annotation tags when available.
		for _, t := range m.Tags {
			if want[strings.ToLower(t)] {
				return true
			}
		}
		for k := range tagsOf(m.Text) {
			if want[k] {
				return true
			}
		}
	}
	return false
}
