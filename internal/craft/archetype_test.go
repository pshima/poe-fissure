package craft

import (
	"testing"

	"github.com/peteshima/poe-fissure/internal/schema"
)

func TestDeriveArchetypeFromBow(t *testing.T) {
	c := &schema.Character{
		Class:     "Ranger",
		Equipment: []schema.Item{{InventoryID: "Weapon", BaseType: "Obliterator Bow", Name: "Viper Thunder"}},
	}
	a := DeriveArchetype(c)
	if a.Family != FamilyAttack || a.Weapon != "bow" {
		t.Fatalf("got %+v, want attack/bow", a)
	}
}

func TestDeriveArchetypeQuarterstaffNotCaster(t *testing.T) {
	c := &schema.Character{Equipment: []schema.Item{{InventoryID: "Weapon", BaseType: "Gelid Quarterstaff"}}}
	if a := DeriveArchetype(c); a.Family != FamilyAttack || a.Weapon != "quarterstaff" {
		t.Fatalf("quarterstaff should be attack/quarterstaff, got %+v", a)
	}
}

func TestClassifyDesirability(t *testing.T) {
	kb := loadKB(t)
	cases := []struct {
		family Family
		mod    string
		want   Desirability
	}{
		{FamilyAttack, "179% increased Physical Damage", Desirable},
		{FamilyAttack, "81% increased Spell Damage", FillerM},
		{FamilyAttack, "+139 to Accuracy Rating", Desirable},
		{FamilyCaster, "81% increased Spell Damage", Desirable},
		{FamilyCaster, "10% increased Attack Speed", FillerM},
	}
	for _, c := range cases {
		if got := kb.classifyDesirability(c.family, c.mod); got != c.want {
			t.Errorf("%s / %q = %q, want %q", c.family, c.mod, got, c.want)
		}
	}
}
