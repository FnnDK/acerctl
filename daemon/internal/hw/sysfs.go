// Package hw reads and writes the kernel interfaces that back the daemon:
// platform-profile, the acer and acer_battery hwmon devices, acer-wmi-extras,
// acer-wmi-battery, power_supply and DMI. Every backend is optional; Probe
// only returns what it found, so capabilities come from the firmware, never
// from the model name.
package hw

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DryRun makes every sysfs write a log line instead of a write.
var DryRun bool

// ErrAbsent is returned by backends whose kernel interface is not present.
var ErrAbsent = errors.New("backend absent")

func readStr(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func readInt(path string) (int64, error) {
	s, err := readStr(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(s, 0, 64)
}

func readUint(path string) (uint64, error) {
	s, err := readStr(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(s, 0, 64)
}

func writeStr(path, value string) error {
	if DryRun {
		slog.Info("dry-run: skip write", "path", path, "value", value)
		return nil
	}
	if err := os.WriteFile(path, []byte(value), 0); err != nil {
		return fmt.Errorf("write %s=%q: %w", path, value, err)
	}
	return nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// hwmonByName returns the directory of the hwmon device whose name file
// matches, or "" if there is none.
func hwmonByName(name string) string {
	dirs, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	for _, d := range dirs {
		if n, err := readStr(filepath.Join(d, "name")); err == nil && n == name {
			return d
		}
	}
	return ""
}

// powerSupplyByType returns the first /sys/class/power_supply entry of the
// given type (Mains, Battery), skipping HID and USB-C PD entries.
func powerSupplyByType(typ string) string {
	dirs, _ := filepath.Glob("/sys/class/power_supply/*")
	for _, d := range dirs {
		if strings.HasPrefix(filepath.Base(d), "hid") || strings.HasPrefix(filepath.Base(d), "ucsi") {
			continue
		}
		if t, err := readStr(filepath.Join(d, "type")); err == nil && t == typ {
			return d
		}
	}
	return ""
}
