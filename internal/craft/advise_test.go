package craft

import (
	"strings"
	"testing"
)

func loadKB(t *testing.T) *Knowledge {
	t.Helper()
	kb, err := LoadKnowledge()
	if err != nil {
		t.Fatal(err)
	}
	return kb
}

func stepWithCurrency(p *Plan, substr string) bool {
	for _, s := range p.Steps {
		for _, c := range s.Currencies {
			if strings.Contains(c, substr) {
				return true
			}
		}
	}
	return false
}

func TestAdviseCorruptedNotCraftable(t *testing.T) {
	it, _ := Parse(realCrossbow)
	p := Advise(it, Goal{TargetText: "increased Physical Damage"}, loadKB(t))
	if p.Craftable {
		t.Fatal("corrupted item should not be craftable")
	}
	if !strings.Contains(p.Reason, "Corrupted") {
		t.Errorf("reason = %q", p.Reason)
	}
}

func TestAdviseUniqueSuggestsDivine(t *testing.T) {
	it, _ := Parse(uniqueArmour)
	p := Advise(it, Goal{}, loadKB(t))
	if p.Craftable {
		t.Fatal("unique should not be affix-craftable")
	}
	if !stepWithCurrency(p, "Divine") {
		t.Error("unique plan should suggest a Divine Orb")
	}
}

func TestAdviseFullSlotNeedsRemoval(t *testing.T) {
	it, _ := Parse(craftableStaff) // full 3 prefix / 3 suffix
	p := Advise(it, Goal{TargetText: "increased Spell Damage", Kind: Prefix}, loadKB(t))
	if !p.Craftable {
		t.Fatal("non-corrupted rare should be craftable")
	}
	if it.OpenPrefix != 0 {
		t.Fatalf("precondition: staff should have 0 open prefixes, got %d", it.OpenPrefix)
	}
	if !stepWithCurrency(p, "Annulment") {
		t.Error("a full prefix item should start with a removal step (Annulment)")
	}
	if len(p.Risks) == 0 {
		t.Error("removal should carry a brick-risk warning")
	}
}

func TestAdviseNeverRecommendsHomogenising(t *testing.T) {
	// On a non-jewellery item, even with a tag overlap, we must NOT recommend the
	// league-illegal Homogenising omen — force the slot type instead.
	const bow = `Item Class: Bow
Rarity: Rare
Foo Bar
Gemini Bow
--------
Item Level: 80
--------
120% increased Physical Damage
+40 to Dexterity`
	it, _ := Parse(bow)
	p := Advise(it, Goal{TargetText: "increased Physical Damage"}, loadKB(t))
	if p.TargetKind != Prefix {
		t.Fatalf("target kind = %q, want prefix", p.TargetKind)
	}
	if stepWithCurrency(p, "Homogenising") {
		t.Error("Homogenising is not obtainable in Runes of Aldur — must not be recommended")
	}
	if !stepWithCurrency(p, "Sinistral Exaltation") {
		t.Error("should force the prefix side with Sinistral Exaltation instead")
	}
}

func TestAdviseJewelleryUsesCatalysingExaltation(t *testing.T) {
	const ring = `Item Class: Ring
Rarity: Rare
Foo Bar
Sapphire Ring
--------
Item Level: 82
--------
Adds 5 to 12 Cold Damage to Attacks
+40 to maximum Life`
	it, _ := Parse(ring)
	p := Advise(it, Goal{TargetText: "Adds Cold Damage to Attacks", Kind: Prefix}, loadKB(t))
	if !stepWithCurrency(p, "Catalysing Exaltation") {
		t.Error("jewellery with a tag overlap should use Catalysing Exaltation (the in-league tag-bias tool)")
	}
}

func TestAdviseNamesEssenceForMagicItem(t *testing.T) {
	const magicES = `Item Class: Body Armour
Rarity: Magic
Archon's Flowing Raiment of the Ice
--------
Energy Shield: 242
--------
Item Level: 80
--------
+24 to maximum Energy Shield
37% increased Energy Shield
+36% to Cold Resistance`
	it, _ := Parse(magicES)
	p := Advise(it, Goal{TargetText: "increased Armour/Evasion/Energy Shield"}, loadKB(t))
	if !stepWithCurrency(p, "Essence of Enhancement") {
		t.Fatalf("magic item targeting Armour/Evasion/ES should name Essence of Enhancement; steps=%+v", p.Steps)
	}
}
