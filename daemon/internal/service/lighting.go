package service

import (
	"errors"
	"log/slog"
	"reflect"

	"github.com/godbus/dbus/v5"

	"acerctl/daemon/internal/hw"
	"acerctl/daemon/internal/state"
)

// SetLight changes the effect of one lighting device. Only the given fields
// change (effect, speed, brightness, direction, colors); the rest is kept
// from the current state and fitted to the effect (hw.LightDevice.Normalize).
func (l *lightingIface) SetLight(sender dbus.Sender, id string, fields map[string]dbus.Variant) *dbus.Error {
	d := l.d
	if d.hw.Lights == nil {
		return errNotSupported("RGB lighting")
	}
	dev, ok := d.lightDevice(id)
	if !ok {
		return errInvalid("unknown lighting device %q (%v)", id, d.lightIDs())
	}
	if err := d.authorize(sender, actionControl); err != nil {
		return err
	}
	st, known := d.lightState(dev)
	if !known {
		st = dev.Default()
	}
	for k, v := range fields {
		var good bool
		switch k {
		case "effect":
			st.Effect, good = v.Value().(string)
		case "direction":
			st.Direction, good = v.Value().(string)
		case "speed":
			st.Speed, good = toUint32(v.Value())
		case "brightness":
			st.Brightness, good = toUint32(v.Value())
		case "colors":
			st.Colors, good = v.Value().([]string)
		default:
			return errInvalid("unknown lighting field %q", k)
		}
		if !good {
			return errInvalid("wrong type for %s: %s", k, v.Signature())
		}
	}
	st, err := dev.Normalize(st)
	if err != nil {
		return errInvalid("%s: %v", id, err)
	}
	if err := d.hw.Lights.Set(id, st); err != nil {
		return errHardware(err)
	}
	d.st.Update(func(s *state.Data) {
		if s.Lights == nil {
			s.Lights = map[string]hw.LightState{}
		}
		s.Lights[id] = st
	})
	d.refreshLights()
	return nil
}

func (d *Daemon) lightDevice(id string) (hw.LightDevice, bool) {
	for _, dev := range d.hw.Lights.Devices() {
		if dev.ID == id {
			return dev, true
		}
	}
	return hw.LightDevice{}, false
}

func (d *Daemon) lightIDs() []string {
	var ids []string
	for _, dev := range d.hw.Lights.Devices() {
		ids = append(ids, dev.ID)
	}
	return ids
}

// lightState is what a device shows: its readback, or what the daemon last
// applied when it cannot be read. known is false when there is neither.
func (d *Daemon) lightState(dev hw.LightDevice) (hw.LightState, bool) {
	if dev.Readback {
		st, err := d.hw.Lights.Get(dev.ID)
		if err == nil {
			return st, true
		}
		slog.Warn("lighting: read failed", "device", dev.ID, "err", err)
	}
	st, ok := d.st.LightsSnapshot()[dev.ID]
	return st, ok
}

// lightStates is the Lighting.Devices property: id -> state, for the
// devices whose state is known.
func (d *Daemon) lightStates() map[string]map[string]dbus.Variant {
	out := map[string]map[string]dbus.Variant{}
	if d.hw.Lights == nil {
		return out
	}
	for _, dev := range d.hw.Lights.Devices() {
		st, ok := d.lightState(dev)
		if !ok {
			continue
		}
		m := map[string]dbus.Variant{
			"effect":     dbus.MakeVariant(st.Effect),
			"brightness": dbus.MakeVariant(st.Brightness),
			"readback":   dbus.MakeVariant(dev.Readback),
		}
		if st.Speed > 0 {
			m["speed"] = dbus.MakeVariant(st.Speed)
		}
		if st.Direction != "" {
			m["direction"] = dbus.MakeVariant(st.Direction)
		}
		if len(st.Colors) > 0 {
			m["colors"] = dbus.MakeVariant(st.Colors)
		}
		out[dev.ID] = m
	}
	return out
}

func (d *Daemon) refreshLights() { d.setProp(ifLighting, "Devices", d.lightStates()) }

// restoreLights reapplies the remembered effects: the PH16-71 firmware loses
// its lighting at boot and on resume, and a USB keyboard cannot tell. A
// readable device that already shows the remembered state is left alone.
func (d *Daemon) restoreLights() {
	if d.hw.Lights == nil {
		return
	}
	saved := d.st.LightsSnapshot()
	for _, dev := range d.hw.Lights.Devices() {
		want, ok := saved[dev.ID]
		if !ok {
			continue
		}
		want, err := dev.Normalize(want)
		if err != nil {
			slog.Warn("lighting: ignoring a remembered state the device does not take", "device", dev.ID, "err", err)
			continue
		}
		if dev.Readback {
			if have, err := d.hw.Lights.Get(dev.ID); err == nil && reflect.DeepEqual(have, want) {
				continue
			} else if err != nil && !errors.Is(err, hw.ErrNoReadback) {
				slog.Warn("lighting: read failed", "device", dev.ID, "err", err)
			}
		}
		if err := d.hw.Lights.Set(dev.ID, want); err != nil {
			slog.Error("lighting: restore failed", "device", dev.ID, "err", err)
			continue
		}
		slog.Info("lighting restored", "device", dev.ID, "effect", want.Effect)
	}
	d.refreshLights()
}
