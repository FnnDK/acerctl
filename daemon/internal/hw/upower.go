package hw

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/godbus/dbus/v5"
)

// UPower owns the charge threshold on desktops (GNOME "Preserve Battery
// Health") and re-applies its own saved state whenever it starts. When
// acer-wmi-battery exposes charge_control_end_threshold on the ACPI battery,
// the charge limit goes through UPower so both stay in agreement; writing
// health_mode behind its back would be undone on the next boot.

const (
	upowerName  = "org.freedesktop.UPower"
	upowerPath  = "/org/freedesktop/UPower"
	upowerIface = "org.freedesktop.UPower"
	upowerDev   = "org.freedesktop.UPower.Device"
	upowerCall  = 5 * time.Second
)

type upowerLimit struct {
	native string // power_supply name, e.g. "BAT1"
}

// probeUPowerLimit finds the battery carrying the acer-wmi-battery
// power_supply extension (kernel >= 6.15, driver with the battery hook).
func probeUPowerLimit() *upowerLimit {
	if Root != "" { // UPower drives the real batteries, not a fixture tree
		return nil
	}
	m := glob("/sys/class/power_supply/*/extensions/acer-wmi-battery")
	if len(m) == 0 {
		return nil
	}
	return &upowerLimit{native: filepath.Base(filepath.Dir(filepath.Dir(m[0])))}
}

// set calls EnableChargeThreshold on the UPower device of that battery. The
// connection is per call: the limit changes rarely and this keeps the
// daemon's own bus connection out of it. Root is always authorized by polkit.
func (u *upowerLimit) set(on bool) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), upowerCall)
	defer cancel()

	var devs []dbus.ObjectPath
	if err := conn.Object(upowerName, upowerPath).CallWithContext(ctx, upowerIface+".EnumerateDevices", 0).Store(&devs); err != nil {
		return err
	}
	for _, p := range devs {
		obj := conn.Object(upowerName, p)
		var native dbus.Variant
		if err := obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, upowerDev, "NativePath").Store(&native); err != nil {
			continue
		}
		if s, _ := native.Value().(string); s != u.native {
			continue
		}
		var supported dbus.Variant
		if err := obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, upowerDev, "ChargeThresholdSupported").Store(&supported); err != nil {
			return err
		}
		if ok, _ := supported.Value().(bool); !ok {
			return errors.New("UPower: charge threshold not supported on " + u.native)
		}
		return obj.CallWithContext(ctx, upowerDev+".EnableChargeThreshold", 0, on).Err
	}
	return errors.New("UPower: no device for " + u.native)
}
