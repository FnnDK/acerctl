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
	"sync"
)

// DryRun makes every sysfs write a log line instead of a write.
var DryRun bool

// Root is put in front of every kernel path this package reads or writes:
// "" on a real machine, a fixture tree in the model tests and with
// acerd --root (testdata/models). Paths inside the package stay the real
// ones; only the functions below that touch files apply Root.
var Root string

func rooted(path string) string { return Root + path }

// glob is filepath.Glob under Root; it returns real paths.
func glob(pattern string) []string {
	m, _ := filepath.Glob(rooted(pattern))
	for i := range m {
		m[i] = strings.TrimPrefix(m[i], Root)
	}
	return m
}

// readFile reads a file that is not a WMI attribute, so without wmiMu.
func readFile(path string) ([]byte, error) { return os.ReadFile(rooted(path)) }

// wmiMu serializes every sysfs access of this package. The firmware answers
// WMI calls through one shared SMM buffer and the kernel only serializes
// calls of the same ACPI method, so two different methods in flight at once
// (say the keyboard state and the thermal profile) return garbage. Within
// the daemon this lock removes that; other processes can still collide.
var wmiMu sync.Mutex

// ErrAbsent is returned by backends whose kernel interface is not present.
var ErrAbsent = errors.New("backend absent")

func readStr(path string) (string, error) {
	wmiMu.Lock()
	b, err := os.ReadFile(rooted(path))
	wmiMu.Unlock()
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
	wmiMu.Lock()
	err := os.WriteFile(rooted(path), []byte(value), 0)
	wmiMu.Unlock()
	if err != nil {
		return fmt.Errorf("write %s=%q: %w", path, value, err)
	}
	return nil
}

func exists(path string) bool {
	_, err := os.Stat(rooted(path))
	return err == nil
}

// hwmonByName returns the directory of the hwmon device whose name file
// matches, or "" if there is none.
func hwmonByName(name string) string {
	for _, d := range glob("/sys/class/hwmon/hwmon*") {
		if n, err := readStr(filepath.Join(d, "name")); err == nil && n == name {
			return d
		}
	}
	return ""
}

// powerSupplyByType returns the first /sys/class/power_supply entry of the
// given type (Mains, Battery), skipping HID and USB-C PD entries.
func powerSupplyByType(typ string) string {
	for _, d := range glob("/sys/class/power_supply/*") {
		if strings.HasPrefix(filepath.Base(d), "hid") || strings.HasPrefix(filepath.Base(d), "ucsi") {
			continue
		}
		if t, err := readStr(filepath.Join(d, "type")); err == nil && t == typ {
			return d
		}
	}
	return ""
}
