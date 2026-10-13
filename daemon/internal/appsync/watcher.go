package appsync

import (
	"bufio"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// Wine/Proton set the Windows exe name (comm, argv) after the exec
	// event, so a wine process that did not match is looked at again.
	wineRecheck = 2 * time.Second
	// Exit events can be lost on overrun; tracked pids are checked this often.
	liveEvery = 30 * time.Second
)

// Watcher reports which scenario the apps of the active session user call for.
type Watcher struct {
	// ForApp returns the scenario listing one of the process names, or "".
	ForApp func(names []string) string
	// OnChange receives the scenario of the most recently started matching
	// app still running, or "" once the last one exits. It runs on its own
	// goroutine; when it is slow only the latest value is delivered.
	OnChange func(scenario string)

	procRoot string // "/proc"; tests point it elsewhere
	seatFile string // logind's seat0 state, holds ACTIVE_UID

	mu      sync.Mutex // guards t and last (reader and timers goroutines)
	t       tracker
	last    string
	pending chan string
	recheck chan int
	rescan  chan struct{}
}

// New builds a watcher over the real /proc.
func New(forApp func([]string) string, onChange func(string)) *Watcher {
	return &Watcher{ForApp: forApp, OnChange: onChange, procRoot: "/proc", seatFile: "/run/systemd/seats/seat0"}
}

// Start opens the proc connector (the only step that can fail), scans the
// running processes, delivers the first OnChange and keeps watching until
// ctx is done.
func (w *Watcher) Start(ctx context.Context) error {
	c, err := openProcConn()
	if err != nil {
		return err
	}
	w.pending = make(chan string, 1)
	w.recheck = make(chan int, 16)
	w.rescan = make(chan struct{}, 1)
	w.last = "\x00" // the first changed() always delivers, even ""
	go w.deliver(ctx)

	go w.read(ctx, c)
	go w.timers(ctx)
	return nil
}

// read handles the events on the reader goroutine itself: a hand-off to
// another goroutine per datagram costs more than the matching.
func (w *Watcher) read(ctx context.Context, c *procConn) {
	defer c.close()
	w.mu.Lock()
	w.scan()
	w.changed()
	w.mu.Unlock()
	buf := make([]byte, 8192)
	for ctx.Err() == nil {
		evs, err := c.read(buf)
		switch {
		case err == errOverrun:
			slog.Warn("app sync: events lost; rescanning /proc")
			w.mu.Lock()
			w.scan()
			w.mu.Unlock()
		case err != nil:
			slog.Warn("app sync: proc connector read", "err", err)
			time.Sleep(time.Second)
		case len(evs) > 0:
			w.mu.Lock()
			for _, e := range evs {
				if e.exec {
					w.exec(e.pid, false)
				} else {
					w.exit(e.pid)
				}
			}
			w.mu.Unlock()
		}
	}
}

// timers runs the Wine rechecks, rescans and the liveness check.
func (w *Watcher) timers(ctx context.Context) {
	live := time.NewTicker(liveEvery)
	defer live.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case pid := <-w.recheck:
			w.mu.Lock()
			w.exec(pid, true)
			w.mu.Unlock()
		case <-w.rescan:
			w.mu.Lock()
			w.t = tracker{}
			w.scan()
			w.changed()
			w.mu.Unlock()
		case <-live.C:
			w.mu.Lock()
			for _, pid := range w.t.pids() {
				if _, err := os.Stat(filepath.Join(w.procRoot, strconv.Itoa(pid))); err != nil {
					w.exit(pid)
				}
			}
			w.mu.Unlock()
		}
	}
}

// Rescan re-matches every running process, after the scenarios changed.
func (w *Watcher) Rescan() {
	select {
	case w.rescan <- struct{}{}:
	default:
	}
}

// scan rebuilds the tracked set from /proc (start-up and lost events).
func (w *Watcher) scan() {
	alive := map[int]bool{}
	entries, _ := os.ReadDir(w.procRoot)
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		alive[pid] = true
		if !w.t.has(pid) {
			w.exec(pid, true)
		}
	}
	for _, pid := range w.t.pids() {
		if !alive[pid] {
			w.exit(pid)
		}
	}
}

// exec looks at a process that just started (or is looked at again).
func (w *Watcher) exec(pid int, again bool) {
	names, wine := w.names(pid)
	if len(names) == 0 {
		return
	}
	scen := w.ForApp(names)
	if scen == "" {
		if w.t.remove(pid) { // re-exec'd into something else
			w.changed()
		}
		if wine && !again {
			time.AfterFunc(wineRecheck, func() {
				select {
				case w.recheck <- pid:
				default:
				}
			})
		}
		return
	}
	if !w.userProcess(pid) {
		return
	}
	if w.t.add(pid, scen) {
		slog.Info("app sync: app started", "pid", pid, "names", names, "scenario", scen)
	}
	w.changed()
}

func (w *Watcher) exit(pid int) {
	if w.t.remove(pid) {
		slog.Info("app sync: app exited", "pid", pid)
		w.changed()
	}
}

func (w *Watcher) changed() {
	top := w.t.top()
	if top == w.last {
		return
	}
	w.last = top
	select { // keep only the latest value
	case <-w.pending:
	default:
	}
	w.pending <- top
}

func (w *Watcher) deliver(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case s := <-w.pending:
			w.OnChange(s)
		}
	}
}

// names returns comm and, when comm is not enough (truncated to 15 chars or
// a Wine loader), the argv[0] basename plus argv[1] after a Wine loader;
// wine tells whether the process looks like Wine/Proton.
func (w *Watcher) names(pid int) (names []string, wine bool) {
	dir := w.procRoot + "/" + strconv.Itoa(pid)
	c, ok := readSmall(dir + "/comm")
	if !ok {
		return nil, false
	}
	names = append(names, c)
	if len(c) < 15 && !isWine(c) {
		return names, false // the common case: one read per exec
	}
	cmd, _ := os.ReadFile(filepath.Join(dir, "cmdline"))
	argv := strings.Split(strings.TrimRight(string(cmd), "\x00"), "\x00")
	if len(argv) > 0 && argv[0] != "" {
		a0 := base(argv[0])
		names = append(names, a0)
		if isWine(a0) && len(argv) > 1 {
			names = append(names, base(argv[1]))
		}
	}
	for _, n := range names {
		wine = wine || isWine(n)
	}
	return names, wine
}

// userProcess tells whether pid belongs to the user of the active session
// on seat0; without logind data any regular user (uid >= 1000) counts.
func (w *Watcher) userProcess(pid int) bool {
	uid := w.procUID(pid)
	if uid < 0 {
		return false
	}
	if active := w.activeUID(); active >= 0 {
		return uid == active
	}
	return uid >= 1000
}

func (w *Watcher) procUID(pid int) int {
	f, err := os.Open(filepath.Join(w.procRoot, strconv.Itoa(pid), "status"))
	if err != nil {
		return -1
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(sc.Text(), "Uid:"); ok {
			if fs := strings.Fields(rest); len(fs) > 0 {
				if uid, err := strconv.Atoi(fs[0]); err == nil {
					return uid
				}
			}
		}
	}
	return -1
}

func (w *Watcher) activeUID() int {
	b, err := os.ReadFile(w.seatFile)
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(line, "ACTIVE_UID="); ok {
			if uid, err := strconv.Atoi(v); err == nil {
				return uid
			}
		}
	}
	return -1
}

// readSmall reads a short /proc file with plain syscalls: os.ReadFile adds an
// fstat and a netpoller registration, which dominate on every exec.
func readSmall(path string) (string, bool) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return "", false
	}
	var buf [64]byte
	n, err := syscall.Read(fd, buf[:])
	syscall.Close(fd)
	if err != nil || n <= 0 {
		return "", false
	}
	return strings.TrimSpace(string(buf[:n])), true
}

func base(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

func isWine(n string) bool {
	n = strings.ToLower(n)
	return strings.Contains(n, "wine") || strings.Contains(n, "preloader") || strings.Contains(n, "proton")
}

// tracker keeps the matching processes in start order; the newest one
// decides the scenario.
type tracker struct{ list []entry }

type entry struct {
	pid  int
	scen string
}

// add records pid (moving it to the top); true when it was new.
func (t *tracker) add(pid int, scen string) bool {
	isNew := !t.remove(pid)
	t.list = append(t.list, entry{pid, scen})
	return isNew
}

func (t *tracker) remove(pid int) bool {
	for i, e := range t.list {
		if e.pid == pid {
			t.list = append(t.list[:i], t.list[i+1:]...)
			return true
		}
	}
	return false
}

func (t *tracker) has(pid int) bool {
	for _, e := range t.list {
		if e.pid == pid {
			return true
		}
	}
	return false
}

func (t *tracker) top() string {
	if len(t.list) == 0 {
		return ""
	}
	return t.list[len(t.list)-1].scen
}

func (t *tracker) pids() []int {
	out := make([]int, len(t.list))
	for i, e := range t.list {
		out[i] = e.pid
	}
	return out
}
