// Package ppd is a small client of net.hadess.PowerProfiles (tuned-ppd or
// power-profiles-daemon), used to keep its ActiveProfile close to the Acer
// mode and to notice profile changes made elsewhere (docs/api-dbus.md §11).
package ppd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"
)

// BusName is the well-known name both PPD implementations own.
const BusName = "net.hadess.PowerProfiles"

const (
	objPath = "/net/hadess/PowerProfiles"
	iface   = "net.hadess.PowerProfiles"
)

// Client talks to the PPD on the same bus as the daemon.
type Client struct {
	conn *dbus.Conn
	obj  dbus.BusObject
	// Provider names the program that owned the name at Connect, e.g. tuned-ppd.
	Provider string
}

// Connect returns nil when no PPD owns the name on the bus.
func Connect(conn *dbus.Conn) *Client {
	var owner string
	if err := conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, BusName).Store(&owner); err != nil {
		slog.Info("no power-profiles-daemon on the bus; PPD sync starts when one appears")
		return nil
	}
	p := provider(conn, owner)
	slog.Info("power-profiles-daemon found", "owner", owner, "provider", p)
	return &Client{conn: conn, obj: conn.Object(BusName, objPath), Provider: p}
}

// ProviderName names the program that owns the name now, "" when none does.
func ProviderName(conn *dbus.Conn) string {
	var owner string
	if conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, BusName).Store(&owner) != nil {
		return ""
	}
	return provider(conn, owner)
}

// provider reads the owner's process name; the bus name when it cannot.
func provider(conn *dbus.Conn, owner string) string {
	var pid uint32
	if conn.BusObject().Call("org.freedesktop.DBus.GetConnectionUnixProcessID", 0, owner).Store(&pid) != nil {
		return BusName
	}
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	name := strings.TrimSpace(string(b))
	if err != nil || name == "" {
		return BusName
	}
	// comm keeps 15 characters (power-profiles-daemon -> power-profiles-)
	if len(name) == 15 {
		if cmd, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); err == nil {
			if argv0 := filepath.Base(strings.SplitN(string(cmd), "\x00", 2)[0]); strings.HasPrefix(argv0, name) {
				name = argv0
			}
		}
	}
	return name
}

// Present reports whether a PPD owns the name, without logging.
func Present(conn *dbus.Conn) bool {
	var has bool
	err := conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, BusName).Store(&has)
	return err == nil && has
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
