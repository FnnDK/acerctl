package main

import (
	"os/exec"
	"syscall"

	"github.com/godbus/dbus/v5"

	"acerctl/daemon/client"
	"acerctl/daemon/usercfg"
)

// The session agent (acer-tray) shows the tray icon and opens this app on
// the Sense key. It starts at login (XDG autostart) and exits when neither
// is wanted; the GUI tells it about changes and starts it when needed.

const (
	agentName   = "io.github.fnndk.AcerTray"
	agentPath   = dbus.ObjectPath("/io/github/fnndk/AcerTray")
	agentBinary = "acer-tray"
)

// TrayShown tells whether the tray icon is on (user setting).
func (a *App) TrayShown() bool { return !usercfg.Load().TrayHidden }

// SetTrayShown turns the tray icon on or off.
func (a *App) SetTrayShown(on bool) error {
	s := usercfg.Load()
	s.TrayHidden = !on
	if err := usercfg.Save(s); err != nil {
		return uiError(err)
	}
	a.ensureAgent()
	return nil
}

// Lang is the language chosen in Settings ("" follows the system).
func (a *App) Lang() string { return usercfg.Load().Lang }

// SetLang saves the language for the GUI and the tray agent.
func (a *App) SetLang(lang string) error {
	if lang != "pt" && lang != "en" {
		lang = ""
	}
	s := usercfg.Load()
	s.Lang = lang
	if err := usercfg.Save(s); err != nil {
		return uiError(err)
	}
	a.ensureAgent()
	return nil
}

// SetSenseKey chooses what the Sense key does: "app" or "cycle" (daemon).
func (a *App) SetSenseKey(action string) error {
	if err := a.call(client.IfSystem, "SetSenseKey", action); err != nil {
		return err
	}
	a.ensureAgent()
	return nil
}

// ensureAgent reloads a running agent or starts one when it has a job.
func (a *App) ensureAgent() {
	conn := a.sessionBus // opened for the title bar; not on desktops with a system frame
	if conn == nil {
		var err error
		if conn, err = dbus.SessionBus(); err != nil {
			return
		}
	}
	var running bool
	_ = conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, agentName).Store(&running)
	if running {
		conn.Object(agentName, agentPath).Go(agentName+".Reload", 0, nil)
		return
	}
	need := !usercfg.Load().TrayHidden
	if cl, err := a.client(); err == nil {
		if v, err := cl.Property(client.IfSystem, "SenseKey"); err == nil && v.Value() == "app" {
			need = true
		}
	}
	if !need {
		return
	}
	bin, err := exec.LookPath(agentBinary)
	if err != nil {
		bin = "/usr/local/bin/" + agentBinary
	}
	cmd := exec.Command(bin)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err == nil {
		go func() { _ = cmd.Wait() }()
	}
}
