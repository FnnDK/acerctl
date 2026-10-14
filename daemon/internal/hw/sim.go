package hw

import (
	"fmt"
	"sync"
)

// Simulate returns an in-memory Hardware with the capabilities of a known
// model, for developing clients without the hardware. Only "ANV15-52" exists
// for now; its values are the ones read from the real machine.
func Simulate(model string) (*Hardware, error) {
	if model != "ANV15-52" {
		return nil, fmt.Errorf("unknown simulated model %q", model)
	}
	s := &sim{
		profile: "balanced",
		fans:    map[string]FanState{"cpu": {RPM: 2100, Mode: "auto"}, "gpu": {RPM: 0, Mode: "auto"}},
		timeout: 30, usb: 30, limit: true,
		power: Power{Source: "ac", Percent: 80},
	}
	return &Hardware{
		Model: map[string]interface{}{"vendor": "Acer", "product": "Nitro ANV15-52 (simulated)", "bios": "V1.31",
			"kernel": "simulated", "modules": map[string]string{"acer_wmi": "sim", "acer_wmi_extras": "sim", "acer_wmi_battery": "sim"}},
		Profiles: s, Fans: s, Thermal: s, Keyboard: s, USB: simUSB{s}, BatteryCtl: s, Battery: s, GPU: s, SenseKey: true,
	}, nil
}

type sim struct {
	mu      sync.Mutex
	profile string
	fans    map[string]FanState
	timeout uint32
	on      bool
	usb     uint32
	limit   bool
	calib   bool
	power   Power
}

func (s *sim) Supported() []string {
	return []string{"low-power", "quiet", "balanced", "balanced-performance"}
}
func (s *sim) FromFirmware() bool { return true }
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
