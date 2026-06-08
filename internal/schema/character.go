package schema

// Character is GGG's canonical character object. PoE2 populates Skills (socketed
// skill gems) and Jewels; PoE1-only fields (Inventory, Rucksack, Ruthless) are
// modeled for completeness but will be nil for poe2 realm responses.
type Character struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Realm      string `json:"realm"`
	Class      string `json:"class"`
	League     string `json:"league,omitempty"`
	Level      int    `json:"level"`
	Experience int64  `json:"experience"`

	Ruthless *bool `json:"ruthless,omitempty"`
	Expired  *bool `json:"expired,omitempty"`

	Equipment []Item    `json:"equipment,omitempty"`
	Skills    []Item    `json:"skills,omitempty"`    // PoE2: socketed skill gems
	Inventory []Item    `json:"inventory,omitempty"` // PoE1 only
	Rucksack  []Item    `json:"rucksack,omitempty"`  // PoE1 only
	Jewels    []Item    `json:"jewels,omitempty"`
	Passives  *Passives `json:"passives,omitempty"`
}

// Passives is the allocated passive tree state.
type Passives struct {
	Hashes         []int          `json:"hashes,omitempty"`
	HashesEx       []int          `json:"hashes_ex,omitempty"`
	MasteryEffects map[string]int `json:"mastery_effects,omitempty"`
	SkillOverrides map[string]any `json:"skill_overrides,omitempty"`
	BanditChoice   string         `json:"bandit_choice,omitempty"`
	JewelData      map[string]any `json:"jewel_data,omitempty"`
}

// CharacterResponse wraps the single-character GET endpoint response.
type CharacterResponse struct {
	Character *Character `json:"character"`
}

// CharacterListResponse wraps the list-characters endpoint response.
type CharacterListResponse struct {
	Characters []Character `json:"characters"`
}

// EquippedBySlot indexes equipment by its inventory slot name for quick lookup.
func (c *Character) EquippedBySlot() map[string]Item {
	out := make(map[string]Item, len(c.Equipment))
	for _, it := range c.Equipment {
		if it.InventoryID != "" {
			out[it.InventoryID] = it
		}
	}
	return out
}
