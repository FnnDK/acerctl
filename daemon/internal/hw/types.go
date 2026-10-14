package hw

import (
	"context"
	"math"
)

// Power is the current power source and battery charge.
type Power struct {
	Source  string // "ac" or "battery"
	Percent uint32
}

// FanState mirrors one fan of Fans.State.
type FanState struct {
	RPM   uint32
	Mode  string // auto, max, manual
	Speed uint32 // percent, meaningful in manual
}

// Range is the manual speed range of one fan, in percent.
type Range struct {
	Min, Max uint32
}

// Health is what Battery.Health exposes.
type Health struct {
	Cycles  uint32
	Full    uint64 // µWh
	Design  uint64 // µWh
	Status  string
	Voltage uint32  // mV, 0 if unknown
	Current int32   // mA
	Temp    float64 // °C, NaN if there is no sensor
}

// Profiles is the thermal profile backend (platform_profile).
type Profiles interface {
	Supported() []string
	FromFirmware() bool
	Current() (string, error)
	Set(profile string) error
}

// ProfileWatcher is implemented by profile backends that can signal a
// change of platform_profile without polling (sysfs_notify + poll()).
type ProfileWatcher interface {
	// Watch delivers one token per notification until ctx is done.
	Watch(ctx context.Context) <-chan struct{}
}

// Fans is the hwmon fan backend.
type Fans interface {
	IDs() []string
	Controllable() bool
	Range(id string) Range
	// State reads RPM, mode and speed. Mode and speed go through WMI/SMM
	// (~23 ms per fan on the ANV15-52); RPM alone is an EC read (~0.2 ms).
	State() (map[string]FanState, error)
	// RPM reads only the speeds, for the frequent refreshes.
	RPM() (map[string]uint32, error)
	SetMode(id, mode string) error
	SetSpeed(id string, percent uint32) error
}

// Thermal reads the firmware temperature sensors.
type Thermal interface {
	Temps() (map[string]float64, error) // cpu, gpu, external
}

// Keyboard is the single-colour keyboard backlight (acer-wmi-extras).
type Keyboard interface {
	Timeouts() []uint32
	Timeout() (uint32, error)
	SetTimeout(seconds uint32) error
	On() (bool, error)
}

// USBCharging is the off-state USB charger (acer-wmi-extras).
type USBCharging interface {
	Values() []uint32
	Get() (uint32, error)
	Set(percent uint32) error
}

// BatteryControl is the charge limit and calibration (acer-wmi-battery).
type BatteryControl interface {
	HasChargeLimit() bool
	HasCalibration() bool
	ChargeLimit() (bool, error)
	SetChargeLimit(on bool) error
	Calibrating() (bool, error)
	SetCalibrating(on bool) error
}

// BatteryInfo is the generic power_supply view plus the acer_battery hwmon.
type BatteryInfo interface {
	Power() (Power, error)
	Health() (Health, error)
	HasSensors() bool
}

// GPUSample is one reading of the discrete GPU.
type GPUSample struct {
	Off             bool // runtime-suspended: not queried, other fields zero
	Temp            float64
	Usage           uint32
	Clock, MemClock uint32
	Power           float64
}

// GPU is the NVIDIA reader (NVML, nvidia-smi as fallback).
type GPU interface {
	Sample() (GPUSample, error)
}

// Hardware is everything Probe found. Nil fields are absent backends.
type Hardware struct {
	Model      map[string]interface{}
	Profiles   Profiles
	Fans       Fans
	Thermal    Thermal
	Keyboard   Keyboard
	Lights     Lighting
	USB        USBCharging
	BatteryCtl BatteryControl
	Battery    BatteryInfo
	GPU        GPU
	SenseKey   bool
	Setup      Setup // system changes the app can make (setup.go)
}

// Capabilities builds the System.Capabilities dictionary from what was
// probed. Keys are only present when the backend exists.
func (h *Hardware) Capabilities() map[string]interface{} {
	caps := map[string]interface{}{}
	if h.Profiles != nil {
		caps["profiles"] = h.Profiles.Supported()
		caps["profiles_from_firmware"] = h.Profiles.FromFirmware()
	}
	if h.Fans != nil && h.Fans.Controllable() {
		ranges := map[string]Range{}
		for _, id := range h.Fans.IDs() {
			ranges[id] = h.Fans.Range(id)
		}
		caps["fans"] = map[string]interface{}{
			"ids":   h.Fans.IDs(),
			"modes": []string{"auto", "max", "manual"},
			"range": ranges,
			"curve": false,
		}
	}
	if h.Keyboard != nil {
		rgb, zones := false, uint32(0)
		if h.Lights != nil {
			for _, d := range h.Lights.Devices() {
				if d.ID == "keyboard" {
					rgb, zones = true, d.Zones
				}
			}
		}
		caps["kbd_backlight"] = map[string]interface{}{
			"timeouts": h.Keyboard.Timeouts(),
			"rgb":      rgb,
			"zones":    zones,
		}
	}
	if h.Lights != nil {
		caps["lighting"] = lightCaps(h.Lights.Devices())
	}
	if h.USB != nil {
		caps["usb_charging"] = h.USB.Values()
	}
	if h.BatteryCtl != nil || (h.Battery != nil && h.Battery.HasSensors()) {
		b := map[string]interface{}{
			"charge_limit": h.BatteryCtl != nil && h.BatteryCtl.HasChargeLimit(),
			"calibration":  h.BatteryCtl != nil && h.BatteryCtl.HasCalibration(),
			"sensors":      h.Battery != nil && h.Battery.HasSensors(),
		}
		caps["battery"] = b
	}
	caps["gpu"] = map[string]interface{}{"nvml": h.GPU != nil, "oc": false}
	caps["cpu_power_limit"] = false
	caps["sense_key"] = h.SenseKey
	return caps
}

// NoTemp is the Health.Temp value when there is no battery sensor.
var NoTemp = math.NaN()
