package craft

import (
	_ "embed"
	"encoding/json"
	"strings"
)

// kb050 is the bundled 0.5.0 crafting knowledge base. It is embedded so the
// binary is self-contained; a new patch is a new data file + Knowledge.Version.
//
//go:embed data/0.5.0.json
var kb050 []byte

// Effect pairs a currency/omen display name with what it does.
type Effect struct {
	Name   string `json:"name"`
	Effect string `json:"effect"`
}

// ArchetypeMatchers are keyword lists deciding desirable vs filler mods per build
// family. Matching is case-insensitive substring.
type ArchetypeMatchers struct {
	Desirable []string `json:"desirable"`
	Filler    []string `json:"filler"`
}

// Technique is one of the codex's named crafting moves.
type Technique struct {
	Name string `json:"name"`
	When string `json:"when"`
	How  string `json:"how"`
}

// Breakpoint records the item level at which a modifier's top tier unlocks.
type Breakpoint struct {
	Mod  string `json:"mod"`
	Slot string `json:"slot"`
	Ilvl int    `json:"ilvl"`
}

// EssenceTarget maps a desired mod to the essence that guarantees it.
type EssenceTarget struct {
	Essence    string   `json:"essence"`
	Guarantees string   `json:"guarantees"`
	Match      []string `json:"match"`
}

// Knowledge is the parsed crafting knowledge base for one patch.
type Knowledge struct {
	Version           string                       `json:"version"`
	League            string                       `json:"league"`
	Currencies        map[string]Effect            `json:"currencies"`
	Omens             map[string]Effect            `json:"omens"`
	SuggestedTargets  map[string][]string          `json:"suggestedTargets"`
	ArchetypeMods     map[string]ArchetypeMatchers `json:"archetypeMods"`
	EssenceTargets    []EssenceTarget              `json:"essenceTargets"`
	IlvlBreakpoints   []Breakpoint                 `json:"ilvlBreakpoints"`
	Techniques        []Technique                  `json:"techniques"`
	WorkflowOrder     []string                     `json:"workflowOrder"`
	LeagueUnavailable []string                     `json:"leagueUnavailable"`
	Sources           []string                     `json:"sources"`
}

// topTierIlvl returns the highest top-tier ilvl gate among breakpoints that apply
// to the given item class (slot ""=any), or 0 if none recorded.
func (k *Knowledge) topTierIlvl(itemClass string) int {
	best := 0
	for _, b := range k.IlvlBreakpoints {
		if b.Slot != "" && !strings.Contains(itemClass, b.Slot) {
			continue
		}
		if b.Ilvl > best {
			best = b.Ilvl
		}
	}
	return best
}

// LoadKnowledge returns the bundled knowledge base for the current patch.
func LoadKnowledge() (*Knowledge, error) {
	var k Knowledge
	if err := json.Unmarshal(kb050, &k); err != nil {
		return nil, err
	}
	return &k, nil
}

// targetsFor returns suggested target mods for an item class, matching loosely
// (e.g. "Vaal Body Armour" → "Body Armour").
func (k *Knowledge) targetsFor(itemClass string) []string {
	if t, ok := k.SuggestedTargets[itemClass]; ok {
		return t
	}
	for key, t := range k.SuggestedTargets {
		if strings.Contains(itemClass, key) {
			return t
		}
	}
	return nil
}

// SuggestionsFor returns suggested target mods for an item class (exported for
// the HTTP layer).
func (k *Knowledge) SuggestionsFor(itemClass string) []string {
	return k.targetsFor(itemClass)
}

// essenceFor finds the essence that guarantees a mod matching the target text
// (case-insensitive substring on the essence's match keywords).
func (k *Knowledge) essenceFor(target string) (EssenceTarget, bool) {
	t := strings.ToLower(strings.TrimSpace(target))
	if t == "" {
		return EssenceTarget{}, false
	}
	for _, e := range k.EssenceTargets {
		for _, m := range e.Match {
			if strings.Contains(t, strings.ToLower(m)) {
				return e, true
			}
		}
	}
	return EssenceTarget{}, false
}

// currency / omen accessors with a safe fallback name.
func (k *Knowledge) currency(id string) Effect { return lookup(k.Currencies, id) }
func (k *Knowledge) omen(id string) Effect     { return lookup(k.Omens, id) }

func lookup(m map[string]Effect, id string) Effect {
	if e, ok := m[id]; ok {
		return e
	}
	return Effect{Name: id}
}
