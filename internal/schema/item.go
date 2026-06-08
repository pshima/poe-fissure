// Package schema models Grinding Gear Games' canonical Item and Character JSON
// objects as returned by the official Path of Exile API. The same Item object is
// reused across the character, stash, and trade APIs, so this is the single
// source of truth for item shape throughout poe-fissure.
//
// Field names and types follow the public API reference:
// https://www.pathofexile.com/developer/docs/reference#type-Item
package schema

// Item is GGG's canonical item object. Pointers / omitempty are used for fields
// the API only emits when present (e.g. corrupted is absent unless true), so
// marshalling round-trips closely to the original payload.
type Item struct {
	Verified   bool   `json:"verified"`
	W          int    `json:"w"`
	H          int    `json:"h"`
	Icon       string `json:"icon,omitempty"`
	League     string `json:"league,omitempty"`
	ID         string `json:"id,omitempty"`
	Name       string `json:"name"`
	TypeLine   string `json:"typeLine"`
	BaseType   string `json:"baseType"`
	Rarity     string `json:"rarity,omitempty"`
	Identified bool   `json:"identified"`
	ItemLevel  int    `json:"ilvl,omitempty"`
	Note       string `json:"note,omitempty"`

	Corrupted    *bool `json:"corrupted,omitempty"`
	Unmodifiable *bool `json:"unmodifiable,omitempty"`
	Duplicated   *bool `json:"duplicated,omitempty"`

	Sockets       []ItemSocket `json:"sockets,omitempty"`
	SocketedItems []Item       `json:"socketedItems,omitempty"`

	Properties           []ItemProperty `json:"properties,omitempty"`
	Requirements         []ItemProperty `json:"requirements,omitempty"`
	AdditionalProperties []ItemProperty `json:"additionalProperties,omitempty"`

	ImplicitMods  []string `json:"implicitMods,omitempty"`
	ExplicitMods  []string `json:"explicitMods,omitempty"`
	CraftedMods   []string `json:"craftedMods,omitempty"`
	EnchantMods   []string `json:"enchantMods,omitempty"`
	RuneMods      []string `json:"runeMods,omitempty"` // PoE2 socketed-rune granted mods
	FracturedMods []string `json:"fracturedMods,omitempty"`

	// InventoryId is the equipment slot name (Weapon, BodyArmour, Ring, Ring2, ...).
	InventoryID string `json:"inventoryId,omitempty"`
	// FrameType encodes rarity/special framing (0 normal .. 3 unique, etc.).
	FrameType int `json:"frameType,omitempty"`
}

// ItemSocket describes a single socket on an item.
type ItemSocket struct {
	Group   int    `json:"group"`
	Attr    string `json:"attr,omitempty"`
	SColour string `json:"sColour,omitempty"`
	Type    string `json:"type,omitempty"`
	Item    string `json:"item,omitempty"`
}

// ItemProperty is the {name, values, displayMode} tuple the API uses for stats,
// requirements, and similar. Values is a list of [stringValue, valueType] pairs.
type ItemProperty struct {
	Name        string   `json:"name"`
	Values      [][]any  `json:"values"`
	DisplayMode int      `json:"displayMode,omitempty"`
	Type        *int     `json:"type,omitempty"`
	Progress    *float64 `json:"progress,omitempty"`
}
