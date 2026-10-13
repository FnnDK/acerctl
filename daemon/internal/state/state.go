// Package state persists what does not live in the EC: the last thermal
// profile per power source and the fan setting (docs/api-dbus.md §11).
package state

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"nitroctl/daemon/internal/scenarios"
)

// DefaultPath is where the system daemon keeps its state.
const DefaultPath = "/var/lib/nitrod/state.json"

// Fan is the remembered setting of one fan.
type Fan struct {
	Mode  string `json:"mode"`
	Speed uint32 `json:"speed,omitempty"`
}

// Data is the on-disk document.
type Data struct {
	ModeAC      string         `json:"mode_ac,omitempty"`
	ModeBattery string         `json:"mode_battery,omitempty"`
	Fans        map[string]Fan `json:"fans,omitempty"`
	// Scenario is the active scenario ("" once something is changed by hand).
	Scenario string `json:"scenario,omitempty"`
	AppSync  bool   `json:"app_sync,omitempty"`
	// NitroKey is the chosen action of the Nitro key ("app" or "cycle");
	// empty means the --nitro-key default.
	NitroKey string `json:"nitro_key,omitempty"`
	// While App Sync holds a scenario: what to go back to and which
	// scenario it applied (if Scenario differs, the user took over).
	SyncRevert  *scenarios.Scenario `json:"app_sync_revert,omitempty"`
	SyncApplied string              `json:"app_sync_applied,omitempty"`
}

// Store is a Data with a file behind it.
type Store struct {
	mu   sync.Mutex
	path string
	Data Data
}

// Load reads the file if it exists; a missing file is an empty state.
func Load(path string) *Store {
	s := &Store{path: path}
	b, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("state: cannot read", "path", path, "err", err)
		}
		return s
	}
	if err := json.Unmarshal(b, &s.Data); err != nil {
		slog.Warn("state: ignoring corrupt file", "path", path, "err", err)
		s.Data = Data{}
	}
	return s
}

// Update applies fn under the lock and writes the file.
func (s *Store) Update(fn func(d *Data)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.Data)
	if s.path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		slog.Warn("state: mkdir", "err", err)
		return
	}
	b, _ := json.MarshalIndent(s.Data, "", "  ")
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		slog.Warn("state: write", "err", err)
		return
	}
	if err := os.Rename(tmp, s.path); err != nil {
		slog.Warn("state: rename", "err", err)
	}
}

// ModeFor returns the remembered profile for a power source.
func (s *Store) ModeFor(source string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if source == "battery" {
		return s.Data.ModeBattery
	}
	return s.Data.ModeAC
}

// FansSnapshot returns a copy of the remembered fan settings.
func (s *Store) FansSnapshot() map[string]Fan {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]Fan, len(s.Data.Fans))
	for id, f := range s.Data.Fans {
		out[id] = f
	}
	return out
}

// View runs fn on a copy of the data under the lock.
func (s *Store) View(fn func(d Data)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s.Data)
}
