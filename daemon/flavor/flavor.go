// Package flavor is the branding of the desktop app and the tray agent. The
// same code ships as nitro-control-center (red [N], Nitro and Predator) or
// acer-control-center (green [a], the other lines); packaging/install-gui.sh
// picks one per product line. Only names, icons and colours change:
// capabilities always come from the firmware.
package flavor

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Flavor is one branding of the app.
type Flavor struct {
	ID     string // "nitro" or "acer"
	Name   string // window title, launcher and tray name
	AppID  string // desktop file and icon name, Wayland app_id, single-instance id
	Binary string // installed executable
}

var (
	Nitro = Flavor{ID: "nitro", Name: "Nitro Control Center", AppID: "io.github.fnndk.NitroControlCenter", Binary: "nitro-control-center"}
	Acer  = Flavor{ID: "acer", Name: "Acer Control Center", AppID: "io.github.fnndk.AcerControlCenter", Binary: "acer-control-center"}
)

// Detect picks the flavor: ACERCTL_FLAVOR (nitro|acer), else the name of the
// running executable when it is one of the apps (the GUI), else the app that
// is installed (the tray; the installer keeps only one), else Nitro.
func Detect() Flavor {
	switch os.Getenv("ACERCTL_FLAVOR") {
	case Acer.ID:
		return Acer
	case Nitro.ID:
		return Nitro
	}
	if exe, err := os.Executable(); err == nil {
		switch filepath.Base(exe) {
		case Acer.Binary:
			return Acer
		case Nitro.Binary:
			return Nitro
		}
	}
	if installed(Acer.Binary) && !installed(Nitro.Binary) {
		return Acer
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
