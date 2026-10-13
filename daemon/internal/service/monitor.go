package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/godbus/dbus/v5"
)

const minInterval = 500 * time.Millisecond

type subscriber struct {
	interval time.Duration
	stop     chan struct{}
}

// ---- io.github.fnndk.Nitrod1.Monitor ----------------------------------------

type monitorIface struct{ d *Daemon }

// Get returns one sample.
func (m *monitorIface) Get() (map[string]dbus.Variant, *dbus.Error) {
	return variants(m.d.sampler.Sample()), nil
}

// Subscribe starts unicast Sample signals to the caller every interval_ms.
func (m *monitorIface) Subscribe(sender dbus.Sender, intervalMs uint32) *dbus.Error {
	d := m.d
	iv := time.Duration(intervalMs) * time.Millisecond
	if iv < minInterval {
		iv = minInterval
	}
	d.subMu.Lock()
	defer d.subMu.Unlock()
	if old, ok := d.subs[string(sender)]; ok {
		close(old.stop)
	}
	s := &subscriber{interval: iv, stop: make(chan struct{})}
	d.subs[string(sender)] = s
	go d.stream(string(sender), s)
	slog.Debug("monitor subscribe", "sender", sender, "interval", iv)
	return nil
}

// Unsubscribe stops the caller's stream.
func (m *monitorIface) Unsubscribe(sender dbus.Sender) *dbus.Error {
	m.d.removeSubscriber(string(sender))
	return nil
}

func (d *Daemon) removeSubscriber(name string) {
	d.subMu.Lock()
	defer d.subMu.Unlock()
	if s, ok := d.subs[name]; ok {
		close(s.stop)
		delete(d.subs, name)
	}
}

func (d *Daemon) stopAllSubscribers() {
	d.subMu.Lock()
	defer d.subMu.Unlock()
	for name, s := range d.subs {
		close(s.stop)
		delete(d.subs, name)
	}
}

func (d *Daemon) stream(dest string, s *subscriber) {
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			d.emitTo(dest, ifMonitor, "Sample", variants(d.sampler.Sample()))
		}
	}
}

// emitTo sends a signal to one bus name instead of broadcasting it.
func (d *Daemon) emitTo(dest, iface, member string, args ...interface{}) {
	msg := &dbus.Message{
		Type: dbus.TypeSignal,
		Headers: map[dbus.HeaderField]dbus.Variant{
			dbus.FieldPath:        dbus.MakeVariant(objPath),
			dbus.FieldInterface:   dbus.MakeVariant(iface),
			dbus.FieldMember:      dbus.MakeVariant(member),
			dbus.FieldDestination: dbus.MakeVariant(dest),
		},
		Body: args,
	}
	if len(args) > 0 {
		msg.Headers[dbus.FieldSignature] = dbus.MakeVariant(dbus.SignatureOf(args...))
	}
	if err := d.conn.Send(msg, nil); err != nil {
		slog.Warn("signal send failed", "dest", dest, "err", err)
	}
}

// watchNameOwners drops subscribers that left the bus.
func (d *Daemon) watchNameOwners(ctx context.Context) {
	if err := d.conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
	); err != nil {
		slog.Warn("cannot watch NameOwnerChanged", "err", err)
		return
	}
	ch := make(chan *dbus.Signal, 32)
	d.conn.Signal(ch)
	for {
		select {
		case <-ctx.Done():
			return
		case s := <-ch:
			if s == nil || s.Name != "org.freedesktop.DBus.NameOwnerChanged" || len(s.Body) < 3 {
				continue
			}
			name, _ := s.Body[0].(string)
			newOwner, _ := s.Body[2].(string)
			if newOwner == "" {
				d.removeSubscriber(name)
			}
		}
	}
}
