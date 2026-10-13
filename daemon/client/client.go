// Package client talks to nitrosensed over D-Bus (docs/api-dbus.md). It is
// shared by the CLI and, later, the GUI and tray; it holds no policy of its
// own: the daemon validates and explains every refusal.
package client

import (
	"errors"
	"fmt"
	"strings"

	"github.com/godbus/dbus/v5"
)

const (
	BusName = "org.nitrosense.Daemon1"
	ObjPath = dbus.ObjectPath("/org/nitrosense/Daemon1")

	IfSystem    = BusName + ".System"
	IfMode      = BusName + ".Mode"
	IfFans      = BusName + ".Fans"
	IfMonitor   = BusName + ".Monitor"
	IfLighting  = BusName + ".Lighting"
	IfBattery   = BusName + ".Battery"
	IfScenarios = BusName + ".Scenarios"

	errBase = BusName + ".Error."
)

// Client is a connection to the daemon.
type Client struct {
	conn *dbus.Conn
	obj  dbus.BusObject
}

// Connect opens the system bus (or the session bus, for a daemon started
// with --session) and checks that the daemon is there.
func Connect(session bool) (*Client, error) {
	var conn *dbus.Conn
	var err error
	if session {
		conn, err = dbus.ConnectSessionBus()
	} else {
		conn, err = dbus.ConnectSystemBus()
	}
	if err != nil {
		return nil, fmt.Errorf("connect to D-Bus: %w", err)
	}
	var has bool
	if err := conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, BusName).Store(&has); err != nil || !has {
		conn.Close()
		return nil, errors.New("nitrosensed is not running (systemctl status nitrosensed)")
	}
	return &Client{conn: conn, obj: conn.Object(BusName, ObjPath)}, nil
}

// Close closes the connection.
func (c *Client) Close() { c.conn.Close() }

// Conn exposes the bus connection (signal subscriptions).
func (c *Client) Conn() *dbus.Conn { return c.conn }

// Property reads one property.
func (c *Client) Property(iface, name string) (dbus.Variant, error) {
	var v dbus.Variant
	err := c.obj.Call("org.freedesktop.DBus.Properties.Get", 0, iface, name).Store(&v)
	return v, wrap(err)
}

// Properties reads every property of an interface.
func (c *Client) Properties(iface string) (map[string]dbus.Variant, error) {
	var m map[string]dbus.Variant
	err := c.obj.Call("org.freedesktop.DBus.Properties.GetAll", 0, iface).Store(&m)
	return m, wrap(err)
}

// Call invokes iface.method and stores the reply in out (may be nil). It
// allows interactive polkit authentication (password dialog).
func (c *Client) Call(iface, method string, out []interface{}, args ...interface{}) error {
	call := c.obj.Call(iface+"."+method, dbus.FlagAllowInteractiveAuthorization, args...)
	if call.Err != nil {
		return wrap(call.Err)
	}
	if len(out) > 0 {
		return wrap(call.Store(out...))
	}
	return nil
}

// Error is a refusal from the daemon with a short kind.
type Error struct {
	Kind string // NotSupported, NotAllowed, InvalidArgument, Hardware, AccessDenied, or the D-Bus name
	Msg  string
}

func (e *Error) Error() string {
	kind := map[string]string{
		"NotSupported":    "not supported",
		"NotAllowed":      "not allowed",
		"InvalidArgument": "invalid argument",
		"Hardware":        "hardware error",
		"AccessDenied":    "not authorized",
	}[e.Kind]
	if kind == "" {
		kind = e.Kind
	}
	if e.Msg == "" {
		return kind
	}
	return kind + ": " + e.Msg
}

func wrap(err error) error {
	var de dbus.Error
	if errors.As(err, &de) {
		kind := de.Name
		switch {
		case strings.HasPrefix(de.Name, errBase):
			kind = strings.TrimPrefix(de.Name, errBase)
		case de.Name == "org.freedesktop.DBus.Error.AccessDenied":
			kind = "AccessDenied"
		}
		msg := ""
		if len(de.Body) > 0 {
			msg, _ = de.Body[0].(string)
		}
		return &Error{Kind: kind, Msg: msg}
	}
	return err
}

// Subscribe starts the Monitor stream; samples arrive on the returned
// channel until stop is called.
func (c *Client) Subscribe(intervalMs uint32) (samples <-chan map[string]dbus.Variant, stop func(), err error) {
	if err := c.conn.AddMatchSignal(
		dbus.WithMatchObjectPath(ObjPath),
		dbus.WithMatchInterface(IfMonitor),
		dbus.WithMatchMember("Sample"),
	); err != nil {
		return nil, nil, err
	}
	sigs := make(chan *dbus.Signal, 16)
	c.conn.Signal(sigs)
	if err := c.Call(IfMonitor, "Subscribe", nil, intervalMs); err != nil {
		c.conn.RemoveSignal(sigs)
		return nil, nil, err
	}
	out := make(chan map[string]dbus.Variant, 4)
	done := make(chan struct{})
	go func() {
		defer close(out)
		for {
			select {
			case <-done:
				return
			case s := <-sigs:
				if s == nil || s.Name != IfMonitor+".Sample" || len(s.Body) < 1 {
					continue
				}
				if m, ok := s.Body[0].(map[string]dbus.Variant); ok {
					select {
					case out <- m:
					default: // the reader is slow; drop rather than block
					}
				}
			}
		}
	}()
	stop = func() {
		_ = c.Call(IfMonitor, "Unsubscribe", nil)
		c.conn.RemoveSignal(sigs)
		close(done)
	}
	return out, stop, nil
}
