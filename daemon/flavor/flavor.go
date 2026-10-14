// Package flavor is the branding of the desktop app and the tray agent. The
// same code ships as nitro-control-center (red [N], Nitro),
// predator-control-center (blue-grey [P], Predator) or acer-control-center
// (green [a], the other lines); packaging/install-gui.sh picks one per
// product line. Only names, icons and colours change: capabilities always
// come from the firmware.
package flavor

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Flavor is one branding of the app.
type Flavor struct {
	ID     string // "nitro", "predator" or "acer"
	Name   string // window title, launcher and tray name
	AppID  string // desktop file and icon name, Wayland app_id, single-instance id
	Binary string // installed executable
}

var (
	Nitro    = Flavor{ID: "nitro", Name: "Nitro Control Center", AppID: "io.github.fnndk.NitroControlCenter", Binary: "nitro-control-center"}
	Predator = Flavor{ID: "predator", Name: "Predator Control Center", AppID: "io.github.fnndk.PredatorControlCenter", Binary: "predator-control-center"}
	Acer     = Flavor{ID: "acer", Name: "Acer Control Center", AppID: "io.github.fnndk.AcerControlCenter", Binary: "acer-control-center"}

	// All lists the flavors; the first one is the default.
	All = []Flavor{Nitro, Predator, Acer}
)

// Detect picks the flavor: ACERCTL_FLAVOR (nitro|predator|acer), else the
// name of the running executable when it is one of the apps (the GUI), else
// the one app that is installed (the tray; the installer keeps only one),
// else Nitro.
func Detect() Flavor {
	env := os.Getenv("ACERCTL_FLAVOR")
	for _, f := range All {
		if env == f.ID {
			return f
		}
	}
	if exe, err := os.Executable(); err == nil {
		for _, f := range All {
			if filepath.Base(exe) == f.Binary {
				return f
			}
		}
	}
	var found []Flavor
	for _, f := range All {
		if installed(f.Binary) {
			found = append(found, f)
		}
	}
	if len(found) == 1 {
		return found[0]
	}
	return Nitro
}

// Path returns the installed executable of the flavor.
func (f Flavor) Path() string {
	if p, err := exec.LookPath(f.Binary); err == nil {
		return p
	}
	return "/usr/local/bin/" + f.Binary
}

func installed(bin string) bool {
	if _, err := exec.LookPath(bin); err == nil {
		return true
	}
	_, err := os.Stat("/usr/local/bin/" + bin)
	return err == nil
}
