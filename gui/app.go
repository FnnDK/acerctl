package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"nitroctl/daemon/client"
)

// Events sent to the frontend.
const (
	evState  = "state"  // full State(), after every PropertiesChanged or (dis)connection
	evSample = "sample" // one Monitor sample
)

// App is the bridge between the frontend and nitrod. Every method bound to
// the frontend maps to one daemon call; the daemon decides and explains.
type App struct {
	ctx     context.Context
	session bool

	mu      sync.Mutex
	cl      *client.Client
	lastErr string
	monWant uint32 // requested Monitor interval in ms, 0 = off
	monStop func()
	quit    chan struct{}

	sessionBus *dbus.Conn // settings portal (title bar)
}

func NewApp(session bool) *App {
	return &App{session: session, quit: make(chan struct{})}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.watchTitlebar()
	go a.run()
}

func (a *App) shutdown(ctx context.Context) {
	close(a.quit)
	if a.sessionBus != nil {
		a.sessionBus.Close()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopMonitorLocked()
	if a.cl != nil {
		a.cl.Close()
		a.cl = nil
	}
}

// secondInstance brings the window back when the app is launched again
// (desktop shortcut on the Nitro key, launcher).
func (a *App) secondInstance(options.SecondInstanceData) {
	runtime.WindowUnminimise(a.ctx)
	runtime.Show(a.ctx)
}

// run keeps a connection to the daemon, reconnecting when it restarts.
func (a *App) run() {
	for {
		cl, err := client.Connect(a.session)
		if err != nil {
			a.setDisconnected(err.Error())
			select {
			case <-a.quit:
				return
			case <-time.After(2 * time.Second):
				continue
			}
		}
		a.serve(cl)
		select {
		case <-a.quit:
			return
		default:
		}
	}
}

// serve forwards daemon signals until the daemon leaves the bus.
func (a *App) serve(cl *client.Client) {
	conn := cl.Conn()
	sigs := make(chan *dbus.Signal, 64)
	conn.Signal(sigs)
	defer conn.RemoveSignal(sigs)
	_ = conn.AddMatchSignal(
		dbus.WithMatchObjectPath(client.ObjPath),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
	)
	_ = conn.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, client.BusName),
	)

	a.mu.Lock()
	a.cl, a.lastErr = cl, ""
	if a.monWant > 0 {
		a.startMonitorLocked()
	}
	a.mu.Unlock()
	a.emitState()
	a.ensureAgent() // first run after install: no login yet to autostart it

	// PropertiesChanged comes in bursts (one per interface); coalesce them.
	var pending <-chan time.Time
	for {
		select {
		case <-a.quit:
			return
		case <-pending:
			pending = nil
			a.emitState()
		case s, ok := <-sigs:
			if !ok {
				a.drop(cl, "connection to D-Bus lost")
				return
			}
			switch s.Name {
			case "org.freedesktop.DBus.Properties.PropertiesChanged":
				if pending == nil {
					pending = time.After(80 * time.Millisecond)
				}
			case "org.freedesktop.DBus.NameOwnerChanged":
				if len(s.Body) == 3 && s.Body[2] == "" {
					a.drop(cl, "nitrod stopped")
					return
				}
			}
		}
	}
}

func (a *App) drop(cl *client.Client, why string) {
	a.mu.Lock()
	a.stopMonitorLocked()
	if a.cl == cl {
		a.cl = nil
	}
	a.mu.Unlock()
	cl.Close()
	a.setDisconnected(why)
}

func (a *App) setDisconnected(why string) {
	a.mu.Lock()
	changed := a.lastErr != why
	a.lastErr = why
	a.mu.Unlock()
	if changed {
		a.emitState()
	}
}

func (a *App) emitState() { runtime.EventsEmit(a.ctx, evState, a.State()) }

func (a *App) client() (*client.Client, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cl == nil {
		return nil, uiError(&client.Error{Kind: "Disconnected", Msg: a.lastErr})
	}
	return a.cl, nil
}

// State is everything the daemon publishes, one dictionary per interface
// (property names as in docs/api-dbus.md), plus the connection status.
func (a *App) State() map[string]interface{} {
	out := map[string]interface{}{"connected": false}
	a.mu.Lock()
	cl, why := a.cl, a.lastErr
	a.mu.Unlock()
	if cl == nil {
		out["error"] = why
		return out
	}
	ifaces := map[string]string{
		"system": client.IfSystem, "mode": client.IfMode, "fans": client.IfFans,
		"lighting": client.IfLighting, "battery": client.IfBattery, "scenarios": client.IfScenarios,
	}
	for key, iface := range ifaces {
		props, err := cl.Properties(iface)
		if err != nil {
			out["error"] = err.Error()
			return out
		}
		out[key] = client.PlainMap(props)
	}
	out["connected"] = true
	return out
}

// Monitor starts (interval > 0) or stops the sample stream. The frontend
// stops it while the window is hidden: without subscribers the daemon does
// not sample at all.
func (a *App) Monitor(intervalMs uint32) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if intervalMs == a.monWant {
		return
	}
	a.stopMonitorLocked()
	a.monWant = intervalMs
	if intervalMs > 0 && a.cl != nil {
		a.startMonitorLocked()
	}
}

func (a *App) startMonitorLocked() {
	samples, stop, err := a.cl.Subscribe(a.monWant)
	if err != nil {
		runtime.LogWarningf(a.ctx, "monitor: %v", err)
		return
	}
	a.monStop = stop
	go func() {
		for s := range samples {
			runtime.EventsEmit(a.ctx, evSample, client.PlainMap(s))
		}
	}()
}

func (a *App) stopMonitorLocked() {
	if a.monStop != nil {
		a.monStop()
		a.monStop = nil
	}
}

// call runs one daemon method and turns its refusal into a frontend error.
func (a *App) call(iface, method string, args ...interface{}) error {
	cl, err := a.client()
	if err != nil {
		return err
	}
	return uiError(cl.Call(iface, method, nil, args...))
}

// uiError encodes daemon errors as JSON {kind, msg} so the frontend can
// translate the kind and show the daemon's reason as is.
func uiError(err error) error {
	if err == nil {
		return nil
	}
	e := &client.Error{Kind: "Other", Msg: err.Error()}
	errors.As(err, &e)
	b, _ := json.Marshal(map[string]string{"kind": e.Kind, "msg": e.Msg})
	return errors.New(string(b))
}

func (a *App) SetMode(profile string) error { return a.call(client.IfMode, "Set", profile) }

func (a *App) SetFanMode(fan, mode string) error { return a.call(client.IfFans, "SetMode", fan, mode) }

func (a *App) SetFanSpeed(fan string, percent uint32) error {
	return a.call(client.IfFans, "SetSpeed", fan, percent)
}

func (a *App) SetKeyboardTimeout(seconds uint32) error {
	return a.call(client.IfLighting, "SetKeyboardTimeout", seconds)
}

func (a *App) SetChargeLimit(on bool) error { return a.call(client.IfBattery, "SetChargeLimit", on) }

func (a *App) StartCalibration() error { return a.call(client.IfBattery, "StartCalibration") }

func (a *App) StopCalibration() error { return a.call(client.IfBattery, "StopCalibration") }

func (a *App) SetUsbCharging(percent uint32) error {
	return a.call(client.IfBattery, "SetUsbCharging", percent)
}

func (a *App) ApplyScenario(name string) error { return a.call(client.IfScenarios, "Apply", name) }

func (a *App) DeleteScenario(name string) error { return a.call(client.IfScenarios, "Delete", name) }

func (a *App) SetAppSync(on bool) error { return a.call(client.IfScenarios, "SetAppSync", on) }

func (a *App) Reload() error { return a.call(client.IfSystem, "Reload") }

// FanSetting is one fan inside a scenario.
type FanSetting struct {
	Mode  string `json:"mode"`
	Speed uint32 `json:"speed,omitempty"`
}

// Scenario mirrors the daemon format (api-dbus.md §9); empty fields are left
// out so the scenario does not touch them.
type Scenario struct {
	Name        string                `json:"name"`
	ModeAC      string                `json:"mode_ac,omitempty"`
	ModeBattery string                `json:"mode_battery,omitempty"`
	Fans        map[string]FanSetting `json:"fans,omitempty"`
	KbdTimeout  *uint32               `json:"kbd_timeout_s,omitempty"`
	Apps        []string              `json:"apps,omitempty"`
}

func (a *App) SaveScenario(sc Scenario) error {
	m := map[string]dbus.Variant{"name": dbus.MakeVariant(strings.TrimSpace(sc.Name))}
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
	if len(sc.Apps) > 0 {
		m["apps"] = dbus.MakeVariant(sc.Apps)
	}
	return a.call(client.IfScenarios, "Save", m)
}

// Autostart reports whether the app starts with the desktop session.
func (a *App) Autostart() bool {
	_, err := os.Stat(autostartPath())
	return err == nil
}

// SetAutostart writes or removes the XDG autostart entry (minimized start).
func (a *App) SetAutostart(on bool) error {
	p := autostartPath()
	if !on {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return uiError(err)
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return uiError(err)
	}
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Nitro Control Center
Exec=%s --minimized
Icon=%s
X-GNOME-Autostart-enabled=true
`, exe, appID)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return uiError(err)
	}
	return uiError(os.WriteFile(p, []byte(entry), 0o644))
}

func autostartPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "autostart", appID+".desktop")
}
