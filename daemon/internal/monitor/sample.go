// Package monitor builds the Monitor.Get / Sample dictionary from /proc,
// /sys and the hardware backends (docs/api-dbus.md §6).
package monitor

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"nitrosense/daemon/internal/hw"
)

// maxGPUPowerW is the sanity bound for GPU power readings.
const maxGPUPowerW = 400.0

// Sampler keeps the deltas needed for CPU usage, power and network rates.
type Sampler struct {
	hw *hw.Hardware

	mu        sync.Mutex
	lastIdle  uint64
	lastTotal uint64
	lastEnerg uint64
	lastRx    uint64
	lastTx    uint64
	lastTime  time.Time
}

// New returns a sampler over the given hardware.
func New(h *hw.Hardware) *Sampler { return &Sampler{hw: h} }

// Sample returns one reading. Keys are absent when their source is.
func (s *Sampler) Sample() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	dt := now.Sub(s.lastTime).Seconds()
	first := s.lastTime.IsZero()
	out := map[string]interface{}{"ts": uint64(now.UnixMilli())}

	cpu := map[string]interface{}{}
	if idle, total, ok := cpuTimes(); ok {
		if !first && total > s.lastTotal {
			busy := float64((total-s.lastTotal)-(idle-s.lastIdle)) / float64(total-s.lastTotal)
			cpu["usage"] = uint32(math.Round(busy * 100))
		}
		s.lastIdle, s.lastTotal = idle, total
	}
	if f, ok := cpuFreqMHz(); ok {
		cpu["freq"] = f
	}
	if e, ok := readU64("/sys/class/powercap/intel-rapl:0/energy_uj"); ok {
		if !first && dt > 0 && e >= s.lastEnerg {
			cpu["power"] = float64(e-s.lastEnerg) / 1e6 / dt
		}
		s.lastEnerg = e
	}
	if s.hw.Thermal != nil {
		if t, err := s.hw.Thermal.Temps(); err == nil {
			if v, ok := t["cpu"]; ok {
				cpu["temp"] = v
			}
		}
	}
	if len(cpu) > 0 {
		out["cpu"] = cpu
	}

	if s.hw.GPU != nil {
		if g, err := s.hw.GPU.Sample(); err == nil {
			gpu := map[string]interface{}{"temp": g.Temp, "usage": g.Usage, "clock": g.Clock, "mem_clock": g.MemClock}
			// The first reading after the dGPU wakes from D3cold reports a
			// bogus power draw (590 W seen on the RTX 4060); drop it.
			if g.Power > 0 && g.Power < maxGPUPowerW {
				gpu["power"] = g.Power
			}
			out["gpu"] = gpu
		}
	}

	if s.hw.Fans != nil {
		if st, err := s.hw.Fans.State(); err == nil {
			fans := map[string]interface{}{}
			for id, f := range st {
				fans[id] = f.RPM
			}
			out["fans"] = fans
		}
	}

	if used, total, ok := memInfo(); ok {
		out["ram"] = map[string]interface{}{"used": used, "total": total}
	}

	if rx, tx, ok := netTotals(); ok {
		if !first && dt > 0 && rx >= s.lastRx && tx >= s.lastTx {
			out["net"] = map[string]interface{}{
				"rx": uint64(float64(rx-s.lastRx) / dt),
				"tx": uint64(float64(tx-s.lastTx) / dt),
			}
		}
		s.lastRx, s.lastTx = rx, tx
	}

	if s.hw.Battery != nil {
		p, perr := s.hw.Battery.Power()
		h, herr := s.hw.Battery.Health()
		if perr == nil && herr == nil {
			b := map[string]interface{}{"percent": p.Percent, "status": h.Status,
				"voltage": h.Voltage, "current": h.Current}
			if !math.IsNaN(h.Temp) {
				b["temp"] = h.Temp
			}
			out["battery"] = b
		}
	}

	s.lastTime = now
	return out
}

func cpuTimes() (idle, total uint64, ok bool) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return 0, 0, false
	}
	fields := strings.Fields(sc.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, false
	}
	for i, fld := range fields[1:] {
		v, _ := strconv.ParseUint(fld, 10, 64)
		total += v
		if i == 3 || i == 4 { // idle, iowait
			idle += v
		}
	}
	return idle, total, true
}

func cpuFreqMHz() (uint32, bool) {
	paths, _ := filepath.Glob("/sys/devices/system/cpu/cpu*/cpufreq/scaling_cur_freq")
	if len(paths) == 0 {
		return 0, false
	}
	var sum, n uint64
	for _, p := range paths {
		if v, ok := readU64(p); ok {
			sum += v
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return uint32(sum / n / 1000), true
}

func memInfo() (used, total uint64, ok bool) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	var avail uint64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		v, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			total = v * 1024
		case "MemAvailable:":
			avail = v * 1024
		}
	}
	if total == 0 {
		return 0, 0, false
	}
	return total - avail, total, true
}

func netTotals() (rx, tx uint64, ok bool) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		i := strings.Index(line, ":")
		if i < 0 {
			continue
		}
		name := strings.TrimSpace(line[:i])
		if name == "lo" || strings.HasPrefix(name, "virbr") || strings.HasPrefix(name, "docker") || strings.HasPrefix(name, "veth") {
			continue
		}
		fields := strings.Fields(line[i+1:])
		if len(fields) < 9 {
			continue
		}
		r, _ := strconv.ParseUint(fields[0], 10, 64)
		t, _ := strconv.ParseUint(fields[8], 10, 64)
		rx += r
		tx += t
		ok = true
	}
	return rx, tx, ok
}

func readU64(path string) (uint64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	return v, err == nil
}
