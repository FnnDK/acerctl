package hw

import (
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
)

// The supported-profiles read after boot: a transient firmware error is
// retried, while "not supported" and a missing file are final at once.
func TestReadSupportedRetries(t *testing.T) {
	old, oldRead := profilesRetry, profilesRead
	profilesRetry.delay = 0
	t.Cleanup(func() { profilesRetry, profilesRead = old, oldRead })

	const list = "low-power quiet balanced balanced-performance"
	for _, tc := range []struct {
		name  string
		errs  []error // one per call; nil means the list is read
		calls int
		ok    bool
	}{
		{"transient then ok", []error{syscall.EIO, syscall.EIO, nil}, 3, true},
		{"always failing", []error{syscall.EIO, syscall.EIO, syscall.EIO, syscall.EIO, syscall.EIO}, 5, false},
		{"not supported", []error{syscall.EOPNOTSUPP}, 1, false},
		{"module not loaded", []error{&fs.PathError{Op: "open", Err: fs.ErrNotExist}}, 1, false},
	} {
		calls := 0
		profilesRead = func(string) (string, error) {
			err := tc.errs[calls]
			calls++
			if err != nil {
				return "", err
			}
			return list, nil
		}
		s, err := readSupported("supported_profiles")
		if calls != tc.calls || (err == nil) != tc.ok || (tc.ok && s != list) {
			t.Errorf("%s: %d calls, %q, %v", tc.name, calls, s, err)
		}
		if !tc.ok && err != nil && !errors.Is(err, tc.errs[len(tc.errs)-1]) {
			t.Errorf("%s: err %v is not the last one", tc.name, err)
		}
	}
}

// TestProbeNoTurboAfterBootFailure is the ANV15-52 boot of 2026-10-21: the
// firmware failed the supported-profiles call right after boot, the daemon
// took acer_wmi's list and offered Turbo, which the firmware rejects. With
// the read failing a few times and then answering, the probe must still end
// with the firmware's list; failing for good, it must say the list is not the
// firmware's.
func TestProbeNoTurboAfterBootFailure(t *testing.T) {
	old, oldRead := profilesRetry, profilesRead
	profilesRetry.delay = 0
	t.Cleanup(func() { profilesRetry, profilesRead = old, oldRead })

	dir := t.TempDir()
	copyTree(t, "testdata/models/ANV15-52/root", dir)
	Root = dir
	t.Cleanup(func() { Root = "" })

	for _, tc := range []struct {
		name         string
		failures     int
		fromFirmware bool
	}{
		{"fails twice, then answers", 2, true},
		{"never answers", 99, false},
	} {
		left := tc.failures
		profilesRead = func(p string) (string, error) {
			if filepath.Base(p) == "supported_profiles" && left > 0 {
				left--
				return "", syscall.EIO
			}
			return readStr(p)
		}
		h := Probe()
		if h.Profiles == nil {
			t.Fatalf("%s: no profiles", tc.name)
		}
		got := h.Profiles.Supported()
		if h.Profiles.FromFirmware() != tc.fromFirmware {
			t.Errorf("%s: FromFirmware = %v, profiles %v", tc.name, h.Profiles.FromFirmware(), got)
		}
		if tc.fromFirmware && slices.Contains(got, "performance") {
			t.Errorf("%s: Turbo (performance) offered: %v", tc.name, got)
		}
	}
}
