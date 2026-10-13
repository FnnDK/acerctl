// Package scenarios keeps the saved scenarios (docs/api-dbus.md §9): a
// thermal profile per power source, the fans, the keyboard timeout and the
// executables that trigger the scenario through App Sync.
package scenarios

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Fan is the setting of one fan inside a scenario.
type Fan struct {
	Mode  string `json:"mode"`
	Speed uint32 `json:"speed,omitempty"`
}

// Scenario is one saved scenario; every field but Name is optional.
type Scenario struct {
	Name        string         `json:"name"`
	ModeAC      string         `json:"mode_ac,omitempty"`
	ModeBattery string         `json:"mode_battery,omitempty"`
	Fans        map[string]Fan `json:"fans,omitempty"`
	KbdTimeout  *uint32        `json:"kbd_timeout_s,omitempty"`
	Apps        []string       `json:"apps,omitempty"`
}

// Limits is what the hardware accepts, used to validate a scenario.
type Limits struct {
	Profiles    []string
	FanIDs      []string
	FanRange    func(id string) (min, max uint32)
	KbdTimeouts []uint32
}

// Validate checks a scenario against the hardware and normalises Apps to
// lower-case basenames.
func (s *Scenario) Validate(l Limits) error {
	s.Name = strings.TrimSpace(s.Name)
	if s.Name == "" {
		return errors.New("name is required")
	}
	for _, m := range []string{s.ModeAC, s.ModeBattery} {
		if m != "" && !contains(l.Profiles, m) {
			return fmt.Errorf("profile %q not in %v", m, l.Profiles)
		}
	}
	for id, f := range s.Fans {
		if !contains(l.FanIDs, id) {
			return fmt.Errorf("unknown fan %q (ids: %v)", id, l.FanIDs)
		}
		switch f.Mode {
		case "auto", "max":
			f.Speed = 0
			s.Fans[id] = f
		case "manual":
			if min, max := l.FanRange(id); f.Speed < min || f.Speed > max {
				return fmt.Errorf("%d%% is outside the %s range %d-%d", f.Speed, id, min, max)
			}
		default:
			return fmt.Errorf("unknown fan mode %q (auto, max, manual)", f.Mode)
		}
	}
	if s.KbdTimeout != nil {
		ok := false
		for _, t := range l.KbdTimeouts {
			ok = ok || t == *s.KbdTimeout
		}
		if !ok {
			return fmt.Errorf("keyboard timeout %d not in %v", *s.KbdTimeout, l.KbdTimeouts)
		}
	}
	apps := []string{}
	for _, a := range s.Apps {
		if a = NormalizeApp(a); a != "" && !contains(apps, a) {
			apps = append(apps, a)
		}
	}
	s.Apps = apps
	return nil
}

// NormalizeApp turns "C:\Games\Foo.exe" or "/usr/bin/foo" into "foo.exe"/"foo".
func NormalizeApp(a string) string {
	a = strings.TrimSpace(a)
	if i := strings.LastIndexAny(a, `/\`); i >= 0 {
		a = a[i+1:]
	}
	return strings.ToLower(a)
}

// Store is the list of scenarios with a file behind it.
type Store struct {
	mu   sync.Mutex
	path string
	list []Scenario
	apps []appEntry // index for ForApp, rebuilt on every change
}

type appEntry struct{ app, scenario string }

// Load reads the file if it exists; a missing file is an empty list.
func Load(path string) *Store {
	s := &Store{path: path}
	b, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("scenarios: cannot read", "path", path, "err", err)
		}
		return s
	}
	if err := json.Unmarshal(b, &s.list); err != nil {
		slog.Warn("scenarios: ignoring corrupt file", "path", path, "err", err)
		s.list = nil
	}
	s.index()
	return s
}

// index lists every app with its scenario, scenarios in name order.
func (s *Store) index() {
	sorted := append([]Scenario(nil), s.list...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	s.apps = s.apps[:0]
	for _, sc := range sorted {
		for _, a := range sc.Apps {
			s.apps = append(s.apps, appEntry{a, sc.Name})
		}
	}
}

// List returns a copy sorted by name.
func (s *Store) List() []Scenario {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Scenario(nil), s.list...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Get finds a scenario by name.
func (s *Store) Get(name string) (Scenario, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sc := range s.list {
		if sc.Name == name {
			return sc, true
		}
	}
	return Scenario{}, false
}

// Put creates or replaces a scenario by name.
func (s *Store) Put(sc Scenario) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.list {
		if s.list[i].Name == sc.Name {
			s.list[i] = sc
			return s.write()
		}
	}
	s.list = append(s.list, sc)
	return s.write()
}

// Delete removes a scenario; false when there was none.
func (s *Store) Delete(name string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.list {
		if s.list[i].Name == name {
			s.list = append(s.list[:i], s.list[i+1:]...)
			return true, s.write()
		}
	}
	return false, nil
}

// ForApp returns the name of the first scenario (by name) listing one of
// the given process names, or "". It runs on every exec in the system.
func (s *Store) ForApp(names []string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.apps {
		for _, n := range names {
			if MatchApp(e.app, n) {
				return e.scenario
			}
		}
	}
	return ""
}

// MatchApp compares a normalised app entry with a process name: case-
// insensitive, ".exe" optional on either side, and a 15-character comm
// (TASK_COMM_LEN) matches by prefix.
func MatchApp(app, name string) bool {
	name = strings.ToLower(name)
	if name == "" || app == "" {
		return false
	}
	a, n := strings.TrimSuffix(app, ".exe"), strings.TrimSuffix(name, ".exe")
	if a == n {
		return true
	}
	return len(name) == 15 && strings.HasPrefix(app, name)
}

func (s *Store) write() error {
	s.index()
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s.list, "", "  ")
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
