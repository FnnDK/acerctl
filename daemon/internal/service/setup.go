package service

import (
	"log/slog"

	"github.com/godbus/dbus/v5"
)

// ApplySetup makes the system changes the app lists from System.Setup (ids
// in the "needed" state; signing_key also in "reboot", which replaces the
// pending request with a new code). It returns the one-time code the
// firmware asks for at the next boot when the signing key was enrolled.
// These change the boot configuration, so polkit always asks for a password.
func (s *systemIface) ApplySetup(sender dbus.Sender, ids []string) (string, *dbus.Error) {
	d := s.d
	if d.hw.Setup == nil {
		return "", errNotSupported("system setup")
	}
	items := map[string]string{}
	for _, it := range d.hw.Setup.Items() {
		items[it.ID] = it.State
	}
	for _, id := range ids {
		state, ok := items[id]
		if !ok || !(state == "needed" || (id == "signing_key" && state == "reboot")) {
			return "", errInvalid("%s: nothing to apply (state %q)", id, state)
		}
	}
	if err := d.authorize(sender, actionSetup); err != nil {
		return "", err
	}
	code := ""
	for _, id := range ids {
		c, err := d.hw.Setup.Apply(id)
		if err != nil {
			d.refreshSetup()
			return "", errHardware(err)
		}
		if c != "" {
			code = c
		}
		slog.Info("setup applied", "item", id)
	}
	d.refreshSetup()
	return code, nil
}

// RefreshSetup re-reads System.Setup without re-probing the hardware (Reload
// does that, NVML included, too much for every start of the app): the app
// and acerctl setup call it when Setup lists something to do, since the
// system may have changed behind acerd (DKMS builds the modules once the
// kernel headers are installed). It only reads, like a property, so no polkit.
func (s *systemIface) RefreshSetup() *dbus.Error {
	s.d.refreshSetup()
	return nil
}

// setupItems is the System.Setup property.
func (d *Daemon) setupItems() []map[string]dbus.Variant {
	out := []map[string]dbus.Variant{}
	if d.hw.Setup == nil {
		return out
	}
	for _, it := range d.hw.Setup.Items() {
		out = append(out, map[string]dbus.Variant{
			"id":     dbus.MakeVariant(it.ID),
			"state":  dbus.MakeVariant(it.State),
			"detail": dbus.MakeVariant(it.Detail),
		})
	}
	return out
}

func (d *Daemon) refreshSetup() { d.setProp(ifSystem, "Setup", d.setupItems()) }
