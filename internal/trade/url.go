package trade

import (
	"fmt"
	"net/url"
	"strings"
)

// TradeBaseURL is the PoE2 trade site search base.
const TradeBaseURL = "https://www.pathofexile.com/trade2/search"

// URL renders a clickable trade2 search URL for the given realm/league. The
// query is embedded in the `q` parameter so the trade site executes it on open;
// no API request is made by poe-fissure.
func URL(realm, league string, q Query) (string, error) {
	encoded, err := q.encodeForURL()
	if err != nil {
		return "", err
	}
	// Path: /trade2/search/<realm>/<league>
	path := fmt.Sprintf("%s/%s/%s", TradeBaseURL, url.PathEscape(realm), url.PathEscape(league))
	return path + "?q=" + encoded, nil
}

// WeaponCategory maps a weapon/quiver base type to its trade2 type-filter
// category option (e.g. "Obliterator Bow" -> "weapon.bow", a quiver ->
// "armour.quiver"). Returns "" if the base is not a recognised weapon/quiver, so
// the caller can fall back to SlotCategory. Order matters: "crossbow" contains
// "bow" and "quarterstaff" contains "staff", so the more specific names go first.
func WeaponCategory(baseType string) string {
	b := strings.ToLower(baseType)
	switch {
	case strings.Contains(b, "quiver"):
		return "armour.quiver"
	case strings.Contains(b, "crossbow"):
		return "weapon.crossbow"
	case strings.Contains(b, "bow"):
		return "weapon.bow"
	case strings.Contains(b, "quarterstaff"):
		return "weapon.warstaff"
	case strings.Contains(b, "wand"):
		return "weapon.wand"
	case strings.Contains(b, "sceptre"):
		return "weapon.sceptre"
	case strings.Contains(b, "staff"):
		return "weapon.staff"
	}
	return ""
}

// SlotCategory maps an equipment inventory slot to a trade2 type-filter category
// option. Empty string means "no category filter" (search will be broad). For
// weapon/offhand slots the type varies by build, so prefer WeaponCategory on the
// equipped item; this only provides the slot-stable armour/accessory categories.
func SlotCategory(slot string) string {
	switch slot {
	case "Weapon", "Offhand", "Weapon2", "Offhand2":
		return "" // weapon types vary; derive from the equipped item via WeaponCategory
	case "Helm":
		return "armour.helmet"
	case "BodyArmour":
		return "armour.chest"
	case "Gloves":
		return "armour.gloves"
	case "Boots":
		return "armour.boots"
	case "Ring", "Ring2":
		return "accessory.ring"
	case "Amulet":
		return "accessory.amulet"
	case "Belt":
		return "accessory.belt"
	default:
		return ""
	}
}
