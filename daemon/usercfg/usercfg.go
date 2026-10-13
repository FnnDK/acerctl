// Package usercfg is the per-user settings file shared by the GUI and the
// session agent (acer-tray): ~/.config/acerctl/settings.json. System-wide
// choices (the Nitro key action) live in the daemon instead.
package usercfg

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings are the user's preferences; a missing file means the defaults.
type Settings struct {
	// TrayHidden hides the tray icon (shown by default).
	TrayHidden bool `json:"tray_hidden,omitempty"`
}

// Path is the settings file of the current user.
func Path() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "acerctl", "settings.json")
}

// Load reads the settings; errors give the defaults.
func Load() Settings {
	var s Settings
	if b, err := os.ReadFile(Path()); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

// Save writes the settings atomically.
func Save(s Settings) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
