// nitro-control-center is the desktop app of acerctl: a window over the
// acerd D-Bus API (docs/api-dbus.md), with the same rules and errors as the
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

// winMargin is the transparent border around the page where the frontend
// draws the window shadow (--win-margin in style.css); window sizes include
// it. Only with client-side decorations.
const winMargin = 18

func main() {
	// ACERCTL_SESSION=1 does the same as --session under `wails dev`, whose
	// own flag parser rejects unknown flags.
	session := flag.Bool("session", os.Getenv("ACERCTL_SESSION") == "1", "talk to a daemon on the session bus (development)")
	minimized := flag.Bool("minimized", false, "start minimized (autostart)")
	flag.Parse()

	csd := clientDecorations()
	app := NewApp(*session, csd)
	margin := 0
	bg := &options.RGBA{R: 0x0f, G: 0x0d, B: 0x11, A: 255} // --bg in style.css
	if csd {
		margin = winMargin
		// Transparent behind the page so the frontend can round the corners.
		bg = &options.RGBA{R: 0, G: 0, B: 0, A: 0}
	}
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
		Width:              1320 + 2*margin,
		Height:             780 + 2*margin,
		MinWidth:           960 + 2*margin,
		MinHeight:          640 + 2*margin,
		WindowStartState:   start,
		Frameless:          csd, // title bar drawn by the frontend (titlebar.go)
		AssetServer:        &assetserver.Options{Assets: assets},
		BackgroundColour:   bg,
		OnStartup:          app.startup,
		OnShutdown:         app.shutdown,
		SingleInstanceLock: single,
		Linux: &linux.Options{
			Icon:                icon,
			WindowIsTranslucent: csd,
			ProgramName:         appID, // Wayland app_id = .desktop file name
			WebviewGpuPolicy:    linux.WebviewGpuPolicyOnDemand,
		},
		Bind: []interface{}{app},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "nitro-control-center:", err)
		os.Exit(1)
	}
}
