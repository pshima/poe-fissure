package craft

import "testing"

// realCrossbow is a verbatim clipboard paste from PoE2 0.5 (default copy, no
// Advanced Mod Descriptions) — used to validate the parser against real text.
const realCrossbow = `Item Class: Crossbow
Rarity: Rare
Storm Core
Trarthan Cannon
--------
Quality: +20%
Physical Damage: 340-692
Critical Hit Chance: 5.00%
Attacks per Second: 1.54
--------
Requires: Level 65, 114 Str, 63 Dex
--------
Sockets: S S
--------
Item Level: 76
--------
36% increased Physical Damage (rune)
Bonded: 40% increased effect of Fully Broken Armour (rune)
--------
Cannot load or fire Ammunition (implicit)
--------
179% increased Physical Damage
+139 to Accuracy Rating
10% increased Attack Speed
+16 to Dexterity
Grenade Skills have +1 Cooldown Use
Adds 32 to 49 Physical Damage (crafted)
--------
Corrupted`

func TestParseRealCrossbow(t *testing.T) {
	it, err := Parse(realCrossbow)
	if err != nil {
		t.Fatal(err)
	}
	if it.ItemClass != "Crossbow" {
		t.Errorf("itemClass = %q", it.ItemClass)
	}
	if it.Rarity != "Rare" {
		t.Errorf("rarity = %q", it.Rarity)
	}
	if it.Name != "Storm Core" {
		t.Errorf("name = %q", it.Name)
	}
	if it.BaseType != "Trarthan Cannon" {
		t.Errorf("baseType = %q", it.BaseType)
	}
	if it.ItemLevel != 76 {
		t.Errorf("itemLevel = %d", it.ItemLevel)
	}
	if it.Quality != 20 {
		t.Errorf("quality = %d", it.Quality)
	}
	if !it.Corrupted {
		t.Error("expected corrupted")
	}
	if it.Sockets != "S S" {
		t.Errorf("sockets = %q", it.Sockets)
	}
	if it.Annotated {
		t.Error("default copy should not be annotated")
	}

	// Source tagging.
	bySource := map[ModSource]int{}
	for _, m := range it.Mods {
		bySource[m.Source]++
	}
	if bySource[SrcRune] != 2 {
		t.Errorf("rune mods = %d, want 2", bySource[SrcRune])
	}
	if bySource[SrcImplicit] != 1 {
		t.Errorf("implicit mods = %d, want 1", bySource[SrcImplicit])
	}
	if bySource[SrcCrafted] != 1 {
		t.Errorf("crafted mods = %d, want 1", bySource[SrcCrafted])
	}

	// Heuristic classification: 179% phys + flat phys (crafted) = prefixes;
	// accuracy + attack speed + dex = suffixes; grenade cooldown = unclassified.
	if it.Prefixes != 2 {
		t.Errorf("prefixes = %d, want 2", it.Prefixes)
	}
	if it.Suffixes != 3 {
		t.Errorf("suffixes = %d, want 3", it.Suffixes)
	}
	var unknown int
	for _, m := range it.Mods {
		if m.occupiesAffix() && m.Kind == Unknown {
			unknown++
		}
	}
	if unknown != 1 {
		t.Errorf("unclassified affixes = %d, want 1 (grenade cooldown)", unknown)
	}
}

// desecratedStaff: every mod is a (desecrated) Abyss modifier; item is corrupted.
const desecratedStaff = `Item Class: Staff
Rarity: Rare
Rage Goad
Reaping Staff
--------
Quality: +20%
Grants Skill: Level 18 Reap
--------
Requires: Level 78, 137 Int
--------
Sockets: S
--------
Item Level: 82
--------
Gain 32% of Damage as Extra Cold Damage (desecrated)
Gain 59% of Damage as Extra Lightning Damage (desecrated)
81% increased Spell Damage (desecrated)
75% increased Critical Hit Chance for Spells (desecrated)
43% increased Critical Spell Damage Bonus (desecrated)
Gain 15 Mana per enemy killed (desecrated)
--------
Corrupted`

func TestParseDesecratedStaff(t *testing.T) {
	it, err := Parse(desecratedStaff)
	if err != nil {
		t.Fatal(err)
	}
	if it.BaseType != "Reaping Staff" || it.ItemLevel != 82 || !it.Corrupted {
		t.Fatalf("base/ilvl/corrupted wrong: %q %d %v", it.BaseType, it.ItemLevel, it.Corrupted)
	}
	// "Grants Skill:" is a base property, not a mod.
	for _, m := range it.Mods {
		if m.Text == "Grants Skill: Level 18 Reap" {
			t.Error("Grants Skill should not be a mod")
		}
	}
	desec := 0
	for _, m := range it.Mods {
		if m.Source == SrcDesecrated {
			desec++
		}
	}
	if desec != 6 {
		t.Errorf("desecrated mods = %d, want 6", desec)
	}
	if it.Prefixes != 3 || it.Suffixes != 3 {
		t.Errorf("prefixes/suffixes = %d/%d, want 3/3", it.Prefixes, it.Suffixes)
	}
}

// craftableStaff: a non-corrupted rare with the same archetype as plain explicits.
const craftableStaff = `Item Class: Staff
Rarity: Rare
Demon Mast
Gelid Staff
--------
Grants Skill: Level 14 Freezing Shards
--------
Requires: Level 58, 103 Int
--------
Item Level: 62
--------
Gain 51% of Damage as Extra Fire Damage
Gain 38% of Damage as Extra Cold Damage
110% increased Cold Damage
Gain 21 Life per enemy killed
Gain 22 Mana per enemy killed
47% increased Freeze Buildup`

func TestParseCraftableStaff(t *testing.T) {
	it, err := Parse(craftableStaff)
	if err != nil {
		t.Fatal(err)
	}
	if it.Corrupted {
		t.Error("should not be corrupted")
	}
	if it.ItemLevel != 62 {
		t.Errorf("ilvl = %d", it.ItemLevel)
	}
	// 3 prefixes (2x "as extra", increased cold) + 3 suffixes (2x on-kill, freeze buildup).
	if it.Prefixes != 3 || it.Suffixes != 3 {
		t.Fatalf("prefixes/suffixes = %d/%d, want 3/3 (full item)", it.Prefixes, it.Suffixes)
	}
	if it.OpenPrefix != 0 || it.OpenSuffix != 0 {
		t.Errorf("open slots = %d/%d, want 0/0", it.OpenPrefix, it.OpenSuffix)
	}
}

// uniqueArmour: uniques have fixed mods, not prefix/suffix affixes.
const uniqueArmour = `Item Class: Vaal Body Armour
Rarity: Unique
Atziri's Splendour
Sacrificial Regalia
--------
Armour: 718
Evasion Rating: 652
Energy Shield: 76
--------
Requires: Level 65
--------
Sockets: S S S S S S
--------
Item Level: 81
--------
+1 to Level of all Corrupted Skill Gems (implicit)
--------
Only Soul Cores can be Socketed in this item
This item gains bonuses from Socketed Soul Cores as though it was also a Shield
Has no Attribute Requirements
163% increased Armour and Evasion
+15% to all Elemental Resistances
Skills from Corrupted Gems have 50% of Mana Costs Converted to Life Costs`

func TestParseUniqueArmour(t *testing.T) {
	it, err := Parse(uniqueArmour)
	if err != nil {
		t.Fatal(err)
	}
	if it.Rarity != "Unique" || it.Name != "Atziri's Splendour" || it.BaseType != "Sacrificial Regalia" {
		t.Fatalf("header wrong: %q %q %q", it.Rarity, it.Name, it.BaseType)
	}
	if it.Sockets != "S S S S S S" {
		t.Errorf("sockets = %q", it.Sockets)
	}
	// Uniques: do not report craftable prefix/suffix slots.
	if it.Prefixes != 0 || it.Suffixes != 0 {
		t.Errorf("unique should report 0/0 affixes, got %d/%d", it.Prefixes, it.Suffixes)
	}
	// The +1 corrupted gems line is an implicit, not an explicit mod.
	implicits := 0
	for _, m := range it.Mods {
		if m.Source == SrcImplicit {
			implicits++
		}
	}
	if implicits != 1 {
		t.Errorf("implicits = %d, want 1", implicits)
	}
}

// annotatedItem exercises the Advanced Mod Descriptions path.
const annotatedItem = `Item Class: Body Armours
Rarity: Rare
Crimson Mantle
Vile Robe
--------
Item Level: 83
--------
{ Prefix Modifier "Wraithlike" (Tier: 1) — Defense, Life }
+95 to Maximum Life
{ Suffix Modifier "of the Polar Bear" (Tier: 2) — Resistance, Cold }
+38% to Cold Resistance`

func TestParseAnnotated(t *testing.T) {
	it, err := Parse(annotatedItem)
	if err != nil {
		t.Fatal(err)
	}
	if !it.Annotated {
		t.Fatal("expected annotated = true")
	}
	if it.Prefixes != 1 || it.Suffixes != 1 {
		t.Fatalf("prefixes=%d suffixes=%d, want 1/1", it.Prefixes, it.Suffixes)
	}
	if it.OpenPrefix != 2 || it.OpenSuffix != 2 {
		t.Errorf("open prefix/suffix = %d/%d, want 2/2", it.OpenPrefix, it.OpenSuffix)
	}
	life := it.Mods[0]
	if life.Kind != Prefix || life.Tier != 1 || life.Name != "Wraithlike" {
		t.Errorf("life mod parsed wrong: %+v", life)
	}
	if len(life.Tags) != 2 || life.Tags[0] != "Defense" {
		t.Errorf("life tags = %v", life.Tags)
	}
}
