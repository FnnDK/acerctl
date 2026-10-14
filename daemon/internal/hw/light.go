package hw

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// EffectParams says which LightState fields an effect uses.
type EffectParams struct {
	Color     bool
	Speed     bool
	Direction bool
}

// Effects are the lighting effects the daemon knows. The first eight are the
// ones the gaming WMI interface numbers 0-7 for the EC keyboard and light bar
// (method 20 byte 0, docs/firmware-wmi.md §11); the rest are the other
// effects of the PH16-71's USB keyboard, with the fields predator-sense's
// keyboard_rgb.rs gives them.
var Effects = map[string]EffectParams{
	"static":    {Color: true},
	"breathing": {Color: true, Speed: true},
	"neon":      {Speed: true},
	"wave":      {Speed: true, Direction: true},
	"shifting":  {Color: true, Speed: true, Direction: true},
	"zoom":      {Color: true, Speed: true},
	"meteor":    {Color: true, Speed: true},
	"twinkling": {Color: true, Speed: true},
	"snake":     {Color: true, Speed: true},
	"ripple":    {Color: true, Speed: true},
	"rainbow":   {Speed: true},
	"rain":      {Color: true, Speed: true},
	"lightning": {Color: true, Speed: true},
	"spot":      {Color: true, Speed: true},
	"stars":     {Color: true, Speed: true},
	"fireball":  {Color: true, Speed: true},
	"snow":      {Color: true, Speed: true},
	"heartbeat": {Color: true, Speed: true},
}

// LightDevice is one RGB lighting device and what its firmware accepts.
type LightDevice struct {
	ID         string   // keyboard, lightbar, logo
	Effects    []string // keys of Effects, in display order
	Zones      uint32   // "static" takes one colour per zone; 0 = one colour for the device
	Speed      Range    // effect speed in firmware steps (1-9 on the EC keyboard)
	Directions []string // "left", "right"; empty when no effect has a direction
	// Readback is false when the device cannot be read (USB keyboards): the
	// daemon then shows what it last applied.
	Readback bool
}

// LightState is the effect of one device. Fields an effect does not use are
// zero (Normalize clears them).
type LightState struct {
	Effect     string   `json:"effect"`
	Speed      uint32   `json:"speed,omitempty"`
	Brightness uint32   `json:"brightness"` // percent
	Direction  string   `json:"direction,omitempty"`
	Colors     []string `json:"colors,omitempty"` // "#rrggbb": one per zone for a zoned "static", else one
}

// Lighting is the RGB keyboard and the other LEDs (light bar, logo).
type Lighting interface {
	Devices() []LightDevice
	// Get reads a device back; ErrNoReadback when Readback is false.
	Get(id string) (LightState, error)
	Set(id string, st LightState) error
}

// ErrNoReadback is Lighting.Get on a device that cannot be read.
var ErrNoReadback = errors.New("the device cannot be read back")

var hexColor = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// Default is the state shown for a device before anything is known: its first
// effect, white, half brightness, middle speed.
func (d LightDevice) Default() LightState {
	st, _ := d.Normalize(LightState{Effect: d.Effects[0], Brightness: 50, Colors: []string{"#ffffff"}})
	return st
}

// Normalize fits a state to the device: it fills a missing speed, direction
// or colour from the defaults, spreads one colour over the zones of a zoned
// "static" and keeps only the first colour elsewhere, clears what the effect
// does not use, and rejects values the firmware would not take.
func (d LightDevice) Normalize(st LightState) (LightState, error) {
	known := false
	for _, e := range d.Effects {
		known = known || e == st.Effect
	}
	if !known {
		return st, fmt.Errorf("effect %q not in %v", st.Effect, d.Effects)
	}
	p := Effects[st.Effect]
	if st.Brightness > 100 {
		return st, fmt.Errorf("brightness %d is not a percentage", st.Brightness)
	}

	if !p.Speed {
		st.Speed = 0
	} else if st.Speed == 0 {
		st.Speed = (d.Speed.Min + d.Speed.Max + 1) / 2
	} else if st.Speed < d.Speed.Min || st.Speed > d.Speed.Max {
		return st, fmt.Errorf("speed %d outside %d-%d", st.Speed, d.Speed.Min, d.Speed.Max)
	}

	switch {
	case !p.Direction || len(d.Directions) == 0:
		st.Direction = ""
	case st.Direction == "":
		st.Direction = d.Directions[0]
	default:
		ok := false
		for _, v := range d.Directions {
			ok = ok || v == st.Direction
		}
		if !ok {
			return st, fmt.Errorf("direction %q not in %v", st.Direction, d.Directions)
		}
	}

	if !p.Color {
		st.Colors = nil
		return st, nil
	}
	colors := make([]string, 0, len(st.Colors))
	for _, c := range st.Colors {
		c = strings.ToLower(c)
		if !strings.HasPrefix(c, "#") {
			c = "#" + c
		}
		if !hexColor.MatchString(c) {
			return st, fmt.Errorf("colour %q is not #rrggbb", c)
		}
		colors = append(colors, c)
	}
	if len(colors) == 0 {
		colors = []string{"#ffffff"}
	}
	n := 1
	if st.Effect == "static" && d.Zones > 0 {
		n = int(d.Zones)
	}
	if len(colors) > n {
		if n > 1 {
			return st, fmt.Errorf("%d colours for %d zones", len(colors), n)
		}
		colors = colors[:1]
	}
	for len(colors) < n {
		colors = append(colors, colors[len(colors)-1])
	}
	st.Colors = colors
	return st, nil
}

// lightCaps is the System.Capabilities entry of the lighting devices.
func lightCaps(devs []LightDevice) map[string]interface{} {
	out := map[string]interface{}{}
	for _, d := range devs {
		var color, speed, dir []string
		for _, e := range d.Effects {
			p := Effects[e]
			if p.Color {
				color = append(color, e)
			}
			if p.Speed {
				speed = append(speed, e)
			}
			if p.Direction && len(d.Directions) > 0 {
				dir = append(dir, e)
			}
		}
		out[d.ID] = map[string]interface{}{
			"effects":        d.Effects,
			"with_color":     nonNilStrings(color),
			"with_speed":     nonNilStrings(speed),
			"with_direction": nonNilStrings(dir),
			"zones":          d.Zones,
			"speed":          d.Speed,
			"directions":     nonNilStrings(d.Directions),
			"readback":       d.Readback,
		}
	}
	return out
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
