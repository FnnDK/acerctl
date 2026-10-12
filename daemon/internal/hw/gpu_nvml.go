package hw

import (
	"fmt"
	"log/slog"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

// nvmlGPU reads the discrete GPU through libnvidia-ml.so (loaded at run
// time by go-nvml); without the library or the driver it is simply absent.
type nvmlGPU struct{ dev nvml.Device }

func probeNVML() *nvmlGPU {
	if ret := nvml.Init(); ret != nvml.SUCCESS {
		slog.Info("NVML unavailable", "ret", nvml.ErrorString(ret))
		return nil
	}
	n, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS || n == 0 {
		nvml.Shutdown()
		return nil
	}
	dev, ret := nvml.DeviceGetHandleByIndex(0)
	if ret != nvml.SUCCESS {
		nvml.Shutdown()
		return nil
	}
	name, _ := dev.GetName()
	slog.Info("NVML ready", "gpu", name)
	return &nvmlGPU{dev: dev}
}

func (g *nvmlGPU) Sample() (GPUSample, error) {
	var s GPUSample
	temp, ret := g.dev.GetTemperature(nvml.TEMPERATURE_GPU)
	if ret != nvml.SUCCESS {
		return s, fmt.Errorf("nvml temperature: %s", nvml.ErrorString(ret))
	}
	s.Temp = float64(temp)
	if u, ret := g.dev.GetUtilizationRates(); ret == nvml.SUCCESS {
		s.Usage = u.Gpu
	}
	if c, ret := g.dev.GetClockInfo(nvml.CLOCK_GRAPHICS); ret == nvml.SUCCESS {
		s.Clock = c
	}
	if c, ret := g.dev.GetClockInfo(nvml.CLOCK_MEM); ret == nvml.SUCCESS {
		s.MemClock = c
	}
	if mw, ret := g.dev.GetPowerUsage(); ret == nvml.SUCCESS {
		s.Power = float64(mw) / 1000
	}
	return s, nil
}
