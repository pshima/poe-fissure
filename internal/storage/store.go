// Package storage persists character snapshots locally as JSON files, keeping a
// timestamped history so equipment changes can be diffed over time. JSON keeps
// the data inspectable and dependency-free; the raw API payload is stored
// verbatim alongside the parsed character.
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/peteshima/poe-fissure/internal/schema"
)

// Snapshot is one captured state of a character at a point in time.
type Snapshot struct {
	CapturedAt time.Time         `json:"captured_at"`
	Realm      string            `json:"realm"`
	Character  *schema.Character `json:"character"`
	// Raw is the original API JSON body, preserved losslessly.
	Raw json.RawMessage `json:"raw,omitempty"`
}

// Store is a directory of snapshot files: <dir>/<character>/<timestamp>.json.
type Store struct {
	Dir string
}

// DefaultDir returns ~/.local/share/poefissure (or OS equivalent under config).
func DefaultDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "poefissure", "snapshots"), nil
}

func (s Store) charDir(character string) string {
	return filepath.Join(s.Dir, sanitize(character))
}

// Save writes a snapshot and returns its file path.
func (s Store) Save(snap Snapshot) (string, error) {
	if snap.Character == nil {
		return "", fmt.Errorf("snapshot has no character")
	}
	dir := s.charDir(snap.Character.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// UTC, filename-safe RFC3339 (colons replaced) for chronological sorting.
	stamp := snap.CapturedAt.UTC().Format("2006-01-02T15-04-05Z")
	path := filepath.Join(dir, stamp+".json")
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// list returns snapshot file paths for a character, oldest first.
func (s Store) list(character string) ([]string, error) {
	dir := s.charDir(character)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var paths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func readSnapshot(path string) (Snapshot, error) {
	var snap Snapshot
	data, err := os.ReadFile(path)
	if err != nil {
		return snap, err
	}
	err = json.Unmarshal(data, &snap)
	return snap, err
}

// Latest returns the most recent snapshot for a character, or (nil, nil) if none.
func (s Store) Latest(character string) (*Snapshot, error) {
	paths, err := s.list(character)
	if err != nil || len(paths) == 0 {
		return nil, err
	}
	snap, err := readSnapshot(paths[len(paths)-1])
	if err != nil {
		return nil, err
	}
	return &snap, nil
}

// History returns all snapshots for a character, oldest first.
func (s Store) History(character string) ([]Snapshot, error) {
	paths, err := s.list(character)
	if err != nil {
		return nil, err
	}
	out := make([]Snapshot, 0, len(paths))
	for _, p := range paths {
		snap, err := readSnapshot(p)
		if err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, nil
}

func sanitize(name string) string {
	repl := func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}
	return strings.Map(repl, name)
}
