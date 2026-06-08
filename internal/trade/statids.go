package trade

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
)

// StatIDs maps poe-fissure internal stat keys (analysis.Stat*) to PoE2 trade
// stat ids (e.g. "explicit.stat_3299347043"). These ids are GGG-internal (from
// the trade2 data/stats endpoint). A small, common set is bundled (see
// DefaultStatIDs) so trade filters work out of the box; a user file can extend or
// override it. Any stat without a mapping is omitted from generated queries.
type StatIDs map[string]string

//go:embed default_stat_ids.json
var defaultStatIDs []byte

// DefaultStatIDs returns the bundled stat-id map (life, resistances, and a few
// common offensive/attribute stats). These cover the stats the scorer searches
// on today; ids can go stale across patches, so a user file overrides them.
func DefaultStatIDs() StatIDs {
	ids := StatIDs{}
	_ = json.Unmarshal(defaultStatIDs, &ids)
	return ids
}

// LoadStatIDsMerged returns the bundled defaults overlaid with any entries from
// the user's file at path (file entries win). A missing file yields the defaults.
func LoadStatIDsMerged(path string) (StatIDs, error) {
	merged := DefaultStatIDs()
	fileIDs, err := LoadStatIDs(path)
	if err != nil {
		return merged, err
	}
	for k, v := range fileIDs {
		merged[k] = v
	}
	return merged, nil
}

// DefaultStatIDsPath returns ~/.config/poefissure/trade_stat_ids.json.
func DefaultStatIDsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "poefissure", "trade_stat_ids.json"), nil
}

// LoadStatIDs reads a stat-id map from path. A missing file yields an empty map
// (queries are still generated, just without stat filters).
func LoadStatIDs(path string) (StatIDs, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return StatIDs{}, nil
		}
		return nil, err
	}
	ids := StatIDs{}
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}
