package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func loadFixture(t *testing.T) CharacterResponse {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "character_frozenmulligan.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var resp CharacterResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return resp
}

func TestCharacterDecode(t *testing.T) {
	resp := loadFixture(t)
	c := resp.Character
	if c == nil {
		t.Fatal("character is nil")
	}
	if c.Name != "Frozenmulligan" {
		t.Errorf("name = %q, want Frozenmulligan", c.Name)
	}
	if c.Realm != "poe2" {
		t.Errorf("realm = %q, want poe2", c.Realm)
	}
	if c.League != "Runes of Aldur" {
		t.Errorf("league = %q", c.League)
	}
	if len(c.Equipment) != 4 {
		t.Errorf("equipment count = %d, want 4", len(c.Equipment))
	}
	if len(c.Skills) != 2 {
		t.Errorf("skills count = %d, want 2", len(c.Skills))
	}
	if c.Passives == nil || len(c.Passives.Hashes) != 4 {
		t.Errorf("passives hashes not parsed")
	}
}

func TestEquippedBySlot(t *testing.T) {
	c := loadFixture(t).Character
	bySlot := c.EquippedBySlot()
	helm, ok := bySlot["Helm"]
	if !ok {
		t.Fatal("Helm slot missing")
	}
	if helm.Name != "Doom Visor" {
		t.Errorf("Helm name = %q", helm.Name)
	}
}

func TestRoundTrip(t *testing.T) {
	c := loadFixture(t).Character
	// Marshal then unmarshal; modeled fields must survive.
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var got Character
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != c.Name || got.Level != c.Level || len(got.Equipment) != len(c.Equipment) {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if got.Equipment[1].RuneMods[0] != "+12% to Lightning Resistance" {
		t.Errorf("runeMods not preserved: %v", got.Equipment[1].RuneMods)
	}
}
