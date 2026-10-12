package service

import (
	"log/slog"

	"github.com/godbus/dbus/v5"
)

// polkit action ids (packaging/org.nitrosense.daemon.policy).
const (
	actionControl  = "org.nitrosense.daemon.control"
	actionSettings = "org.nitrosense.daemon.settings"
)

type polkitSubject struct {
	Kind    string
	Details map[string]dbus.Variant
}

type polkitResult struct {
	IsAuthorized bool
	IsChallenge  bool
	Details      map[string]string
}

// authorize asks polkit whether the calling bus name may perform action.
// With --no-polkit (development on the session bus) everything is allowed.
func (d *Daemon) authorize(sender dbus.Sender, action string) *dbus.Error {
	if d.cfg.NoPolkit {
		return nil
	}
	subject := polkitSubject{Kind: "system-bus-name", Details: map[string]dbus.Variant{
		"name": dbus.MakeVariant(string(sender)),
	}}
	var res polkitResult
	const allowUserInteraction = uint32(1)
	err := d.conn.Object("org.freedesktop.PolicyKit1", "/org/freedesktop/PolicyKit1/Authority").
		Call("org.freedesktop.PolicyKit1.Authority.CheckAuthorization", 0,
			subject, action, map[string]string{}, allowUserInteraction, "").Store(&res)
	if err != nil {
		slog.Error("polkit check failed", "action", action, "err", err)
		return errDenied(action)
	}
	if !res.IsAuthorized {
		return errDenied(action)
	}
	return nil
}
