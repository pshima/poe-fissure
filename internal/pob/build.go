package pob

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/peteshima/poe-fissure/internal/schema"
)

// markupRe matches GGG's API description hint markup. The API wraps game terms as
// [Reference] or [Reference|Display Text]; the in-game/clipboard "standard" text
// shows only the display side.
var markupRe = regexp.MustCompile(`\[([^\[\]]+)\]`)

// cleanMarkup strips GGG's [Reference|Display] hint markup down to the displayed
// text, matching how an item reads when copied in-game.
func cleanMarkup(s string) string {
	return markupRe.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[1 : len(m)-1]
		if i := strings.IndexByte(inner, '|'); i >= 0 {
			return inner[i+1:]
		}
		return inner
	})
}

// BuildCode produces a Path of Building 2 import code from a character snapshot.
// The generated XML is best-effort: it captures equipped items (in PoB's
// copy-paste item-text format), the allocated passive tree, and skill gems.
// Always verify by importing into PoB2.
func BuildCode(c *schema.Character) (string, error) {
	x, err := BuildXML(c)
	if err != nil {
		return "", err
	}
	return Encode(x)
}

// BuildXML renders the PoB2 build XML for a character.
func BuildXML(c *schema.Character) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("nil character")
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<PathOfBuilding>` + "\n")
	fmt.Fprintf(&b, "  <Build level=\"%d\" className=\"%s\" ascendClassName=\"\">\n", c.Level, xmlEscape(c.Class))
	b.WriteString("  </Build>\n")

	// Items + a single ItemSet mapping slots to item ids.
	b.WriteString("  <Items>\n")
	type slotRef struct{ slot, id string }
	var refs []slotRef
	for i, it := range c.Equipment {
		id := strconv.Itoa(i + 1)
		fmt.Fprintf(&b, "    <Item id=\"%s\">%s</Item>\n", id, xmlEscape(ItemText(it)))
		if slot := pobSlot(it.InventoryID); slot != "" {
			refs = append(refs, slotRef{slot: slot, id: id})
		}
	}
	b.WriteString("    <ItemSet>\n")
	for _, r := range refs {
		fmt.Fprintf(&b, "      <Slot name=\"%s\" itemId=\"%s\"/>\n", xmlEscape(r.slot), r.id)
	}
	b.WriteString("    </ItemSet>\n")
	b.WriteString("  </Items>\n")

	// Passive tree spec: comma-separated allocated node hashes.
	if c.Passives != nil && len(c.Passives.Hashes) > 0 {
		nodes := make([]string, 0, len(c.Passives.Hashes))
		for _, h := range c.Passives.Hashes {
			nodes = append(nodes, strconv.Itoa(h))
		}
		b.WriteString("  <Tree>\n")
		fmt.Fprintf(&b, "    <Spec nodes=\"%s\"/>\n", strings.Join(nodes, ","))
		b.WriteString("  </Tree>\n")
	}

	// Skills: one group per socketed skill gem.
	if len(c.Skills) > 0 {
		b.WriteString("  <Skills>\n")
		b.WriteString("    <SkillSet>\n")
		for _, s := range c.Skills {
			name := s.BaseType
			if name == "" {
				name = s.TypeLine
			}
			fmt.Fprintf(&b, "      <Skill enabled=\"true\"><Gem nameSpec=\"%s\" level=\"1\" quality=\"0\"/></Skill>\n", xmlEscape(name))
		}
		b.WriteString("    </SkillSet>\n")
		b.WriteString("  </Skills>\n")
	}

	b.WriteString("</PathOfBuilding>\n")
	return []byte(b.String()), nil
}

// ItemText renders an item in Path of Building's copy-paste text format.
func ItemText(it schema.Item) string {
	var b strings.Builder
	rarity := it.Rarity
	if rarity == "" {
		rarity = "Normal"
	}
	fmt.Fprintf(&b, "Rarity: %s\n", strings.ToUpper(rarity))
	if it.Name != "" {
		b.WriteString(it.Name + "\n")
	}
	base := it.BaseType
	if base == "" {
		base = it.TypeLine
	}
	b.WriteString(base + "\n")
	b.WriteString("--------\n")
	if it.ItemLevel > 0 {
		fmt.Fprintf(&b, "Item Level: %d\n--------\n", it.ItemLevel)
	}
	for _, m := range it.EnchantMods {
		b.WriteString(cleanMarkup(m) + "\n")
	}
	for _, m := range it.ImplicitMods {
		fmt.Fprintf(&b, "%s\n", cleanMarkup(m))
	}
	if len(it.ImplicitMods) > 0 {
		b.WriteString("--------\n")
	}
	for _, m := range it.ExplicitMods {
		b.WriteString(cleanMarkup(m) + "\n")
	}
	for _, m := range it.CraftedMods {
		fmt.Fprintf(&b, "%s (crafted)\n", cleanMarkup(m))
	}
	for _, m := range it.RuneMods {
		fmt.Fprintf(&b, "%s (rune)\n", cleanMarkup(m))
	}
	if it.Corrupted != nil && *it.Corrupted {
		b.WriteString("--------\nCorrupted\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// pobSlot maps an API inventoryId to a PoB slot name.
func pobSlot(inventoryID string) string {
	switch inventoryID {
	case "Weapon":
		return "Weapon 1"
	case "Offhand":
		return "Weapon 2"
	case "Weapon2":
		return "Weapon 1 Swap"
	case "Offhand2":
		return "Weapon 2 Swap"
	case "Helm":
		return "Helmet"
	case "BodyArmour":
		return "Body Armour"
	case "Gloves":
		return "Gloves"
	case "Boots":
		return "Boots"
	case "Ring":
		return "Ring 1"
	case "Ring2":
		return "Ring 2"
	case "Amulet":
		return "Amulet"
	case "Belt":
		return "Belt"
	default:
		return ""
	}
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
