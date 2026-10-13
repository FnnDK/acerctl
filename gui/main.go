// nitro-control-center is the desktop app of nitroctl: a window over the
// nitrod D-Bus API (docs/api-dbus.md), with the same rules and errors as the
// CLI. It holds no hardware logic of its own.
package main

import (
	"embed"
	"flag"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

const appID = "io.github.fnndk.NitroControlCenter"

func main() {
	// NITROCTL_SESSION=1 does the same as --session under `wails dev`, whose
	// own flag parser rejects unknown flags.
	session := flag.Bool("session", os.Getenv("NITROCTL_SESSION") == "1", "talk to a daemon on the session bus (development)")
	minimized := flag.Bool("minimized", false, "start minimized (autostart)")
	flag.Parse()

	app := NewApp(*session)
	start := options.Normal
	if *minimized {
		start = options.Minimised
	}
	// Single instance, except for development (--session), so a dev build can
	// run next to the installed app.
	var single *options.SingleInstanceLock
	if !*session {
		single = &options.SingleInstanceLock{UniqueId: appID, OnSecondInstanceLaunch: app.secondInstance}
	}
	err := wails.Run(&options.App{
		Title:              "Nitro Control Center",
		Width:              1180,
		Height:             760,
		MinWidth:           960,
		MinHeight:          640,
		WindowStartState:   start,
		AssetServer:        &assetserver.Options{Assets: assets},
		BackgroundColour:   &options.RGBA{R: 16, G: 14, B: 18, A: 255},
		OnStartup:          app.startup,
		OnShutdown:         app.shutdown,
		SingleInstanceLock: single,
		Linux: &linux.Options{
			Icon:             icon,
			ProgramName:      appID, // Wayland app_id = .desktop file name
			WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand,
		},
		Bind: []interface{}{app},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "nitro-control-center:", err)
		os.Exit(1)
	}
}
