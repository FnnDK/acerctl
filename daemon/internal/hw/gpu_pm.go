package hw

import (
	"os"
	"path/filepath"
	"strings"
)

// pmGatedGPU skips the reader while the dGPU is runtime-suspended (D3cold
// on Optimus laptops). Any NVML or nvidia-smi query wakes it: ~2 s and
// ~1.4 s of kernel CPU in the driver on the ANV15-52, plus 1-10 W while it
// stays up, and periodic queries keep resetting its idle timer.
type pmGatedGPU struct {
	GPU
	status string // .../power/runtime_status of the NVIDIA display controller
}

// gateRuntimePM wraps g when the NVIDIA GPU has runtime PM; without it the
// GPU is always on and g is returned as is.
func gateRuntimePM(g GPU) GPU {
	devs, _ := filepath.Glob("/sys/bus/pci/devices/*")
	for _, d := range devs {
		vendor, _ := os.ReadFile(filepath.Join(d, "vendor"))
		class, _ := os.ReadFile(filepath.Join(d, "class"))
		if strings.TrimSpace(string(vendor)) != "0x10de" || !strings.HasPrefix(string(class), "0x03") {
			continue
		}
		st := filepath.Join(d, "power", "runtime_status")
		if exists(st) {
			return &pmGatedGPU{GPU: g, status: st}
		}
	}
	return g
}

// Sample reads the GPU only when it is already awake. Not through readStr:
// this file is not a WMI attribute and must not wait for wmiMu.
func (g *pmGatedGPU) Sample() (GPUSample, error) {
	if b, err := os.ReadFile(g.status); err == nil && strings.TrimSpace(string(b)) != "active" {
		return GPUSample{Off: true}, nil
	}
	return g.GPU.Sample()
}
