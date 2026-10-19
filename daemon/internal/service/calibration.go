package service

import (
	"log/slog"
	"os"

	"github.com/godbus/dbus/v5"
)

// While the battery calibrates the laptop has to stay awake on AC: the
// firmware drains the battery through the running system, so a suspend
// stalls the discharge, and a shutdown in the middle left the EC answering
// acer-wmi-battery with no modes at the next boot (STATUS). acerd holds a
// logind block inhibitor on sleep and the lid switch until the calibration
// ends; taken by root, it also refuses the desktop's suspend when idle.
const calibrationInhibit = "sleep:handle-lid-switch"

// updateCalibrating rereads Calibrating, publishes it and takes or releases
// the inhibitor to match.
func (d *Daemon) updateCalibrating() {
	on := d.calibrating()
	d.setProp(ifBattery, "Calibrating", on)

	d.inhibitMu.Lock()
	defer d.inhibitMu.Unlock()
	switch {
	case on && d.calibInhibit == nil:
		f, err := inhibitSleep("Battery calibration: the laptop has to stay awake on AC until it ends")
		if err != nil {
			if !d.inhibitFailed {
				slog.Warn("calibration: cannot keep the laptop awake", "err", err)
				d.inhibitFailed = true
			}
			return
		}
		d.calibInhibit = f
		slog.Info("calibration: sleep and lid switch inhibited")
	case !on && d.calibInhibit != nil:
		d.calibInhibit.Close()
		d.calibInhibit = nil
		d.inhibitFailed = false
		slog.Info("calibration over: sleep allowed again")
	}
}

// inhibitSleep takes a logind block inhibitor; closing the file releases it.
// logind is on the system bus also when acerd serves the session bus.
func inhibitSleep(why string) (*os.File, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	var fd dbus.UnixFD
	err = conn.Object("org.freedesktop.login1", "/org/freedesktop/login1").
		Call("org.freedesktop.login1.Manager.Inhibit", 0, calibrationInhibit, "acerd", why, "block").Store(&fd)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), "logind-inhibitor"), nil
}
