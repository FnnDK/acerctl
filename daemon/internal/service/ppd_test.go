package service

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"

	"acerctl/daemon/internal/hw"
	"acerctl/daemon/internal/ppd"
	"acerctl/daemon/internal/scenarios"
	"acerctl/daemon/internal/state"
)

// privateBus starts a session dbus-daemon for the test and points
// DBUS_SESSION_BUS_ADDRESS at it. Without dbus-daemon the test is skipped,
// except in CI, where it must run.
func privateBus(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("dbus-daemon"); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("dbus-daemon is required in CI")
		}
		t.Skip("dbus-daemon not installed")
	}
	cmd := exec.Command("dbus-daemon", "--session", "--nofork", "--print-address=1")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	addr, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", strings.TrimSpace(addr))
}

// fakePPD owns net.hadess.PowerProfiles with a writable ActiveProfile and
// records every value written to it.
type fakePPD struct {
	conn *dbus.Conn
	mu   sync.Mutex
	sets []string
}

func startFakePPD(t *testing.T, active string) *fakePPD {
	t.Helper()
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	f := &fakePPD{conn: conn}
	_, err = prop.Export(conn, "/net/hadess/PowerProfiles", prop.Map{
		"net.hadess.PowerProfiles": {
			"ActiveProfile": {Value: active, Writable: true, Emit: prop.EmitTrue,
				Callback: func(c *prop.Change) *dbus.Error {
					f.mu.Lock()
					f.sets = append(f.sets, c.Value.(string))
					f.mu.Unlock()
					return nil
				}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply, err := conn.RequestName(ppd.BusName, dbus.NameFlagDoNotQueue); err != nil ||
		reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("own %s: %v %v", ppd.BusName, reply, err)
	}
	return f
}

func (f *fakePPD) written() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.sets...)
}

// startDaemon runs a simulated ANV15-52 on the private bus until the test ends.
func startDaemon(t *testing.T) *dbus.Conn {
	t.Helper()
	h, err := hw.Simulate("ANV15-52")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	d := New(Config{Session: true, NoPolkit: true, Version: "test"}, h,
		state.Load(filepath.Join(dir, "state.json")), scenarios.Load(filepath.Join(dir, "scenarios.json")))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { d.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	// The name is taken early; a readable Mode.Current means the export is done.
	waitFor(t, "acerd exported", func() bool { return currentMode(conn) != "" })
	return conn
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for end := time.Now().Add(15 * time.Second); time.Now().Before(end); time.Sleep(100 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

func modeProp(conn *dbus.Conn, name string) string {
	v, err := conn.Object(busName, objPath).GetProperty(ifMode + "." + name)
	if err != nil {
		return ""
	}
	s, _ := v.Value().(string)
	return s
}

func currentMode(conn *dbus.Conn) string { return modeProp(conn, "Current") }

// desktopPicks writes ActiveProfile the way the GNOME power menu does.
func desktopPicks(t *testing.T, conn *dbus.Conn, profile string) {
	t.Helper()
	err := conn.Object(ppd.BusName, "/net/hadess/PowerProfiles").Call("org.freedesktop.DBus.Properties.Set", 0,
		"net.hadess.PowerProfiles", "ActiveProfile", dbus.MakeVariant(profile)).Err
	if err != nil {
		t.Fatal(err)
	}
}

func contains(l []string, s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

// TestPPDAppearsAfterStart is the boot race on RPM installs: tuned-ppd takes
// its name after the daemon looked for it. The daemon must still pick it up,
// move it to the current mode and follow desktop changes.
func TestPPDAppearsAfterStart(t *testing.T) {
	privateBus(t)
	conn := startDaemon(t)
	time.Sleep(500 * time.Millisecond) // the daemon has looked for the PPD and found none
	if p := modeProp(conn, "PPD"); p != "" {
		t.Fatalf("Mode.PPD = %q with no PPD on the bus", p)
	}
	f := startFakePPD(t, "power-saver")
	waitFor(t, "the daemon moving the new PPD to balanced", func() bool { return contains(f.written(), "balanced") })
	waitFor(t, "Mode.PPD naming the new PPD", func() bool { return modeProp(conn, "PPD") != "" })
	// The sync with the new PPD is still waiting for tuned when the desktop
	// changes the profile; the desktop choice must be the one that stays.
	desktopPicks(t, conn, "performance")
	waitFor(t, "the mode following the desktop", func() bool { return currentMode(conn) == "balanced-performance" })
	time.Sleep(settleTimeout + time.Second)
	if m := currentMode(conn); m != "balanced-performance" {
		t.Fatalf("mode went back to %s after the desktop chose performance", m)
	}
	f.conn.Close() // the PPD leaves the bus
	waitFor(t, "Mode.PPD cleared", func() bool { return modeProp(conn, "PPD") == "" })
}

// TestPPDPresentAtStart is the ordered start: the PPD is already on the bus.
func TestPPDPresentAtStart(t *testing.T) {
	privateBus(t)
	startFakePPD(t, "balanced")
	conn := startDaemon(t)
	if p := modeProp(conn, "PPD"); p == "" {
		t.Fatal("Mode.PPD empty with a PPD on the bus")
	}
	desktopPicks(t, conn, "performance")
	waitFor(t, "the mode following the desktop", func() bool { return currentMode(conn) == "balanced-performance" })
}

// TestUnitOrderedAfterPPD keeps the boot ordering: both PPD implementations
// are Type=dbus, so After= means their name is on the bus when acerd starts.
func TestUnitOrderedAfterPPD(t *testing.T) {
	b, err := os.ReadFile("../../../packaging/acerd.service")
	if err != nil {
		t.Fatal(err)
	}
	var after []string
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "After="); ok {
			after = append(after, strings.Fields(v)...)
		}
	}
	for _, u := range []string{"tuned-ppd.service", "power-profiles-daemon.service"} {
		if !contains(after, u) {
			t.Errorf("acerd.service is not ordered after %s (After=%v)", u, after)
		}
	}
}
