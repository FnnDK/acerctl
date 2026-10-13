package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// The window is frameless; the frontend draws its own title bar and takes
// from the desktop what a native GNOME title bar would use: the button
// layout (org.gnome.desktop.wm.preferences button-layout) and the
// window-*-symbolic icons of the current icon theme, read at runtime.

const (
	evTitlebar = "titlebar"

	portalDest  = "org.freedesktop.portal.Desktop"
	portalPath  = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	portalIface = "org.freedesktop.portal.Settings"
	wmNamespace = "org.gnome.desktop.wm.preferences"
	ifNamespace = "org.gnome.desktop.interface"
)

// Titlebar is the button layout (left and right of the title) and the SVG
// of each button icon found in the icon theme (missing ones are left out;
// the frontend has its own fallback).
type Titlebar struct {
	Left  []string          `json:"left"`
	Right []string          `json:"right"`
	Icons map[string]string `json:"icons"`
}

// Titlebar reads the layout and icons now.
func (a *App) Titlebar() Titlebar {
	left, right := parseLayout(a.setting(wmNamespace, "button-layout"))
	theme := a.setting(ifNamespace, "icon-theme")
	icons := map[string]string{}
	for _, name := range []string{"window-minimize", "window-maximize", "window-restore", "window-close"} {
		if svg := findIcon(theme, name+"-symbolic"); svg != "" {
			icons[name] = svg
		}
	}
	return Titlebar{Left: left, Right: right, Icons: icons}
}

// watchTitlebar re-sends the title bar when the desktop setting changes.
func (a *App) watchTitlebar() {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return
	}
	a.sessionBus = conn
	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(portalPath),
		dbus.WithMatchInterface(portalIface),
		dbus.WithMatchMember("SettingChanged"),
	); err != nil {
		return
	}
	sigs := make(chan *dbus.Signal, 8)
	conn.Signal(sigs)
	go func() {
		for s := range sigs {
			if len(s.Body) < 2 {
				continue
			}
			ns, _ := s.Body[0].(string)
			key, _ := s.Body[1].(string)
			if (ns == wmNamespace && key == "button-layout") || (ns == ifNamespace && key == "icon-theme") {
				runtime.EventsEmit(a.ctx, evTitlebar, a.Titlebar())
			}
		}
	}()
}

// setting reads a desktop setting through the settings portal (works on the
// host and inside a Flatpak), falling back to gsettings.
func (a *App) setting(ns, key string) string {
	if conn := a.sessionBus; conn != nil {
		obj := conn.Object(portalDest, portalPath)
		var v dbus.Variant
		if err := obj.Call(portalIface+".ReadOne", 0, ns, key).Store(&v); err == nil {
			if s, ok := v.Value().(string); ok {
				return s
			}
		}
		// Portal version 1 only has Read, which nests the value in one more variant.
		if err := obj.Call(portalIface+".Read", 0, ns, key).Store(&v); err == nil {
			if inner, ok := v.Value().(dbus.Variant); ok {
				v = inner
			}
			if s, ok := v.Value().(string); ok {
				return s
			}
		}
	}
	out, err := exec.Command("gsettings", "get", ns, key).Output()
	if err != nil {
		return ""
	}
	return strings.Trim(strings.TrimSpace(string(out)), "'")
}

// parseLayout turns "appmenu:minimize,maximize,close" into the window buttons
// on each side. Without a readable setting all three go on the right.
func parseLayout(s string) (left, right []string) {
	if s == "" {
		return nil, []string{"minimize", "maximize", "close"}
	}
	l, r, _ := strings.Cut(s, ":")
	pick := func(part string) []string {
		var out []string
		for _, b := range strings.Split(part, ",") {
			switch b = strings.TrimSpace(b); b {
			case "minimize", "maximize", "close":
				out = append(out, b)
			}
		}
		return out
	}
	return pick(l), pick(r)
}

// findIcon looks for a symbolic icon in the theme and then in Adwaita, in the
// XDG data directories (GNOME ≥ 46 keeps them in symbolic/ui, older Adwaita
// in scalable/ui or scalable/actions).
func findIcon(theme, name string) string {
	var bases []string
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		bases = append(bases, d)
	} else if h, err := os.UserHomeDir(); err == nil {
		bases = append(bases, filepath.Join(h, ".local/share"))
	}
	dirs := os.Getenv("XDG_DATA_DIRS")
	if dirs == "" {
		dirs = "/usr/local/share:/usr/share"
	}
	bases = append(bases, filepath.SplitList(dirs)...)
	themes := []string{"Adwaita"}
	if theme != "" && theme != "Adwaita" {
		themes = []string{theme, "Adwaita"}
	}
	for _, th := range themes {
		for _, base := range bases {
			for _, sub := range []string{"symbolic/ui", "scalable/ui", "scalable/actions"} {
				b, err := os.ReadFile(filepath.Join(base, "icons", th, sub, name+".svg"))
				if err == nil && len(b) < 64<<10 {
					return string(b)
				}
			}
		}
	}
	return ""
}

func (a *App) Minimise()       { runtime.WindowMinimise(a.ctx) }
func (a *App) ToggleMaximise() { runtime.WindowToggleMaximise(a.ctx) }
func (a *App) Close()          { runtime.Quit(a.ctx) }

// IsMaximised tells the frontend whether to show the restore icon and drop
// the rounded corners.
func (a *App) IsMaximised() bool { return runtime.WindowIsMaximised(a.ctx) }
