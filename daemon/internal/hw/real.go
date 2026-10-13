package hw

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const (
	extrasDir     = "/sys/devices/platform/acer-wmi-extras"
	batteryDrvDir = "/sys/bus/wmi/drivers/acer-wmi-battery"
	ppClassGlob   = "/sys/class/platform-profile/*/profile"
	ppLegacy      = "/sys/firmware/acpi/platform_profile"
)

// Probe detects every backend on the running machine.
func Probe() *Hardware {
	h := &Hardware{Model: probeModel()}
	if p := probeProfiles(); p != nil {
		h.Profiles = p
	}
	if f := probeFans(); f != nil {
		h.Fans = f
		h.Thermal = f
	}
	if exists(filepath.Join(extrasDir, "kbd_backlight_timeout")) {
		h.Keyboard = &extrasKeyboard{}
	}
	if exists(filepath.Join(extrasDir, "usb_charging")) {
		h.USB = &extrasUSB{}
	}
	if b := probeBatteryControl(); b != nil {
		h.BatteryCtl = b
	}
	h.Battery = probeBatteryInfo()
	if g := probeNVML(); g != nil {
		h.GPU = gateRuntimePM(g)
	} else if g := probeGPU(); g != nil {
		h.GPU = gateRuntimePM(g) // nvidia-smi fallback
	}
	h.NitroKey = probeNitroKey()
	slog.Info("probe done", "caps", h.Capabilities())
	return h
}

// ---- model / DMI -----------------------------------------------------------

func probeModel() map[string]interface{} {
	m := map[string]interface{}{}
	for key, file := range map[string]string{"vendor": "sys_vendor", "product": "product_name", "bios": "bios_version"} {
		if v, err := readStr("/sys/class/dmi/id/" + file); err == nil {
			m[key] = v
		}
	}
	if v, err := readStr("/proc/sys/kernel/osrelease"); err == nil {
		m["kernel"] = v
	}
	mods := map[string]string{}
	for _, name := range []string{"acer_wmi", "acer_wmi_extras", "acer_wmi_battery"} {
		dir := "/sys/module/" + name
		if !exists(dir) {
			mods[name] = "absent"
			continue
		}
		if v, err := readStr(dir + "/version"); err == nil {
			mods[name] = v
		} else {
			mods[name] = "loaded"
		}
	}
	m["modules"] = mods
	if v, err := readStr("/sys/module/acer_wmi/parameters/force_caps"); err == nil {
		if n, err := strconv.ParseInt(v, 0, 64); err == nil && n >= 0 {
			m["acer_wmi_caps"] = uint32(n)
		}
	}
	return m
}

// ---- thermal profiles ------------------------------------------------------

type sysfsProfiles struct {
	path         string
	supported    []string
	fromFirmware bool
}

func probeProfiles() *sysfsProfiles {
	var path string
	if m, _ := filepath.Glob(ppClassGlob); len(m) > 0 {
		path = m[0]
	} else if exists(ppLegacy) {
		path = ppLegacy
	} else {
		return nil
	}
	p := &sysfsProfiles{path: path}
	if s, err := readStr(filepath.Join(extrasDir, "supported_profiles")); err == nil && s != "" {
		p.supported = strings.Fields(s)
		p.fromFirmware = true
		return p
	}
	choices := filepath.Join(filepath.Dir(path), "choices")
	if path == ppLegacy {
		choices = ppLegacy + "_choices"
	}
	if s, err := readStr(choices); err == nil {
		p.supported = strings.Fields(s)
		slog.Warn("acer-wmi-extras absent: profile list comes from the driver, not the firmware", "profiles", p.supported)
		return p
	}
	return nil
}

// Watch polls the class attribute and the legacy /sys/firmware/acpi file:
// writes through either path notify at least one of them. Every poll() wake
// re-reads the attribute (a WMI call, so under wmiMu) to re-arm it.
func (p *sysfsProfiles) Watch(ctx context.Context) <-chan struct{} {
	ch := make(chan struct{}, 1)
	var fds []unix.PollFd
	for _, f := range []string{p.path, ppLegacy} {
		if !exists(f) {
			continue
		}
		fd, err := unix.Open(f, unix.O_RDONLY|unix.O_CLOEXEC, 0)
		if err != nil {
			continue
		}
		fds = append(fds, unix.PollFd{Fd: int32(fd), Events: unix.POLLPRI | unix.POLLERR})
	}
	if len(fds) == 0 {
		close(ch)
		return ch
	}
	rearm := func() {
		buf := make([]byte, 64)
		wmiMu.Lock()
		for _, fd := range fds {
			_, _ = unix.Pread(int(fd.Fd), buf, 0)
		}
		wmiMu.Unlock()
	}
	rearm()
	go func() {
		defer func() {
			for _, fd := range fds {
				_ = unix.Close(int(fd.Fd))
			}
			close(ch)
		}()
		for ctx.Err() == nil {
			n, err := unix.Poll(fds, 1000)
			if err != nil && err != unix.EINTR {
				slog.Warn("platform_profile poll failed", "err", err)
				return
			}
			if n <= 0 {
				continue
			}
			for i := range fds {
				fds[i].Revents = 0
			}
			rearm()
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}()
	return ch
}

func (p *sysfsProfiles) Supported() []string      { return p.supported }
func (p *sysfsProfiles) FromFirmware() bool       { return p.fromFirmware }
func (p *sysfsProfiles) Current() (string, error) { return readStr(p.path) }
func (p *sysfsProfiles) Set(profile string) error { return writeStr(p.path, profile) }

// ---- fans and temperatures (hwmon "acer") ----------------------------------

type acerHwmon struct {
	dir string
	ids []string // in channel order: cpu=1, gpu=2
	pwm bool
}

var fanChannel = map[string]int{"cpu": 1, "gpu": 2}
var tempChannel = map[string]int{"cpu": 1, "gpu": 2, "external": 3}

func probeFans() *acerHwmon {
	dir := hwmonByName("acer")
	if dir == "" {
		return nil
	}
	f := &acerHwmon{dir: dir}
	for _, id := range []string{"cpu", "gpu"} {
		if exists(filepath.Join(dir, fmt.Sprintf("fan%d_input", fanChannel[id]))) {
			f.ids = append(f.ids, id)
		}
	}
	f.pwm = exists(filepath.Join(dir, "pwm1_enable"))
	return f
}

func (f *acerHwmon) IDs() []string      { return f.ids }
func (f *acerHwmon) Controllable() bool { return f.pwm }
func (f *acerHwmon) Range(string) Range { return Range{0, 100} }

func (f *acerHwmon) RPM() (map[string]uint32, error) {
	out := map[string]uint32{}
	for _, id := range f.ids {
		if rpm, err := readUint(filepath.Join(f.dir, fmt.Sprintf("fan%d_input", fanChannel[id]))); err == nil {
			out[id] = uint32(rpm)
		}
	}
	return out, nil
}

func (f *acerHwmon) State() (map[string]FanState, error) {
	out := map[string]FanState{}
	for _, id := range f.ids {
		ch := fanChannel[id]
		st := FanState{Mode: "auto"}
		if rpm, err := readUint(filepath.Join(f.dir, fmt.Sprintf("fan%d_input", ch))); err == nil {
			st.RPM = uint32(rpm)
		}
		if f.pwm {
			switch en, _ := readInt(filepath.Join(f.dir, fmt.Sprintf("pwm%d_enable", ch))); en {
			case 0:
				st.Mode = "max"
			case 1:
				st.Mode = "manual"
			default:
				st.Mode = "auto"
			}
			if pwm, err := readUint(filepath.Join(f.dir, fmt.Sprintf("pwm%d", ch))); err == nil {
				st.Speed = uint32((pwm*100 + 127) / 255)
			}
		}
		out[id] = st
	}
	return out, nil
}

func (f *acerHwmon) targets(id string) ([]string, error) {
	if id == "all" {
		return f.ids, nil
	}
	for _, x := range f.ids {
		if x == id {
			return []string{id}, nil
		}
	}
	return nil, fmt.Errorf("unknown fan %q", id)
}

func (f *acerHwmon) SetMode(id, mode string) error {
	if !f.pwm {
		return ErrAbsent
	}
	enable := map[string]string{"max": "0", "manual": "1", "auto": "2"}[mode]
	if enable == "" {
		return fmt.Errorf("unknown fan mode %q", mode)
	}
	ids, err := f.targets(id)
	if err != nil {
		return err
	}
	for _, x := range ids {
		if err := writeStr(filepath.Join(f.dir, fmt.Sprintf("pwm%d_enable", fanChannel[x])), enable); err != nil {
			return err
		}
	}
	return nil
}

func (f *acerHwmon) SetSpeed(id string, percent uint32) error {
	if !f.pwm {
		return ErrAbsent
	}
	ids, err := f.targets(id)
	if err != nil {
		return err
	}
	pwm := strconv.FormatUint(uint64((percent*255+50)/100), 10)
	for _, x := range ids {
		ch := fanChannel[x]
		if en, _ := readInt(filepath.Join(f.dir, fmt.Sprintf("pwm%d_enable", ch))); en != 1 {
			if err := writeStr(filepath.Join(f.dir, fmt.Sprintf("pwm%d_enable", ch)), "1"); err != nil {
				return err
			}
		}
		if err := writeStr(filepath.Join(f.dir, fmt.Sprintf("pwm%d", ch)), pwm); err != nil {
			return err
		}
	}
	return nil
}

func (f *acerHwmon) Temps() (map[string]float64, error) {
	out := map[string]float64{}
	for id, ch := range tempChannel {
		if v, err := readInt(filepath.Join(f.dir, fmt.Sprintf("temp%d_input", ch))); err == nil {
			out[id] = float64(v) / 1000
		}
	}
	return out, nil
}

// ---- acer-wmi-extras: keyboard timeout, USB charging -----------------------

type extrasKeyboard struct{}

func (extrasKeyboard) Timeouts() []uint32 { return []uint32{0, 30} }
func (extrasKeyboard) Timeout() (uint32, error) {
	v, err := readUint(filepath.Join(extrasDir, "kbd_backlight_timeout"))
	return uint32(v), err
}
func (extrasKeyboard) SetTimeout(s uint32) error {
	return writeStr(filepath.Join(extrasDir, "kbd_backlight_timeout"), strconv.FormatUint(uint64(s), 10))
}
func (extrasKeyboard) On() (bool, error) {
	v, err := readInt(filepath.Join(extrasDir, "kbd_backlight_state"))
	return v != 0, err
}

type extrasUSB struct{}

func (extrasUSB) Values() []uint32 { return []uint32{0, 10, 20, 30} }
func (extrasUSB) Get() (uint32, error) {
	v, err := readUint(filepath.Join(extrasDir, "usb_charging"))
	return uint32(v), err
}
func (extrasUSB) Set(p uint32) error {
	return writeStr(filepath.Join(extrasDir, "usb_charging"), strconv.FormatUint(uint64(p), 10))
}

// ---- acer-wmi-battery: charge limit, calibration ---------------------------

type wmiBattery struct {
	limit, calib bool
	upower       *upowerLimit // nil: no power_supply extension, write health_mode
}

func probeBatteryControl() *wmiBattery {
	if !exists(batteryDrvDir) {
		return nil
	}
	b := &wmiBattery{}
	// The driver reports -1 when the firmware lacks the function.
	if v, err := readInt(filepath.Join(batteryDrvDir, "health_mode")); err == nil && v >= 0 {
		b.limit = true
	}
	if v, err := readInt(filepath.Join(batteryDrvDir, "calibration_mode")); err == nil && v >= 0 {
		b.calib = true
	}
	if !b.limit && !b.calib {
		return nil
	}
	if b.limit {
		if b.upower = probeUPowerLimit(); b.upower != nil {
			slog.Info("charge limit through UPower", "battery", b.upower.native)
		}
	}
	return b
}

func (b *wmiBattery) HasChargeLimit() bool { return b.limit }
func (b *wmiBattery) HasCalibration() bool { return b.calib }
func (b *wmiBattery) ChargeLimit() (bool, error) {
	v, err := readInt(filepath.Join(batteryDrvDir, "health_mode"))
	return v > 0, err
}
func (b *wmiBattery) SetChargeLimit(on bool) error {
	if b.upower != nil {
		if DryRun {
			slog.Info("dry-run: skip UPower EnableChargeThreshold", "on", on)
			return nil
		}
		err := b.upower.set(on)
		if err == nil {
			return nil
		}
		slog.Warn("UPower charge threshold failed, writing health_mode", "err", err)
	}
	return writeStr(filepath.Join(batteryDrvDir, "health_mode"), boolStr(on))
}
func (b *wmiBattery) Calibrating() (bool, error) {
	v, err := readInt(filepath.Join(batteryDrvDir, "calibration_mode"))
	return v > 0, err
}
func (b *wmiBattery) SetCalibrating(on bool) error {
	return writeStr(filepath.Join(batteryDrvDir, "calibration_mode"), boolStr(on))
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// ---- power_supply + hwmon "acer_battery" -----------------------------------

type psBattery struct {
	ac, bat, hwmon string

	mu       sync.Mutex
	temp     float64   // acer_battery temp1: a WMI call (~5 ms), and it moves slowly
	tempRead time.Time // so it is read at most every batteryTempEvery
}

const batteryTempEvery = 30 * time.Second

func probeBatteryInfo() *psBattery {
	return &psBattery{
		ac:    powerSupplyByType("Mains"),
		bat:   powerSupplyByType("Battery"),
		hwmon: hwmonByName("acer_battery"),
	}
}

func (p *psBattery) HasSensors() bool { return p.hwmon != "" }

func (p *psBattery) Power() (Power, error) {
	pw := Power{Source: "ac", Percent: 100}
	if p.ac != "" {
		if on, err := readInt(filepath.Join(p.ac, "online")); err == nil && on == 0 {
			pw.Source = "battery"
		}
	}
	if p.bat != "" {
		if c, err := readUint(filepath.Join(p.bat, "capacity")); err == nil {
			pw.Percent = uint32(c)
		}
	}
	return pw, nil
}

// batteryTemp returns the acer_battery temperature, re-read at most every
// batteryTempEvery; NoTemp without the sensor.
func (p *psBattery) batteryTemp() float64 {
	if p.hwmon == "" {
		return NoTemp
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tempRead.IsZero() || time.Since(p.tempRead) >= batteryTempEvery {
		if v, err := readInt(filepath.Join(p.hwmon, "temp1_input")); err == nil {
			p.temp, p.tempRead = float64(v)/1000, time.Now()
		} else if p.tempRead.IsZero() {
			return NoTemp
		}
	}
	return p.temp
}

func (p *psBattery) Health() (Health, error) {
	h := Health{Temp: NoTemp}
	if p.bat == "" {
		return h, ErrAbsent
	}
	if v, err := readUint(filepath.Join(p.bat, "cycle_count")); err == nil {
		h.Cycles = uint32(v)
	}
	h.Full, _ = readUint(filepath.Join(p.bat, "energy_full"))
	h.Design, _ = readUint(filepath.Join(p.bat, "energy_full_design"))
	if h.Full == 0 {
		// charge_* devices (the ANV15-52) report µAh: convert at the
		// nominal voltage, as UPower does (3 733 000 µAh × 15.4 V = 57.5 Wh).
		full, _ := readUint(filepath.Join(p.bat, "charge_full"))
		design, _ := readUint(filepath.Join(p.bat, "charge_full_design"))
		uv, _ := readUint(filepath.Join(p.bat, "voltage_min_design"))
		h.Full, h.Design = full*uv/1e6, design*uv/1e6
	}
	h.Status, _ = readStr(filepath.Join(p.bat, "status"))
	// Voltage and current: power_supply is a cached ACPI read; the
	// acer_battery hwmon (SBS over WMI) only when power_supply lacks them.
	// power_supply gives the magnitude; the sign follows the status, as in
	// the signed SBS current (negative while discharging).
	if v, err := readInt(filepath.Join(p.bat, "voltage_now")); err == nil {
		h.Voltage = uint32(v / 1000)
	} else if p.hwmon != "" {
		if v, err := readInt(filepath.Join(p.hwmon, "in0_input")); err == nil {
			h.Voltage = uint32(v)
		}
	}
	if v, err := readInt(filepath.Join(p.bat, "current_now")); err == nil {
		h.Current = int32(v / 1000)
		if h.Status == "Discharging" && h.Current > 0 {
			h.Current = -h.Current
		}
	} else if p.hwmon != "" {
		if v, err := readInt(filepath.Join(p.hwmon, "curr1_input")); err == nil {
			h.Current = int32(v)
		}
	}
	h.Temp = p.batteryTemp()
	return h, nil
}

// ---- GPU via nvidia-smi (NVML comes later) ---------------------------------

type smiGPU struct{ bin string }

func probeGPU() *smiGPU {
	bin, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return nil
	}
	g := &smiGPU{bin: bin}
	if _, err := g.Sample(); err != nil {
		slog.Warn("nvidia-smi present but not usable", "err", err)
		return nil
	}
	return g
}

func (g *smiGPU) Sample() (GPUSample, error) {
	out, err := exec.Command(g.bin, "--query-gpu=temperature.gpu,utilization.gpu,clocks.gr,clocks.mem,power.draw",
		"--format=csv,noheader,nounits").Output()
	if err != nil {
		return GPUSample{}, err
	}
	f := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(f) < 5 {
		return GPUSample{}, fmt.Errorf("unexpected nvidia-smi output %q", out)
	}
	num := func(s string) float64 { v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64); return v }
	return GPUSample{Temp: num(f[0]), Usage: uint32(num(f[1])), Clock: uint32(num(f[2])),
		MemClock: uint32(num(f[3])), Power: num(f[4])}, nil
}

// ---- Nitro key -------------------------------------------------------------

// probeNitroKey checks that the internal keyboard exists (see nitrokey.go).
func probeNitroKey() bool { return nitroKeyDevice() != "" }
