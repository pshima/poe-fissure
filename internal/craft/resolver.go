package craft

// ModResolver supplies per-base modifier facts (tiers, ilvl gates, weights) that
// the in-game clipboard text does NOT contain. Phase A ships nullResolver (always
// "unknown") so the rating falls back to heuristics; phase B adds a poe2db-backed
// resolver behind this same interface with no call-site changes.
type ModResolver interface {
	// Tier returns the modifier's tier (1 = best) for a rolled value on a base,
	// or 0 if unknown.
	Tier(base, modText string, value float64) int
	// IlvlGate returns the minimum item level for the modifier's top tier, or 0.
	IlvlGate(base, modText string) int
	// Weight returns the modifier's relative roll weight on a base, or 0.
	Weight(base, modText string) int
	// PoolFor returns the base's full modifier pool, or nil if unknown.
	PoolFor(base string) []ModInfo
}

// ModInfo describes one modifier available on a base.
type ModInfo struct {
	Text     string   `json:"text"`
	Tier     int      `json:"tier"`
	IlvlGate int      `json:"ilvlGate"`
	Weight   int      `json:"weight"`
	Tags     []string `json:"tags"`
}

// nullResolver knows nothing; every lookup is "unknown".
type nullResolver struct{}

func (nullResolver) Tier(string, string, float64) int { return 0 }
func (nullResolver) IlvlGate(string, string) int      { return 0 }
func (nullResolver) Weight(string, string) int        { return 0 }
func (nullResolver) PoolFor(string) []ModInfo         { return nil }

// NullResolver returns a resolver that always reports "unknown" (phase A default).
func NullResolver() ModResolver { return nullResolver{} }
