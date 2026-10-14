package hw

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// simModel is what Simulate gives one model.
type simModel struct {
	product, bios string
	profiles      []string
	kbdTimeout    bool // single-colour backlight timeout (acer-wmi-extras)
	lights        []LightDevice
	initial       map[string]LightState // what the readable devices report at start
}

var (
	predatorProfiles = []string{"low-power", "quiet", "balanced", "balanced-performance", "performance"}
	ecEffects        = []string{"static", "breathing", "neon", "wave", "shifting", "zoom", "meteor", "twinkling"}
	lightbarEffects  = []string{"static", "breathing", "neon", "wave", "twinkling"}
	lr               = []string{"left", "right"}
)

// simModels are the models Simulate knows. The ANV15-52 values were read
// from the real machine. The two Predators follow what public sources report
// (docs/firmware-wmi.md §11), not a machine we could test:
//   - PHN16-72: 4-zone keyboard (methods 6/20, target 1) and logo (method 20,
//     target 2, the light bar channel of other models);
//   - PH16-71: USB keyboard that cannot be read back, and a light bar
//     (method 20, target 2); no single-colour backlight timeout.
var simModels = map[string]simModel{
	"ANV15-52": {
		product: "Nitro ANV15-52", bios: "V1.31",
		profiles:   []string{"low-power", "quiet", "balanced", "balanced-performance"},
		kbdTimeout: true,
	},
	"PHN16-72": {
		product: "Predator PHN16-72", bios: "V1.31",
		profiles: predatorProfiles, kbdTimeout: true,
		lights: []LightDevice{
			{ID: "keyboard", Effects: ecEffects, Zones: 4, Speed: Range{1, 9}, Directions: lr, Readback: true},
			{ID: "logo", Effects: lightbarEffects, Speed: Range{1, 5}, Directions: lr, Readback: true},
		},
		initial: map[string]LightState{
			"keyboard": {Effect: "static", Brightness: 80, Colors: []string{"#00a2ff", "#3d7bff", "#7a54ff", "#b833ff"}},
			"logo":     {Effect: "static", Brightness: 100, Colors: []string{"#00a2ff"}},
		},
	},
	"PH16-71": {
		product: "Predator PH16-71", bios: "V1.31",
		profiles: predatorProfiles,
		lights: []LightDevice{
			{ID: "keyboard", Effects: []string{"static", "breathing", "wave", "snake", "ripple", "rainbow", "rain",
				"lightning", "spot", "stars", "fireball", "snow", "heartbeat"}, Speed: Range{1, 9}, Directions: lr},
			{ID: "lightbar", Effects: lightbarEffects, Speed: Range{1, 5}, Directions: lr, Readback: true},
		},
		initial: map[string]LightState{
			"lightbar": {Effect: "breathing", Speed: 3, Brightness: 100, Colors: []string{"#00a2ff"}},
		},
	},
}

// SimModels lists the model names Simulate accepts.
func SimModels() []string {
	out := make([]string, 0, len(simModels))
	for k := range simModels {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Simulate returns an in-memory Hardware with the capabilities of a known
// model (simModels), for developing clients without the hardware.
func Simulate(model string) (*Hardware, error) {
	m, ok := simModels[model]
	if !ok {
		return nil, fmt.Errorf("unknown simulated model %q (%s)", model, strings.Join(SimModels(), ", "))
	}
	s := &sim{
		profiles: m.profiles,
		profile:  "balanced",
		fans:     map[string]FanState{"cpu": {RPM: 2100, Mode: "auto"}, "gpu": {RPM: 0, Mode: "auto"}},
		timeout:  30, usb: 30, limit: true,
		power:  Power{Source: "ac", Percent: 80},
		lights: map[string]LightState{},
	}
	for id, st := range m.initial {
		s.lights[id] = st
	}
	h := &Hardware{
		Model: map[string]interface{}{"vendor": "Acer", "product": m.product + " (simulated)", "bios": m.bios,
			"kernel": "simulated", "modules": map[string]string{"acer_wmi": "sim", "acer_wmi_extras": "sim", "acer_wmi_battery": "sim"}},
		Profiles: s, Fans: s, Thermal: s, USB: simUSB{s}, BatteryCtl: s, Battery: s, GPU: s, SenseKey: true,
	}
	if m.kbdTimeout {
		h.Keyboard = s
	}
	if len(m.lights) > 0 {
		h.Lights = simLights{s, m.lights}
	}
	return h, nil
}

type sim struct {
	mu       sync.Mutex
	profiles []string
	profile  string
	fans     map[string]FanState
	timeout  uint32
	on       bool
	usb      uint32
	limit    bool
	calib    bool
	power    Power
	lights   map[string]LightState
}

func (s *sim) Supported() []string { return s.profiles }
func (s *sim) FromFirmware() bool  { return true }
func (s *sim) Current() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.profile, nil
}
func (s *sim) Set(p string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profile = p
	return nil
}

func (s *sim) IDs() []string      { return []string{"cpu", "gpu"} }
func (s *sim) Controllable() bool { return true }
func (s *sim) Range(string) Range { return Range{0, 100} }
func (s *sim) State() (map[string]FanState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]FanState{}
	for k, v := range s.fans {
		out[k] = v
	}
	return out, nil
}
func (s *sim) RPM() (map[string]uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]uint32{}
	for k, v := range s.fans {
		out[k] = v.RPM
	}
	return out, nil
}
func (s *sim) each(id string, f func(st *FanState)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "all" {
		for k := range s.fans {
			st := s.fans[k]
			f(&st)
			s.fans[k] = st
		}
		return nil
	}
	st, ok := s.fans[id]
	if !ok {
		return fmt.Errorf("unknown fan %q", id)
	}
	f(&st)
	s.fans[id] = st
	return nil
}
func (s *sim) SetMode(id, mode string) error {
	return s.each(id, func(st *FanState) {
		st.Mode = mode
		switch mode {
		case "max":
			st.RPM = 5800
		case "auto":
			st.RPM = 2100
		}
	})
}
func (s *sim) SetSpeed(id string, pct uint32) error {
	return s.each(id, func(st *FanState) { st.Mode = "manual"; st.Speed = pct; st.RPM = 1200 + pct*46 })
}
func (s *sim) Temps() (map[string]float64, error) {
	return map[string]float64{"cpu": 52, "gpu": 0, "external": 43}, nil
}

func (s *sim) Timeouts() []uint32         { return []uint32{0, 30} }
func (s *sim) Timeout() (uint32, error)   { return s.timeout, nil }
func (s *sim) SetTimeout(t uint32) error  { s.timeout = t; return nil }
func (s *sim) On() (bool, error)          { return s.on, nil }
func (s *sim) Values() []uint32           { return []uint32{0, 10, 20, 30} }
func (s *sim) Get() (uint32, error)       { return s.usb, nil }
func (s *sim) SetUSB(p uint32) error      { s.usb = p; return nil }
func (s *sim) HasChargeLimit() bool       { return true }
func (s *sim) HasCalibration() bool       { return true }
func (s *sim) ChargeLimit() (bool, error) { return s.limit, nil }
func (s *sim) SetChargeLimit(b bool) error {
	s.limit = b
	return nil
}
func (s *sim) Calibrating() (bool, error)  { return s.calib, nil }
func (s *sim) SetCalibrating(b bool) error { s.calib = b; return nil }
func (s *sim) HasSensors() bool            { return true }
func (s *sim) Power() (Power, error)       { return s.power, nil }
func (s *sim) Health() (Health, error) {
	return Health{Cycles: 41, Full: 54_000_000, Design: 57_480_000, Status: "Not charging",
		Voltage: 16440, Current: 0, Temp: 25.6}, nil
}
func (s *sim) Sample() (GPUSample, error) {
	return GPUSample{Temp: 41, Usage: 3, Clock: 210, MemClock: 405, Power: 7.5}, nil
}

// Set implements USBCharging.Set; the Profiles.Set method above has the same
// Go name, so the USB variant is exposed through this wrapper type.
type simUSB struct{ *sim }

func (u simUSB) Set(p uint32) error { return u.SetUSB(p) }
func (u simUSB) Values() []uint32   { return u.sim.Values() }
func (u simUSB) Get() (uint32, error) {
	return u.sim.Get()
}

// simLights is the Lighting of a simulated model: it keeps what was set and
// answers like the firmware, including the USB keyboard that cannot be read.
type simLights struct {
	*sim
	devs []LightDevice
}

func (l simLights) Devices() []LightDevice { return l.devs }

func (l simLights) device(id string) (LightDevice, error) {
	for _, d := range l.devs {
		if d.ID == id {
			return d, nil
		}
	}
	return LightDevice{}, fmt.Errorf("unknown lighting device %q", id)
}

func (l simLights) Get(id string) (LightState, error) {
	d, err := l.device(id)
	if err != nil {
		return LightState{}, err
	}
	if !d.Readback {
		return LightState{}, ErrNoReadback
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	st, ok := l.lights[id]
	if !ok {
		return d.Default(), nil
	}
	return st, nil
}

func (l simLights) Set(id string, st LightState) error {
	d, err := l.device(id)
	if err != nil {
		return err
	}
	if st, err = d.Normalize(st); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lights[id] = st
	return nil
}
