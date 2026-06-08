package craft

import (
	"strings"

	"github.com/peteshima/poe-fissure/internal/schema"
)

// Family is a build's broad damage/role category, which decides what counts as a
// "desirable" mod versus filler when rating an item.
type Family string

const (
	FamilyAttack  Family = "attack"
	FamilyCaster  Family = "caster"
	FamilyMinion  Family = "minion"
	FamilyDefence Family = "defence"
)

// Archetype is a coarse description of the player's build, derived from their
// equipped weapon and class. It drives build-aware item rating.
type Archetype struct {
	Family Family `json:"family"`
	Weapon string `json:"weapon"` // bow, crossbow, staff, wand, sceptre, melee, ...
}

// DeriveArchetype infers an archetype from a character's equipped weapon (most
// reliable) with the class as a fallback. Unknown inputs default to attack.
func DeriveArchetype(c *schema.Character) Archetype {
	if c != nil {
		for _, it := range c.Equipment {
			if it.InventoryID == "Weapon" || it.InventoryID == "Weapon2" {
				if a, ok := archetypeFromBase(it.BaseType); ok {
					return a
				}
			}
		}
		if a, ok := archetypeFromClass(c.Class); ok {
			return a
		}
	}
	return Archetype{Family: FamilyAttack, Weapon: "unknown"}
}

// archetypeFromBase maps a weapon base type to an archetype. Order matters:
// "Quarterstaff" must be checked before "Staff".
func archetypeFromBase(base string) (Archetype, bool) {
	b := strings.ToLower(base)
	switch {
	case strings.Contains(b, "crossbow"):
		return Archetype{FamilyAttack, "crossbow"}, true
	case strings.Contains(b, "bow"):
		return Archetype{FamilyAttack, "bow"}, true
	case strings.Contains(b, "quarterstaff"):
		return Archetype{FamilyAttack, "quarterstaff"}, true
	case strings.Contains(b, "wand"):
		return Archetype{FamilyCaster, "wand"}, true
	case strings.Contains(b, "sceptre"):
		return Archetype{FamilyMinion, "sceptre"}, true
	case strings.Contains(b, "staff"):
		return Archetype{FamilyCaster, "staff"}, true
	case containsAny(b, "mace", "axe", "sword", "spear", "flail", "dagger", "claw", "hammer"):
		return Archetype{FamilyAttack, "melee"}, true
	}
	return Archetype{}, false
}

func archetypeFromClass(class string) (Archetype, bool) {
	switch strings.ToLower(class) {
	case "witch", "sorceress", "druid", "chronomancer", "stormweaver":
		return Archetype{FamilyCaster, "unknown"}, true
	case "ranger", "mercenary", "warrior", "monk", "huntress", "deadeye", "pathfinder", "gladiator":
		return Archetype{FamilyAttack, "unknown"}, true
	}
	return Archetype{}, false
}

// Desirability classifies a mod for an archetype.
type Desirability string

const (
	Desirable Desirability = "desirable"
	NeutralM  Desirability = "neutral"
	FillerM   Desirability = "filler"
)

// classifyDesirability decides whether modText helps the archetype, is filler, or
// neutral, using the KB's data-driven keyword matchers (filler checked first so a
// mod like "Critical Spell Damage" reads as filler for an attack build).
func (k *Knowledge) classifyDesirability(family Family, modText string) Desirability {
	am, ok := k.ArchetypeMods[string(family)]
	if !ok {
		return NeutralM
	}
	s := strings.ToLower(modText)
	for _, f := range am.Filler {
		if strings.Contains(s, strings.ToLower(f)) {
			return FillerM
		}
	}
	for _, d := range am.Desirable {
		if strings.Contains(s, strings.ToLower(d)) {
			return Desirable
		}
	}
	return NeutralM
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
