package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Token holds the OAuth tokens plus a computed access-token expiry. Refresh
// tokens for public clients last 7 days; access tokens 10 hours.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Scope        string    `json:"scope"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Expired reports whether the access token is expired or within skew of expiry.
func (t Token) Expired(skew time.Duration) bool {
	if t.AccessToken == "" {
		return true
	}
	return time.Now().Add(skew).After(t.ExpiresAt)
}

// tokenResponse is the raw /oauth/token JSON payload.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
}

func (r tokenResponse) toToken(now time.Time) Token {
	return Token{
		AccessToken:  r.AccessToken,
		RefreshToken: r.RefreshToken,
		TokenType:    r.TokenType,
		Scope:        r.Scope,
		ExpiresAt:    now.Add(time.Duration(r.ExpiresIn) * time.Second),
	}
}

// Store persists a Token to a 0600 file. Tokens are secrets and must never be
// world-readable or logged.
type Store struct {
	Path string
}

// DefaultStorePath returns ~/.config/poefissure/token.json.
func DefaultStorePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "poefissure", "token.json"), nil
}

// Load reads the stored token. Returns os.ErrNotExist if no token is saved.
func (s Store) Load() (Token, error) {
	var t Token
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return t, err
	}
	if err := json.Unmarshal(data, &t); err != nil {
		return t, fmt.Errorf("parse token store: %w", err)
	}
	return t, nil
}

// Save writes the token atomically with 0600 permissions.
func (s Store) Save(t Token) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}

// Clear removes the stored token (logout).
func (s Store) Clear() error {
	err := os.Remove(s.Path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
