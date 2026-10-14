package hw

import (
	"context"
	"encoding/binary"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	senseKeyboardName = "AT Translated Set 2 keyboard"
	senseKeyScancode  = 0xF5
	evKey             = 1
	keyProg1          = 148        // scancode 0xF5 mapped by systemd hwdb
	inputEventSize    = 24         // struct input_event on 64-bit (timeval 16 + type 2 + code 2 + value 4)
	eviocgkeycode     = 0x80084504 // _IOR('E', 0x04, unsigned int[2])
)

// senseKeyMapped reports whether the keyboard at dev maps scancode 0xF5 to
// KEY_PROG1. systemd's hwdb does that only on models with the NitroSense /
// PredatorSense button; elsewhere 0xF5 is a generic Acer key (the Aspire
// AG15-71P gets KEY_PRESENTATION), so the internal keyboard alone says nothing.
func senseKeyMapped(dev string) bool {
	if f, err := os.Open(dev); err == nil {
		defer f.Close()
		m := [2]uint32{senseKeyScancode, 0} // scancode, keycode
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), eviocgkeycode, uintptr(unsafe.Pointer(&m[0])))
		if errno == 0 {
			return m[1] == keyProg1
		}
	}
	// No access to the device (development on the session bus): the property
	// udev set from the hwdb.
	devnum, err := readStr("/sys/class/input/" + filepath.Base(dev) + "/dev")
	if err != nil {
		return false
	}
	b, err := os.ReadFile("/run/udev/data/c" + devnum)
	if err != nil {
		return false
	}
	for _, l := range strings.Split(string(b), "\n") {
		if strings.EqualFold(l, "E:KEYBOARD_KEY_f5=prog1") {
			return true
		}
	}
	return false
}

// senseKeyDevice returns the /dev/input/eventN of the internal keyboard.
func senseKeyDevice() string {
	names, _ := filepath.Glob("/sys/class/input/event*/device/name")
	for _, n := range names {
		if v, err := readStr(n); err == nil && v == senseKeyboardName {
			return "/dev/input/" + filepath.Base(filepath.Dir(filepath.Dir(n)))
		}
	}
	return ""
}

// WatchSenseKey calls fn on every press of KEY_PROG1 on the internal
// keyboard. It never grabs the device (the desktop keeps receiving the key)
// and reopens it if it goes away.
func WatchSenseKey(ctx context.Context, fn func()) {
	buf := make([]byte, inputEventSize*8)
	for ctx.Err() == nil {
		dev := senseKeyDevice()
		if dev == "" {
			slog.Warn("sense key: internal keyboard not found, retrying in 30 s")
			sleepCtx(ctx, 30*time.Second)
			continue
		}
		f, err := os.Open(dev)
		if err != nil {
			slog.Warn("sense key: open failed", "dev", dev, "err", err)
			sleepCtx(ctx, 30*time.Second)
			continue
		}
		slog.Info("sense key: watching", "dev", dev)
		go func() { <-ctx.Done(); f.Close() }()
		for ctx.Err() == nil {
			n, err := f.Read(buf)
			if err != nil {
				slog.Warn("sense key: read failed, reopening", "err", err)
				break
			}
			for off := 0; off+inputEventSize <= n; off += inputEventSize {
				typ := binary.LittleEndian.Uint16(buf[off+16:])
				code := binary.LittleEndian.Uint16(buf[off+18:])
				value := int32(binary.LittleEndian.Uint32(buf[off+20:]))
				if typ == evKey && code == keyProg1 && value == 1 {
					fn()
				}
			}
		}
		f.Close()
		sleepCtx(ctx, 2*time.Second)
	}
}

func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
