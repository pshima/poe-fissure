// Package trade2 is an ON-DEMAND, bounded client for Path of Exile 2's
// undocumented trade API (api/trade2). It exists only to price-check a single
// item/slot when the user explicitly asks — never on a timer, never in bulk.
// Search and fetch each have an independent rate limiter that self-throttles
// from GGG's X-Rate-Limit response headers (mirroring how price-check overlays
// like Exiled-Exchange-2 stay within the accepted usage lane).
package trade2

// SearchResult is the POST /api/trade2/search response: a query id plus the
// matching listing ids (ordered by our requested sort — price ascending).
type SearchResult struct {
	ID     string   `json:"id"`
	Result []string `json:"result"`
	Total  int      `json:"total"`
}

// Price is a listing's asking price.
type Price struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// Listing is one trade result, trimmed to what the UI needs.
type Listing struct {
	Price    Price    `json:"price"`
	Seller   string   `json:"seller"`
	Name     string   `json:"name"`
	BaseType string   `json:"baseType"`
	Mods     []string `json:"mods"`
}

// --- raw fetch response shapes (api/trade2/fetch) ---

type fetchResponse struct {
	Result []fetchEntry `json:"result"`
}

type fetchEntry struct {
	ID      string       `json:"id"`
	Listing rawListing   `json:"listing"`
	Item    rawTradeItem `json:"item"`
}

type rawListing struct {
	Price   rawPrice   `json:"price"`
	Account rawAccount `json:"account"`
}

type rawPrice struct {
	Type     string  `json:"type"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type rawAccount struct {
	Name string `json:"name"`
}

type rawTradeItem struct {
	Name         string   `json:"name"`
	TypeLine     string   `json:"typeLine"`
	BaseType     string   `json:"baseType"`
	ImplicitMods []string `json:"implicitMods"`
	ExplicitMods []string `json:"explicitMods"`
	RuneMods     []string `json:"runeMods"`
}
