package hw

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/models/*/expected.json from the current code")

// TestModels runs Probe against each fixture tree in testdata/models (a copy,
// so writes stay in the test), reads every backend it found, makes one change
// through each and compares all of it, including the files written, with the
// model's expected.json. A change meant for one model that alters what a
// known model gets fails here; accept intended changes with
//
//	go test ./internal/hw -run TestModels -update
//
// and review the diff of expected.json.
func TestModels(t *testing.T) {
	roots, _ := filepath.Glob("testdata/models/*/root")
	if len(roots) == 0 {
		t.Fatal("no fixtures in testdata/models")
	}
	for _, src := range roots {
		model := filepath.Base(filepath.Dir(src))
		t.Run(model, func(t *testing.T) {
			dir := t.TempDir()
			copyTree(t, src, dir)
			before := snapshot(t, dir)
			Root = dir
			defer func() { Root = "" }()

			got := exercise(Probe())
			got["writes"] = changed(before, snapshot(t, dir))
			b, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			b = append(b, '\n')

			golden := filepath.Join(filepath.Dir(src), "expected.json")
			if *update {
				if err := os.WriteFile(golden, b, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (create it with -update)", err)
			}
			if string(want) != string(b) {
				t.Errorf("%s differs from what the code gives now (-update accepts it):\n%s", golden, lineDiff(string(want), string(b)))
			}
		})
	}
}

// TestModelsKbdDMI checks each fixture made from a real machine against that
// machine's DMI type 170 table (dmi-170.txt, as dmidecode -t 170 -u prints it)
// with the rule acer-wmi-extras applies: a table that lists the keyboard
// backlight (code 0x84) as absent means no kbd_backlight_* attributes, even
// though the WMI keyboard call answers status 0 there. With the backlight
// present the attributes depend on that WMI answer, which a fixture does not
// keep, so only the table itself is checked.
func TestModelsKbdDMI(t *testing.T) {
	tables, _ := filepath.Glob("testdata/models/*/dmi-170.txt")
	if len(tables) == 0 {
		t.Fatal("no dmi-170.txt in testdata/models")
	}
	for _, f := range tables {
		dir := filepath.Dir(f)
		t.Run(filepath.Base(dir), func(t *testing.T) {
			text, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			listed, present, err := kbdBacklightInDMI(string(text))
			if err != nil {
				t.Fatalf("%s: %v", f, err)
			}
			attrs, _ := filepath.Glob(filepath.Join(dir, "root/sys/devices/platform/acer-wmi-extras/kbd_backlight_*"))
			if listed && !present && len(attrs) > 0 {
				t.Errorf("the DMI table lists no keyboard backlight, so acer-wmi-extras hides %v", attrs)
			}
		})
	}
}

// kbdBacklightInDMI decodes a DMI type 0xAA structure as acer-wmi-extras does:
// after the header, five u16 bitmaps of the functions present (communication,
// application, media, display, others), then { code, 2, u16 bit } entries.
func kbdBacklightInDMI(dmidecode string) (listed, present bool, err error) {
	var b []byte
	in := false
	for _, line := range strings.Split(dmidecode, "\n") {
		if strings.TrimSpace(line) == "Header and Data:" {
			in = true
			continue
		}
		if !in {
			continue
		}
		if !strings.HasPrefix(line, "\t\t") {
			break
		}
		for _, tok := range strings.Fields(line) {
			v, err := strconv.ParseUint(tok, 16, 8)
			if err != nil {
				return false, false, fmt.Errorf("byte %q: %v", tok, err)
			}
			b = append(b, byte(v))
		}
	}
	if len(b) < 14 || b[0] != 0xAA || int(b[1]) != len(b) {
		return false, false, fmt.Errorf("not a complete type 170 structure (%d bytes)", len(b))
	}
	le16 := func(i int) uint16 { return uint16(b[i]) | uint16(b[i+1])<<8 }
	for i := 14; i+4 <= len(b); i += 4 {
		if b[i+1] != 2 {
			return false, false, fmt.Errorf("entry at byte %d has size %d, not 2", i, b[i+1])
		}
		if b[i] == 0x84 {
			return true, le16(12)&le16(i+2) != 0, nil
		}
	}
	return false, false, nil
}

// exercise reads every backend of h, then changes each once, in a fixed order.
func exercise(h *Hardware) map[string]interface{} {
	out := map[string]interface{}{
		"capabilities": h.Capabilities(),
		"model":        h.Model,
		"setup":        h.Setup.Items(),
	}
	read := map[string]interface{}{}
	var actions []string
	do := func(what string, err error) {
		if err != nil {
			what += ": " + err.Error()
		}
		actions = append(actions, what)
	}

	if h.Profiles != nil {
		read["profile"], _ = h.Profiles.Current()
		sup := h.Profiles.Supported()
		p := sup[len(sup)-1]
		do("profile "+p, h.Profiles.Set(p))
	}
	if h.Fans != nil {
		read["fans"], _ = h.Fans.State()
		if h.Fans.Controllable() {
			do("fans all max", h.Fans.SetMode("all", "max"))
			do("fan cpu 40%", h.Fans.SetSpeed("cpu", 40))
		}
	}
	if h.Thermal != nil {
		read["temps"], _ = h.Thermal.Temps()
	}
	if h.Keyboard != nil {
		t, _ := h.Keyboard.Timeout()
		on, _ := h.Keyboard.On()
		read["keyboard"] = map[string]interface{}{"timeout": t, "on": on}
		do("keyboard timeout 0", h.Keyboard.SetTimeout(0))
	}
	if h.USB != nil {
		read["usb_charging"], _ = h.USB.Get()
		do("usb charging 20%", h.USB.Set(20))
	}
	if h.BatteryCtl != nil {
		limit, _ := h.BatteryCtl.ChargeLimit()
		calib, _ := h.BatteryCtl.Calibrating()
		read["battery_control"] = map[string]interface{}{"charge_limit": limit, "calibrating": calib}
		if h.BatteryCtl.HasChargeLimit() {
			do(fmt.Sprintf("charge limit %v", !limit), h.BatteryCtl.SetChargeLimit(!limit))
		}
		if h.BatteryCtl.HasCalibration() {
			do("calibration on", h.BatteryCtl.SetCalibrating(true))
		}
	}
	if h.Battery != nil {
		read["power"], _ = h.Battery.Power()
		hl, err := h.Battery.Health()
		if err == nil {
			health := map[string]interface{}{"cycles": hl.Cycles, "full_uwh": hl.Full, "design_uwh": hl.Design,
				"status": hl.Status, "voltage_mv": hl.Voltage, "current_ma": hl.Current}
			if !math.IsNaN(hl.Temp) {
				health["temp"] = hl.Temp
			}
			read["health"] = health
		}
	}
	for _, it := range h.Setup.Items() {
		if it.State == "needed" && it.ID != "signing_key" {
			_, err := h.Setup.Apply(it.ID)
			do("setup "+it.ID, err)
		}
	}
	out["read"] = read
	out["actions"] = actions
	return out
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// snapshot maps each file of the tree (real path) to its content.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files[strings.TrimPrefix(p, dir)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// changed lists the files that are new or different after the actions.
func changed(before, after map[string]string) map[string]string {
	out := map[string]string{}
	for p, v := range after {
		if old, ok := before[p]; !ok || old != v {
			out[p] = v
		}
	}
	return out
}

// lineDiff prints the lines that differ, with their line numbers.
func lineDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	var b strings.Builder
	for i := 0; i < len(w) || i < len(g); i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			fmt.Fprintf(&b, "%4d - %s\n     + %s\n", i+1, wl, gl)
		}
	}
	return b.String()
}
