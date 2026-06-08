package pob

import (
	"strings"
	"testing"

	"github.com/peteshima/poe-fissure/internal/schema"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	xml := []byte(`<?xml version="1.0"?><PathOfBuilding><Build level="87"/></PathOfBuilding>`)
	code, err := Encode(xml)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(code, "+/") {
		t.Errorf("code is not URL-safe: %q", code)
	}
	got, err := Decode(code)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(xml) {
		t.Errorf("round-trip mismatch:\n got %q\nwant %q", got, xml)
	}
}

func TestDecodeLenient(t *testing.T) {
	xml := []byte("hello path of building")
	code, _ := Encode(xml)
	// Simulate a standard-base64 variant by swapping the alphabet back.
	std := strings.NewReplacer("-", "+", "_", "/").Replace(code)
	got, err := Decode(std)
	if err != nil {
		t.Fatalf("lenient decode failed: %v", err)
	}
	if string(got) != string(xml) {
		t.Errorf("got %q", got)
	}
}

func TestBuildCode(t *testing.T) {
	c := &schema.Character{
		Name:  "Frozenmulligan",
		Class: "Sorceress",
		Level: 87,
		Equipment: []schema.Item{
			{Name: "Doom Visor", BaseType: "Lacquered Helmet", Rarity: "Rare", InventoryID: "Helm",
				ItemLevel: 81, ExplicitMods: []string{"+72 to maximum Life", "+34% to Fire Resistance"}},
		},
		Passives: &schema.Passives{Hashes: []int{12345, 23456}},
		Skills:   []schema.Item{{BaseType: "Frostbolt"}},
	}
	code, err := BuildCode(c)
	if err != nil {
		t.Fatal(err)
	}
	xml, err := Decode(code)
	if err != nil {
		t.Fatal(err)
	}
	s := string(xml)
	for _, want := range []string{"<PathOfBuilding>", `className="Sorceress"`, "Doom Visor", `nodes="12345,23456"`, "Frostbolt", `name="Helmet"`} {
		if !strings.Contains(s, want) {
			t.Errorf("generated XML missing %q\n---\n%s", want, s)
		}
	}
}

func TestItemTextFormat(t *testing.T) {
	it := schema.Item{
		Name: "Doom Visor", BaseType: "Lacquered Helmet", Rarity: "Rare", ItemLevel: 81,
		ImplicitMods: []string{"+18 to maximum Mana"},
		ExplicitMods: []string{"+72 to maximum Life"},
	}
	txt := ItemText(it)
	if !strings.HasPrefix(txt, "Rarity: RARE") {
		t.Errorf("missing rarity header: %q", txt)
	}
	if !strings.Contains(txt, "Item Level: 81") {
		t.Errorf("missing item level: %q", txt)
	}
}
