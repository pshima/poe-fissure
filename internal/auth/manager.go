package auth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Manager loads a stored token and transparently refreshes it when expired,
// persisting the refreshed token. It satisfies poeapi.TokenSource.
type Manager struct {
	cfg   Config
	store Store

	mu  sync.Mutex
	tok Token
}

// NewManager wires an OAuth config to a token store.
func NewManager(cfg Config, store Store) *Manager {
	return &Manager{cfg: cfg, store: store}
}

// AccessToken returns a valid access token, refreshing if needed.
func (m *Manager) AccessToken(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.tok.AccessToken == "" {
		t, err := m.store.Load()
		if err != nil {
			return "", fmt.Errorf("no stored token; run `poefissure auth login`: %w", err)
		}
		m.tok = t
	}
	if !m.tok.Expired(60 * time.Second) {
		return m.tok.AccessToken, nil
	}
	if m.tok.RefreshToken == "" {
		return "", fmt.Errorf("access token expired and no refresh token; run `poefissure auth login`")
	}
	refreshed, err := m.cfg.Refresh(ctx, m.tok.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("refresh token (re-run `poefissure auth login`): %w", err)
	}
	m.tok = refreshed
	if err := m.store.Save(refreshed); err != nil {
		return "", fmt.Errorf("persist refreshed token: %w", err)
	}
	return m.tok.AccessToken, nil
}
