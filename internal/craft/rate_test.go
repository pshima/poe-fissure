package craft

import (
	"strings"
	"testing"
)

var attackArch = Archetype{Family: FamilyAttack, Weapon: "bow"}
var casterArch = Archetype{Family: FamilyCaster, Weapon: "staff"}

func rate(t *testing.T, text string, arch Archetype) *Rating {
	t.Helper()
	it, err := Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	return Rate(it, arch, loadKB(t), NullResolver())
}

func TestRateCorruptedLocked(t *testing.T) {
	r := rate(t, realCrossbow, attackArch)
	if r.Grade != "—" || !strings.Contains(r.Verdict, "Locked") {
		t.Fatalf("corrupted should be Locked, got grade=%q verdict=%q", r.Grade, r.Verdict)
	}
}

func TestRateUniqueNotCraftBase(t *testing.T) {
	r := rate(t, uniqueArmour, attackArch)
	if r.Grade != "—" || !strings.Contains(r.Verdict, "Not a craft base") {
		t.Fatalf("unique should be 'Not a craft base', got grade=%q verdict=%q", r.Grade, r.Verdict)
	}
}

func TestRateBuildAwareness(t *testing.T) {
	// The caster staff is a caster weapon: rating it for an attack build must score
	// strictly worse than for a caster build (wrong-weapon cap + filler).
	attack := rate(t, craftableStaff, attackArch)
	caster := rate(t, craftableStaff, casterArch)
	if attack.Score >= caster.Score {
		t.Fatalf("attack score (%d) should be < caster score (%d)", attack.Score, caster.Score)
	}
	if attack.Score > 35 {
		t.Errorf("attack on a caster weapon should be capped low, got %d", attack.Score)
	}
}

func TestRateBlankHighIlvlBaseIsGood(t *testing.T) {
	const blankBow = `Item Class: Bow
Rarity: Normal
Gemini Bow
--------
Item Level: 82`
	r := rate(t, blankBow, attackArch)
	if r.Score < 80 {
		t.Fatalf("a clean ilvl-82 bow base for an attack build should grade well, got %d (%s)", r.Score, r.Grade)
	}
	if !strings.Contains(r.Verdict, "Craft on it") {
		t.Errorf("verdict = %q", r.Verdict)
	}
}
