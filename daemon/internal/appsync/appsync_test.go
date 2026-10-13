package appsync

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"acerctl/daemon/internal/scenarios"
)

// datagram builds one netlink message carrying a proc_event.
func datagram(what uint32, pid, tgid int) []byte {
	b := make([]byte, nlmsgHdrLen+cnMsgLen+40)
	le := binary.LittleEndian
	le.PutUint32(b[0:], uint32(len(b)))
	le.PutUint32(b[16:], cnIdxProc)
	le.PutUint32(b[20:], cnValProc)
	ev := b[nlmsgHdrLen+cnMsgLen:]
	le.PutUint32(ev[0:], what)
	le.PutUint32(ev[16:], uint32(pid))
	le.PutUint32(ev[20:], uint32(tgid))
	return b
}

func TestParse(t *testing.T) {
	var b []byte
	b = append(b, datagram(procEventExec, 100, 100)...)
	b = append(b, datagram(procEventExit, 101, 100)...) // a thread: ignored
	b = append(b, datagram(procEventExit, 100, 100)...)
	b = append(b, datagram(0x1, 102, 102)...) // fork: ignored
	got := parse(b)
	want := []event{{exec: true, pid: 100}, {exec: false, pid: 100}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parse = %+v, want %+v", got, want)
	}
}

func TestTracker(t *testing.T) {
	var tr tracker
	tr.add(1, "a")
	tr.add(2, "b")
	if tr.top() != "b" {
		t.Fatalf("top = %q", tr.top())
	}
	tr.remove(2)
	if tr.top() != "a" {
		t.Fatalf("top after exit = %q", tr.top())
	}
	if tr.add(1, "a") {
		t.Fatal("re-adding pid 1 reported as new")
	}
	tr.remove(1)
	if tr.top() != "" {
		t.Fatalf("top when empty = %q", tr.top())
	}
}

func fakeProc(t *testing.T, root string, pid, uid int, comm string, argv ...string) {
	dir := filepath.Join(root, strconv.Itoa(pid))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := ""
	for _, a := range argv {
		cmd += a + "\x00"
	}
	os.WriteFile(filepath.Join(dir, "comm"), []byte(comm+"\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmd), 0o644)
	os.WriteFile(filepath.Join(dir, "status"), []byte("Name:\t"+comm+"\nUid:\t"+strconv.Itoa(uid)+"\t"+strconv.Itoa(uid)+"\t0\t0\n"), 0o644)
}

func TestWatcherMatching(t *testing.T) {
	root := t.TempDir()
	seat := filepath.Join(root, "seat0")
	os.WriteFile(seat, []byte("IS_SEAT0=1\nACTIVE_UID=1000\n"), 0o644)
	store := scenarios.Load("")
	store.Put(scenarios.Scenario{Name: "game", Apps: []string{"eldenring.exe"}})
	store.Put(scenarios.Scenario{Name: "build", Apps: []string{"cc1plus"}})

	fakeProc(t, root, 10, 1000, "eldenring.exe", `Z:\games\ELDEN RING\Game\eldenring.exe`)
	fakeProc(t, root, 11, 1000, "wine64-preload", "/usr/lib/wine/wine64-preloader", `C:\x\EldenRing.exe`)
	fakeProc(t, root, 12, 0, "cc1plus", "/usr/libexec/gcc/cc1plus") // root: not the session user
	fakeProc(t, root, 13, 1000, "bash", "/bin/bash")

	w := &Watcher{ForApp: store.ForApp, procRoot: root, seatFile: seat, pending: make(chan string, 1)}
	w.scan()
	if got := w.t.pids(); len(got) != 2 {
		t.Fatalf("tracked = %v, want pids 10 and 11", got)
	}
	if w.last != "game" {
		t.Fatalf("scenario = %q", w.last)
	}
	os.RemoveAll(filepath.Join(root, "10"))
	os.RemoveAll(filepath.Join(root, "11"))
	w.scan()
	if w.last != "" {
		t.Fatalf("scenario after exit = %q", w.last)
	}
}

func TestMatchApp(t *testing.T) {
	for _, c := range []struct {
		app, name string
		want      bool
	}{
		{"eldenring.exe", "eldenring.exe", true},
		{"eldenring.exe", "EldenRing", true},
		{"eldenring", "eldenring.exe", true},
		{"cyberpunk2077.exe", "Cyberpunk2077.e", true}, // comm truncated to 15
		{"cyberpunk2077.exe", "cyberpunk", false},
		{"steam", "steamwebhelper", false},
	} {
		if got := scenarios.MatchApp(c.app, c.name); got != c.want {
			t.Errorf("MatchApp(%q, %q) = %v", c.app, c.name, got)
		}
	}
}
