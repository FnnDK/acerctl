package hw

import "math"

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

// Fans is the hwmon fan backend.
type Fans interface {
	IDs() []string
	Controllable() bool
	Range(id string) Range
	State() (map[string]FanState, error)
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
	Temp            float64
	Usage           uint32
	Clock, MemClock uint32
	Power           float64
}

// GPU is the NVIDIA reader (nvidia-smi in the skeleton; NVML later).
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
	USB        USBCharging
	BatteryCtl BatteryControl
	Battery    BatteryInfo
	GPU        GPU
	NitroKey   bool
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
		caps["kbd_backlight"] = map[string]interface{}{
			"timeouts": h.Keyboard.Timeouts(),
			"rgb":      false,
			"zones":    uint32(0),
		}
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
	caps["nitro_key"] = h.NitroKey
	return caps
}

// NoTemp is the Health.Temp value when there is no battery sensor.
var NoTemp = math.NaN()
