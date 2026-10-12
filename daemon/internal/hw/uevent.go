package hw

import (
	"bytes"
	"context"
	"log/slog"

	"golang.org/x/sys/unix"
)

// WatchPowerSupply delivers one token per kernel uevent of the power_supply
// subsystem (AC plugged/unplugged, charge level changes). It listens on the
// kernel multicast group of NETLINK_KOBJECT_UEVENT, so it needs no udevd
// and no extra library. Returns nil when the socket cannot be opened.
func WatchPowerSupply(ctx context.Context) <-chan struct{} {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.NETLINK_KOBJECT_UEVENT)
	if err != nil {
		slog.Warn("uevent: socket failed", "err", err)
		return nil
	}
	if err := unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK, Groups: 1}); err != nil {
		slog.Warn("uevent: bind failed", "err", err)
		unix.Close(fd)
		return nil
	}
	ch := make(chan struct{}, 1)
	go func() { <-ctx.Done(); unix.Close(fd) }()
	go func() {
		defer close(ch)
		buf := make([]byte, 8192)
		for ctx.Err() == nil {
			n, _, err := unix.Recvfrom(fd, buf, 0)
			if err != nil {
				if err == unix.EINTR {
					continue
				}
				if ctx.Err() == nil {
					slog.Warn("uevent: recv failed", "err", err)
				}
				return
			}
			// Kernel format: "action@devpath\0KEY=VALUE\0...".
			if bytes.Contains(buf[:n], []byte("\x00SUBSYSTEM=power_supply\x00")) {
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
	}()
	slog.Info("uevent: watching power_supply")
	return ch
}
