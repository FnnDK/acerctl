package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/godbus/dbus/v5"

	"nitroctl/daemon/internal/state"
)

// resumeDelay gives the firmware, tuned and the power_supply uevents time to
// settle after a resume before the daemon reads and reapplies anything. On
// the ANV15-52 the firmware resets platform_profile ~0.8 s after the resume
// signal and tuned finishes following it ~1.5 s after.
const resumeDelay = 3 * time.Second

// watchSleep follows logind's PrepareForSleep: false means the machine just
// resumed. Nothing has to happen before sleeping, so no inhibitor is taken.
func (d *Daemon) watchSleep(ctx context.Context) {
	if err := d.conn.AddMatchSignal(
		dbus.WithMatchObjectPath("/org/freedesktop/login1"),
		dbus.WithMatchInterface("org.freedesktop.login1.Manager"),
		dbus.WithMatchMember("PrepareForSleep"),
	); err != nil {
		slog.Warn("cannot watch PrepareForSleep", "err", err)
		return
	}
	ch := make(chan *dbus.Signal, 8)
	d.conn.Signal(ch)
	for {
		select {
		case <-ctx.Done():
			return
		case s := <-ch:
			if s == nil || s.Name != "org.freedesktop.login1.Manager.PrepareForSleep" || len(s.Body) < 1 {
				continue
			}
			sleeping, _ := s.Body[0].(bool)
			d.mu.Lock()
			d.resuming = true // also when the "going to sleep" signal was missed
			d.mu.Unlock()
			if sleeping {
				slog.Info("going to sleep")
			} else {
				go d.onResume()
			}
		}
	}
}

// onResume logs what the firmware kept across the sleep, then reapplies the
// profile and the fans and re-reads everything else (docs/api-dbus.md §11).
// Profile writes by the firmware and tuned are ignored until it is done.
func (d *Daemon) onResume() {
	defer func() {
		d.mu.Lock()
		d.resuming = false
		d.mu.Unlock()
	}()
	time.Sleep(resumeDelay)
	found := []any{}
	if d.hw.Profiles != nil {
		cur, _ := d.hw.Profiles.Current()
		found = append(found, "profile", cur)
	}
	if d.hw.Fans != nil {
		if st, err := d.hw.Fans.State(); err == nil {
			for id, f := range st {
				found = append(found, "fan_"+id, f.Mode)
			}
		}
	}
	slog.Info("resumed", found...)

	d.refreshPower()
	if d.hw.Profiles != nil {
		if cur, err := d.hw.Profiles.Current(); err == nil {
			d.mu.Lock()
			d.current = cur
			d.mu.Unlock()
			d.setProp(ifMode, "Current", cur)
		}
	}
	d.restoreMode()
	d.restoreFans(false)
	d.refreshFans(true)
	d.refreshSlow()
}

// restoreFans brings the fans back to the remembered setting. At startup a
// manual fan is put back in auto instead: a low fixed speed under a load
// nobody is watching is the risk (§11). After a resume manual is restored.
func (d *Daemon) restoreFans(startup bool) {
	if d.hw.Fans == nil || !d.hw.Fans.Controllable() {
		return
	}
	saved := d.st.FansSnapshot()
	if len(saved) == 0 {
		return
	}
	actual, err := d.hw.Fans.State()
	if err != nil {
		slog.Warn("restore fans: cannot read state", "err", err)
		return
	}
	for id, want := range saved {
		have, ok := actual[id]
		if !ok {
			continue
		}
		mode := want.Mode
		if startup && mode == "manual" {
			slog.Info("fan was manual; starting in auto", "fan", id, "speed", want.Speed)
			mode = "auto"
			d.st.Update(func(s *state.Data) { s.Fans[id] = state.Fan{Mode: "auto"} })
		}
		switch {
		case mode == "manual":
			if have.Mode == "manual" && have.Speed == want.Speed {
				continue
			}
			err = d.hw.Fans.SetSpeed(id, want.Speed)
		case mode == "auto" || mode == "max":
			if have.Mode == mode {
				continue
			}
			err = d.hw.Fans.SetMode(id, mode)
		default:
			continue
		}
		if err != nil {
			slog.Error("restore fan failed", "fan", id, "mode", mode, "err", err)
			continue
		}
		slog.Info("fan restored", "fan", id, "mode", mode, "was", have.Mode)
	}
	d.refreshFans(true)
}
