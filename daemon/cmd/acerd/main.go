// acerd is the acerctl daemon: it owns io.github.fnndk.Acerd1 on the
// system bus and drives the kernel interfaces described in docs/api-dbus.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"acerctl/daemon/internal/hw"
	"acerctl/daemon/internal/scenarios"
	"acerctl/daemon/internal/service"
	"acerctl/daemon/internal/state"
)

// Version is set by the build (-ldflags "-X main.Version=...").
var Version = "0.1.0-dev"

func main() {
	var (
		session  = flag.Bool("session", false, "use the session bus instead of the system bus (development)")
		noPolkit = flag.Bool("no-polkit", false, "skip polkit authorization (development)")
		dryRun   = flag.Bool("dry-run", false, "log sysfs writes instead of performing them")
		simulate = flag.String("simulate", "", "simulate a model instead of probing the hardware: ANV15-52, or the Predators PHN16-72 (4-zone keyboard, logo) and PH16-71 (USB keyboard, light bar)")
		statePth = flag.String("state", state.DefaultPath, "state file")
		senseKey = flag.String("sense-key", "cycle", "default action of the Sense key until one is chosen over D-Bus: cycle (thermal profile) or app (the session agent opens the app; 'none' is an alias)")
		debug    = flag.Bool("debug", false, "debug logging")
		showVer  = flag.Bool("version", false, "print the version and exit")
	)
	flag.Parse()
	if *showVer {
		fmt.Println(Version)
		return
	}
	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	hw.DryRun = *dryRun

	cfg := service.Config{Session: *session, NoPolkit: *noPolkit || *session, Version: Version, SenseKey: *senseKey}
	var h *hw.Hardware
	if *simulate != "" {
		var err error
		if h, err = hw.Simulate(*simulate); err != nil {
			slog.Error("simulate", "err", err)
			os.Exit(2)
		}
		slog.Info("simulating", "model", *simulate)
	} else {
		h = hw.Probe()
		cfg.Reprobe = hw.Probe
	}

	st := state.Load(*statePth)
	sc := scenarios.Load(filepath.Join(filepath.Dir(*statePth), "scenarios.json"))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := service.New(cfg, h, st, sc).Run(ctx); err != nil {
		slog.Error("acerd", "err", err)
		os.Exit(1)
	}
}
