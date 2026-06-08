// Package trade builds Path of Exile 2 trade-site search queries and renders
// them as clickable URLs. It performs NO network requests: per the project's
// ToS-safe design, it emits a pathofexile.com/trade2 URL the user opens in a
// browser. The trade site executes the query encoded in the `q` parameter.
package trade

import (
	"encoding/json"
	"net/url"

	"github.com/peteshima/poe-fissure/internal/analysis"
)

// Query is the trade2 search request body, mirrored to the structure the trade
// site accepts via its `q` URL parameter.
type Query struct {
	Query QueryBody `json:"query"`
	Sort  Sort      `json:"sort,omitempty"`
}

// QueryBody holds the search criteria.
type QueryBody struct {
	Status  StatusFilter `json:"status"`
	Stats   []StatGroup  `json:"stats,omitempty"`
	Filters Filters      `json:"filters,omitempty"`
}

// StatusFilter selects online/any sellers.
type StatusFilter struct {
	Option string `json:"option"` // "online" | "onlineleague" | "any"
}

// StatGroup is a group of stat filters combined with Type ("and", "count", ...).
// For a "count" group, Value.Min sets how many of the filters must match.
type StatGroup struct {
	Type    string       `json:"type"`
	Value   *StatValue   `json:"value,omitempty"`
	Filters []StatFilter `json:"filters"`
}

// StatFilter requires a single stat id to fall within Value.
type StatFilter struct {
	ID    string     `json:"id"`
	Value *StatValue `json:"value,omitempty"`
}

// StatValue bounds a stat filter.
type StatValue struct {
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

// Filters groups type and trade filters.
type Filters struct {
	TypeFilters  *FilterSet `json:"type_filters,omitempty"`
	TradeFilters *FilterSet `json:"trade_filters,omitempty"`
}

// FilterSet is the generic {filters: {...}} wrapper the trade API uses.
type FilterSet struct {
	Filters map[string]any `json:"filters"`
}

// Sort orders results (cheapest first by default).
type Sort struct {
	Price string `json:"price,omitempty"`
}

// MaxFilters is the trade API cap on total filters per query.
const MaxFilters = 35

// BuildOptions configures query construction for a single slot.
type BuildOptions struct {
	Category       string // trade2 type-filter category (e.g. "armour.gloves")
	Wants          []analysis.WantStat
	StatIDs        map[string]string // internal stat key -> trade2 stat id
	BudgetMax      float64
	BudgetCurrency string
	OnlineOnly     bool
}

// Build constructs a Query from options, honouring the 35-filter cap by keeping
// the highest-priority wants for which a stat id is known. Returns the query and
// the wants that were dropped because no stat id was available.
func Build(opts BuildOptions) (Query, []analysis.WantStat) {
	status := "any"
	if opts.OnlineOnly {
		status = "online"
	}
	q := Query{
		Query: QueryBody{Status: StatusFilter{Option: status}},
		Sort:  Sort{Price: "asc"},
	}

	if opts.Category != "" {
		q.Query.Filters.TypeFilters = &FilterSet{Filters: map[string]any{
			"category": map[string]any{"option": opts.Category},
		}}
	}
	if opts.BudgetMax > 0 {
		price := map[string]any{"max": opts.BudgetMax}
		if opts.BudgetCurrency != "" {
			price["option"] = opts.BudgetCurrency
		}
		q.Query.Filters.TradeFilters = &FilterSet{Filters: map[string]any{"price": price}}
	}

	var unmapped []analysis.WantStat
	group := StatGroup{Type: "and"}
	for _, w := range opts.Wants {
		if len(group.Filters) >= MaxFilters {
			break
		}
		id, ok := opts.StatIDs[w.Stat]
		if !ok {
			unmapped = append(unmapped, w)
			continue
		}
		min := w.Min
		group.Filters = append(group.Filters, StatFilter{ID: id, Value: &StatValue{Min: &min}})
	}
	if len(group.Filters) > 0 {
		// Requiring ALL wanted stats (an "and" group) is usually too strict to find
		// a real upgrade — e.g. few items carry all three resistances at once. With
		// 2+ stats, ask for "at least 2 of these" (a count group) so the search
		// returns plausible upgrades sorted cheapest-first instead of nothing.
		if len(group.Filters) >= 2 {
			group.Type = "count"
			min := 2.0
			group.Value = &StatValue{Min: &min}
		}
		q.Query.Stats = []StatGroup{group}
	}
	return q, unmapped
}

// JSON returns the compact JSON encoding used in the `q` URL parameter.
func (q Query) JSON() ([]byte, error) {
	return json.Marshal(q)
}

// encodeForURL returns the URL-encoded compact query for the `q` parameter.
func (q Query) encodeForURL() (string, error) {
	b, err := q.JSON()
	if err != nil {
		return "", err
	}
	return url.QueryEscape(string(b)), nil
}
