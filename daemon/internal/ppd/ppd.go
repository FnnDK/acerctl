// Package ppd is a small client of net.hadess.PowerProfiles (tuned-ppd or
// power-profiles-daemon), used to keep its ActiveProfile close to the Acer
// mode and to notice profile changes made elsewhere (docs/api-dbus.md §11).
package ppd

import (
	"log/slog"

	"github.com/godbus/dbus/v5"
)

const (
	busName = "net.hadess.PowerProfiles"
	objPath = "/net/hadess/PowerProfiles"
	iface   = "net.hadess.PowerProfiles"
)

// Client talks to the PPD on the same bus as the daemon.
type Client struct {
	conn *dbus.Conn
	obj  dbus.BusObject
}

// Connect returns nil when no PPD owns the name on the bus.
func Connect(conn *dbus.Conn) *Client {
	var owner string
	if err := conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, busName).Store(&owner); err != nil {
		slog.Info("no power-profiles-daemon on the bus; PPD sync disabled")
		return nil
	}
	slog.Info("power-profiles-daemon found", "owner", owner)
	return &Client{conn: conn, obj: conn.Object(busName, objPath)}
}

// Active returns the current PPD profile.
func (c *Client) Active() (string, error) {
	v, err := c.obj.GetProperty(iface + ".ActiveProfile")
	if err != nil {
		return "", err
	}
	s, _ := v.Value().(string)
	return s, nil
}

// Set changes the PPD profile; it is a no-op when already equal.
func (c *Client) Set(profile string) error {
	if cur, err := c.Active(); err == nil && cur == profile {
		return nil
	}
	return c.obj.Call("org.freedesktop.DBus.Properties.Set", 0, iface, "ActiveProfile", dbus.MakeVariant(profile)).Err
}

// Watch delivers every ActiveProfile change on the returned channel.
func (c *Client) Watch() (<-chan string, error) {
	if err := c.conn.AddMatchSignal(
		dbus.WithMatchObjectPath(objPath),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
	); err != nil {
		return nil, err
	}
	sigs := make(chan *dbus.Signal, 16)
	c.conn.Signal(sigs)
	out := make(chan string, 4)
	go func() {
		for s := range sigs {
			if s.Path != objPath || len(s.Body) < 2 {
				continue
			}
			if name, _ := s.Body[0].(string); name != iface {
				continue
			}
			changed, _ := s.Body[1].(map[string]dbus.Variant)
			if v, ok := changed["ActiveProfile"]; ok {
				if p, ok := v.Value().(string); ok {
					out <- p
				}
			}
		}
	}()
	return out, nil
}
