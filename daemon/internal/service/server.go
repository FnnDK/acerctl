// Package service exports the io.github.fnndk.Nitrod1 object on D-Bus and
// implements the daemon behaviour described in docs/api-dbus.md.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"reflect"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"

	"nitroctl/daemon/internal/appsync"
	"nitroctl/daemon/internal/hw"
	"nitroctl/daemon/internal/monitor"
	"nitroctl/daemon/internal/ppd"
	"nitroctl/daemon/internal/rules"
	"nitroctl/daemon/internal/scenarios"
	"nitroctl/daemon/internal/state"
)

const (
	busName     = "io.github.fnndk.Nitrod1"
	objPath     = dbus.ObjectPath("/io/github/fnndk/Nitrod1")
	ifSystem    = busName + ".System"
	ifMode      = busName + ".Mode"
	ifFans      = busName + ".Fans"
	ifMonitor   = busName + ".Monitor"
	ifLighting  = busName + ".Lighting"
	ifBattery   = busName + ".Battery"
	ifScenarios = busName + ".Scenarios"

	// Refresh cadences. Reads of fans, keyboard, USB and battery control go
	// through WMI/SMM and cost 5-10 ms each, so they are spread out; AC and
	// charge come from power_supply and are free.
	powerEvery    = 2 * time.Second  // without uevents
	powerFallback = 30 * time.Second // with uevents
	fansEvery     = 5 * time.Second  // RPM only (EC read, ~0.2 ms)
	fansFullEvery = 60 * time.Second // mode and speed too (WMI, ~23 ms per fan): catches other writers
	slowEvery     = 30 * time.Second // keyboard, USB charging, charge limit, calibration, health
	modeFallback  = 10 * time.Second // re-read platform_profile even without a poll() event
	modeDebounce  = 150 * time.Millisecond
	settleTimeout = 2 * time.Second // how long tuned gets to write platform_profile
	sourceDelay   = 1500 * time.Millisecond
)

// Config is what main passes in.
type Config struct {
	Session  bool // use the session bus (development)
	NoPolkit bool // skip polkit checks (development)
	Version  string
	// Reprobe rebuilds the hardware view (System.Reload); nil when simulating.
	Reprobe func() *hw.Hardware
	// NitroKey is what the daemon does on KEY_PROG1: "cycle" the profile or
	// "none" (the GUI/tray owns the key, as in the original app it opens the app).
	NitroKey string
}

// Daemon is the exported object with its state.
type Daemon struct {
	cfg     Config
	hw      *hw.Hardware
	st      *state.Store
	sc      *scenarios.Store
	ctx     context.Context
	conn    *dbus.Conn
	props   *prop.Properties
	ppd     *ppd.Client
	sampler *monitor.Sampler

	mu          sync.Mutex
	power       hw.Power
	powerKnown  bool
	current     string
	allowed     []string
	extras      []string
	expectPPD   string    // PPD profile the daemon itself asked for
	ignoreSysfs time.Time // until then, platform_profile changes are tuned's, not the user's
	resuming    bool      // from PrepareForSleep(true) until onResume is done

	subMu sync.Mutex
	subs  map[string]*subscriber

	fanMu    sync.Mutex
	fanModes map[string]hw.FanState // mode and speed from the last full read

	// syncMu serialises scenario application (D-Bus Apply and App Sync).
	syncMu    sync.Mutex
	syncStop  context.CancelFunc // App Sync watcher running when non-nil
	syncWatch *appsync.Watcher
}

// New builds the daemon; Run connects it.
func New(cfg Config, h *hw.Hardware, st *state.Store, sc *scenarios.Store) *Daemon {
	return &Daemon{cfg: cfg, hw: h, st: st, sc: sc, sampler: monitor.New(h), subs: map[string]*subscriber{}}
}

// Run serves until ctx is done.
func (d *Daemon) Run(ctx context.Context) error {
	d.ctx = ctx
	var err error
	if d.cfg.Session {
		d.conn, err = dbus.ConnectSessionBus()
	} else {
		d.conn, err = dbus.ConnectSystemBus()
	}
	if err != nil {
		return fmt.Errorf("connect bus: %w", err)
	}
	defer d.conn.Close()

	reply, err := d.conn.RequestName(busName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return fmt.Errorf("request name: %w", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("name %s already taken", busName)
	}
	d.ppd = ppd.Connect(d.conn)

	d.readInitial()
	if err := d.export(); err != nil {
		return err
	}
	slog.Info("nitrod ready", "bus", map[bool]string{true: "session", false: "system"}[d.cfg.Session],
		"version", d.cfg.Version)

	go d.refreshLoop(ctx)
	go d.modeLoop(ctx)
	go d.watchNameOwners(ctx)
	if d.ppd != nil {
		go d.watchPPD(ctx)
	}
	if !d.cfg.Session {
		go d.watchSleep(ctx)
	}
	if d.hw.NitroKey && d.cfg.Reprobe != nil && !d.cfg.Session && d.cfg.NitroKey == "cycle" {
		go hw.WatchNitroKey(ctx, func() {
			if next, err := d.cycle(); err != nil {
				slog.Warn("nitro key: cycle failed", "err", err)
			} else {
				slog.Info("nitro key", "profile", next)
			}
		})
	}
	d.restoreMode()
	d.restoreFans(true)
	d.startAppSync()

	<-ctx.Done()
	d.stopAllSubscribers()
	_, _ = d.conn.ReleaseName(busName)
	return nil
}

// ---- export ----------------------------------------------------------------

func (d *Daemon) export() error {
	var active string
	d.st.View(func(s state.Data) { active = s.Scenario })
	spec := prop.Map{
		ifSystem: {
			"Version":      {Value: d.cfg.Version, Emit: prop.EmitTrue},
			"Capabilities": {Value: variants(d.hw.Capabilities()), Emit: prop.EmitTrue},
			"Model":        {Value: variants(d.hw.Model), Emit: prop.EmitTrue},
		},
		ifMode: {
			"Current":        {Value: d.current, Emit: prop.EmitTrue},
			"PowerSource":    {Value: d.power.Source, Emit: prop.EmitTrue},
			"BatteryPercent": {Value: d.power.Percent, Emit: prop.EmitTrue},
			"Allowed":        {Value: nonNil(d.allowed), Emit: prop.EmitTrue},
			"Extras":         {Value: nonNil(d.extras), Emit: prop.EmitTrue},
		},
		ifFans: {
			"State": {Value: d.readFans(true), Emit: prop.EmitTrue},
		},
		ifLighting: {
			"Keyboard": {Value: d.keyboardState(), Emit: prop.EmitTrue},
		},
		ifBattery: {
			"ChargeLimit": {Value: d.chargeLimit(), Emit: prop.EmitTrue},
			"Calibrating": {Value: d.calibrating(), Emit: prop.EmitTrue},
			"UsbCharging": {Value: d.usbCharging(), Emit: prop.EmitTrue},
			"Health":      {Value: d.health(), Emit: prop.EmitTrue},
		},
		ifScenarios: {
			"List":    {Value: d.scenarioList(), Emit: prop.EmitTrue},
			"Active":  {Value: active, Emit: prop.EmitTrue},
			"AppSync": {Value: false, Emit: prop.EmitTrue}, // set when the watcher starts
		},
	}
	var err error
	d.props, err = prop.Export(d.conn, objPath, spec)
	if err != nil {
		return fmt.Errorf("export properties: %w", err)
	}

	objs := map[string]interface{}{
		ifSystem:    &systemIface{d},
		ifMode:      &modeIface{d},
		ifFans:      &fansIface{d},
		ifMonitor:   &monitorIface{d},
		ifLighting:  &lightingIface{d},
		ifBattery:   &batteryIface{d},
		ifScenarios: &scenariosIface{d},
	}
	node := &introspect.Node{Name: string(objPath), Interfaces: []introspect.Interface{introspect.IntrospectData, prop.IntrospectData}}
	for name, obj := range objs {
		if err := d.conn.Export(obj, objPath, name); err != nil {
			return fmt.Errorf("export %s: %w", name, err)
		}
		iface := introspect.Interface{Name: name, Methods: introspect.Methods(obj), Properties: d.props.Introspection(name)}
		if name == ifMonitor {
			iface.Signals = []introspect.Signal{{Name: "Sample", Args: []introspect.Arg{{Name: "sample", Type: "a{sv}"}}}}
		}
		node.Interfaces = append(node.Interfaces, iface)
	}
	return d.conn.Export(introspect.NewIntrospectable(node), objPath, "org.freedesktop.DBus.Introspectable")
}

// setProp updates a property only when its value changed, so that
// PropertiesChanged is only emitted on real changes.
func (d *Daemon) setProp(iface, name string, v interface{}) {
	if d.props == nil {
		return
	}
	if reflect.DeepEqual(d.props.GetMust(iface, name), v) {
		return
	}
	d.props.SetMust(iface, name, v)
}

// ---- state readers ---------------------------------------------------------

func (d *Daemon) readInitial() {
	d.refreshPower()
	if d.hw.Profiles != nil {
		if cur, err := d.hw.Profiles.Current(); err == nil {
			d.current = cur
		}
	}
	d.recompute()
}

func (d *Daemon) refreshLoop(ctx context.Context) {
	var events <-chan struct{}
	every := powerEvery
	if d.cfg.Reprobe != nil { // real hardware
		if events = hw.WatchPowerSupply(ctx); events != nil {
			every = powerFallback
		}
	}
	power := time.NewTicker(every)
	fans := time.NewTicker(fansEvery)
	fansFull := time.NewTicker(fansFullEvery)
	slow := time.NewTicker(slowEvery)
	defer power.Stop()
	defer fans.Stop()
	defer fansFull.Stop()
	defer slow.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-events:
			if !ok {
				events = nil
				power.Reset(powerEvery)
				continue
			}
			d.refreshPower()
		case <-power.C:
			d.refreshPower()
		case <-fans.C:
			d.refreshFans(false)
		case <-fansFull.C:
			d.refreshFans(true)
		case <-slow.C:
			d.refreshSlow()
		}
	}
}

// refreshSlow re-reads the EC-backed settings; also called after a resume.
func (d *Daemon) refreshSlow() {
	d.setProp(ifLighting, "Keyboard", d.keyboardState())
	d.setProp(ifBattery, "ChargeLimit", d.chargeLimit())
	d.setProp(ifBattery, "Calibrating", d.calibrating())
	d.setProp(ifBattery, "UsbCharging", d.usbCharging())
	d.setProp(ifBattery, "Health", d.health())
}

// modeLoop checks platform_profile on sysfs notifications when the backend
// supports poll(), and on a slow timer otherwise / as a fallback.
func (d *Daemon) modeLoop(ctx context.Context) {
	if d.hw.Profiles == nil {
		return
	}
	var events <-chan struct{}
	every := powerEvery
	if w, ok := d.hw.Profiles.(hw.ProfileWatcher); ok {
		events = w.Watch(ctx)
		every = modeFallback
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			d.watchSysfsMode()
		case _, ok := <-events:
			if !ok {
				events = nil
				t.Reset(powerEvery)
				continue
			}
			d.watchSysfsMode()
		}
	}
}

func (d *Daemon) readPower() hw.Power {
	if d.hw.Battery == nil {
		return hw.Power{Source: "ac", Percent: 100}
	}
	p, err := d.hw.Battery.Power()
	if err != nil {
		return hw.Power{Source: "ac", Percent: 100}
	}
	return p
}

// refreshPower re-reads AC/battery, recomputes Allowed and reacts to a
// source change or to the current profile becoming forbidden.
func (d *Daemon) refreshPower() {
	p := d.readPower()
	d.mu.Lock()
	prev, known := d.power, d.powerKnown
	d.power, d.powerKnown = p, true
	d.mu.Unlock()
	d.setProp(ifMode, "PowerSource", p.Source)
	d.setProp(ifMode, "BatteryPercent", p.Percent)
	d.recompute()
	if !known || d.hw.Profiles == nil {
		return
	}
	if prev.Source != p.Source {
		slog.Info("power source changed", "from", prev.Source, "to", p.Source)
		go d.onSourceChange(p.Source)
		return
	}
	d.mu.Lock()
	cur, allowed := d.current, d.allowed
	d.mu.Unlock()
	if cur != "" && !rules.Contains(allowed, cur) {
		if fb := rules.Fallback(allowed, d.st.ModeFor(p.Source)); fb != "" && fb != cur {
			slog.Info("profile no longer allowed", "current", cur, "fallback", fb)
			if err := d.applyMode(fb, false); err != nil {
				slog.Error("fallback failed", "err", err)
			}
		}
	}
}

func (d *Daemon) recompute() {
	if d.hw.Profiles == nil {
		return
	}
	d.mu.Lock()
	d.allowed, d.extras = rules.Allowed(d.hw.Profiles.Supported(), d.power)
	allowed, extras := d.allowed, d.extras
	d.mu.Unlock()
	d.setProp(ifMode, "Allowed", nonNil(allowed))
	d.setProp(ifMode, "Extras", nonNil(extras))
}

func (d *Daemon) onSourceChange(source string) {
	time.Sleep(sourceDelay) // let tuned-ppd switch/restore its profile first
	d.mu.Lock()
	allowed := d.allowed
	d.mu.Unlock()
	target := rules.Fallback(allowed, d.st.ModeFor(source))
	if target == "" {
		return
	}
	if err := d.applyMode(target, false); err != nil {
		slog.Error("apply profile after source change failed", "err", err)
	}
}

// restoreMode applies the remembered profile for the current source at
// startup; the unit orders the daemon after tuned so this wins.
func (d *Daemon) restoreMode() {
	if d.hw.Profiles == nil {
		return
	}
	d.mu.Lock()
	allowed, cur, source := d.allowed, d.current, d.power.Source
	d.mu.Unlock()
	target := rules.Fallback(allowed, d.st.ModeFor(source))
	if target == "" || target == cur {
		return
	}
	slog.Info("restoring profile", "source", source, "profile", target, "was", cur)
	if err := d.applyMode(target, false); err != nil {
		slog.Error("restore failed", "err", err)
	}
}

// watchSysfsMode detects platform_profile writes made by others.
func (d *Daemon) watchSysfsMode() {
	if d.hw.Profiles == nil {
		return
	}
	v, err := d.hw.Profiles.Current()
	if err != nil {
		return // a concurrent WMI call from another process garbled the answer
	}
	d.mu.Lock()
	cur, ignore, power, allowed, resuming := d.current, d.ignoreSysfs, d.power, d.allowed, d.resuming
	d.mu.Unlock()
	// The firmware puts platform_profile back to balanced on resume and tuned
	// follows; neither is the user's choice.
	if v == cur || resuming || time.Now().Before(ignore) {
		return
	}
	// Garbled reads show up as values outside the firmware list or as a
	// value that is gone on the next read; only a stable, known value counts.
	if !rules.Contains(d.hw.Profiles.Supported(), v) && !rules.Contains(platformProfileNames, v) {
		return
	}
	time.Sleep(modeDebounce)
	if again, err := d.hw.Profiles.Current(); err != nil || again != v {
		return
	}
	resolved := rules.FromExternal(v, power, allowed)
	slog.Info("external platform_profile write", "written", v, "applied", resolved)
	if err := d.applyMode(resolved, true); err != nil {
		slog.Error("apply external profile failed", "err", err)
	}
}

// watchPPD turns profile changes made through the PPD (GNOME menu) into the
// best Acer mode for the current power source.
func (d *Daemon) watchPPD(ctx context.Context) {
	ch, err := d.ppd.Watch()
	if err != nil {
		slog.Warn("cannot watch PPD", "err", err)
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case p := <-ch:
			d.mu.Lock()
			if p == d.expectPPD {
				d.expectPPD = ""
				d.mu.Unlock()
				continue
			}
			if d.resuming {
				d.mu.Unlock()
				slog.Info("PPD changed while resuming; ignored", "ppd", p)
				continue
			}
			d.ignoreSysfs = time.Now().Add(settleTimeout + time.Second)
			power, allowed := d.power, d.allowed
			d.mu.Unlock()
			mode := rules.FromPPD(p, power, allowed)
			slog.Info("PPD profile changed elsewhere", "ppd", p, "mode", mode)
			time.Sleep(time.Second) // tuned writes platform_profile; let it finish
			if err := d.applyMode(mode, true); err != nil {
				slog.Error("apply PPD-derived profile failed", "err", err)
			}
		}
	}
}

// applyMode is the single writer of platform_profile: it first moves the
// PPD to the nearest profile (the tuned write that follows is ignored), then
// writes the real mode, then remembers it per source when asked.
func (d *Daemon) applyMode(profile string, remember bool) error {
	if d.ppd != nil {
		target := rules.NearestPPD(profile)
		if cur, err := d.ppd.Active(); err == nil && cur != target {
			d.mu.Lock()
			d.expectPPD = target
			d.ignoreSysfs = time.Now().Add(settleTimeout + time.Second)
			d.mu.Unlock()
			if err := d.ppd.Set(target); err != nil {
				slog.Warn("PPD set failed", "profile", target, "err", err)
			} else {
				d.waitSysfsSettle()
			}
		}
	}
	if err := d.hw.Profiles.Set(profile); err != nil {
		return err
	}
	d.mu.Lock()
	d.current = profile
	source := d.power.Source
	d.mu.Unlock()
	d.setProp(ifMode, "Current", profile)
	if remember {
		d.leaveScenario()
		d.st.Update(func(s *state.Data) {
			if source == "battery" {
				s.ModeBattery = profile
			} else {
				s.ModeAC = profile
			}
		})
	}
	return nil
}

// waitSysfsSettle waits for tuned to write platform_profile after a PPD
// change, up to settleTimeout.
func (d *Daemon) waitSysfsSettle() {
	start, _ := d.hw.Profiles.Current()
	deadline := time.Now().Add(settleTimeout)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		if v, _ := d.hw.Profiles.Current(); v != start {
			time.Sleep(200 * time.Millisecond)
			return
		}
	}
}

// cycle is Mode.Cycle without authorization (used by the Nitro key).
func (d *Daemon) cycle() (string, error) {
	d.mu.Lock()
	next := rules.Next(d.allowed, d.current)
	d.mu.Unlock()
	if next == "" {
		return "", fmt.Errorf("no profile allowed now")
	}
	return next, d.applyMode(next, true)
}

// ---- property values -------------------------------------------------------

// readFans builds Fans.State. A full read also re-reads mode and speed,
// which cost WMI calls; otherwise only the RPM is read and mode/speed come
// from the last full read (writes and resume always do a full read).
func (d *Daemon) readFans(full bool) map[string]map[string]dbus.Variant {
	out := map[string]map[string]dbus.Variant{}
	if d.hw.Fans == nil {
		return out
	}
	d.fanMu.Lock()
	defer d.fanMu.Unlock()
	if full || d.fanModes == nil {
		st, err := d.hw.Fans.State()
		if err != nil {
			return out
		}
		d.fanModes = st
	} else if rpm, err := d.hw.Fans.RPM(); err == nil {
		for id, r := range rpm {
			f := d.fanModes[id]
			f.RPM = r
			d.fanModes[id] = f
		}
	}
	for id, f := range d.fanModes {
		out[id] = map[string]dbus.Variant{
			"rpm":   dbus.MakeVariant(f.RPM),
			"mode":  dbus.MakeVariant(f.Mode),
			"speed": dbus.MakeVariant(f.Speed),
		}
	}
	return out
}

func (d *Daemon) refreshFans(full bool) { d.setProp(ifFans, "State", d.readFans(full)) }

func (d *Daemon) keyboardState() map[string]dbus.Variant {
	out := map[string]dbus.Variant{}
	if d.hw.Keyboard == nil {
		return out
	}
	if on, err := d.hw.Keyboard.On(); err == nil {
		out["on"] = dbus.MakeVariant(on)
	}
	if t, err := d.hw.Keyboard.Timeout(); err == nil {
		out["timeout_s"] = dbus.MakeVariant(t)
	}
	return out
}

func (d *Daemon) chargeLimit() bool {
	if d.hw.BatteryCtl == nil {
		return false
	}
	v, _ := d.hw.BatteryCtl.ChargeLimit()
	return v
}

func (d *Daemon) calibrating() bool {
	if d.hw.BatteryCtl == nil {
		return false
	}
	v, _ := d.hw.BatteryCtl.Calibrating()
	return v
}

func (d *Daemon) usbCharging() uint32 {
	if d.hw.USB == nil {
		return 0
	}
	v, _ := d.hw.USB.Get()
	return v
}

func (d *Daemon) health() map[string]dbus.Variant {
	out := map[string]dbus.Variant{}
	if d.hw.Battery == nil {
		return out
	}
	h, err := d.hw.Battery.Health()
	if err != nil {
		return out
	}
	out["cycles"] = dbus.MakeVariant(h.Cycles)
	out["full"] = dbus.MakeVariant(h.Full)
	out["design"] = dbus.MakeVariant(h.Design)
	if h.Design > 0 {
		out["health_pct"] = dbus.MakeVariant(uint32(h.Full * 100 / h.Design))
	}
	out["status"] = dbus.MakeVariant(h.Status)
	out["voltage"] = dbus.MakeVariant(h.Voltage)
	out["current"] = dbus.MakeVariant(h.Current)
	if !math.IsNaN(h.Temp) {
		out["temp"] = dbus.MakeVariant(h.Temp)
	}
	return out
}

// platformProfileNames are the values the kernel can hold; a write of
// performance (Turbo) on a model without it is still a real external write.
var platformProfileNames = []string{"low-power", "quiet", "balanced", "balanced-performance", "performance"}

// ---- helpers ---------------------------------------------------------------

// variants converts a nested map into a{sv}, recursing into sub-maps.
func variants(m map[string]interface{}) map[string]dbus.Variant {
	out := map[string]dbus.Variant{}
	for k, v := range m {
		if sub, ok := v.(map[string]interface{}); ok {
			out[k] = dbus.MakeVariant(variants(sub))
			continue
		}
		out[k] = dbus.MakeVariant(v)
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
