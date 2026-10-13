package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/godbus/dbus/v5"

	"acerctl/daemon/internal/appsync"
	"acerctl/daemon/internal/rules"
	"acerctl/daemon/internal/scenarios"
	"acerctl/daemon/internal/state"
)

// ---- io.github.fnndk.Acerd1.Scenarios --------------------------------------

type scenariosIface struct{ d *Daemon }

func (s *scenariosIface) Apply(sender dbus.Sender, name string) *dbus.Error {
	d := s.d
	if err := d.authorize(sender, actionControl); err != nil {
		return err
	}
	sc, ok := d.sc.Get(name)
	if !ok {
		return errInvalid("no scenario named %q", name)
	}
	if err := sc.Validate(d.limits()); err != nil { // the hardware may have changed
		return errInvalid("%v", err)
	}
	d.syncMu.Lock()
	defer d.syncMu.Unlock()
	if err := d.applyScenario(sc, sc.Name); err != nil {
		return errHardware(err)
	}
	slog.Info("scenario applied", "name", sc.Name)
	return nil
}

func (s *scenariosIface) Save(sender dbus.Sender, v map[string]dbus.Variant) *dbus.Error {
	d := s.d
	if err := d.authorize(sender, actionControl); err != nil {
		return err
	}
	sc, err := scenarioFromVariant(v)
	if err != nil {
		return errInvalid("%v", err)
	}
	if err := sc.Validate(d.limits()); err != nil {
		return errInvalid("%v", err)
	}
	if err := d.sc.Put(sc); err != nil {
		return errHardware(err)
	}
	d.scenariosChanged()
	return nil
}

func (s *scenariosIface) Delete(sender dbus.Sender, name string) *dbus.Error {
	d := s.d
	if err := d.authorize(sender, actionControl); err != nil {
		return err
	}
	found, err := d.sc.Delete(name)
	if err != nil {
		return errHardware(err)
	}
	if !found {
		return errInvalid("no scenario named %q", name)
	}
	var active string
	d.st.View(func(s state.Data) { active = s.Scenario })
	if active == name {
		d.setActive("")
	}
	d.scenariosChanged()
	return nil
}

func (s *scenariosIface) SetAppSync(sender dbus.Sender, enabled bool) *dbus.Error {
	d := s.d
	if err := d.authorize(sender, actionControl); err != nil {
		return err
	}
	if err := d.setAppSync(enabled); err != nil {
		return errHardware(err)
	}
	d.st.Update(func(s *state.Data) { s.AppSync = enabled })
	return nil
}

// ---- applying --------------------------------------------------------------

func (d *Daemon) limits() scenarios.Limits {
	l := scenarios.Limits{FanRange: func(string) (uint32, uint32) { return 0, 0 }}
	if d.hw.Profiles != nil {
		l.Profiles = d.hw.Profiles.Supported()
	}
	if d.hw.Fans != nil && d.hw.Fans.Controllable() {
		l.FanIDs = d.hw.Fans.IDs()
		l.FanRange = func(id string) (uint32, uint32) { r := d.hw.Fans.Range(id); return r.Min, r.Max }
	}
	if d.hw.Keyboard != nil {
		l.KbdTimeouts = d.hw.Keyboard.Timeouts()
	}
	return l
}

// applyScenario writes the scenario into the remembered state (so source
// changes, resume and boot follow it), applies it and marks active as the
// active scenario. The caller holds syncMu.
func (d *Daemon) applyScenario(sc scenarios.Scenario, active string) error {
	d.st.Update(func(s *state.Data) {
		if sc.ModeAC != "" {
			s.ModeAC = sc.ModeAC
		}
		if sc.ModeBattery != "" {
			s.ModeBattery = sc.ModeBattery
		}
		for id, f := range sc.Fans {
			if s.Fans == nil {
				s.Fans = map[string]state.Fan{}
			}
			s.Fans[id] = state.Fan{Mode: f.Mode, Speed: f.Speed}
		}
	})
	var errs []error
	if d.hw.Profiles != nil && (sc.ModeAC != "" || sc.ModeBattery != "") {
		d.mu.Lock()
		allowed, cur, source := d.allowed, d.current, d.power.Source
		d.mu.Unlock()
		if target := rules.Fallback(allowed, d.st.ModeFor(source)); target != "" && target != cur {
			if err := d.applyMode(target, false); err != nil {
				errs = append(errs, fmt.Errorf("profile %s: %w", target, err))
			}
		}
	}
	if len(sc.Fans) > 0 && d.hw.Fans != nil && d.hw.Fans.Controllable() {
		for id, f := range sc.Fans {
			var err error
			if f.Mode == "manual" {
				err = d.hw.Fans.SetSpeed(id, f.Speed)
			} else {
				err = d.hw.Fans.SetMode(id, f.Mode)
			}
			if err != nil {
				errs = append(errs, fmt.Errorf("fan %s: %w", id, err))
			}
		}
		d.refreshFans(true)
	}
	if sc.KbdTimeout != nil && d.hw.Keyboard != nil {
		if cur, err := d.hw.Keyboard.Timeout(); err != nil || cur != *sc.KbdTimeout {
			if err := d.hw.Keyboard.SetTimeout(*sc.KbdTimeout); err != nil {
				errs = append(errs, fmt.Errorf("keyboard timeout: %w", err))
			}
		}
		d.setProp(ifLighting, "Keyboard", d.keyboardState())
	}
	d.setActive(active)
	return errors.Join(errs...)
}

func (d *Daemon) setActive(name string) {
	d.st.Update(func(s *state.Data) { s.Scenario = name })
	d.setProp(ifScenarios, "Active", name)
}

// leaveScenario is called on every manual change of a setting a scenario
// holds: the scenario is no longer what the machine runs.
func (d *Daemon) leaveScenario() {
	var active string
	d.st.View(func(s state.Data) { active = s.Scenario })
	if active != "" {
		slog.Info("scenario left by a manual change", "name", active)
		d.setActive("")
	}
}

// snapshot is the current setting as an unnamed scenario, named after the
// active scenario so that going back restores Active too.
func (d *Daemon) snapshot() scenarios.Scenario {
	var data state.Data
	d.st.View(func(s state.Data) { data = s })
	d.mu.Lock()
	cur, source := d.current, d.power.Source
	d.mu.Unlock()
	sc := scenarios.Scenario{Name: data.Scenario, ModeAC: data.ModeAC, ModeBattery: data.ModeBattery}
	if source == "battery" && sc.ModeBattery == "" {
		sc.ModeBattery = cur
	} else if source != "battery" && sc.ModeAC == "" {
		sc.ModeAC = cur
	}
	if d.hw.Fans != nil && d.hw.Fans.Controllable() {
		if st, err := d.hw.Fans.State(); err == nil {
			sc.Fans = map[string]scenarios.Fan{}
			for id, f := range st {
				sc.Fans[id] = scenarios.Fan{Mode: f.Mode, Speed: f.Speed}
			}
		}
	}
	if d.hw.Keyboard != nil {
		if t, err := d.hw.Keyboard.Timeout(); err == nil {
			sc.KbdTimeout = &t
		}
	}
	return sc
}

// ---- App Sync --------------------------------------------------------------

// setAppSync starts or stops the process watcher.
func (d *Daemon) setAppSync(on bool) error {
	d.syncMu.Lock()
	if on == (d.syncStop != nil) {
		d.syncMu.Unlock()
		return nil
	}
	if on {
		ctx, cancel := context.WithCancel(d.ctx)
		d.syncWatch = appsync.New(d.sc.ForApp, d.onAppSync)
		if err := d.syncWatch.Start(ctx); err != nil {
			cancel()
			d.syncWatch = nil
			d.syncMu.Unlock()
			return err
		}
		d.syncStop = cancel
		d.syncMu.Unlock()
		slog.Info("app sync on")
		d.setProp(ifScenarios, "AppSync", true)
		return nil
	}
	d.syncStop()
	d.syncStop, d.syncWatch = nil, nil
	d.revertAppSync()
	d.syncMu.Unlock()
	slog.Info("app sync off")
	d.setProp(ifScenarios, "AppSync", false)
	return nil
}

// onAppSync receives the scenario the running apps call for ("" when none).
func (d *Daemon) onAppSync(name string) {
	d.syncMu.Lock()
	defer d.syncMu.Unlock()
	if d.syncStop == nil { // turned off while this was queued
		return
	}
	if name == "" {
		d.revertAppSync()
		return
	}
	sc, ok := d.sc.Get(name)
	if !ok {
		return
	}
	if err := sc.Validate(d.limits()); err != nil {
		slog.Warn("app sync: scenario no longer valid", "name", name, "err", err)
		return
	}
	var revert *scenarios.Scenario
	d.st.View(func(s state.Data) { revert = s.SyncRevert })
	if revert == nil {
		snap := d.snapshot()
		d.st.Update(func(s *state.Data) { s.SyncRevert = &snap })
	}
	if err := d.applyScenario(sc, sc.Name); err != nil {
		slog.Error("app sync: apply failed", "name", name, "err", err)
	}
	d.st.Update(func(s *state.Data) { s.SyncApplied = sc.Name })
	slog.Info("app sync: scenario applied", "name", sc.Name)
}

// revertAppSync goes back to what ran before App Sync stepped in, unless the
// user changed something meanwhile. The caller holds syncMu.
func (d *Daemon) revertAppSync() {
	var data state.Data
	d.st.View(func(s state.Data) { data = s })
	if data.SyncRevert == nil {
		return
	}
	if data.Scenario == data.SyncApplied {
		if err := d.applyScenario(*data.SyncRevert, data.SyncRevert.Name); err != nil {
			slog.Error("app sync: revert failed", "err", err)
		}
		slog.Info("app sync: reverted", "to", data.SyncRevert.Name)
	} else {
		slog.Info("app sync: settings changed by hand meanwhile; not reverting")
	}
	d.st.Update(func(s *state.Data) { s.SyncRevert, s.SyncApplied = nil, "" })
}

func (d *Daemon) scenariosChanged() {
	d.setProp(ifScenarios, "List", d.scenarioList())
	d.syncMu.Lock()
	if d.syncWatch != nil {
		d.syncWatch.Rescan()
	}
	d.syncMu.Unlock()
}

// ---- D-Bus conversion ------------------------------------------------------

func (d *Daemon) scenarioList() []map[string]dbus.Variant {
	out := []map[string]dbus.Variant{}
	for _, sc := range d.sc.List() {
		out = append(out, scenarioToVariant(sc))
	}
	return out
}

func scenarioToVariant(sc scenarios.Scenario) map[string]dbus.Variant {
	m := map[string]dbus.Variant{"name": dbus.MakeVariant(sc.Name)}
	if sc.ModeAC != "" {
		m["mode_ac"] = dbus.MakeVariant(sc.ModeAC)
	}
	if sc.ModeBattery != "" {
		m["mode_battery"] = dbus.MakeVariant(sc.ModeBattery)
	}
	if len(sc.Fans) > 0 {
		fans := map[string]map[string]dbus.Variant{}
		for id, f := range sc.Fans {
			fm := map[string]dbus.Variant{"mode": dbus.MakeVariant(f.Mode)}
			if f.Mode == "manual" {
				fm["speed"] = dbus.MakeVariant(f.Speed)
			}
			fans[id] = fm
		}
		m["fans"] = dbus.MakeVariant(fans)
	}
	if sc.KbdTimeout != nil {
		m["kbd_timeout_s"] = dbus.MakeVariant(*sc.KbdTimeout)
	}
	m["apps"] = dbus.MakeVariant(append([]string{}, sc.Apps...))
	return m
}

func scenarioFromVariant(m map[string]dbus.Variant) (scenarios.Scenario, error) {
	var sc scenarios.Scenario
	for k, v := range m {
		var ok bool
		switch k {
		case "name":
			sc.Name, ok = v.Value().(string)
		case "mode_ac":
			sc.ModeAC, ok = v.Value().(string)
		case "mode_battery":
			sc.ModeBattery, ok = v.Value().(string)
		case "kbd_timeout_s":
			var t uint32
			if t, ok = toUint32(v.Value()); ok {
				sc.KbdTimeout = &t
			}
		case "apps":
			sc.Apps, ok = v.Value().([]string)
		case "fans":
			sc.Fans, ok = fansFromVariant(v.Value())
		default:
			return sc, fmt.Errorf("unknown key %q", k)
		}
		if !ok {
			return sc, fmt.Errorf("key %q has the wrong type %s", k, v.Signature())
		}
	}
	return sc, nil
}

// fansFromVariant accepts a{sa{sv}} and also a{sv} holding a{sv} values.
func fansFromVariant(v interface{}) (map[string]scenarios.Fan, bool) {
	raw := map[string]map[string]dbus.Variant{}
	switch t := v.(type) {
	case map[string]map[string]dbus.Variant:
		raw = t
	case map[string]dbus.Variant:
		for id, inner := range t {
			m, ok := inner.Value().(map[string]dbus.Variant)
			if !ok {
				return nil, false
			}
			raw[id] = m
		}
	default:
		return nil, false
	}
	out := map[string]scenarios.Fan{}
	for id, m := range raw {
		var f scenarios.Fan
		for k, v := range m {
			var ok bool
			switch k {
			case "mode":
				f.Mode, ok = v.Value().(string)
			case "speed":
				f.Speed, ok = toUint32(v.Value())
			}
			if !ok {
				return nil, false
			}
		}
		out[id] = f
	}
	return out, true
}

func toUint32(v interface{}) (uint32, bool) {
	switch n := v.(type) {
	case uint32:
		return n, true
	case int32:
		return uint32(n), n >= 0
	case uint16:
		return uint32(n), true
	case int16:
		return uint32(n), n >= 0
	case byte:
		return uint32(n), true
	case uint64:
		return uint32(n), n <= 1<<32-1
	case int64:
		return uint32(n), n >= 0 && n <= 1<<32-1
	}
	return 0, false
}

// startAppSync resumes App Sync at start-up when it was on; when it was off
// but a revert is pending (the daemon died mid-game), it reverts now.
func (d *Daemon) startAppSync() {
	var on, pending bool
	d.st.View(func(s state.Data) { on, pending = s.AppSync, s.SyncRevert != nil })
	if on {
		if err := d.setAppSync(true); err != nil {
			slog.Warn("app sync: cannot start", "err", err)
		}
		return
	}
	if pending {
		d.syncMu.Lock()
		d.revertAppSync()
		d.syncMu.Unlock()
	}
}
