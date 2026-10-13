package service

import (
	"github.com/godbus/dbus/v5"

	"nitrosense/daemon/internal/rules"
	"nitrosense/daemon/internal/state"
)

// ---- org.nitrosense.Daemon1.System -----------------------------------------

type systemIface struct{ d *Daemon }

// Reload re-probes the kernel interfaces (after insmod or an update).
func (s *systemIface) Reload(sender dbus.Sender) *dbus.Error {
	d := s.d
	if err := d.authorize(sender, actionControl); err != nil {
		return err
	}
	if d.cfg.Reprobe == nil {
		return nil
	}
	d.hw = d.cfg.Reprobe()
	d.setProp(ifSystem, "Capabilities", variants(d.hw.Capabilities()))
	d.setProp(ifSystem, "Model", variants(d.hw.Model))
	d.readInitial()
	d.setProp(ifMode, "Current", d.current)
	return nil
}

// ---- org.nitrosense.Daemon1.Mode -------------------------------------------

type modeIface struct{ d *Daemon }

func (m *modeIface) Set(sender dbus.Sender, profile string) *dbus.Error {
	d := m.d
	if d.hw.Profiles == nil {
		return errNotSupported("thermal profiles")
	}
	if err := d.authorize(sender, actionControl); err != nil {
		return err
	}
	if !rules.Contains(d.hw.Profiles.Supported(), profile) {
		return errNotSupported("profile " + profile)
	}
	d.mu.Lock()
	allowed, power := d.allowed, d.power
	d.mu.Unlock()
	if !rules.Contains(allowed, profile) {
		return errNotAllowed("%s is not allowed on %s at %d%% (allowed: %v)", profile, power.Source, power.Percent, allowed)
	}
	if err := d.applyMode(profile, true); err != nil {
		return errHardware(err)
	}
	return nil
}

func (m *modeIface) Cycle(sender dbus.Sender) (string, *dbus.Error) {
	d := m.d
	if d.hw.Profiles == nil {
		return "", errNotSupported("thermal profiles")
	}
	if err := d.authorize(sender, actionControl); err != nil {
		return "", err
	}
	next, err := d.cycle()
	if err != nil {
		return "", errHardware(err)
	}
	return next, nil
}

// ---- org.nitrosense.Daemon1.Fans -------------------------------------------

type fansIface struct{ d *Daemon }

func (f *fansIface) check(sender dbus.Sender, fan string) *dbus.Error {
	d := f.d
	if d.hw.Fans == nil || !d.hw.Fans.Controllable() {
		return errNotSupported("fan control")
	}
	if err := d.authorize(sender, actionControl); err != nil {
		return err
	}
	if fan != "all" && !rules.Contains(d.hw.Fans.IDs(), fan) {
		return errInvalid("unknown fan %q (ids: %v, or all)", fan, d.hw.Fans.IDs())
	}
	return nil
}

func (f *fansIface) remember(fan string, fn func(*state.Fan)) {
	f.d.leaveScenario()
	ids := []string{fan}
	if fan == "all" {
		ids = f.d.hw.Fans.IDs()
	}
	f.d.st.Update(func(s *state.Data) {
		if s.Fans == nil {
			s.Fans = map[string]state.Fan{}
		}
		for _, id := range ids {
			st := s.Fans[id]
			fn(&st)
			s.Fans[id] = st
		}
	})
}

func (f *fansIface) SetMode(sender dbus.Sender, fan, mode string) *dbus.Error {
	if err := f.check(sender, fan); err != nil {
		return err
	}
	switch mode {
	case "auto", "max", "manual":
	default:
		return errInvalid("unknown fan mode %q (auto, max, manual)", mode)
	}
	if err := f.d.hw.Fans.SetMode(fan, mode); err != nil {
		return errHardware(err)
	}
	f.remember(fan, func(s *state.Fan) {
		s.Mode = mode
		if mode != "manual" {
			s.Speed = 0
		}
	})
	f.d.setProp(ifFans, "State", f.d.fanState())
	return nil
}

func (f *fansIface) SetSpeed(sender dbus.Sender, fan string, percent uint32) *dbus.Error {
	if err := f.check(sender, fan); err != nil {
		return err
	}
	ids := []string{fan}
	if fan == "all" {
		ids = f.d.hw.Fans.IDs()
	}
	for _, id := range ids {
		r := f.d.hw.Fans.Range(id)
		if percent < r.Min || percent > r.Max {
			return errInvalid("%d%% is outside the %s range %d-%d", percent, id, r.Min, r.Max)
		}
	}
	if err := f.d.hw.Fans.SetSpeed(fan, percent); err != nil {
		return errHardware(err)
	}
	f.remember(fan, func(s *state.Fan) { s.Mode = "manual"; s.Speed = percent })
	f.d.setProp(ifFans, "State", f.d.fanState())
	return nil
}

// ---- org.nitrosense.Daemon1.Lighting ---------------------------------------

type lightingIface struct{ d *Daemon }

func (l *lightingIface) SetKeyboardTimeout(sender dbus.Sender, seconds uint32) *dbus.Error {
	d := l.d
	if d.hw.Keyboard == nil {
		return errNotSupported("keyboard backlight timeout")
	}
	if err := d.authorize(sender, actionControl); err != nil {
		return err
	}
	ok := false
	for _, t := range d.hw.Keyboard.Timeouts() {
		ok = ok || t == seconds
	}
	if !ok {
		return errInvalid("timeout %d not in %v", seconds, d.hw.Keyboard.Timeouts())
	}
	if err := d.hw.Keyboard.SetTimeout(seconds); err != nil {
		return errHardware(err)
	}
	d.leaveScenario()
	d.setProp(ifLighting, "Keyboard", d.keyboardState())
	return nil
}

// ---- org.nitrosense.Daemon1.Battery ----------------------------------------

type batteryIface struct{ d *Daemon }

func (b *batteryIface) SetChargeLimit(sender dbus.Sender, on bool) *dbus.Error {
	d := b.d
	if d.hw.BatteryCtl == nil || !d.hw.BatteryCtl.HasChargeLimit() {
		return errNotSupported("charge limit")
	}
	if err := d.authorize(sender, actionSettings); err != nil {
		return err
	}
	if err := d.hw.BatteryCtl.SetChargeLimit(on); err != nil {
		return errHardware(err)
	}
	d.setProp(ifBattery, "ChargeLimit", d.chargeLimit())
	return nil
}

func (b *batteryIface) StartCalibration(sender dbus.Sender) *dbus.Error {
	return b.setCalibration(sender, true)
}

func (b *batteryIface) StopCalibration(sender dbus.Sender) *dbus.Error {
	return b.setCalibration(sender, false)
}

func (b *batteryIface) setCalibration(sender dbus.Sender, on bool) *dbus.Error {
	d := b.d
	if d.hw.BatteryCtl == nil || !d.hw.BatteryCtl.HasCalibration() {
		return errNotSupported("battery calibration")
	}
	if err := d.authorize(sender, actionSettings); err != nil {
		return err
	}
	if on && d.readPower().Source != "ac" {
		return errNotAllowed("calibration needs the AC adapter connected")
	}
	if err := d.hw.BatteryCtl.SetCalibrating(on); err != nil {
		return errHardware(err)
	}
	d.setProp(ifBattery, "Calibrating", d.calibrating())
	return nil
}

func (b *batteryIface) SetUsbCharging(sender dbus.Sender, percent uint32) *dbus.Error {
	d := b.d
	if d.hw.USB == nil {
		return errNotSupported("USB charging")
	}
	if err := d.authorize(sender, actionSettings); err != nil {
		return err
	}
	ok := false
	for _, v := range d.hw.USB.Values() {
		ok = ok || v == percent
	}
	if !ok {
		return errInvalid("%d not in %v", percent, d.hw.USB.Values())
	}
	if err := d.hw.USB.Set(percent); err != nil {
		return errHardware(err)
	}
	d.setProp(ifBattery, "UsbCharging", d.usbCharging())
	return nil
}
