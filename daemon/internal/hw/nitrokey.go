package hw

import (
	"context"
	"encoding/binary"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

const (
	nitroKeyboardName = "AT Translated Set 2 keyboard"
	evKey             = 1
	keyProg1          = 148 // scancode 0xF5 mapped by systemd hwdb
	inputEventSize    = 24  // struct input_event on 64-bit (timeval 16 + type 2 + code 2 + value 4)
)

// nitroKeyDevice returns the /dev/input/eventN of the internal keyboard.
func nitroKeyDevice() string {
	names, _ := filepath.Glob("/sys/class/input/event*/device/name")
	for _, n := range names {
		if v, err := readStr(n); err == nil && v == nitroKeyboardName {
			return "/dev/input/" + filepath.Base(filepath.Dir(filepath.Dir(n)))
		}
	}
	return ""
}

// WatchNitroKey calls fn on every press of KEY_PROG1 on the internal
// keyboard. It never grabs the device (the desktop keeps receiving the key)
// and reopens it if it goes away.
func WatchNitroKey(ctx context.Context, fn func()) {
	buf := make([]byte, inputEventSize*8)
	for ctx.Err() == nil {
		dev := nitroKeyDevice()
		if dev == "" {
			slog.Warn("nitro key: internal keyboard not found, retrying in 30 s")
			sleepCtx(ctx, 30*time.Second)
			continue
		}
		f, err := os.Open(dev)
		if err != nil {
			slog.Warn("nitro key: open failed", "dev", dev, "err", err)
			sleepCtx(ctx, 30*time.Second)
			continue
		}
		slog.Info("nitro key: watching", "dev", dev)
		go func() { <-ctx.Done(); f.Close() }()
		for ctx.Err() == nil {
			n, err := f.Read(buf)
			if err != nil {
				slog.Warn("nitro key: read failed, reopening", "err", err)
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
