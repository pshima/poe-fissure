// Package config loads poe-fissure runtime configuration from a YAML file.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all user-tunable settings. Secrets (tokens) are NOT stored here;
// see internal/auth for token persistence.
type Config struct {
	// ClientID is the OAuth public client id. Defaults to "pob", the shared public
	// client Path of Building ships (no secret — it's plaintext in their source),
	// which lets this tool authenticate without a separate GGG registration. Set
	// your own GGG-approved client id here to stop riding on pob's registration.
	ClientID string `yaml:"client_id"`
	// Contact is the email embedded in the required User-Agent header.
	Contact string `yaml:"contact"`
	// Version is reported in the User-Agent header.
	Version string `yaml:"version"`

	// Realm is the API realm segment. For PoE2 this is "poe2".
	Realm string `yaml:"realm"`
	// Character is the default character name to query.
	Character string `yaml:"character"`
	// League is the exact league name (e.g. "Runes of Aldur").
	League string `yaml:"league"`

	// RedirectPort is the localhost port for the OAuth callback when using your own
	// client_id (redirect URI is the bare http://localhost:<port>, no path). It is
	// IGNORED for the default "pob" client, which must use pob's registered ports
	// (49082-49084); the login flow binds one of those automatically.
	RedirectPort int `yaml:"redirect_port"`

	// Budget bounds upgrade suggestions.
	Budget Budget `yaml:"budget"`
	// Weights tune the heuristic upgrade scorer.
	Weights Weights `yaml:"weights"`
}

// Budget caps how expensive a suggested upgrade may be.
type Budget struct {
	Currency string  `yaml:"currency"` // e.g. "divine", "exalted", "chaos"
	Amount   float64 `yaml:"amount"`
}

// Weights are multipliers applied per scoring goal. Higher = more influence on
// the ranked upgrade output.
type Weights struct {
	DPS           float64 `yaml:"dps"`
	Survivability float64 `yaml:"survivability"`
	Resists       float64 `yaml:"resists"`
	Attributes    float64 `yaml:"attributes"`
}

// Default returns config with sensible defaults filled in.
func Default() Config {
	return Config{
		ClientID:     "pob",
		Contact:      "",
		Version:      "0.1.0",
		Realm:        "poe2",
		Character:    "Frozenmulligan",
		League:       "Runes of Aldur",
		RedirectPort: 49082,
		Budget:       Budget{Currency: "divine", Amount: 50},
		Weights:      Weights{DPS: 1.0, Survivability: 1.0, Resists: 1.5, Attributes: 1.0},
	}
}

// UserAgent returns the GGG-mandated User-Agent header value.
// Format: OAuth {clientId}/{version} (contact: {email})
func (c Config) UserAgent() string {
	id := c.ClientID
	if id == "" {
		id = "poe-fissure"
	}
	return fmt.Sprintf("OAuth %s/%s (contact: %s)", id, c.Version, c.Contact)
}

// DefaultPath returns the standard config location (~/.config/poefissure/config.yaml).
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "poefissure", "config.yaml"), nil
}

// Load reads config from path, overlaying onto defaults. A missing file is not
// an error: defaults are returned so the tool is usable for offline work.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}
