// nitro-tray is the session agent of nitroctl: the tray icon
// (StatusNotifierItem) with the quick modes, and the process that opens the
// app when the Nitro key is set to "app" (a system service cannot start
// desktop apps; nitrod sends System.NitroKeyPressed). It exits when neither
// is wanted: tray icon hidden in the user settings and the key cycling modes.
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"fyne.io/systray"
	"github.com/godbus/dbus/v5"

	"nitroctl/daemon/client"
	"nitroctl/daemon/usercfg"
)

//go:embed icon.png
var iconPNG []byte

const (
	agentName = "io.github.fnndk.NitroTray"
	agentPath = dbus.ObjectPath("/io/github/fnndk/NitroTray")
	appBinary = "nitro-control-center"
)

// Fixed menu order; profiles missing from the capabilities stay hidden.
var allProfiles = []string{"low-power", "quiet", "balanced", "balanced-performance", "performance"}

func main() {
	session := flag.Bool("session", os.Getenv("NITROCTL_SESSION") == "1", "talk to a daemon on the session bus (development)")
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	sbus, err := dbus.ConnectSessionBus()
	if err != nil {
		slog.Error("session bus", "err", err)
		os.Exit(1)
	}
	// One agent per session: the GUI starts it again whenever it may be needed.
	if reply, err := sbus.RequestName(agentName, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		return
	}
	a := &agent{session: *session, sbus: sbus, cfg: usercfg.Load(), refresh: make(chan struct{}, 1)}
	if err := sbus.Export(a, agentPath, agentName); err != nil {
		slog.Error("export", "err", err)
		os.Exit(1)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-sigs; a.quit() }()

	if a.cfg.TrayHidden {
		a.daemonLoop() // only for the Nitro key
		return
	}
	systray.Run(a.onReady, nil)
}

type agent struct {
	session bool
	sbus    *dbus.Conn
	cfg     usercfg.Settings

	mu      sync.Mutex
	cl      *client.Client
	keyMode string // System.NitroKey of the daemon, "" until known

	menu    *menu // nil without the tray icon
	refresh chan struct{}
}

// Reload is called by the GUI after it changes the user settings.
func (a *agent) Reload() *dbus.Error {
	cfg := usercfg.Load()
	go func() {
		time.Sleep(100 * time.Millisecond) // let the reply go out
		if cfg.TrayHidden != a.cfg.TrayHidden {
			a.restart() // the icon is set up once, at start
		}
		a.mu.Lock()
		a.cfg = cfg
		a.mu.Unlock()
		a.exitIfIdle()
	}()
	return nil
}

func (a *agent) restart() {
	exe, err := os.Executable()
	if err == nil {
		_ = syscall.Exec(exe, os.Args, os.Environ())
	}
	slog.Error("restart", "err", err)
	a.quit()
}

func (a *agent) quit() {
	if a.menu != nil {
		systray.Quit()
		return
	}
	os.Exit(0)
}

// exitIfIdle ends the agent when it has nothing to do.
func (a *agent) exitIfIdle() {
	a.mu.Lock()
	idle := a.cfg.TrayHidden && a.keyMode != "" && a.keyMode != "app"
	a.mu.Unlock()
	if idle {
		slog.Info("tray hidden and Nitro key cycling modes: nothing to do")
		a.quit()
	}
}

// ---- daemon ----------------------------------------------------------------

// daemonLoop keeps a connection to nitrod, reconnecting when it restarts.
// Without the tray icon, giving up after a while is fine: nothing to show.
func (a *agent) daemonLoop() {
	failures := 0
	for {
		cl, err := client.Connect(a.session)
		if err != nil {
			a.setClient(nil)
			if failures++; a.menu == nil && failures > 15 {
				slog.Info("nitrod unreachable, exiting", "err", err)
				return
			}
			time.Sleep(2 * time.Second)
			continue
		}
		failures = 0
		a.serve(cl)
	}
}

func (a *agent) setClient(cl *client.Client) {
	a.mu.Lock()
	a.cl = cl
	a.mu.Unlock()
	a.poke()
}

func (a *agent) client() *client.Client {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cl
}

func (a *agent) poke() {
	select {
	case a.refresh <- struct{}{}:
	default:
	}
}

func (a *agent) serve(cl *client.Client) {
	conn := cl.Conn()
	sigs := make(chan *dbus.Signal, 32)
	conn.Signal(sigs)
	_ = conn.AddMatchSignal(dbus.WithMatchObjectPath(client.ObjPath),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"), dbus.WithMatchMember("PropertiesChanged"))
	// Only nitrod itself may send the key press (the bus resolves the name to its owner).
	_ = conn.AddMatchSignal(dbus.WithMatchSender(client.BusName), dbus.WithMatchObjectPath(client.ObjPath),
		dbus.WithMatchInterface(client.IfSystem), dbus.WithMatchMember("NitroKeyPressed"))
	_ = conn.AddMatchSignal(dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"), dbus.WithMatchArg(0, client.BusName))
	owner := ""
	_ = conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, client.BusName).Store(&owner)

	a.setClient(cl)
	a.update()
	var pending <-chan time.Time
	for {
		select {
		case <-pending:
			pending = nil
			a.update()
		case s, ok := <-sigs:
			if !ok {
				a.setClient(nil)
				return
			}
			switch s.Name {
			case "org.freedesktop.DBus.Properties.PropertiesChanged":
				if pending == nil {
					pending = time.After(80 * time.Millisecond)
				}
			case client.IfSystem + ".NitroKeyPressed":
				if s.Sender == owner && len(s.Body) == 1 && s.Body[0] == "app" {
					openApp()
				}
			case "org.freedesktop.DBus.NameOwnerChanged":
				if len(s.Body) == 3 && s.Body[2] == "" {
					conn.RemoveSignal(sigs)
					cl.Close()
					a.setClient(nil)
					return
				}
			}
		}
	}
}

// update reads what the agent needs and refreshes the menu.
func (a *agent) update() {
	cl := a.client()
	if cl == nil {
		return
	}
	if v, err := cl.Property(client.IfSystem, "NitroKey"); err == nil {
		k, _ := v.Value().(string)
		a.mu.Lock()
		a.keyMode = k
		a.mu.Unlock()
	}
	a.exitIfIdle()
	a.poke()
}

// openApp starts the GUI; a running instance just comes to the front.
func openApp() {
	bin, err := exec.LookPath(appBinary)
	if err != nil {
		bin = "/usr/local/bin/" + appBinary
	}
	cmd := exec.Command(bin)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		slog.Warn("open app", "err", err)
		return
	}
	go func() { _ = cmd.Wait() }()
}

// notify shows a desktop notification (daemon refusals).
func (a *agent) notify(title, body string) {
	obj := a.sbus.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	obj.Call("org.freedesktop.Notifications.Notify", 0, "Nitro Control Center", uint32(0),
		"io.github.fnndk.NitroControlCenter", title, body, []string{}, map[string]dbus.Variant{}, int32(5000))
}

func (a *agent) do(err error) {
	if err == nil {
		return
	}
	title, msg := tr("error"), err.Error()
	if e, ok := err.(*client.Error); ok {
		title, msg = errTitle(e.Kind), e.Msg
	}
	a.notify(title, msg)
}

// ---- menu ------------------------------------------------------------------

type menu struct {
	status, open, fansMax, scen, appSync, quit *systray.MenuItem
	modes                                      map[string]*systray.MenuItem

	scenNames []string
	scenItems map[string]*systray.MenuItem
}

func (a *agent) onReady() {
	systray.SetIcon(iconPNG)
	systray.SetTooltip("Nitro Control Center")
	m := &menu{modes: map[string]*systray.MenuItem{}, scenItems: map[string]*systray.MenuItem{}}
	m.status = systray.AddMenuItem("Nitro Control Center", "")
	m.status.Disable()
	m.open = systray.AddMenuItem(tr("open"), "")
	systray.AddSeparator()
	for _, p := range allProfiles {
		it := systray.AddMenuItemCheckbox(profileName(p), "", false)
		it.Hide()
		m.modes[p] = it
		go a.onClick(it, func() error { return a.call(client.IfMode, "Set", p) })
	}
	systray.AddSeparator()
	m.fansMax = systray.AddMenuItemCheckbox(tr("fans_max"), "", false)
	m.scen = systray.AddMenuItem(tr("scenarios"), "")
	m.appSync = m.scen.AddSubMenuItemCheckbox("App Sync", tr("appsync_hint"), false)
	m.scen.AddSeparator()
	systray.AddSeparator()
	m.quit = systray.AddMenuItem(tr("quit"), "")
	a.menu = m

	go a.onClick(m.open, func() error { openApp(); return nil })
	go a.onClick(m.fansMax, func() error {
		mode := "max"
		if m.fansMax.Checked() {
			mode = "auto"
		}
		return a.call(client.IfFans, "SetMode", "all", mode)
	})
	go a.onClick(m.appSync, func() error { return a.call(client.IfScenarios, "SetAppSync", !m.appSync.Checked()) })
	go func() {
		for range m.quit.ClickedCh {
			systray.Quit()
		}
	}()
	go a.menuLoop()
	go a.daemonLoop()
}

func (a *agent) onClick(it *systray.MenuItem, fn func() error) {
	for range it.ClickedCh {
		a.do(fn())
	}
}

func (a *agent) call(iface, method string, args ...interface{}) error {
	cl := a.client()
	if cl == nil {
		return &client.Error{Kind: "Disconnected"}
	}
	return cl.Call(iface, method, nil, args...)
}

// menuLoop redraws the menu from the daemon's properties.
func (a *agent) menuLoop() {
	for range a.refresh {
		a.redraw()
	}
}

func (a *agent) redraw() {
	m := a.menu
	cl := a.client()
	if cl == nil {
		m.status.SetTitle(tr("disconnected"))
		systray.SetTooltip("Nitro Control Center — " + tr("disconnected"))
		for _, it := range m.modes {
			it.Disable()
		}
		m.fansMax.Disable()
		m.scen.Disable()
		return
	}
	sys, _ := cl.Properties(client.IfSystem)
	mode, _ := cl.Properties(client.IfMode)
	caps := client.PlainMap(sys["Capabilities"])
	md := client.PlainMap(mode)

	current, _ := md["Current"].(string)
	allowed := strList(md["Allowed"])
	extras := strList(md["Extras"])
	supported := strList(caps["profiles"])
	for _, p := range allProfiles {
		it := m.modes[p]
		if !contains(supported, p) {
			it.Hide()
			continue
		}
		title := profileName(p)
		if contains(extras, p) {
			title += " · " + tr("extra")
		}
		it.SetTitle(title)
		it.Show()
		setChecked(it, p == current)
		if contains(allowed, p) {
			it.Enable()
		} else {
			it.Disable()
		}
	}

	pct := fmt.Sprint(md["BatteryPercent"])
	src := tr("ac")
	if md["PowerSource"] == "battery" {
		src = tr("battery")
	}
	status := fmt.Sprintf("%s · %s %% · %s", profileName(current), pct, src)
	m.status.SetTitle(status)
	systray.SetTooltip("Nitro Control Center — " + status)

	if _, ok := caps["fans"]; ok {
		st, _ := cl.Property(client.IfFans, "State")
		fans := client.PlainMap(st)
		allMax := len(fans) > 0
		for _, f := range fans {
			if fm, _ := f.(map[string]interface{}); fm["mode"] != "max" {
				allMax = false
			}
		}
		setChecked(m.fansMax, allMax)
		m.fansMax.Enable()
		m.fansMax.Show()
	} else {
		m.fansMax.Hide()
	}

	sc, _ := cl.Properties(client.IfScenarios)
	scp := client.PlainMap(sc)
	active, _ := scp["Active"].(string)
	setChecked(m.appSync, scp["AppSync"] == true)
	var names []string
	if list, ok := scp["List"].([]interface{}); ok {
		for _, x := range list {
			if xm, ok := x.(map[string]interface{}); ok {
				if n, ok := xm["name"].(string); ok {
					names = append(names, n)
				}
			}
		}
	}
	if strings.Join(names, "\x00") != strings.Join(m.scenNames, "\x00") {
		for _, it := range m.scenItems {
			it.Remove()
		}
		m.scenItems = map[string]*systray.MenuItem{}
		for _, n := range names {
			it := m.scen.AddSubMenuItemCheckbox(n, "", false)
			m.scenItems[n] = it
			go a.onClick(it, func() error { return a.call(client.IfScenarios, "Apply", n) })
		}
		m.scenNames = names
	}
	for n, it := range m.scenItems {
		setChecked(it, n == active)
	}
	m.scen.Enable()
}

func setChecked(it *systray.MenuItem, on bool) {
	if on != it.Checked() {
		if on {
			it.Check()
		} else {
			it.Uncheck()
		}
	}
}

func strList(v interface{}) []string {
	s, _ := v.([]string)
	return s
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// ---- texts -------------------------------------------------------------------

var pt = strings.HasPrefix(firstEnv("LC_ALL", "LC_MESSAGES", "LANG"), "pt")

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

var texts = map[string][2]string{ // en, pt
	"open":         {"Open Nitro Control Center", "Abrir o Nitro Control Center"},
	"fans_max":     {"Fans at maximum", "Ventoinhas no máximo"},
	"scenarios":    {"Scenarios", "Cenários"},
	"appsync_hint": {"Apply scenarios when their apps run", "Aplicar cenários quando os apps deles abrirem"},
	"quit":         {"Hide until next login", "Ocultar até o próximo login"},
	"disconnected": {"nitrod is not running", "nitrod não está rodando"},
	"extra":        {"extra", "extra"},
	"ac":           {"on AC", "na tomada"},
	"battery":      {"on battery", "na bateria"},
	"error":        {"Error", "Erro"},

	"low-power": {"Eco", "Eco"}, "quiet": {"Quiet", "Silencioso"}, "balanced": {"Balanced", "Equilibrado"},
	"balanced-performance": {"Performance", "Desempenho"}, "performance": {"Turbo", "Turbo"},

	"err.NotSupported":    {"Not supported on this laptop", "Não suportado neste notebook"},
	"err.NotAllowed":      {"Not allowed right now", "Não permitido agora"},
	"err.InvalidArgument": {"Invalid value", "Valor inválido"},
	"err.Hardware":        {"Hardware error", "Erro do hardware"},
	"err.AccessDenied":    {"Not authorized", "Não autorizado"},
	"err.Disconnected":    {"nitrod is not running", "nitrod não está rodando"},
}

func tr(k string) string {
	t, ok := texts[k]
	if !ok {
		return k
	}
	if pt {
		return t[1]
	}
	return t[0]
}

func profileName(p string) string {
	if _, ok := texts[p]; ok {
		return tr(p)
	}
	return p
}

func errTitle(kind string) string {
	if _, ok := texts["err."+kind]; ok {
		return tr("err." + kind)
	}
	return kind
}
