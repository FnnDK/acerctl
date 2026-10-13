// nitroctl is the command-line client of nitrod: everything the
// GUI does, over the same D-Bus API (docs/api-dbus.md).
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"

	"nitroctl/daemon/client"
)

const usage = `usage: nitroctl [--json] [--session] <command> [args]

Status
  status                          everything at a glance (default)
  caps                            what the firmware exposes on this machine
  monitor [-i ms] [-n count]      live sensors (Ctrl-C to stop)

Thermal profile
  mode                            current, allowed and extra profiles
  mode <profile>                  set: low-power|eco, quiet, balanced,
                                  balanced-performance, performance|turbo
  mode cycle                      next allowed profile (what the Nitro key does)

Fans
  fan                             rpm, mode and speed of each fan
  fan <cpu|gpu|all> auto|max      firmware curve / full speed
  fan <cpu|gpu|all> <percent>     manual speed

Keyboard, battery, USB
  kbd timeout <seconds>           backlight timeout (values in caps)
  battery                         charge limit, calibration, health
  battery limit on|off            stop charging at 80 %
  battery calibrate start|stop    calibration (needs AC)
  usb-charging <percent>          charge USB devices while off (0 = off)

Scenarios
  scenario [list]                 saved scenarios; * marks the active one
  scenario apply|delete <name>
  scenario save <name> [--from-current] [--mode-ac P] [--mode-battery P]
                [--fan id=auto|max|PCT]... [--kbd-timeout S] [--app EXE]...
  appsync [on|off]                apply scenarios when their apps run

Other
  reload                          re-probe kernel modules and capabilities
  version
`

// Acer names of the platform_profile values.
var acerName = map[string]string{
	"low-power": "Eco", "quiet": "Quiet", "balanced": "Balanced",
	"balanced-performance": "Performance", "performance": "Turbo",
}

var (
	jsonOut bool
	cl      *client.Client
)

func main() {
	fs := flag.NewFlagSet("nitroctl", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	session := fs.Bool("session", false, "talk to a daemon on the session bus (development)")
	fs.BoolVar(&jsonOut, "json", false, "machine-readable output")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	args := fs.Args()
	if len(args) == 0 {
		args = []string{"status"}
	}
	if args[0] == "help" || args[0] == "-h" {
		fmt.Print(usage)
		return
	}

	var err error
	if cl, err = client.Connect(*session); err != nil {
		fail(err)
	}
	defer cl.Close()

	cmds := map[string]func([]string) error{
		"status": cmdStatus, "caps": cmdCaps, "monitor": cmdMonitor,
		"mode": cmdMode, "fan": cmdFan, "fans": cmdFan, "kbd": cmdKbd,
		"battery": cmdBattery, "usb-charging": cmdUSB,
		"scenario": cmdScenario, "scenarios": cmdScenario, "appsync": cmdAppSync,
		"reload": cmdReload, "version": cmdVersion,
	}
	run, ok := cmds[args[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", args[0], usage)
		os.Exit(2)
	}
	if err := run(args[1:]); err != nil {
		fail(err)
	}
}

func fail(err error) {
	var ue usageError
	if errors.As(err, &ue) {
		fmt.Fprintf(os.Stderr, "nitroctl: %v\n(see nitroctl help)\n", err)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "nitroctl: %v\n", err)
	os.Exit(1)
}

type usageError string

func (e usageError) Error() string { return string(e) }

func usagef(format string, a ...interface{}) error { return usageError(fmt.Sprintf(format, a...)) }

// ---- status ----------------------------------------------------------------

func cmdStatus(args []string) error {
	all := map[string]map[string]dbus.Variant{}
	for _, iface := range []string{client.IfSystem, client.IfMode, client.IfFans, client.IfLighting, client.IfBattery, client.IfScenarios} {
		p, err := cl.Properties(iface)
		if err != nil {
			return err
		}
		all[strings.TrimPrefix(iface, client.BusName+".")] = p
	}
	if jsonOut {
		return printJSON(plain(all))
	}
	sys, mode, fans, light, bat, sc := all["System"], all["Mode"], all["Fans"], all["Lighting"], all["Battery"], all["Scenarios"]
	caps := plainMap(sys["Capabilities"])
	model := plainMap(sys["Model"])

	row("Model", fmt.Sprintf("%s %s (BIOS %s), daemon %s", str(model["vendor"]), str(model["product"]), str(model["bios"]), str(sys["Version"].Value())))
	src := "AC"
	if str(mode["PowerSource"].Value()) == "battery" {
		src = "battery"
	}
	row("Power", fmt.Sprintf("%s, battery %d %%", src, num(mode["BatteryPercent"].Value())))
	if _, ok := caps["profiles"]; ok {
		row("Mode", profileLine(mode))
	}
	if _, ok := caps["fans"]; ok {
		row("Fans", fanLine(plainMap(fans["State"])))
	}
	if _, ok := caps["kbd_backlight"]; ok {
		k := plainMap(light["Keyboard"])
		on := "off"
		if b, _ := k["on"].(bool); b {
			on = "on"
		}
		row("Keyboard", fmt.Sprintf("backlight %s, timeout %s", on, timeout(k["timeout_s"])))
	}
	row("Battery", batteryLine(caps, bat))
	active := str(sc["Active"].Value())
	if active == "" {
		active = "none"
	}
	sync := "off"
	if b, _ := sc["AppSync"].Value().(bool); b {
		sync = "on"
	}
	row("Scenario", fmt.Sprintf("%s (App Sync %s)", active, sync))
	return nil
}

func profileLine(mode map[string]dbus.Variant) string {
	cur := str(mode["Current"].Value())
	allowed, _ := mode["Allowed"].Value().([]string)
	extras, _ := mode["Extras"].Value().([]string)
	var parts []string
	for _, p := range allowed {
		if contains(extras, p) {
			p += " (extra)"
		}
		parts = append(parts, p)
	}
	return fmt.Sprintf("%s (%s); allowed now: %s", cur, acerName[cur], strings.Join(parts, ", "))
}

func fanLine(state map[string]interface{}) string {
	var parts []string
	for _, id := range sortedKeys(state) {
		f, _ := state[id].(map[string]interface{})
		m := str(f["mode"])
		if m == "manual" {
			m = fmt.Sprintf("manual %d %%", num(f["speed"]))
		}
		parts = append(parts, fmt.Sprintf("%s %d rpm %s", id, num(f["rpm"]), m))
	}
	return strings.Join(parts, "; ")
}

func batteryLine(caps map[string]interface{}, bat map[string]dbus.Variant) string {
	bc, _ := caps["battery"].(map[string]interface{})
	var parts []string
	if b, _ := bc["charge_limit"].(bool); b {
		parts = append(parts, "limit 80 % "+onOff(bat["ChargeLimit"].Value()))
	}
	if b, _ := bc["calibration"].(bool); b && bat["Calibrating"].Value() == true {
		parts = append(parts, "calibrating")
	}
	if _, ok := caps["usb_charging"]; ok {
		u := num(bat["UsbCharging"].Value())
		if u == 0 {
			parts = append(parts, "USB charging off")
		} else {
			parts = append(parts, fmt.Sprintf("USB charging until %d %%", u))
		}
	}
	h := plainMap(bat["Health"])
	if _, ok := h["health_pct"]; ok {
		parts = append(parts, fmt.Sprintf("health %d %%, %d cycles", num(h["health_pct"]), num(h["cycles"])))
	}
	if t, ok := h["temp"].(float64); ok {
		parts = append(parts, fmt.Sprintf("%.1f °C", t))
	}
	return strings.Join(parts, "; ")
}

func cmdCaps(args []string) error {
	v, err := cl.Property(client.IfSystem, "Capabilities")
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(plain(v))
	}
	caps := plainMap(v)
	for _, k := range sortedKeys(caps) {
		row(k, compact(caps[k]))
	}
	return nil
}

func cmdVersion(args []string) error {
	v, err := cl.Property(client.IfSystem, "Version")
	if err != nil {
		return err
	}
	fmt.Println("daemon", str(v.Value()))
	return nil
}

func cmdReload(args []string) error {
	return cl.Call(client.IfSystem, "Reload", nil)
}

// ---- mode ------------------------------------------------------------------

func cmdMode(args []string) error {
	if len(args) == 0 {
		mode, err := cl.Properties(client.IfMode)
		if err != nil {
			return err
		}
		if jsonOut {
			return printJSON(plain(mode))
		}
		fmt.Println(profileLine(mode))
		return nil
	}
	if args[0] == "cycle" {
		var next string
		if err := cl.Call(client.IfMode, "Cycle", []interface{}{&next}); err != nil {
			return err
		}
		fmt.Printf("%s (%s)\n", next, acerName[next])
		return nil
	}
	p := map[string]string{"eco": "low-power", "turbo": "performance"}[strings.ToLower(args[0])]
	if p == "" {
		p = strings.ToLower(args[0])
	}
	err := cl.Call(client.IfMode, "Set", nil, p)
	var ce *client.Error
	if p == "performance" && errors.As(err, &ce) && ce.Kind == "NotSupported" {
		return fmt.Errorf("%w (performance is Acer's Turbo; Acer's Performance mode is balanced-performance)", err)
	}
	return err
}

// ---- fans ------------------------------------------------------------------

func cmdFan(args []string) error {
	if len(args) == 0 {
		v, err := cl.Property(client.IfFans, "State")
		if err != nil {
			return err
		}
		if jsonOut {
			return printJSON(plain(v))
		}
		fmt.Println(fanLine(plainMap(v)))
		return nil
	}
	if len(args) != 2 {
		return usagef("fan <cpu|gpu|all> auto|max|<percent>")
	}
	id, what := args[0], strings.TrimSuffix(args[1], "%")
	if what == "auto" || what == "max" || what == "manual" {
		return cl.Call(client.IfFans, "SetMode", nil, id, what)
	}
	pct, err := strconv.ParseUint(what, 10, 32)
	if err != nil {
		return usagef("fan speed must be auto, max or a percentage, not %q", args[1])
	}
	return cl.Call(client.IfFans, "SetSpeed", nil, id, uint32(pct))
}

// ---- keyboard, battery, USB ------------------------------------------------

func cmdKbd(args []string) error {
	if len(args) == 0 {
		v, err := cl.Property(client.IfLighting, "Keyboard")
		if err != nil {
			return err
		}
		if jsonOut {
			return printJSON(plain(v))
		}
		k := plainMap(v)
		fmt.Printf("backlight %s, timeout %s\n", onOff(k["on"]), timeout(k["timeout_s"]))
		return nil
	}
	if len(args) != 2 || args[0] != "timeout" {
		return usagef("kbd timeout <seconds>")
	}
	s, err := strconv.ParseUint(strings.TrimSuffix(args[1], "s"), 10, 32)
	if err != nil {
		return usagef("timeout must be a number of seconds, not %q", args[1])
	}
	return cl.Call(client.IfLighting, "SetKeyboardTimeout", nil, uint32(s))
}

func cmdBattery(args []string) error {
	if len(args) == 0 {
		bat, err := cl.Properties(client.IfBattery)
		if err != nil {
			return err
		}
		if jsonOut {
			return printJSON(plain(bat))
		}
		caps, err := cl.Property(client.IfSystem, "Capabilities")
		if err != nil {
			return err
		}
		fmt.Println(batteryLine(plainMap(caps), bat))
		h := plainMap(bat["Health"])
		if _, ok := h["design"]; ok {
			fmt.Printf("capacity %.1f of %.1f Wh, %s", float64(num(h["full"]))/1e6, float64(num(h["design"]))/1e6, str(h["status"]))
			if v := num(h["voltage"]); v > 0 {
				fmt.Printf(", %.2f V, %d mA", float64(v)/1000, h["current"])
			}
			fmt.Println()
		}
		return nil
	}
	switch {
	case len(args) == 2 && args[0] == "limit" && (args[1] == "on" || args[1] == "off"):
		return cl.Call(client.IfBattery, "SetChargeLimit", nil, args[1] == "on")
	case len(args) == 2 && args[0] == "calibrate" && args[1] == "start":
		return cl.Call(client.IfBattery, "StartCalibration", nil)
	case len(args) == 2 && args[0] == "calibrate" && args[1] == "stop":
		return cl.Call(client.IfBattery, "StopCalibration", nil)
	}
	return usagef("battery [limit on|off | calibrate start|stop]")
}

func cmdUSB(args []string) error {
	if len(args) != 1 {
		return usagef("usb-charging <percent> (0 = off)")
	}
	a := strings.TrimSuffix(args[0], "%")
	if a == "off" {
		a = "0"
	}
	p, err := strconv.ParseUint(a, 10, 32)
	if err != nil {
		return usagef("usb-charging takes a percentage or off, not %q", args[0])
	}
	return cl.Call(client.IfBattery, "SetUsbCharging", nil, uint32(p))
}

// ---- scenarios -------------------------------------------------------------

func cmdScenario(args []string) error {
	if len(args) == 0 || args[0] == "list" {
		return scenarioList()
	}
	switch args[0] {
	case "apply", "delete":
		if len(args) != 2 {
			return usagef("scenario %s <name>", args[0])
		}
		return cl.Call(client.IfScenarios, map[string]string{"apply": "Apply", "delete": "Delete"}[args[0]], nil, args[1])
	case "save":
		return scenarioSave(args[1:])
	}
	return usagef("unknown scenario command %q", args[0])
}

func scenarioList() error {
	p, err := cl.Properties(client.IfScenarios)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(plain(p))
	}
	list, _ := plain(p["List"]).([]interface{})
	active := str(p["Active"].Value())
	if len(list) == 0 {
		fmt.Println("no scenarios (create one with: nitroctl scenario save <name> --from-current)")
	}
	for _, x := range list {
		sc, _ := x.(map[string]interface{})
		name := str(sc["name"])
		mark := " "
		if name == active {
			mark = "*"
		}
		var parts []string
		if m := str(sc["mode_ac"]); m != "" {
			parts = append(parts, "AC "+m)
		}
		if m := str(sc["mode_battery"]); m != "" {
			parts = append(parts, "battery "+m)
		}
		if f, ok := sc["fans"].(map[string]interface{}); ok {
			for _, id := range sortedKeys(f) {
				fm, _ := f[id].(map[string]interface{})
				m := str(fm["mode"])
				if m == "manual" {
					m = fmt.Sprintf("%d %%", num(fm["speed"]))
				}
				parts = append(parts, "fan "+id+" "+m)
			}
		}
		if t, ok := sc["kbd_timeout_s"]; ok {
			parts = append(parts, "kbd "+timeout(t))
		}
		if apps, ok := sc["apps"].([]string); ok && len(apps) > 0 {
			parts = append(parts, "apps "+strings.Join(apps, ","))
		}
		fmt.Printf("%s %-16s %s\n", mark, name, strings.Join(parts, "; "))
	}
	if b, _ := p["AppSync"].Value().(bool); !b && len(list) > 0 {
		fmt.Println("(App Sync is off)")
	}
	return nil
}

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

func scenarioSave(args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return usagef("scenario save <name> [options]")
	}
	name := args[0]
	fs := flag.NewFlagSet("scenario save", flag.ContinueOnError)
	fromCurrent := fs.Bool("from-current", false, "start from the current profile, fans and keyboard timeout")
	modeAC := fs.String("mode-ac", "", "profile on AC")
	modeBat := fs.String("mode-battery", "", "profile on battery")
	kbd := fs.Int("kbd-timeout", -1, "keyboard backlight timeout in seconds")
	var fans, apps multi
	fs.Var(&fans, "fan", "id=auto|max|PERCENT (repeatable; id may be all)")
	fs.Var(&apps, "app", "executable that triggers the scenario (repeatable)")
	if err := fs.Parse(args[1:]); err != nil {
		return usageError(err.Error())
	}
	if fs.NArg() > 0 {
		return usagef("unexpected %q", fs.Arg(0))
	}

	sc := map[string]dbus.Variant{"name": dbus.MakeVariant(name)}
	fanSet := map[string]map[string]dbus.Variant{}
	if *fromCurrent {
		mode, err := cl.Properties(client.IfMode)
		if err != nil {
			return err
		}
		key := "mode_ac"
		if str(mode["PowerSource"].Value()) == "battery" {
			key = "mode_battery"
		}
		sc[key] = mode["Current"]
		if st, err := cl.Property(client.IfFans, "State"); err == nil {
			for id, f := range plainMap(st) {
				fm, _ := f.(map[string]interface{})
				m := map[string]dbus.Variant{"mode": dbus.MakeVariant(str(fm["mode"]))}
				if str(fm["mode"]) == "manual" {
					m["speed"] = dbus.MakeVariant(uint32(num(fm["speed"])))
				}
				fanSet[id] = m
			}
		}
		if k, err := cl.Property(client.IfLighting, "Keyboard"); err == nil {
			if t, ok := plainMap(k)["timeout_s"]; ok {
				sc["kbd_timeout_s"] = dbus.MakeVariant(uint32(num(t)))
			}
		}
	}
	if *modeAC != "" {
		sc["mode_ac"] = dbus.MakeVariant(*modeAC)
	}
	if *modeBat != "" {
		sc["mode_battery"] = dbus.MakeVariant(*modeBat)
	}
	if *kbd >= 0 {
		sc["kbd_timeout_s"] = dbus.MakeVariant(uint32(*kbd))
	}
	for _, f := range fans {
		id, v, ok := strings.Cut(f, "=")
		if !ok {
			return usagef("--fan takes id=auto|max|PERCENT, not %q", f)
		}
		m := map[string]dbus.Variant{}
		switch v = strings.TrimSuffix(v, "%"); v {
		case "auto", "max":
			m["mode"] = dbus.MakeVariant(v)
		default:
			p, err := strconv.ParseUint(v, 10, 32)
			if err != nil {
				return usagef("--fan %s: speed must be auto, max or a percentage", f)
			}
			m["mode"] = dbus.MakeVariant("manual")
			m["speed"] = dbus.MakeVariant(uint32(p))
		}
		ids := []string{id}
		if id == "all" {
			st, err := cl.Property(client.IfFans, "State")
			if err != nil {
				return err
			}
			ids = sortedKeys(plainMap(st))
		}
		for _, x := range ids {
			fanSet[x] = m
		}
	}
	if len(fanSet) > 0 {
		sc["fans"] = dbus.MakeVariant(fanSet)
	}
	if len(apps) > 0 {
		sc["apps"] = dbus.MakeVariant([]string(apps))
	}
	if len(sc) == 1 {
		return usagef("an empty scenario does nothing; pass --from-current or some options")
	}
	return cl.Call(client.IfScenarios, "Save", nil, sc)
}

func cmdAppSync(args []string) error {
	if len(args) == 0 {
		v, err := cl.Property(client.IfScenarios, "AppSync")
		if err != nil {
			return err
		}
		fmt.Println("App Sync", onOff(v.Value()))
		return nil
	}
	if args[0] != "on" && args[0] != "off" {
		return usagef("appsync [on|off]")
	}
	return cl.Call(client.IfScenarios, "SetAppSync", nil, args[0] == "on")
}

// ---- monitor ---------------------------------------------------------------

func cmdMonitor(args []string) error {
	fs := flag.NewFlagSet("monitor", flag.ContinueOnError)
	iv := fs.Uint("i", 1000, "interval in ms (minimum 500)")
	count := fs.Int("n", 0, "stop after this many samples (0 = until Ctrl-C)")
	if err := fs.Parse(args); err != nil {
		return usageError(err.Error())
	}
	samples, stop, err := cl.Subscribe(uint32(*iv))
	if err != nil {
		return err
	}
	defer stop()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	for n := 0; *count == 0 || n < *count; n++ {
		select {
		case <-sig:
			return nil
		case s, ok := <-samples:
			if !ok {
				return nil
			}
			if jsonOut {
				b, _ := json.Marshal(plain(s))
				fmt.Println(string(b))
				continue
			}
			fmt.Println(sampleLine(plain(s).(map[string]interface{})))
		}
	}
	return nil
}

func sampleLine(s map[string]interface{}) string {
	parts := []string{time.Now().Format("15:04:05")}
	if c, ok := s["cpu"].(map[string]interface{}); ok {
		parts = append(parts, fmt.Sprintf("cpu %s %3d%% %4d MHz %s", temp(c["temp"]), num(c["usage"]), num(c["freq"]), watts(c["power"])))
	}
	if g, ok := s["gpu"].(map[string]interface{}); ok && g["active"] == false {
		parts = append(parts, "gpu off (suspended)")
	} else if ok {
		parts = append(parts, fmt.Sprintf("gpu %s %3d%% %4d MHz %s", temp(g["temp"]), num(g["usage"]), num(g["clock"]), watts(g["power"])))
	}
	if f, ok := s["fans"].(map[string]interface{}); ok {
		var rpm []string
		for _, id := range sortedKeys(f) {
			rpm = append(rpm, fmt.Sprintf("%d", num(f[id])))
		}
		parts = append(parts, "fans "+strings.Join(rpm, "/")+" rpm")
	}
	if r, ok := s["ram"].(map[string]interface{}); ok {
		parts = append(parts, fmt.Sprintf("ram %.1f/%.1f GiB", gib(r["used"]), gib(r["total"])))
	}
	if n, ok := s["net"].(map[string]interface{}); ok {
		parts = append(parts, fmt.Sprintf("net ↓%s ↑%s", rate(n["rx"]), rate(n["tx"])))
	}
	if b, ok := s["battery"].(map[string]interface{}); ok {
		parts = append(parts, fmt.Sprintf("bat %d%% %d mA", num(b["percent"]), b["current"]))
	}
	return strings.Join(parts, " | ")
}

// ---- formatting helpers ----------------------------------------------------

// plain unwraps variants recursively into maps, slices and scalars.
func plain(v interface{}) interface{} {
	switch t := v.(type) {
	case dbus.Variant:
		return plain(t.Value())
	case map[string]dbus.Variant:
		m := map[string]interface{}{}
		for k, x := range t {
			m[k] = plain(x)
		}
		return m
	case map[string]map[string]dbus.Variant:
		m := map[string]interface{}{}
		for k, x := range t {
			m[k] = plain(x)
		}
		return m
	case map[string]map[string]interface{}:
		return t
	case map[string]interface{}:
		m := map[string]interface{}{}
		for k, x := range t {
			m[k] = plain(x)
		}
		return m
	case []map[string]dbus.Variant:
		out := make([]interface{}, len(t))
		for i, x := range t {
			out[i] = plain(x)
		}
		return out
	}
	return v
}

func plainMap(v interface{}) map[string]interface{} {
	m, _ := plain(v).(map[string]interface{})
	if m == nil {
		m = map[string]interface{}{}
	}
	return m
}

func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func row(k, v string) { fmt.Printf("%-10s %s\n", k, v) }

func str(v interface{}) string { s, _ := v.(string); return s }

func num(v interface{}) uint64 {
	switch n := v.(type) {
	case uint32:
		return uint64(n)
	case uint64:
		return n
	case int32:
		return uint64(n)
	case int64:
		return uint64(n)
	case float64:
		return uint64(n)
	}
	return 0
}

func onOff(v interface{}) string {
	if b, _ := v.(bool); b {
		return "on"
	}
	return "off"
}

func timeout(v interface{}) string {
	if v == nil {
		return "?"
	}
	if num(v) == 0 {
		return "never"
	}
	return fmt.Sprintf("%d s", num(v))
}

func temp(v interface{}) string {
	if t, ok := v.(float64); ok {
		return fmt.Sprintf("%3.0f°C", t)
	}
	return "  -°C"
}

func watts(v interface{}) string {
	if w, ok := v.(float64); ok {
		return fmt.Sprintf("%5.1f W", w)
	}
	return "    - W"
}

func gib(v interface{}) float64 { return float64(num(v)) / (1 << 30) }

func rate(v interface{}) string {
	b := float64(num(v))
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MiB/s", b/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0f KiB/s", b/(1<<10))
	}
	return fmt.Sprintf("%.0f B/s", b)
}

func compact(v interface{}) string {
	switch t := v.(type) {
	case map[string]interface{}:
		var parts []string
		for _, k := range sortedKeys(t) {
			parts = append(parts, k+"="+compact(t[k]))
		}
		return strings.Join(parts, " ")
	case []string:
		return strings.Join(t, ",")
	}
	return fmt.Sprint(v)
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
