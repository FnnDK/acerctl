package main

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"acerctl/daemon/flavor"
)

// The app icons are rendered from the SVG sources in assets/logo (needs
// rsvg-convert, from librsvg).
//go:generate rsvg-convert -w 512 -h 512 -o build/appicon.png ../assets/logo/n-icon-cut.svg
//go:generate rsvg-convert -w 512 -h 512 -o build/appicon-a.png ../assets/logo/a-icon-cut.svg

// brand is this build's flavor: the installed name of the executable decides
// (nitro-control-center or acer-control-center); ACERCTL_FLAVOR overrides it.
var brand = flavor.Detect()

func appIcon() []byte {
	if brand.ID == flavor.Acer.ID {
		return iconAcer
	}
	return iconNitro
}

// flavorMiddleware marks the page before the frontend runs, as
// <html data-flavor="…" data-app-name="…">, so the first paint already has
// the flavor's name, mark and colours (frontend/src/flavor.ts).
func flavorMiddleware(next http.Handler) http.Handler {
	attrs := fmt.Sprintf(`<html data-flavor="%s" data-app-name="%s"`, brand.ID, html.EscapeString(brand.Name))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			next.ServeHTTP(w, r)
			return
		}
		r.Header.Del("Accept-Encoding") // keep the page uncompressed to edit it
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)
		body := strings.Replace(rec.Body.String(), "<html", attrs, 1)
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(rec.Code)
		_, _ = io.WriteString(w, body)
	})
}

var _ assetserver.Middleware = flavorMiddleware
