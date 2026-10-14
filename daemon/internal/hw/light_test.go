package hw

import (
	"reflect"
	"testing"
)

var testKbd = LightDevice{ID: "keyboard", Effects: []string{"static", "breathing", "neon", "wave", "shifting"},
	Zones: 4, Speed: Range{1, 9}, Directions: []string{"left", "right"}, Readback: true}

func TestNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   LightState
		want LightState
	}{
		{"one colour spread over the zones",
			LightState{Effect: "static", Brightness: 80, Colors: []string{"FF0000"}},
			LightState{Effect: "static", Brightness: 80, Colors: []string{"#ff0000", "#ff0000", "#ff0000", "#ff0000"}}},
		{"zones kept, speed and direction cleared",
			LightState{Effect: "static", Speed: 4, Direction: "left", Brightness: 10, Colors: []string{"#000001", "#000002", "#000003", "#000004"}},
			LightState{Effect: "static", Brightness: 10, Colors: []string{"#000001", "#000002", "#000003", "#000004"}}},
		{"breathing keeps the first colour, default speed",
			LightState{Effect: "breathing", Brightness: 50, Colors: []string{"#00ff00", "#0000ff"}},
			LightState{Effect: "breathing", Speed: 5, Brightness: 50, Colors: []string{"#00ff00"}}},
		{"neon has no colour",
			LightState{Effect: "neon", Speed: 9, Brightness: 100, Colors: []string{"#00ff00"}},
			LightState{Effect: "neon", Speed: 9, Brightness: 100}},
		{"wave gets a default direction",
			LightState{Effect: "wave", Speed: 1, Brightness: 100},
			LightState{Effect: "wave", Speed: 1, Brightness: 100, Direction: "left"}},
		{"shifting without a colour gets white",
			LightState{Effect: "shifting", Speed: 2, Direction: "right", Brightness: 100},
			LightState{Effect: "shifting", Speed: 2, Direction: "right", Brightness: 100, Colors: []string{"#ffffff"}}},
	}
	for _, c := range cases {
		got, err := testKbd.Normalize(c.in)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestNormalizeRejects(t *testing.T) {
	bad := []LightState{
		{Effect: "zoom", Brightness: 50},                                                                            // not on this device
		{Effect: "static", Brightness: 101},                                                                         // not a percentage
		{Effect: "breathing", Speed: 10, Brightness: 50},                                                            // outside 1-9
		{Effect: "wave", Speed: 3, Direction: "up", Brightness: 50},                                                 // unknown direction
		{Effect: "static", Brightness: 50, Colors: []string{"#12345"}},                                              // not #rrggbb
		{Effect: "static", Brightness: 50, Colors: []string{"#000001", "#000002", "#000003", "#000004", "#000005"}}, // more colours than zones
		{Effect: "breathing", Brightness: 50, Colors: []string{"#ff00zz"}},                                          // not hex
	}
	for _, st := range bad {
		if _, err := testKbd.Normalize(st); err == nil {
			t.Errorf("%+v: accepted", st)
		}
	}
}

func TestNoDirectionsOnDevice(t *testing.T) {
	bar := LightDevice{ID: "logo", Effects: []string{"wave"}, Speed: Range{1, 5}}
	got, err := bar.Normalize(LightState{Effect: "wave", Direction: "left", Brightness: 1})
	if err != nil || got.Direction != "" || got.Speed != 3 {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestSimulatedModels(t *testing.T) {
	for _, m := range SimModels() {
		h, err := Simulate(m)
		if err != nil {
			t.Fatal(err)
		}
		if h.Lights == nil {
			continue
		}
		caps := h.Capabilities()["lighting"].(map[string]interface{})
		for _, dev := range h.Lights.Devices() {
			if _, ok := caps[dev.ID]; !ok {
				t.Errorf("%s: %s missing from capabilities", m, dev.ID)
			}
			for _, e := range dev.Effects {
				if _, ok := Effects[e]; !ok {
					t.Errorf("%s: %s has unknown effect %q", m, dev.ID, e)
				}
			}
			if _, err := dev.Normalize(dev.Default()); err != nil {
				t.Errorf("%s: %s default: %v", m, dev.ID, err)
			}
			st, err := h.Lights.Get(dev.ID)
			if dev.Readback != (err == nil) {
				t.Errorf("%s: %s readback %v but Get: %v", m, dev.ID, dev.Readback, err)
			}
			if err == nil {
				if _, err := dev.Normalize(st); err != nil {
					t.Errorf("%s: %s initial state: %v", m, dev.ID, err)
				}
			}
		}
	}
}
